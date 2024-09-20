package rete

import (
	"log"

	"github.com/artisoft-io/jetstore/jets/jetrules/rdf"
)

// AlphaNode type -- is a connector to the rdf graph for a antecedent or consequent term
// Note: following descriptioin is comming from JetStore c++ code.
//
//	in the form of a triple (u, v, w) where
//	   - u and v can be ?s, or a constant
//	   - w can be ?s, constant, or an expression
//
// AlphaNode is parametrized by a triplet of functors: (Fu, Fv, Fw)
//
// A beta relation is associated with a rule antecedent and possibly multiple
// rule consequent terms.
// Rule antecedent and consequent terms are represented by AlphaNodes.
// As a result, there are more AlphaNodes than BetaRelations, however
// each AlphaNode point to a NodeVertex corresponding to the associated
// beta relation of the AlphaNode.
//
// Specialized AlphaNode classes exist to represent the various rule terms
// the rete network must support.
// Two classes of AlphaNodes exists:
//   - AlphaNodes representing Antecedent Terms
//   - AlphaNodes representing Consequent Terms
//
// In both cases, specialized AlphaNodes are parametrized with 3 functors:
// AlphaNode<Fu, Fv, Fw>. The possible functor are different for
// antecedent and consequent terms.
//
// Possible functor for antecedent terms:
//   - Fu: F_binded, F_var, F_cst
//   - Fv: F_binded, F_var, F_cst
//   - Fw: F_binded, F_var, F_cst
//
// Possible functor for consequent terms:
//   - Fu: F_binded, F_cst
//   - Fv: F_binded, F_cst
//   - Fw: F_binded, F_cst, F_expr
//
// Description of each functor:
//   - F_cst: Constant resource, such as rdf:type in: (?s rdf:type ?C)
//   - F_var: A variable as ?s in: (?s rdf:type ?C)
//   - F_binded: A binded variable to a previous term, such as ?C in:
//     (?s rdf:type ?C).(?C subclassOf Thing)
//   - F_expr: An expression involving binded variables and constant terms.
//
// Note that consequent terms cannot have unbinded variable, so F_var
// is not applicable.
//
// Note: AlphaNodes contains metadata information only and are managed by the	ReteMetaStore.

// AphaNode is a connector to the rdf graph for a antecedents and consequents term.
type AlphaNode struct {
	Fu              AlphaFunctor
	Fv              AlphaFunctor
	Fw              AlphaFunctor
	NdVertex        *NodeVertex
	IsHeadNode      bool
	IsAntecedent    bool
	IsConsequent    bool
	NormalizedLabel string
}

func NewAlphaNode(fu, fv, fw AlphaFunctor, vertex *NodeVertex, isAntecedent bool, label string) *AlphaNode {
	return &AlphaNode{
		Fu:              fu,
		Fv:              fv,
		Fw:              fw,
		NdVertex:        vertex,
		IsHeadNode:      vertex.Vertex == 0,
		IsAntecedent:    isAntecedent,
		IsConsequent:    vertex.Vertex > 0 && !isAntecedent,
		NormalizedLabel: label,
	}
}

func NewRootAlphaNode(vertex *NodeVertex) *AlphaNode {
	return &AlphaNode{
		Fu:              &FVariable{variable: "*"},
		Fv:              &FVariable{variable: "*"},
		Fw:              &FVariable{variable: "*"},
		NdVertex:        vertex,
		IsHeadNode:      true,
		IsAntecedent:    false,
		IsConsequent:    false,
		NormalizedLabel: "(* * *)",
	}
}

func (an *AlphaNode) RegisterCallback(rs *ReteSession) {
	u := an.Fu.StaticValue()
	v := an.Fv.StaticValue()
	w := an.Fw.StaticValue()
	// //***
	// if an.NdVertex.Vertex == 119 {
	// 	log.Printf("Vertex 119 = RegisterCallback w/ (%v, %v, %v)", u, v, w)
	// }
	cb := NewReteCallback(rs, an.NdVertex.Vertex, u, v, w)

	rs.RdfSession.AssertedGraph.CallbackMgr.AddCallback(cb)
	rs.RdfSession.InferredGraph.CallbackMgr.AddCallback(cb)
}

