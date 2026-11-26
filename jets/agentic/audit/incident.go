package audit

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgconn"
)

// Reading incidents and their hypotheses back — the
// read path for the incident and hypothesis tables.
//
// **This is an earlier finding for a third pair of tables, and it arrives before the
// writer rather than after it.** The audit store had a write path and no read
// path; `change_proposal` had long been written and never selected. Here
// there is no writer at all yet — the shadow-mode wiring is to come — so these
// functions read tables that are, today, always empty. That is deliberate and it
// is stated rather than hidden: a screen built against an empty table is a screen
// whose empty state is its main behaviour, and building the read side first is
// what makes the empty state something designed instead of something observed.
//
// # Two vocabularies, and the reason both are carried
//
// An analysis found that JetStore's execution record supports a taxonomy of
// **locus** — where in the record the evidence sits — and does not support a
// taxonomy of **cause** below the level of a hypothesis. The schema therefore puts both
// on the row: `incident_locus` is required, `classification` is optional,
// and reading one without the other is that mistake arriving in a query. So neither
// function here returns a locus without its classification, exactly as
// ReadTranscript does not return events without their verdict — the reasons are
// different, and the shape of the remedy is the same one.
//
// # What is not here
//
// **No write path** — in *this file*. An incident's status walks the incident state
// machine and the transitions that matter are adjudications — `reclassified`,
// `verified`, `suppressed_as_benign` — which is exactly what needs an
// actor and a timestamp. That work was decided on
// 2026-09-04 and **it landed the same day**: `RecordIncidentTransition` in
// `incident_transition.go`. The sentence is kept because its reasoning is why the
// two files are separate, and corrected because *no write path* now reads as a
// claim about the package rather than about the file.

// The eleven loci, mirroring `IncidentLocus` in model.py and the
// CHECK the DDL generates from it: nine positions in a run's execution record,
// then two in a feed's deliveries. Named constants for the same reason the
// approval states are: a vocabulary written as string literals is a typo away
// from a filter that silently matches nothing.
const (
	LocusRunNotStarted                  = "run_not_started"
	LocusStepNeverStarted               = "step_never_started"
	LocusWorkerNotTerminated            = "worker_not_terminated"
	LocusWorkerFailed                   = "worker_failed"
	LocusSinkFailedUnderCompletedWorker = "sink_failed_under_completed_worker"
	LocusRowsLostSilently               = "rows_lost_silently"
	LocusPerRecordFailuresReported      = "per_record_failures_reported"
	LocusPerRecordFailuresUnreportable  = "per_record_failures_unreportable"
	LocusWrittenNotArrived              = "written_not_arrived"
	LocusDeliveryOverdue                = "delivery_overdue"
	LocusDeliveryDepartsProfile         = "delivery_departs_profile"
)

// IncidentLoci is the vocabulary in its defined order, which is the order the
// table is written in and roughly the order in which a run fails. The two
// delivery loci come last because the vocabulary is appended to and never
// renumbered, not because a delivery fails after a run: it precedes one. It is sent to
// the screen rather than repeated there, on the same argument the transition set
// is: the client's copy of a vocabulary is the copy that cannot be enforced.
var IncidentLoci = []string{
	LocusRunNotStarted, LocusStepNeverStarted, LocusWorkerNotTerminated,
	LocusWorkerFailed, LocusSinkFailedUnderCompletedWorker, LocusRowsLostSilently,
	LocusPerRecordFailuresReported, LocusPerRecordFailuresUnreportable,
	LocusWrittenNotArrived,
	LocusDeliveryOverdue, LocusDeliveryDepartsProfile,
}

// IncidentStatuses is `IncidentStatus`'s eleven values. **Three of them are
// adjudications rather than progress** — `reclassified`, `verified` and
// `suppressed_as_benign` — and that is the reason this
// list is ordered with them last rather than interleaved: a screen that filters
// on status is filtering on two different kinds of thing, and the order is the
// only thing here that says so.
var IncidentStatuses = []string{
	"detected", "triaged", "diagnosed", "remediation_proposed", "awaiting_approval",
	"remediating", "resolved", "closed",
	"verified", "reclassified", "suppressed_as_benign",
}

