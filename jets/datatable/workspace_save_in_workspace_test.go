package datatable

// The two rows that check a saved file against the rest of the workspace
// (a row's inWorkspace, workspace_file_validators.go): a form's table, and the
// reports files a reports/config.json runs.

import (
	"strings"
	"testing"
	"testing/fstest"

	"github.com/artisoft-io/jetstore/jets/run_reports/directives"
	"github.com/artisoft-io/jetstore/jets/wsvalidate"
)

const formDrawingTables = `{
  "schemaVersion": 1,
  "forms": {
    "pick": {
      "rows": [[{ "field": "dataTable", "key": "picked", "table": "sampleTable" }]],
      "actions": [{ "label": "Next", "action": "next" }]
    },
    "review": {
      "rows": [
        [{ "field": "dataTable", "key": "shown", "table": "missingTable" }],
        [{ "field": "dataTable", "key": "again", "table": "sampleTable" }]
      ],
      "actions": [{ "label": "Done", "action": "done" }]
    }
  }
}`

// A form that draws a table the workspace does not hold is saved with a
// warning naming the form, the table and the file, at the field's pointer. A
// form whose tables are all there is saved with none, and with no workspace to
// read the form is checked alone, as before.
func TestSaveCheckWarnsOfAFormTableTheWorkspaceLacks(t *testing.T) {
	const name = "user_flows/sampleUF.form.json"
	if findings := validatorFor(name)(formDrawingTables); len(findings) != 0 {
		t.Fatalf("the test form is not a valid form document: %v", findings)
	}
	ws := fstest.MapFS{"table_configs/sampleTable.tc.json": {Data: []byte(`{}`)}}
	warnings, err := checkWorkspaceFileIn(name, formDrawingTables, ws)
	if err != nil {
		t.Fatalf("a form drawing a missing table was refused, want a warning: %v", err)
	}
	if len(warnings) != 1 {
		t.Fatalf("got %d warnings, want 1 for missingTable: %v", len(warnings), warnings)
	}
	w := warnings[0]
	if w.Severity != wsvalidate.Warning || w.Code != "form_table_missing" ||
		w.Path != "/forms/review/rows/0/0/table" ||
		!strings.Contains(w.Message, `"review"`) || !strings.Contains(w.Message, "table_configs/missingTable.tc.json") {
		t.Errorf("warning = %+v", w)
	}

	ws["table_configs/missingTable.tc.json"] = &fstest.MapFile{Data: []byte(`{}`)}
	if warnings, err = checkWorkspaceFileIn(name, formDrawingTables, ws); err != nil || len(warnings) != 0 {
		t.Errorf("a form whose tables exist: %v %v, want no warning", warnings, err)
	}
	if warnings, err = checkWorkspaceFile(name, formDrawingTables); err != nil || len(warnings) != 0 {
		t.Errorf("a form checked with no workspace: %v %v, want no warning", warnings, err)
	}
}

// reportsConfigRunning runs reports/exports.sql as reports, and
// reports/update.sql as a script.
const reportsConfigRunning = `{
  "Example_Report": {
    "reportScripts": ["update.sql", "exports.sql", "merge_optimizer_statistics"],
    "reportProperties": { "update.sql": { "reportOrScript": "script" } },
    "statementProperties": { "{PROCESSNAME}/declared": { "outputFormat": "parquet" } }
  },
  "Defaulted": {}
}`

// What run_reports refuses in a reports file the directives run is refused at
// the save of the directives: a report with no output format, a date pattern
// not closed, a name with no statement, and a file that does not read as
// reports. A script, a reportScripts entry that is no file (a report
// function), and a name whose format a run's values decide are left alone.
func TestSaveCheckRefusesReportsFilesRunReportsWouldRefuse(t *testing.T) {
	const name = directives.ConfigFile
	good := `--{PROCESSNAME}/counts.csv;
SELECT 1;
--{PROCESSNAME}/declared;
SELECT 2;
--{PROCESSNAME}/{D:YYYY_MM_DD}_out.json;
SELECT 3;
--{PROCESSNAME}/by_{ORIGINALFILENAME};
SELECT 4;
`
	ws := fstest.MapFS{
		"reports/exports.sql": {Data: []byte(good)},
		// A script runs whole: an UPDATE with no name is what it holds.
		"reports/update.sql": {Data: []byte("BEGIN;\nUPDATE lk SET v = 1;\nCOMMIT;\n")},
	}
	if warnings, err := checkWorkspaceFileIn(name, reportsConfigRunning, ws); err != nil || len(warnings) != 0 {
		t.Fatalf("good reports files: %v %v", warnings, err)
	}

	for _, c := range []struct {
		file, content string
		want          []string
	}{
		{"reports/exports.sql", good + "--update the lookup;\nUPDATE lk SET v = 1;\n",
			[]string{`"Example_Report"`, "reports/exports.sql, line 9", `"update the lookup" has no output format`}},
		{"reports/exports.sql", "--{PROCESSNAME}/{D:YYYY_out.csv;\nSELECT 1;\n",
			[]string{"reports/exports.sql, line 1", "incomplete date pattern"}},
		{"reports/exports.sql", "--{PROCESSNAME}/a.csv;\n",
			[]string{"reports/exports.sql", `report "{PROCESSNAME}/a.csv" has no statement`}},
		{"reports/exports.sql", "SELECT 1;\n",
			[]string{"reports/exports.sql", "statement has no output file name"}},
		// No reportScripts: <report name>.sql runs.
		{"reports/Defaulted.sql", "--plain;\nSELECT 1;\n",
			[]string{`"Defaulted"`, "reports/Defaulted.sql, line 1", `"plain" has no output format`}},
	} {
		bad := fstest.MapFS{}
		for k, v := range ws {
			bad[k] = v
		}
		bad[c.file] = &fstest.MapFile{Data: []byte(c.content)}
		_, err := checkWorkspaceFileIn(name, reportsConfigRunning, bad)
		if err == nil {
			t.Errorf("%s holding %q was not refused", c.file, c.content)
			continue
		}
		for _, want := range c.want {
			if !strings.Contains(err.Error(), want) {
				t.Errorf("%s: refusal %q does not contain %q", c.file, err, want)
			}
		}
	}

	// The same files, checked with no workspace to read: the directives alone.
	if _, err := checkWorkspaceFile(name, reportsConfigRunning); err != nil {
		t.Errorf("checked alone: %v", err)
	}
}

