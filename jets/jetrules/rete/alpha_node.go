package rete

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
	IsAntecedent    bool
	NormalizedLabel string
}

func NewAlphaNode(fu, fv, fw AlphaFunctor, vertex *NodeVertex, isAntecedent bool, label string) *AlphaNode {
	return &AlphaNode{
		Fu: fu,
		Fv: fv,
		Fw: fw,
		NdVertex: vertex,
		IsAntecedent: isAntecedent,
		NormalizedLabel: label,
	}
}