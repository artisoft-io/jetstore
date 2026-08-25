package datatable

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"strings"
	"testing"
)

// The write-side counterpart of read_capability_test.go. I-125, I-126.
//
// **A source-level test rather than a behavioural one, for the same reason
// read_capability_test.go is one**: exercising these paths needs a database, a
// token and a seeded role, so nothing in this environment can prove the 401
// happens. A test that reads the source cannot prove a check *works*; it can
// prove a check is *present* and that nothing touches the database, the
// workspace or S3 ahead of it — and "present" is exactly what was missing from
// DropTable, which accepted a token and never read it.
//
// **Why the write side had nothing and the read side did.** read_capability_test.go
// was written for I-2, which was about four read functions. The write paths were
// left to the per-statement Capability field in sql_stmts.go, which covers the
// two handlers that resolve a statement and says nothing at all about a handler
// that resolves none. DropTable resolves none. Neither did the /purgeData
// endpoint one package over, which write_dispatch_test.go in jets/apiserver
// covers because it is not a method on this type.

// writePathsRequiring lists every function on the /dataTable dispatch that
// writes, deletes or drops, and the capability it must require. The value is a
// substring the rendered gating statement has to contain -- a bare capability
// name or the constant that carries it, never a quoted Go literal, because
// ast.Fprint escapes the quotes. An empty value means the capability is resolved
// from sqlInsertStmts at run time and what to look for is the statement instead.
//
// **InsertRawRows is deliberately absent.** Its gate lives inside the
// raw_rows/process_mapping arm rather than at the top of the function, because
// the arm is what performs the delete (I-124), and insert_raw_rows_authz_test.go
// asserts that ordering precisely. Duplicating it here with a coarser rule would
// weaken it. TestInsertRawRowsIsCoveredByItsOwnTest keeps the delegation from
// rotting silently.
var writePathsRequiring = map[string]string{
	// data_table_action.go
	"ExecDataManagementStatement": "workspace_ide",
	"InsertRows":                  "",
	"DropTable":                   "CapabilityRunPipelines",
	// workspace_data_table_action.go
	"WorkspaceInsertRows":       "",
	"AddWorkspaceFile":          "workspace_ide",
	"DeleteWorkspaceFile":       "workspace_ide",
	"SaveWorkspaceFileContent":  "workspace_ide",
	"SaveWorkspaceClientConfig": "workspace_ide",
	"DeleteWorkspaceChanges":    "workspace_ide",
	"DeleteAllWorkspaceChanges": "workspace_ide",
}

// writePathSources are the files writePathsRequiring's functions live in. Both
// are parsed, because the dispatch splits its write handlers across them for
// reasons of file size rather than of policy.
var writePathSources = []string{"data_table_action.go", "workspace_data_table_action.go"}

// gateMarkers are the two ways a function in this package asks for a capability.
var gateMarkers = []string{"VerifyUserPermission", "requireCapability"}

// effectMarkers are the textual tells that a statement reaches the database, the
// workspace file system or S3. The list is a lower bound and is meant to be:
// a handler can act through a helper that names none of these, and for the
// handlers watched here every such helper is called after a gate that is the
// function's first statement, so the weaker rule still binds. Widen the list
// when it stops binding, rather than trusting it to be complete.
var effectMarkers = []string{
	"Dbpool",
	"wsfile.",
	"awsi.",
	"execDDL",
	"RunUpdateDb",
	"addWorkspaceFile",
	"InsertPipelineExecutionStatus",
}

func containsAny(s string, needles []string) bool {
	for _, n := range needles {
		if strings.Contains(s, n) {
			return true
		}
	}
	return false
}

// TestEveryWritePathRequiresACapability is the guard I-125 asked for: not a fix
// to one function, but the assertion that would have failed on it.
//
// The rule is *gate before you act*, not *gate first*. InsertRows and
// WorkspaceInsertRows have to resolve their sql statement before they can know
// which capability to demand, so demanding statement zero would be demanding the
// wrong thing. What must hold for all of them is that nothing touching the
// database, the workspace or S3 precedes the gate.
func TestEveryWritePathRequiresACapability(t *testing.T) {
	fset := token.NewFileSet()
	found := map[string]bool{}

	for _, src := range writePathSources {
		file, err := parser.ParseFile(fset, src, nil, 0)
		if err != nil {
			t.Fatalf("parsing %s: %v", src, err)
		}
		for _, decl := range file.Decls {
			fn, ok := decl.(*ast.FuncDecl)
			if !ok || fn.Recv == nil || fn.Body == nil {
				continue
			}
			want, watched := writePathsRequiring[fn.Name.Name]
			if !watched {
				continue
			}
			found[fn.Name.Name] = true

			gateAt, effectAt, gateStmt := -1, -1, ""
			for i, stmt := range fn.Body.List {
				var buf strings.Builder
				ast.Fprint(&buf, fset, stmt, nil)
				rendered := buf.String()
				if gateAt < 0 && containsAny(rendered, gateMarkers) {
					gateAt, gateStmt = i, rendered
				}
				if effectAt < 0 && containsAny(rendered, effectMarkers) {
					effectAt = i
				}
			}

			if gateAt < 0 {
				t.Errorf("%s (%s) checks no capability at all; every authenticated caller "+
					"of any role can reach what it does", fn.Name.Name, src)
				continue
			}
			if effectAt >= 0 && effectAt < gateAt {
				t.Errorf("%s (%s) reaches the database, the workspace or S3 at statement %d "+
					"and only checks a capability at statement %d", fn.Name.Name, src, effectAt, gateAt)
			}
			switch want {
			case "":
				// Capability comes from sqlInsertStmts; the gate must be reading a
				// resolved statement rather than naming a capability of its own.
				if !strings.Contains(gateStmt, "sqlStmt") {
					t.Errorf("%s (%s) gates on a literal capability; its capability is supposed to "+
						"come from the sqlInsertStmts entry for the target table", fn.Name.Name, src)
				}
			default:
				if !strings.Contains(gateStmt, want) {
					t.Errorf("%s (%s) gates on something other than %s", fn.Name.Name, src, want)
				}
			}
		}
	}

	for name := range writePathsRequiring {
		if !found[name] {
			t.Errorf("%s is no longer in %v; this list is stale", name, writePathSources)
		}
	}
}

