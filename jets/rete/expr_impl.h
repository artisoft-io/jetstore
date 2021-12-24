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
template<class T>
typename ExprConjunction<T>::ExprDataType
ExprConjunction<T>::eval(ReteSession<T> * rete_session, BetaRow const* beta_row)const 
{
  ExprConjunction<T>::ExprDataType v = false;  // default value if list is empty
  for(auto const& item: this->data_) {
    v = item->eval(rete_session, beta_row);
    if(not rdf::to_bool(&v)) return v;
  }
  return v;
}

// ExprDisjunction
// --------------------------------------------------------------------------------------
template<class T>
typename ExprDisjunction<T>::ExprDataType
ExprDisjunction<T>::eval(ReteSession<T> * rete_session, BetaRow const* beta_row)const 
{
  ExprDisjunction<T>::ExprDataType v = false;  // default value if list is empty
  for(auto const& item: this->data_) {
    v = item->eval(rete_session, beta_row);
    if(rdf::to_bool(&v)) return v;
  }
  return v;
}

// ExprCst
// --------------------------------------------------------------------------------------
template<class T>
typename ExprCst<T>::ExprDataType
ExprCst<T>::eval(ReteSession<T> * rete_session, BetaRow const* beta_row)const 
{
  return data_;
}

// ExprBindedVar
// --------------------------------------------------------------------------------------
template<class T>
typename ExprBindedVar<T>::ExprDataType
ExprBindedVar<T>::eval(ReteSession<T> * rete_session, BetaRow const* beta_row)const 
{
  return *beta_row->get(data_);
}

// ExprBinaryOp
// --------------------------------------------------------------------------------------
template<class T, class Op>
typename ExprBinaryOp<T, Op>::ExprDataType
ExprBinaryOp<T, Op>::eval(ReteSession<T> * rete_session, BetaRow const* beta_row)const 
{
  ExprBinaryOp<T, Op>::ExprDataType lhs = this->lhs_->eval(rete_session, beta_row);
  ExprBinaryOp<T, Op>::ExprDataType rhs = this->rhs_->eval(rete_session, beta_row);
  ExprBinaryOp<T, Op>::ExprDataType result = this->oper_->eval(rete_session, lhs, rhs);
  return result;
}

// ExprUnaryOp
// --------------------------------------------------------------------------------------
template<class T, class Op>
typename ExprUnaryOp<T, Op>::ExprDataType
ExprUnaryOp<T, Op>::eval(ReteSession<T> * rete_session, BetaRow const* beta_row)const 
{
  ExprUnaryOp<T, Op>::ExprDataType arg = this->arg_->eval(rete_session, beta_row);
  ExprUnaryOp<T, Op>::ExprDataType result = this->oper_->eval(rete_session, arg);
  return result;
}

} // namespace jets::rete
#endif // JETS_RETE_EXPR_IMPL_H
