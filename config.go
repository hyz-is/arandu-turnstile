package turnstile

import (
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// Origin is the one address Cloudflare serves Turnstile from: the script, the
// frame the challenge is drawn in, and the verification endpoint.
//
// The script is loaded from it directly and never copied to this origin.
// Cloudflare changes the file without changing its address, and a copy served
// from anywhere else stops working on the day it does.
const Origin = "https://challenges.cloudflare.com"

// DefaultEndpoint is where a token is verified when Config leaves Endpoint
// empty.
const DefaultEndpoint = Origin + "/turnstile/v0/siteverify"

// FieldName is the form field the widget writes its token into, and the field
// a rejection is reported under.
//
// It is Cloudflare's own default and is not configurable here: the widget, the
// handler that reads the field and the view that draws the message all name it,
// and one constant is what keeps the three agreeing.
const FieldName = "cf-turnstile-response"

// MaxTokenLength is the longest token Cloudflare issues. A longer value is
// refused without asking Cloudflare, because it cannot be a token.
const MaxTokenLength = 2048

// DefaultTimeout bounds one verification when Config leaves Client nil.
const DefaultTimeout = 10 * time.Second

// The keys Cloudflare publishes for testing. They work on any hostname,
// including localhost, and only with each other: a test secret accepts only
// TestToken, and a production secret refuses it.
//
// They are for development and for test suites. An application configured
// with TestSecretKey in production accepts every submission.
const (
	// TestSiteKey draws a visible widget that always passes.
	TestSiteKey = "1x00000000000000000000AA"
	// TestSecretKey verifies TestToken as a success.
	TestSecretKey = "1x0000000000000000000000000000000AA"
	// TestSecretKeyFails verifies every token as a failure.
	TestSecretKeyFails = "2x0000000000000000000000000000000AA"
	// TestSecretKeySpent answers every token as already spent.
	TestSecretKeySpent = "3x0000000000000000000000000000000AA"
	// TestToken is the token a widget drawn with a test site key produces.
	TestToken = "XXXX.DUMMY.TOKEN.XXXX"
)

// The sentences a refused submission is answered with when Config leaves
// Messages empty.
const (
	DefaultRejectedMessage    = "Confirm that you are not a robot, then send the form again."
	DefaultUnavailableMessage = "We could not confirm that you are not a robot. Wait a moment and send the form again."
)

// Config is what the application passes when it builds the Verifier.
//
// The keys arrive as values the application read from its own configuration.
// Nothing here reads the environment: a key read at call time is a key whose
// absence is discovered by the first visitor rather than at boot.
type Config struct {
	// SiteKey identifies the widget. It is public and is written into the page.
	SiteKey string
	// SecretKey authenticates the verification. It never reaches a page, a
	// log line or an error message.
	SecretKey string
	// Hostnames, when set, are the only hostnames a token may have been solved
	// on. Empty accepts the hostnames the widget is configured for in the
	// Cloudflare dashboard, which is where that list belongs; set this when one
	// widget serves several applications and each should accept only its own.
	Hostnames []string
	// Client sends the verification. Nil is a client that gives up after
	// DefaultTimeout.
	Client *http.Client
	// Endpoint is the verification address. Empty is DefaultEndpoint. It has
	// to be https, because the request carries the secret key.
	Endpoint string
	// Messages are the sentences a refused submission is answered with.
	Messages Messages
}

// Messages are the sentences a refused submission is answered with, in the
// language of the application. An empty one is the English default.
type Messages struct {
	// Rejected is said when the token was missing or refused: the person has
	// to solve the challenge again.
	Rejected string
	// Unavailable is said when Cloudflare could not be asked: the person did
	// nothing wrong and can send the form again.
	Unavailable string
}

// Validate reports what the configuration cannot be used with. New calls it,
// so a key that is missing fails where the application is wired.
func (c Config) Validate() error {
	if strings.TrimSpace(c.SiteKey) == "" {
		return errors.New("turnstile: Config.SiteKey is empty; it is the site key of the widget in the Cloudflare dashboard")
	}
	if strings.TrimSpace(c.SecretKey) == "" {
		return errors.New("turnstile: Config.SecretKey is empty; it is the secret key of the widget in the Cloudflare dashboard")
	}
	if c.Endpoint != "" {
		u, err := url.Parse(c.Endpoint)
		if err != nil || u.Scheme != "https" || u.Host == "" {
			return fmt.Errorf("turnstile: Config.Endpoint is %q and has to be an https address, because the request carries the secret key", c.Endpoint)
		}
	}
	for _, host := range c.Hostnames {
		if host == "" || strings.ContainsAny(host, "/: ") {
			return fmt.Errorf("turnstile: Config.Hostnames holds %q, which is not a bare hostname", host)
		}
	}
	return nil
}

// String describes the configuration with the secret key left out, so a
// Config that reaches a log line or a panic does not carry it there.
func (c Config) String() string {
	return fmt.Sprintf("turnstile.Config{SiteKey: %q, SecretKey: [redacted], Hostnames: %q, Endpoint: %q}",
		c.SiteKey, c.Hostnames, c.Endpoint)
}

// GoString is String, for the %#v verb.
func (c Config) GoString() string { return c.String() }
