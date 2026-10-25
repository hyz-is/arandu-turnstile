package unit_test

import (
	"context"
	"errors"
	"net/http"
	"testing"

	"github.com/arandu-io/hesape/validation"
	"github.com/hyz-is/arandu-turnstile"
)

func TestCheckAnswersARefusalAsAFieldError(t *testing.T) {
	t.Parallel()
	for name, c := range map[string]struct {
		answer func(http.ResponseWriter, *http.Request)
		token  string
		want   string
	}{
		"missing": {answering(accepted), "", turnstile.DefaultRejectedMessage},
		"refused": {answering(`{"success":false,"error-codes":["timeout-or-duplicate"]}`), "token", turnstile.DefaultRejectedMessage},
		"outage": {func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusServiceUnavailable)
		}, "token", turnstile.DefaultUnavailableMessage},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			v := newSiteverify(t, c.answer).verifier(t, turnstile.Config{})
			err := v.Check(context.Background(), c.token, "", "signup")

			var refused validation.Errors
			if !errors.As(err, &refused) {
				t.Fatalf("err = %v, want validation.Errors", err)
			}
			if got := refused[turnstile.FieldName]; len(got) != 1 || got[0] != c.want {
				t.Errorf("messages under %s = %q, want %q", turnstile.FieldName, got, c.want)
			}
			if len(refused) != 1 {
				t.Errorf("fields = %v, want only %s", refused.Keys(), turnstile.FieldName)
			}
		})
	}
}

func TestCheckSpeaksTheApplicationsLanguage(t *testing.T) {
	t.Parallel()
	v := newSiteverify(t, answering(`{"success":false}`)).verifier(t, turnstile.Config{
		Messages: turnstile.Messages{Rejected: "Confirme que você não é um robô."},
	})

	var refused validation.Errors
	if err := v.Check(context.Background(), "token", "", ""); !errors.As(err, &refused) {
		t.Fatalf("err = %v, want validation.Errors", err)
	}
	if got := refused[turnstile.FieldName][0]; got != "Confirme que você não é um robô." {
		t.Errorf("message = %q", got)
	}
}

func TestCheckAcceptsAGoodToken(t *testing.T) {
	t.Parallel()
	v := newSiteverify(t, answering(accepted)).verifier(t, turnstile.Config{})
	if err := v.Check(context.Background(), "token", "203.0.113.7", "signup"); err != nil {
		t.Errorf("Check: %v", err)
	}
}

// A secret key Cloudflare refuses is the operator's fault, and it has to reach
// the error report rather than be drawn as a message no visitor can act on.
func TestCheckDoesNotHideAMisconfiguration(t *testing.T) {
	t.Parallel()
	v := newSiteverify(t, answering(`{"success":false,"error-codes":["invalid-input-secret"]}`)).verifier(t, turnstile.Config{})

	err := v.Check(context.Background(), "token", "", "")
	if !errors.Is(err, turnstile.ErrMisconfigured) {
		t.Fatalf("err = %v, want ErrMisconfigured", err)
	}
	var refused validation.Errors
	if errors.As(err, &refused) {
		t.Error("a misconfiguration came back as a field error")
	}
}
