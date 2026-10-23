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
	refused := func(what string, doc *ComputePipesConfig, current []PipeSpec) {
		t.Helper()
		err := startup.ValidatePipeSpecConfig(doc, current)
		if err == nil || !strings.Contains(err.Error(), GoEngineRetired) ||
			!strings.Contains(err.Error(), "use_jet_rules_go") {
			t.Errorf("%s: want a refusal naming use_jet_rules_go and the retirement, got %v", what, err)
		}
	}

	pipe := jetrulesPipe(true)
	refused("on the starting step", &ComputePipesConfig{PipesConfig: pipe}, pipe)

	clean := []PipeSpec{{
		Type:         "fan_out",
		InputChannel: InputChannelConfig{Name: "input_row"},
		Apply:        []TransformationSpec{{Type: "map_record", OutputChannel: OutputChannelConfig{Name: "mapped_row"}}},
	}}
	refused("on a later step", &ComputePipesConfig{
		PipesConfig:         clean,
		ReducingPipesConfig: [][]PipeSpec{jetrulesPipe(true)},
	}, clean)
	refused("on a conditional step", &ComputePipesConfig{
		PipesConfig:            clean,
		ConditionalPipesConfig: []ConditionalPipeSpec{{StepName: "later", PipesConfig: jetrulesPipe(true)}},
	}, clean)

	override := jetrulesPipe(false)
	override[0].Apply[0].ConditionalConfig = []*ConditionalTransformationSpec{{
		Then: TransformationSpec{JetrulesConfig: &JetrulesSpec{UseJetRulesGo: true}},
	}}
	refused("in a conditional_config override", &ComputePipesConfig{PipesConfig: override}, override)

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
