#ifndef JETS_RETE_ALPHA_FUNCTORS_H
#define JETS_RETE_ALPHA_FUNCTORS_H

#include <iostream>
#include <memory>
#include <string>
#include <utility>

#include "jets/rdf/rdf_types.h"
#include "jets/rete/rete_err.h"
#include "jets/rete/node_vertex.h"
#include "jets/rete/beta_row.h"
#include "jets/rete/expr.h"
#include "jets/rete/rete_session.h"
#include "jets/rete/antecedent_query_visitor.h"

// This file contains the AlphaNode class parameters Fu, Fv, Fw classes
// Methods usage:
//  - to_const is used for determining the ReteCallBackImpl to use
//  - rdf::AllOrRIndex is used by antecedent terms to invoke find on the rdf_session
//  - to_r_index is used by consequent terms to evaluate  the functor
namespace jets::rete {
// F_binded
// --------------------------------------------------------------------------------------
struct F_binded {
  F_binded(int parent_pos): data(parent_pos){}

  F_binded(F_binded const&) = default;
  F_binded(F_binded &&) = default;
  F_binded & operator=(F_binded const&) = default;

  inline
  rdf::r_index 
  to_cst()const
  {
    return nullptr;
  }

  inline
  rdf::r_index
  to_r_index(ReteSession *, BetaRow const* parent_row)const
  {
    return parent_row->get(data);
  }

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

// F_var
// --------------------------------------------------------------------------------------
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

} // namespace jets::rete
#endif // JETS_RETE_ALPHA_FUNCTORS_H
