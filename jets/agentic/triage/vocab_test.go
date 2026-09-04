package triage

import (
	"os"
	"regexp"
	"slices"
	"strings"
	"testing"

	"github.com/artisoft-io/jetstore/jets/agentic/observe"
)

// The generated DDL is the authority for the locus vocabulary. These tests need
// no database: they read the CHECK constraint out of the SQL the audit package
// embeds and assert the Go constants against it, so regenerating the model with
// a locus added or removed fails here rather than on an insert. It is
// observe.TestVocabulariesMatchDDL one entity over, and it is the second use of
// that arrangement rather than the first, which is the only test an assertion's
// author cannot run.
const ddlPath = "../audit/agent_audit.sql"

var quoted = regexp.MustCompile(`'([a-z0-9_]+)'`)

func vocabularyIn(t *testing.T, ddl, constraint string) []string {
	t.Helper()
	i := strings.Index(ddl, constraint)
	if i < 0 {
		t.Fatalf("constraint %s is not in %s; has the model been regenerated?", constraint, ddlPath)
	}
	rest := ddl[i:]
	end := len(rest)
	for _, delim := range []string{"\n  CONSTRAINT", "\n);"} {
		if j := strings.Index(rest, delim); j >= 0 && j < end {
			end = j
		}
	}
	rest = rest[:end]
	var out []string
	for _, m := range quoted.FindAllStringSubmatch(rest, -1) {
		out = append(out, m[1])
	}
	if len(out) == 0 {
		t.Fatalf("no members found in constraint %s", constraint)
	}
	return out
}

func TestLociMatchDDL(t *testing.T) {
	b, err := os.ReadFile(ddlPath)
	if err != nil {
		t.Fatalf("reading the generated DDL at %s: %v", ddlPath, err)
	}
	got, want := slices.Clone(Loci), vocabularyIn(t, string(b), "incident_locus_ck")
	slices.Sort(got)
	slices.Sort(want)
	if !slices.Equal(got, want) {
		t.Errorf("the locus vocabulary drifted from the generated DDL:\n  Go:  %v\n  DDL: %v", got, want)
	}
	if len(Loci) != 9 {
		t.Errorf("section 9.4 is a nine-row table and Loci holds %d", len(Loci))
	}
}

// The classifier writes confounders onto a finding from observe's vocabulary,
// and jetsapi.incident's own CHECK admits exactly that vocabulary. This asserts
// the two are the same list rather than two lists that happen to agree — the
// failure it guards against is a confounder added to the anomaly table and not
// to the incident one, which would be refused at the insert with a message
// naming a constraint rather than a field.
func TestIncidentConfoundersAreTheAnomalyVocabulary(t *testing.T) {
	b, err := os.ReadFile(ddlPath)
	if err != nil {
		t.Fatalf("reading the generated DDL: %v", err)
	}
	ddl := string(b)
	incident := vocabularyIn(t, ddl, "incident_confounders_ck")
	anomaly := vocabularyIn(t, ddl, "anomaly_confounders_ck")
	slices.Sort(incident)
	slices.Sort(anomaly)
	if !slices.Equal(incident, anomaly) {
		t.Errorf("incident_confounders_ck and anomaly_confounders_ck are different vocabularies:\n  incident: %v\n  anomaly:  %v",
			incident, anomaly)
	}
	for _, c := range incident {
		if !observe.IsConfounder(c) {
			t.Errorf("the DDL admits confounder %q and observe.IsConfounder does not know it", c)
		}
	}
}

