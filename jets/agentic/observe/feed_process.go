package observe

// The feed process: what runs the eight feed detectors over a deployment's
// feeds, as the feed_detect command does.
//
// # Two modes that judge, and one that exports
//
// A window judges the deliveries registered in [Since, AsOf], and judges
// feed_overdue at AsOf. That is the daily run.
//
// A backfill judges every delivery registered in [Since, AsOf] as a daily run
// would have judged it on the day: feed_overdue is judged at each scheduled
// step, every Step back from AsOf to Since, against the deliveries registered
// by then, and every other finding is dated at the first step at or after its
// delivery's arrival. The other seven detectors judge a delivery against the
// deliveries registered before it and nothing after (feed_delivery.go,
// feed_claimset.go), so judging them once, at AsOf, says what they would have
// said on the day; only the date they would have said it needs the steps.
//
// The export, and judging what the export returns, are feed_process_export.go.
// Both run this same code against a database that answers from the results a
// person pasted back, so they measure these detectors and not a copy of them.
//
// # What a delivery is
//
// A file at its first registration, as both detector families read it
// (feed_delivery_registrations.go). The run reads every feed's registrations
// once and gives the same reading to both families, so the export prints that
// statement once. A later registration of a delivered file is re-processing:
// no detector judges it, and every detector's coverage counts it, per feed, as
// re-processed files and re-registrations whose own arrival is in the window.
//
// # What the run records
//
// Every reason a feed or a delivery was not judged, and every verdict that
// carries history_truncated, is counted per detector in FeedProcessCoverage,
// which the command writes to its log and to its run row. A not-judged
// delivery is not a clean one, and a run whose record said only what fired
// would make the two look alike.
//
// # A feed whose statement fails
//
// The content checks read each feed's claim-set table with a statement of its
// own. When one fails, that feed's content verdicts are lost and the other
// feeds' are kept: FeedProcessResult.FeedErrors names the feed and the error,
// and the run is reported failed after the other feeds' findings are written.
// An error is not a declaration. A feed the profile does not let a check judge
// is a choice someone made and is recorded as not judged; a feed whose
// statement errs is a fault in the table or the profile that someone has to
// fix, and a run recorded as ok would hide it among the declared ones.
//
// The error is recorded by its SQLSTATE alone. PostgreSQL's message for a
// value that cannot be read as the type a profile implies -- a member column
// declared as the date of service -- quotes the value, and the value is a
// claim's.

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/artisoft-io/jetstore/jets/agentic/feedprofile"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

// FeedProcessOptions bound one run of the feed process.
type FeedProcessOptions struct {
	// Since and AsOf bound the arrivals judged, both inclusive. AsOf is also
	// when feed_overdue is judged and, in a window, every anomaly's
	// detected_at. A zero Since judges every delivery registered by AsOf.
	Since, AsOf time.Time
	// Step is the interval between the scheduled runs a backfill replays,
	// counted back from AsOf. Zero is a window: one run, at AsOf.
	Step time.Duration
	// MinMembers is the content checks' small-delivery threshold
	// (ClaimSetOptions.MinMembers).
	MinMembers int
	// Generation is every detector's generation.
	Generation int
}

// DefaultFeedProcessOptions is a window ending now: generation 1 and the
// content checks' default minimum membership.
func DefaultFeedProcessOptions(asOf time.Time) FeedProcessOptions {
	return FeedProcessOptions{AsOf: asOf.UTC(), MinMembers: claimSetDefaultMinMembers, Generation: 1}
}

// Steps returns the scheduled instants the run judges at, oldest first: AsOf
// alone for a window, and for a backfill every Step back from AsOf that is not
// before Since.
func (o FeedProcessOptions) Steps() []time.Time {
	if o.Step <= 0 || o.Since.IsZero() {
		return []time.Time{o.AsOf}
	}
	var out []time.Time
	for t := o.AsOf; !t.Before(o.Since); t = t.Add(-o.Step) {
		out = append(out, t)
	}
	slices.Reverse(out)
	return out
}

