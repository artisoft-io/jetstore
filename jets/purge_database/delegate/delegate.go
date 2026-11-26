package delegate

import (
	"context"
	"fmt"
	"log"
	"os"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/artisoft-io/jetstore/jets/agentic/retention"
	"github.com/artisoft-io/jetstore/jets/awsi"
	"github.com/artisoft-io/jetstore/jets/sessionpurge"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// The delegate that performs the database purge.
// Env variables:
// JETS_DSN_SECRET
// JETS_REGION
// USING_SSH_TUNNEL for testing or running locally
// RETENTION_DAYS site global retention days, sessions are purged if > 0
// PURGE_DRY_RUN a dry run unless exactly "false" (see IsDryRun)
// AGENT_AUDIT_RETENTION_DAYS the agent's audit trail is purged after this many
// days if set; unset, it is never purged (see AuditRetentionDays)

// runBudget is how long a run starts sessions for, and sweepBudget how long
// after that the agentic retention sweep starts incidents for: two minutes
// inside the lambda's fifteen are left, so the work in hand, the archive and
// tombstone purges and VACUUM can finish. What is left is done by the next run.
const (
	runBudget   = 12 * time.Minute
	sweepBudget = time.Minute
)

// archiveDays is how long an archived run is kept, from its own last_update, as
// the live table kept it before runs were archived; the retention and orphan
// tombstones are kept as long.
const archiveDays = 180

// userTombstoneDays is how long a user's deletion is remembered: as long as a
// lookback read can reach.
const userTombstoneDays = 400

// IsDryRun reads PURGE_DRY_RUN: only the value "false" means real deletions.
// Unset, empty or anything else is a dry run, so a release carrying the session
// purge deletes nothing at a site until the site has read a dry run's report and
// set the variable.
func IsDryRun(value string) bool {
	return strings.TrimSpace(value) != "false"
}

// AuditRetentionDays reads AGENT_AUDIT_RETENTION_DAYS: the days the agent's
// audit trail (jetsapi.agent_audit, agent_run and approval_event) is kept.
// Unset or empty is 0, never purged, which is the default: an audit trail is
// out of any routine deletion unless a site decides otherwise. Anything that is
// not a whole number of days, 1 or more, is an error.
func AuditRetentionDays(value string) (int, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return 0, nil
	}
	n, err := strconv.Atoi(value)
	if err != nil || n < 1 {
		return 0, fmt.Errorf("AGENT_AUDIT_RETENTION_DAYS must be a whole number of days, 1 or more, or unset; it is %q",
			value)
	}
	return n, nil
}

// * TODO set retention_days at client level (client_registry)
func DoPurgeSessions() error {
	start := time.Now()
	rd := os.Getenv("RETENTION_DAYS")
	if len(rd) == 0 {
		log.Println("Env var RETENTION_DAYS not specified, bailing out!")
		return nil
	}
	retentionDays, err := strconv.Atoi(rd)
	if err != nil {
		return fmt.Errorf("while converting RETENTION_DAYS to int : %v", err)
	}
	if retentionDays < 1 {
		log.Println("Retention days must be at least 1, we have", retentionDays)
		return fmt.Errorf("retention days must be at least 1, we have %d", retentionDays)
	}
	dryRun := IsDryRun(os.Getenv("PURGE_DRY_RUN"))
	auditDays, err := AuditRetentionDays(os.Getenv("AGENT_AUDIT_RETENTION_DAYS"))
	if err != nil {
		return err
	}

	// open db connection
	// Get the dsn from the aws secret
	dsn, err := awsi.GetDsnFromSecret(
		os.Getenv("JETS_DSN_SECRET"),
		len(os.Getenv("USING_SSH_TUNNEL")) > 0,
		5)
	if err != nil {
		return fmt.Errorf("while getting dsn from aws secret: %v", err)
	}
	dbpool, err := pgxpool.New(context.Background(), dsn)
	if err != nil {
		return fmt.Errorf("while opening db connection: %v", err)
	}
	defer dbpool.Close()

	return purgeSessions(context.Background(), dbpool, purgeConfig{
		RetentionDays:      retentionDays,
		AuditRetentionDays: auditDays,
		DryRun:             dryRun,
		Now:                start,
		Deadline:           start.Add(runBudget),
	})
}

type purgeConfig struct {
	RetentionDays int
	// AuditRetentionDays is the agent audit trail's retention; 0 never purges it.
	AuditRetentionDays int
	DryRun             bool
	Now                time.Time // the run's start, from which every age is counted
	Deadline           time.Time // no session is started after it
}

