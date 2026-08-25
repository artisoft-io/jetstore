package datatable

import (
	"go/ast"
	"go/parser"
	"go/token"
	"strings"
	"testing"
)

// The InsertRawRows pre-processing hook for raw_rows/process_mapping deletes an
// existing mapping before delegating to InsertRows, and InsertRows is where the
// capability check used to live. That ordering meant a caller who failed the
// check got the delete committed and nothing inserted to replace it. I-124.
//
// **A source-level test rather than a behavioural one, for the same reason
// read_capability_test.go is one**: exercising this path needs a database, a
// token and a seeded role. A test that reads the source cannot prove the check
// *works*; it can prove the check is *present and first*, and "first" is the
// whole of this defect.

// TestProcessMappingAuthorizesBeforeItDeletes asserts the ordering directly: in
// the raw_rows/process_mapping arm, the capability check must appear before
// anything touches the database.
func TestProcessMappingAuthorizesBeforeItDeletes(t *testing.T) {
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, "data_table_action.go", nil, 0)
	if err != nil {
		t.Fatalf("parsing: %v", err)
	}

	var arm *ast.CaseClause
	for _, decl := range file.Decls {
		fn, ok := decl.(*ast.FuncDecl)
		if !ok || fn.Name.Name != "InsertRawRows" || fn.Body == nil {
			continue
		}
		ast.Inspect(fn.Body, func(n ast.Node) bool {
			clause, ok := n.(*ast.CaseClause)
			if !ok {
				return true
			}
			for _, expr := range clause.List {
				lit, ok := expr.(*ast.BasicLit)
				if ok && lit.Value == `"raw_rows/process_mapping"` {
					arm = clause
				}
			}
			return true
		})
	}
	if arm == nil {
		t.Fatal("no case \"raw_rows/process_mapping\" arm in InsertRawRows; this test is stale")
	}

	verifyAt, dbAt := -1, -1
	for i, stmt := range arm.Body {
		var buf strings.Builder
		ast.Fprint(&buf, fset, stmt, nil)
		rendered := buf.String()
		if verifyAt < 0 && strings.Contains(rendered, "VerifyUserPermission") {
			verifyAt = i
		}
		if dbAt < 0 && strings.Contains(rendered, "Dbpool") {
			dbAt = i
		}
	}

	if verifyAt < 0 {
		t.Fatal("the raw_rows/process_mapping arm does not check a capability at all; " +
			"the DELETE it performs is reachable by any authenticated caller")
	}
	if dbAt < 0 {
		t.Fatal("the raw_rows/process_mapping arm no longer touches Dbpool; this test is stale")
	}
	if verifyAt > dbAt {
		t.Errorf("the raw_rows/process_mapping arm touches the database at statement %d "+
			"and only checks the capability at statement %d; a caller who fails the check "+
			"gets the delete committed", dbAt, verifyAt)
	}
}

// TestProcessMappingPreCheckUsesTheStatementInsertRowsWillUse pins the tie the
// fix relies on. The pre-check looks up sqlInsertStmts["process_mapping"], and
// the arm then sets FromClauses[0].Table to "process_mapping" so that InsertRows
// looks up the same entry. If that entry loses its capability the pre-check
// still refuses -- VerifyUserPermission treats an empty capability as a
// configuration error -- but it would refuse everyone, so the value is worth
// pinning rather than assuming.
func TestProcessMappingPreCheckUsesTheStatementInsertRowsWillUse(t *testing.T) {
	sqlStmt, ok := sqlInsertStmts["process_mapping"]
	if !ok {
		t.Fatal("sqlInsertStmts has no \"process_mapping\" entry; the pre-check in " +
			"InsertRawRows and the check in InsertRows both resolve through this key")
	}
	if sqlStmt.Capability != "client_config" {
		t.Errorf("process_mapping requires %q; jets/jets_init_db.sql grants client_config",
			sqlStmt.Capability)
	}
}