// terminalRunStatuses is a copy of the list inside observe.WorkerRow.Stalled,
// which is unexported and is the only other place in the tree that decides what
// a finished run is. This pins the two together: a status added to one and not
// the other means a run this package calls terminal is one Stalled does not,
// and locus run_not_started and locus worker_not_terminated would then disagree
// about the same header.
func TestTerminalRunStatusesAgreeWithObserve(t *testing.T) {
	for _, status := range terminalRunStatuses {
		w := observe.WorkerRow{HasHeader: true, Status: observe.StatusInProgress, RunStatus: status}
		if !w.Stalled() {
			t.Errorf("triage calls run status %q terminal and observe.WorkerRow.Stalled does not", status)
		}
	}
	// And the other direction, over the statuses a run header can plausibly
	// hold: a status observe treats as terminal that this package does not.
	for _, status := range []string{"in progress", "submitted", "running", "pending"} {
		w := observe.WorkerRow{HasHeader: true, Status: observe.StatusInProgress, RunStatus: status}
		h := RunHeader{Status: status}
		if w.Stalled() != h.Terminal() {
			t.Errorf("run status %q: observe.Stalled says %v and RunHeader.Terminal says %v",
				status, w.Stalled(), h.Terminal())
		}
	}
}

func TestFindingValidate(t *testing.T) {
	good := func() *Finding {
		return &Finding{
			SessionId:     "s1",
			Locus:         LocusWorkerFailed,
			Verdict:       Present,
			Basis:         "one worker row at failed",
			ClassifierRef: "triage@1",
		}
	}
	if err := good().Validate(); err != nil {
		t.Fatalf("a complete finding did not validate: %v", err)
	}

	for name, mutate := range map[string]func(*Finding){
		"no session":     func(f *Finding) { f.SessionId = "" },
		"no basis":       func(f *Finding) { f.Basis = "" },
		"no ref":         func(f *Finding) { f.ClassifierRef = "" },
		"bad locus":      func(f *Finding) { f.Locus = "worker_broke" },
		"bad verdict":    func(f *Finding) { f.Verdict = "maybe" },
		"bad confounder": func(f *Finding) { f.Confounders = []string{"probably_fine"} },
		// The invariant jetsapi.incident enforces as incident_step_confounder_ck.
		"step without its confounder": func(f *Finding) { f.StepRef = "reducing01" },
	} {
		f := good()
		mutate(f)
		if err := f.Validate(); err == nil {
			t.Errorf("%s: Validate accepted it", name)
		}
	}

	// And the same finding with the confounder is accepted, so the test above
	// is about the invariant rather than about the field.
	f := good()
	f.StepRef = "reducing01"
	f.Confounders = []string{observe.ConfounderStepLabelAmbiguous}
	if err := f.Validate(); err != nil {
		t.Errorf("a step_ref carrying step_label_ambiguous was refused: %v", err)
	}
}

// Every finding Classify produces must validate, whatever the evidence. The
// cheapest way this breaks is a Present verdict that names a step and forgets
// the confounder, which is exactly what the database would refuse at 3 a.m.
func assertAllValid(t *testing.T, r *Report) {
	t.Helper()
	if len(r.Findings) != len(Loci) {
		t.Fatalf("Classify returned %d findings for %d loci", len(r.Findings), len(Loci))
	}
	seen := map[string]bool{}
	for i := range r.Findings {
		f := &r.Findings[i]
		if err := f.Validate(); err != nil {
			t.Errorf("finding for locus %s does not validate: %v", f.Locus, err)
		}
		if seen[f.Locus] {
			t.Errorf("locus %s reported twice", f.Locus)
		}
		seen[f.Locus] = true
	}
	for _, l := range Loci {
		if !seen[l] {
			t.Errorf("locus %s is missing from the report", l)
		}
	}
}

// A classifier with no evidence at all must report nine NotEvaluable verdicts
// and no absent ones. It is the degenerate case and it is the one a boolean
// predicate would get wrong nine times over.
func TestNoEvidenceIsNineNotEvaluable(t *testing.T) {
	r := Default().Classify(&Evidence{SessionId: "s1"})
	assertAllValid(t, r)
	for i := range r.Findings {
		if r.Findings[i].Verdict != NotEvaluable {
			t.Errorf("locus %s is %q with no evidence at all; it must be not_evaluable",
				r.Findings[i].Locus, r.Findings[i].Verdict)
		}
	}
	if r.Evaluable != 0 {
		t.Errorf("Evaluable is %d with no evidence", r.Evaluable)
	}
	if len(r.Fired()) != 0 {
		t.Errorf("%d loci fired with no evidence", len(r.Fired()))
	}
}
