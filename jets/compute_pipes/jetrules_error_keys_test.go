package compute_pipes_test

// The keys on a jetrules error row: pipeline_execution_status_key, grouping_key,
// row_jets_key and input_column, at each of the four sites that write one, and
// the report of a text property with several values.
//
// The cases run in the child process of TestExecuteRulesCharacterisation, which
// calls runErrorKeysCases once it has compiled the scratch test_ws (see that
// file's header for why it has to be a child process). Run it with -count=1.

import (
	"database/sql"
	"fmt"
	"slices"
	"testing"

	"github.com/artisoft-io/jetstore/jets/compute_pipes"
)

// The run the error-keys cases report into, as NewProcessError reads it.
const (
	errorKeysPEKey     = 4242
	errorKeysSessionId = "error-keys-session"
	groupedChannel     = "cases.grouped"
)

// errorKeysHarness is a worker whose source is written by a group_by on the
// domain key "Case", whose key is xr:group. The class has a second domain key,
// so the group_by is what picks the grouping key, not the class.
func errorKeysHarness(t *testing.T, config *compute_pipes.JetrulesSpec) *harness {
	t.Helper()
	cpConfig := &compute_pipes.ComputePipesConfig{
		PipesConfig: []compute_pipes.PipeSpec{{
			Type: "fan_out",
			Apply: []compute_pipes.TransformationSpec{{
				Type:          "group_by",
				GroupByConfig: &compute_pipes.GroupBySpec{DomainKey: "Case"},
				OutputChannel: compute_pipes.OutputChannelConfig{Name: groupedChannel},
			}},
		}},
	}
	columns := make(map[string]int)
	for i, c := range caseInputColumns {
		columns[c] = i
	}
	source := &compute_pipes.InputChannel{
		Name:           groupedChannel,
		HasGroupedRows: true,
		Columns:        &columns,
		Config:         &compute_pipes.ChannelSpec{Name: groupedChannel, ClassName: "xr:Case", Columns: caseInputColumns},
		DomainKeySpec: &compute_pipes.DomainKeysSpec{
			HashingOverride: "none",
			DomainKeys: map[string]*compute_pipes.DomainKeyInfo{
				"Case":  {KeyExpr: []string{"xr:group"}, ObjectType: "Case"},
				"Other": {KeyExpr: []string{"xr:mode"}, ObjectType: "Other"},
			},
		},
	}
	return newHarnessWith(t, executeRulesRuleFile, config,
		compute_pipes.NewBuilderContextForTest(errorKeysPEKey, errorKeysSessionId, cpConfig), source)
}

// caseRow is an xr:Case input row laid out as caseInputColumns.
func caseRow(key, mode, group string) []any {
	return []any{key, []any{"xr:Case"}, nil, mode, group}
}

func nullString(s string) sql.NullString { return sql.NullString{String: s, Valid: true} }

// assertKeys checks the keys of one error row.
func assertKeys(t *testing.T, pe compute_pipes.ProcessError, grouping, rowJetsKey, inputColumn sql.NullString) {
	t.Helper()
	if pe.PEKey != errorKeysPEKey {
		t.Errorf("pipeline_execution_status_key = %d, want %d, the run's key", pe.PEKey, errorKeysPEKey)
	}
	if pe.GroupingKey != grouping {
		t.Errorf("grouping_key = %+v, want %+v", pe.GroupingKey, grouping)
	}
	if pe.RowJetsKey != rowJetsKey {
		t.Errorf("row_jets_key = %+v, want %+v", pe.RowJetsKey, rowJetsKey)
	}
	if pe.InputColumn != inputColumn {
		t.Errorf("input_column = %+v, want %+v", pe.InputColumn, inputColumn)
	}
}

