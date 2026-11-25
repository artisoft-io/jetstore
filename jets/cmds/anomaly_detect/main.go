// anomaly_detect runs JetStore's deterministic anomaly detectors over the
// execution record and writes what they find to jetsapi.anomaly.
//
// It is the "Go emitter" half of the agentic layer's detection design:
// the detectors are two SQL reads and a predicate each, and this is the process
// that runs them. Two detectors — an output count
// that collapsed relative to what the same step usually writes for what it reads,
// and a step that failed having previously succeeded on the same source.
//
// The volume detector judges each (client, process, step) against the median
// ratio of its own recent runs, and applies an absolute floor only to a step with
// too little history; the -volume-* flags and -min-ratio set both, and their
// defaults are unsourced. Its generation is set apart from step regression's, so
// that the change of basis re-judges volume without re-emitting step regression.
//
// Usage:
//
//	anomaly_detect -since 24h -baseline 720h
//	anomaly_detect -session <session id> -dry-run
//
// The DSN comes from JETS_DSN_URI_VALUE, or from JETS_DSN_JSON_VALUE by the
// same route update_db and cpipes_native_server take (awsi.GetDsnFromJson), or from
// -dsn. Nothing here writes outside jetsapi.anomaly.
//
// # Two things it refuses to do rather than doing badly
//
// It refuses to run when jetsapi.anomaly is absent, naming the migration.
// The table is created by `update_db -migrateDb`, which calls
// audit.InstallSchema (jets/update_db/main.go:71), and it was absent from all
// four production environments measured on 2026-08-25 — so the first symptom
// of running this against an unmigrated deployment would be an insert failing
// on a missing relation, which reads as a defect in a detector.
//
// And it refuses to run the step-regression detector with -baseline 0, rather
// than comparing against an empty history. A step with no baseline is not a
// step that never fails.
//
// # Its own run is recorded too
//
// Each run writes one row to jetsapi.agentic_detection_run, from start to end,
// with what it found as the row's summary (jets/agentic/detectrun). That row is
// what the Detection screen shows, and a refusal above is recorded as a failed
// run rather than left as an absence.
package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/artisoft-io/jetstore/jets/agentic/detectrun"
	"github.com/artisoft-io/jetstore/jets/agentic/observe"
	"github.com/artisoft-io/jetstore/jets/awsi"
	"github.com/artisoft-io/jetstore/jets/utils"
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

	// The volume detector's thresholds. Each step is judged against its own
	// history, and the floor applies only to a step with too little of it
	// (observe.VolumeCollapse). Every default is unsourced, as the floor was:
	// see the file comment of jets/agentic/observe/detect.go.
	minRatio          = flag.Float64("min-ratio", 0.5, "volume collapse: the absolute floor on the output/input ratio, applied only to a step with fewer than -volume-min-prior-runs prior runs")
	minInput          = flag.Int64("min-input", 1000, "volume collapse ignores workers that read fewer rows than this, and runs of a step that read fewer in all")
	volumeHistoryRuns = flag.Int("volume-history-runs", 10, "volume collapse: how many of a step's most recent prior runs its baseline is the median of")
	volumeMinPrior    = flag.Int("volume-min-prior-runs", 3, "volume collapse: how many prior runs a step needs before it is judged against its own history rather than the floor")
	volumeMaxDrop     = flag.Float64("volume-drop", 0.5, "volume collapse fires when a step's ratio falls this far below its own median: 0.5 fires below half of it")
	volumeGeneration  = flag.Int("volume-generation", 2, "volume collapse generation; 2 is the first to judge a step against its own history, so it re-judges the rows generation 1 judged")

	minPriorRuns = flag.Int64("min-prior-runs", 3, "step regression needs at least this many prior runs in the baseline")
	generation   = flag.Int("generation", 1, "step regression generation; bumping it re-emits over rows already judged")

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

// summary is what this run recorded as its row's summary.
type summary struct {
	DryRun         bool           `json:"dryRun"`
	Since          time.Time      `json:"since"`
	Regime         string         `json:"purgeRegime"`
	ChannelDetails bool           `json:"channelDetails"`
	WorkerRows     int            `json:"workerRows"`
	Headerless     int            `json:"headerlessWorkerRows"`
	Truncated      bool           `json:"truncated"`
	BaselineRead   bool           `json:"baselineRead"`
	Baselines      int            `json:"baselines"`
	VolumeSteps    int            `json:"volumeSteps"`
	Found          int            `json:"anomaliesFound"`
	Written        int            `json:"anomaliesWritten"`
	Already        int            `json:"anomaliesAlreadyRecorded"`
	ByDetector     map[string]int `json:"byDetector"`
}

