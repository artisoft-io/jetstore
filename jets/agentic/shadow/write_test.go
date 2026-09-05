package shadow

import (
	"context"
	"errors"
	"slices"
	"strings"
	"testing"

	"github.com/artisoft-io/jetstore/jets/agentic/audit"
	"github.com/artisoft-io/jetstore/jets/agentic/rca"
	"github.com/artisoft-io/jetstore/jets/agentic/triage"
)

// TestTriageWritesAnIncidentPerLocusWithItsHypotheses is the whole of AC.3's
// forward path against a real database, and it is the first time in this project
// that anything other than a test fixture has written jetsapi.incident (F288).
func TestTriageWritesAnIncidentPerLocusWithItsHypotheses(t *testing.T) {
	pool := freshDB(t, "shadow_write", migratedTables, true)
	ctx := context.Background()
	multiLocusSession(t, pool, "s-multi")

	w := testWriter()
	res, err := w.Run(ctx, pool, "s-multi")
	if err != nil {
		t.Fatalf("Run: %v", err)
	}

	// The report carries all nine verdicts, not the hits — the denominator
	// criterion 44 asks for survives the wiring.
	if len(res.Report.Findings) != len(triage.Loci) {
		t.Errorf("the result carries %d findings; a per-class denominator needs all %d",
			len(res.Report.Findings), len(triage.Loci))
	}
	fired := res.Report.Loci()
	if len(fired) < 2 {
		t.Fatalf("the fixture fired %d loci; it was built to fire several: %v", len(fired), fired)
	}
	if len(res.Written) != len(fired) {
		t.Errorf("%d loci fired and %d incidents were written; the rule is one per (session, locus)",
			len(fired), len(res.Written))
	}
	t.Logf("session s-multi: %d loci present %v, %d incidents, %d hypotheses over all of them",
		len(fired), fired, len(res.Written), len(res.Ranking.Hypotheses))

	list, err := audit.ListIncidents(ctx, pool, nil, 0)
	if err != nil {
		t.Fatalf("ListIncidents: %v", err)
	}
	if len(list) != len(res.Written) {
		t.Fatalf("wrote %d incidents and read back %d", len(res.Written), len(list))
	}

	seen := map[string]bool{}
	diagnosed := 0
	for _, s := range list {
		if seen[s.Locus] {
			t.Errorf("locus %s has two incidents in one session", s.Locus)
		}
		seen[s.Locus] = true
		if s.SessionId != "s-multi" {
			t.Errorf("incident %s names session %q", s.IncidentId, s.SessionId)
		}
		// The one decision in the write path that could have gone the other way:
		// the ranker names a top class and the incident does not claim it.
		if s.Classification != "" {
			t.Errorf("incident %s claims classification %q; a deterministic triage step evidences a "+
				"locus and never a cause (§9.5, I-289)", s.IncidentId, s.Classification)
		}
		if s.Severity != SeverityInfo {
			t.Errorf("incident %s has severity %q, want %q: shadow mode's severity is a posture and "+
				"no locus determines one (I-306)", s.IncidentId, s.Severity, SeverityInfo)
		}
		if s.RunRef != "" {
			t.Errorf("incident %s names run %q; a deterministic classifier is not an AgentRun (R-44)",
				s.IncidentId, s.RunRef)
		}
		if !IsShadowStatus(s.Status) {
			t.Errorf("incident %s is at %q, which shadow mode may not write", s.IncidentId, s.Status)
		}
		if s.Status == audit.IncidentDiagnosed {
			diagnosed++
			if s.HypothesisCount == 0 {
				t.Errorf("incident %s is diagnosed and carries no hypothesis", s.IncidentId)
			}
		}
		if s.Status == audit.IncidentTriaged && s.HypothesisCount != 0 {
			t.Errorf("incident %s is only triaged and carries %d hypotheses", s.IncidentId, s.HypothesisCount)
		}
		// §9.6's invariant, enforced by CHECK and asserted here so the write path
		// is seen to satisfy it rather than assumed to.
		if s.StepRef != "" {
			found := false
			for _, c := range s.Confounders {
				if c == "step_label_ambiguous" {
					found = true
				}
			}
			if !found {
				t.Errorf("incident %s names step %q without step_label_ambiguous", s.IncidentId, s.StepRef)
			}
		}
	}
	if diagnosed == 0 {
		t.Error("no incident reached `diagnosed`; the ranker produced hypotheses for none of the loci")
	}

	// The hypotheses, read back the way a screen reads them.
	for _, s := range list {
		if s.HypothesisCount == 0 {
			continue
		}
		inc, err := audit.ReadIncident(ctx, pool, s.IncidentId, audit.DisclosePHI)
		if err != nil {
			t.Fatalf("ReadIncident %s: %v", s.IncidentId, err)
		}
		for i, h := range inc.Hypotheses {
			if h.Rank != int64(i+1) {
				t.Errorf("%s hypothesis %d has rank %d; §A.2.8's ranks are 1-based and dense within an incident",
					s.IncidentId, i, h.Rank)
			}
			if len(h.SupportingEvidence) == 0 {
				t.Errorf("%s hypothesis %s has no supporting evidence", s.IncidentId, h.HypothesisId)
			}
			if h.ContradictingEvidence == nil {
				t.Errorf("%s hypothesis %s came back with nil contradicting evidence; the column is NOT NULL "+
					"and an empty array is the assertion §A.2.8 asks for", s.IncidentId, h.HypothesisId)
			}
			for _, e := range append(append([]audit.Evidence{}, h.SupportingEvidence...), h.ContradictingEvidence...) {
				if e.Statement == "" || e.Source == "" || e.SourceRef == "" {
					t.Errorf("%s hypothesis %s carries an evidence item missing a field: %+v",
						s.IncidentId, h.HypothesisId, e)
				}
			}
		}
	}
}

