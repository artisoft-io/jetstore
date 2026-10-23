package compute_pipes

import (
	"strings"
	"testing"
)

// THE GO ENGINE IS RETIRED, AND A REQUEST FOR IT IS REFUSED (rules_benchmark Phase 6).
//
// use_jet_rules_go: true fails the pipeline when its first step starts, wherever the
// document sets it; false asks for nothing and is accepted. A jetrules step on a node
// with no rule engine fails naming the native node instead of falling back.

func jetrulesPipe(useGo bool) []PipeSpec {
	return []PipeSpec{{
		Type:         "fan_out",
		InputChannel: InputChannelConfig{Name: "input_row"},
		Apply: []TransformationSpec{{
			Type:           "jetrules",
			JetrulesConfig: &JetrulesSpec{ProcessName: "p", UseJetRulesGo: useGo},
		}},
	}}
}

func TestUseJetRulesGoIsRefusedAtStart(t *testing.T) {
	startup := &CpipesStartup{}
	// where is the step and position as the document names them.
	refused := func(what, where string, doc *ComputePipesConfig, current []PipeSpec) {
		t.Helper()
		err := startup.ValidatePipeSpecConfig(doc, current)
		if err == nil || !strings.Contains(err.Error(), GoEngineRetired) ||
			!strings.Contains(err.Error(), "use_jet_rules_go") || !strings.Contains(err.Error(), where) {
			t.Errorf("%s: want a refusal naming %s, use_jet_rules_go and the retirement, got %v", what, where, err)
		}
	}

	pipe := jetrulesPipe(true)
	refused("on the starting step", "pipes_config, pipe 0, apply 0", &ComputePipesConfig{PipesConfig: pipe}, pipe)

	clean := []PipeSpec{{
		Type:         "fan_out",
		InputChannel: InputChannelConfig{Name: "input_row"},
		Apply:        []TransformationSpec{{Type: "map_record", OutputChannel: OutputChannelConfig{Name: "mapped_row"}}},
	}}
	refused("on a later step", "reducing_pipes_config[0], pipe 0, apply 0", &ComputePipesConfig{
		PipesConfig:         clean,
		ReducingPipesConfig: [][]PipeSpec{jetrulesPipe(true)},
	}, clean)
	refused("on a conditional step", `conditional_pipes_config[0] (step_name "later"), pipe 0, apply 0`, &ComputePipesConfig{
		PipesConfig:            clean,
		ConditionalPipesConfig: []ConditionalPipeSpec{{StepName: "later", PipesConfig: jetrulesPipe(true)}},
	}, clean)

	override := jetrulesPipe(false)
	override[0].Apply[0].ConditionalConfig = []*ConditionalTransformationSpec{{
		Then: TransformationSpec{JetrulesConfig: &JetrulesSpec{UseJetRulesGo: true}},
	}}
	refused("in a conditional_config override", "pipes_config, pipe 0, apply 0", &ComputePipesConfig{PipesConfig: override}, override)

	// false is what twelve configurations set; it asks for nothing.
	if err := validateNoGoEngine(&ComputePipesConfig{PipesConfig: jetrulesPipe(false)}); err != nil {
		t.Errorf("use_jet_rules_go: false was refused: %v", err)
	}
}

type noEngineProxy struct{}

func (noEngineProxy) GetNativeFactory() JetRulesFactory { return nil }

func TestJetrulesStepRefusesTheGoEngineAndANodeWithNone(t *testing.T) {
	spec := func(useGo bool) *TransformationSpec {
		return &TransformationSpec{Type: "jetrules", JetrulesConfig: &JetrulesSpec{ProcessName: "p", UseJetRulesGo: useGo}}
	}

	// A configuration that reached the node without passing the start check.
	_, err := (&BuilderContext{jetRules: noEngineProxy{}}).NewJetrulesTransformationPipe(nil, nil, spec(true))
	if err == nil || !strings.Contains(err.Error(), GoEngineRetired) {
		t.Errorf("use_jet_rules_go on the node: want the retirement, got %v", err)
	}

	// No engine, whether the binary carries none or no proxy was given: an error
	// naming the native node, not a fallback.
	for what, ctx := range map[string]*BuilderContext{
		"a proxy with no engine": {jetRules: noEngineProxy{}},
		"no proxy":               {},
	} {
		_, err := ctx.NewJetrulesTransformationPipe(nil, nil, spec(false))
		if err == nil || !strings.Contains(err.Error(), "cpipesNativeSM") {
			t.Errorf("%s: want an error naming cpipesNativeSM, got %v", what, err)
		}
	}
}

// A jetrules step in a process on cpipesSM is refused when the run starts, naming
// cpipesNativeSM, rather than failing on every cp_node part way through the run.
func TestRulesOnCpipesSMAreRefusedAtStart(t *testing.T) {
	clean := []PipeSpec{{
		Type:         "fan_out",
		InputChannel: InputChannelConfig{Name: "input_row"},
		Apply:        []TransformationSpec{{Type: "map_record", OutputChannel: OutputChannelConfig{Name: "mapped_row"}}},
	}}
	withRules := &ComputePipesConfig{
		PipesConfig:            clean,
		ConditionalPipesConfig: []ConditionalPipeSpec{{StepName: "rules", PipesConfig: jetrulesPipe(false)}},
	}
	err := validateNoRulesOnCpipesSM(withRules, "a_process", "cpipesSM")
	if err == nil || !strings.Contains(err.Error(), "cpipesNativeSM") || !strings.Contains(err.Error(), `step_name "rules"`) {
		t.Errorf("rules on cpipesSM: want a refusal naming cpipesNativeSM and the step, got %v", err)
	}
	// A jetrules step brought in by a conditional_config override counts too.
	override := []PipeSpec{{Type: "fan_out", InputChannel: InputChannelConfig{Name: "input_row"},
		Apply: []TransformationSpec{{Type: "map_record", ConditionalConfig: []*ConditionalTransformationSpec{{
			Then: TransformationSpec{Type: "jetrules", JetrulesConfig: &JetrulesSpec{ProcessName: "p"}}}}}}}}
	if err := validateNoRulesOnCpipesSM(&ComputePipesConfig{PipesConfig: override}, "a_process", "cpipesSM"); err == nil {
		t.Errorf("a jetrules override on cpipesSM was accepted")
	}

	// The native state machine runs rules; cpipesSM runs everything else.
	for _, c := range []struct {
		doc *ComputePipesConfig
		sm  string
	}{
		{withRules, "cpipesNativeSM"},
		{&ComputePipesConfig{PipesConfig: clean}, "cpipesSM"},
	} {
		if err := validateNoRulesOnCpipesSM(c.doc, "a_process", c.sm); err != nil {
			t.Errorf("%s: refused: %v", c.sm, err)
		}
	}
}
