package datatable

// The rules unit test tables sort their integer key as an integer. Project
// rules_unit_test_harness, Phase 1, found by the browser pass (AG.1).
//
// buildQuery selects every column cast to text ("key"::text, makeSelectColumns)
// and orders by the bare sort column when no table is named, which PostgreSQL
// resolves to the text output column -- so run 10 sorted before run 8. Naming
// the table makes ORDER BY reach the integer table column. The request is built
// from the shipped rutCheckTable.tc.json as the client builds it. Skipped unless
// JETS_TEST_DSN names a PostgreSQL; see rules_unit_test_runs_test.go.

import (
	"context"
	"fmt"
	"net/http"
	"testing"

	"github.com/artisoft-io/jetstore/jets/user"
)

func TestRulesUnitTestCheckTableSortsItsKeyAsAnInteger(t *testing.T) {
	f := newRutFixture(t)
	bg := context.Background()
	for _, s := range []string{
		`INSERT INTO jetsapi.users (user_email, name, password, is_active, encrypted_roles)
			VALUES ('` + rutKnowledgeEngineerEmail + `', 'KE', 'x', 1, '{knowledge_engineer}')`,
		`INSERT INTO jetsapi.role_capability (role, capability) VALUES ('knowledge_engineer', 'workspace_ide')`,
		`INSERT INTO jetsapi.rules_unit_test_check (key, execution_key, session_name, kind, "check", outcome, found)
			SELECT k, 1, 's', 'expected', 'check ' || k, 'pass', '' FROM generate_series(8, 12) AS k`,
	} {
		if _, err := f.pool.Exec(bg, s); err != nil {
			t.Fatalf("seeding: %v\n%s", err, s)
		}
	}
	token, err := user.CreateToken(rutKnowledgeEngineerEmail)
	if err != nil {
		t.Fatal(err)
	}
	keysOf := func(t *testing.T, action *DataTableAction) []string {
		t.Helper()
		res, status, err := f.ctx.DoWorkspaceReadAction(action, token)
		if err != nil || status != http.StatusOK {
			t.Fatalf("reading: %d %v", status, err)
		}
		keys := []string{}
		for _, row := range *(*res)["rows"].(*[][]any) {
			keys = append(keys, fmt.Sprint(row[0]))
		}
		return keys
	}
	formState := map[string]string{"workspace_name": rutActiveWorkspace, "rutExecutionTable": "1"}

	action, _ := readRequestFor(t, "rutCheckTable", formState)
	if action.SortColumn != "key" || action.SortColumnTable != "rules_unit_test_check" {
		t.Fatalf("rutCheckTable sorts on %q of %q, want key of rules_unit_test_check", action.SortColumn, action.SortColumnTable)
	}
	if got := keysOf(t, action); fmt.Sprint(got) != "[8 9 10 11 12]" {
		t.Errorf("ascending by key: got %v, want [8 9 10 11 12]", got)
	}
	action, _ = readRequestFor(t, "rutCheckTable", formState)
	action.SortAscending = false
	if got := keysOf(t, action); fmt.Sprint(got) != "[12 11 10 9 8]" {
		t.Errorf("descending by key: got %v, want [12 11 10 9 8]", got)
	}

	// The mechanism, so the test above is not passing for another reason: the
	// same request with the sort column unqualified sorts the text output column.
	action, _ = readRequestFor(t, "rutCheckTable", formState)
	action.SortColumnTable = ""
	if got := keysOf(t, action); fmt.Sprint(got) != "[10 11 12 8 9]" {
		t.Errorf("unqualified: got %v, want the text order [10 11 12 8 9]", got)
	}
}
