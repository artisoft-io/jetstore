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
		t.Errorf("§9.5 puts ten cause classes against the nine loci and CauseCategories holds %d",
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
		t.Error("detector_confounder is not in the data model: it is the member AB.1 added so that a " +
			"confounder can be cited as contradicting evidence, and this package's floor depends on it")
	}
}

// §9.5 is a table over the nine loci, and every locus it names must be one.
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
	for sig, l := range signalLocus {
		if !slices.Contains(triage.Loci, l) {
			t.Errorf("signal %s maps to locus %q, which is not in triage.Loci", sig, l)
		}
	}
}

// **Two of the nine loci are in no row of §9.5, and that is the gate's table
// rather than a transcription slip.** The floor emits no hypothesis when either
// fires and reports them in Ranking.UnmappedLoci; this test pins the count, so
// that a future widening of §9.5 has to come here and say so rather than
// quietly changing what a ranking omits.
func TestTwoLociMapToNoCauseClass(t *testing.T) {
	var unmapped []string
	for _, l := range triage.Loci {
		if len(ClassesFor(l)) == 0 {
			unmapped = append(unmapped, l)
		}
	}
	want := []string{triage.LocusStepNeverStarted, triage.LocusPerRecordFailuresUnreportable}
	slices.Sort(unmapped)
	slices.Sort(want)
	if !slices.Equal(unmapped, want) {
		t.Errorf("the loci §9.5 maps to no cause class have changed:\n  got:  %v\n  want: %v\n"+
			"If §9.5 has been widened, update this test and Ranking.UnmappedLoci's comment together.",
			unmapped, want)
	}
}

// **I-262's three classes are three by evidenceability and two by locus, and
// the difference decides whether the floor can emit one at all.**
//
// I-262 says three of the imported ten have no substrate in JetStore:
// source_delivery_failure, dependency_failure and capacity_or_cost_deviation.
// §9.5's *Loci that can carry it* column attaches the last two to nothing, so
// the floor can never emit them whatever a run did. It attaches
// source_delivery_failure to locus run_not_started — the position the evidence
// *would* sit at — while its third column answers **No**, because nothing
// records what was due.
//
// So the floor does emit a source_delivery_failure hypothesis when locus 1
// fires, and the thing that says the record cannot support it is the
// contradicting evidence item §9.5's note becomes. **That is the design working
// rather than a leak**: a class that cannot be evidenced and cannot be
// mentioned is a class an operator never learns is unavailable. The two counts
// are pinned separately below so that a reader quoting "three classes have no
// substrate" beside "two can never be emitted" sees they are not the same
// statement.
func TestClassesWithNoSubstrateAreThreeAndClassesWithNoLocusAreTwo(t *testing.T) {
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
	// transformation_defect is the fourth here and is not I-262's: §9.5 answers
	// "the locus yes, the cause no" for it, which is the same evidenceability
	// and a different reason.
	wantNoEvidence := []string{CauseSourceDeliveryFailure, CauseTransformationDefect,
		CauseDependencyFailure, CauseCapacityOrCostDeviation}
	slices.Sort(noEvidence)
	slices.Sort(wantNoEvidence)
	if !slices.Equal(noEvidence, wantNoEvidence) {
		t.Errorf("the classes §9.5 says the record cannot evidence have changed:\n  got:  %v\n  want: %v",
			noEvidence, wantNoEvidence)
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
		t.Errorf("the confounder vocabulary holds %d members and §9.7's argument is written about fourteen",
			len(observe.Confounders()))
	}
}