// purgeSessions is the purge, given a database:
//
//  1. Rules unit test runs and detection runs, by age, in a dry run too.
//  2. A dry run reads the retention and orphan candidates, runs them through
//     sessionpurge.ExecuteRequests with every transaction rolled back, logs the
//     report and stops. It writes nothing.
//  3. A real run queues the retention candidates, then the orphans, executes the
//     queue until the deadline, sweeps the agentic incidents no session they
//     stand for is still held (jets/agentic/retention), purges the agent audit
//     trail when its retention is set, purges the archive and the tombstones,
//     and runs VACUUM.
//
// A dry run sweeps the incidents inside its own transaction, after its
// sessions, so it reports the incidents a real run would delete, and counts the
// audit rows a real run would delete; it writes nothing.
//
// A session or an incident that fails is logged and retried by the next run; it
// is not an error of the run, which would have the lambda retried, and neither
// is a failure of the agentic sweep or of the audit retention, which are logged.
func purgeSessions(ctx context.Context, dbpool *pgxpool.Pool, cfg purgeConfig) error {
	purgeFrom := cfg.Now.AddDate(0, 0, -cfg.RetentionDays)
	mode := "real run"
	if cfg.DryRun {
		mode = "dry run, PURGE_DRY_RUN is not false: nothing is deleted"
	}
	log.Printf("Purging all sessions prior to %s (%s)", purgeFrom, mode)

	// Rules unit test runs are not sessions, so they are purged by age, as they
	// were before the session purge was rebuilt.
	purgeRulesUnitTestRuns(dbpool, purgeFrom)
	purgeDetectionRuns(dbpool, purgeFrom)

	sessionCands, err := sessionpurge.RetentionCandidates(ctx, dbpool, purgeFrom)
	if err != nil {
		return fmt.Errorf("while reading the retention candidates: %v", err)
	}
	orphans, err := sessionpurge.OrphanCandidates(ctx, dbpool, purgeFrom)
	if err != nil {
		return fmt.Errorf("while reading the orphan candidates: %v", err)
	}
	log.Printf("Candidates: %d sessions past retention, %d orphan sessions", len(sessionCands), len(orphans))

	opts := sessionpurge.Options{Deadline: cfg.Deadline, DryRun: cfg.DryRun}
	sweepOpts := retention.Options{DryRun: cfg.DryRun, Deadline: cfg.Deadline.Add(sweepBudget)}
	if cfg.DryRun {
		// The sweep runs in the dry run's transaction, after its sessions, so it
		// sees them deleted as a real run would.
		opts.AfterDryRun = func(ctx context.Context, tx pgx.Tx) error {
			sweepIncidents(ctx, tx, sweepOpts, "DRY RUN, nothing written: ")
			return nil
		}
		report, err := sessionpurge.ExecuteRequests(ctx, dbpool, append(sessionCands, orphans...), opts)
		logReport("DRY RUN, nothing written: ", &report)
		if err != nil {
			return fmt.Errorf("while executing the dry run: %v", err)
		}
		purgeAuditTrail(ctx, dbpool, cfg, "DRY RUN, nothing written: ")
		return nil
	}

	if _, err = sessionpurge.Queue(ctx, dbpool, sessionCands); err != nil {
		return fmt.Errorf("while queueing the retention candidates: %v", err)
	}
	if _, err = sessionpurge.Queue(ctx, dbpool, orphans); err != nil {
		return fmt.Errorf("while queueing the orphan candidates: %v", err)
	}
	report, err := sessionpurge.Execute(ctx, dbpool, opts)
	logReport("", &report)
	if err != nil {
		return fmt.Errorf("while executing the session deletions: %v", err)
	}
	sweepIncidents(ctx, dbpool, sweepOpts, "")
	purgeAuditTrail(ctx, dbpool, cfg, "")

	archiveFrom := cfg.Now.AddDate(0, 0, -archiveDays)
	n, err := sessionpurge.PurgeArchive(ctx, dbpool, archiveFrom)
	if err != nil {
		return err
	}
	log.Printf("Purged %d archived pipeline runs last updated on or before %s", n, archiveFrom)
	n, err = sessionpurge.PurgeTombstones(ctx, dbpool, archiveFrom, cfg.Now.AddDate(0, 0, -userTombstoneDays))
	if err != nil {
		return err
	}
	log.Printf("Purged %d session deletion tombstones", n)

	// Perform Vaccum on database
	log.Println("   Performing VACUUM")
	_, err = dbpool.Exec(ctx, "VACUUM")
	if err != nil {
		return fmt.Errorf("while performing VACUUM: %v", err)
	}
	return nil
}

// logReport logs a run's report: the sessions by reason, the rows per table, the
// status rows archived and the sessions refused or failed.
func logReport(prefix string, r *sessionpurge.Report) {
	log.Printf("Session purge: %s%s", prefix, r.Summary())
	tables := make([]string, 0, len(r.Rows))
	for t := range r.Rows {
		tables = append(tables, t)
	}
	sort.Strings(tables)
	for _, t := range tables {
		log.Printf("   %s%d rows from %s", prefix, r.Rows[t], t)
	}
	if len(r.RefusedSessions) > 0 {
		log.Printf("   refused, a run pending or submitted: %s", strings.Join(r.RefusedSessions, ", "))
	}
	if len(r.FailedSessions) > 0 {
		log.Printf("   failed, retried by the next run: %s", strings.Join(r.FailedSessions, ", "))
	}
}

