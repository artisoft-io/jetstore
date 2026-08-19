package eval

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// --- criterion 20: what the report may and may not say --------------------

func TestReport_RefusesToPublishWithoutAnEra(t *testing.T) {
	r := &Report{
		Operators:    []OperatorResult{{Operator: "map_record", Attempted: 10, Passed: 7, LiveInstances: 241}},
		HeldOutFiles: []string{"a.pc.json"},
	}
	if err := r.Validate(); err == nil {
		t.Fatal("a report with no era must be refused: it will be compared with one from the other side")
	}
	if !strings.Contains(r.String(), "INVALID REPORT") {
		t.Error("an invalid report must render loudly rather than silently")
	}
}

func TestReport_RefusesAReportNobodyCanPlace(t *testing.T) {
	for _, tc := range []struct {
		name string
		r    *Report
	}{
		{"no held-out files", &Report{
			Era:       EraPreTemplates,
			Operators: []OperatorResult{{Operator: "x", LiveInstances: 1}},
		}},
		{"no operators", &Report{Era: EraPreTemplates, HeldOutFiles: []string{"a"}}},
		{"passed exceeds attempted", &Report{
			Era: EraPreTemplates, HeldOutFiles: []string{"a"},
			Operators: []OperatorResult{{Operator: "x", Attempted: 2, Passed: 3, LiveInstances: 9}},
		}},
		{"untested but attempted", &Report{
			Era: EraPreTemplates, HeldOutFiles: []string{"a"},
			Operators: []OperatorResult{{Operator: "clustering", Attempted: 1, LiveInstances: 0}},
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if err := tc.r.Validate(); err == nil {
				t.Error("expected the report to be refused")
			}
		})
	}
}

// The three reporting shapes decision 13 requires, each with the reason it is
// not the obvious one.
func TestReport_RendersEachOperatorHonestly(t *testing.T) {
	r := &Report{
		Era:          EraPreTemplates,
		HeldOutFiles: []string{"qc_hra.pc.json"},
		Operators: []OperatorResult{
			// Enough cases for a percentage to carry information.
			{Operator: "map_record", Attempted: 20, Passed: 13, LiveInstances: 241},
			// Too few: "3 of 4" is honest, "75%" invites a comparison the
			// sample cannot support.
			{Operator: "analyze", Attempted: 4, Passed: 3, LiveInstances: 4},
			// No live instances at all — untested, not zero. Zero is a
			// measurement; this is the absence of one.
			{Operator: "clustering", LiveInstances: 0},
			// Instances exist but the split held none out: not run, which is a
			// fact about the split rather than about the model.
			{Operator: "merge", Attempted: 0, LiveInstances: 8},
		},
	}
	out := r.String()

	if !strings.Contains(out, "13 of 20 cases compiled (65%)") {
		t.Errorf("map_record should report a rate with its denominator:\n%s", out)
	}
	if !strings.Contains(out, "3 of 4 cases compiled (too few for a rate)") {
		t.Errorf("analyze should report cases, not a rate:\n%s", out)
	}
	if strings.Contains(out, "75%") {
		t.Errorf("analyze reported a percentage on four cases:\n%s", out)
	}
	if !strings.Contains(out, "clustering") || !strings.Contains(out, "untested") {
		t.Errorf("clustering should report untested:\n%s", out)
	}
	if !strings.Contains(out, "merge") || !strings.Contains(out, "not run") {
		t.Errorf("merge should report not-run rather than a zero rate:\n%s", out)
	}
	// The era, and the held-out files, so a figure can be placed.
	if !strings.Contains(out, string(EraPreTemplates)) || !strings.Contains(out, "qc_hra.pc.json") {
		t.Errorf("the report cannot be placed:\n%s", out)
	}
}

// The ban is the point of the type, so it is asserted rather than assumed: no
// total, and a note saying so, because a reader who wants one should meet the
// reason rather than the absence.
func TestReport_PublishesNoAggregate(t *testing.T) {
	r := &Report{
		Era:          EraPreTemplates,
		HeldOutFiles: []string{"a.pc.json"},
		Operators: []OperatorResult{
			{Operator: "map_record", Attempted: 100, Passed: 90, LiveInstances: 241},
			{Operator: "analyze", Attempted: 4, Passed: 0, LiveInstances: 4},
		},
	}
	out := r.String()
	// 90/104 would be 87% — the flattering number the ban exists to prevent.
	for _, forbidden := range []string{"87%", "total", "overall", "aggregate compile-pass"} {
		if strings.Contains(strings.ToLower(out), strings.ToLower(forbidden)) &&
			!strings.Contains(out, "No aggregate figure is published") {
			t.Errorf("the report contains %q:\n%s", forbidden, out)
		}
	}
	if !strings.Contains(out, "No aggregate figure is published") {
		t.Errorf("the report does not say why there is no total:\n%s", out)
	}
}

// --- the corpus and its split ---------------------------------------------

const twoPipes = `{
  "conditional_pipes_config": [
    {"apply": [{"type":"map_record"},{"type":"analyze"}]},
    {"apply": [{"type":"partition_writer"}]}
  ]
}`

