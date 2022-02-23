#ifndef JETS_RETE_EXPR_OPERATOR_FACTORY_H
#define JETS_RETE_EXPR_OPERATOR_FACTORY_H

#include <string>
#include <string_view>

#include <glog/logging.h>

#include "jets/rete/rete_err.h"
#include "jets/rete/expr_operators.h"
#include "jets/rete/expr.h"

// This file contains basic operator used in rule expression 
// see ExprUnaryOp and ExprBinaryOp classes.
namespace jets::rete {

inline ExprBasePtr
create_binary_expr(int key, ExprBasePtr lhs, std::string const& op, ExprBasePtr rhs)
{
  if(op == "+") return create_expr_binary_operator<AddVisitor>(key, lhs, rhs);
  if(op == "==") return create_expr_binary_operator<EqVisitor>(key, lhs, rhs);
  if(op == "<=") return create_expr_binary_operator<LeVisitor>(key, lhs, rhs);
  if(op == "r?") return create_expr_binary_operator<RegexVisitor>(key, lhs, rhs);
  LOG(ERROR) << "create_binary_expr: ERROR unknown binary operator: "<<
    op<<", called with key "<<key;
  RETE_EXCEPTION("create_binary_expr: ERROR unknown binary operator: "<<
    op<<", called with key "<<key);
}

inline ExprBasePtr
create_unary_expr(int key, std::string const& op, ExprBasePtr arg)
{
  if(op == "toUpper") return create_expr_unary_operator<ToUpperVisitor>(key, arg);
  if(op == "toLower") return create_expr_unary_operator<ToLowerVisitor>(key, arg);
  if(op == "trim") return create_expr_unary_operator<TrimVisitor>(key, arg);
  LOG(ERROR) << "create_unary_expr: ERROR unknown unary operator: "<<
    op<<", called with key "<<key;
  RETE_EXCEPTION("create_unary_expr: ERROR unknown unary operator: "<<
    op<<", called with key "<<key);
}


} // namespace jets::rete
#endif // JETS_RETE_EXPR_OPERATOR_FACTORY_H
