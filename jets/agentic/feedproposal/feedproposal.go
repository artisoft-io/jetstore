// Package feedproposal drafts a change proposal for a feed whose deliveries
// look cumulative while its source configuration declares it incremental.
//
// # The case
//
// A feed declared incremental (source_config.is_cumulative = 0) is expected to
// carry claims not delivered before. The claim-set detectors compare each
// delivery with its predecessor and write two anomalies that bear on the
// declaration, both on subject feed and signal cardinality, told apart by
// their detector_ref:
//
//   - delivery_overlap, which on an incremental feed records as its observed
//     value the share of the delivery's distinct keys already in its
//     predecessor, and fires when the share is above the profile's
//     max_redelivered_share;
//   - delivery_unchanged, which fires when a delivery's key set equals its
//     predecessor's and its newest date of service did not advance: a share of
//     1 with nothing new.
//
// One such anomaly is a restated delivery, and a restatement is not a change
// of shape. The case is the shape persisting.
//
// # What "look cumulative" means, precisely
//
// A delivery is evidence when it carries a delivery_overlap anomaly whose
// observed share is at least Options.MinShare (default 0.5: most of the
// delivery had been delivered before), or a delivery_unchanged anomaly. The
// overlap's other observed value, "N keys absent", is what the detector writes
// on a feed declared cumulative whose delivery does not contain its
// predecessor; it is not a share and is never evidence.
//
// A feed looks cumulative when its latest Options.MinStreak deliveries
// (default 3), in registration order, are all evidence: a run of evidence that
// ends at the feed's latest delivery. A delivery is a file, as the detectors
// read it (observe.ReadFeedRegistrations): a distinct file_key at its first
// registration. A later registration of the same file is re-processing (a
// re-map, a backfill registering a file twice) and is not a delivery, so it
// neither ends the run nor splits it. So one restated delivery never
// triggers a draft, nor do restatements scattered through a history, nor a
// cumulative stretch the feed has since left: a delivery that is not evidence
// ends the run, and a delivery the detectors did not judge (its claims not
// loaded, say) is not evidence either. Only aggregates are read: the share is
// a ratio of counts, and no key, member or date leaves the database.
//
// # What it writes, and what it does not
//
// One row of jetsapi.change_proposal in state draft, per feed and case, with a
// deterministic id, so a second pass finds it and writes nothing. Its trigger
// is "incident" and its trigger_ref the incident at delivery_departs_profile
// on the first delivery of the run that has one: the onset, which is the
// vocabulary's own ordering principle. It proposes setting is_cumulative for
// the feed, and says in its rationale what that changes.
//
// It does not move the incident, which stays where shadow triage left it. It
// writes nothing to jetsapi.remediation. It does not change the source
// configuration, and it does not move the proposal past draft: that is a
// person's act, through the proposals screen and the lifecycle in
// jets/agentic/audit/lifecycle.go.
package feedproposal

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/artisoft-io/jetstore/jets/agentic/audit"
	"github.com/artisoft-io/jetstore/jets/agentic/observe"
	"github.com/artisoft-io/jetstore/jets/agentic/triage"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

// CaseCumulativeUnderIncremental names the one case this package drafts for.
// It is part of the proposal's identity, so a second case for the same feed
// would be a second proposal.
const CaseCumulativeUnderIncremental = "cumulative_under_incremental"

// Trigger is the change_proposal.trigger written on every draft: the proposal
// was prompted by an incident, and trigger_ref names it.
const Trigger = audit.TriggerIncident

// The two detectors whose anomalies are evidence, by the name in their
// detector_ref (which carries "@<generation>" after it).
var evidenceDetectors = []string{observe.ClaimSetDeliveryOverlap, observe.ClaimSetDeliveryUnchanged}

// DB is the slice of pgx this package needs, satisfied by *pgxpool.Pool and
// *pgx.Conn.
type DB interface {
	Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
	Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error)
}

