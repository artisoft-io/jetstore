package main

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"strings"
	"testing"
)

// The half of the write-path sweep that cannot live in jets/datatable. I-125,
// I-126.
//
// jets/datatable/write_capability_test.go asserts that every write *method* on
// the /dataTable dispatch gates before it acts. It cannot see two things:
//
//  1. **arms handled inline in DoDataTableAction**, which are not methods on
//     DataTableContext and so are in no watched map — resubmit_pipeline writes
//     to jetsapi.pipeline_execution_status and starts pipelines, and
//     fetch_file_from_stage reads an object out of the S3 stage;
//  2. **a new arm added to the switch**, which a watched map of function names
//     cannot notice at all. That is the failure mode I-125 named: "a per-function
//     fix leaves the next one uncovered".
//
// So this file is organised around the *switch* rather than around a list of
// functions. Every case literal must be classified, and a case that is in none
// of the three maps fails the test.
//
// **Source-level, for the reason the two tests it joins are**: there is no
// reachable database here, so nothing can prove a 401 is returned. What is
// provable is that a check is written and that nothing acts ahead of it.

// delegatedActions are the arms whose whole body is a call to a method that
// carries its own gate. The value names the method, and the assertion is that
// the arm calls it; whether *that* method gates is
// jets/datatable/write_capability_test.go's and read_capability_test.go's.
var delegatedActions = map[string]string{
	"raw_query":                    "ExecRawQuery",
	"raw_query_tool":               "ExecRawQuery",
	"exec_ddl":                     "ExecDataManagementStatement",
	"raw_query_map":                "ExecRawQueryMap",
	"insert_raw_rows":              "InsertRawRows",
	"insert_rows":                  "InsertRows",
	"workspace_insert_rows":        "WorkspaceInsertRows",
	"workspace_query_structure":    "WorkspaceQueryStructure",
	"add_workspace_file":           "AddWorkspaceFile",
	"delete_workspace_files":       "DeleteWorkspaceFile",
	"get_workspace_file_content":   "GetWorkspaceFileContent",
	"save_workspace_file_content":  "SaveWorkspaceFileContent",
	"delete_workspace_changes":     "DeleteWorkspaceChanges",
	"delete_all_workspace_changes": "DeleteAllWorkspaceChanges",
	"workspace_read":               "DoWorkspaceReadAction",
	"save_workspace_client_config": "SaveWorkspaceClientConfig",
	"read":                         "DoReadAction",
	"preview_file":                 "DoPreviewFileAction",
	"drop_table":                   "DropTable",
}

// inlineGatedActions are the arms that do the work here rather than delegating,
// and so must ask for a capability here. The value is a substring the rendered
// gate has to contain, so it is the bare constant name rather than the qualified
// one: ast.Fprint renders a selector as two nodes and "datatable.X" never appears
// contiguously.
var inlineGatedActions = map[string]string{
	// Reads an arbitrary path under JETS_s3_STAGE_PREFIX out of S3. The same
	// authority preview_file needs, and DoPreviewFileAction is the nearest
	// neighbour that already requires it.
	"fetch_file_from_stage": "CapabilityReadData",
	// Inserts a row into jetsapi.pipeline_execution_status and starts the pending
	// task. insert_rows on the same table resolves sqlInsertStmts, which requires
	// run_pipelines; this arm hand-writes the statement and resolves nothing, so
	// it has to name the capability. The two are tied by
	// TestResubmitPipelineUsesTheSameCapabilityAsItsInsert in jets/datatable.
	"resubmit_pipeline": "CapabilityRunPipelines",
}

// ungatedActions are the arms that ask for nothing, with the reason. **A reason
// here is a claim somebody made, not a proof** — the point of the map is that
// adding an arm to it is a deliberate act with a sentence attached, rather than
// the default that ungated arms used to enjoy.
var ungatedActions = map[string]string{
	"refresh_token": "returns an empty result; the token round trip is authh's and " +
		"addToken's, and nothing is read or written",
	"get_workspace_uri": "returns four environment variables naming the deployed " +
		"workspace, touching no database and reading no request field; both UIs need " +
		"them to render, for every authenticated user",
}

