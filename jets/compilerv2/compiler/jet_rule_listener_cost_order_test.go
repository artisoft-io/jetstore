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
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"testing"

	"github.com/artisoft-io/jetstore/jets/jetrules/rete"
	"github.com/artisoft-io/jetstore/jets/optstats"
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

// A term's subject, predicate or object: a name, or a literal written as
// type(value), as text(open) or int(3).
const labelToken = `(?:\w+\([^()]*\)|[^()\s]+)`

var termLabel = regexp.MustCompile(`(not)?\(` + labelToken + ` ` + labelToken + ` ` + labelToken + `\)`)

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
			"0 with a variable predicate, 1 with a predicate not in the file; 1 with o=false; 0 of one antecedent",
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

// THE CASES THE FIXTURE DOES NOT REACH (BY.2's review, 2026-10-24), each with
// statistics built here rather than read from a file.

// compileWithStatistics compiles src with st as its rule set's statistics.
func compileWithStatistics(t *testing.T, name, src string, st *optstats.RuleSet) (*Compiler, map[string]*rete.JetruleNode) {
	t.Helper()
	c := NewCompiler("./testdata", name, false, false, true)
	if st != nil {
		c.SetOptimizerStatistics("test statistics", st)
	}
	if err := c.CompileBuffer(fmt.Sprintf("@JetCompilerDirective source_file = %q;\n%s", name, src)); err != nil {
		t.Fatalf("%v\n%s", err, c.ErrorLog().String())
	}
	rules := make(map[string]*rete.JetruleNode)
	for _, r := range c.JetRuleModel().Jetrules {
		rules[r.Name] = r
	}
	return c, rules
}

// ruleSetStatistics builds one session's statistics: predicates as
// {triples, subjects, objects}, and the constant pairs.
func ruleSetStatistics(sessions int64, preds map[string][3]int64, subj, obj map[string]map[string]int64) *optstats.RuleSet {
	rs := optstats.NewRuleSet()
	rs.Sessions = sessions
	for p, c := range preds {
		rs.Predicates[p] = &optstats.Predicate{Triples: c[0], Subjects: c[1], Objects: c[2]}
	}
	for p, m := range subj {
		rs.SubjectConstants[p] = m
	}
	for p, m := range obj {
		rs.ObjectConstants[p] = m
	}
	return rs
}

// firstEstimates gives each antecedent's estimate with nothing bound, by its
// label: what the cost order compares for the first place.
func firstEstimates(c *Compiler, rule *rete.JetruleNode) map[string]float64 {
	e := c.listener.newCostEstimator(rule.Antecedents)
	out := make(map[string]float64)
	for i, label := range ruleOrder(rule) {
		out[label] = e.rows(rule.Antecedents[i], map[string]bool{})
	}
	return out
}

// A constant whose pair the file does not have is estimated from its
// predicate's averages, N/DS for a constant subject and N/DO for a constant
// object (BX.1 rule 7), not as 0 and not as N. With the pairs, each rule's
// constant term is dearer than (?a acme:wide ?b), 500 rows a session; with the
// averages it is cheaper, and goes first.
func TestCostEstimateTakesTheAveragesForAMissingPair(t *testing.T) {
	src := `
		[SUBJ]:
		(?a acme:wide ?b).
		(acme:Config acme:lists ?a)
		->
		(?a acme:listed ?b);

		[OBJ]:
		(?a acme:wide ?b).
		(?a acme:kind acme:Open)
		->
		(?a acme:open ?b);
	`
	preds := map[string][3]int64{
		"acme:wide":  {1000, 1000, 1000}, // 500 a session
		"acme:lists": {1600, 8, 1600},    // N/DS = 800/4 = 200
		"acme:kind":  {2000, 2000, 8},    // N/DO = 1000/4 = 250
	}
	withPairs := ruleSetStatistics(2, preds,
		map[string]map[string]int64{"acme:lists": {"acme:Config": 1800}},
		map[string]map[string]int64{"acme:kind": {"acme:Open": 1200}})
	without := ruleSetStatistics(2, preds, nil, nil)

	cases := []struct {
		rule, term string
		pair, avg  float64
	}{
		{"SUBJ", "(acme:Config acme:lists ?a)", 900, 200},
		{"OBJ", "(?a acme:kind acme:Open)", 600, 250},
	}
	cWith, byWith := compileWithStatistics(t, "pairs.jr", src, withPairs)
	cWithout, byWithout := compileWithStatistics(t, "pairs.jr", src, without)
	for _, tc := range cases {
		if got := firstEstimates(cWith, byWith[tc.rule])[tc.term]; got != tc.pair {
			t.Errorf("%s: with the pair, %s is estimated at %v, want %v", tc.rule, tc.term, got, tc.pair)
		}
		if got := firstEstimates(cWithout, byWithout[tc.rule])[tc.term]; got != tc.avg {
			t.Errorf("%s: without the pair, %s is estimated at %v, want %v", tc.rule, tc.term, got, tc.avg)
		}
		if got := ruleOrder(byWith[tc.rule]); got[0] != "(?a acme:wide ?b)" {
			t.Errorf("%s: with the pair the order is %v; the membership term should come second", tc.rule, got)
		}
		if got := ruleOrder(byWithout[tc.rule]); got[0] != tc.term {
			t.Errorf("%s: without the pair the order is %v; %s should come first", tc.rule, got, tc.term)
		}
	}
}

