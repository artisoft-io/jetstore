package rulestest

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/artisoft-io/jetstore/jets/wsvalidate"
)

// validDefinition is the input document's example, widened to every form the
// format has: per-session checks, a pattern, a count bound by a pattern, typed
// and bare objects, legacy, expect and rule_config.
const validDefinition = `{
  "test_name": "looping test 1",
  "comment": "a valid definition using every form",
  "legacy": {"recompute_on_retraction": false},
  "expect": {"status": "error", "reason": "max vertex visit reached"},
  "jetrules_config": {
    "comment": "This is a compute_pipes.JetrulesSpec entity",
    "process_name": "jet_rules/unit_tests/looping_test/looping_test1.jr",
    "max_looping": 20,
    "is_debug": true,
    "input_rdf_type": "hc:Eligibility",
    "current_source_period": 1,
    "current_source_period_type": "month_period",
    "rule_config": [
      {"jets:key": "jets:iState", "jets:max_vertex_visits": {"value": "50", "type": "int"}, "_0:no_truth_main_on_exist": {"value": "1", "type": "int"}}
    ]
  },
  "input_sessions": [
    {
      "name": "session1",
      "comment": "input in the same directory",
      "input": ["looping_input.csv"],
      "expected_triples": [
        {"s": "jets:iState", "p": "jets:completed", "o": {"value": "1", "type": "int"}}
      ],
      "excluded_triples": [
        {"s": "b10305943c9a5e5e", "p": "hc:Plan_Type", "o": "?any"}
      ]
    },
    {"name": "session2", "input": ["sub/other.csv"]}
  ],
  "expected_triples": [
    {"pattern": [
      {"s": "?p", "p": "rdf:type", "o": {"value": "lp:Person", "type": "resource"}},
      {"s": "?p", "p": "lp:name", "o": "person_2"}
    ]},
    {"pattern": [{"s": "?p", "p": "rdf:type", "o": {"value": "lp:Person", "type": "resource"}}],
     "s": "?p", "p": "lp:name",
     "count": {"lhs": {"type": "select", "expr": "count"}, "op": "==", "rhs": {"type": "value", "expr": "1"}}},
    {"s": "jets:iState", "p": "_0:mergeList",
     "count": {"lhs": {"type": "select", "expr": "count"}, "op": ">=", "rhs": {"type": "value", "expr": "2"}}}
  ],
  "excluded_triples": [
    {"s": "jets:iState", "p": "jets:exception", "o": "?e"}
  ]
}`

func parse(t *testing.T, content string) (*TestDefinition, []wsvalidate.Finding) {
	t.Helper()
	return ParseTestDefinition([]byte(content))
}