func (an *AlphaNode) InitializeIndexes(parentBetaRelation *BetaRelation) {
	if an == nil {
		return
	}
	if !an.IsAntecedent {
		// Not applicable
		return
	}
	u := an.Fu.BetaRowIndex()
	v := an.Fv.BetaRowIndex()
	w := an.Fw.BetaRowIndex()
	uI := u >= 0
	u0 := u == -1
	vI := v >= 0
	v0 := v == -1
	wI := w >= 0
	w0 := w == -1
	switch {
	case uI && v0 && w0:
		an.NdVertex.AntecedentQueryKey = parentBetaRelation.AddQuery1()
	case uI && vI && w0:
		an.NdVertex.AntecedentQueryKey = parentBetaRelation.AddQuery2()
	case u0 && vI && wI:
		an.NdVertex.AntecedentQueryKey = parentBetaRelation.AddQuery2()
	case u0 && v0 && wI:
		an.NdVertex.AntecedentQueryKey = parentBetaRelation.AddQuery1()
	case uI && v0 && wI:
		an.NdVertex.AntecedentQueryKey = parentBetaRelation.AddQuery2()
	case u0 && vI && w0:
		an.NdVertex.AntecedentQueryKey = parentBetaRelation.AddQuery1()
	case uI && vI && wI:
		an.NdVertex.AntecedentQueryKey = parentBetaRelation.AddQuery3()
	}
}

func (an *AlphaNode) AddIndex4BetaRow(parentBetaRelation *BetaRelation, row *BetaRow) {
	if an == nil {
		return
	}
	if !an.IsAntecedent {
		// Not applicable
		return
	}
	key := an.NdVertex.AntecedentQueryKey
	u := an.Fu.BetaRowIndex()
	v := an.Fv.BetaRowIndex()
	w := an.Fw.BetaRowIndex()
	uI := u >= 0
	u0 := u == -1
	vI := v >= 0
	v0 := v == -1
	wI := w >= 0
	w0 := w == -1
	switch {
	case uI && v0 && w0:
		parentBetaRelation.AddIndex1(key, an.Fu.Eval(nil, row), row)
	case uI && vI && w0:
		parentBetaRelation.AddIndex2(key, an.Fu.Eval(nil, row), an.Fv.Eval(nil, row), row)
	case u0 && vI && wI:
		parentBetaRelation.AddIndex2(key, an.Fv.Eval(nil, row), an.Fw.Eval(nil, row), row)
	case u0 && v0 && wI:
		parentBetaRelation.AddIndex1(key, an.Fw.Eval(nil, row), row)
	case uI && v0 && wI:
		parentBetaRelation.AddIndex2(key, an.Fu.Eval(nil, row), an.Fw.Eval(nil, row), row)
	case u0 && vI && w0:
		parentBetaRelation.AddIndex1(key, an.Fv.Eval(nil, row), row)
	case uI && vI && wI:
		parentBetaRelation.AddIndex3(key, an.Fu.Eval(nil, row), an.Fv.Eval(nil, row), an.Fw.Eval(nil, row), row)
	}
}

func (an *AlphaNode) EraseIndex4BetaRow(parentBetaRelation *BetaRelation, row *BetaRow) {
	if an == nil {
		return
	}
	if !an.IsAntecedent {
		// Not applicable
		return
	}
	key := an.NdVertex.AntecedentQueryKey
	u := an.Fu.BetaRowIndex()
	v := an.Fv.BetaRowIndex()
	w := an.Fw.BetaRowIndex()
	uI := u >= 0
	u0 := u == -1
	vI := v >= 0
	v0 := v == -1
	wI := w >= 0
	w0 := w == -1
	switch {
	case uI && v0 && w0:
		parentBetaRelation.EraseIndex1(key, an.Fu.Eval(nil, row), row)
	case uI && vI && w0:
		parentBetaRelation.EraseIndex2(key, an.Fu.Eval(nil, row), an.Fv.Eval(nil, row), row)
	case u0 && vI && wI:
		parentBetaRelation.EraseIndex2(key, an.Fv.Eval(nil, row), an.Fw.Eval(nil, row), row)
	case u0 && v0 && wI:
		parentBetaRelation.EraseIndex1(key, an.Fw.Eval(nil, row), row)
	case uI && v0 && wI:
		parentBetaRelation.EraseIndex2(key, an.Fu.Eval(nil, row), an.Fw.Eval(nil, row), row)
	case u0 && vI && w0:
		parentBetaRelation.EraseIndex1(key, an.Fv.Eval(nil, row), row)
	case uI && vI && wI:
		parentBetaRelation.EraseIndex3(key, an.Fu.Eval(nil, row), an.Fu.Eval(nil, row), an.Fu.Eval(nil, row), row)
	}
}

