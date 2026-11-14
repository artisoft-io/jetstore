package datatable

import (
	"fmt"
	"path"
	"reflect"
	"sort"
	"strings"

	"github.com/artisoft-io/jetstore/jets/compute_pipes"
	"github.com/artisoft-io/jetstore/jets/run_reports/directives"
	"github.com/artisoft-io/jetstore/jets/wsvalidate"
)

// The thirteenth row of the validator table (2026-11-14): a workspace's report
// directives, `reports/config.json`, which the IDE lists in its `reports`
// section (wsfile/sections.go) and so lets a user edit. Until this row a save
// checked only that the file was a JSON object, and a file run_reports cannot
// follow was found when a report next ran.
//
// It refuses what run_reports refuses before any report runs, with the same
// code (package directives):
//
//   - a document that does not decode into the directives
//     (directives.Decode, which CoordinateWorkAndUpdateStatus reads it with);
//   - a runWhen criterion that checks nothing, in any report's
//     reportProperties or statementProperties
//     (directives.ReportDirectives.RunWhenErrors). run_reports checks only the
//     directives of the report it runs; the save checks every report's, since
//     each is the one some run follows.
//
// And it warns of every member the directives have no field for, which
// run_reports ignores: a misspelt key is a setting that silently does not
// apply, as for a pipeline configuration (compute_pipes.UnknownJSONFields).
// A report name is a key of the document, so it is never unknown. A
// `description` or `comment` member is not warned of (reportsConfigNotes):
// the workspaces write their notes in those, on most reports, and a warning
// on every one of them would bury the misspelling the warnings are for.
//
// The row is scoped to the one file run_reports reads (isReportsConfig): any
// other `config.json` of the workspace is not a report directives file.

// isReportsConfig reports whether the workspace-relative fileName is the
// workspace's report directives, `reports/config.json`, which run_reports reads
// by that exact name.
func isReportsConfig(fileName string) bool {
	return path.Clean(strings.ReplaceAll(fileName, "\\", "/")) == directives.ConfigFile
}

// validateReportsConfig refuses report directives run_reports would refuse
// before running a report, and warns of the members it ignores.
func validateReportsConfig(content string) []wsvalidate.Finding {
	config, err := directives.Decode([]byte(content))
	if err != nil {
		return []wsvalidate.Finding{{
			Severity: wsvalidate.Error,
			Code:     "reports_config_decode",
			Message:  fmt.Sprintf("no report can run: the report directives do not decode: %v", err),
		}}
	}
	var findings []wsvalidate.Finding
	for _, name := range sortedNames(config) {
		rd := config[name]
		for _, e := range rd.RunWhenErrors() {
			findings = append(findings, wsvalidate.Finding{
				Severity: wsvalidate.Error,
				Code:     "reports_config_run_when",
				Message: fmt.Sprintf("report %q cannot run: %s %q, runWhen[%d]: %v",
					name, e.Section, e.Entry, e.Index, e.Err),
				Path: fmt.Sprintf("/%s/%s/%s/runWhen/%d",
					escapeJSONPointer(name), e.Section, escapeJSONPointer(e.Entry), e.Index),
			})
		}
	}
	for _, p := range compute_pipes.UnknownJSONFields([]byte(content),
		reflect.TypeFor[map[string]directives.ReportDirectives]()) {
		if reportsConfigNotes[lastJSONPointerSegment(p)] {
			continue
		}
		findings = append(findings, wsvalidate.Finding{
			Severity: wsvalidate.Warning,
			Code:     "reports_config_unknown_field",
			Message:  fmt.Sprintf("ignored when the reports run: unknown field %q", lastJSONPointerSegment(p)),
			Path:     p,
		})
	}
	return findings
}

// reportsConfigNotes are the member names a reports/config.json carries notes
// in, which run_reports ignores and the save check does not warn of. On
// 2026-11-14 the four workspaces' files carried 21 `description` members and
// one `comment`; their other unknown members were four, which the save now
// warns of: three `type` members beside the `reportOrScript` that decides,
// and one `warning`.
var reportsConfigNotes = map[string]bool{"description": true, "comment": true}

func sortedNames(config map[string]directives.ReportDirectives) []string {
	names := make([]string, 0, len(config))
	for name := range config {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

// escapeJSONPointer escapes a JSON Pointer segment (RFC 6901 §3): a report
// name or a statement name may hold a `/`.
func escapeJSONPointer(segment string) string {
	return strings.ReplaceAll(strings.ReplaceAll(segment, "~", "~0"), "/", "~1")
}

// lastJSONPointerSegment is the member name a JSON Pointer ends with, unescaped.
func lastJSONPointerSegment(p string) string {
	segment := p[strings.LastIndex(p, "/")+1:]
	return strings.ReplaceAll(strings.ReplaceAll(segment, "~1", "/"), "~0", "~")
}
