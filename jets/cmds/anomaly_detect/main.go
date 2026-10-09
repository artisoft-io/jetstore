// anomaly_detect runs JetStore's deterministic anomaly detectors over the
// execution record and writes what they find to jetsapi.anomaly.
//
// It is the "Go emitter" half of the shape N.3 decided (Phase 3 section 18):
// the detectors are two SQL reads and a predicate each, and this is the process
// that runs them. Two detectors, section 12.2 rows 2 and 3 — an output count
// that collapsed relative to its input, and a step that failed having
// previously succeeded on the same source.
//
// Usage:
//
//	anomaly_detect -since 24h -baseline 720h
//	anomaly_detect -session <session id> -dry-run
//
// The DSN comes from JETS_DSN_URI_VALUE, or from JETS_DSN_JSON_VALUE by the
// same route update_db and cpipes_server take (awsi.GetDsnFromJson), or from
// -dsn. Nothing here writes outside jetsapi.anomaly.
//
// # Two things it refuses to do rather than doing badly
//
// It refuses to run when jetsapi.anomaly is absent, naming the migration.
// The table is created by `update_db -migrateDb`, which calls
// audit.InstallSchema (jets/update_db/main.go:71), and it was absent from all
// four production environments measured on 2026-08-25 — so the first symptom
// of running this against an unmigrated deployment would be an insert failing
// on a missing relation, which reads as a defect in a detector (I-169).
//
// And it refuses to run the step-regression detector with -baseline 0, rather
// than comparing against an empty history. A step with no baseline is not a
// step that never fails.
package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/artisoft-io/jetstore/jets/agentic/observe"
	"github.com/artisoft-io/jetstore/jets/awsi"
	"github.com/jackc/pgx/v5/pgxpool"
)

var (
	dsnFlag  = flag.String("dsn", "", "database DSN; defaults to JETS_DSN_URI_VALUE, then JETS_DSN_JSON_VALUE")
	since    = flag.Duration("since", 24*time.Hour, "judge worker rows that started within this window")
	baseline = flag.Duration("baseline", 30*24*time.Hour, "how far back before the window the step baseline reaches; 0 runs the volume detector only")
	client   = flag.String("client", "", "restrict to one client")
	process  = flag.String("process", "", "restrict to one process name")
	session  = flag.String("session", "", "restrict to one run; the baseline is never restricted with it")
	dryRun   = flag.Bool("dry-run", false, "report what would be written and write nothing")

	minRatio     = flag.Float64("min-ratio", 0.5, "volume collapse fires below this output/input ratio")
	minInput     = flag.Int64("min-input", 1000, "volume collapse ignores workers that read fewer rows than this")
	minPriorRuns = flag.Int64("min-prior-runs", 3, "step regression needs at least this many prior runs in the baseline")
	generation   = flag.Int("generation", 1, "detector generation; bumping it re-emits over rows already judged")

	usingSshTunnel = flag.Bool("usingSshTunnel", false, "connect to the database through an open ssh tunnel")
)

func resolveDsn() (string, error) {
	if *dsnFlag != "" {
		return *dsnFlag, nil
	}
	if v := os.Getenv("JETS_DSN_URI_VALUE"); v != "" {
		return v, nil
	}
	dsn, err := awsi.GetDsnFromJson(os.Getenv("JETS_DSN_JSON_VALUE"), *usingSshTunnel, 3)
	if err != nil {
		return "", fmt.Errorf("while reading the dsn from JETS_DSN_JSON_VALUE: %v", err)
	}
	if dsn == "" {
		return "", fmt.Errorf("no dsn: set -dsn, JETS_DSN_URI_VALUE or JETS_DSN_JSON_VALUE")
	}
	return dsn, nil
}