// detectedAt is the first step at or after an arrival: the run that would
// first have seen the delivery.
func (o FeedProcessOptions) detectedAt(steps []time.Time, arrival time.Time) time.Time {
	i := sort.Search(len(steps), func(i int) bool { return !steps[i].Before(arrival) })
	if i == len(steps) {
		return o.AsOf
	}
	return steps[i]
}

// inWindow reports whether an arrival is judged by the run.
func (o FeedProcessOptions) inWindow(arrival time.Time) bool {
	return !arrival.After(o.AsOf) && (o.Since.IsZero() || !arrival.Before(o.Since))
}

// FeedProcessFeeds are the feeds a run judges, as the profiles were read.
type FeedProcessFeeds struct {
	Clients    []string
	Profiled   []feedprofile.Feed
	Unprofiled []feedprofile.Feed
	// Unmatched are the table names a profile file declares that no feed
	// carries. A misspelt name would otherwise leave its feed unjudged
	// without a word.
	Unmatched []string
}

const feedProcessClientsSQL = `SELECT DISTINCT client FROM jetsapi.source_config ORDER BY client`

// ReadFeedProcessFeeds reads the feeds of the clients named, or of every client
// with a source configuration when none is. With profiles, the profiles come
// from that map, as feedprofile.LoadFile reads a file, and replace any stored
// one; without, they are the stored ones.
func ReadFeedProcessFeeds(ctx context.Context, db DB, clients []string,
	profiles map[string]feedprofile.Profile) (FeedProcessFeeds, error) {
	var out FeedProcessFeeds
	if len(clients) == 0 {
		rows, err := db.Query(ctx, feedProcessClientsSQL)
		if err != nil {
			return out, fmt.Errorf("while listing the clients in source_config: %w", err)
		}
		for rows.Next() {
			var c string
			if err := rows.Scan(&c); err != nil {
				rows.Close()
				return out, fmt.Errorf("while listing the clients in source_config: %w", err)
			}
			clients = append(clients, c)
		}
		rows.Close()
		if err := rows.Err(); err != nil {
			return out, fmt.Errorf("while listing the clients in source_config: %w", err)
		}
	}
	out.Clients = clients
	var all []feedprofile.Feed
	for _, c := range clients {
		p, u, err := feedprofile.Load[pgx.Rows](ctx, db, c)
		if err != nil {
			return out, err
		}
		all = append(append(all, p...), u...)
	}
	if profiles == nil {
		for _, f := range all {
			if f.Profile.DeclaredBy != "" && len(f.Findings) == 0 {
				out.Profiled = append(out.Profiled, f)
			} else {
				out.Unprofiled = append(out.Unprofiled, f)
			}
		}
		return out, nil
	}
	out.Profiled, out.Unprofiled, out.Unmatched = feedprofile.Attach(all, profiles)
	return out, nil
}

// FeedProcessNotJudgedFeed is a feed a detector does not judge, and why.
type FeedProcessNotJudgedFeed struct {
	TableName string `json:"tableName"`
	Reason    string `json:"reason"`
}

// FeedProcessCoverage is one detector's account of a run: which feeds it
// judged, which it did not and why, and how its deliveries in the window were
// judged.
type FeedProcessCoverage struct {
	Detector       string                     `json:"detector"`
	FeedsJudged    int                        `json:"feedsJudged"`
	FeedsNotJudged []FeedProcessNotJudgedFeed `json:"feedsNotJudged,omitempty"`
	// Verdicts counts the verdicts in the window: per delivery, or for
	// feed_overdue per feed at each step it was judged.
	Verdicts int `json:"verdicts"`
	Fires    int `json:"fires"`
	Silent   int `json:"silent"`
	// NotJudged counts the not-judged verdicts by reason.
	NotJudged map[string]int `json:"notJudged,omitempty"`
	// HistoryTruncated counts, by feed, the verdicts that carry
	// history_truncated: the purge may have removed what they needed.
	HistoryTruncated map[string]int `json:"historyTruncated,omitempty"`
	// Notes are what the delivery-grain detectors could not attribute
	// cleanly: registrations under another client, session ids that are not a
	// time.
	Notes []string `json:"notes,omitempty"`
	// Reprocessed counts, by feed the detector judges, the re-registrations
	// in the window of files already delivered, and the files: re-processing,
	// which no detector judges.
	Reprocessed map[string]FeedReprocessed `json:"reprocessed,omitempty"`
}

