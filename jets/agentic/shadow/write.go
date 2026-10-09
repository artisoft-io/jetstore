package shadow

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/artisoft-io/jetstore/jets/agentic/audit"
	"github.com/artisoft-io/jetstore/jets/agentic/observe"
	"github.com/artisoft-io/jetstore/jets/agentic/rca"
	"github.com/artisoft-io/jetstore/jets/agentic/triage"
)

// Writer runs triage and RCA over one session and writes what they concluded.
//
// It is the first thing in JetStore that writes jetsapi.incident — until this
// task the two INSERT sites in the tree were both test fixtures (F288) — and it
// is deliberately the only one.
type Writer struct {
	// Classifier is AC.1's nine predicates.
	Classifier triage.Classifier
	// Ranker is AC.2's deterministic floor. **The model arm is not wired**:
	// rca.Consult is a measurement rather than a step (§19.6), it needs a model
	// name from the environment, and a run that silently consulted one would
	// make every hypothesis's provenance depend on a variable nothing records
	// on the row.
	Ranker rca.Ranker

	// Actor is the agent identity written on every transition. Required: a
	// transition with no actor is the one thing the incident_event table exists
	// to prevent, and there is no honest default for *who this was*.
	Actor string

	// Severity is written on every incident, and **it is a posture rather than
	// a measurement** (I-306).
	//
	// jetsapi.incident.severity is NOT NULL over five values and no locus
	// determines any of them: §9.4's nine rows say where evidence sits and none
	// says how much it matters, and F201 establishes that the record carries no
	// cost, no memory, no CPU and no duration finer than one wall clock per
	// worker — so there is no impact signal to derive one from either. AC.1
	// therefore gives Finding no severity field, and the first writer has to
	// supply what the classifier refuses to invent.
	//
	// **The default is `info`, argued from the deployment posture and not from
	// the failure.** Nothing acts on these rows: they are read by a supervisor
	// and by nothing else, so every incident this writer produces is
	// informational *by construction* — which is a property of shadow mode
	// rather than an opinion about the run. The day something acts on one, the
	// same argument stops holding and a policy is owed; that is the value's
	// falsification condition and it is the reason `info` is not merely the
	// smallest number available.
	//
	// **Two consequences, stated rather than left to be found.** The column is
	// constant across every row this writer produces, so it sorts nothing and
	// filters nothing — which is why AE.1's decision to order the supervision
	// list by detection time rather than by severity is now moot as well as
	// right. And a constant is the *least bad* option rather than a good one:
	// the vocabulary has no `not_assessed` member and the column is NOT NULL, so
	// there is no way to say *this was not judged*. **Q-48** asks whether the
	// column should be nullable or the vocabulary widened, on I-289's argument
	// one column over.
	Severity string

	// ModelVersion is written to incident_model_version. See the package
	// constant for why it is a hand-transcribed string.
	ModelVersion string

	// Baseline is how far back the step history reaches. Zero reads none, which
	// makes locus step_never_started NotEvaluable rather than Absent — a
	// legitimate answer and not a degraded one (§13.4).
	Baseline time.Duration

	// DryRun classifies, ranks and reports, and writes nothing.
	DryRun bool

	// Now is the clock, injectable so a test can order transitions
	// unambiguously. Nil means time.Now().UTC().
	Now func() time.Time
}

// DefaultWriter is the starting point. Actor has no default and must be set.
func DefaultWriter() Writer {
	return Writer{
		Classifier:   triage.Default(),
		Ranker:       rca.Default(),
		Severity:     SeverityInfo,
		ModelVersion: ModelVersion,
		Baseline:     30 * 24 * time.Hour,
	}
}

func (w *Writer) now() time.Time {
	if w.Now != nil {
		return w.Now().UTC()
	}
	return time.Now().UTC()
}

