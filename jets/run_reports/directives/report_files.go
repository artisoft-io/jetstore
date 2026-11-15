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

// NameTokens are the {TOKEN} patterns run_reports substitutes in a report's
// name before it reads the name's date pattern (reportFileName,
// jets/run_reports/delegate, whose test checks that it substitutes each of
// them). A pattern not in the list is left in the name as written.
var NameTokens = []string{
	"{CLIENT}", "{SESSIONID}", "{ORG}", "{OBJECT_TYPE}", "{ENVIRONMENT}", "{ORIGINALFILENAME}", "{PROCESSNAME}",
}

// HasIncompleteDatePattern reports whether name, once its NameTokens are
// substituted, opens a date pattern, `{D:`, and does not close it.
//
// **The tokens are substituted first, as the run does (2026-11-14).** It read
// the name as written, so `{D:YYYY{CLIENT}` passed: the `}` closing `{CLIENT}`
// looked like the one closing the date pattern, and the run, which substitutes
// `{CLIENT}` before it looks, refused the name. A token is substituted here by
// nothing, which stands for every value that holds no brace. A value is known
// only to a run, so what one holding a `}` or a `{D:` does to the name is left
// to the run, which refuses the name then as before: in practice a client
// name, a session id, an org, an object type, an environment or a process
// name holds no brace, and an original file name has its `:` and `.` replaced
// but may hold one.
func HasIncompleteDatePattern(name string) bool {
	for _, token := range NameTokens {
		name = strings.ReplaceAll(name, token, "")
	}
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
	Script string // the reportScripts entry, a path relative to reports/, or the report's name
	File   string // the file, relative to the workspace's root; reports/<Script> when empty
	Line   int
	Err    error
}

func (e *ReportsFileError) Error() string {
	file := e.File
	if file == "" {
		file = "reports/" + e.Script
	}
	if e.Line > 0 {
		return fmt.Sprintf("%s, line %d: %v", file, e.Line, e.Err)
	}
	return fmt.Sprintf("%s: %v", file, e.Err)
}

func (e *ReportsFileError) Unwrap() error { return e.Err }

// ReportsFile is one file a report's directives run as a reports file
// (ReportDirectives.ReportsFiles).
type ReportsFile struct {
	// Script is the reportScripts entry that runs the file, the key of its
	// reportProperties: the file's path relative to reports/, or the report's
	// name when the directives list no reportScripts.
	Script string
	// File is the file's path relative to the workspace's root.
	File string
}

// ReportsFiles is the files run_reports reads as reports files when it runs
// these directives, those of the report reportName (CoordinateWorkAndUpdateStatus,
// jets/run_reports/delegate): each reportScripts entry, under reports/, or
// `reports/<reportName>.sql` when there is none.
//
// It leaves out what run_reports does not read as a reports file: an entry
// declared `reportOrScript: "script"`, which runs whole, and an entry whose
// path leaves reports/. An entry that names a report function is listed, and
// a caller that finds no such file skips it, as run_reports does.
//
// **With no reportScripts, the entry is the report's name and not its file**,
// as the run has it: run_reports runs `reports/<reportName>.sql` and reads its
// reportProperties under `<reportName>`. The save check read them under
// `<reportName>.sql` until 2026-11-14, so a report declared a script that way
// was read as reports at the save.
func (rd *ReportDirectives) ReportsFiles(reportName string) []ReportsFile {
	if rd == nil {
		return nil
	}
	var files []ReportsFile
	add := func(script, file string) {
		if rd.ReportProperties[script].Type == "script" {
			return
		}
		file = path.Join("reports", file)
		if !fs.ValidPath(file) || !strings.HasPrefix(file, "reports/") {
			return
		}
		files = append(files, ReportsFile{Script: script, File: file})
	}
	if len(rd.ReportScripts) == 0 {
		add(reportName, reportName+".sql")
		return files
	}
	for _, script := range rd.ReportScripts {
		add(script, script)
	}
	return files
}

// ReportsFileErrors is what run_reports would refuse in the reports files
// these directives run (ReportsFiles), read from workspace, the workspace's
// root: what CheckReportsFile refuses in each. reportName is the directives'
// key. A file the workspace does not hold is skipped, as run_reports skips it
// (or calls the report function the entry names).
func (rd *ReportDirectives) ReportsFileErrors(reportName string, workspace fs.FS) []*ReportsFileError {
	if rd == nil || workspace == nil {
		return nil
	}
	var errs []*ReportsFileError
	for _, f := range rd.ReportsFiles(reportName) {
		content, err := fs.ReadFile(workspace, f.File)
		if err != nil {
			continue
		}
		errs = append(errs, rd.CheckReportsFile(f, string(content))...)
	}
	return errs
}

// CheckReportsFile is what run_reports would refuse in content, the text of
// file, a reports file these directives run (ReportsFiles), before it runs
// any of the file: a file it cannot read as reports (ParseReportDefinitions),
// a report name whose date pattern is not closed (HasIncompleteDatePattern),
// and a report with no output format where its name decides that
// (outputFormatDecidable). Shared by the save of reports/config.json, which
// reads the files it runs, and the save of a reports file, which is checked
// against the directives that run it.
func (rd *ReportDirectives) CheckReportsFile(file ReportsFile, content string) []*ReportsFileError {
	script := file.Script
	reports, err := ParseReportDefinitions(content)
	if err != nil {
		return []*ReportsFileError{{Script: script, File: file.File, Err: err}}
	}
	var errs []*ReportsFileError
	for _, r := range reports {
		switch {
		case HasIncompleteDatePattern(r.Name):
			errs = append(errs, &ReportsFileError{Script: script, File: file.File, Line: r.Line,
				Err: ErrIncompleteDatePattern(r.Name)})
		case outputFormatDecidable(r.Name) &&
			OutputFormat(r.Name, rd.StatementProperties[r.Name].OutputFormat) == "":
			errs = append(errs, &ReportsFileError{Script: script, File: file.File, Line: r.Line,
				Err: ErrNoOutputFormat(r.Name)})
		}
	}
	return errs
}
