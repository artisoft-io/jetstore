package delegate

import (
	"context"
	"errors"
	"fmt"
	"log"

	"github.com/artisoft-io/jetstore/jets/dblogin"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

// The SQL a workspace's reports/ directory holds runs on the IDE login
// (dblogin.IDE), the login the IDE's Submit DDL and the workspace's init scripts
// (update_db) run on, and never on the pool run_reports was started with. A
// workspace is edited from the IDE, so its reports get no more than Submit DDL
// does: they read and change every table except the security tables (users,
// roles, role_capability, secret_rotation), which they may only read, and they
// may drop or alter only a table the IDE login created.
//
// What stays on the apiserver's own pool is the SQL JetStore composes itself:
// the session's source_period_key and output counts, input_registry
// (registerReport), report_execution_status, the lookup-table sync and compile,
// and a deployment's own report functions (InstallSpecificFunc), which are
// deployment code rather than workspace files.

// workspaceSQLPoolSize is the IDE pool's size: run_reports sends one statement
// at a time.
const workspaceSQLPoolSize = 2

// errNoWorkspaceSQLPool is a report statement sent before the IDE login was
// opened: the report fails rather than running on the apiserver's privileges.
var errNoWorkspaceSQLPool = errors.New("error: the IDE database login the reports run on is not open (CommandArguments.SQLPool)")

// OpenWorkspaceSQLPool opens the IDE login beside dbpool, with the password
// dblogin keeps in jetsapi.restricted_login, which dbpool's user reads. The
// apiserver and update_db set the login up at every start (dblogin.Ensure); a
// database neither has started against yet has no password row, and gets the
// login set up here.
func OpenWorkspaceSQLPool(ctx context.Context, dbpool *pgxpool.Pool) (*pgxpool.Pool, error) {
	pool, err := dblogin.Open(ctx, dbpool, dblogin.IDE, workspaceSQLPoolSize)
	if err == nil {
		return pool, nil
	}
	log.Printf("run_reports: could not open the %s login (%v); setting up the restricted logins", dblogin.IDE, err)
	if err2 := dblogin.Ensure(ctx, dbpool); err2 != nil {
		return nil, fmt.Errorf("while opening the %s login: %v; and while setting it up: %w", dblogin.IDE, err, err2)
	}
	return dblogin.Open(ctx, dbpool, dblogin.IDE, workspaceSQLPoolSize)
}

// workspaceSQL is the pool the workspace's report SQL runs on.
func (ca *CommandArguments) workspaceSQL() (*pgxpool.Pool, error) {
	if ca.SQLPool == nil {
		return nil, errNoWorkspaceSQLPool
	}
	return ca.SQLPool, nil
}

// withIDEHint adds, to a statement the IDE login was not allowed to run, what
// that login may do, so a refusal reads as a rule rather than a fault.
func withIDEHint(err error) error {
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && pgErr.Code == "42501" {
		return fmt.Errorf("%w (reports run as %s, like Submit DDL: they may read and change every table "+
			"except the security tables, which they may only read, and may drop, alter or index only a table they created)",
			err, dblogin.IDE)
	}
	return err
}
