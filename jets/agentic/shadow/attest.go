package shadow

import (
	"context"
	"fmt"
	"strings"

	"github.com/artisoft-io/jetstore/jets/agentic/observe"
)

// Attest reads the incident record back and reports what it shows about whether
// anything acted. It is criterion 47's second clause as a query.
//
// # Why a query rather than a stored claim
//
// The obvious way to satisfy *the audit record shows it* is to write a row
// saying so. That would be an assertion by the party being asked about, which is
// the same objection plan §10.7 makes to a label the measured system wrote. What
// this does instead is **derive** the answer from rows nothing here authored for
// the purpose: the incident's own transition history, the remediation table's
// contents, and the agreement between the two. A derivation can be re-run by
// somebody who does not trust the deriver; a stored claim cannot.
//
// # The four questions, and why the third is the one worth having
//
//   - How many incidents and transitions exist at all. A proof of a negative
//     over an empty database proves nothing about the wiring, so the population
//     is reported with the negatives rather than left to be asked for.
//   - Are there any remediations. jetsapi.remediation was tabled by AB.2 with no
//     executor; a row in it would mean something proposed an action.
//   - **Has any incident ever been moved into an acting status.** This is asked
//     of jetsapi.incident_event and not of jetsapi.incident.status, and the
//     difference is the whole value of the answer: the status column is mutable
//     and holds one value, so it says where an incident is *now* and an incident
//     that had been remediated and then closed would answer `closed`. The event
//     table is written once per transition and never updated, so it answers *has
//     it ever been*, which is the question criterion 47 asks.
//   - **Is every incident's current status explained by its own history.** An
//     incident is either at `detected` with no events, or its status is the
//     `to_status` of its latest event. Anything else is a status that arrived
//     without a transition — an out-of-band UPDATE — and it is the one way the
//     third answer above could be true and misleading at once. Reporting it is
//     what turns *no event names an acting status* into *no incident reached one*.
//
// # Two bounds, stated here rather than left to be discovered
//
// **jetsapi.incident_event has no append-only trigger.** The DDL says so in
// terms: unlike jetsapi.agent_audit it is not trigger-protected, so a row can be
// deleted. The hash chain covers a transition only where the incident names an
// AgentRun (AB.4), and a deterministic classifier is not one — so every incident
// this package writes is outside it (R-44, gap 27). What the attestation
// establishes is therefore a property of the record **as it stands**, not a
// proof that nothing was removed from it.
//
// **And an absent table is not an empty one.** On a database that has never run
// `update_db -migrateDb` since AB.2, jetsapi.remediation does not exist, and
// "zero remediations" there would be reporting the migration rather than the
// posture. Attestation.Holds is false in that state and Report says which table
// was missing, which is AC.1's three-valued discipline arriving at the
// attestation: *checked and none* and *could not check* are different answers.
type Attestation struct {
	Targets *Targets

	Incidents   int64
	Transitions int64
	// IncidentsByStatus is the current-status census, for the population figure
	// a reader wants beside the negatives.
	IncidentsByStatus map[string]int64

	// Remediations is the row count of jetsapi.remediation. Zero is the claim;
	// see Targets for what a missing table does to it.
	Remediations int64
	// TransitionsIntoActing counts incident_event rows whose to_status is one of
	// ActingStatuses — **ever**, not currently.
	TransitionsIntoActing int64
	// IncidentsAtActingStatus counts incidents whose current status is acting.
	// Redundant with the line above where the history is complete, and that
	// redundancy is the point: the two disagreeing is the signal.
	IncidentsAtActingStatus int64
	// UnexplainedStatuses counts incidents whose current status is neither
	// `detected` with no events nor the to_status of their latest event.
	UnexplainedStatuses int64
	// AdjudicationsByAgent counts transitions into one of the three human
	// verdicts that an agent made. Not an acting failure — nothing executes —
	// but it would mean the system had labelled itself, which is the other thing
	// this wiring is refused (plan §10.7).
	AdjudicationsByAgent int64
}

// Holds reports whether the record demonstrates that nothing acted.
//
// It requires the tables to be present: an attestation over a database where the
// subject does not exist is not evaluable, and returning true for it would be
// the boolean-predicate mistake §13.4 is written against.
func (a *Attestation) Holds() bool {
	return a.Targets != nil && a.Targets.Ready() && a.Targets.Remediation &&
		a.Remediations == 0 &&
		a.TransitionsIntoActing == 0 &&
		a.IncidentsAtActingStatus == 0 &&
		a.UnexplainedStatuses == 0 &&
		a.AdjudicationsByAgent == 0
}