// IncidentClassifications is the ten causes `classification` may name, in the
// order the CHECK carries them.
//
// **It exists because a screen that offers a reclassification has to offer the
// vocabulary to reclassify *into***. Until then nothing needed the
// list in Go: the column is optional and every reader rendered whatever
// value it found. It travels to the browser with the incident on the same
// argument as `IncidentLoci` and the transition sets — a client's copy of a
// controlled vocabulary is the copy that cannot be enforced —
// and `TestIncidentClassificationsMatchTheGeneratedCheck` holds it against the
// generated SQL rather than against a comment.
//
// **Do not read it as a taxonomy of cause that the record supports.** The analysis's
// finding is the opposite: the execution record supports a taxonomy of *locus*
// and does not determine a cause, which is exactly why this column is nullable
// and why a human's choice from this list is a judgement rather than a lookup.
var IncidentClassifications = []string{
	"source_delivery_failure", "source_content_change", "transport_failure",
	"parse_failure", "validation_breach", "transformation_defect",
	"infrastructure_failure", "dependency_failure", "capacity_or_cost_deviation",
	"benign_variation",
}

// KnownIncidentClassification reports whether v is one of the ten. A request may
// carry anything; the CHECK would refuse it at the INSERT, and refusing it here
// names the vocabulary instead of naming a constraint.
func KnownIncidentClassification(v string) bool {
	return inVocabulary(IncidentClassifications, v)
}

// KnownLocus and KnownIncidentStatus report vocabulary membership. A row cannot
// hold anything else — the CHECKs see to that — but a request body can, and a
// caller naming a value that does not exist should be told so rather than
// silently handed an empty list, which is indistinguishable from a quiet system.
func KnownLocus(v string) bool { return inVocabulary(IncidentLoci, v) }

// KnownIncidentStatus reports whether v is one of `IncidentStatus`'s eleven.
func KnownIncidentStatus(v string) bool { return inVocabulary(IncidentStatuses, v) }

func inVocabulary(vocab []string, v string) bool {
	for _, s := range vocab {
		if s == v {
			return true
		}
	}
	return false
}

// Evidence is one item of a hypothesis's supporting or contradicting evidence —
// `Evidence` in model.py, a value object with no table of its own, stored inline
// as jsonb.
//
// **`Statement` is the only property in the whole domain model carrying
// `data_classification = "PHI"`** (`statement`,
// `tools/jets_agentic/jets_agentic/model.py:661`), and the marker reaches the
// workspace as a rule-visible triple.
//
// **~~Nothing in Go or in the browser reads that marker.~~ It was given a
// reader on 2026-09-04.**
// The marker is now generated into `DataClassifiedProperties` and enforced by
// `phi.go`: `Statement` is withheld unless the read was asked for `DisclosePHI`,
// which the apiserver passes only for a caller holding the PHI capability. The
// rule-visible triple is unchanged and is still matched by no rule; what gained
// a consumer is the marker, not the triple.
type Evidence struct {
	Statement string `json:"statement"`
	Source    string `json:"source"`
	SourceRef string `json:"source_ref"`
	// StatementRedacted says the statement was withheld rather than empty.
	// `json:"-"` because this is a property of the *read*, not of the stored
	// value: it must never round-trip into the jsonb column, and a writer that
	// serialised it would be recording a policy decision as data.
	//
	// **Blanked and flagged rather than replaced with a placeholder**: a
	// placeholder in the value is indistinguishable from an agent that wrote
	// those words, and a screen has to tell *withheld* from *the evidence is
	// empty*.
	StatementRedacted bool `json:"-"`
}

// HypothesisBasis is the `basis` column: how the rank was arrived at, as counts.
//
// **Added by the user's decision, and it is numbers rather than
// prose on purpose.** The ranker emits a sentence per hypothesis and a sentence per
// ranking; a sentence cannot be checked against anything and a count can — these
// two are the lengths of the evidence arrays on the same row, so a reader can
// confirm months later that the stored confidence is the ratio it claims to be.
//
// **Why it is stored at all**, which the question had to settle rather than
// assume: *reconstructable by re-running* is false for the model arm (16
// hypotheses on one run and 13 on the next from identical input),
// and the evidence expires while the row persists, on two clocks the row does
// not share — `RETENTION_DAYS`, an environment variable with no default, and six
// months hard-coded on the run header.
type HypothesisBasis struct {
	SupportingCount    int `json:"supporting_count"`
	ContradictingCount int `json:"contradicting_count"`
	// Evidenceability is the per-class evidence rating reduced to five tiers, and it
	// is the ranker's **primary** sort key rather than a gloss: a class the
	// substrate cannot speak to outranks nothing, whatever its counts. That
	// inversion is what a ratio over evidence positions got wrong on its first
	// run, which is why the tier is persisted and not recomputed.
	Evidenceability string `json:"evidenceability"`
}