// Line renders the coverage as one line of a run's log.
func (c FeedProcessCoverage) Line() string {
	var b strings.Builder
	fmt.Fprintf(&b, "coverage %s: %d feed(s) judged; %d verdict(s): %d fire, %d silent",
		c.Detector, c.FeedsJudged, c.Verdicts, c.Fires, c.Silent)
	if len(c.NotJudged) > 0 {
		parts := make([]string, 0, len(c.NotJudged))
		for _, r := range feedDeliverySortedKeys(c.NotJudged) {
			parts = append(parts, fmt.Sprintf("%s %d", r, c.NotJudged[r]))
		}
		fmt.Fprintf(&b, ", not judged: %s", strings.Join(parts, ", "))
	}
	if len(c.FeedsNotJudged) > 0 {
		parts := make([]string, len(c.FeedsNotJudged))
		for i, n := range c.FeedsNotJudged {
			parts[i] = fmt.Sprintf("%s (%s)", n.TableName, n.Reason)
		}
		fmt.Fprintf(&b, "; feeds not judged: %s", strings.Join(parts, ", "))
	}
	if len(c.HistoryTruncated) > 0 {
		parts := make([]string, 0, len(c.HistoryTruncated))
		for _, t := range feedDeliverySortedKeys(c.HistoryTruncated) {
			parts = append(parts, fmt.Sprintf("%s %d", t, c.HistoryTruncated[t]))
		}
		fmt.Fprintf(&b, "; %s: %s", ConfounderHistoryTruncated, strings.Join(parts, ", "))
	}
	for _, n := range c.Notes {
		fmt.Fprintf(&b, "; %s", n)
	}
	for _, t := range feedReprocessedTables(c.Reprocessed) {
		fmt.Fprintf(&b, "; %s", feedReprocessedLine(t, c.Reprocessed[t]))
	}
	return b.String()
}

// reprocessed counts a judged feed's re-processing in the run's window.
func (c *FeedProcessCoverage) reprocessed(regs *FeedRegistrations, table string, o FeedProcessOptions) {
	n := FeedReprocessedIn(regs.Deliveries(table, o.AsOf), o.Since, o.AsOf)
	if n.Registrations == 0 {
		return
	}
	if c.Reprocessed == nil {
		c.Reprocessed = map[string]FeedReprocessed{}
	}
	c.Reprocessed[table] = n
}

func (c *FeedProcessCoverage) count(table, verdict, reason string, confounders []string) {
	c.Verdicts++
	switch verdict {
	case FeedDeliveryFires:
		c.Fires++
	case FeedDeliverySilent:
		c.Silent++
	default:
		if c.NotJudged == nil {
			c.NotJudged = map[string]int{}
		}
		c.NotJudged[reason]++
	}
	if slices.Contains(confounders, ConfounderHistoryTruncated) {
		if c.HistoryTruncated == nil {
			c.HistoryTruncated = map[string]int{}
		}
		c.HistoryTruncated[table]++
	}
}

// FeedProcessFeedError is a feed whose content checks failed.
type FeedProcessFeedError struct {
	TableName string `json:"tableName"`
	Error     string `json:"error"`
}