// Options bound what counts as the case.
type Options struct {
	// MinStreak is how many of a feed's latest deliveries must all be
	// evidence. Below 2 a single restated delivery would draft a proposal, so
	// a value below 2 is refused.
	MinStreak int
	// MinShare is the least share of a delivery's distinct keys already in its
	// predecessor for a delivery_overlap anomaly to be evidence.
	MinShare float64
	// ModelVersion is written to proposal_model_version.
	ModelVersion string
	// DryRun assesses and reports, and writes nothing.
	DryRun bool
}

// Defaults are the starting point. Both thresholds are unsourced arguments:
// three deliveries is the smallest run that is not a restatement followed by
// one coincidence, and a half is the share above which a delivery is mostly
// what was delivered before.
const (
	DefaultMinStreak = 3
	DefaultMinShare  = 0.5
)

// DefaultOptions returns the defaults with the given model version.
func DefaultOptions(modelVersion string) Options {
	return Options{MinStreak: DefaultMinStreak, MinShare: DefaultMinShare, ModelVersion: modelVersion}
}

func (o *Options) check() error {
	if o.MinStreak < 2 {
		return fmt.Errorf("MinStreak is %d; below 2 one restated delivery would draft a proposal", o.MinStreak)
	}
	if o.MinShare <= 0 || o.MinShare > 1 {
		return fmt.Errorf("MinShare is %v; it is a share of keys, in (0, 1]", o.MinShare)
	}
	if o.ModelVersion == "" {
		return errors.New("ModelVersion is required: proposal_model_version is NOT NULL")
	}
	return nil
}

// Delivery is one registration of a feed, with what the two detectors said
// about it.
type Delivery struct {
	SessionID string
	// Ordinal is the delivery's position among the feed's registrations, from 1.
	Ordinal int
	// Share is the highest delivery_overlap share recorded on the delivery, or
	// nil when none was. A delivery_unchanged anomaly sets Unchanged and not
	// Share.
	Share     *float64
	Unchanged bool
	// AnomalyIDs are the evidence anomalies on this delivery.
	AnomalyIDs []string
	// IncidentID is the incident at delivery_departs_profile on this
	// delivery's session, or "" when none was written.
	IncidentID string
	// IncidentStatus is that incident's current status.
	IncidentStatus string
	// Reprocessed counts the later registrations of this delivery's file:
	// re-processing, which is not a delivery and carries no finding.
	Reprocessed int
}

// Evidence reports whether the delivery is evidence of a cumulative shape.
func (d *Delivery) Evidence(minShare float64) bool {
	return d.Unchanged || (d.Share != nil && *d.Share >= minShare)
}

// Assessment is the verdict on one feed.
type Assessment struct {
	TableName  string
	Client     string
	Org        string
	ObjectType string
	// Cumulative is the feed's declaration: is_cumulative != 0.
	Cumulative bool
	// Deliveries are the feed's deliveries, in registration order: each file
	// at its first registration.
	Deliveries []Delivery
	// Reregistrations counts the later registrations of the feed's files,
	// which are re-processing and not deliveries, and Unordered its
	// registrations whose session id is not a time, which are neither.
	Reregistrations int
	Unordered       int
	// Streak is the run of evidence ending at the latest delivery, oldest first.
	Streak []Delivery
	// SharedObjectType counts the other source configurations of the same
	// client and object type, and Undeclared how many of them are not declared
	// cumulative. The cumulative read is per client and object type, so a
	// change to one feed alone leaves the object type mixed.
	SharedObjectType int
	Undeclared       int
	// Qualifies is the verdict; Reason says why not when it is false.
	Qualifies bool
	Reason    string
	// TriggerIncident is the incident the draft names, when Qualifies.
	TriggerIncident string
}

// Why a feed does not qualify.
const (
	ReasonDeclaredCumulative = "declared_cumulative"
	ReasonNoConfiguration    = "no_source_configuration"
	ReasonShortStreak        = "evidence_run_too_short"
	ReasonNoIncident         = "no_incident_on_the_evidence"
)

