# Changelog

Everything worth knowing about a release of Arandu Turnstile is recorded here.

The format follows [Keep a Changelog](https://keepachangelog.com/en/1.1.0/) and
the versions follow [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

A published module version is immutable: Go serves it from the proxy forever, so
a release is corrected by another release and never by moving a tag.

## [Unreleased]

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
  `HX-Redirect` so the page loads with its own policy.
- `Widget`, in `components`, draws the challenge inside a form, with the
  refusal from `Page` below it, and loads the `turnstile` behaviour, which
  renders the widget explicitly, renews its token after each htmx request of
  its form and removes it when the markup leaves the page.
- `TestSiteKey`, `TestSecretKey`, `TestSecretKeyFails`, `TestSecretKeySpent`
  and `TestToken`, Cloudflare's published keys for development and tests.
