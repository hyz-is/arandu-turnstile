---
name: turnstile-package
description: Install, wire and use Arandu Turnstile (Cloudflare Turnstile, the anti-bot challenge) in an Arandu application. Use when the request is to "add a captcha", "add Turnstile", "protect the sign-up form from bots", "anti-robot check", "go get github.com/hyz-is/arandu-turnstile", "the widget does not appear", "Refused to load the script challenges.cloudflare.com", "Content Security Policy blocks Turnstile", "the form always says confirm that you are not a robot", "timeout-or-duplicate", "the widget works on refresh but not after clicking a link", or when a project's go.mod already requires github.com/hyz-is/arandu-turnstile. Covers the keys, the Verifier in bootstrap/app.go, the Headers middleware on the routes that draw the widget, the Widget component in the view, the Check call in the handler, the test keys, and the symptoms of each missing piece.
license: MIT
metadata:
  audience: app
---

# Using Arandu Turnstile

Four pieces, each one a line somebody writes. A missing piece has a symptom, and
the table at the end maps each symptom to the piece.

## 1. The keys

A widget in the Cloudflare dashboard (Turnstile, Add widget) for the
application's hostnames gives a site key, which is public, and a secret key,
which is not. They go in `.env` and in the deployment's secrets, never in the
repository:

```dotenv
TURNSTILE_SITE_KEY=0x4AAAAAAA...
TURNSTILE_SECRET_KEY=0x4AAAAAAA...
```

The application's config reads them like any other setting. For local
development and test suites, use Cloudflare's published keys, which work on
`localhost` and always pass: `turnstile.TestSiteKey` and
`turnstile.TestSecretKey`. A production deployment with the test secret
accepts every submission, so they never reach production.

## 2. The Verifier, in bootstrap/app.go

One for the application, built where the other collaborators are built:

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

`New` refuses an empty key, so a deployment without the keys fails at boot. The
Verifier is handed to the controllers that check a form, as a constructor
parameter, like any other collaborator.

## 3. The routes that draw the widget

```go
r = r.Group("", turnstile.Headers(middleware.DefaultContentSecurityPolicy()))
```

`middleware` is `github.com/arandu-io/hesape/http/middleware`. Pass the policy
the rest of the application sends: if `bootstrap/app.go` configures image
origins, pass the same ones to `DefaultContentSecurityPolicy`.

Wrap the GET of every page that draws the widget, and only those. The
middleware adds `https://challenges.cloudflare.com` to `script-src` and
`frame-src`, and answers a boosted navigation with `HX-Redirect` so the page is
loaded with its own policy: the layout's `hx-boost` swaps the body and keeps
the policy of the page the link was on.

A widget inside a fragment loaded by `hx-get` into another page needs the page
it lands in to carry the policy too. Wrap that page's route as well.

## 4. The component, in the view

```go
import turnstile "github.com/hyz-is/arandu-turnstile/components"
```

```html
{!! turnstile.Widget(turnstile.WidgetProps{
	SiteKey: .TurnstileSiteKey,
	Action:  "signup",
	Page:    .,
}) !!}
```

It goes inside the `<form>`. The handler of the page puts `captcha.SiteKey()` in
the page data. `Action` names what the form does, up to 32 letters, digits, `_`
and `-`, and the handler checks the same name. `Page: .` is what draws the
refusal below the challenge. `Theme`, `Size`, `Language` and `Appearance` pass
straight to Cloudflare.

Run `aru view:build` afterwards.

## 5. The check, in the handler

```go
// The field the widget writes into the form. The tag is the value of
// turnstile.FieldName.
type challengeRequest struct {
	Token string `form:"cf-turnstile-response"`
}

var challenge challengeRequest
if err := ctx.Bind(&challenge); err != nil {
	return err
}
if err := captcha.Check(ctx.Ctx(), challenge.Token, ctx.IP(), "signup"); err != nil {
	return err
}
```

Here `turnstile` is the root package, `github.com/hyz-is/arandu-turnstile`. The
token is read with `ctx.Bind` like every other field: `aru doctor` reports a
field read with `ctx.Input` as `input-read-by-hand`. The check is the first
thing the handler does with the submission, before validation of the other
fields, and it is done once: a token is spent by verifying it.

A refusal returns `validation.Errors` under `turnstile.FieldName`, and returning
it sends the visitor back to the form with the message. An outage at Cloudflare
is a refusal too, with the `Unavailable` message. A secret key Cloudflare
refuses returns `turnstile.ErrMisconfigured`, which reaches the error page and
the error report: fix the key.

In a test, build the Verifier with `turnstile.TestSecretKey` and post
`turnstile.TestToken` under `turnstile.FieldName`; with
`turnstile.TestSecretKeyFails` every token is refused. Both reach Cloudflare, so
a suite that must stay offline points `Config.Endpoint` and `Config.Client` at
an `httptest.NewTLSServer`.

## Symptoms

| what happens | the missing piece |
| --- | --- |
| the console says `arandu-turnstile: https://challenges.cloudflare.com/... did not load` | the route of the page is not wrapped in `turnstile.Headers` |
| it works after a refresh and not after clicking a link | the same: the page was swapped in by a boosted link and kept the old policy |
| `arandu.ui is not on the page` | the layout does not load `ui.js` before the component's script |
| the widget space is empty and nothing is in the console | `SiteKey` is empty in the page data |
| every submission comes back with the Rejected message | the handler reads another field than `turnstile.FieldName`, the `Action` differs between view and handler, or the widget is outside the `<form>` |
| the second attempt after a refusal is refused as spent (`timeout-or-duplicate`) | the form was posted without htmx and the page was not drawn again; with htmx the widget renews itself |
| `turnstile: Cloudflare refused the secret key` on the error page | the secret key in the environment is wrong or belongs to another widget |
