#ifndef JETS_RETE_ALPHA_FUNCTORS_H
#define JETS_RETE_ALPHA_FUNCTORS_H

#include <iostream>
#include <memory>
#include <string>
#include <utility>

#include "../rdf/rdf_types.h"
#include "../rete/rete_err.h"
#include "../rete/node_vertex.h"
#include "../rete/beta_row.h"
#include "../rete/expr.h"
#include "../rete/rete_session.h"
#include "../rete/antecedent_query_visitor.h"

// This file contains the AlphaNode class parameters Fu, Fv, Fw classes
// Methods usage:
//  - to_const is used ReteCallBackImpl to determine the rdf graph filter
//  - rdf::AllOrRIndex is used by antecedent terms to invoke find on the rdf_session
//  - to_r_index is used by consequent terms to evaluate  the functor using the beta row
//    of the current antecedent (vertex)
namespace jets::rete {
// F_binded
// --------------------------------------------------------------------------------------
/**
 * @brief Functor for binded variable
 * The data member 'data' indicate the position of the beta row elm it is binded to.
 *  - Case of antecedent terms, data is the position of the parent beta row
 *  - Case of consequent terms and filters, data is the position of the 
 *    associated antecedent (vertex), not it's parent as for antecedent term
 */
struct F_binded {
  F_binded(int var_pos): data(var_pos){}

  F_binded(F_binded const&) = default;
  F_binded(F_binded &&) = default;
  F_binded & operator=(F_binded const&) = default;

  inline
  rdf::r_index 
  to_cst()const
  {
    return nullptr;
  }

  /**
   * @brief Evaluate functor for consequent and filter terms
   * 
   * @param beta_row of the associated antecedent term (aka current row)
   * @return rdf::r_index 
   */
  inline
  rdf::r_index
  to_r_index(ReteSession *, BetaRow const* beta_row)const
  {
    return beta_row->get(data);
  }

  /**
   * @brief Evaluate functor for antecedent term
   * 
   * @param parent_row beta row of parent antecedent
   * @return rdf::AllOrRIndex 
   */
  inline
  rdf::AllOrRIndex
  to_AllOrRIndex(BetaRow const* parent_row)const
  {
    return {parent_row->get(data)};
  }

  inline
  AQV
  to_AQV()const
  {
    return {AQIndex(data)};
  }

  int data;
};

inline std::ostream & operator<<(std::ostream & out, F_binded const& node)
{
  out << "binded("<<node.data<<")";
  return out;
}

// F_var
// --------------------------------------------------------------------------------------
/**
 * @brief Functor for unbinded var, applicable to antecedent only
 * 
 */
struct F_var {
  F_var(std::string const& var_name): data(var_name){}
  F_var(std::string && var_name)
    : data(std::forward<std::string>(var_name)){}

  F_var(F_var const&) = default;
  F_var(F_var &&) = default;
  F_var & operator=(F_var const&) = default;

  inline
  rdf::r_index 
  to_cst()const
  {
    return nullptr;
  }

  inline
  rdf::r_index
  to_r_index(ReteSession *, BetaRow const*)const
  {
    return nullptr;
  }

  inline
  rdf::AllOrRIndex
  to_AllOrRIndex(BetaRow const* parent_row)const
  {
    return {rdf::StarMatch()};
  }

  inline
  AQV
  to_AQV()const
  {
    return {AQOther()};
  }

  std::string data;
};

inline std::ostream & operator<<(std::ostream & out, F_var const& node)
{
  out << "var("<<node.data<<")";
  return out;
}

// F_cst
// --------------------------------------------------------------------------------------
struct F_cst {
  F_cst(rdf::r_index r): data(r){}

  F_cst(F_cst const&) = default;
  F_cst(F_cst &&) = default;
  F_cst & operator=(F_cst const&) = default;

  inline
  rdf::r_index 
  to_cst()const
  {
    return data;
  }

  inline
  rdf::r_index
  to_r_index(ReteSession *, BetaRow const*)const
  {
    return data;
  }

  inline
  rdf::AllOrRIndex
  to_AllOrRIndex(BetaRow const* parent_row)const
  {
    return {data};
  }

  inline
  AQV
  to_AQV()const
  {
    return {AQOther()};
  }

  rdf::r_index data;
};

inline std::ostream & operator<<(std::ostream & out, F_cst const& node)
{
  out << "cst("<<node.data<<")";
  return out;
}

// F_expr
// --------------------------------------------------------------------------------------
struct F_expr {
  explicit F_expr(ExprBasePtr expr) : data(expr){}

  F_expr(F_expr const&) = default;
  F_expr(F_expr &&) = default;
  F_expr & operator=(F_expr const&) = default;

  inline
  rdf::r_index 
  to_cst()const
  {
    return nullptr;
  }

  inline
  rdf::r_index
  to_r_index(ReteSession * rete_session, BetaRow const* parent_row)const
  {
    auto * rmgr = rete_session->rdf_session()->rmgr();
    auto rv = data->eval(rete_session, parent_row);
    return rmgr->insert_item( std::make_shared<rdf::RdfAstType>(rv) );
  }

  inline
  rdf::AllOrRIndex
  to_AllOrRIndex(BetaRow const* parent_row)const
  {
    RETE_EXCEPTION("Error: F_exp::to_AllOrRIndex should never be called as this functor"
      " cannot be used as an antecedent term");
  }

  inline
  AQV
  to_AQV()const
  {
    return {AQOther()};
  }

  ExprBasePtr data;
};

inline std::ostream & operator<<(std::ostream & out, F_expr const& node)
{
  out << "expr("<<node.data<<")";
  return out;
}

} // namespace jets::rete
#endif // JETS_RETE_ALPHA_FUNCTORS_H