// mutate decodes validDefinition, applies f to the generic document, and
// re-encodes it, so each case states only what it breaks.
func mutate(t *testing.T, f func(doc map[string]any)) string {
	t.Helper()
	var doc map[string]any
	if err := json.Unmarshal([]byte(validDefinition), &doc); err != nil {
		t.Fatal(err)
	}
	f(doc)
	b, err := json.Marshal(doc)
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

func obj(doc map[string]any, keys ...any) map[string]any {
	var cur any = doc
	for _, k := range keys {
		switch kk := k.(type) {
		case string:
			cur = cur.(map[string]any)[kk]
		case int:
			cur = cur.([]any)[kk]
		}
	}
	return cur.(map[string]any)
}

func requireFinding(t *testing.T, findings []wsvalidate.Finding, code, path, msgPart string) {
	t.Helper()
	for _, f := range findings {
		if f.Code == code && f.Path == path && strings.Contains(f.Message, msgPart) {
			if f.Severity != wsvalidate.Error {
				t.Errorf("finding %s at %s is %s, want error", code, path, f.Severity)
			}
			return
		}
	}
	t.Errorf("no finding %s at %q containing %q; got %+v", code, path, msgPart, findings)
}

func TestValidDefinition(t *testing.T) {
	td, findings := parse(t, validDefinition)
	if len(findings) != 0 {
		t.Fatalf("unexpected findings: %+v", findings)
	}
	if td.TestName != "looping test 1" || td.JetrulesConfig.ProcessName != "jet_rules/unit_tests/looping_test/looping_test1.jr" {
		t.Errorf("decoded %q / %q", td.TestName, td.JetrulesConfig.ProcessName)
	}
	if td.JetrulesConfig.MaxLooping != 20 || !td.JetrulesConfig.IsDebug || len(td.JetrulesConfig.RuleConfig) != 1 {
		t.Errorf("jetrules_config decoded as %+v", td.JetrulesConfig)
	}
	if got := td.LegacyMode(); got != LegacyModeFixed1 {
		t.Errorf("legacy mode %q, want %q", got, LegacyModeFixed1)
	}
	if td.Expect == nil || td.Expect.Status != StatusError {
		t.Errorf("expect decoded as %+v", td.Expect)
	}
	if len(td.InputSessions) != 2 || len(td.InputSessions[0].ExpectedTriples) != 1 || len(td.ExpectedTriples) != 3 {
		t.Fatalf("sessions/checks decoded as %+v", td)
	}
	o := td.InputSessions[0].ExcludedTriples[0].O
	if !o.IsVariable() || o.Variable != "?any" {
		t.Errorf("bare ?any decoded as %+v", o)
	}
	o = td.ExpectedTriples[0].Pattern[1].O
	if o.IsVariable() || o.Type != "text" || o.Value != "person_2" {
		t.Errorf("bare string decoded as %+v", o)
	}
	if td.ExpectedTriples[1].Kind(KindExpected) != KindCount || td.ExpectedTriples[0].Kind(KindExpected) != KindExpected {
		t.Errorf("kinds wrong")
	}
	if n := len(td.InputSessions[0].ExpectedTriples[0].Triples()); n != 1 {
		t.Errorf("a single triple is a pattern of one, got %d", n)
	}
	if td.ExpectedTriples[2].Triples() != nil {
		t.Errorf("a count without a pattern has no triples to match")
	}
}

// The input document's own example, verbatim.
func TestInputDocumentExample(t *testing.T) {
	_, findings := parse(t, `{
  "test_name" : "looping test 1",
  "jetrules_config": {
    "comment": "This is a compute_pipes.JetrulesSpec entity",
    "process_name": "jet_rules/unit_tests/looping_test/looping_test1.jr"
  },
  "input_sessions": [
    {
      "name": "session1",
      "comment": "This is an input session for looping test 1, file located in same directory is this file is in",
      "input": ["looping_input.csv"]
    }
  ],
  "expected_triples": [],
  "excluded_triples": []
}`)
	if len(findings) != 0 {
		t.Fatalf("unexpected findings: %+v", findings)
	}
}

// Criterion 5: an unknown key is refused with its path, at every depth.
func TestUnknownKeysRefusedWithPath(t *testing.T) {
	cases := []struct {
		name, path, hint string
		f                func(map[string]any)
	}{
		{"top level misspelt", "/test_nmae", `did you mean "test_name"`, func(d map[string]any) { d["test_nmae"] = "x" }},
		{"top level wrong case", "/Test_Name", `did you mean "test_name"`, func(d map[string]any) { d["Test_Name"] = "x" }},
		{"jetrules_config", "/jetrules_config/proces_name", `did you mean "process_name"`, func(d map[string]any) {
			obj(d, "jetrules_config")["proces_name"] = "x"
		}},
		{"jetrules_config wrong case", "/jetrules_config/Max_Looping", `did you mean "max_looping"`, func(d map[string]any) {
			obj(d, "jetrules_config")["Max_Looping"] = 3
		}},
		{"input session", "/input_sessions/1/inputs", `did you mean "input"`, func(d map[string]any) {
			obj(d, "input_sessions", 1)["inputs"] = []any{"a.csv"}
		}},
		{"legacy", "/legacy/withdrawn_row_stay", `did you mean "withdrawn_row_stays"`, func(d map[string]any) {
			obj(d, "legacy")["withdrawn_row_stay"] = true
		}},
		{"check", "/expected_triples/0/patern", `did you mean "pattern"`, func(d map[string]any) {
			obj(d, "expected_triples", 0)["patern"] = []any{}
		}},
		{"pattern triple", "/expected_triples/0/pattern/1/obj", "", func(d map[string]any) {
			obj(d, "expected_triples", 0, "pattern", 1)["obj"] = "x"
		}},
		{"object term", "/input_sessions/0/expected_triples/0/o/typ", `did you mean "type"`, func(d map[string]any) {
			obj(d, "input_sessions", 0, "expected_triples", 0, "o")["typ"] = "int"
		}},
		{"count expression", "/expected_triples/2/count/lhs/exp", `did you mean "expr"`, func(d map[string]any) {
			obj(d, "expected_triples", 2, "count", "lhs")["exp"] = "count"
		}},
		{"expect", "/expect/reasons", `did you mean "reason"`, func(d map[string]any) { obj(d, "expect")["reasons"] = "x" }},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			td, findings := parse(t, mutate(t, c.f))
			if td != nil {
				t.Errorf("a definition with an unknown key decoded")
			}
			requireFinding(t, findings, CodeUnknownField, c.path, c.hint)
		})
	}
}