// IncidentWritten is one incident this run produced.
type IncidentWritten struct {
	IncidentId string
	Locus      string
	// Status is where the incident was left: `triaged` when the ranker produced
	// no hypothesis for the locus, `diagnosed` when it did. **Never anything
	// else** — see ShadowStatuses.
	Status     string
	Hypotheses int
	// ChainSeq is the audit chain seq the last transition produced, and it is
	// **0 for everything this writer creates**. RecordIncidentTransition appends
	// a chain event only when the incident names an AgentRun (AB.4), and a
	// deterministic classifier is not one — so these transitions are durable and
	// attributable and are not tamper-evident (R-44, gap 27). It is reported
	// rather than dropped so that the zero is visible in the output instead of
	// being a property somebody has to know.
	ChainSeq int
}

// SessionResult is what one call concluded and what it wrote.
type SessionResult struct {
	SessionId string
	// Report is all nine verdicts, kept whole. A caller counting per class
	// iterates this and not Written: the loci that did *not* fire are the
	// denominator, and a result that carried only the hits would supply
	// numerators alone (§13.4).
	Report  *triage.Report
	Ranking *rca.Ranking

	Written []IncidentWritten
	// AlreadyRaised are the fired loci this session already had an incident for.
	// **This is Q-34's answer in one field** — see Run.
	AlreadyRaised []string
	// UnmappedLoci are the fired loci §9.5's table maps to no cause class, so
	// the incident is written and carries no hypothesis. Two of the nine are
	// permanently in this state (F393).
	UnmappedLoci []string
	// Anomalies is how many of N.4's detector rows were available to the ranker.
	// Zero is the ordinary case: jetsapi.anomaly is deployed nowhere that has
	// not migrated since N.2, and no detector runs on a schedule.
	Anomalies int
}

