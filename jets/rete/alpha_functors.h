#ifndef JETS_RETE_ALPHA_FUNCTORS_H
#define JETS_RETE_ALPHA_FUNCTORS_H

#include <iostream>
#include <memory>
#include <string>
#include <utility>

#include "jets/rdf/rdf_types.h"
#include "node_vertex.h"
#include "jets/rete/beta_row.h"
#include "jets/rete/expr.h"
#include "jets/rete/rete_session.h"

// This file contains the AlphaNode class parameters Fu, Fv, Fw classes
namespace jets::rete {
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
  eval(BetaRow const* parent_row)const
  {
    return parent_row->get(data);
  }

  int data;
};

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
  eval(BetaRow const* parent_row)const
  {
    return nullptr;
  }

  std::string data;
};

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
  eval(BetaRow const* parent_row)const
  {
    return data;
  }

  rdf::r_index data;
};

struct F_expr {
  F_expr(ReteSession * rete_session, ExprBasePtr expr)
    : rete_session(rete_session), data(expr){}

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
  eval(BetaRow const* parent_row)
  {
    auto * rmgr = rete_session->rdf_session()->rmgr();
    auto rv = data->eval(rete_session, parent_row);
    return rmgr->insert_item( std::make_shared<rdf::RdfAstType>(rv) );
  }

  ReteSession * rete_session;
  ExprBasePtr data;
};

} // namespace jets::rete
#endif // JETS_RETE_ALPHA_FUNCTORS_H
