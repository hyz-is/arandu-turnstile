package unit_test

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/hyz-is/arandu-turnstile"
)

const secret = "0x4AAAAAAAsecret-that-must-not-leak"

// siteverify is a stand-in for Cloudflare's endpoint: it records what it was
// sent and answers what the test wrote.
type siteverify struct {
	server *httptest.Server
	calls  atomic.Int32
	form   atomic.Value // url.Values of the last call
}

func newSiteverify(t *testing.T, answer func(w http.ResponseWriter, r *http.Request)) *siteverify {
	t.Helper()
	s := &siteverify{}
	s.server = httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		s.calls.Add(1)
		if err := r.ParseForm(); err != nil {
			t.Errorf("the verification is not a form: %v", err)
		}
		s.form.Store(r.PostForm)
		answer(w, r)
	}))
	t.Cleanup(s.server.Close)
	return s
}

func (s *siteverify) verifier(t *testing.T, cfg turnstile.Config) *turnstile.Verifier {
	t.Helper()
	cfg.SiteKey = "0x4AAAAAAAsitekey"
	cfg.SecretKey = secret
	cfg.Endpoint = s.server.URL
	cfg.Client = s.server.Client()
	v, err := turnstile.New(cfg)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	return v
}

func answering(body string) func(http.ResponseWriter, *http.Request) {
	return func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, body)
	}
}

const accepted = `{"success":true,"challenge_ts":"2026-10-25T12:00:00.000Z","hostname":"app.example.com","error-codes":[],"action":"signup","cdata":"plan-pro"}`

func TestAnAcceptedTokenReturnsTheResult(t *testing.T) {
	t.Parallel()
	s := newSiteverify(t, answering(accepted))
	v := s.verifier(t, turnstile.Config{})

	result, err := v.Verify(context.Background(), turnstile.Challenge{Token: "token", RemoteIP: "203.0.113.7", Action: "signup"})
	if err != nil {
		t.Fatalf("Verify: %v", err)
	}
	if !result.Success || result.Hostname != "app.example.com" || result.Action != "signup" || result.CData != "plan-pro" {
		t.Errorf("result = %+v", result)
	}
	if want := time.Date(2026, 10, 25, 12, 0, 0, 0, time.UTC); !result.ChallengeTS.Equal(want) {
		t.Errorf("ChallengeTS = %v, want %v", result.ChallengeTS, want)
	}

	form := s.form.Load().(url.Values)
	for field, want := range map[string]string{"secret": secret, "response": "token", "remoteip": "203.0.113.7"} {
		if got := strings.Join(form[field], ","); got != want {
			t.Errorf("sent %s = %q, want %q", field, got, want)
		}
	}
}

func TestATokenThatCannotBeOneIsRefusedWithoutAskingCloudflare(t *testing.T) {
	t.Parallel()
	s := newSiteverify(t, answering(accepted))
	v := s.verifier(t, turnstile.Config{})

	for name, c := range map[string]struct {
		token string
		want  error
	}{
		"empty":    {"", turnstile.ErrMissingToken},
		"blank":    {"   ", turnstile.ErrMissingToken},
		"too long": {strings.Repeat("a", turnstile.MaxTokenLength+1), turnstile.ErrRejected},
	} {
		if _, err := v.Verify(context.Background(), turnstile.Challenge{Token: c.token}); !errors.Is(err, c.want) {
			t.Errorf("%s: err = %v, want %v", name, err, c.want)
		}
	}
	if n := s.calls.Load(); n != 0 {
		t.Errorf("Cloudflare was asked %d times about tokens that cannot be tokens", n)
	}
}

func TestCloudflaresAnswerDecidesTheReason(t *testing.T) {
	t.Parallel()
	for name, c := range map[string]struct {
		answer func(http.ResponseWriter, *http.Request)
		want   error
	}{
		"invalid token":  {answering(`{"success":false,"error-codes":["invalid-input-response"]}`), turnstile.ErrRejected},
		"spent token":    {answering(`{"success":false,"error-codes":["timeout-or-duplicate"]}`), turnstile.ErrRejected},
		"no reason":      {answering(`{"success":false}`), turnstile.ErrRejected},
		"no token":       {answering(`{"success":false,"error-codes":["missing-input-response"]}`), turnstile.ErrMissingToken},
		"wrong secret":   {answering(`{"success":false,"error-codes":["invalid-input-secret"]}`), turnstile.ErrMisconfigured},
		"no secret":      {answering(`{"success":false,"error-codes":["missing-input-secret"]}`), turnstile.ErrMisconfigured},
		"internal error": {answering(`{"success":false,"error-codes":["internal-error"]}`), turnstile.ErrUnavailable},
		"not json":       {answering(`<html>`), turnstile.ErrUnavailable},
		"server error": {func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusBadGateway)
		}, turnstile.ErrUnavailable},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			v := newSiteverify(t, c.answer).verifier(t, turnstile.Config{})
			_, err := v.Verify(context.Background(), turnstile.Challenge{Token: "token"})
			if !errors.Is(err, c.want) {
				t.Fatalf("err = %v, want %v", err, c.want)
			}
			if strings.Contains(err.Error(), secret) {
				t.Errorf("the error carries the secret key: %v", err)
			}
		})
	}
}