// Draft is what one qualifying feed produced.
type Draft struct {
	Assessment *Assessment
	Proposal   *audit.Proposal
	// Written is true when this call inserted the row; false when the row was
	// already there (Existing) or the call was a dry run.
	Written bool
	// Existing is the state the already-recorded proposal is in, or "".
	Existing string
}

// Result is what one call concluded and wrote.
type Result struct {
	// Assessed are the feeds that carry at least one evidence anomaly, each
	// with its verdict: the denominator beside the drafts.
	Assessed []Assessment
	Drafts   []Draft
}

// Written counts the drafts this call inserted.
func (r *Result) Written() int {
	n := 0
	for _, d := range r.Drafts {
		if d.Written {
			n++
		}
	}
	return n
}

// ProposalIDFor is the deterministic key of one (feed, case): chg_ and the
// first 24 hex digits of a SHA-256 over the case and the feed's table name.
// It carries no name, and it matches the model's ^chg_[a-z0-9]+$.
func ProposalIDFor(caseName, tableName string) string {
	sum := sha256.Sum256([]byte(caseName + "\x1f" + tableName))
	return "chg_" + hex.EncodeToString(sum[:])[:24]
}

// candidatesSQL lists every feed that carries an anomaly from one of the two
// detectors, with its configuration. is_cumulative is read through to_jsonb,
// as feedprofile reads it, so a database without the column reads every feed
// as incremental.
const candidatesSQL = `SELECT a.table_name,
       sc.table_name IS NOT NULL,
       coalesce(sc.client, ''), coalesce(sc.org, ''), coalesce(sc.object_type, ''),
       coalesce((to_jsonb(sc) ->> 'is_cumulative')::int, 0)
  FROM (SELECT DISTINCT anomaly_subject_ref AS table_name
          FROM jetsapi.anomaly
         WHERE anomaly_subject_type = $1
           AND split_part(anomaly_detector_ref, '@', 1) = ANY($2::text[])) a
  LEFT JOIN jetsapi.source_config sc ON sc.table_name = a.table_name
 WHERE cardinality($3::text[]) = 0 OR a.table_name = ANY($3::text[])
 ORDER BY a.table_name`

// evidenceSQL reads the two detectors' anomalies on a feed: the session, the
// detector and the observed value, which for these two is a share or the word
// "equal", never a member-level value.
const evidenceSQL = `SELECT anomaly_id, anomaly_session_id, split_part(anomaly_detector_ref, '@', 1),
       anomaly_observed_value
  FROM jetsapi.anomaly
 WHERE anomaly_subject_type = $1 AND anomaly_subject_ref = $2
   AND split_part(anomaly_detector_ref, '@', 1) = ANY($3::text[])
 ORDER BY anomaly_id`

// incidentsSQL reads the incidents at the departs-profile locus on a feed's
// sessions.
const incidentsSQL = `SELECT incident_session_id, incident_id, status
  FROM jetsapi.incident
 WHERE incident_locus = $1 AND incident_session_id = ANY($2::text[])`

// sharedSQL counts the other configurations of the same client and object
// type, and how many of them are not declared cumulative.
const sharedSQL = `SELECT count(*),
       count(*) FILTER (WHERE coalesce((to_jsonb(sc) ->> 'is_cumulative')::int, 0) = 0)
  FROM jetsapi.source_config sc
 WHERE sc.client = $1 AND sc.object_type = $2 AND sc.table_name <> $3`

