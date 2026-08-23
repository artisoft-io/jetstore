package compute_pipes

import (
	"fmt"
	"testing"
)

// AggregateChannelResults is the whole of the mapping from what the writers
// report to what jetsapi.pipeline_execution_channel_details records, so it is
// tested here rather than through the database — the insert itself needs a
// Postgres and is exercised by a running pipeline.

// A worker with two output tables and no splitter: one row per table, each
// naming the channel that fed it and the output table's key.
func TestAggregateChannelResults_OutputTables(t *testing.T) {
	// Both entries of patient_profile.pc.json: two output_tables keys, one
	// channel_spec_name, one table.
	results := []ComputePipesResult{
		{Type: SinkDbTable, EntityName: "jetsapi.process_errors", InputChannel: "process_errors",
			OutputChannel: "infer_errors", OutputChannelSpec: "process_errors", CopyRowCount: 12},
		{Type: SinkDbTable, EntityName: "jetsapi.process_errors", InputChannel: "process_errors",
			OutputChannel: "process_errors", OutputChannelSpec: "process_errors", CopyRowCount: 3},
	}
	details := AggregateChannelResults(results)
	if len(details) != 2 {
		t.Fatalf("expecting 2 rows, got %d", len(details))
	}
	// Two output_tables entries writing the same table are two edges: the
	// table name does not distinguish them and the key does.
	if details[0].OutputChannel != "infer_errors" || details[1].OutputChannel != "process_errors" {
		t.Errorf("unexpected edges: %v", details)
	}
	for i := range details {
		if details[i].SinksCount != 1 {
			t.Errorf("row %d: expecting a single sink, got %d", i, details[i].SinksCount)
		}
		if details[i].OutputEntity != "jetsapi.process_errors" {
			t.Errorf("row %d: expecting the table name carried, got %q", i, details[i].OutputEntity)
		}
	}
	if details[0].RowCount != 12 || details[1].RowCount != 3 {
		t.Errorf("unexpected row counts: %v", details)
	}
	for i := range details {
		if details[i].OutputChannelSpec != "process_errors" {
			t.Errorf("row %d: expecting the channel spec carried, got %q", i, details[i].OutputChannelSpec)
		}
	}
}

// A splitter writing one output channel into many jets_partitions: one row for
// the edge, the counts summed, and no partition label pretending to be the
// sink of the aggregate.
func TestAggregateChannelResults_SplitterFoldsToOneEdge(t *testing.T) {
	results := make([]ComputePipesResult, 0)
	for i := range 5 {
		results = append(results, ComputePipesResult{
			Type:              SinkJetsPartition,
			EntityName:        fmt.Sprintf("%04dP", i),
			InputChannel:      "input_row",
			OutputChannel:     "partition_out",
			OutputChannelSpec: "partition_spec",
			CopyRowCount:      int64(10 * (i + 1)),
			PartsCount:        2,
		})
	}
	details := AggregateChannelResults(results)
	if len(details) != 1 {
		t.Fatalf("expecting the 5 partitions folded into 1 edge, got %d rows", len(details))
	}
	d := details[0]
	if d.SinksCount != 5 {
		t.Errorf("expecting SinksCount 5, got %d", d.SinksCount)
	}
	if d.RowCount != 150 || d.PartsCount != 10 {
		t.Errorf("expecting 150 rows over 10 parts, got %d over %d", d.RowCount, d.PartsCount)
	}
	if d.OutputEntity != "" {
		t.Errorf("expecting no single sink named on an aggregate row, got %q", d.OutputEntity)
	}
	if d.InputChannel != "input_row" || d.OutputChannel != "partition_out" {
		t.Errorf("unexpected edge: %v", d)
	}
	if d.OutputChannelSpec != "partition_spec" {
		t.Errorf("expecting the channel spec carried onto the aggregate row, got %q", d.OutputChannelSpec)
	}
}

// The sum over the child rows is the worker's output_records_count: the parent
// row stays the authority and a missing child set is detectable rather than
// silent.
func TestAggregateChannelResults_SumMatchesWorkerTotal(t *testing.T) {
	results := []ComputePipesResult{
		{Type: SinkDbTable, EntityName: "jetsapi.process_errors", InputChannel: "errors",
			OutputChannel: "process_errors", CopyRowCount: 7},
		{Type: SinkJetsPartition, EntityName: "0000P", InputChannel: "input_row",
			OutputChannel: "partition_out", CopyRowCount: 100},
		{Type: SinkJetsPartition, EntityName: "0001P", InputChannel: "input_row",
			OutputChannel: "partition_out", CopyRowCount: 200},
	}
	var workerTotal int64
	for i := range results {
		workerTotal += results[i].CopyRowCount
	}
	var childTotal int64
	for _, d := range AggregateChannelResults(results) {
		childTotal += d.RowCount
	}
	if childTotal != workerTotal {
		t.Errorf("child rows sum to %d, worker total is %d", childTotal, workerTotal)
	}
}

// Two different input channels into the same output channel are two edges: the
// pair is the key, not either half of it.
func TestAggregateChannelResults_KeyIsThePair(t *testing.T) {
	results := []ComputePipesResult{
		{Type: SinkJetsPartition, EntityName: "0000P", InputChannel: "chan_b",
			OutputChannel: "partition_out", CopyRowCount: 5},
		{Type: SinkJetsPartition, EntityName: "0000P", InputChannel: "chan_a",
			OutputChannel: "partition_out", CopyRowCount: 6},
	}
	details := AggregateChannelResults(results)
	if len(details) != 2 {
		t.Fatalf("expecting 2 edges, got %d", len(details))
	}
	// Ordered, so the row set does not depend on which writer finished first.
	if details[0].InputChannel != "chan_a" || details[1].InputChannel != "chan_b" {
		t.Errorf("expecting a deterministic order, got %v", details)
	}
}

// An error on one sink is carried on its edge and does not contaminate another.
func TestAggregateChannelResults_ErrorsAreCarriedPerEdge(t *testing.T) {
	results := []ComputePipesResult{
		{Type: SinkDbTable, EntityName: "jetsapi.t1", InputChannel: "c1", OutputChannel: "t1",
			Err: fmt.Errorf("boom")},
		{Type: SinkDbTable, EntityName: "jetsapi.t2", InputChannel: "c2", OutputChannel: "t2",
			CopyRowCount: 4},
	}
	details := AggregateChannelResults(results)
	if len(details) != 2 {
		t.Fatalf("expecting 2 rows, got %d", len(details))
	}
	if details[0].ErrMsg != "boom" {
		t.Errorf("expecting the error on its own edge, got %q", details[0].ErrMsg)
	}
	if details[1].ErrMsg != "" {
		t.Errorf("expecting no error on the other edge, got %q", details[1].ErrMsg)
	}
}

func TestAggregateChannelResults_Empty(t *testing.T) {
	if len(AggregateChannelResults(nil)) != 0 {
		t.Error("expecting no rows for a worker that wrote nothing")
	}
}