// FeedProcessResult is what a run found.
type FeedProcessResult struct {
	Options FeedProcessOptions
	Steps   []time.Time
	Feeds   FeedProcessFeeds
	// Delivery holds the delivery-grain detectors' verdicts in the window:
	// size and back-dating per delivery, overdue per feed at each step.
	Delivery []FeedDeliveryVerdict
	// DeliveryDetector names the detector of each of Delivery's verdicts.
	DeliveryDetector []string
	// ClaimSets holds the content checks' verdicts in the window.
	ClaimSets []ClaimSetVerdict
	// Summaries are the claim-set summaries the run computed for deliveries
	// that had none, which WriteFeedProcess writes before the anomalies.
	Summaries []ClaimSetSummary
	// Anomalies are every finding, once each, dated at the step that would
	// first have reported it.
	Anomalies  []Anomaly
	Coverage   []FeedProcessCoverage
	FeedErrors []FeedProcessFeedError
}

// Err is the run's failure, if any feed's content checks failed.
func (r FeedProcessResult) Err() error {
	if len(r.FeedErrors) == 0 {
		return nil
	}
	names := make([]string, len(r.FeedErrors))
	for i, e := range r.FeedErrors {
		names[i] = e.TableName
	}
	return fmt.Errorf("the content checks of %d feed(s) failed and judged nothing: %s; "+
		"every other feed was judged", len(r.FeedErrors), strings.Join(names, ", "))
}

