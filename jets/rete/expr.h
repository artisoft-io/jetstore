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
template<class T>
class ReteSession;
class BetaRow;

template<class T>
class ExprBase;
template<class T>
using ExprBasePtr = std::shared_ptr<ExprBase<T>>;

// ExprBase - Abstract base class for an expression tree
// The expression tree is performing operation on rdf::RdfAstType
// which is the rdf variant type
template<class T>
class ExprBase {
 public:
  using RDFSession = T;
  using ExprDataType = rdf::RdfAstType;

  ExprBase() {}
  virtual ~ExprBase() {}

  virtual ExprDataType
  eval(ReteSession<T> * rete_session, BetaRow const* beta_row)const=0;

  inline bool
  eval_filter(ReteSession<T> * rete_session, BetaRow const* beta_row)const
  {
    auto result = eval(rete_session, beta_row);
    return rdf::to_bool(&result);
  }
};

// Implementation Classes
// ======================================================================================
// ExprConjunction
// --------------------------------------------------------------------------------------
template<class T>
class ExprConjunction: public ExprBase<T> {
 public:
 using typename ExprBase<T>::ExprDataType;
 using data_type = std::vector<ExprBasePtr<T>>;

  // ExprConjunction(data_type v): ExprBase<T>(), data_(std::move(v)) {}
  ExprConjunction(data_type const&v): ExprBase<T>(), data_(v) {}
  ExprConjunction(data_type &&v): ExprBase<T>(), data_(std::forward<data_type>(v)) {}
  virtual ~ExprConjunction() {}

  ExprDataType
  eval(ReteSession<T> * rete_session, BetaRow const* beta_row)const override;

 private:
  
  data_type data_;
};
template<class T>
ExprBasePtr<T> create_expr_conjunction(typename ExprConjunction<T>::data_type v)
{
  return std::make_shared<ExprConjunction<T>>(std::move(v));
}
template<class T>
ExprBasePtr<T> create_expr_conjunction(typename ExprConjunction<T>::data_type const&v)
{
  return std::make_shared<ExprConjunction<T>>(v);
}
template<class T>
ExprBasePtr<T> create_expr_conjunction(typename ExprConjunction<T>::data_type &&v)
{
  return std::make_shared<ExprConjunction<T>>(std::forward<typename ExprConjunction<T>::data_type>(v));
}

// ExprDisjunction
// --------------------------------------------------------------------------------------
template<class T>
class ExprDisjunction: public ExprBase<T> {
 public:
 using typename ExprBase<T>::ExprDataType;
 using data_type = std::vector<ExprBasePtr<T>>;

  ExprDisjunction(data_type v): ExprBase<T>(), data_(std::move(v)) {}
  ExprDisjunction(data_type const&v): ExprBase<T>(), data_(v) {}
  ExprDisjunction(data_type &&v): ExprBase<T>(), data_(std::forward<data_type>(v)) {}
  virtual ~ExprDisjunction() {}

  ExprDataType
  eval(ReteSession<T> * rete_session, BetaRow const* beta_row)const override;

 private:
  data_type data_;
};
template<class T>
ExprBasePtr<T> create_expr_disjunction(typename ExprDisjunction<T>::data_type v)
{
  return std::make_shared<ExprDisjunction<T>>(std::move(v));
}
template<class T>
ExprBasePtr<T> create_expr_disjunction(typename ExprDisjunction<T>::data_type const&v)
{
  return std::make_shared<ExprDisjunction<T>>(v);
}
template<class T>
ExprBasePtr<T> create_expr_disjunction(typename ExprDisjunction<T>::data_type &&v)
{
  return std::make_shared<ExprDisjunction<T>>(std::forward<typename ExprDisjunction<T>::data_type>(v));
}

// ExprCst
// --------------------------------------------------------------------------------------
template<class T>
class ExprCst: public ExprBase<T> {
 public:
 using typename ExprBase<T>::ExprDataType;
 using data_type = ExprDataType;

  ExprCst(data_type v): ExprBase<T>(), data_(std::move(v)) {}
  ExprCst(data_type const&v): ExprBase<T>(), data_(v) {}
  ExprCst(data_type &&v): ExprBase<T>(), data_(std::forward<data_type>(v)) {}
  virtual ~ExprCst() {}

  ExprDataType
  eval(ReteSession<T> * rete_session, BetaRow const* beta_row)const override;

 private:
  data_type data_;
};
template<class T>
ExprBasePtr<T> create_expr_cst(typename ExprCst<T>::data_type v)
{
  return std::make_shared<ExprCst<T>>(std::move(v));
}
template<class T>
ExprBasePtr<T> create_expr_cst(typename ExprCst<T>::data_type const&v)
{
  return std::make_shared<ExprCst<T>>(v);
}
template<class T>
ExprBasePtr<T> create_expr_cst(typename ExprCst<T>::data_type &&v)
{
  return std::make_shared<ExprCst<T>>(std::forward<typename ExprCst<T>::data_type>(v));
}

// ExprBindedVar
// --------------------------------------------------------------------------------------
template<class T>
class ExprBindedVar: public ExprBase<T> {
 public:
 using typename ExprBase<T>::ExprDataType;
 using data_type = int;

  ExprBindedVar(data_type v): ExprBase<T>(), data_(v) {}
  virtual ~ExprBindedVar() {}

  ExprDataType
  eval(ReteSession<T> * rete_session, BetaRow const* beta_row)const override;

 private:
  data_type data_;
};
template<class T>
ExprBasePtr<T> create_expr_binded_var(typename ExprBindedVar<T>::data_type v)
{
  return std::make_shared<ExprBindedVar<T>>(v);
}

// ExprBinaryOp
// --------------------------------------------------------------------------------------
template<class T, class Op>
class ExprBinaryOp: public ExprBase<T> {
 public:
 using typename ExprBase<T>::ExprDataType;

  ExprBinaryOp(std::shared_ptr<Op> oper, ExprBasePtr<T> lhs, ExprBasePtr<T> rhs)
    : ExprBase<T>(), oper_(oper), lhs_(lhs), rhs_(rhs) {}
  virtual ~ExprBinaryOp() {}

  ExprDataType
  eval(ReteSession<T> * rete_session, BetaRow const* beta_row)const override;

 private:
  std::shared_ptr<Op> oper_;
  ExprBasePtr<T> lhs_;
  ExprBasePtr<T> rhs_;
};
template<class T, class Op>
ExprBasePtr<T> create_expr_binary_operator(std::shared_ptr<Op> oper, ExprBasePtr<T> lhs, ExprBasePtr<T> rhs)
{
  return std::make_shared<ExprBinaryOp<T, Op>>(oper, lhs, rhs);
}

// ExprUnaryOp
// --------------------------------------------------------------------------------------
template<class T, class Op>
class ExprUnaryOp: public ExprBase<T> {
 public:
 using typename ExprBase<T>::ExprDataType;

  ExprUnaryOp(std::shared_ptr<Op> oper, ExprBasePtr<T> arg)
    : ExprBase<T>(), oper_(oper), arg_(arg) {}
  virtual ~ExprUnaryOp() {}

  ExprDataType
  eval(ReteSession<T> * rete_session, BetaRow const* beta_row)const override;

 private:
  std::shared_ptr<Op> oper_;
  ExprBasePtr<T> arg_;
};
template<class T, class Op>
ExprBasePtr<T> create_expr_unary_operator(std::shared_ptr<Op> oper, ExprBasePtr<T> arg)
{
  return std::make_shared<ExprUnaryOp<T, Op>>(oper, arg);
}

} // namespace jets::rete
#endif // JETS_RETE_EXPR_H
