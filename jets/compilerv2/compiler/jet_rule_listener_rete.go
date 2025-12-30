package compiler

// This file contains the function to transform the rules into a Rete network

import (
	"fmt"
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
//   - Add BetaVarNodes to antecedent
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
		Type:             "head_node",
		ParentVertex:     -1, // root has no parent
		ChildrenVertexes: []int{},
		BetaVarNodes:     []*rete.BetaVarNode{},
		Rules:            []string{},
		Salience:         []int{},
		NormalizedLabel:  "head_node",
		// Other fields are nil or zeroed
	}
	// Node.Vertex is the index in the ReteNodes slice for antecedents, for consequents
	// it is the index of it's parent antecedent node
	// The root node is always at index 0
	l.jetRuleModel.ReteNodes = append(l.jetRuleModel.ReteNodes, rootNode)

	// For each rule, add its antecedents to the Rete network
	// Keep track of consequents by rule's last antecedent
	consequentsByVertex := make(map[int][]*rete.RuleTerm)
	for _, rule := range l.jetRuleModel.Jetrules {
		if l.trace {
			fmt.Fprintf(l.parseLog, "** Processing rule: %s\n", rule.Name)
		}
		// for each antecedent, look for a node with the same normalized label
		// if found, use it as the rete node
		// if not found, use this rule as the rete node.
		parentVertex := 0 // start at root
		mergedAntecedents := make(map[int]*rete.RuleTerm)
		for i, antecedent := range rule.Antecedents {
			if l.trace {
				fmt.Fprintf(l.parseLog, "   Antecedent %d: %s\n", i+1, antecedent.NormalizedLabel)
			}
			// Look for a node with the same normalized label and parent vertex
			parentNode := l.jetRuleModel.ReteNodes[parentVertex]
			reteNode := l.reteNodeByNormalizedLabel(parentVertex, antecedent.NormalizedLabel)
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
				// Add this node as a child of the parent node
				parentNode.ChildrenVertexes = append(parentNode.ChildrenVertexes, reteNode.Vertex)

			} else {
				mergedAntecedents[i] = reteNode
				if l.trace {
					// Matching node found, use it as the rete node
					fmt.Fprintf(l.parseLog, "***   Found matching Rete node: %s (vertex %d)\n", reteNode.NormalizedLabel, reteNode.Vertex)
				}
			}
			parentVertex = reteNode.Vertex
		}
		// Set the merged antecedents back to the rule
		for i, a := range mergedAntecedents {
			rule.Antecedents[i] = a
		}
		// Carry rule's name and salience to rete_nodes:
		//   - associated with the last antecedent of the rule
		//   - associated with the consequent term of the rule
		// Set the vertex for each consequent to be the last antecedent's vertex (current value of
		// parentVertex)
		lastAntecedent := l.jetRuleModel.ReteNodes[parentVertex]
		lastAntecedent.Rules = append(lastAntecedent.Rules, rule.Name)
		lastAntecedent.Salience = append(lastAntecedent.Salience, rule.Salience)
		consequentOffset := len(consequentsByVertex[parentVertex])
		for i := range rule.Consequents {
			rule.Consequents[i].Vertex = parentVertex
			rule.Consequents[i].ConsequentForRule = rule.Name
			rule.Consequents[i].ConsequentSalience = rule.Salience
			rule.Consequents[i].ConsequentSeq = consequentOffset + i + 1
		}
		consequentsByVertex[parentVertex] = append(consequentsByVertex[parentVertex], rule.Consequents...)
	}

	// Add the consequent nodes at the end of the antecedent nodes
	// For each rule, add its consequents to the Rete network
	for _, rule := range l.jetRuleModel.Jetrules {
		l.jetRuleModel.ReteNodes = append(l.jetRuleModel.ReteNodes, rule.Consequents...)
	}

	// Build the Beta Node for each antecedent of the rete network
	for _, node := range l.jetRuleModel.ReteNodes[0].ChildrenVertexes {
		bindedVars := make(map[string]bool)
		replacementBindedVarNodes := make(map[string]*rete.ResourceNode)
		l.BuildBetaNodesRecursively(l.jetRuleModel.ReteNodes[node], bindedVars, replacementBindedVarNodes, consequentsByVertex)
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

func (l *JetRuleListener) reteNodeByNormalizedLabel(parentVertex int, normalizedLabel string) *rete.RuleTerm {
	for _, node := range l.jetRuleModel.ReteNodes {
		if node.ParentVertex == parentVertex && node.NormalizedLabel == normalizedLabel {
			return node
		}
	}
	return nil
}

// Collect all the var resources used by the descendents of the given rete.RuleTerm
// That exclude the given rete.RuleTerm itself
func (l *JetRuleListener) CollectDescendentsReqVars(vars map[string]bool, r *rete.RuleTerm,
	consequentsByVertex map[int][]*rete.RuleTerm) {

	// visit children
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
		l.CollectDescendentsReqVars(vars, childNode, consequentsByVertex)
	}
	if r.Type == "consequent" {
		return
	}

	// visit consequents associated with this vertex / antecedent
	for _, rNode := range consequentsByVertex[r.Vertex] {
		l.CollectDescendentsReqVars(vars, rNode, consequentsByVertex)
	}
}