// Criterion 5: a pipeline-only field is refused with its path (definition §3.2).
func TestPipelineOnlyFieldsRefused(t *testing.T) {
	fields := map[string]any{
		"pool_size": 2, "max_input_count": 10, "max_rete_sessions_saved": 1,
		"output_channels": []any{}, "error_channel": map[string]any{"name": "x"},
		"on_error": "drop", "max_error_count": 5,
		"metadata_input_sources": []any{},
	}
	for name, value := range fields {
		t.Run(name, func(t *testing.T) {
			_, findings := parse(t, mutate(t, func(d map[string]any) { obj(d, "jetrules_config")[name] = value }))
			requireFinding(t, findings, CodeRefused, "/jetrules_config/"+name, name)
		})
	}
	// Every refused name is one JetrulesConfig does not declare, and vice versa.
	used := map[string]bool{}
	for _, n := range UsedJetrulesSpecFields() {
		used[n] = true
	}
	for _, n := range RefusedJetrulesSpecFields() {
		if used[n] {
			t.Errorf("%s is both used and refused", n)
		}
		if _, ok := fields[n]; !ok {
			t.Errorf("refused field %s has no case in this test", n)
		}
	}
}

// Criterion 5: legacy, compare_engines and expect below the top level are
// refused with the path, naming where they belong.
func TestTopLevelFieldsMisplaced(t *testing.T) {
	cases := []struct {
		path string
		f    func(map[string]any)
	}{
		{"/jetrules_config/legacy", func(d map[string]any) {
			obj(d, "jetrules_config")["legacy"] = map[string]any{"withdrawn_row_stays": false}
		}},
		{"/input_sessions/0/legacy", func(d map[string]any) {
			obj(d, "input_sessions", 0)["legacy"] = map[string]any{"withdrawn_row_stays": false}
		}},
		{"/jetrules_config/compare_engines", func(d map[string]any) { obj(d, "jetrules_config")["compare_engines"] = true }},
		{"/input_sessions/1/compare_engines", func(d map[string]any) { obj(d, "input_sessions", 1)["compare_engines"] = true }},
		{"/input_sessions/0/expect", func(d map[string]any) {
			obj(d, "input_sessions", 0)["expect"] = map[string]any{"status": "error"}
		}},
		{"/expected_triples/0/legacy", func(d map[string]any) { obj(d, "expected_triples", 0)["legacy"] = map[string]any{} }},
	}
	for _, c := range cases {
		t.Run(c.path, func(t *testing.T) {
			_, findings := parse(t, mutate(t, c.f))
			requireFinding(t, findings, CodeMisplaced, c.path, "top-level field")
		})
	}
}

func TestEngineChoice(t *testing.T) {
	_, findings := parse(t, mutate(t, func(d map[string]any) {
		obj(d, "jetrules_config")["use_jet_rules_native"] = true
		obj(d, "jetrules_config")["use_jet_rules_go"] = true
	}))
	requireFinding(t, findings, CodeConflict, "/jetrules_config/use_jet_rules_go", "both set")

	_, findings = parse(t, mutate(t, func(d map[string]any) {
		d["compare_engines"] = true
		obj(d, "jetrules_config")["use_jet_rules_native"] = true
	}))
	requireFinding(t, findings, CodeConflict, "/compare_engines", "runs both engines")

	// _0:no_truth_main_on_exist is accepted on C++ (validDefinition sets it) and
	// refused when the test runs Go, alone or compared.
	noTruth := "/jetrules_config/rule_config/0/_0:no_truth_main_on_exist"
	_, findings = parse(t, mutate(t, func(d map[string]any) { obj(d, "jetrules_config")["use_jet_rules_go"] = true }))
	requireFinding(t, findings, CodeRefused, noTruth, "only in the C++ engine")
	_, findings = parse(t, mutate(t, func(d map[string]any) { d["compare_engines"] = true }))
	requireFinding(t, findings, CodeRefused, noTruth, "only in the C++ engine")
	td, findings := parse(t, mutate(t, func(d map[string]any) { obj(d, "jetrules_config")["use_jet_rules_native"] = true }))
	if len(findings) != 0 || !td.JetrulesConfig.UseJetRulesNative {
		t.Errorf("use_jet_rules_native alone: %+v", findings)
	}
}

