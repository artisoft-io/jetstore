package main

import (
	"go/ast"
	"go/parser"
	"go/token"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"reflect"
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
		// A real file that is *not* under assets/ and is not the shell. Without
		// one, nothing exercises the branch that serves an existing unhashed
		// file, and that is the branch where `immutable` would be unrecoverable.
		"favicon.ico": "\x00icon",
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

	// The expected values are written out as literals rather than taken from the
	// constants in static_ide.go. Everything else here compares the handler with
	// itself; this one line is a second opinion about what should go on the wire,
	// so an edit to both constants still has to get past it.
	if res := get(t, h, "/"); res.Code != http.StatusOK {
		t.Fatalf("serving index: got %d, want 200", res.Code)
	} else if cc := res.Header().Get("Cache-Control"); cc != "no-cache" {
		t.Errorf("the real index.html: Cache-Control = %q, want no-cache — it names the hashed bundles, so it has to revalidate", cc)
	}

	// Pull the emitted asset urls straight out of the html and fetch each one.
	refs := regexp.MustCompile(`(?:src|href)="([^"]+)"`).FindAllStringSubmatch(string(index), -1)
	checked := 0
	hashed := 0
	for _, m := range refs {
		ref := m[1]
		if !strings.HasPrefix(ref, appAssetPrefix) {
			t.Errorf("asset %q is not under %q — vite `base` and the Go prefix disagree", ref, appAssetPrefix)
			continue
		}
		res := get(t, h, ref)
		if res.Code != http.StatusOK {
			t.Errorf("asset %q: got %d, want 200", ref, res.Code)
		}
		// Applied to the urls vite really emits, which is the one thing the
		// synthetic fixtures cannot check: a build that stopped writing into
		// assets/ would hand every bundle a policy meant for the shell.
		want := "no-cache"
		if strings.HasPrefix(ref, "/assets/") {
			want = "public, max-age=31536000, immutable"
			hashed++
		}
		if cc := res.Header().Get("Cache-Control"); cc != want {
			t.Errorf("real asset %q: Cache-Control = %q, want %q", ref, cc, want)
		}
		checked++
	}
	if hashed == 0 {
		t.Error("no emitted url was under /assets/; the immutable half of the policy reached nothing in this build")
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

// TestTheCachePolicyIsDerivedFromTheRequestPath is the whole of the policy in
// one table, and every row asserts both halves of it.
//
// The bug this pins is a pair, not a single mistake, and a test asking only
// "is some Cache-Control set" passes over both halves of it:
//
//   - the html shell served as cacheable is a browser that answers a reload from
//     disk without asking. The request never leaves the browser, so the server
//     cannot log it: the symptom is a blank screen beside a completely still
//     log, while curl from the same machine works perfectly. It goes away the
//     moment devtools is opened, because devtools disables the cache.
//   - an asset served `no-cache` is merely slow — a 949KB bundle revalidated on
//     every load — which is why it is the half nobody notices.
//
// So each row states the value it wants *and* names the other value as one it
// must not have. The exact-equality check alone would catch both today; the
// explicit negative is what keeps them legible as one bug when someone widens
// this table.
func TestTheCachePolicyIsDerivedFromTheRequestPath(t *testing.T) {
	h := appHandler(appAssetPrefix, buildIdeDir(t))

	for _, tc := range []struct {
		name    string
		target  string
		want    string
		notWant string
	}{
		// Content-hashed: the hash is the version, so the bytes under this url
		// can never change.
		{"hashed js", "/assets/index-abc123.js", immutableCachePolicy, revalidateCachePolicy},
		{"hashed css", "/assets/index-abc123.css", immutableCachePolicy, revalidateCachePolicy},

		// Unhashed, and each reaches ServeFile by a different route: the shell
		// through the fallback, the favicon through the real-file branch, the
		// deep link through the fallback with a path that is not "/". A policy
		// applied at one ServeFile call and not the other shows up here.
		{"the shell at the root", "/", revalidateCachePolicy, immutableCachePolicy},
		{"a real unhashed file", "/favicon.ico", revalidateCachePolicy, immutableCachePolicy},
		{"a deep client route", "/workspace/cedargate_ws/jet_rules/main.jr", revalidateCachePolicy, immutableCachePolicy},
		{"an unmatched route", "/no-such-page", revalidateCachePolicy, immutableCachePolicy},
	} {
		t.Run(tc.name, func(t *testing.T) {
			res := get(t, h, tc.target)
			if res.Code != http.StatusOK {
				t.Fatalf("GET %s: got %d, want 200", tc.target, res.Code)
			}
			got := res.Header().Get("Cache-Control")
			if got != tc.want {
				t.Errorf("GET %s: Cache-Control = %q, want %q", tc.target, got, tc.want)
			}
			if got == tc.notWant {
				t.Errorf("GET %s: Cache-Control = %q, which is the other half of the policy", tc.target, got)
			}
		})
	}
}

// TestTheTwoCachePoliciesAreDistinct stops the table above from going quiet.
//
// Every row asserts one constant and denies the other, so if the two constants
// were ever edited to the same string the table would still pass while asserting
// nothing at all. That is this repository's standing failure shape — a check
// that examined nothing reports clean — and one line closes it.
func TestTheTwoCachePoliciesAreDistinct(t *testing.T) {
	if immutableCachePolicy == revalidateCachePolicy {
		t.Fatalf("the two cache policies are the same string (%q); every policy test above is now vacuous", immutableCachePolicy)
	}
	// `no-store` would discard a working conditional request and buy nothing:
	// the shell must still be *cached*, merely revalidated.
	if strings.Contains(revalidateCachePolicy, "no-store") {
		t.Errorf("revalidateCachePolicy = %q; no-store throws away the conditional request that makes a reload cheap", revalidateCachePolicy)
	}
	if !strings.Contains(immutableCachePolicy, "immutable") {
		t.Errorf("immutableCachePolicy = %q, which no longer tells the browser not to revalidate", immutableCachePolicy)
	}
}

// TestEveryServeFileGoesThroughTheCachePolicy is the guard on the guard.
//
// The tests above check the paths that exist today. What put this defect in the
// tree in the first place is a *second* ServeFile call appearing beside the
// first and not carrying the policy with it — the tests stay green, because they
// only ask about the routes they know, and the new route ships with no
// directives at all.
//
// So the constraint is asserted against this file's own syntax tree rather than
// against a list of urls: ServeFile is called in exactly one function, that
// function is serveWithCachePolicy, and it is the only place Cache-Control is
// set. Adding a third call site fails here, naming the function it is in.
func TestEveryServeFileGoesThroughTheCachePolicy(t *testing.T) {
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, "static_ide.go", nil, parser.ParseComments)
	if err != nil {
		t.Fatal(err)
	}

	var serveFileIn, cacheControlIn []string
	for _, decl := range file.Decls {
		fn, ok := decl.(*ast.FuncDecl)
		if !ok {
			continue
		}
		ast.Inspect(fn, func(n ast.Node) bool {
			call, ok := n.(*ast.CallExpr)
			if !ok {
				return true
			}
			sel, ok := call.Fun.(*ast.SelectorExpr)
			if !ok {
				return true
			}
			if pkg, ok := sel.X.(*ast.Ident); ok && pkg.Name == "http" && sel.Sel.Name == "ServeFile" {
				serveFileIn = append(serveFileIn, fn.Name.Name)
			}
			// ...Header().Set("Cache-Control", ...) — matched on the literal
			// argument, so it catches the header wherever it is set from.
			if sel.Sel.Name == "Set" && len(call.Args) > 0 {
				if lit, ok := call.Args[0].(*ast.BasicLit); ok && lit.Kind == token.STRING && lit.Value == `"Cache-Control"` {
					cacheControlIn = append(cacheControlIn, fn.Name.Name)
				}
			}
			return true
		})
	}

	want := []string{"serveWithCachePolicy"}
	if !reflect.DeepEqual(serveFileIn, want) {
		t.Errorf("http.ServeFile is called in %v, want exactly %v — a call site outside the helper serves bytes with no cache policy", serveFileIn, want)
	}
	if !reflect.DeepEqual(cacheControlIn, want) {
		t.Errorf("Cache-Control is set in %v, want exactly %v — two places setting it is how the two paths drift apart", cacheControlIn, want)
	}
}
