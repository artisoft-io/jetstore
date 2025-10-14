package compiler

// This file contains the function to transform the rules into a Rete network

import (
	"fmt"
	"log"
	"slices"
	"sort"

	"github.com/artisoft-io/jetstore/jets/jetrules/rete"
)

// BuildReteNetwork builds the Rete network from the rules in the Jetrule model
// It creates the Alpha nodes with Beta nodes configuration and connects them according to the rules
// The Rete network optimizations (at rule level) is performed prior to this step.
//
// Augment JetRule structure with rete markups:
//   - Add to antecedent: parent_vertex, vertex, and children_vertexes
//   - Add to consequent: vertex
//
// parent_vertex and vertex are integers
// --
// Approach:
// Connect nodes across rules by matching normalized labels (merging common antecedents).
func (l *JetRuleListener) BuildReteNetwork() {
	if l.trace {
		fmt.Fprintf(l.parseLog, "** entering BuildReteNetwork for %d rules\n", len(l.jetRuleModel.Jetrules))
	}

	// Prepare the ReteNodes structure by initializing it with the root node
	l.jetRuleModel.ReteNodes = make([]*rete.RuleTerm, 0, len(l.jetRuleModel.Jetrules))
	rootNode := &rete.RuleTerm{
		Vertex:           0,
		Type:             "root",
		ParentVertex:     -1, // root has no parent
		ChildrenVertexes: []int{},
		BetaVarNodes:     []*rete.BetaVarNode{},
		Rules:            []string{},
		Salience:         []int{},
		NormalizedLabel:  "root",
		// Other fields are nil or zeroed
	}
	// Node.Vertex is the index in the ReteNodes slice for antecedents, for consequents
	// it is the index of it's parent antecedent node
	// The root node is always at index 0
	l.jetRuleModel.ReteNodes = append(l.jetRuleModel.ReteNodes, rootNode)

	// For each rule, add its antecedents to the Rete network
	for _, rule := range l.jetRuleModel.Jetrules {
		if l.trace {
			fmt.Fprintf(l.parseLog, "** Processing rule: %s\n", rule.Name)
		}
		// for each antecedent, look for a node with the same normalized label
		// if found, use it as the rete node
		// if not found, use this rule as the rete node.
		parentVertex := 0 // start at root
		for i, antecedent := range rule.Antecedents {
			if l.trace {
				fmt.Fprintf(l.parseLog, "   Antecedent %d: %s\n", i+1, antecedent.NormalizedLabel)
			}
			// Look for a node with the same normalized label and parent vertex
			parentNode := l.jetRuleModel.ReteNodes[parentVertex]
			reteNode := l.ReteNodeByNormalizedLabel(parentVertex, antecedent.NormalizedLabel)
			if reteNode == nil {
				// No matching node found, use current antecedent as the rete node
				if l.trace {
					fmt.Fprintf(l.parseLog, "***   No matching Rete node found, using antecedent: %s\n", antecedent.NormalizedLabel)
				}
				reteNode = antecedent
				l.jetRuleModel.ReteNodes = append(l.jetRuleModel.ReteNodes, reteNode)
				// Set the vertex and parent vertex
				reteNode.Vertex = len(l.jetRuleModel.ReteNodes) - 1 // index in the slice
				reteNode.ParentVertex = parentVertex
				// Set the vertex of the beta nodes of the antecedent
				for _, betaNode := range reteNode.BetaVarNodes {
					betaNode.Vertex = reteNode.Vertex
				}
				// Add this node as a child of the parent node
				parentNode.ChildrenVertexes = append(parentNode.ChildrenVertexes, reteNode.Vertex)

			} else {
				if l.trace {
					// Matching node found, use it as the rete node
					fmt.Fprintf(l.parseLog, "***   Found matching Rete node: %s (vertex %d)\n", reteNode.NormalizedLabel, reteNode.Vertex)
				}
			}
			parentVertex = reteNode.Vertex
		}
		// Carry rule's name and salience to rete_node:
		//   - associated with the last antecedent of the rule
		//   - associated with the consequent term of the rule
		// Set the vertex for each consequent to be the last antecedent's vertex (current value of
		// parentVertex)
		lastAntecedent := l.jetRuleModel.ReteNodes[parentVertex]
		lastAntecedent.Rules = append(lastAntecedent.Rules, rule.Name)
		lastAntecedent.Salience = append(lastAntecedent.Salience, rule.Salience)
		for i := range rule.Consequents {
			rule.Consequents[i].Vertex = parentVertex
			rule.Consequents[i].ConsequentForRule = rule.Name
			rule.Consequents[i].ConsequentSalience = rule.Salience
			rule.Consequents[i].ConsequentSeq = i + 1
		}
	}

	// Add the consequent nodes at the end of the antecedent nodes
	// For each rule, add its consequents to the Rete network
	for _, rule := range l.jetRuleModel.Jetrules {
		l.jetRuleModel.ReteNodes = append(l.jetRuleModel.ReteNodes, rule.Consequents...)
	}

	if l.trace {
		fmt.Fprintf(l.parseLog, "** Rete Network built with %d nodes\n", len(l.jetRuleModel.ReteNodes))
	} else {
		//*TODO remove unused variables in the json output
		// // Clean up the rete nodes, remove ParentBindedVars, DescendentsReqVars, SelfVars,
		// // BetaRelationVars, and PrunedVars as they are no longer needed
		// for _, node := range l.jetRuleModel.ReteNodes {
		// 	node.BetaRelationVars = nil
		// 	node.PrunedVars = nil
		// }
	}
}

