# Contributing

Thank you for considering a contribution.

## Before you open a pull request

Run the five gates. They are what CI runs, and a pull request that fails one of
them fails for everybody who pulls after it.

```bash
export GOWORK=off
aru view:build
gofmt -l $(find . -name '*.go' -not -path '*/testdata/*' -not -name '*.kyse.go')
go build ./...
go vet ./...
go test -race ./...
```

A change to `components/widget.kyse.go` is committed with the `widget.go` that
`aru view:build` writes from it. The compiled file is what installers import.

The two filters on `gofmt` are not preference. `gofmt` is the only tool in the
chain that ignores build tags, so it reads files the compiler deliberately
excludes; `testdata/` holds fixtures that are invalid on purpose.

## What a change has to keep true

1. **It fails closed.** No path lets a submission through without Cloudflare
   accepting its token for the expected hostname and action, including when
   Cloudflare does not answer.
2. **The token is verified once, by the handler.** Nothing in the package
   verifies a token before the code that acts on the submission asks.
3. **The secret key stays on the server.** It goes into the verification
   request and nowhere else.
4. **The page keeps its policy.** The behaviour runs from this origin with no
   inline script and no evaluated string, Cloudflare's script is loaded from
   Cloudflare, and the policy is widened only on the routes that draw the
   widget.
5. **This stays a library.** No route, no table, no policy, no `Module`, no
   import outside the list in `TestThePackageIsALibrary`, and an `init` that
   only registers the script.
6. **`arandu.mod.toml` matches the code.** A new destination, a file access or a
   process is declared there in the same commit.

No test calls Cloudflare. The endpoint is a local TLS server.

## Style

Everything in the source is in English: identifiers, doc comments, internal
comments, error messages, the script, and the names and messages of tests.
`pkg.go.dev` publishes the doc comments, and its readers are users of this
package.

Every exported symbol carries a doc comment saying what it does. The comment
documents the symbol and nothing else: no dates, no issue numbers, no
references to other repositories. Why a signature is what it is belongs there
when it is a fact about the code; anything else belongs in the pull request.

## Commits

One commit per logical change. Formatting is not mixed with behaviour, because
a diff that does both is a diff nobody reviews.

## Reporting a vulnerability

Do not open an issue. See [SECURITY.md](SECURITY.md).

## Licence

By contributing you agree that your contribution is licensed under the MIT
licence, the same as the rest of github.com/hyz-is/arandu-turnstile.