// Assess reads the evidence for every feed that carries an anomaly from one of
// the two detectors, or for the named tables only, and reaches a verdict on
// each. It writes nothing.
func Assess(ctx context.Context, db DB, tables []string, o Options) ([]Assessment, error) {
	if err := o.check(); err != nil {
		return nil, err
	}
	if tables == nil {
		tables = []string{}
	}
	rows, err := db.Query(ctx, candidatesSQL, observe.SubjectFeed, evidenceDetectors, tables)
	if err != nil {
		return nil, fmt.Errorf("while listing the feeds carrying overlap or unchanged anomalies: %w", err)
	}
	var out []Assessment
	var configured []bool
	for rows.Next() {
		var a Assessment
		var has bool
		var cumulative int
		if err := rows.Scan(&a.TableName, &has, &a.Client, &a.Org, &a.ObjectType, &cumulative); err != nil {
			rows.Close()
			return nil, fmt.Errorf("while scanning a candidate feed: %w", err)
		}
		a.Cumulative = cumulative != 0
		out = append(out, a)
		configured = append(configured, has)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("while listing candidate feeds: %w", err)
	}
	for i := range out {
		if err := assessOne(ctx, db, &out[i], configured[i], o); err != nil {
			return nil, err
		}
	}
	return out, nil
}

func assessOne(ctx context.Context, db DB, a *Assessment, configured bool, o Options) error {
	if err := readDeliveries(ctx, db, a); err != nil {
		return err
	}
	a.Streak = trailingEvidence(a.Deliveries, o.MinShare)
	switch {
	case !configured:
		a.Reason = ReasonNoConfiguration
		return nil
	case a.Cumulative:
		a.Reason = ReasonDeclaredCumulative
		return nil
	case len(a.Streak) < o.MinStreak:
		a.Reason = ReasonShortStreak
		return nil
	}
	for _, d := range a.Streak {
		if d.IncidentID != "" {
			a.TriggerIncident = d.IncidentID
			break
		}
	}
	if a.TriggerIncident == "" {
		a.Reason = ReasonNoIncident
		return nil
	}
	if err := db.QueryRow(ctx, sharedSQL, a.Client, a.ObjectType, a.TableName).Scan(
		&a.SharedObjectType, &a.Undeclared); err != nil {
		return fmt.Errorf("while counting the configurations sharing feed %s's object type: %w", a.TableName, err)
	}
	a.Qualifies = true
	return nil
}

// trailingEvidence returns the run of evidence deliveries that ends at the
// latest delivery, oldest first.
func trailingEvidence(ds []Delivery, minShare float64) []Delivery {
	i := len(ds)
	for i > 0 && ds[i-1].Evidence(minShare) {
		i--
	}
	return slices.Clone(ds[i:])
}

func readDeliveries(ctx context.Context, db DB, a *Assessment) error {
	// The detectors' own reading of a feed's deliveries, so a delivery here
	// is the delivery they judged and a re-registration is skipped as they
	// skip it.
	regs, err := observe.ReadFeedRegistrations(ctx, db, []string{a.TableName})
	if err != nil {
		return fmt.Errorf("while reading the deliveries of feed %s: %w", a.TableName, err)
	}
	a.Unordered = regs.Unordered[a.TableName]
	bySession := map[string]int{}
	for _, d := range regs.Deliveries(a.TableName, time.Time{}) {
		bySession[d.SessionId] = len(a.Deliveries)
		a.Deliveries = append(a.Deliveries, Delivery{SessionID: d.SessionId, Ordinal: d.Ordinal,
			Reprocessed: len(d.Reprocessed)})
		a.Reregistrations += len(d.Reprocessed)
	}

	rows, err := db.Query(ctx, evidenceSQL, observe.SubjectFeed, a.TableName, evidenceDetectors)
	if err != nil {
		return fmt.Errorf("while reading the overlap and unchanged anomalies of feed %s: %w", a.TableName, err)
	}
	for rows.Next() {
		var id, session, det, observed string
		if err := rows.Scan(&id, &session, &det, &observed); err != nil {
			rows.Close()
			return fmt.Errorf("while scanning an anomaly of feed %s: %w", a.TableName, err)
		}
		i, ok := bySession[session]
		if !ok {
			// An anomaly on a session that is not a delivery (no longer
			// registered, or a re-processing): it cannot be placed in the
			// order of deliveries, so it is not evidence.
			continue
		}
		d := &a.Deliveries[i]
		switch det {
		case observe.ClaimSetDeliveryUnchanged:
			d.Unchanged = true
			d.AnomalyIDs = append(d.AnomalyIDs, id)
		case observe.ClaimSetDeliveryOverlap:
			share, err := strconv.ParseFloat(strings.TrimSpace(observed), 64)
			if err != nil {
				// "N keys absent": the cumulative branch, not a share.
				continue
			}
			if d.Share == nil || share > *d.Share {
				d.Share = &share
			}
			d.AnomalyIDs = append(d.AnomalyIDs, id)
		}
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return fmt.Errorf("while reading the anomalies of feed %s: %w", a.TableName, err)
	}

	sessions := make([]string, len(a.Deliveries))
	for i := range a.Deliveries {
		sessions[i] = a.Deliveries[i].SessionID
	}
	rows, err = db.Query(ctx, incidentsSQL, triage.LocusDeliveryDepartsProfile, sessions)
	if err != nil {
		return fmt.Errorf("while reading the incidents on feed %s's deliveries: %w", a.TableName, err)
	}
	for rows.Next() {
		var session, id, status string
		if err := rows.Scan(&session, &id, &status); err != nil {
			rows.Close()
			return fmt.Errorf("while scanning an incident of feed %s: %w", a.TableName, err)
		}
		if i, ok := bySession[session]; ok {
			a.Deliveries[i].IncidentID, a.Deliveries[i].IncidentStatus = id, status
		}
	}
	rows.Close()
	return rows.Err()
}

