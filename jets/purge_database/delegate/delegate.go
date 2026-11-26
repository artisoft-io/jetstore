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
	"github.com/artisoft-io/jetstore/jets/datatable"
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
// DOMAIN_TABLES_RETENTION_DAYS the days a session's rows in the domain tables
// and process_errors are kept; unset, RETENTION_DAYS (see
// DomainTablesRetentionDays)

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

// DomainTablesRetentionDays reads DOMAIN_TABLES_RETENTION_DAYS against
// retentionDays, RETENTION_DAYS: the days a session's table data (its rows in
// the domain tables and in process_errors) is kept, while the session itself is
// kept for retentionDays. Unset or empty is retentionDays, so the table data goes
// with the session, as it did before the variable existed. A value above
// retentionDays is capped to it, and capped says so: the session's own deletion
// takes its table data anyway. Anything that is not a whole number of days, 1
// or more, is an error.
func DomainTablesRetentionDays(value string, retentionDays int) (days int, capped bool, err error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return retentionDays, false, nil
	}
	n, err := strconv.Atoi(value)
	if err != nil || n < 1 {
		return 0, false, fmt.Errorf(
			"DOMAIN_TABLES_RETENTION_DAYS must be a whole number of days, 1 or more, or unset; it is %q", value)
	}
	if n > retentionDays {
		return retentionDays, true, nil
	}
	return n, false, nil
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

	// DOMAIN_TABLES_RETENTION_DAYS is read by purgeSessions, which refuses a bad
	// value once the database is open, so the refusal is recorded as the job's
	// failure.
	return purgeSessions(context.Background(), dbpool, purgeConfig{
		RetentionDays:         retentionDays,
		DomainTablesRetention: os.Getenv("DOMAIN_TABLES_RETENTION_DAYS"),
		AuditRetentionDays:    auditDays,
		DryRun:                dryRun,
		Now:                   start,
		Deadline:              start.Add(runBudget),
	})
}

type purgeConfig struct {
	RetentionDays int
	// DomainTablesRetention is DOMAIN_TABLES_RETENTION_DAYS as set, read by
	// DomainTablesRetentionDays; empty is RetentionDays.
	DomainTablesRetention string
	// AuditRetentionDays is the agent audit trail's retention; 0 never purges it.
	AuditRetentionDays int
	DryRun             bool
	Now                time.Time // the run's start, from which every age is counted
	Deadline           time.Time // no session is started after it
}

