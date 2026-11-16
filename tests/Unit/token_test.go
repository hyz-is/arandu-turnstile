package unit_test

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/hyz-is/arandu-turnstile"
)

// TestTheClientScriptHandsOutTokensWithoutAForm runs assets/turnstile.js
// against a stand-in for the page and for Cloudflare's script, in Node, and
// reports each scenario of testdata/token_harness.js as a subtest.
//
// The harness is the only JavaScript the suite runs, because the script is the
// only JavaScript this package ships. It makes no network call and opens no
// browser: it plays Cloudflare by calling the callbacks it was handed.
func TestTheClientScriptHandsOutTokensWithoutAForm(t *testing.T) {
	node, err := exec.LookPath("node")
	if err != nil {
		if os.Getenv("CI") != "" {
			t.Fatal("node is not installed, and the client script is tested with it")
		}
		t.Skip("node is not installed; the client script is not run")
	}

	root := packageRoot(t)
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	command := exec.CommandContext(ctx, node,
		filepath.Join(root, "tests", "Unit", "testdata", "token_harness.js"),
		filepath.Join(root, "assets", turnstile.ScriptAsset))
	var stdout, stderr bytes.Buffer
	command.Stdout, command.Stderr = &stdout, &stderr
	runErr := command.Run()

	type scenario struct {
		Name  string `json:"name"`
		Error string `json:"error"`
	}
	var ran []scenario
	lines := bufio.NewScanner(&stdout)
	lines.Buffer(make([]byte, 0, 64<<10), 1<<20)
	for lines.Scan() {
		var s scenario
		if err := json.Unmarshal(lines.Bytes(), &s); err != nil {
			t.Fatalf("the harness printed %q, which is not a result: %v", lines.Text(), err)
		}
		ran = append(ran, s)
	}

	// A harness that crashed, or that printed nothing, passes no scenario.
	var exit *exec.ExitError
	if runErr != nil && !errors.As(runErr, &exit) {
		t.Fatalf("running node: %v", runErr)
	}
	if len(ran) < 30 {
		t.Fatalf("the harness ran %d scenarios, want at least 30 (exit: %v)\n%s", len(ran), runErr, stderr.String())
	}
	failed := false
	for _, s := range ran {
		t.Run(s.Name, func(t *testing.T) {
			if s.Error != "" {
				failed = true
				t.Error(s.Error)
			}
		})
	}
	if runErr != nil && !failed {
		t.Fatalf("the harness failed with every scenario passing: %v\n%s", runErr, stderr.String())
	}

	// The behaviours the documentation promises are scenarios, by name: renaming
	// or dropping one has to be a decision, not an accident.
	names := map[string]bool{}
	for _, s := range ran {
		names[s.Name] = true
	}
	for _, promised := range []string{
		"a token is asked for without a form, a component or arandu.ui",
		"a token held until it expires is solved again, and the new one is handed over",
		"a script that Cloudflare was not allowed to load rejects as unavailable",
		"a handle gives one token",
		"the widget behaviour renders on mount, renews after a request and removes on destroy",
	} {
		if !names[promised] {
			t.Errorf("the harness has no scenario %q", promised)
		}
	}
}

// The script that hands out tokens is the one that draws the widget, so a page
// that has the component has the API and a page that has only the script tag
// has the API without the component.
func TestTheTokenAPIIsServedWithTheWidgetScript(t *testing.T) {
	t.Parallel()
	body, err := os.ReadFile(filepath.Join(packageRoot(t), "assets", turnstile.ScriptAsset))
	if err != nil {
		t.Fatal(err)
	}
	script := string(body)
	for _, want := range []string{
		"arandu.turnstile = { behavior: 'turnstile', token: token, prepare: prepare }",
		"'response-field': false", "'refresh-expired': 'never'", "'expired-callback'",
		"appearance: 'interaction-only'", "challenge_unavailable", "challenge_failed",
	} {
		if !strings.Contains(script, want) {
			t.Errorf("the script does not contain %q", want)
		}
	}
	// A page with no component and no arandu.ui must still get tokens, so the
	// script may not return before the API is defined.
	if strings.Index(script, "arandu.turnstile = {") > strings.Index(script, "arandu.ui.define('turnstile'") {
		t.Error("the token API is defined after the widget behaviour, which needs arandu.ui")
	}
}
