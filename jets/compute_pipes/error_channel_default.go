package compute_pipes

// Built-in error reporting: the error channel an operator gets when its
// configuration does not name one.
//
// Reporting one operator's row-level failures used to take four coordinated edits
// in one .pc.json -- an error_channel on the operator, a channel declaration
// listing that channel's columns, an output_tables entry binding a channel to
// jetsapi.process_errors, and a whole further pipe reading the error channel and
// map_record'ing it into the channel the table writer drains. Ten declarations in
// the rule corpus carry between them one distinct column list, one table and two
// channel names whose specs are byte-identical, so every instance of the feature is
// the same instance. This file makes that instance the default.
//
// The synthesis runs at startup, on the step's pipe config, after the conditional
// transformation specs have been applied and before anything validates or prunes.
// What it emits is an ordinary configuration: the node receives error channels,
// a channel spec and a table entry it cannot tell from hand-written ones, which is
// what keeps the runtime unchanged.
//
// An explicit error_channel wins. SynthesizeDefaultErrorChannels only fills a
// nil one, so the ten existing declarations keep their names, their specs and
// their consuming pipes, and nothing about their behaviour moves.
//
// One writer, not one per operator. validateErrorChannels requires each operator
// instance to own its error channel, since the pipe executor closes that channel
// when the operator is done -- so the synthesis gives each of them its own. It does
// not give each of them its own table writer: WriteTableSource takes a pooled
// connection for the whole life of the step and the cpipes servers default to a
// pool of three (dbPoolSize, jets/cmds/cpipes_server/main.go), while one corpus
// step carries 53 operators that report row-level failures. Fifty-three writers on
// a pool of three is a deadlock rather than a cost, so the synthesised channels
// fan into one sink channel and that sink is the single table writer's source.
// See startDefaultErrorChannelFanIn.

import (
	"fmt"
	"log"
	"sync"
)

const (
	// DefaultErrorChannelSpecName is the channel spec the synthesised error
	// channels share, and the name of the sink channel the default
	// jetsapi.process_errors writer drains. A channel spec name is also a channel
	// name in the registry (see the channelsInUse loop in compute_pipes.go), which
	// is what gives the sink a channel without declaring one.
	//
	// It doubles as the marker for a synthesised channel: an error channel whose
	// spec is this one fans into the default table. That is deliberately a rule
	// about the configuration rather than a hidden flag, so an author who wants
	// the default column list and the default binding can opt in by naming it.
	DefaultErrorChannelSpecName = "jets_process_errors"

	// DefaultErrorTableKey is the key of the synthesised output_tables entry.
	DefaultErrorTableKey = "jets_process_errors"

	// DefaultErrorTableName is the table the synthesised entry binds to. The
	// identity is never in doubt: all ten corpus declarations bind here.
	DefaultErrorTableName = "jetsapi.process_errors"
)

// DefaultProcessErrorColumns is the column list the synthesised channel spec
// declares. It is the nine columns every corpus declaration carries plus the three
// ProcessErrorDiscriminatorColumns the table gained for triage.
//
// The three are the point rather than a bonus. A process_errors row is assembled
// against its channel's declared columns and not against the table's
// (write2Chan, jets/compute_pipes/jetsrules_process_error.go:110), so the ten
// hand-written specs write NULL for all three until eleven files across three
// workspace repositories are edited. A synthesised spec carries them from the
// first run.
var DefaultProcessErrorColumns = []string{
	"pipeline_execution_status_key",
	"session_id",
	"grouping_key",
	"row_jets_key",
	"input_column",
	"error_message",
	"rete_session_saved",
	"rete_session_triples",
	"shard_id",
	"cpipes_step_id",
	"error_channel",
	"operator_type",
}

