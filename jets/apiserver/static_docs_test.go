package main

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/artisoft-io/jetstore/jets/user"
	jwt "github.com/golang-jwt/jwt/v5"
	"github.com/gorilla/mux"
)

const docsTestSecret = "docs-test-secret"

// withDocsTokenSettings sets the token globals for one test and restores them.
func withDocsTokenSettings(t *testing.T) {
	t.Helper()
	prevSecret, prevExp := user.ApiSecret, user.TokenExpiration
	user.ApiSecret, user.TokenExpiration = docsTestSecret, 60
	t.Cleanup(func() { user.ApiSecret, user.TokenExpiration = prevSecret, prevExp })
}

// buildDocsDir lays out a directory shaped like the built site, inside a
// parent that also holds a file the handler must never serve.
func buildDocsDir(t *testing.T, with404 bool) string {
	t.Helper()
	parent := t.TempDir()
	if err := os.WriteFile(filepath.Join(parent, "secret.txt"), []byte("outside-the-site"), 0o644); err != nil {
		t.Fatal(err)
	}
	dir := filepath.Join(parent, "docs")
	files := map[string]string{
		"index.html":           "<html>docs-landing</html>",
		"user/index.html":      "<html>user-guide</html>",
		"user/page/index.html": "<html>user-page</html>",
		"_astro/app.Ab12Cd.js": "console.log('docs')",
		"pagefind/pagefind.js": "// search",
		"favicon.svg":          "<svg/>",
		"empty/.keep":          "",
	}
	if with404 {
		files["404.html"] = "<html>site-not-found</html>"
	}
	for name, body := range files {
		p := filepath.Join(dir, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return dir
}

func validDocsToken(t *testing.T) string {
	t.Helper()
	tok, err := user.CreateToken("someone@example.com")
	if err != nil {
		t.Fatal(err)
	}
	return tok
}

func signedToken(t *testing.T, method jwt.SigningMethod, key any, claims jwt.MapClaims) string {
	t.Helper()
	tok, err := jwt.NewWithClaims(method, claims).SignedString(key)
	if err != nil {
		t.Fatal(err)
	}
	return tok
}

func docsGet(t *testing.T, h http.Handler, target, cookie string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(http.MethodGet, target, nil)
	if cookie != "" {
		req.AddCookie(&http.Cookie{Name: docsCookieName, Value: cookie})
	}
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec
}

func assertDocsUnauthorized(t *testing.T, rec *httptest.ResponseRecorder, what string) {
	t.Helper()
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("%s: got %d, want 401", what, rec.Code)
	}
	body := rec.Body.String()
	for _, want := range []string{
		"<title>JetStore documentation</title>",
		"Open the documentation from JetStore's Help button.",
		`href="/"`,
	} {
		if !strings.Contains(body, want) {
			t.Errorf("%s: 401 page lacks %q", what, want)
		}
	}
	if strings.Contains(body, "id=root") || strings.Contains(body, "docs-landing") {
		t.Errorf("%s: 401 served content: %q", what, body)
	}
	if ct := rec.Header().Get("Content-Type"); !strings.HasPrefix(ct, "text/html") {
		t.Errorf("%s: Content-Type %q, want text/html", what, ct)
	}
}

func TestDocsWithoutCookieIs401(t *testing.T) {
	withDocsTokenSettings(t)
	h := docsHandler(buildDocsDir(t, true))
	for _, p := range []string{"/docs/", "/docs/user/", "/docs/_astro/app.Ab12Cd.js", "/docs/missing/"} {
		assertDocsUnauthorized(t, docsGet(t, h, p, ""), "no cookie "+p)
	}
}

func TestDocsRejectsInvalidTokens(t *testing.T) {
	withDocsTokenSettings(t)
	h := docsHandler(buildDocsDir(t, true))
	future := time.Now().Add(time.Hour).Unix()
	cases := map[string]string{
		"garbage":       "not-a-jwt",
		"bad signature": signedToken(t, jwt.SigningMethodHS256, []byte("another-secret"), jwt.MapClaims{"email": "a@b.c", "exp": future}),
		"expired":       signedToken(t, jwt.SigningMethodHS256, []byte(docsTestSecret), jwt.MapClaims{"email": "a@b.c", "exp": time.Now().Add(-time.Minute).Unix()}),
		"alg none":      signedToken(t, jwt.SigningMethodNone, jwt.UnsafeAllowNoneSignatureType, jwt.MapClaims{"email": "a@b.c", "exp": future}),
		"no email":      signedToken(t, jwt.SigningMethodHS256, []byte(docsTestSecret), jwt.MapClaims{"exp": future}),
		"email not str": signedToken(t, jwt.SigningMethodHS256, []byte(docsTestSecret), jwt.MapClaims{"email": 42, "exp": future}),
	}
	for name, tok := range cases {
		assertDocsUnauthorized(t, docsGet(t, h, "/docs/", tok), name)
	}
}

func TestDocsServesFilesWithAValidCookie(t *testing.T) {
	withDocsTokenSettings(t)
	h := docsHandler(buildDocsDir(t, true))
	tok := validDocsToken(t)

	cases := []struct{ path, body string }{
		{"/docs/", "docs-landing"},
		{"/docs/user/", "user-guide"},
		{"/docs/user/page/", "user-page"},
		{"/docs/favicon.svg", "<svg/>"},
		{"/docs/user/page/index.html", "user-page"},
	}
	for _, c := range cases {
		rec := docsGet(t, h, c.path, tok)
		if rec.Code != http.StatusOK {
			t.Errorf("GET %s: got %d, want 200", c.path, rec.Code)
			continue
		}
		if got := rec.Body.String(); !strings.Contains(got, c.body) {
			t.Errorf("GET %s: body %q, want %q", c.path, got, c.body)
		}
	}
}

func TestDocsDirectoryWithoutSlashRedirects(t *testing.T) {
	withDocsTokenSettings(t)
	h := docsHandler(buildDocsDir(t, true))
	rec := docsGet(t, h, "/docs/user/page?x=1", validDocsToken(t))
	if rec.Code != http.StatusMovedPermanently {
		t.Fatalf("got %d, want 301", rec.Code)
	}
	if loc := rec.Header().Get("Location"); loc != "/docs/user/page/?x=1" {
		t.Errorf("Location %q, want /docs/user/page/?x=1", loc)
	}
}

func TestDocsMissingPageServesTheSites404(t *testing.T) {
	withDocsTokenSettings(t)
	h := docsHandler(buildDocsDir(t, true))
	tok := validDocsToken(t)
	// A missing file, a missing directory, a directory with no index.html, and a
	// missing hashed asset.
	for _, p := range []string{"/docs/nope.html", "/docs/user/nope/", "/docs/empty/", "/docs/_astro/gone.js"} {
		rec := docsGet(t, h, p, tok)
		if rec.Code != http.StatusNotFound {
			t.Errorf("GET %s: got %d, want 404", p, rec.Code)
			continue
		}
		if got := rec.Body.String(); !strings.Contains(got, "site-not-found") {
			t.Errorf("GET %s: body %q, want the site's 404.html", p, got)
		}
		if cc := rec.Header().Get("Cache-Control"); strings.Contains(cc, "immutable") {
			t.Errorf("GET %s: a 404 carries %q", p, cc)
		}
	}
}

func TestDocsMissingPageWithout404HtmlIsPlain404(t *testing.T) {
	withDocsTokenSettings(t)
	h := docsHandler(buildDocsDir(t, false))
	rec := docsGet(t, h, "/docs/nope/", validDocsToken(t))
	if rec.Code != http.StatusNotFound {
		t.Fatalf("got %d, want 404", rec.Code)
	}
	if strings.Contains(rec.Body.String(), "<html") {
		t.Errorf("plain 404 expected, got %q", rec.Body.String())
	}
}

func TestDocsNotDeployedIs404(t *testing.T) {
	withDocsTokenSettings(t)
	h := docsHandler(filepath.Join(t.TempDir(), "not-deployed"))
	if rec := docsGet(t, h, "/docs/", validDocsToken(t)); rec.Code != http.StatusNotFound {
		t.Fatalf("got %d, want 404", rec.Code)
	}
}

func TestDocsRejectsTraversal(t *testing.T) {
	withDocsTokenSettings(t)
	h := docsHandler(buildDocsDir(t, true))
	tok := validDocsToken(t)
	for _, p := range []string{
		"/docs/../secret.txt",
		"/docs/user/../../secret.txt",
		"/docs/%2e%2e/secret.txt",
		"/docs/user/%2e%2e/%2e%2e/%2e%2e/secret.txt",
	} {
		for _, cookie := range []string{tok, ""} {
			rec := docsGet(t, h, p, cookie)
			if strings.Contains(rec.Body.String(), "outside-the-site") {
				t.Errorf("GET %s served a file outside the site", p)
			}
			if rec.Code == http.StatusOK {
				t.Errorf("GET %s: got 200", p)
			}
		}
	}
}

func TestDocsRootRedirectsKeepingTheQuery(t *testing.T) {
	withDocsTokenSettings(t)
	h := docsHandler(buildDocsDir(t, true))
	for target, want := range map[string]string{
		"/docs":          "/docs/",
		"/docs?q=search": "/docs/?q=search",
	} {
		rec := docsGet(t, h, target, "")
		if rec.Code != http.StatusMovedPermanently {
			t.Errorf("GET %s: got %d, want 301", target, rec.Code)
			continue
		}
		if loc := rec.Header().Get("Location"); loc != want {
			t.Errorf("GET %s: Location %q, want %q", target, loc, want)
		}
	}
}

func TestDocsCachePolicy(t *testing.T) {
	withDocsTokenSettings(t)
	h := docsHandler(buildDocsDir(t, true))
	tok := validDocsToken(t)

	rec := docsGet(t, h, "/docs/_astro/app.Ab12Cd.js", tok)
	if rec.Code != http.StatusOK {
		t.Fatalf("hashed asset: got %d", rec.Code)
	}
	if cc := rec.Header().Get("Cache-Control"); cc != docsImmutableCachePolicy {
		t.Errorf("hashed asset: Cache-Control %q, want %q", cc, docsImmutableCachePolicy)
	}
	if !strings.Contains(docsImmutableCachePolicy, "max-age=31536000") || !strings.Contains(docsImmutableCachePolicy, "immutable") {
		t.Errorf("immutable policy is %q", docsImmutableCachePolicy)
	}

	for _, p := range []string{"/docs/", "/docs/user/", "/docs/favicon.svg", "/docs/pagefind/pagefind.js"} {
		rec := docsGet(t, h, p, tok)
		if cc := rec.Header().Get("Cache-Control"); !strings.Contains(cc, "no-cache") || strings.Contains(cc, "immutable") {
			t.Errorf("GET %s: Cache-Control %q, want no-cache", p, cc)
		}
	}
}

// docsSessionChain is the handler chain server.go registers for POST /docsSession.
func docsSessionChain() http.HandlerFunc {
	return jsonh(corsh(authh(docsSession)))
}

func TestDocsSessionWithoutBearerIs401AndSetsNoCookie(t *testing.T) {
	withDocsTokenSettings(t)
	for name, auth := range map[string]string{
		"no header":     "",
		"bad signature": "Bearer " + signedToken(t, jwt.SigningMethodHS256, []byte("another-secret"), jwt.MapClaims{"email": "a@b.c", "exp": time.Now().Add(time.Hour).Unix()}),
	} {
		req := httptest.NewRequest(http.MethodPost, "/docsSession", nil)
		if auth != "" {
			req.Header.Set("Authorization", auth)
		}
		// A client-supplied Token header must not become the cookie.
		req.Header.Set("Token", "client-supplied")
		rec := httptest.NewRecorder()
		docsSessionChain()(rec, req)
		if rec.Code != http.StatusUnauthorized {
			t.Errorf("%s: got %d, want 401", name, rec.Code)
		}
		if sc := rec.Header().Values("Set-Cookie"); len(sc) != 0 {
			t.Errorf("%s: set a cookie: %v", name, sc)
		}
	}
}

func TestDocsSessionSetsTheCookie(t *testing.T) {
	withDocsTokenSettings(t)
	req := httptest.NewRequest(http.MethodPost, "/docsSession", nil)
	req.Header.Set("Authorization", "Bearer "+validDocsToken(t))
	rec := httptest.NewRecorder()
	docsSessionChain()(rec, req)
	if rec.Code != http.StatusNoContent {
		t.Fatalf("got %d, want 204 (%s)", rec.Code, rec.Body.String())
	}
	if rec.Body.Len() != 0 {
		t.Errorf("204 carries a body: %q", rec.Body.String())
	}
	cookies := rec.Result().Cookies()
	if len(cookies) != 1 {
		t.Fatalf("got %d cookies, want 1", len(cookies))
	}
	c := cookies[0]
	if c.Name != docsCookieName {
		t.Errorf("name %q, want %q", c.Name, docsCookieName)
	}
	if email, err := user.TokenStringValid(c.Value); err != nil || email != "someone@example.com" {
		t.Errorf("cookie value is not a valid token for the caller: %q, %v", email, err)
	}
	if !c.HttpOnly {
		t.Error("cookie is not HttpOnly")
	}
	if !c.Secure {
		t.Error("cookie is not Secure")
	}
	if c.SameSite != http.SameSiteStrictMode {
		t.Errorf("SameSite %v, want Strict", c.SameSite)
	}
	if c.Path != "/docs/" {
		t.Errorf("Path %q, want /docs/", c.Path)
	}
	if c.MaxAge != 60*60 {
		t.Errorf("Max-Age %d, want %d", c.MaxAge, 60*60)
	}
	raw := rec.Header().Get("Set-Cookie")
	for _, attr := range []string{"HttpOnly", "Secure", "SameSite=Strict", "Path=/docs/", "Max-Age=3600"} {
		if !strings.Contains(raw, attr) {
			t.Errorf("Set-Cookie %q lacks %s", raw, attr)
		}
	}
}

// routerLikeServerWithDocs mirrors server.go's order: api routes, the docs
// routes, then the web app's catch-all.
func routerLikeServerWithDocs(t *testing.T, docsDir string) *mux.Router {
	t.Helper()
	r := mux.NewRouter()
	r.HandleFunc("/login", func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte("login-handler"))
	}).Methods("POST")
	r.HandleFunc("/docsSession", docsSessionChain()).Methods("POST")
	r.MatcherFunc(isDocsPath).Handler(docsHandler(docsDir)).Methods("GET")
	r.PathPrefix(appAssetPrefix).
		Handler(appHandler(appAssetPrefix, buildIdeDir(t))).
		Methods("GET")
	return r
}

