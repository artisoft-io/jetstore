#ifndef JETS_RETE_EXPR_OP_ARITHMETICS_H
#define JETS_RETE_EXPR_OP_ARITHMETICS_H

#include <cctype>
#include <cstdint>
#include <limits>
#include <type_traits>
#include <algorithm>
#include <string>
#include <memory>
#include <utility>
#include <regex>
#include <sstream>
#include <vector>

#include <boost/numeric/conversion/cast.hpp>

#include "../rdf/rdf_types.h"
#include "../rete/rete_err.h"
#include "../rete/beta_row.h"
#include "../rete/rete_session.h"
#include "../rete/expr_op_logics.h"

// This file contains basic operator used in rule expression 
// see ExprUnaryOp and ExprBinaryOp classes.
namespace jets::rete {

inline 
rdf::r_index
get_resource(ReteSession * rs, rdf::RdfAstType && r)
{
  if( r.which() != rdf::rdf_named_resource_t) return nullptr;
  return rs->rdf_session()->rmgr()->get_resource(
    std::move<std::string>(boost::get<rdf::NamedResource>(std::forward<rdf::RdfAstType>(r)).name));
}

using RDFTTYPE = rdf::RdfAstType;

// Truth maintenance for the aggregate operators: min_of, max_of, sorted_head,
// sum_values and join_values.
// --------------------------------------------------------------------------------------
// All five fold a set of triples into one value, and all five take an rhs that is either
// an operator config carrying jets:entity_property and/or jets:value_property, or the
// multi-valued data property itself. This registers the graph callbacks that make the
// engine replay the rule term when that set changes.
//
// AN AGGREGATE OVER (s, entityP, ?o).(?o, valueP, ?v) DEPENDS ON BOTH PROPERTIES, and
// until 2026-09-09 each of these operators watched jets:value_property alone. Watching
// the value property is not enough, and the case it misses is the ordinary one: a child
// entity is materialised with its own value and THEN attached to the subject, so at the
// moment the (s, entityP, ?o) triple lands there is no value triple to notice, and the
// aggregate silently keeps the figure it had when the rule term was first evaluated.
// Only a change to an ALREADY LINKED child was seen. Measured against the C++ engine by
// AggregateTruthMaintenanceTest in expr_op_specialty_test.cc.
//
// `fallback_to_rhs` is for the operators that also accept the rhs as a multi-valued data
// property directly -- (?s sum_values someMultiValuedProperty). sorted_head passes false
// because its rhs must be a config: it needs jets:operator as well.
inline int
register_callbacks_for_aggregate(ReteSession * rs, int vertex, rdf::r_index rhs,
  bool fallback_to_rhs)
{
  if(not rs or not rhs) {
    return 0;
  }
  auto * rdf_session_p = rs->rdf_session();
  auto const* jr = rdf_session_p->rmgr()->jets();

  auto register_on = [rs, vertex, rdf_session_p](rdf::r_index p_filter) {
    if(not p_filter) return;
    auto cb = create_rete_callback_for_visitors(rs, vertex, p_filter);
    rdf_session_p->asserted_graph()->register_callback(cb);
    rdf_session_p->inferred_graph()->register_callback(cb);
  };

  auto value_p  = rdf_session_p->get_object(rhs, jr->jets__value_property);
  auto entity_p = rdf_session_p->get_object(rhs, jr->jets__entity_property);
  if(not value_p and not entity_p) {
    // The rhs is not a config. For the operators that accept it, it is itself the
    // multi-valued data property to watch.
    if(fallback_to_rhs) register_on(rhs);
    return 0;
  }
  // Either or both may be absent, and each valid config form is covered by registering
  // on whichever are present:
  //  - entity + value: the aggregate walks both, so watch both.
  //  - value alone:    the aggregate is over (s, valueP, ?v).
  //  - entity alone:   min_of/max_of over the objects themselves, no value involved.
  register_on(value_p);
  register_on(entity_p);
  return 0;
}

// AddVisitor
// --------------------------------------------------------------------------------------
// + - * / on numbers: a double operand, on either side, makes the result a double. Until
// 2026-10 an integer met first truncated the double in + and - (2 + 0.5 was 2); * and /
// already gave a double. Integers alone keep the left operand's type, the right converted
// by numeric_cast, and / is integer division. The Go engine follows the same table.
struct AddVisitor: public boost::static_visitor<RDFTTYPE>, public NoCallbackNeeded
{
  AddVisitor(ReteSession * rs, BetaRow const* br): rs(rs), br(br) {}
  AddVisitor(): rs(nullptr), br(nullptr) {}
  template<class T, class U> RDFTTYPE operator()(T lhs, U rhs) const {if(br==nullptr) return rdf::Null(); else RETE_EXCEPTION("Invalid arguments for addition: "<<lhs<<"("<<typeid(lhs).name()<<"), "<<rhs<<"("<<typeid(rhs).name()<<")");};

  RDFTTYPE operator()(rdf::LInt32  lhs, rdf::LInt32  rhs)const{return rdf::LInt32{lhs.data+rhs.data};}
  RDFTTYPE operator()(rdf::LInt32  lhs, rdf::LUInt32 rhs)const{return rdf::LInt32{lhs.data+boost::numeric_cast<int32_t>(rhs.data)};}
  RDFTTYPE operator()(rdf::LInt32  lhs, rdf::LInt64  rhs)const{return rdf::LInt32{lhs.data+boost::numeric_cast<int32_t>(rhs.data)};}
  RDFTTYPE operator()(rdf::LInt32  lhs, rdf::LUInt64 rhs)const{return rdf::LInt32{lhs.data+boost::numeric_cast<int32_t>(rhs.data)};}
  RDFTTYPE operator()(rdf::LInt32  lhs, rdf::LDouble rhs)const{return rdf::LDouble{boost::numeric_cast<double>(lhs.data)+rhs.data};}

  RDFTTYPE operator()(rdf::LUInt32 lhs, rdf::LInt32  rhs)const{return rdf::LUInt32{lhs.data+boost::numeric_cast<uint32_t>(rhs.data)};}
  RDFTTYPE operator()(rdf::LUInt32 lhs, rdf::LUInt32 rhs)const{return rdf::LUInt32{lhs.data+rhs.data};}
  RDFTTYPE operator()(rdf::LUInt32 lhs, rdf::LInt64  rhs)const{return rdf::LUInt32{lhs.data+boost::numeric_cast<uint32_t>(rhs.data)};}
  RDFTTYPE operator()(rdf::LUInt32 lhs, rdf::LUInt64 rhs)const{return rdf::LUInt32{lhs.data+boost::numeric_cast<uint32_t>(rhs.data)};}
  RDFTTYPE operator()(rdf::LUInt32 lhs, rdf::LDouble rhs)const{return rdf::LDouble{boost::numeric_cast<double>(lhs.data)+rhs.data};}

  RDFTTYPE operator()(rdf::LInt64  lhs, rdf::LInt32  rhs)const{return rdf::LInt64{lhs.data+boost::numeric_cast<int64_t>(rhs.data)};}
  RDFTTYPE operator()(rdf::LInt64  lhs, rdf::LUInt32 rhs)const{return rdf::LInt64{lhs.data+boost::numeric_cast<int64_t>(rhs.data)};}
  RDFTTYPE operator()(rdf::LInt64  lhs, rdf::LInt64  rhs)const{return rdf::LInt64{lhs.data+rhs.data};}
  RDFTTYPE operator()(rdf::LInt64  lhs, rdf::LUInt64 rhs)const{return rdf::LInt64{lhs.data+boost::numeric_cast<int64_t>(rhs.data)};}
  RDFTTYPE operator()(rdf::LInt64  lhs, rdf::LDouble rhs)const{return rdf::LDouble{boost::numeric_cast<double>(lhs.data)+rhs.data};}

  RDFTTYPE operator()(rdf::LUInt64 lhs, rdf::LInt32  rhs)const{return rdf::LUInt64{lhs.data+boost::numeric_cast<uint64_t>(rhs.data)};}
  RDFTTYPE operator()(rdf::LUInt64 lhs, rdf::LUInt32 rhs)const{return rdf::LUInt64{lhs.data+boost::numeric_cast<uint64_t>(rhs.data)};}
  RDFTTYPE operator()(rdf::LUInt64 lhs, rdf::LInt64  rhs)const{return rdf::LUInt64{lhs.data+boost::numeric_cast<uint64_t>(rhs.data)};}
  RDFTTYPE operator()(rdf::LUInt64 lhs, rdf::LUInt64 rhs)const{return rdf::LUInt64{lhs.data+rhs.data};}
  RDFTTYPE operator()(rdf::LUInt64 lhs, rdf::LDouble rhs)const{return rdf::LDouble{boost::numeric_cast<double>(lhs.data)+rhs.data};}

