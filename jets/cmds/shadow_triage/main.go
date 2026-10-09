// shadow_triage runs JetStore's deterministic triage classifier and its RCA
// floor over the execution record, writes what they concluded to
// jetsapi.incident and jetsapi.hypothesis, and **acts on none of it**.
//
// It is the process half of task AC.3, on jets/cmds/anomaly_detect's model: the
// classifier and the ranker are libraries and this is the thing that runs them
// against a deployment.
//
// Usage:
//
//	shadow_triage -since 24h                  # every run that started in the window
//	shadow_triage -session <session id>       # one run
//	shadow_triage -since 24h -dry-run         # classify, rank, report, write nothing
//	shadow_triage -attest                     # read the record back and say whether anything acted
//
// The DSN comes from JETS_DSN_URI_VALUE, or from JETS_DSN_JSON_VALUE by the same
// route update_db and cpipes_server take (awsi.GetDsnFromJson), or from -dsn.
//
// # What it will not do
//
// **It writes no status past `diagnosed`.** Appendix A.5's incident machine makes
// `remediation_proposed` the point at which a corrective action has a subject,
// and every status beyond it is reachable only through it — so shadow mode's
// ceiling is that articulation point and not a list somebody typed. The guard is
// in the library (shadow.ErrWouldAct) rather than here, so a second caller
// inherits it.
//
// **It writes nothing to jetsapi.remediation**, which has no writer anywhere in
// the tree — asserted over the source by shadow's own suite rather than left as
// a fact about what nobody got round to.
//
// **It refuses to run against a database that has not been migrated**, naming
// the command. jetsapi.incident, hypothesis, incident_event and remediation all
// arrive on `update_db -migrateDb`, and no production environment measured on
// 2026-08-25 had them (I-132, I-169). Untreated, the first symptom would be an
// insert failing on a missing relation, which reads as a defect in the
// classifier.
//
// **It does not reclassify.** A second run over a session it has already raised
// incidents for adds the loci that are newly present and touches nothing else: a
// verdict is a dated claim about what the record supported when it was read, and
// a later read that can see less is not evidence against it (Q-34). What corrects
// an incident is a person, through the supervision screen's `reclassified`
// transition.
package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/artisoft-io/jetstore/jets/agentic/rca"
	"github.com/artisoft-io/jetstore/jets/agentic/shadow"
	"github.com/artisoft-io/jetstore/jets/agentic/triage"
	"github.com/artisoft-io/jetstore/jets/awsi"
	"github.com/jackc/pgx/v5/pgxpool"
)

