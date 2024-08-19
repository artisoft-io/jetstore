package rete

// NodeVertex is the metadata node associated with a BetaNode / antecedent AlphaNode
// This is the key rete network construct

// Note the child_nodes and consequent_alpha_vertexes properties are set after
// construction by ReteMetaStore as part of metadata initialization routine.
type NodeVertex struct {
	Vertex               int
	ParentNodeVertex     *NodeVertex
	ChildAlphaNodes      []*AlphaNode
	ConsequentAlphaNodes []*AlphaNode
	IsNegation           bool
	Salience             int
	FilterExpr           Expression
	NormalizedLabel      string
	RowInitializer       *BetaRowInitializer
	AntecedentQueryKey   int
	AssociatedRules      []string
}

func NewNodeVertex(vertex int, parent *NodeVertex, isNeg bool, salience int,
	filter Expression, label string, rules []string, rowInitializer *BetaRowInitializer) *NodeVertex {
	return &NodeVertex{
		Vertex:               vertex,
		ParentNodeVertex:     parent,
		ChildAlphaNodes:      make([]*AlphaNode, 0),
		ConsequentAlphaNodes: make([]*AlphaNode, 0),
		IsNegation:           isNeg,
		Salience:             salience,
		FilterExpr:           filter,
		NormalizedLabel:      label,
		AssociatedRules:      rules,
		RowInitializer:       rowInitializer,
	}
}

func (node *NodeVertex) String() string {
	if node.IsHead() {
		return "head_node (*, *, *)"
	}
	return node.NormalizedLabel
}

func (node *NodeVertex) IsHead() bool {
	return node.Vertex == 0
}

func (node *NodeVertex) HasExpression() bool {
	return node.FilterExpr != nil
}

// Children as descendent NodeVertex (antecedent AlphaNode)
func (node *NodeVertex) HasChildren() bool {
	return len(node.ChildAlphaNodes) > 0
}

func (node *NodeVertex) HasConsequentTerms() bool {
	return len(node.ConsequentAlphaNodes) > 0
}

func (node *NodeVertex) AddChildAlphaNode(alphaNd *AlphaNode) {
	node.ChildAlphaNodes = append(node.ChildAlphaNodes, alphaNd)
}

func (node *NodeVertex) AddConsequentTerm(alphaNd *AlphaNode) {
	node.ConsequentAlphaNodes = append(node.ConsequentAlphaNodes, alphaNd)
}
