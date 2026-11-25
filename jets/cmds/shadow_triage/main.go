// shadow_triage runs JetStore's deterministic triage classifier and its RCA
// floor over the execution record, writes what they concluded to
// jetsapi.incident and jetsapi.hypothesis, and **acts on none of it**.
//
// It is the process half of shadow-mode triage, on jets/cmds/anomaly_detect's model: the
// classifier and the ranker are libraries and this is the thing that runs them
// against a deployment.
//
// Usage:
//
//	shadow_triage -since 24h                  # every run that started in the window, and every
//	                                          # session carrying a feed anomaly detected in it
//	shadow_triage -session <session id>       # one run
//	shadow_triage -since 24h -dry-run         # classify, rank, report, write nothing
//	shadow_triage -feeds-only -since 6000h -limit 2000
//	                                          # only the sessions carrying a feed anomaly, after
//	                                          # a feed_detect backfill
//	shadow_triage -attest                     # read the record back and say whether anything acted
//
// The DSN comes from JETS_DSN_URI_VALUE, or from JETS_DSN_JSON_VALUE by the same
// route update_db and cpipes_native_server take (awsi.GetDsnFromJson), or from -dsn.
//
// # What it will not do
//
// **It writes no status past `diagnosed`.** The incident lifecycle makes
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
// 2026-08-25 had them. Untreated, the first symptom would be an
// insert failing on a missing relation, which reads as a defect in the
// classifier.
//
// **It does not reclassify.** A second run over a session it has already raised
// incidents for adds the loci that are newly present and touches nothing else: a
// verdict is a dated claim about what the record supported when it was read, and
// a later read that can see less is not evidence against it. What corrects
// an incident is a person, through the supervision screen's `reclassified`
// transition.
//
// # Feed findings, on the sessions they hang on
//
// A feed detector's anomaly hangs on a registration session: the delivery that
// departs from the feed's profile, or, for a feed that stopped, its last
// delivery, the one after which nothing came. Such a session may have no run,
// so -since selects the sessions carrying a feed anomaly detected in the window
// beside the runs that started in it, and the two delivery loci become
// incidents at `diagnosed` like any other. -client is the feed's own client,
// from its source configuration.
//
// **A feed's standing condition is one incident.** Consecutive deliveries of a
// feed carrying findings at one locus are a condition, with one incident on its
// first delivery; each later delivery joins it, and is logged as joined rather
// than written. A run that triages a condition's later delivery first raises
// the condition's incident on its first delivery. After a person's verdict, a
// finding detected later raises a new incident naming the judged one
// (jets/agentic/feedcondition).
//
// **-feeds-only classifies those sessions and no run**, for after a
// `feed_detect -mode backfill`. A backfill dates each finding at the first
// daily run that would have seen it, so raising its findings takes a -since
// reaching back months, and that window would otherwise classify every run
// in it as well, raising run incidents for runs no daily triage ever read.
//
// # A draft proposal for a feed that looks cumulative
//
// After the incidents are written, a feed whose latest deliveries each
// re-deliver most of their predecessor while its source configuration declares
// it incremental gets a change proposal in `draft`, proposing to declare it
// cumulative, with its incident as the trigger (jets/agentic/feedproposal).
// The incident is not moved, no remediation is written, and a second run does
// not draft it again. A person approves or rejects it on the proposals screen.
//
// # Its own run is recorded too
//
// Each run writes one row to jetsapi.agentic_detection_run with what it
// concluded -- the per-locus census, what it wrote, and whether the attestation
// held -- as the row's summary (jets/agentic/detectrun). That is what the
// Detection screen shows.
package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/artisoft-io/jetstore/jets/agentic/detectrun"
	"github.com/artisoft-io/jetstore/jets/agentic/rca"
	"github.com/artisoft-io/jetstore/jets/agentic/shadow"
	"github.com/artisoft-io/jetstore/jets/agentic/triage"
	"github.com/artisoft-io/jetstore/jets/awsi"
	"github.com/artisoft-io/jetstore/jets/utils"
	"github.com/jackc/pgx/v5/pgxpool"
)