// dataTableActionCount is how many actions the /dataTable dispatch has, measured
// rather than remembered:
//
//	python3 -c 'import re,sys; src=open("jets/apiserver/api_tables.go").read();
//	  start=src.index("switch dataTableAction.Action {");
//	  end=src.index("\n\tdefault:", start);
//	  print(len(re.findall(r"^\tcase \"[a-z_]+\":", src[start:end], re.M)))'
//
// It is pinned here because a document wanted to cite it and cited 21, from
// counting a grep output by eye. A number a test can hold should not be a number
// prose asserts.
const dataTableActionCount = 23

// apiserverEffectMarkers are the tells that an inline arm has reached the
// database or S3. Deliberately short and deliberately a lower bound; see the
// same note in jets/datatable/write_capability_test.go.
var apiserverEffectMarkers = []string{
	"server.dbpool",
	"awsi.",
	"ReserveSessionId",
	"StartPendingTasks",
}

// dispatchArms returns the case clauses of the switch inside DoDataTableAction,
// keyed by the action literal each names.
func dispatchArms(t *testing.T) (map[string]string, *token.FileSet) {
	t.Helper()
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, "api_tables.go", nil, 0)
	if err != nil {
		t.Fatalf("parsing api_tables.go: %v", err)
	}
	arms := map[string]string{}
	for _, decl := range file.Decls {
		fn, ok := decl.(*ast.FuncDecl)
		if !ok || fn.Name.Name != "DoDataTableAction" || fn.Body == nil {
			continue
		}
		ast.Inspect(fn.Body, func(n ast.Node) bool {
			clause, ok := n.(*ast.CaseClause)
			if !ok {
				return true
			}
			var buf strings.Builder
			for _, stmt := range clause.Body {
				ast.Fprint(&buf, fset, stmt, nil)
			}
			for _, expr := range clause.List {
				lit, ok := expr.(*ast.BasicLit)
				if !ok || lit.Kind != token.STRING {
					continue
				}
				arms[strings.Trim(lit.Value, `"`)] = buf.String()
			}
			return true
		})
	}
	if len(arms) == 0 {
		t.Fatal("no case clauses found in DoDataTableAction; this test is stale")
	}
	return arms, fset
}

// TestEveryDataTableActionIsClassified is the assertion I-125 asked for in place
// of a per-function fix. An action added to the dispatch and left out of all
// three maps fails here, whether it writes or not — deciding which map it goes
// in is the work, and this test is what makes somebody do it.
func TestEveryDataTableActionIsClassified(t *testing.T) {
	arms, _ := dispatchArms(t)

	for action := range arms {
		var in []string
		if _, ok := delegatedActions[action]; ok {
			in = append(in, "delegated")
		}
		if _, ok := inlineGatedActions[action]; ok {
			in = append(in, "inline-gated")
		}
		if _, ok := ungatedActions[action]; ok {
			in = append(in, "ungated")
		}
		switch len(in) {
		case 0:
			t.Errorf("the /dataTable dispatch has a %q action and nothing in this test "+
				"classifies it; say whether it delegates to a gated method, gates here, or "+
				"needs no capability and why", action)
		case 1:
		default:
			t.Errorf("%q is classified %v; it can be only one", action, in)
		}
	}

	for _, m := range []map[string]string{delegatedActions, inlineGatedActions, ungatedActions} {
		for action := range m {
			if _, ok := arms[action]; !ok {
				t.Errorf("%q is no longer an action of the /dataTable dispatch; this list is stale", action)
			}
		}
	}

	if len(arms) != dataTableActionCount {
		t.Errorf("the /dataTable dispatch has %d actions and dataTableActionCount says %d; "+
			"correct the constant, and any document citing it", len(arms), dataTableActionCount)
	}
}

