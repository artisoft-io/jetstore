package compute_pipes

import (
	"encoding/json"
	"strings"
	"testing"
)

// corpusWiring is the shape all ten error channel declarations in the rule corpus
// take, and it is one shape rather than a family: nine of them are on a jetrules
// operator and one on ollama, the two channel names ever used have byte-identical
// specs, all ten output_tables entries bind to jetsapi.process_errors, and every one
// of them is followed by a pipe reading the error channel and map_record'ing it into
// the channel the table writer drains. Measured 2026-09-05 over the 45 .pc.json
// documents of cedargate_ws, jets_ws, usi_ws and walrus_ws.
//
// The backwards-compatibility clause of this change is a claim about these ten, so
// the tests below are written against the shape rather than against an invented one.
func corpusWiring() (*ComputePipesConfig, []PipeSpec) {
	cpConfig := &ComputePipesConfig{
		Channels: []ChannelSpec{
			{Name: "process_errors", Columns: append([]string{}, legacyProcessErrorColumns...)},
		},
		OutputTables: []*TableSpec{
			{Key: "process_errors", Name: "jetsapi.process_errors", ChannelSpecName: "process_errors"},
		},
	}
	pipeConfig := []PipeSpec{
		{
			InputChannel: InputChannelConfig{Name: "BaseClaim.merged"},
			Apply: []TransformationSpec{{
				Type: "jetrules",
				JetrulesConfig: &JetrulesSpec{
					OutputChannels: []OutputChannelConfig{{Name: "MedicalClaim.out", SpecName: "MedicalClaim"}},
					ErrorChannel: &OutputChannelConfig{
						Name: "process_errors.out", SpecName: "process_errors"},
				},
			}},
		},
		{
			InputChannel: InputChannelConfig{Name: "process_errors.out"},
			Apply: []TransformationSpec{{
				Type:          "map_record",
				OutputChannel: OutputChannelConfig{Type: "sql", OutputTableKey: "process_errors"},
			}},
		},
	}
	return cpConfig, pipeConfig
}

func mustJSON(t *testing.T, v any) string {
	t.Helper()
	b, err := json.Marshal(v)
	if err != nil {
		t.Fatalf("marshalling: %v", err)
	}
	return string(b)
}

// The ten declarations keep working, and "keep working" is stated as "nothing about
// them changes": the synthesis is a default and a default that edits an author's
// configuration is not one.
//
// Note what the second pipe's map_record proves as a side effect. It is an operator
// that reports row-level failures and names no error channel of its own, so a
// synthesis that looked only at the operator would have given the corpus's own error
// *writer* an error channel. It does not, because the map_record here does carry a
// config -- but a bare one, which is the same state 240 corpus instances are in. See
// TestSynthesizeGivesTheErrorWriterAChannelToo for what does happen and why it is
// harmless.
func TestSynthesizeLeavesAnAuthoredErrorChannelAlone(t *testing.T) {
	cpConfig, pipeConfig := corpusWiring()
	before := mustJSON(t, pipeConfig[0])

	SynthesizeDefaultErrorChannels(cpConfig, pipeConfig)

	if got := mustJSON(t, pipeConfig[0]); got != before {
		t.Errorf("the authored jetrules operator changed\n before: %s\n  after: %s", before, got)
	}
	ec := errorChannelConfig(&pipeConfig[0].Apply[0])
	if ec.Name != "process_errors.out" || ec.SpecName != "process_errors" {
		t.Errorf("authored error channel is now %+v", ec)
	}
	// The authored channel spec and table entry are untouched and still first.
	if cpConfig.Channels[0].Name != "process_errors" {
		t.Errorf("authored channel spec displaced: %v", cpConfig.Channels[0].Name)
	}
	if cpConfig.OutputTables[0].Key != "process_errors" {
		t.Errorf("authored table entry displaced: %v", cpConfig.OutputTables[0].Key)
	}
}

// The corpus's own error writer is a bare map_record, so it gets a default channel
// like any other. That is correct rather than tolerated: a mapping failure inside
// the error writer is a row-level failure with nowhere to go today.
func TestSynthesizeGivesTheErrorWriterAChannelToo(t *testing.T) {
	cpConfig, pipeConfig := corpusWiring()
	if n := SynthesizeDefaultErrorChannels(cpConfig, pipeConfig); n != 1 {
		t.Fatalf("synthesised %d channels, want 1", n)
	}
	ec := errorChannelConfig(&pipeConfig[1].Apply[0])
	if ec == nil {
		t.Fatal("the map_record got no error channel")
	}
	if ec.SpecName != DefaultErrorChannelSpecName {
		t.Errorf("spec name is %q, want %q", ec.SpecName, DefaultErrorChannelSpecName)
	}
	if !isDefaultErrorChannel(ec) {
		t.Errorf("%+v is not recognised as a default error channel", ec)
	}
	// The emitted contract marks output_table_key applicable on a channel of type
	// sql alone, so the synthesised channel must not carry one: the binding travels
	// by the spec name instead.
	if ec.OutputTableKey != "" {
		t.Errorf("output_table_key is %q on a memory channel; the contract says sql only", ec.OutputTableKey)
	}
	if ec.Name == ec.SpecName {
		t.Error("name must differ from the spec name, validateOutputChConfig refuses that")
	}
	// The two must not collide with the authored ones.
	if ec.Name == "process_errors.out" || ec.Name == "process_errors" {
		t.Errorf("synthesised name %q collides with the authored wiring", ec.Name)
	}
}

