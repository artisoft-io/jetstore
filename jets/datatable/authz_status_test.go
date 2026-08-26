package datatable

import (
	"go/ast"
	"go/parser"
	"go/token"
	"net/http"
	"testing"
)

// The guard for ui_refresh C.17 / I-189: a capability refusal and a dead session
// are two answers and 401 could only carry one of them.
//
// **What this file asserts is a mapping, not a mechanism.** Exercising a gate needs
// a database, a token and a seeded role, so nothing here proves a 403 reaches a
// browser -- the same limitation read_capability_test.go and write_capability_test.go
// state for themselves. What it can prove, and what the defect was made of, is that
// the classification exists, that it is applied at every gate rather than at the one
// somebody remembered, and that the message did not move while the status did.

// TestAuthzStatusSeparatesIdentityFromPolicy pins the split the whole change rests
// on. The two policy refusals are 403 -- we know who is asking and the answer is no.
// Everything else is 401.
func TestAuthzStatusSeparatesIdentityFromPolicy(t *testing.T) {
	for _, tc := range []struct {
		name string
		err  error
		want int
	}{
		{"missing capability", ErrMissingCapability, http.StatusForbidden},
		{"admin only", ErrAdminOnly, http.StatusForbidden},
		{"cannot get user info", ErrNoUserInfo, http.StatusUnauthorized},
		{"capability not configured", ErrCapabilityNotConfigured, http.StatusUnauthorized},
		{"an error from somewhere else", errString("boom"), http.StatusUnauthorized},
	} {
		if got := AuthzStatusFor(tc.err); got != tc.want {
			t.Errorf("AuthzStatusFor(%s) = %d, want %d", tc.name, got, tc.want)
		}
	}
}

type errString string

func (e errString) Error() string { return string(e) }

// TestRefusalKeepsTheMessageCollapsed is the other half, and it is the half that
// keeps I-124's fix intact. That fix turns on the two gates on the insert_raw_rows
// path being byte-identical so neither becomes an oracle for whether a mapping
// exists; a status that varies is fine there, because both gates on that path
// demand the same capability and therefore refuse with the same status.
//
// **The wire message must not gain the distinction the status now carries.**
func TestRefusalKeepsTheMessageCollapsed(t *testing.T) {
	const want = "error: unauthorized, cannot get user info or does not have permission"
	for _, err := range []error{ErrMissingCapability, ErrAdminOnly, ErrNoUserInfo, ErrCapabilityNotConfigured} {
		status, refusal := RefusalFor(err)
		if refusal.Error() != want {
			t.Errorf("RefusalFor(%v) returned %q; every gate in this package answers %q",
				err, refusal.Error(), want)
		}
		if status != http.StatusUnauthorized && status != http.StatusForbidden {
			t.Errorf("RefusalFor(%v) returned status %d; a refusal is one of the two", err, status)
		}
	}
}

// TestTheFourRefusalTextsAreUnchanged pins what C.17 promised not to move. The
// argument for the change was that the status is the only thing that moves; a
// reworded message would make that claim false silently, and jets/apiserver's
// TestPurgeDataRefusalIsIndistinguishableFromTheOtherEndpoints greps for two of
// these literals one package over.
func TestTheFourRefusalTextsAreUnchanged(t *testing.T) {
	for _, tc := range []struct {
		err  error
		want string
	}{
		{ErrCapabilityNotConfigured, "error: unauthorized, configuration error: missing capability on sql statement"},
		{ErrNoUserInfo, "error: unauthorized, cannot get user info"},
		{ErrAdminOnly, "error: unauthorized, only admin can perform statement"},
		{ErrMissingCapability, "error: unauthorized, user do not have required capability"},
	} {
		if tc.err.Error() != tc.want {
			t.Errorf("a refusal text moved: got %q, want %q", tc.err.Error(), tc.want)
		}
	}
}

// TestNoGateHardCodesTheUnauthorizedStatus is the guard against the next handler
// re-conflating the two, and it is the reason this change is fourteen call sites
// rather than one.
//
// **The defect was not that 401 was the wrong status. It was that each gate chose
// its status independently**, so the classification lived in whoever wrote the
// handler rather than in the package. Fourteen sites each wrote
// `httpStatus = http.StatusUnauthorized` beside a VerifyUserPermission call, and a
// fifteenth would have too. AuthzStatusFor is now the one function allowed to name
// the status.
//
// **It inspects the AST rather than rendering it, and that is not a style choice.**
// ast.Fprint follows the Obj back-links on identifiers, so printing a function body
// that calls RefusalFor also prints RefusalFor's body, AuthzStatusFor's body, and
// the http.StatusUnauthorized inside it. The first version of this test failed on
// five functions that name no status at all. See I-243.
func TestNoGateHardCodesTheUnauthorizedStatus(t *testing.T) {
	fset := token.NewFileSet()
	for _, src := range writePathSources {
		file, err := parser.ParseFile(fset, src, nil, 0)
		if err != nil {
			t.Fatalf("parsing %s: %v", src, err)
		}
		for _, decl := range file.Decls {
			fn, ok := decl.(*ast.FuncDecl)
			if !ok || fn.Body == nil || fn.Name.Name == "AuthzStatusFor" {
				continue
			}
			ast.Inspect(fn.Body, func(n ast.Node) bool {
				sel, ok := n.(*ast.SelectorExpr)
				if !ok || sel.Sel.Name != "StatusUnauthorized" {
					return true
				}
				pkg, ok := sel.X.(*ast.Ident)
				if !ok || pkg.Name != "http" {
					return true
				}
				t.Errorf("%s (%s) names http.StatusUnauthorized directly; a gate's status comes "+
					"from RefusalFor, so that a capability refusal is a 403 and a client can "+
					"refuse in place rather than signing the user out (I-189)", fn.Name.Name, src)
				return true
			})
		}
	}
}

// TestEveryGateGoesThroughRefusalFor is the positive form of the test above. The
// negative one passes vacuously if somebody deletes a gate; this one counts.
//
// **The number is a lower bound written as an equality on purpose.** Fourteen is
// what C.17 converted, and a fifteenth gate is a change worth reading this file
// over -- either it went through RefusalFor, in which case raise the number, or it
// did not, in which case the test above has already failed.
func TestEveryGateGoesThroughRefusalFor(t *testing.T) {
	const want = 14
	got := 0
	fset := token.NewFileSet()
	for _, src := range writePathSources {
		file, err := parser.ParseFile(fset, src, nil, 0)
		if err != nil {
			t.Fatalf("parsing %s: %v", src, err)
		}
		for _, decl := range file.Decls {
			fn, ok := decl.(*ast.FuncDecl)
			if !ok || fn.Body == nil || fn.Name.Name == "RefusalFor" {
				continue
			}
			ast.Inspect(fn.Body, func(n ast.Node) bool {
				call, ok := n.(*ast.CallExpr)
				if !ok {
					return true
				}
				if id, ok := call.Fun.(*ast.Ident); ok && id.Name == "RefusalFor" {
					got++
				}
				return true
			})
		}
	}
	if got != want {
		t.Errorf("%d gates go through RefusalFor; C.17 converted %d. If a gate was added, "+
			"check it classifies its refusal and raise this number; if one was removed, lower it",
			got, want)
	}
}
