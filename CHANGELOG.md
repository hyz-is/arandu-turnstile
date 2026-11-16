# Changelog

Everything worth knowing about a release of Arandu Turnstile is recorded here.

The format follows [Keep a Changelog](https://keepachangelog.com/en/1.1.0/) and
the versions follow [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

A published module version is immutable: Go serves it from the proxy forever, so
a release is corrected by another release and never by moving a tag.

## [Unreleased]

## [0.5.0] - 2026-11-16

### Added

- `arandu.turnstile.token(options)`, in the client script that already draws
  `Widget`: a page with no form asks for a Turnstile token and sends it with a
  request of its own (a `fetch` that starts a payment step). It answers with a
  Promise of the token, draws nothing unless a person has to act
  (`appearance: "interaction-only"`), writes no hidden input, removes its widget
  as soon as the token is read, and needs neither the component nor
  `arandu.ui`. A rejection is an `Error` whose `code` is
  `challenge_unavailable`, `challenge_failed`, `challenge_unconfigured`,
  `challenge_cancelled` or `challenge_spent`, and whose `reason` says why.
- `arandu.turnstile.prepare(options)`, for a token that is asked for before the
  request is made: it solves now and hands the token over with `token()`. A
  token that expires before it is taken is solved again by the widget, so
  `token()` never answers with an expired one. `cancel()` drops it.
- The API works on a strict Content-Security-Policy: the script is the
  application's own, loaded with `view.Asset(turnstile.ScriptAsset)`, Cloudflare's
  is the one origin `Policy` allows, and nothing is inline. The README shows the
  server half for a request that is not a form: `Verify` with the action and the
  address, and the status each error maps to.

### Changed

- The script defines `arandu.turnstile` whether or not `arandu.ui` is on the
  page. Without `arandu.ui` the widget behaviour is not registered, as before,
  and the console says so only when the page draws a widget; a page that only
  asks for tokens is told nothing.

## [0.4.1] - 2026-11-16

### Changed

- Requires Hesape v0.109.0 and Kyse v0.53.1. No code changed.

## [0.4.0] - 2026-11-15

### Changed

- The widget is served to YPY pages: `Headers` recognises a boosted navigation
  by `YPY-Request` and `YPY-Boosted` and answers it with `YPY-Redirect`, the
  behaviour renews the token on `ypy:finally:request`, and the element
  Cloudflare draws into keeps its children through a morph. `UPGRADE.md` has the
  three consequences. Requires the Hesape and Kyse releases that draw views with
  YPY.

## [0.3.0] - 2026-11-14

### Changed

- The client script is served as `turnstile.js` and no longer as `arandu-turnstile.js`, so the address a visitor's page loads, `/<prefix>/assets/<hash>/turnstile.js`, carries nothing of the framework's name. `ScriptAsset` is the new name and `Widget` writes it, so a view that draws the widget needs no change.

## [0.2.0] - 2026-11-03

### Added

- `Config.OnRefused`: `Check` tells the application what Cloudflare answered
  each time it refuses a submission (the error codes, the hostname, the action
  and the reason), never the token or the secret key. A token the widget showed
  as solved and the server refused ("timeout-or-duplicate", a hostname that is
  not the application's) left no trace before, because `Check` answered only
  the sentence for the visitor.

## [0.1.3] - 2026-10-25

### Changed

- Requires Hesape v0.86.0 (it was v0.85.0) and Kyse v0.44.0. No code changed,
  and the suite is green on both.

## [0.1.2] - 2026-10-25

### Changed

- The documentation, the package skill and the example in the package comment
  read the token with `ctx.Bind` into a struct instead of `ctx.Input`, because
  `aru doctor` reports a field read by hand as `input-read-by-hand` in the
  application that followed them. No code changed.

## [0.1.1] - 2026-10-25

### Fixed

- `Verify` no longer holds the answer to a published test secret key
  (`TestSecretKey` and the two others) to `Config.Hostnames` or to the action
  the form expects. Cloudflare answers those keys with the hostname
  `example.com` and no action, so a development machine configured with the
  test keys was refused on every submission. A secret key that is not one of
  the three is held to both, as before.

## [0.1.0] - 2026-10-25

### Added

- `New` builds a `Verifier` from a `Config` with the site key and the secret
  key, and refuses an empty key, a verification address that is not https and
  a hostname that is not bare.
- `Verify` asks Cloudflare's `siteverify` about one token and returns the
  `Result`. It fails closed, and every error wraps `ErrMissingToken`,
  `ErrRejected`, `ErrUnavailable` or `ErrMisconfigured`. A token solved on a
  hostname outside `Config.Hostnames`, or for another action than the one
  expected, is refused.
- `Check` is `Verify` for a form handler: `nil`, or `validation.Errors` under
  `FieldName` with the `Rejected` or the `Unavailable` message, and
  `ErrMisconfigured` as it is.
- `Policy` adds `https://challenges.cloudflare.com` to `script-src` and
  `frame-src` of a Content-Security-Policy, and `Headers` writes it on the
  routes that draw the widget, answering a boosted navigation with
  the old library's redirect header so the page loads with its own policy.
- `Widget`, in `components`, draws the challenge inside a form, with the
  refusal from `Page` below it, and loads the `turnstile` behaviour, which
  renders the widget explicitly, renews its token after each old-library request of
  its form and removes it when the markup leaves the page.
- `TestSiteKey`, `TestSecretKey`, `TestSecretKeyFails`, `TestSecretKeySpent`
  and `TestToken`, Cloudflare's published keys for development and tests.
