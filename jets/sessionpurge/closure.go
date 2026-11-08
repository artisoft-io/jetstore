package sessionpurge

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"sort"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Querier is what a function here needs of a pool or a transaction:
// *pgxpool.Pool and pgx.Tx both satisfy it.
type Querier interface {
	Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error)
	Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
}

// sessionLockClass is the first key of the per-session advisory lock ("SRG"). It
// is distinct from every other two-key advisory lock in the repository
// (runbookLockClass, "RBK", in jets/datatable); the one-key locks, such as the
// agentic audit's install lock, are a separate key space in PostgreSQL.
const sessionLockClass int32 = 0x53524700

// LockSession takes the transaction-scoped advisory lock on sessionID in tx,
// waiting for whoever holds it. It is released when tx commits or rolls back.
//
// It is the one place the statement is written. Three callers take it, and each
// takes it before any row lock, which is what keeps them from deadlocking one
// another:
//
//   - the executor, in deleteRows, first of all, before its claim; one session,
//     so one lock, at a time;
//   - a pipeline start, before it inserts a status row whose input_session_id is
//     sessionID (jets/datatable, GuardInputSession); one lock;
//   - delete_sessions, on every session it is about to queue, in sorted order,
//     before it recomputes the closure and queues it (jets/datatable).
//
// So a start and a deletion of its input session are ordered whichever comes
// first. The second key is hashtext(sessionID), so two sessions may share a lock:
// a start then waits for one session's deletion, nothing more.
//
// The one exception to "one at a time, or sorted" is the dry run, which holds the
// lock of every session it reaches until it ends, taken in queue order (see run).
func LockSession(ctx context.Context, tx pgx.Tx, sessionID string) error {
	if _, err := tx.Exec(ctx, `SELECT pg_advisory_xact_lock($1::int, hashtext($2::text))`,
		sessionLockClass, sessionID); err != nil {
		return fmt.Errorf("while taking the session lock on %s: %w", sessionID, err)
	}
	return nil
}

// activeRunWhere is the predicate, on jetsapi.pipeline_execution_status, of a run
// that keeps the session named by expr from being deleted: a pending or submitted
// run on it or reading it. The executor refuses on it and Closure reports it as
// Dependent.Active, so the confirmation says what the executor will do.
func activeRunWhere(expr string) string {
	return fmt.Sprintf(`status IN ('pending', 'submitted') AND (session_id = %[1]s OR input_session_id = %[1]s)`, expr)
}

// Dependent is one session of a deletion's closure.
type Dependent struct {
	SessionID   string `json:"session_id"`
	Client      string `json:"client"`
	SourceType  string `json:"source_type"`
	MonthPeriod *int   `json:"month_period"`
	// ReadsSession is the session of the closure it reads, through
	// pipeline_execution_status.input_session_id; "" for a selected session.
	ReadsSession string `json:"reads_session"`
	// ProcessName and ObjectType say what the session is, for the confirmation:
	// the process of its own run, and the object type it registered (or its run's
	// main object type). Either may be "".
	ProcessName string `json:"process_name"`
	ObjectType  string `json:"object_type"`
	// Active: a pending or submitted run on it or reading it, so the executor
	// will leave it queued until that run ends.
	Active bool `json:"active"`
}

// ErrSessionNotRegistered is wrapped by Closure's error when a selected session
// has no session_registry row.
var ErrSessionNotRegistered = errors.New("not registered in session_registry")

// Closure returns the selected sessions and, transitively, every session whose
// pipeline_execution_status row has input_session_id in the set (Q-4): selected
// first, then by depth, then by session id. A selected session absent from
// session_registry is an error naming it, wrapping ErrSessionNotRegistered.
//
// A dependent is found by its run, so it may have no session_registry row (a run
// that failed before registering); its client and month period are then its
// run's. What it does not reach is a session that read one of the set through a
// lookback, which records no input_session_id, nor a merged input, of which a run
// records only its main input.
//
// Every read is in one repeatable-read, read-only transaction, so the closure is
// one snapshot. This is the preview's; delete_sessions uses ClosureTx inside the
// transaction that holds the session locks and queues.
func Closure(ctx context.Context, db *pgxpool.Pool, sessionIDs []string) ([]Dependent, error) {
	tx, err := db.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.RepeatableRead, AccessMode: pgx.ReadOnly})
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(context.Background())
	return ClosureTx(ctx, tx, sessionIDs)
}

