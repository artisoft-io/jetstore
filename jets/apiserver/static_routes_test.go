package main

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/gorilla/mux"
)

// The Workspace IDE is registered as a PathPrefix ahead of the Flutter routes and
// ahead of the api routes, which puts it in a position to shadow either of them.
// gorilla/mux matches routes in registration order and PathPrefix matches on a
// prefix rather than the whole path, so "does the new route steal traffic from an
// old one" is a question about ordering that only a router can answer — reading
// the handler in isolation cannot.
//
// This mirrors the registration order in server.go rather than calling it, since
// the real function needs a database. If that order changes, change it here too;
// a divergence makes this test worse than useless.
func routerLikeServer(t *testing.T, uiDir, ideDir string) *mux.Router {
	t.Helper()
	r := mux.NewRouter()

	r.PathPrefix(ideAssetPrefix).
		Handler(ideHandler(ideAssetPrefix, ideDir)).
		Methods("GET")

	fs := http.FileServer(http.Dir(uiDir))
	r.Handle("/", fs).Methods("GET")
	r.Handle("/flutter.js", fs).Methods("GET")
	r.Handle("/main.dart.js", fs).Methods("GET")

	// A stand-in for the api routes, which are registered after the static ones.
	r.HandleFunc("/login", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("login-handler"))
	}).Methods("POST")

	return r
}

// flutterDir lays out a directory shaped like `flutter build web` output.
func flutterDir(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	for name, body := range map[string]string{
		"index.html":   "<html>flutter shell</html>",
		"flutter.js":   "// flutter loader",
		"main.dart.js": "// compiled app",
	} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return dir
}

func TestIdePrefixDoesNotShadowTheFlutterApp(t *testing.T) {
	r := routerLikeServer(t, flutterDir(t), buildIdeDir(t))

	// "/" is what the browser asks for when the user opens the app, including
	// when the url carries a "#/login" fragment — the fragment never reaches the
	// server, so a 404 here is what a user reports as "cannot get to the login
	// page".
	res := get(t, r, "/")
	if res.Code != http.StatusOK {
		t.Fatalf("GET /: got %d, want 200", res.Code)
	}
	if body := res.Body.String(); body != "<html>flutter shell</html>" {
		t.Errorf("GET / served %q, want the Flutter shell", body)
	}

	for _, asset := range []string{"/flutter.js", "/main.dart.js"} {
		if res := get(t, r, asset); res.Code != http.StatusOK {
			t.Errorf("GET %s: got %d, want 200", asset, res.Code)
		}
	}
}

func TestIdePrefixDoesNotShadowTheApiRoutes(t *testing.T) {
	r := routerLikeServer(t, flutterDir(t), buildIdeDir(t))

	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/login", nil))
	if rec.Code != http.StatusOK || rec.Body.String() != "login-handler" {
		t.Fatalf("POST /login: got %d %q, want 200 login-handler", rec.Code, rec.Body.String())
	}
}

func TestIdeRoutesStillResolveAlongsideTheFlutterApp(t *testing.T) {
	r := routerLikeServer(t, flutterDir(t), buildIdeDir(t))

	if res := get(t, r, "/ide/"); res.Code != http.StatusOK {
		t.Errorf("GET /ide/: got %d, want 200", res.Code)
	}
	if res := get(t, r, "/ide/assets/index-abc123.js"); res.Code != http.StatusOK {
		t.Errorf("GET /ide asset: got %d, want 200", res.Code)
	}
}

// TestFlutterRootIs404WithoutABundle reproduces the reported failure and pins its
// real cause, which is not routing. WEB_APP_DEPLOYMENT_DIR defaults to
// /usr/local/lib/web; on a development machine that path does not exist, so the
// FileServer behind "/" has nothing to serve and every Flutter url 404s while
// /ide/ keeps working. The fix is to point the flag at `flutter build web`
// output, not to change any route.
func TestFlutterRootIs404WithoutABundle(t *testing.T) {
	missing := filepath.Join(t.TempDir(), "not-deployed")
	r := routerLikeServer(t, missing, buildIdeDir(t))

	if res := get(t, r, "/"); res.Code != http.StatusNotFound {
		t.Fatalf("GET / with no bundle: got %d, want 404", res.Code)
	}
	// The IDE is unaffected, which is exactly the asymmetry that makes this look
	// like the IDE broke the Flutter app.
	if res := get(t, r, "/ide/"); res.Code != http.StatusOK {
		t.Fatalf("GET /ide/ with no Flutter bundle: got %d, want 200", res.Code)
	}
}
