package compute_pipes

import (
	"testing"
)

// newTestErrorChannel builds an OutputChannel over the given column list, the way
// the channel registry does from a channel spec.
func newTestErrorChannel(name string, columns []string) (*OutputChannel, chan []any) {
	columnsMap := make(map[string]int, len(columns))
	for i, c := range columns {
		columnsMap[c] = i
	}
	ch := make(chan []any, 1)
	return &OutputChannel{
		Name:    name,
		Channel: ch,
		Columns: &columnsMap,
		Config:  &ChannelSpec{Name: name, Columns: columns},
	}, ch
}

var legacyProcessErrorColumns = []string{
	"pipeline_execution_status_key", "session_id", "grouping_key", "row_jets_key",
	"input_column", "error_message", "rete_session_saved", "rete_session_triples", "shard_id",
}

// A channel spec that declares the discriminators gets them, and the step id is the
// one the worker row carries, so the two join.
func TestWrite2ChanWritesDiscriminators(t *testing.T) {
	columns := append(append([]string{}, legacyProcessErrorColumns...), ProcessErrorDiscriminatorColumns...)
	outCh, ch := newTestErrorChannel("process_errors.out", columns)

	ctx := &BuilderContext{
		peKey:     123,
		sessionId: "s1",
		nodeId:    7,
		cpConfig:  &ComputePipesConfig{CommonRuntimeArgs: &ComputePipesCommonArgs{MainInputStepId: "reducing01"}},
	}
	peRow := ctx.NewProcessError("jetrules")
	peRow.ErrorMessage = "jets:exception caught: boom"
	peRow.write2Chan(outCh, make(chan struct{}))

	row := <-ch
	got := map[string]any{}
	for name, pos := range *outCh.Columns {
		got[name] = row[pos]
	}
	if got["cpipes_step_id"] != "reducing01" {
		t.Errorf("cpipes_step_id: got %v, want reducing01", got["cpipes_step_id"])
	}
	if got["error_channel"] != "process_errors.out" {
		t.Errorf("error_channel: got %v, want process_errors.out", got["error_channel"])
	}
	if got["operator_type"] != "jetrules" {
		t.Errorf("operator_type: got %v, want jetrules", got["operator_type"])
	}
	if got["session_id"] != "s1" || got["shard_id"] != 7 {
		t.Errorf("the original columns must be unchanged, got session_id=%v shard_id=%v",
			got["session_id"], got["shard_id"])
	}
}

// The widening is additive: a channel spec written before it still produces the row
// it always did.
//
// This is the case the comma-ok lookup in setColumn exists for. The unguarded map
// lookup it replaced returns 0 for a column the channel does not declare, so each of
// the three discriminators would have been written over row[0] -- the
// pipeline_execution_status_key -- and the last one would have won. Nothing would
// have failed; the run key would simply have become the operator's name.
func TestWrite2ChanLeavesLegacyChannelsAlone(t *testing.T) {
	outCh, ch := newTestErrorChannel("process_errors.out", legacyProcessErrorColumns)

	ctx := &BuilderContext{
		peKey:     456,
		sessionId: "s2",
		nodeId:    3,
		cpConfig:  &ComputePipesConfig{CommonRuntimeArgs: &ComputePipesCommonArgs{MainInputStepId: "reducing01"}},
	}
	peRow := ctx.NewProcessError("map_record")
	peRow.ErrorMessage = "mapping error"
	peRow.write2Chan(outCh, make(chan struct{}))

	row := <-ch
	if len(row) != len(legacyProcessErrorColumns) {
		t.Fatalf("row width: got %d, want %d", len(row), len(legacyProcessErrorColumns))
	}
	if row[(*outCh.Columns)["pipeline_execution_status_key"]] != int64(456) {
		t.Errorf("pipeline_execution_status_key was overwritten: got %v", row[0])
	}
	if row[(*outCh.Columns)["error_message"]] != "mapping error" {
		t.Errorf("error_message: got %v", row[(*outCh.Columns)["error_message"]])
	}
}

// A reducing step can carry an empty cpipes_step_id, and the worker row records that
// same empty string. The error row must record it too rather than mapping it to
// something else, or the join the column exists for does not hold for those steps.
func TestNewProcessErrorKeepsAnEmptyStepId(t *testing.T) {
	ctx := &BuilderContext{
		cpConfig: &ComputePipesConfig{CommonRuntimeArgs: &ComputePipesCommonArgs{MainInputStepId: ""}},
	}
	if got := ctx.NewProcessError("jetrules").CpipesStepId; got != "" {
		t.Errorf("CpipesStepId: got %q, want the empty string", got)
	}
}
