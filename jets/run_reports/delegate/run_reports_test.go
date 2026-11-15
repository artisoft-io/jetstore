package delegate

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/artisoft-io/jetstore/jets/run_reports/directives"
	"github.com/artisoft-io/jetstore/jets/utils"
	"github.com/jackc/pgx/v5/pgxpool"
)

// unreadCounts is the record counts of a run that must not need them: reading
// them fails the test.
func unreadCounts(t *testing.T) *outputRecordCounts {
	t.Helper()
	return &outputRecordCounts{read: func() (int64, int64) {
		t.Error("the session's record counts were read, and no runWhen criterion asks for them")
		return 0, 0
	}}
}

// fixedCounts is the record counts of a session that read db records and wrote
// out, counting how many times they were read.
func fixedCounts(db, out int64, reads *int) *outputRecordCounts {
	return &outputRecordCounts{read: func() (int64, int64) {
		*reads++
		return db, out
	}}
}

func TestRunWhenMet(t *testing.T) {
	key := map[string]interface{}{"client": "Acme", "org": "Ex", "vendor": 3, "flag": true}
	for _, tc := range []struct {
		name     string
		criteria []RunWhenCriteria
		db, out  int64
		want     bool
	}{
		{"no criteria", nil, 0, 0, true},
		{"hasValue equal", []RunWhenCriteria{{FileKeyComponent: "client", HasValue: "Acme"}}, 0, 0, true},
		{"hasValue different", []RunWhenCriteria{{FileKeyComponent: "client", HasValue: "Other"}}, 0, 0, false},
		{"hasNotValue different", []RunWhenCriteria{{FileKeyComponent: "client", HasNotValue: "Other"}}, 0, 0, true},
		{"hasNotValue equal", []RunWhenCriteria{{FileKeyComponent: "client", HasNotValue: "Acme"}}, 0, 0, false},
		{"component only, present", []RunWhenCriteria{{FileKeyComponent: "org"}}, 0, 0, true},
		{"component missing", []RunWhenCriteria{{FileKeyComponent: "file_org", HasNotValue: "x"}}, 0, 0, false},
		{"component a number, present", []RunWhenCriteria{{FileKeyComponent: "vendor"}}, 0, 0, true},
		{"component neither text nor a number", []RunWhenCriteria{{FileKeyComponent: "flag"}}, 0, 0, false},
		{"every criterion must hold", []RunWhenCriteria{
			{FileKeyComponent: "client", HasValue: "Acme"},
			{FileKeyComponent: "org", HasValue: "Other"},
		}, 0, 0, false},
		{"output records, some written",
			[]RunWhenCriteria{{FileKeyComponent: "client", HasNonZeroOutputRecords: true}}, 10, 4, true},
		{"output records, none written",
			[]RunWhenCriteria{{FileKeyComponent: "client", HasNonZeroOutputRecords: true}}, 10, 0, false},
		{"output records, none read",
			[]RunWhenCriteria{{FileKeyComponent: "client", HasNonZeroOutputRecords: true}}, 0, 0, true},
		// A criterion naming no component checks only hasOutputRecordsOnly.
		// Until 2026-11-14 the missing component failed it, so it never held.
		{"output records with no component, some written",
			[]RunWhenCriteria{{HasNonZeroOutputRecords: true}}, 10, 4, true},
		{"output records with no component, none written",
			[]RunWhenCriteria{{HasNonZeroOutputRecords: true}}, 10, 0, false},
		{"output records with no component, none read",
			[]RunWhenCriteria{{HasNonZeroOutputRecords: true}}, 0, 0, true},
		// A criterion that checks nothing is refused by CheckRunWhen before
		// any report runs; should one get here, it fails rather than holds.
		{"no component and nothing to check", []RunWhenCriteria{{}}, 10, 4, false},
		{"a value with no component", []RunWhenCriteria{{HasValue: "Acme"}}, 10, 4, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			reads := 0
			got, why := runWhenMet(tc.criteria, key, fixedCounts(tc.db, tc.out, &reads))
			if got != tc.want {
				t.Fatalf("runWhenMet = %v (%s), want %v", got, why, tc.want)
			}
			if !got && why == "" {
				t.Error("runWhenMet gave no reason for a criterion not met")
			}
		})
	}
}

