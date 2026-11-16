<p align="center">
  <img src=".github/logo.svg" alt="Arandu" width="180">
</p>

# Arandu Turnstile

Cloudflare Turnstile in front of an Arandu form: the view component that draws
the challenge, the policy the page needs to load it, and the server-side check
that decides whether the submission came from a person. A page with no form
gets the same proof from `arandu.turnstile.token`, a call in the browser that
answers with a token to send along with a request of its own.

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

The Verifier, once, where the application builds its other services
(`NewAppServiceProvider`, in `app/Providers`), from the application's own config:

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
navigation with `YPY-Redirect`: a document keeps the policy of its first load,
so a page with the widget has to be loaded, not swapped in.

## Draw it

The component goes inside the form. The handler hands the page the site key:

```go
import turnstile "github.com/hyz-is/arandu-turnstile/components"
```

```html
<form method="post" action="{{ .StoreURL }}" ypy-post="{{ .StoreURL }}">
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
below the challenge. After every YPY request its form sends, the widget is
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

## Without a form

A script that has to prove a person is in the browser before it sends a request
of its own (a `fetch` that starts a payment step, a button that calls an API)
has no form to carry a token. The client script that draws the widget also
exposes `arandu.turnstile.token`, which solves one challenge and answers with
its token:

```js
arandu.turnstile.token({ sitekey: siteKey, action: 'card-tokenization' }).then(function (token) {
	return fetch('/card-tokenization', {
		method: 'POST',
		headers: { 'Content-Type': 'application/json' },
		body: JSON.stringify({ turnstile_token: token }),
	});
}).catch(function (error) {
	// error.code is challenge_unavailable, challenge_failed or another below;
	// error.reason says why (Cloudflare's error code, "timeout", "unsupported").
});
```

The page loads the script, which is served from this application's origin with
the rest of the page's assets, and the route that serves the page is wrapped in
`turnstile.Headers` (or its policy is `turnstile.Policy`, which a page that
writes its own Content-Security-Policy adds with `turnstile.Origin` to
`script-src` and `frame-src`). Neither a component nor `arandu.ui` is needed:

```html
<script src="{{ view.Asset(turnstile.ScriptAsset) }}" defer></script>
```

`sitekey` is the value `Verifier.SiteKey()` returns; the handler puts it in the
page data as it does for the widget. The options are the ones `Widget` takes
(`action`, `cData`, `theme`, `size`, `language`) plus:

| option | |
| --- | --- |
| `appearance` | `interaction-only` by default: nothing is drawn unless a person has to act |
| `container` | an element or a selector to draw into; by default a `div[data-turnstile-token]` added to the body and removed with the widget |
| `onInteractive` | called with `true` when the challenge needs a person, so the page can make room for it, and with `false` when it no longer does |
| `timeout` | milliseconds `token` waits before it rejects; default 60000, `0` waits for Cloudflare alone |

The widget never writes a hidden input: the token is the Promise's value. It is
removed as soon as the token is read, and a token proves one request, so each
call solves a new challenge. To hide the delay of solving it, ask earlier and
take the token when the request is made:

```js
var challenge = arandu.turnstile.prepare({ sitekey: siteKey, action: 'card-tokenization' });
// ... the buyer fills in the card ...
challenge.token().then(send);   // at once, if it was solved in the meantime
// challenge.cancel() when the buyer leaves
```

A token Cloudflare issues lives a few minutes. A prepared token that expires
before it is taken is solved again by the widget, so `token()` never answers
with an expired one. A handle gives one token: a second `token()` rejects with
`challenge_spent`.

Every rejection is an `Error` with a `code`: `challenge_unavailable` (Cloudflare's
script did not load: the policy refused it, or the network did),
`challenge_failed` (the challenge errored, timed out or is not supported),
`challenge_unconfigured` (no `sitekey`), `challenge_cancelled` and
`challenge_spent`. It is a rejection and never an exception, so the request that
needed the token can be refused with a message.

The server checks the token like any other, once, before it acts on the request.
`Verify` takes the action the call asked for, so a token solved for one step
cannot be spent on another, and the hostnames the application serves:

```go
captcha, err := turnstile.New(turnstile.Config{
	SiteKey:   cfg.Turnstile.SiteKey,
	SecretKey: cfg.Turnstile.SecretKey,
	Hostnames: []string{"app.example.com"},
})

result, err := captcha.Verify(ctx, turnstile.Challenge{
	Token:    body.TurnstileToken,
	RemoteIP: ctx.IP(),
	Action:   "card-tokenization",
})
switch {
case err == nil:
	// proceed
case errors.Is(err, turnstile.ErrMissingToken), errors.Is(err, turnstile.ErrRejected):
	// 403: the proof is missing or does not verify; result says why
case errors.Is(err, turnstile.ErrUnavailable), errors.Is(err, turnstile.ErrMisconfigured):
	// 503: not the visitor's fault
}
```

## Layout

```
doc.go                   the package comment
config.go                Config, the constants and the test keys
verify.go                Verifier, New, Verify and the errors
check.go                 Check, for a form handler
headers.go               Policy and Headers
assets.go                the client behaviour, registered with the view layer
assets/                  turnstile.js: the widget behaviour and arandu.turnstile.token
components/widget.kyse.go  the Widget component, and widget.go compiled from it
```

## Tests

```bash
GOWORK=off go test -race ./...
```

The verification is tested against a stand-in for Cloudflare's endpoint, so the
suite makes no call outside the process. The client script is run in Node
against a stand-in for the page and for Cloudflare's script
(`tests/Unit/testdata/token_harness.js`); the test is skipped where `node` is not
installed, and fails there when `CI` is set.

## Licence

MIT. See [LICENSE.md](LICENSE.md). Copyright Paulo R. Lima.