func (l *JetRuleListener) ReteNodeByNormalizedLabel(parentVertex int, normalizedLabel string) *rete.RuleTerm {
	for _, node := range l.jetRuleModel.ReteNodes {
		if node.ParentVertex == parentVertex && node.NormalizedLabel == normalizedLabel {
			return node
		}
	}
	return nil
}

func (l *JetRuleListener) CollectDescendentsReqVars(vars map[string]bool, r *rete.RuleTerm) {

	// Add self filter vars
	if r.Filter != nil {
		varsInFilter := make(map[int]bool)
		l.collectVarResourcesFromExpr(r.Filter, varsInFilter)
		for key := range varsInFilter {
			r := l.resourceManager.ResourceByKey[key]
			vars[r.Id] = true
		}
	}

	for _, childVertex := range r.ChildrenVertexes {
		childNode := l.jetRuleModel.ReteNodes[childVertex]
		if childNode.SubjectKey > 0 {
			r := l.resourceManager.ResourceByKey[childNode.SubjectKey]
			if r.Type == "var" {
				vars[r.Id] = true
			}
		}
		if childNode.PredicateKey > 0 {
			r := l.resourceManager.ResourceByKey[childNode.PredicateKey]
			if r.Type == "var" {
				vars[r.Id] = true
			}
		}
		if childNode.ObjectKey > 0 {
			r := l.resourceManager.ResourceByKey[childNode.ObjectKey]
			if r.Type == "var" {
				vars[r.Id] = true
			}
		}
		if childNode.ObjectExpr != nil {
			varsKeyInExpr := make(map[int]bool)
			l.collectVarResourcesFromExpr(childNode.ObjectExpr, varsKeyInExpr)
			for key := range varsKeyInExpr {
				vars[l.resourceManager.ResourceByKey[key].Id] = true
			}
		}
		l.CollectDescendentsReqVars(vars, childNode)
	}
	if r.Type == "consequent" {
		return
	}

	// visit consequent associated with this antecedent
	for _, rNode := range l.jetRuleModel.ReteNodes {
		if rNode.Type != "consequent" {
			continue
		}
		// If this consequent is for this antecedent
		if rNode.Vertex == r.Vertex {
			if rNode.SubjectKey > 0 {
				r := l.resourceManager.ResourceByKey[rNode.SubjectKey]
				if r.Type == "var" {
					vars[r.Id] = true
				}
			}
			if rNode.PredicateKey > 0 {
				r := l.resourceManager.ResourceByKey[rNode.PredicateKey]
				if r.Type == "var" {
					vars[r.Id] = true
				}
			}
			if rNode.ObjectKey > 0 {
				r := l.resourceManager.ResourceByKey[rNode.ObjectKey]
				if r.Type == "var" {
					vars[r.Id] = true
				}
			}
			if rNode.ObjectExpr != nil {
				varsKeyInExpr := make(map[int]bool)
				l.collectVarResourcesFromExpr(rNode.ObjectExpr, varsKeyInExpr)
				for key := range varsKeyInExpr {
					vars[l.resourceManager.ResourceByKey[key].Id] = true
				}
			}
			l.CollectDescendentsReqVars(vars, rNode)
		}
	}
}

