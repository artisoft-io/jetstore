package datatable

import (
	"strings"
	"testing"
)

// A runbook's SQL file is refused at its save when it holds a statement no
// runbook step may run (runbook.ForbiddenStatement), with the statement's line;
// the run refused it when it started. The same text is saved anywhere but
// under runbooks/, where nothing runs it as a runbook step (2026-11-14).
func TestSaveCheckRefusesAStatementNoRunbookStepMayRun(t *testing.T) {
	for _, c := range []struct {
		content string
		want    []string // empty: saved
	}{
		{"SELECT count(*) FROM jetsapi.input_registry WHERE session_id = :session_id", nil},
		{"-- a note\nSET search_path TO jetsapi;\nUPDATE t SET a = 1;\n", nil},
		{"BEGIN;\nUPDATE t SET a = 1;\nCOMMIT;\n",
			[]string{"line 1: a runbook step may not run this statement: BEGIN is transaction control",
				"line 3: a runbook step may not run this statement: COMMIT"}},
		{"-- switch role\nSET ROLE admin;\n", []string{"line 2", "SET ROLE changes the database identity"}},
		{"COPY t TO '/tmp/out.csv'", []string{"line 1", "COPY reads and writes server files"}},
		{"UPDATE t SET a = 1;\n\n(LOAD 'lib')", []string{"line 3", "LOAD loads a shared library"}},
		// A word in a string or a comment is not a statement.
		{"SELECT 'COMMIT' AS word; -- ROLLBACK\n", nil},
		// Lexed first, as every SQL file is.
		{"SELECT 'open", []string{"line 1"}},
	} {
		for _, name := range []string{"runbooks/health_check/check.sql", "runbooks/check.sql",
			"./runbooks/health_check/check.sql", "runbooks/health_check/CHECK.SQL"} {
			_, err := checkWorkspaceFile(name, c.content)
			switch {
			case len(c.want) == 0 && err != nil:
				t.Errorf("%s holding %q: %v, want saved", name, c.content, err)
			case len(c.want) > 0 && err == nil:
				t.Errorf("%s holding %q was saved, want refused", name, c.content)
			case err != nil:
				for _, want := range c.want {
					if !strings.Contains(err.Error(), want) {
						t.Errorf("%s: refusal %q does not contain %q", name, err, want)
					}
				}
			}
		}
	}
	// Outside runbooks/, a script may open its own transaction.
	for _, name := range []string{"process_config/workspace_init_db.sql", "runbooks/../process_config/x.sql",
		"runbookss/x.sql", "reports/x.sql"} {
		if _, err := checkWorkspaceFile(name, "BEGIN;\nUPDATE t SET a = 1;\nCOMMIT;\n"); err != nil {
			t.Errorf("%s: %v, want saved", name, err)
		}
	}
}