func TestDocsRoutesAreNotShadowedByTheApp(t *testing.T) {
	withDocsTokenSettings(t)
	r := routerLikeServerWithDocs(t, buildDocsDir(t, true))

	for _, p := range []string{"/docs/x", "/docs/", "/docs/user/page/"} {
		rec := docsGet(t, r, p, "")
		assertDocsUnauthorized(t, rec, "router "+p)
	}
	if rec := docsGet(t, r, "/docs", ""); rec.Code != http.StatusMovedPermanently {
		t.Errorf("router /docs: got %d, want 301", rec.Code)
	}
	// A missing page is the site's 404, not the app shell.
	rec := docsGet(t, r, "/docs/x", validDocsToken(t))
	if rec.Code != http.StatusNotFound || strings.Contains(rec.Body.String(), "id=root") {
		t.Errorf("router /docs/x with cookie: got %d %q, want the site's 404", rec.Code, rec.Body.String())
	}
	// Paths that only share the prefix's letters stay the app's.
	for _, p := range []string{"/docsfoo", "/documents/1"} {
		if rec := docsGet(t, r, p, ""); rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), "id=root") {
			t.Errorf("router %s: got %d %q, want the app shell", p, rec.Code, rec.Body.String())
		}
	}
	// The router cleans ".." before any handler sees it.
	rec = docsGet(t, r, "/docs/../secret.txt", validDocsToken(t))
	if strings.Contains(rec.Body.String(), "outside-the-site") {
		t.Error("router served a file outside the site")
	}
}