// TestTheBasisReachesTheRecord is criterion 45's last clause meeting the write.
//
// jetsapi.hypothesis has no basis column and no locus column (F402), which §19.7
// grades as *met for what AC.2 emits and not surviving the write*. Two of the
// three losses are repaired by where things are written rather than by a schema
// change, and this asserts both.
func TestTheBasisReachesTheRecord(t *testing.T) {
	pool := freshDB(t, "shadow_basis", migratedTables, true)
	ctx := context.Background()
	multiLocusSession(t, pool, "s-basis")

	w := testWriter()
	res, err := w.Run(ctx, pool, "s-basis")
	if err != nil {
		t.Fatalf("Run: %v", err)
	}

	byLocus := map[string]triage.Finding{}
	for _, f := range res.Report.Findings {
		byLocus[f.Locus] = f
	}
	for _, got := range res.Written {
		ts, err := audit.IncidentTransitionsFor(ctx, pool, got.IncidentId)
		if err != nil {
			t.Fatalf("IncidentTransitionsFor %s: %v", got.IncidentId, err)
		}
		if len(ts) == 0 {
			t.Fatalf("incident %s has no transitions; the classifier having run is not in the record", got.IncidentId)
		}
		if ts[0].FromStatus != audit.IncidentDetected || ts[0].ToStatus != audit.IncidentTriaged {
			t.Errorf("%s's first transition is %s -> %s", got.IncidentId, ts[0].FromStatus, ts[0].ToStatus)
		}
		if ts[0].ActorKind != audit.ActorAgent || ts[0].Actor != "triage@1" {
			t.Errorf("%s's first transition is %q/%q", got.IncidentId, ts[0].Actor, ts[0].ActorKind)
		}
		// The locus verdict's own account of what it compared against.
		if ts[0].Rationale != byLocus[got.Locus].Basis {
			t.Errorf("%s's triage transition does not carry the finding's basis.\n  rationale: %s\n  basis:     %s",
				got.IncidentId, ts[0].Rationale, byLocus[got.Locus].Basis)
		}
		if got.Hypotheses == 0 {
			continue
		}
		last := ts[len(ts)-1]
		if last.ToStatus != audit.IncidentDiagnosed {
			t.Errorf("%s carries %d hypotheses and its last transition is to %s",
				got.IncidentId, got.Hypotheses, last.ToStatus)
		}
		// Criterion 45's last clause, in the database.
		if last.Rationale != res.Ranking.Basis {
			t.Errorf("%s's diagnosis transition does not carry the ranking's basis.\n  rationale: %s",
				got.IncidentId, last.Rationale)
		}
		if !strings.Contains(last.Rationale, "locus") {
			t.Errorf("%s's ranking basis does not mention a locus: %s", got.IncidentId, last.Rationale)
		}
		// Every transition on an incident nothing agentic raised is outside the
		// hash chain. Asserted rather than left to be assumed closed (R-44).
		if last.RunRef != "" {
			t.Errorf("%s's transition names run %q; nothing agentic raised it", got.IncidentId, last.RunRef)
		}
	}

	// The locus survives without a column, by the join HypothesesFor makes.
	for _, got := range res.Written {
		if got.Hypotheses == 0 {
			continue
		}
		hs, err := audit.HypothesesFor(ctx, pool, got.IncidentId, audit.DisclosePHI)
		if err != nil {
			t.Fatalf("HypothesesFor: %v", err)
		}
		for _, h := range hs {
			if !strings.Contains(h.HypothesisId, got.IncidentId) {
				t.Errorf("hypothesis %s does not resolve to incident %s", h.HypothesisId, got.IncidentId)
			}
		}
	}
}

