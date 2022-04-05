#ifndef JETS_RETE_EXPR_OP_OTHERS_H
#define JETS_RETE_EXPR_OP_OTHERS_H

#include <cctype>
#include <cstdint>
#include <type_traits>
#include <algorithm>
#include <string>
#include <memory>
#include <utility>
#include <regex>

#include <boost/numeric/conversion/cast.hpp>

#include "../rdf/rdf_types.h"
#include "../rete/rete_err.h"
#include "../rete/beta_row.h"
#include "../rete/rete_session.h"
#include "../rete/lookup_sql_helper.h"

// This file contains basic operator used in rule expression 
// see ExprUnaryOp and ExprBinaryOp classes.
namespace jets::rete {

using RDFTTYPE = rdf::RdfAstType;

// LookupVisitor
// --------------------------------------------------------------------------------------
struct LookupVisitor: public boost::static_visitor<RDFTTYPE>
{
  // This operator is used as: lookup_uri lookup key where lookup_uri is a resource and key is a text literal or a resource
  LookupVisitor(ReteSession * rs, BetaRow const* br, rdf::r_index lhs, rdf::r_index rhs): rs(rs), br(br), lhs_(lhs), rhs_(rhs) {}
  template<class T, class U> RDFTTYPE operator()(T lhs, U rhs) const {RETE_EXCEPTION("Invalid arguments for lookup: ("<<lhs<<", "<<rhs<<")");};
  RDFTTYPE operator()(rdf::NamedResource lhs, rdf::NamedResource rhs)const{return this->lookup(lhs.name, rhs.name);}
  RDFTTYPE operator()(rdf::NamedResource lhs, rdf::LInt32        rhs)const{return this->lookup(lhs.name, std::to_string(rhs.data));}
  RDFTTYPE operator()(rdf::NamedResource lhs, rdf::LUInt32       rhs)const{return this->lookup(lhs.name, std::to_string(rhs.data));}
  RDFTTYPE operator()(rdf::NamedResource lhs, rdf::LInt64        rhs)const{return this->lookup(lhs.name, std::to_string(rhs.data));}
  RDFTTYPE operator()(rdf::NamedResource lhs, rdf::LUInt64       rhs)const{return this->lookup(lhs.name, std::to_string(rhs.data));}
  RDFTTYPE operator()(rdf::NamedResource lhs, rdf::LString       rhs)const{return this->lookup(lhs.name, rhs.data);}

  RDFTTYPE operator()(rdf::LString       lhs, rdf::NamedResource rhs)const{return this->lookup(lhs.data, rhs.name);}
  RDFTTYPE operator()(rdf::LString       lhs, rdf::LInt32        rhs)const{return this->lookup(lhs.data, std::to_string(rhs.data));}
  RDFTTYPE operator()(rdf::LString       lhs, rdf::LUInt32       rhs)const{return this->lookup(lhs.data, std::to_string(rhs.data));}
  RDFTTYPE operator()(rdf::LString       lhs, rdf::LInt64        rhs)const{return this->lookup(lhs.data, std::to_string(rhs.data));}
  RDFTTYPE operator()(rdf::LString       lhs, rdf::LUInt64       rhs)const{return this->lookup(lhs.data, std::to_string(rhs.data));}
  RDFTTYPE operator()(rdf::LString       lhs, rdf::LString       rhs)const{return this->lookup(lhs.data, rhs.data);}
  
  RDFTTYPE lookup(std::string const& lookup_tbl, std::string const& key)const
  {
    RDFTTYPE out;
    auto helper = this->rs->rule_ms()->get_lookup_sql_helper();
    if(not helper) {
      RETE_EXCEPTION("Invalid lookup helper! Arguments: ("<<lookup_tbl<<", "<<key<<")");
    }
    if(helper->lookup(this->rs, lookup_tbl, key, &out)) {
      return {};
    }
    return out;
  }

  ReteSession * rs;
  BetaRow const* br;
  rdf::r_index lhs_;  // Note: This is the lhs as an r_index, may not exist in r_manager if this is
  rdf::r_index rhs_;  //       transitory resource
};

// MultiLookupVisitor
// --------------------------------------------------------------------------------------
struct MultiLookupVisitor: public boost::static_visitor<RDFTTYPE>
{
  // This operator is used as: lookup_uri lookup key where lookup_uri is a resource and key is a text literal or a resource
  MultiLookupVisitor(ReteSession * rs, BetaRow const* br, rdf::r_index lhs, rdf::r_index rhs): rs(rs), br(br), lhs_(lhs), rhs_(rhs) {}
  template<class T, class U> RDFTTYPE operator()(T lhs, U rhs) const {RETE_EXCEPTION("Invalid arguments for lookup: ("<<lhs<<", "<<rhs<<")");};
  RDFTTYPE operator()(rdf::NamedResource lhs, rdf::NamedResource rhs)const{return this->multi_lookup(lhs.name, rhs.name);}
  RDFTTYPE operator()(rdf::NamedResource lhs, rdf::LInt32        rhs)const{return this->multi_lookup(lhs.name, std::to_string(rhs.data));}
  RDFTTYPE operator()(rdf::NamedResource lhs, rdf::LUInt32       rhs)const{return this->multi_lookup(lhs.name, std::to_string(rhs.data));}
  RDFTTYPE operator()(rdf::NamedResource lhs, rdf::LInt64        rhs)const{return this->multi_lookup(lhs.name, std::to_string(rhs.data));}
  RDFTTYPE operator()(rdf::NamedResource lhs, rdf::LUInt64       rhs)const{return this->multi_lookup(lhs.name, std::to_string(rhs.data));}
  RDFTTYPE operator()(rdf::NamedResource lhs, rdf::LString       rhs)const{return this->multi_lookup(lhs.name, rhs.data);}

  RDFTTYPE operator()(rdf::LString       lhs, rdf::NamedResource rhs)const{return this->multi_lookup(lhs.data, rhs.name);}
  RDFTTYPE operator()(rdf::LString       lhs, rdf::LInt32        rhs)const{return this->multi_lookup(lhs.data, std::to_string(rhs.data));}
  RDFTTYPE operator()(rdf::LString       lhs, rdf::LUInt32       rhs)const{return this->multi_lookup(lhs.data, std::to_string(rhs.data));}
  RDFTTYPE operator()(rdf::LString       lhs, rdf::LInt64        rhs)const{return this->multi_lookup(lhs.data, std::to_string(rhs.data));}
  RDFTTYPE operator()(rdf::LString       lhs, rdf::LUInt64       rhs)const{return this->multi_lookup(lhs.data, std::to_string(rhs.data));}
  RDFTTYPE operator()(rdf::LString       lhs, rdf::LString       rhs)const{return this->multi_lookup(lhs.data, rhs.data);}
  
  RDFTTYPE multi_lookup(std::string const& lookup_tbl, std::string const& key)const
  {
    RDFTTYPE out;
    auto helper = this->rs->rule_ms()->get_lookup_sql_helper();
    if(not helper) {
      RETE_EXCEPTION("Invalid (multi)lookup helper! Arguments: ("<<lookup_tbl<<", "<<key<<")");
    }
    if(helper->multi_lookup(this->rs, lookup_tbl, key, &out)) {
      return {};
    }
    return out;
  }

  ReteSession * rs;
  BetaRow const* br;
  rdf::r_index lhs_;  // Note: This is the lhs as an r_index, may not exist in r_manager if this is
  rdf::r_index rhs_;  //       transitory resource
};

} // namespace jets::rete
#endif // JETS_RETE_EXPR_OP_OTHERS_H
