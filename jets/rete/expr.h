#ifndef JETS_RETE_EXPR_H
#define JETS_RETE_EXPR_H

#include <type_traits>
#include <string>
#include <memory>
#include <utility>
#include <vector>

#include <glog/logging.h>
#include "boost/variant.hpp"
#include <boost/variant/detail/apply_visitor_unary.hpp>

#include "../rdf/rdf_types.h"
#include "../rete/node_vertex.h"
#include "../rete/beta_row.h"

// This file contains the classes for rule expression.
// Abstract class hierarchy for representing an expression.
// Expression are used as:
//  - filter component of antecedent terms.
//  - object compoent of consequent terms.
// These classes are designed with consideration of expression evaluation speed and not
// building and manipulating the expression syntax tree.
// The expression parsing and transformation to it's final extression tree is done in the rule compiler.
namespace jets::rete {
// //////////////////////////////////////////////////////////////////////////////////////
// ExprBase class -- Abstract base class for an expression tree
// --------------------------------------------------------------------------------------
// Definition of ExprBasePtr is in node_vertex.h
class ReteSession;

// Utility function for operators that need to get a graph rdf::r_index
inline 
std::pair<rdf::r_index, rdf::r_index>
get_resources(rdf::RManager *rmgr, std::string && lhs, std::string && rhs)
{
  auto * l = rmgr->get_resource(std::forward<std::string>(lhs));
  auto * r = rmgr->get_resource(std::forward<std::string>(rhs));
  return {l, r};
}

// ExprBase - Abstract base class for an expression tree
// The expression tree is performing operation on rdf::RdfAstType
// which is the rdf variant type
class ExprBase {
 public:
  using ExprDataType = rdf::RdfAstType;

  ExprBase(): key(-1) {}
  explicit ExprBase(int key): key(key) {}
  virtual ~ExprBase() {}

  /**
   * @brief Register callback with graph
   * 
   * Applicable to operator having predicates as argument
   * that need to participate to the truth maintenance
   * e.g. operators exists and exists_not
   * 
   * Only some binary operator do participate to the truth maintenance
   * 
   * @param rete_session 
   * @param vertex of the rule term (node_vertex) where the filter is attached to
   * @return int 
   */
  virtual int
  register_callback(ReteSession * rete_session, int vertex)const
  {
    return 0;
  }

  virtual ExprDataType
  eval(ReteSession * rete_session, BetaRow const* beta_row)const=0;

  /**
   * @brief True when evaluating the expression twice on the same beta row is
   * guaranteed to give the same value.
   *
   * False when an operator in the tree mints a resource (create_entity,
   * create_uuid_resource, range), reads the graph (exist, size_of, the aggregates)
   * or reads a lookup table. A consequent carrying such an expression records the
   * triples it inferred on its beta row, and retraction replays that record instead
   * of evaluating the expression again (see ReteSession::compute_consequent_triples).
   */
  virtual bool
  is_row_pure()const
  {
    return true;
  }

  inline bool
  eval_filter(ReteSession * rete_session, BetaRow const* beta_row)const
  {
    auto result = eval(rete_session, beta_row);
    return rdf::to_bool(&result);
  }
  virtual std::ostream & describe(std::ostream & out)const=0;

  int key;
};

inline std::ostream & operator<<(std::ostream & out, ExprBasePtr node)
{
  if(not node) out << "NULL";
  else {
    node->describe(out);
  }
  return out;
}

// Marker base for an operator visitor whose result is NOT a pure function of its
// arguments: it mints a resource, reads the graph or reads a lookup table. An
// ExprBinaryOp or ExprUnaryOp over such a visitor reports is_row_pure() false.
struct NotRowPure {};

template<class Op>
inline constexpr bool op_is_row_pure = not std::is_base_of_v<NotRowPure, Op>;

// Utility class for operator visitor that don't need to register callbacks
// for truth maintenance
struct NoCallbackNeeded
{
  int
  register_callback(int vertex, ExprBase::ExprDataType && lhs, ExprBase::ExprDataType && rhs)const
  {
    return 0;
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
  explicit ExprConjunction(data_type const&v): ExprBase(), data_(v) {}
  explicit ExprConjunction(data_type &&v): ExprBase(), data_(std::forward<data_type>(v)) {}
  virtual ~ExprConjunction() {}

  int
  register_callback(ReteSession * rete_session, int vertex)const override
  {
    for(auto const& item: this->data_) {
      item->register_callback(rete_session, vertex);
    }
    return 0;
  }

  bool
  is_row_pure()const override
  {
    for(auto const& item: this->data_) {
      if(not item->is_row_pure()) return false;
    }
    return true;
  }

  ExprDataType
  eval(ReteSession * rete_session, BetaRow const* beta_row)const override;

  std::ostream & 
  describe(std::ostream & out)const override
  {
    out << "conjunction("<< this->key << ")";
    return out;
  }

 private:
  
  data_type data_;
};
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