// The strict decode is the backstop for what the key walk leaves to it: types.
func TestTypeMismatch(t *testing.T) {
	_, findings := parse(t, mutate(t, func(d map[string]any) { d["compare_engines"] = "yes" }))
	requireFinding(t, findings, CodeInvalidValue, "/compare_engines", "boolean")
	_, findings = parse(t, mutate(t, func(d map[string]any) { obj(d, "jetrules_config")["max_looping"] = "20" }))
	requireFinding(t, findings, CodeInvalidValue, "/jetrules_config/max_looping", "number")
	_, findings = parse(t, `{"test_name": "x",`)
	requireFinding(t, findings, CodeInvalidJSON, "", "not valid JSON")
	_, findings = parse(t, `[1]`)
	requireFinding(t, findings, CodeInvalidJSON, "", "JSON object")
}

func TestRequiredAndConflicts(t *testing.T) {
	cases := []struct {
		code, path, msg string
		f               func(map[string]any)
	}{
		{CodeRequired, "/test_name", "test_name", func(d map[string]any) { delete(d, "test_name") }},
		{CodeRequired, "/jetrules_config", "jetrules_config", func(d map[string]any) { delete(d, "jetrules_config") }},
		{CodeRequired, "/jetrules_config/process_name", "process_name", func(d map[string]any) {
			delete(obj(d, "jetrules_config"), "process_name")
		}},
		{CodeRequired, "/input_sessions", "input session", func(d map[string]any) { d["input_sessions"] = []any{} }},
		{CodeRequired, "/input_sessions/1/name", "name", func(d map[string]any) { obj(d, "input_sessions", 1)["name"] = "" }},
		{CodeConflict, "/input_sessions/1/name", "already used", func(d map[string]any) { obj(d, "input_sessions", 1)["name"] = "session1" }},
		{CodeRequired, "/input_sessions/1/input", "CSV", func(d map[string]any) { obj(d, "input_sessions", 1)["input"] = []any{} }},
		{CodeInvalidValue, "/input_sessions/1/input/0", "leaves", func(d map[string]any) { obj(d, "input_sessions", 1)["input"] = []any{"../x.csv"} }},
		{CodeInvalidValue, "/input_sessions/1/input/0", "absolute", func(d map[string]any) { obj(d, "input_sessions", 1)["input"] = []any{"/tmp/x.csv"} }},
		{CodeInvalidValue, "/expect/status", "error", func(d map[string]any) { obj(d, "expect")["status"] = "pass" }},
		{CodeRequired, "/jetrules_config/current_source_period_type", "reset to 0", func(d map[string]any) {
			delete(obj(d, "jetrules_config"), "current_source_period_type")
		}},
		{CodeInvalidValue, "/jetrules_config/max_looping", "negative", func(d map[string]any) { obj(d, "jetrules_config")["max_looping"] = -1 }},
		{CodeInvalidValue, "/jetrules_config/rule_config/0/jets:max_vertex_visits/value", "JSON string", func(d map[string]any) {
			obj(d, "jetrules_config", "rule_config", 0, "jets:max_vertex_visits")["value"] = 50
		}},
		{CodeInvalidValue, "/jetrules_config/rule_config/0/jets:max_vertex_visits/type", "unknown type", func(d map[string]any) {
			obj(d, "jetrules_config", "rule_config", 0, "jets:max_vertex_visits")["type"] = "integer"
		}},
		{CodeInvalidValue, "/jetrules_config/rule_config/0/jets:key", "string (text)", func(d map[string]any) {
			obj(d, "jetrules_config", "rule_config", 0)["jets:key"] = 3
		}},
	}
	for _, c := range cases {
		t.Run(c.path+" "+c.msg, func(t *testing.T) {
			_, findings := parse(t, mutate(t, c.f))
			requireFinding(t, findings, c.code, c.path, c.msg)
		})
	}
}