var (
	dsnFlag  = flag.String("dsn", "", "database DSN; defaults to JETS_DSN_URI_VALUE, then JETS_DSN_JSON_VALUE")
	since    = flag.Duration("since", 24*time.Hour, "classify runs that started within this window")
	baseline = flag.Duration("baseline", 30*24*time.Hour, "how far back before each run the step history reaches; 0 reads none, which makes locus step_never_started not_evaluable")
	client   = flag.String("client", "", "restrict to one client")
	process  = flag.String("process", "", "restrict to one process name")
	session  = flag.String("session", "", "classify one run and ignore -since")
	limit    = flag.Int("limit", 200, "at most this many runs")
	dryRun   = flag.Bool("dry-run", false, "classify, rank and report; write nothing")
	attest   = flag.Bool("attest", false, "read the incident record back and report whether anything acted; classify nothing")

	actor        = flag.String("actor", "", "the agent identity recorded on every transition; defaults to the classifier ref")
	severity     = flag.String("severity", shadow.SeverityInfo, "the severity written on every incident. No locus determines one (I-306) and shadow mode's default is `info`, argued from the posture: nothing acts on these rows")
	modelVersion = flag.String("model-version", shadow.ModelVersion, "the domain model version written on every incident")
	generation   = flag.Int("generation", 1, "classifier and ranker generation")

	minRatio     = flag.Float64("min-ratio", 0.5, "locus rows_lost_silently fires below this output/input ratio (unsourced, R-21)")
	minInput     = flag.Int64("min-input", 1000, "locus rows_lost_silently ignores workers that read fewer rows than this (unsourced, R-21)")
	minPriorRuns = flag.Int64("min-prior-runs", 3, "locus step_never_started needs at least this many prior runs in the baseline (unsourced, R-36)")

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

func writerFromFlags() shadow.Writer {
	c := triage.Default()
	c.Generation = *generation
	c.MinPriorRuns = *minPriorRuns
	c.Volume.MinRatio = *minRatio
	c.Volume.MinInput = *minInput

	w := shadow.DefaultWriter()
	w.Classifier = c
	w.Ranker = rca.Ranker{Generation: *generation}
	w.Actor = *actor
	if w.Actor == "" {
		w.Actor = c.Ref()
	}
	w.Severity = *severity
	w.ModelVersion = *modelVersion
	w.Baseline = *baseline
	w.DryRun = *dryRun
	return w
}

func doAttest(ctx context.Context, db *pgxpool.Pool) error {
	a, err := shadow.Attest(ctx, db)
	if err != nil {
		return err
	}
	fmt.Print(a.Report())
	if !a.Holds() {
		// A non-zero exit, so that a deployment can run this as a check rather
		// than as a report somebody reads.
		return fmt.Errorf("the record does not show that nothing acted")
	}
	return nil
}

func doJob(ctx context.Context, db *pgxpool.Pool) error {
	targets, err := shadow.ReadTargets(ctx, db)
	if err != nil {
		return err
	}
	if !targets.Ready() {
		if !*dryRun {
			return fmt.Errorf("%v absent from this database; `update_db -migrateDb` is what installs them",
				targets.Missing())
		}
		log.Printf("record: %v absent; -dry-run classifies anyway and writes nothing", targets.Missing())
	}

	sessions := []string{*session}
	if *session == "" {
		if sessions, err = shadow.RecentSessions(ctx, db, time.Now().UTC().Add(-*since),
			*client, *process, *limit); err != nil {
			return err
		}
	}
	if len(sessions) == 0 {
		log.Printf("no run started in the last %s", since.String())
		return nil
	}
	log.Printf("classifying %d run(s); severity is %q for every incident, which is a posture and not a "+
		"measurement (I-306)", len(sessions), *severity)

	w := writerFromFlags()
	census := map[string]int{}
	incidents, hypotheses, already := 0, 0, 0
	for _, s := range sessions {
		res, err := w.Run(ctx, db, s)
		if err != nil {
			return err
		}
		for i := range res.Report.Findings {
			f := &res.Report.Findings[i]
			census[fmt.Sprintf("%s/%s", f.Locus, f.Verdict)]++
		}
		already += len(res.AlreadyRaised)
		for _, wr := range res.Written {
			incidents++
			hypotheses += wr.Hypotheses
			log.Printf("%s %s locus %s -> %s, %d hypothesis(es), audit chain seq %d",
				s, wr.IncidentId, wr.Locus, wr.Status, wr.Hypotheses, wr.ChainSeq)
		}
		if len(res.UnmappedLoci) > 0 {
			log.Printf("%s: %v fired and §9.5 maps them to no cause class, so those incidents carry no hypothesis",
				s, res.UnmappedLoci)
		}
	}

	// The per-locus census, printed whatever fired. A classifier that reported
	// only its hits would supply numerators and no denominators, and so would a
	// process that printed only what it wrote.
	log.Printf("%-38s %8s %8s %14s", "locus", "present", "absent", "not_evaluable")
	for _, l := range triage.Loci {
		log.Printf("%-38s %8d %8d %14d", l,
			census[l+"/"+string(triage.Present)],
			census[l+"/"+string(triage.Absent)],
			census[l+"/"+string(triage.NotEvaluable)])
	}
	switch {
	case *dryRun:
		log.Printf("%d incident(s) would be written over %d run(s), with %d hypothesis(es); nothing written (-dry-run)",
			incidents, len(sessions), hypotheses)
		return nil
	default:
		log.Printf("%d incident(s) written over %d run(s), with %d hypothesis(es); %d locus/session pairs "+
			"were already raised and were left alone (Q-34)", incidents, len(sessions), hypotheses, already)
	}

	// Say what the record now shows, in the same run that wrote it. Criterion 47
	// asks for the audit record to show that nothing acted, and the cheapest way
	// for that to be true of an operator's console is for the writer to print it.
	return doAttest(ctx, db)
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

	if *attest {
		if err := doAttest(ctx, db); err != nil {
			log.Fatal(err)
		}
		return
	}

	extent, err := triage.ReadExtent(ctx, db)
	if err != nil {
		log.Fatal(err)
	}
	if !extent.ExecutionRecord {
		log.Fatal("jetsapi.pipeline_execution_status and jetsapi.pipeline_execution_details are not " +
			"both present in this database, so there is nothing to classify. They are created by " +
			"`update_db -migrateDb`; run that first.")
	}
	log.Printf("record: %d run headers, oldest %s; oldest worker row %s; purge regime %q",
		extent.Headers, stamp(extent.OldestHeader), stamp(extent.OldestWorker), extent.Regime())
	if !extent.ChannelDetails {
		log.Printf("record: jetsapi.pipeline_execution_channel_details is absent, so locus " +
			"sink_failed_under_completed_worker is not_evaluable on every run here (I-132)")
	}
	if err := doJob(ctx, db); err != nil {
		log.Fatal(err)
	}
}

func stamp(t time.Time) string {
	if t.IsZero() {
		return "(none)"
	}
	return t.Format(time.RFC3339)
}
