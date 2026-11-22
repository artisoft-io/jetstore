// feed_detect runs JetStore's eight feed detectors over a deployment's feeds
// and writes what they find to jetsapi.anomaly.
//
// The detectors judge each feed against the profile declared for it in its
// source configuration (jets/agentic/feedprofile): three read the feed's
// registrations -- overdue, size, back-dated -- and five read each delivery's
// claims in the table the profile names, aggregated inside the database --
// staleness, span, completeness, overlap, unchanged. A feed with no profile is
// reported as not judged, never as clean. Nothing here needs an inference
// endpoint, and nothing reads a member-level value out of the database.
//
// # Modes
//
//	feed_detect -since 26h                       judge what was registered in the window (the daily run)
//	feed_detect -mode backfill -from 2026-01-01  judge every delivery since, as each day's run would have
//	feed_detect -mode export -from 2026-01-01 -as-of 2026-06-01T05:00:00Z -results results.txt
//	feed_detect -mode judge  -from 2026-01-01 -as-of 2026-06-01T05:00:00Z -results results.txt
//
// A window judges the deliveries registered in the last -since, and judges
// whether each feed is overdue now. A backfill judges every delivery
// registered since -from, each against the deliveries registered before it
// and nothing after, and dates each finding at the first daily run (every
// -step back from -as-of) that would have seen it; it judges overdue at every
// such run. Both write their findings unless -dry-run is given, and a re-run
// over the same deliveries writes nothing twice.
//
// The export is for a database this command cannot reach, through a query
// tool that runs one statement at a time. It prints the statements the
// detectors would run, with the profile and the window written in as
// literals; each returns one row with one text column. The returned texts are
// appended to the -results file and the export is run again, which prints the
// statements that depend on them, until it says it is complete. The judge
// mode then runs the detectors over those results and prints their verdicts:
// the same detectors, judging the client's own aggregates. Neither mode opens
// a database connection or writes anything, and both need the same flags.
//
// Profiles come from the source configuration, or from -profiles, a JSON file
// keyed by table_name, for a database that does not carry them.
//
// The DSN comes from JETS_DSN_URI_VALUE, or from JETS_DSN_JSON_VALUE by the
// route the other detection commands take, or from -dsn.
//
// # Its run is recorded
//
// A window or backfill run writes one row to jetsapi.agentic_detection_run
// (jets/agentic/detectrun), with what it found and its coverage -- every
// reason a feed or a delivery was not judged, and every verdict carrying
// history_truncated -- as the row's summary. A feed whose content checks fail
// does not stop the others: they are judged and written, and the run is then
// recorded failed, naming the feed.
package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"log"
	"os"
	"strings"
	"time"

	"github.com/artisoft-io/jetstore/jets/agentic/detectrun"
	"github.com/artisoft-io/jetstore/jets/agentic/feedprofile"
	"github.com/artisoft-io/jetstore/jets/agentic/observe"
	"github.com/artisoft-io/jetstore/jets/awsi"
	"github.com/artisoft-io/jetstore/jets/utils"
	"github.com/jackc/pgx/v5/pgxpool"
)

// The modes.
const (
	modeWindow   = "window"
	modeBackfill = "backfill"
	modeExport   = "export"
	modeJudge    = "judge"
)

type config struct {
	mode       string
	dsn        string
	sshTunnel  bool
	since      time.Duration
	from       string
	asOf       string
	step       time.Duration
	clients    string
	profiles   string
	dryRun     bool
	minMembers int
	generation int
	results    string
	maxBytes   int
	jsonOut    string
}