  explicit ExprDisjunction(data_type const&v): ExprBase(), data_(v) {}
  explicit ExprDisjunction(data_type &&v): ExprBase(), data_(std::forward<data_type>(v)) {}
  virtual ~ExprDisjunction() {}

  int
  register_callback(ReteSession * rete_session, int vertex)const override
  {
    for(auto const& item: this->data_) {
      item->register_callback(rete_session, vertex);
    }
    return 0;
  }

  bool
  is_row_pure()const override
  {
    for(auto const& item: this->data_) {
      if(not item->is_row_pure()) return false;
    }
    return true;
  }

  // defined in expr_impl.h
  ExprDataType
  eval(ReteSession * rete_session, BetaRow const* beta_row)const override;

  std::ostream & 
  describe(std::ostream & out)const override
  {
    out << "disjunction("<< this->key << ")";
    return out;
  }

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

  explicit ExprCst(data_type const&v): ExprBase(), data_(v) {}
  explicit ExprCst(data_type &&v): ExprBase(), data_(std::forward<data_type>(v)) {}
  virtual ~ExprCst() {}

  // defined in expr_impl.h
  ExprDataType
  eval(ReteSession * rete_session, BetaRow const* beta_row)const override;

  std::ostream & 
  describe(std::ostream & out)const override
  {
    out << "cst("<< this->data_ << ")";
    return out;
  }

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
/**
 * @brief Expr with a F_binded functor, binded to BetaRow value
 * 
 */
class ExprBindedVar: public ExprBase {
 public:
 using ExprBase::ExprDataType;
 using data_type = int;

  explicit ExprBindedVar(data_type v): ExprBase(), data_(v) {}
  virtual ~ExprBindedVar() {}

  // defined in expr_impl.h
  ExprDataType
  eval(ReteSession * rete_session, BetaRow const* beta_row)const override;

  std::ostream & 
  describe(std::ostream & out)const override
  {
    out << "binded("<< this->data_ << ")";
    return out;
  }

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
/**
 * @brief Binary operator
 * 
 * @tparam Op is a visitor pattern for the binary operator
 */
template<class Op>
class ExprBinaryOp: public ExprBase {
 public:
 using ExprBase::ExprDataType;

  ExprBinaryOp(int key, ExprBasePtr lhs, ExprBasePtr rhs)
    : ExprBase(key), lhs_(lhs), rhs_(rhs) {}
  virtual ~ExprBinaryOp() {}

  int
  register_callback(ReteSession * rete_session, int vertex)const override
  {
    auto err = this->lhs_->register_callback(rete_session, vertex);
    if(err) return err;
    err = this->rhs_->register_callback(rete_session, vertex);
    if(err) return err;
    return Op(rete_session, nullptr).register_callback(vertex, this->lhs_->eval(rete_session, nullptr), 
      this->rhs_->eval(rete_session, nullptr));
  }

  bool
  is_row_pure()const override
  {
    return op_is_row_pure<Op> and this->lhs_->is_row_pure() and this->rhs_->is_row_pure();
  }

  // defined in expr_impl.h
  ExprDataType
  eval(ReteSession * rete_session, BetaRow const* beta_row)const override;

  std::ostream & 
  describe(std::ostream & out)const override
  {
    out << "binary("<< this->key << ")";
    return out;
  }

 private:
  ExprBasePtr lhs_;
  ExprBasePtr rhs_;
};
template<class Op>
ExprBasePtr 
create_expr_binary_operator(int key, ExprBasePtr lhs, ExprBasePtr rhs)
{
  return std::make_shared<ExprBinaryOp<Op>>(key, lhs, rhs);
}

// ExprUnaryOp
// --------------------------------------------------------------------------------------
/**
 * @brief Unary operator
 * 
 * @tparam Op is a visitor pattern for the unary operator
 */
template<class Op>
class ExprUnaryOp: public ExprBase {
 public:
 using ExprBase::ExprDataType;

  ExprUnaryOp(int key, ExprBasePtr arg)
    : ExprBase(key), arg_(arg) {}
  virtual ~ExprUnaryOp() {}

  bool
  is_row_pure()const override
  {
    return op_is_row_pure<Op> and this->arg_->is_row_pure();
  }

  // defined in expr_impl.h
  ExprDataType
  eval(ReteSession * rete_session, BetaRow const* beta_row)const override;

  std::ostream & 
  describe(std::ostream & out)const override
  {
    out << "unary("<< this->key << ")";
    return out;
  }

 private:
  ExprBasePtr arg_;
};
template<class Op>
ExprBasePtr create_expr_unary_operator(int key, ExprBasePtr arg)
{
  return std::make_shared<ExprUnaryOp<Op>>(key, arg);
}

} // namespace jets::rete
#endif // JETS_RETE_EXPR_H