// purgeSessions is the purge, given a database:
//
//  1. Rules unit test runs and detection runs, by age; a dry run counts them
//     in a transaction it rolls back.
//  2. A dry run reads the retention and orphan candidates, runs them through
//     sessionpurge.ExecuteRequests with every transaction rolled back, logs the
//     report and stops. It writes nothing.
//  3. A real run queues the retention candidates, then the orphans, executes the
//     queue until the deadline, deletes the table data of the sessions past
//     DOMAIN_TABLES_RETENTION_DAYS that it did not delete whole (purgeTableData),
//     sweeps the agentic incidents no session they stand for is still held
//     (jets/agentic/retention), purges the agent audit trail when its retention
//     is set, purges the archive and the tombstones, and runs VACUUM.
//
// A dry run deletes the table data and sweeps the incidents inside its own
// transaction, after its sessions, so it reports the table data and the
// incidents a real run would delete, and counts the audit rows a real run would
// delete; it writes nothing.
//
// A DOMAIN_TABLES_RETENTION_DAYS that is not a whole number of days, 1 or more,
// refuses the run before it does anything, and is recorded as its failure.
//
// A session or an incident that fails is logged and retried by the next run; it
// is not an error of the run, which would have the lambda retried, and neither
// is a failure of the agentic sweep or of the audit retention, which are logged.
//
// Every run that goes wrong, whether it stops with an error or only fails some
// of its work, records one failure of the purge job (recordRun), so the failure
// is seen on the Pipelines Status screen and by the agentic triage rather than
// only in the lambda's log. A dry run records too: its failure is the job's
// failure, and the sessions it fails are those a real run would fail.
func purgeSessions(ctx context.Context, dbpool *pgxpool.Pool, cfg purgeConfig) (err error) {
	var problems []string
	defer func() { recordRun(ctx, dbpool, cfg, err, problems) }()

	tableDays, capped, err := DomainTablesRetentionDays(cfg.DomainTablesRetention, cfg.RetentionDays)
	if err != nil {
		return err
	}
	purgeFrom := cfg.Now.AddDate(0, 0, -cfg.RetentionDays)
	tableDataFrom := cfg.Now.AddDate(0, 0, -tableDays)
	mode := "real run"
	if cfg.DryRun {
		mode = "dry run, PURGE_DRY_RUN is not false: nothing is deleted"
	}
	log.Printf("Purging all sessions prior to %s, RETENTION_DAYS %d (%s)", purgeFrom, cfg.RetentionDays, mode)
	switch {
	case capped:
		log.Printf("DOMAIN_TABLES_RETENTION_DAYS %s is above RETENTION_DAYS %d and is capped to it: "+
			"the table data goes with its session", strings.TrimSpace(cfg.DomainTablesRetention), cfg.RetentionDays)
	case tableDays == cfg.RetentionDays:
		log.Printf("Table data is kept as long as its session: DOMAIN_TABLES_RETENTION_DAYS is %d",
			tableDays)
	default:
		log.Printf("Purging the table data of sessions prior to %s, DOMAIN_TABLES_RETENTION_DAYS %d",
			tableDataFrom, tableDays)
	}
	// The table-data purge runs only on a clock of its own: on RETENTION_DAYS's,
	// every session it could select is deleted whole, and the run is today's.
	tableData := tableDays < cfg.RetentionDays

	// Rules unit test runs are not sessions, so they are purged by age, as they
	// were before the session purge was rebuilt.
	purgeRulesUnitTestRuns(dbpool, purgeFrom, cfg.DryRun)
	purgeDetectionRuns(dbpool, purgeFrom, cfg.DryRun)

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
			if tableData {
				problems = append(problems, purgeTableData(ctx, tx, tableDataFrom, purgeFrom, opts,
					"DRY RUN, nothing written: ")...)
			}
			problems = append(problems, sweepIncidents(ctx, tx, sweepOpts, "DRY RUN, nothing written: ")...)
			return nil
		}
		report, err := sessionpurge.ExecuteRequests(ctx, dbpool, append(sessionCands, orphans...), opts)
		logReport("DRY RUN, nothing written: ", &report)
		problems = append(problems, failedSessions(&report)...)
		if err != nil {
			return fmt.Errorf("while executing the dry run: %v", err)
		}
		problems = append(problems, purgeAuditTrail(ctx, dbpool, cfg, "DRY RUN, nothing written: ")...)
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
	problems = append(problems, failedSessions(&report)...)
	if err != nil {
		return fmt.Errorf("while executing the session deletions: %v", err)
	}
	if tableData {
		problems = append(problems, purgeTableData(ctx, dbpool, tableDataFrom, purgeFrom, opts, "")...)
	}
	problems = append(problems, sweepIncidents(ctx, dbpool, sweepOpts, "")...)
	problems = append(problems, purgeAuditTrail(ctx, dbpool, cfg, "")...)

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

// purgeTableData deletes the table data of the sessions registered on or before
// before and after wholeBefore, the session deletion's cutoff, under opts's
// deadline, and logs its report under "Table data purge:", apart from the
// session deletions'. A failure is logged, not returned: the sessions are
// selected again by the next run. What failed is returned as lines for the run's
// failure record, none when nothing did.
func purgeTableData(ctx context.Context, db sessionpurge.TxBeginner, before, wholeBefore time.Time,
	opts sessionpurge.Options, prefix string) []string {
	var problems []string
	r, err := sessionpurge.PurgeTableData(ctx, db, before, wholeBefore, opts)
	log.Printf("Table data purge: %s%s", prefix, r.Summary())
	tables := make([]string, 0, len(r.Rows))
	for t := range r.Rows {
		tables = append(tables, t)
	}
	sort.Strings(tables)
	for _, t := range tables {
		log.Printf("   table data: %s%d rows from %s", prefix, r.Rows[t], t)
	}
	if len(r.RefusedSessions) > 0 {
		log.Printf("   table data: refused, a run pending or submitted: %s", listed(r.RefusedSessions))
	}
	if len(r.HeldSessions) > 0 {
		log.Printf("   table data: held, claim rows feed_detect has not yet summarised: %s", listed(r.HeldSessions))
	}
	if len(r.FailedSessions) > 0 {
		log.Printf("   table data: failed, retried by the next run: %s", strings.Join(r.FailedSessions, ", "))
		named := make([]string, 0, min(len(r.FailedSessions), maxListed))
		for _, id := range r.FailedSessions[:min(len(r.FailedSessions), maxListed)] {
			named = append(named, fmt.Sprintf("%s (%s)", id, r.FailedErrors[id]))
		}
		more := ""
		if n := len(r.FailedSessions) - len(named); n > 0 {
			more = fmt.Sprintf(", and %d more", n)
		}
		problems = append(problems, fmt.Sprintf("%d sessions' table data failed to delete, retried by the next run: %s%s",
			len(r.FailedSessions), strings.Join(named, "; "), more))
	}
	if err != nil {
		log.Println("Warning: while purging the table data:", err)
		problems = append(problems, fmt.Sprintf("the table data purge failed: %v", err))
	}
	return problems
}

