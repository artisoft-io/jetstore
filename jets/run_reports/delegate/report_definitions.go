package delegate

import (
	"fmt"
	"strings"

	"github.com/artisoft-io/jetstore/jets/sqlscript"
)

// A report script under a workspace's `reports/` directory is a sequence of
// pairs: a `--` comment naming the output file, terminated by a semicolon, then
// the SQL statement that produces it.
//
//	--process={PROCESSNAME}/{ORIGINALFILENAME}_Opportunity.csv;
//	SELECT ... FROM "wrs:Opportunity" WHERE session_id='$SESSIONID';
//
// This used to be read by taking the text up to the next `;` byte as the name
// and the text up to the one after that as the statement. A `;` is an ordinary
// character inside a comment, a string literal, a quoted identifier or a
// dollar-quoted body, so any of those split a statement in two and the halves
// were submitted as two reports — the first unterminated, the second beginning
// with whatever followed the semicolon. What made this bite rather than merely
// lurk is that the *name* is a comment, so the format invites the author to
// write prose one line above the SQL and punishes the punctuation.
//
// The name is recovered from the comment block that precedes the statement
// rather than from a byte offset, which is what lets sqlscript.Split ignore
// semicolons everywhere they are not a terminator. The block may hold ordinary
// comments as well; the name is the last comment in it that ends with a
// semicolon, so a note written directly above the SQL does not become the
// output file name. The trailing semicolon is no longer a delimiter and is kept
// as the marker that says "this comment is a name" — every report script in
// `workspaces/` is written that way, and without it any comment above a
// statement would be read as one.
type reportDefinition struct {
	name string // output file name, before substitution of the {TOKEN} patterns
	stmt string // the SQL statement, without its terminating semicolon
	line int    // 1-based line of the name comment, for error messages
}

func parseReportDefinitions(script string) ([]reportDefinition, error) {
	stmts, err := sqlscript.Split(script)
	if err != nil {
		return nil, err
	}
	var out []reportDefinition
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
		out = append(out, reportDefinition{name: name, stmt: s.Body, line: nameLine})
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