// year, month and day are numbers once the file key is read, and a criterion
// on one compares its decimal text, with or without leading zeros. Until
// 2026-11-14 the component had to be a string, so such a criterion never held.
func TestRunWhenOnANumericComponent(t *testing.T) {
	fileKey := "client=Acme/object_type=Claim/year=2026/month=03/day=7/file.csv"
	key := utils.SplitFileKeyIntoComponents(map[string]any{}, &fileKey)
	if _, isInt := key["month"].(int); !isInt {
		t.Fatalf("month is a %T, want the int this test is about", key["month"])
	}
	for _, tc := range []struct {
		name string
		c    RunWhenCriteria
		want bool
	}{
		{"month, hasValue without leading zero", RunWhenCriteria{FileKeyComponent: "month", HasValue: "3"}, true},
		{"month, hasValue with leading zero", RunWhenCriteria{FileKeyComponent: "month", HasValue: "03"}, true},
		{"month, hasValue other", RunWhenCriteria{FileKeyComponent: "month", HasValue: "4"}, false},
		{"month, hasValue not a number", RunWhenCriteria{FileKeyComponent: "month", HasValue: "March"}, false},
		{"month, hasValue signed", RunWhenCriteria{FileKeyComponent: "month", HasValue: "+3"}, false},
		{"month, hasNotValue same with leading zero", RunWhenCriteria{FileKeyComponent: "month", HasNotValue: "03"}, false},
		{"month, hasNotValue other", RunWhenCriteria{FileKeyComponent: "month", HasNotValue: "12"}, true},
		{"day, hasValue with leading zero", RunWhenCriteria{FileKeyComponent: "day", HasValue: "07"}, true},
		{"year, hasValue", RunWhenCriteria{FileKeyComponent: "year", HasValue: "2026"}, true},
		{"year, hasNotValue", RunWhenCriteria{FileKeyComponent: "year", HasNotValue: "2026"}, false},
		// Text is compared exactly: leading zeros are only forgiven for numbers.
		{"text, leading zero is not ignored", RunWhenCriteria{FileKeyComponent: "client", HasValue: "0Acme"}, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got, why := runWhenMet([]RunWhenCriteria{tc.c}, key, unreadCounts(t))
			if got != tc.want {
				t.Errorf("runWhenMet = %v (%s), want %v", got, why, tc.want)
			}
		})
	}
	// A component written as text with a leading zero stays text and is
	// compared exactly.
	text := map[string]interface{}{"batch": "03"}
	for want, holds := range map[string]bool{"03": true, "3": false} {
		got, _ := runWhenMet([]RunWhenCriteria{{FileKeyComponent: "batch", HasValue: want}}, text, unreadCounts(t))
		if got != holds {
			t.Errorf("text component \"03\", hasValue %q: %v, want %v", want, got, holds)
		}
	}
}

