// Package turnstile puts Cloudflare Turnstile in front of a form: a view
// component that draws the challenge, the client behaviour that renders it and
// renews its token, the Content-Security-Policy the page needs to load it, and
// the server-side verification that decides whether the submission came from a
// person.
//
// The four pieces are used together, and each one is a line somebody writes:
//
//	// bootstrap/app.go: one Verifier for the application.
//	captcha, err := turnstile.New(turnstile.Config{
//		SiteKey:   cfg.Turnstile.SiteKey,
//		SecretKey: cfg.Turnstile.SecretKey,
//	})
//
//	// routes: the pages that draw the widget carry the policy that loads it.
//	r = r.Group("", turnstile.Headers(middleware.DefaultContentSecurityPolicy()))
//
//	// view: the component goes inside the form.
//	{!! turnstile.Widget(turnstile.WidgetProps{
//		SiteKey: .TurnstileSiteKey, Action: "signup", Page: .,
//	}) !!}
//
//	// handler: the token is read like any other field, and checked before
//	// anything else is done.
//	var challenge struct {
//		Token string `form:"cf-turnstile-response"` // turnstile.FieldName
//	}
//	if err := ctx.Bind(&challenge); err != nil {
//		return err
//	}
//	if err := captcha.Check(ctx.Ctx(), challenge.Token, ctx.IP(), "signup"); err != nil {
//		return err
//	}
//
// A rejected token comes back as validation.Errors under FieldName, so the
// form is drawn again with the message beside the widget, exactly as a field
// that failed its rule would be.
//
// The verification fails closed. A token that is missing, malformed, spent,
// expired, solved on another hostname or for another action is refused, and so
// is a submission made while Cloudflare cannot be reached: a check that let a
// request through whenever it could not answer is a check an attacker can turn
// off by making it slow.
package turnstile
