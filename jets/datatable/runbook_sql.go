package datatable

// How a runbook's SQL reaches PostgreSQL.
//
// # Values only as bind parameters
//
// Every statement is what runbook.Bind returns: the file's text with each
// `:name` rewritten to `$n`, and the values beside it. The values go to pgx as
// arguments and nothing else ever enters the text. A list is the typed slice
// Bind makes ([]string, []int64, ...), passed as it is, so a statement that
// casts (`CAST(:x AS text[])`) or uses `= ANY(:x)` gets a real array.
//
// **Every statement is sent with the extended protocol**, also when it has no
// parameter, by sending it through Query with pgx.QueryExecModeDescribeExec.
// pgx's Exec sends a statement with no argument with the *simple* protocol, which
// would let PostgreSQL run several statements from one string; the extended
// protocol refuses that ("cannot insert multiple commands into a prepared
// statement"), so the statement boundaries are sqlscript.Split's and nobody
// else's. A utility statement (CREATE, DROP) takes no parameter in PostgreSQL;
// one written without a `:name` is sent with no argument and runs.
//
// # Time zone and timeout
//
// A timestamp value is a time.Time in UTC (runbook.Value), and pgx sends a
// time.Time to a `timestamp` parameter by its wall clock, so `CAST(:t AS
// timestamp)` is the UTC wall time. Every step's transaction also sets the
// session time zone to UTC (`set_config('TimeZone', 'UTC', true)`, which is
// SET LOCAL), so a comparison with a timestamptz column, and now() cast to
// timestamp, mean UTC as well, whatever the server's default.
//
// The step's timeout is `set_config('statement_timeout', <ms>, true)` — SET
// LOCAL, with the value as a bind parameter — so the database cancels a
// statement that runs past it. The client's context carries a
// longer deadline as a backstop.
//
// # Read-only queries
//
// A query or wait runs in a transaction opened READ ONLY, so an INSERT,
// UPDATE, DELETE or DDL in a query file fails with PostgreSQL's own error
// (SQLSTATE 25006, "cannot execute INSERT in a read-only transaction"). The
// timeout is set by the transaction's first statement, after which `SET
// TRANSACTION READ WRITE` is refused by PostgreSQL too.
//
// # Statements no runbook may send
//
// Transaction control, a change of role, COPY, LOAD and the rest
// (runbook.ForbiddenStatement) are refused where the runbook is read: by
// runbook.Validate when a run starts, so a run never starts with one, and by
// runbook.Bind, which every statement sent here comes from. The transaction
// status is also checked after every statement as a second line.

import (
	"context"
	"database/sql/driver"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"strconv"
	"strings"
	"time"

	"github.com/artisoft-io/jetstore/jets/runbook"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"
)

// runbookRowsByteCap is the most JSON a step row keeps of a result's rows. Past
// it the rows are cut and the step marked truncated; the outputs are still
// computed from every row kept under max_rows.
const runbookRowsByteCap = 256 * 1024

// runbookClientGrace is how much longer than the statement timeout the client
// waits before cancelling the statement itself.
const runbookClientGrace = 30 * time.Second

// setStepSettings is a step transaction's first statement: its timeout and the
// UTC time zone, both SET LOCAL.
func setStepSettings(ctx context.Context, tx pgx.Tx, timeout time.Duration) error {
	_, err := tx.Exec(ctx, `SELECT set_config('statement_timeout', $1, true), set_config('TimeZone', 'UTC', true)`,
		strconv.FormatInt(timeout.Milliseconds(), 10))
	if err != nil {
		return fmt.Errorf("while setting the step's statement timeout: %v", err)
	}
	return nil
}

// stmtArgs prepends the protocol mode to a statement's arguments.
func stmtArgs(st runbook.BoundStatement) []any {
	return append([]any{pgx.QueryExecModeDescribeExec}, st.Args...)
}

// queryResult is what a query or wait step read.
type queryResult struct {
	Columns   []string
	Rows      [][]any // canonical Go values, at most max_rows
	RowCount  int64   // every row the statement returned
	Truncated bool
}

