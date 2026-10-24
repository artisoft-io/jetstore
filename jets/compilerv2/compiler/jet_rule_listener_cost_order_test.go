package compiler

// THE COST ORDER (rules_benchmark Phase 9, BY.2).
//
// testdata/cost_order holds a rule set and its statistics, in jetstore's own
// vocabulary, with three rules whose best order is a property of the data:
//   - COST_A: a membership term against a join through a bound object, which
//     the score ties (260 each) and keeps as written;
//   - COST_B: a broad scan by type the score puts ahead of a selective join;
//   - COST_C: a long rule whose join on a shared bound value must come right
//     after the anchor's attributes.
// The tests assert each rule's cost order, that the score order differs, and
// what must not change: the fallback, determinism, and the re-validation.

import (
	"bytes"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"testing"

	"github.com/artisoft-io/jetstore/jets/jetrules/rete"
)

const costOrderBase = "testdata/cost_order"
const costOrderMain = "jet_rules/cost_order_main.jr"

// fixture copies the fixture's main rule file into a new workspace and, when
// withStats, its statistics file, which Compile then reads.
func fixture(t *testing.T, withStats bool) string {
	t.Helper()
	dir := t.TempDir()
	src, err := os.ReadFile(filepath.Join(costOrderBase, costOrderMain))
	if err != nil {
		t.Fatal(err)
	}
	writeFixture(t, dir, string(src))
	if withStats {
		stats, err := os.ReadFile(filepath.Join(costOrderBase, OptimizerStatisticsFile))
		if err != nil {
			t.Fatal(err)
		}
		write(t, dir, OptimizerStatisticsFile, string(stats))
	}
	return dir
}

// compileCostOrder compiles the fixture, with the fixture's statistics when
// withStats, and returns the compiler and its rules by name.
func compileCostOrder(t *testing.T, withStats bool) (*Compiler, map[string]*rete.JetruleNode) {
	t.Helper()
	return compileWorkspace(t, fixture(t, withStats), withStats)
}

func compileWorkspace(t *testing.T, dir string, trace bool) (*Compiler, map[string]*rete.JetruleNode) {
	t.Helper()
	c := NewCompiler(dir, costOrderMain, false, false, true)
	c.SetCostOrderTrace(trace)
	if err := c.Compile(); err != nil {
		t.Fatalf("%v\n%s", err, c.ErrorLog().String())
	}
	rules := make(map[string]*rete.JetruleNode)
	for _, r := range c.JetRuleModel().Jetrules {
		rules[r.Name] = r
	}
	return c, rules
}

// ruleOrder gives a rule's antecedents, in their compiled order, as its label
// writes them: with the variables' authored names, which the network's own
// variables do not keep.
func ruleOrder(rule *rete.JetruleNode) []string {
	body := strings.SplitN(strings.SplitN(rule.Label, "]:", 2)[1], " -> ", 2)[0]
	body = filterLabel.ReplaceAllString(body, "")
	return termLabel.FindAllString(body, -1)
}

var filterLabel = regexp.MustCompile(`\.\[[^\]]*\]`)
var termLabel = regexp.MustCompile(`(not)?\([^()\s]+ [^()\s]+ [^()\s]+\)`)

// authoredOrder gives a rule's antecedents in their written order, from the
// fixture compiled with the optimizer off.
func authoredOrder(t *testing.T, name string) []string {
	t.Helper()
	src, err := os.ReadFile(filepath.Join(costOrderBase, costOrderMain))
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	writeFixture(t, dir, strings.ReplaceAll(string(src), "["+name+"]", "["+name+", o=false]"))
	_, rules := compileWorkspace(t, dir, false)
	return ruleOrder(rules[name])
}

// writeFixture writes the fixture's main rule file, as src, under dir.
func writeFixture(t *testing.T, dir, src string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Join(dir, filepath.Dir(costOrderMain)), 0o755); err != nil {
		t.Fatal(err)
	}
	write(t, dir, costOrderMain, src)
}

func positions(authored []string, order ...int) []string {
	out := make([]string, len(order))
	for i, k := range order {
		out[i] = authored[k]
	}
	return out
}

