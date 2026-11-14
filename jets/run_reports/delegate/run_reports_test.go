package delegate

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

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
	key := map[string]interface{}{"client": "Acme", "org": "Ex", "vendor": 3}
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
		{"component not a string", []RunWhenCriteria{{FileKeyComponent: "vendor"}}, 0, 0, false},
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
		// The rule as it has stood since runWhen was added: a criterion is
		// about a file key component, so one naming none never holds.
		{"output records with no component",
			[]RunWhenCriteria{{HasNonZeroOutputRecords: true}}, 10, 4, false},
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