// TestInsertRawRowsIsCoveredByItsOwnTest keeps the one deliberate omission from
// writePathsRequiring honest. If insert_raw_rows_authz_test.go is ever deleted or
// renamed, the omission stops being a delegation and becomes a hole.
func TestInsertRawRowsIsCoveredByItsOwnTest(t *testing.T) {
	if _, watched := writePathsRequiring["InsertRawRows"]; watched {
		t.Fatal("InsertRawRows is in writePathsRequiring now; delete this test, " +
			"or the comment above the map that says it is not")
	}
	src, err := os.ReadFile("insert_raw_rows_authz_test.go")
	if err != nil {
		t.Fatalf("InsertRawRows is omitted from writePathsRequiring on the grounds that "+
			"insert_raw_rows_authz_test.go covers it, and that file cannot be read: %v", err)
	}
	if !strings.Contains(string(src), "InsertRawRows") {
		t.Error("insert_raw_rows_authz_test.go no longer mentions InsertRawRows; " +
			"the write path it was covering is now covered by nothing")
	}
}

// TestDropTableSanitisesItsIdentifiers pins the second half of I-125. The schema
// and table are request fields either way, so sanitising does not decide *which*
// table may be dropped — see the TODO the function still carries and I-138. What
// it does is stop a quoted identifier from being escaped out of, which is the
// habit ResetDomainTables already has one package over.
func TestDropTableSanitisesItsIdentifiers(t *testing.T) {
	src, err := os.ReadFile("data_table_action.go")
	if err != nil {
		t.Fatalf("reading data_table_action.go: %v", err)
	}
	text := string(src)
	if strings.Contains(text, `DROP TABLE "%s"."%s"`) || strings.Contains(text, `DROP TABLE public."%s"`) {
		t.Error("DropTable interpolates its identifiers into the statement again; " +
			"use pgx.Identifier{schema, table}.Sanitize() as ResetDomainTables does")
	}
	if !strings.Contains(text, "pgx.Identifier{") {
		t.Error("data_table_action.go names no pgx.Identifier; DropTable is supposed to sanitise")
	}
}

// TestWriteCapabilityNamesMatchTheSeedFile is the write-side companion of
// TestCapabilityNamesMatchTheSeedFile. A capability that does not exist refuses
// everyone silently, which on a write path reads as a broken screen rather than
// as a policy.
func TestWriteCapabilityNamesMatchTheSeedFile(t *testing.T) {
	if CapabilityRunPipelines != "run_pipelines" {
		t.Errorf("CapabilityRunPipelines is %q; jets_init_db.sql grants run_pipelines", CapabilityRunPipelines)
	}
	sql, err := os.ReadFile("../jets_init_db.sql")
	if err != nil {
		t.Fatalf("reading jets_init_db.sql: %v", err)
	}
	if !strings.Contains(string(sql), "'"+CapabilityRunPipelines+"')") {
		t.Errorf("jets_init_db.sql grants no role the %q capability", CapabilityRunPipelines)
	}
}

// TestResubmitPipelineUsesTheSameCapabilityAsItsInsert pins the tie the
// apiserver's inline resubmit_pipeline arm depends on, and it lives here because
// sqlInsertStmts is unexported.
//
// The arm hand-writes an INSERT into jetsapi.pipeline_execution_status rather
// than going through InsertRows, so it resolves no statement and inherits no
// capability. It names datatable.CapabilityRunPipelines instead. If the entry
// for that table ever required something else, the two routes to the same table
// would disagree — which is the shape the whole finding is about.
func TestResubmitPipelineUsesTheSameCapabilityAsItsInsert(t *testing.T) {
	sqlStmt, ok := sqlInsertStmts["pipeline_execution_status"]
	if !ok {
		t.Fatal("sqlInsertStmts has no \"pipeline_execution_status\" entry; the inline " +
			"resubmit_pipeline arm in jets/apiserver is gated on the capability this entry names")
	}
	if sqlStmt.Capability != CapabilityRunPipelines {
		t.Errorf("insert_rows on pipeline_execution_status requires %q and resubmit_pipeline "+
			"requires %q; the two routes to the same table must agree",
			sqlStmt.Capability, CapabilityRunPipelines)
	}
}