func TestCostOrderFindsTheDataDependentOrders(t *testing.T) {
	cScore, byScore := compileCostOrder(t, false)
	cCost, byCost := compileCostOrder(t, true)
	cases := []struct {
		rule        string
		cost, score []int // authored positions
	}{
		// the join through ?code, then ?d's provider; the membership last, both ends bound
		{"COST_A", []int{0, 1, 3, 4, 2}, []int{0, 1, 2, 3, 4}},
		// the join through ?n before the scan by type
		{"COST_B", []int{0, 1, 3, 4, 2}, []int{0, 1, 3, 2, 4}},
		// the anchor's attributes, then the join on the service date, then the pairing term
		{"COST_C", []int{0, 3, 4, 5, 6, 1, 2, 7, 8}, []int{0, 3, 4, 5, 1, 6, 2, 7, 8}},
	}
	for _, tc := range cases {
		authored := authoredOrder(t, tc.rule)
		want := positions(authored, tc.cost...)
		if got := ruleOrder(byCost[tc.rule]); !slices.Equal(got, want) {
			t.Errorf("%s: cost order\n  got  %v\n  want %v", tc.rule, got, want)
		}
		wantScore := positions(authored, tc.score...)
		gotScore := ruleOrder(byScore[tc.rule])
		if !slices.Equal(gotScore, wantScore) {
			t.Errorf("%s: score order\n  got  %v\n  want %v", tc.rule, gotScore, wantScore)
		}
		if slices.Equal(gotScore, want) {
			t.Errorf("%s: the score order already is the cost order; the fixture tests nothing", tc.rule)
		}
		checkBindingOrder(t, cCost, byCost[tc.rule])
	}

	// COST_A's choice is a tie in the score: the membership term and the join
	// through ?code both score 260 once ?c and ?code are bound
	l := cScore.listener
	order := ruleOrder(byScore["COST_A"])
	bound := map[string]bool{}
	var membership, join *rete.RuleTerm
	for i, a := range byScore["COST_A"].Antecedents {
		switch order[i] {
		case "(?c rdf:type acme:Claim)", "(?c acme:code ?code)":
			l.collectVars(bound, a)
		case "(acme:Config acme:watchedProvider ?p)":
			membership = a
		case "(?d acme:code ?code)":
			join = a
		}
	}
	if membership == nil || join == nil {
		t.Fatalf("COST_A: terms not found in %v", order)
	}
	if m, j := l.evaluateScore(bound, membership), l.evaluateScore(bound, join); m != 260 || j != 260 {
		t.Errorf("COST_A: the membership term scores %d and the join %d, not 260 each", m, j)
	}

	// The report, and the trace naming positions only
	log := cCost.ParseLog().String()
	for _, want := range []string{
		"Cost order: rule set jet_rules/cost_order_main.jr, statistics jet_rules/optimizer_statistics.json: " +
			"3 rules cost-ordered (3 differ from the score order); fell back to the score order: " +
			"0 with a variable predicate, 1 with a predicate not in the file; 1 with o=false",
		"  COST_A (jet_rules/cost_order_main.jr): score order T0 T1 T2 T3 T4, est. rows held 84102000; cost order T0 T1 T3 T4 T2, est. rows held 62000",
	} {
		if !strings.Contains(log, want) {
			t.Errorf("the parse log does not say\n  %s\nit says\n%s", want, log)
		}
	}
	if strings.Contains(cScore.ParseLog().String(), "Cost order") {
		t.Errorf("a compile without statistics reports the cost order:\n%s", cScore.ParseLog().String())
	}
}

// A rule the statistics do not cover keeps the score order, and so does every
// rule of a rule set the file does not have.
func TestCostOrderFallsBackToTheScore(t *testing.T) {
	_, byScore := compileCostOrder(t, false)
	_, byCost := compileCostOrder(t, true)
	for _, name := range []string{"COST_FALLBACK", "COST_OFF"} {
		if got, want := ruleOrder(byCost[name]), ruleOrder(byScore[name]); !slices.Equal(got, want) {
			t.Errorf("%s: with statistics %v, without %v", name, got, want)
		}
	}

	// The rule set absent from the file: every rule as without statistics
	dir := fixture(t, true)
	stats, err := os.ReadFile(filepath.Join(dir, OptimizerStatisticsFile))
	if err != nil {
		t.Fatal(err)
	}
	write(t, dir, OptimizerStatisticsFile, strings.Replace(string(stats), costOrderMain, "jet_rules/another_main.jr", 1))
	c, rules := compileWorkspace(t, dir, true)
	for name, r := range rules {
		if got, want := ruleOrder(r), ruleOrder(byScore[name]); !slices.Equal(got, want) {
			t.Errorf("%s: rule set not in the file %v, without statistics %v", name, got, want)
		}
	}
	if !strings.Contains(c.ParseLog().String(), "Cost order: rule set jet_rules/cost_order_main.jr is not in jet_rules/optimizer_statistics.json; rules ordered by score") {
		t.Errorf("the parse log does not report the rule set as absent:\n%s", c.ParseLog().String())
	}
}

// A statistics file the compiler cannot read refuses the compile, naming it.
func TestUnreadableStatisticsRefuseTheCompile(t *testing.T) {
	good, err := os.ReadFile(filepath.Join(costOrderBase, OptimizerStatisticsFile))
	if err != nil {
		t.Fatal(err)
	}
	cases := []struct{ name, content, want string }{
		{"version", strings.Replace(string(good), `"version": 1`, `"version": 2`, 1), "version 2 is not supported"},
		{"format", strings.Replace(string(good), `"jetstore.optimizer_statistics"`, `"something.else"`, 1), "not a statistics file"},
		{"malformed", string(good[:len(good)/2]), "malformed statistics file"},
	}
	for _, tc := range cases {
		dir := fixture(t, false)
		write(t, dir, OptimizerStatisticsFile, tc.content)
		c := NewCompiler(dir, costOrderMain, false, false, true)
		err := c.Compile()
		if err == nil {
			t.Errorf("%s: the compile succeeded", tc.name)
			continue
		}
		if !strings.Contains(err.Error(), filepath.Join(dir, OptimizerStatisticsFile)) || !strings.Contains(err.Error(), tc.want) {
			t.Errorf("%s: the error does not name the file and say %q: %v", tc.name, tc.want, err)
		}
	}
}

