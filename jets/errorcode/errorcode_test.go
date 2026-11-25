package errorcode

import (
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"testing"
)

func ptr(n int) *int { return &n }

func TestSlug(t *testing.T) {
	cases := []struct{ code, want string }{
		{"SKU not found", "sku_not_found"},
		{"jetrules: uncatalogued", "jetrules_uncatalogued"},
		{"  --Leading and trailing--  ", "leading_and_trailing"},
		{"a::b,,c  d", "a_b_c_d"},
		{"Region_Name not set", "region_name_not_set"},
		{"élan", "lan"},
		{"!!!", ""},
		{"", ""},
		{"x9 Y8", "x9_y8"},
		// cut at 48, and a cut that leaves a separator is trimmed
		{strings.Repeat("a", 60), strings.Repeat("a", 48)},
		{strings.Repeat("a", 47) + " bcd", strings.Repeat("a", 47)},
		{strings.Repeat("a", 46) + " bcd", strings.Repeat("a", 46) + "_b"},
	}
	for _, c := range cases {
		if got := Slug(c.code); got != c.want {
			t.Errorf("Slug(%q) = %q, want %q", c.code, got, c.want)
		}
		if len(Slug(c.code)) > MaxSlugLength {
			t.Errorf("Slug(%q) is longer than %d", c.code, MaxSlugLength)
		}
	}
}

