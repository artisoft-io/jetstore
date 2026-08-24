package compute_pipes

import (
	"cmp"
	"context"
	"fmt"
	"log"
	"slices"

	"github.com/jackc/pgx/v5/pgxpool"
)

// Sink kinds reported by ComputePipesResult.Type. They are the discriminant
// for EntityName and for how the other fields are to be read.
const (
	SinkDbTable       = "db_table"       // written by WriteTableSource (write_table.go)
	SinkJetsPartition = "jets_partition" // written by PartitionWriterTransformationPipe
	// SinkOutputFile is the merge_files operator's single output file. It is not
	// an output *channel*: OutputFiles sits on the config and a merge_files pipe
	// names one through PipeSpec.OutputFile, so there is no edge in the compute
	// graph to report and the row is synthesised by StartMergeFiles. The pair
	// (input channel, output channel) degenerates to (the merge input channel,
	// the OutputFileSpec key), which is the same degeneracy SinkDbTable already
	// carries for output_tables[].key.
	SinkOutputFile = "output_file" // written by StartMergeFiles (pipe_executor_merge_files.go)
)

// ComputePipesResult is what a writer reports back when it is done: which edge
// of the compute graph it was, how many rows crossed it, and how many file
// parts it produced.
//
// EntityName was called TableName and held either a schema-qualified table name
// or the string "jets_partition=<label>" depending on which writer sent it —
// an overloaded field with its discriminant smuggled into the value. Type now
// carries the discriminant and EntityName holds the identity alone.
//
// InputChannel and OutputChannel are the configured names of the DAG edge this
// result belongs to. They are what jetsapi.pipeline_execution_channel_details
// is keyed on, and neither is derivable from EntityName: a table name is not
// the channel that fed it (30 of the 42 output_tables entries in the rule
// corpus have a `key` differing from the table `name`, and two entries of
// patient_profile.pc.json write the same jetsapi.process_errors table), and a
// jets_partition label is a shard rather than an edge.
type ComputePipesResult struct {
	// Type is the sink kind: SinkDbTable or SinkJetsPartition.
	Type string
	// EntityName is the sink's identity in its own namespace: the
	// schema-qualified table for SinkDbTable (env vars already substituted),
	// the jets_partition label for SinkJetsPartition.
	EntityName string
	// InputChannel is the configured name of the channel the writer reads.
	InputChannel string
	// OutputChannel is the configured name of the writer's output in the
	// cpipes config: output_tables[].key for SinkDbTable — the name the rest
	// of the config refers to that output by — and the output_channel name for
	// SinkJetsPartition.
	OutputChannel string
	// OutputChannelSpec is that output's channel_spec_name: OutputChannelConfig.SpecName
	// (pipes_model.go, JSON channel_spec_name) or TableSpec.ChannelSpecName.
	// It is required — validateOutputChannel (actions_start_common.go, the
	// "sql" arm) errors when it cannot be derived, and the graph build errors
	// at "channel spec %s not found in Channel Registry" (compute_pipes.go)
	// when it is absent — and it is guaranteed *different* from OutputChannel,
	// since the same validator refuses output_channel.name equal to
	// channel_spec_name. So the two columns are the edge and the shape of what
	// crosses it, and neither substitutes for the other.
	OutputChannelSpec string
	// OutputLocation is where the data was physically written, as a URI with
	// env vars already substituted: "s3://<bucket>/<prefix>" for a file sink,
	// "sql://<schema>.<table>" for a database one. It is the prefix rather than
	// the per-sink path — a splitter writes one output channel into many
	// jets_partitions and the edge row is their aggregate, so a per-sink path
	// would reintroduce the aggregation the edge grain exists to remove.
	//
	// Empty is meaningful rather than unknown, and Type is the discriminant: a
	// "memory" edge never left the process and has no physical location. A
	// reader must not take an empty location under a known Type for a lost
	// write.
	OutputLocation string
	// RowCountUnknown says that CopyRowCount is not a measurement: no row count
	// exists for this sink. The merge_files multipart-copy path moves bytes and
	// never parses a record, so 0 there would be a collapse to any detector
	// reading it. The zero value means the count is known, so every writer that
	// counts rows is untouched.
	RowCountUnknown bool
	// PartsCount is nbr of file part in jets_partition
	CopyRowCount int64
	PartsCount   int64
	Err          error
}
type LoadFromS3FilesResult struct {
	LoadRowCount int64
	BadRowCount  int64
	Err          error
}
type JetrulesWorkerResult struct {
	ReteSessionCount int64
	ErrorsCount      int64
	Err              error
}
type ClusteringResult struct {
	Err error
}

