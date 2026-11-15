package delegate

import (
	"github.com/artisoft-io/jetstore/jets/run_reports/directives"
)

// A reports file's grammar, and why it is read the way it is, is documented
// with the code that reads it, directives.ParseReportDefinitions, which the
// workspace IDE's save check of reports/config.json shares (2026-11-14).
type reportDefinition struct {
	name string // output file name, before substitution of the {TOKEN} patterns
	stmt string // the SQL statement, without its terminating semicolon
	line int    // 1-based line of the name comment, for error messages
}

func parseReportDefinitions(script string) ([]reportDefinition, error) {
	defs, err := directives.ParseReportDefinitions(script)
	if err != nil {
		return nil, err
	}
	out := make([]reportDefinition, len(defs))
	for i, d := range defs {
		out[i] = reportDefinition{name: d.Name, stmt: d.Stmt, line: d.Line}
	}
	return out, nil
}
