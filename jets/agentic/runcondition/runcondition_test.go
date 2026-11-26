package runcondition

import (
	"os"
	"regexp"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/artisoft-io/jetstore/jets/agentic/audit"
	"github.com/artisoft-io/jetstore/jets/agentic/triage"
)

// The verdict vocabulary is the generated CHECK's, member for member.
func TestVerdictsMatchTheGeneratedCheck(t *testing.T) {
	sql, err := os.ReadFile("../audit/agent_audit.sql")
	if err != nil {
		t.Fatal(err)
	}
	m := regexp.MustCompile(`run_finding_verdict_ck CHECK \(verdict IN \(([^)]*)\)\)`).FindSubmatch(sql)
	if m == nil {
		t.Fatal("no run_finding_verdict_ck in agent_audit.sql; this test is stale")
	}
	var got []string
	for _, lit := range strings.Split(string(m[1]), ",") {
		got = append(got, strings.Trim(strings.TrimSpace(lit), "'"))
	}
	if !slices.Equal(got, Verdicts) {
		t.Errorf("the CHECK admits %v and Verdicts is %v", got, Verdicts)
	}
}

func perRecordReport(verdict triage.Verdict, codes ...triage.CodeCount) *triage.Report {
	return &triage.Report{SessionId: "s1", Findings: []triage.Finding{
		{Locus: triage.LocusWorkerFailed, Verdict: triage.Present, ClassifierRef: "triage@1"},
		{Locus: triage.LocusPerRecordFailuresReported, Verdict: verdict, ClassifierRef: "triage@1", Codes: codes},
		{Locus: triage.LocusDeliveryOverdue, Verdict: triage.NotEvaluable, ClassifierRef: "triage@1"},
	}}
}

// The rows a report stores, and the verdict a code reads from them: a code with
// no row of its own is absent where the locus was evaluated, and not
// evaluable where it was not.
func TestRowsOfAndVerdictAt(t *testing.T) {
	at := time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC)
	rows := RowsOf(perRecordReport(triage.Present,
		triage.CodeCount{Code: "a", StoredRows: 3, State: triage.CodeReports},
		triage.CodeCount{Code: "b", StoredRows: 1, State: triage.CodeQuiet},
		triage.CodeCount{Code: "c", StoredRows: 9, State: triage.CodeDebug}), at)
	if len(rows) != 5 {
		t.Fatalf("%d rows, want 5 (no delivery locus): %+v", len(rows), rows)
	}
	for _, c := range []struct{ locus, code, want string }{
		{triage.LocusWorkerFailed, "", VerdictPresent},
		{triage.LocusPerRecordFailuresReported, "", VerdictPresent},
		{triage.LocusPerRecordFailuresReported, "a", VerdictPresent},
		{triage.LocusPerRecordFailuresReported, "b", VerdictQuiet},
		{triage.LocusPerRecordFailuresReported, "c", VerdictQuiet},
		{triage.LocusPerRecordFailuresReported, "d", VerdictAbsent},
		{triage.LocusRunNotStarted, "", ""},
	} {
		if got, _ := VerdictAt(rows, c.locus, c.code); got != c.want {
			t.Errorf("%s %q reads %q, want %q", c.locus, c.code, got, c.want)
		}
	}
	if _, r := VerdictAt(rows, triage.LocusPerRecordFailuresReported, "a"); r.StoredRows == nil || *r.StoredRows != 3 {
		t.Errorf("code a's row does not carry its stored rows: %+v", r)
	}
	ne := RowsOf(perRecordReport(triage.NotEvaluable), at)
	if got, _ := VerdictAt(ne, triage.LocusPerRecordFailuresReported, "a"); got != VerdictNotEvaluable {
		t.Errorf("a code on a not-evaluable locus reads %q", got)
	}
	if len(ne) != 2 {
		t.Errorf("a not-evaluable per-record locus stores %d rows, want its one row and worker_failed's", len(ne))
	}
}

func runAt(session string, hour int, verdict string) Run {
	return Run{SessionId: session, StartTime: time.Date(2026, 1, 1, hour, 0, 0, 0, time.UTC), Verdict: verdict,
		ClassifiedAt: time.Date(2026, 2, 1, 0, 0, 0, 0, time.UTC)}
}

var key = Key{Client: "c1", Process: "loader", Locus: triage.LocusWorkerFailed}

