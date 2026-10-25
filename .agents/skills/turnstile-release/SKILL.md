---
name: turnstile-release
description: Get this Arandu package green, declared and published. Use when the request is to "run the gates", "check it builds", "the tests fail", "go build says the directory prefix does not contain modules listed in go.work", "aru doctor says this is not an Arandu project", "the component change does not show up", "update arandu.mod.toml", "add a dependency", "bump hesape", "bump kyse", "tag a release", "publish it", "go get cannot find the module", "write the changelog", or before opening a pull request. Covers the five gates, the compiled component, what the manifest declares and who checks it, why the module requires Hesape and kyse and nothing else, the immutability of a published module version, and what breaks somebody else's form without touching a signature.
license: MIT
---

# Getting it green and out

## The five gates

Nothing is finished until all five exit zero.

```sh
export GOWORK=off
aru view:build
gofmt -l $(find . -name '*.go' -not -path '*/testdata/*' -not -name '*.kyse.go')
go build ./...
go vet ./...
go test -race ./...
```

`aru view:build` first. It writes `components/widget.go` from
`components/widget.kyse.go`, and the compiled file is what installers import:
`go get` has no build step. If `git status` shows `widget.go` changed after the
build, the committed file was stale, and the commit carries both.

`GOWORK=off` is not a habit. This checkout may sit beside a Go workspace that
lists the framework repositories and does not list this one. When it does,
`go build ./...` exits 1 before compiling anything:

```
pattern ./...: directory prefix . does not contain modules listed in go.work
or their selected dependencies
```

With the workspace off, the module resolves the versions in `go.mod`, which is
what CI compiles against and what somebody's `go get` will get.

`aru doctor` is not a gate. It exits 1 here with *"this is not an Arandu
project: no go.mod, main.go and arandu.toml together"*: it reads applications,
and this is a library.

## What the manifest declares, and who reads it

`arandu.mod.toml` is a promise, and the check that gives it weight runs here.
`aru doctor` audits the tree of the application it is run inside and never loads
a dependency, so the manifest of an installed package is opened by nothing but
that package's own suite.

```toml
name = "hyz-is/turnstile"
framework = ">= 0.66"
profiles = ["conventional", "performance"]

[permissions]
network = true
filesystem = false
exec = false
migrations = false
```

`TestTheDeclaredCapabilitiesAreWhatTheCodeDoes`, in `tests/Unit/audit_test.go`,
compares the four permissions against what the code *calls*, by AST.
`network = true` is the one POST to Cloudflare's `siteverify`. A second
destination is a change to the manifest's comment in the same commit.

The `framework` line is the first Framework release built on the Hesape this
module requires, because the framework itself is not in `go.mod`. When Hesape
is bumped, find the Framework release that requires the new version and move
the floor with it, in the same commit.

## Adding a dependency

Do not. This module goes into other people's builds, and it requires
`github.com/arandu-io/hesape` and `github.com/arandu-io/kyse` and nothing else.
`TestTheModuleRequiresNothingButHesapeAndKyse` refuses any other direct
requirement, and `TestThePackageIsALibrary` refuses an import outside its list.

```sh
export GOWORK=off
go list -m -f '{{if and (not .Indirect) (not .Main)}}{{.Path}} {{.Version}}{{end}}' all
```

Bump hesape and kyse together, to the releases an application on the current
framework resolves: a kyse that requires a newer hesape raises the installer's
hesape with it.

## Publishing

A published Go module version is **immutable**. The proxy serves it forever, and
a mistake is corrected by another release, never by moving a tag.

Before tagging:

- the five gates pass with `GOWORK=off`, and `components/widget.go` is the
  output of the current `.kyse.go`;
- `arandu.mod.toml` matches the code;
- `CHANGELOG.md` has the entry under the version, in the Keep a Changelog
  sections, and `UPGRADE.md` has a `## vX.Y.Z` heading for the same version;
- `currentVersion` in `tests/Unit/release_test.go` names that version;
- the entry points (`New`, `Verify`, `Check`, `Headers`, `Policy`, `Widget`,
  `FieldName`) are named in a released changelog entry.

While the version starts with `v0.`, breaking is allowed. Breaking *quietly* is
not. The CI step named `api diff against the last release` fails unless
`UPGRADE.md` names each removal.

Four changes break an installer without touching a signature, and each one is
worth a line in the changelog and in `UPGRADE.md`:

- **`FieldName`, or the props the component hands the behaviour.** Forms
  drawn by the old version send a field the new handler does not read.
- **What `Verify` refuses.** Refusing more locks out visitors an application
  used to accept; refusing less lets through what it used to stop.
- **The directives `Policy` adds.** Removing one breaks the widget on every
  page; adding one widens every installer's policy.
- **The behaviour's name or the asset's name.** A page compiled against the old
  component names a script or a behaviour that is no longer registered.

## Reporting a vulnerability

Not in an issue and not in a pull request. `SECURITY.md` has the private
advisory address. A submission accepted without a valid token is in scope, and
so is the secret key reaching anything but the verification request.
