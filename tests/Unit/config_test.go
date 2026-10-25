package unit_test

import (
	"fmt"
	"strings"
	"testing"

	"github.com/hyz-is/arandu-turnstile"
)

func TestNewRefusesAConfigurationThatCannotWork(t *testing.T) {
	t.Parallel()
	good := turnstile.Config{SiteKey: turnstile.TestSiteKey, SecretKey: turnstile.TestSecretKey}

	for name, mutate := range map[string]func(*turnstile.Config){
		"no site key":        func(c *turnstile.Config) { c.SiteKey = " " },
		"no secret key":      func(c *turnstile.Config) { c.SecretKey = "" },
		"plain http":         func(c *turnstile.Config) { c.Endpoint = "http://challenges.cloudflare.com/turnstile/v0/siteverify" },
		"not an address":     func(c *turnstile.Config) { c.Endpoint = "challenges.cloudflare.com" },
		"hostname with path": func(c *turnstile.Config) { c.Hostnames = []string{"app.example.com/signup"} },
		"empty hostname":     func(c *turnstile.Config) { c.Hostnames = []string{""} },
	} {
		cfg := good
		mutate(&cfg)
		if _, err := turnstile.New(cfg); err == nil {
			t.Errorf("%s: New accepted it", name)
		}
	}

	v, err := turnstile.New(good)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	if v.SiteKey() != turnstile.TestSiteKey {
		t.Errorf("SiteKey = %q", v.SiteKey())
	}
}

func TestTheConfigurationNeverPrintsTheSecret(t *testing.T) {
	t.Parallel()
	cfg := turnstile.Config{SiteKey: "site", SecretKey: secret}
	for _, verb := range []string{"%v", "%+v", "%s", "%#v"} {
		if out := fmt.Sprintf(verb, cfg); strings.Contains(out, secret) {
			t.Errorf("%s prints the secret: %s", verb, out)
		}
	}
}
