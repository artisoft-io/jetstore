// Package retention deletes the agentic rows the session purge leaves behind
// once nothing they stand for is still held. It is run by the purge
// (jets/purge_database/delegate) after the session deletions, in the purge's
// own process, because deciding whether an incident is still held means
// reading its condition, which jets/sessionpurge must not import.
//
// # What the session purge deletes, and what it leaves
//
// A session's anomalies and run verdicts (jetsapi.anomaly, jetsapi.run_finding)
// are deleted with it, by jets/sessionpurge: their evidence is the session's
// registration and data. Its incidents are not, because an incident is how an
// open condition is remembered: a feed or a process that keeps failing the same
// way has one incident, on its first delivery or run, and each later one joins
// it. Deleted with its first session, the incident would be raised again by the
// next delivery or run of a condition still open, and a person's verdict on it
// would be lost while the fault is current.
//
// # When an incident goes: Sweep
//
// **An incident is deleted when no session it stands for is still held**: its
// own session is held no more (see sessionHeldSQL), and no live member of its
// condition is placed in it. Which members are placed in it is the resolver's
// answer, the one shadow triage writes by:
//
//   - a run incident carrying its key (client, process, locus and error code)
//     and its position: runcondition, History.Members;
//   - a feed incident carrying its feed and its position: feedcondition,
//     Feed.Members, through Resolve;
//   - an incident with no key (a run or feed incident written before incidents
//     carried one, or any other): its own session alone decides, since no
//     condition finds it once its session is gone.
//
// Its hypotheses, remediations and events (jetsapi.hypothesis, remediation and
// incident_event, joined to it by id with no foreign key) go with it, in one
// transaction per incident. Only incidents whose own session is held no more
// are read, so a day's sweep reads the incidents whose sessions the purge has
// reached, and the conditions they belong to, each once.
//
// The audit trail (agent_audit, agent_run, change_proposal, approval_event) is
// never touched by Sweep. PurgeAudit deletes from it only when a retention of
// its own is set.
package retention

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"sort"
	"strings"
	"time"

	"github.com/artisoft-io/jetstore/jets/agentic/feedcondition"
	"github.com/artisoft-io/jetstore/jets/agentic/observe"
	"github.com/artisoft-io/jetstore/jets/agentic/runcondition"
	"github.com/artisoft-io/jetstore/jets/agentic/triage"
	"github.com/jackc/pgx/v5"
)

// DB is what the sweep reads and writes through: a pool, whose Begin starts a
// transaction, or a transaction, whose Begin starts a savepoint.
type DB interface {
	observe.DB
	Begin(ctx context.Context) (pgx.Tx, error)
}

// Options govern one Sweep.
type Options struct {
	// DryRun deletes as a real run would, inside one transaction rolled back at
	// the end, so the report says what a real run would delete and nothing is
	// written. Given a transaction (the session purge's dry run), the sweep
	// sees the sessions that dry run deleted.
	DryRun bool
	// Deadline stops the sweep starting incidents after it; zero for none.
	// What is left is read again by the next run.
	Deadline time.Time
}

// The tables an incident's rows are deleted from, the keys of Report.Rows.
const (
	IncidentTable      = "jetsapi.incident"
	HypothesisTable    = "jetsapi.hypothesis"
	RemediationTable   = "jetsapi.remediation"
	IncidentEventTable = "jetsapi.incident_event"
)

// Report says what a sweep did, or for a dry run what it would have done.
type Report struct {
	// Installed reports whether jetsapi.incident was there to sweep; a
	// database without the agentic tables has nothing to do.
	Installed bool
	// Candidates counts the incidents whose own session is held no more.
	Candidates int
	// Deleted counts those deleted: no live member of their condition is
	// placed in them. Held counts those kept because one is.
	Deleted, Held int
	// Failed counts those whose deletion failed and was rolled back, and
	// NotReached those not started before the deadline. Both are read again
	// by the next run.
	Failed, NotReached int
	// Rows is the rows deleted per table.
	Rows map[string]int64
	// DeletedIncidents and FailedIncidents name the incidents behind Deleted
	// and Failed.
	DeletedIncidents, FailedIncidents []string
}

