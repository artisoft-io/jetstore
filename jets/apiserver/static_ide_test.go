package main

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// buildIdeDir lays out a directory shaped like a vite build.
func buildIdeDir(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, "assets"), 0o755); err != nil {
		t.Fatal(err)
	}
	files := map[string]string{
		"index.html":              "<!doctype html><div id=root></div>",
		"assets/index-abc123.js":  "console.log(1)",
		"assets/index-abc123.css": ".a{}",
	}
	for name, body := range files {
		if err := os.WriteFile(filepath.Join(dir, filepath.FromSlash(name)), []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return dir
}

func get(t *testing.T, h http.Handler, target string) *httptest.ResponseRecorder {
	t.Helper()
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, target, nil))
	return rec
}

func TestIdeHandlerServesHashedAssets(t *testing.T) {
	h := appHandler(appAssetPrefix, buildIdeDir(t))

	res := get(t, h, "/assets/index-abc123.js")
	if res.Code != http.StatusOK {
		t.Fatalf("got %d, want 200", res.Code)
	}
	if body := res.Body.String(); body != "console.log(1)" {
		t.Errorf("unexpected body %q", body)
	}
	// Hashed names change with content, so they are safe to cache indefinitely.
	if cc := res.Header().Get("Cache-Control"); cc != "public, max-age=31536000, immutable" {
		t.Errorf("asset Cache-Control = %q", cc)
	}
}

func TestIdeHandlerServesIndexAtRoot(t *testing.T) {
	h := appHandler(appAssetPrefix, buildIdeDir(t))
	res := get(t, h, "/")
	if res.Code != http.StatusOK {
		t.Fatalf("got %d, want 200", res.Code)
	}
	if cc := res.Header().Get("Cache-Control"); cc != "no-cache" {
		t.Errorf("index Cache-Control = %q, want no-cache", cc)
	}
}

func TestIdeHandlerFallsBackToIndexForClientRoutes(t *testing.T) {
	h := appHandler(appAssetPrefix, buildIdeDir(t))
	// A deep client-side route must survive a reload rather than 404.
	res := get(t, h, "/workspace/cedargate_ws/jet_rules/main.jr")
	if res.Code != http.StatusOK {
		t.Fatalf("got %d, want 200", res.Code)
	}
	if body := res.Body.String(); body != "<!doctype html><div id=root></div>" {
		t.Errorf("expected the html shell, got %q", body)
	}
}

func TestIdeHandlerDoesNotFallBackForMissingAssets(t *testing.T) {
	h := appHandler(appAssetPrefix, buildIdeDir(t))
	// Returning index.html here would surface as "unexpected token '<'" in the
	// browser, which points nowhere near the real problem.
	res := get(t, h, "/assets/index-deadbeef.js")
	if res.Code != http.StatusNotFound {
		t.Fatalf("got %d, want 404", res.Code)
	}
}

func TestIdeHandlerRejectsTraversal(t *testing.T) {
	dir := buildIdeDir(t)
	secret := filepath.Join(filepath.Dir(dir), "secret.txt")
	if err := os.WriteFile(secret, []byte("do not serve"), 0o644); err != nil {
		t.Fatal(err)
	}
	h := appHandler(appAssetPrefix, dir)

	for _, target := range []string{
		"/../secret.txt",
		"/assets/../../secret.txt",
		"/..%2f..%2fsecret.txt",
	} {
		res := get(t, h, target)
		if body := res.Body.String(); body == "do not serve" {
			t.Errorf("%s escaped the bundle directory", target)
		}
	}
}

// TestIdeHandlerServesTheRealBundle checks the one thing the synthetic fixtures
// above cannot: that the asset paths vite actually emits line up with the prefix
// this handler actually strips. Skipped unless the bundle has been built, since
// jetsclient_ide/dist is a build artifact and is not committed.
func TestIdeHandlerServesTheRealBundle(t *testing.T) {
	dir, err := filepath.Abs(filepath.Join("..", "..", "jetsclient_ide", "dist"))
	if err != nil {
		t.Fatal(err)
	}
	index, err := os.ReadFile(filepath.Join(dir, "index.html"))
	if err != nil {
		t.Skip("jetsclient_ide/dist not built; run `npm run build` in jetsclient_ide")
	}
	h := appHandler(appAssetPrefix, dir)

	if res := get(t, h, "/"); res.Code != http.StatusOK {
		t.Fatalf("serving index: got %d, want 200", res.Code)
	}

	// Pull the emitted asset urls straight out of the html and fetch each one.
	refs := regexp.MustCompile(`(?:src|href)="([^"]+)"`).FindAllStringSubmatch(string(index), -1)
	checked := 0
	for _, m := range refs {
		ref := m[1]
		if !strings.HasPrefix(ref, appAssetPrefix) {
			t.Errorf("asset %q is not under %q — vite `base` and the Go prefix disagree", ref, appAssetPrefix)
			continue
		}
		if res := get(t, h, ref); res.Code != http.StatusOK {
			t.Errorf("asset %q: got %d, want 200", ref, res.Code)
		}
		checked++
	}
	if checked == 0 {
		t.Error("index.html referenced no assets; the build looks wrong")
	}
}

// TestIdeShipsItsOwnFavicon guards a dependency that is easy to reintroduce by
// accident. With no <link rel="icon"> the browser falls back to /favicon.ico at
// the origin root — which was the *Flutter* app's icon until X.1 and is now this
// bundle's own file if one happens to be there and a 404 otherwise. The icon has
// to ship with this bundle and be declared, which is what stops the tab from
// depending on what else is in the deployment directory.
func TestIdeShipsItsOwnFavicon(t *testing.T) {
	dir, err := filepath.Abs(filepath.Join("..", "..", "jetsclient_ide", "dist"))
	if err != nil {
		t.Fatal(err)
	}
	index, err := os.ReadFile(filepath.Join(dir, "index.html"))
	if err != nil {
		t.Skip("jetsclient_ide/dist not built; run `npm run build` in jetsclient_ide")
	}

	icon := regexp.MustCompile(`rel="icon"[^>]*href="([^"]+)"`).FindSubmatch(index)
	if icon == nil {
		t.Fatal("index.html declares no rel=icon; the tab icon would depend on what else is in the deployment dir")
	}
	href := string(icon[1])
	if !strings.HasPrefix(href, appAssetPrefix) {
		t.Fatalf("favicon %q is not under %q, so it does not come from this bundle", href, appAssetPrefix)
	}
	if res := get(t, appHandler(appAssetPrefix, dir), href); res.Code != http.StatusOK {
		t.Fatalf("favicon %q: got %d, want 200", href, res.Code)
	}
}

func TestIdeHandlerReportsMissingDeployment(t *testing.T) {
	// An apiserver built without the IDE bundle should say so rather than 500.
	h := appHandler(appAssetPrefix, filepath.Join(t.TempDir(), "absent"))
	res := get(t, h, "/")
	if res.Code != http.StatusNotFound {
		t.Fatalf("got %d, want 404", res.Code)
	}
}