// TestASecondRunAddsAndDoesNotRetract is Q-34, with the "can see less" case
// produced rather than described: the table locus per_record_failures_reported
// reads is dropped between the two runs, so the second classification answers
// not_evaluable where the first answered present.
func TestASecondRunAddsAndDoesNotRetract(t *testing.T) {
	pool := freshDB(t, "shadow_q34", migratedTables, true)
	ctx := context.Background()
	multiLocusSession(t, pool, "s-q34")

	w := testWriter()
	first, err := w.Run(ctx, pool, "s-q34")
	if err != nil {
		t.Fatalf("first Run: %v", err)
	}
	if len(first.Written) == 0 {
		t.Fatal("the first run wrote nothing")
	}
	before, err := audit.ListIncidents(ctx, pool, nil, 0)
	if err != nil {
		t.Fatalf("ListIncidents: %v", err)
	}

	// The deployment now sees less than it did. This is F273's shape produced by
	// a table rather than by a purge, and it is the case the question asks about.
	if _, err := pool.Exec(ctx, `DROP TABLE jetsapi.process_errors`); err != nil {
		t.Fatalf("dropping process_errors: %v", err)
	}
	second, err := w.Run(ctx, pool, "s-q34")
	if err != nil {
		t.Fatalf("second Run: %v", err)
	}
	if v := verdict(second.Report, triage.LocusPerRecordFailuresReported); v != triage.NotEvaluable {
		t.Fatalf("after dropping the table the locus answers %q; the fixture no longer produces Q-34's case", v)
	}
	if len(second.Written) != 0 {
		t.Errorf("the second run wrote %d further incidents: %v", len(second.Written), second.Written)
	}
	if len(second.AlreadyRaised) == 0 {
		t.Error("the second run reported nothing already raised; a caller cannot tell a quiet run from a repeat")
	}

	after, err := audit.ListIncidents(ctx, pool, nil, 0)
	if err != nil {
		t.Fatalf("ListIncidents: %v", err)
	}
	if len(after) != len(before) {
		t.Errorf("the incident count moved from %d to %d across a run that could see less; "+
			"this writer adds and never retracts (Q-34)", len(before), len(after))
	}
	for i := range before {
		if before[i].IncidentId != after[i].IncidentId || before[i].Status != after[i].Status {
			t.Errorf("incident %s changed across the second run: %q -> %q",
				before[i].IncidentId, before[i].Status, after[i].Status)
		}
	}
	t.Logf("Q-34: %d incidents before, %d after a run that answered not_evaluable where the first "+
		"answered present; %d already raised", len(before), len(after), len(second.AlreadyRaised))
}