// A file key that carries no month has none. SplitFileKeyIntoComponents sets
// an absent month to 1, and its callers rely on that, so runWhen reads the
// key through runWhenComponents. Until 2026-11-14 it read the defaults, and a
// criterion on month with hasValue "1" held for this key.
func TestRunWhenOnADateComponentTheKeyDoesNotCarry(t *testing.T) {
	fileKey := "client=Acme/object_type=Claim/year=2026/file.csv"
	ca := &CommandArguments{
		FileKey:           fileKey,
		FileKeyComponents: utils.SplitFileKeyIntoComponents(map[string]any{}, &fileKey),
	}
	key := ca.runWhenComponents()
	for _, tc := range []struct {
		name string
		c    RunWhenCriteria
		want bool
	}{
		{"month absent, hasValue the default", RunWhenCriteria{FileKeyComponent: "month", HasValue: "1"}, false},
		{"month absent, hasValue the default with a leading zero", RunWhenCriteria{FileKeyComponent: "month", HasValue: "01"}, false},
		{"month absent, hasNotValue the default", RunWhenCriteria{FileKeyComponent: "month", HasNotValue: "1"}, true},
		{"month absent, hasNotValue other", RunWhenCriteria{FileKeyComponent: "month", HasNotValue: "12"}, true},
		{"month absent, component only", RunWhenCriteria{FileKeyComponent: "month"}, false},
		{"month absent, hasValue and hasNotValue", RunWhenCriteria{FileKeyComponent: "month", HasValue: "2", HasNotValue: "1"}, false},
		{"day absent, hasValue the default", RunWhenCriteria{FileKeyComponent: "day", HasValue: "1"}, false},
		{"day absent, hasNotValue the default", RunWhenCriteria{FileKeyComponent: "day", HasNotValue: "1"}, true},
		{"year present, hasValue", RunWhenCriteria{FileKeyComponent: "year", HasValue: "2026"}, true},
		{"year present, hasNotValue", RunWhenCriteria{FileKeyComponent: "year", HasNotValue: "2026"}, false},
		// A component other than the three, absent, still fails the criterion.
		{"other component absent, hasNotValue", RunWhenCriteria{FileKeyComponent: "org", HasNotValue: "x"}, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got, why := runWhenMet([]RunWhenCriteria{tc.c}, key, unreadCounts(t))
			if got != tc.want {
				t.Errorf("runWhenMet = %v (%s), want %v", got, why, tc.want)
			}
		})
	}
	// The reports still read the defaults.
	if ca.FileKeyComponents["month"] != 1 || ca.FileKeyComponents["day"] != 1 || ca.FileKeyComponents["year"] != 2026 {
		t.Errorf("FileKeyComponents changed: year %v, month %v, day %v",
			ca.FileKeyComponents["year"], ca.FileKeyComponents["month"], ca.FileKeyComponents["day"])
	}
	// A key carrying all three is read as it is.
	full := "client=Acme/year=2026/month=01/day=01/file.csv"
	ca = &CommandArguments{FileKey: full, FileKeyComponents: utils.SplitFileKeyIntoComponents(map[string]any{}, &full)}
	if ok, why := runWhenMet([]RunWhenCriteria{{FileKeyComponent: "month", HasValue: "1"}, {FileKeyComponent: "day", HasValue: "01"}},
		ca.runWhenComponents(), unreadCounts(t)); !ok {
		t.Errorf("month=01/day=01: %s", why)
	}
}

// A runWhen criterion that checks nothing is a configuration error, refused
// before any report runs, in reportProperties and in statementProperties.
func TestRunWhenThatChecksNothingIsRefused(t *testing.T) {
	dir := reportsDir(t)
	run := func(rd *ReportDirectives) error {
		rd.ReportScripts = []string{"thresholds.sql"}
		ca := &CommandArguments{
			FileKeyComponents:       map[string]interface{}{"client": "Acme"},
			SQLPool:                 unreachablePool(t),
			CurrentReportDirectives: rd,
			// No such file. Run as a script it fails without naming runWhen,
			// and as a reports file it is skipped, so an error naming runWhen
			// is the refusal and nothing else.
			ReportScriptPaths: []string{filepath.Join(dir, "thresholds.sql")},
		}
		return ca.RunReports(nil)
	}
	for name, rd := range map[string]*ReportDirectives{
		"report, empty criterion": {ReportProperties: map[string]ReportProperty{
			"thresholds.sql": {Type: "script", RunWhen: []RunWhenCriteria{{}}}}},
		"report, value with no component": {ReportProperties: map[string]ReportProperty{
			"thresholds.sql": {Type: "script", RunWhen: []RunWhenCriteria{{HasNotValue: "DEFAULT"}}}}},
		"statement, empty criterion": {StatementProperties: map[string]StatementProperty{
			"{PROCESSNAME}/x.csv": {RunWhen: []RunWhenCriteria{{FileKeyComponent: "client"}, {}}}}},
	} {
		err := run(rd)
		if err == nil || !strings.Contains(err.Error(), "runWhen") {
			t.Errorf("%s: RunReports error %v, want the runWhen refused", name, err)
		}
	}
	// hasOutputRecordsOnly alone is a criterion, and is not refused. What it
	// decides needs the session's counts, which TestRunWhenMet covers.
	ok := &ReportDirectives{ReportProperties: map[string]ReportProperty{
		"thresholds.sql": {Type: "script", RunWhen: []RunWhenCriteria{{HasNonZeroOutputRecords: true}}}}}
	if err := ok.CheckRunWhen(); err != nil {
		t.Errorf("CheckRunWhen on hasOutputRecordsOnly alone: %v", err)
	}
}