// BuildBetaNodesForJetrule builds the Beta nodes for a given JetruleNode
// It creates the Beta nodes for the antecedents provided the rule is
// marked as valid.
// The beta nodes are invariant, meaning they can be build at the rule level
// before the rete network is built.
// Replace the temp variable nodes created during parsing with the actual variable nodes
// once their pos and binded status is determined.
// The temp variable nodes are provided by currentRuleVarByValue
func (l *JetRuleListener) BuildBetaNodesForJetrule(rule *rete.JetruleNode) {
	if !rule.IsValid {
		return
	}
	if l.trace {
		fmt.Fprintf(l.parseLog, "** entering BuildBetaNodesForJetrule for rule %s\n", rule.Name)
	}
	// Wrap the antecedents and consequents in RuleTermWrapper and connect them as parent-child
	// Collect the binded variables for each antecedent including the inherited from parent
	ruleAntecedents := make([]*RuleTermWrapper, 0, len(rule.Antecedents))
	for _, ant := range rule.Antecedents {
		wrapper := &RuleTermWrapper{
			RuleTerm:               ant,
			Descendents:            make([]*RuleTermWrapper, 0),
			BindedVars:             make(map[string]bool),
			RequiredDescendentVars: make(map[string]bool),
		}
		if len(ruleAntecedents) > 0 {
			// Set parent-child relationship
			parent := ruleAntecedents[len(ruleAntecedents)-1]
			wrapper.Parent = parent
			parent.Descendents = append(parent.Descendents, wrapper)
			// Inherit binded vars from parent
			for v := range parent.BindedVars {
				wrapper.BindedVars[v] = true
			}
		}
		// Add self vars to binded vars
		l.collectVars(wrapper.BindedVars, ant)
		ruleAntecedents = append(ruleAntecedents, wrapper)
		if l.trace {
			fmt.Fprintf(l.parseLog, "   Antecedent: %s, binded vars: %v\n", ant.NormalizedLabel, wrapper.BindedVars)
		}
	}
	// Add the consequents to the last antecedent,
	// set their parent to the last antecedent
	// Add the self vars of each consequent to the last antecedent's
	// descendent required vars and apply to all ancestors
	lastAntecedent := ruleAntecedents[len(ruleAntecedents)-1]
	for i, cons := range rule.Consequents {
		cons.ConsequentSeq = i + 1
		wrapper := &RuleTermWrapper{
			RuleTerm:    cons,
			Descendents: nil,
			Parent:      lastAntecedent,
		}
		lastAntecedent.Descendents = append(lastAntecedent.Descendents, wrapper)
		// Add self vars to lastAntecedent's descendent required vars
		l.collectVars(lastAntecedent.RequiredDescendentVars, cons)
		// Apply to all ancestors
		current := lastAntecedent
		parent := lastAntecedent.Parent
		// Add filters's var of current node to node's required descendent vars
		if parent == nil {
			// Special case: there is only 1 antecedent, add the filter vars to its own
			l.updateVarsFromFilter(current.RuleTerm.Filter, current.RequiredDescendentVars)
		}
		for parent != nil {
			l.updateVarsFromFilter(current.RuleTerm.Filter, current.RequiredDescendentVars)
			for v := range current.RequiredDescendentVars {
				parent.RequiredDescendentVars[v] = true
			}
			l.collectVars(parent.RequiredDescendentVars, current.RuleTerm)
			current = parent
			parent = parent.Parent
		}
	}

	// Now build the Beta nodes for each antecedent
	// Create also the associated ResourceNode (which will replace the temp var node)
	for _, ant := range ruleAntecedents {
		newVars := make(map[string]*rete.ResourceNode)
		// Create the Beta nodes for this antecedent
		ant.RuleTerm.BetaRelationVars = make([]string, 0, len(ant.BindedVars))
		ant.RuleTerm.PrunedVars = make([]string, 0, len(ant.BindedVars))
		ant.RuleTerm.BetaVarNodes = make([]*rete.BetaVarNode, 0, len(ant.BindedVars))
		// For each binded var that are required by descendents, add to BetaRelationVars
		for bvar := range ant.BindedVars {
			if ant.RequiredDescendentVars[bvar] {
				ant.RuleTerm.BetaRelationVars = append(ant.RuleTerm.BetaRelationVars, bvar)
			} else {
				ant.RuleTerm.PrunedVars = append(ant.RuleTerm.PrunedVars, bvar)
				// Still create a replacement ResourceNode for pruned vars
				r := l.addVarResourceByDomainKey(&rete.ResourceNode{
					Type: "var",
					Id:   bvar,
				})
				newVars[r.Id] = r
			}
		}
		// Sort the BetaRelationVars slice
		sort.Strings(ant.RuleTerm.BetaRelationVars)
		sort.Strings(ant.RuleTerm.PrunedVars)

		// Get the var position in the triple of current antecedent
		varPos := l.getVarPosition(ant.RuleTerm)

		// For each BetaRelationVar create a BetaVarNode
		for i, bvar := range ant.RuleTerm.BetaRelationVars {
			isBinded := false
			pos := i
			pp := varPos[bvar]
			if pp != nil {
				// This var is in the triple, use its position
				pos = *pp
			} else {
				// Not in the triple, it must be binded in parent
				isBinded = true
			}
			node := &rete.BetaVarNode{
				Type:           "var",
				Id:             bvar,
				IsBinded:       isBinded,
				VarPos:         pos,
				SourceFileName: l.currentRuleFileName,
			}
			ant.RuleTerm.BetaVarNodes = append(ant.RuleTerm.BetaVarNodes, node)
			// Create the associated ResourceNode (which will replace the temp var node)
			r := l.addVarResourceByDomainKey(&rete.ResourceNode{
				Type:     "var",
				Id:       node.Id,
				IsBinded: node.IsBinded,
				VarPos:   node.VarPos,
			})
			newVars[r.Id] = r
		}
		// Replace the var in the rule antecedents and consequents, incl filters and expressions
		// with the actual ResourceNode created above
		replaceVar := func(id string) (r *rete.ResourceNode) {
			r = newVars[id]
			if r == nil {
				log.Println("** Internal error: temp var not found:", id)
				fmt.Fprintf(l.errorLog, "** Internal error: temp var not found: %s\n", id)
				fmt.Fprintf(l.parseLog, "** Internal error: temp var not found: %s\n", id)
			}
			return r
		}
		l.replaceVarInAntecedent(ant, replaceVar)

	}

	// Delete the temp var nodes created during parsing
	// Remove from resourceManager.ResourceByKey and jetRuleModel.Resources
	tempNodes := make(map[int]bool)
	for _, r := range l.currentRuleVarByValue {
		tempNodes[r.Key] = true
		delete(l.resourceManager.ResourceByKey, r.Key)
	}
	l.jetRuleModel.Resources = slices.DeleteFunc(l.jetRuleModel.Resources, func(res *rete.ResourceNode) bool {
		return tempNodes[res.Key]
	})

	if l.trace {
		fmt.Fprintf(l.parseLog, "** leaving BuildBetaNodesForJetrule for rule %s\n", rule.Name)
	}
}

