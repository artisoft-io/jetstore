package rete

import "fmt"

// ReteMetaStore -- metadata store for a rete network named by it's ruleset uri
// The ReteMetaStore correspond to a complete rule set organized as a rete network.
type ReteMetaStore struct {
	LookupHelper *LookupSqlHelper
	AlphaNodes   []*AlphaNode
	NodeVertices []*NodeVertex
}

func NewReteMetaStore(h *LookupSqlHelper, an []*AlphaNode, nv []*NodeVertex) (*ReteMetaStore, error) {
	ms := &ReteMetaStore{
		LookupHelper: h,
		AlphaNodes:   an,
		NodeVertices: nv,
	}
	// Initialize routine perform important connection between the
	// metadata entities, such as reverse lookup of the consequent terms
	// and children lookup for each NodeVertex.
	var err error
	// Perform reverse lookup of children NodeVertex (AlphaNode) using
	// the NodeVertex parentNode property
	for _, node := range ms.NodeVertices {
		// Root node does not have a parent node
		if node.ParentNodeVertex != nil {
			node.ParentNodeVertex.AddChildAlphaNode(ms.AlphaNodes[node.Vertex])
		}
	}

	// Assign consequent terms vertex (AlphaNode) to NodeVertex
	// and validate that alpha node at ipos < nbr_vertices are antecedent nodes
	nbrVertices := ms.NbrVertices()
	for ipos, alphaNode := range ms.AlphaNodes {
		if ipos < nbrVertices && !alphaNode.IsAntecedent {
			err = fmt.Errorf("NewReteMetaStore: AlphaNode with vertex %d < nbrVertices %d that is NOT antecedent term",
				ipos, nbrVertices)
			return nil, err
		}
		if !alphaNode.IsAntecedent {
			alphaNode.NdVertex.AddConsequentTerm(alphaNode)
		}
	}
	return ms, nil
}

func (ms *ReteMetaStore) NbrVertices() int {
	if ms == nil {
		return 0
	}
	return len(ms.NodeVertices)
}