// Run classifies one session, ranks what it found, and writes one incident per
// present locus with that locus's hypotheses under it.
//
// # Q-34 — a re-classification that can see less
//
// Asked: what does a later run that answers `not_evaluable` where an earlier one
// answered `present` mean for an incident already written?
//
// **Answered: this writer adds and never retracts, and it does not re-derive a
// locus it has already raised.** Three things make that the right answer rather
// than the convenient one.
//
//   - **A verdict is dated.** An incident records what the record supported when
//     it was read. A later read that can see *less* — a purged worker record
//     (F273), a table dropped, a retention clock that moved — is not evidence
//     against the earlier verdict; it is the absence of evidence, which is the
//     distinction the whole three-valued design exists to keep (§13.4).
//   - **Retraction would make the retention clock a redactor.** RETENTION_DAYS
//     has no default (P3 F54) and is set by a deployment rather than by this
//     repository. A writer that withdrew incidents when their evidence aged out
//     would let an environment variable delete a governance record, silently and
//     on a schedule nobody here controls.
//   - **The correction path already exists and is a person's.** An incident
//     raised on a verdict that was wrong is corrected by a human `reclassified`
//     transition, which is the instrument plan §10.7 built and which records who
//     said so. Letting the classifier revise itself would produce exactly the
//     row that section calls not a label: one the measured system wrote.
//
// The asymmetric case is deliberately *not* symmetric. A locus that was
// `not_evaluable` or `absent` and is now `present` is **new information** — a
// migration ran, a table arrived, a detector was deployed — and a later call
// writes that incident. So the rule is: this writer's output over a session
// grows and never shrinks, and the only thing that removes a claim is a person.
//
// The bound on that: an incident raised from a *defective* verdict stays until
// somebody adjudicates it, and F273's partial-purge hole is a known way to
// produce one. That is stated rather than guarded, because the guard would be
// the retraction this paragraph argues against.
func (w *Writer) Run(ctx context.Context, db DB, sessionId string) (*SessionResult, error) {
	if sessionId == "" {
		return nil, errors.New("cannot classify without a session id: an incident cannot be keyed below the session (F202)")
	}
	if w.Actor == "" {
		return nil, errors.New("Writer.Actor is empty: every incident_event carries an actor, and an unattributable transition is the one thing that record exists to prevent")
	}
	severity := w.Severity
	if severity == "" {
		severity = SeverityInfo
	}
	modelVersion := w.ModelVersion
	if modelVersion == "" {
		modelVersion = ModelVersion
	}

	targets, err := ReadTargets(ctx, db)
	if err != nil {
		return nil, err
	}
	if !w.DryRun && !targets.Ready() {
		return nil, fmt.Errorf("shadow mode has nowhere to write: %v absent from this database. "+
			"They are created by `update_db -migrateDb`, which calls audit.InstallSchema "+
			"(jets/update_db/main.go:71) — the same run that installs jetsapi.anomaly (I-169)",
			targets.Missing())
	}

	ev, err := triage.Gather(ctx, db, sessionId, w.Baseline)
	if err != nil {
		return nil, err
	}
	report := w.Classifier.Classify(ev)

	var anomalies []observe.Anomaly
	if ev.Extent != nil && ev.Extent.Anomalies {
		if anomalies, err = observe.ReadAnomalies(ctx, db, sessionId); err != nil {
			return nil, err
		}
	}
	ranking := w.Ranker.Rank(&rca.Input{Report: report, Evidence: ev, Anomalies: anomalies})
	if err := ranking.Validate(); err != nil {
		// The floor cannot produce an invalid ranking, so this is a guard on a
		// contract rather than a branch anybody expects to take — and it is here
		// rather than nowhere because the alternative is discovering it as a
		// CHECK violation halfway through writing an incident's children.
		return nil, fmt.Errorf("the ranker produced a ranking that does not validate, so nothing was written: %w", err)
	}

	out := &SessionResult{
		SessionId:    sessionId,
		Report:       report,
		Ranking:      ranking,
		Anomalies:    len(anomalies),
		UnmappedLoci: ranking.UnmappedLoci,
	}

	existing := map[string]string{}
	if targets.Incident {
		if existing, err = raisedLoci(ctx, db, sessionId); err != nil {
			return nil, err
		}
	}

	byLocus := map[string][]rca.Hypothesis{}
	for i := range ranking.Hypotheses {
		h := ranking.Hypotheses[i]
		byLocus[h.Locus] = append(byLocus[h.Locus], h)
	}

	for i := range report.Findings {
		f := &report.Findings[i]
		if !f.Fired() {
			continue
		}
		if id, ok := existing[f.Locus]; ok {
			out.AlreadyRaised = append(out.AlreadyRaised, fmt.Sprintf("%s (%s)", f.Locus, id))
			continue
		}
		if err := f.Validate(); err != nil {
			return out, fmt.Errorf("the classifier produced a finding that does not validate, so it was not written: %w", err)
		}
		written := IncidentWritten{
			IncidentId: IncidentIdFor(sessionId, f.Locus),
			Locus:      f.Locus,
			Status:     audit.IncidentDetected,
			Hypotheses: len(byLocus[f.Locus]),
		}
		if w.DryRun {
			// Report where it *would* have been left, so a dry run and a real
			// run describe the same outcome.
			written.Status = audit.IncidentTriaged
			if written.Hypotheses > 0 {
				written.Status = audit.IncidentDiagnosed
			}
			out.Written = append(out.Written, written)
			continue
		}
		if err := w.writeOne(ctx, db, f, byLocus[f.Locus], ranking, severity, modelVersion, &written); err != nil {
			return out, err
		}
		out.Written = append(out.Written, written)
	}
	return out, nil
}

