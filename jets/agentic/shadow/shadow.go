// Package shadow is Phase 4's deployment posture as code (task AC.3): triage
// and RCA run over the execution record, their conclusions are written to
// jetsapi.incident and jetsapi.hypothesis, a supervision screen reads them, and
// **nothing acts**.
//
// # The hard half of criterion 47 is the second clause
//
// The criterion reads *"Nothing acts: no remediation executes, and the audit
// record shows it."* The first clause is cheap — this repository contains no
// executor, so nothing could have executed — and the second is not. *No
// remediation ran* and *the record demonstrates that no remediation ran* are
// different claims, and a phase that satisfies the first by writing no code has
// satisfied nothing a reader can check.
//
// So this package builds the demonstration rather than the absence, in four
// parts, each of which fails loudly when it stops being true:
//
//   - **The ceiling is derived, not declared.** ActingStatuses is asserted to be
//     exactly the set of IncidentStatus values that no path from `detected`
//     reaches without passing through `remediation_proposed` — a property of
//     Appendix A.5's own machine, computed from audit.IncidentTransitions rather
//     than from an opinion held here. TestActingStatusesAreTheUnreachableHalf is
//     what recomputes it.
//   - **The writer refuses to cross it.** Writer.Run writes only the three
//     statuses of ShadowStatuses, and moveTo returns ErrWouldAct for anything
//     else. That is a guard on a code path rather than a discipline, and a
//     negative control watches it fire.
//   - **No writer for jetsapi.remediation exists in the tree**, asserted over
//     the source by TestNothingWritesTheRemediationTable. AB.2 tabled the entity
//     with no executor *so that Phase 5 has something to gate*; that absence is
//     therefore a property worth asserting rather than a fact about what nobody
//     got round to.
//   - **The record answers for itself.** Attest reads the incident record back
//     and reports what it holds: how many incidents, how many transitions, how
//     many of those ever named an acting status, how many remediations exist,
//     and whether every incident's current status is explained by its own
//     transition history. See attest.go for what that last one is for and for
//     the two bounds on all of it.
//
// # What the demonstration is worth, stated with it
//
// **jetsapi.incident_event has no append-only trigger.** Unlike jetsapi.agent_audit
// it can be deleted from, and unlike jetsapi.anomaly its subject is mutable. The
// hash chain covers a transition only when the incident names an AgentRun (AB.4),
// and **a deterministic classifier is not an AgentRun** — so every incident this
// package writes is in the unchained half, which is R-44 and gap 27's accepted
// cost rather than an oversight. Q-43 asked whether such an incident should be
// chainable at all and the obvious answer — mint an AgentRun for the triage step
// — was refused twice (Q-32, Q-43); it is not worked around here.
//
// The honest form of the claim is therefore: *the record, as it stands, shows
// that no incident has ever been moved into an acting status and that no
// remediation exists* — durable, attributable and reproducible, and
// tamper-evident only for the incidents an agent run raised, of which there are
// none.
//
// # What this package does not do
//
// It does not decide a cause. AC.2's ranker attaches hypotheses and its
// contradicting evidence is what makes them advisory; nothing here reads a model.
// It does not adjudicate: `reclassified`, `verified` and `suppressed_as_benign`
// are human verdicts and plan §10.7's whole argument is that a label the measured
// system wrote is not a label, so this writer is refused those three by the same
// guard that refuses it the acting half.
package shadow

import (
	"context"
	"fmt"
	"slices"

	"github.com/artisoft-io/jetstore/jets/agentic/audit"
	"github.com/artisoft-io/jetstore/jets/agentic/observe"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

// DB is the slice of pgx this package needs, satisfied by *pgxpool.Pool and
// *pgx.Conn. Begin is here because RecordIncidentTransition wants a transaction
// of its own; the reads are observe.DB's two methods.
type DB interface {
	Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
	Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error)
	Begin(ctx context.Context) (pgx.Tx, error)
}

// ShadowStatuses are the three IncidentStatus values this wiring may write, in
// the order it walks them.
//
// **`detected` is written and then left immediately**, rather than the incident
// being inserted at `triaged`. The transition is the record of the classifier
// having run, and an incident that arrived already triaged would have a verdict
// and no event saying who reached it — which is exactly the gap I-276 was raised
// for, reintroduced by the first writer.
var ShadowStatuses = []string{
	audit.IncidentDetected, audit.IncidentTriaged, audit.IncidentDiagnosed,
}

// ActingStatuses are the six statuses on the far side of the ceiling.
//
// **They are not chosen here.** Appendix A.5's Incident machine makes
// `remediation_proposed` an articulation point: every one of these is reachable
// from `detected` only through it, and none of `triaged`, `diagnosed`,
// `reclassified` or `suppressed_as_benign` is. That is what *acting* means in
// this phase — the point at which a corrective action has a subject — and
// TestActingStatusesAreTheUnreachableHalf recomputes the set from
// audit.IncidentTransitions on every run, so an edge added to A.5 moves this
// list or fails the suite.
var ActingStatuses = []string{
	audit.IncidentRemediationProposed,
	audit.IncidentAwaitingApproval,
	audit.IncidentRemediating,
	audit.IncidentResolved,
	audit.IncidentVerified,
	audit.IncidentClosed,
}