  RDFTTYPE operator()(rdf::LDouble lhs, rdf::LInt32  rhs)const{return rdf::LDouble{lhs.data+boost::numeric_cast<double>(rhs.data)};}
  RDFTTYPE operator()(rdf::LDouble lhs, rdf::LUInt32 rhs)const{return rdf::LDouble{lhs.data+boost::numeric_cast<double>(rhs.data)};}
  RDFTTYPE operator()(rdf::LDouble lhs, rdf::LInt64  rhs)const{return rdf::LDouble{lhs.data+boost::numeric_cast<double>(rhs.data)};}
  RDFTTYPE operator()(rdf::LDouble lhs, rdf::LUInt64 rhs)const{return rdf::LDouble{lhs.data+boost::numeric_cast<double>(rhs.data)};}
  RDFTTYPE operator()(rdf::LDouble lhs, rdf::LDouble rhs)const{return rdf::LDouble{lhs.data+rhs.data};}

  RDFTTYPE operator()(rdf::LString lhs, rdf::RDFNull  rhs)const{return rdf::LString{lhs.data+std::string("null")};}
  RDFTTYPE operator()(rdf::LString lhs, rdf::LInt32  rhs)const{return rdf::LString{lhs.data+std::to_string(rhs.data)};}
  RDFTTYPE operator()(rdf::LString lhs, rdf::LUInt32 rhs)const{return rdf::LString{lhs.data+std::to_string(rhs.data)};}
  RDFTTYPE operator()(rdf::LString lhs, rdf::LInt64  rhs)const{return rdf::LString{lhs.data+std::to_string(rhs.data)};}
  RDFTTYPE operator()(rdf::LString lhs, rdf::LUInt64 rhs)const{return rdf::LString{lhs.data+std::to_string(rhs.data)};}
  RDFTTYPE operator()(rdf::LString lhs, rdf::LDouble rhs)const{return rdf::LString{lhs.data+rdf::double_to_text(rhs.data)};}
  RDFTTYPE operator()(rdf::LString lhs, rdf::LString rhs)const{return rdf::LString{lhs.data+rhs.data};}
  RDFTTYPE operator()(rdf::LString lhs, rdf::LDate   rhs)const{return rdf::LString{lhs.data+rdf::to_string(rhs.data)};}
  RDFTTYPE operator()(rdf::LString lhs, rdf::LDatetime rhs)const{return rdf::LString{lhs.data+rdf::to_string(rhs.data)};}

  // -------------------------------------------------------------------------------------------
  RDFTTYPE operator()(rdf::LInt32        lhs, rdf::LDate       rhs)const{return rdf::LDate{rdf::add_days(std::move(rhs.data), lhs.data)};}
  RDFTTYPE operator()(rdf::LUInt32       lhs, rdf::LDate       rhs)const{return rdf::LDate{rdf::add_days(std::move(rhs.data), lhs.data)};}
  RDFTTYPE operator()(rdf::LInt64        lhs, rdf::LDate       rhs)const{return rdf::LDate{rdf::add_days(std::move(rhs.data), lhs.data)};}
  RDFTTYPE operator()(rdf::LUInt64       lhs, rdf::LDate       rhs)const{return rdf::LDate{rdf::add_days(std::move(rhs.data), lhs.data)};}
  RDFTTYPE operator()(rdf::LDouble       lhs, rdf::LDate       rhs)const{return rdf::LDate{rdf::add_days(std::move(rhs.data), int(lhs.data))};}
  // -------------------------------------------------------------------------------------------
  RDFTTYPE operator()(rdf::LInt32        lhs, rdf::LDatetime   rhs)const{return rdf::LDatetime{rdf::datetime{rdf::add_days(rhs.data.date(), lhs.data), rhs.data.time_of_day()}};}
  RDFTTYPE operator()(rdf::LUInt32       lhs, rdf::LDatetime   rhs)const{return rdf::LDatetime{rdf::datetime{rdf::add_days(rhs.data.date(), lhs.data), rhs.data.time_of_day()}};}
  RDFTTYPE operator()(rdf::LInt64        lhs, rdf::LDatetime   rhs)const{return rdf::LDatetime{rdf::datetime{rdf::add_days(rhs.data.date(), lhs.data), rhs.data.time_of_day()}};}
  RDFTTYPE operator()(rdf::LUInt64       lhs, rdf::LDatetime   rhs)const{return rdf::LDatetime{rdf::datetime{rdf::add_days(rhs.data.date(), lhs.data), rhs.data.time_of_day()}};}
  RDFTTYPE operator()(rdf::LDouble       lhs, rdf::LDatetime   rhs)const{return rdf::LDatetime{rdf::datetime{rdf::add_days(rhs.data.date(), int(lhs.data)), rhs.data.time_of_day()}};}
  // -------------------------------------------------------------------------------------------
  RDFTTYPE operator()(rdf::LDate       lhs, rdf::LInt32        rhs)const{return rdf::LDate{rdf::add_days(std::move(lhs.data), rhs.data)};}
  RDFTTYPE operator()(rdf::LDate       lhs, rdf::LUInt32       rhs)const{return rdf::LDate{rdf::add_days(std::move(lhs.data), rhs.data)};}
  RDFTTYPE operator()(rdf::LDate       lhs, rdf::LInt64        rhs)const{return rdf::LDate{rdf::add_days(std::move(lhs.data), rhs.data)};}
  RDFTTYPE operator()(rdf::LDate       lhs, rdf::LUInt64       rhs)const{return rdf::LDate{rdf::add_days(std::move(lhs.data), rhs.data)};}
  RDFTTYPE operator()(rdf::LDate       lhs, rdf::LDouble       rhs)const{return rdf::LDate{rdf::add_days(std::move(lhs.data), int(rhs.data))};}
  
  RDFTTYPE operator()(rdf::LDatetime   lhs, rdf::LInt32        rhs)const{return rdf::LDatetime{rdf::datetime{rdf::add_days(lhs.data.date(), rhs.data), lhs.data.time_of_day()}};}
  RDFTTYPE operator()(rdf::LDatetime   lhs, rdf::LUInt32       rhs)const{return rdf::LDatetime{rdf::datetime{rdf::add_days(lhs.data.date(), rhs.data), lhs.data.time_of_day()}};}
  RDFTTYPE operator()(rdf::LDatetime   lhs, rdf::LInt64        rhs)const{return rdf::LDatetime{rdf::datetime{rdf::add_days(lhs.data.date(), rhs.data), lhs.data.time_of_day()}};}
  RDFTTYPE operator()(rdf::LDatetime   lhs, rdf::LUInt64       rhs)const{return rdf::LDatetime{rdf::datetime{rdf::add_days(lhs.data.date(), rhs.data), lhs.data.time_of_day()}};}
  RDFTTYPE operator()(rdf::LDatetime   lhs, rdf::LDouble       rhs)const{return rdf::LDatetime{rdf::datetime{rdf::add_days(lhs.data.date(), int(rhs.data)), lhs.data.time_of_day()}};}
  ReteSession * rs;
  BetaRow const* br;
};

// SubsVisitor
// --------------------------------------------------------------------------------------
struct SubsVisitor: public boost::static_visitor<RDFTTYPE>, public NoCallbackNeeded
{
  SubsVisitor(ReteSession * rs, BetaRow const* br): rs(rs), br(br) {}
  template<class T, class U> RDFTTYPE operator()(T lhs, U rhs) const {if(br==nullptr) return rdf::Null(); else RETE_EXCEPTION("Invalid arguments for substraction: "<<lhs<<"("<<typeid(lhs).name()<<"), "<<rhs<<"("<<typeid(rhs).name()<<")");};

  RDFTTYPE operator()(rdf::LInt32  lhs, rdf::LInt32  rhs)const{return rdf::LInt32{lhs.data-rhs.data};}
  RDFTTYPE operator()(rdf::LInt32  lhs, rdf::LUInt32 rhs)const{return rdf::LInt32{lhs.data-boost::numeric_cast<int32_t>(rhs.data)};}
  RDFTTYPE operator()(rdf::LInt32  lhs, rdf::LInt64  rhs)const{return rdf::LInt32{lhs.data-boost::numeric_cast<int32_t>(rhs.data)};}
  RDFTTYPE operator()(rdf::LInt32  lhs, rdf::LUInt64 rhs)const{return rdf::LInt32{lhs.data-boost::numeric_cast<int32_t>(rhs.data)};}
  RDFTTYPE operator()(rdf::LInt32  lhs, rdf::LDouble rhs)const{return rdf::LDouble{boost::numeric_cast<double>(lhs.data)-rhs.data};}

  RDFTTYPE operator()(rdf::LUInt32 lhs, rdf::LInt32  rhs)const{return rdf::LUInt32{lhs.data-boost::numeric_cast<uint32_t>(rhs.data)};}
  RDFTTYPE operator()(rdf::LUInt32 lhs, rdf::LUInt32 rhs)const{return rdf::LUInt32{lhs.data-rhs.data};}
  RDFTTYPE operator()(rdf::LUInt32 lhs, rdf::LInt64  rhs)const{return rdf::LUInt32{lhs.data-boost::numeric_cast<uint32_t>(rhs.data)};}
  RDFTTYPE operator()(rdf::LUInt32 lhs, rdf::LUInt64 rhs)const{return rdf::LUInt32{lhs.data-boost::numeric_cast<uint32_t>(rhs.data)};}
  RDFTTYPE operator()(rdf::LUInt32 lhs, rdf::LDouble rhs)const{return rdf::LDouble{boost::numeric_cast<double>(lhs.data)-rhs.data};}