// writeOne inserts the incident at `detected`, moves it to `triaged` with the
// finding's own basis as the rationale, and — where the ranker produced
// hypotheses for the locus — writes them and moves it to `diagnosed` with the
// ranking's basis.
//
// # Where criterion 45's last clause goes
//
// jetsapi.hypothesis has eight columns and **neither a basis nor a locus is one
// of them** (F402), which §19.7 grades as *met for what AC.2 emits and not
// surviving the write*. Two of the three losses are repaired here and the third
// is not:
//
//   - **The locus survives by construction of the write.** One incident per
//     (session, locus) and each hypothesis written under the incident of its own
//     locus means jetsapi.incident.incident_locus *is* the hypothesis's locus,
//     recovered by the join HypothesesFor already makes.
//   - **The ranking's basis survives in the transition record.** rca's
//     Ranking.Basis says which loci fired, which produced no hypothesis and why,
//     which could not be evaluated, which classes can never be emitted and how
//     many anomalies were read — and incident_event.transition_rationale is
//     documented as *why, in the actor's words*, which is exactly what that
//     string is. Criterion 45's last clause says *the ranking's basis*, and
//     after this it is in the database.
//   - **The locus and the basis are also columns as of Q-46**, answered by the
//     user on 2026-09-04 while this task was running and applied before the first
//     row was written rather than as a migration afterwards. `hypothesis_locus`
//     carries the `AC.1` verdict against the same CHECK `incident_locus` uses;
//     `basis` carries the two evidence counts and §9.5's evidenceability tier, as
//     numbers, because a count can be checked against the arrays beside it and a
//     sentence cannot.
//   - **What still does not survive is the per-hypothesis basis *prose* and the
//     session-wide rank**: ranks are renumbered dense within the incident because
//     §A.2.8 nests hypotheses inside one, so *ranked 4 of 13 across the session*
//     is gone. The counts are what the sentence was computed from, so the
//     arithmetic survives and the wording does not.
func (w *Writer) writeOne(ctx context.Context, db DB, f *triage.Finding, hyps []rca.Hypothesis,
	ranking *rca.Ranking, severity, modelVersion string, written *IncidentWritten) error {

	detectedAt := w.now()
	confounders := f.Confounders
	if confounders == nil {
		confounders = []string{}
	}
	tag, err := db.Exec(ctx,
		`INSERT INTO jetsapi.incident (
		   incident_id, incident_session_id, incident_run_ref, incident_detected_at,
		   incident_locus, classification, severity, status,
		   incident_step_ref, incident_shard_ref, incident_confounders, incident_model_version)
		 VALUES ($1, $2, NULL, $3, $4, NULL, $5, $6, NULLIF($7, ''), $8, $9, $10)
		 ON CONFLICT (incident_id) DO NOTHING`,
		written.IncidentId, f.SessionId, detectedAt,
		f.Locus, severity, audit.IncidentDetected,
		f.StepRef, f.ShardRef, confounders, modelVersion)
	if err != nil {
		return fmt.Errorf("while raising incident %s: %w", written.IncidentId, err)
	}
	if tag.RowsAffected() == 0 {
		// A concurrent writer got there first. The id is deterministic per
		// (session, locus) precisely so this is a no-op rather than a duplicate,
		// and Q-34's rule — add, never retract — makes leaving the other
		// writer's row alone the correct outcome rather than a compromise.
		written.Status = ""
		return nil
	}

	// **classification is deliberately left NULL, and this is the one decision
	// in the write path that could have gone the other way.** The ranker's
	// top-ranked hypothesis names a class in the imported ten, and promoting it
	// to the incident's own classification would fill the screen's *Cause*
	// column for every incident. It is refused: §9.5's finding is that the record
	// evidences a locus and never a cause, I-289 made the column optional so a
	// deterministic step need not invent one, and a floor whose confidence is a
	// ratio of counted evidence items is not a settled verdict (R-48). Thirteen
	// ranked claims, each with its own case against it, would become one asserted
	// claim with none — which deletes exactly the calibration
	// contradicting_evidence exists for. What sets classification is a human
	// `reclassified` transition, which is plan §10.7's instrument and carries
	// who said so (I-372).

	if err := w.moveTo(ctx, db, written, audit.IncidentDetected, audit.IncidentTriaged,
		detectedAt, f.Basis); err != nil {
		return err
	}
	if len(hyps) == 0 {
		return nil
	}
	for i := range hyps {
		h := hyps[i]
		h.IncidentRef = written.IncidentId
		h.HypothesisId = HypothesisIdFor(written.IncidentId, h.CauseCategory)
		// Dense within the incident: §A.2.8 nests hypotheses inside one, so a
		// rank is a position among *this incident's* claims.
		h.Rank = i + 1
		if err := h.Validate(); err != nil {
			return fmt.Errorf("the ranker produced a hypothesis that does not validate, so incident %s has none: %w",
				written.IncidentId, err)
		}
		if err := insertHypothesis(ctx, db, &h); err != nil {
			return err
		}
	}
	return w.moveTo(ctx, db, written, audit.IncidentTriaged, audit.IncidentDiagnosed,
		detectedAt.Add(time.Millisecond), ranking.Basis)
}

