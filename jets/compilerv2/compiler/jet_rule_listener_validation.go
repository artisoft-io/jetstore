package compiler

import (
	"fmt"

	"github.com/artisoft-io/jetstore/jets/jetrules/rete"
)

// This file contains the validation logic

// collectVarResourcesFromExpr recursively collects variable resource keys from an ExpressionNode.
func (s *JetRuleListener) collectVarResourcesFromExpr(expr *rete.ExpressionNode, varSet map[int]bool) {
	if expr == nil {
		return
	}
	if expr.Type == "identifier" {
		res := s.Resource(expr.Value)
		if res != nil && res.Type == "var" {
			varSet[expr.Value] = true
		}
	}
	if expr.Lhs != nil {
		s.collectVarResourcesFromExpr(expr.Lhs, varSet)
	}
	if expr.Rhs != nil {
		s.collectVarResourcesFromExpr(expr.Rhs, varSet)
	}
	if expr.Arg != nil {
		s.collectVarResourcesFromExpr(expr.Arg, varSet)
	}
}

// ValidateRuleTerm validates a RuleTerm:
// Must have at least one of Type "var" among subject, predicate, object
func (s *JetRuleListener) ValidateRuleTerm(term *rete.RuleTerm) {
	if term.Type == "antecedent" {
		// Must have at least one of Type "var" among subject, predicate, object
		if s.Resource(term.SubjectKey).Type != "var" &&
			s.Resource(term.PredicateKey).Type != "var" &&
			s.Resource(term.ObjectKey).Type != "var" {
			fmt.Fprintf(s.errorLog,
				"** error: antecedent must have at least one of subject, predicate, object as variable: (%s, %s, %s)\n",
				s.Resource(term.SubjectKey).SKey(),
				s.Resource(term.PredicateKey).SKey(),
				s.Resource(term.ObjectKey).SKey())
		}
	}
}

// ValidateJetruleNode validates a JetruleNode
// All ResourceNode of Type "?var" in the Consequents must appear in the Antecedents
func (s *JetRuleListener) ValidateJetruleNode(rule *rete.JetruleNode) {
	// Build a set of variable resource keys from the antecedents
	varSet := make(map[int]bool)
	for i := range rule.Antecedents {
		if s.Resource(rule.Antecedents[i].SubjectKey).Type == "var" {
			varSet[rule.Antecedents[i].SubjectKey] = true
		}
		if s.Resource(rule.Antecedents[i].PredicateKey).Type == "var" {
			varSet[rule.Antecedents[i].PredicateKey] = true
		}
		if s.Resource(rule.Antecedents[i].ObjectKey).Type == "var" {
			varSet[rule.Antecedents[i].ObjectKey] = true
		}
	}
	// Check that all variable resource keys in the consequents are in the set
	for i := range rule.Consequents {
		if s.Resource(rule.Consequents[i].SubjectKey).Type == "var" {
			if _, exists := varSet[rule.Consequents[i].SubjectKey]; !exists {
				fmt.Fprintf(s.errorLog,
					"** error: consequent subject variable %s not found in antecedents\n",
					s.Resource(rule.Consequents[i].SubjectKey).SKey())
			}
		}
		if s.Resource(rule.Consequents[i].PredicateKey).Type == "var" {
			if _, exists := varSet[rule.Consequents[i].PredicateKey]; !exists {
				fmt.Fprintf(s.errorLog,
					"** error: consequent predicate variable %s not found in antecedents\n",
					s.Resource(rule.Consequents[i].PredicateKey).SKey())
			}
		}
		o := s.Resource(rule.Consequents[i].ObjectKey)
		if o != nil && o.Type == "var" {
			if _, exists := varSet[rule.Consequents[i].ObjectKey]; !exists {
				fmt.Fprintf(s.errorLog,
					"** error: consequent object variable %s not found in antecedents\n",
					s.Resource(rule.Consequents[i].ObjectKey).SKey())
			}
		}
		objExpr := rule.Consequents[i].ObjectExpr
		if objExpr != nil {
			// Build a set of variable resource keys from the expression
			exprVarSet := make(map[int]bool)
			s.collectVarResourcesFromExpr(objExpr, exprVarSet)
			// Check that all variable resource keys in the expression are in the antecedent varSet
			for vKey := range exprVarSet {
				if _, exists := varSet[vKey]; !exists {
					fmt.Fprintf(s.errorLog,
						"** error: consequent object expression variable %s not found in antecedents\n",
						s.Resource(vKey).SKey())
				}
			}
		}
	}
}