func parseFlags(args []string) (config, error) {
	var c config
	fs := flag.NewFlagSet("feed_detect", flag.ContinueOnError)
	fs.StringVar(&c.mode, "mode", modeWindow, "window, backfill, export or judge")
	fs.StringVar(&c.dsn, "dsn", "", "database DSN; defaults to JETS_DSN_URI_VALUE, then JETS_DSN_JSON_VALUE")
	fs.BoolVar(&c.sshTunnel, "usingSshTunnel", false, "connect to the database through an open ssh tunnel")
	fs.DurationVar(&c.since, "since", 26*time.Hour, "window: judge the deliveries registered within this long before -as-of")
	fs.StringVar(&c.from, "from", "", "backfill, export, judge: judge the deliveries registered from this date (2006-01-02) or instant (RFC 3339)")
	fs.StringVar(&c.asOf, "as-of", "", "the detection time, RFC 3339; defaults to now. An export prints the one it used, and its later rounds and the judge need the same")
	fs.DurationVar(&c.step, "step", 24*time.Hour, "backfill, export, judge with -from: the interval between the daily runs replayed, counted back from -as-of")
	fs.StringVar(&c.clients, "client", "", "comma-separated clients; defaults to every client with a source configuration")
	fs.StringVar(&c.profiles, "profiles", "", "a JSON file of feed profiles keyed by table_name, replacing the stored ones")
	fs.BoolVar(&c.dryRun, "dry-run", false, "window, backfill: report what would be written and write nothing")
	fs.IntVar(&c.minMembers, "min-members", 11, "content checks: below this many distinct members a delivery's lag is recorded in months; unsourced")
	fs.IntVar(&c.generation, "generation", 1, "every detector's generation; bumping it re-emits over deliveries already judged")
	fs.StringVar(&c.results, "results", "", "export, judge: the file the returned texts are appended to")
	fs.IntVar(&c.maxBytes, "max-statement-bytes", 100000, "export: combine statements into parts of at most this size")
	fs.StringVar(&c.jsonOut, "json", "", "judge: also write the verdicts, findings and coverage to this file as JSON")
	if err := fs.Parse(args); err != nil {
		return c, err
	}
	switch c.mode {
	case modeWindow, modeBackfill, modeExport, modeJudge:
	default:
		return c, fmt.Errorf("-mode %q: want window, backfill, export or judge", c.mode)
	}
	if c.mode == modeBackfill && c.from == "" {
		return c, errors.New("a backfill needs -from")
	}
	if c.mode == modeJudge && c.results == "" {
		return c, errors.New("judge needs -results")
	}
	return c, nil
}

// options turns the flags into the process's options, now being the clock.
func (c config) options(now time.Time) (observe.FeedProcessOptions, error) {
	asOf := now.UTC().Truncate(time.Second)
	if c.asOf != "" {
		t, err := time.Parse(time.RFC3339, c.asOf)
		if err != nil {
			return observe.FeedProcessOptions{}, fmt.Errorf("-as-of %q: %w", c.asOf, err)
		}
		asOf = t.UTC()
	}
	o := observe.DefaultFeedProcessOptions(asOf)
	o.MinMembers, o.Generation = c.minMembers, c.generation
	if c.from == "" {
		if c.mode == modeWindow || c.mode == modeExport || c.mode == modeJudge {
			o.Since = asOf.Add(-c.since)
		}
		return o, nil
	}
	from, err := time.Parse(time.DateOnly, c.from)
	if err != nil {
		if from, err = time.Parse(time.RFC3339, c.from); err != nil {
			return o, fmt.Errorf("-from %q: want a date (2006-01-02) or an RFC 3339 instant", c.from)
		}
	}
	if c.mode == modeWindow {
		return o, errors.New("-from is for backfill, export and judge; a window takes -since")
	}
	o.Since, o.Step = from.UTC(), c.step
	return o, nil
}

func (c config) clientList() []string {
	var out []string
	for _, s := range strings.Split(c.clients, ",") {
		if s = strings.TrimSpace(s); s != "" {
			out = append(out, s)
		}
	}
	return out
}

func (c config) profileMap() (map[string]feedprofile.Profile, error) {
	if c.profiles == "" {
		return nil, nil
	}
	return feedprofile.LoadFile(c.profiles)
}

// window is what the run row records as its window.
func (c config) window(o observe.FeedProcessOptions) string {
	if o.Step > 0 && !o.Since.IsZero() {
		return "from " + o.Since.Format(time.RFC3339)
	}
	return detectrun.Window(c.since)
}

