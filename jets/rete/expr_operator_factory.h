#ifndef JETS_RETE_EXPR_OPERATOR_FACTORY_H
#define JETS_RETE_EXPR_OPERATOR_FACTORY_H

#include <string>
#include <string_view>

#include <glog/logging.h>

#include "../rete/rete_err.h"
#include "../rete/expr_op_arithmetics.h"
#include "../rete/expr_op_logics.h"
#include "../rete/expr_op_strings.h"
#include "../rete/expr_op_resources.h"
#include "../rete/expr_op_others.h"
#include "../rete/expr.h"

// This file contains basic operator used in rule expression 
// see ExprUnaryOp and ExprBinaryOp classes.
namespace jets::rete {

ExprBasePtr
ReteMetaStoreFactory::create_binary_expr(int key, ExprBasePtr lhs, std::string const& op, ExprBasePtr rhs)
{
  // BINARY OPERATORS
  // ------------------------------------------------------------------------------------
  // Arithmetic operators
  if(op == "+")                 return create_expr_binary_operator<AddVisitor>(key, lhs, rhs);
  if(op == "-")                 return create_expr_binary_operator<SubsVisitor>(key, lhs, rhs);
  if(op == "/")                 return create_expr_binary_operator<DivVisitor>(key, lhs, rhs);
  if(op == "*")                 return create_expr_binary_operator<MultVisitor>(key, lhs, rhs);
  if(op == "max_of")            return create_expr_binary_operator<MaxOfVisitor>(key, lhs, rhs);
  if(op == "min_of")            return create_expr_binary_operator<MinOfVisitor>(key, lhs, rhs);
  if(op == "sorted_head")       return create_expr_binary_operator<SortedHeadVisitor>(key, lhs, rhs);
  if(op == "sum_values")        return create_expr_binary_operator<SumValuesVisitor>(key, lhs, rhs);

  // Logical operators
  if(op == "and")               return create_expr_binary_operator<AndVisitor>(key, lhs, rhs);
  if(op == "or")                return create_expr_binary_operator<OrVisitor>(key, lhs, rhs);
  if(op == "==")                return create_expr_binary_operator<EqVisitor>(key, lhs, rhs);
  if(op == "!=")                return create_expr_binary_operator<NeVisitor>(key, lhs, rhs);
  if(op == "<")                 return create_expr_binary_operator<LtVisitor>(key, lhs, rhs);
  if(op == "<=")                return create_expr_binary_operator<LeVisitor>(key, lhs, rhs);
  if(op == ">")                 return create_expr_binary_operator<GtVisitor>(key, lhs, rhs);
  if(op == ">=")                return create_expr_binary_operator<GeVisitor>(key, lhs, rhs);

  // String operators
  if(op == "literal_regex")     return create_expr_binary_operator<RegexVisitor>(key, lhs, rhs);
  if(op == "apply_format")      return create_expr_binary_operator<ApplyFormatVisitor>(key, lhs, rhs);
  if(op == "contains")          return create_expr_binary_operator<ContainsVisitor>(key, lhs, rhs);
  if(op == "starts_with")       return create_expr_binary_operator<StartsWithVisitor>(key, lhs, rhs);
  if(op == "substring_of")      return create_expr_binary_operator<SubstringOfVisitor>(key, lhs, rhs);
  if(op == "char_at")           return create_expr_binary_operator<CharAtVisitor>(key, lhs, rhs);
  if(op == "replace_char_of")   return create_expr_binary_operator<ReplaceCharOfVisitor>(key, lhs, rhs);

  // Resource operators
  if(op == "size_of")           return create_expr_binary_operator<SizeOfVisitor>(key, lhs, rhs);
  if(op == "exist")             return create_expr_binary_operator<ExistVisitor>(key, lhs, rhs);
  if(op == "exist_not")         return create_expr_binary_operator<ExistNotVisitor>(key, lhs, rhs);

  // Lookup operators (in expr_op_others.h)
  if(op == "lookup")            return create_expr_binary_operator<LookupVisitor>(key, lhs, rhs);
  if(op == "multi_lookup")      return create_expr_binary_operator<MultiLookupVisitor>(key, lhs, rhs);

  // Utility operators (in expr_op_others.h)
  if(op == "age_as_of")            return create_expr_binary_operator<AgeAsOfVisitor>(key, lhs, rhs);
  if(op == "age_in_months_as_of")  return create_expr_binary_operator<AgeInMonthsAsOfVisitor>(key, lhs, rhs);

  // //* TODO FIX ME to_type_of operator
  // // Cast operators (in expr_op_others.h)
  // if(op == "to_type_of")        return create_expr_binary_operator<ToTypeOfOperator>(key, lhs, rhs);
  // if(op == "cast_to")           return create_expr_binary_operator<ToTypeOfOperator>(key, lhs, rhs);
  
  LOG(ERROR) << "create_binary_expr: ERROR unknown binary operator: "<<
    op<<", called with key "<<key;
  RETE_EXCEPTION("create_binary_expr: ERROR unknown binary operator: "<<
    op<<", called with key "<<key);
}

ExprBasePtr
ReteMetaStoreFactory::create_unary_expr(int key, std::string const& op, ExprBasePtr arg)
{
  // UNARY OPERATORS
  // ------------------------------------------------------------------------------------
  // Arithmetic operators
  if(op == "abs")               return create_expr_unary_operator<AbsVisitor>(key, arg);
  if(op == "to_int")            return create_expr_unary_operator<ToIntVisitor>(key, arg);
  if(op == "to_double")         return create_expr_unary_operator<ToDoubleVisitor>(key, arg);
  
  // Date/Datetime operators
  if(op == "to_timestamp")      return create_expr_unary_operator<ToTimestampVisitor>(key, arg);
  if(op == "month_period_of")   return create_expr_unary_operator<MonthPeriodVisitor>(key, arg);
  if(op == "week_period_of")    return create_expr_unary_operator<WeekPeriodVisitor>(key, arg);
  if(op == "day_period_of")     return create_expr_unary_operator<DayPeriodVisitor>(key, arg);
  
  // Logical operators
  if(op == "not")               return create_expr_unary_operator<NotVisitor>(key, arg);

  // String operators
  if(op == "to_upper")           return create_expr_unary_operator<To_upperVisitor>(key, arg);
  if(op == "to_lower")           return create_expr_unary_operator<To_lowerVisitor>(key, arg);
  if(op == "trim")               return create_expr_unary_operator<TrimVisitor>(key, arg);
  if(op == "length_of")          return create_expr_unary_operator<LengthOfVisitor>(key, arg);
  if(op == "parse_usd_currency") return create_expr_unary_operator<ParseUsdCurrencyVisitor>(key, arg);
  if(op == "uuid_md5")           return create_expr_unary_operator<CreateNamedMd5UUIDVisitor>(key, arg);
  if(op == "uuid_sha1")          return create_expr_unary_operator<CreateNamedSha1UUIDVisitor>(key, arg);

  // Resource operators
  if(op == "create_entity")     return create_expr_unary_operator<CreateEntityVisitor>(key, arg);
  if(op == "create_literal")    return create_expr_unary_operator<CreateLiteralVisitor>(key, arg);
  if(op == "create_resource")   return create_expr_unary_operator<CreateResourceVisitor>(key, arg);
  if(op == "create_uuid_resource") return create_expr_unary_operator<CreateUUIDResourceVisitor>(key, arg);
  if(op == "is_literal")        return create_expr_unary_operator<IsLiteralVisitor>(key, arg);
  if(op == "is_resource")       return create_expr_unary_operator<IsResourceVisitor>(key, arg);
  if(op == "raise_exception")   return create_expr_unary_operator<RaiseExceptionVisitor>(key, arg);

  // Lookup operators (in expr_op_others.h)
  if(op == "lookup_rand")       return create_expr_unary_operator<LookupRandVisitor>(key, arg);
  if(op == "multi_lookup_rand") return create_expr_unary_operator<MultiLookupRandVisitor>(key, arg);

  LOG(ERROR) << "create_unary_expr: ERROR unknown unary operator: "<<
    op<<", called with key "<<key;
  RETE_EXCEPTION("create_unary_expr: ERROR unknown unary operator: "<<
    op<<", called with key "<<key);
}


} // namespace jets::rete
#endif // JETS_RETE_EXPR_OPERATOR_FACTORY_H