// A text and an int constant are looked up by their keys, "\"open\"" and
// "3^^int", through ObjectCount, and the count decides the order.
func TestCostOrderLooksUpTextAndIntConstants(t *testing.T) {
	src := `
		[TEXT]:
		(?a acme:wide ?b).
		(?a acme:status "open")
		->
		(?a acme:isOpen ?b);

		[INT]:
		(?a acme:wide ?b).
		(?a acme:count 3)
		->
		(?a acme:hasThree ?b);
	`
	preds := map[string][3]int64{
		"acme:wide":   {1000, 1000, 1000}, // 500 a session
		"acme:status": {2000, 2000, 2},    // N/DO = 1000/1 = 1000
		"acme:count":  {2000, 2000, 2},
	}
	withPairs := ruleSetStatistics(2, preds, nil, map[string]map[string]int64{
		"acme:status": {`"open"`: 20},
		"acme:count":  {"3^^int": 20},
	})
	without := ruleSetStatistics(2, preds, nil, nil)
	cases := []struct{ rule, term string }{
		{"TEXT", `(?a acme:status text(open))`},
		{"INT", `(?a acme:count int(3))`},
	}
	cWith, byWith := compileWithStatistics(t, "literals.jr", src, withPairs)
	_, byWithout := compileWithStatistics(t, "literals.jr", src, without)
	for _, tc := range cases {
		if got := firstEstimates(cWith, byWith[tc.rule])[tc.term]; got != 10 {
			t.Errorf("%s: %s is estimated at %v, want its pair's 10", tc.rule, tc.term, got)
		}
		if got := ruleOrder(byWith[tc.rule]); got[0] != tc.term {
			t.Errorf("%s: with the pair the order is %v; %s should come first", tc.rule, got, tc.term)
		}
		if got := ruleOrder(byWithout[tc.rule]); got[0] != "(?a acme:wide ?b)" {
			t.Errorf("%s: without the pair the order is %v; %s should come second", tc.rule, got, tc.term)
		}
	}
}

// A rule with a variable predicate keeps the score order, though the
// statistics cover its other terms, and the report counts it.
func TestCostOrderLeavesAVariablePredicateToTheScore(t *testing.T) {
	src := `
		[VARP]:
		(?a acme:wide ?b).
		(?a ?p ?v).
		(acme:Config acme:lists ?a)
		->
		(?a acme:has ?v);
	`
	st := ruleSetStatistics(1, map[string][3]int64{
		"acme:wide":  {10, 10, 10},
		"acme:lists": {1000, 1, 1000},
	}, map[string]map[string]int64{"acme:lists": {"acme:Config": 1000}}, nil)
	_, byScore := compileWithStatistics(t, "varp.jr", src, nil)
	c, byCost := compileWithStatistics(t, "varp.jr", src, st)
	if got, want := ruleOrder(byCost["VARP"]), ruleOrder(byScore["VARP"]); !slices.Equal(got, want) {
		t.Errorf("with statistics %v, without %v", got, want)
	}
	if want := "0 rules cost-ordered (0 differ from the score order); fell back to the score order: 1 with a variable predicate"; !strings.Contains(c.ParseLog().String(), want) {
		t.Errorf("the parse log does not say\n  %s\nit says\n%s", want, c.ParseLog().String())
	}
}

