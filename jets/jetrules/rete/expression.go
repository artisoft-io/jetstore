package rete

import "github.com/artisoft-io/jetstore/jets/jetrules/rdf"

// This file contains the types for rule expression.
// Expression are used as:
//  - filter component of antecedent terms.
//  - object component of consequent terms.
// These classes are designed with consideration of expression evaluation speed and not
// building and manipulating the expression syntax tree.
// The expression parsing and transformation to it's final extression tree is done in the rule compiler.

type Expression interface {

	// Register callback with graph
	//
	// Applicable to operator having predicates as argument
	// that need to participate to the truth maintenance
	// e.g. operators exists and exists_not
	// Only some binary operator do participate to the truth maintenance
	RegisterCallback(reteSession *ReteSession, vertex int) error
	// Eval the Expression node
	Eval(reteSession *ReteSession, row *BetaRow) *rdf.Node
	// StaticEval Expression, used for RegisterCallback
	StaticEval(reteSession *ReteSession) *rdf.Node
	EvalFilter(reteSession *ReteSession, row *BetaRow) bool
}

// Expression Implementation

// Constant term
type ExprCst struct {
	data *rdf.Node
}

func NewExprCst(r *rdf.Node) Expression {
	return &ExprCst{data: r}
}

func (expr *ExprCst) RegisterCallback(reteSession *ReteSession, vertex int) error {
	return nil
}

func (expr *ExprCst) Eval(reteSession *ReteSession, row *BetaRow) *rdf.Node {
	return expr.data
}

func (expr *ExprCst) StaticEval(reteSession *ReteSession) *rdf.Node {
	return expr.data
}

func (expr *ExprCst) EvalFilter(reteSession *ReteSession, row *BetaRow) bool {
	if expr.data == nil {
		return false
	}
	return expr.data.Bool()
}

// Binded variable term
type ExprBindedVar struct {
	data int
}

func NewExprBindedVar(idx int) Expression {
	return &ExprBindedVar{data: idx}
}

func (expr *ExprBindedVar) RegisterCallback(reteSession *ReteSession, vertex int) error {
	return nil
}

func (expr *ExprBindedVar) Eval(reteSession *ReteSession, row *BetaRow) *rdf.Node {
	if row == nil {
		return rdf.Null()
	}
	v := row.Get(expr.data)
	if v == nil {
		return rdf.Null()
	}
	return v
}

func (expr *ExprBindedVar) StaticEval(reteSession *ReteSession) *rdf.Node {
	return nil
}

func (expr *ExprBindedVar) EvalFilter(reteSession *ReteSession, row *BetaRow) bool {
	v := expr.Eval(reteSession, row)
	return v.Bool()
}

// Binary Operator term
type ExprBinaryOp struct {
	op  BinaryOperator
	lhs Expression
	rhs Expression
}

func NewExprBinaryOp(lhs Expression, op BinaryOperator, rhs Expression) Expression {
	return &ExprBinaryOp{op: op, lhs: lhs, rhs: rhs}
}

func (expr *ExprBinaryOp) RegisterCallback(reteSession *ReteSession, vertex int) error {
	// Propagate the RegisterCallback
	expr.lhs.RegisterCallback(reteSession, vertex)
	expr.rhs.RegisterCallback(reteSession, vertex)

	// perform StaticEval for calling RegisterCallback on the operator
	lhs := expr.lhs.StaticEval(reteSession)
	rhs := expr.rhs.StaticEval(reteSession)
	return expr.op.RegisterCallback(reteSession, vertex, lhs, rhs)
}

func (expr *ExprBinaryOp) Eval(reteSession *ReteSession, row *BetaRow) *rdf.Node {
	lhs := expr.lhs.Eval(reteSession, row)
	rhs := expr.rhs.Eval(reteSession, row)
	return expr.op.Eval(reteSession, row, lhs, rhs)
}

func (expr *ExprBinaryOp) StaticEval(reteSession *ReteSession) *rdf.Node {
	return nil
}

func (expr *ExprBinaryOp) EvalFilter(reteSession *ReteSession, row *BetaRow) bool {
	v := expr.Eval(reteSession, row)
	return v.Bool()
}

// Unary Operator term
type ExprUnaryOp struct {
	op  UnaryOperator
	rhs Expression
}

func NewExprUnaryOp(op UnaryOperator, rhs Expression) Expression {
	return &ExprUnaryOp{op: op, rhs: rhs}
}

func (expr *ExprUnaryOp) RegisterCallback(reteSession *ReteSession, vertex int) error {
	// Propagate the RegisterCallback
	expr.rhs.RegisterCallback(reteSession, vertex)

	// perform StaticEval for calling RegisterCallback on the operator
	rhs := expr.rhs.StaticEval(reteSession)
	return expr.op.RegisterCallback(reteSession, vertex, rhs)
}

func (expr *ExprUnaryOp) Eval(reteSession *ReteSession, row *BetaRow) *rdf.Node {
	rhs := expr.rhs.Eval(reteSession, row)
	return expr.op.Eval(reteSession, row, rhs)
}

func (expr *ExprUnaryOp) StaticEval(reteSession *ReteSession) *rdf.Node {
	return nil
}

func (expr *ExprUnaryOp) EvalFilter(reteSession *ReteSession, row *BetaRow) bool {
	v := expr.Eval(reteSession, row)
	return v.Bool()
}
