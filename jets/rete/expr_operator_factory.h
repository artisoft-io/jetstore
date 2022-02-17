#ifndef JETS_RETE_EXPR_OPERATOR_FACTORY_H
#define JETS_RETE_EXPR_OPERATOR_FACTORY_H

#include <string>
#include <string_view>

#include "jets/rete/expr_operators.h"
#include "jets/rete/expr.h"

// This file contains basic operator used in rule expression 
// see ExprUnaryOp and ExprBinaryOp classes.
namespace jets::rete {

inline ExprBasePtr
create_binary_expr(ExprBasePtr lhs, std::string_view op, ExprBasePtr rhs)
{
  if(op == "+") return create_expr_binary_operator<AddVisitor>(lhs, rhs);
  if(op == "==") return create_expr_binary_operator<EqVisitor>(lhs, rhs);
  if(op == "<=") return create_expr_binary_operator<LeVisitor>(lhs, rhs);
  if(op == "r?") return create_expr_binary_operator<RegexVisitor>(lhs, rhs);
  return {};
}

inline ExprBasePtr
create_unary_expr(std::string_view op, ExprBasePtr arg)
{
  if(op == "toUpper") return create_expr_unary_operator<ToUpperVisitor>(arg);
  if(op == "toLower") return create_expr_unary_operator<ToLowerVisitor>(arg);
  if(op == "trim") return create_expr_unary_operator<TrimVisitor>(arg);
  return {};
}


} // namespace jets::rete
#endif // JETS_RETE_EXPR_OPERATOR_FACTORY_H
