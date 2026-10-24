package compiler

// WHAT THE RULE OPTIMIZER MUST KEEP (rules_benchmark Phase 8, task BS.1).
//
// OptimizeJetruleNode reorders a rule's antecedents, re-places its filters and renames
// its variables, after validation has checked the rule in its authored order. It is on
// for every rule that does not set o=false, which is most rules in production. These
// tests pin what a reordering must not change: a negated term comes after the terms
// that bind its variables, a filter's variables are bound where it lands, the
// consequents name the antecedents' variables, and terms the heuristic scores equally
// keep their authored order. Each checks the property on the optimized rule, rather
// than a label, so that a heuristic that orders differently still passes.

import (
	"fmt"
	"strings"
	"testing"

	"github.com/artisoft-io/jetstore/jets/jetrules/rete"
)

// compileForOptimizer compiles src and returns the compiler and its rules, failing the
// test on a compile error.
func compileForOptimizer(t *testing.T, name, src string) (*Compiler, []*rete.JetruleNode) {
	t.Helper()
	c := NewCompiler("./testdata", name, false, false, true)
	if err := c.CompileBuffer(fmt.Sprintf("@JetCompilerDirective source_file = %q;\n%s", name, src)); err != nil {
		t.Fatal(err.Error())
	}
	if c.ErrorLog().Len() > 0 {
		t.Fatal(c.ErrorLog().String())
	}
	return c, c.JetRuleModel().Jetrules
}

// Variables are named here, not keyed: once the network is built, each occurrence of a
// variable is a resource of its own, and the name is what they share.

// termVars gives the names of a term's variables, in subject, predicate, object order.
func termVars(c *Compiler, term *rete.RuleTerm) []string {
	names := make([]string, 0, 3)
	for _, k := range []int{term.SubjectKey, term.PredicateKey, term.ObjectKey} {
		if r := c.listener.resourceManager.ResourceByKey[k]; k > 0 && r != nil && r.Type == "var" {
			names = append(names, r.Id)
		}
	}
	return names
}

// exprVars gives the names of the variables an expression reads.
func exprVars(c *Compiler, expr *rete.ExpressionNode) map[string]bool {
	keys := make(map[int]bool)
	c.listener.collectVarResourcesFromExpr(expr, keys)
	names := make(map[string]bool)
	for k := range keys {
		names[c.listener.resourceManager.ResourceByKey[k].Id] = true
	}
	return names
}

// checkBindingOrder reports each negated term that comes before a positive term binding
// one of its variables, and each filter that reads a variable no positive term at or
// before it binds. Only a positive term binds: a negated term matches nothing.
func checkBindingOrder(t *testing.T, c *Compiler, rule *rete.JetruleNode) {
	t.Helper()
	bound := make(map[string]bool)
	for i, a := range rule.Antecedents {
		if a.IsNot {
			for _, v := range termVars(c, a) {
				if !bound[v] {
					t.Errorf("%s: antecedent %d, negated, reads %s before a positive term binds it\n  %s",
						rule.Name, i, v, rule.NormalizedLabel)
				}
			}
		} else {
			for _, v := range termVars(c, a) {
				bound[v] = true
			}
		}
		if a.Filter != nil {
			for v := range exprVars(c, a.Filter) {
				if !bound[v] {
					t.Errorf("%s: antecedent %d's filter reads %s, which no positive term at or before it binds\n  %s",
						rule.Name, i, v, rule.NormalizedLabel)
				}
			}
		}
	}
}

// I-200: a negated term the heuristic scores above the term that binds its variable.
// not(?c rdf:type ex:Closed) scores 380 with ?c unbound, for rdf:type's bonus, and
// (?c ex:amount ?v) 180, so a reordering by score alone puts the negation first.
func TestOptimizerKeepsANegatedTermAfterItsBindings(t *testing.T) {
	c, rules := compileForOptimizer(t, "opt_negation.jr", `
		[NEG1]:
		(?c ex:amount ?v).
		not(?c rdf:type ex:Closed)
		->
		(?c ex:open_amount ?v);

		[NEG2]:
		(?c rdf:type ex:Claim).
		(?c ex:member ?m).
		not(?m rdf:type ex:Excluded).
		(?c ex:amount ?v)
		->
		(?c ex:kept ?v);

		[NEG3]:
		(?c ex:member ?m).
		(?m ex:plan ?p).
		not(?p ex:status ex:Inactive)
		->
		(?c ex:active_plan ?p);`)
	for _, rule := range rules {
		checkBindingOrder(t, c, rule)
	}
}