// moveTo is the ceiling, as a function every status change goes through.
//
// **The guard is here rather than in the caller** for the reason
// RecordIncidentTransition puts its own guard inside the write: a rule enforced
// by every caller remembering it is a rule that holds until the next caller.
func (w *Writer) moveTo(ctx context.Context, db DB, written *IncidentWritten,
	from, to string, at time.Time, rationale string) error {

	if !IsShadowStatus(to) {
		return &ErrWouldAct{IncidentRef: written.IncidentId, ToStatus: to}
	}
	t := &audit.IncidentTransition{
		IncidentEventId: fmt.Sprintf("ie_%s_%s", written.IncidentId, to),
		IncidentRef:     written.IncidentId,
		FromStatus:      from,
		ToStatus:        to,
		Actor:           w.Actor,
		ActorKind:       audit.ActorAgent,
		TransitionedAt:  at,
		Rationale:       rationale,
	}
	seq, err := audit.RecordIncidentTransition(ctx, db, t)
	if err != nil {
		return fmt.Errorf("while moving incident %s from %s to %s: %w", written.IncidentId, from, to, err)
	}
	written.Status = to
	written.ChainSeq = seq
	return nil
}

const insertHypothesisSQL = `INSERT INTO jetsapi.hypothesis (
    hypothesis_id, hypothesis_incident_ref, cause, cause_category,
    confidence, rank, supporting_evidence, contradicting_evidence,
    hypothesis_locus, basis)
  VALUES ($1, $2, $3, NULLIF($4, ''), $5, $6, $7, $8, $9, $10)
  ON CONFLICT (hypothesis_id) DO NOTHING`

func insertHypothesis(ctx context.Context, db DB, h *rca.Hypothesis) error {
	// **nil and empty are different on both columns and the difference has to
	// survive the marshal.** Validate has already refused a nil contradicting
	// slice; this makes the empty one serialise as `[]` rather than as `null`,
	// which is what the column's NOT NULL means and what a screen renders as
	// *the ranker found nothing against this* rather than as a blank.
	supporting, err := marshalEvidence(h.SupportingEvidence)
	if err != nil {
		return fmt.Errorf("hypothesis %s: supporting_evidence: %w", h.HypothesisId, err)
	}
	contradicting, err := marshalEvidence(h.ContradictingEvidence)
	if err != nil {
		return fmt.Errorf("hypothesis %s: contradicting_evidence: %w", h.HypothesisId, err)
	}
	// The basis, as counts (Q-46). **The two numbers are taken from the slices
	// that are being written in the same statement**, not from anything the
	// ranker reported about them, so the column cannot disagree with the arrays
	// it describes — which is the whole of what makes a count checkable where a
	// sentence is not. The tier is read from §9.5's table by cause class, and a
	// hypothesis naming no class gets `none`, the tier that ranks last.
	basis, err := json.Marshal(audit.HypothesisBasis{
		SupportingCount:    len(h.SupportingEvidence),
		ContradictingCount: len(h.ContradictingEvidence),
		Evidenceability:    string(rca.EvidenceabilityOf(h.CauseCategory)),
	})
	if err != nil {
		return fmt.Errorf("hypothesis %s: basis: %w", h.HypothesisId, err)
	}
	if _, err := db.Exec(ctx, insertHypothesisSQL,
		h.HypothesisId, h.IncidentRef, h.Cause, h.CauseCategory,
		h.Confidence, h.Rank, supporting, contradicting,
		h.Locus, basis); err != nil {
		return fmt.Errorf("while writing hypothesis %s: %w", h.HypothesisId, err)
	}
	return nil
}

