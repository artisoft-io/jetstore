package main

import (
	"errors"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/artisoft-io/jetstore/jets/user"
	"github.com/gorilla/mux"
)

// Serving for the documentation site (docs/), under /docs/.
//
// The site is static, but it carries installation-specific pages, so it is
// served only with a cookie set by an authenticated call (POST /docsSession).
// The React app holds its token in memory, so a new browser tab has no other
// way to prove who it is. The cookie carries an ordinary api token and is
// checked with the same function as the Authorization header.
//
// This is a file server, not a single-page app: a missing page is the site's
// own 404.html, never the React shell.

const (
	// docsPathPrefix is the url space of the site; docsRootPath redirects to it.
	docsPathPrefix = "/docs/"
	docsRootPath   = "/docs"

	// docsCookieName is set by POST /docsSession and read only by this file.
	docsCookieName = "jetstore_docs"

	// docsAssetPathPrefix is where the site generator writes content-hashed
	// files, relative to the site root.
	docsAssetPathPrefix = "/_astro/"

	docsNotFoundFile = "404.html"
)

// The documentation is authenticated content, so both policies are private:
// a shared cache must not hand it to a client without the cookie.
const (
	docsImmutableCachePolicy  = "private, max-age=31536000, immutable"
	docsRevalidateCachePolicy = "private, no-cache"
)

// docsUnauthorizedPage is answered with 401 when the cookie is missing or
// invalid. Self-contained: no stylesheet or script, since those would be
// refused too.
const docsUnauthorizedPage = `<!doctype html>
<html lang="en">
<head>
<meta charset="utf-8">
<meta name="viewport" content="width=device-width, initial-scale=1">
<title>JetStore documentation</title>
<style>
body{font-family:system-ui,sans-serif;max-width:36rem;margin:4rem auto;padding:0 1rem;line-height:1.5}
</style>
</head>
<body>
<h1>JetStore documentation</h1>
<p>Open the documentation from JetStore's Help button.</p>
<p><a href="/">Go to JetStore</a></p>
</body>
</html>
`

// isDocsPath matches /docs and everything under /docs/, and nothing else
// (not /docsSession, not /docsfoo).
func isDocsPath(r *http.Request, _ *mux.RouteMatch) bool {
	p := r.URL.Path
	return p == docsRootPath || strings.HasPrefix(p, docsPathPrefix)
}

// docsCachePolicyFor returns the Cache-Control value for the file addressed by
// clean, a cleaned path relative to the site root.
func docsCachePolicyFor(clean string) string {
	if strings.HasPrefix(clean, docsAssetPathPrefix) {
		return docsImmutableCachePolicy
	}
	return docsRevalidateCachePolicy
}

// docsHandler serves the documentation site from dir.
func docsHandler(dir string) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == docsRootPath {
			redirectKeepingQuery(w, r, docsPathPrefix)
			return
		}

		if _, err := docsCookieValid(r); err != nil {
			writeDocsUnauthorized(w)
			return
		}

		// Clean the absolute form before joining, so ".." cannot leave dir.
		rel := strings.TrimPrefix(r.URL.Path, docsRootPath)
		clean := path.Clean("/" + rel)
		target := filepath.Join(dir, filepath.FromSlash(clean))

		info, err := os.Stat(target)
		if err == nil && info.IsDir() {
			if !strings.HasSuffix(r.URL.Path, "/") {
				// Relative links in the index resolve against the directory,
				// so address it with its slash, as http.FileServer does.
				redirectKeepingQuery(w, r, path.Join(docsRootPath, clean)+"/")
				return
			}
			clean = path.Join(clean, "index.html")
			target = filepath.Join(target, "index.html")
			info, err = os.Stat(target)
		}
		if err != nil || info.IsDir() {
			writeDocsNotFound(w, dir)
			return
		}
		serveDocsFile(w, r, clean, target)
	})
}

// docsCookieValid returns the email carried by the documentation cookie.
func docsCookieValid(r *http.Request) (string, error) {
	c, err := r.Cookie(docsCookieName)
	if err != nil {
		return "", err
	}
	if c.Value == "" {
		return "", errors.New("empty documentation cookie")
	}
	return user.TokenStringValid(c.Value)
}

// serveDocsFile is the only way a file of the site leaves this handler with
// status 200, so the cache policy and the send cannot drift apart.
func serveDocsFile(w http.ResponseWriter, r *http.Request, clean, file string) {
	f, err := os.Open(file)
	if err != nil {
		http.Error(w, "404 page not found", http.StatusNotFound)
		return
	}
	defer f.Close()
	info, err := f.Stat()
	if err != nil {
		http.Error(w, "404 page not found", http.StatusNotFound)
		return
	}
	w.Header().Set("Cache-Control", docsCachePolicyFor(clean))
	// ServeContent rather than ServeFile: ServeFile redirects any request
	// ending in /index.html and refuses a raw path holding "..", neither of
	// which belongs here once the path is cleaned.
	http.ServeContent(w, r, filepath.Base(file), info.ModTime(), f)
}

// writeDocsNotFound answers 404 with the site's own 404.html, or a plain 404
// when the site has none (for instance when it is not deployed).
func writeDocsNotFound(w http.ResponseWriter, dir string) {
	w.Header().Set("Cache-Control", docsRevalidateCachePolicy)
	body, err := os.ReadFile(filepath.Join(dir, docsNotFoundFile))
	if err != nil {
		http.Error(w, "404 page not found", http.StatusNotFound)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Content-Length", strconv.Itoa(len(body)))
	w.WriteHeader(http.StatusNotFound)
	_, _ = w.Write(body)
}

func writeDocsUnauthorized(w http.ResponseWriter) {
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(http.StatusUnauthorized)
	_, _ = w.Write([]byte(docsUnauthorizedPage))
}

func redirectKeepingQuery(w http.ResponseWriter, r *http.Request, to string) {
	if r.URL.RawQuery != "" {
		to += "?" + r.URL.RawQuery
	}
	http.Redirect(w, r, to, http.StatusMovedPermanently)
}

// docsSession sets the documentation cookie to the fresh token authh put in
// the request, and answers 204. Registered behind authh; the token is
// checked again here so the handler cannot set an unchecked value if it is
// ever registered without it.
func docsSession(w http.ResponseWriter, r *http.Request) {
	token := r.Header.Get("Token")
	if _, err := user.TokenStringValid(token); err != nil {
		ERROR(w, http.StatusUnauthorized, errors.New("Unauthorized"))
		return
	}
	http.SetCookie(w, &http.Cookie{
		Name:     docsCookieName,
		Value:    token,
		Path:     docsPathPrefix,
		MaxAge:   user.TokenExpiration * 60,
		HttpOnly: true,
		Secure:   true,
		SameSite: http.SameSiteStrictMode,
	})
	w.WriteHeader(http.StatusNoContent)
}
