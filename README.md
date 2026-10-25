<p align="center">
  <img src=".github/logo.svg" alt="Arandu" width="180">
</p>

# Arandu Turnstile

Cloudflare Turnstile in front of an Arandu form: the view component that draws
the challenge, the policy the page needs to load it, and the server-side check
that decides whether the submission came from a person.

It is a library. It registers no route, owns no table and decides no
authorization. An application builds one `Verifier`, wraps the routes that draw
the widget, puts the component in the form and calls `Check` in the handler.

## Install

```bash
go get github.com/hyz-is/arandu-turnstile
```

Create a widget in the Cloudflare dashboard (Turnstile, then Add widget) for the
application's hostnames. It gives a site key, which is public, and a secret key,
which is not. Keep both in the environment:

```dotenv
TURNSTILE_SITE_KEY=0x4AAAAAAA...
TURNSTILE_SECRET_KEY=0x4AAAAAAA...
```

For local development and tests, Cloudflare publishes keys that always pass:
`turnstile.TestSiteKey` and `turnstile.TestSecretKey`. Never deploy them.

## Wire it

The Verifier, once, in `bootstrap/app.go`, from the application's own config:

```go
captcha, err := turnstile.New(turnstile.Config{
	SiteKey:   cfg.Turnstile.SiteKey,
	SecretKey: cfg.Turnstile.SecretKey,
	Messages: turnstile.Messages{
		Rejected:    "Confirme que você não é um robô e envie o formulário de novo.",
		Unavailable: "Não conseguimos confirmar que você não é um robô. Aguarde um instante e envie de novo.",
	},
})
if err != nil {
	return nil, err
}
```

The routes whose pages draw the widget, wrapped in the policy that loads it.
Pass the policy the rest of the application sends, with the same image origins:

```go
r = r.Group("", turnstile.Headers(middleware.DefaultContentSecurityPolicy()))
```

`turnstile.Headers` adds `https://challenges.cloudflare.com` to `script-src`
and `frame-src` on those pages and nowhere else, and answers a boosted
navigation with `HX-Redirect`: a document keeps the policy of its first load,
so a page with the widget has to be loaded, not swapped in.

## Draw it

The component goes inside the form. The handler hands the page the site key:

```go
import turnstile "github.com/hyz-is/arandu-turnstile/components"
```

```html
<form method="post" action="{{ .StoreURL }}" hx-post="{{ .StoreURL }}">
	@csrf
	...
	{!! turnstile.Widget(turnstile.WidgetProps{
		SiteKey: .TurnstileSiteKey,
		Action:  "signup",
		Page:    .,
	}) !!}
	<button type="submit">Create account</button>
</form>
```

The widget writes its token into a hidden field named `turnstile.FieldName`
(`cf-turnstile-response`). A refusal comes back under that field and is drawn
below the challenge. After every htmx request its form sends, the widget is
renewed, because a token can be spent only once.

Run `aru view:build` after installing, as for any component.

## Check it

In the handler, before anything else is done with the submission:

```go
// challengeRequest is the field the widget writes into the form.
type challengeRequest struct {
	Token string `form:"cf-turnstile-response"` // turnstile.FieldName
}

func (c *AccountController) Store(ctx *http.Context) error {
	var challenge challengeRequest
	if err := ctx.Bind(&challenge); err != nil {
		return err
	}
	if err := captcha.Check(ctx.Ctx(), challenge.Token, ctx.IP(), "signup"); err != nil {
		return err
	}
	// ...
}
```

The token is read with `ctx.Bind` into a struct, like every other field:
`aru doctor` flags a field read by hand with `ctx.Input`. The tag has to be the
value of `turnstile.FieldName`.

`Check` returns `nil`, or `validation.Errors` under `turnstile.FieldName`,
which sends the visitor back to the form with the message. It fails closed: a
missing, spent, expired or forged token is refused, and so is a submission made
while Cloudflare cannot be reached. A secret key Cloudflare refuses is returned
as `turnstile.ErrMisconfigured`, for the error page and the error report.

`Verify` is the lower-level call, for a caller that needs the `Result` or the
reason (`ErrMissingToken`, `ErrRejected`, `ErrUnavailable`, `ErrMisconfigured`).

## Layout

```
doc.go                   the package comment
config.go                Config, the constants and the test keys
verify.go                Verifier, New, Verify and the errors
check.go                 Check, for a form handler
headers.go               Policy and Headers
assets.go                the client behaviour, registered with the view layer
assets/                  arandu-turnstile.js
components/widget.kyse.go  the Widget component, and widget.go compiled from it
```

## Tests

```bash
GOWORK=off go test -race ./...
```

The verification is tested against a stand-in for Cloudflare's endpoint, so the
suite makes no call outside the process.

## Licence

MIT. See [LICENSE.md](LICENSE.md). Copyright Paulo R. Lima.