// RunFeedProcess runs the eight feed detectors over the feeds and returns what
// they found. It writes nothing: WriteFeedProcess does.
func RunFeedProcess(ctx context.Context, db DB, feeds FeedProcessFeeds, o FeedProcessOptions) (FeedProcessResult, error) {
	if o.AsOf.IsZero() {
		return FeedProcessResult{}, errors.New("the feed process needs an as-of time")
	}
	o.AsOf = o.AsOf.UTC()
	if !o.Since.IsZero() {
		o.Since = o.Since.UTC()
	}
	if o.Generation <= 0 {
		o.Generation = 1
	}
	if o.MinMembers <= 0 {
		o.MinMembers = claimSetDefaultMinMembers
	}
	res := FeedProcessResult{Options: o, Steps: o.Steps(), Feeds: feeds}
	seen := map[string]bool{}
	keep := func(a Anomaly, at time.Time) {
		if seen[a.AnomalyId] {
			return
		}
		seen[a.AnomalyId] = true
		a.DetectedAt = at
		res.Anomalies = append(res.Anomalies, a)
	}

	// Every feed's registrations, read once, and the delivery-grain
	// detectors' input as of the last step.
	all := append(append([]feedprofile.Feed(nil), feeds.Profiled...), feeds.Unprofiled...)
	tables := make([]string, len(all))
	for i, f := range all {
		tables[i] = f.TableName
	}
	regs, err := ReadFeedRegistrations(ctx, db, tables)
	if err != nil {
		return res, err
	}
	in := FeedDeliveryInputOf(regs, feeds.Profiled, feeds.Unprofiled, o.AsOf)
	arrivals := map[string]time.Time{}
	for _, ds := range in.Deliveries {
		for _, d := range ds {
			arrivals[d.SessionId] = d.Arrival
		}
	}
	overdue := FeedOverdue{Generation: o.Generation}
	size := DefaultDeliverySize()
	size.Generation = o.Generation
	backdated := DeliveryBackdated{Generation: o.Generation}

	// feed_overdue, at every step, against what was registered by then.
	cov := FeedProcessCoverage{Detector: feedprofile.FeedOverdue}
	for i, step := range res.Steps {
		r, err := overdue.Detect(feedProcessAsOf(in, step))
		if err != nil {
			return res, err
		}
		for _, v := range r.Verdicts {
			res.Delivery = append(res.Delivery, v)
			res.DeliveryDetector = append(res.DeliveryDetector, r.Detector)
			cov.count(v.TableName, v.Verdict, v.Reason, v.Confounders)
		}
		for _, a := range r.Anomalies {
			keep(a, step)
		}
		if i == len(res.Steps)-1 {
			feedProcessCoverFeeds(&cov, r.Coverage, regs, o)
		}
	}
	res.Coverage = append(res.Coverage, cov)

	// delivery_size and delivery_backdated, once, each delivery against its
	// predecessors.
	for _, d := range []interface {
		Detect(*FeedDeliveryInput) (FeedDeliveryResult, error)
	}{size, backdated} {
		r, err := d.Detect(in)
		if err != nil {
			return res, err
		}
		cov := FeedProcessCoverage{Detector: r.Detector}
		feedProcessCoverFeeds(&cov, r.Coverage, regs, o)
		fired := map[string]bool{}
		for _, v := range r.Verdicts {
			at, ok := arrivals[v.SessionId]
			if !ok || !o.inWindow(at) {
				continue
			}
			res.Delivery = append(res.Delivery, v)
			res.DeliveryDetector = append(res.DeliveryDetector, r.Detector)
			cov.count(v.TableName, v.Verdict, v.Reason, v.Confounders)
			if v.AnomalyId != "" {
				fired[v.AnomalyId] = true
			}
		}
		for _, a := range r.Anomalies {
			if fired[a.AnomalyId] {
				keep(a, o.detectedAt(res.Steps, arrivals[a.SessionId]))
			}
		}
		res.Coverage = append(res.Coverage, cov)
	}

	// The five content checks, once, each delivery against its predecessor.
	co := ClaimSetOptions{Since: o.Since, Until: o.AsOf.Add(time.Millisecond), Now: o.AsOf,
		MinMembers: o.MinMembers, Generation: o.Generation}
	verdicts, sums, err := detectClaimSetsFrom(ctx, db, all, regs, co)
	if err != nil {
		res.FeedErrors = feedProcessFeedErrors(err)
	}
	res.ClaimSets = verdicts
	res.Summaries = sums
	byDetector := map[string]*FeedProcessCoverage{}
	for _, det := range ClaimSetDetectors {
		c := &FeedProcessCoverage{Detector: det}
		for _, f := range all {
			if gate := claimSetFieldGate(det, f); gate != "" {
				c.FeedsNotJudged = append(c.FeedsNotJudged, FeedProcessNotJudgedFeed{TableName: f.TableName,
					Reason: feedProcessFeedReason(gate, f)})
			} else {
				c.FeedsJudged++
				c.reprocessed(regs, f.TableName, o)
			}
		}
		sort.SliceStable(c.FeedsNotJudged, func(i, j int) bool {
			return c.FeedsNotJudged[i].TableName < c.FeedsNotJudged[j].TableName
		})
		byDetector[det] = c
	}
	for _, v := range verdicts {
		byDetector[v.Detector].count(v.TableName, v.Verdict, v.Reason, v.Confounders)
		for _, a := range v.Anomalies {
			at := o.AsOf
			if ms, err := strconv.ParseInt(v.SessionID, 10, 64); err == nil && ms > 0 {
				at = o.detectedAt(res.Steps, time.UnixMilli(ms).UTC())
			}
			keep(a, at)
		}
	}
	for _, det := range ClaimSetDetectors {
		res.Coverage = append(res.Coverage, *byDetector[det])
	}
	return res, nil
}

// feedProcessAsOf is the delivery input as a run at step would have read it:
// the deliveries registered by then, which are a prefix of each feed's list
// because registration order is arrival order.
func feedProcessAsOf(in *FeedDeliveryInput, step time.Time) *FeedDeliveryInput {
	out := *in
	out.AsOf = step
	out.Deliveries = make(map[string][]FeedDelivery, len(in.Deliveries))
	for t, ds := range in.Deliveries {
		n := sort.Search(len(ds), func(i int) bool { return ds[i].Arrival.After(step) })
		out.Deliveries[t] = ds[:n]
	}
	return &out
}