// marshalEvidence renders one side of a hypothesis's case as the jsonb the
// column holds.
//
// **It converts to audit.Evidence rather than marshalling rca.Evidence, and
// that is a defect avoided rather than a tidiness.** The two types are the same
// three fields in two packages — the ranker's and the reader's — and only the
// reader's carries the wire contract: `statement`, `source`, `source_ref`.
// rca.Evidence has no struct tags at all, so marshalling it directly writes
// `Statement`, `Source` and `SourceRef`. encoding/json matches the first two back
// case-insensitively and **silently drops the third**, because `SourceRef` and
// `source_ref` differ by a character that case folding does not reach. The
// column would hold evidence whose reference into the record was gone, the
// screen would render a statement with no source ref, and every layer would
// report success. TestEvidenceSurvivesTheRoundTripByItsWireNames is the negative
// control, and it fails when this conversion is removed.
//
// A nil slice becomes `[]`, never `null`: the column is NOT NULL and audit's
// decode turns an empty array back into an empty slice, so the round trip
// preserves *asked, and there are none*.
func marshalEvidence(items []rca.Evidence) ([]byte, error) {
	out := make([]audit.Evidence, 0, len(items))
	for _, e := range items {
		out = append(out, audit.Evidence{
			Statement: e.Statement, Source: e.Source, SourceRef: e.SourceRef,
		})
	}
	return json.Marshal(out)
}

// raisedLoci returns the loci this session already has an incident for, keyed by
// locus. It is what makes a second run over a session add rather than duplicate.
func raisedLoci(ctx context.Context, db DB, sessionId string) (map[string]string, error) {
	rows, err := db.Query(ctx,
		`SELECT incident_locus, incident_id FROM jetsapi.incident WHERE incident_session_id = $1`,
		sessionId)
	if err != nil {
		return nil, fmt.Errorf("while reading the incidents already raised for session %s: %w", sessionId, err)
	}
	defer rows.Close()
	out := map[string]string{}
	for rows.Next() {
		var locus, id string
		if err := rows.Scan(&locus, &id); err != nil {
			return nil, fmt.Errorf("while scanning a raised locus: %w", err)
		}
		out[locus] = id
	}
	return out, rows.Err()
}

// RecentSessions returns the run headers that started within the window, newest
// first, as session ids.
//
// **It selects on the header rather than on the worker rows**, which is a choice
// with a consequence: locus `run_not_started` is a terminal header with no worker
// row at all, so a selector keyed on worker rows could never reach the one run
// shape the first locus exists for. The cost is that a header whose worker rows
// have been purged is selected too, which is F273's ambiguity — and the
// classifier answers `not_evaluable` there rather than `present`, so the cost is
// a wasted read rather than a wrong incident.
func RecentSessions(ctx context.Context, db DB, since time.Time, client, process string, limit int) ([]string, error) {
	if limit <= 0 {
		limit = 200
	}
	rows, err := db.Query(ctx,
		`SELECT session_id FROM jetsapi.pipeline_execution_status
		  WHERE start_time >= $1
		    AND ($2 = '' OR client = $2)
		    AND ($3 = '' OR process_name = $3)
		  ORDER BY start_time DESC, key DESC
		  LIMIT $4`, since, client, process, limit)
	if err != nil {
		return nil, fmt.Errorf("while listing recent sessions: %w", err)
	}
	defer rows.Close()
	var out []string
	for rows.Next() {
		var s string
		if err := rows.Scan(&s); err != nil {
			return nil, fmt.Errorf("while scanning a session id: %w", err)
		}
		out = append(out, s)
	}
	return out, rows.Err()
}

// IncidentIdFor is the deterministic key of one (session, locus).
//
// **Deterministic rather than minted**, so that idempotence is a primary-key
// property instead of a read-then-write race: two shadow runs over one session,
// concurrent or not, produce one incident per locus. It is legible rather than a
// hash because it is the identifier an operator quotes back at you out of a
// governance record.
func IncidentIdFor(sessionId, locus string) string {
	return fmt.Sprintf("inc_%s_%s", sessionId, locus)
}

// HypothesisIdFor is the deterministic key of one (incident, cause class). The
// ranker emits at most one hypothesis per pair by construction (rca.ClassesFor),
// so this is unique without a counter — and it is stable across generations,
// where a rank is not.
func HypothesisIdFor(incidentId, causeCategory string) string {
	return fmt.Sprintf("hyp_%s_%s", incidentId, causeCategory)
}
