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
	// RecordsConsequents is set by AddConsequentTerm when a consequent term mints a
	// resource (see expr_mints_resource.go) or, with LEGACY defect 1 turned off, reads
	// the graph (expr_reads_graph.go). A beta row of such a vertex records the triples
	// it inferred, and retraction replays that record rather than recomputing the
	// consequents.
	RecordsConsequents bool
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

// AddConsequentTerm adds a consequent term to the vertex and decides, once, whether
// the vertex records what its rows infer. A consequent that mints a resource always
// records. One that reads the graph records only when mode turns LEGACY defect 1 off
// (RetractionMode.ReplayGraphReadingConsequents); in the legacy mode it is recomputed
// on retraction, which is the defect (README.md, 2026-10-09).
func (node *NodeVertex) AddConsequentTerm(alphaNd *AlphaNode, mode RetractionMode) {
	node.ConsequentAlphaNodes = append(node.ConsequentAlphaNodes, alphaNd)
	if alphaNd.MintsResource() || (mode.ReplayGraphReadingConsequents && alphaNd.ReadsGraph()) {
		node.RecordsConsequents = true
	}
}