func TestDocsSessionCookieOpensTheSite(t *testing.T) {
	withDocsTokenSettings(t)
	r := routerLikeServerWithDocs(t, buildDocsDir(t, true))

	req := httptest.NewRequest(http.MethodPost, "/docsSession", nil)
	req.Header.Set("Authorization", "Bearer "+validDocsToken(t))
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	if rec.Code != http.StatusNoContent {
		t.Fatalf("POST /docsSession: got %d, want 204", rec.Code)
	}
	cookies := rec.Result().Cookies()
	if len(cookies) != 1 {
		t.Fatalf("got %d cookies", len(cookies))
	}
	got := docsGet(t, r, "/docs/user/", cookies[0].Value)
	if got.Code != http.StatusOK || !strings.Contains(got.Body.String(), "user-guide") {
		t.Errorf("GET /docs/user/ with the session cookie: got %d %q", got.Code, got.Body.String())
	}
}

// TestTheDocsRegistrationIsAboveTheCatchAllInServerGo reads the real order,
// since the router above is a copy of it.
func TestTheDocsRegistrationIsAboveTheCatchAllInServerGo(t *testing.T) {
	source, err := os.ReadFile("server.go")
	if err != nil {
		t.Fatal(err)
	}
	src := string(source)
	static := strings.Index(src, "server.Router.PathPrefix(appAssetPrefix)")
	if static < 0 {
		t.Fatal("server.go does not register the app over a path prefix any more; this test is stale")
	}
	for _, reg := range []string{
		`server.Router.HandleFunc("/docsSession", jsonh(corsh(authh(docsSession))))`,
		`server.Router.MatcherFunc(isDocsPath)`,
	} {
		at := strings.Index(src, reg)
		if at < 0 {
			t.Errorf("server.go does not register %s", reg)
			continue
		}
		if at > static {
			t.Errorf("%s is registered after the catch-all, so the app shadows it", reg)
		}
	}
}