// A step whose operators all name an error channel gets no synthesis at all: no
// channel spec, no table entry, nothing added to the configuration that travels to
// the node.
func TestSynthesizeAddsNothingWhenEveryOperatorIsWired(t *testing.T) {
	cpConfig, pipeConfig := corpusWiring()
	// Wire the error writer by hand as well, so nothing is left bare.
	pipeConfig[1].Apply[0].MapRecordConfig = &MapRecordSpec{
		ErrorChannel: &OutputChannelConfig{Name: "writer_errors.out", SpecName: "process_errors"},
	}
	channelsBefore, tablesBefore := len(cpConfig.Channels), len(cpConfig.OutputTables)

	if n := SynthesizeDefaultErrorChannels(cpConfig, pipeConfig); n != 0 {
		t.Fatalf("synthesised %d channels, want 0", n)
	}
	if len(cpConfig.Channels) != channelsBefore || len(cpConfig.OutputTables) != tablesBefore {
		t.Errorf("channels %d -> %d, tables %d -> %d; want neither to move",
			channelsBefore, len(cpConfig.Channels), tablesBefore, len(cpConfig.OutputTables))
	}
	if defaultErrorTableIsActive(cpConfig) {
		t.Error("the default table is present in a configuration that needed no default")
	}
}

// Every operator that reports row-level failures gets a channel, and only those.
func TestSynthesizeCoversTheReportingOperators(t *testing.T) {
	pipeConfig := []PipeSpec{{Apply: []TransformationSpec{
		{Type: "map_record"},
		{Type: "jetrules", JetrulesConfig: &JetrulesSpec{}},
		{Type: "ollama", OllamaConfig: &OllamaSpec{}},
		{Type: "embed", EmbedConfig: &EmbedSpec{}},
		{Type: "vllm", VllmConfig: &VllmSpec{}},
		{Type: "filter"},
		{Type: "group_by"},
		{Type: "partition_writer"},
	}}}
	cpConfig := &ComputePipesConfig{}

	if n := SynthesizeDefaultErrorChannels(cpConfig, pipeConfig); n != 5 {
		t.Fatalf("synthesised %d channels, want 5", n)
	}
	for i := range pipeConfig[0].Apply {
		ts := &pipeConfig[0].Apply[i]
		got := errorChannelConfig(ts) != nil
		want := reportsRowLevelFailures(ts.Type)
		if got != want {
			t.Errorf("%s: has error channel %v, want %v", ts.Type, got, want)
		}
	}
	// reportsRowLevelFailures and errorChannelConfig must name the same operators;
	// a divergence gives an operator a channel nothing can find.
	for _, typ := range []string{"map_record", "jetrules", "ollama", "embed", "vllm",
		"filter", "sort", "anonymize", "clustering", "merge", "partition_writer"} {
		ts := &TransformationSpec{Type: typ}
		if reportsRowLevelFailures(typ) != (errorChannelConfigTypeIsKnown(ts)) {
			t.Errorf("%s: reportsRowLevelFailures and errorChannelConfig disagree", typ)
		}
	}
}

// errorChannelConfigTypeIsKnown says whether errorChannelConfig has an arm for this
// operator type, independently of whether the operator carries a config struct.
func errorChannelConfigTypeIsKnown(ts *TransformationSpec) bool {
	switch ts.Type {
	case "map_record", "jetrules", "ollama", "embed", "vllm":
		return true
	}
	return false
}

// The synthesised channel spec declares the three discriminators the ten hand-written
// specs do not, which is the difference between an error row that names its operator,
// channel and step and one that writes NULL for all three.
func TestSynthesizedSpecDeclaresTheDiscriminators(t *testing.T) {
	cpConfig := &ComputePipesConfig{}
	pipeConfig := []PipeSpec{{Apply: []TransformationSpec{{Type: "map_record"}}}}
	SynthesizeDefaultErrorChannels(cpConfig, pipeConfig)

	spec := cpConfig.GetChannelSpec(DefaultErrorChannelSpecName)
	if spec == nil {
		t.Fatal("no synthesised channel spec")
	}
	if missing := missingDiscriminatorColumns(spec); len(missing) > 0 {
		t.Errorf("the synthesised spec is missing %v", missing)
	}
	for _, c := range legacyProcessErrorColumns {
		if !strings.Contains(strings.Join(spec.Columns, ","), c) {
			t.Errorf("the synthesised spec is missing the legacy column %q", c)
		}
	}
	if len(spec.Columns) != len(legacyProcessErrorColumns)+len(ProcessErrorDiscriminatorColumns) {
		t.Errorf("the synthesised spec has %d columns, want %d",
			len(spec.Columns), len(legacyProcessErrorColumns)+len(ProcessErrorDiscriminatorColumns))
	}
}

