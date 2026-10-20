package rulesbench

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/artisoft-io/jetstore/jets/compute_pipes"
	"github.com/artisoft-io/jetstore/jets/rulestest"
)

const minimalSpec = `{
  "bench_name": "b",
  "jetrules_config": {"process_name": "p.jr"},
  "input_sessions": [{"name": "m1", "input": ["m1.csv"]}],
  "max_vertex_visits": 100
}`

func TestParseSpec(t *testing.T) {
	spec, err := ParseSpec([]byte(minimalSpec))
	if err != nil {
		t.Fatal(err)
	}
	if spec.Name() != "b" || !slices.Equal(spec.EnginesOrDefault(), []string{"native", "go"}) {
		t.Errorf("name %q, engines %v; want b, native then go", spec.Name(), spec.EnginesOrDefault())
	}

	// A .tst.json, expectations and all, is a spec once a cap is given.
	tst := `{
	  "test_name": "t", "comment": "c",
	  "legacy": {"recompute_on_retraction": true},
	  "compare_engines": true,
	  "expect": {"status": "error"},
	  "jetrules_config": {"process_name": "p.jr", "rule_config": [{"jets:key": "jets:iState"}]},
	  "input_sessions": [{"name": "m1", "input": ["m1.csv"],
	    "expected_triples": [{"s": "a", "p": "b", "o": "c"}], "excluded_triples": []}],
	  "expected_triples": [], "excluded_triples": []
	}`
	if spec, err := ParseSpec([]byte(tst)); err != nil || spec.Name() != "t" {
		t.Errorf("a .tst.json: %v, name %q", err, spec.Name())
	}

	for _, c := range []struct{ name, spec, want string }{
		{"unknown_key", `{"bench_name":"b","jetrules_config":{"process_name":"p"},"input_sessions":[{"name":"m","input":["x"]}],"pool":2}`, "unknown field"},
		{"pipeline_field_in_jetrules_config", `{"bench_name":"b","jetrules_config":{"process_name":"p","pool_size":10},"input_sessions":[{"name":"m","input":["x"]}]}`, "unknown field"},
		{"no_name", `{"jetrules_config":{"process_name":"p"},"input_sessions":[{"name":"m","input":["x"]}]}`, "bench_name"},
		{"no_process", `{"bench_name":"b","jetrules_config":{},"input_sessions":[{"name":"m","input":["x"]}]}`, "process_name"},
		{"no_input", `{"bench_name":"b","jetrules_config":{"process_name":"p"}}`, "input_sessions or input_dir"},
		{"bad_engine", `{"bench_name":"b","jetrules_config":{"process_name":"p"},"input_sessions":[{"name":"m","input":["x"]}],"engines":["cpp"]}`, "unknown engine"},
		{"legacy_off", `{"bench_name":"b","jetrules_config":{"process_name":"p"},"input_sessions":[{"name":"m","input":["x"]}],"legacy":{"withdrawn_row_stays":false}}`, "production's retraction mode"},
		{"cpp_only_switch", `{"bench_name":"b","jetrules_config":{"process_name":"p","rule_config":[{"_0:no_truth_main_on_exist":"true"}]},"input_sessions":[{"name":"m","input":["x"]}]}`, "C++ engine"},
		{"bad_regex", `{"bench_name":"b","jetrules_config":{"process_name":"p"},"input_dir":{"path":"d","name_regex":"("}}`, "name_regex"},
		{"negative", `{"bench_name":"b","jetrules_config":{"process_name":"p"},"input_sessions":[{"name":"m","input":["x"]}],"repetitions":-1}`, "negative"},
		{"two_values", minimalSpec + `{}`, "more than one"},
	} {
		if _, err := ParseSpec([]byte(c.spec)); err == nil || !strings.Contains(err.Error(), c.want) {
			t.Errorf("%s: error %v, want one containing %q", c.name, err, c.want)
		}
	}
}