// Hypothesis is one row of jetsapi.hypothesis: a ranked causal claim about an
// incident, with the evidence on both sides.
//
// **The two evidence slices are both present or the read fails**, which is a
// property of the column rather than of this struct: `contradicting_evidence` is
// NOT NULL because it is a calibration control, and an empty array is
// the honest value where an agent asserts none. A reader that treated absent and
// empty alike would erase that distinction on the way to the screen.
type Hypothesis struct {
	HypothesisId string
	IncidentRef  string
	Cause        string
	// CauseCategory is "" when the hypothesis names no class from the imported
	// ten. Nullable for the same reason `Incident.Classification` is: the record
	// evidences a locus and not a cause.
	CauseCategory         string
	Confidence            float64
	Rank                  int64
	SupportingEvidence    []Evidence
	ContradictingEvidence []Evidence
	// Locus is the triage verdict the hypothesis was raised from, in the same
	// nine-value vocabulary `Incident.Locus` carries and constrained by a CHECK
	// built from the same list (`hypothesis_locus_ck`).
	//
	// **It is not redundant with the incident's locus and that is the point.**
	// The shadow writer writes one incident per (session, locus) and each hypothesis under
	// the incident of its own locus, so today the two agree by construction —
	// but a ranker that proposed a cause at a locus triage did **not** find
	// present would produce a row where they do not, and the ranker's evaluation measured that as
	// 20 of 29 on its model arm. Without the column such a row is
	// indistinguishable from a sound one once written.
	Locus string
	// Basis is the `basis` column: criterion 45's last clause, persisted.
	Basis HypothesisBasis
}

// IncidentSummary is one row of the incident list.
//
// **Unlike ProposalSummary this is the whole row and not a subset**, because the
// row is eleven narrow columns with no free text in it: there is nothing here a
// list would have to fetch in order to truncate. What it adds is the hypothesis
// count, which is the one thing that changes whether an incident is worth opening
// and the one thing not on the table.
type IncidentSummary struct {
	IncidentId string
	SessionId  string
	// RunRef is the AgentRun that raised the incident, "" when nothing agentic
	// did — which is what decides whether a transition on this
	// incident reaches the hash chain.
	//
	// It sits on the summary because `Incident` embeds this struct, so the
	// detail read needs it here; **the list screen does not render it**, a
	// seventh column costing more than it tells at a glance. Both queries select
	// it all the same, since the column is narrow and a summary that omitted it
	// would make the two reads disagree about what a row is.
	RunRef         string
	DetectedAt     time.Time
	Locus          string
	Classification string
	Severity       string
	Status         string
	StepRef        string
	// ShardRef is a pointer because **0 is the first shard**, so coalescing a
	// null to zero would invent a localisation the incident does not claim. Every
	// other nullable column here is text and coalesces to "" safely.
	ShardRef        *int64
	Confounders     []string
	ModelVersion    string
	HypothesisCount int
	// Client and ObjectType are the registration of the incident's session,
	// "" when nothing registered it or the registration has been purged.
	// Only the paged list reads them (ListIncidentPage); see listing.go.
	Client     string
	ObjectType string
}

// Incident is one incident with its ranked hypotheses, which is how a human reads
// one and the only way this package returns one.
type Incident struct {
	IncidentSummary
	Hypotheses []Hypothesis
}

// maxIncidentPage bounds one page of the incident list, on maxProposalPage's
// terms: the screen pages by offset (ListIncidentPage), and this is the ceiling
// on a page.
const maxIncidentPage = 500

// ErrNoIncident is returned when no such incident exists — a stale link or a
// mistyped id, which is the one failure a caller acts on.
type ErrNoIncident struct{ IncidentId string }

func (e *ErrNoIncident) Error() string { return fmt.Sprintf("no incident %s", e.IncidentId) }

// ErrTablesNotDeployed is returned when the query names a table that does not
// exist — Postgres 42P01.
//
// **This is the precondition criterion 34 carried and its text did not say.**
// `jetsapi.incident` and `jetsapi.hypothesis` reach a database only through
// `audit.InstallSchema`, which runs on `update_db -migrateDb`,
// so on any deployment that has not been migrated since they were added these tables are
// absent. Without this the screen shows *"ERROR: relation jetsapi.incident does
// not exist"*, which reads as an outage; with it the screen can say the one true
// thing, which is that a migration has not been run.
//
// **It is here rather than in the handler because the handler cannot tell.** By
// the time an error is a string it is indistinguishable from every other database
// failure, and the code is on the pgconn error rather than in the message.
type ErrTablesNotDeployed struct{ Detail string }