// DraftAll assesses the feeds (all that carry evidence, or the named tables)
// and records a draft proposal for each that qualifies and has none. It never
// writes a second proposal for a feed and case, whatever state the first is
// in: a rejected draft stays the record of a person's decision, and drafting
// it again would ask the question they answered.
func DraftAll(ctx context.Context, db DB, tables []string, o Options) (*Result, error) {
	assessed, err := Assess(ctx, db, tables, o)
	if err != nil {
		return nil, err
	}
	res := &Result{Assessed: assessed}
	for i := range assessed {
		a := &assessed[i]
		if !a.Qualifies {
			continue
		}
		p := Build(a, o)
		d := Draft{Assessment: a, Proposal: p}
		existing, err := audit.ReadProposal(ctx, db, p.ProposalId)
		var missing *audit.ErrNoProposal
		switch {
		case err == nil:
			d.Existing = existing.ApprovalState
		case !errors.As(err, &missing):
			return nil, err
		case o.DryRun:
		default:
			if err := audit.RecordProposal(ctx, db, p); err != nil {
				var pgErr *pgconn.PgError
				if !errors.As(err, &pgErr) || pgErr.Code != "23505" {
					return nil, err
				}
				// Another pass recorded it between the read and the insert.
				d.Existing = audit.StateDraft
			} else {
				d.Written = true
			}
		}
		res.Drafts = append(res.Drafts, d)
	}
	return res, nil
}

// Build composes the proposal for a qualifying assessment. It reads nothing.
func Build(a *Assessment, o Options) *audit.Proposal {
	return &audit.Proposal{
		ProposalId:    ProposalIDFor(CaseCumulativeUnderIncremental, a.TableName),
		Trigger:       Trigger,
		TriggerRef:    a.TriggerIncident,
		Rationale:     Rationale(a, o),
		ApprovalState: audit.StateDraft,
		ModelVersion:  o.ModelVersion,
		// The asset the change touches is the feed's source configuration.
		// The pipelines that read the feed with a lookback are not resolved
		// here, so affected_pipelines is empty: asked, and not known.
		ImpactAffectedAssets: []string{"jetsapi.source_config:" + a.TableName},
	}
}

