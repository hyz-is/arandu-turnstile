package unit_test

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/arandu-io/hesape/http/middleware"
	"github.com/hyz-is/arandu-turnstile"
)

func directive(policy, name string) string {
	for _, part := range strings.Split(policy, "; ") {
		if strings.HasPrefix(part, name+" ") {
			return part
		}
	}
	return ""
}

func TestThePolicyAddsCloudflareAndNothingElse(t *testing.T) {
	t.Parallel()
	base := middleware.DefaultContentSecurityPolicy("https://cdn.example.com")
	got := turnstile.Policy(base).String()

	if d := directive(got, "script-src"); d != "script-src 'self' "+turnstile.Origin {
		t.Errorf("script-src = %q", d)
	}
	if d := directive(got, "frame-src"); d != "frame-src 'self' "+turnstile.Origin {
		t.Errorf("frame-src = %q, want 'self' kept beside Cloudflare", d)
	}
	for _, name := range []string{"default-src", "style-src", "img-src", "connect-src", "object-src", "frame-ancestors", "form-action"} {
		if directive(got, name) != directive(base.String(), name) {
			t.Errorf("%s changed: %q -> %q", name, directive(base.String(), name), directive(got, name))
		}
	}
}

func TestThePolicyKeepsAFrameSourceTheApplicationWrote(t *testing.T) {
	t.Parallel()
	base := middleware.DefaultContentSecurityPolicy().Allow("frame-src", "https://player.example.com")
	if d := directive(turnstile.Policy(base).String(), "frame-src"); d != "frame-src https://player.example.com "+turnstile.Origin {
		t.Errorf("frame-src = %q", d)
	}
}

func serve(r *http.Request) (*httptest.ResponseRecorder, bool) {
	reached := false
	next := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		reached = true
		w.WriteHeader(http.StatusOK)
	})
	w := httptest.NewRecorder()
	// What SecurityHeaders wrote before the route's middleware ran.
	w.Header().Set("Content-Security-Policy", middleware.DefaultContentSecurityPolicy().String())
	turnstile.Headers(middleware.DefaultContentSecurityPolicy())(next).ServeHTTP(w, r)
	return w, reached
}

func TestHeadersWidenThePolicyOfThePage(t *testing.T) {
	t.Parallel()
	w, reached := serve(httptest.NewRequest(http.MethodGet, "/signup", nil))
	if !reached {
		t.Fatal("the page was not drawn")
	}
	if got := w.Header().Get("Content-Security-Policy"); !strings.Contains(directive(got, "script-src"), turnstile.Origin) {
		t.Errorf("Content-Security-Policy = %q", got)
	}
	if !strings.Contains(w.Header().Get("Vary"), "HX-Boosted") {
		t.Errorf("Vary = %q, want HX-Boosted", w.Header().Get("Vary"))
	}
}

// A boosted link swaps the body of a document whose policy was decided when it
// loaded, so the page with the widget is loaded instead of swapped in.
func TestABoostedNavigationLoadsThePage(t *testing.T) {
	t.Parallel()
	r := httptest.NewRequest(http.MethodGet, "/signup?plan=pro", nil)
	r.Header.Set("HX-Request", "true")
	r.Header.Set("HX-Boosted", "true")

	w, reached := serve(r)
	if reached {
		t.Error("the page was drawn into a document that cannot load the widget")
	}
	if got := w.Header().Get("HX-Redirect"); got != "/signup?plan=pro" {
		t.Errorf("HX-Redirect = %q", got)
	}
}

func TestOtherHTMXRequestsReachThePage(t *testing.T) {
	t.Parallel()
	for name, r := range map[string]*http.Request{
		"a form post": httptest.NewRequest(http.MethodPost, "/signup", nil),
		"an hx-get":   httptest.NewRequest(http.MethodGet, "/signup/fields", nil),
	} {
		r.Header.Set("HX-Request", "true")
		if r.Method == http.MethodPost {
			r.Header.Set("HX-Boosted", "true")
		}
		w, reached := serve(r)
		if !reached || w.Header().Get("HX-Redirect") != "" {
			t.Errorf("%s: reached = %v, HX-Redirect = %q", name, reached, w.Header().Get("HX-Redirect"))
		}
	}
}