// Summary renders the report in one line.
func (r *Report) Summary() string {
	if !r.Installed {
		return "no incident table, nothing to do"
	}
	return fmt.Sprintf("%d incidents whose own session is no longer held: %d deleted, %d kept (a delivery or "+
		"run still held is placed in them), %d failed, %d not reached before the deadline",
		r.Candidates, r.Deleted, r.Held, r.Failed, r.NotReached)
}

// Tables returns the table names of Rows, sorted.
func (r *Report) Tables() []string {
	out := make([]string, 0, len(r.Rows))
	for t := range r.Rows {
		out = append(out, t)
	}
	sort.Strings(out)
	return out
}

// sessionHeldSQL is true when the database still holds the session named by
// %[1]s: its registration, a run on it, an input registered under it, or an
// anomaly on it. These are what make a session a position of a run or feed
// condition, and what the session purge deletes; a session in none of them
// is no longer held, whether the purge deleted it or a user did. A run that
// failed, or one still running, is held by its status row even before (or
// without) a registration.
const sessionHeldSQL = `(EXISTS (SELECT 1 FROM jetsapi.session_registry r WHERE r.session_id = %[1]s)
     OR EXISTS (SELECT 1 FROM jetsapi.pipeline_execution_status s WHERE s.session_id = %[1]s)
     OR EXISTS (SELECT 1 FROM jetsapi.input_registry ir WHERE ir.session_id = %[1]s)
     OR EXISTS (SELECT 1 FROM jetsapi.anomaly a WHERE a.anomaly_session_id = %[1]s))`

// SessionHeldSQL returns sessionHeldSQL for a SQL expression naming a session.
func SessionHeldSQL(expr string) string { return fmt.Sprintf(sessionHeldSQL, expr) }

// tablesSQL says which of the tables the sweep reads and writes are there.
const tablesSQL = `SELECT to_regclass('jetsapi.incident') IS NOT NULL,
       to_regclass('jetsapi.anomaly') IS NOT NULL,
       to_regclass('jetsapi.session_registry') IS NOT NULL
         AND to_regclass('jetsapi.pipeline_execution_status') IS NOT NULL
         AND to_regclass('jetsapi.input_registry') IS NOT NULL,
       to_regclass('jetsapi.hypothesis') IS NOT NULL,
       to_regclass('jetsapi.remediation') IS NOT NULL,
       to_regclass('jetsapi.incident_event') IS NOT NULL,
       EXISTS (SELECT 1 FROM information_schema.columns
                WHERE table_schema = 'jetsapi' AND table_name = 'incident' AND column_name = 'incident_client'),
       EXISTS (SELECT 1 FROM information_schema.columns
                WHERE table_schema = 'jetsapi' AND table_name = 'incident' AND column_name = 'incident_feed')`

// candidate is an incident whose own session is held no more.
type candidate struct {
	id, session, locus    string
	feed                  string
	client, process, code string
	anchor                *time.Time
}

func (c *candidate) runKey() (runcondition.Key, bool) {
	if c.client == "" || c.anchor == nil || !runcondition.IsRunLocus(c.locus) {
		return runcondition.Key{}, false
	}
	return runcondition.Key{Client: c.client, Process: c.process, Locus: c.locus, Code: c.code}, true
}

func (c *candidate) feedKeyed() bool {
	return c.feed != "" && c.anchor != nil && slices.Contains(triage.DeliveryLoci, c.locus)
}

// childTable is a table holding an incident's own rows, and its column naming
// the incident.
type childTable struct{ name, column string }

var childTables = []childTable{
	{HypothesisTable, "hypothesis_incident_ref"},
	{RemediationTable, "incident_ref"},
	{IncidentEventTable, "event_incident_ref"},
}