func TestMakeCase_RemovesOneInstanceAndKeepsTheRest(t *testing.T) {
	c, err := MakeCase([]byte(twoPipes), Instance{
		File: "t.pc.json", Operator: "analyze",
		Path:  []Step{key("conditional_pipes_config"), at(0), key("apply")},
		Index: 1,
	})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(c.Expected), "analyze") {
		t.Errorf("the removed instance is not the expected answer: %s", c.Expected)
	}
	if strings.Contains(string(c.Context), "analyze") {
		t.Errorf("the context still contains the answer: %s", c.Context)
	}
	// The rest of the document is untouched — the context is what an author
	// would see with one step missing, not a reduced document.
	for _, keep := range []string{"map_record", "partition_writer"} {
		if !strings.Contains(string(c.Context), keep) {
			t.Errorf("the context lost %s: %s", keep, c.Context)
		}
	}
}

func TestMakeCase_RefusesAnInstanceThatIsNotThere(t *testing.T) {
	if _, err := MakeCase([]byte(twoPipes), Instance{
		Path: []Step{key("conditional_pipes_config"), at(9), key("apply")},
	}); err == nil {
		t.Error("expected a missing pipe to be refused")
	}
	if _, err := MakeCase([]byte(twoPipes), Instance{
		Path: []Step{key("nope")},
	}); err == nil {
		t.Error("expected a path that does not resolve to be refused")
	}
}

func TestSplit_HoldsOutFilesAndRefusesADegenerateSplit(t *testing.T) {
	c := &Corpus{Files: []string{"a", "b", "c", "d", "e", "f"}}
	s, err := c.SplitFiles(3)
	if err != nil {
		t.Fatal(err)
	}
	if len(s.HeldOut) != 2 || len(s.Train) != 4 {
		t.Errorf("split is %d held out and %d train, want 2 and 4", len(s.HeldOut), len(s.Train))
	}
	for _, h := range s.HeldOut {
		for _, tr := range s.Train {
			if h == tr {
				t.Errorf("%s is on both sides of the split", h)
			}
		}
	}
	if _, err := c.SplitFiles(1); err == nil {
		t.Error("holding out every file leaves nothing to learn from and must be refused")
	}
}

// Coverage is what tells a reader an operator could not be measured by this
// split, which the report then renders as not-run rather than as a failure.
func TestCoverage_NamesWhatTheSplitCannotMeasure(t *testing.T) {
	c := &Corpus{
		Files: []string{"a", "b"},
		Instances: []Instance{
			{File: "a", Operator: "map_record"},
			{File: "b", Operator: "analyze"},
		},
	}
	cov := c.Coverage(&Split{HeldOut: []string{"a"}, Train: []string{"b"}})
	if cov["map_record"] != 1 {
		t.Errorf("map_record coverage = %d, want 1", cov["map_record"])
	}
	if _, ok := cov["analyze"]; ok {
		t.Error("analyze is only in the training half and must not be reported as covered")
	}
}

// Against the real corpus when it is present. The workspaces are submodules and
// a prose-only or partial checkout will not have them, so this skips rather
// than fails — but where they are present, the counts are the ones every figure
// in the project rests on.
func TestAgainstTheRealCorpus(t *testing.T) {
	root := repoRootWithWorkspaces(t)
	c, err := LoadCorpus(root)
	if err != nil {
		t.Skipf("no corpus under %s: %v", root, err)
	}
	if len(c.Files) == 0 {
		t.Skip("corpus is empty")
	}
	byOp := c.ByOperator()
	t.Logf("corpus: %d live files, %d instances, %d operators",
		len(c.Files), len(c.Instances), len(byOp))
	// The figures every count in this project rests on, re-measured 2026-08-16
	// at B.4 and asserted here so the walk cannot quietly disagree with them.
	if len(c.Files) != 45 {
		t.Errorf("corpus has %d live files, want 45 (I-13's definition: workspaces/*/pipes_config/**)",
			len(c.Files))
	}
	if len(c.Instances) != 458 {
		t.Errorf("corpus has %d transformation instances, want 458; a flat walk of the top-level "+
			"pipes finds 257, which is the mistake this assertion exists to catch", len(c.Instances))
	}
	for _, op := range []string{"map_record", "partition_writer", "ollama", "high_freq", "distinct"} {
		if byOp[op] == 0 {
			t.Errorf("%s has no instances; nested apply arrays are not being walked", op)
		}
	}
	// The skew the reporting rules exist because of — asserted, so that if it
	// ever stops being true the rules can be revisited deliberately.
	head := byOp["map_record"] + byOp["partition_writer"]
	if head*100/len(c.Instances) < 70 {
		t.Errorf("the two head operators are %d%% of %d instances; the skew decision 13 assumes has changed",
			head*100/len(c.Instances), len(c.Instances))
	}
}

func repoRootWithWorkspaces(t *testing.T) string {
	t.Helper()
	// The corpus lives in the *parent* checkout: workspaces/ are submodules of
	// jetstore_agentic_ai, not of this repo. A worktree that initialised only
	// jetstore_ai has none, which is why this test skips rather than fails —
	// and why the override exists, since the parent is not always one level up.
	if root := os.Getenv("JETS_EVAL_CORPUS_ROOT"); root != "" {
		return root
	}
	wd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	// jets/agentic/eval -> jetstore_ai -> the parent checkout.
	return filepath.Clean(filepath.Join(wd, "..", "..", "..", ".."))
}