func doJob(ctx context.Context, db *pgxpool.Pool) error {
	detect := observe.Window{
		Since:       time.Now().UTC().Add(-*since),
		Client:      *client,
		ProcessName: *process,
		SessionId:   *session,
	}
	ev, err := observe.Gather(ctx, db, detect, *baseline)
	if err != nil {
		return err
	}

	// What the deployment's own retention has left, said before anything is
	// judged: a quiet window and a purged one look the same in the output.
	log.Printf("record: %d run headers, oldest %s; oldest worker row %s; purge regime %q",
		ev.Extent.Headers, stamp(ev.Extent.OldestHeader), stamp(ev.Extent.OldestWorker), ev.Extent.Regime())
	log.Printf("window: %d worker rows since %s, %d of them with no surviving run header",
		len(ev.Workers.Rows), detect.Since.Format(time.RFC3339), ev.Workers.Headerless)
	if ev.Workers.Truncated {
		log.Printf("window: the read was capped, so this is not the whole of it")
	}
	if !ev.Extent.ChannelDetails {
		log.Printf("record: jetsapi.pipeline_execution_channel_details is absent, so no anomaly can say " +
			"which DAG edge lost rows; every volume anomaly carries cross_step_join_unavailable")
	}

	found := append([]observe.Anomaly(nil),
		observe.VolumeCollapse{MinRatio: *minRatio, MinInput: *minInput, Generation: *generation}.Detect(ev)...)
	if ev.Prior != nil {
		reg, err := observe.StepRegression{MinPriorRuns: *minPriorRuns, Generation: *generation}.Detect(ev)
		if err != nil {
			return err
		}
		found = append(found, reg...)
		log.Printf("baseline: %d (client, process, object type, step) baselines over %s, %d worker rows "+
			"excluded for having no run header", len(ev.Prior.Baselines), baseline.String(), ev.Prior.Headerless)
	} else {
		log.Printf("baseline: none read, so the step-regression detector did not run")
	}

	if len(found) == 0 {
		log.Printf("no anomalies")
		return nil
	}
	written, already := 0, 0
	for i := range found {
		a := &found[i]
		log.Printf("%s %s %s subject %s observed %q confounders %v",
			a.DetectorRef, a.SignalType, a.SubjectType, a.SubjectRef, a.ObservedValue, a.Confounders)
		log.Printf("    basis: %s", a.ExpectedBasis)
		if *dryRun {
			continue
		}
		isNew, err := observe.InsertAnomalyIfNew(ctx, db, a)
		if err != nil {
			return err
		}
		if isNew {
			written++
		} else {
			already++
		}
	}
	switch {
	case *dryRun:
		log.Printf("%d anomalies, none written (-dry-run)", len(found))
	default:
		log.Printf("%d anomalies: %d written, %d already recorded by this detector generation",
			len(found), written, already)
	}
	return nil
}

func stamp(t time.Time) string {
	if t.IsZero() {
		return "(none)"
	}
	return t.Format(time.RFC3339)
}

func main() {
	flag.Parse()
	dsn, err := resolveDsn()
	if err != nil {
		log.Fatal(err)
	}
	ctx := context.Background()
	db, err := pgxpool.New(ctx, dsn)
	if err != nil {
		log.Fatalf("while opening the database connection: %v", err)
	}
	defer db.Close()

	extent, err := observe.ReadExtent(ctx, db)
	if err != nil {
		log.Fatal(err)
	}
	if !extent.ExecutionRecord {
		log.Fatal("jetsapi.pipeline_execution_status and jetsapi.pipeline_execution_details are not " +
			"both present in this database, so there is nothing to read. They are created by " +
			"`update_db -migrateDb`; run that first.")
	}
	if !extent.Anomalies {
		log.Fatal("jetsapi.anomaly does not exist in this database, so there is nowhere to write. " +
			"It is created by `update_db -migrateDb`, which also creates " +
			"jetsapi.pipeline_execution_channel_details; run that first.")
	}
	if err := doJob(ctx, db); err != nil {
		log.Fatal(err)
	}
}