  RDFTTYPE operator()(rdf::LInt64  lhs, rdf::LInt32  rhs)const{return rdf::LInt64{lhs.data-boost::numeric_cast<int64_t>(rhs.data)};}
  RDFTTYPE operator()(rdf::LInt64  lhs, rdf::LUInt32 rhs)const{return rdf::LInt64{lhs.data-boost::numeric_cast<int64_t>(rhs.data)};}
  RDFTTYPE operator()(rdf::LInt64  lhs, rdf::LInt64  rhs)const{return rdf::LInt64{lhs.data-rhs.data};}
  RDFTTYPE operator()(rdf::LInt64  lhs, rdf::LUInt64 rhs)const{return rdf::LInt64{lhs.data-boost::numeric_cast<int64_t>(rhs.data)};}
  RDFTTYPE operator()(rdf::LInt64  lhs, rdf::LDouble rhs)const{return rdf::LDouble{boost::numeric_cast<double>(lhs.data)-rhs.data};}

  RDFTTYPE operator()(rdf::LUInt64 lhs, rdf::LInt32  rhs)const{return rdf::LUInt64{lhs.data-boost::numeric_cast<uint64_t>(rhs.data)};}
  RDFTTYPE operator()(rdf::LUInt64 lhs, rdf::LUInt32 rhs)const{return rdf::LUInt64{lhs.data-boost::numeric_cast<uint64_t>(rhs.data)};}
  RDFTTYPE operator()(rdf::LUInt64 lhs, rdf::LInt64  rhs)const{return rdf::LUInt64{lhs.data-boost::numeric_cast<uint64_t>(rhs.data)};}
  RDFTTYPE operator()(rdf::LUInt64 lhs, rdf::LUInt64 rhs)const{return rdf::LUInt64{lhs.data-boost::numeric_cast<uint64_t>(rhs.data)};}
  RDFTTYPE operator()(rdf::LUInt64 lhs, rdf::LDouble rhs)const{return rdf::LDouble{boost::numeric_cast<double>(lhs.data)-rhs.data};}

  RDFTTYPE operator()(rdf::LDouble lhs, rdf::LInt32  rhs)const{return rdf::LDouble{lhs.data-boost::numeric_cast<double>(rhs.data)};}
  RDFTTYPE operator()(rdf::LDouble lhs, rdf::LUInt32 rhs)const{return rdf::LDouble{lhs.data-boost::numeric_cast<double>(rhs.data)};}
  RDFTTYPE operator()(rdf::LDouble lhs, rdf::LInt64  rhs)const{return rdf::LDouble{lhs.data-boost::numeric_cast<double>(rhs.data)};}
  RDFTTYPE operator()(rdf::LDouble lhs, rdf::LUInt64 rhs)const{return rdf::LDouble{lhs.data-boost::numeric_cast<double>(rhs.data)};}
  RDFTTYPE operator()(rdf::LDouble lhs, rdf::LDouble rhs)const{return rdf::LDouble{lhs.data-boost::numeric_cast<double>(rhs.data)};}

  // -------------------------------------------------------------------------------------------
  RDFTTYPE operator()(rdf::LInt32        lhs, rdf::LDatetime   rhs)const{return rdf::LDatetime{rdf::datetime{rdf::add_days(rhs.data.date(), -lhs.data), rhs.data.time_of_day()}};}
  RDFTTYPE operator()(rdf::LUInt32       lhs, rdf::LDatetime   rhs)const{return rdf::LDatetime{rdf::datetime{rdf::add_days(rhs.data.date(), -lhs.data), rhs.data.time_of_day()}};}
  RDFTTYPE operator()(rdf::LInt64        lhs, rdf::LDatetime   rhs)const{return rdf::LDatetime{rdf::datetime{rdf::add_days(rhs.data.date(), -lhs.data), rhs.data.time_of_day()}};}
  RDFTTYPE operator()(rdf::LUInt64       lhs, rdf::LDatetime   rhs)const{return rdf::LDatetime{rdf::datetime{rdf::add_days(rhs.data.date(), -lhs.data), rhs.data.time_of_day()}};}
  // -------------------------------------------------------------------------------------------
  RDFTTYPE operator()(rdf::LDate       lhs, rdf::LInt32        rhs)const{return rdf::LDate{rdf::add_days(std::move(lhs.data), -rhs.data)};}
  RDFTTYPE operator()(rdf::LDate       lhs, rdf::LUInt32       rhs)const{return rdf::LDate{rdf::add_days(std::move(lhs.data), -rhs.data)};}
  RDFTTYPE operator()(rdf::LDate       lhs, rdf::LInt64        rhs)const{return rdf::LDate{rdf::add_days(std::move(lhs.data), -rhs.data)};}
  RDFTTYPE operator()(rdf::LDate       lhs, rdf::LUInt64       rhs)const{return rdf::LDate{rdf::add_days(std::move(lhs.data), -rhs.data)};}
  RDFTTYPE operator()(rdf::LDate       lhs, rdf::LDate         rhs)const{return rdf::LInt32{rdf::days(std::move(lhs.data), std::move(rhs.data))};}
  
  RDFTTYPE operator()(rdf::LDatetime   lhs, rdf::LInt32        rhs)const{return rdf::LDatetime{rdf::datetime{rdf::add_days(lhs.data.date(), -rhs.data), lhs.data.time_of_day()}};}
  RDFTTYPE operator()(rdf::LDatetime   lhs, rdf::LUInt32       rhs)const{return rdf::LDatetime{rdf::datetime{rdf::add_days(lhs.data.date(), -rhs.data), lhs.data.time_of_day()}};}
  RDFTTYPE operator()(rdf::LDatetime   lhs, rdf::LInt64        rhs)const{return rdf::LDatetime{rdf::datetime{rdf::add_days(lhs.data.date(), -rhs.data), lhs.data.time_of_day()}};}
  RDFTTYPE operator()(rdf::LDatetime   lhs, rdf::LUInt64       rhs)const{return rdf::LDatetime{rdf::datetime{rdf::add_days(lhs.data.date(), -rhs.data), lhs.data.time_of_day()}};}
  ReteSession * rs;
  BetaRow const* br;
};

// MultVisitor
// --------------------------------------------------------------------------------------
struct MultVisitor: public boost::static_visitor<RDFTTYPE>, public NoCallbackNeeded
{
  MultVisitor(ReteSession * rs, BetaRow const* br): rs(rs), br(br) {}
  template<class T, class U> RDFTTYPE operator()(T lhs, U rhs) const {if(br==nullptr) return rdf::Null(); else RETE_EXCEPTION("Invalid arguments for multiplication: "<<lhs<<"("<<typeid(lhs).name()<<"), "<<rhs<<"("<<typeid(rhs).name()<<")");};

  RDFTTYPE operator()(rdf::LInt32  lhs, rdf::LInt32  rhs)const{return rdf::LInt32{lhs.data*rhs.data};}
  RDFTTYPE operator()(rdf::LInt32  lhs, rdf::LUInt32 rhs)const{return rdf::LInt32{lhs.data*boost::numeric_cast<int32_t>(rhs.data)};}
  RDFTTYPE operator()(rdf::LInt32  lhs, rdf::LInt64  rhs)const{return rdf::LInt32{lhs.data*boost::numeric_cast<int32_t>(rhs.data)};}
  RDFTTYPE operator()(rdf::LInt32  lhs, rdf::LUInt64 rhs)const{return rdf::LInt32{lhs.data*boost::numeric_cast<int32_t>(rhs.data)};}
  RDFTTYPE operator()(rdf::LInt32  lhs, rdf::LDouble rhs)const{return rdf::LDouble{boost::numeric_cast<double>(lhs.data)*rhs.data};}

  RDFTTYPE operator()(rdf::LUInt32 lhs, rdf::LInt32  rhs)const{return rdf::LUInt32{lhs.data*boost::numeric_cast<uint32_t>(rhs.data)};}
  RDFTTYPE operator()(rdf::LUInt32 lhs, rdf::LUInt32 rhs)const{return rdf::LUInt32{lhs.data*rhs.data};}
  RDFTTYPE operator()(rdf::LUInt32 lhs, rdf::LInt64  rhs)const{return rdf::LUInt32{lhs.data*boost::numeric_cast<uint32_t>(rhs.data)};}
  RDFTTYPE operator()(rdf::LUInt32 lhs, rdf::LUInt64 rhs)const{return rdf::LUInt32{lhs.data*boost::numeric_cast<uint32_t>(rhs.data)};}
  RDFTTYPE operator()(rdf::LUInt32 lhs, rdf::LDouble rhs)const{return rdf::LDouble{boost::numeric_cast<double>(lhs.data)*rhs.data};}