// ChannelResults holds the channel reporting back results.
// LoadFromS3FilesResultCh: results from loading files (row count)
// Copy2DbResultCh: results of records written to JetStore DB (row count)
// WritePartitionsResultCh: report on rows output to s3 (row count)
// S3PutObjectResultCh: reports on nbr of files put to s3 (file count)
// JetrulesWorkerResultCh: reports on nbr of rete session and errors
// ClusteringResultCh: reports on nbr of clusters identified and errors
type ChannelResults struct {
	LoadFromS3FilesResultCh chan LoadFromS3FilesResult
	Copy2DbResultCh         chan chan ComputePipesResult
	WritePartitionsResultCh chan chan ComputePipesResult
	S3PutObjectResultCh     chan ComputePipesResult
	JetrulesWorkerResultCh  chan chan JetrulesWorkerResult
	ClusteringResultCh      chan chan ClusteringResult
}

type SaveResultsContext struct {
	dbpool        *pgxpool.Pool
	JetsPartition string
	NodeId        int
	SessionId     string
}

func NewSaveResultsContext(dbpool *pgxpool.Pool) *SaveResultsContext {
	return &SaveResultsContext{dbpool: dbpool}
}

func (ctx *SaveResultsContext) Save(category string, result *ComputePipesResult) {
	if result == nil {
		return
	}
	stmt := `INSERT INTO jetsapi.cpipes_results (
		session_id, jets_partition, node_id, category, name, row_count, parts_count, err) 
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)`
	jetsPartition := ctx.JetsPartition
	nodeId := ctx.NodeId
	sessionId := ctx.SessionId
	var errMsg string
	if result.Err != nil {
		errMsg = result.Err.Error()
	}
	_, err := ctx.dbpool.Exec(context.Background(), stmt, sessionId, jetsPartition, nodeId, category,
		result.EntityName, result.CopyRowCount, result.PartsCount, errMsg)
	if err != nil {
		log.Printf("error inserting in jetsapi.cpipes_results table: %v", err)
		return
	}
}

// ChannelExecutionDetail is one row of jetsapi.pipeline_execution_channel_details:
// one edge of the worker's compute graph — an (input channel, output channel)
// pair — with the rows and file parts that crossed it.
//
// The parent row in pipeline_execution_details is one per worker and its
// output_records_count is the sum over these, which is why this table is a
// child of it rather than a re-graining of it: three readers in
// jets/datatable/status_update.go (:53, :75 and cpipes_execution_status_details
// at :249), one in jets/compute_pipes/actions_start_reducing_cp.go (:61,
// main_input_row_count) and one in jets/run_reports/delegate/run_reports.go
// (:615) read the worker grain, and the input-side sums among them would
// multiply by the output fan-out under any other.
//
// SinksCount is the number of sink instances folded into the row. A splitter
// writes one output channel into many jets_partitions — one result each, up to
// the 15000-buffer of pipes_runtime_model.go:218 — and the edge is their sum;
// EntityName is carried only when the row has a single sink, so an aggregate is
// never mistaken for an instance.
//
// OutputLocation is the destination of the edge, identical across the sinks it
// folds; RowCountUnknown makes RowCount NULL in the row rather than 0, since 0
// is a measurement and NULL is "not measurable here".
type ChannelExecutionDetail struct {
	InputChannel      string
	OutputChannel     string
	OutputChannelSpec string
	OutputType        string
	OutputEntity      string
	OutputLocation    string
	SinksCount        int
	RowCount          int64
	RowCountUnknown   bool
	PartsCount        int64
	ErrMsg            string
}