// Sweep deletes the incidents no session they stand for is still held, with
// their hypotheses, remediations and events, one transaction per incident (see
// the package comment). It never deletes an incident whose condition places a
// live delivery or run in it. A failure on one incident is counted and rolled
// back, and the sweep goes on; an error is returned only when the sweep cannot
// read its work, or, in a dry run, cannot go on.
func Sweep(ctx context.Context, db DB, opts Options) (Report, error) {
	r := Report{Rows: map[string]int64{}}
	var incident, anomaly, core, hyp, rem, ev, runKeyed, feedKeyed bool
	if err := db.QueryRow(ctx, tablesSQL).Scan(&incident, &anomaly, &core, &hyp, &rem, &ev, &runKeyed,
		&feedKeyed); err != nil {
		return r, fmt.Errorf("while asking which agentic tables are installed: %w", err)
	}
	if !incident || !anomaly || !core {
		return r, nil
	}
	r.Installed = true
	present := map[string]bool{HypothesisTable: hyp, RemediationTable: rem, IncidentEventTable: ev}
	var children []childTable
	for _, t := range childTables {
		if present[t.name] {
			children = append(children, t)
		}
	}
	if opts.DryRun {
		tx, err := db.Begin(ctx)
		if err != nil {
			return r, fmt.Errorf("while starting the dry run's transaction: %w", err)
		}
		// Nothing a dry run did is ever committed.
		defer tx.Rollback(context.Background())
		db = tx
	}

	cands, err := readCandidates(ctx, db, runKeyed, feedKeyed)
	if err != nil {
		return r, err
	}
	r.Candidates = len(cands)
	held, unread := heldIncidents(ctx, db, cands)
	for i := range cands {
		c := &cands[i]
		if err := ctx.Err(); err != nil {
			r.NotReached += len(cands) - i
			return r, err
		}
		if !opts.Deadline.IsZero() && !time.Now().Before(opts.Deadline) {
			r.NotReached += len(cands) - i
			break
		}
		if err := unread[c.id]; err != nil {
			// Its condition could not be read, so it is not known to be
			// unheld: kept, and read again by the next run.
			r.Failed++
			r.FailedIncidents = append(r.FailedIncidents, c.id)
			continue
		}
		if held[c.id] {
			r.Held++
			continue
		}
		rows, gone, err := deleteIncident(ctx, db, c.id, children)
		switch {
		case errors.Is(err, errBroken):
			r.NotReached += len(cands) - i
			return r, err
		case err != nil:
			r.Failed++
			r.FailedIncidents = append(r.FailedIncidents, c.id)
		case !gone:
			// Its session is held again, or the incident went meanwhile.
			r.Held++
		default:
			r.Deleted++
			r.DeletedIncidents = append(r.DeletedIncidents, c.id)
			for t, n := range rows {
				r.Rows[t] += n
			}
		}
	}
	return r, nil
}

// readCandidates reads the incidents whose own session is held no more, by id.
func readCandidates(ctx context.Context, db observe.DB, runKeyed, feedKeyed bool) ([]candidate, error) {
	runCols := `'', '', ''`
	if runKeyed {
		runCols = `coalesce(i.incident_client, ''), coalesce(i.incident_process_name, ''),
		 coalesce(i.incident_error_code, '')`
	}
	feedCol := `''`
	if feedKeyed {
		feedCol = `coalesce(i.incident_feed, '')`
	}
	anchor := `CAST(NULL AS timestamp with time zone)`
	if runKeyed || feedKeyed {
		anchor = `i.incident_anchor_start_time`
	}
	rows, err := db.Query(ctx, `SELECT i.incident_id, i.incident_session_id, i.incident_locus, `+feedCol+`, `+
		runCols+`, `+anchor+`
		  FROM jetsapi.incident i
		 WHERE NOT `+SessionHeldSQL("i.incident_session_id")+`
		 ORDER BY i.incident_id`)
	if err != nil {
		return nil, fmt.Errorf("while reading the incidents whose session is no longer held: %w", err)
	}
	defer rows.Close()
	var out []candidate
	for rows.Next() {
		var c candidate
		if err := rows.Scan(&c.id, &c.session, &c.locus, &c.feed, &c.client, &c.process, &c.code,
			&c.anchor); err != nil {
			return nil, fmt.Errorf("while scanning an incident: %w", err)
		}
		out = append(out, c)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("while reading the incidents whose session is no longer held: %w", err)
	}
	return out, nil
}

