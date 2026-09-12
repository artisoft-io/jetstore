package delegate

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestParseReportDefinitions(t *testing.T) {
	// The shape every report script in workspaces/ has, including the two
	// things the previous reader could not survive: a semicolon inside a
	// comment, and a statement whose terminator sits alone on its own line.
	script := `--client={CLIENT}/export/registry.csv;
SELECT * FROM jetsapi.client_org_registry WHERE client = '$CLIENT';

-- This one counts claims. Note: the count excludes denials; see the rules.
--process={PROCESSNAME}/reports/counts.csv;
SELECT 'a' AS entity, COUNT(*) FROM "hc:Claim" WHERE session_id='$SESSIONID'
UNION ALL
--SELECT 'b' AS entity, COUNT(*) FROM "hc:Claim" WHERE x='1'
--UNION ALL
SELECT 'c' AS entity, COUNT(*) FROM "hc:Claim"
;
-- end of file
`
	got, err := parseReportDefinitions(script)
	if err != nil {
		t.Fatalf("parseReportDefinitions: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("got %d reports, want 2: %+v", len(got), got)
	}
	if got[0].name != "client={CLIENT}/export/registry.csv" || got[0].line != 1 {
		t.Errorf("report 0 = %q at line %d", got[0].name, got[0].line)
	}
	if got[0].stmt != "SELECT * FROM jetsapi.client_org_registry WHERE client = '$CLIENT'" {
		t.Errorf("report 0 stmt = %q", got[0].stmt)
	}
	// The free-form comment above the name is not the name, and the one with a
	// semicolon in the middle of a sentence does not end the statement.
	if got[1].name != "process={PROCESSNAME}/reports/counts.csv" || got[1].line != 5 {
		t.Errorf("report 1 = %q at line %d, want the name comment at line 5", got[1].name, got[1].line)
	}
	if !strings.HasPrefix(got[1].stmt, "SELECT 'a'") || strings.HasSuffix(got[1].stmt, ";") {
		t.Errorf("report 1 stmt = %q", got[1].stmt)
	}
	// A comment inside the statement stays in the statement: it is how the
	// corpus disables a UNION arm.
	if !strings.Contains(got[1].stmt, "--SELECT 'b'") {
		t.Errorf("report 1 stmt lost its inline comments: %q", got[1].stmt)
	}
}

func TestParseReportDefinitionsSemicolonInStringLiteral(t *testing.T) {
	script := `--out.csv;
SELECT 'Drug on Formulary; Non-preferred' AS label FROM t;
`
	got, err := parseReportDefinitions(script)
	if err != nil {
		t.Fatalf("parseReportDefinitions: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("got %d reports, want 1: %+v", len(got), got)
	}
	if !strings.Contains(got[0].stmt, "Non-preferred' AS label FROM t") {
		t.Errorf("stmt = %q", got[0].stmt)
	}
}

func TestParseReportDefinitionsErrors(t *testing.T) {
	for _, tc := range []struct{ name, script, want string }{
		{
			"name with no statement",
			"--a.csv;\nSELECT 1;\n--b.csv;\n",
			`line 3: report "b.csv" has no statement`,
		},
		{
			// A plain SQL script, which is what reportOrScript: "script"
			// declares and is executed whole by runSqlScriptDelegate. Reaching
			// here means it was wired as a report by mistake; the previous
			// reader took the first 'n' characters of the SQL as a file name.
			"statement with no name",
			"BEGIN;\nSELECT 1;\nCOMMIT;\n",
			"line 1: statement has no output file name",
		},
		{
			"unterminated literal",
			"--a.csv;\nSELECT 'oops;\n",
			"unterminated string literal opened on line 2",
		},
	} {
		_, err := parseReportDefinitions(tc.script)
		if err == nil {
			t.Errorf("%s: want an error", tc.name)
			continue
		}
		if !strings.Contains(err.Error(), tc.want) {
			t.Errorf("%s: error = %q, want it to contain %q", tc.name, err, tc.want)
		}
	}
}

// TestReportCorpusParses runs the parser over the real report scripts. It is
// the only assertion available that the format this file describes is the
// format the workspaces are written in, and it is the check a workspace author
// wants before a deployment rather than during one.
//
// It skips unless JETS_REPORTS_CORPUS_DIR names a directory to walk for
// reports/*.sql, because the workspaces are separate repositories and are not
// present in a plain checkout of this one — the pattern JETS_PC_CORPUS_DIR
// established. On the machine that wrote this the run is:
//
//	JETS_REPORTS_CORPUS_DIR=<...>/workspaces go test -count=1 -run TestReportCorpus ./jets/run_reports/delegate/
//
// Run it with -count=1: the corpus is outside this Go module, so nothing in the
// test cache key changes when the workspaces move.
//
// It asserts only that a script parsing as a report script parses cleanly, and
// counts the rest. Of the 26 reports/*.sql in the corpus on 2026-09-12, twenty
// parsed as report scripts carrying 86 reports and six carry no name comment at
// all: three are declared `reportOrScript: "script"` in their config.json —
// cedargate's two update_qc_*_thresholds.sql and walrus's
// update_drug_class_interchange_lookups.sql, executed whole by
// runSqlScriptDelegate, two of them opening their own transaction — and three
// are wired by nothing (usi's network_csv_output.sql and
// network_json_output.sql, walrus's drug_class_interchange_savings.sql). So a
// test asserting that every .sql under reports/ parses as a report would be
// asserting something false. The figures are a dated measurement of living
// repositories, not an assertion; what is asserted is the property.
func TestReportCorpusParses(t *testing.T) {
	dir := os.Getenv("JETS_REPORTS_CORPUS_DIR")
	if dir == "" {
		t.Skip("JETS_REPORTS_CORPUS_DIR is not set; see the header of this test")
	}
	var files []string
	err := filepath.WalkDir(dir, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !d.IsDir() && strings.HasSuffix(path, ".sql") &&
			filepath.Base(filepath.Dir(path)) == "reports" {
			files = append(files, path)
		}
		return nil
	})
	if err != nil {
		t.Fatalf("walking %s: %v", dir, err)
	}
	if len(files) == 0 {
		t.Fatalf("no reports/*.sql under %s", dir)
	}
	parsed, reports, notReports := 0, 0, 0
	for _, f := range files {
		b, err := os.ReadFile(f)
		if err != nil {
			t.Errorf("%s: %v", f, err)
			continue
		}
		defs, err := parseReportDefinitions(string(b))
		switch {
		case err == nil:
			parsed++
			reports += len(defs)
			for _, d := range defs {
				if d.name == "" || d.stmt == "" {
					t.Errorf("%s:%d: empty report definition", f, d.line)
				}
			}
		case strings.Contains(err.Error(), "has no output file name"):
			// A plain SQL script rather than a report script.
			notReports++
			t.Logf("plain SQL script: %s", f)
		default:
			t.Errorf("%s: %v", f, err)
		}
	}
	t.Logf("%d files: %d parsed as report scripts (%d reports), %d are plain SQL scripts",
		len(files), parsed, reports, notReports)
}