// runReadOnlyQuery runs a query step's one statement in a read-only
// transaction.
func runReadOnlyQuery(ctx context.Context, conn *pgx.Conn, stmts []runbook.BoundStatement,
	timeout time.Duration, maxRows int) (*queryResult, error) {
	if len(stmts) != 1 {
		return nil, fmt.Errorf("a query runs exactly one statement, the file has %d", len(stmts))
	}
	ctx, cancel := context.WithTimeout(ctx, timeout+runbookClientGrace)
	defer cancel()
	tx, err := conn.BeginTx(ctx, pgx.TxOptions{AccessMode: pgx.ReadOnly})
	if err != nil {
		return nil, fmt.Errorf("while opening the read-only transaction: %v", err)
	}
	defer tx.Rollback(context.Background())
	if err = setStepSettings(ctx, tx, timeout); err != nil {
		return nil, err
	}
	rows, err := tx.Query(ctx, stmts[0].SQL, stmtArgs(stmts[0])...)
	if err != nil {
		return nil, err
	}
	res := &queryResult{}
	for _, fd := range rows.FieldDescriptions() {
		res.Columns = append(res.Columns, fd.Name)
	}
	for rows.Next() {
		res.RowCount++
		if int(res.RowCount) > maxRows {
			res.Truncated = true
			continue
		}
		vals, err := rows.Values()
		if err != nil {
			rows.Close()
			return nil, err
		}
		for i := range vals {
			vals[i] = canonicalDBValue(vals[i])
		}
		res.Rows = append(res.Rows, vals)
	}
	rows.Close()
	if err = rows.Err(); err != nil {
		return nil, err
	}
	if conn.PgConn().TxStatus() != 'T' {
		return nil, errors.New("the statement ended the step's transaction")
	}
	return res, tx.Commit(ctx)
}

// previewEntry is one statement of a ddl, as previewed or executed.
type previewEntry struct {
	Statement    int    `json:"statement"`
	CommandTag   string `json:"command_tag"`
	RowsAffected int64  `json:"rows_affected"`
	SQL          string `json:"sql"`
}

// runDDL runs a ddl step's statements in one read-write transaction. With
// preview it rolls back and reports each statement's command tag; otherwise
// it calls record inside the transaction, so the step's record and its effect
// commit together, and commits.
func runDDL(ctx context.Context, conn *pgx.Conn, stmts []runbook.BoundStatement, timeout time.Duration,
	preview bool, record func(tx pgx.Tx, entries []previewEntry) error) ([]previewEntry, error) {
	ctx, cancel := context.WithTimeout(ctx, time.Duration(len(stmts)+1)*(timeout+runbookClientGrace))
	defer cancel()
	tx, err := conn.BeginTx(ctx, pgx.TxOptions{AccessMode: pgx.ReadWrite})
	if err != nil {
		return nil, fmt.Errorf("while opening the transaction: %v", err)
	}
	defer tx.Rollback(context.Background())
	if err = setStepSettings(ctx, tx, timeout); err != nil {
		return nil, err
	}
	entries := make([]previewEntry, 0, len(stmts))
	for i, st := range stmts {
		rows, err := tx.Query(ctx, st.SQL, stmtArgs(st)...)
		if err != nil {
			return entries, fmt.Errorf("statement %d: %w", i+1, err)
		}
		rows.Close()
		if err = rows.Err(); err != nil {
			return entries, fmt.Errorf("statement %d: %w", i+1, err)
		}
		tag := rows.CommandTag()
		entries = append(entries, previewEntry{Statement: i + 1, CommandTag: tag.String(),
			RowsAffected: tag.RowsAffected(), SQL: st.SQL})
		if conn.PgConn().TxStatus() != 'T' {
			return entries, fmt.Errorf("statement %d ended the step's transaction", i+1)
		}
	}
	if preview {
		return entries, tx.Rollback(ctx)
	}
	if record != nil {
		if err = record(tx, entries); err != nil {
			return entries, err
		}
	}
	return entries, tx.Commit(ctx)
}

