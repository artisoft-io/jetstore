package main

import (
	"net/http"
	"os"
	"path"
	"path/filepath"
	"strings"
)

// Serving for the web app (jetsclient_ide/).
//
// One handler over a path prefix, rather than a per-asset route list. That is not
// a style preference: vite emits content-hashed file names (index-DlXT-v6f.js),
// so the set of assets changes on every build and cannot be enumerated in Go at
// all. The hashing is also what makes the cache policy below safe.
//
// **Tasks X.1 and X.2, 2026-08-26.** This served `/ide/` beside 64 hand-listed
// Flutter asset routes; the Flutter app is gone and this serves the root. The
// paragraph that used to sit here said consolidating the two "means moving the
// static registration after the api routes so a catch-all cannot shadow /login —
// a change worth making on its own when the Flutter app is retired, not as a
// rider on this one". It was right on both counts, and that is where the
// registration now is: the bottom of `Run`, after every api route.

// appAssetPrefix is the url space this app owns, which is all of it that the api
// has not claimed. Everything under it is either a bundled asset or a
// client-side route.
//
// **`/` since X.2, and the rename is three files rather than one.** The prefix is
// written here, as `base` in `jetsclient_ide/vite.config.ts`, and as `BASENAME` in
// `jetsclient_ide/src/base.ts`; vite bakes it into every asset url at build time,
// so the bundle is not relocatable and changing this alone would serve a page
// whose scripts 404. The old value named one screen of an application that now
// has twenty — the ui refresh project's I-26, accepted as debt by its decision 4
// and payable "when the Flutter app retires", which is this task.
const appAssetPrefix = "/"

// appHandler serves a single-page app from dir.
//
// Requests resolve to a file when one exists; anything else falls back to
// index.html so client-side routes survive a reload. The fallback is what makes
// this a SPA handler rather than a file server, and it is why the api routes must
// never live under this prefix.
func appHandler(prefix, dir string) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		rel := strings.TrimPrefix(r.URL.Path, prefix)
		// path.Clean("/"+rel) collapses any ".." before it can escape dir. Joining
		// the *cleaned absolute* form is what makes traversal impossible; cleaning
		// after the join would be too late.
		clean := path.Clean("/" + rel)
		target := filepath.Join(dir, filepath.FromSlash(clean))

		if info, err := os.Stat(target); err == nil && !info.IsDir() {
			// Hashed asset names change whenever the content does, so they can be
			// cached hard. index.html carries no hash and must not be.
			if strings.HasPrefix(clean, "/assets/") {
				w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
			} else {
				w.Header().Set("Cache-Control", "no-cache")
			}
			http.ServeFile(w, r, target)
			return
		}

		// A missing asset must 404 rather than silently returning the html shell:
		// handing index.html to a request for a .js file produces a console error
		// about an unexpected '<' that says nothing about the real cause.
		if strings.HasPrefix(clean, "/assets/") {
			http.NotFound(w, r)
			return
		}

		index := filepath.Join(dir, "index.html")
		if _, err := os.Stat(index); err != nil {
			http.Error(w, "Workspace IDE is not deployed on this server", http.StatusNotFound)
			return
		}
		w.Header().Set("Cache-Control", "no-cache")
		http.ServeFile(w, r, index)
	})
}
