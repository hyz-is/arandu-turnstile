# Security policy

## Supported versions

The latest minor release of Arandu Turnstile receives security fixes. Older
minor releases do not: a published Go module version is immutable, so a fix is
a new release rather than a moved tag.

## Reporting a vulnerability

Report it privately, through GitHub's advisory form:

<https://github.com/hyz-is/arandu-turnstile/security/advisories/new>

Do not open a public issue and do not describe the problem in a pull request.
A report that arrives in public is a report every reader of this repository can
act on before there is a release to upgrade to.

Include what you need to reproduce it: the version, the answer from Cloudflare
or the request that triggers it. Never a real secret key: a test key shows the
same defect.

Expect an acknowledgement within a few days. If the report is confirmed, the
fix, the release and the advisory are published together, and you are credited
unless you ask not to be.

## What is in scope

Anything that lets a submission through that Cloudflare did not accept, or that
exposes what the package keeps on the server. In particular:

- `Verify` or `Check` returning success for a token that is missing, refused,
  spent, solved on another hostname than the configured ones, or solved for
  another action than the one expected;
- `Check` returning `nil` while Cloudflare cannot be reached;
- the secret key reaching a page, an error, a log line or a formatted `Config`;
- the component writing unescaped input into the page, or the behaviour
  evaluating anything the page carries;
- `Policy` or `Headers` widening a directive other than `script-src` and
  `frame-src`, or adding an origin other than Cloudflare's;
- a panic on any input, from any exported function.

## What is not

- A person passing the challenge with help, or a service that solves it for
  pay. Turnstile raises the cost of automation; it does not prove identity.
- A vulnerability in Cloudflare's widget or endpoint. Report that to
  Cloudflare.
- A vulnerability in Hesape or kyse. Report that to them.
