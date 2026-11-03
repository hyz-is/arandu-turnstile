package turnstile

import (
	"context"
	"errors"

	"github.com/arandu-io/hesape/validation"
)

// Check verifies the token of one form submission and answers the way a
// handler returns a failed rule.
//
// It returns nil when the token is good. A token that is missing or refused,
// and a verification Cloudflare could not complete, return validation.Errors
// with one message under FieldName, which the router answers by sending the
// visitor back to the form with the message beside the widget. The two cases
// carry different messages, because only the first asks the person to do
// anything differently.
//
// ErrMisconfigured is returned as it is: a secret key Cloudflare refuses is a
// fault of the deployment, and it belongs on the error page and in the error
// report rather than in front of a visitor who can do nothing about it.
//
// Config.OnRefused, when set, is told what Cloudflare answered each time this
// refuses, so a refusal of a token that looked solved leaves its reason behind.
//
// Verify is the call for a caller that needs the Result or the reason.
func (v *Verifier) Check(ctx context.Context, token, remoteIP, action string) error {
	result, err := v.Verify(ctx, Challenge{Token: token, RemoteIP: remoteIP, Action: action})
	if err != nil && v.onRefused != nil {
		v.onRefused(ctx, result, err)
	}
	switch {
	case err == nil:
		return nil
	case errors.Is(err, ErrMisconfigured):
		return err
	}

	message := v.messages.Rejected
	if errors.Is(err, ErrUnavailable) {
		message = v.messages.Unavailable
	}
	refused := validation.Errors{}
	refused.Add(FieldName, message)
	return refused
}
