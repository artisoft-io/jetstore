package datatable

// Who may start rules unit tests and read their results. Project
// rules_unit_test_harness, Phase 1, task AD.8; criterion 13's flow half.
//
// The flow's route has no client guard (definition F49), so it is protected
// exactly as well as the server protects what it reads and writes. A user who
// holds every capability but workspace_ide -- client_config, run_pipelines and
// jetstore_read -- is refused on the rules_unit_test insert target and on the
// three reads the flow makes, each built from the table document the flow
// ships, so a table whose apiAction drifted back to read would fail here. A
// knowledge engineer reading the same three gets the rows, which is what keeps
// the refusals from being vacuous.
//
// Uses track AC's fixture (rules_unit_test_runs_test.go) and is skipped unless
// JETS_TEST_DSN names a PostgreSQL; see that file's header.

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"testing"

	"github.com/artisoft-io/jetstore/jets/user"
)

const rutKnowledgeEngineerEmail = "ke@rut.test"

// tableDocument is the part of a .tc.json a read request is built from.
type tableDocument struct {
	ApiAction string `json:"apiAction"`
	Columns   []struct {
		Name  string `json:"name"`
		Table string `json:"table"`
	} `json:"columns"`
	SortColumn    string       `json:"sortColumn"`
	SortAscending bool         `json:"sortAscending"`
	From          []FromClause `json:"from"`
	Where         []struct {
		Column       string `json:"column"`
		FormStateKey string `json:"formStateKey"`
	} `json:"where"`
}

// readRequestFor builds the request the client sends for a flow table, the way
// datatable/query.ts does: the document's columns, its from clauses, and each
// where clause's formStateKey resolved from formState.
func readRequestFor(t *testing.T, tableKey string, formState map[string]string) (*DataTableAction, string) {
	t.Helper()
	raw, err := os.ReadFile("../workspace_assets/table_configs/" + tableKey + ".tc.json")
	if err != nil {
		t.Fatal(err)
	}
	var doc tableDocument
	if err = json.Unmarshal(raw, &doc); err != nil {
		t.Fatalf("%s: %v", tableKey, err)
	}
	action := &DataTableAction{
		Action:        doc.ApiAction,
		WorkspaceName: formState["workspace_name"],
		FromClauses:   append([]FromClause(nil), doc.From...),
		SortColumn:    doc.SortColumn,
		SortAscending: doc.SortAscending,
		Limit:         20,
	}
	for _, c := range doc.Columns {
		action.Columns = append(action.Columns, Column{Table: c.Table, Column: c.Name})
		// The client sends the sort column's own table, not the document's
		// sortColumnTable (resolveSortColumn, jetsclient_ide/src/datatable/model.ts).
		if c.Name == doc.SortColumn {
			action.SortColumnTable = c.Table
		}
	}
	for _, w := range doc.Where {
		value, ok := formState[w.FormStateKey]
		if !ok {
			t.Fatalf("%s filters on %s, which the test's form state does not hold", tableKey, w.FormStateKey)
		}
		action.WhereClauses = append(action.WhereClauses, WhereClause{Column: w.Column, Values: []string{value}})
	}
	return action, doc.ApiAction
}

// readFlowTable dispatches the request as jets/apiserver's /dataTable switch does.
func (f *rutFixture) readFlowTable(t *testing.T, tableKey, token string, formState map[string]string) (*map[string]any, int, error) {
	t.Helper()
	action, apiAction := readRequestFor(t, tableKey, formState)
	switch apiAction {
	case "workspace_read":
		return f.ctx.DoWorkspaceReadAction(action, token)
	case "", "read":
		action.Action = "read"
		return f.ctx.DoReadAction(action, token)
	default:
		t.Fatalf("%s names apiAction %q", tableKey, apiAction)
		return nil, 0, nil
	}
}