// Called to query rows from parent beta node matching (s, p, o), case merging with new triples from inferred graph
// Applicable to antecedent terms only, will panic otherwise
func (an *AlphaNode) FindMatchingRows(parentBetaRelation *BetaRelation, s, p, o *rdf.Node) map[*BetaRow]bool {
	if an == nil {
		return nil
	}
	if !an.IsAntecedent {
		log.Panic("bug: AlphaNode.FindMatchingRows called on consequent term")
	}
	u := an.Fu.BetaRowIndex()
	v := an.Fv.BetaRowIndex()
	w := an.Fw.BetaRowIndex()
	key := an.NdVertex.AntecedentQueryKey
	uI := u >= 0
	u0 := u == -1
	vI := v >= 0
	v0 := v == -1
	wI := w >= 0
	w0 := w == -1
	// //**
	// if an.NdVertex.ParentNodeVertex.Vertex == 0 {
	// 	log.Printf("vertex %d (parent vertex %d) FindMatchingRows: %s, BetaRowIndex: (Fu: %d, Fv: %d, Fw: %d)", an.NdVertex.Vertex, an.NdVertex.ParentNodeVertex.Vertex, rdf.ToString(&[3]*rdf.Node{s, p, o}), u, v, w)
	// }
	switch {
	case uI && v0 && w0:
		return parentBetaRelation.FindMatchingRows1(key, s)
	case uI && vI && w0:
		return parentBetaRelation.FindMatchingRows2(key, s, p)
	case u0 && v0 && wI:
		return parentBetaRelation.FindMatchingRows1(key, o)
	case u0 && vI && wI:
		return parentBetaRelation.FindMatchingRows2(key, p, o)
	case uI && v0 && wI:
		return parentBetaRelation.FindMatchingRows2(key, s, o)
	case u0 && vI && w0:
		return parentBetaRelation.FindMatchingRows1(key, p)
	case uI && vI && wI:
		return parentBetaRelation.FindMatchingRows3(key, s, p, o)
	case u0 && v0 && w0:
		if parentBetaRelation.NdVertex.Vertex == 0 && len(parentBetaRelation.rowIndexes0) == 0 {
			log.Panicf("root node has no beta row!!")
		}
		return parentBetaRelation.rowIndexes0
	}
	return nil
}

/**
   * @brief Get all triples from rdf session matching `parent_row`
   *
   * Invoking the functors to_AllOrRIndex methods, case:
   *  - F_cst: return the rdf resource of the functor (constant value)
   *  - F_binded: return the binded rdf resource from parent_row @ index of the functor.
   *  - F_var: return 'any' (StarMatch) to indicate a unbinded variable
   *
   * Applicable to antecedent terms only, call during initial graph visit only
   * Will throw if called on a consequent term
   * @param rdf_session
   * @param parent_row
   * @return AlphaNode::Iterator = rdf::RDFSession::Iterator
	 * from c++ implementation
*/
func (an *AlphaNode) FindMatchingTriples(rs *ReteSession, parentRow *BetaRow) *rdf.RdfSessionIterator {
	// //***
	// if an.NdVertex.Vertex == 119 {
	// 	log.Printf("* FindMatchingTriples for alpha node %d: %s called", an.NdVertex.Vertex, an.NormalizedLabel)
	// }

	if !an.IsAntecedent {
		log.Panicf("AlphaNode.FindMatchingTriples called on non antecedent node, vertex %d", an.NdVertex.Vertex)
	}
	return rs.RdfSession.FindSPO(an.Fu.Eval(rs, parentRow), an.Fv.Eval(rs, parentRow), an.Fw.Eval(rs, parentRow))
}

// Return consequent `triple` for BetaRow
// Applicable to consequent terms only,
// will panic if called on an antecedent term
func (an *AlphaNode) ComputeConsequentTriple(rs *ReteSession, row *BetaRow) *rdf.Triple {
	if an == nil {
		return nil
	}
	if an.IsAntecedent {
		log.Panic("bug: AlphaNode.ComputeConsequentTriple called on antecedent term")
	}
	return &rdf.Triple{
		an.Fu.Eval(rs, row),
		an.Fv.Eval(rs, row),
		an.Fw.Eval(rs, row),
	}
}