// The counts are a query, so they are read once per run and only when a
// criterion asks for them.
func TestRunWhenReadsTheCountsOnce(t *testing.T) {
	key := map[string]interface{}{"client": "Acme"}
	reads := 0
	counts := fixedCounts(10, 4, &reads)
	asks := []RunWhenCriteria{{FileKeyComponent: "client", HasNonZeroOutputRecords: true}}
	for range 3 {
		if ok, why := runWhenMet(asks, key, counts); !ok {
			t.Fatalf("runWhenMet: %s", why)
		}
	}
	if reads != 1 {
		t.Errorf("counts read %d times, want 1", reads)
	}
	if ok, _ := runWhenMet([]RunWhenCriteria{{FileKeyComponent: "client"}}, key, unreadCounts(t)); !ok {
		t.Error("runWhenMet without hasOutputRecordsOnly: not met")
	}
}

// reportsDir points the package at a fresh workspace and returns its reports
// directory.
func reportsDir(t *testing.T) string {
	t.Helper()
	savedHome, savedPrefix := workspaceHome, wprefix
	t.Cleanup(func() { workspaceHome, wprefix = savedHome, savedPrefix })
	workspaceHome, wprefix = t.TempDir(), "run_when_ws"
	dir := filepath.Join(workspaceHome, wprefix, "reports")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	return dir
}

// unreachablePool is a pool whose every query fails to connect: a statement
// that reaches it was run, not skipped.
func unreachablePool(t *testing.T) *pgxpool.Pool {
	t.Helper()
	pool, err := pgxpool.New(context.Background(), "postgres://nobody@127.0.0.1:1/none?connect_timeout=2")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(pool.Close)
	return pool
}

// A report whose reportProperties runWhen is not met is not run.
func TestReportRunWhenSkipsTheReport(t *testing.T) {
	dir := reportsDir(t)
	newArgs := func(client string) *CommandArguments {
		return &CommandArguments{
			FileKeyComponents: map[string]interface{}{"client": client},
			SQLPool:           unreachablePool(t),
			CurrentReportDirectives: &ReportDirectives{
				ReportScripts: []string{"thresholds.sql"},
				ReportProperties: map[string]ReportProperty{
					"thresholds.sql": {Type: "script", RunWhen: []RunWhenCriteria{
						{FileKeyComponent: "client", HasNotValue: "DEFAULT"},
					}},
				},
			},
			// No such file: running the script fails, skipping it does not.
			ReportScriptPaths: []string{filepath.Join(dir, "thresholds.sql")},
		}
	}
	if err := newArgs("DEFAULT").RunReports(nil); err != nil {
		t.Errorf("RunReports with runWhen not met: %v, want the report skipped", err)
	}
	if err := newArgs("Acme").RunReports(nil); err == nil {
		t.Error("RunReports with runWhen met: no error, want the missing script to have been run")
	}
}

