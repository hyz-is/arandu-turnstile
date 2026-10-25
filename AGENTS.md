# Working on Arandu Turnstile

This is an Arandu package that puts Cloudflare Turnstile in front of a form:
the `Widget` component in `components/`, the client behaviour it loads, the
Content-Security-Policy its page needs, and the server-side verification. It is
a Go module somebody `go get`s and imports.

It began as the package skeleton, configured as `hyz-is/turnstile`, and the
HTTP module the template ships was removed on purpose: there is no entity, no
table, no migration, no policy, no route and no `Module` to register. The
verification is a call the handler makes with values it already has, and
nothing here should grow back into a module.

Read `.agents/skills/` before writing code. Each skill is a procedure, and the
one you need is named by the situation you are in.

## The gates

Nothing is finished until all five exit zero.

```sh
export GOWORK=off
aru view:build
gofmt -l $(find . -name '*.go' -not -path '*/testdata/*' -not -name '*.kyse.go')
go build ./...
go vet ./...
go test -race ./...
```

`aru view:build` compiles `components/widget.kyse.go` into the `widget.go`
beside it. The compiled file is committed, because `go get` has no build step:
what is in the repository is what every installer imports. A change to the
`.kyse.go` without the rebuild ships the old component.

`GOWORK=off` is not a preference. This checkout may sit beside a Go workspace
that lists the framework repositories and does not list this one; when it does,
every command above fails before it compiles anything:

```
pattern ./...: directory prefix . does not contain modules listed in go.work
or their selected dependencies
```

With the workspace off, the module resolves the versions in `go.mod`, which is
what CI compiles against and what somebody's `go get` will get.

`aru doctor` is not one of the gates. It reads applications, and exits 1 here.
It does not read this package after it is installed either, which is why
`tests/Unit/audit_test.go` compares `arandu.mod.toml` with what the code calls.

## What this repository holds

| | measured with |
| --- | --- |
| 6 Go files in the root package, 1 component and its compiled file in `components/` | `ls *.go components/` |
| 1 script, served from the application's origin | `ls assets/` |
| 0 routes, 0 tables, 1 capability (`network`) | `arandu.mod.toml`, `TestThePackageIsALibrary` |
| 2 direct dependencies, `hesape` and `kyse` | `go list -m -f '{{if and (not .Indirect) (not .Main)}}{{.Path}} {{.Version}}{{end}}' all` |

## What does not exist here, on purpose

| A model reaches for | What is here instead |
| --- | --- |
| a `Module`, a route, a line in `k.Register` | nothing to register. `New` builds a Verifier the application keeps |
| middleware that verifies the token before the handler | `Check`, called by the handler. A token is spent by verifying it, so it is verified once, by the code that acts on the submission |
| a fallback that lets the form through when Cloudflare is down | nothing. The check fails closed; a check that opens when it is slow is one an attacker turns off |
| a copy of Cloudflare's `api.js` served from this origin | the script loaded from `https://challenges.cloudflare.com`, which Cloudflare requires: the file changes without changing its address |
| `class="cf-turnstile"` and Cloudflare's implicit render | `render=explicit`, driven by the `turnstile` behaviour, because the implicit scan runs once and misses every widget an htmx swap brings in |
| `data-callback="someGlobal"` or an inline script | the behaviour registered with `arandu.ui.define`, reading props the component wrote. The page stays at `script-src 'self'` plus one origin |
| a global widening of the policy | `Headers` on the routes that draw the widget. Every other page keeps the default |
| the secret key in a log line, an error or `%v` | `Config.String` and `GoString` redact it, and no error carries the request body |
| an `init` that does more than register the script | `view.RegisterAsset` and nothing else. `TestThePackageIsALibrary` holds it |
| an import of the framework | the standard library, `hesape/http`, `hesape/http/middleware`, `hesape/validation`, `hesape/view` and `kyse/components`. `TestThePackageIsALibrary` holds the list |

## The four properties

A change that breaks one of them is not merged, whatever else it improves.

1. **It fails closed.** Every path out of `Verify` that is not Cloudflare
   saying yes, for the expected hostname and action, is an error, and `Check`
   turns every error into a refusal or into `ErrMisconfigured`, never into
   `nil`.
2. **The token is verified once, by the handler.** Nothing in the package
   verifies a token on its own initiative.
3. **The secret key stays on the server.** It reaches the verification body
   and nothing else: not the page, not an error, not a formatted `Config`.
4. **`arandu.mod.toml` matches the code.** `network = true` for the one call to
   `siteverify`, the other three `false`, and `tests/Unit/audit_test.go`
   compares that with what the code calls.

## Writing code

Everything in the source is in English: identifiers, doc comments, internal
comments, error messages, the script, and the names and messages of tests. The
default messages are English; an application passes its own in `Messages`.

Every exported symbol carries a doc comment, and the comment documents the
symbol and nothing else. Why a signature is what it is belongs there when it is
a fact about the code. A date, an issue number, a version in progress or the
name of another repository does not.

The component is written in `components/widget.kyse.go`, never in the compiled
`widget.go`. Its parts are published in `PartNames`, and
`TestEveryPublishedPartIsDrawn` holds the list to the markup in both directions.

Tests go under `tests/Unit`, declaring `package unit_test`, and import the
packages by their module paths, which is what makes them see exactly what a
caller sees. No test calls Cloudflare: the endpoint is a local TLS server.
