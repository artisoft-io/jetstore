#ifndef JETS_RETE_EXPR_IMPL_H
#define JETS_RETE_EXPR_IMPL_H

#include "jets/rete/beta_row.h"
#include "jets/rete/expr.h"

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
  return *beta_row->get(data_);
}

// ExprBinaryOp
// --------------------------------------------------------------------------------------
template<class Op>
typename ExprBinaryOp<Op>::ExprDataType
ExprBinaryOp<Op>::eval(ReteSession * rete_session, BetaRow const* beta_row)const 
{
  ExprBinaryOp<Op>::ExprDataType lhs = this->lhs_->eval(rete_session, beta_row);
  ExprBinaryOp<Op>::ExprDataType rhs = this->rhs_->eval(rete_session, beta_row);
  ExprBinaryOp<Op>::ExprDataType result = this->oper_->eval(rete_session, lhs, rhs);
  return result;
}

// ExprUnaryOp
// --------------------------------------------------------------------------------------
template<class Op>
typename ExprUnaryOp<Op>::ExprDataType
ExprUnaryOp<Op>::eval(ReteSession * rete_session, BetaRow const* beta_row)const 
{
  ExprUnaryOp<Op>::ExprDataType arg = this->arg_->eval(rete_session, beta_row);
  ExprUnaryOp<Op>::ExprDataType result = this->oper_->eval(rete_session, arg);
  return result;
}

} // namespace jets::rete
#endif // JETS_RETE_EXPR_IMPL_H