// ActingFrontier is the status the ceiling sits under: the first one at which a
// remediation has been proposed for an incident.
const ActingFrontier = audit.IncidentRemediationProposed

// AdjudicationStatuses are the three verdicts a *person* reaches, which this
// writer is refused for a different reason than the acting six: they are labels,
// and plan §10.7's argument is that a label the measured system may have written
// is not a label. They are the population HumanVerdicts counts.
var AdjudicationStatuses = []string{
	audit.IncidentVerified, audit.IncidentReclassified, audit.IncidentSuppressedAsBenign,
}

// IsActingStatus reports whether a status is on the far side of the ceiling.
func IsActingStatus(s string) bool { return slices.Contains(ActingStatuses, s) }

// IsShadowStatus reports whether this wiring may write a status.
func IsShadowStatus(s string) bool { return slices.Contains(ShadowStatuses, s) }

// ErrWouldAct is returned when something asks this wiring to move an incident
// past the ceiling. It is a distinct type because it is the one error in this
// package that means criterion 47 was about to be broken rather than that a
// database was unhappy, and a caller that swallowed it would be swallowing the
// phase's deployment posture.
type ErrWouldAct struct {
	IncidentRef string
	ToStatus    string
}

func (e *ErrWouldAct) Error() string {
	return fmt.Sprintf("refusing to move incident %s to %q: shadow mode writes only %v, and %q is on the "+
		"acting side of Appendix A.5's machine — every status reachable from `detected` only through %q. "+
		"Nothing in this phase executes a remediation (criterion 47)",
		e.IncidentRef, e.ToStatus, ShadowStatuses, e.ToStatus, ActingFrontier)
}

// The five members of the Severity vocabulary, mirroring incident_severity_ck.
// Named here rather than in audit because this is the package that has to pick
// one; see Writer.Severity for why the pick is a posture and not a measurement.
const (
	SeverityInfo     = "info"
	SeverityLow      = "low"
	SeverityMedium   = "medium"
	SeverityHigh     = "high"
	SeverityCritical = "critical"
)

// Severities is the vocabulary, in the CHECK's order.
var Severities = []string{SeverityInfo, SeverityLow, SeverityMedium, SeverityHigh, SeverityCritical}

// ModelVersion is the domain model version written on every incident.
//
// **It is transcribed by hand and nothing checks it.** The value lives at
// MODEL_VERSION in tools/jets_agentic/jets_agentic/model.py and reaches the .jr,
// the sidecar, the JSON Schema and the DDL header from there; **no Go constant
// is generated for it**, so every Go caller that needs one types the string —
// four test files do today. That is F68's two-files-and-nothing-compares-them
// shape at the granularity of a version string, and it is recorded rather than
// fixed here: generating it belongs with the generator that already emits
// DataClassifiedProperties, not with the first caller to want it.
const ModelVersion = "0.1.0"

// Targets is which of the four tables this wiring writes and reads exist in a
// database.
//
// **It is separate from triage.Extent on purpose.** That type answers *what can
// be read about a run*; this answers *is there anywhere to put a conclusion*,
// and the two go wrong independently: jetsapi.incident arrived at AB.1 and
// jetsapi.anomaly at N.2, so a deployment migrated between them has one and not
// the other. Both ride audit.InstallSchema on `update_db -migrateDb`
// (jets/update_db/main.go:71), which is the precondition I-169 recorded for one
// table and which now covers four.
type Targets struct {
	Incident      bool
	Hypothesis    bool
	IncidentEvent bool
	// Remediation is read for the attestation rather than for the writer:
	// **nothing writes that table** (criterion 47), and an attestation that
	// reported *zero remediations* against a database where the table does not
	// exist would be reporting the migration rather than the posture.
	Remediation bool
}

// Ready reports whether the three tables the writer needs are present.
func (t *Targets) Ready() bool { return t.Incident && t.Hypothesis && t.IncidentEvent }

// Missing names the tables that are absent, for an error a deployer can act on.
func (t *Targets) Missing() []string {
	var out []string
	for _, p := range []struct {
		name string
		have bool
	}{
		{"jetsapi.incident", t.Incident},
		{"jetsapi.hypothesis", t.Hypothesis},
		{"jetsapi.incident_event", t.IncidentEvent},
		{"jetsapi.remediation", t.Remediation},
	} {
		if !p.have {
			out = append(out, p.name)
		}
	}
	return out
}

const targetsSQL = `SELECT
  to_regclass('jetsapi.incident') IS NOT NULL,
  to_regclass('jetsapi.hypothesis') IS NOT NULL,
  to_regclass('jetsapi.incident_event') IS NOT NULL,
  to_regclass('jetsapi.remediation') IS NOT NULL`

// ReadTargets reports which of the four tables exist.
//
// to_regclass returns NULL rather than raising, which is the property F107
// established is required of any report about a deployment: a precondition check
// that fails on the missing relation it is checking for reports nothing at all.
func ReadTargets(ctx context.Context, db observe.DB) (*Targets, error) {
	var t Targets
	if err := db.QueryRow(ctx, targetsSQL).Scan(
		&t.Incident, &t.Hypothesis, &t.IncidentEvent, &t.Remediation); err != nil {
		return nil, fmt.Errorf("while reading which agentic tables exist: %w", err)
	}
	return &t, nil
}