// TestTheCeilingRefusesToAct is criterion 47's guard, seen to fire.
func TestTheCeilingRefusesToAct(t *testing.T) {
	pool := freshDB(t, "shadow_ceiling", migratedTables, true)
	ctx := context.Background()
	multiLocusSession(t, pool, "s-ceil")

	w := testWriter()
	res, err := w.Run(ctx, pool, "s-ceil")
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	var diagnosed *IncidentWritten
	for i := range res.Written {
		if res.Written[i].Status == audit.IncidentDiagnosed {
			diagnosed = &res.Written[i]
		}
	}
	if diagnosed == nil {
		t.Fatal("no incident reached `diagnosed`, so the ceiling cannot be tested from below it")
	}

	// A.5 permits diagnosed -> remediation_proposed. The lifecycle would allow
	// it and the CHECK would accept it; this package will not.
	if !audit.IncidentTransitionAllowed(audit.IncidentDiagnosed, ActingFrontier) {
		t.Fatalf("A.5 no longer permits %s -> %s, so this test is asserting nothing",
			audit.IncidentDiagnosed, ActingFrontier)
	}
	err = w.moveTo(ctx, pool, diagnosed, audit.IncidentDiagnosed, ActingFrontier, fixtureNow, "because")
	var would *ErrWouldAct
	if !errors.As(err, &would) {
		t.Fatalf("moving an incident to %s returned %v, want an ErrWouldAct", ActingFrontier, err)
	}
	t.Logf("the ceiling fired: %v", err)

	// And the refusal is a refusal rather than a rollback: the incident did not
	// move, so nothing partial reached the record.
	inc, err := audit.ReadIncident(ctx, pool, diagnosed.IncidentId, audit.RedactPHI)
	if err != nil {
		t.Fatalf("ReadIncident: %v", err)
	}
	if inc.Status != audit.IncidentDiagnosed {
		t.Errorf("the incident is at %q after a refused transition", inc.Status)
	}
}

// TestAnUnmigratedDatabaseIsRefusedByName is F107's lesson at the write end.
//
// Every deployment measured on 2026-08-25 is in this state (I-132): the execution
// record is there and the agentic tables are not. Untreated, the first symptom is
// an insert failing on a missing relation, which reads as a defect in the
// classifier rather than as a migration nobody has run.
func TestAnUnmigratedDatabaseIsRefusedByName(t *testing.T) {
	pool := freshDB(t, "shadow_unmigrated", unmigratedTables, false)
	ctx := context.Background()
	failedWorkerSession(t, pool, "s-unmig")

	w := testWriter()
	_, err := w.Run(ctx, pool, "s-unmig")
	if err == nil {
		t.Fatal("shadow mode wrote into a database with none of its tables")
	}
	for _, want := range []string{"jetsapi.incident", "update_db -migrateDb"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("the refusal does not name %q: %v", want, err)
		}
	}

	// A dry run is still useful there, and says so by working.
	w.DryRun = true
	res, err := w.Run(ctx, pool, "s-unmig")
	if err != nil {
		t.Fatalf("a dry run on an unmigrated database should still classify: %v", err)
	}
	if len(res.Written) == 0 {
		t.Error("the dry run reported no incident it would have written")
	}
	list, err := audit.ListIncidents(ctx, pool, nil, 0)
	if err == nil {
		t.Fatalf("the incident table exists after a dry run: %d rows", len(list))
	}
	var missing *audit.ErrTablesNotDeployed
	if !errors.As(err, &missing) {
		t.Errorf("reading incidents on an unmigrated database returned %v, want ErrTablesNotDeployed", err)
	}
}

func verdict(r *triage.Report, locus string) triage.Verdict {
	for i := range r.Findings {
		if r.Findings[i].Locus == locus {
			return r.Findings[i].Verdict
		}
	}
	return ""
}