// Rationale is the proposal's text: the declaration, the evidence by
// delivery, the change proposed, what it changes, and what this proposal does
// not do. It carries counts, shares, session ids and names from the source
// configuration, and nothing read from a claim.
func Rationale(a *Assessment, o Options) string {
	var b strings.Builder
	org := "org " + a.Org
	if a.Org == "" {
		org = "no org"
	}
	fmt.Fprintf(&b, "Proposed change: declare feed %s cumulative, by setting source_config.is_cumulative "+
		"to 1 for it (client %s, %s, object type %s).\n\n", a.TableName, a.Client, org, a.ObjectType)

	fmt.Fprintf(&b, "Why. The feed is declared incremental (is_cumulative = 0), so each delivery should "+
		"carry claims not delivered before. Its latest %d of %d deliveries each re-deliver most of what "+
		"their predecessor held, which is the shape of a cumulative feed: every delivery the whole feed "+
		"so far. A delivery counts when its delivery_overlap share (its distinct keys already in its "+
		"predecessor, over its distinct keys) is at least %.2f, or when delivery_unchanged found its key "+
		"set equal to its predecessor's with no newer date of service; it takes %d such deliveries in a row, "+
		"ending at the latest, so a single restated delivery does not draft this.\n",
		len(a.Streak), len(a.Deliveries), o.MinShare, o.MinStreak)
	if a.Reregistrations > 0 {
		fmt.Fprintf(&b, "A delivery is a file at its first registration; the feed's %d later registrations of "+
			"files it already holds are re-processing and are not counted.\n", a.Reregistrations)
	}
	for _, d := range a.Streak {
		var what []string
		if d.Share != nil {
			what = append(what, fmt.Sprintf("overlap share %.4f", *d.Share))
		}
		if d.Unchanged {
			what = append(what, "unchanged")
		}
		inc := "no incident"
		if d.IncidentID != "" {
			inc = fmt.Sprintf("incident %s, %s", d.IncidentID, d.IncidentStatus)
		}
		again := ""
		if d.Reprocessed > 0 {
			again = fmt.Sprintf("; its file registered %d more time(s) since", d.Reprocessed)
		}
		fmt.Fprintf(&b, "  - delivery %d, session %s: %s (%s%s)\n", d.Ordinal, d.SessionID,
			strings.Join(what, ", "), inc, again)
	}

	b.WriteString("\nWhat it changes. Two things read the declaration. The feed detectors would judge the feed " +
		"as cumulative: delivery_overlap would check that each delivery contains its predecessor, rather than " +
		"that it re-delivers little, so a growing delivery would no longer be an overlap departure; " +
		"delivery_unchanged judges a cumulative feed as it judges an incremental one. And a " +
		"pipeline whose stage or merge channel names this object type in source_object_type, with " +
		"lookback_periods, would read only the latest delivery across the whole lookback (the most recent " +
		"period holding data, then its latest session) instead of every delivery of every period. A channel " +
		"with no lookback, or one that does not name the object type, reads as before.\n")
	switch {
	case a.SharedObjectType == 0:
	case a.Undeclared > 0:
		fmt.Fprintf(&b, "\nThe cumulative read is per client and object type, and %d other source "+
			"configuration(s) of client %s share object type %s, %d of them not declared cumulative. "+
			"Declaring this feed alone leaves the object type mixed, and a cumulative read of a mixed "+
			"object type fails the run; they need the same declaration, or a decision that they differ.\n",
			a.SharedObjectType, a.Client, a.ObjectType, a.Undeclared)
	default:
		fmt.Fprintf(&b, "\n%d other source configuration(s) of client %s share object type %s, and all "+
			"are already declared cumulative.\n", a.SharedObjectType, a.Client, a.ObjectType)
	}

	fmt.Fprintf(&b, "\nWhat this proposal does not do. Nothing has been applied: the source configuration is "+
		"unchanged, and this proposal stays in draft until a person moves it. The incident it names, %s, "+
		"stays where shadow triage left it, and no remediation has been recorded. If the feed is genuinely "+
		"incremental and these deliveries were restatements, reject it: the reason recorded with the "+
		"decision is the record, and the same proposal is not drafted again.\n", a.TriggerIncident)
	return b.String()
}
