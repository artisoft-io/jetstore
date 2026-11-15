package datatable

import (
	"fmt"
	"io/fs"
	"path"
	"reflect"
	"sort"
	"strings"

	"github.com/artisoft-io/jetstore/jets/compute_pipes"
	"github.com/artisoft-io/jetstore/jets/datatable/wsfile"
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
// in, which run_reports ignores and the save check does not warn of.
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

// reportsFilesFindings is the `reports/config.json` row's check against the
// workspace (2026-11-14): what run_reports refuses in the reports files the
// directives run, when it reads them, before any report of the file runs.
// Each report's are checked, since each is the one some run follows
// (directives.ReportDirectives.ReportsFileErrors): a file run_reports cannot
// read as reports, a report name whose date pattern is not closed, and a
// report with no output format, where its name decides that and not a run's
// values.
//
// The refusals need the reports files and validateReportsConfig is given the
// directives alone, which is why until this check they were found only when a
// report ran. A reports file saved after the directives is checked against
// them by its own save since later the same day (reportsSQLFindings).
func reportsFilesFindings(_, content string, workspace fs.FS) []wsvalidate.Finding {
	config, err := directives.Decode([]byte(content))
	if err != nil {
		return nil // validateReportsConfig refused it
	}
	var findings []wsvalidate.Finding
	for _, name := range sortedNames(config) {
		rd := config[name]
		for _, e := range rd.ReportsFileErrors(name, workspace) {
			findings = append(findings, wsvalidate.Finding{
				Severity: wsvalidate.Error,
				Code:     "reports_config_reports_file",
				Message:  fmt.Sprintf("report %q cannot run: %v", name, e),
				Path:     "/" + escapeJSONPointer(name),
			})
		}
	}
	return findings
}

// reportsDir is the workspace directory of the reports files and of the report
// directives, `reports`.
var reportsDir = path.Dir(directives.ConfigFile)

// isReportsSQL reports whether the workspace-relative fileName is in reports/,
// where run_reports reads its reports files and scripts.
func isReportsSQL(fileName string) bool {
	return wsfile.InDir(fileName, reportsDir)
}

// reportsSQLFindings is the reports `.sql` row's check against the workspace
// (2026-11-14): a reports file is refused for what run_reports would refuse in
// it under the directives in reports/config.json that run it, the checks the
// directives' own save makes of it (reportsFilesFindings,
// directives.ReportDirectives.CheckReportsFile), made of the content being
// saved. Until this row a reports file saved after its directives was checked
// lexically only, and run_reports found it wrong when a report ran.
//
// A file no directives run as a reports file is not checked: one declared
// `reportOrScript: "script"` runs whole, and one no entry names is either run
// by a report with no directives, for a process of its name, or by nothing,
// and a save cannot tell which (the workspaces keep report SQL nothing runs,
// for debugging). Nor is a file whose directives do not decode, which their
// own save refuses and the run refuses.
func reportsSQLFindings(fileName, content string, workspace fs.FS) []wsvalidate.Finding {
	raw, err := fs.ReadFile(workspace, directives.ConfigFile)
	if err != nil {
		return nil
	}
	config, err := directives.Decode(raw)
	if err != nil {
		return nil
	}
	file := path.Clean(strings.ReplaceAll(fileName, "\\", "/"))
	var findings []wsvalidate.Finding
	for _, name := range sortedNames(config) {
		rd := config[name]
		for _, f := range rd.ReportsFiles(name) {
			if f.File != file {
				continue
			}
			for _, e := range rd.CheckReportsFile(f, content) {
				findings = append(findings, wsvalidate.Finding{
					Severity: wsvalidate.Error,
					Code:     "reports_file",
					Message:  fmt.Sprintf("report %q in %s cannot run: %v", name, directives.ConfigFile, e.Err),
					Line:     e.Line,
				})
			}
		}
	}
	return findings
}