// Report is the attestation in the words an operator reads, and it is what a
// reader of criterion 47 is meant to be able to point at.
func (a *Attestation) Report() string {
	var b strings.Builder
	if a.Targets == nil || !a.Targets.Ready() || !a.Targets.Remediation {
		missing := []string{"(unknown)"}
		if a.Targets != nil {
			missing = a.Targets.Missing()
		}
		fmt.Fprintf(&b, "not evaluable: %v absent from this database, so there is nothing to attest over. "+
			"`update_db -migrateDb` is what installs them.\n", missing)
		return b.String()
	}
	verdict := "holds"
	if !a.Holds() {
		verdict = "DOES NOT HOLD"
	}
	fmt.Fprintf(&b, "shadow-mode attestation: %s\n", verdict)
	fmt.Fprintf(&b, "  population: %d incidents, %d recorded transitions\n", a.Incidents, a.Transitions)
	for _, s := range append(append([]string{}, ShadowStatuses...), ActingStatuses...) {
		if n := a.IncidentsByStatus[s]; n > 0 {
			fmt.Fprintf(&b, "      %-22s %d\n", s, n)
		}
	}
	fmt.Fprintf(&b, "  no remediation exists:                  %d rows in jetsapi.remediation\n", a.Remediations)
	fmt.Fprintf(&b, "  no incident ever entered an acting status: %d transitions into %v\n",
		a.TransitionsIntoActing, ActingStatuses)
	fmt.Fprintf(&b, "  no incident is at one now:              %d\n", a.IncidentsAtActingStatus)
	fmt.Fprintf(&b, "  every status is explained by its own history: %d unexplained\n", a.UnexplainedStatuses)
	fmt.Fprintf(&b, "  no agent adjudicated its own work:      %d agent transitions into %v\n",
		a.AdjudicationsByAgent, AdjudicationStatuses)
	b.WriteString("  bound: jetsapi.incident_event has no append-only trigger, and a transition is " +
		"hash-chained only where the incident names an AgentRun (AB.4). A deterministic classifier is " +
		"not one, so these rows are durable and attributable and are not tamper-evident (R-44).\n")
	return b.String()
}

const attestSQL = `SELECT
  (SELECT count(*) FROM jetsapi.incident),
  (SELECT count(*) FROM jetsapi.incident_event),
  (SELECT count(*) FROM jetsapi.remediation),
  (SELECT count(*) FROM jetsapi.incident_event WHERE to_status = ANY($1::text[])),
  (SELECT count(*) FROM jetsapi.incident WHERE status = ANY($1::text[])),
  (SELECT count(*) FROM jetsapi.incident_event
     WHERE event_actor_kind = 'agent' AND to_status = ANY($2::text[])),
  (SELECT count(*) FROM jetsapi.incident i
     LEFT JOIN LATERAL (
       SELECT e.to_status
         FROM jetsapi.incident_event e
        WHERE e.event_incident_ref = i.incident_id
        ORDER BY e.transitioned_at DESC, e.incident_event_id DESC
        LIMIT 1) last ON true
    WHERE CASE WHEN last.to_status IS NULL THEN i.status <> 'detected'
               ELSE i.status <> last.to_status END)`

// Attest computes the attestation. It reads five tables and writes nothing.
func Attest(ctx context.Context, db observe.DB) (*Attestation, error) {
	targets, err := ReadTargets(ctx, db)
	if err != nil {
		return nil, err
	}
	a := &Attestation{Targets: targets, IncidentsByStatus: map[string]int64{}}
	if !targets.Ready() || !targets.Remediation {
		return a, nil
	}
	if err := db.QueryRow(ctx, attestSQL, ActingStatuses, AdjudicationStatuses).Scan(
		&a.Incidents, &a.Transitions, &a.Remediations,
		&a.TransitionsIntoActing, &a.IncidentsAtActingStatus,
		&a.AdjudicationsByAgent, &a.UnexplainedStatuses); err != nil {
		return nil, fmt.Errorf("while attesting that nothing acted: %w", err)
	}
	rows, err := db.Query(ctx, `SELECT status, count(*) FROM jetsapi.incident GROUP BY status`)
	if err != nil {
		return nil, fmt.Errorf("while counting incidents by status: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		var s string
		var n int64
		if err := rows.Scan(&s, &n); err != nil {
			return nil, fmt.Errorf("while scanning the status census: %w", err)
		}
		a.IncidentsByStatus[s] = n
	}
	return a, rows.Err()
}
