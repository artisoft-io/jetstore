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
	refused("in a conditional_config override", "pipes_config, pipe 0, apply 0, conditional_config[0]", &ComputePipesConfig{PipesConfig: override}, override)

	// The process is named when the jetrules_config names one, and only then.
	err := validateNoGoEngine(&ComputePipesConfig{PipesConfig: jetrulesPipe(true)})
	if err == nil || !strings.Contains(err.Error(), `(process "p")`) {
		t.Errorf("want the process named, got %v", err)
	}
	unnamed := jetrulesPipe(true)
	unnamed[0].Apply[0].JetrulesConfig.ProcessName = ""
	if err := validateNoGoEngine(&ComputePipesConfig{PipesConfig: unnamed}); err == nil || strings.Contains(err.Error(), "process") {
		t.Errorf("want no process named when it is empty, got %v", err)
	}

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

// A jetrules step that runs on cp_node, the cpipesSM node, is refused when the run
// starts, naming cpipesNativeSM, rather than failing on every node part way through
// the run. A step that the ECS task or the Python node runs, or whose placement or
// selection is an expression of the run's env, is accepted: only what is certain to
// run on cp_node is refused.
func TestRulesOnCpipesSMAreRefusedAtStart(t *testing.T) {
	mapStep := func() ConditionalPipeSpec {
		return ConditionalPipeSpec{StepName: "map", PipesConfig: []PipeSpec{{
			Type:         "fan_out",
			InputChannel: InputChannelConfig{Name: "input_row"},
			Apply:        []TransformationSpec{{Type: "map_record", OutputChannel: OutputChannelConfig{Name: "mapped_row"}}},
		}}}
	}
	rulesStep := func(set func(*ConditionalPipeSpec)) ConditionalPipeSpec {
		c := ConditionalPipeSpec{StepName: "rules", PipesConfig: jetrulesPipe(false)}
		if set != nil {
			set(&c)
		}
		return c
	}
	when := &ExpressionNode{Type: "value", Expr: "1"}
	cases := []struct {
		name    string
		doc     *ComputePipesConfig
		refused string // the site the refusal names, or "" for accepted
	}{
		{"a reducing step on the node", &ComputePipesConfig{ConditionalPipesConfig: []ConditionalPipeSpec{
			mapStep(), rulesStep(nil)}}, `conditional_pipes_config[1] (step_name "rules"), pipe 0, apply 0`},
		{"the sharding step, whatever it sets", &ComputePipesConfig{ConditionalPipesConfig: []ConditionalPipeSpec{
			rulesStep(func(c *ConditionalPipeSpec) { c.UseEcsTasks = true })}}, `conditional_pipes_config[0] (step_name "rules")`},
		{"a reducing_pipes_config step", &ComputePipesConfig{ReducingPipesConfig: [][]PipeSpec{
			mapStep().PipesConfig, jetrulesPipe(false)}}, "reducing_pipes_config[1], pipe 0, apply 0"},
		{"a conditional_config override", &ComputePipesConfig{ConditionalPipesConfig: []ConditionalPipeSpec{
			mapStep(), {StepName: "later", PipesConfig: []PipeSpec{{Type: "fan_out", InputChannel: InputChannelConfig{Name: "input_row"},
				Apply: []TransformationSpec{{Type: "map_record", ConditionalConfig: []*ConditionalTransformationSpec{{
					When: *when,
					Then: TransformationSpec{Type: "jetrules", JetrulesConfig: &JetrulesSpec{ProcessName: "p"}}}}}}}}}}},
			`conditional_pipes_config[1] (step_name "later"), pipe 0, apply 0, conditional_config[0]`},

		// The ECS task's container runs the C++ engine (cpipes_native_server).
		{"a reducing step in the ECS task", &ComputePipesConfig{ConditionalPipesConfig: []ConditionalPipeSpec{
			mapStep(), rulesStep(func(c *ConditionalPipeSpec) { c.UseEcsTasks = true })}}, ""},
		{"a reducing step on the Python node", &ComputePipesConfig{ConditionalPipesConfig: []ConditionalPipeSpec{
			mapStep(), rulesStep(func(c *ConditionalPipeSpec) { c.UsePythonNode = true })}}, ""},
		{"an ECS choice that is an expression", &ComputePipesConfig{ConditionalPipesConfig: []ConditionalPipeSpec{
			mapStep(), rulesStep(func(c *ConditionalPipeSpec) { c.UseEcsTasksWhen = when })}}, ""},
		{"a step with a when", &ComputePipesConfig{ConditionalPipesConfig: []ConditionalPipeSpec{
			mapStep(), rulesStep(func(c *ConditionalPipeSpec) { c.When = when })}}, ""},
		{"the authored pipes_config, never selected", &ComputePipesConfig{PipesConfig: jetrulesPipe(false),
			ConditionalPipesConfig: []ConditionalPipeSpec{mapStep()}}, ""},
		{"no jetrules step", &ComputePipesConfig{ConditionalPipesConfig: []ConditionalPipeSpec{mapStep()}}, ""},
	}
	for _, c := range cases {
		err := validateNoRulesOnCpipesSM(c.doc, "a_process", "cpipesSM")
		switch {
		case c.refused == "" && err != nil:
			t.Errorf("%s: refused: %v", c.name, err)
		case c.refused != "" && (err == nil || !strings.Contains(err.Error(), c.refused) ||
			!strings.Contains(err.Error(), "cpipesNativeSM") || !strings.Contains(err.Error(), "cp_node")):
			t.Errorf("%s: want a refusal naming %s, cp_node and cpipesNativeSM, got %v", c.name, c.refused, err)
		}
		// The native state machine runs every one of them.
		if err := validateNoRulesOnCpipesSM(c.doc, "a_process", "cpipesNativeSM"); err != nil {
			t.Errorf("%s on cpipesNativeSM: refused: %v", c.name, err)
		}
	}
}