// TestTheBasisAndLocusAreWritableAndCheckable is Q-46's two columns, answered by
// the user 2026-09-04 and applied before the first row was written.
//
// **Three claims, and the third is the one the columns exist for.** The counts
// on `basis` are the lengths of the two evidence arrays on the same row, so the
// stored confidence can be checked against them without the run. The
// evidenceability tier is §9.5's for the hypothesis's cause class, which is the
// ranker's primary sort key and cannot be recomputed once the gate's table moves.
// And **the locus a hypothesis was raised from is stored, so a hypothesis raised
// at a locus triage did not find present is distinguishable in stored data from
// one raised at a locus it did** — which is `AC.2`'s headline finding (20 of 29
// on its model arm) made detectable rather than persisted invisibly.
func TestTheBasisAndLocusAreWritableAndCheckable(t *testing.T) {
	pool := freshDB(t, "shadow_basis_cols", migratedTables, true)
	ctx := context.Background()
	multiLocusSession(t, pool, "s-cols")

	w := testWriter()
	res, err := w.Run(ctx, pool, "s-cols")
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	present := map[string]bool{}
	for _, l := range res.Report.Loci() {
		present[l] = true
	}

	checked := 0
	for _, got := range res.Written {
		hs, err := audit.HypothesesFor(ctx, pool, got.IncidentId, audit.DisclosePHI)
		if err != nil {
			t.Fatalf("HypothesesFor %s: %v", got.IncidentId, err)
		}
		for _, h := range hs {
			checked++
			// The counts describe the arrays they were written beside.
			if h.Basis.SupportingCount != len(h.SupportingEvidence) {
				t.Errorf("%s: basis says %d supporting and the column holds %d",
					h.HypothesisId, h.Basis.SupportingCount, len(h.SupportingEvidence))
			}
			if h.Basis.ContradictingCount != len(h.ContradictingEvidence) {
				t.Errorf("%s: basis says %d contradicting and the column holds %d",
					h.HypothesisId, h.Basis.ContradictingCount, len(h.ContradictingEvidence))
			}
			// And the confidence the row carries is the ratio those two counts
			// give, which is the whole reason they are numbers.
			total := h.Basis.SupportingCount + h.Basis.ContradictingCount
			if total > 0 {
				want := float64(h.Basis.SupportingCount) / float64(total)
				if diff := h.Confidence - want; diff > 1e-9 || diff < -1e-9 {
					t.Errorf("%s: confidence %v is not %d/%d from its own basis",
						h.HypothesisId, h.Confidence, h.Basis.SupportingCount, total)
				}
			}
			if !slices.Contains(rca.Evidenceabilities, rca.Evidenceability(h.Basis.Evidenceability)) {
				t.Errorf("%s: evidenceability %q is not in the vocabulary",
					h.HypothesisId, h.Basis.Evidenceability)
			}
			if h.Basis.Evidenceability != string(rca.EvidenceabilityOf(h.CauseCategory)) {
				t.Errorf("%s: basis says %q and §9.5 says %q for class %q", h.HypothesisId,
					h.Basis.Evidenceability, rca.EvidenceabilityOf(h.CauseCategory), h.CauseCategory)
			}
			// The claim the column exists for.
			if h.Locus == "" {
				t.Errorf("%s: no locus, so nothing distinguishes it from a hypothesis raised "+
					"at a locus triage did not find present", h.HypothesisId)
			}
			if !present[h.Locus] {
				t.Errorf("%s: raised at locus %q, which triage did not find present. That is a "+
					"legitimate row for a model arm to write and this writer must not produce one",
					h.HypothesisId, h.Locus)
			}
			if h.Locus != got.Locus {
				t.Errorf("%s: locus %q under incident %s at locus %q; the shadow writer files a "+
					"hypothesis under the incident of its own locus", h.HypothesisId, h.Locus,
					got.IncidentId, got.Locus)
			}
		}
	}
	if checked == 0 {
		t.Fatal("no hypothesis was checked")
	}
	t.Logf("%d hypotheses: every basis count matches its array, every confidence is that ratio, "+
		"and every locus is one triage found present", checked)
}
