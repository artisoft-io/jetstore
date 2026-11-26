package triage

import (
	"context"
	"fmt"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/artisoft-io/jetstore/jets/errorcode"
	"github.com/jackc/pgx/v5/pgxpool"
)

// The per-record locus's error codes, against a real Postgres: every row of the
// fixture gets a code, JetStore's own messages match their fixed codes in the
// form before map_record's prefix and after it, a code below its threshold
// raises nothing and is still counted, a debug code never raises, and only the
// planted unknown message reads uncatalogued.

// codedError is one process_errors row to plant: operator "" is a null
// operator_type.
type codedError struct {
	operator, message string
	n                 int
}

// The fixture workspace's raises, as its catalogue names them (invented text).
const (
	raiseNotNumeric = "error amount is not numeric"
	raiseNotInTable = "warning code not in the lookup"
	raiseDebug      = "debug: breaking on session"
)

func plantErrors(t *testing.T, pool *pgxpool.Pool, execKey int, session string, rows []codedError) {
	t.Helper()
	i := 0
	for _, r := range rows {
		for range r.n {
			var op any
			if r.operator != "" {
				op = r.operator
			}
			if _, err := pool.Exec(context.Background(), `INSERT INTO jetsapi.process_errors
				(pipeline_execution_status_key, session_id, row_jets_key, error_message, shard_id, operator_type)
				VALUES ($1, $2, $3, $4, 0, $5)`, execKey, session, fmt.Sprintf("k%d", i), r.message, op); err != nil {
				t.Fatalf("inserting process_errors row: %v", err)
			}
			i++
		}
	}
}

func catalogue(t *testing.T, pool *pgxpool.Pool) {
	t.Helper()
	if _, err := pool.Exec(context.Background(), `INSERT INTO jetsapi.error_code_catalogue
		(operator_type, match_prefix, code, kind, report_threshold, source, source_ref, description) VALUES
		('jetrules', $1, NULL, 'error', 2, 'rules', 'rules.jr amount_rule', NULL),
		('jetrules', $2, NULL, 'error', 5, 'rules', 'rules.jr lookup_rule', NULL),
		('jetrules', $3, 'debug break', 'debug', NULL, 'rules', 'rules.jr debug_rule', NULL),
		('*', '', NULL, 'error', 10, 'rules', 'workspace default', NULL)`,
		raiseNotNumeric, raiseNotInTable, raiseDebug); err != nil {
		t.Fatalf("filling the catalogue: %v", err)
	}
}

func codeNamed(f Finding, code string) (CodeCount, bool) {
	i := slices.IndexFunc(f.Codes, func(c CodeCount) bool { return c.Code == code })
	if i < 0 {
		return CodeCount{}, false
	}
	return f.Codes[i], true
}

func TestEveryPerRecordRowGetsACode(t *testing.T) {
	pool := freshDB(t, "triage_error_codes", append(slices.Clone(migratedTables), "error_code_catalogue"))
	catalogue(t, pool)
	now := time.Now().UTC()
	k := header(t, pool, "client2", "loader", "claims", "s-codes", "completed", now.Add(-time.Hour))
	workerRow(t, pool, k, "client2", "loader", "s-codes", "reducing01", 0, "completed", 1000, 1000, "", now.Add(-time.Hour))
	const unknown = "something nobody catalogued"
	plantErrors(t, pool, k, "s-codes", []codedError{
		{errorcode.OperatorJetrules, errorcode.RuleExceptionPrefix + raiseNotNumeric + ": 12.5", 2},
		// A null operator type, read from the message's prefix.
		{"", errorcode.RuleExceptionPrefix + raiseNotNumeric + ": 7", 1},
		{errorcode.OperatorJetrules, errorcode.RuleExceptionPrefix + raiseNotInTable + ": X9", 2},
		{errorcode.OperatorJetrules, errorcode.RuleExceptionPrefix + raiseDebug + " 17", 4},
		{errorcode.OperatorJetrules, "MAX LOOP REACHED for rule set main", 1},
		// map_record's mapping message before the prefix and after it.
		{errorcode.OperatorMapRecord, "error mapping column at output position 3: bad date", 1},
		{errorcode.OperatorMapRecord, errorcode.MapRecordPrefix + "error mapping column member_dob at output position 3: bad date", 1},
		{errorcode.OperatorJetrules, errorcode.RuleExceptionPrefix + unknown, 1},
	})
	r := classify(t, pool, "s-codes", 0)
	assertAllValid(t, r)
	f := wantVerdict(t, r, LocusPerRecordFailuresReported, Present)

	want := map[string]struct {
		rows  int64
		state string
	}{
		raiseNotNumeric:                        {3, CodeReports},
		raiseNotInTable:                        {2, CodeQuiet},
		"debug break":                          {4, CodeDebug},
		errorcode.CodeMaxLoopReached:           {1, CodeReports},
		errorcode.CodeMappingColumn:            {2, CodeReports},
		errorcode.UncataloguedCode("jetrules"): {1, CodeReports},
	}
	var total int64
	for _, c := range f.Codes {
		total += c.StoredRows
		w, ok := want[c.Code]
		if !ok {
			t.Errorf("code %q was not expected: %+v", c.Code, c)
			continue
		}
		if c.StoredRows != w.rows || c.State != w.state {
			t.Errorf("code %q: %d rows, %s; want %d, %s", c.Code, c.StoredRows, c.State, w.rows, w.state)
		}
		if c.Uncatalogued != (c.Code == errorcode.UncataloguedCode("jetrules")) {
			t.Errorf("code %q: uncatalogued %v", c.Code, c.Uncatalogued)
		}
	}
	if len(f.Codes) != len(want) || total != 13 {
		t.Errorf("%d codes over %d rows; want %d over 13: %+v", len(f.Codes), total, len(want), f.Codes)
	}
	if c, _ := codeNamed(f, raiseNotInTable); c.Threshold != 5 {
		t.Errorf("the quiet code's threshold is %d, want its own 5", c.Threshold)
	}
	if c, _ := codeNamed(f, errorcode.CodeMaxLoopReached); c.Threshold != 1 {
		t.Errorf("a fixed code's threshold is %d, want 1 whatever the workspace default", c.Threshold)
	}

	// The basis names every code and count, the quiet and debug ones included,
	// says the counts are a lower bound, and quotes no message.
	for _, w := range []string{"lower bound", raiseNotNumeric, `"` + raiseNotInTable + `" 2 stored rows, quiet: below its threshold of 5`,
		`"debug break" 4 stored rows, a debug raise`, errorcode.CodeMappingColumn, "uncatalogued",
		"12 of these rows naming their operator type"} {
		if !strings.Contains(f.Basis, w) {
			t.Errorf("the basis does not say %q:\n%s", w, f.Basis)
		}
	}
	for _, tail := range []string{"12.5", "X9", "member_dob", "bad date", unknown, "rule set main", " 17"} {
		if strings.Contains(f.Basis, tail) {
			t.Errorf("the basis quotes a message's text %q:\n%s", tail, f.Basis)
		}
	}
	if got := f.Reporting(); len(got) != 4 {
		t.Errorf("%d codes report, want 4: %+v", len(got), got)
	}
}

