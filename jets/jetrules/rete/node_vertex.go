package rete

// NodeVertex is the metadata node associated with a BetaNode / antecedent AlphaNode
// This is the key rete network construct

// Note the child_nodes and consequent_alpha_vertexes properties are set after
// construction by ReteMetaStore as part of metadata initialization routine.
type NodeVertex struct {
	Vertex                      int
	ParentNodeVertex            *NodeVertex
	ChildNodes                  map[*NodeVertex]bool
	ConsequentAlphaNodeVertexes map[int]bool
	IsNegation                  bool
	Salience                    int
	FilterExpr                  *Expression
	NormalizedLabel             string
	RowInitializer              *BetaRowInitializer
	AntecedentQueryKey          int
}

func NewNodeVertex(vertex int, parent *NodeVertex, isNeg bool, salience int,
	filter *Expression, label string, rowInitializer *BetaRowInitializer) *NodeVertex {
	return &NodeVertex{
		Vertex:                      vertex,
		ParentNodeVertex:            parent,
		ChildNodes:                  make(map[*NodeVertex]bool),
		ConsequentAlphaNodeVertexes: make(map[int]bool),
		IsNegation:                  isNeg,
		Salience:                    salience,
		FilterExpr:                  filter,
		NormalizedLabel:             label,
		RowInitializer:              rowInitializer,
	}
}

func (node *NodeVertex) IsHead() bool {
	return node.Vertex == 0
}

func (node *NodeVertex) HasExpression() bool {
	return node.FilterExpr != nil
}

// Children as descendent NodeVertex (antecedent AlphaNode)
func (node *NodeVertex) HasChildren() bool {
	return len(node.ChildNodes) > 0
}

func (node *NodeVertex) HasConsequentTerms() bool {
	return len(node.ConsequentAlphaNodeVertexes) > 0
}