func TestRulesUnitTestFlowRefusesAUserWithoutWorkspaceIde(t *testing.T) {
	f := newRutFixture(t)
	bg := context.Background()
	seed := []string{
		// The runner of AC's fixture holds client_config and run_pipelines; give it
		// jetstore_read too, so it can read everything a read-only user can.
		`INSERT INTO jetsapi.role_capability (role, capability) VALUES ('runner', 'jetstore_read')`,
		`INSERT INTO jetsapi.users (user_email, name, password, is_active, encrypted_roles)
			VALUES ('` + rutKnowledgeEngineerEmail + `', 'KE', 'x', 1, '{knowledge_engineer}')`,
		`INSERT INTO jetsapi.role_capability (role, capability)
			VALUES ('knowledge_engineer', 'jetstore_read'), ('knowledge_engineer', 'workspace_ide')`,
	}
	for _, s := range seed {
		if _, err := f.pool.Exec(bg, s); err != nil {
			t.Fatalf("seeding: %v\n%s", err, s)
		}
	}
	var runKey int
	if err := f.pool.QueryRow(bg, `INSERT INTO jetsapi.rules_unit_test_execution
		(workspace_name, test_path, test_name, status, user_email) VALUES ($1, $2, 'test a', 'pass', 'x')
		RETURNING key`, rutActiveWorkspace, rutTestA).Scan(&runKey); err != nil {
		t.Fatal(err)
	}
	if _, err := f.pool.Exec(bg, `INSERT INTO jetsapi.rules_unit_test_check
		(execution_key, session_name, kind, "check", outcome, found) VALUES ($1, 's1', 'expected', 'a check', 'pass', '')`,
		runKey); err != nil {
		t.Fatal(err)
	}
	runnerToken, err := user.CreateToken(rutPlainEmail)
	if err != nil {
		t.Fatal(err)
	}
	keToken, err := user.CreateToken(rutKnowledgeEngineerEmail)
	if err != nil {
		t.Fatal(err)
	}
	formState := map[string]string{
		"workspace_name":    rutActiveWorkspace,
		"rutExecutionTable": fmt.Sprint(runKey),
	}

	t.Run("rules_unit_test", func(t *testing.T) {
		_, status, err := f.startTests(t, runnerToken, rutActiveWorkspace, rutTestA)
		if status != http.StatusForbidden || !errors.Is(err, ErrRefused) {
			t.Fatalf("got %d %v, want %d and ErrRefused", status, err, http.StatusForbidden)
		}
		if n := f.countRuns(t, RulesUnitTestQueued); n != 0 {
			t.Errorf("%d rows queued for a user without workspace_ide", n)
		}
	})

	for _, table := range []string{"rutExecutionTable", "rutTestCaseTable", "rutCheckTable"} {
		t.Run(table, func(t *testing.T) {
			_, status, err := f.readFlowTable(t, table, runnerToken, formState)
			if status != http.StatusForbidden || !errors.Is(err, ErrRefused) {
				t.Fatalf("a user without workspace_ide read %s: got %d %v, want %d and ErrRefused",
					table, status, err, http.StatusForbidden)
			}
			res, status, err := f.readFlowTable(t, table, keToken, formState)
			if err != nil || status != http.StatusOK {
				t.Fatalf("a knowledge engineer could not read %s: %d %v", table, status, err)
			}
			if n := (*res)["totalRowCount"].(int); n == 0 {
				t.Errorf("a knowledge engineer read no rows from %s", table)
			}
		})
	}

	// Why the tables say workspace_read: the default read path asks for
	// jetstore_read alone, which this user holds, and would have served it the
	// runs of every workspace.
	t.Run("the read path the tables do not take", func(t *testing.T) {
		action, _ := readRequestFor(t, "rutExecutionTable", formState)
		action.Action = "read"
		if _, status, err := f.ctx.DoReadAction(action, runnerToken); err != nil || status != http.StatusOK {
			t.Fatalf("got %d %v; the premise that read admits this user no longer holds", status, err)
		}
	})
}