  RDFTTYPE operator()(rdf::LInt64  lhs, rdf::LInt32  rhs)const{return rdf::LInt64{lhs.data*boost::numeric_cast<int64_t>(rhs.data)};}
  RDFTTYPE operator()(rdf::LInt64  lhs, rdf::LUInt32 rhs)const{return rdf::LInt64{lhs.data*boost::numeric_cast<int64_t>(rhs.data)};}
  RDFTTYPE operator()(rdf::LInt64  lhs, rdf::LInt64  rhs)const{return rdf::LInt64{lhs.data*rhs.data};}
  RDFTTYPE operator()(rdf::LInt64  lhs, rdf::LUInt64 rhs)const{return rdf::LInt64{lhs.data*boost::numeric_cast<int64_t>(rhs.data)};}
  RDFTTYPE operator()(rdf::LInt64  lhs, rdf::LDouble rhs)const{return rdf::LDouble{boost::numeric_cast<double>(lhs.data)*rhs.data};}

  RDFTTYPE operator()(rdf::LUInt64 lhs, rdf::LInt32  rhs)const{return rdf::LUInt64{lhs.data*boost::numeric_cast<uint64_t>(rhs.data)};}
  RDFTTYPE operator()(rdf::LUInt64 lhs, rdf::LUInt32 rhs)const{return rdf::LUInt64{lhs.data*boost::numeric_cast<uint64_t>(rhs.data)};}
  RDFTTYPE operator()(rdf::LUInt64 lhs, rdf::LInt64  rhs)const{return rdf::LUInt64{lhs.data*boost::numeric_cast<uint64_t>(rhs.data)};}
  RDFTTYPE operator()(rdf::LUInt64 lhs, rdf::LUInt64 rhs)const{return rdf::LUInt64{lhs.data*boost::numeric_cast<uint64_t>(rhs.data)};}
  RDFTTYPE operator()(rdf::LUInt64 lhs, rdf::LDouble rhs)const{return rdf::LDouble{boost::numeric_cast<double>(lhs.data)*rhs.data};}

  RDFTTYPE operator()(rdf::LDouble lhs, rdf::LInt32  rhs)const{return rdf::LDouble{lhs.data*boost::numeric_cast<double>(rhs.data)};}
  RDFTTYPE operator()(rdf::LDouble lhs, rdf::LUInt32 rhs)const{return rdf::LDouble{lhs.data*boost::numeric_cast<double>(rhs.data)};}
  RDFTTYPE operator()(rdf::LDouble lhs, rdf::LInt64  rhs)const{return rdf::LDouble{lhs.data*boost::numeric_cast<double>(rhs.data)};}
  RDFTTYPE operator()(rdf::LDouble lhs, rdf::LUInt64 rhs)const{return rdf::LDouble{lhs.data*boost::numeric_cast<double>(rhs.data)};}
  RDFTTYPE operator()(rdf::LDouble lhs, rdf::LDouble rhs)const{return rdf::LDouble{lhs.data*boost::numeric_cast<double>(rhs.data)};}

  ReteSession * rs;
  BetaRow const* br;
};

// DivVisitor
// --------------------------------------------------------------------------------------
// An integer divided by an integer zero is null (Q-69); until 2026-10 it raised SIGFPE,
// which killed the process. A division with a double operand follows IEEE: +inf, -inf,
// and NaN for 0/0. The Go engine follows the same table.
struct DivVisitor: public boost::static_visitor<RDFTTYPE>, public NoCallbackNeeded
{
  DivVisitor(ReteSession * rs, BetaRow const* br): rs(rs), br(br) {}
  template<class T, class U> RDFTTYPE operator()(T lhs, U rhs) const {if(br==nullptr) return rdf::Null(); else RETE_EXCEPTION("Invalid arguments for division: "<<lhs<<"("<<typeid(lhs).name()<<"), "<<rhs<<"("<<typeid(rhs).name()<<")");};

  RDFTTYPE operator()(rdf::LInt32  lhs, rdf::LInt32  rhs)const{if(rhs.data == 0) return rdf::Null(); return rdf::LInt32{lhs.data/rhs.data};}
  RDFTTYPE operator()(rdf::LInt32  lhs, rdf::LUInt32 rhs)const{if(rhs.data == 0) return rdf::Null(); return rdf::LInt32{lhs.data/boost::numeric_cast<int32_t>(rhs.data)};}
  RDFTTYPE operator()(rdf::LInt32  lhs, rdf::LInt64  rhs)const{if(rhs.data == 0) return rdf::Null(); return rdf::LInt32{lhs.data/boost::numeric_cast<int32_t>(rhs.data)};}
  RDFTTYPE operator()(rdf::LInt32  lhs, rdf::LUInt64 rhs)const{if(rhs.data == 0) return rdf::Null(); return rdf::LInt32{lhs.data/boost::numeric_cast<int32_t>(rhs.data)};}
  RDFTTYPE operator()(rdf::LInt32  lhs, rdf::LDouble rhs)const{return rdf::LDouble{boost::numeric_cast<double>(lhs.data)/rhs.data};}

  RDFTTYPE operator()(rdf::LUInt32 lhs, rdf::LInt32  rhs)const{if(rhs.data == 0) return rdf::Null(); return rdf::LUInt32{lhs.data/boost::numeric_cast<uint32_t>(rhs.data)};}
  RDFTTYPE operator()(rdf::LUInt32 lhs, rdf::LUInt32 rhs)const{if(rhs.data == 0) return rdf::Null(); return rdf::LUInt32{lhs.data/rhs.data};}
  RDFTTYPE operator()(rdf::LUInt32 lhs, rdf::LInt64  rhs)const{if(rhs.data == 0) return rdf::Null(); return rdf::LUInt32{lhs.data/boost::numeric_cast<uint32_t>(rhs.data)};}
  RDFTTYPE operator()(rdf::LUInt32 lhs, rdf::LUInt64 rhs)const{if(rhs.data == 0) return rdf::Null(); return rdf::LUInt32{lhs.data/boost::numeric_cast<uint32_t>(rhs.data)};}
  RDFTTYPE operator()(rdf::LUInt32 lhs, rdf::LDouble rhs)const{return rdf::LDouble{boost::numeric_cast<double>(lhs.data)/rhs.data};}

  RDFTTYPE operator()(rdf::LInt64  lhs, rdf::LInt32  rhs)const{if(rhs.data == 0) return rdf::Null(); return rdf::LInt64{lhs.data/boost::numeric_cast<int64_t>(rhs.data)};}
  RDFTTYPE operator()(rdf::LInt64  lhs, rdf::LUInt32 rhs)const{if(rhs.data == 0) return rdf::Null(); return rdf::LInt64{lhs.data/boost::numeric_cast<int64_t>(rhs.data)};}
  RDFTTYPE operator()(rdf::LInt64  lhs, rdf::LInt64  rhs)const{if(rhs.data == 0) return rdf::Null(); return rdf::LInt64{lhs.data/rhs.data};}
  RDFTTYPE operator()(rdf::LInt64  lhs, rdf::LUInt64 rhs)const{if(rhs.data == 0) return rdf::Null(); return rdf::LInt64{lhs.data/boost::numeric_cast<int64_t>(rhs.data)};}
  RDFTTYPE operator()(rdf::LInt64  lhs, rdf::LDouble rhs)const{return rdf::LDouble{boost::numeric_cast<double>(lhs.data)/rhs.data};}

  RDFTTYPE operator()(rdf::LUInt64 lhs, rdf::LInt32  rhs)const{if(rhs.data == 0) return rdf::Null(); return rdf::LUInt64{lhs.data/boost::numeric_cast<uint64_t>(rhs.data)};}
  RDFTTYPE operator()(rdf::LUInt64 lhs, rdf::LUInt32 rhs)const{if(rhs.data == 0) return rdf::Null(); return rdf::LUInt64{lhs.data/boost::numeric_cast<uint64_t>(rhs.data)};}
  RDFTTYPE operator()(rdf::LUInt64 lhs, rdf::LInt64  rhs)const{if(rhs.data == 0) return rdf::Null(); return rdf::LUInt64{lhs.data/boost::numeric_cast<uint64_t>(rhs.data)};}
  RDFTTYPE operator()(rdf::LUInt64 lhs, rdf::LUInt64 rhs)const{if(rhs.data == 0) return rdf::Null(); return rdf::LUInt64{lhs.data/boost::numeric_cast<uint64_t>(rhs.data)};}
  RDFTTYPE operator()(rdf::LUInt64 lhs, rdf::LDouble rhs)const{return rdf::LDouble{boost::numeric_cast<double>(lhs.data)/rhs.data};}

  RDFTTYPE operator()(rdf::LDouble lhs, rdf::LInt32  rhs)const{return rdf::LDouble{lhs.data/boost::numeric_cast<double>(rhs.data)};}
  RDFTTYPE operator()(rdf::LDouble lhs, rdf::LUInt32 rhs)const{return rdf::LDouble{lhs.data/boost::numeric_cast<double>(rhs.data)};}
  RDFTTYPE operator()(rdf::LDouble lhs, rdf::LInt64  rhs)const{return rdf::LDouble{lhs.data/boost::numeric_cast<double>(rhs.data)};}
  RDFTTYPE operator()(rdf::LDouble lhs, rdf::LUInt64 rhs)const{return rdf::LDouble{lhs.data/boost::numeric_cast<double>(rhs.data)};}
  RDFTTYPE operator()(rdf::LDouble lhs, rdf::LDouble rhs)const{return rdf::LDouble{lhs.data/boost::numeric_cast<double>(rhs.data)};}

