// Package inputregistry says which relation holds the live rows of
// jetsapi.input_registry, on a database the migration may not have reached.
//
// A session's deletion (jets/sessionpurge) marks its input_registry rows
// deleted, setting deleted_at, rather than deleting them: a run that read the
// session keeps the registration of its input (its schema provider, its
// source), and a file registered again after its first registration was
// deleted is still known to have arrived before. A NULL deleted_at is a live
// registration, and the view jetsapi.input_registry_live (jets_init_db.sql)
// holds those alone.
//
// So the readers of input_registry are of two kinds. One lists or acts on the
// registrations that exist now: the registration screens, a pipeline start, the
// purge's own selection of sessions and tables. It reads the view, through
// LiveRelation. The other resolves what a run that already happened read, or
// whether a file arrived before: it reads the table, every row, marked or not,
// and needs nothing from here. Writers write the table.
//
// The column and the view are added by the migration. New code can run against
// a database the migration has not reached, where neither exists, nothing is
// marked and every row of the table is live: LiveRelation then names the table,
// and HasDeletedAt, the purge's probe, says to delete rows as before. Old code on
// a migrated database reads and deletes the table's rows as it always did.
package inputregistry

import (
	"context"
	"fmt"
	"sync"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Table is the table, every registration, marked or not.
const Table = "jetsapi.input_registry"

// LiveView is the view of the live registrations, those whose deleted_at is
// NULL.
const LiveView = "jetsapi.input_registry_live"

// Column is the column that marks a registration deleted: a datetime, NULL
// while the registration is live.
const Column = "deleted_at"

// Querier is what the probes need of a pool, a connection or a transaction.
type Querier interface {
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
}

// viewSQL asks whether the view exists; to_regclass is NULL, not an error, for
// a relation that does not.
const viewSQL = `SELECT to_regclass('jetsapi.input_registry_live') IS NOT NULL`

// columnSQL asks whether jetsapi.input_registry has the column, false where the
// table itself does not exist.
const columnSQL = `SELECT EXISTS (SELECT 1 FROM pg_attribute
	WHERE attrelid = to_regclass('jetsapi.input_registry') AND attname = 'deleted_at' AND NOT attisdropped)`

// Only a positive answer is cached, per database. A database gains the view and
// the column once and keeps them, so a yes holds for the life of the process; a
// no is asked again, so a process started before the migration sees them once
// they exist. The cache is keyed by the database the querier is connected to
// (host, port and name), so a process that reaches two databases, as the tests
// do, never answers for one from the other; a querier whose database cannot be
// read is not cached.
var hasView, hasColumn sync.Map // database key -> struct{}

// databaseKey names the database q is connected to, when q says.
func databaseKey(q Querier) (string, bool) {
	var cfg *pgx.ConnConfig
	switch v := q.(type) {
	case *pgxpool.Pool:
		cfg = v.Config().ConnConfig
	case *pgxpool.Conn:
		cfg = v.Conn().Config()
	case *pgx.Conn:
		cfg = v.Config()
	case pgx.Tx:
		if c := v.Conn(); c != nil {
			cfg = c.Config()
		}
	}
	if cfg == nil {
		return "", false
	}
	return fmt.Sprintf("%s:%d/%s", cfg.Host, cfg.Port, cfg.Database), true
}

func probe(ctx context.Context, q Querier, cache *sync.Map, stmt, what string) (bool, error) {
	key, keyed := databaseKey(q)
	if keyed {
		if _, ok := cache.Load(key); ok {
			return true, nil
		}
	}
	var ok bool
	if err := q.QueryRow(ctx, stmt).Scan(&ok); err != nil {
		return false, fmt.Errorf("while looking for %s: %w", what, err)
	}
	if ok && keyed {
		cache.Store(key, struct{}{})
	}
	return ok, nil
}

// LiveRelation names the relation a reader of the live registrations reads:
// LiveView where it exists, else Table, where nothing can be marked. The name
// is qualified and needs no quoting; give it an alias in the statement.
func LiveRelation(ctx context.Context, q Querier) (string, error) {
	ok, err := probe(ctx, q, &hasView, viewSQL, LiveView)
	if err != nil {
		return "", err
	}
	if ok {
		return LiveView, nil
	}
	return Table, nil
}

// HasDeletedAt reports whether jetsapi.input_registry has the deleted_at
// column, so a deletion can mark rows rather than delete them.
func HasDeletedAt(ctx context.Context, q Querier) (bool, error) {
	return probe(ctx, q, &hasColumn, columnSQL, "jetsapi.input_registry.deleted_at")
}

// DeletedExpr is an expression, on the row of jetsapi.input_registry aliased
// alias, that is true when the registration is marked deleted. It reads the
// column through to_jsonb of the row, so a statement carrying it needs no probe
// and runs unchanged on a database without the column, where it is false. It is
// for a reader of the table that must tell the marked rows apart in a statement
// whose text is fixed, as a statement an export prints is.
func DeletedExpr(alias string) string {
	return fmt.Sprintf("((to_jsonb(%s) ->> 'deleted_at') IS NOT NULL)", pgx.Identifier{alias}.Sanitize())
}

// Forget drops the cached answers. For a test that removes the view or the
// column, and for nothing else.
func Forget() {
	hasView.Clear()
	hasColumn.Clear()
}