// canonicalDBValue turns what pgx decodes into a value runbook.NewValue and
// encoding/json both accept: a numeric as float64, a uuid as text, an
// interval and other driver values as their text, arrays element by element.
func canonicalDBValue(v any) any {
	switch x := v.(type) {
	case nil, string, bool, int64, int32, int16, int8, int, float64, float32, time.Time:
		return x
	case pgtype.Numeric:
		f, err := x.Float64Value()
		if err != nil || !f.Valid {
			return nil
		}
		return f.Float64
	case [16]byte:
		return fmt.Sprintf("%x-%x-%x-%x-%x", x[0:4], x[4:6], x[6:8], x[8:10], x[10:16])
	case []byte:
		return string(x)
	case []any:
		out := make([]any, len(x))
		for i := range x {
			out[i] = canonicalDBValue(x[i])
		}
		return out
	case map[string]any:
		return x
	case driver.Valuer:
		dv, err := x.Value()
		if err != nil {
			return fmt.Sprint(x)
		}
		return canonicalDBValue(dv)
	case fmt.Stringer:
		return x.String()
	}
	return fmt.Sprint(v)
}

// jsonSafe is a canonical value as JSON can carry it: a time as RFC 3339 in
// UTC, a NaN or an infinity as null.
func jsonSafe(v any) any {
	switch x := v.(type) {
	case time.Time:
		return x.UTC().Format(time.RFC3339Nano)
	case float64:
		if math.IsNaN(x) || math.IsInf(x, 0) {
			return nil
		}
	case float32:
		return jsonSafe(float64(x))
	case []any:
		out := make([]any, len(x))
		for i := range x {
			out[i] = jsonSafe(x[i])
		}
		return out
	}
	return v
}

// rowsJSON is a result's rows as a step row keeps them: one object per row,
// keyed by column, cut at runbookRowsByteCap.
func rowsJSON(res *queryResult) ([]byte, bool, error) {
	out := make([]map[string]any, 0, len(res.Rows))
	size := 2
	cut := false
	for _, r := range res.Rows {
		m := make(map[string]any, len(res.Columns))
		for i, c := range res.Columns {
			m[c] = jsonSafe(r[i])
		}
		b, err := json.Marshal(m)
		if err != nil {
			return nil, false, err
		}
		if size+len(b)+1 > runbookRowsByteCap {
			cut = true
			break
		}
		size += len(b) + 1
		out = append(out, m)
	}
	b, err := json.Marshal(out)
	return b, cut, err
}

// valuesJSON is a set of runbook values as JSON, and their types.
func valuesJSON(vals map[string]runbook.Value) ([]byte, map[string]string, error) {
	plain := make(map[string]any, len(vals))
	types := make(map[string]string, len(vals))
	for k, v := range vals {
		plain[k] = jsonSafe(v.V)
		types[k] = v.Type.String()
	}
	b, err := json.Marshal(plain)
	return b, types, err
}

// boundJSON is each bind parameter's name and value, as sent, over every
// statement; and each statement's text with the name each $n stands for.
func boundJSON(stmts []runbook.BoundStatement) (bound []byte, statements []byte, text string, err error) {
	names := map[string]any{}
	type stmtView struct {
		Statement int               `json:"statement"`
		SQL       string            `json:"sql"`
		Params    map[string]string `json:"params"`
	}
	views := make([]stmtView, 0, len(stmts))
	texts := make([]string, 0, len(stmts))
	for i, st := range stmts {
		params := map[string]string{}
		for j, n := range st.Names {
			names[n] = jsonSafe(argForJSON(st.Args[j]))
			params["$"+strconv.Itoa(j+1)] = n
		}
		views = append(views, stmtView{Statement: i + 1, SQL: st.SQL, Params: params})
		texts = append(texts, st.SQL)
	}
	if bound, err = json.Marshal(names); err != nil {
		return
	}
	if statements, err = json.Marshal(views); err != nil {
		return
	}
	return bound, statements, strings.Join(texts, ";\n\n"), nil
}

// argForJSON turns a typed slice Bind made into a []any for JSON.
func argForJSON(a any) any {
	switch x := a.(type) {
	case []string:
		return toAny(x)
	case []int64:
		return toAny(x)
	case []float64:
		return toAny(x)
	case []bool:
		return toAny(x)
	case []time.Time:
		return toAny(x)
	}
	return a
}

func toAny[T any](s []T) []any {
	out := make([]any, len(s))
	for i := range s {
		out[i] = jsonSafe(s[i])
	}
	return out
}

// pgErrorText is an error as a step records it: PostgreSQL's message and code
// when it is one.
func pgErrorText(err error) string {
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) {
		return fmt.Sprintf("%s (SQLSTATE %s)", pgErr.Message, pgErr.Code)
	}
	return err.Error()
}