// ClosureTx is Closure on q, typically a transaction of the caller's.
//
// Under read committed its statements are not one snapshot, and that is what
// delete_sessions needs: it takes the session locks first and then calls this,
// so each statement sees every run a start committed before the locks were
// granted. A repeatable-read transaction would not, because its snapshot is taken
// by its first statement, the lock itself, before the wait.
func ClosureTx(ctx context.Context, q Querier, sessionIDs []string) ([]Dependent, error) {
	selected := slices.Clone(sessionIDs)
	slices.Sort(selected)
	selected = slices.Compact(selected)
	if len(selected) == 0 || selected[0] == "" {
		return nil, errors.New("a deletion needs at least one session id, and no empty one")
	}
	tx := q

	rows, err := tx.Query(ctx, `SELECT s FROM unnest($1::text[]) AS s
		WHERE NOT EXISTS (SELECT 1 FROM jetsapi.session_registry r WHERE r.session_id = s)
		ORDER BY s`, selected)
	if err != nil {
		return nil, fmt.Errorf("while reading session_registry: %w", err)
	}
	missing, err := pgx.CollectRows(rows, pgx.RowTo[string])
	if err != nil {
		return nil, fmt.Errorf("while reading session_registry: %w", err)
	}
	if len(missing) > 0 {
		return nil, fmt.Errorf("session %s: %w", strings.Join(missing, ", "), ErrSessionNotRegistered)
	}

	// Walk input_session_id breadth first, so each session is placed at its
	// shallowest depth and a cycle ends.
	reads := map[string]string{}
	seen := map[string]bool{}
	for _, id := range selected {
		seen[id] = true
	}
	order := slices.Clone(selected)
	frontier := selected
	for len(frontier) > 0 {
		rows, err := tx.Query(ctx, `SELECT session_id, min(input_session_id)
			FROM jetsapi.pipeline_execution_status
			WHERE input_session_id = ANY($1::text[]) AND session_id IS NOT NULL AND session_id <> ''
			GROUP BY session_id`, frontier)
		if err != nil {
			return nil, fmt.Errorf("while reading the sessions that read %d sessions: %w", len(frontier), err)
		}
		type edge struct{ session, reads string }
		edges, err := pgx.CollectRows(rows, func(row pgx.CollectableRow) (edge, error) {
			var e edge
			err := row.Scan(&e.session, &e.reads)
			return e, err
		})
		if err != nil {
			return nil, fmt.Errorf("while reading the sessions that read %d sessions: %w", len(frontier), err)
		}
		var next []string
		for _, e := range edges {
			if seen[e.session] {
				continue
			}
			seen[e.session] = true
			reads[e.session] = e.reads
			next = append(next, e.session)
		}
		sort.Strings(next)
		order = append(order, next...)
		frontier = next
	}

	// What each session is, from its registration and from its own runs.
	rows, err = tx.Query(ctx, `SELECT s.id,
			COALESCE(r.client, pe.client, ''),
			COALESCE(r.source_type, ''),
			COALESCE(r.month_period, pe.month_period),
			COALESCE(pe.process_name, ''),
			COALESCE(ir.object_type, pe.main_object_type, ''),
			EXISTS (SELECT 1 FROM jetsapi.pipeline_execution_status a WHERE `+activeRunWhere("s.id")+`)
		FROM unnest($1::text[]) AS s(id)
		LEFT JOIN jetsapi.session_registry r ON r.session_id = s.id
		LEFT JOIN LATERAL (
			SELECT min(p.client) AS client, min(p.process_name) AS process_name,
				min(p.main_object_type) AS main_object_type, min(sp.month_period) AS month_period
			FROM jetsapi.pipeline_execution_status p
			LEFT JOIN jetsapi.source_period sp ON sp.key = p.source_period_key
			WHERE p.session_id = s.id) pe ON true
		LEFT JOIN LATERAL (
			SELECT min(i.object_type) AS object_type FROM jetsapi.input_registry i WHERE i.session_id = s.id) ir ON true`,
		order)
	if err != nil {
		return nil, fmt.Errorf("while describing the sessions of the closure: %w", err)
	}
	described := map[string]Dependent{}
	for rows.Next() {
		var d Dependent
		if err := rows.Scan(&d.SessionID, &d.Client, &d.SourceType, &d.MonthPeriod, &d.ProcessName,
			&d.ObjectType, &d.Active); err != nil {
			rows.Close()
			return nil, fmt.Errorf("while describing the sessions of the closure: %w", err)
		}
		described[d.SessionID] = d
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("while describing the sessions of the closure: %w", err)
	}

	out := make([]Dependent, 0, len(order))
	for _, id := range order {
		d := described[id]
		d.SessionID = id
		d.ReadsSession = reads[id]
		out = append(out, d)
	}
	return out, nil
}

// Requests turns a closure into the deletion requests that queue it, with reason
// and requestedBy. A dependent with no client, which only a run with no client
// could give, gets ClientUnknown, as an orphan does.
func Requests(closure []Dependent, reason Reason, requestedBy string) []Request {
	out := make([]Request, len(closure))
	for i, d := range closure {
		client := d.Client
		if client == "" {
			client = ClientUnknown
		}
		out[i] = Request{SessionID: d.SessionID, Client: client, SourceType: d.SourceType,
			MonthPeriod: d.MonthPeriod, Reason: reason, RequestedBy: requestedBy}
	}
	return out
}