// heldIncidents names the candidates a live member of their condition is
// placed in, reading each run key and each feed once. A candidate whose
// condition could not be read is in unread, with the error, and is kept.
func heldIncidents(ctx context.Context, db observe.DB, cands []candidate) (held map[string]bool, unread map[string]error) {
	held, unread = map[string]bool{}, map[string]error{}
	runs := map[runcondition.Key]*runcondition.History{}
	runErrs := map[runcondition.Key]error{}
	feeds := map[string]*feedcondition.Feed{}
	feedErrs := map[string]error{}
	for i := range cands {
		c := &cands[i]
		if key, ok := c.runKey(); ok {
			h, read := runs[key]
			if !read && runErrs[key] == nil {
				var err error
				if h, err = runcondition.Read(ctx, db, key); err != nil {
					runErrs[key] = err
				} else {
					runs[key] = h
				}
			}
			if err := runErrs[key]; err != nil {
				unread[c.id] = err
			} else if _, members := h.Members(c.id); len(members) > 0 {
				held[c.id] = true
			}
			continue
		}
		if c.feedKeyed() {
			f, read := feeds[c.feed]
			if !read && feedErrs[c.feed] == nil {
				var err error
				if f, err = feedcondition.Read(ctx, db, c.feed); err != nil {
					feedErrs[c.feed] = err
				} else {
					feeds[c.feed] = f
				}
			}
			if err := feedErrs[c.feed]; err != nil {
				unread[c.id] = err
			} else if _, members := f.Members(f.IncidentById(c.id)); len(members) > 0 {
				held[c.id] = true
			}
		}
		// No key: its own session alone decides, and it is held no more.
	}
	return held, unread
}

// errBroken: the dry run's transaction cannot go on.
var errBroken = errors.New("the transaction cannot continue")

// deleteIncident deletes one incident and its rows in a transaction of its own
// (a savepoint, in a dry run): the incident first, and only while its session
// is still held no more, then its hypotheses, remediations and events. gone is
// false when the incident was not deleted because it is gone or its session is
// held again.
func deleteIncident(ctx context.Context, db DB, id string, children []childTable) (map[string]int64, bool, error) {
	tx, err := db.Begin(ctx)
	if err != nil {
		return nil, false, fmt.Errorf("%w: %v", errBroken, err)
	}
	rows, gone, err := deleteIncidentRows(ctx, tx, id, children)
	if err != nil || !gone {
		if rbErr := tx.Rollback(context.Background()); rbErr != nil {
			return nil, false, fmt.Errorf("%w: rolling back incident %s after %v: %v", errBroken, id, err, rbErr)
		}
		return nil, false, err
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, false, fmt.Errorf("while committing the deletion of incident %s: %w", id, err)
	}
	return rows, true, nil
}

func deleteIncidentRows(ctx context.Context, tx pgx.Tx, id string, children []childTable) (map[string]int64, bool, error) {
	tag, err := tx.Exec(ctx, `DELETE FROM jetsapi.incident i
		WHERE i.incident_id = $1 AND NOT `+SessionHeldSQL("i.incident_session_id"), id)
	if err != nil {
		return nil, false, fmt.Errorf("on %s: %w", IncidentTable, err)
	}
	if tag.RowsAffected() == 0 {
		return nil, false, nil
	}
	rows := map[string]int64{IncidentTable: tag.RowsAffected()}
	for _, t := range children {
		tag, err := tx.Exec(ctx, `DELETE FROM `+pgx.Identifier(strings.Split(t.name, ".")).Sanitize()+
			` WHERE `+pgx.Identifier{t.column}.Sanitize()+` = $1`, id)
		if err != nil {
			return nil, false, fmt.Errorf("on %s: %w", t.name, err)
		}
		rows[t.name] += tag.RowsAffected()
	}
	return rows, true, nil
}