// A reports file is checked when it is saved against the directives that run
// it, as the directives' save checks it (2026-11-14): until then a reports file
// saved after its directives was checked lexically only. The content checked is
// the one being saved, not the file the workspace holds.
func TestSaveCheckRefusesAReportsFileItsDirectivesWouldRefuse(t *testing.T) {
	good := "--{PROCESSNAME}/counts.csv;\nSELECT 1;\n"
	ws := fstest.MapFS{
		directives.ConfigFile: {Data: []byte(reportsConfigRunning)},
		"reports/exports.sql": {Data: []byte(good)},
	}
	for _, c := range []struct {
		file, content string
		want          []string // empty: saved
	}{
		{"reports/exports.sql", good, nil},
		{"reports/exports.sql", good + "--update the lookup;\nUPDATE lk SET v = 1;\n",
			[]string{"line 3", `report "Example_Report" in reports/config.json cannot run`, `"update the lookup" has no output format`}},
		{"reports/exports.sql", "--{PROCESSNAME}/{D:YYYY_out.csv;\nSELECT 1;\n",
			[]string{"line 1", "incomplete date pattern"}},
		// The date pattern read once the tokens are substituted, as the run reads it.
		{"reports/exports.sql", "--{PROCESSNAME}/{D:YYYY{CLIENT}_out.csv;\nSELECT 1;\n",
			[]string{"line 1", "incomplete date pattern"}},
		{"reports/exports.sql", "--{PROCESSNAME}/{D:YYYY{CLIENT}}_out.csv;\nSELECT 1;\n", nil},
		{"reports/exports.sql", "SELECT 1;\n", []string{"statement has no output file name"}},
		{"./reports/exports.sql", "SELECT 1;\n", []string{"statement has no output file name"}},
		// Still lexed first, as every SQL file is.
		{"reports/exports.sql", "--a.csv;\nSELECT 'open;\n", []string{"line 2"}},
		// Run whole, as a script: an UPDATE with no name is what it holds.
		{"reports/update.sql", "BEGIN;\nUPDATE lk SET v = 1;\nCOMMIT;\n", nil},
		// Run by no directives: either by a report of its name with none, or by
		// nothing, which a save cannot tell apart.
		{"reports/unlisted.sql", "SELECT 1;\n", nil},
		// No reportScripts: <report name>.sql runs.
		{"reports/Defaulted.sql", "--plain;\nSELECT 1;\n",
			[]string{`report "Defaulted"`, "line 1", `"plain" has no output format`}},
	} {
		_, err := checkWorkspaceFileIn(c.file, c.content, ws)
		switch {
		case len(c.want) == 0 && err != nil:
			t.Errorf("%s holding %q: %v, want saved", c.file, c.content, err)
		case len(c.want) > 0 && err == nil:
			t.Errorf("%s holding %q was saved, want refused", c.file, c.content)
		case err != nil:
			for _, want := range c.want {
				if !strings.Contains(err.Error(), want) {
					t.Errorf("%s: refusal %q does not contain %q", c.file, err, want)
				}
			}
		}
	}

	// With no directives to read, or none that decode, a reports file is
	// checked lexically only, as it was.
	for _, ws := range []fstest.MapFS{{}, {directives.ConfigFile: {Data: []byte(`{"R": []}`)}}} {
		if _, err := checkWorkspaceFileIn("reports/Defaulted.sql", "SELECT 1;\n", ws); err != nil {
			t.Errorf("no directives to read: %v, want saved", err)
		}
	}
}

// A report with no reportScripts runs `reports/<report>.sql`, and run_reports
// reads its reportProperties under the report's name, not the file's. So
// such a file declared a script runs whole, and is not read as reports by
// either save. Until 2026-11-14 the directives' save read the properties under
// `<report>.sql`, and refused these directives.
func TestADefaultReportsFileDeclaredAScriptIsNotReadAsReports(t *testing.T) {
	config := `{"Lookups": {"reportProperties": {"Lookups": {"reportOrScript": "script"}}}}`
	script := "BEGIN;\nUPDATE lk SET v = 1;\nCOMMIT;\n"
	ws := fstest.MapFS{
		directives.ConfigFile: {Data: []byte(config)},
		"reports/Lookups.sql": {Data: []byte(script)},
	}
	if _, err := checkWorkspaceFileIn(directives.ConfigFile, config, ws); err != nil {
		t.Errorf("the directives: %v, want saved", err)
	}
	if _, err := checkWorkspaceFileIn("reports/Lookups.sql", script, ws); err != nil {
		t.Errorf("the script: %v, want saved", err)
	}
}
