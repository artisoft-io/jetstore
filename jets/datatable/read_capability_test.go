package datatable

import (
	"go/ast"
	"go/parser"
	"go/token"
	"net/http"
	"strings"
	"testing"
)

// Every function reachable from the /dataTable dispatch that reads data, and the
// capability it must require. I-2.
//
// **A source-level test rather than a behavioural one, deliberately.** Exercising
// these through HTTP needs a database, a token and a seeded role, which is why
// nothing covered them for as long as it did — the same gap S.4 hit with its
// wiring. A test that reads the source cannot prove the check *works*, but it can
// prove the check is *present*, and "present" is exactly what was missing here
// for four functions.
var readPathsRequiring = map[string]string{
	"DoReadAction":        "CapabilityReadData",
	"ExecRawQueryMap":     "CapabilityReadData",
	"DoPreviewFileAction": "CapabilityReadData",
	// ExecRawQuery takes its capability as a parameter, because raw_query and
	// raw_query_tool are not equally dangerous; api_tables_test.go checks that.
	"ExecRawQuery": "capability",
	// The runtime document read, in the other file — see readPathFiles.
	"GetWorkspaceDocument": "CapabilityReadData",
}

// The files this test parses.
//
// **It read one file until 2026-08-26 and the list above was written as though
// that were the whole dispatch.** It is not: the workspace actions are a second
// file reached from the same `/dataTable` switch, and a read path added there
// was invisible to this test — which is the shape of gap this test exists to
// close, arriving one file over. GetWorkspaceDocument is the first read path in
// that file to use `requireCapability`; GetWorkspaceFileContent gates by calling
// VerifyUserPermission directly and so is not expressible here yet.
var readPathFiles = []string{"data_table_action.go", "workspace_data_table_action.go"}

func TestEveryReadPathRequiresACapability(t *testing.T) {
	fset := token.NewFileSet()
	var decls []ast.Decl
	for _, name := range readPathFiles {
		file, err := parser.ParseFile(fset, name, nil, 0)
		if err != nil {
			t.Fatalf("parsing %s: %v", name, err)
		}
		decls = append(decls, file.Decls...)
	}

	found := map[string]bool{}
	for _, decl := range decls {
		fn, ok := decl.(*ast.FuncDecl)
		if !ok || fn.Recv == nil || fn.Body == nil {
			continue
		}
		want, watched := readPathsRequiring[fn.Name.Name]
		if !watched {
			continue
		}
		found[fn.Name.Name] = true

		// The check must be the first thing the function does. A read that
		// happens before the gate is a read that happened.
		if len(fn.Body.List) == 0 {
			t.Errorf("%s has an empty body", fn.Name.Name)
			continue
		}
		var buf strings.Builder
		ast.Fprint(&buf, fset, fn.Body.List[0], nil)
		first := buf.String()
		if !strings.Contains(first, "requireCapability") {
			t.Errorf("%s does not gate before doing anything else; its first statement is not a capability check",
				fn.Name.Name)
			continue
		}
		if !strings.Contains(first, want) {
			t.Errorf("%s gates on something other than %s", fn.Name.Name, want)
		}
	}

	for name := range readPathsRequiring {
		if !found[name] {
			t.Errorf("%s is no longer in %v; this list is stale", name, readPathFiles)
		}
	}
}

// TestRequireCapabilityRefusesAnEmptyCapability pins the one behaviour that is
// testable without a database: VerifyUserPermission treats an empty capability
// as a configuration error rather than as "no requirement", and this helper must
// not paper over that.
//
// **The status is 500 as of ui_refresh D.2, and it was 401 until 2026-08-27** --
// I-241. What the empty capability means did not change; what changed is that
// saying it with a 401 signed the user out for a mistake in the server. This is
// also the reachable end of that defect: every production caller of
// requireCapability passes one of the Capability* constants, so the string that
// gets here comes from this test rather than from a request.
func TestRequireCapabilityRefusesAnEmptyCapability(t *testing.T) {
	ctx := &DataTableContext{}
	code, err := ctx.requireCapability("", "any-token")
	if err == nil {
		t.Fatal("an empty capability must not authorise anything")
	}
	if code != http.StatusInternalServerError {
		t.Errorf("expected %d, got %d (I-241: a missing capability on a statement is "+
			"a defect in the server, not a refusal of the caller)",
			http.StatusInternalServerError, code)
	}
}

// TestCapabilityNamesMatchTheSeedFile keeps the constants tied to the capability
// names the database actually grants. A capability nobody holds refuses
// everyone; a capability that does not exist refuses everyone silently.
func TestCapabilityNamesMatchTheSeedFile(t *testing.T) {
	if CapabilityReadData != "jetstore_read" {
		t.Errorf("CapabilityReadData is %q; jets_init_db.sql grants jetstore_read", CapabilityReadData)
	}
	if CapabilityQueryTool != "workspace_ide" {
		t.Errorf("CapabilityQueryTool is %q; jets_init_db.sql grants workspace_ide", CapabilityQueryTool)
	}
}
