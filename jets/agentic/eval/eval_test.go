package eval

import (
	"encoding/json"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
)

// --- criterion 20: what the report may and may not say --------------------

func TestReport_RefusesToPublishWithoutAnEra(t *testing.T) {
	r := &Report{
		Model:        "granite4.1:3b",
		CaseSource:   "mutation cases from workspaces/*/pipes_config/**",
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
			Era: EraPreTemplates, Model: "m", CaseSource: "s",
			Operators: []OperatorResult{{Operator: "x", LiveInstances: 1}},
		}},
		{"no operators", &Report{
			Era: EraPreTemplates, Model: "m", CaseSource: "s", HeldOutFiles: []string{"a"},
		}},
		{"passed exceeds attempted", &Report{
			Era: EraPreTemplates, Model: "m", CaseSource: "s", HeldOutFiles: []string{"a"},
			Operators: []OperatorResult{{Operator: "x", Attempted: 2, Passed: 3, LiveInstances: 9}},
		}},
		{"untested but attempted", &Report{
			Era: EraPreTemplates, Model: "m", CaseSource: "s", HeldOutFiles: []string{"a"},
			Operators: []OperatorResult{{Operator: "clustering", Attempted: 1, LiveInstances: 0}},
		}},
		// P.1's two additions. A figure that cannot say what produced it or
		// what it measured is the one that travels furthest.
		{"no model", &Report{
			Era: EraPreTemplates, CaseSource: "s", HeldOutFiles: []string{"a"},
			Operators: []OperatorResult{{Operator: "x", LiveInstances: 1}},
		}},
		{"no case source", &Report{
			Era: EraPreTemplates, Model: "m", HeldOutFiles: []string{"a"},
			Operators: []OperatorResult{{Operator: "x", LiveInstances: 1}},
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
		Model:        "granite4.1:3b",
		CaseSource:   "mutation cases from workspaces/*/pipes_config/**",
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
		Model:        "granite4.1:3b",
		CaseSource:   "mutation cases from workspaces/*/pipes_config/**",
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
// than fails.
//
// **Nothing here asserts a count, and that is the whole of BB.3.** Until
// 2026-09-11 this test asserted `len(c.Files) == 41` and `len(c.Instances) ==
// 455` as literals. They are a property of four *other* repositories --
// cedargate_ws, jets_ws, usi_ws and walrus_ws are living workspaces that track
// real installations -- so every legitimate asset change landed here as a red
// test. The comment this one replaces was a four-paragraph account of the three
// times that happened in a month, and a fourth -- BA.1 removing two drain pipes,
// 455 to 453 -- is what retired them. **Its own record is the argument: three
// false failures and, in that account, not one walker defect.** A detector with
// that ratio is worse than none, because a test that is red for a legitimate
// reason is one somebody eventually edits without reading.
//
// **The two literals were doing three jobs and only one of them wanted an
// equality** (Phase 8 §1.7.1). Each is now served by the thing it actually
// needed, and each assertion below says which job it is standing in for, so that
// a future reader does not "restore" a number:
//
//   - **Guard the walker.** `instancesIn` finds `apply` arrays under both
//     authored roots and at both depths; the first version of it read one shape
//     and found 257 of them. That is a *relation*, not a count -- the nested
//     walk finds strictly more than a flat one -- and `flatInstances` below
//     recomputes the flat number from the same documents on every run, so the
//     relation is checked rather than remembered.
//   - **Guard the loader.** A manifest-exclusion bug that drops the corpus must
//     fail rather than skip. That is a *floor*, and growth cannot break a floor.
//   - **Be the denominator** every operator-coverage figure in this project
//     rests on. That is a *measurement*, logged with the run that produced it
//     and written down with its date -- here, and in the prose at BB.4.
//
// **What this loses is real and is R-112.** A floor and a relation cannot catch
// a walker that miscounts by a constant; an equality could. What recovers most
// of it is `TestTheWalkerCountsNestedShapes`, which asserts exact numbers over a
// committed fixture -- a defect that shows over the shapes the fixture carries
// is caught exactly, by name and by operator. What is genuinely gone is a defect
// that only manifests over a shape the corpus has and the fixture does not, and
// that is the price of not being told the news. The operator canaries and the
// head-operator skew assertion below are properties rather than counts, predate
// this phase, and have never gone stale.
//
// **The -count=1 hazard is untouched by any of this (F1139).** The corpus lives
// outside the Go module, so nothing in the cache key changes when `workspaces/`
// moves: an old-corpus `ok` is replayed against a new corpus, and it is
// indistinguishable from a real pass. That cost three merges in September.
// **Run this and every corpus-reading test with -count=1.** An invariant that is
// never re-run is not weaker than a literal that is never re-run; it is exactly
// as weak.
//
// **The figures, measured 2026-09-11 with -count=1 over the four workspaces at
// the pin this commit records: 41 live files, 453 transformation instances, 14
// operators.** A flat walk of the same 41 documents finds 257 across 8. They are
// here as a dated measurement and nothing asserts them.
func TestAgainstTheRealCorpus(t *testing.T) {
	root := repoRootWithWorkspaces(t)
	onDisk := pcFilesOnDisk(t, root)
	if len(onDisk) == 0 {
		// No corpus in this checkout, which is a prose-only worktree rather than
		// a defect. Distinguished from an empty *load* deliberately: the second
		// is what the floor below exists to catch, and reading both as "skip" is
		// how a loader bug would pass.
		t.Skipf("no .pc.json under %s/workspaces/*/pipes_config", root)
	}
	c, err := LoadCorpus(root)
	if err != nil {
		t.Fatalf("the corpus is on disk (%d documents) and does not load: %v", len(onDisk), err)
	}
	byOp := c.ByOperator()
	flat := flatInstances(t, root, c.Files)
	t.Logf("corpus: %d live files of %d on disk, %d instances, %d operators "+
		"(a flat walk of the same documents finds %d)",
		len(c.Files), len(onDisk), len(c.Instances), len(byOp), flat)
	for _, op := range sortedOperators(byOp) {
		t.Logf("    %-20s %4d", op, byOp[op])
	}

	// **The floor, standing in for `len(c.Files) == 41`.** Its job is the
	// loader: `jetstoreOwnedAssets` excludes what the installer's manifest
	// names, and a bug there -- a manifest that names everything, a comparison
	// that matches on the wrong field -- empties the corpus silently. 35 is
	// below every measurement this walk has produced (41 today, 45 and 49 under
	// two earlier definitions) and far above what any exclusion bug would leave.
	// **It is deliberately not tight.** A floor that tracks the corpus is an
	// equality wearing a different operator, and would fail on the next asset a
	// client adds. What makes the failure diagnosable rather than bare is the
	// second number: `pcFilesOnDisk` is what the walk was offered, so "loaded 3
	// of 41" names the exclusion and "loaded 3 of 3" names the checkout.
	if len(c.Files) < 35 {
		t.Errorf("the corpus loaded %d of the %d .pc.json on disk; the exclusion is dropping "+
			"authored configs (I-13's definition is workspaces/*/pipes_config/**, less what "+
			"the asset manifest names)", len(c.Files), len(onDisk))
	}
	// **The relation, standing in for `len(c.Instances) == 455`.** This is the
	// defect the instance count was really guarding, stated as the property it
	// is: the walk must find strictly more than a flat one does. Both numbers
	// are measured from the same 41 documents in the same run, so the assertion
	// cannot go stale -- the corpus can grow, shrink or be re-partitioned and
	// the relation holds as long as the walk still reaches what a flat one
	// cannot. It fails the moment somebody reduces `instancesIn` to one authored
	// shape, which is what the first version of it was.
	if flat >= len(c.Instances) {
		t.Errorf("the nested walk finds %d instances and a flat walk of the same documents finds "+
			"%d; the walk is no longer reaching the apply arrays under conditional overrides "+
			"(Instance, corpus.go)", len(c.Instances), flat)
	}
	// **"ollama" became "infer" on 2026-09-05** when patient_profile.pc.json moved to
	// the backend-agnostic infer operator, which ResolveInferBackend rewrites into an
	// ollama or vllm step at startup. The corpus records what was *authored*, so the
	// only raw ollama instance stopped existing and this canary had to move with it.
	//
	// The count did not change -- 459 either way -- because one operator became
	// another rather than being added or removed, which is why this list caught it and
	// the instance assertion did not. **That is the argument for the canaries
	// outliving the literals rather than an aside**: they name what must be
	// reachable, and a name is not a quantity. infer sits in the same nested apply
	// array ollama did, so a walk that misses nested arrays still fails here.
	for _, op := range []string{"map_record", "partition_writer", "infer", "high_freq", "distinct"} {
		if byOp[op] == 0 {
			t.Errorf("%s has no instances; nested apply arrays are not being walked", op)
		}
	}
	// The skew the reporting rules exist because of — asserted, so that if it
	// ever stops being true the rules can be revisited deliberately. A ratio
	// rather than a count, which is why it has never gone stale.
	head := byOp["map_record"] + byOp["partition_writer"]
	if head*100/len(c.Instances) < 70 {
		t.Errorf("the two head operators are %d%% of %d instances; the skew decision 13 assumes has changed",
			head*100/len(c.Instances), len(c.Instances))
	}
}

// flatInstances counts the transformation instances a *flat* walk of the same
// documents finds, and it is the right-hand side of the relation above.
//
// **It is a reconstruction of the walk that was wrong, not an invented weaker
// one.** `Instance`'s doc block (corpus.go) records that the first version read
// only the top-level pipes of the authored shapes, found 257 instances across 8
// operators, and undercounted by 44%. This reproduces that exactly -- 257 across
// 8 on 2026-09-11 -- by descending through *array* structure from the two
// authored roots and stopping at the first object key that is not `apply`. That
// is the mistake in one sentence: `reducing_pipes_config` is an array of arrays
// of pipes, so following arrays alone reaches all 257 of its instances, while
// `conditional_pipes_config[i].pipes_config[j].apply` is behind a named key and
// is missed entirely. Today that is 196 instances in 25 of the 41 documents.
//
// **Reproducing the historical number is what makes the relation worth
// asserting.** A flat walk defined as "find nothing" would satisfy `flat <
// nested` forever and prove nothing; this one is the specific undercount the
// count existed to catch, and it is recomputed on every run rather than
// remembered.
func flatInstances(t *testing.T, root string, files []string) int {
	t.Helper()
	total := 0
	for _, rel := range files {
		raw, err := os.ReadFile(filepath.Join(root, rel))
		if err != nil {
			t.Fatalf("re-reading %s for the flat walk: %v", rel, err)
		}
		total += flatInstancesIn(t, raw)
	}
	return total
}

// flatInstancesIn is the flat walk of one document. It is split out so that the
// fixture test can pin it to an exact number, which is the only place an exact
// number belongs.
func flatInstancesIn(t *testing.T, raw []byte) int {
	t.Helper()
	var doc map[string]any
	if err := json.Unmarshal(raw, &doc); err != nil {
		t.Fatalf("the flat walk cannot parse the document: %v", err)
	}
	total := 0
	var descend func(node any)
	descend = func(node any) {
		switch n := node.(type) {
		case []any:
			for _, e := range n {
				descend(e)
			}
		case map[string]any:
			// The pipe's own apply array, and no further: an object key is
			// exactly the hop this walk does not make, and neither is a
			// recursion into an apply element.
			if apply, ok := n["apply"].([]any); ok {
				for _, e := range apply {
					if m, ok := e.(map[string]any); ok {
						if op, ok := m["type"].(string); ok && op != "" {
							total++
						}
					}
				}
			}
		}
	}
	for _, authoredRoot := range []string{"conditional_pipes_config", "reducing_pipes_config"} {
		descend(doc[authoredRoot])
	}
	return total
}

// pcFilesOnDisk is every .pc.json under the workspaces, before any exclusion.
//
// It exists so that "there is no corpus in this checkout" and "the loader
// returned nothing" are different outcomes: the first is a prose-only worktree
// and skips, the second is the bug the floor is for and fails. Reading both off
// `len(c.Files)`, which is what this test did until 2026-09-11, makes an
// exclusion bug indistinguishable from a partial checkout -- and the partial
// checkout is the common case, so the tie would be broken the wrong way.
func pcFilesOnDisk(t *testing.T, root string) []string {
	t.Helper()
	dirs, err := filepath.Glob(filepath.Join(root, "workspaces", "*", "pipes_config"))
	if err != nil {
		t.Fatal(err)
	}
	var out []string
	for _, dir := range dirs {
		err := filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
			if err != nil || d.IsDir() || !strings.HasSuffix(path, ".pc.json") {
				return err
			}
			out = append(out, path)
			return nil
		})
		if err != nil {
			t.Fatalf("walking %s: %v", dir, err)
		}
	}
	return out
}