// hint is the command line that continues an export, with the as-of time the
// first round used, so every round and the judge read the same window.
func (c config) hint(o observe.FeedProcessOptions) string {
	parts := []string{"feed_detect", "-mode", modeExport, "-results", orDefault(c.results, "results.txt")}
	if c.from != "" {
		parts = append(parts, "-from", c.from, "-step", c.step.String())
	} else {
		parts = append(parts, "-since", c.since.String())
	}
	parts = append(parts, "-as-of", o.AsOf.Format(time.RFC3339))
	if c.clients != "" {
		parts = append(parts, "-client", c.clients)
	}
	if c.profiles != "" {
		parts = append(parts, "-profiles", c.profiles)
	}
	if c.minMembers != 11 {
		parts = append(parts, "-min-members", fmt.Sprint(c.minMembers))
	}
	if c.generation != 1 {
		parts = append(parts, "-generation", fmt.Sprint(c.generation))
	}
	return strings.Join(parts, " ")
}

func orDefault(s, d string) string {
	if s == "" {
		return d
	}
	return s
}

func readResults(path string) (observe.FeedProcessResults, error) {
	if path == "" {
		return observe.FeedProcessResults{}, nil
	}
	f, err := os.Open(path)
	if errors.Is(err, os.ErrNotExist) {
		return observe.FeedProcessResults{}, nil
	}
	if err != nil {
		return nil, err
	}
	defer f.Close()
	return observe.ReadFeedProcessResults(f)
}

// runExport prints the next round of the export to w.
func runExport(ctx context.Context, c config, now time.Time, w io.Writer) error {
	o, err := c.options(now)
	if err != nil {
		return err
	}
	profiles, err := c.profileMap()
	if err != nil {
		return err
	}
	results, err := readResults(c.results)
	if err != nil {
		return err
	}
	exp, err := observe.ExportFeedProcess(ctx, results, c.clientList(), profiles, o)
	if err != nil {
		return err
	}
	return exp.Write(w, c.maxBytes, c.hint(o))
}

// judged is what -json writes.
type judged struct {
	Summary   observe.FeedProcessSummary `json:"summary"`
	Verdicts  []string                   `json:"verdicts"`
	Anomalies []observe.Anomaly          `json:"anomalies"`
}

// runJudge runs the detectors over the results and prints what they find to w.
func runJudge(ctx context.Context, c config, now time.Time, w io.Writer) error {
	o, err := c.options(now)
	if err != nil {
		return err
	}
	profiles, err := c.profileMap()
	if err != nil {
		return err
	}
	results, err := readResults(c.results)
	if err != nil {
		return err
	}
	res, err := observe.JudgeFeedProcess(ctx, results, c.clientList(), profiles, o)
	if err != nil {
		return err
	}
	for _, l := range res.Lines() {
		fmt.Fprintln(w, l)
	}
	verdicts := observe.FeedProcessVerdictLines(res)
	fmt.Fprintln(w, "detector\tfeed\tdelivery\tsession\tverdict\treason")
	for _, l := range verdicts {
		fmt.Fprintln(w, l)
	}
	if c.jsonOut != "" {
		doc, err := json.MarshalIndent(judged{Summary: res.Summary(true, observe.FeedProcessWritten{}),
			Verdicts: verdicts, Anomalies: res.Anomalies}, "", "  ")
		if err != nil {
			return err
		}
		if err := os.WriteFile(c.jsonOut, doc, 0o644); err != nil {
			return err
		}
	}
	return res.Err()
}