// validateErrorChannels is the check the synthesis has to satisfy rather than
// weaken: each operator instance owns its error channel because the pipe executor
// closes that channel when the operator is done.
func TestSynthesizedChannelsSatisfyValidateErrorChannels(t *testing.T) {
	apply := make([]TransformationSpec, 0, 53)
	for range 53 {
		apply = append(apply, TransformationSpec{
			Type:          "map_record",
			OutputChannel: OutputChannelConfig{Name: "qc_metrics.writer", SpecName: "qc_metrics"},
		})
	}
	// One authored channel, to check the synthesised names do not walk into it.
	apply = append(apply, TransformationSpec{
		Type: "jetrules",
		JetrulesConfig: &JetrulesSpec{
			ErrorChannel: &OutputChannelConfig{Name: "process_errors.out", SpecName: "process_errors"},
		},
	})
	cpConfig := &ComputePipesConfig{}
	pipeConfig := []PipeSpec{{Apply: apply}}

	if n := SynthesizeDefaultErrorChannels(cpConfig, pipeConfig); n != 53 {
		t.Fatalf("synthesised %d channels, want 53", n)
	}
	if err := validateErrorChannels(pipeConfig); err != nil {
		t.Errorf("validateErrorChannels: %v", err)
	}
}

// A step with 53 reporting operators gets 53 channels and one table entry. One table
// spec is one WriteTableSource holding one pooled connection, and the cpipes servers
// default to a pool of three -- so the deduplication is what makes the default
// deployable rather than a tidy-up.
func TestSelectActiveOutputTableKeepsTheDefaultOnce(t *testing.T) {
	apply := make([]TransformationSpec, 0, 53)
	for range 53 {
		apply = append(apply, TransformationSpec{Type: "map_record"})
	}
	cpConfig := &ComputePipesConfig{}
	pipeConfig := []PipeSpec{{Apply: apply}}
	SynthesizeDefaultErrorChannels(cpConfig, pipeConfig)

	active, err := SelectActiveOutputTable(cpConfig.OutputTables, pipeConfig)
	if err != nil {
		t.Fatalf("SelectActiveOutputTable: %v", err)
	}
	if len(active) != 1 {
		t.Fatalf("selected %d tables, want 1", len(active))
	}
	if active[0].Key != DefaultErrorTableKey || active[0].Name != DefaultErrorTableName {
		t.Errorf("selected %+v", active[0])
	}
	if got := len(defaultErrorChannelNames(pipeConfig)); got != 53 {
		t.Errorf("%d channels feed the table, want 53", got)
	}
}

// The corpus's own selection is unchanged: one table, reached through the consuming
// map_record's output_channel exactly as before.
func TestSelectActiveOutputTableUnchangedForTheCorpusShape(t *testing.T) {
	cpConfig, pipeConfig := corpusWiring()
	active, err := SelectActiveOutputTable(cpConfig.OutputTables, pipeConfig)
	if err != nil {
		t.Fatalf("SelectActiveOutputTable: %v", err)
	}
	if len(active) != 1 || active[0].Key != "process_errors" {
		t.Fatalf("selected %d tables: %+v", len(active), active)
	}
}

// newFanInFixture builds a registry holding the sink and every synthesised source.
func newFanInFixture(t *testing.T, nbrOperators int) (*ComputePipesConfig, []PipeSpec, *ChannelRegistry) {
	t.Helper()
	apply := make([]TransformationSpec, 0, nbrOperators)
	for range nbrOperators {
		apply = append(apply, TransformationSpec{Type: "map_record"})
	}
	cpConfig := &ComputePipesConfig{}
	pipeConfig := []PipeSpec{{Apply: apply}}
	SynthesizeDefaultErrorChannels(cpConfig, pipeConfig)
	cpConfig.PipesConfig = pipeConfig
	active, err := SelectActiveOutputTable(cpConfig.OutputTables, pipeConfig)
	if err != nil {
		t.Fatalf("SelectActiveOutputTable: %v", err)
	}
	// The node receives the pruned list, which is what defaultErrorTableIsActive reads.
	cpConfig.OutputTables = active

	spec := cpConfig.GetChannelSpec(DefaultErrorChannelSpecName)
	columnsMap := make(map[string]int, len(spec.Columns))
	for i, c := range spec.Columns {
		columnsMap[c] = i
	}
	spec.columnsMap = &columnsMap
	reg := &ChannelRegistry{
		ComputeChannels: make(map[string]*Channel),
		ClosedChannels:  make(map[string]bool),
	}
	reg.ComputeChannels[DefaultErrorChannelSpecName] = &Channel{
		Name: DefaultErrorChannelSpecName, Channel: make(chan []any), Columns: &columnsMap, Config: spec}
	for _, name := range defaultErrorChannelNames(pipeConfig) {
		reg.ComputeChannels[name] = &Channel{
			Name: name, Channel: make(chan []any), Columns: &columnsMap, Config: spec}
	}
	return cpConfig, pipeConfig, reg
}