func sortedOperators(byOp map[string]int) []string {
	out := make([]string, 0, len(byOp))
	for op := range byOp {
		out = append(out, op)
	}
	sort.Slice(out, func(i, j int) bool {
		if byOp[out[i]] != byOp[out[j]] {
			return byOp[out[i]] > byOp[out[j]]
		}
		return out[i] < out[j]
	})
	return out
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

// --- P.1's additions: filling a hole, and saying why one was not filled ----

// The corpus API could cut a hole and could not fill one, which is what the
// first caller needed: a compile-pass gate judges a whole config, and a
// proposed transformation is not one.
func TestCase_FillPutsAnAnswerBackWhereTheInstanceWas(t *testing.T) {
	inst := Instance{
		File: "t.pc.json", Operator: "analyze",
		Path:  []Step{key("conditional_pipes_config"), at(0), key("apply")},
		Index: 1,
	}
	c, err := MakeCase([]byte(twoPipes), inst)
	if err != nil {
		t.Fatal(err)
	}
	if c.Hole.Index != 1 || len(c.Hole.Path) != 3 {
		t.Fatalf("the case does not carry the hole it cut: %+v", c.Hole)
	}
	filled, err := c.Fill(json.RawMessage(`{"type":"analyze","note":"proposed"}`))
	if err != nil {
		t.Fatal(err)
	}
	var doc map[string]any
	if err := json.Unmarshal(filled, &doc); err != nil {
		t.Fatal(err)
	}
	apply := doc["conditional_pipes_config"].([]any)[0].(map[string]any)["apply"].([]any)
	if len(apply) != 2 {
		t.Fatalf("filled apply array has %d entries, want 2", len(apply))
	}
	// Position matters: an answer appended to the end is a different config
	// from one restored where the instance was, and cpipes steps are ordered.
	if apply[0].(map[string]any)["type"] != "map_record" {
		t.Errorf("the answer displaced its sibling: %s", filled)
	}
	if apply[1].(map[string]any)["note"] != "proposed" {
		t.Errorf("the answer is not at the hole: %s", filled)
	}
}

// Filling the last position is legal and the naive bound refuses it: the index
// is a position in the original array and the context is that array one
// shorter, so index == len is exactly the case where the cut instance was last.
func TestCase_FillAcceptsTheLastPositionAndRefusesPastIt(t *testing.T) {
	c, err := MakeCase([]byte(twoPipes), Instance{
		File: "t.pc.json", Operator: "partition_writer",
		Path:  []Step{key("conditional_pipes_config"), at(1), key("apply")},
		Index: 0,
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := c.Fill(json.RawMessage(`{"type":"partition_writer"}`)); err != nil {
		t.Errorf("filling the only position of an emptied apply array must work: %v", err)
	}
	c.Hole.Index = 4
	if _, err := c.Fill(json.RawMessage(`{"type":"x"}`)); err == nil {
		t.Error("expected a position past the end to be refused")
	}
	c.Hole.Index = 0
	if _, err := c.Fill(json.RawMessage(`not json`)); err == nil {
		t.Error("expected a non-JSON answer to be refused rather than spliced in")
	}
}

// Two operators that were never attempted, for two different reasons, must not
// render as the same sentence.
func TestReport_DistinguishesNotRunByTheSplitFromNotRunByTheHarness(t *testing.T) {
	r := &Report{
		Era:          EraPreTemplates,
		Model:        "granite4.1:3b",
		CaseSource:   "mutation cases from workspaces/*/pipes_config/**",
		HeldOutFiles: []string{"a.pc.json"},
		Operators: []OperatorResult{
			{Operator: "merge", Attempted: 0, LiveInstances: 8},
			{Operator: "map_record", Attempted: 0, LiveInstances: 241,
				NotRun: "schema is ~28,754 tokens and does not fit the 32,768 context"},
		},
	}
	out := r.String()
	if !strings.Contains(out, "merge") || !strings.Contains(out, "8 live instances available") {
		t.Errorf("a split that held nothing out should say so:\n%s", out)
	}
	if !strings.Contains(out, "does not fit the 32,768 context") {
		t.Errorf("an operator the harness refused should say why:\n%s", out)
	}
}

// --- BB.3: the exact counts, on a fixture rather than on the corpus --------

// nestedShapes is the walker's fixture: both authored roots, both depths, and
// an `apply` inside an `apply`.
//
// **An exact count over a committed fixture is legitimate and an exact count
// over `workspaces/` is not**, which is the whole of the trade BB.3 makes. This
// document is this repository's; it changes when somebody means it to, and the
// numbers below move in the same commit. The corpus is four other repositories
// tracking real installations, and its numbers move on somebody else's Tuesday.
//
// It is not a realistic pipeline and is not trying to be. It carries one of each
// shape the walk has to reach:
//
//	conditional_pipes_config[0].apply           2  a pipe written at the root
//	conditional_pipes_config[1].pipes_config[]  3  behind a named key -- the miss
//	conditional_pipes_config[2].apply           3  an apply element with its own apply
//	reducing_pipes_config[0][].apply            3  an array of arrays of pipes
//
// The second row is the defect the retired instance count existed to catch, and
// the third is the one **nothing in the corpus exercises**: measured 2026-09-11,
// a walk that counts every `apply` array but never recurses *into* an apply
// element finds all 453 corpus instances, so `instancesIn`'s recursion there is
// covered by this fixture alone.
const nestedShapes = `{
  "conditional_pipes_config": [
    {"apply": [{"type":"map_record"},{"type":"partition_writer"}]},
    {"when": "$NBR_PARTITIONS > 1",
     "pipes_config": [
       {"apply": [{"type":"jetrules"},{"type":"partition_writer"}]},
       {"apply": [{"type":"infer"}]}
     ]},
    {"apply": [{"type":"fan_out","apply":[{"type":"map_record"},{"type":"sort"}]}]}
  ],
  "reducing_pipes_config": [
    [
      {"apply": [{"type":"merge"}]},
      {"apply": [{"type":"analyze"},{"type":"distinct"}]}
    ]
  ]
}`

// TestTheWalkerCountsNestedShapes is where the exact numbers live now.
//
// It is what recovers most of what TestAgainstTheRealCorpus gave up: a walker
// that miscounts by a constant, or that stops descending at one of the four
// shapes above, fails here by name and by operator rather than by a total nobody
// can place. What it cannot see is a shape the corpus has and this document does
// not -- R-112, accepted.
func TestTheWalkerCountsNestedShapes(t *testing.T) {
	got, err := instancesIn([]byte(nestedShapes), "nested.pc.json")
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 11 {
		t.Errorf("the walk finds %d instances, want 11", len(got))
	}
	byOp := map[string]int{}
	for _, i := range got {
		byOp[i.Operator]++
	}
	for op, want := range map[string]int{
		"map_record": 2, "partition_writer": 2, "jetrules": 1, "infer": 1,
		"fan_out": 1, "sort": 1, "merge": 1, "analyze": 1, "distinct": 1,
	} {
		if byOp[op] != want {
			t.Errorf("%s: %d instances, want %d", op, byOp[op], want)
		}
	}
	if len(byOp) != 9 {
		t.Errorf("the walk finds %d operators, want 9: %v", len(byOp), byOp)
	}
	// The five instances a flat walk reaches, and the six it does not. This is
	// the relation TestAgainstTheRealCorpus asserts against the live corpus,
	// pinned to an exact difference here -- so a change that makes the flat walk
	// and the real one agree fails in a file somebody can read, rather than
	// turning a corpus assertion green.
	if flat := flatInstancesIn(t, []byte(nestedShapes)); flat != 6 {
		t.Errorf("a flat walk of the fixture finds %d instances, want 6", flat)
	}

	// Every path resolves to the instance it claims. A count is only worth what
	// its paths are worth: MakeCase navigates each one and hands back the
	// element it cut, so a path that is right by accident of ordering fails
	// here. This is the assertion that covers the nested shapes' *locations*
	// rather than their number.
	for _, inst := range got {
		c, err := MakeCase([]byte(nestedShapes), inst)
		if err != nil {
			t.Errorf("%s at %s: %v", inst.Operator, pathString(inst), err)
			continue
		}
		if !strings.Contains(string(c.Expected), `"`+inst.Operator+`"`) {
			t.Errorf("%s at %s: cut %s", inst.Operator, pathString(inst), c.Expected)
		}
	}
}