// A statement whose statementProperties runWhen is not met is skipped and the
// others in its file run. It used to be parsed and never evaluated, so every
// statement ran.
func TestStatementRunWhenSkipsTheStatement(t *testing.T) {
	dir := reportsDir(t)
	path := filepath.Join(dir, "exports.sql")
	if err := os.WriteFile(path, []byte(`--{PROCESSNAME}/everyone.csv;
SELECT 1;

--{PROCESSNAME}/defaults_only.csv;
SELECT 2;
`), 0o644); err != nil {
		t.Fatal(err)
	}
	directives := &ReportDirectives{StatementProperties: map[string]StatementProperty{
		"{PROCESSNAME}/everyone.csv": {RunWhen: []RunWhenCriteria{
			{FileKeyComponent: "client", HasNotValue: "DEFAULT"},
		}},
		"{PROCESSNAME}/defaults_only.csv": {RunWhen: []RunWhenCriteria{
			{FileKeyComponent: "client", HasValue: "DEFAULT"},
		}},
	}}
	run := func(client string) error {
		ca := &CommandArguments{
			ProcessName:             "p",
			OutputPath:              "out",
			FileKeyComponents:       map[string]interface{}{"client": client},
			CurrentReportDirectives: directives,
		}
		return ca.runReportsDelegate(unreachablePool(t), t.TempDir(), path, &[]string{}, unreadCounts(t))
	}

	// Each client meets exactly one statement's runWhen, so exactly one is run,
	// and the error of the unreachable pool names which.
	for client, ran := range map[string]string{"DEFAULT": "SELECT 2", "Acme": "SELECT 1"} {
		err := run(client)
		if err == nil {
			t.Fatalf("client %s: no error, want the statement %q run against the unreachable pool", client, ran)
		}
		if !strings.Contains(err.Error(), ran) {
			t.Errorf("client %s: error %q, want it to name %q", client, err, ran)
		}
	}

	// A key that meets neither runs nothing, and reaches no pool at all.
	ca := &CommandArguments{
		ProcessName:             "p",
		FileKeyComponents:       map[string]interface{}{},
		CurrentReportDirectives: directives,
	}
	updated := []string{}
	if err := ca.runReportsDelegate(nil, t.TempDir(), path, &updated, unreadCounts(t)); err != nil {
		t.Errorf("runReportsDelegate with no statement's runWhen met: %v", err)
	}
	if len(updated) != 0 {
		t.Errorf("updated keys %v, want none", updated)
	}
}

// A registerReport entry with no org registers the file key's. Between 2024
// and this test it registered an empty org.
func TestReportRegistrationDefaults(t *testing.T) {
	ca := &CommandArguments{Org: "Ex", ObjectType: "Claim"}
	for _, tc := range []struct {
		name                string
		rr                  RegisterReportSpec
		org, objType, sType string
	}{
		{"all defaulted", RegisterReportSpec{TableName: "t"}, "Ex", "Claim", "report_table"},
		{"all declared",
			RegisterReportSpec{TableName: "t", Org: "Own", ObjectType: "QcClaim", SourceType: "file"},
			"Own", "QcClaim", "file"},
		{"object type only", RegisterReportSpec{TableName: "t", ObjectType: "QcClaim"}, "Ex", "QcClaim", "report_table"},
	} {
		org, objType, sType := ca.reportRegistration(&tc.rr)
		if org != tc.org || objType != tc.objType || sType != tc.sType {
			t.Errorf("%s: got (%q, %q, %q), want (%q, %q, %q)",
				tc.name, org, objType, sType, tc.org, tc.objType, tc.sType)
		}
	}
	// A file key with no org leaves it empty: there is nothing to fall back to.
	if org, _, _ := (&CommandArguments{}).reportRegistration(&RegisterReportSpec{TableName: "t"}); org != "" {
		t.Errorf("no org anywhere: got %q", org)
	}
}

// The save check of a report's name substitutes directives.NameTokens before
// it reads the date pattern, standing for the run (2026-11-14). So the run
// must substitute each of them: a token it left as written would close a date
// pattern at the run that the save reads as open.
func TestReportFileNameSubstitutesEveryNameToken(t *testing.T) {
	ca := &CommandArguments{Client: "c", SessionId: "s", Org: "o", ObjectType: "t",
		Environment: "e", OriginalFileName: "f", ProcessName: "p"}
	for _, token := range directives.NameTokens {
		got, err := ca.reportFileName("out_" + token + ".csv")
		if err != nil {
			t.Fatalf("%s: %v", token, err)
		}
		if strings.Contains(got, token) || strings.ContainsAny(got, "{}") {
			t.Errorf("%s is not substituted by the run: %q", token, got)
		}
	}
	// The case the save check reads differently since that day: the date
	// pattern of {D:YYYY{CLIENT} is not closed once {CLIENT} is substituted.
	const name = "out_{D:YYYY{CLIENT}.csv"
	if _, err := ca.reportFileName(name); err == nil {
		t.Errorf("%s: the run named the file", name)
	}
	if !directives.HasIncompleteDatePattern(name) {
		t.Errorf("%s: the save check reads the date pattern as closed", name)
	}
}