// TestDelegatedActionsCallTheMethodTheyClaimTo keeps delegatedActions from
// becoming a list of assertions about code that has moved. It does not check the
// gate — that is the callee's package's job — it checks that the callee named
// here is the callee the arm actually has.
func TestDelegatedActionsCallTheMethodTheyClaimTo(t *testing.T) {
	arms, _ := dispatchArms(t)
	for action, method := range delegatedActions {
		body, ok := arms[action]
		if !ok {
			continue // reported by TestEveryDataTableActionIsClassified
		}
		if !strings.Contains(body, method) {
			t.Errorf("the %q arm no longer calls %s; whatever it calls now may not be gated",
				action, method)
		}
	}
}

// TestInlineArmsGateBeforeTheyAct is the finding this file was written for.
// resubmit_pipeline and fetch_file_from_stage are handled here rather than by a
// method, so no watched map of function names could see them, and neither asked
// for anything.
func TestInlineArmsGateBeforeTheyAct(t *testing.T) {
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, "api_tables.go", nil, 0)
	if err != nil {
		t.Fatalf("parsing api_tables.go: %v", err)
	}
	seen := map[string]bool{}
	for _, decl := range file.Decls {
		fn, ok := decl.(*ast.FuncDecl)
		if !ok || fn.Name.Name != "DoDataTableAction" || fn.Body == nil {
			continue
		}
		ast.Inspect(fn.Body, func(n ast.Node) bool {
			clause, ok := n.(*ast.CaseClause)
			if !ok {
				return true
			}
			for _, expr := range clause.List {
				lit, ok := expr.(*ast.BasicLit)
				if !ok || lit.Kind != token.STRING {
					continue
				}
				action := strings.Trim(lit.Value, `"`)
				want, watched := inlineGatedActions[action]
				if !watched {
					continue
				}
				seen[action] = true
				gateAt, effectAt, gateStmt := -1, -1, ""
				for i, stmt := range clause.Body {
					var buf strings.Builder
					ast.Fprint(&buf, fset, stmt, nil)
					rendered := buf.String()
					if gateAt < 0 && strings.Contains(rendered, "RequireCapability") {
						gateAt, gateStmt = i, rendered
					}
					if effectAt < 0 {
						for _, marker := range apiserverEffectMarkers {
							if strings.Contains(rendered, marker) {
								effectAt = i
								break
							}
						}
					}
				}
				if gateAt < 0 {
					t.Errorf("the %q arm checks no capability at all; every authenticated "+
						"caller of any role can reach what it does", action)
					continue
				}
				if effectAt >= 0 && effectAt < gateAt {
					t.Errorf("the %q arm reaches the database or S3 at statement %d and only "+
						"checks a capability at statement %d", action, effectAt, gateAt)
				}
				if !strings.Contains(gateStmt, want) {
					t.Errorf("the %q arm gates on something other than %s", action, want)
				}
			}
			return true
		})
	}
	for action := range inlineGatedActions {
		if !seen[action] {
			t.Errorf("no %q arm in DoDataTableAction; this list is stale", action)
		}
	}
}

