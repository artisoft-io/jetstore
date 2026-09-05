package shadow

import (
	"context"
	"strings"
	"testing"

	"github.com/artisoft-io/jetstore/jets/agentic/audit"
)

// TestTheRecordShowsNothingActed is criterion 47's second clause, and its four
// negative controls are the reason it is worth anything.
//
// **A negative that has never been seen to fail is a negative nobody has
// tested.** So each of the four ways the attestation could be true-by-accident is
// produced deliberately and the attestation is required to notice: a remediation
// row, an incident moved into an acting status through the ordinary transition
// path, a status changed behind the transition record's back, and an agent
// reaching one of the three verdicts that are a person's.
func TestTheRecordShowsNothingActed(t *testing.T) {
	pool := freshDB(t, "shadow_attest", migratedTables, true)
	ctx := context.Background()
	multiLocusSession(t, pool, "s-att")

	w := testWriter()
	res, err := w.Run(ctx, pool, "s-att")
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if len(res.Written) == 0 {
		t.Fatal("nothing was written, so an attestation over this database would prove nothing")
	}

	a, err := Attest(ctx, pool)
	if err != nil {
		t.Fatalf("Attest: %v", err)
	}
	if !a.Holds() {
		t.Fatalf("the attestation does not hold after an ordinary shadow run:\n%s", a.Report())
	}
	if a.Incidents == 0 || a.Transitions == 0 {
		t.Errorf("the attestation reports %d incidents and %d transitions; a negative over an empty "+
			"record is not evidence about the wiring", a.Incidents, a.Transitions)
	}
	t.Logf("%s", a.Report())

	// It says what it is worth as well as what it found.
	if !strings.Contains(a.Report(), "not tamper-evident") {
		t.Error("the report does not carry R-44's bound, so a reader would take it for more than it is")
	}

	t.Run("a remediation row breaks it", func(t *testing.T) {
		if _, err := pool.Exec(ctx, `INSERT INTO jetsapi.remediation
			(remediation_id, incident_ref, autonomy_tier_required, reversibility, remediation_approval_state)
			VALUES ('rem_control', $1, 'T1', 'fully_reversible', 'draft')`,
			res.Written[0].IncidentId); err != nil {
			t.Fatalf("inserting the control remediation: %v", err)
		}
		defer pool.Exec(ctx, `DELETE FROM jetsapi.remediation WHERE remediation_id = 'rem_control'`)
		got, err := Attest(ctx, pool)
		if err != nil {
			t.Fatalf("Attest: %v", err)
		}
		if got.Holds() {
			t.Errorf("the attestation holds with a remediation in the table:\n%s", got.Report())
		}
		if got.Remediations != 1 {
			t.Errorf("Remediations is %d, want 1", got.Remediations)
		}
	})

	t.Run("an incident moved into an acting status breaks it", func(t *testing.T) {
		var target string
		for _, wr := range res.Written {
			if wr.Status == audit.IncidentDiagnosed {
				target = wr.IncidentId
			}
		}
		if target == "" {
			t.Skip("no incident reached `diagnosed`, so there is nothing to move")
		}
		// Through the ordinary path, which permits it — the ceiling is this
		// package's and not the audit store's (I-299).
		if _, err := audit.RecordIncidentTransition(ctx, pool, &audit.IncidentTransition{
			IncidentEventId: "ie_control_acting", IncidentRef: target,
			FromStatus: audit.IncidentDiagnosed, ToStatus: ActingFrontier,
			Actor: "phase5@1", ActorKind: audit.ActorAgent,
		}); err != nil {
			t.Fatalf("moving the control incident: %v", err)
		}
		got, err := Attest(ctx, pool)
		if err != nil {
			t.Fatalf("Attest: %v", err)
		}
		if got.Holds() {
			t.Errorf("the attestation holds with an incident at %s:\n%s", ActingFrontier, got.Report())
		}
		if got.TransitionsIntoActing != 1 || got.IncidentsAtActingStatus != 1 {
			t.Errorf("TransitionsIntoActing=%d IncidentsAtActingStatus=%d, want 1 and 1",
				got.TransitionsIntoActing, got.IncidentsAtActingStatus)
		}

		// **And the history outlives the row**, which is why the count that
		// matters is asked of incident_event. Move it on to a status that is not
		// acting: the current-status count returns to zero and the historical one
		// does not, so *is it acting now* and *has it ever acted* give different
		// answers — which is the whole reason both are reported.
		if _, err := audit.RecordIncidentTransition(ctx, pool, &audit.IncidentTransition{
			IncidentEventId: "ie_control_back", IncidentRef: target,
			FromStatus: ActingFrontier, ToStatus: audit.IncidentTriaged,
			Actor: "phase5@1", ActorKind: audit.ActorAgent,
		}); err != nil {
			t.Fatalf("moving the control incident back: %v", err)
		}
		back, err := Attest(ctx, pool)
		if err != nil {
			t.Fatalf("Attest: %v", err)
		}
		if back.IncidentsAtActingStatus != 0 {
			t.Errorf("IncidentsAtActingStatus is %d after moving back", back.IncidentsAtActingStatus)
		}
		if back.TransitionsIntoActing != 1 {
			t.Errorf("TransitionsIntoActing is %d after moving back; the event record is the one that "+
				"answers *has it ever*", back.TransitionsIntoActing)
		}
		if back.Holds() {
			t.Errorf("the attestation holds again once the incident moved out of the acting status; "+
				"the mutable column would say nothing acted and the history says otherwise:\n%s",
				back.Report())
		}
	})

	t.Run("a status changed behind the record breaks it", func(t *testing.T) {
		pool2 := freshDB(t, "shadow_attest_oob", migratedTables, true)
		multiLocusSession(t, pool2, "s-oob")
		w2 := testWriter()
		r2, err := w2.Run(ctx, pool2, "s-oob")
		if err != nil {
			t.Fatalf("Run: %v", err)
		}
		if got, err := Attest(ctx, pool2); err != nil || !got.Holds() {
			t.Fatalf("the control database does not start clean: %v\n%s", err, got.Report())
		}
		// The UPDATE the transition record cannot see. jetsapi.incident is
		// mutable by design — status walks A.5's machine — so this is what an
		// executor bypassing the write path would look like.
		if _, err := pool2.Exec(ctx,
			`UPDATE jetsapi.incident SET status = 'remediating' WHERE incident_id = $1`,
			r2.Written[0].IncidentId); err != nil {
			t.Fatalf("the out-of-band update: %v", err)
		}
		got, err := Attest(ctx, pool2)
		if err != nil {
			t.Fatalf("Attest: %v", err)
		}
		if got.Holds() {
			t.Errorf("the attestation holds with a status nothing transitioned into:\n%s", got.Report())
		}
		if got.UnexplainedStatuses != 1 {
			t.Errorf("UnexplainedStatuses is %d, want 1: an incident whose status is not its last "+
				"event's to_status is the case that makes the history incomplete", got.UnexplainedStatuses)
		}
	})

	t.Run("an agent adjudicating its own work breaks it", func(t *testing.T) {
		pool3 := freshDB(t, "shadow_attest_adj", migratedTables, true)
		multiLocusSession(t, pool3, "s-adj")
		w3 := testWriter()
		r3, err := w3.Run(ctx, pool3, "s-adj")
		if err != nil {
			t.Fatalf("Run: %v", err)
		}
		var target string
		for _, wr := range r3.Written {
			if wr.Status == audit.IncidentTriaged || wr.Status == audit.IncidentDiagnosed {
				target = wr.IncidentId
			}
		}
		// triaged -> suppressed_as_benign is one of A.5's three adjudications and
		// is exactly the verdict plan §10.7 says the measured system must not
		// reach for itself.
		inc, err := audit.ReadIncident(ctx, pool3, target, audit.RedactPHI)
		if err != nil {
			t.Fatalf("ReadIncident: %v", err)
		}
		if inc.Status != audit.IncidentTriaged {
			t.Skipf("the target incident is at %s; this control needs one at %s",
				inc.Status, audit.IncidentTriaged)
		}
		if _, err := audit.RecordIncidentTransition(ctx, pool3, &audit.IncidentTransition{
			IncidentEventId: "ie_control_adj", IncidentRef: target,
			FromStatus: audit.IncidentTriaged, ToStatus: audit.IncidentSuppressedAsBenign,
			Actor: "triage@1", ActorKind: audit.ActorAgent,
		}); err != nil {
			t.Fatalf("the agent adjudication: %v", err)
		}
		got, err := Attest(ctx, pool3)
		if err != nil {
			t.Fatalf("Attest: %v", err)
		}
		if got.Holds() {
			t.Errorf("the attestation holds with an agent's own verdict on its own work:\n%s", got.Report())
		}
		if got.AdjudicationsByAgent != 1 {
			t.Errorf("AdjudicationsByAgent is %d, want 1", got.AdjudicationsByAgent)
		}
		// And HumanVerdicts, the labelled population, is still empty — which is
		// the point of the control: an agent verdict must not become a label.
		labels, err := audit.HumanVerdicts(ctx, pool3, fixtureNow.AddDate(0, 0, -1))
		if err != nil {
			t.Fatalf("HumanVerdicts: %v", err)
		}
		if len(labels) != 0 {
			t.Errorf("HumanVerdicts returned %d rows for a verdict an agent reached", len(labels))
		}
	})
}

// TestAnAttestationOverAnUnmigratedDatabaseIsNotEvaluable is §13.4's three-valued
// discipline arriving at the attestation: *checked and none* and *could not
// check* are different answers, and a proof that a table is empty is not
// available where the table does not exist.
func TestAnAttestationOverAnUnmigratedDatabaseIsNotEvaluable(t *testing.T) {
	pool := freshDB(t, "shadow_attest_unmig", unmigratedTables, false)
	ctx := context.Background()

	a, err := Attest(ctx, pool)
	if err != nil {
		t.Fatalf("Attest must answer on a database with none of its tables: %v", err)
	}
	if a.Holds() {
		t.Error("the attestation holds over a database where jetsapi.remediation does not exist; " +
			"that reports the migration rather than the posture")
	}
	report := a.Report()
	if !strings.Contains(report, "not evaluable") || !strings.Contains(report, "jetsapi.remediation") {
		t.Errorf("the report does not say what it could not check:\n%s", report)
	}
	t.Logf("%s", report)
}
