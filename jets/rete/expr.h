#ifndef JETS_RETE_EXPR_H
#define JETS_RETE_EXPR_H

#include <type_traits>
#include <string>
#include <memory>
#include <utility>
#include <vector>

#include "jets/rdf/rdf_types.h"
#include "jets/rete/node_vertex.h"

// This file contains the classes for rule expression.
// Abstract class hierarchy for representing an expression.
// Expression are used as:
//  - filter component of antecedent terms.
//  - object compoent of consequent terms.
// These classes are designed with consideration of expression evaluation speed and not
// building and manipulating the expression syntax tree.
// The expression parsing and transformation to it's final extression tree is done in python.
namespace jets::rete {
// //////////////////////////////////////////////////////////////////////////////////////
// ExprBase class -- Abstract base class for an expression tree
// --------------------------------------------------------------------------------------
class ReteSession;
class BetaRow;

class ExprBase;
using ExprBasePtr = std::shared_ptr<ExprBase>;

// ExprBase - Abstract base class for an expression tree
// The expression tree is performing operation on rdf::RdfAstType
// which is the rdf variant type
class ExprBase {
 public:
  using ExprDataType = rdf::RdfAstType;

  ExprBase() {}
  virtual ~ExprBase() {}

  virtual ExprDataType
  eval(ReteSession * rete_session, BetaRow const* beta_row)const=0;

  inline bool
  eval_filter(ReteSession * rete_session, BetaRow const* beta_row)const
  {
    auto result = eval(rete_session, beta_row);
    return rdf::to_bool(&result);
  }
};

// Implementation Classes
// ======================================================================================
// ExprConjunction
// --------------------------------------------------------------------------------------
class ExprConjunction: public ExprBase {
 public:
 using ExprBase::ExprDataType;
 using data_type = std::vector<ExprBasePtr>;

  // ExprConjunction(data_type v): ExprBase(), data_(std::move(v)) {}
  ExprConjunction(data_type const&v): ExprBase(), data_(v) {}
  ExprConjunction(data_type &&v): ExprBase(), data_(std::forward<data_type>(v)) {}
  virtual ~ExprConjunction() {}

  ExprDataType
  eval(ReteSession * rete_session, BetaRow const* beta_row)const override;

 private:
  
  data_type data_;
};
inline ExprBasePtr 
create_expr_conjunction(typename ExprConjunction::data_type v)
{
  return std::make_shared<ExprConjunction>(std::move(v));
}
inline ExprBasePtr 
create_expr_conjunction(typename ExprConjunction::data_type const&v)
{
  return std::make_shared<ExprConjunction>(v);
}
inline ExprBasePtr 
create_expr_conjunction(typename ExprConjunction::data_type &&v)
{
  return std::make_shared<ExprConjunction>(std::forward<typename ExprConjunction::data_type>(v));
}

// ExprDisjunction
// --------------------------------------------------------------------------------------
class ExprDisjunction: public ExprBase {
 public:
 using ExprBase::ExprDataType;
 using data_type = std::vector<ExprBasePtr>;

  ExprDisjunction(data_type const&v): ExprBase(), data_(v) {}
  ExprDisjunction(data_type &&v): ExprBase(), data_(std::forward<data_type>(v)) {}
  virtual ~ExprDisjunction() {}

  ExprDataType
  eval(ReteSession * rete_session, BetaRow const* beta_row)const override;

 private:
  data_type data_;
};
inline ExprBasePtr 
create_expr_disjunction(typename ExprDisjunction::data_type const&v)
{
  return std::make_shared<ExprDisjunction>(v);
}
inline ExprBasePtr 
create_expr_disjunction(typename ExprDisjunction::data_type &&v)
{
  return std::make_shared<ExprDisjunction>(std::forward<typename ExprDisjunction::data_type>(v));
}

// ExprCst
// --------------------------------------------------------------------------------------
class ExprCst: public ExprBase {
 public:
 using ExprBase::ExprDataType;
 using data_type = ExprDataType;

  ExprCst(data_type const&v): ExprBase(), data_(v) {}
  ExprCst(data_type &&v): ExprBase(), data_(std::forward<data_type>(v)) {}
  virtual ~ExprCst() {}

  ExprDataType
  eval(ReteSession * rete_session, BetaRow const* beta_row)const override;

 private:
  data_type data_;
};
inline ExprBasePtr 
create_expr_cst(typename ExprCst::data_type const&v)
{
  return std::make_shared<ExprCst>(v);
}
inline ExprBasePtr 
create_expr_cst(typename ExprCst::data_type &&v)
{
  return std::make_shared<ExprCst>(std::forward<typename ExprCst::data_type>(v));
}

// ExprBindedVar
// --------------------------------------------------------------------------------------
class ExprBindedVar: public ExprBase {
 public:
 using ExprBase::ExprDataType;
 using data_type = int;

  ExprBindedVar(data_type v): ExprBase(), data_(v) {}
  virtual ~ExprBindedVar() {}

  ExprDataType
  eval(ReteSession * rete_session, BetaRow const* beta_row)const override;

 private:
  data_type data_;
};
inline ExprBasePtr 
create_expr_binded_var(typename ExprBindedVar::data_type v)
{
  return std::make_shared<ExprBindedVar>(v);
}

// ExprBinaryOp
// --------------------------------------------------------------------------------------
template<class Op>
class ExprBinaryOp: public ExprBase {
 public:
 using ExprBase::ExprDataType;

  ExprBinaryOp(std::shared_ptr<Op> oper, ExprBasePtr lhs, ExprBasePtr rhs)
    : ExprBase(), oper_(oper), lhs_(lhs), rhs_(rhs) {}
  virtual ~ExprBinaryOp() {}

  ExprDataType
  eval(ReteSession * rete_session, BetaRow const* beta_row)const override;

 private:
  std::shared_ptr<Op> oper_;
  ExprBasePtr lhs_;
  ExprBasePtr rhs_;
};
template<class Op>
ExprBasePtr 
create_expr_binary_operator(std::shared_ptr<Op> oper, ExprBasePtr lhs, ExprBasePtr rhs)
{
  return std::make_shared<ExprBinaryOp<Op>>(oper, lhs, rhs);
}

// ExprUnaryOp
// --------------------------------------------------------------------------------------
template<class Op>
class ExprUnaryOp: public ExprBase {
 public:
 using ExprBase::ExprDataType;

  ExprUnaryOp(std::shared_ptr<Op> oper, ExprBasePtr arg)
    : ExprBase(), oper_(oper), arg_(arg) {}
  virtual ~ExprUnaryOp() {}

  ExprDataType
  eval(ReteSession * rete_session, BetaRow const* beta_row)const override;

 private:
  std::shared_ptr<Op> oper_;
  ExprBasePtr arg_;
};
template<class Op>
ExprBasePtr create_expr_unary_operator(std::shared_ptr<Op> oper, ExprBasePtr arg)
{
  return std::make_shared<ExprUnaryOp<Op>>(oper, arg);
}

} // namespace jets::rete
#endif // JETS_RETE_EXPR_H