func TestDefaultCode(t *testing.T) {
	cases := map[string]string{
		"error SKU not found in the price list, SKU: ": "error SKU not found in the price list, SKU",
		"Debugging order ":            "Debugging order",
		"Stopping on every session":   "Stopping on every session",
		"error unknown_region_code: ": "error unknown_region_code",
		"a, : ,":                      "a",
		"":                            "",
	}
	for in, want := range cases {
		if got := DefaultCode(in); got != want {
			t.Errorf("DefaultCode(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestStripPrefixAndInferOperator(t *testing.T) {
	cases := []struct {
		op, msg, body string
		stripped      bool
	}{
		{OperatorJetrules, "jets:exception caught: error X", "error X", true},
		{OperatorJetrules, "MAX LOOP REACHED, maxLooping is 3", "MAX LOOP REACHED, maxLooping is 3", false},
		{OperatorMapRecord, "map_record: while casting value", "while casting value", true},
		{OperatorMapRecord, "while casting value", "while casting value", false},
		{OperatorRender, "render operator (template 'x'): y", "render operator (template 'x'): y", false},
		{"", "jets:exception caught: error X", "jets:exception caught: error X", false},
	}
	for _, c := range cases {
		body, stripped := StripPrefix(c.op, c.msg)
		if body != c.body || stripped != c.stripped {
			t.Errorf("StripPrefix(%q, %q) = %q, %v; want %q, %v", c.op, c.msg, body, stripped, c.body, c.stripped)
		}
	}
	if InferOperator("jets:exception caught: x") != OperatorJetrules ||
		InferOperator("map_record: x") != OperatorMapRecord ||
		InferOperator("property p is not multi-value") != "" {
		t.Error("InferOperator does not read the fixed prefixes")
	}
}

// A workspace catalogue in the shape a base init script writes it.
func sampleRows() []Entry {
	return []Entry{
		{OperatorType: DefaultOperator, MatchPrefix: "", Code: "", Kind: KindError, ReportThreshold: ptr(10), Source: SourceRules},
		{OperatorType: OperatorJetrules, MatchPrefix: "error ", Code: "generic error", Source: SourceRules},
		{OperatorType: OperatorJetrules, MatchPrefix: "error SKU not found in the price list", Code: "SKU not found", Source: SourceRules, SourceRef: "a.jr R1"},
		{OperatorType: OperatorJetrules, MatchPrefix: "error unknown_region_code: ", Code: "unknown region code", ReportThreshold: ptr(1), Source: SourceRules},
		{OperatorType: OperatorJetrules, MatchPrefix: "Stopping on every session", Code: "debug stop on every session", Kind: KindDebug, Source: SourceRules},
		{OperatorType: OperatorJetrules, MatchPrefix: "Debugging order ", Source: SourceRules, Kind: KindDebug},
		{OperatorType: OperatorMapRecord, MatchPrefix: "Member id is required", Code: "member id missing", Source: SourcePipelineConfig, ReportThreshold: ptr(3)},
	}
}

func TestMatch(t *testing.T) {
	c := NewCatalogue(sampleRows())
	cases := []struct {
		name, op, msg string
		code          string
		threshold     int
		debug, uncat  bool
	}{
		{"longest prefix wins", OperatorJetrules,
			"jets:exception caught: error SKU not found in the price list, SKU: 123",
			"SKU not found", 10, false, false},
		{"shorter row when the longer does not match", OperatorJetrules,
			"jets:exception caught: error something else", "generic error", 10, false, false},
		{"own threshold beats the default row", OperatorJetrules,
			"jets:exception caught: error unknown_region_code: X1", "unknown region code", 1, false, false},
		{"debug kind", OperatorJetrules, "jets:exception caught: Stopping on every session",
			"debug stop on every session", 10, true, false},
		{"code defaults from the prefix", OperatorJetrules, "jets:exception caught: Debugging order 42",
			"Debugging order", 10, true, false},
		{"uncatalogued rule exception", OperatorJetrules, "jets:exception caught: Something new",
			"jetrules: uncatalogued", 1, false, true},
		{"null operator, inferred from the exception prefix", "",
			"jets:exception caught: error SKU not found in the price list, SKU: 9",
			"SKU not found", 10, false, false},
		{"null operator, uncatalogued exception", "", "jets:exception caught: zzz",
			"jetrules: uncatalogued", 1, false, true},
		{"null operator, no prefix, nothing matches", "", "something nobody writes",
			"unknown_operator: uncatalogued", 1, false, true},
		{"a rule row does not match a worker message", OperatorJetrules,
			"ExecuteRules returned error: boom", CodeExecuteRulesError, 1, false, false},
		{"null operator, worker message", "", "MAX LOOP REACHED, maxLooping is 5", CodeMaxLoopReached, 1, false, false},
		{"multiple text values", OperatorJetrules,
			"property ex:X is not multi-value but has 3 text values for subject _:b1, keeping all of them as an array",
			CodeMultipleTextValues, 1, false, false},
		{"multiple values set to null, null operator", "",
			"property ex:X is not multi-value but has multiple values for subject _:b1, setting value to null",
			CodeMultipleValuesNull, 1, false, false},
		{"engine exception beats a shorter rule row", OperatorJetrules,
			"jets:exception caught: error: a rule tested text for truth: 'maybe' is not a bool",
			CodeTextTestedForTruth, 1, false, false},
		{"map_record mapping column, old form", OperatorMapRecord,
			"error mapping column at output position 3: invalid date", CodeMappingColumn, 1, false, false},
		{"map_record mapping column, new form", OperatorMapRecord,
			"map_record: error mapping column dob at output position 3: invalid date", CodeMappingColumn, 1, false, false},
		{"configured err_msg, old form", OperatorMapRecord,
			"error mapping column at output position 12: Member id is required", "member id missing", 3, false, false},
		{"configured err_msg, new form", OperatorMapRecord,
			"map_record: error mapping column member_id at output position 12: Member id is required",
			"member id missing", 3, false, false},
		{"configured err_msg, null operator, new form", "",
			"map_record: error mapping column member_id at output position 12: Member id is required",
			"member id missing", 3, false, false},
		{"map_record cast, both forms", OperatorMapRecord,
			"map_record: while casting value to rdf type 'date': bad", CodeCastToRdfType, 1, false, false},
		{"map_record cast, old form, null operator", "",
			"while casting value to rdf type 'date': bad", CodeCastToRdfType, 1, false, false},
		{"case_expr wrapping a mapping column error", OperatorMapRecord,
			"map_record: while evaluating case_expr else clause: error mapping column at output position 0: x",
			CodeCaseExpr, 1, false, false},
		{"lookup key", OperatorMapRecord, "map_record: while making the lookup key: x", CodeLookupKey, 1, false, false},
		{"lookup row, old form", OperatorMapRecord, "while fetching the lookup row: x", CodeLookupRow, 1, false, false},
		{"map_record uncatalogued", OperatorMapRecord, "map_record: error mapColumnEval.update cannot have nil",
			"map_record: uncatalogued", 1, false, true},
		{"render", OperatorRender, "render operator (template 'note'): missing key", CodeRender, 1, false, false},
		{"inference error", OperatorOllama, "ollama operator (model m:7b): timeout", InferenceCode(OperatorOllama), 1, false, false},
		{"provenance findings", OperatorVllm,
			"vllm operator (model a/b): provenance schema 'brief': 2 findings", ProvenanceFindingsCode(OperatorVllm), 1, false, false},
		{"provenance not checked", OperatorEmbed,
			"embed operator (model e): the briefing could not be checked against provenance schema 'b': bad json",
			ProvenanceUncheckedCode(OperatorEmbed), 1, false, false},
		{"site operator", "my_site_op", "row rejected", "my_site_op: uncatalogued", 1, false, true},
		{"operator type is trimmed", " jetrules ", "jets:exception caught: error SKU not found in the price list",
			"SKU not found", 10, false, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			m := c.Match(tc.op, tc.msg)
			if m.Code() != tc.code || m.Threshold != tc.threshold || m.IsDebug() != tc.debug || m.Uncatalogued != tc.uncat {
				t.Errorf("Match(%q, %q) = code %q threshold %d debug %v uncatalogued %v; want %q %d %v %v",
					tc.op, tc.msg, m.Code(), m.Threshold, m.IsDebug(), m.Uncatalogued,
					tc.code, tc.threshold, tc.debug, tc.uncat)
			}
		})
	}
}

func TestMatchWithoutDefaultRow(t *testing.T) {
	rows := []Entry{{OperatorType: OperatorJetrules, MatchPrefix: "error ", Source: SourceRules}}
	m := MatchMessage(rows, OperatorJetrules, "jets:exception caught: error x")
	if m.Code() != "error" || m.Threshold != DefaultThreshold {
		t.Errorf("got %q at %d, want the default code at %d", m.Code(), m.Threshold, DefaultThreshold)
	}
	m = MatchMessage(nil, OperatorJetrules, "jets:exception caught: error x")
	if !m.Uncatalogued || m.Threshold != 1 {
		t.Errorf("an empty catalogue: got %+v", m)
	}
}

func TestOverrideOfAFixedCode(t *testing.T) {
	head := "property % is not multi-value but has multiple values for subject "
	rows := []Entry{
		{OperatorType: DefaultOperator, ReportThreshold: ptr(10), Source: SourceRules},
		{OperatorType: OperatorJetrules, MatchPrefix: head, Code: "ignored", ReportThreshold: ptr(50),
			Source: SourceJetStore, Description: "routine here"},
		{OperatorType: OperatorJetrules, MatchPrefix: "ExecuteRules returned error", Kind: KindDebug, Source: SourceJetStore},
	}
	c := NewCatalogue(rows)
	m := c.Match("", "property p is not multi-value but has multiple values for subject s, setting value to null")
	if m.Code() != CodeMultipleValuesNull || m.Threshold != 50 || !m.Overridden || m.Entry.Description != "routine here" {
		t.Errorf("override not applied: %+v", m)
	}
	m = c.Match(OperatorJetrules, "ExecuteRules returned error: x")
	if !m.IsDebug() || m.Threshold != 1 || !m.Overridden {
		t.Errorf("a kind override keeps threshold 1, not the workspace default: %+v", m)
	}
	// Without the override the fixed code is at 1 whatever the default row says
	m = c.Match(OperatorJetrules, "MAX LOOP REACHED, maxLooping is 1")
	if m.Threshold != 1 || m.Overridden {
		t.Errorf("fixed code: %+v", m)
	}
	// The override does not leak into the shared fixed list
	for _, f := range Fixed() {
		if f.ReportThreshold != nil {
			t.Errorf("Fixed() was changed by an override: %+v", f)
		}
	}
}

func TestADefaultRowMatchesNothing(t *testing.T) {
	rows := []Entry{
		{OperatorType: DefaultOperator, ReportThreshold: ptr(7), Source: SourceRules},
		{OperatorType: DefaultOperator, MatchPrefix: "error", Source: SourceRules},
	}
	c := NewCatalogue(rows)
	if th, ok := c.DefaultRowThreshold(); !ok || th != 7 {
		t.Errorf("default threshold %d %v", th, ok)
	}
	if m := c.Match("", "error x"); !m.Uncatalogued {
		t.Errorf("a '*' row matched: %+v", m)
	}
}

func TestAnOperatorCatchAllRow(t *testing.T) {
	c := NewCatalogue([]Entry{{OperatorType: "my_site_op", MatchPrefix: "", Code: "site failure", Source: SourcePipelineConfig}})
	if m := c.Match("my_site_op", "anything"); m.Code() != "site failure" {
		t.Errorf("an empty prefix is a catch-all for its operator: %+v", m)
	}
	if m := c.Match("my_site_op", "anything"); m.Threshold != 1 {
		t.Errorf("threshold %d", m.Threshold)
	}
}

// Every fixed entry has a code, a kind, a source, a key of its own and a slug
// of its own, and its uncatalogued sibling's slug differs from all of them.
func TestFixedEntriesAreDistinct(t *testing.T) {
	keys := map[string]bool{}
	slugs := map[string]string{}
	for _, f := range Fixed() {
		if f.Code == "" || f.Kind != KindError || f.Source != SourceJetStore || f.SourceRef == "" || f.Description == "" {
			t.Errorf("incomplete fixed entry %+v", f)
		}
		if keys[f.Key()] {
			t.Errorf("duplicate key %q", f.Key())
		}
		keys[f.Key()] = true
		for _, code := range []string{f.Code, UncataloguedCode(f.OperatorType)} {
			s := Slug(code)
			if s == "" {
				t.Errorf("code %q has no slug", code)
			}
			if other, ok := slugs[s]; ok && other != code {
				t.Errorf("codes %q and %q share the slug %q", code, other, s)
			}
			slugs[s] = code
		}
	}
}

// fakeRows is a Rows over values, scanned the way database/sql scans them.
type fakeRows struct {
	data [][]any
	i    int
	err  error
}

func (f *fakeRows) Next() bool { f.i++; return f.i <= len(f.data) }
func (f *fakeRows) Err() error { return f.err }
func (f *fakeRows) Scan(dest ...any) error {
	row := f.data[f.i-1]
	if len(dest) != len(row) {
		return fmt.Errorf("scan: %d destinations for %d columns", len(dest), len(row))
	}
	for i, d := range dest {
		if s, ok := d.(sql.Scanner); ok {
			if err := s.Scan(row[i]); err != nil {
				return err
			}
			continue
		}
		return fmt.Errorf("destination %d is not a scanner", i)
	}
	return nil
}

func TestReadCatalogue(t *testing.T) {
	rows := &fakeRows{data: [][]any{
		{"*", "", "", "error", int64(10), "rules", nil, nil},
		{"jetrules", "error SKU", "SKU", "error", nil, "rules", "a.jr R1", "desc"},
	}}
	got, err := ReadCatalogue(rows)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 || got[0].ReportThreshold == nil || *got[0].ReportThreshold != 10 ||
		got[1].ReportThreshold != nil || got[1].SourceRef != "a.jr R1" || got[1].Description != "desc" {
		t.Errorf("ReadCatalogue = %+v", got)
	}
	if len(Columns) != 8 {
		t.Errorf("Columns has %d", len(Columns))
	}
	for _, col := range Columns {
		if !strings.Contains(SelectCatalogueSQL, col) {
			t.Errorf("SelectCatalogueSQL does not read %s", col)
		}
	}
	if _, err := ReadCatalogue(&fakeRows{err: errors.New("boom")}); err == nil {
		t.Error("Err() not returned")
	}
}

type sqlStateErr string

func (e sqlStateErr) Error() string    { return "pg error " + string(e) }
func (e sqlStateErr) SQLState() string { return string(e) }

func TestIsMissingTable(t *testing.T) {
	if !IsMissingTable(fmt.Errorf("reading: %w", sqlStateErr("42P01"))) {
		t.Error("42P01 wrapped is a missing table")
	}
	if IsMissingTable(sqlStateErr("42703")) || IsMissingTable(errors.New("x")) || IsMissingTable(nil) {
		t.Error("other errors are not a missing table")
	}
}
