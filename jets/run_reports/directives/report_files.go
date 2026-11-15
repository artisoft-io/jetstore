package directives

import (
	"fmt"
	"io/fs"
	"path"
	"strings"

	"github.com/artisoft-io/jetstore/jets/sqlscript"
)

// A reports file under a workspace's `reports/` directory is a sequence of
// pairs: a `--` comment naming the output file, terminated by a semicolon, then
// the SQL statement that produces it.
//
//	--process={PROCESSNAME}/{ORIGINALFILENAME}_Opportunity.csv;
//	SELECT ... FROM "ex:Opportunity" WHERE session_id='$SESSIONID';
//
// **Which of the two shapes a file has is declared, not inferred.**
// `reportOrScript: "script"` in `reports/config.json` says the file is a single
// `Exec` of the whole text, and **that is why those files may open their own
// transaction**: a lookup-update script may open with BEGIN and rely on
// `ON COMMIT DROP` temp tables. Anything else, including a file no
// `config.json` entry mentions, is a reports file and is read here. So a
// `reports/*.sql` with no name comments is not thereby wrong: it is either
// declared a script or is not being run, and nothing in a file's own text
// distinguishes those from an authoring slip.
//
// This used to be read by taking the text up to the next `;` byte as the name
// and the text up to the one after that as the statement. A `;` is an ordinary
// character inside a comment, a string literal, a quoted identifier or a
// dollar-quoted body, so any of those split a statement in two and the halves
// were submitted as two reports. What made this bite rather than merely lurk is
// that the *name* is a comment, so the format invites the author to write prose
// one line above the SQL and punishes the punctuation.
//
// The name is recovered from the comment block that precedes the statement
// rather than from a byte offset, which is what lets sqlscript.Split ignore
// semicolons everywhere they are not a terminator. The block may hold ordinary
// comments as well; the name is the last comment in it that ends with a
// semicolon, so a note written directly above the SQL does not become the
// output file name. The trailing semicolon is kept as the marker that says
// "this comment is a name"; without it any comment above a statement would be
// read as one.
//
// The parsing and the output-format rule live here, beside the directives,
// since 2026-11-14, so that run_reports (jets/run_reports/delegate) and the
// workspace IDE's save check of `reports/config.json` (jets/datatable) refuse
// the same files with the same code.

// ReportDefinition is one report of a reports file.
type ReportDefinition struct {
	Name string // output file name, before substitution of the {TOKEN} patterns
	Stmt string // the SQL statement, without its terminating semicolon
	Line int    // 1-based line of the name comment, for error messages
}

// ParseReportDefinitions reads a reports file. An error is a file run_reports
// refuses before running any of it: one sqlscript.Split cannot read, a name
// with no statement under it, or a statement with no name above it.
func ParseReportDefinitions(script string) ([]ReportDefinition, error) {
	stmts, err := sqlscript.Split(script)
	if err != nil {
		return nil, err
	}
	var out []ReportDefinition
	for _, s := range stmts {
		name, nameLine := reportNameOf(s)
		if s.Body == "" {
			// A chunk of nothing but comments: the end of the file, normally.
			// A name with no statement under it is an authoring slip, and used
			// to be reported as "stmt is empty".
			if name != "" {
				return nil, fmt.Errorf("line %d: report %q has no statement", nameLine, name)
			}
			continue
		}
		if name == "" {
			return nil, fmt.Errorf(
				"line %d: statement has no output file name; expecting a comment ending in ';' above it",
				s.BodyLine)
		}
		out = append(out, ReportDefinition{Name: name, Stmt: s.Body, Line: nameLine})
	}
	return out, nil
}

// reportNameOf returns the output file name a statement's leading comment block
// declares, and the line it is on, or "" when the block declares none.
func reportNameOf(s sqlscript.Statement) (string, int) {
	name, line := "", 0
	for _, c := range s.Comments {
		if strings.HasSuffix(c.Text, ";") {
			name, line = strings.TrimSpace(strings.TrimSuffix(c.Text, ";")), c.Line
		}
	}
	return name, line
}

// OutputFormat is the format a report is written in, parquet, csv or json,
// taken from its statementProperties entry or its file name's suffix, in that
// order of formats. It is "" when neither names one, and such a report is
// refused.
//
// That case used to be logged and skipped, so a statement written into a
// reports file to be executed (DDL, DML, a lookup update) did nothing and
// nobody was told. It was never an "execute" path: it was the hook for
// `reportsAsTable`, which saved the result as a table and was removed in 2024,
// leaving a branch that only logged. A statement to execute belongs in a file
// declared `reportOrScript: "script"`, which runs whole.
func OutputFormat(name, declared string) string {
	switch {
	case declared == "parquet" || strings.HasSuffix(name, ".parquet"):
		return "parquet"
	case declared == "csv" || strings.HasSuffix(name, ".csv"):
		return "csv"
	case declared == "json" || strings.HasSuffix(name, ".json"):
		return "json"
	}
	return ""
}

