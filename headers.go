package turnstile

import (
	"net/http"
	"strings"

	hhttp "github.com/arandu-io/hesape/http"
	"github.com/arandu-io/hesape/http/middleware"
)

// Policy is base widened by what the widget needs: Cloudflare's script, and the
// frame the challenge is drawn in. Nothing else is added, and no other
// directive changes.
//
// When base has no frame-src of its own, frames were governed by default-src,
// so 'self' is kept beside Cloudflare: a frame-src that named only Cloudflare
// would refuse the application's own frames on the pages that draw the widget.
func Policy(base middleware.ContentSecurityPolicy) middleware.ContentSecurityPolicy {
	widened := base.Allow("script-src", Origin)
	if strings.Contains("; "+base.String(), "; frame-src ") {
		return widened.Allow("frame-src", Origin)
	}
	return widened.Allow("frame-src", "'self'", Origin)
}

// Headers is the middleware for the routes whose pages draw the widget. It
// writes Policy(base) on the answer, replacing the policy SecurityHeaders wrote,
// so the widening reaches those pages and no other.
//
// base is the policy the application sends everywhere else, built the way the
// application builds it -- middleware.DefaultContentSecurityPolicy with the
// same image origins -- so the pages with the widget lose nothing the rest of
// the application allows.
//
// A boosted navigation is answered with HX-Redirect to the same address. The
// policy that governs a document is the one its first load carried, and a
// boosted link swaps the body without loading a document: a page with the
// widget reached that way would run under the policy of the page the link was
// on, and Cloudflare's script would be refused. The redirect makes the browser
// load the page, and the page arrives with its own policy. It costs one round
// trip, on the boosted navigations that reach these routes and on nothing else.
func Headers(base middleware.ContentSecurityPolicy) hhttp.Middleware {
	policy := Policy(base)

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Add("Vary", "HX-Boosted")
			if boostedNavigation(r) {
				w.Header().Set("HX-Redirect", r.URL.RequestURI())
				w.WriteHeader(http.StatusOK)
				return
			}
			policy.Apply(w)
			next.ServeHTTP(w, r)
		})
	}
}

// boostedNavigation reports whether r is htmx following a boosted link: a GET
// or HEAD with both of the headers htmx sends for one.
func boostedNavigation(r *http.Request) bool {
	if r.Method != http.MethodGet && r.Method != http.MethodHead {
		return false
	}
	return r.Header.Get("HX-Request") == "true" && r.Header.Get("HX-Boosted") == "true"
}