// TestPurgeDataAuthorizesBeforeItDispatches is I-126.
//
// **The gate belongs at the entry point and that is not a convenience.**
// ResetDomainTables has two callers inside this package that are not HTTP
// requests at all — checkJetStoreSchema and checkDomainTablesVersion, both run at
// server start, before any user exists and with no token to check. A gate pushed
// down into the action would have to be bypassed by both, and a gate that its own
// package routinely bypasses is the next finding rather than a fix. Gating in
// DoPurgeDataAction leaves the startup path untouched and covers every arm the
// switch grows.
func TestPurgeDataAuthorizesBeforeItDispatches(t *testing.T) {
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, "api_purgedata.go", nil, 0)
	if err != nil {
		t.Fatalf("parsing api_purgedata.go: %v", err)
	}
	var fn *ast.FuncDecl
	for _, decl := range file.Decls {
		d, ok := decl.(*ast.FuncDecl)
		if ok && d.Name.Name == "DoPurgeDataAction" && d.Body != nil {
			fn = d
		}
	}
	if fn == nil {
		t.Fatal("no DoPurgeDataAction in api_purgedata.go; this test is stale")
	}

	gateAt, dispatchAt := -1, -1
	for i, stmt := range fn.Body.List {
		var buf strings.Builder
		ast.Fprint(&buf, fset, stmt, nil)
		rendered := buf.String()
		if gateAt < 0 && strings.Contains(rendered, "IsAdmin") {
			gateAt = i
		}
		if dispatchAt < 0 && (strings.Contains(rendered, "ResetDomainTables") ||
			strings.Contains(rendered, "RunWorkspaceBaseDbInit")) {
			dispatchAt = i
		}
	}
	if gateAt < 0 {
		t.Fatal("DoPurgeDataAction does not require the admin account; /purgeData drops every table named " +
			"in input_loader_status and truncates two registries for any authenticated caller")
	}
	if dispatchAt < 0 {
		t.Fatal("DoPurgeDataAction dispatches to neither action; this test is stale")
	}
	if gateAt > dispatchAt {
		t.Errorf("DoPurgeDataAction dispatches at statement %d and checks the capability at %d",
			dispatchAt, gateAt)
	}
}

// TestPurgeDataCapabilityIsSeeded follows TestAgentSupervisionCapabilityIsSeeded.
// A capability no role holds refuses everyone, which on this endpoint would look
// like a broken menu item rather than like a policy.
func TestPurgeDataCapabilityIsSeeded(t *testing.T) {
	sql, err := os.ReadFile("../jets_init_db.sql")
	if err != nil {
		t.Fatalf("reading jets_init_db.sql: %v", err)
	}
	if !strings.Contains(string(sql), fmt.Sprintf("'%s')", PurgeDataCapability)) {
		t.Errorf("jets_init_db.sql grants no role the %q capability", PurgeDataCapability)
	}
}

// TestPurgeDataRefusalIsIndistinguishableFromTheOtherEndpoints keeps the four
// capability gates in this package returning the same two messages, so that a
// caller cannot tell which endpoint refused it or why beyond the two states the
// others already reveal.
func TestPurgeDataRefusalIsIndistinguishableFromTheOtherEndpoints(t *testing.T) {
	for _, name := range []string{"api_purgedata.go", "api_filekey.go", "api_infer_server.go", "api_agentic.go"} {
		src, err := os.ReadFile(name)
		if err != nil {
			t.Fatalf("reading %s: %v", name, err)
		}
		text := string(src)
		for _, want := range []string{
			`errors.New("error: unauthorized, cannot get user info")`,
			`errors.New("error: unauthorized, user do not have required capability")`,
		} {
			if !strings.Contains(text, want) {
				t.Errorf("%s does not return %s; the three gates in this package are supposed "+
					"to be byte-identical so they are not an oracle", name, want)
			}
		}
	}
}

