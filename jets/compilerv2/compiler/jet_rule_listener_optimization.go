package compiler

import (
	"fmt"
	"slices"

	"github.com/artisoft-io/jetstore/jets/jetrules/rete"
	"github.com/artisoft-io/jetstore/jets/stack"
)

// This file contains the functions for JetRule optimization

// OptimizeJetruleNode performs optimizations on a JetruleNode
// by re-ordering the antecedents to minimize the number of intermediate beta nodes
// in the Rete network.
// The optimization is based on a simple heuristic:
// - Antecedents with the least number of variables are placed first
// - Antecedents that share variables with already placed antecedents are placed next
// - Antecedents that do not share variables with already placed antecedents are placed last
// This is a greedy algorithm and may not produce the optimal ordering in all cases.
// More advanced algorithms can be implemented in the future.
func (s *JetRuleListener) OptimizeJetruleNode(rule *rete.JetruleNode) {
	if !rule.Optimization {
		// Optimization is disabled for this rule
		return
	}
	antecedents := rule.Antecedents
	n := len(antecedents)
	if n <= 1 {
		// No need to optimize
		return
	}

	// Simple greedy algorithm to reorder antecedents
	ordered := make([]*rete.RuleTerm, 0, n)
	filters := make([]*rete.ExpressionNode, 0)
	// Collect filters from original rule's antecedent so to reallocated them after
	// the antecedent have been reordered
	for i := range antecedents {
		if antecedents[i].Filter != nil {
			filters = append(filters, antecedents[i].Filter)
		}
	}

	// Keep track of the binded variables
	bindedVars := make(map[string]*int)

	// Iterativelly allocate the antecedent with the top priority
	// Priority is computed as: 1000 - score
	// where score is computed based on triple configuration
	// ---------------------------------------------------------------------------------
	// optimized_antecedents
	// ---------------------------------------------------------------------------------
	// While we still have antecedent to place
	type antPriority struct {
		index    int
		priority int
	}
	for len(antecedents) > 0 {
		// create a priority list
		plist := make([]antPriority, 0, len(antecedents))
		for i := range antecedents {
			score := s.evaluateScore(bindedVars, antecedents[i])
			plist = append(plist, antPriority{i, 1000 - score})
			if s.trace {
				s.LogParse(fmt.Sprintf("*** Antecedent %d score: %d, priority: %d", i, score, 1000-score))
			}
		}

		// find the antecedent with the highest priority
		slices.SortFunc(plist, func(a, b antPriority) int {
			return a.priority - b.priority
		})
		// take the first one
		best := plist[0]
		ordered = append(ordered, antecedents[best.index])
		if s.trace {
			s.LogParse(fmt.Sprintf("*** Selected antecedent %d with priority %d", best.index, best.priority))
		}

		// update bindedVars
		s.updateBindedVars(bindedVars, antecedents[best.index])

		// remove it from the antecedents list
		antecedents = slices.Delete(antecedents, best.index, best.index+1)
	}

	if len(filters) == 0 {
		// No filters to reallocate
		goto post_process
	}

	// Reallocate the filters to the reordered antecedents
	// ---------------------------------------------------------------------------------
	bindedVars = make(map[string]*int)
	// For each antecedent in order, check if any filter can be applied
	// A filter can be applied if all its variables are binded by the antecedents
	// placed before it.
	// If multiple filters can be applied, they are combined using AND operator
	// and added to the antecedent.
	// The filters that have been applied are removed from the filters list.
	// ---------------------------------------------------------------------------------
	// For each antecedent in order
	for i := range ordered {
		// update bindedVars
		s.updateBindedVars(bindedVars, ordered[i])

		// use a stack to keep track of the filters that can be applied to this antecedent
		matchedFilters := stack.NewStack[rete.ExpressionNode](5)
		matchedPos := make([]int, 0, 5)

		// check if any filter can be applied to this antecedent
		for j, filter := range filters {
			varsInFilter := make(map[int]bool)
			s.collectVarResourcesFromExpr(filter, varsInFilter)
			allVarsBinded := true
			for key := range varsInFilter {
				r := s.resourceManager.ResourceByKey[key]
				if bindedVars[r.Id] == nil {
					allVarsBinded = false
					break
				}
			}
			if allVarsBinded {
				matchedFilters.Push(filter)
				matchedPos = append(matchedPos, j)
			}
		}
		// Combine all matched filters into a single filter using AND operator
		var lfilter *rete.ExpressionNode
		for !matchedFilters.IsEmpty() {
			if lfilter != nil {
				rfilter, _ := matchedFilters.Pop()
				lfilter = &rete.ExpressionNode{
					Type: "binary",
					Op:   "and",
					Lhs:  lfilter,
					Rhs:  rfilter,
				}
			} else {
				lfilter, _ = matchedFilters.Pop()
			}
		}
		// Add the combined filter to the antecedent
		ordered[i].Filter = lfilter

		// Remove the matched filters from the filters list
		// We need to remove from the end to avoid messing up the indices
		slices.Sort(matchedPos)
		for k := len(matchedPos) - 1; k >= 0; k-- {
			pos := matchedPos[k]
			filters = slices.Delete(filters, pos, pos+1)
		}
	}

post_process:
	// Update the rule's antecedents with the reordered ones
	rule.Antecedents = ordered

	// Verify that we placed all filters
	if len(filters) > 0 {
		// This should never happen
		s.LogError("Internal error: not all filters were reallocated during rule optimization")
	}

	// Re-normalize the rule's variable names
	varIdByKey := make(map[int]string)
	// visit the rule antecedents in order and collect the variable keys
	for _, a := range rule.Antecedents {
		if a.SubjectKey > 0 && varIdByKey[a.SubjectKey] == "" {
			r := s.resourceManager.ResourceByKey[a.SubjectKey]
			if r.Type == "var" {
				r.Id = fmt.Sprintf("?x%02d", len(varIdByKey)+1)
				varIdByKey[a.SubjectKey] = r.Id
			}
		}
		if a.PredicateKey > 0 && varIdByKey[a.PredicateKey] == "" {
			r := s.resourceManager.ResourceByKey[a.PredicateKey]
			if r.Type == "var" {
				r.Id = fmt.Sprintf("?x%02d", len(varIdByKey)+1)
				varIdByKey[a.PredicateKey] = r.Id
			}
		}
		if a.ObjectKey > 0 && varIdByKey[a.ObjectKey] == "" {
			r := s.resourceManager.ResourceByKey[a.ObjectKey]
			if r.Type == "var" {
				r.Id = fmt.Sprintf("?x%02d", len(varIdByKey)+1)
				varIdByKey[a.ObjectKey] = r.Id
			}
		}
	}
}