func (l *JetRuleListener) BuildBetaNodesRecursively(node *rete.RuleTerm,
	bindedVars map[string]bool, replacementBindedVarNodes map[string]*rete.ResourceNode,
	consequentsByVertex map[int][]*rete.RuleTerm) {

	// Get the var required by descendents of node
	descendentsReqVars := make(map[string]bool)
	l.CollectDescendentsReqVars(descendentsReqVars, node, consequentsByVertex)

	// Add self filter vars to descendentsReqVars
	if node.Filter != nil {
		varsInFilter := make(map[int]bool)
		l.collectVarResourcesFromExpr(node.Filter, varsInFilter)
		for key := range varsInFilter {
			r := l.resourceManager.ResourceByKey[key]
			descendentsReqVars[r.Id] = true
		}
	}

	// Get the var position in the triple of current antecedent
	varPos := l.getVarPosition(node)
	// Add node's var to bindedVars
	for v := range varPos {
		bindedVars[v] = true
	}

	// //***
	// fmt.Fprintf(l.parseLog, "Got bindedVars: %v, descendentsReqVars: %v for node vertex %d %s\n",
	// 	bindedVars, descendentsReqVars, node.Vertex, node.NormalizedLabel)

	// Build the Beta node configuration for this node
	// For each binded var that are required by descendents or current node, add to BetaRelationVars
	for v := range bindedVars {
		if descendentsReqVars[v] || varPos[v] != nil {
			node.BetaRelationVars = append(node.BetaRelationVars, v)
		} else {
			node.PrunedVars = append(node.PrunedVars, v)
		}
	}
	// Sort the BetaRelationVars slice
	sort.Strings(node.BetaRelationVars)
	sort.Strings(node.PrunedVars)

	// //***
	// fmt.Fprintf(l.parseLog, "Got BetaRelationVars: %v, PrunedVars: %v for node vertex %d %s\n",
	// 	node.BetaRelationVars, node.PrunedVars, node.Vertex, node.NormalizedLabel)

	// For each BetaRelationVar create a BetaVarNode
	// Keep track of new var created for replacement
	newVars := make(map[string]*rete.ResourceNode)
	for i, bvar := range node.BetaRelationVars {
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
		bv := &rete.BetaVarNode{
			Type:           "var",
			Id:             bvar,
			IsBinded:       isBinded,
			Vertex:         node.Vertex,
			VarPos:         pos,
			SourceFileName: l.currentRuleFileName,
		}
		node.BetaVarNodes = append(node.BetaVarNodes, bv)
		// Create the associated ResourceNode (which will replace the temp var node)
		r := l.addVarResourceByDomainKey(&rete.ResourceNode{
			Type:     "var",
			Id:       bv.Id,
			IsBinded: bv.IsBinded,
			Vertex:   bv.Vertex,
			VarPos:   bv.VarPos,
		})
		newVars[r.Id] = r
		if bv.IsBinded {
			// Collect all binded var for replacement in consequents
			replacementBindedVarNodes[r.Id] = r
		} else {
			// Create the binded version for replacement in consequents
			bindedR := l.addVarResourceByDomainKey(&rete.ResourceNode{
				Type:     "var",
				Id:       bv.Id,
				IsBinded: true,
				Vertex:   bv.Vertex,
				VarPos:   bv.VarPos,
			})
			replacementBindedVarNodes[bindedR.Id] = bindedR
		}
	}

	// Replace the var in node's term, incl filter
	// with the actual ResourceNode created above
	l.replaceVarInTerm(node, newVars)

	// Replace the var in node's consequents with the binded version
	for _, cons := range consequentsByVertex[node.Vertex] {
		l.replaceVarInTerm(cons, replacementBindedVarNodes)
	}

	// Visit the descendents
	for _, childVertex := range node.ChildrenVertexes {
		childNode := l.jetRuleModel.ReteNodes[childVertex]

		l.BuildBetaNodesRecursively(childNode, bindedVars, replacementBindedVarNodes, consequentsByVertex)
	}
}