func TestCheckForms(t *testing.T) {
	count := map[string]any{"lhs": map[string]any{"type": "select", "expr": "count"}, "op": "==", "rhs": map[string]any{"type": "value", "expr": "1"}}
	cases := []struct {
		code, path, msg string
		check           map[string]any
		excluded        bool
	}{
		{CodeRequired, "/expected_triples/0", "a check is", map[string]any{}, false},
		{CodeRequired, "/expected_triples/0/o", "o is required", map[string]any{"s": "a:b", "p": "a:c"}, false},
		{CodeRequired, "/expected_triples/0/s", "s is required", map[string]any{"p": "a:c", "o": "x"}, false},
		{CodeInvalidValue, "/expected_triples/0/s", "not a variable name", map[string]any{"s": "?1x", "p": "a:c", "o": "x"}, false},
		{CodeInvalidValue, "/expected_triples/0/o", "not a variable name", map[string]any{"s": "a:b", "p": "a:c", "o": "?"}, false},
		{CodeConflict, "/expected_triples/0/o", "does not go with pattern", map[string]any{
			"pattern": []any{map[string]any{"s": "?x", "p": "a:c", "o": "y"}}, "o": "z"}, false},
		{CodeRequired, "/expected_triples/0/pattern/0/o", "o is required", map[string]any{
			"pattern": []any{map[string]any{"s": "?x", "p": "a:c"}}}, false},
		{CodeInvalidValue, "/excluded_triples/0/count", "belongs in expected_triples", map[string]any{
			"s": "a:b", "p": "a:c", "count": count}, true},
		{CodeConflict, "/expected_triples/0/o", "not o", map[string]any{"s": "a:b", "p": "a:c", "o": "x", "count": count}, false},
		{CodeInvalidValue, "/expected_triples/0/s", "must be bound", map[string]any{"s": "?p", "p": "a:c", "count": count}, false},
		{CodeInvalidValue, "/expected_triples/0/o", "a string (text, or a ?variable)", map[string]any{"s": "a:b", "p": "a:c", "o": 3}, false},
	}
	for _, c := range cases {
		t.Run(c.path+" "+c.msg, func(t *testing.T) {
			content := mutate(t, func(d map[string]any) {
				d["expected_triples"], d["excluded_triples"] = []any{}, []any{}
				if c.excluded {
					d["excluded_triples"] = []any{c.check}
				} else {
					d["expected_triples"] = []any{c.check}
				}
			})
			_, findings := parse(t, content)
			requireFinding(t, findings, c.code, c.path, c.msg)
		})
	}
}

func TestLoadTestDefinitionChecksInputFiles(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "looping_test1.tst.json")
	if err := os.WriteFile(path, []byte(validDefinition), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "looping_input.csv"), []byte("jets:key\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(filepath.Join(dir, "sub"), 0o755); err != nil {
		t.Fatal(err)
	}
	td, findings, err := LoadTestDefinition(path)
	if err != nil || td == nil {
		t.Fatalf("LoadTestDefinition: %v", err)
	}
	requireFinding(t, findings, CodeMissingFile, "/input_sessions/1/input/0", "sub/other.csv")
	if len(findings) != 1 || !HasErrors(findings) {
		t.Errorf("want exactly the missing file, got %+v", findings)
	}
	if err := os.WriteFile(filepath.Join(dir, "sub", "other.csv"), []byte("jets:key\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, findings, _ = LoadTestDefinition(path); len(findings) != 0 {
		t.Errorf("with both files present: %+v", findings)
	}
	if _, _, err := LoadTestDefinition(filepath.Join(dir, "absent.tst.json")); err == nil {
		t.Errorf("an unreadable file is an error, not a finding")
	}
	// An invalid definition is returned with its findings when it decodes, so
	// the compile can record its name.
	if err := os.WriteFile(path, []byte(mutate(t, func(d map[string]any) { obj(d, "jetrules_config")["use_jet_rules_go"] = true })), 0o644); err != nil {
		t.Fatal(err)
	}
	td, findings, _ = LoadTestDefinition(path)
	if td == nil || td.TestName != "looping test 1" || !HasErrors(findings) {
		t.Errorf("an invalid but decodable definition: td=%v findings=%+v", td, findings)
	}
}

func TestValidateContentIsAValidator(t *testing.T) {
	var v wsvalidate.Validator = ValidateContent
	if f := v(validDefinition); len(f) != 0 {
		t.Errorf("valid: %+v", f)
	}
	if f := v(`{"test_name": 1}`); !HasErrors(f) {
		t.Errorf("invalid accepted")
	}
}