// detect is a window or a backfill against the database: it reads, judges,
// logs, writes unless dry, and fills the summary for the run row.
func detect(ctx context.Context, db *pgxpool.Pool, c config, o observe.FeedProcessOptions,
	sum *observe.FeedProcessSummary) error {
	*sum = observe.FeedProcessSummary{DryRun: c.dryRun, Mode: c.mode, AsOf: o.AsOf, ByDetector: map[string]int{}}
	if err := prerequisites(ctx, db, c.dryRun); err != nil {
		return err
	}
	profiles, err := c.profileMap()
	if err != nil {
		return err
	}
	feeds, err := observe.ReadFeedProcessFeeds(ctx, db, c.clientList(), profiles)
	if err != nil {
		return err
	}
	res, err := observe.RunFeedProcess(ctx, db, feeds, o)
	if err != nil {
		return err
	}
	for _, l := range res.Lines() {
		log.Print(l)
	}
	var w observe.FeedProcessWritten
	if !c.dryRun {
		if w, err = observe.WriteFeedProcess(ctx, db, res); err != nil {
			return err
		}
	}
	*sum = res.Summary(c.dryRun, w)
	switch {
	case c.dryRun:
		log.Printf("%d finding(s), none written (-dry-run)", len(res.Anomalies))
	default:
		log.Printf("%d finding(s): %d written, %d already recorded by this detector generation",
			len(res.Anomalies), w.Inserted, w.Existing)
	}
	return res.Err()
}

// prerequisites refuses to run where there is nothing to read or nowhere to
// write, naming the migration, rather than failing on a missing relation.
func prerequisites(ctx context.Context, db *pgxpool.Pool, dryRun bool) error {
	var registry, anomaly bool
	err := db.QueryRow(ctx, `SELECT to_regclass('jetsapi.input_registry') IS NOT NULL,
		to_regclass('jetsapi.anomaly') IS NOT NULL`).Scan(&registry, &anomaly)
	if err != nil {
		return fmt.Errorf("while checking the database: %w", err)
	}
	if !registry {
		return errors.New("jetsapi.input_registry does not exist in this database, so there are no deliveries to read")
	}
	if !anomaly && !dryRun {
		return errors.New("jetsapi.anomaly does not exist in this database, so there is nowhere to write. " +
			"It is created by `update_db -migrateDb`; run that first, or run with -dry-run")
	}
	return nil
}

func resolveDsn(c config) (string, error) {
	if c.dsn != "" {
		return c.dsn, nil
	}
	if v := os.Getenv("JETS_DSN_URI_VALUE"); v != "" {
		return v, nil
	}
	dsn, err := awsi.GetDsnFromJson(os.Getenv("JETS_DSN_JSON_VALUE"), c.sshTunnel, 3)
	if err != nil {
		return "", fmt.Errorf("while reading the dsn from JETS_DSN_JSON_VALUE: %v", err)
	}
	if dsn == "" {
		return "", errors.New("no dsn: set -dsn, JETS_DSN_URI_VALUE or JETS_DSN_JSON_VALUE")
	}
	return dsn, nil
}

// runDetect opens the database, records the run, and judges.
func runDetect(ctx context.Context, c config, now time.Time) error {
	o, err := c.options(now)
	if err != nil {
		return err
	}
	dsn, err := resolveDsn(c)
	if err != nil {
		return err
	}
	db, err := pgxpool.New(ctx, dsn)
	if err != nil {
		return fmt.Errorf("while opening the database connection: %v", err)
	}
	defer db.Close()
	return recorded(ctx, db, c, o)
}

// recorded runs detect between the run row's Begin and Finish.
func recorded(ctx context.Context, db *pgxpool.Pool, c config, o observe.FeedProcessOptions) error {
	rec := detectrun.Begin(ctx, db, detectrun.CommandFeedDetect, c.window(o))
	var sum observe.FeedProcessSummary
	err := detect(ctx, db, c, o, &sum)
	rec.Finish(ctx, sum, err)
	return err
}

func main() {
	utils.UseJetStoreLogger()
	c, err := parseFlags(os.Args[1:])
	if err != nil {
		log.Fatal(err)
	}
	ctx := context.Background()
	now := time.Now()
	switch c.mode {
	case modeExport:
		err = runExport(ctx, c, now, os.Stdout)
	case modeJudge:
		err = runJudge(ctx, c, now, os.Stdout)
	default:
		err = runDetect(ctx, c, now)
	}
	if err != nil {
		log.Fatal(err)
	}
}