  ReteSession * rs;
  BetaRow const* br;
};

// AbsVisitor
// --------------------------------------------------------------------------------------
struct AbsVisitor: public boost::static_visitor<RDFTTYPE>, public NoCallbackNeeded
{
  AbsVisitor(ReteSession * rs, BetaRow const* br): rs(rs), br(br) {}
  template<class T> RDFTTYPE operator()(T lhs) const {if(br==nullptr) return rdf::Null(); else RETE_EXCEPTION("Invalid arguments for abs: "<<lhs<<"("<<typeid(lhs).name()<<")");};

  RDFTTYPE operator()(rdf::LInt32  lhs)const{return rdf::LInt32{abs(lhs.data)};}
  RDFTTYPE operator()(rdf::LUInt32 lhs)const{return lhs;}
  RDFTTYPE operator()(rdf::LInt64  lhs)const{return rdf::LInt64{abs(lhs.data)};}
  RDFTTYPE operator()(rdf::LUInt64 lhs)const{return lhs;}
  RDFTTYPE operator()(rdf::LDouble lhs)const{return rdf::LDouble{fabs(lhs.data)};}

  ReteSession * rs;
  BetaRow const* br;
};

// The order that breaks a tie between equal values in min_of, max_of and sorted_head,
// so that the object chosen does not depend on the order the graph holds them in (a hash
// set's, which varies from run to run). A total order, the same in the Go engine:
//  - by kind: a named resource, then a blank node, then a literal, then null;
//  - a named resource by its name, a literal by its text (a double by double_to_text),
//    compared as bytes; two literals of equal text by type, signed integers before
//    unsigned ones (int32, int64, uint32, uint64, then double, string, date, datetime);
//  - blank nodes by key, last: a key follows the order blank nodes were made in.
// An entity minted with a UUID name still varies from run to run under this order.
struct TieBreakText: public boost::static_visitor<std::string>
{
  std::string operator()(rdf::RDFNull       const& )const{return {};}
  std::string operator()(rdf::BlankNode     const& )const{return {};}
  std::string operator()(rdf::NamedResource const&v)const{return v.name;}
  std::string operator()(rdf::LInt32        const&v)const{return std::to_string(v.data);}
  std::string operator()(rdf::LUInt32       const&v)const{return std::to_string(v.data);}
  std::string operator()(rdf::LInt64        const&v)const{return std::to_string(v.data);}
  std::string operator()(rdf::LUInt64       const&v)const{return std::to_string(v.data);}
  std::string operator()(rdf::LDouble       const&v)const{return rdf::double_to_text(v.data);}
  std::string operator()(rdf::LString       const&v)const{return v.data;}
  std::string operator()(rdf::LDate         const&v)const{return rdf::to_string(v.data);}
  std::string operator()(rdf::LDatetime     const&v)const{return rdf::to_string(v.data);}
};

inline int
tie_break_kind(rdf::RdfAstType const& v)
{
  switch(v.which()) {
  case rdf::rdf_named_resource_t: return 0;
  case rdf::rdf_blank_node_t:     return 1;
  case rdf::rdf_null_t:           return 3;
  default:                        return 2;
  }
}

// The type order between literals of equal text: signed, then unsigned, then the rest
inline int
tie_break_type(rdf::RdfAstType const& v)
{
  switch(v.which()) {
  case rdf::rdf_literal_int32_t:    return 0;
  case rdf::rdf_literal_int64_t:    return 1;
  case rdf::rdf_literal_uint32_t:   return 2;
  case rdf::rdf_literal_uint64_t:   return 3;
  case rdf::rdf_literal_double_t:   return 4;
  case rdf::rdf_literal_string_t:   return 5;
  case rdf::rdf_literal_date_t:     return 6;
  case rdf::rdf_literal_datetime_t: return 7;
  default:                          return 8;
  }
}

// true if a comes before b in the tie-break order
inline bool
tie_break_less(rdf::r_index a, rdf::r_index b)
{
  if(a == b) return false;
  int ka = tie_break_kind(*a), kb = tie_break_kind(*b);
  if(ka != kb) return ka < kb;
  if(ka == 1) return boost::get<rdf::BlankNode>(*a).key < boost::get<rdf::BlankNode>(*b).key;
  if(ka == 3) return false;
  TieBreakText text;
  int c = boost::apply_visitor(text, *a).compare(boost::apply_visitor(text, *b));
  if(c != 0) return c < 0;
  return tie_break_type(*a) < tie_break_type(*b);
}

struct ApplyMinMaxVisitor
{
  ApplyMinMaxVisitor(ReteSession * rs, bool is_min, bool ret_obj): rs(rs), is_min(is_min), ret_obj(ret_obj) {}

  // Apply the visitor to find the min/max value.
  //  - case datap == nullptr: return ?o such that min/max ?o in (s, objp, ?o)
  //  - case datap != nullptr: return ?o or ?v such that min/max ?v in (s, objp, ?o).(?o datap ?v)
  // Below in implementation we have:
  //  ?o is currentObj and ?v is currentValue with
  //  (s, objp, currentObj).(currentObj, datap, currentValue), with currentObj = currentValue if datap==nullptr
  RDFTTYPE operator()(rdf::r_index s, rdf::r_index objp, rdf::r_index datap=nullptr)const
  {
    if( not s or not objp) {
      RETE_EXCEPTION("Invalid arguments for min_of/max_of operator, cannot have null lhs or rhs");
    }
    GtVisitor visitor;
    EqVisitor equal;
    auto itor = rs->rdf_session()->find(s, objp);
    bool is_first = true;
    rdf::r_index resultObj = rdf::gnull();
    rdf::r_index resultValue = rdf::gnull();
    rdf::r_index lhs, rhs;
    while(!itor.is_end()) {
      rdf::r_index currentObj = itor.get_object();
      if(currentObj == nullptr) {
        RETE_EXCEPTION("BUG in ApplyMinMaxVisitor: unexpected null value");
      }
      rdf::r_index currentValue = currentObj;
      if(datap != nullptr) {
        currentValue = rs->rdf_session()->get_object(currentObj, datap);
        // Check if no object exist for the relation (currentObj, datap)
        // if so then skip it
      }
      // skip missing values (currentValue would be null)
      if(currentValue != nullptr) {
        if(is_first) {
          resultObj = currentObj;
          resultValue = currentValue;
          is_first = false;
        } else {
          // visitor is for: lhs > rhs
          // to have min, do if resultValue > currentValue, then resultValue = currentValue
          // to have max, do if currentValue > resultValue, then resultValue = currentValue
          if(this->is_min) {
            lhs = resultValue;
            rhs = currentValue;
          } else {
            lhs = currentValue;
            rhs = resultValue;
          }
          if(rdf::to_bool(boost::apply_visitor(visitor, *lhs, *rhs))) {
            resultObj = currentObj;
            resultValue = currentValue; 
          } else if(rdf::to_bool(boost::apply_visitor(equal, *lhs, *rhs)) and
                    tie_break_less(currentObj, resultObj)) {
            // equal values, by the rules' own == (EqVisitor): the object first in the
            // tie-break order, whatever the order they are met in. Until 2026-10 the first
            // met won (I-33). A tie is equality, not "neither is greater", which is not
            // transitive: a NaN or an infinity never ties, and values that cannot be
            // compared (text and a number) keep the first met, a known limit.
            resultObj = currentObj;
            resultValue = currentValue; 
          }
        }
      }
      itor.next();
    }
    if(datap and not this->ret_obj) return *resultValue;
    return *resultObj;
  }

  ReteSession * rs;
  bool is_min;
  bool ret_obj;
};

// MaxOfVisitor * Add truth maintenance
// --------------------------------------------------------------------------------------
inline RDFTTYPE
doMinMaxOf(ReteSession * rs, rdf::r_index lhs, rdf::r_index rhs, bool doMin)
{
  // Determine which mode the operator is to be used
  auto * sess = rs->rdf_session();
  auto const* jr = sess->rmgr()->jets();
  auto objp  = sess->get_object(rhs, jr->jets__entity_property);
  // if objp == null then mode is min/max of a multi value property
  if (objp == nullptr) {
    ApplyMinMaxVisitor av(rs, doMin, true);
    return av(lhs, rhs);
  }
  // Mode is min/max of multi value obj property based on values of datap
  auto datap = sess->get_object(rhs, jr->jets__value_property);
  ApplyMinMaxVisitor av(rs, doMin, false);
  return av(lhs, objp, datap);
}

inline int
registerCallback4MinMaxOf(ReteSession * rs, int vertex, rdf::r_index lhs, rdf::r_index rhs)
{
  // The rhs is either a config carrying jets:entity_property and/or jets:value_property,
  // or the multi valued property itself -- see register_callbacks_for_aggregate, which
  // covers every one of those forms and, since 2026-09-09, watches the entity property
  // as well as the value property.
  return register_callbacks_for_aggregate(rs, vertex, rhs, true);
}

struct MaxOfVisitor: public boost::static_visitor<RDFTTYPE>, public ReadsGraph
{
  MaxOfVisitor(ReteSession * rs, BetaRow const* br): rs(rs), br(br) {}
  template<class T, class U> RDFTTYPE operator()(T lhs, U rhs) const {if(br==nullptr) return rdf::Null(); else RETE_EXCEPTION("Invalid arguments for max_of: ("<<lhs<<", "<<rhs<<")");};