// feedProcessCoverFeeds copies a delivery-grain coverage line's feeds and
// notes, and counts the judged feeds' re-processing in the run's window.
func feedProcessCoverFeeds(c *FeedProcessCoverage, src FeedDeliveryCoverage, regs *FeedRegistrations,
	o FeedProcessOptions) {
	c.FeedsJudged = len(src.Judged)
	for _, t := range src.Judged {
		c.reprocessed(regs, t, o)
	}
	for _, n := range src.NotJudged {
		c.FeedsNotJudged = append(c.FeedsNotJudged, FeedProcessNotJudgedFeed{TableName: n.TableName, Reason: n.Reason})
	}
	for _, e := range src.Elsewhere {
		c.Notes = append(c.Notes, fmt.Sprintf("%s: %d deliveries registered under client %q org %q, not the feed's",
			e.TableName, e.Deliveries, e.Client, e.Org))
	}
	for _, t := range feedDeliverySortedKeys(src.Unordered) {
		c.Notes = append(c.Notes, fmt.Sprintf("%s: %d registrations whose session id is not a time", t, src.Unordered[t]))
	}
}

// feedProcessFeedReason is the reason a content check gives for a whole feed,
// told apart for a stored profile that was refused, as the delivery-grain
// detectors tell it.
func feedProcessFeedReason(gate string, f feedprofile.Feed) string {
	if gate == ClaimSetNoProfile && len(f.Findings) > 0 {
		return FeedDeliveryProfileRefused
	}
	return gate
}

// feedProcessFeedErrors splits DetectClaimSets' joined error into one entry per
// feed. Each part reads "feed <table_name>: <error>".
func feedProcessFeedErrors(err error) []FeedProcessFeedError {
	var parts []error
	if j, ok := err.(interface{ Unwrap() []error }); ok {
		parts = j.Unwrap()
	} else {
		parts = []error{err}
	}
	out := make([]FeedProcessFeedError, 0, len(parts))
	for _, p := range parts {
		msg := p.Error()
		table := ""
		if rest, ok := strings.CutPrefix(msg, "feed "); ok {
			if i := strings.Index(rest, ": "); i >= 0 {
				table, msg = rest[:i], rest[i+2:]
			}
		}
		var pg *pgconn.PgError
		if errors.As(p, &pg) {
			msg = fmt.Sprintf("a statement over the claim-set table failed with SQLSTATE %s; the database's "+
				"message is not recorded, because it can quote the value of a claim that could not be read, "+
				"and the value may be a member's", pg.Code)
		}
		out = append(out, FeedProcessFeedError{TableName: table, Error: msg})
	}
	return out
}

// FeedProcessWritten is what WriteFeedProcess wrote.
type FeedProcessWritten struct {
	Inserted, Existing int
	Summaries          int
}

// WriteFeedProcess writes the run's claim-set summaries, then its anomalies
// through InsertAnomalyIfNew, so a run over a window an earlier run covered
// writes no anomaly twice. The summaries come first: they are what a later run
// judges from once the rows are gone.
func WriteFeedProcess(ctx context.Context, db Exec, res FeedProcessResult) (FeedProcessWritten, error) {
	var w FeedProcessWritten
	n, err := WriteClaimSetSummaries(ctx, db, res.Summaries)
	w.Summaries = n
	if err != nil {
		return w, err
	}
	for i := range res.Anomalies {
		isNew, err := InsertAnomalyIfNew(ctx, db, &res.Anomalies[i])
		if err != nil {
			return w, err
		}
		if isNew {
			w.Inserted++
		} else {
			w.Existing++
		}
	}
	return w, nil
}