func (e *ErrTablesNotDeployed) Error() string {
	return fmt.Sprintf("the incident tables are not deployed in this database; "+
		"run `update_db -migrateDb`, which is what installs them (%s)", e.Detail)
}

// notDeployed maps Postgres's undefined_table to ErrTablesNotDeployed and passes
// everything else through unchanged.
func notDeployed(err error) error {
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && pgErr.Code == "42P01" {
		return &ErrTablesNotDeployed{Detail: pgErr.Message}
	}
	return err
}

// ListIncidents returns the incident list, most recently detected first, bounded
// by limit. An empty status filter means every status.
//
// **The order is detection time and not severity**, which is a choice worth
// stating: severity is a five-value judgement an agent wrote, and ordering a
// supervision queue by the subject's own opinion of itself puts the rows nobody
// has checked at the bottom. Detection time is the only column here that no agent
// judges.
func ListIncidents(ctx context.Context, db Querier, statuses []string, limit int) ([]IncidentSummary, error) {
	rows, _, err := ListIncidentPage(ctx, db, statuses, ListPage{Limit: limit})
	return rows, err
}

// IncidentsOnSessions returns the incidents written on the given sessions, at
// any locus, by session and then by locus. It is what a person who holds a
// session, such as a pipeline run's or a delivery's, asks first: which
// incidents are on it. A session with none has no row, and no session is an
// empty list.
//
// **It answers only for the session itself.** A later delivery of a feed's
// standing condition carries no incident of its own: the condition's incident
// is on another session, and which one is derived by the feedcondition
// package, not read here.
func IncidentsOnSessions(ctx context.Context, db Querier, sessions []string) ([]IncidentSummary, error) {
	if len(sessions) == 0 {
		return nil, nil
	}
	rows, err := db.Query(ctx,
		`SELECT i.incident_id, i.incident_session_id, coalesce(i.incident_run_ref, ''),
		        i.incident_detected_at,
		        i.incident_locus, coalesce(i.classification, ''), i.severity, i.status,
		        coalesce(i.incident_step_ref, ''), i.incident_shard_ref,
		        i.incident_confounders, i.incident_model_version,
		        coalesce(h.n, 0)
		   FROM jetsapi.incident i
		   LEFT JOIN (
		         SELECT hypothesis_incident_ref, count(*) AS n
		           FROM jetsapi.hypothesis
		          GROUP BY hypothesis_incident_ref) h
		     ON h.hypothesis_incident_ref = i.incident_id
		  WHERE i.incident_session_id = ANY($1::text[])
		  ORDER BY i.incident_session_id, i.incident_locus, i.incident_id
		  LIMIT $2`, sessions, maxIncidentPage)
	if err != nil {
		return nil, fmt.Errorf("while reading the incidents on sessions %v: %w", sessions, notDeployed(err))
	}
	defer rows.Close()
	var out []IncidentSummary
	for rows.Next() {
		var s IncidentSummary
		if err := rows.Scan(&s.IncidentId, &s.SessionId, &s.RunRef, &s.DetectedAt, &s.Locus,
			&s.Classification, &s.Severity, &s.Status, &s.StepRef, &s.ShardRef,
			&s.Confounders, &s.ModelVersion, &s.HypothesisCount); err != nil {
			return nil, fmt.Errorf("while scanning an incident summary: %w", err)
		}
		out = append(out, s)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("while reading the incidents on sessions %v: %w", sessions, notDeployed(err))
	}
	return out, nil
}