  RDFTTYPE operator()(rdf::NamedResource lhs, rdf::NamedResource rhs)const
  {
    if(not this->rs) return rdf::Null();
    auto * sess = this->rs->rdf_session();
    auto pr = get_resources(sess->rmgr(), std::move(lhs.name), std::move(rhs.name));
    if(not pr.first or not pr.second) return rdf::Null();
    return doMinMaxOf(this->rs, pr.first, pr.second, false);
  }

  int
  register_callback(int vertex, ExprBase::ExprDataType && lhs, ExprBase::ExprDataType && rhs)const
  {
    VLOG(40)<<"MaxOfVisitor::register callback for vertex "<<vertex<<" with pattern (*,"<<rhs<<",*)";
    return registerCallback4MinMaxOf(rs, vertex, 
      get_resource(rs, std::forward<ExprBase::ExprDataType>(lhs)), 
      get_resource(rs, std::forward<ExprBase::ExprDataType>(rhs)));
  }

  ReteSession * rs;
  BetaRow const* br;
};

// MinOfVisitor * Add truth maintenance
// --------------------------------------------------------------------------------------
struct MinOfVisitor: public boost::static_visitor<RDFTTYPE>, public ReadsGraph
{
  MinOfVisitor(ReteSession * rs, BetaRow const* br): rs(rs), br(br) {}
  template<class T, class U> RDFTTYPE operator()(T lhs, U rhs) const {if(br==nullptr) return rdf::Null(); else RETE_EXCEPTION("Invalid arguments for min_of: ("<<lhs<<", "<<rhs<<")");};

  RDFTTYPE operator()(rdf::NamedResource lhs, rdf::NamedResource rhs)const
  {
    if(not this->rs) return rdf::Null();
    auto * sess = this->rs->rdf_session();
    auto pr = get_resources(sess->rmgr(), std::move(lhs.name), std::move(rhs.name));
    if(not pr.first or not pr.second) return rdf::Null();
    return doMinMaxOf(this->rs, pr.first, pr.second, true);
  }

  int
  register_callback(int vertex, ExprBase::ExprDataType && lhs, ExprBase::ExprDataType && rhs)const
  {
    VLOG(40)<<"MinOfVisitor::register callback for vertex "<<vertex<<" with pattern (*,"<<rhs<<",*)";
    return registerCallback4MinMaxOf(rs, vertex, 
      get_resource(rs, std::forward<ExprBase::ExprDataType>(lhs)), 
      get_resource(rs, std::forward<ExprBase::ExprDataType>(rhs)));
  }

  ReteSession * rs;
  BetaRow const* br;
};

// SortedHeadVisitor * Add truth maintenance
// --------------------------------------------------------------------------------------
struct SortedHeadVisitor: public boost::static_visitor<RDFTTYPE>, public ReadsGraph
{
  SortedHeadVisitor(ReteSession * rs, BetaRow const* br): rs(rs), br(br) {}
  template<class T, class U> RDFTTYPE operator()(T lhs, U rhs) const {if(br==nullptr) return rdf::Null(); else RETE_EXCEPTION("Invalid arguments for sorted_head: ("<<lhs<<", "<<rhs<<")");};

  RDFTTYPE operator()(rdf::NamedResource lhs, rdf::NamedResource rhs)const
  {
    auto * sess = this->rs->rdf_session();
    auto pr = get_resources(sess->rmgr(), std::move(lhs.name), std::move(rhs.name));
    if(not pr.first or not pr.second) {
      RETE_EXCEPTION(
        "Invalid argument for sorted_head, must lhs and rhs must "
        "be existing resources, we have "<<lhs<<", and "<<rhs<<" which map to "<<
        pr.first<<", and "<<pr.second
      );
    }
    auto const* jr = sess->rmgr()->jets();
    auto objp  = sess->get_object(pr.second, jr->jets__entity_property);
    auto datap = sess->get_object(pr.second, jr->jets__value_property);
    auto op    = sess->get_object(pr.second, jr->jets__operator);
    // op must be text literal with value "<" or ">"
    bool err = objp==nullptr or datap==nullptr;
    bool is_min = true;
    if(op->which() == rdf::rdf_literal_string_t) {
      auto const& opv = boost::get<rdf::LString>(op)->data;
      if(opv == "<") {
        is_min = true;
      } else if(opv == ">") {
        is_min = false;
      } else {
        err = true;
      }
    } else {
      err = true;
    }
    if(err) {
      RETE_EXCEPTION(
        "Invalid config for sorted_head, must have "
        "jets:operator, jets:entity_property, and jets:data_property set, and "
        "jets:operator must be a text literal with value '<' or '>' (single char text)"
      );
    }
    ApplyMinMaxVisitor av(this->rs, is_min, true);
    return av(pr.first, objp, datap);
  }

  int
  register_callback(int vertex, ExprBase::ExprDataType && lhs, ExprBase::ExprDataType && rhs)const
  {
    VLOG(40)<<"SortedHeadVisitor::register callback for vertex "<<vertex<<" with pattern (*,"<<rhs<<",*)";

    // false: sorted_head has no direct-property form, its rhs must be a config since it
    // needs jets:operator as well.
    return register_callbacks_for_aggregate(rs, vertex,
      get_resource(rs, std::forward<ExprBase::ExprDataType>(rhs)), false);
  }

  ReteSession * rs;
  BetaRow const* br;
};

// SUM VALUES ==========================================================================

// The sum of a property's values, which do not depend on the order the graph holds them
// in (a hash set's, which varies from run to run):
//  - a null is skipped wherever it comes; no value, or only nulls, is null. Until
//    2026-10 a null met first made the whole sum null and one met later was skipped;
//  - numbers: a double present makes the sum a double, every value converted, summed in
//    the order met, so the order can move the last bit only. Integers alone are summed
//    exactly and take the widest integer type present (int32 < uint32 < int64 < uint64),
//    or int64, then double, when the sum does not fit it. Until 2026-10 the first value
//    met set the type, so an int met before a double truncated the double;
//  - a value that is not a number (text, a date, a resource) cannot be summed, and makes
//    the sum null, wherever it comes. Until 2026-10 sum_values folded with +, so text
//    concatenated and a mix depended on the order (5, "x", 2 gave "x25" or no answer).
// The Go engine's SumValuesOp follows the same rules.
class SumOfValues {
 public:
  // false: a value that is not a number was met, and the sum is null
  inline bool add(rdf::RdfAstType const& v)
  {
    switch(v.which()) {
    case rdf::rdf_null_t:           return true;
    case rdf::rdf_literal_int32_t:  return add_int(boost::get<rdf::LInt32>(v).data, 0);
    case rdf::rdf_literal_uint32_t: return add_int(boost::get<rdf::LUInt32>(v).data, 1);
    case rdf::rdf_literal_int64_t:  return add_int(boost::get<rdf::LInt64>(v).data, 2);
    case rdf::rdf_literal_uint64_t: return add_int(boost::get<rdf::LUInt64>(v).data, 3);
    case rdf::rdf_literal_double_t:
      count_++;
      has_double_ = true;
      dsum_ += boost::get<rdf::LDouble>(v).data;
      return true;
    default: return false;
    }
  }

  inline RDFTTYPE result()const
  {
    if(count_ == 0) return rdf::Null();
    if(has_double_) return rdf::LDouble{dsum_};
    switch(rank_) {
    case 0: if(fits<int32_t>())  return rdf::LInt32{static_cast<int32_t>(isum_)};   break;
    case 1: if(fits<uint32_t>()) return rdf::LUInt32{static_cast<uint32_t>(isum_)}; break;
    case 2: break;
    default: if(fits<uint64_t>()) return rdf::LUInt64{static_cast<uint64_t>(isum_)}; break;
    }
    if(fits<int64_t>()) return rdf::LInt64{static_cast<int64_t>(isum_)};
    return rdf::LDouble{static_cast<double>(isum_)};
  }

 private:
  template<class I>
  inline bool add_int(I v, int rank)
  {
    count_++;
    isum_ += v;
    dsum_ += static_cast<double>(v);
    if(rank > rank_) rank_ = rank;
    return true;
  }

  template<class I>
  inline bool fits()const
  {
    return isum_ >= static_cast<__int128>(std::numeric_limits<I>::min()) and
           isum_ <= static_cast<__int128>(std::numeric_limits<I>::max());
  }

