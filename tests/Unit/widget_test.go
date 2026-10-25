package unit_test

import (
	"encoding/json"
	"html"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"testing"

	"github.com/arandu-io/hesape/view"
	"github.com/arandu-io/kyse/components"
	"github.com/hyz-is/arandu-turnstile"
	ui "github.com/hyz-is/arandu-turnstile/components"
)

// refusedPage is a page whose previous attempt was refused at the widget.
type refusedPage struct{ message string }

func (p refusedPage) FieldError(name string) string {
	if name == turnstile.FieldName {
		return p.message
	}
	return ""
}

func (refusedPage) OldOr(_, fallback string) string { return fallback }

func attribute(t *testing.T, markup, name string) string {
	t.Helper()
	match := regexp.MustCompile(name + `="([^"]*)"`).FindStringSubmatch(markup)
	if match == nil {
		t.Fatalf("no %s in:\n%s", name, markup)
	}
	return html.UnescapeString(match[1])
}

func TestTheWidgetHandsItsOptionsToTheBehaviour(t *testing.T) {
	t.Parallel()
	markup := string(ui.Widget(ui.WidgetProps{
		SiteKey: turnstile.TestSiteKey, Action: "signup", Theme: "dark", Language: "pt-BR",
	}))

	if got := attribute(t, markup, "data-kyse-behavior"); got != ui.WidgetBehavior {
		t.Errorf("behaviour = %q", got)
	}
	var props map[string]string
	if err := json.Unmarshal([]byte(attribute(t, markup, "data-kyse-props")), &props); err != nil {
		t.Fatalf("props: %v", err)
	}
	want := map[string]string{"sitekey": turnstile.TestSiteKey, "action": "signup", "theme": "dark", "language": "pt-BR"}
	if len(props) != len(want) {
		t.Errorf("props = %v, want only %v", props, want)
	}
	for k, v := range want {
		if props[k] != v {
			t.Errorf("props[%s] = %q, want %q", k, props[k], v)
		}
	}
	if !strings.Contains(markup, `<script src="`+view.Asset(turnstile.ScriptAsset)+`" defer></script>`) {
		t.Errorf("the widget does not load its behaviour:\n%s", markup)
	}
	if strings.Contains(markup, `data-part="error"`) {
		t.Error("a widget nobody was refused at draws an error")
	}
}

func TestTheWidgetDrawsTheRefusal(t *testing.T) {
	t.Parallel()
	markup := string(ui.Widget(ui.WidgetProps{
		SiteKey: turnstile.TestSiteKey, Page: refusedPage{"Confirme que você não é um robô."},
	}))
	if !strings.Contains(markup, ">Confirme que você não é um robô.</p>") {
		t.Errorf("the refusal is not drawn:\n%s", markup)
	}
	if got := attribute(t, markup, "aria-describedby"); got != turnstile.FieldName+"-error" {
		t.Errorf("aria-describedby = %q", got)
	}
}

func TestTheWidgetEscapesWhatItIsGiven(t *testing.T) {
	t.Parallel()
	markup := string(ui.Widget(ui.WidgetProps{
		SiteKey:  `"><script>alert(1)</script>`,
		NoScript: `<img src=x onerror=alert(1)>`,
	}))
	if strings.Contains(markup, "<script>alert") || strings.Contains(markup, "<img") {
		t.Errorf("unescaped input reached the markup:\n%s", markup)
	}
}

// TestEveryPublishedPartIsDrawn holds PartNames to the markup in both
// directions, as every kyse component is held.
func TestEveryPublishedPartIsDrawn(t *testing.T) {
	t.Parallel()
	props := ui.WidgetProps{SiteKey: turnstile.TestSiteKey, Page: refusedPage{"no"}}
	markup := string(ui.Widget(props))

	var drawn []string
	for _, m := range regexp.MustCompile(`data-part="([a-z-]+)"`).FindAllStringSubmatch(markup, -1) {
		if !slices.Contains(drawn, m[1]) {
			drawn = append(drawn, m[1])
		}
	}
	published := props.PartNames()
	slices.Sort(drawn)
	slices.Sort(published)
	if !slices.Equal(drawn, published) {
		t.Errorf("drawn parts %v, published %v", drawn, published)
	}
}

func TestTheCallersPartsReachTheMarkup(t *testing.T) {
	t.Parallel()
	markup := string(ui.Widget(ui.WidgetProps{
		SiteKey: turnstile.TestSiteKey,
		ComponentProps: components.ComponentProps{
			Class: "mt-4",
			Parts: components.Parts{"widget": {Class: "flex justify-center"}},
		},
	}))
	for _, want := range []string{`class="mt-4"`, `class="flex justify-center"`} {
		if !strings.Contains(markup, want) {
			t.Errorf("no %s in:\n%s", want, markup)
		}
	}
}

// The script loads Cloudflare from the one origin the policy allows, and
// registers the behaviour under the name the component writes.
func TestTheBehaviourAgreesWithTheGoSide(t *testing.T) {
	t.Parallel()
	body, err := os.ReadFile(filepath.Join(packageRoot(t), "assets", turnstile.ScriptAsset))
	if err != nil {
		t.Fatal(err)
	}
	script := string(body)
	for _, want := range []string{
		"arandu.ui.define('" + ui.WidgetBehavior + "'",
		"'" + turnstile.Origin + "/turnstile/v0/api.js?render=explicit'",
		"turnstile.reset(", "turnstile.remove(", "htmx:afterRequest",
	} {
		if !strings.Contains(script, want) {
			t.Errorf("the behaviour does not contain %q", want)
		}
	}
	for _, refused := range []string{"eval(", "new Function", "innerHTML", "document.write"} {
		if strings.Contains(script, refused) {
			t.Errorf("the behaviour uses %s", refused)
		}
	}
	for _, m := range regexp.MustCompile(`https://[a-z0-9.-]+`).FindAllString(script, -1) {
		if m != turnstile.Origin {
			t.Errorf("the behaviour names %s, which the policy does not allow", m)
		}
	}
}
