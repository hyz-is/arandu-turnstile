package turnstile

import (
	_ "embed"

	"github.com/arandu-io/hesape/view"
)

// ScriptAsset is the name the client behaviour is served under, from this
// origin, by the application's asset handler. Widget writes the script tag
// that loads it.
const ScriptAsset = "arandu-turnstile.js"

// script is the client behaviour: it loads Cloudflare's script once, renders
// each widget the page draws or a swap brings in, renews the token after every
// request its form sends, and removes the widget when its markup leaves the
// page.
//
// It is embedded and registered rather than copied into the project, so the
// behaviour a page runs is the one the installed version of this package
// carries, and an upgrade reaches it.
//
//go:embed assets/arandu-turnstile.js
var script []byte

// The behaviour is registered when the package is linked: a view that draws
// Widget names the asset, and the asset has to exist before the first page is
// drawn.
func init() {
	view.RegisterAsset(ScriptAsset, "application/javascript; charset=utf-8", script)
}