var (
	dsnFlag   = flag.String("dsn", "", "database DSN; defaults to JETS_DSN_URI_VALUE, then JETS_DSN_JSON_VALUE")
	since     = flag.Duration("since", 24*time.Hour, "classify runs that started within this window")
	baseline  = flag.Duration("baseline", 30*24*time.Hour, "how far back before each run the step history reaches; 0 reads none, which makes locus step_never_started not_evaluable")
	client    = flag.String("client", "", "restrict to one client")
	process   = flag.String("process", "", "restrict to one process name")
	session   = flag.String("session", "", "classify one run and ignore -since")
	feedsOnly = flag.Bool("feeds-only", false, "classify only the sessions carrying a feed anomaly detected in the window, and no run; for after a feed_detect backfill")
	limit     = flag.Int("limit", 200, "at most this many runs")
	dryRun    = flag.Bool("dry-run", false, "classify, rank and report; write nothing")
	attest    = flag.Bool("attest", false, "read the incident record back and report whether anything acted; classify nothing")

	actor        = flag.String("actor", "", "the agent identity recorded on every transition; defaults to the classifier ref")
	severity     = flag.String("severity", shadow.SeverityInfo, "the severity written on every incident. No locus determines one and shadow mode's default is `info`, argued from the posture: nothing acts on these rows")
	modelVersion = flag.String("model-version", shadow.ModelVersion, "the domain model version written on every incident")
	generation   = flag.Int("generation", 1, "classifier and ranker generation")

	minRatio     = flag.Float64("min-ratio", 0.5, "locus rows_lost_silently fires below this output/input ratio (unsourced)")
	minInput     = flag.Int64("min-input", 1000, "locus rows_lost_silently ignores workers that read fewer rows than this, and runs of a step that read fewer in all (unsourced)")
	minPriorRuns = flag.Int64("min-prior-runs", 3, "locus step_never_started needs at least this many prior runs in the baseline (unsourced)")

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

// locusCensus is one locus's verdict counts over the runs classified.
type locusCensus struct {
	Present      int `json:"present"`
	Absent       int `json:"absent"`
	NotEvaluable int `json:"notEvaluable"`
}

// summary is what this run recorded as its row's summary. Grouped counts the
// feed findings joined to their condition's incident; its key keeps the name it
// had when a condition was a day's burst, for the scripts that read it.
type summary struct {
	DryRun          bool                   `json:"dryRun"`
	AttestOnly      bool                   `json:"attestOnly"`
	Since           time.Time              `json:"since"`
	Session         string                 `json:"session,omitempty"`
	FeedsOnly       bool                   `json:"feedsOnly,omitempty"`
	Regime          string                 `json:"purgeRegime"`
	ChannelDetails  bool                   `json:"channelDetails"`
	Runs            int                    `json:"runsClassified"`
	FeedSessions    int                    `json:"feedSessionsSelected"`
	Incidents       int                    `json:"incidentsWritten"`
	Hypotheses      int                    `json:"hypothesesWritten"`
	AlreadyRaised   int                    `json:"alreadyRaised"`
	Grouped         int                    `json:"groupedIntoABurst"`
	UnmappedLoci    []string               `json:"unmappedLoci"`
	Census          map[string]locusCensus `json:"census"`
	FeedsAssessed   int                    `json:"feedsAssessedForProposals"`
	Proposals       int                    `json:"proposalsDrafted"`
	ProposalsKnown  int                    `json:"proposalsAlreadyRecorded"`
	AttestationRead bool                   `json:"attestationRead"`
	AttestationHeld bool                   `json:"attestationHeld"`
	Attestation     string                 `json:"attestation"`
}

func doAttest(ctx context.Context, db *pgxpool.Pool, sum *summary) error {
	a, err := shadow.Attest(ctx, db)
	if err != nil {
		return err
	}
	sum.AttestationRead = true
	sum.AttestationHeld = a.Holds()
	sum.Attestation = a.Report()
	fmt.Print(a.Report())
	if !a.Holds() {
		// A non-zero exit, so that a deployment can run this as a check rather
		// than as a report somebody reads.
		return fmt.Errorf("the record does not show that nothing acted")
	}
	return nil
}

func doJob(ctx context.Context, db *pgxpool.Pool, extent *triage.Extent, sum *summary) error {
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
		// The delivery loci hang on registration sessions, which may have no
		// run, so the sessions carrying a feed anomaly are selected as well,
		// or alone with -feeds-only (shadow.SelectSessions).
		from := time.Now().UTC().Add(-*since)
		sel, err := shadow.SelectSessions(ctx, db, from, *client, *process, *limit, *feedsOnly, extent.Anomalies)
		if err != nil {
			return err
		}
		if sel.FeedsSkipped != "" {
			log.Print(sel.FeedsSkipped)
		}
		if *feedsOnly {
			log.Printf("-feeds-only: %d session(s) carrying a feed anomaly detected since %s, and no run",
				len(sel.Feeds), from.Format(time.RFC3339))
		}
		sum.FeedSessions = len(sel.Feeds)
		sessions = sel.Sessions
	}
	sum.Runs = len(sessions)
	if len(sessions) == 0 {
		log.Printf("no run started and no feed anomaly was detected in the last %s", since.String())
		return nil
	}
	log.Printf("classifying %d run(s); severity is %q for every incident, which is a posture and not a "+
		"measurement", len(sessions), *severity)

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
		sum.Grouped += len(res.Grouped)
		for _, wr := range res.Written {
			incidents++
			hypotheses += wr.Hypotheses
			if wr.SessionId != "" && wr.SessionId != s {
				log.Printf("%s %s locus %s -> %s, %d hypothesis(es), audit chain seq %d: the incident of the feed "+
					"condition %s belongs to, raised on the condition's first delivery, session %s",
					s, wr.IncidentId, wr.Locus, wr.Status, wr.Hypotheses, wr.ChainSeq, s, wr.SessionId)
				continue
			}
			log.Printf("%s %s locus %s -> %s, %d hypothesis(es), audit chain seq %d",
				s, wr.IncidentId, wr.Locus, wr.Status, wr.Hypotheses, wr.ChainSeq)
		}
		for _, g := range res.Grouped {
			log.Printf("%s locus %s: a later delivery of a feed condition, joined to the condition's incident "+
				"rather than raised as another", s, g)
		}
		sum.UnmappedLoci = append(sum.UnmappedLoci, res.UnmappedLoci...)
		if len(res.UnmappedLoci) > 0 {
			log.Printf("%s: %v fired and the cause mapping maps them to no cause class, so those incidents carry no hypothesis",
				s, res.UnmappedLoci)
		}
	}

	// The advisory step: a feed whose deliveries look cumulative under an
	// incremental declaration gets a change proposal in draft, triggered by
	// its incident. It moves no incident and leaves the proposal in draft.
	drafts, err := w.DraftFeedProposals(ctx, db, sessions)
	if err != nil {
		return err
	}
	sum.FeedsAssessed = len(drafts.Assessed)
	for _, a := range drafts.Assessed {
		if !a.Qualifies {
			log.Printf("feed %s: no proposal (%s; %d of its latest deliveries look cumulative)",
				a.TableName, a.Reason, len(a.Streak))
		}
	}
	for _, d := range drafts.Drafts {
		switch {
		case d.Written:
			sum.Proposals++
			log.Printf("feed %s: drafted change proposal %s, triggered by %s; it stays in draft until a person moves it",
				d.Assessment.TableName, d.Proposal.ProposalId, d.Proposal.TriggerRef)
		case d.Existing != "":
			sum.ProposalsKnown++
			log.Printf("feed %s: change proposal %s is already recorded, in %s; not drafted again",
				d.Assessment.TableName, d.Proposal.ProposalId, d.Existing)
		default:
			log.Printf("feed %s: change proposal %s would be drafted (-dry-run)",
				d.Assessment.TableName, d.Proposal.ProposalId)
		}
	}

	// The per-locus census, printed whatever fired. A classifier that reported
	// only its hits would supply numerators and no denominators, and so would a
	// process that printed only what it wrote.
	log.Printf("%-38s %8s %8s %14s", "locus", "present", "absent", "not_evaluable")
	sum.Incidents, sum.Hypotheses, sum.AlreadyRaised = incidents, hypotheses, already
	for _, l := range triage.Loci {
		sum.Census[l] = locusCensus{
			Present:      census[l+"/"+string(triage.Present)],
			Absent:       census[l+"/"+string(triage.Absent)],
			NotEvaluable: census[l+"/"+string(triage.NotEvaluable)],
		}
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
			"were already raised and were left alone", incidents, len(sessions), hypotheses, already)
	}

	// Say what the record now shows, in the same run that wrote it. Criterion 47
	// asks for the audit record to show that nothing acted, and the cheapest way
	// for that to be true of an operator's console is for the writer to print it.
	return doAttest(ctx, db, sum)
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

	rec := detectrun.Begin(ctx, db, detectrun.CommandShadowTriage, detectrun.Window(*since))
	sum := &summary{DryRun: *dryRun, AttestOnly: *attest, Session: *session, FeedsOnly: *feedsOnly,
		Census: map[string]locusCensus{}}
	err = run(ctx, db, sum)
	rec.Finish(ctx, sum, err)
	if err != nil {
		log.Fatal(err)
	}
}