func (l *JetRuleListener) replaceVarInAntecedent(ant *RuleTermWrapper, replaceVar func(id string) *rete.ResourceNode) {
	if ant.RuleTerm.SubjectKey > 0 {
		r := l.resourceManager.ResourceByKey[ant.RuleTerm.SubjectKey]
		if r.Type == "var" {
			newR := replaceVar(r.Id)
			if newR != nil {
				ant.RuleTerm.SubjectKey = newR.Key
			} else {
				fmt.Fprintf(l.parseLog, "** internal error: var %s is not found in replacement resource for SubjectKey\n", r.Id)
				fmt.Fprintf(l.errorLog, "** internal error: var %s is not found in replacement resource for SubjectKey\n", r.Id)
			}
		}
	}
	if ant.RuleTerm.PredicateKey > 0 {
		r := l.resourceManager.ResourceByKey[ant.RuleTerm.PredicateKey]
		if r.Type == "var" {
			newR := replaceVar(r.Id)
			if newR != nil {
				ant.RuleTerm.PredicateKey = newR.Key
			} else {
				fmt.Fprintf(l.errorLog, "** internal error: var %s is not found in replacement resource for PredicateKey \n", r.Id)
				fmt.Fprintf(l.parseLog, "** internal error: var %s is not found in replacement resource for PredicateKey\n", r.Id)
			}
		}
	}
	if ant.RuleTerm.ObjectKey > 0 {
		r := l.resourceManager.ResourceByKey[ant.RuleTerm.ObjectKey]
		if r.Type == "var" {
			newR := replaceVar(r.Id)
			if newR != nil {
				ant.RuleTerm.ObjectKey = newR.Key
			} else {
				fmt.Fprintf(l.errorLog, "** internal error: var %s is not found in replacement resource for ObjectKey\n", r.Id)
				fmt.Fprintf(l.parseLog, "** internal error: var %s is not found in replacement resource for ObjectKey\n", r.Id)
			}
		}
	}
	if ant.RuleTerm.ObjectExpr != nil {
		l.replaceVarInExpr(ant.RuleTerm.ObjectExpr, replaceVar)
	}
	if ant.RuleTerm.Filter != nil {
		l.replaceVarInExpr(ant.RuleTerm.Filter, replaceVar)
	}
}