func doJob(ctx context.Context, db *pgxpool.Pool, sum *summary) error {
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
	sum.Since = detect.Since
	sum.Regime = ev.Extent.Regime()
	sum.ChannelDetails = ev.Extent.ChannelDetails
	sum.WorkerRows = len(ev.Workers.Rows)
	sum.Headerless = ev.Workers.Headerless
	sum.Truncated = ev.Workers.Truncated

	// What the deployment's own retention has left, said before anything is
	// judged: a quiet window and a purged one look the same in the output.
	log.Printf("record: %d run headers, %d of them archived with their deleted sessions, oldest %s; "+
		"oldest worker row %s; purge regime %q",
		ev.Extent.Headers, ev.Extent.Archived, stamp(ev.Extent.OldestHeader), stamp(ev.Extent.OldestWorker), ev.Extent.Regime())
	log.Printf("window: %d worker rows since %s, %d of them with no surviving run header",
		len(ev.Workers.Rows), detect.Since.Format(time.RFC3339), ev.Workers.Headerless)
	if ev.Workers.Truncated {
		log.Printf("window: the read was capped, so this is not the whole of it")
	}
	if !ev.Extent.ChannelDetails {
		log.Printf("record: jetsapi.pipeline_execution_channel_details is absent, so no anomaly can say " +
			"which DAG edge lost rows; every volume anomaly carries cross_step_join_unavailable")
	}

	volume := observe.VolumeCollapse{
		MinRatio:     *minRatio,
		MinInput:     *minInput,
		HistoryRuns:  *volumeHistoryRuns,
		MinPriorRuns: *volumeMinPrior,
		MaxDrop:      *volumeMaxDrop,
		Generation:   *volumeGeneration,
	}
	if err := volume.ReadHistory(ctx, db, ev); err != nil {
		return err
	}
	sum.VolumeSteps = len(ev.Volume.Runs)
	log.Printf("volume: history read for %d steps, judged against the median of up to %d prior runs, "+
		"the floor of %.2f only where a step has fewer than %d", len(ev.Volume.Runs), volume.HistoryRuns,
		volume.MinRatio, volume.MinPriorRuns)
	found := append([]observe.Anomaly(nil), volume.Detect(ev)...)
	if ev.Prior != nil {
		reg, err := observe.StepRegression{MinPriorRuns: *minPriorRuns, Generation: *generation}.Detect(ev)
		if err != nil {
			return err
		}
		found = append(found, reg...)
		sum.BaselineRead = true
		sum.Baselines = len(ev.Prior.Baselines)
		log.Printf("baseline: %d (client, process, object type, step) baselines over %s, %d worker rows "+
			"excluded for having no run header", len(ev.Prior.Baselines), baseline.String(), ev.Prior.Headerless)
	} else {
		log.Printf("baseline: none read, so the step-regression detector did not run")
	}

	sum.Found = len(found)
	for i := range found {
		sum.ByDetector[found[i].DetectorRef]++
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
	sum.Written, sum.Already = written, already
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
	utils.UseJetStoreLogger()
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

	rec := detectrun.Begin(ctx, db, detectrun.CommandAnomalyDetect, detectrun.Window(*since))
	sum := &summary{DryRun: *dryRun, ByDetector: map[string]int{}}
	err = run(ctx, db, sum)
	rec.Finish(ctx, sum, err)
	if err != nil {
		log.Fatal(err)
	}
}

func run(ctx context.Context, db *pgxpool.Pool, sum *summary) error {
	extent, err := observe.ReadExtent(ctx, db)
	if err != nil {
		return err
	}
	if !extent.ExecutionRecord {
		return fmt.Errorf("jetsapi.pipeline_execution_status and jetsapi.pipeline_execution_details are not " +
			"both present in this database, so there is nothing to read. They are created by " +
			"`update_db -migrateDb`; run that first.")
	}
	if !extent.Anomalies {
		return fmt.Errorf("jetsapi.anomaly does not exist in this database, so there is nowhere to write. " +
			"It is created by `update_db -migrateDb`, which also creates " +
			"jetsapi.pipeline_execution_channel_details; run that first.")
	}
	return doJob(ctx, db, sum)
}
