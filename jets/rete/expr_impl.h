#ifndef JETS_RETE_EXPR_IMPL_H
#define JETS_RETE_EXPR_IMPL_H

#include "../rete/beta_row.h"
#include "../rete/expr.h"

// This file contains the implementation of the eval virtual method of the Expr
// implementation classes.
namespace jets::rete {
// Implementation Classes
// ======================================================================================
// ExprConjunction
// --------------------------------------------------------------------------------------
inline ExprConjunction::ExprDataType
ExprConjunction::eval(ReteSession * rete_session, BetaRow const* beta_row)const 
{
  ExprConjunction::ExprDataType v = rdf::LInt32(0);  // default value if list is empty
  for(auto const& item: this->data_) {
    v = item->eval(rete_session, beta_row);
    if(not rdf::to_bool(&v)) return v;
  }
  return v;
}

// ExprDisjunction
// --------------------------------------------------------------------------------------
inline ExprDisjunction::ExprDataType
ExprDisjunction::eval(ReteSession * rete_session, BetaRow const* beta_row)const 
{
  ExprDisjunction::ExprDataType v = rdf::LInt32(0);  // default value if list is empty
  for(auto const& item: this->data_) {
    v = item->eval(rete_session, beta_row);
    if(rdf::to_bool(&v)) return v;
  }
  return v;
}

// ExprCst
// --------------------------------------------------------------------------------------
inline ExprCst::ExprDataType
ExprCst::eval(ReteSession * rete_session, BetaRow const* beta_row)const 
{
  return data_;
}

// ExprBindedVar
// --------------------------------------------------------------------------------------
inline ExprBindedVar::ExprDataType
ExprBindedVar::eval(ReteSession * rete_session, BetaRow const* beta_row)const 
{
  if(beta_row == nullptr) return rdf::Null();
  return *beta_row->get(data_);
}

// ExprBinaryOp
// --------------------------------------------------------------------------------------
template<class Op>
typename ExprBinaryOp<Op>::ExprDataType
ExprBinaryOp<Op>::eval(ReteSession * rete_session, BetaRow const* beta_row)const 
{
  typename ExprBinaryOp<Op>::ExprDataType lhs = this->lhs_->eval(rete_session, beta_row);
  typename ExprBinaryOp<Op>::ExprDataType rhs = this->rhs_->eval(rete_session, beta_row);
  return boost::apply_visitor(Op(rete_session, beta_row), lhs, rhs);
}

// ExprUnaryOp
// --------------------------------------------------------------------------------------
template<class Op>
typename ExprUnaryOp<Op>::ExprDataType
ExprUnaryOp<Op>::eval(ReteSession * rete_session, BetaRow const* beta_row)const 
{
  return boost::apply_visitor(Op(rete_session, beta_row), this->arg_->eval(rete_session, beta_row));
}

} // namespace jets::rete
#endif // JETS_RETE_EXPR_IMPL_H