// ReadIncident returns one incident with its hypotheses in rank order.
//
// **The hypotheses come back with the incident and not behind a second call**,
// which is ReadTranscript's argument rather than a convenience: a ranked
// hypothesis is what an incident's diagnosis *is*, and an endpoint that could
// return the classification without the reasoning would offer a caller the choice
// of showing a claim with no basis.
func ReadIncident(ctx context.Context, db Querier, incidentId string, phi PHIAccess) (*Incident, error) {
	if incidentId == "" {
		return nil, fmt.Errorf("cannot read an incident without an id")
	}
	rows, err := db.Query(ctx,
		`SELECT incident_id, incident_session_id, coalesce(incident_run_ref, ''),
		        incident_detected_at, incident_locus,
		        coalesce(classification, ''), severity, status,
		        coalesce(incident_step_ref, ''), incident_shard_ref,
		        incident_confounders, incident_model_version
		   FROM jetsapi.incident
		  WHERE incident_id = $1`, incidentId)
	if err != nil {
		return nil, fmt.Errorf("while reading incident %s: %w", incidentId, notDeployed(err))
	}
	defer rows.Close()
	if !rows.Next() {
		if err := rows.Err(); err != nil {
			return nil, fmt.Errorf("while reading incident %s: %w", incidentId, notDeployed(err))
		}
		return nil, &ErrNoIncident{IncidentId: incidentId}
	}
	var inc Incident
	if err := rows.Scan(&inc.IncidentId, &inc.SessionId, &inc.RunRef, &inc.DetectedAt, &inc.Locus,
		&inc.Classification, &inc.Severity, &inc.Status, &inc.StepRef, &inc.ShardRef,
		&inc.Confounders, &inc.ModelVersion); err != nil {
		return nil, fmt.Errorf("while scanning incident %s: %w", incidentId, err)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("while reading incident %s: %w", incidentId, err)
	}
	rows.Close()

	hs, err := HypothesesFor(ctx, db, incidentId, phi)
	if err != nil {
		return nil, err
	}
	inc.Hypotheses = hs
	inc.HypothesisCount = len(hs)
	return &inc, nil
}

// HypothesesFor returns an incident's hypotheses in rank order, which is the
// only order in which a human reads them and the order the index exists for.
func HypothesesFor(ctx context.Context, db Querier, incidentRef string, phi PHIAccess) ([]Hypothesis, error) {
	rows, err := db.Query(ctx,
		`SELECT hypothesis_id, hypothesis_incident_ref, cause, coalesce(cause_category, ''),
		        confidence, rank, supporting_evidence, contradicting_evidence,
		        hypothesis_locus, basis
		   FROM jetsapi.hypothesis
		  WHERE hypothesis_incident_ref = $1
		  ORDER BY rank, hypothesis_id`, incidentRef)
	if err != nil {
		return nil, fmt.Errorf("while reading hypotheses for %s: %w", incidentRef, notDeployed(err))
	}
	defer rows.Close()
	var out []Hypothesis
	for rows.Next() {
		var h Hypothesis
		var supporting, contradicting, basis []byte
		if err := rows.Scan(&h.HypothesisId, &h.IncidentRef, &h.Cause, &h.CauseCategory,
			&h.Confidence, &h.Rank, &supporting, &contradicting,
			&h.Locus, &basis); err != nil {
			return nil, fmt.Errorf("while scanning a hypothesis for %s: %w", incidentRef, err)
		}
		// The column is jsonb NOT NULL, so a shape this cannot parse is an error
		// rather than a zero basis: three zeros read as *nothing for it, nothing
		// against it, and the record cannot evidence it*, which is a claim rather
		// than a decode failure.
		if err := json.Unmarshal(basis, &h.Basis); err != nil {
			return nil, fmt.Errorf("hypothesis %s: basis: %w", h.HypothesisId, err)
		}
		// Both columns are NOT NULL jsonb holding an array of Evidence. A shape
		// this cannot parse is an error rather than a silently empty list: the
		// first writer is the ranker and a wrong shape reaching a screen as "no
		// evidence" would read as a hypothesis nobody could argue with.
		if h.SupportingEvidence, err = decodeEvidence(supporting); err != nil {
			return nil, fmt.Errorf("hypothesis %s: supporting_evidence: %w", h.HypothesisId, err)
		}
		if h.ContradictingEvidence, err = decodeEvidence(contradicting); err != nil {
			return nil, fmt.Errorf("hypothesis %s: contradicting_evidence: %w", h.HypothesisId, err)
		}
		// The PHI decision is applied here rather than by the caller, so a
		// caller cannot hold an unredacted Evidence without having said
		// DisclosePHI. See phi.go for why it is a parameter.
		applyPHIPolicy(phi, h.SupportingEvidence, h.ContradictingEvidence)
		out = append(out, h)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("while reading hypotheses for %s: %w", incidentRef, notDeployed(err))
	}
	return out, nil
}

// decodeEvidence parses one jsonb evidence column. Empty rather than nil on an
// empty array, so "asked, and there are none" survives to the screen — the same
// distinction RecordProposal preserves when it writes its arrays.
func decodeEvidence(raw []byte) ([]Evidence, error) {
	if len(raw) == 0 {
		return []Evidence{}, nil
	}
	var out []Evidence
	if err := json.Unmarshal(raw, &out); err != nil {
		return nil, err
	}
	if out == nil {
		return []Evidence{}, nil
	}
	return out, nil
}