  int count_{0};
  int rank_{-1};
  bool has_double_{false};
  __int128 isum_{0};
  double dsum_{0};
};

struct ApplySumValuesVisitor
{
  ApplySumValuesVisitor(ReteSession * rs): rs(rs) {}

  // Apply the visitor to find:
  //  - case objp is nullptr: the sum of ?v in (s, datap, ?v)
  //  - case datap is not nullptr: the sum of ?v in (s, objp, ?o).(?o, datap, ?v)
  RDFTTYPE operator()(rdf::r_index s, rdf::r_index objp, rdf::r_index datap)const
  {
    if(not datap) {
      RETE_EXCEPTION("Invalid arguments for ApplySumValuesVisitor, cannot have null datap");
    }
    SumOfValues sum;
    bool numbers = true;
    for_each_value(s, objp, datap, [&](rdf::RdfAstType const& v) {
      numbers = numbers and sum.add(v);
    });
    if(not numbers) return rdf::Null();
    return sum.result();
  }

  // f(value) for each value, in the order the graph holds them
  template<class F>
  void for_each_value(rdf::r_index s, rdf::r_index objp, rdf::r_index datap, F && f)const
  {
    if(objp == nullptr) {
      // ?v: (s, datap, ?v)
      auto itor = rs->rdf_session()->find(s, datap);
      while(!itor.is_end()) {
        auto val = itor.get_object();
        if(not val) {
          RETE_EXCEPTION("BUG in ApplySumValuesVisitor: unexpected null value");
        }
        f(*val);
        itor.next();
      }
    } else {
      // ?v: (s, objp, ?o).(?o, datap, ?v)
      auto itor = rs->rdf_session()->find(s, objp);
      while(!itor.is_end()) {
        auto val = rs->rdf_session()->get_object(itor.get_object(), datap);
        if(val) f(*val);
        itor.next();
      }
    }
  }

  ReteSession * rs;
};

// SumValuesVisitor * Add truth maintenance
// --------------------------------------------------------------------------------------
struct SumValuesVisitor: public boost::static_visitor<RDFTTYPE>, public ReadsGraph
{
  SumValuesVisitor(ReteSession * rs, BetaRow const* br): rs(rs), br(br) {}
  template<class T, class U> RDFTTYPE operator()(T lhs, U rhs) const {if(br==nullptr) return rdf::Null(); else RETE_EXCEPTION("Invalid arguments for sum_values: ("<<lhs<<", "<<rhs<<")");};

  RDFTTYPE operator()(rdf::NamedResource lhs, rdf::NamedResource rhs)const
  {
    auto * sess = this->rs->rdf_session();
    auto pr = get_resources(sess->rmgr(), std::move(lhs.name), std::move(rhs.name));
    if(not pr.first or not pr.second) {
      RETE_EXCEPTION(
        "Invalid argument for sum_values, must lhs and rhs must "
        "be existing resources, we have "<<lhs<<", and "<<rhs<<" which map to "<<
        pr.first<<", and "<<pr.second
      );
    }
    ApplySumValuesVisitor av(this->rs);
    auto const* jr = sess->rmgr()->jets();
    auto datap = sess->get_object(pr.second, jr->jets__value_property);
    if(not datap) {
      // use rhs as data property, i.e. do the sum of ?v in (lhs, rhs, ?v)
      datap = pr.second;
    }
    auto objp = sess->get_object(pr.second, jr->jets__entity_property);
    return av(pr.first, objp, datap);
  }

  int
  register_callback(int vertex, ExprBase::ExprDataType && lhs, ExprBase::ExprDataType && rhs)const
  {
    VLOG(40)<<"SumValuesVisitor::register callback for vertex "<<vertex<<" with pattern (*,"<<rhs<<",*)";

    // Until 2026-09-09 this returned 0 whenever the rhs carried no jets:value_property,
    // so (?s sum_values someMultiValuedProperty) had no truth maintenance at all in the
    // C++ engine while the Go SumValuesOp recomputed it. That fallback is now in
    // register_callbacks_for_aggregate, which also watches the entity property.
    return register_callbacks_for_aggregate(rs, vertex,
      get_resource(rs, std::forward<ExprBase::ExprDataType>(rhs)), true);
  }

  ReteSession * rs;
  BetaRow const* br;
};

// JOIN VALUES =========================================================================
// Fold a multi-valued property into ONE ordered string.
//
// Mirrors sum_values (see ApplySumValuesVisitor above) rather than inventing a shape:
// the same two config forms, the same truth-maintenance callback. What it adds is a
// jets:separator triple on the config and a sort.
//
// THE OUTPUT IS SORTED, AND THE REASON IS REPRODUCIBILITY RATHER THAN TASTE.
// An rdf multi-valued property is a *set* and has no order, so the iteration order is
// an implementation detail of the graph container. An unsorted join would therefore
// make the same entity produce a different string — and, downstream, a different LLM
// prompt — between two runs over identical data, which is noise injected straight into
// an experiment whose design is one variable. Sorting is on the *rendered* strings,
// byte-wise ascending, which is also what the Go engine does; ISO-8601 dates sort
// lexically into chronological order, so the sort costs nothing for the date case that
// motivated the operator.
//
// BEHAVIOUR, DECIDED AND DOCUMENTED:
//  - empty set   -> rdf::Null(). Nothing is asserted, exactly as sum_values does when it
//                   finds no value. An empty string would assert "this entity has an
//                   empty list", which is a different claim from "this entity has no
//                   list", and the rule author can still test for the absence.
//  - single value-> that value rendered, with no separator. A separator separates.
//  - null/absent -> skipped, and no empty slot is left behind. In the
//    values          entity_property+value_property form an object that carries no value
//                    property contributes nothing rather than an empty string, which
//                    matches min_of/max_of/sum_values, all of which skip nulls.
//  - duplicates  -> kept. This is a fold, not a distinct: two pharmacy claims filled on
//                   the same day are two fills, and dropping one would lose a fact.
//  - non-string  -> rendered by JoinTextVisitor below. Dates and datetimes render
//    values         ISO-8601 (YYYY-MM-DD, YYYY-MM-DDTHH:MM:SS), integers as decimal,
//                   doubles by rdf::double_to_text (shortest digits), named resources by
//                   name. The Go engine renders identically — see the note in
//                   jets/jetrules/rete/expr_operator_math_join_values.go, since two
//                   engines that render a date differently is the same defect as two
//                   engines that support different operators.
//
// The separator comes from (config, jets:separator, "..."). It is OPTIONAL and defaults
// to ", ": a missing separator running the values together is a worse failure than a
// sensible default, and the default is the one the fill-date case wants.

// Render one rdf value the way join_values wants it in the joined string.
// Not ToTextVisitor, which returns a literal; the text is the same: a double is written
// by rdf::double_to_text, as the Go engine's join_values writes it. Until 2026-10 it was
// %g's six significant digits.
struct JoinTextVisitor: public boost::static_visitor<std::string>
{
  std::string operator()(rdf::RDFNull       const& )const{return {};}
  std::string operator()(rdf::BlankNode     const&v)const{return {"bn("+std::to_string(v.key)+")"};}
  std::string operator()(rdf::NamedResource const&v)const{return v.name;}
  std::string operator()(rdf::LInt32        const&v)const{return std::to_string(v.data);}
  std::string operator()(rdf::LUInt32       const&v)const{return std::to_string(v.data);}
  std::string operator()(rdf::LInt64        const&v)const{return std::to_string(v.data);}
  std::string operator()(rdf::LUInt64       const&v)const{return std::to_string(v.data);}
  std::string operator()(rdf::LDouble       const&v)const{return rdf::double_to_text(v.data);}
  std::string operator()(rdf::LString       const&v)const{return v.data;}
  std::string operator()(rdf::LDate         const&v)const{return rdf::to_string(v.data);}
  std::string operator()(rdf::LDatetime     const&v)const{return rdf::to_string(v.data);}
};

inline std::string
join_render_value(rdf::r_index v)
{
  if(not v) return {};
  return boost::apply_visitor(JoinTextVisitor(), *v);
}

struct ApplyJoinValuesVisitor
{
  ApplyJoinValuesVisitor(ReteSession * rs, std::string separator)
    : rs(rs), separator(std::move(separator)) {}

  // Apply the visitor to find:
  //  - case objp is nullptr: the sorted join of ?v in (s, datap, ?v)
  //  - case objp is not nullptr: the sorted join of ?v in (s, objp, ?o).(?o, datap, ?v)
  RDFTTYPE operator()(rdf::r_index s, rdf::r_index objp, rdf::r_index datap)const
  {
    if(not datap) {
      RETE_EXCEPTION("Invalid arguments for ApplyJoinValuesVisitor, cannot have null datap");
    }
    std::vector<std::string> items;
    if(objp == nullptr) {
      // join ?v: (s, datap, ?v)
      auto itor = rs->rdf_session()->find(s, datap);
      while(!itor.is_end()) {
        auto val = itor.get_object();
        if(val and val->which() != rdf::rdf_null_t) {
          items.push_back(join_render_value(val));
        }
        itor.next();
      }
    } else {
      // join ?v: (s, objp, ?o).(?o, datap, ?v)
      auto itor = rs->rdf_session()->find(s, objp);
      while(!itor.is_end()) {
        auto val = rs->rdf_session()->get_object(itor.get_object(), datap);
        if(val and val->which() != rdf::rdf_null_t) {
          items.push_back(join_render_value(val));
        }
        itor.next();
      }
    }
    if(items.empty()) return rdf::RDFNull();
    std::sort(items.begin(), items.end());
    std::string res;
    bool is_first = true;
    for(auto const& item: items) {
      if(is_first) {
        is_first = false;
      } else {
        res += this->separator;
      }
      res += item;
    }
    return rdf::LString(std::move(res));
  }