// The re-validation refuses a filter placed before an antecedent binds its
// variables, as it refuses a negation (TestReorderedRuleIsValidatedAgain).
func TestReorderedFilterIsValidatedAgain(t *testing.T) {
	src := `
		[FILTER_EARLY]:
		(?a acme:amount ?v).
		(?a acme:limit ?l).
		[?v > ?l]
		->
		(?a acme:over ?v);
	`
	// Whichever antecedent the optimizer puts first binds one of ?v and ?l,
	// and the filter, moved onto it, reads the other: which one is the score's
	// choice, so the test does not name it
	reorderHookForTest = func(rule *rete.JetruleNode) {
		var filter *rete.ExpressionNode
		for _, a := range rule.Antecedents {
			if a.Filter != nil {
				filter, a.Filter = a.Filter, nil
			}
		}
		rule.Antecedents[0].Filter = filter
	}
	t.Cleanup(func() { reorderHookForTest = nil })
	c := NewCompiler("./testdata", "filter_early.jr", false, false, true)
	err := c.CompileBuffer("@JetCompilerDirective source_file = \"filter_early.jr\";\n" + src)
	if err == nil {
		t.Fatal("a filter placed before its binding compiled")
	}
	want := "rule FILTER_EARLY (filter_early.jr): after ordering, the filter of antecedent 0 reads variable ?"
	if !strings.Contains(c.ErrorLog().String(), want) {
		t.Errorf("the error log does not say\n  %s\nit says\n%s", want, c.ErrorLog().String())
	}

	reorderHookForTest = nil
	c = NewCompiler("./testdata", "filter_early.jr", false, false, true)
	if err := c.CompileBuffer("@JetCompilerDirective source_file = \"filter_early.jr\";\n" + src); err != nil {
		t.Fatalf("%v\n%s", err, c.ErrorLog().String())
	}
}

// The trace is on for JETS_COST_ORDER_TRACE=1 only.
func TestCostOrderTraceEnv(t *testing.T) {
	for _, tc := range []struct {
		value string
		on    bool
	}{{"1", true}, {"0", false}, {"true", false}, {"", false}} {
		t.Setenv(CostOrderTraceEnv, tc.value)
		c := NewCompiler(fixture(t, true), costOrderMain, false, false, true)
		if err := c.Compile(); err != nil {
			t.Fatal(err)
		}
		if got := strings.Contains(c.ParseLog().String(), "  COST_A ("); got != tc.on {
			t.Errorf("%s=%q: the trace is on %v, want %v", CostOrderTraceEnv, tc.value, got, tc.on)
		}
	}
}

// The report counts every valid rule of the rule set once: cost-ordered, fallen
// back by reason, o=false, or of one antecedent, which has no order to choose.
func TestCostOrderReportCountsEveryRule(t *testing.T) {
	src := `
		[ORDERED]:
		(?a acme:wide ?b).
		(acme:Config acme:lists ?a)
		->
		(?a acme:listed ?b);

		[VAR_PRED]:
		(?a acme:wide ?b).
		(?a ?p ?v)
		->
		(?a acme:has ?v);

		[NOT_IN_FILE]:
		(?a acme:wide ?b).
		(?a acme:unlisted ?v)
		->
		(?a acme:unlistedValue ?v);

		[OFF, o=false]:
		(?a acme:wide ?b).
		(acme:Config acme:lists ?a)
		->
		(?a acme:listedToo ?b);

		[SINGLE]:
		(?a acme:wide ?b)
		->
		(?a acme:isWide ?b);
	`
	st := ruleSetStatistics(1, map[string][3]int64{
		"acme:wide":  {500, 500, 500},
		"acme:lists": {1000, 1, 1000},
	}, map[string]map[string]int64{"acme:lists": {"acme:Config": 10}}, nil)
	c, rules := compileWithStatistics(t, "report.jr", src, st)
	want := "1 rules cost-ordered (0 differ from the score order); fell back to the score order: " +
		"1 with a variable predicate, 1 with a predicate not in the file; 1 with o=false; 1 of one antecedent"
	if !strings.Contains(c.ParseLog().String(), want) {
		t.Fatalf("the parse log does not say\n  %s\nit says\n%s", want, c.ParseLog().String())
	}
	r := c.listener.costReport
	if sum := r.costOrdered + r.varPred + r.missingPred + r.optOff + r.single; sum != len(rules) {
		t.Errorf("the report counts %d rules of %d", sum, len(rules))
	}
}
