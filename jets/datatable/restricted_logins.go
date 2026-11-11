package datatable

import (
	"context"
	"errors"
	"fmt"
	"log"
	"os"
	"strconv"
	"sync"

	"github.com/artisoft-io/jetstore/jets/dblogin"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

// ReaderRole and IDERole are the group roles of the two restricted logins
// (package dblogin says why they are logins and what each may do).
//
// The SQL a user sends runs on them, never on the apiserver's own pool:
//   - every read, on the reader pool (readOnlyTx, a runbook's query and wait
//     steps);
//   - the Query Tool, Submit DDL and a runbook's ddl steps, on the IDE pool.
//
// The SQL the application composes and runs for a button (Drop Staging Table,
// the Users screen, a pipeline start) stays on the apiserver's pool.
const (
	ReaderRole = dblogin.Reader
	IDERole    = dblogin.IDE
)

// Pool sizes, each settable from the environment.
const (
	defaultReaderPoolSize = 10
	defaultIDEPoolSize    = 4
)

var (
	restrictedMu sync.RWMutex
	readerPool   *pgxpool.Pool
	idePool      *pgxpool.Pool
)

// errNoRestrictedLogins is a read or a free-SQL statement sent before the
// logins are open: the request fails rather than running on the apiserver's
// own privileges.
var errNoRestrictedLogins = errors.New("error: the database logins for user SQL are not open (EnsureRestrictedLogins)")

// EnsureRestrictedLogins sets up both logins and their grants (dblogin.Ensure)
// and opens their pools beside dbpool, closing any opened before. The
// apiserver calls it at every start, after the migration, and when it reopens
// its own pool.
func EnsureRestrictedLogins(ctx context.Context, dbpool *pgxpool.Pool) error {
	if err := dblogin.Ensure(ctx, dbpool); err != nil {
		return err
	}
	reader, err := dblogin.Open(ctx, dbpool, ReaderRole, poolSizeFromEnv("JETS_READER_POOL_SIZE", defaultReaderPoolSize))
	if err != nil {
		return err
	}
	ide, err := dblogin.Open(ctx, dbpool, IDERole, poolSizeFromEnv("JETS_IDE_POOL_SIZE", defaultIDEPoolSize))
	if err != nil {
		reader.Close()
		return err
	}
	restrictedMu.Lock()
	oldReader, oldIDE := readerPool, idePool
	readerPool, idePool = reader, ide
	restrictedMu.Unlock()
	if oldReader != nil {
		oldReader.Close()
	}
	if oldIDE != nil {
		oldIDE.Close()
	}
	return nil
}

// CloseRestrictedLogins closes both pools.
func CloseRestrictedLogins() {
	restrictedMu.Lock()
	defer restrictedMu.Unlock()
	if readerPool != nil {
		readerPool.Close()
		readerPool = nil
	}
	if idePool != nil {
		idePool.Close()
		idePool = nil
	}
}

func readerDB() (*pgxpool.Pool, error) {
	restrictedMu.RLock()
	defer restrictedMu.RUnlock()
	if readerPool == nil {
		return nil, errNoRestrictedLogins
	}
	return readerPool, nil
}

func ideDB() (*pgxpool.Pool, error) {
	restrictedMu.RLock()
	defer restrictedMu.RUnlock()
	if idePool == nil {
		return nil, errNoRestrictedLogins
	}
	return idePool, nil
}

// restrictedConn takes a connection out of pool for one runbook step, names it
// after the run so Abort finds it (runbookAppName), and hands it over: the
// caller closes it, so nothing the step does to its session outlives the step.
func restrictedConn(ctx context.Context, pool *pgxpool.Pool, key int64) (*pgx.Conn, error) {
	c, err := pool.Acquire(ctx)
	if err != nil {
		return nil, fmt.Errorf("while acquiring a connection: %v", err)
	}
	conn := c.Hijack()
	if _, err = conn.Exec(ctx, "SELECT set_config('application_name', $1, false)", runbookAppName(key)); err != nil {
		conn.Close(context.Background())
		return nil, err
	}
	return conn, nil
}

func poolSizeFromEnv(name string, def int32) int32 {
	v := os.Getenv(name)
	if v == "" {
		return def
	}
	n, err := strconv.Atoi(v)
	if err != nil || n < 1 {
		log.Printf("%s=%q is not a positive integer, using %d", name, v, def)
		return def
	}
	return int32(n)
}

// withIDEHint adds, to a statement the IDE login was not allowed to run, what
// that login may do, so a refusal reads as a rule rather than a fault.
func withIDEHint(err error) error {
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && pgErr.Code == "42501" {
		return fmt.Errorf("%w (the Query Tool, Submit DDL and runbook ddl steps run as %s: "+
			"they may read and change every table except the security tables, which they may only read, "+
			"and may drop or alter only a table they created)", err, IDERole)
	}
	return err
}
