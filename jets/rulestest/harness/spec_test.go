package harness

import (
	"reflect"
	"sort"
	"strings"
	"testing"

	"github.com/artisoft-io/jetstore/jets/compute_pipes"
	"github.com/artisoft-io/jetstore/jets/rulestest"
)

func jsonNames(t reflect.Type) []string {
	var names []string
	for i := 0; i < t.NumField(); i++ {
		name, _, _ := strings.Cut(t.Field(i).Tag.Get("json"), ",")
		if name != "" && name != "-" {
			names = append(names, name)
		}
	}
	sort.Strings(names)
	return names
}

// TestEveryJetrulesSpecFieldIsClassified fails the day JetrulesSpec gains a
// field: a test definition must use it or refuse it (definition §3.2), and the
// choice is made in rulestest.JetrulesConfig, not by default.
func TestEveryJetrulesSpecFieldIsClassified(t *testing.T) {
	classified := map[string]string{}
	for _, n := range rulestest.UsedJetrulesSpecFields() {
		classified[n] = "used"
	}
	for _, n := range rulestest.RefusedJetrulesSpecFields() {
		classified[n] = "refused"
	}
	spec := map[string]bool{}
	for _, n := range jsonNames(reflect.TypeFor[compute_pipes.JetrulesSpec]()) {
		spec[n] = true
		if classified[n] == "" {
			t.Errorf("JetrulesSpec field %q is neither used nor refused by rulestest.JetrulesConfig", n)
		}
	}
	for n, how := range classified {
		if !spec[n] {
			t.Errorf("rulestest %s field %q is not a JetrulesSpec field", how, n)
		}
	}
}

func TestJetrulesSpecOf(t *testing.T) {
	c := &rulestest.JetrulesConfig{
		Comment: "c", ProcessName: "jet_rules/x.jr", UseJetRulesNative: true, InputRdfType: "hc:Eligibility",
		MaxLooping: 7, CurrentSourcePeriod: 3, CurrentSourcePeriodDate: "2024-01-01", CurrentSourcePeriodType: "month_period",
		RuleConfig: []map[string]any{{"jets:key": "jets:iState", "jets:max_vertex_visits": map[string]any{"value": "5", "type": "int"}}},
		IsDebug:    true,
	}
	spec, err := JetrulesSpecOf(c)
	if err != nil {
		t.Fatal(err)
	}
	want := compute_pipes.JetrulesSpec{
		Comment: "c", ProcessName: "jet_rules/x.jr", UseJetRulesNative: true, InputRdfType: "hc:Eligibility",
		MaxLooping: 7, CurrentSourcePeriod: 3, CurrentSourcePeriodDate: "2024-01-01", CurrentSourcePeriodType: "month_period",
		RuleConfig: c.RuleConfig, IsDebug: true,
	}
	if !reflect.DeepEqual(*spec, want) {
		t.Errorf("JetrulesSpecOf:\n got %+v\nwant %+v", *spec, want)
	}
}
