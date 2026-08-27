package main

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gorilla/mux"
)

// The web app is registered as a `PathPrefix("/")` — it matches every path — so
// "does it steal traffic from an api route" is a question about registration
// order that only a router can answer. Reading the handler in isolation cannot.
//
// **The question changed shape at X.1 and got harder, not easier.** While the
// prefix was `/ide/` the risk was bounded by the prefix: the worst case was the
// IDE stealing its own url space. At `/` the prefix matches everything, and the
// only thing standing between an api call and the html shell is that gorilla/mux
// matches in registration order and the static registration sits at the bottom of
// `Run`. That is a property of one line's *position* in a 200-line function,
// which is exactly the kind of thing that survives a refactor by accident.
//
// So this file does two things: mirrors the order to test the behaviour, and
// asserts against `server.go`'s source that the order it mirrors is the real one.
func routerLikeServer(t *testing.T, appDir string) *mux.Router {
	t.Helper()
	r := mux.NewRouter()

	// The api routes come first, as they do in server.go.
	r.HandleFunc("/login", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("login-handler"))
	}).Methods("POST")
	r.HandleFunc("/healthcheck/status", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	}).Methods("GET")

	// And the app takes what is left.
	r.PathPrefix(appAssetPrefix).
		Handler(appHandler(appAssetPrefix, appDir)).
		Methods("GET")

	return r
}

// TestTheStaticRegistrationIsLastInServerGo is the guard on the guard.
//
// The helper above is a copy of an order that lives somewhere else, and a copy
// that drifts makes every test below worse than useless — it would keep passing
// while production served html to `/dataTable`. So the order is read out of the
// source rather than trusted: the static registration must come *after* the api
// routes in the file that registers them.
func TestTheStaticRegistrationIsLastInServerGo(t *testing.T) {
	source, err := os.ReadFile("server.go")
	if err != nil {
		t.Fatal(err)
	}
	src := string(source)

	static := strings.Index(src, "server.Router.PathPrefix(appAssetPrefix)")
	if static < 0 {
		t.Fatal("server.go does not register the app over a path prefix any more; this file is stale")
	}
	// Every api route that must win its own path. `/login` is the one that would
	// hurt most — a user who cannot sign in has no way to report anything.
	for _, api := range []string{
		`server.Router.HandleFunc("/login"`,
		`server.Router.HandleFunc("/register"`,
		`server.Router.HandleFunc("/dataTable"`,
		`server.Router.HandleFunc("/purgeData"`,
		`server.Router.HandleFunc("/healthcheck/status"`,
	} {
		at := strings.Index(src, api)
		if at < 0 {
			t.Errorf("server.go no longer registers %s; this list is stale", api)
			continue
		}
		if at > static {
			t.Errorf("%s is registered after the catch-all, so the app shadows it", api)
		}
	}
}

// buildAppDir lays out a directory shaped like a vite build.
func buildAppDir(t *testing.T) string {
	t.Helper()
	return buildIdeDir(t)
}

func TestTheCatchAllDoesNotShadowTheApiRoutes(t *testing.T) {
	r := routerLikeServer(t, buildAppDir(t))

	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/login", nil))
	if rec.Code != http.StatusOK || rec.Body.String() != "login-handler" {
		t.Fatalf("POST /login: got %d %q, want 200 login-handler", rec.Code, rec.Body.String())
	}

	// A GET api route is the interesting one, because the catch-all is GET-only:
	// a POST route could never be shadowed by it whatever the order.
	if res := get(t, r, "/healthcheck/status"); res.Body.String() != "ok" {
		t.Errorf("GET /healthcheck/status served %q, want the health check", res.Body.String())
	}
}

func TestTheAppIsServedAtTheRoot(t *testing.T) {
	r := routerLikeServer(t, buildAppDir(t))

	res := get(t, r, "/")
	if res.Code != http.StatusOK {
		t.Fatalf("GET /: got %d, want 200", res.Code)
	}
	if body := res.Body.String(); !strings.Contains(body, "id=root") {
		t.Errorf("GET / served %q, want the app shell", body)
	}
	if res := get(t, r, "/assets/index-abc123.js"); res.Code != http.StatusOK {
		t.Errorf("GET an asset: got %d, want 200", res.Code)
	}
}

// TestADeepLinkSurvivesAReload is the reason a catch-all became necessary.
//
// `jetsclient` never called `setPathUrlStrategy`, so every Flutter route lived
// after the `#` and never reached the server — which is why 64 enumerated asset
// routes and no fallback were enough for years. React uses real paths, so a
// reload on any in-app route is a GET the server has to answer with the shell.
// Without this, the app works until someone presses F5.
func TestADeepLinkSurvivesAReload(t *testing.T) {
	r := routerLikeServer(t, buildAppDir(t))

	for _, deep := range []string{
		"/workspace",
		"/workspaces/cedargate_ws/home",
		"/flow/loadFilesUF",
		"/processErrors/123",
		"/register",
	} {
		res := get(t, r, deep)
		if res.Code != http.StatusOK {
			t.Errorf("GET %s: got %d, want 200", deep, res.Code)
			continue
		}
		if !strings.Contains(res.Body.String(), "id=root") {
			t.Errorf("GET %s did not serve the app shell", deep)
		}
	}
}

// A missing bundle is a deployment error and must look like one.
//
// This replaces a test that pinned the same failure for the Flutter bundle:
// `WEB_APP_DEPLOYMENT_DIR` defaulted to a path that does not exist on a
// development machine, so every Flutter url 404'd while `/ide/` kept working, and
// the asymmetry made it look like the IDE had broken the Flutter app. There is no
// asymmetry left to be confused by — there is one bundle — but the failure mode
// is the same and is worth keeping named.
func TestRootIs404WithoutABundle(t *testing.T) {
	missing := filepath.Join(t.TempDir(), "not-deployed")
	r := routerLikeServer(t, missing)

	if res := get(t, r, "/"); res.Code != http.StatusNotFound {
		t.Fatalf("GET / with no bundle: got %d, want 404", res.Code)
	}
}
