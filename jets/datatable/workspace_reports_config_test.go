package datatable

import (
	"strings"
	"testing"

	"github.com/artisoft-io/jetstore/jets/run_reports/directives"
	"github.com/artisoft-io/jetstore/jets/wsvalidate"
)

const goodReportsConfig = `{
  "Example_Report": {
    "reportScripts": ["thresholds.sql", "exports.sql"],
    "updateLookupTables": true,
    "reportProperties": {
      "thresholds.sql": {
        "reportOrScript": "script",
        "runWhen": [{ "fileKeyComponent": "org", "hasNotValue": "TEST" }]
      },
      "exports.sql": {
        "runWhen": [{ "hasOutputRecordsOnly": true }]
      }
    },
    "statementProperties": {
      "{PROCESSNAME}/test_only.csv": {
        "runWhen": [{ "fileKeyComponent": "month", "hasValue": "03" }]
      }
    },
    "registerReport": [{ "table_name": "summary" }]
  },
  "Other_Report": {}
}`

// The thirteenth row: reports/config.json is refused at the save when
// run_reports would refuse it before running a report.
func TestSaveCheckRefusesReportDirectivesRunReportsWouldRefuse(t *testing.T) {
	const name = directives.ConfigFile
	warnings, err := checkWorkspaceFile(name, goodReportsConfig)
	if err != nil {
		t.Fatalf("a good reports/config.json was refused: %v", err)
	}
	if len(warnings) != 0 {
		t.Errorf("a good reports/config.json has warnings: %v", warnings)
	}
	// hasOutputRecordsOnly alone is a criterion.
	if _, err := checkWorkspaceFile(name, `{"R": {"reportProperties": {"a.sql": {"runWhen": [{"hasOutputRecordsOnly": true}]}}}}`); err != nil {
		t.Errorf("hasOutputRecordsOnly alone was refused: %v", err)
	}

	for content, want := range map[string]string{
		// Not the directives' shape: run_reports stops reading the file.
		`{"R": {"reportScripts": "a.sql"}}`: "do not decode",
		`{"R": []}`:                         "do not decode",
		`{"R": {"reportProperties": {"a.sql": {"runWhen": {}}}}}`: "do not decode",
		// A criterion that checks nothing, in either place.
		`{"R": {"reportProperties": {"a.sql": {"runWhen": [{}]}}}}`:                                                                            "/R/reportProperties/a.sql/runWhen/0: report \"R\" cannot run",
		`{"R": {"reportProperties": {"a.sql": {"runWhen": [{"hasValue": "x"}]}}}}`:                                                             "hasValue or hasNotValue and no fileKeyComponent",
		`{"R": {"statementProperties": {"{PROCESSNAME}/x.csv": {"runWhen": [{"fileKeyComponent": "org"}, {"hasOutputRecordsOnly": false}]}}}}`: "/R/statementProperties/{PROCESSNAME}~1x.csv/runWhen/1",
		// Any report's directives, not only the first.
		`{"A": {}, "B": {"reportProperties": {"b.sql": {"runWhen": [{"hasNotValue": "x"}]}}}}`: "/B/reportProperties/b.sql/runWhen/0",
	} {
		_, err := checkWorkspaceFile(name, content)
		if err == nil {
			t.Errorf("%s was accepted", content)
		} else if !strings.Contains(err.Error(), want) {
			t.Errorf("%s: %v, want it to contain %q", content, err, want)
		}
	}
}

// What run_reports ignores is a warning, named with its path: the save goes
// through.
func TestReportDirectivesWarnOfUnknownFields(t *testing.T) {
	content := `{"R": {"description": "a note", "comment": "another", "reportScript": ["a.sql"], "reportProperties": {"a.sql": {"runWhen": [{"fileKeyComponent": "org", "hasValu": "x"}]}}}}`
	warnings, err := checkWorkspaceFile(directives.ConfigFile, content)
	if err != nil {
		t.Fatalf("refused: %v", err)
	}
	got := map[string]bool{}
	for _, w := range warnings {
		if w.Severity != wsvalidate.Warning || w.Code != "reports_config_unknown_field" {
			t.Errorf("unexpected finding %+v", w)
		}
		got[w.Path] = true
	}
	for _, want := range []string{"/R/reportScript", "/R/reportProperties/a.sql/runWhen/0/hasValu"} {
		if !got[want] {
			t.Errorf("no warning at %s; got %v", want, warnings)
		}
	}
	if len(warnings) != 2 {
		t.Errorf("%d warnings, want 2: %v", len(warnings), warnings)
	}
	// The decoder matches a member's name without case, so a key written in
	// another case is read, and is not unknown.
	if warnings, _ := checkWorkspaceFile(directives.ConfigFile, `{"R": {"ReportScripts": ["a.sql"]}}`); len(warnings) != 0 {
		t.Errorf("a field in another case was reported: %v", warnings)
	}
}

// The row is scoped to the file run_reports reads; a config.json anywhere
// else is not report directives, and only the JSON check applies to it.
func TestReportDirectivesRowAppliesToReportsConfigOnly(t *testing.T) {
	const refused = `{"R": {"reportProperties": {"a.sql": {"runWhen": [{}]}}}}`
	for _, name := range []string{"reports/config.json", "reports/./config.json", `reports\config.json`} {
		if _, err := checkWorkspaceFile(name, refused); err == nil {
			t.Errorf("%s: a criterion that checks nothing was accepted", name)
		}
	}
	for _, name := range []string{
		"config.json",
		"pipes_config/config.json",
		"reports/sub/config.json",
		"reports/old_config.json",
		"reports/Config.json",
		"process_config/reports/config.json",
	} {
		if validatorFor(name) != nil {
			t.Errorf("%s is dispatched to a validator; only reports/config.json is report directives", name)
		}
		if _, err := checkWorkspaceFile(name, refused); err != nil {
			t.Errorf("%s was refused: %v", name, err)
		}
	}
}