// Replace the var in node's term, incl filter with the actual ResourceNode created above
func (l *JetRuleListener) replaceVarInTerm(term *rete.RuleTerm, newVars map[string]*rete.ResourceNode) {
	if term.SubjectKey > 0 {
		r := l.resourceManager.ResourceByKey[term.SubjectKey]
		if r.Type == "var" {
			newR := newVars[r.Id]
			if newR != nil {
				term.SubjectKey = newR.Key
			} else {
				fmt.Fprintf(l.parseLog, "** internal error: var %s is not found in replacement resource for SubjectKey (newVar %v)\n", r.Id, newVars)
				fmt.Fprintf(l.errorLog, "** internal error: var %s is not found in replacement resource for SubjectKey (newVar %v)\n", r.Id, newVars)
			}
		}
	}
	if term.PredicateKey > 0 {
		r := l.resourceManager.ResourceByKey[term.PredicateKey]
		if r.Type == "var" {
			newR := newVars[r.Id]
			if newR != nil {
				term.PredicateKey = newR.Key
			} else {
				fmt.Fprintf(l.errorLog, "** internal error: var %s is not found in replacement resource for PredicateKey \n", r.Id)
				fmt.Fprintf(l.parseLog, "** internal error: var %s is not found in replacement resource for PredicateKey\n", r.Id)
			}
		}
	}
	if term.ObjectKey > 0 {
		r := l.resourceManager.ResourceByKey[term.ObjectKey]
		if r.Type == "var" {
			newR := newVars[r.Id]
			if newR != nil {
				term.ObjectKey = newR.Key
			} else {
				fmt.Fprintf(l.errorLog, "** internal error: var %s is not found in replacement resource for ObjectKey\n", r.Id)
				fmt.Fprintf(l.parseLog, "** internal error: var %s is not found in replacement resource for ObjectKey\n", r.Id)
			}
		}
	}
	if term.Filter != nil {
		l.replaceVarInExpr(term.Filter, newVars)
	}
	if term.ObjectExpr != nil {
		l.replaceVarInExpr(term.ObjectExpr, newVars)
	}
}

func (s *JetRuleListener) replaceVarInExpr(expr *rete.ExpressionNode, newVars map[string]*rete.ResourceNode) {
	if expr == nil {
		return
	}
	if expr.Lhs != nil {
		s.replaceVarInExpr(expr.Lhs, newVars)
	}
	if expr.Rhs != nil {
		s.replaceVarInExpr(expr.Rhs, newVars)
	}
	if expr.Arg != nil {
		s.replaceVarInExpr(expr.Arg, newVars)
	}
	if expr.Value > 0 {
		r := s.resourceManager.ResourceByKey[expr.Value]
		if r.Type == "var" {
			newR := newVars[r.Id]
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