// TestRegisterFileKeyAuthorizesBeforeItDispatches is I-138.
//
// **The three handlers behind this endpoint each take a token and, until
// 2026-08-25, none of them read it.** The shape is I-126's rather than I-125's:
// the endpoint sat behind authh and used its token for the audit line alone.
//
// **The gate is asserted at the entry point on purpose.** Every one of the three
// actions is also reached in process, by callers that are not requests — the
// RegisterFileKeyV2 lambda calls RegisterFileKeys directly, RegisterSchemaEvent
// reaches it from three sites in jets/datatable/pipeline_execution.go, and
// SyncFileKeys delegates into it carrying whatever token it was given. So the
// function is mixed-population by construction and the switch is the only place
// where "this arrived over HTTP" is still known.
func TestRegisterFileKeyAuthorizesBeforeItDispatches(t *testing.T) {
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, "api_filekey.go", nil, 0)
	if err != nil {
		t.Fatalf("parsing api_filekey.go: %v", err)
	}
	var fn *ast.FuncDecl
	for _, decl := range file.Decls {
		d, ok := decl.(*ast.FuncDecl)
		if ok && d.Name.Name == "DoRegisterFileKeyAction" && d.Body != nil {
			fn = d
		}
	}
	if fn == nil {
		t.Fatal("no DoRegisterFileKeyAction in api_filekey.go; this test is stale")
	}

	gateAt, dispatchAt := -1, -1
	for i, stmt := range fn.Body.List {
		var buf strings.Builder
		ast.Fprint(&buf, fset, stmt, nil)
		rendered := buf.String()
		if gateAt < 0 && strings.Contains(rendered, "HasCapability") {
			gateAt = i
		}
		if dispatchAt < 0 && (strings.Contains(rendered, "RegisterFileKeys") ||
			strings.Contains(rendered, "SyncFileKeys") ||
			strings.Contains(rendered, "PutSchemaEventToS3")) {
			dispatchAt = i
		}
	}
	if gateAt < 0 {
		t.Fatal("DoRegisterFileKeyAction checks no capability; /registerFileKey registers a file key " +
			"and can start an automated load for any authenticated caller")
	}
	if dispatchAt >= 0 && gateAt > dispatchAt {
		t.Errorf("DoRegisterFileKeyAction dispatches at statement %d and gates at %d; "+
			"the gate must precede the work", dispatchAt, gateAt)
	}
}

// TestRegisterFileKeyCapabilityIsSeeded follows TestPurgeDataCapabilityIsSeeded.
//
// **It also pins the half of I-138's decision that is easy to lose.**
// run_pipelines was chosen because system_role holds it, so the lambda would pass
// this gate if it ever stopped calling RegisterFileKeys in process. If a later
// edit narrows the capability to one system_role does not hold, that property
// goes silently; this asserts it.
func TestRegisterFileKeyCapabilityIsSeeded(t *testing.T) {
	sql, err := os.ReadFile("../jets_init_db.sql")
	if err != nil {
		t.Fatalf("reading jets_init_db.sql: %v", err)
	}
	text := string(sql)
	if !strings.Contains(text, fmt.Sprintf("'%s')", RegisterFileKeyCapability)) {
		t.Errorf("jets_init_db.sql grants no role the %q capability", RegisterFileKeyCapability)
	}
	if !strings.Contains(text, fmt.Sprintf("('system_role', '%s')", RegisterFileKeyCapability)) {
		t.Errorf("system_role no longer holds %q; the lambda would fail this gate if it "+
			"ever came through HTTP, which is why this capability was chosen (ui_refresh I-138)",
			RegisterFileKeyCapability)
	}
}

// TestPurgeDataGatesOnAdminRatherThanCapability is I-139, settled 2026-08-25.
//
// **It pins a negative, which is the only way this decision stays made.**
// HasCapability returns true for the admin account unconditionally, so a gate on
// PurgeDataCapability would pass everyone this gate passes and more -- and would
// look, in a diff, like a tightening rather than the loosening it is. Asserting
// that the handler does not reach for HasCapability is what makes that
// substitution fail out loud.
//
// PurgeDataCapability itself is deliberately still declared and still asserted
// seeded by TestPurgeDataCapabilityIsSeeded: it records which authority this
// endpoint belongs to, which is the thing to restore if the project ever widens
// the gate back from a single account.
func TestPurgeDataGatesOnAdminRatherThanCapability(t *testing.T) {
	src, err := os.ReadFile("api_purgedata.go")
	if err != nil {
		t.Fatalf("reading api_purgedata.go: %v", err)
	}
	text := string(src)
	if !strings.Contains(text, "!jetsUser.IsAdmin()") {
		t.Error("DoPurgeDataAction no longer gates on IsAdmin; ui_refresh I-139 settled that " +
			"/purgeData requires the admin account, not a capability")
	}
	if strings.Contains(text, "HasCapability(PurgeDataCapability)") {
		t.Error("DoPurgeDataAction gates on PurgeDataCapability again; HasCapability is true for " +
			"admin unconditionally, so this widens the gate rather than narrowing it (I-139)")
	}
}
