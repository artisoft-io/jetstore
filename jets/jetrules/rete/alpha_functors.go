package rete

import (
	"fmt"

	"github.com/artisoft-io/jetstore/jets/jetrules/rdf"
)

// AlphaFunctor is the common interface shared by the Fu, Fv, and Fw functors parametrizing
// the AlphaNodes. The methods of the interface are:
//   - StaticValue (aka to_const) is used to determine the AlphaNode::register callback function
//   - Eval (aka to_AllOrRIndex) to evaluate functor 
//				- for antecedent term (to invoke find on the rdf_session), returns nil (case variable) or *rdf.Node (case binded var or cst)
//   			- for consequent and filter terms, returns *rdf.Node (case binded var or cst)
//   - BetaRowIndex (aka to_AVQ) Manage beta_row indexes in beta_relation according to the functors template arguments
//
type AlphaFunctor interface {
	InitializeExpression(reteSession *ReteSession) error
	StaticValue() *rdf.Node
	Eval(*ReteSession, *BetaRow) *rdf.Node
	BetaRowIndex() int
}

// Description of each functor:
//   - FConstant (aka F_cst): Constant resource, such as rdf:type in: (?s rdf:type ?C)
//   - FVariable (aka F_var): A variable as ?s in: (?s rdf:type ?C)
//   - FBinded (aka F_binded): A binded variable to a previous term, such as ?C in second term:
//     (?s rdf:type ?C).(?C subclassOf Thing)
//   - FExpression (aka F_expr): An expression involving binded variables and constant terms.
//

// Constant (aka F_cst): Constant resource, such as rdf:type in: (?s rdf:type ?C)
type FConstant struct {
	node *rdf.Node
}

func (af *FConstant) StaticValue() *rdf.Node {
	return af.node
}
func (af *FConstant) InitializeExpression(reteSession *ReteSession) error {
	return nil
}
func (af *FConstant) Eval(*ReteSession, *BetaRow) *rdf.Node {
	return af.node
}
func (af *FConstant) BetaRowIndex() int {
	return -1
}
func (af *FConstant) String() string {
	return fmt.Sprintf("cst(%s)", af.node)
}

// FVariable (aka F_var): A variable as ?s in: (?s rdf:type ?C)
type FVariable struct {
	variable string
}

func (af *FVariable) StaticValue() *rdf.Node {
	return nil
}
func (af *FVariable) InitializeExpression(reteSession *ReteSession) error {
	return nil
}
func (af *FVariable) Eval(*ReteSession, *BetaRow) *rdf.Node {
	return nil
}
func (af *FVariable) BetaRowIndex() int {
	return -1
}
func (af *FVariable) String() string {
	return fmt.Sprintf("var(%s)", af.variable)
}

// FBinded (aka F_binded): A binded variable to a previous term, such as ?C in second term:
//   (?s rdf:type ?C).(?C subclassOf Thing)
type FBinded struct {
	pos int
}

func (af *FBinded) StaticValue() *rdf.Node {
	return nil
}
func (af *FBinded) InitializeExpression(reteSession *ReteSession) error {
	return nil
}
func (af *FBinded) Eval(reteSession *ReteSession, row *BetaRow) *rdf.Node {
	return row.Get(af.pos)
}
func (af *FBinded) BetaRowIndex() int {
	return af.pos
}
func (af *FBinded) String() string {
	return fmt.Sprintf("binded(%d)", af.pos)
}

// FExpression (aka F_expr): An expression involving binded variables and constant terms.
type FExpression struct {
	expression Expression
}

func (af *FExpression) StaticValue() *rdf.Node {
	return nil
}
func (af *FExpression) InitializeExpression(reteSession *ReteSession) error {
	return af.expression.InitializeExpression(reteSession)
}
func (af *FExpression) Eval(reteSession *ReteSession, row *BetaRow) *rdf.Node {
	r := af.expression.Eval(reteSession, row)
	return reteSession.RdfSession.ResourceMgr.ReifyResource(r)
}
func (af *FExpression) BetaRowIndex() int {
	return -1
}
func (af *FExpression) String() string {
	return fmt.Sprintf("expr(%s)", af.expression)
}
