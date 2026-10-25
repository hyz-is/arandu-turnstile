# Upgrade Guide

## Unreleased

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
