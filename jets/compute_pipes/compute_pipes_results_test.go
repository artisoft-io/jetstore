package compute_pipes

import (
	"fmt"
	"testing"

	"github.com/jackc/pgx/v5"
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

// The sql sink's destination is the same runtime value as its entity, in URI
// notation: it buys uniformity with the file sinks rather than information.
func TestAggregateChannelResults_SqlLocation(t *testing.T) {
	wt := NewWriteTableSource(nil, pgx.Identifier{"jetsapi", "process_errors"}, nil,
		"process_errors", "infer_errors", "process_errors", nil, nil)
	details := AggregateChannelResults([]ComputePipesResult{wt.result()})
	if len(details) != 1 {
		t.Fatalf("expecting 1 row, got %d", len(details))
	}
	if details[0].OutputLocation != `sql://"jetsapi"."process_errors"` {
		t.Errorf("expecting the schema-qualified runtime identifier, got %q", details[0].OutputLocation)
	}
	if details[0].OutputEntity != `"jetsapi"."process_errors"` {
		t.Errorf("expecting the entity unchanged, got %q", details[0].OutputEntity)
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
//
// The invariant is coalesce(sum(child), 0) = parent rather than
// sum(child) = parent, because a merge edge records NULL: it has no row count
// and 0 would be a measurement it did not make. A merge worker satisfies it at
// 0 on both sides, which is the stated cost of the merge row — the missing-set
// check is weaker for merge steps specifically.
func TestAggregateChannelResults_SumMatchesWorkerTotal(t *testing.T) {
	results := []ComputePipesResult{
		{Type: SinkDbTable, EntityName: "jetsapi.process_errors", InputChannel: "errors",
			OutputChannel: "process_errors", CopyRowCount: 7},
		{Type: SinkJetsPartition, EntityName: "0000P", InputChannel: "input_row",
			OutputChannel: "partition_out", CopyRowCount: 100},
		{Type: SinkJetsPartition, EntityName: "0001P", InputChannel: "input_row",
			OutputChannel: "partition_out", CopyRowCount: 200},
		// The merge edge, which contributes nothing to either side: it is the
		// worker's own row count that the parent carries, and a merge does not
		// have one.
		{Type: SinkOutputFile, InputChannel: "input_row", OutputChannel: "merged_file",
			OutputLocation: "s3://jetstore-bucket/out/merged.csv", PartsCount: 1,
			RowCountUnknown: true},
	}
	var workerTotal int64
	for i := range results {
		if results[i].RowCountUnknown {
			continue
		}
		workerTotal += results[i].CopyRowCount
	}
	// coalesce(sum(child), 0): a row with an unknown count contributes nothing,
	// exactly as a NULL does under the SQL sum.
	var childTotal int64
	var nullRows int
	for _, d := range AggregateChannelResults(results) {
		if d.RowCountUnknown {
			nullRows++
			continue
		}
		childTotal += d.RowCount
	}
	if childTotal != workerTotal {
		t.Errorf("child rows sum to %d, worker total is %d", childTotal, workerTotal)
	}
	if nullRows != 1 {
		t.Errorf("expecting the merge edge to carry an unknown row count, got %d such rows", nullRows)
	}
}

// A merge_files worker: one synthetic edge, a destination, and no row count. It
// is the whole of what makes a merge step visible to a reading of the record —
// without it the worker writes no child row at all.
func TestAggregateChannelResults_MergeFilesIsOneEdge(t *testing.T) {
	results := []ComputePipesResult{
		{
			Type:            SinkOutputFile,
			InputChannel:    "input_row",
			OutputChannel:   "merged_file",
			OutputLocation:  "s3://jetstore-bucket/client=acme/merged.csv",
			PartsCount:      1,
			RowCountUnknown: true,
		},
	}
	details := AggregateChannelResults(results)
	if len(details) != 1 {
		t.Fatalf("expecting 1 row for the merge, got %d", len(details))
	}
	d := details[0]
	if d.OutputType != "output_file" {
		t.Errorf("expecting output_type output_file, got %q", d.OutputType)
	}
	if d.InputChannel != "input_row" || d.OutputChannel != "merged_file" {
		t.Errorf("unexpected edge: %v", d)
	}
	if d.OutputLocation != "s3://jetstore-bucket/client=acme/merged.csv" {
		t.Errorf("expecting the destination carried, got %q", d.OutputLocation)
	}
	if d.SinksCount != 1 {
		t.Errorf("expecting a single sink, got %d", d.SinksCount)
	}
	if d.PartsCount != 1 {
		t.Errorf("expecting 1 part, got %d", d.PartsCount)
	}
	if !d.RowCountUnknown {
		t.Error("expecting the row count reported as unknown, so the insert writes NULL rather than 0")
	}
	// The two empties are meaningful under output_file: an OutputFileSpec has
	// no channel_spec_name, and the file is named by OutputChannel already.
	if d.OutputChannelSpec != "" || d.OutputEntity != "" {
		t.Errorf("expecting no channel spec and no entity on a merge row, got %q and %q",
			d.OutputChannelSpec, d.OutputEntity)
	}
}

// A memory edge has no physical location, and that is information rather than
// absence: the data never left the process. output_type is the discriminant, so
// an empty location under a known type must survive as empty.
func TestAggregateChannelResults_EmptyLocationIsNotUnknown(t *testing.T) {
	results := []ComputePipesResult{
		{Type: SinkJetsPartition, EntityName: "0000P", InputChannel: "input_row",
			OutputChannel: "memory_out", OutputChannelSpec: "row_spec", CopyRowCount: 42},
	}
	details := AggregateChannelResults(results)
	if len(details) != 1 {
		t.Fatalf("expecting 1 row, got %d", len(details))
	}
	d := details[0]
	if d.OutputLocation != "" {
		t.Errorf("expecting the empty location kept, got %q", d.OutputLocation)
	}
	if d.OutputType != SinkJetsPartition {
		t.Errorf("expecting the type kept as the discriminant, got %q", d.OutputType)
	}
	if d.RowCountUnknown {
		t.Error("an absent location is not an absent count: the row count is known here")
	}
	if d.RowCount != 42 {
		t.Errorf("expecting 42 rows, got %d", d.RowCount)
	}
}

// A splitter's sinks share one destination prefix, so the edge carries it
// unchanged while output_entity still blanks. A per-sink URI here would
// reintroduce the aggregation this table exists to remove.
func TestAggregateChannelResults_SplitterCarriesThePrefix(t *testing.T) {
	const prefix = "s3://jetstore-bucket/stage/process_name=qc/session_id=123/step_id=reduce01"
	results := make([]ComputePipesResult, 0)
	for i := range 5 {
		results = append(results, ComputePipesResult{
			Type:              SinkJetsPartition,
			EntityName:        fmt.Sprintf("%04dP", i),
			InputChannel:      "input_row",
			OutputChannel:     "partition_out",
			OutputChannelSpec: "partition_spec",
			// The same string on every sink: the writer removes the
			// jets_partition segment before reporting it.
			OutputLocation: prefix,
			CopyRowCount:   int64(10 * (i + 1)),
			PartsCount:     2,
		})
	}
	details := AggregateChannelResults(results)
	if len(details) != 1 {
		t.Fatalf("expecting the 5 sinks folded into 1 edge, got %d rows", len(details))
	}
	d := details[0]
	if d.OutputLocation != prefix {
		t.Errorf("expecting the shared prefix on the edge, got %q", d.OutputLocation)
	}
	if d.OutputEntity != "" {
		t.Errorf("expecting no single sink named on an aggregate row, got %q", d.OutputEntity)
	}
	if d.SinksCount != 5 || d.RowCount != 150 {
		t.Errorf("expecting 5 sinks and 150 rows, got %d and %d", d.SinksCount, d.RowCount)
	}
}

// One sink that cannot count makes the whole edge uncountable: summing a number
// with a non-number gives a number that means nothing.
func TestAggregateChannelResults_UnknownCountFoldsAcrossSinks(t *testing.T) {
	results := []ComputePipesResult{
		{Type: SinkJetsPartition, EntityName: "0000P", InputChannel: "input_row",
			OutputChannel: "partition_out", CopyRowCount: 10},
		{Type: SinkJetsPartition, EntityName: "0001P", InputChannel: "input_row",
			OutputChannel: "partition_out", RowCountUnknown: true},
	}
	details := AggregateChannelResults(results)
	if len(details) != 1 {
		t.Fatalf("expecting 1 edge, got %d", len(details))
	}
	if !details[0].RowCountUnknown {
		t.Error("expecting the edge total reported as unknown when one sink cannot count")
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
