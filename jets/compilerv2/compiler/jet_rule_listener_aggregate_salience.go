package compiler

import (
	"fmt"
	"slices"
	"strings"

	"github.com/artisoft-io/jetstore/jets/jetrules/rete"
)

// An aggregate in a consequent is computed once, when its rule fires.
//
// Neither rule engine recomputes a consequent expression: the C++ engine's
// set_graph_callbacks and the Go engine's Initialize register callbacks for
// antecedents and filters only. So size_of, max_of, min_of, sum_values,
// sorted_head and join_values in a consequent see the values that exist at the
// moment the rule fires, and no others. A rule that aggregates a property must
// therefore fire after every rule that writes that property, which in a salience
// ordered agenda means a strictly lower salience.
//
// At equal salience the engines fall back to vertex order (lower vertex first),
// which follows where rules sit in the source. A rule set that is correct only
// because the producer happens to be written above the aggregate is correct by
// accident: moving a rule, or importing files in another order, changes the
// result with no error anywhere. Three such rules were found in production rules
// (a maximum taken over some of a drug's claims, a reversal partner picked over
// some of a claim's pairs, a coverage count taken over some of a member's links),
// which is why this is an error and not a warning: warnings are not shown to
// anyone who would act on them.
//
// What is checked: the direct producers of the aggregated property, meaning the
// rules with a consequent whose predicate is that property, in the same compile
// unit. For a config resource (one with jets:entity_property or
// jets:value_property triples) both configured properties are checked. A
// property no rule writes is input data and needs nothing.
//
// What is not checked: producers further upstream. An aggregate below its direct
// producer but above a rule that feeds that producer can still fire early; a
// transitive check would also reject rules made safe by a dependency on a
// lower-salience result, so it is left to review.
//
// Two ways to be exempt:
//   - a rule with an antecedent on jets:completed, which cannot fire before the
//     session's loop has completed and so after every producer;
//   - a rule declaring aggregate_check=false in its header, for a deliberate case
//     the check cannot see. The property states the intent where a reader of the
//     rule will find it.

var consequentAggregateOps = map[string]bool{
	"size_of":     true,
	"max_of":      true,
	"min_of":      true,
	"sum_values":  true,
	"sorted_head": true,
	"join_values": true,
}

type aggregateProducer struct {
	rule     string
	salience int
}

// ValidateAggregateSalience logs an error for every rule that computes an
// aggregate in a consequent at a salience equal to or higher than a rule that
// writes the aggregated property.
func (l *JetRuleListener) ValidateAggregateSalience() {
	model := l.jetRuleModel
	nameOf := func(key int) string {
		if r := l.resourceManager.ResourceByKey[key]; r != nil {
			return r.Value
		}
		return ""
	}

	// Config resources: subject -> the properties named by jets:entity_property
	// and jets:value_property.
	configProps := make(map[int][]int)
	for _, t := range model.Triples {
		switch nameOf(t.PredicateKey) {
		case "jets:entity_property", "jets:value_property":
			configProps[t.SubjectKey] = append(configProps[t.SubjectKey], t.ObjectKey)
		}
	}

	// Producers: predicate -> the rules with a consequent on it.
	producers := make(map[int][]aggregateProducer)
	for _, rule := range model.Jetrules {
		if !rule.IsValid {
			continue
		}
		for _, c := range rule.Consequents {
			producers[c.PredicateKey] = append(producers[c.PredicateKey], aggregateProducer{rule.Name, rule.Salience})
		}
	}

	for _, rule := range model.Jetrules {
		if !rule.IsValid || aggregateCheckOptedOut(rule) || l.gatedOnCompletion(rule, nameOf) {
			continue
		}
		for _, c := range rule.Consequents {
			for _, agg := range consequentAggregates(c.ObjectExpr) {
				props := configProps[agg.Rhs.Value]
				if len(props) == 0 {
					props = []int{agg.Rhs.Value}
				}
				var early []string
				lowest := 0
				for _, p := range props {
					for _, prod := range producers[p] {
						if prod.rule == rule.Name || prod.salience > rule.Salience {
							continue
						}
						early = append(early, fmt.Sprintf("%s (s=%d, writes %s)", prod.rule, prod.salience, nameOf(p)))
						if lowest == 0 || prod.salience < lowest {
							lowest = prod.salience
						}
					}
				}
				if len(early) == 0 {
					continue
				}
				slices.Sort(early)
				early = slices.Compact(early)
				l.LogError(fmt.Sprintf(
					"** error: rule %s (%s) computes %s %s in a consequent at salience %d, "+
						"and is fed at the same or a lower salience by %s; an aggregate in a consequent is computed once, when its rule fires, "+
						"so it must fire after every rule that writes what it reads: "+
						"set its salience below %d, gate it on jets:completed, "+
						"or declare aggregate_check=false if the order is guaranteed another way",
					rule.Name, rule.SourceFileName, agg.Op, nameOf(agg.Rhs.Value), rule.Salience,
					strings.Join(early, ", "), lowest))
			}
		}
	}
}

// consequentAggregates returns the aggregate operator nodes of an expression
// whose right-hand side names a resource.
func consequentAggregates(e *rete.ExpressionNode) []*rete.ExpressionNode {
	if e == nil {
		return nil
	}
	var out []*rete.ExpressionNode
	if e.Type == "binary" && consequentAggregateOps[e.Op] && e.Rhs != nil && e.Rhs.Type == "identifier" {
		out = append(out, e)
	}
	out = append(out, consequentAggregates(e.Arg)...)
	out = append(out, consequentAggregates(e.Lhs)...)
	out = append(out, consequentAggregates(e.Rhs)...)
	return out
}

func aggregateCheckOptedOut(rule *rete.JetruleNode) bool {
	switch strings.ToUpper(rule.Properties["aggregate_check"]) {
	case "FALSE", "F", "0":
		return true
	}
	return false
}

func (l *JetRuleListener) gatedOnCompletion(rule *rete.JetruleNode, nameOf func(int) string) bool {
	for _, a := range rule.Antecedents {
		if !a.IsNot && nameOf(a.PredicateKey) == "jets:completed" {
			return true
		}
	}
	return false
}