  ReteSession * rs;
  std::string separator;
};

// JoinValuesVisitor * Add truth maintenance
// --------------------------------------------------------------------------------------
struct JoinValuesVisitor: public boost::static_visitor<RDFTTYPE>, public ReadsGraph
{
  JoinValuesVisitor(ReteSession * rs, BetaRow const* br): rs(rs), br(br) {}
  template<class T, class U> RDFTTYPE operator()(T lhs, U rhs) const {if(br==nullptr) return rdf::Null(); else RETE_EXCEPTION("Invalid arguments for join_values: ("<<lhs<<", "<<rhs<<")");};

  RDFTTYPE operator()(rdf::NamedResource lhs, rdf::NamedResource rhs)const
  {
    auto * sess = this->rs->rdf_session();
    auto pr = get_resources(sess->rmgr(), std::move(lhs.name), std::move(rhs.name));
    if(not pr.first or not pr.second) {
      RETE_EXCEPTION(
        "Invalid argument for join_values, must lhs and rhs must "
        "be existing resources, we have "<<lhs<<", and "<<rhs<<" which map to "<<
        pr.first<<", and "<<pr.second
      );
    }
    auto const* jr = sess->rmgr()->jets();
    // The separator is optional, defaulting to ", ".
    std::string separator{", "};
    auto sep = sess->get_object(pr.second, jr->jets__separator);
    if(sep) {
      if(sep->which() != rdf::rdf_literal_string_t) {
        RETE_EXCEPTION("Invalid argument for join_values: jets:separator must be a text literal, got "<<sep);
      }
      separator = boost::get<rdf::LString>(*sep).data;
    }
    ApplyJoinValuesVisitor av(this->rs, std::move(separator));
    // Three config forms, stated explicitly rather than fallen into, since a config
    // carrying jets:entity_property and no jets:value_property is a misconfiguration
    // that sum_values silently turns into an empty result.
    auto objp = sess->get_object(pr.second, jr->jets__entity_property);
    auto datap = sess->get_object(pr.second, jr->jets__value_property);
    if(objp) {
      // (lhs, objp, ?o).(?o, datap, ?v) -- objp non functional, datap functional
      if(not datap) {
        RETE_EXCEPTION(
          "Invalid argument for join_values: config "<<rhs<<" has jets:entity_property "
          "and no jets:value_property");
      }
    } else if(datap) {
      // config carrying only jets:value_property: join ?v in (lhs, datap, ?v)
    } else {
      // rhs is itself the non functional data property: join ?v in (lhs, rhs, ?v)
      datap = pr.second;
    }
    return av(pr.first, objp, datap);
  }

  int
  register_callback(int vertex, ExprBase::ExprDataType && lhs, ExprBase::ExprDataType && rhs)const
  {
    VLOG(40)<<"JoinValuesVisitor::register callback for vertex "<<vertex<<" with pattern (*,"<<rhs<<",*)";

    // The properties to watch are jets:entity_property and jets:value_property when the
    // rhs is a config, and the rhs itself when it is the multi-valued property.
    //
    // The second case used to be join_values' own: SumValuesVisitor registered nothing
    // for it while the Go SumValuesOp did. That is fixed as of 2026-09-09 and both now
    // go through register_callbacks_for_aggregate, which additionally watches the
    // entity property -- which none of these operators did, on either side.
    return register_callbacks_for_aggregate(rs, vertex,
      get_resource(rs, std::forward<ExprBase::ExprDataType>(rhs)), true);
  }

  ReteSession * rs;
  BetaRow const* br;
};

// ToIntVisitor
// --------------------------------------------------------------------------------------
struct ToIntVisitor: public boost::static_visitor<RDFTTYPE>, public NoCallbackNeeded
{
  ToIntVisitor(ReteSession * rs, BetaRow const* br): rs(rs), br(br) {}
  template<class T> RDFTTYPE operator()(T lhs) const {if(br==nullptr) return rdf::Null(); else RETE_EXCEPTION("Invalid arguments for to_int: ("<<lhs<<")");};

  RDFTTYPE operator()(rdf::LInt32  lhs)const{return lhs;}
  RDFTTYPE operator()(rdf::LUInt32 lhs)const{return rdf::LInt32{boost::numeric_cast<int32_t>(lhs.data)};}
  RDFTTYPE operator()(rdf::LInt64  lhs)const{return rdf::LInt32{boost::numeric_cast<int32_t>(lhs.data)};}
  RDFTTYPE operator()(rdf::LUInt64 lhs)const{return rdf::LInt32{boost::numeric_cast<int32_t>(lhs.data)};}
  RDFTTYPE operator()(rdf::LDouble lhs)const{return rdf::LInt32{boost::numeric_cast<int32_t>(lhs.data)};}
  RDFTTYPE operator()(rdf::LString lhs)const
  {
    auto view = rdf::trim_view(lhs.data);
    if(view.empty()) {
      RETE_EXCEPTION("NULL arguments for to_int");
    }
    return rdf::LInt32(boost::lexical_cast<int32_t>(view));
  }

  ReteSession * rs;
  BetaRow const* br;
};

// ToDoubleVisitor
// --------------------------------------------------------------------------------------
struct ToDoubleVisitor: public boost::static_visitor<RDFTTYPE>, public NoCallbackNeeded
{
  ToDoubleVisitor(ReteSession * rs, BetaRow const* br): rs(rs), br(br) {}
  template<class T> RDFTTYPE operator()(T lhs) const {if(br==nullptr) return rdf::Null(); else RETE_EXCEPTION("Invalid arguments for to_double: ("<<lhs<<")");};

  RDFTTYPE operator()(rdf::LInt32  lhs)const{return rdf::LDouble{boost::numeric_cast<double_t>(lhs.data)};}
  RDFTTYPE operator()(rdf::LUInt32 lhs)const{return rdf::LDouble{boost::numeric_cast<double_t>(lhs.data)};}
  RDFTTYPE operator()(rdf::LInt64  lhs)const{return rdf::LDouble{boost::numeric_cast<double_t>(lhs.data)};}
  RDFTTYPE operator()(rdf::LUInt64 lhs)const{return rdf::LDouble{boost::numeric_cast<double_t>(lhs.data)};}
  RDFTTYPE operator()(rdf::LDouble lhs)const{return lhs;}
  RDFTTYPE operator()(rdf::LString lhs)const
  {
    auto view = rdf::trim_view(lhs.data);
    if(view.empty()) {
      RETE_EXCEPTION("NULL arguments for to_double");
    }
    return rdf::LDouble(boost::lexical_cast<double_t>(view));
  }

  ReteSession * rs;
  BetaRow const* br;
};

// ToTextVisitor
// --------------------------------------------------------------------------------------
struct ToTextVisitor: public boost::static_visitor<RDFTTYPE>, public NoCallbackNeeded
{
  ToTextVisitor(ReteSession * rs, BetaRow const* br): rs(rs), br(br) {}
  template<class T> RDFTTYPE operator()(T lhs) const {if(br==nullptr) return rdf::Null(); else RETE_EXCEPTION("Invalid arguments for to_text: ("<<lhs<<")");};

  RDFTTYPE operator()(rdf::RDFNull   lhs)const{return rdf::LString{std::string("null")};}
  RDFTTYPE operator()(rdf::LInt32    lhs)const{return rdf::LString{std::to_string(lhs.data)};}
  RDFTTYPE operator()(rdf::LUInt32   lhs)const{return rdf::LString{std::to_string(lhs.data)};}
  RDFTTYPE operator()(rdf::LInt64    lhs)const{return rdf::LString{std::to_string(lhs.data)};}
  RDFTTYPE operator()(rdf::LUInt64   lhs)const{return rdf::LString{std::to_string(lhs.data)};}
  RDFTTYPE operator()(rdf::LDouble   lhs)const{return rdf::LString{rdf::double_to_text(lhs.data)};}
  RDFTTYPE operator()(rdf::LString   lhs)const{return lhs;}
  RDFTTYPE operator()(rdf::LDate     lhs)const{return rdf::LString{rdf::to_string(lhs.data)};}
  RDFTTYPE operator()(rdf::LDatetime lhs)const{return rdf::LString{rdf::to_string(lhs.data)};}
  RDFTTYPE operator()(rdf::NamedResource lhs)const{return rdf::LString{lhs.name};}
  ReteSession * rs;
  BetaRow const* br;
};



} // namespace jets::rete
#endif // JETS_RETE_EXPR_OP_ARITHMETICS_H
