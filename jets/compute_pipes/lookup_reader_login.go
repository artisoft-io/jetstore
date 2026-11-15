package compute_pipes

import (
	"context"
	"errors"
	"fmt"
	"log"

	"github.com/artisoft-io/jetstore/jets/dblogin"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

// A sql_lookup's query (LookupSpec.Query) is SQL written in a workspace's
// pipeline configuration, so it runs on the reader login (dblogin.Reader), the
// login every read a user sends SQL on runs as, and never on the pool the
// pipeline was started with, which is the apiserver's own database user. A
// single SELECT can write through a data-modifying CTE, and as the apiserver's
// user it could change the security tables. The reader reads every table in the
// schemas the apiserver's user owns, and public, less the secret columns and
// the login table, and writes nothing.
//
// What stays on the pipeline's pool is the SQL JetStore composes itself: the
// output tables, the status and metrics rows, the shard registry.

// lookupReaderPoolSize is the reader pool's size. The lookups load one after
// the other, so one connection is enough, and every node of a step opens its
// own: a step of N nodes adds at most N connections, for as long as its lookups
// take to load.
const lookupReaderPoolSize = 1

// errNoLookupPool is a sql_lookup in a pipeline that has no database pool.
var errNoLookupPool = errors.New("error: a sql_lookup needs the database, and the pipeline has no database pool")

// openLookupReader opens the reader login beside dbpool, with the password
// dblogin keeps in jetsapi.restricted_login, which dbpool's user reads. The
// apiserver sets the login up at every start (dblogin.Ensure); a database it has
// not started against has no password row, and gets the login set up here.
func openLookupReader(ctx context.Context, dbpool *pgxpool.Pool) (*pgxpool.Pool, error) {
	if dbpool == nil {
		return nil, errNoLookupPool
	}
	pool, err := dblogin.Open(ctx, dbpool, dblogin.Reader, lookupReaderPoolSize)
	if err == nil {
		return pool, nil
	}
	log.Printf("compute pipes: could not open the %s login for the lookup queries (%v); setting up the restricted logins", dblogin.Reader, err)
	if err2 := dblogin.Ensure(ctx, dbpool); err2 != nil {
		return nil, fmt.Errorf("while opening the %s login for the lookup queries: %v; and while setting it up: %w", dblogin.Reader, err, err2)
	}
	return dblogin.Open(ctx, dbpool, dblogin.Reader, lookupReaderPoolSize)
}

// withReaderHint adds, to a lookup query the reader login was not allowed to
// run, what that login may do, so a refusal reads as a rule rather than a fault.
func withReaderHint(err error) error {
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && (pgErr.Code == "42501" || pgErr.Code == "25006") {
		return fmt.Errorf("%w (lookup queries run as %s, read-only: they may read every table in the schemas "+
			"the apiserver's database user owns, and public, except the secret columns and %s; a table another role "+
			"owns needs GRANT SELECT ... TO %s from its owner)",
			err, dblogin.Reader, dblogin.LoginTable, dblogin.Reader)
	}
	return err
}