// ErrNoOutputFormat is the refusal of a report OutputFormat gives no format.
func ErrNoOutputFormat(name string) error {
	return fmt.Errorf("report %q has no output format: its name ends in neither .csv, .json nor .parquet "+
		"and statementProperties declares no outputFormat for it; a statement to execute rather than "+
		"export belongs in a file declared reportOrScript: \"script\" in %s", name, ConfigFile)
}

// datePatternOpen opens the date pattern of a report name, {D:YYYY_MM_DD}.
const datePatternOpen = "{D:"

// ErrIncompleteDatePattern is the refusal of a report name whose date pattern
// is not closed: run_reports cannot name the output file.
func ErrIncompleteDatePattern(name string) error {
	return fmt.Errorf("error: report file name contains incomplete date pattern: %s", name)
}

// HasIncompleteDatePattern reports whether name opens a date pattern, `{D:`,
// and does not close it.
func HasIncompleteDatePattern(name string) bool {
	_, tail, found := strings.Cut(name, datePatternOpen)
	return found && !strings.Contains(tail, "}")
}

// outputFormatDecidable reports whether OutputFormat can be decided from a
// report's name before its {TOKEN} patterns are substituted: when the name has
// no pattern, or when the text after its last pattern holds a `.`, so that the
// suffix is literal. Otherwise a run's values decide it (`{PROCESSNAME}` could
// in principle end in `.csv`), and the save check leaves it to the run.
func outputFormatDecidable(name string) bool {
	last := strings.LastIndex(name, "}")
	if !strings.Contains(name, "{") || last < 0 {
		return true
	}
	return strings.Contains(name[last+1:], ".")
}

// ReportsFileError is a refusal run_reports makes of one reports file. Line is
// the line of the report it is about, or 0 for the file as a whole.
type ReportsFileError struct {
	Script string // the reportScripts entry, a path relative to reports/
	Line   int
	Err    error
}

func (e *ReportsFileError) Error() string {
	if e.Line > 0 {
		return fmt.Sprintf("reports/%s, line %d: %v", e.Script, e.Line, e.Err)
	}
	return fmt.Sprintf("reports/%s: %v", e.Script, e.Err)
}

func (e *ReportsFileError) Unwrap() error { return e.Err }

// ReportsFileErrors is what run_reports would refuse in the reports files
// these directives run, read from workspace, the workspace's root: a file it
// cannot read as reports (ParseReportDefinitions), a report name whose date
// pattern is not closed, and a report with no output format where its name
// decides that (outputFormatDecidable). reportName is the directives' key,
// whose `<reportName>.sql` runs when they list no reportScripts.
//
// It leaves out what run_reports does not read as a reports file: an entry
// declared `reportOrScript: "script"`, which runs whole, and an entry that is
// not a file of the workspace, which run_reports skips (or calls, when it
// names a report function). An entry that is not a local path is left to the
// run, which refuses it.
func (rd *ReportDirectives) ReportsFileErrors(reportName string, workspace fs.FS) []*ReportsFileError {
	if rd == nil || workspace == nil {
		return nil
	}
	scripts := rd.ReportScripts
	if len(scripts) == 0 {
		scripts = []string{reportName + ".sql"}
	}
	var errs []*ReportsFileError
	for _, script := range scripts {
		if rd.ReportProperties[script].Type == "script" {
			continue
		}
		file := path.Join("reports", script)
		if !fs.ValidPath(file) || !strings.HasPrefix(file, "reports/") {
			continue
		}
		content, err := fs.ReadFile(workspace, file)
		if err != nil {
			continue
		}
		reports, err := ParseReportDefinitions(string(content))
		if err != nil {
			errs = append(errs, &ReportsFileError{Script: script, Err: err})
			continue
		}
		for _, r := range reports {
			switch {
			case HasIncompleteDatePattern(r.Name):
				errs = append(errs, &ReportsFileError{Script: script, Line: r.Line,
					Err: ErrIncompleteDatePattern(r.Name)})
			case outputFormatDecidable(r.Name) &&
				OutputFormat(r.Name, rd.StatementProperties[r.Name].OutputFormat) == "":
				errs = append(errs, &ReportsFileError{Script: script, Line: r.Line,
					Err: ErrNoOutputFormat(r.Name)})
			}
		}
	}
	return errs
}
