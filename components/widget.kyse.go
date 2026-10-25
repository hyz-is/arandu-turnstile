//go:build kyse

package components

import (
	"github.com/arandu-io/hesape/view"
	kyse "github.com/arandu-io/kyse/components"

	"github.com/hyz-is/arandu-turnstile"
)

@go
// WidgetBehavior is the name the client behaviour is registered under with
// arandu.ui.define.
const WidgetBehavior = "turnstile"

// DefaultNoScriptText is what a browser without script is shown in place of
// the challenge, when WidgetProps leaves NoScript empty.
const DefaultNoScriptText = "Turn on JavaScript to confirm that you are not a robot."

// WidgetProps is the Turnstile challenge, drawn inside a form.
//
// Cloudflare renders the challenge into the widget part and writes its token
// into a hidden input named turnstile.FieldName inside it, so the token is
// sent with the form it sits in and the handler reads it like any other field.
// The component writes the markup, the props the behaviour renders it with and
// the tag that loads the behaviour; the page it is drawn on has to be served
// through turnstile.Headers, or the browser refuses Cloudflare's script.
//
// A refusal comes back from Page under turnstile.FieldName and is drawn below
// the challenge, the way a field draws the message of its rule.
//
// It publishes root, widget, noscript and error.
type WidgetProps struct {
	// ComponentProps is the class, attributes and parts the caller adds.
	kyse.ComponentProps
	// SiteKey is the public key of the widget: Verifier.SiteKey, handed to the
	// page by its handler. Empty draws the markup with nothing to render.
	SiteKey string
	// Action names what the form does -- "signup", "contact" -- and the
	// handler checks the token against the same name. Up to 32 letters,
	// digits, underscores and hyphens.
	Action string
	// CData is a value Cloudflare returns with the verification. Up to 255
	// letters, digits, underscores and hyphens.
	CData string
	// Theme is "auto", "light" or "dark". Empty is Cloudflare's auto.
	Theme string
	// Size is "normal", "flexible" or "compact". Empty is normal.
	Size string
	// Language is "auto", or a language tag such as "pt-BR". Empty is auto.
	Language string
	// Appearance is "always", "execute" or "interaction-only". Empty is
	// always; interaction-only draws nothing until a person has to act.
	Appearance string
	// NoScript is what a browser without script is shown. Empty is
	// DefaultNoScriptText.
	NoScript string
	// Page is where a refusal of the previous attempt is looked up. Nil draws
	// no message.
	Page kyse.Page
}

// Error is the message the previous attempt was refused with, or empty.
func (p WidgetProps) Error() string {
	if p.Page == nil {
		return ""
	}
	return p.Page.FieldError(turnstile.FieldName)
}

// NoScriptText is what a browser without script is shown.
func (p WidgetProps) NoScriptText() string {
	if p.NoScript != "" {
		return p.NoScript
	}
	return DefaultNoScriptText
}

// RootAttrs are the outermost element's attributes, with the client bridge
// filled in when the caller named no behaviour of their own.
//
// The options are handed over under the names Cloudflare's render call takes,
// and one left empty is left out, so Cloudflare's default stands for it.
func (p WidgetProps) RootAttrs() map[string]string {
	if p.Behavior.Name == "" {
		props := map[string]any{"sitekey": p.SiteKey}
		for name, value := range map[string]string{
			"action": p.Action, "cData": p.CData, "theme": p.Theme, "size": p.Size,
			"language": p.Language, "appearance": p.Appearance,
		} {
			if value != "" {
				props[name] = value
			}
		}
		p.Behavior = kyse.Behavior{Name: WidgetBehavior, Props: props}
	}
	return p.ComponentProps.RootAttrs()
}

// ErrorID is the id of the message, which the challenge is described by.
func (p WidgetProps) ErrorID() string { return turnstile.FieldName + "-error" }

// PartNames are the parts this component publishes.
func (p WidgetProps) PartNames() []string { return []string{"root", "widget", "noscript", "error"} }
@endgo

<div
	data-part="root"
	@if(.RootClass() != "")
		class="{{ .RootClass() }}"
	@endif
	@attributes(.RootAttrs())
>
	<div
		data-part="widget"
		@if(.PartClass("widget") != "")
			class="{{ .PartClass("widget") }}"
		@endif
		@if(.Error() != "")
			aria-describedby="{{ .ErrorID() }}"
		@endif
		@attributes(.PartAttrs("widget"))
	></div>
	<noscript>
		<p
			data-part="noscript"
			class="{{ .PartClass("noscript", "text-muted-foreground text-sm") }}"
			@attributes(.PartAttrs("noscript"))
		>{{ .NoScriptText() }}</p>
	</noscript>
	@if(.Error() != "")
		<p
			data-part="error"
			id="{{ .ErrorID() }}"
			class="{{ .PartClass("error", "text-destructive text-sm") }}"
			role="alert"
			@attributes(.PartAttrs("error"))
		>{{ .Error() }}</p>
	@endif
	<script src="{{ view.Asset(turnstile.ScriptAsset) }}" defer></script>
</div>
