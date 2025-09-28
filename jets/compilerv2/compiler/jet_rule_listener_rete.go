package compiler

// This file contains the function to transform the rules into a Rete network

import (
	"fmt"
	"sort"

	"github.com/artisoft-io/jetstore/jets/jetrules/rete"
)

// BuildReteNetwork builds the Rete network from the rules in the Jetrule model
// It creates the Alpha nodes with Beta nodes configuration and connects them according to the rules
// The Rete network optimizations is performed prior to this step.
//
// Augment JetRule structure with rete markups:
//   - Add to antecedent: parent_vertex and vertex
//   - Add to consequent: vertex
//
// parent_vertex and vertex are integers
// --
// Approach:
// Build a rete network with beta nodes corresponding to rule antecedents.
// Add the rule's antecedent to the rete network.
// Connect nodes across rules by matching normalized labels (merging common antecedents)
func (l *JetRuleListener) BuildReteNetwork() {
	if l.trace {
		fmt.Fprintf(l.parseLog, "** entering BuildReteNetwork for %d rules\n", len(l.jetRuleModel.Jetrules))
	}

	// Prepare the ReteNodes structure by initializing it with the root node
	l.jetRuleModel.ReteNodes = make([]*rete.RuleTerm, 0, len(l.jetRuleModel.Jetrules))
	rootNode := &rete.RuleTerm{
		Vertex:             0,
		Type:               "root",
		ParentVertex:       -1, // root has no parent
		ChildrenVertexes:   []int{},
		BetaVarNodes:       []*rete.BetaVarNode{},
		SelfVars:           make(map[string]*int),
		ParentBindedVars:   make(map[string]bool),
		DescendentsReqVars: make(map[string]bool),
		Rules:              []string{},
		Salience:           []int{},
		NormalizedLabel:    "root",
		// Other fields are nil or zeroed
	}
	// Node.Vertex is the index in the ReteNodes slice for antecedents, for consequents
	// it is the index of it's parent antecedent node
	// The root node is always at index 0
	l.jetRuleModel.ReteNodes = append(l.jetRuleModel.ReteNodes, rootNode)

	// Optimize the rete network construction by selectioning the most
	// common 1st antecedent among all rules (TODO)

	// For each rule, add its antecedents to the Rete network
	for _, rule := range l.jetRuleModel.Jetrules {
		fmt.Fprintf(l.parseLog, "** Processing rule: %s\n", rule.Name)
		// for each antecedent, look for a node with the same normalized label
		// if found, use it as the rete node
		// if not found, use this rule as the rete node.
		parentVertex := 0 // start at root
		for i, antecedent := range rule.Antecedents {
			fmt.Fprintf(l.parseLog, "   Antecedent %d: %s\n", i+1, antecedent.NormalizedLabel)
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
				// Add this node as a child of the parent node
				parentNode.ChildrenVertexes = append(parentNode.ChildrenVertexes, reteNode.Vertex)

				// Update the internal state
				reteNode.SelfVars = make(map[string]*int)
				reteNode.DescendentsReqVars = make(map[string]bool)
				reteNode.ParentBindedVars = make(map[string]bool)

				// Carry the binded variables from parent to child
				for v := range parentNode.ParentBindedVars {
					reteNode.ParentBindedVars[v] = true
				}
				// Add the parent's now binded variables
				for v := range parentNode.SelfVars {
					reteNode.ParentBindedVars[v] = true
				}

				// Collect the unbinded variables in the antecedent
				l.collectUnbindedVars(reteNode.ParentBindedVars, reteNode.SelfVars, reteNode)
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
		// Carry the consequents' self var to last antecedent's descendent required vars
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

	// Set the rete node descendent required vars
	for _, node := range l.jetRuleModel.ReteNodes {
		if node.DescendentsReqVars == nil {
			node.DescendentsReqVars = make(map[string]bool)
		}
		// Collect the descendent required vars
		l.CollectDescendentsReqVars(node.DescendentsReqVars, node)
	}

	// Now the rete network (l.jetRuleModel.ReteNodes) is in place
	// Do post processing:
	// - For each node, compute the beta relation variables
	// - For each node, prune variables that are not needed by descendent nodes
	// - For each node, create the BetaVarNodes structure
	for _, node := range l.jetRuleModel.ReteNodes {
		if node.Type == "root" || node.Type == "consequent" {
			continue
		}
		// Compute the beta relation variables
		// collect in a set the variables that are in SelfVars or ParentBindedVars
		varSet := make(map[string]bool, len(node.SelfVars)+len(node.ParentBindedVars))
		for v := range node.SelfVars {
			varSet[v] = true
		}
		for v := range node.ParentBindedVars {
			varSet[v] = true
		}
		// Convert the set to a slice
		node.BetaRelationVars = make([]string, 0, len(varSet))
		for v := range varSet {
			node.BetaRelationVars = append(node.BetaRelationVars, v)
		}
		// Sort the BetaRelationVars slice
		sort.Strings(node.BetaRelationVars)

		// Compute the pruned variables
		// saves them in a set for computing the BetaVarNodes structure
		prunedVarSet := make(map[string]bool, len(node.BetaRelationVars))
		for v := range node.BetaRelationVars {
			if _, ok := node.DescendentsReqVars[node.BetaRelationVars[v]]; !ok {
				// Not needed by descendents, prune it
				node.PrunedVars = append(node.PrunedVars, node.BetaRelationVars[v])
				prunedVarSet[node.BetaRelationVars[v]] = true
			}
		}
		// Create the BetaVarNodes structure
		node.BetaVarNodes = make([]*rete.BetaVarNode, 0, len(node.BetaRelationVars))
		for v := range node.BetaRelationVars {
			nodeId := node.BetaRelationVars[v]
			// Only add it if not pruned
			if !prunedVarSet[nodeId] {
				// Not pruned, create a BetaVarNode
				// Check if var is binded in parent
				var isBinded bool
				var pos int
				if node.ParentBindedVars[nodeId] {
					isBinded = true
					pos = len(node.BetaVarNodes) // position in BetaVarNodes
				} else {
					v := node.SelfVars[nodeId]
					if v != nil {
						pos = *v // position in triple (0-based)
					} else {
						fmt.Fprintf(l.errorLog, "** ERROR: variable %s not found in SelfVars of node %d\n", nodeId, node.Vertex)
					}
				}
				node.BetaVarNodes = append(node.BetaVarNodes, &rete.BetaVarNode{
					Type:     "var",
					Id:       nodeId,
					IsBinded: isBinded,
					Vertex:   node.Vertex,
					VarPos:   pos,
				})
			}
		}
		// Sort the BetaVarNodes by Id (variable name)
		sort.Slice(node.BetaVarNodes, func(i, j int) bool {
			return node.BetaVarNodes[i].Id < node.BetaVarNodes[j].Id
		})

		if l.trace {
			fmt.Fprintf(l.parseLog, "Rete Node %d: %s\n", node.Vertex, node.NormalizedLabel)
			fmt.Fprintf(l.parseLog, "   Parent Vertex: %d\n", node.ParentVertex)
			fmt.Fprintf(l.parseLog, "   Children Vertexes: %v\n", node.ChildrenVertexes)
			fmt.Fprintf(l.parseLog, "   Self Vars: %v\n", node.SelfVars)
			fmt.Fprintf(l.parseLog, "   Parent Binded Vars: %v\n", node.ParentBindedVars)
			fmt.Fprintf(l.parseLog, "   Descendents Required Vars: %v\n", node.DescendentsReqVars)
			fmt.Fprintf(l.parseLog, "   Beta Relation Vars: %v\n", node.BetaRelationVars)
			fmt.Fprintf(l.parseLog, "   Pruned Vars: %v\n", node.PrunedVars)
			fmt.Fprintf(l.parseLog, "   Beta Var Nodes:\n")
			for _, bv := range node.BetaVarNodes {
				fmt.Fprintf(l.parseLog, "      Id: %s, IsBinded: %t, VarPos: %d\n", bv.Id, bv.IsBinded, bv.VarPos)
			}
			fmt.Fprintf(l.parseLog, "   Rules: %v\n", node.Rules)
			fmt.Fprintf(l.parseLog, "   Salience: %v\n", node.Salience)
		}
	}

	if l.trace {
		fmt.Fprintf(l.parseLog, "** Rete Network built with %d nodes\n", len(l.jetRuleModel.ReteNodes))
	} else {
		// Clean up the rete nodes, remove ParentBindedVars, DescendentsReqVars, SelfVars,
		// BetaRelationVars, and PrunedVars as they
		// are no longer needed
		for _, node := range l.jetRuleModel.ReteNodes {
			node.ParentBindedVars = nil
			node.DescendentsReqVars = nil
			node.SelfVars = nil
			node.BetaRelationVars = nil
			node.PrunedVars = nil
		}
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
