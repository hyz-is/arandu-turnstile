# Upgrade Guide

## Unreleased

## v0.5.0

Nothing to change in the code: no exported Go name changes. A page that has no
form and wrote its own script around Cloudflare's `turnstile.render` can replace
it with `arandu.turnstile.token({ sitekey, action })`, which answers with a
Promise of the token; the README, "Without a form", has the options and the
errors. The page loads the script with `view.Asset(turnstile.ScriptAsset)`, and
its Content-Security-Policy is `turnstile.Policy`'s or carries the same two
directives.

## v0.4.1

Nothing to change. It raises Hesape to v0.109.0 and Kyse to v0.53.1.

## v0.4.0

Nothing to change in the code: no exported name changes. The page it serves is
a YPY page, and three things follow from that.

- **`Headers` reads and writes the YPY headers.** A boosted navigation is
  recognised by `YPY-Request` and `YPY-Boosted`, answered with `YPY-Redirect` to
  the same address, and the answer carries `Vary: YPY-Boosted`. A proxy or a
  test that matched the headers of the old library matches these instead.
- **The widget renews its token on `ypy:finally:request`**, which YPY fires
  after every request its form sends, whatever the answer was.
- **The element Cloudflare draws into carries `ypy-preserve="children"`.** A
  swap that names no style morphs, and the answer holds that element with nothing
  in it; without the attribute the morph would empty the challenge.

## v0.3.0

Nothing to change in the code. The script is registered and served as `turnstile.js` instead of `arandu-turnstile.js`; a page that was cached before the upgrade names the old file, and its address answers 404 until the page is drawn again. Nothing that is not a cached page names it: `Widget` writes `ScriptAsset`.

## v0.2.0

Nothing to change. A deployment that does nothing is checked as before. To learn
why a token the widget showed as solved was refused, set `Config.OnRefused` and
log `result.ErrorCodes`, `result.Hostname`, `result.Action` and the reason: it is
called by `Check` on every refusal and never carries the token.

## v0.1.3

Nothing to change. An application on Framework v0.66.0 has its Hesape raised
from v0.85.0 to v0.86.0 by installing this, which Hesape v0.86.0 documents as
changing nothing that compiled or ran.

## v0.1.2

Nothing to change in the code. An application that reads the token with
`ctx.Input(turnstile.FieldName)` keeps working, and `aru doctor` reports it as
`input-read-by-hand`; reading it with `ctx.Bind` into a struct tagged
`form:"cf-turnstile-response"` clears the finding, and `aru skills:sync --apply`
brings the corrected skill.

## v0.1.1

Nothing to change. A deployment configured with Cloudflare's test keys is
accepted as it was meant to be; one configured with its own keys is checked as
before.

## v0.1.0

The first release. Nothing to upgrade from.
