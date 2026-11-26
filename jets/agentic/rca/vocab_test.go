package rca

import (
	"os"
	"regexp"
	"slices"
	"strings"
	"testing"

	"github.com/artisoft-io/jetstore/jets/agentic/observe"
	"github.com/artisoft-io/jetstore/jets/agentic/triage"
)

// The vocabularies against their generators. These need no database: they read
// the generated DDL and the generated .jr out of the tree and assert the Go
// constants against them, so a regeneration that adds or removes a member fails
// here rather than on an insert — or, for the two evidence columns, rather than
// never failing at all.
//
// It is triage.TestLociMatchDDL one entity over, which is the third use of
// observe.TestVocabulariesMatchDDL's arrangement.
const (
	ddlPath = "../audit/agent_audit.sql"
	jrPath  = "../../workspace_assets/data_model/jets_agentic.jr"
)

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

func readDDL(t *testing.T) string {
	t.Helper()
	b, err := os.ReadFile(ddlPath)
	if err != nil {
		t.Fatalf("reading the generated DDL at %s: %v", ddlPath, err)
	}
	return string(b)
}

// The cause vocabulary is jetsapi.hypothesis's CHECK, and the same list is
// jetsapi.incident's. Both are asserted, because a hypothesis's cause_category
// and an incident's classification are the same claim written on two rows and
// a member added to one and not the other would be refused at whichever insert
// happened second.
func TestCauseCategoriesMatchDDL(t *testing.T) {
	ddl := readDDL(t)
	got := slices.Clone(CauseCategories)
	slices.Sort(got)
	for _, constraint := range []string{"hypothesis_cause_category_ck", "incident_classification_ck"} {
		want := vocabularyIn(t, ddl, constraint)
		slices.Sort(want)
		if !slices.Equal(got, want) {
			t.Errorf("the cause vocabulary drifted from %s:\n  Go:  %v\n  DDL: %v", constraint, got, want)
		}
	}
	if len(CauseCategories) != 10 {
		t.Errorf("the gate's table puts ten cause classes against the nine loci and CauseCategories holds %d",
			len(CauseCategories))
	}
}

// EvidenceSource has no CHECK anywhere, because the two evidence columns are
// jsonb. So the .jr is the only generated statement of the vocabulary, and
// Hypothesis.Validate is the only thing that will refuse an invented source —
// which is why this test exists and why it reads a different artefact from the
// one above.
func TestEvidenceSourcesMatchTheDataModel(t *testing.T) {
	b, err := os.ReadFile(jrPath)
	if err != nil {
		t.Fatalf("reading the generated data model at %s: %v", jrPath, err)
	}
	member := regexp.MustCompile(`jetsa:EVIDENCE_SOURCE_[A-Z_]+ = "([a-z_]+)"`)
	var want []string
	for _, m := range member.FindAllStringSubmatch(string(b), -1) {
		want = append(want, m[1])
	}
	if len(want) == 0 {
		t.Fatalf("no EVIDENCE_SOURCE_* literals in %s; has the emitter changed shape?", jrPath)
	}
	got := slices.Clone(EvidenceSources)
	slices.Sort(got)
	slices.Sort(want)
	if !slices.Equal(got, want) {
		t.Errorf("the evidence-source vocabulary drifted from the data model:\n  Go: %v\n  .jr: %v", got, want)
	}
	if !slices.Contains(want, SourceDetectorConfounder) {
		t.Error("detector_confounder is not in the data model: it is the member added so that a " +
			"confounder can be cited as contradicting evidence, and this package's floor depends on it")
	}
}

// The gate's table is a table over the nine loci, and every locus it names must be one.
// The failure this guards against is a rename in triage that leaves this
// package silently mapping a locus that no longer exists — the ranker would
// then emit nothing for the renamed locus and report it in neither
// UnmappedLoci nor UnaskedLoci, because it would simply never appear.
func TestEveryMappedLocusIsInTheTriageVocabulary(t *testing.T) {
	for _, c := range Classes() {
		for _, l := range c.Loci {
			if !slices.Contains(triage.Loci, l) {
				t.Errorf("cause class %s names locus %q, which is not in triage.Loci", c.Name, l)
			}
		}
	}
}

// **Two of the nine run loci are in no row of the gate's table, and that is the
// table rather than a transcription slip.** The floor emits no hypothesis when
// either fires and reports them in Ranking.UnmappedLoci; this test pins the set,
// so that a future widening of the table has to come here and say so rather than
// quietly changing what a ranking omits.
//
// **The two delivery loci were in this list for one release and are not any
// more.** The vocabulary was widened first, so that a feed anomaly had somewhere
// to land; the cause classes a delivery locus can carry were the table's own
// change, and they are now in it: three classes each, not one.
func TestLociThatMapToNoCauseClass(t *testing.T) {
	var unmapped []string
	for _, l := range triage.Loci {
		if len(ClassesFor(l)) == 0 {
			unmapped = append(unmapped, l)
		}
	}
	// job_failed is in no row: the table's classes are causes of a run's or a
	// delivery's failure, and a job's failure is neither. Its incident carries
	// no hypothesis, and its failure_details is the evidence of why.
	want := []string{triage.LocusStepNeverStarted, triage.LocusPerRecordFailuresUnreportable,
		triage.LocusJobFailed}
	slices.Sort(unmapped)
	slices.Sort(want)
	if !slices.Equal(unmapped, want) {
		t.Errorf("the loci the gate's table maps to no cause class have changed:\n  got:  %v\n  want: %v\n"+
			"If the gate's table has been widened, update this test and Ranking.UnmappedLoci's comment together.",
			unmapped, want)
	}
}

