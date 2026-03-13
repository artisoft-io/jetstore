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
}

func (ctx *BuilderContext) NewProcessError() ProcessError {
	peRow := ProcessError{
		PEKey:            int64(ctx.peKey),
		SessionId:        ctx.sessionId,
		ReteSessionSaved: "N",
		ShardId:          ctx.nodeId,
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
	return buf.String()
}

// wrtie ProcessError to ch as slice of interfaces
func (peRow ProcessError) write2Chan(outCh *OutputChannel, doneCh chan struct{}) {

	row := make([]any, len(outCh.Config.Columns))

	row[(*outCh.Columns)["pipeline_execution_status_key"]] = peRow.PEKey
	row[(*outCh.Columns)["session_id"]] = peRow.SessionId
	row[(*outCh.Columns)["grouping_key"]] = peRow.GroupingKey
	row[(*outCh.Columns)["row_jets_key"]] = peRow.RowJetsKey
	row[(*outCh.Columns)["input_column"]] = peRow.InputColumn
	row[(*outCh.Columns)["error_message"]] = peRow.ErrorMessage
	row[(*outCh.Columns)["rete_session_saved"]] = peRow.ReteSessionSaved
	row[(*outCh.Columns)["rete_session_triples"]] = peRow.ReteSessionTriples
	row[(*outCh.Columns)["shard_id"]] = peRow.ShardId

	// Send out the row
	log.Println("*** ERROR ROW: ", peRow.String())
	select {
	case outCh.Channel <- row:
	case <-doneCh:
		log.Println("Write ProcessError interrupted")
	}
}