func runErrorKeysCases(t *testing.T) {
	// The three rule-set sites, on a session of one input row: the grouping key is
	// the group's domain key and row_jets_key is the row's jets:key.
	for _, c := range []struct{ mode, message string }{
		{"exception", msgException},
		{"endless", msgMaxLoop("2")},
		{"runaway", msgExecuteRulesError},
	} {
		t.Run("error_keys_rule_set_site_"+c.mode, func(t *testing.T) {
			h := errorKeysHarness(t, &compute_pipes.JetrulesSpec{})
			r := h.executeBundle([]any{caseRow("case-1", c.mode, "G-1")})
			assertBundleKept(t, r)
			assertMessages(t, r, c.message)
			if len(r.errorRows) != 1 {
				t.FailNow()
			}
			assertKeys(t, r.errorRows[0], nullString("G-1"), nullString("case-1"), sql.NullString{})
		})
	}

	// A session of several rows: the grouping key is the group's, and a failure of
	// the session is about no one row, so row_jets_key is null.
	t.Run("error_keys_group_of_two_rows", func(t *testing.T) {
		h := errorKeysHarness(t, &compute_pipes.JetrulesSpec{})
		r := h.executeBundle([]any{caseRow("case-a", "exception", "G-2"), caseRow("case-b", "exception", "G-2")})
		assertBundleKept(t, r)
		assertMessages(t, r, msgException)
		if len(r.errorRows) != 1 {
			t.FailNow()
		}
		assertKeys(t, r.errorRows[0], nullString("G-2"), sql.NullString{}, sql.NullString{})
	})

	// Each session carries its own keys, not the previous one's.
	t.Run("error_keys_are_the_current_sessions", func(t *testing.T) {
		h := errorKeysHarness(t, &compute_pipes.JetrulesSpec{})
		h.executeBundle([]any{caseRow("case-1", "exception", "G-1")})
		r := h.executeBundle([]any{caseRow("case-2", "exception", "G-3")})
		if len(r.errorRows) != 1 {
			t.FailNow()
		}
		assertKeys(t, r.errorRows[0], nullString("G-3"), nullString("case-2"), sql.NullString{})
	})

	// The multi-value site, a numeric property: set to null and reported, as
	// before, now with the subject's key and the property in their columns.
	t.Run("error_keys_numeric_property_with_two_values", func(t *testing.T) {
		h := errorKeysHarness(t, &compute_pipes.JetrulesSpec{})
		r := h.executeBundle([]any{caseRow("case-n", "two_amounts", "G-4"), caseRow("case-o", "ok", "G-4")})
		assertBundleKept(t, r)
		assertMessages(t, r, "property xr:amount is not multi-value but has multiple values for subject case-n, setting value to null")
		if len(r.errorRows) != 1 {
			t.FailNow()
		}
		assertKeys(t, r.errorRows[0], nullString("G-4"), nullString("case-n"), nullString("xr:amount"))
		for _, row := range r.cases {
			if row[0] == "case-n" && row[3] != nil {
				t.Errorf("xr:amount = %v, want null", row[3])
			}
		}
	})

	// The multi-value site, a text property: still written as an array, and now
	// reported, saying so.
	t.Run("error_keys_text_property_with_two_values", func(t *testing.T) {
		h := errorKeysHarness(t, &compute_pipes.JetrulesSpec{})
		r := h.executeBundle([]any{caseRow("case-t", "two_texts", "G-5")})
		assertBundleKept(t, r)
		assertMessages(t, r, "property xr:result is not multi-value but has 2 text values for subject case-t, keeping all of them as an array")
		if len(r.errorRows) != 1 {
			t.FailNow()
		}
		assertKeys(t, r.errorRows[0], nullString("G-5"), nullString("case-t"), nullString("xr:result"))
		got, ok := caseResult(t, r).([]any)
		if !ok {
			t.Fatalf("xr:result = %v (%T), want the two values as an array", caseResult(t, r), caseResult(t, r))
		}
		values := make([]string, 0, len(got))
		for _, v := range got {
			values = append(values, fmt.Sprint(v))
		}
		slices.Sort(values)
		if !slices.Equal(values, []string{"first", "second"}) {
			t.Errorf("xr:result = %q, want first and second", values)
		}
	})

	// A text conflict counts toward max_error_count like every other error.
	t.Run("error_keys_text_conflict_counts_toward_max_error_count", func(t *testing.T) {
		h := errorKeysHarness(t, &compute_pipes.JetrulesSpec{MaxErrorCount: 1})
		first := h.executeBundle([]any{caseRow("case-t1", "two_texts", "G-6")})
		second := h.executeBundle([]any{caseRow("case-t2", "two_texts", "G-7")})
		if len(first.errorRows) != 1 || len(second.errorRows) != 0 {
			t.Errorf("reported %d then %d rows, want 1 then 0 with max_error_count 1",
				len(first.errorRows), len(second.errorRows))
		}
		if got := h.worker.ErrorCountForTest(); got != 2 {
			t.Errorf("errorCount = %d, want 2, counting the unreported conflict", got)
		}
		if _, ok := caseResult(t, second).([]any); !ok {
			t.Errorf("an unreported conflict still writes the array, got %v", caseResult(t, second))
		}
	})

	// Without a domain key to group by, the rows still carry the run's key and
	// row_jets_key, and no grouping key.
	t.Run("error_keys_without_a_domain_key", func(t *testing.T) {
		h := newHarnessWith(t, executeRulesRuleFile, &compute_pipes.JetrulesSpec{},
			compute_pipes.NewBuilderContextForTest(errorKeysPEKey, errorKeysSessionId, nil),
			&compute_pipes.InputChannel{Name: "input", HasGroupedRows: true})
		r := h.executeBundle([]any{caseRow("case-x", "exception", "G-8")})
		if len(r.errorRows) != 1 {
			t.FailNow()
		}
		assertKeys(t, r.errorRows[0], sql.NullString{}, nullString("case-x"), sql.NullString{})
	})
}