func TestSessions(t *testing.T) {
	dir := t.TempDir()
	in := filepath.Join(dir, "members")
	if err := os.Mkdir(in, 0o755); err != nil {
		t.Fatal(err)
	}
	for _, f := range []string{"member_0002.csv", "member_0001.csv", "member_0010.csv", "notes.txt", "other.csv"} {
		if err := os.WriteFile(filepath.Join(in, f), []byte("jets:key\n"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	names := func(ss []Session) []string {
		var out []string
		for _, s := range ss {
			out = append(out, s.Name)
		}
		return out
	}
	spec := &Spec{InputDir: &InputDir{Path: "members"}}
	ss, err := spec.Sessions(dir)
	if err != nil {
		t.Fatal(err)
	}
	if got := names(ss); !slices.Equal(got, []string{"member_0001", "member_0002", "member_0010", "other"}) {
		t.Errorf("default rule: %v", got)
	}
	if ss[0].Files[0] != filepath.Join(in, "member_0001.csv") {
		t.Errorf("file %s, want it under the spec's directory", ss[0].Files[0])
	}
	spec.InputDir.NameRegex = `^member_(\d+)\.csv$`
	spec.InputDir.Limit = 2
	if ss, err = spec.Sessions(dir); err != nil || !slices.Equal(names(ss), []string{"0001", "0002"}) {
		t.Errorf("regex rule and limit: %v, %v", names(ss), err)
	}
	spec.InputDir.NameRegex = `^member_\d+`
	spec.InputDir.Limit = 0
	if ss, err = spec.Sessions(dir); err != nil || !slices.Equal(names(ss), []string{"member_0001", "member_0002", "member_0010"}) {
		t.Errorf("regex without a group: %v, %v", names(ss), err)
	}

	// input_sessions first, absolute paths kept, names unique.
	spec.InputDir = &InputDir{Path: in, NameRegex: `^member_(0001)\.csv$`}
	spec.InputSessions = Session2{{"0001", "/abs/a.csv"}}.toInput()
	if _, err := spec.Sessions(dir); err == nil || !strings.Contains(err.Error(), "two input sessions") {
		t.Errorf("a duplicate name: %v", err)
	}
	spec.InputSessions = Session2{{"first", "/abs/a.csv"}}.toInput()
	ss, err = spec.Sessions(dir)
	if err != nil || !slices.Equal(names(ss), []string{"first", "0001"}) || ss[0].Files[0] != "/abs/a.csv" {
		t.Errorf("input_sessions then input_dir: %v %v, %v", names(ss), ss, err)
	}
	spec.InputDir = &InputDir{Path: in, Glob: "*.json"}
	if _, err := spec.Sessions(dir); err == nil {
		t.Errorf("an input_dir matching no file was accepted")
	}
}

func TestTakeMaxVertexVisits(t *testing.T) {
	rc := []map[string]any{
		{"jets:key": map[string]any{"value": "jets:iState", "type": "resource"},
			"jets:max_vertex_visits": map[string]any{"value": "1000000", "type": "int"}},
		{"usi_sm:clientName": "x"},
	}
	out, n, err := takeMaxVertexVisits(rc)
	if err != nil || n != 1000000 || len(out) != 2 {
		t.Fatalf("got %v, %d, %v", out, n, err)
	}
	if _, ok := out[0]["jets:max_vertex_visits"]; ok {
		t.Errorf("the cap is still in rule_config")
	}
	if _, ok := rc[0]["jets:max_vertex_visits"]; !ok {
		t.Errorf("the spec's rule_config was changed")
	}
	if _, n, err := takeMaxVertexVisits(rc[1:]); err != nil || n != 0 {
		t.Errorf("no cap: %d, %v", n, err)
	}
	bad := []map[string]any{{"jets:max_vertex_visits": map[string]any{"value": "10", "type": "text"}}}
	if _, _, err := takeMaxVertexVisits(bad); err == nil {
		t.Errorf("a text cap was accepted")
	}
}

func TestNewDumper(t *testing.T) {
	dir := t.TempDir()
	path := func(dir, engine, member string, index int, ruleSet string) string {
		return filepath.Join(dir, engine, member, fmt.Sprintf("%02d_%s.tsv", index+1, filepath.Base(ruleSet)))
	}
	write := func(w io.Writer, _ compute_pipes.JetRdfSession) error {
		_, err := io.WriteString(w, "s\tp\to\n")
		return err
	}
	dump := NewDumper(dir, path, write)
	for _, p := range []DumpPoint{
		{Engine: "go", Session: "member1", Index: 0, RuleSet: "a.jr", Repetition: 1},
		{Engine: "go", Session: "member1", Index: 0, RuleSet: "a.jr", Repetition: 2},
		{Engine: "native", Session: "a/b", Index: 3, RuleSet: "x/b.jr", Repetition: 1, Warmup: true},
	} {
		if err := dump(p, nil); err != nil {
			t.Fatal(err)
		}
	}
	for _, f := range []string{"r01/go/member1/01_a.jr.tsv", "r02/go/member1/01_a.jr.tsv", "w01/native/a_b/04_b.jr.tsv"} {
		if b, err := os.ReadFile(filepath.Join(dir, f)); err != nil || string(b) != "s\tp\to\n" {
			t.Errorf("%s: %q, %v", f, b, err)
		}
	}
}

func TestCPUList(t *testing.T) {
	cpus := parseCPUList("0-3,5,8-9")
	if !slices.Equal(cpus, []int{0, 1, 2, 3, 5, 8, 9}) {
		t.Errorf("parse: %v", cpus)
	}
	if got := formatCPUList(cpus); got != "0-3,5,8-9" {
		t.Errorf("format: %s", got)
	}
	e := &Environment{Governors: map[string]string{"performance": "0-5"}}
	if !e.GovernorsOK() {
		t.Errorf("performance on every CPU is not OK")
	}
	e.Governors["powersave"] = "6"
	if e.GovernorsOK() {
		t.Errorf("a powersave CPU is OK")
	}
}

// Session2 is a compact way to write input sessions in a test: name, file.
type Session2 [][2]string

func (s Session2) toInput() []rulestest.InputSession {
	var out []rulestest.InputSession
	for _, p := range s {
		out = append(out, rulestest.InputSession{Name: p[0], Input: []string{p[1]}})
	}
	return out
}