// updateBindedVars updates the bindedVars map with the variables in the antecedent
// The map value indicates the position of the variable in the triple:
// 0 = subject, 1 = predicate, 2 = object
func (s *JetRuleListener) updateBindedVars(bindedVars map[string]*int, antecedent *rete.RuleTerm) {
	if antecedent.SubjectKey > 0 {
		r := s.resourceManager.ResourceByKey[antecedent.SubjectKey]
		if r.Type == "var" {
			pos := 0
			bindedVars[r.Id] = &pos
		}
	}
	if antecedent.PredicateKey > 0 {
		r := s.resourceManager.ResourceByKey[antecedent.PredicateKey]
		if r.Type == "var" {
			pos := 1
			bindedVars[r.Id] = &pos
		}
	}
	if antecedent.ObjectKey > 0 {
		r := s.resourceManager.ResourceByKey[antecedent.ObjectKey]
		if r.Type == "var" {
			pos := 2
			bindedVars[r.Id] = &pos
		}
	}
}

// collectUnbindedVars collects the unbinded variables in the antecedent
// and adds them to the unbindedVars map
// The map value indicates the position of the variable in the triple:
// 0 = subject, 1 = predicate, 2 = object
func (s *JetRuleListener) collectUnbindedVars(parentBindedVars map[string]bool, unbindedVars map[string]*int, antecedent *rete.RuleTerm) {
	if antecedent.SubjectKey > 0 {
		r := s.resourceManager.ResourceByKey[antecedent.SubjectKey]
		if r.Type == "var" {
			if !parentBindedVars[r.Id] {
				pos := 0
				unbindedVars[r.Id] = &pos
			}
		}
	}
	if antecedent.PredicateKey > 0 {
		r := s.resourceManager.ResourceByKey[antecedent.PredicateKey]
		if r.Type == "var" {
			if !parentBindedVars[r.Id] {
				pos := 1
				unbindedVars[r.Id] = &pos
			}
		}
	}
	if antecedent.ObjectKey > 0 {
		r := s.resourceManager.ResourceByKey[antecedent.ObjectKey]
		if r.Type == "var" {
			if !parentBindedVars[r.Id] {
				pos := 2
				unbindedVars[r.Id] = &pos
			}
		}
	}
}

func (s *JetRuleListener) evaluateScore(bindedVars map[string]*int, antecedent *rete.RuleTerm) int {
	sscore := 0
	pscore := 0
	oscore := 0

	// subject score
	if antecedent.SubjectKey > 0 {
		r := s.resourceManager.ResourceByKey[antecedent.SubjectKey]
		switch r.Type {
		case "var":
			if bindedVars[r.Id] != nil {
				sscore = 200 + 20
			} else {
				sscore = 0 + 40
			}
		case "identifier", "resource", "volatile_resource":
			sscore = 100 + 0
		}
	}

	// predicate score
	if antecedent.PredicateKey > 0 {
		r := s.resourceManager.ResourceByKey[antecedent.PredicateKey]
		switch r.Type {
		case "var":
			if bindedVars[r.Id] != nil {
				pscore = 200 + 0
			} else {
				pscore = 0 + 0
			}
		case "identifier", "resource", "volatile_resource":
			pscore = 100 + 40
			if r.Value == "rdf:type" {
				pscore += 100
			}
		}
	}

	// object score
	if antecedent.ObjectKey > 0 {
		r := s.resourceManager.ResourceByKey[antecedent.ObjectKey]
		switch r.Type {
		case "var":
			if bindedVars[r.Id] != nil {
				oscore = 200 + 40
			} else {
				oscore = 0 + 20
			}
		case "identifier", "resource", "volatile_resource":
			oscore = 100 + 20
		}
	}
	// return total score
	return sscore + pscore + oscore
}
