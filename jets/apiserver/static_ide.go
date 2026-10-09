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

// assetPathPrefix is the url space vite writes content-hashed files into. It is
// the *address* the policy below is derived from, never a directory listing and
// never a file extension: the hash is in the name, so the url is the only place
// the "these bytes can never change" claim is legible.
const assetPathPrefix = "/assets/"

// indexFileName is the html shell — the one file in the bundle that carries no
// content hash, because it is what names the hashed ones.
const indexFileName = "index.html"

// The two halves of the cache policy, and they are each other's inverse.
//
//   - immutableCachePolicy is safe only because the name carries a content hash.
//     Applied to anything unhashed it is close to unrecoverable: a browser
//     holding an immutable index.html cannot be told about a new deploy at all,
//     short of the user clearing site data.
//   - revalidateCachePolicy is `no-cache`, which does *not* mean "do not cache".
//     It caches and forces a conditional request, so an unchanged shell still
//     costs one 304 rather than 2KB. `no-store` would be wrong here — it throws
//     away a working conditional request and buys nothing.
//
// The failure this exists to prevent is silent in one direction only. With no
// directive at all a browser applies *heuristic* freshness and may answer a
// reload entirely from disk cache: the request never leaves the browser, so the
// server cannot log it, and the symptom is a blank screen beside a completely
// still log while curl from the same machine works perfectly.
const (
	immutableCachePolicy  = "public, max-age=31536000, immutable"
	revalidateCachePolicy = "no-cache"
)

// cachePolicyFor returns the Cache-Control value for bytes addressed by clean.
//
// clean is a cleaned url path, and deriving the policy from it is the whole
// design: a hand-kept list of asset directories is a second spelling of the
// build's output layout, and it goes stale on the day that layout changes —
// silently, and in the dangerous direction, because the fallback arm of such a
// list is the one that hands out `immutable`.
func cachePolicyFor(clean string) string {
	if strings.HasPrefix(clean, assetPathPrefix) {
		return immutableCachePolicy
	}
	return revalidateCachePolicy
}

// serveWithCachePolicy is the only way bytes leave this file.
//
// clean is the address of *the bytes being served*, which is not always the
// request path: the SPA fallback answers /whatever with index.html and must ask
// about index.html. Two ServeFile calls each setting their own header is exactly
// how this defect comes back — a policy applied at one of them and not the other
// — so the header and the send are one call, and
// TestEveryServeFileGoesThroughTheCachePolicy reads this file's syntax tree to
// keep it that way.
func serveWithCachePolicy(w http.ResponseWriter, r *http.Request, clean, file string) {
	w.Header().Set("Cache-Control", cachePolicyFor(clean))
	http.ServeFile(w, r, file)
}

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
			serveWithCachePolicy(w, r, clean, target)
			return
		}

		// A missing asset must 404 rather than silently returning the html shell:
		// handing index.html to a request for a .js file produces a console error
		// about an unexpected '<' that says nothing about the real cause.
		if strings.HasPrefix(clean, assetPathPrefix) {
			http.NotFound(w, r)
			return
		}

		index := filepath.Join(dir, indexFileName)
		if _, err := os.Stat(index); err != nil {
			http.Error(w, "Workspace IDE is not deployed on this server", http.StatusNotFound)
			return
		}
		// The address asked about is index.html's own, not the request's. These
		// are the same bytes as `GET /` and must carry the same policy whatever
		// url reached them — and asking about `clean` would make the fallback's
		// policy a function of the caller's path, so removing the /assets/ 404
		// above would quietly start serving the html shell as `immutable`.
		serveWithCachePolicy(w, r, "/"+indexFileName, index)
	})
}