// A filter lands on the first antecedent at which its variables are all bound. A filter
// on ?c moved ahead with a negation that ?c is "bound" by reads ?c before any positive
// term binds it.
func TestOptimizerPlacesAFilterWhereItsVariablesAreBound(t *testing.T) {
	c, rules := compileForOptimizer(t, "opt_filter.jr", `
		[FLT1]:
		(?a rdf:type ex:Item).
		(?b rdf:type ex:Item).
		(?a ex:p ?v).
		[?a != ?v]
		(?b ex:p ?v).
		[?b != ?a]
		->
		(?a ex:pair ?b);

		[FLT2]:
		(?c ex:amount ?v).
		[?c != ex:excluded]
		not(?c rdf:type ex:Closed)
		->
		(?c ex:open_amount ?v);

		[FLT3]:
		(?c ex:amount ?v).
		(?c ex:limit ?l).
		[?v > ?l]
		(?c rdf:type ex:Claim)
		->
		(?c ex:over_limit true);`)
	for _, rule := range rules {
		checkBindingOrder(t, c, rule)
	}
}

// Renaming gives each variable a new name everywhere it occurs: the consequents read the
// names the antecedents bind, in a triple and in an object expression.
func TestOptimizerKeepsTheConsequentsVariables(t *testing.T) {
	c, rules := compileForOptimizer(t, "opt_consequents.jr", `
		[CQ1]:
		(?c ex:amount ?a).
		(?c ex:fee ?f).
		(?m ex:claim ?c).
		(?c rdf:type ex:Claim)
		->
		(?m ex:total (?a + ?f)).
		(?c ex:member ?m);`)
	rule := rules[0]
	bound := make(map[string]bool)
	for _, a := range rule.Antecedents {
		for _, v := range termVars(c, a) {
			bound[v] = true
		}
	}
	for i, cq := range rule.Consequents {
		reads := exprVars(c, cq.ObjectExpr)
		for _, v := range termVars(c, cq) {
			reads[v] = true
		}
		for v := range reads {
			if !bound[v] {
				t.Errorf("consequent %d reads %s, which no antecedent binds\n  %s", i, v, rule.NormalizedLabel)
			}
		}
	}
	// and the label is the rule as it now stands: (?m ex:total (?a + ?f)) with the names
	// the antecedents give ?m, ?a and ?f
	var m, a, f string
	for _, ant := range rule.Antecedents {
		label := c.listener.resourceManager.ResourceByKey[ant.PredicateKey].Value
		switch label {
		case "ex:claim":
			m = termVars(c, ant)[0]
		case "ex:amount":
			a = termVars(c, ant)[1]
		case "ex:fee":
			f = termVars(c, ant)[1]
		}
	}
	if want := fmt.Sprintf("(%s ex:total (%s + %s))", m, a, f); !strings.Contains(rule.NormalizedLabel, want) {
		t.Errorf("label %s does not contain %s", rule.NormalizedLabel, want)
	}
}

// I-201: terms the heuristic scores equally keep their authored order. After the
// rdf:type term binds ?x, each (?x ex:pNN ?vNN) scores 400, and each (?zNN ex:qNN ?x)
// 260. slices.SortFunc is not stable: it sorts 12 elements or fewer by insertion, which
// keeps the order, and more with pdqsort, which need not. So the rule has more than 12
// terms, of two scores, interleaved.
func TestOptimizerKeepsTiesInTheAuthoredOrder(t *testing.T) {
	const n = 10
	var b strings.Builder
	b.WriteString("[TIE1]:\n(?x rdf:type ex:Item)")
	for i := 1; i <= n; i++ {
		fmt.Fprintf(&b, ".\n(?x ex:p%02d ?v%02d).\n(?z%02d ex:q%02d ?x)", i, i, i, i)
	}
	b.WriteString("\n->\n(?x ex:seen true);\n")
	c, rules := compileForOptimizer(t, "opt_ties.jr", b.String())
	var got []string
	for _, a := range rules[0].Antecedents {
		got = append(got, c.listener.resourceManager.ResourceByKey[a.PredicateKey].Value)
	}
	// every 400 before every 260, each group in its authored order
	want := []string{"rdf:type"}
	for i := 1; i <= n; i++ {
		want = append(want, fmt.Sprintf("ex:p%02d", i))
	}
	for i := 1; i <= n; i++ {
		want = append(want, fmt.Sprintf("ex:q%02d", i))
	}
	if strings.Join(got, " ") != strings.Join(want, " ") {
		t.Errorf("predicates in order\n  got  %s\n  want %s", strings.Join(got, " "), strings.Join(want, " "))
	}
}