// sweepIncidents runs the agentic retention sweep and logs its report: the
// incidents deleted, those kept because a delivery or run still held is placed
// in them, and the rows per table. A failure is logged, not returned: the
// incidents are swept again by the next run.
func sweepIncidents(ctx context.Context, db retention.DB, opts retention.Options, prefix string) {
	r, err := retention.Sweep(ctx, db, opts)
	log.Printf("Incident retention: %s%s", prefix, r.Summary())
	for _, t := range r.Tables() {
		log.Printf("   incident retention: %s%d rows from %s", prefix, r.Rows[t], t)
	}
	if len(r.FailedIncidents) > 0 {
		log.Printf("   incident retention: failed, retried by the next run: %s", strings.Join(r.FailedIncidents, ", "))
	}
	if err != nil {
		log.Println("Warning: while sweeping the agentic incidents:", err)
	}
}

// purgeAuditTrail purges the agent's audit trail older than its retention, or
// says it is kept when none is set. A failure is logged, not returned.
func purgeAuditTrail(ctx context.Context, db *pgxpool.Pool, cfg purgeConfig, prefix string) {
	if cfg.AuditRetentionDays < 1 {
		log.Println("Agent audit retention: AGENT_AUDIT_RETENTION_DAYS is not set, the audit trail is kept")
		return
	}
	before := cfg.Now.AddDate(0, 0, -cfg.AuditRetentionDays)
	r, err := retention.PurgeAudit(ctx, db, before, cfg.DryRun)
	if err != nil {
		log.Println("Warning: while purging the agent audit trail:", err)
		return
	}
	log.Printf("Agent audit retention: %srows written before %s", prefix, before.UTC().Format(time.RFC3339))
	for _, t := range []string{retention.AgentAuditTable, retention.AgentRunTable, retention.ApprovalEventTable} {
		if n, ok := r.Rows[t]; ok {
			log.Printf("   agent audit retention: %s%d rows from %s", prefix, n, t)
		}
	}
	for _, n := range r.Notes {
		log.Printf("   agent audit retention: %s", n)
	}
}

// purgeRulesUnitTestRuns deletes the rules unit test runs last updated on or
// before purgeFrom, with their checks. A run still queued or executing is kept
// whatever its age: the apiserver's start-up sweep is what ends those. Errors are
// logged and not returned, as for the other secondary purges here -- a database
// the apiserver has not migrated yet has neither table. purgeFrom is cast to
// timestamptz because last_update is a timestamp without time zone and pgx
// would otherwise encode it by its wall clock in its own location.
func purgeRulesUnitTestRuns(dbpool *pgxpool.Pool, purgeFrom time.Time) (runs, checks int64) {
	ctx := context.Background()
	tag, err := dbpool.Exec(ctx, `DELETE FROM jetsapi.rules_unit_test_check WHERE execution_key IN (
		SELECT key FROM jetsapi.rules_unit_test_execution
		WHERE last_update <= $1::timestamptz AND status NOT IN ('queued', 'executing'))`, purgeFrom)
	if err != nil {
		log.Println("Warning: while purging rules_unit_test_check:", err)
		return 0, 0
	}
	checks = tag.RowsAffected()
	tag, err = dbpool.Exec(ctx, `DELETE FROM jetsapi.rules_unit_test_execution
		WHERE last_update <= $1::timestamptz AND status NOT IN ('queued', 'executing')`, purgeFrom)
	if err != nil {
		log.Println("Warning: while purging rules_unit_test_execution:", err)
		return 0, checks
	}
	runs = tag.RowsAffected()
	log.Printf("Purged %d rules unit test runs and %d of their checks\n", runs, checks)
	return runs, checks
}

// purgeDetectionRuns deletes the records of agentic detection runs last updated
// on or before purgeFrom (jets/agentic/detectrun). The anomalies those runs
// wrote are deleted with their sessions, and their incidents by the agentic
// retention sweep, not here. A run still
// requested or running is kept whatever its age, as purgeRulesUnitTestRuns keeps
// a queued test, and errors are logged and not returned for the same reason:
// an unmigrated database has no table.
func purgeDetectionRuns(dbpool *pgxpool.Pool, purgeFrom time.Time) int64 {
	tag, err := dbpool.Exec(context.Background(), `DELETE FROM jetsapi.agentic_detection_run
		WHERE last_update <= $1::timestamptz AND status NOT IN ('requested', 'running')`, purgeFrom)
	if err != nil {
		log.Println("Warning: while purging agentic_detection_run:", err)
		return 0
	}
	log.Printf("Purged %d agentic detection run records\n", tag.RowsAffected())
	return tag.RowsAffected()
}
