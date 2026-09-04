package compute_pipes

import (
	"database/sql"
	"log"
	"strconv"
	"strings"
)

// This file contains the code for reporting the errors from the jetrules pool manager and workers
// These errors are sent to the error output channel of the jetrules operator, they are generally loaded
// onto the table jetsapi.process_errors.

type ProcessError struct {
	PEKey              int64
	SessionId          string
	GroupingKey        sql.NullString
	RowJetsKey         sql.NullString
	InputColumn        sql.NullString
	ErrorMessage       string
	ReteSessionSaved   string
	ReteSessionTriples sql.NullString
	ShardId            int
	// CpipesStepId is the step the failing operator ran in. It is the same string
	// UpdatePipelineExecutionStatus writes to pipeline_execution_details.cpipes_step_id
	// for this worker, so an error row joins to its worker row on
	// (session_id, cpipes_step_id, shard_id) rather than to every step of the run.
	CpipesStepId string
	// OperatorType is the operator's type as the configuration spells it -- jetrules,
	// map_record, ollama, embed, vllm. Taken from the construction site rather than
	// recovered from the message prefix, which map_record does not write.
	OperatorType string
}

// NewProcessError builds the row an operator reports a record-level failure on.
// operatorType is the operator's configuration type; it is a parameter rather than
// a derived value because it is the one discriminator no caller can recover later:
// the four jetrules sites and the map_record site write no prefix of their own.
func (ctx *BuilderContext) NewProcessError(operatorType string) ProcessError {
	peRow := ProcessError{
		PEKey:            int64(ctx.peKey),
		SessionId:        ctx.sessionId,
		ReteSessionSaved: "N",
		ShardId:          ctx.nodeId,
		OperatorType:     operatorType,
	}
	if ctx.cpConfig != nil && ctx.cpConfig.CommonRuntimeArgs != nil {
		peRow.CpipesStepId = ctx.cpConfig.CommonRuntimeArgs.MainInputStepId
	}
	return peRow
}
func (peRow ProcessError) String() string {
	var buf strings.Builder
	buf.WriteString(strconv.FormatInt(peRow.PEKey, 10))
	buf.WriteString(" | ")
	if peRow.SessionId != "" {
		buf.WriteString(peRow.SessionId)
	} else {
		buf.WriteString("NULL")
	}
	buf.WriteString(" | ")
	if peRow.GroupingKey.Valid {
		buf.WriteString(peRow.GroupingKey.String)
	} else {
		buf.WriteString("NULL")
	}
	buf.WriteString(" | ")
	if peRow.RowJetsKey.Valid {
		buf.WriteString(peRow.RowJetsKey.String)
	} else {
		buf.WriteString("NULL")
	}
	buf.WriteString(" | ")
	if peRow.InputColumn.Valid {
		buf.WriteString(peRow.InputColumn.String)
	} else {
		buf.WriteString("NULL")
	}
	buf.WriteString(" | ")
	if peRow.ErrorMessage != "" {
		buf.WriteString(peRow.ErrorMessage)
	} else {
		buf.WriteString("NULL")
	}
	buf.WriteString(" | ")
	buf.WriteString(peRow.ReteSessionSaved)
	buf.WriteString(" | ")
	buf.WriteString(peRow.OperatorType)
	buf.WriteString(" | ")
	buf.WriteString(peRow.CpipesStepId)
	return buf.String()
}

// ProcessErrorDiscriminatorColumns are the columns that say which operator, which
// channel and which step produced an error row. They are the widening this table
// took for triage; a channel spec written before it does not declare them, so
// write2Chan places them only when the channel has a slot for them.
var ProcessErrorDiscriminatorColumns = []string{"cpipes_step_id", "error_channel", "operator_type"}

// setColumn places v in row at the channel's slot for the named column, and does
// nothing when the channel does not declare it. The comma-ok form is what makes the
// discriminator columns additive: a map lookup that misses returns 0, so the
// unguarded form would have written every undeclared column over row[0].
func setColumn(row []any, outCh *OutputChannel, column string, v any) {
	pos, ok := (*outCh.Columns)[column]
	if !ok || pos >= len(row) {
		return
	}
	row[pos] = v
}

// wrtie ProcessError to ch as slice of interfaces
func (peRow ProcessError) write2Chan(outCh *OutputChannel, doneCh chan struct{}) {

	row := make([]any, len(outCh.Config.Columns))

	setColumn(row, outCh, "pipeline_execution_status_key", peRow.PEKey)
	setColumn(row, outCh, "session_id", peRow.SessionId)
	setColumn(row, outCh, "grouping_key", peRow.GroupingKey)
	setColumn(row, outCh, "row_jets_key", peRow.RowJetsKey)
	setColumn(row, outCh, "input_column", peRow.InputColumn)
	setColumn(row, outCh, "error_message", peRow.ErrorMessage)
	setColumn(row, outCh, "rete_session_saved", peRow.ReteSessionSaved)
	setColumn(row, outCh, "rete_session_triples", peRow.ReteSessionTriples)
	setColumn(row, outCh, "shard_id", peRow.ShardId)

	// The three discriminators. error_channel is read off the channel the row is
	// going to, so it needs no plumbing; the other two are carried on the row.
	// cpipes_step_id is written as it stands rather than as NULL when empty: a
	// reducing step can legitimately carry an empty label, and the worker row
	// records that same empty string, so mapping it to NULL here would break the
	// join it exists to make.
	setColumn(row, outCh, "cpipes_step_id", peRow.CpipesStepId)
	setColumn(row, outCh, "error_channel", outCh.Name)
	setColumn(row, outCh, "operator_type", peRow.OperatorType)

	// Send out the row
	log.Println("*** ERROR ROW: ", peRow.String())
	select {
	case outCh.Channel <- row:
	case <-doneCh:
		log.Println("Write ProcessError interrupted")
	}
}