// Lines is the run's log: what it read, every detector's coverage line, the
// feeds whose checks failed, and one line per finding with its basis. Nothing
// in it is member-level: the bases are the detectors', which carry counts,
// lags and the declaration, never a claim's value.
func (r FeedProcessResult) Lines() []string {
	o := r.Options
	window := "every delivery registered by " + o.AsOf.Format(time.RFC3339)
	if !o.Since.IsZero() {
		window = fmt.Sprintf("deliveries registered from %s to %s", o.Since.Format(time.RFC3339),
			o.AsOf.Format(time.RFC3339))
	}
	mode := "window"
	if len(r.Steps) > 1 {
		mode = fmt.Sprintf("backfill, %d steps of %s", len(r.Steps), o.Step)
	}
	out := []string{
		fmt.Sprintf("feeds: %d client(s), %d profiled feed(s), %d without a usable profile",
			len(r.Feeds.Clients), len(r.Feeds.Profiled), len(r.Feeds.Unprofiled)),
		fmt.Sprintf("run: %s; %s; generation %d", mode, window, o.Generation),
	}
	out = append(out, fmt.Sprintf("claim-set summaries: %d computed for deliveries with none", len(r.Summaries)))
	if len(r.Feeds.Unmatched) > 0 {
		out = append(out, fmt.Sprintf("profiles: %d declared table name(s) match no feed: %s",
			len(r.Feeds.Unmatched), strings.Join(r.Feeds.Unmatched, ", ")))
	}
	for _, c := range r.Coverage {
		out = append(out, c.Line())
	}
	for _, e := range r.FeedErrors {
		out = append(out, fmt.Sprintf("failed: the content checks of feed %s judged nothing: %s", e.TableName, e.Error))
	}
	for _, a := range r.Anomalies {
		out = append(out, fmt.Sprintf("%s %s subject %s %s session %s detected %s observed %q",
			a.DetectorRef, a.SignalType, a.SubjectType, a.SubjectRef, a.SessionId,
			a.DetectedAt.Format(time.RFC3339), a.ObservedValue))
		out = append(out, "    basis: "+a.ExpectedBasis)
	}
	if len(r.Anomalies) == 0 {
		out = append(out, "no findings")
	}
	return out
}

// FeedProcessSummary is what a run records on its row of the detection-run
// table.
type FeedProcessSummary struct {
	DryRun     bool                   `json:"dryRun"`
	Mode       string                 `json:"mode"`
	Since      *time.Time             `json:"since,omitempty"`
	AsOf       time.Time              `json:"asOf"`
	Steps      int                    `json:"steps"`
	Clients    int                    `json:"clients"`
	Profiled   int                    `json:"profiledFeeds"`
	Unprofiled int                    `json:"unprofiledFeeds"`
	Unmatched  []string               `json:"unmatchedProfiles,omitempty"`
	Found      int                    `json:"anomaliesFound"`
	Written    int                    `json:"anomaliesWritten"`
	Already    int                    `json:"anomaliesAlreadyRecorded"`
	Summaries  int                    `json:"claimSetSummariesWritten"`
	ByDetector map[string]int         `json:"byDetector"`
	Coverage   []FeedProcessCoverage  `json:"coverage"`
	FeedErrors []FeedProcessFeedError `json:"feedErrors,omitempty"`
}

// Summary is the run row's summary of the result.
func (r FeedProcessResult) Summary(dryRun bool, w FeedProcessWritten) FeedProcessSummary {
	s := FeedProcessSummary{DryRun: dryRun, Mode: "window", AsOf: r.Options.AsOf, Steps: len(r.Steps),
		Clients: len(r.Feeds.Clients), Profiled: len(r.Feeds.Profiled), Unprofiled: len(r.Feeds.Unprofiled),
		Unmatched: r.Feeds.Unmatched, Found: len(r.Anomalies), Written: w.Inserted, Already: w.Existing, Summaries: w.Summaries,
		ByDetector: map[string]int{}, Coverage: r.Coverage, FeedErrors: r.FeedErrors}
	if r.Options.Step > 0 && !r.Options.Since.IsZero() {
		s.Mode = "backfill"
	}
	if !r.Options.Since.IsZero() {
		since := r.Options.Since
		s.Since = &since
	}
	for _, a := range r.Anomalies {
		s.ByDetector[a.DetectorRef]++
	}
	return s
}
