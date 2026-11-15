package datatable

import (
	"github.com/artisoft-io/jetstore/jets/datatable/wsfile"
	"github.com/artisoft-io/jetstore/jets/runbook"
	"github.com/artisoft-io/jetstore/jets/sqlscript"
	"github.com/artisoft-io/jetstore/jets/wsvalidate"
)

// The runbook SQL row of the validator table (2026-11-14): a `.sql` file under
// `runbooks/`, where a runbook's steps read their statements (a step's `sql`
// field is a path relative to runbook.Dir, or `<runbook>/<id>.sql` by default,
// runbook.Step.SQLPath).
//
// It refuses a statement no runbook step may run (runbook.ForbiddenStatement:
// transaction control, a change of role or of the step's transaction, COPY,
// LOAD and the rest), with its line and the reason the executor gives. A run
// refuses such a file when it starts (runbook.Validate, from runbook.Load),
// and the runbook list shows the error; until this row the file saved, through
// the `.sql` row, which checks every SQL file lexically only.
//
// It is scoped to the directory and not to the files a runbook names, because
// a save has the file and not the runbooks that read it: a SQL file under
// `runbooks/` is a runbook's, and one no runbook reads yet is one a runbook is
// being written to read. What needs the runbook (the statement count a step
// type allows, and the `:name` bindings) is still checked when a run starts.

// isRunbookSQL reports whether the workspace-relative fileName is under
// runbooks/, in runbook.Dir or a subfolder of it.
func isRunbookSQL(fileName string) bool {
	return wsfile.InDir(fileName, runbook.Dir)
}

// validateRunbookSQL refuses a runbook's SQL file that does not lex
// (sqlscript.ValidateScript, the `.sql` row's check) or that holds a statement
// no runbook step may run.
func validateRunbookSQL(content string) []wsvalidate.Finding {
	if findings := sqlscript.ValidateScript(content); len(wsvalidate.ErrorsOnly(findings)) > 0 {
		return findings
	}
	stmts, err := sqlscript.Split(content)
	if err != nil {
		// ValidateScript accepted it, so Split reads it; this is its error if not.
		return []wsvalidate.Finding{{Severity: wsvalidate.Error, Code: sqlscript.UnterminatedCode, Message: err.Error()}}
	}
	var findings []wsvalidate.Finding
	for _, st := range stmts {
		if st.Body == "" {
			continue
		}
		if why := runbook.ForbiddenStatement(st.Body); why != "" {
			findings = append(findings, wsvalidate.Finding{
				Severity: wsvalidate.Error,
				Code:     runbook.CodeSQLForbidden,
				Message:  "a runbook step may not run this statement: " + why,
				Line:     st.BodyLine,
			})
		}
	}
	return findings
}