func TestLegacyModeLabels(t *testing.T) {
	f, tr := false, true
	cases := []struct {
		l    *LegacyConfig
		want string
	}{
		{nil, LegacyModeLegacy},
		{&LegacyConfig{}, LegacyModeLegacy},
		{&LegacyConfig{RecomputeOnRetraction: &tr, WithdrawnRowStays: &tr}, LegacyModeLegacy},
		{&LegacyConfig{RecomputeOnRetraction: &f, WithdrawnRowStays: &f}, LegacyModeFixed},
		{&LegacyConfig{RecomputeOnRetraction: &f}, LegacyModeFixed1},
		{&LegacyConfig{WithdrawnRowStays: &f}, LegacyModeFixed2},
	}
	for _, c := range cases {
		if got := c.l.ModeLabel(); got != c.want {
			t.Errorf("%+v: %q, want %q", c.l, got, c.want)
		}
	}
	var td *TestDefinition
	if td.LegacyMode() != LegacyModeLegacy {
		t.Errorf("nil definition is legacy")
	}
}

func TestObjectTermRoundTrip(t *testing.T) {
	for _, in := range []string{`"?x"`, `"plain text"`, `{"value":"1","type":"int"}`, `{"value":"t","type":"text"}`} {
		var o ObjectTerm
		if err := json.Unmarshal([]byte(in), &o); err != nil {
			t.Fatal(err)
		}
		out, err := json.Marshal(o)
		if err != nil {
			t.Fatal(err)
		}
		if string(out) != in {
			t.Errorf("%s round-tripped to %s", in, out)
		}
	}
	c := Check{Comment: "dropped", S: "jets:iState", P: "jets:completed", O: &ObjectTerm{Value: "1", Type: "int", typed: true}}
	if got := c.String(); got != `{"s":"jets:iState","p":"jets:completed","o":{"value":"1","type":"int"}}` {
		t.Errorf("Check.String: %s", got)
	}
}

func TestResultFileRoundTrip(t *testing.T) {
	path := filepath.Join(t.TempDir(), "result.json")
	r := &Result{
		TestPath: "jet_rules/unit_tests/x/x.tst.json", TestName: "x", Status: StatusFail, Reason: "1 of 2 checks failed",
		Engine: EngineNative, LegacyMode: LegacyModeLegacy, LoadMs: 12, RunMs: 3, DurationMs: 20, Sessions: 1, Loops: 2,
		Checks: []CheckResult{{SessionName: "s1", Kind: KindExpected, Check: `{"s":"a:b","p":"a:c","o":"x"}`, Outcome: OutcomeFail, Found: "no match"}},
	}
	if err := WriteResultFile(path, r); err != nil {
		t.Fatal(err)
	}
	got, err := ReadResultFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if got.Status != r.Status || got.Checks[0].Found != "no match" || got.LoadMs != 12 {
		t.Errorf("round trip: %+v", got)
	}
	// The JSON names are the column names.
	b, _ := os.ReadFile(path)
	for _, col := range []string{`"status"`, `"reason"`, `"engine"`, `"legacy_mode"`, `"load_ms"`, `"duration_ms"`,
		`"sessions"`, `"loops"`, `"session_name"`, `"kind"`, `"check"`, `"outcome"`, `"found"`} {
		if !strings.Contains(string(b), col) {
			t.Errorf("result file lacks %s", col)
		}
	}
	if entries, _ := os.ReadDir(filepath.Dir(path)); len(entries) != 1 {
		t.Errorf("a temporary file was left beside the result: %v", entries)
	}
	if err := os.WriteFile(path, []byte(`{"status":"executing"}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := ReadResultFile(path); err == nil {
		t.Errorf("a non-terminal status was accepted")
	}
}

func TestClosestName(t *testing.T) {
	names := []string{"hc:Member_ID", "hc:Member_DOB", "hc:Plan_Type", "jets:key"}
	for in, want := range map[string]string{
		"hc:member_id": "hc:Member_ID",
		"hc:Member_Id": "hc:Member_ID",
		"hc:Plan_Typ":  "hc:Plan_Type",
		"hc:Plan_Tpye": "hc:Plan_Type",
		"jets:kye":     "jets:key",
		"zzz":          "",
	} {
		if got := ClosestName(in, names); got != want {
			t.Errorf("ClosestName(%q) = %q, want %q", in, got, want)
		}
	}
}