func TestATokenSolvedElsewhereIsRefused(t *testing.T) {
	t.Parallel()
	s := newSiteverify(t, answering(accepted))

	other := s.verifier(t, turnstile.Config{Hostnames: []string{"admin.example.com"}})
	if _, err := other.Verify(context.Background(), turnstile.Challenge{Token: "token"}); !errors.Is(err, turnstile.ErrRejected) {
		t.Errorf("a token solved on another hostname: err = %v, want ErrRejected", err)
	}
	same := s.verifier(t, turnstile.Config{Hostnames: []string{"App.Example.com"}})
	if _, err := same.Verify(context.Background(), turnstile.Challenge{Token: "token"}); err != nil {
		t.Errorf("hostnames compare without case: %v", err)
	}

	v := s.verifier(t, turnstile.Config{})
	if _, err := v.Verify(context.Background(), turnstile.Challenge{Token: "token", Action: "contact"}); !errors.Is(err, turnstile.ErrRejected) {
		t.Errorf("a token solved for another action: err = %v, want ErrRejected", err)
	}
	if _, err := v.Verify(context.Background(), turnstile.Challenge{Token: "token"}); err != nil {
		t.Errorf("no action expected, any is accepted: %v", err)
	}
}

// Cloudflare's answer to its published test secret key carries the hostname
// example.com and no action, so a development machine that is held to its own
// hostname and to the action of the form is refused on every submission.
func TestATestSecretKeyIsNotHeldToTheHostnameOrTheAction(t *testing.T) {
	t.Parallel()
	const testingAnswer = `{"success":true,"challenge_ts":"2026-10-25T12:00:00.000Z","hostname":"example.com","error-codes":[],"metadata":{"result_with_testing_key":true}}`
	s := newSiteverify(t, answering(testingAnswer))

	cfg := turnstile.Config{SiteKey: turnstile.TestSiteKey, SecretKey: turnstile.TestSecretKey, Hostnames: []string{"localhost"}, Endpoint: s.server.URL, Client: s.server.Client()}
	v, err := turnstile.New(cfg)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	if _, err := v.Verify(context.Background(), turnstile.Challenge{Token: turnstile.TestToken, Action: "login"}); err != nil {
		t.Errorf("the test secret key was held to a hostname and an action Cloudflare does not answer: %v", err)
	}

	// The exemption is the published keys', not whatever an answer claims.
	production := s.verifier(t, turnstile.Config{Hostnames: []string{"localhost"}})
	if _, err := production.Verify(context.Background(), turnstile.Challenge{Token: "token", Action: "login"}); !errors.Is(err, turnstile.ErrRejected) {
		t.Errorf("another secret key was exempted from the hostname and the action: err = %v, want ErrRejected", err)
	}
}

func TestAnAddressThatIsNotOneIsNotSent(t *testing.T) {
	t.Parallel()
	s := newSiteverify(t, answering(accepted))
	v := s.verifier(t, turnstile.Config{})

	if _, err := v.Verify(context.Background(), turnstile.Challenge{Token: "token", RemoteIP: "203.0.113.7, 10.0.0.1"}); err != nil {
		t.Fatalf("Verify: %v", err)
	}
	if form := s.form.Load().(url.Values); len(form["remoteip"]) != 0 {
		t.Errorf("remoteip = %q, want it left out", form["remoteip"])
	}
}

func TestAVerificationThatDoesNotFinishIsUnavailable(t *testing.T) {
	t.Parallel()
	release := make(chan struct{})
	s := newSiteverify(t, func(http.ResponseWriter, *http.Request) { <-release })
	defer close(release)
	v := s.verifier(t, turnstile.Config{})

	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()
	if _, err := v.Verify(ctx, turnstile.Challenge{Token: "token"}); !errors.Is(err, turnstile.ErrUnavailable) {
		t.Errorf("err = %v, want ErrUnavailable", err)
	}
}