// Every operator's rows reach the one sink, and the sink closes when the last
// operator's channel does -- which is what lets the single CopyFrom return.
func TestDefaultErrorChannelFanIn(t *testing.T) {
	const nbrOperators = 8
	cpConfig, pipeConfig, reg := newFanInFixture(t, nbrOperators)
	done := make(chan struct{})
	defer close(done)

	if started := startDefaultErrorChannelFanIn(reg, cpConfig, done); started != nbrOperators {
		t.Fatalf("started %d copiers, want %d", started, nbrOperators)
	}

	names := defaultErrorChannelNames(pipeConfig)
	go func() {
		for _, name := range names {
			source := reg.ComputeChannels[name]
			source.Channel <- []any{name}
			reg.CloseChannel(name)
		}
	}()

	sink := reg.ComputeChannels[DefaultErrorChannelSpecName].Channel
	seen := make(map[string]bool)
	for row := range sink {
		seen[row[0].(string)] = true
	}
	if len(seen) != nbrOperators {
		t.Errorf("the sink received rows from %d operators, want %d", len(seen), nbrOperators)
	}
	for _, name := range names {
		if !seen[name] {
			t.Errorf("no row from %q reached the sink", name)
		}
	}
}

// The fan-in owns closing the sink, so it closes it even when it finds no sources.
// A writer that copies nothing finishes; a writer whose channel never closes hangs
// the step, and nothing else in the engine would close this one.
func TestDefaultErrorChannelFanInClosesTheSinkWithNoSources(t *testing.T) {
	cpConfig, pipeConfig, reg := newFanInFixture(t, 2)
	for _, name := range defaultErrorChannelNames(pipeConfig) {
		delete(reg.ComputeChannels, name)
	}
	done := make(chan struct{})
	defer close(done)

	if started := startDefaultErrorChannelFanIn(reg, cpConfig, done); started != 0 {
		t.Fatalf("started %d copiers, want 0", started)
	}
	if _, ok := <-reg.ComputeChannels[DefaultErrorChannelSpecName].Channel; ok {
		t.Error("the sink delivered a row it was never sent")
	}
}

// A configuration that wires its error channels by hand starts no fan-in, so the
// ten corpus declarations run through exactly the code they ran through before.
func TestDefaultErrorChannelFanInIsANoOpForTheCorpusShape(t *testing.T) {
	cpConfig, pipeConfig := corpusWiring()
	pipeConfig[1].Apply[0].MapRecordConfig = &MapRecordSpec{
		ErrorChannel: &OutputChannelConfig{Name: "writer_errors.out", SpecName: "process_errors"},
	}
	cpConfig.PipesConfig = pipeConfig
	reg := &ChannelRegistry{ComputeChannels: make(map[string]*Channel), ClosedChannels: make(map[string]bool)}
	done := make(chan struct{})
	defer close(done)

	if started := startDefaultErrorChannelFanIn(reg, cpConfig, done); started != 0 {
		t.Errorf("started %d copiers for a hand-wired configuration, want 0", started)
	}
}

// Synthesis is idempotent. Both startup paths call it once, but a config that has
// been through it must not grow a second spec or a second table entry.
func TestSynthesizeIsIdempotent(t *testing.T) {
	cpConfig := &ComputePipesConfig{}
	pipeConfig := []PipeSpec{{Apply: []TransformationSpec{{Type: "map_record"}, {Type: "map_record"}}}}

	if n := SynthesizeDefaultErrorChannels(cpConfig, pipeConfig); n != 2 {
		t.Fatalf("first pass synthesised %d, want 2", n)
	}
	first := mustJSON(t, cpConfig)
	if n := SynthesizeDefaultErrorChannels(cpConfig, pipeConfig); n != 0 {
		t.Fatalf("second pass synthesised %d, want 0", n)
	}
	if got := mustJSON(t, cpConfig); got != first {
		t.Errorf("the second pass changed the config\n first: %s\n  then: %s", first, got)
	}
}