// AggregateChannelResults folds a worker's writer results into one row per DAG
// edge, keyed on (Type, InputChannel, OutputChannel). The result is ordered so
// that it does not depend on the order the writers happened to finish in.
func AggregateChannelResults(results []ComputePipesResult) []ChannelExecutionDetail {
	type edgeKey struct {
		outputType    string
		inputChannel  string
		outputChannel string
	}
	byEdge := make(map[edgeKey]*ChannelExecutionDetail)
	order := make([]edgeKey, 0)
	for i := range results {
		r := &results[i]
		k := edgeKey{r.Type, r.InputChannel, r.OutputChannel}
		d := byEdge[k]
		if d == nil {
			d = &ChannelExecutionDetail{
				InputChannel:      r.InputChannel,
				OutputChannel:     r.OutputChannel,
				OutputChannelSpec: r.OutputChannelSpec,
				OutputType:        r.Type,
				OutputEntity:      r.EntityName,
				// The location is the edge's, not the sink's: it is taken from
				// the first result and is the same string on all of them, the
				// same way OutputChannelSpec is.
				OutputLocation: r.OutputLocation,
			}
			byEdge[k] = d
			order = append(order, k)
		}
		d.SinksCount += 1
		d.RowCount += r.CopyRowCount
		// One sink that cannot count makes the edge total not a measurement:
		// summing a number with a non-number gives a number that means nothing.
		d.RowCountUnknown = d.RowCountUnknown || r.RowCountUnknown
		d.PartsCount += r.PartsCount
		if r.Err != nil {
			if len(d.ErrMsg) > 0 {
				d.ErrMsg += ","
			}
			d.ErrMsg += r.Err.Error()
		}
	}
	details := make([]ChannelExecutionDetail, 0, len(order))
	for _, k := range order {
		d := byEdge[k]
		if d.SinksCount > 1 {
			// The row is the sum over several sinks; naming one of them would
			// be the aggregation this table exists to remove.
			d.OutputEntity = ""
		}
		details = append(details, *d)
	}
	slices.SortFunc(details, func(a, b ChannelExecutionDetail) int {
		return cmp.Or(
			cmp.Compare(a.OutputType, b.OutputType),
			cmp.Compare(a.InputChannel, b.InputChannel),
			cmp.Compare(a.OutputChannel, b.OutputChannel))
	})
	return details
}

// InsertChannelExecutionDetails records the worker's per-channel detail rows as
// children of its pipeline_execution_details row.
//
// It logs and returns the error rather than failing the worker: this record is
// additive observability, and a pipeline that has run correctly should not be
// reported as failed because a detail row did not insert (the table is created
// by update_db, which a deployment can lag behind). The caller does not
// propagate it. A missing or partial child set is detectable rather than
// silent — the parent's output_records_count is the sum over these rows, so
// sum(child) != parent is the check. That check is
// coalesce(sum(child), 0) != parent since a merge step records a NULL row
// count, and it is correspondingly weaker for merge steps: they satisfy it at 0
// on both sides. That is the price of not inventing a row count for a path that
// moves bytes without parsing a record.
func InsertChannelExecutionDetails(dbpool *pgxpool.Pool, pipelineExecutionDetailsKey int,
	sessionId string, details []ChannelExecutionDetail) error {
	if len(details) == 0 {
		return nil
	}
	stmt := `INSERT INTO jetsapi.pipeline_execution_channel_details (
		pipeline_execution_details_key, session_id, input_channel, output_channel,
		output_channel_spec, output_type, output_entity, output_location,
		output_sinks_count, output_records_count, parts_count, error_message)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12)`
	for i := range details {
		d := &details[i]
		// NULL rather than 0 when no row count exists for the sink; the column
		// is nullable, so this needs no schema change.
		var rowCount any = d.RowCount
		if d.RowCountUnknown {
			rowCount = nil
		}
		_, err := dbpool.Exec(context.Background(), stmt,
			pipelineExecutionDetailsKey, sessionId, d.InputChannel, d.OutputChannel,
			d.OutputChannelSpec, d.OutputType, d.OutputEntity, d.OutputLocation,
			d.SinksCount, rowCount, d.PartsCount, d.ErrMsg)
		if err != nil {
			err = fmt.Errorf("error inserting in jetsapi.pipeline_execution_channel_details table: %v", err)
			log.Println(err)
			return err
		}
	}
	return nil
}