// Two compiles with the same statistics write the same workspace.db, and the
// cost order is in it: it differs from the one written without statistics.
func TestCostOrderIsDeterministic(t *testing.T) {
	saved := workspaceSchema
	workspaceSchema = filepath.Join("..", "..", "workspace_schema.sql")
	t.Cleanup(func() { workspaceSchema = saved })

	compileDB := func(withStats bool) []byte {
		dir := fixture(t, withStats)
		c := NewCompiler(dir, costOrderMain, true, false, true)
		if err := c.Compile(); err != nil {
			t.Fatalf("%v\n%s", err, c.ErrorLog().String())
		}
		db, err := os.ReadFile(filepath.Join(dir, "workspace.db"))
		if err != nil {
			t.Fatal(err)
		}
		return db
	}
	first := compileDB(true)
	for i := 0; i < 4; i++ {
		if got := compileDB(true); !bytes.Equal(first, got) {
			t.Fatalf("compile %d with statistics wrote a different workspace.db (%d and %d bytes)", i+2, len(first), len(got))
		}
	}
	if bytes.Equal(first, compileDB(false)) {
		t.Fatal("the workspace.db written with statistics is the one written without them")
	}
}

// The re-validation refuses an order that reads a variable before it is
// bound, whoever chose it. The hook puts a negation first, as the optimizer
// before jetstore#2357 could (I-232).
func TestReorderedRuleIsValidatedAgain(t *testing.T) {
	src := `
		[NEG_FIRST]:
		(?c acme:amount ?v).
		not(?c rdf:type acme:Closed)
		->
		(?c acme:openAmount ?v);
	`
	reorderHookForTest = func(rule *rete.JetruleNode) {
		for i, a := range rule.Antecedents {
			if a.IsNot {
				rule.Antecedents = append([]*rete.RuleTerm{a}, slices.Delete(slices.Clone(rule.Antecedents), i, i+1)...)
				return
			}
		}
	}
	t.Cleanup(func() { reorderHookForTest = nil })
	c := NewCompiler("./testdata", "neg_first.jr", false, false, true)
	err := c.CompileBuffer("@JetCompilerDirective source_file = \"neg_first.jr\";\n" + src)
	if err == nil {
		t.Fatal("a negation placed before its binding compiled")
	}
	want := "rule NEG_FIRST (neg_first.jr): after ordering, negated antecedent 0 reads variable ?c before an antecedent binds it"
	if !strings.Contains(c.ErrorLog().String(), want) {
		t.Errorf("the error log does not say\n  %s\nit says\n%s", want, c.ErrorLog().String())
	}

	// And the same rule, left as the optimizer orders it, compiles
	reorderHookForTest = nil
	c = NewCompiler("./testdata", "neg_first.jr", false, false, true)
	if err := c.CompileBuffer("@JetCompilerDirective source_file = \"neg_first.jr\";\n" + src); err != nil {
		t.Fatalf("%v\n%s", err, c.ErrorLog().String())
	}
}

func TestTermKey(t *testing.T) {
	cases := []struct {
		r   rete.ResourceNode
		key string
		ok  bool
	}{
		{rete.ResourceNode{Type: "resource", Value: "acme:Claim"}, "acme:Claim", true},
		{rete.ResourceNode{Type: "volatile_resource", Value: "_0:tmp"}, "_0:tmp", true},
		{rete.ResourceNode{Type: "volatile_resource", Value: "tmp"}, "_0:tmp", true},
		{rete.ResourceNode{Type: "text", Value: `a "b"`}, `"a \"b\""`, true},
		{rete.ResourceNode{Type: "int", Value: "+7"}, "7^^int", true},
		{rete.ResourceNode{Type: "int", Value: "-12"}, "-12^^int", true},
		{rete.ResourceNode{Type: "long", Value: "7"}, "", false},
		{rete.ResourceNode{Type: "double", Value: "1.5"}, "", false},
		{rete.ResourceNode{Type: "keyword", Value: "true"}, "", false},
		{rete.ResourceNode{Type: "date", Value: "2026-10-24"}, "", false},
		{rete.ResourceNode{Type: "var", Value: "?x"}, "", false},
	}
	for _, tc := range cases {
		key, ok := termKey(&tc.r)
		if key != tc.key || ok != tc.ok {
			t.Errorf("termKey(%s|%s) = %q, %v; want %q, %v", tc.r.Type, tc.r.Value, key, ok, tc.key, tc.ok)
		}
	}
}