// sweepIncidents runs the agentic retention sweep and logs its report: the
// incidents deleted, those kept because a delivery or run still held is placed
// in them, and the rows per table. A failure is logged, not returned: the
// incidents are swept again by the next run. What failed is returned as lines
// for the run's failure record, none when nothing did.
func sweepIncidents(ctx context.Context, db retention.DB, opts retention.Options, prefix string) []string {
	var problems []string
	r, err := retention.Sweep(ctx, db, opts)
	log.Printf("Incident retention: %s%s", prefix, r.Summary())
	for _, t := range r.Tables() {
		log.Printf("   incident retention: %s%d rows from %s", prefix, r.Rows[t], t)
	}
	if len(r.FailedIncidents) > 0 {
		log.Printf("   incident retention: failed, retried by the next run: %s", strings.Join(r.FailedIncidents, ", "))
		problems = append(problems, fmt.Sprintf("%d incidents failed to delete, retried by the next run: %s",
			len(r.FailedIncidents), listed(r.FailedIncidents)))
	}
	if err != nil {
		log.Println("Warning: while sweeping the agentic incidents:", err)
		problems = append(problems, fmt.Sprintf("the incident retention sweep failed: %v", err))
	}
	return problems
}

// purgeAuditTrail purges the agent's audit trail older than its retention, or
// says it is kept when none is set. A failure is logged, not returned, and is
// returned as a line for the run's failure record.
func purgeAuditTrail(ctx context.Context, db *pgxpool.Pool, cfg purgeConfig, prefix string) []string {
	if cfg.AuditRetentionDays < 1 {
		log.Println("Agent audit retention: AGENT_AUDIT_RETENTION_DAYS is not set, the audit trail is kept")
		return nil
	}
	before := cfg.Now.AddDate(0, 0, -cfg.AuditRetentionDays)
	r, err := retention.PurgeAudit(ctx, db, before, cfg.DryRun)
	if err != nil {
		log.Println("Warning: while purging the agent audit trail:", err)
		return []string{fmt.Sprintf("the agent audit retention failed: %v", err)}
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
	return nil
}

// recordFailure records a failure of the purge job; a variable so a test can
// make the record itself fail. Production code must not reassign it.
var recordFailure = datatable.RecordJobFailure

// maxListed is how many sessions or incidents a failure record names; the rest
// are counted. The log names them all.
const maxListed = 50

// recordRun records the run as one failure of the purge job when it went wrong:
// runErr is the error that stopped it, and problems the work it failed and went
// on from (sessions it could not delete, the incident sweep, the audit
// retention). One row per run, however many things failed, so a session that
// fails every day is one failure a day, not one per session. A run that went
// right records nothing. The record is written even when the run's context was
// cancelled, since a cancelled run is a failed one, and a failure to record is
// logged: the run's own error is what the lambda returns.
func recordRun(ctx context.Context, dbpool *pgxpool.Pool, cfg purgeConfig, runErr error, problems []string) {
	if runErr == nil && len(problems) == 0 {
		return
	}
	var lines []string
	if cfg.DryRun {
		lines = append(lines, "dry run, nothing was deleted")
	}
	if runErr != nil {
		lines = append(lines, "the purge stopped: "+runErr.Error())
	}
	lines = append(lines, problems...)
	sessionId, err := recordFailure(context.WithoutCancel(ctx), dbpool, datatable.JobFailure{
		Job:     datatable.JobPurgeData,
		Details: strings.Join(lines, "\n"),
	})
	if err != nil {
		log.Println("Warning: the purge's failure could not be recorded:", err)
		return
	}
	log.Printf("Recorded the purge's failure under session %s", sessionId)
}

// failedSessions is the record's line for the sessions a run failed to delete,
// with the error of each, or none.
func failedSessions(r *sessionpurge.Report) []string {
	if len(r.FailedSessions) == 0 {
		return nil
	}
	named := make([]string, 0, min(len(r.FailedSessions), maxListed))
	for _, id := range r.FailedSessions[:min(len(r.FailedSessions), maxListed)] {
		named = append(named, fmt.Sprintf("%s (%s)", id, r.FailedErrors[id]))
	}
	more := ""
	if n := len(r.FailedSessions) - len(named); n > 0 {
		more = fmt.Sprintf(", and %d more", n)
	}
	return []string{fmt.Sprintf("%d sessions failed to delete, retried by the next run: %s%s",
		len(r.FailedSessions), strings.Join(named, "; "), more)}
}

// listed names up to maxListed ids and counts the rest.
func listed(ids []string) string {
	if len(ids) <= maxListed {
		return strings.Join(ids, ", ")
	}
	return fmt.Sprintf("%s, and %d more", strings.Join(ids[:maxListed], ", "), len(ids)-maxListed)
}

// purgeRulesUnitTestRuns deletes the rules unit test runs last updated on or
// before purgeFrom, with their checks. A run still queued or executing is kept
// whatever its age: the apiserver's start-up sweep is what ends those. Errors are
// logged and not returned, as for the other secondary purges here -- a database
// the apiserver has not migrated yet has neither table. purgeFrom is cast to
// timestamptz because last_update is a timestamp without time zone and pgx
// would otherwise encode it by its wall clock in its own location.
//
// A dry run deletes in a transaction it rolls back, so it counts what a real
// run would delete and writes nothing (JM-349: it once deleted for real).
func purgeRulesUnitTestRuns(dbpool *pgxpool.Pool, purgeFrom time.Time, dryRun bool) (runs, checks int64) {
	err := ageTx(dbpool, dryRun, func(ctx context.Context, tx pgx.Tx) error {
		tag, err := tx.Exec(ctx, `DELETE FROM jetsapi.rules_unit_test_check WHERE execution_key IN (
			SELECT key FROM jetsapi.rules_unit_test_execution
			WHERE last_update <= $1::timestamptz AND status NOT IN ('queued', 'executing'))`, purgeFrom)
		if err != nil {
			return fmt.Errorf("while purging rules_unit_test_check: %w", err)
		}
		checks = tag.RowsAffected()
		tag, err = tx.Exec(ctx, `DELETE FROM jetsapi.rules_unit_test_execution
			WHERE last_update <= $1::timestamptz AND status NOT IN ('queued', 'executing')`, purgeFrom)
		if err != nil {
			return fmt.Errorf("while purging rules_unit_test_execution: %w", err)
		}
		runs = tag.RowsAffected()
		return nil
	})
	if err != nil {
		log.Println("Warning:", err)
		return 0, 0
	}
	log.Printf("%sPurged %d rules unit test runs and %d of their checks\n", dryRunPrefix(dryRun), runs, checks)
	return runs, checks
}

// purgeDetectionRuns deletes the records of agentic detection runs last updated
// on or before purgeFrom (jets/agentic/detectrun). The anomalies those runs
// wrote are deleted with their sessions, and their incidents by the agentic
// retention sweep, not here. A run still
// requested or running is kept whatever its age, as purgeRulesUnitTestRuns keeps
// a queued test, and errors are logged and not returned for the same reason:
// an unmigrated database has no table. A dry run counts and writes nothing, as
// purgeRulesUnitTestRuns's does.
func purgeDetectionRuns(dbpool *pgxpool.Pool, purgeFrom time.Time, dryRun bool) int64 {
	var n int64
	err := ageTx(dbpool, dryRun, func(ctx context.Context, tx pgx.Tx) error {
		tag, err := tx.Exec(ctx, `DELETE FROM jetsapi.agentic_detection_run
			WHERE last_update <= $1::timestamptz AND status NOT IN ('requested', 'running')`, purgeFrom)
		if err != nil {
			return fmt.Errorf("while purging agentic_detection_run: %w", err)
		}
		n = tag.RowsAffected()
		return nil
	})
	if err != nil {
		log.Println("Warning:", err)
		return 0
	}
	log.Printf("%sPurged %d agentic detection run records\n", dryRunPrefix(dryRun), n)
	return n
}

// ageTx runs one of the age purges in a transaction, committed by a real run
// and rolled back by a dry run.
func ageTx(dbpool *pgxpool.Pool, dryRun bool, purge func(ctx context.Context, tx pgx.Tx) error) error {
	ctx := context.Background()
	tx, err := dbpool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("while starting an age purge: %w", err)
	}
	defer tx.Rollback(ctx)
	if err := purge(ctx, tx); err != nil {
		return err
	}
	if dryRun {
		return nil // rolled back by the deferred Rollback
	}
	return tx.Commit(ctx)
}

// dryRunPrefix is the prefix of a dry run's log lines.
func dryRunPrefix(dryRun bool) string {
	if dryRun {
		return "DRY RUN, nothing written: "
	}
	return ""
}
