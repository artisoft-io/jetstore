package datatable

import (
	"context"
	"errors"
	"fmt"
	"log"
	"strconv"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

// ApiServerWriteTimeout is the apiserver's http.Server WriteTimeout. It is
// declared here, and the apiserver reads it, so that the query tool's statement
// timeout below cannot drift away from it.
const ApiServerWriteTimeout = 120 * time.Second

// QueryToolStatementTimeout is the statement_timeout the query button
// (raw_query_tool) runs under.
//
// Past the write timeout the browser gets a 502 while the statement keeps
// running on the database, holding its locks, until someone cancels it by hand.
// Ending the statement on the server ten seconds before the response gives up
// means the user is told what happened, and the database stops working on a
// result nobody will receive. It is the server's timeout rather than a context
// deadline because a deadline on the client closes the connection and leaves
// the statement running.
//
// Submit DDL (exec_ddl) has no such limit: a long export is sent there, and it
// completes on the database after the response has timed out.
const QueryToolStatementTimeout = ApiServerWriteTimeout - 10*time.Second

// queryToolStatementTimeout is QueryToolStatementTimeout, a variable so that a
// test need not wait for it.
var queryToolStatementTimeout = QueryToolStatementTimeout

// execQueryToolStatement runs the query button's statement on one connection
// under queryToolStatementTimeout, and resets the timeout before the connection
// goes back to the pool.
//
// It is set for the session and reset afterwards rather than set in a
// transaction, because the button is not a transaction: VACUUM and CREATE INDEX
// CONCURRENTLY, which refuse a transaction block, run through it.
func execQueryToolStatement(pool *pgxpool.Pool, dataTableAction *DataTableAction, query *string,
	timeout time.Duration) (*[][]any, *[]DataTableColumnDef, error) {
	bg := context.Background()
	conn, err := pool.Acquire(bg)
	if err != nil {
		return nil, nil, fmt.Errorf("while acquiring a connection: %w", err)
	}
	defer conn.Release()
	_, err = conn.Exec(bg, "SELECT set_config('statement_timeout', $1, false)",
		strconv.FormatInt(timeout.Milliseconds(), 10))
	if err != nil {
		conn.Conn().Close(bg)
		return nil, nil, fmt.Errorf("while setting the query tool's statement timeout: %w", err)
	}
	defer func() {
		if _, err := conn.Exec(bg, "RESET statement_timeout"); err != nil {
			// Released with the limit still set, the connection would impose it on
			// every later user of the pool. Closed, the pool replaces it.
			log.Printf("while resetting the query tool's statement timeout, closing the connection: %v", err)
			conn.Conn().Close(bg)
		}
	}()
	resultRows, columnDefs, err := execQuery(conn, dataTableAction, query)
	if isStatementTimeout(err) {
		return nil, nil, &queryToolTimeoutError{timeout: timeout}
	}
	return resultRows, columnDefs, err
}

// queryToolTimeoutError is the query button's statement cancelled by
// QueryToolStatementTimeout, said in terms the user can act on.
type queryToolTimeoutError struct {
	timeout time.Duration
}

func (e *queryToolTimeoutError) Error() string {
	return fmt.Sprintf("the query ran past %d s and was cancelled; "+
		"a long-running statement, such as an export, belongs in Submit DDL", int(e.timeout.Seconds()))
}

func isQueryToolTimeout(err error) bool {
	var e *queryToolTimeoutError
	return errors.As(err, &e)
}

// isStatementTimeout reports whether err is PostgreSQL cancelling a statement
// for statement_timeout. query_canceled (57014) is also what a cancel request
// raises, so the message decides.
func isStatementTimeout(err error) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.Code == "57014" &&
		strings.Contains(pgErr.Message, "statement timeout")
}