// Quiet, not-evaluable and never-classified runs neither join nor end a
// condition; only an absent one ends it.
func TestOnlyAnAbsentRunEndsACondition(t *testing.T) {
	h := &History{Key: key, Runs: []Run{
		runAt("r1", 1, VerdictPresent), runAt("r2", 2, VerdictQuiet), runAt("r3", 3, VerdictPresent),
		runAt("r4", 4, VerdictNotEvaluable), runAt("r5", 5, ""), runAt("r6", 6, VerdictPresent),
		runAt("r7", 7, VerdictAbsent), runAt("r8", 8, VerdictPresent),
	}}
	cs := h.Conditions()
	if len(cs) != 2 {
		t.Fatalf("%d conditions, want 2: %+v", len(cs), cs)
	}
	if got := cs[0].Sessions(); !slices.Equal(got, []string{"r1", "r3", "r6"}) || cs[0].Skipped != 3 ||
		cs[0].EndedBy == nil || cs[0].EndedBy.SessionId != "r7" || cs[0].After != nil {
		t.Errorf("first condition %v, skipped %d, ended by %v", got, cs[0].Skipped, cs[0].EndedBy)
	}
	if got := cs[1].Sessions(); !slices.Equal(got, []string{"r8"}) || !cs[1].Open() || cs[1].After.SessionId != "r7" {
		t.Errorf("second condition %v, open %v", got, cs[1].Open())
	}
	if !strings.Contains(cs[0].Describe(), "3 runs between them say nothing here") {
		t.Errorf("the description does not count the skipped runs: %s", cs[0].Describe())
	}
}

// A condition finds its incident by position, so an incident whose own run was
// deleted is still the one its later runs join.
func TestAnIncidentWhoseRunIsDeletedIsStillFound(t *testing.T) {
	h := &History{Key: key, Runs: []Run{
		runAt("r0", 0, VerdictAbsent), runAt("r2", 2, VerdictPresent), runAt("r3", 3, VerdictPresent),
	}, Incidents: []*Incident{
		{IncidentId: "inc_old", SessionId: "rold", Position: time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)},
		{IncidentId: "inc_r1", SessionId: "r1", Position: time.Date(2026, 1, 1, 1, 0, 0, 0, time.UTC)},
	}}
	for _, s := range []string{"r2", "r3"} {
		if p := h.Resolve(s); p.Kind != Join || p.Target.IncidentId != "inc_r1" {
			t.Errorf("%s: %s into %v; want joined to inc_r1", s, p.Kind, p.Target)
		}
	}
	c, members := h.Members("inc_r1")
	if c == nil || len(members) != 2 {
		t.Errorf("inc_r1 has %d members, want r2 and r3", len(members))
	}
	if c, _ := h.Members("inc_old"); c != nil {
		t.Error("an incident before the absent run is held by the condition after it")
	}
}

// A run whose finding is stored after a person's verdict on its condition's
// incident opens a new incident naming it; one stored before joins.
func TestAFindingAfterAVerdictIsANewIncident(t *testing.T) {
	verdictAt := time.Date(2026, 2, 1, 12, 0, 0, 0, time.UTC)
	later := runAt("r3", 3, VerdictPresent)
	later.ClassifiedAt = verdictAt.Add(time.Hour)
	h := &History{Key: key, Runs: []Run{runAt("r1", 1, VerdictPresent), runAt("r2", 2, VerdictPresent), later},
		Incidents: []*Incident{{IncidentId: "inc_r1", SessionId: "r1", Position: runAt("r1", 1, "").StartTime,
			Verdict: &audit.IncidentTransition{ToStatus: audit.IncidentSuppressedAsBenign, Actor: "p@example.com",
				TransitionedAt: verdictAt}}}}
	if p := h.Resolve("r2"); p.Kind != Join {
		t.Errorf("a finding stored before the verdict: %s", p.Kind)
	}
	p := h.Resolve("r3")
	if p.Kind != WriteHere || p.Judged == nil || p.Judged.IncidentId != "inc_r1" {
		t.Fatalf("a finding stored after the verdict: %s, judged %v", p.Kind, p.Judged)
	}
	if b := p.Basis(); !strings.Contains(b, "The previous incident at this key is inc_r1") ||
		!strings.Contains(b, "suppressed_as_benign by p@example.com") {
		t.Errorf("the basis does not name the judged incident: %s", b)
	}
	// A dry overlay that says less does not replace what is held.
	h.Overlay("r3", VerdictNotEvaluable, verdictAt)
	if h.RunOf("r3").Verdict != VerdictPresent {
		t.Error("an overlay retracted a present verdict")
	}
}