// SynthesizeDefaultErrorChannels gives every operator that reports row-level
// failures and names no error channel a channel of its own, and adds the shared
// channel spec and table binding those channels need. It returns the number of
// channels it synthesised.
//
// It mutates cpConfig and pipeConfig in place and adds no PipeSpec, so the caller's
// slice header stays valid; a synthesised channel reaches its table through the
// fan-in rather than through a consuming pipe.
//
// Call it after ApplyAllConditionalTransformationSpec and before either
// SelectActiveOutputTable or ValidatePipeSpecConfig: the sharding path prunes the
// output tables before it validates and the reducing path validates before it
// prunes, so the only position that serves both is ahead of both.
func SynthesizeDefaultErrorChannels(cpConfig *ComputePipesConfig, pipeConfig []PipeSpec) int {
	if cpConfig == nil {
		return 0
	}
	// The names already spoken for, so a synthesised one cannot collide with an
	// authored channel and trip validateErrorChannels on a name nobody wrote.
	taken := make(map[string]bool)
	for i := range cpConfig.Channels {
		taken[cpConfig.Channels[i].Name] = true
	}
	for i := range pipeConfig {
		for j := range pipeConfig[i].Apply {
			for _, ch := range outputChannelNames(&pipeConfig[i].Apply[j]) {
				taken[ch] = true
			}
			if ec := errorChannelConfig(&pipeConfig[i].Apply[j]); ec != nil {
				taken[ec.Name] = true
			}
		}
	}

	synthesized := 0
	for i := range pipeConfig {
		for j := range pipeConfig[i].Apply {
			transformationConfig := &pipeConfig[i].Apply[j]
			if !reportsRowLevelFailures(transformationConfig.Type) {
				continue
			}
			// An explicit error channel wins, and so does an explicit one whose
			// name the author left empty: that is a configuration error the
			// operators report themselves, and quietly filling it in would hide it.
			if errorChannelConfig(transformationConfig) != nil {
				continue
			}
			name := defaultErrorChannelName(transformationConfig.Type, i, j, taken)
			taken[name] = true
			// type, name and channel_spec_name and nothing else. The emitted cpipes
			// contract marks output_table_key applicable on a channel of type sql
			// alone (matrix/fields.csv, OutputChannelConfig/memory), and pointing
			// the synthesised channel at its table that way would move an axis of
			// that contract -- which A§9.2 names as the unreviewed extraction gap 2b
			// exists to prevent, and as an argument for scoping rather than
			// skipping. The binding is carried by the spec name instead, which is a
			// value rather than an axis.
			ec := &OutputChannelConfig{
				Comment: "synthesised by SynthesizeDefaultErrorChannels; " +
					"an explicit error_channel on this operator overrides it",
				Type:     "memory",
				Name:     name,
				SpecName: DefaultErrorChannelSpecName,
			}
			if !setErrorChannelConfig(transformationConfig, ec) {
				continue
			}
			synthesized++
		}
	}
	if synthesized == 0 {
		return 0
	}
	ensureDefaultErrorChannelSpec(cpConfig)
	ensureDefaultErrorTable(cpConfig)
	log.Printf("Built-in error reporting: synthesised %d error channel(s) into %s",
		synthesized, DefaultErrorTableName)
	return synthesized
}

// reportsRowLevelFailures says whether an operator of this type reports row-level
// failures on an error channel. It is errorChannelConfig's switch read as a
// predicate: the two must name the same operators or an operator gets a channel
// nothing can find, or finds a channel nothing gave it.
func reportsRowLevelFailures(operatorType string) bool {
	switch operatorType {
	case "map_record", "jetrules", "ollama", "embed", "vllm":
		return true
	}
	return false
}

// setErrorChannelConfig places ec on the operator's own config, returning false
// when the operator has no config struct to place it on -- an operator with no
// map_record_config, jetrules_config and so on is a configuration error the
// builders report, and this is not the place to report it a second time.
func setErrorChannelConfig(transformationConfig *TransformationSpec, ec *OutputChannelConfig) bool {
	switch transformationConfig.Type {
	case "map_record":
		if transformationConfig.MapRecordConfig == nil {
			// map_record is the one operator whose config is optional: its
			// builder reads a nil config and applies the defaults. Give it one
			// rather than skip it, since it is the operator the corpus wires an
			// error channel on 0 times in 240.
			transformationConfig.MapRecordConfig = &MapRecordSpec{}
		}
		transformationConfig.MapRecordConfig.ErrorChannel = ec
		return true
	case "jetrules":
		if transformationConfig.JetrulesConfig == nil {
			return false
		}
		transformationConfig.JetrulesConfig.ErrorChannel = ec
		return true
	case "ollama":
		if transformationConfig.OllamaConfig == nil {
			return false
		}
		transformationConfig.OllamaConfig.ErrorChannel = ec
		return true
	case "embed":
		if transformationConfig.EmbedConfig == nil {
			return false
		}
		transformationConfig.EmbedConfig.ErrorChannel = ec
		return true
	case "vllm":
		if transformationConfig.VllmConfig == nil {
			return false
		}
		transformationConfig.VllmConfig.ErrorChannel = ec
		return true
	}
	return false
}

// defaultErrorChannelName names the channel after the operator that writes to it
// and its position in the step. The name is what lands in the error_channel column
// of process_errors (write2Chan reads it off the channel), so it is written to be
// read by whoever is looking at the row rather than to be short.
func defaultErrorChannelName(operatorType string, pipeIdx, applyIdx int, taken map[string]bool) string {
	name := fmt.Sprintf("%s.%s.%d.%d", DefaultErrorChannelSpecName, operatorType, pipeIdx, applyIdx)
	if !taken[name] {
		return name
	}
	for n := 2; ; n++ {
		candidate := fmt.Sprintf("%s.%d", name, n)
		if !taken[candidate] {
			return candidate
		}
	}
}

