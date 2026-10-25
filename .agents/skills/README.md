# Skills

Procedures an assistant follows when working on this package.

They live in `.agents/skills/<name>/SKILL.md`, which is the path the coding
assistants read from. It is one directory rather than a file per vendor, so a
skill written once is read by whatever this package is being written with.

Each file opens with frontmatter carrying a `name` and a `description`. The
`name` has to equal the directory name exactly, or the skill is not loaded. The
description is what a tool reads to decide whether the skill is relevant, so it
names the situation you are in rather than the topic it covers.

| skill | when it fires |
| --- | --- |
| `turnstile-release` | the gates, the compiled component, the manifest, a dependency, a version, a tag |
| `turnstile-vault-notes` | writing the note, the gap or the journal entry, when this checkout sits inside the Arandu Obsidian vault |
| `turnstile-package` | putting the Turnstile challenge in front of a form **in an application** |

The template this package started from carried a `-policy` and a `-module`
skill. Both taught the HTTP module the template ships, and this package removed
it: there is no policy to open and no route, handler or migration to add.

`turnstile-package` has a different audience from the other two, and that is
on purpose: it travels with the package so that an assistant working in
somebody else's project — the one running `go get` — has the four pieces of
wiring and their symptoms in front of it instead of writing a CAPTCHA check of
its own.

Its frontmatter says so, under `metadata`, with `audience: app`. That line is
what `aru skills:sync` reads to copy the skill into an application whose
`go.mod` requires this package, and no other skill here carries it: an
application that received the release procedure would follow it.

`turnstile-vault-notes` fires on a condition rather than on a task: it applies
only when `MOC-arandu.md`, `plans/cmd/audit-vault/` and `45-modules/` are
actually beside this checkout. Outside the vault it is inert, and it says so
first, so a package cloned somewhere else never grows a folder tree imitating
one.

## Why these exist

The common failure modes when changing this package are a check that lets a
submission through when Cloudflare is slow, middleware that verifies a token
before the handler and spends it, a copy of Cloudflare's script served from the
application's origin, a policy widened for the whole application, an inline
script or a global callback, and a secret key that reaches a log line. None
belongs here.

The package is built to be checked rather than trusted. The verification is
tested against a local stand-in for Cloudflare's endpoint on every answer it
can give, the component is rendered and read back, and the structure test holds
the library shape. Running `go test -race ./...` exercises all of it.

## Adding your own

A skill in this directory is yours and travels with the repository. Keep it a
procedure rather than a description: a file that says "read the documentation"
never changes what anybody does.
