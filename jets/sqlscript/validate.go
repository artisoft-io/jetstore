package sqlscript

import (
	"errors"

	"github.com/artisoft-io/jetstore/jets/wsvalidate"
)

// The code every finding from this package carries.
const UnterminatedCode = "sqlUnterminated"

// ValidateScript is the save-time check for a workspace `.sql` file, dispatched
// by the per-suffix validator table in jets/datatable.
//
// **It reports one thing, and the narrowness is the design.** A construct the
// script opens and never closes — a string literal, a quoted identifier, a
// block comment, a dollar-quoted body — makes the file unloadable, and the
// author finds out at deployment, from an error that names a line in whatever
// the unclosed construct swallowed rather than the line that opened it. That is
// exactly what Split already knows and nothing else here does.
//
// Severity is Error, which blocks the save, and that is licensed by a
// measurement rather than by confidence: **all 89 `.sql` files in this
// repository and the four workspaces lex clean** — 82 under `workspaces/` and 7
// here, 2026-09-12. A lexical check that refuses a file somebody has to edit
// would be worse than no check, so the number is the precondition for the
// severity and is worth re-taking if this scanner grows a rule.
// TestCorpusLexesClean re-takes it.
//
// # What it deliberately does not check
//
// **A report script's `--name;` comments.** `runReportsDelegate` needs one
// above every statement, but six of the 26 `reports/*.sql` legitimately have
// none — three declared `reportOrScript: "script"` in their `config.json` and
// three wired by nothing — and a validator sees a file name and its content,
// never the config that says which kind this is. Flagging 6 of 26 correct files
// is the 38%-false-flag result that agentic_ai's Phase 3 §17.4 measured and
// rejected for the citation checker, arriving at a different door.
//
// **A final statement with no `;`.** It reads like a truncated file and is not:
// `usi_ws/reports/auth_error_report.sql` and `elig_error_report.sql` are both
// written that way and both work, under this reader and the one it replaced.
//
// **Anything needing a parser**, which is the whole class the cast-in-an-INSERT-
// column-list defect belongs to (`getColumns`,
// jets/datatable/wsfile/save_client_config.go). Split is a boundary scanner: it
// knows where a statement ends and nothing about what is in one.
func ValidateScript(content string) []wsvalidate.Finding {
	_, err := Split(content)
	if err == nil {
		return nil
	}
	// The message deliberately omits the line, which travels as Line: rendering
	// both would say it twice, and a caller that wants the sentence has
	// err.Error().
	f := wsvalidate.Finding{Severity: wsvalidate.Error, Code: UnterminatedCode, Message: err.Error()}
	var ue *UnterminatedError
	if errors.As(err, &ue) {
		f.Message, f.Line = "unterminated "+ue.Construct, ue.Line
	}
	return []wsvalidate.Finding{f}
}