// **The three unsubstantiated classes are three by evidenceability and two by locus, and
// the difference decides whether the floor can emit one at all.**
//
// Three of the imported ten have no substrate in JetStore:
// source_delivery_failure, dependency_failure and capacity_or_cost_deviation.
// The table's *Loci that can carry it* column attaches the last two to nothing, so
// the floor can never emit them whatever a run did. It attaches
// source_delivery_failure to locus run_not_started — the position the evidence
// *would* sit at — while its third column answers **No**, because nothing
// records what was due.
//
// **The set answering "no" is exactly those three, and it took a correction to
// become so.** The table also answered "no" for transformation_defect, on the
// ground that nothing binds a run to a workspace version; a later finding supersedes that
// in terms, the binding having landed the same day, so this file
// classifies that row Coarse and cites the finding for it. Had the
// transcription been literal, a class the record can now distinguish coarsely
// would have been ranked below every class it can evidence at all.
//
// So the floor does emit a source_delivery_failure hypothesis when locus 1
// fires, and the thing that says the record cannot support it is the
// contradicting evidence item the table's note becomes. **That is the design working
// rather than a leak**: a class that cannot be evidenced and cannot be
// mentioned is a class an operator never learns is unavailable. The two counts
// are pinned separately below so that a reader quoting "three classes have no
// substrate" beside "two can never be emitted" sees they are not the same
// statement.
//
// **Three became two everywhere and stayed three on a run.** A feed's declared
// cadence is a record of what was due, so source_delivery_failure is graded
// conditional and is evidenced at delivery_overdue, where a cadence is read. At
// run_not_started nothing links the run to a cadence, and its grade there is
// still none (CauseClass.At). The class-wide set answering "no" is the two with
// no locus; the per-locus grade is what keeps the third ranked last on a run.
func TestClassesWithNoSubstrateAndClassesWithNoLocus(t *testing.T) {
	var noLocus, noEvidence []string
	for _, c := range Classes() {
		if len(c.Loci) == 0 {
			noLocus = append(noLocus, c.Name)
			if c.Evidenceability != None {
				t.Errorf("class %s is attached to no locus but its evidenceability is %q", c.Name,
					c.Evidenceability)
			}
		}
		if c.Evidenceability == None {
			noEvidence = append(noEvidence, c.Name)
		}
	}
	wantNoLocus := []string{CauseDependencyFailure, CauseCapacityOrCostDeviation}
	slices.Sort(noLocus)
	slices.Sort(wantNoLocus)
	if !slices.Equal(noLocus, wantNoLocus) {
		t.Errorf("the classes the floor can never emit have changed:\n  got:  %v\n  want: %v",
			noLocus, wantNoLocus)
	}
	wantNoEvidence := []string{CauseDependencyFailure, CauseCapacityOrCostDeviation}
	slices.Sort(noEvidence)
	slices.Sort(wantNoEvidence)
	if !slices.Equal(noEvidence, wantNoEvidence) {
		t.Errorf("the classes the gate's table says the record cannot evidence anywhere have changed:\n  got:  %v\n  want: %v",
			noEvidence, wantNoEvidence)
	}
	// And source_delivery_failure, graded conditional on a declared cadence,
	// is still none at the one run locus it is attached to, which is what keeps
	// it ranked last on a run that never started.
	if g := EvidenceabilityAt(CauseSourceDeliveryFailure, triage.LocusRunNotStarted); g != None {
		t.Errorf("source_delivery_failure at run_not_started is %q, want none: nothing links a run to a cadence", g)
	}
	if g := EvidenceabilityAt(CauseSourceDeliveryFailure, triage.LocusDeliveryOverdue); g != Conditional {
		t.Errorf("source_delivery_failure at delivery_overdue is %q, want conditional", g)
	}
	if g := EvidenceabilityAt(CauseSourceContentChange, triage.LocusRowsLostSilently); g != Coarse {
		t.Errorf("source_content_change at rows_lost_silently is %q, want coarse", g)
	}
	if g := EvidenceabilityAt(CauseSourceContentChange, triage.LocusDeliveryDepartsProfile); g != Conditional {
		t.Errorf("source_content_change at delivery_departs_profile is %q, want conditional", g)
	}
}

// The confounder split has to cover the whole vocabulary, or a member added to
// observe would fall silently onto the contradicting side for every class
// including benign — which is the safe direction and is still a decision
// nobody made.
//
// It is asserted as the *complement of observe.RecordConfounders* rather than
// compared member by member, because the two lists were built for different
// reasons and their agreement is the thing worth watching: RecordConfounders is
// "what the record can establish without reading a config", this is "what a
// configured behaviour explains". They coincide today. The day they stop, this
// test is where somebody finds out.
func TestConfounderSplitIsTheComplementOfRecordConfounders(t *testing.T) {
	var complement []string
	for _, c := range observe.Confounders() {
		if !slices.Contains(observe.RecordConfounders, c) {
			complement = append(complement, c)
		}
	}
	got := slices.Clone(configuredBehaviourConfounders)
	slices.Sort(got)
	slices.Sort(complement)
	if !slices.Equal(got, complement) {
		t.Errorf("the configured-behaviour split is no longer observe.RecordConfounders' complement:\n"+
			"  rca: %v\n  complement: %v\n"+
			"The two lists answer different questions and have agreed until now; decide which side a new "+
			"member falls on rather than editing one of them to match.", got, complement)
	}
	// And the split has to be exhaustive: every member is on one side.
	for _, c := range observe.Confounders() {
		on := benignExplanation(c)
		if !on && !slices.Contains(observe.RecordConfounders, c) {
			t.Errorf("confounder %s is on neither side of the split", c)
		}
	}
	if len(observe.Confounders()) != 14 {
		t.Errorf("the confounder vocabulary holds %d members and the split's argument is written about fourteen",
			len(observe.Confounders()))
	}
}