func run(ctx context.Context, db *pgxpool.Pool, sum *summary) error {
	if *attest {
		return doAttest(ctx, db, sum)
	}
	if *feedsOnly && *session != "" {
		return fmt.Errorf("-feeds-only selects sessions by -since, and -session names one; use one or the other")
	}
	sum.Since = time.Now().UTC().Add(-*since)

	extent, err := triage.ReadExtent(ctx, db)
	if err != nil {
		return err
	}
	if !extent.ExecutionRecord {
		return fmt.Errorf("jetsapi.pipeline_execution_status and jetsapi.pipeline_execution_details are not " +
			"both present in this database, so there is nothing to classify. They are created by " +
			"`update_db -migrateDb`; run that first.")
	}
	sum.Regime = extent.Regime()
	sum.ChannelDetails = extent.ChannelDetails
	log.Printf("record: %d run headers, %d of them archived with their deleted sessions, oldest %s; "+
		"oldest worker row %s; purge regime %q",
		extent.Headers, extent.Archived, stamp(extent.OldestHeader), stamp(extent.OldestWorker), extent.Regime())
	if !extent.ChannelDetails {
		log.Printf("record: jetsapi.pipeline_execution_channel_details is absent, so locus " +
			"sink_failed_under_completed_worker is not_evaluable on every run here")
	}
	return doJob(ctx, db, extent, sum)
}

func stamp(t time.Time) string {
	if t.IsZero() {
		return "(none)"
	}
	return t.Format(time.RFC3339)
}