func (s *JetRuleListener) replaceVarInExpr(expr *rete.ExpressionNode, replaceVar func(id string) *rete.ResourceNode) {
	if expr == nil {
		return
	}
	if expr.Lhs != nil {
		s.replaceVarInExpr(expr.Lhs, replaceVar)
	}
	if expr.Rhs != nil {
		s.replaceVarInExpr(expr.Rhs, replaceVar)
	}
	if expr.Arg != nil {
		s.replaceVarInExpr(expr.Arg, replaceVar)
	}
	if expr.Value > 0 {
		r := s.resourceManager.ResourceByKey[expr.Value]
		if r.Type == "var" {
			newR := replaceVar(r.Id)
			if newR != nil {
				expr.Value = newR.Key
			} else {
				fmt.Fprintf(s.errorLog, "** internal error: var %s is not found in replacement resource", r.Id)
				fmt.Fprintf(s.parseLog, "** internal error: var %s is not found in replacement resource", r.Id)
			}
		}
	}
}

func (l *JetRuleListener) updateVarsFromFilter(filter *rete.ExpressionNode, vars map[string]bool) {
	if filter == nil {
		return
	}
	varsInFilter := make(map[int]bool)
	l.collectVarResourcesFromExpr(filter, varsInFilter)
	for key := range varsInFilter {
		r := l.resourceManager.ResourceByKey[key]
		vars[r.Id] = true
	}
}

// getVarPosition returns the position of the variable in the triple
// The map value indicates the position of the variable in the triple:
// 0 = subject, 1 = predicate, 2 = object
func (s *JetRuleListener) getVarPosition(antecedent *rete.RuleTerm) map[string]*int {
	selfVars := make(map[string]*int)
	if antecedent.SubjectKey > 0 {
		r := s.resourceManager.ResourceByKey[antecedent.SubjectKey]
		if r.Type == "var" {
			pos := 0
			selfVars[r.Id] = &pos
		}
	}
	if antecedent.PredicateKey > 0 {
		r := s.resourceManager.ResourceByKey[antecedent.PredicateKey]
		if r.Type == "var" {
			pos := 1
			selfVars[r.Id] = &pos
		}
	}
	if antecedent.ObjectKey > 0 {
		r := s.resourceManager.ResourceByKey[antecedent.ObjectKey]
		if r.Type == "var" {
			pos := 2
			selfVars[r.Id] = &pos
		}
	}
	return selfVars
}