// ensureDefaultErrorChannelSpec adds the shared channel spec unless the config
// already declares one by that name, in which case the author's wins.
func ensureDefaultErrorChannelSpec(cpConfig *ComputePipesConfig) {
	if cpConfig.GetChannelSpec(DefaultErrorChannelSpecName) != nil {
		return
	}
	columns := make([]string, len(DefaultProcessErrorColumns))
	copy(columns, DefaultProcessErrorColumns)
	cpConfig.Channels = append(cpConfig.Channels, ChannelSpec{
		Comment: "synthesised by SynthesizeDefaultErrorChannels: the columns of " +
			DefaultErrorTableName + " a row-level failure is reported on",
		Name:    DefaultErrorChannelSpecName,
		Columns: columns,
	})
}

// ensureDefaultErrorTable adds the output_tables entry unless the config already
// carries one under that key.
func ensureDefaultErrorTable(cpConfig *ComputePipesConfig) {
	for _, tbl := range cpConfig.OutputTables {
		if tbl != nil && tbl.Key == DefaultErrorTableKey {
			return
		}
	}
	cpConfig.OutputTables = append(cpConfig.OutputTables, &TableSpec{
		Comment: "synthesised by SynthesizeDefaultErrorChannels; the table already " +
			"exists, so no column metadata is carried -- see PrepareOutoutTable",
		Key:             DefaultErrorTableKey,
		Name:            DefaultErrorTableName,
		ChannelSpecName: DefaultErrorChannelSpecName,
	})
}

// defaultErrorTableIsActive says whether the step carries the synthesised
// output_tables entry, which is the same thing as saying a writer is about to drain
// the sink channel and something has to close it.
func defaultErrorTableIsActive(cpConfig *ComputePipesConfig) bool {
	for _, tbl := range cpConfig.OutputTables {
		if tbl != nil && tbl.Key == DefaultErrorTableKey {
			return true
		}
	}
	return false
}

// isDefaultErrorChannel says whether an error channel is bound to the default
// table. It is the one test, called from both SelectActiveOutputTable -- which
// decides whether the table survives pruning -- and defaultErrorChannelNames, which
// decides what feeds it. Two tests here would let the set that keeps the table alive
// and the set that feeds it come apart, and a table whose channel nothing closes
// hangs the step.
func isDefaultErrorChannel(errorChannel *OutputChannelConfig) bool {
	return errorChannel != nil &&
		errorChannel.SpecName == DefaultErrorChannelSpecName &&
		len(errorChannel.Name) > 0
}

// defaultErrorChannelNames returns the error channels bound to the default table, in
// configuration order.
func defaultErrorChannelNames(pipeConfig []PipeSpec) []string {
	names := make([]string, 0)
	seen := make(map[string]bool)
	for i := range pipeConfig {
		for j := range pipeConfig[i].Apply {
			ec := errorChannelConfig(&pipeConfig[i].Apply[j])
			if !isDefaultErrorChannel(ec) {
				continue
			}
			if seen[ec.Name] {
				continue
			}
			seen[ec.Name] = true
			names = append(names, ec.Name)
		}
	}
	return names
}

// startDefaultErrorChannelFanIn copies every error channel bound to the default
// table into the sink channel that table's writer drains, and closes the sink once
// the last of them has closed. It returns the number of copiers it started.
//
// The sink is closed here and nowhere else: it is no operator's output channel, so
// neither pipe executor's closing set contains it. That makes closing it an
// obligation rather than a step -- the writer's CopyFrom returns when its source
// channel closes and not before -- so the fan-in closes the sink whenever the table
// is active, including when it finds no source channels at all. A writer that
// copies nothing finishes; a writer whose channel never closes hangs the step.
//
// Both directions select on done so that an interrupted step cannot leave a copier
// blocked on a send, which would leave the operator on the other end blocked on its
// own.
//
// It is a no-op when the default table is not active, which is every configuration
// that wires its error channels by hand.
func startDefaultErrorChannelFanIn(reg *ChannelRegistry, cpConfig *ComputePipesConfig, done chan struct{}) int {
	if reg == nil || cpConfig == nil || !defaultErrorTableIsActive(cpConfig) {
		return 0
	}
	names := defaultErrorChannelNames(cpConfig.PipesConfig)
	sink := reg.ComputeChannels[DefaultErrorChannelSpecName]
	if sink == nil {
		// The writer is built from this same name, so if the sink is absent the
		// writer is too and the rows have nowhere to go. Saying so once is better
		// than blocking every operator that reports one.
		log.Printf("warning: built-in error reporting: sink channel '%s' is not in the channel registry, "+
			"%d error channel(s) bound to %s will not be written",
			DefaultErrorChannelSpecName, len(names), DefaultErrorTableName)
		return 0
	}
	var wg sync.WaitGroup
	started := 0
	for _, name := range names {
		source := reg.ComputeChannels[name]
		if source == nil {
			continue
		}
		wg.Add(1)
		started++
		go func(source *Channel) {
			defer wg.Done()
			for {
				select {
				case row, ok := <-source.Channel:
					if !ok {
						return
					}
					select {
					case sink.Channel <- row:
					case <-done:
						return
					}
				case <-done:
					return
				}
			}
		}(source)
	}
	go func() {
		wg.Wait()
		reg.CloseChannel(DefaultErrorChannelSpecName)
	}()
	return started
}