// A run whose codes are all below their thresholds, or debug, has no finding at
// the locus, and its basis still counts them.
func TestACodeBelowItsThresholdRaisesNothing(t *testing.T) {
	pool := freshDB(t, "triage_error_codes_quiet", append(slices.Clone(migratedTables), "error_code_catalogue"))
	catalogue(t, pool)
	now := time.Now().UTC()
	k := header(t, pool, "client2", "loader", "claims", "s-quiet", "completed", now.Add(-time.Hour))
	workerRow(t, pool, k, "client2", "loader", "s-quiet", "reducing01", 0, "completed", 1000, 1000, "", now.Add(-time.Hour))
	plantErrors(t, pool, k, "s-quiet", []codedError{
		{errorcode.OperatorJetrules, errorcode.RuleExceptionPrefix + raiseNotInTable + ": X9", 4},
		{errorcode.OperatorJetrules, errorcode.RuleExceptionPrefix + raiseDebug + " 1", 30},
	})
	r := classify(t, pool, "s-quiet", 0)
	assertAllValid(t, r)
	f := wantVerdict(t, r, LocusPerRecordFailuresReported, Absent)
	if len(f.Codes) != 2 || len(f.Reporting()) != 0 {
		t.Errorf("codes %+v; want two, neither reporting", f.Codes)
	}
	for _, w := range []string{"None reports", "quiet: below its threshold of 5", "a debug raise, counted and never reported",
		"lower bound"} {
		if !strings.Contains(f.Basis, w) {
			t.Errorf("the basis does not say %q:\n%s", w, f.Basis)
		}
	}
}

// Without the catalogue table every workspace message reads uncatalogued, at
// threshold 1, the fixed codes still apply, and the basis says so.
func TestAMissingCatalogueIsAnEmptyOne(t *testing.T) {
	pool := freshDB(t, "triage_error_codes_nocat", migratedTables)
	now := time.Now().UTC()
	k := header(t, pool, "client2", "loader", "claims", "s-nocat", "completed", now.Add(-time.Hour))
	workerRow(t, pool, k, "client2", "loader", "s-nocat", "reducing01", 0, "completed", 1000, 1000, "", now.Add(-time.Hour))
	plantErrors(t, pool, k, "s-nocat", []codedError{
		{errorcode.OperatorJetrules, errorcode.RuleExceptionPrefix + raiseNotInTable + ": X9", 1},
		{errorcode.OperatorMapRecord, "while casting value to rdf type date", 1},
	})
	r := classify(t, pool, "s-nocat", 0)
	f := wantVerdict(t, r, LocusPerRecordFailuresReported, Present)
	if _, ok := codeNamed(f, errorcode.UncataloguedCode("jetrules")); !ok {
		t.Errorf("a workspace raise with no catalogue does not read uncatalogued: %+v", f.Codes)
	}
	if _, ok := codeNamed(f, errorcode.CodeCastToRdfType); !ok {
		t.Errorf("a fixed message does not keep its fixed code with no catalogue: %+v", f.Codes)
	}
	if !strings.Contains(f.Basis, "catalogue (jetsapi.error_code_catalogue) is absent") {
		t.Errorf("the basis does not say the catalogue is absent:\n%s", f.Basis)
	}
}
