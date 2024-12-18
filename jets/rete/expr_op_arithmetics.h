#ifndef JETS_RETE_EXPR_OP_ARITHMETICS_H
#define JETS_RETE_EXPR_OP_ARITHMETICS_H

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

// AddVisitor
// --------------------------------------------------------------------------------------
struct AddVisitor: public boost::static_visitor<RDFTTYPE>, public NoCallbackNeeded
{
  AddVisitor(ReteSession * rs, BetaRow const* br): rs(rs), br(br) {}
  AddVisitor(): rs(nullptr), br(nullptr) {}
  template<class T, class U> RDFTTYPE operator()(T lhs, U rhs) const {if(br==nullptr) return rdf::Null(); else RETE_EXCEPTION("Invalid arguments for ADD: ("<<lhs<<", "<<rhs<<")");};

  RDFTTYPE operator()(rdf::LInt32  lhs, rdf::LInt32  rhs)const{return rdf::LInt32{lhs.data+rhs.data};}
  RDFTTYPE operator()(rdf::LInt32  lhs, rdf::LUInt32 rhs)const{return rdf::LInt32{lhs.data+boost::numeric_cast<int32_t>(rhs.data)};}
  RDFTTYPE operator()(rdf::LInt32  lhs, rdf::LInt64  rhs)const{return rdf::LInt32{lhs.data+boost::numeric_cast<int32_t>(rhs.data)};}
  RDFTTYPE operator()(rdf::LInt32  lhs, rdf::LUInt64 rhs)const{return rdf::LInt32{lhs.data+boost::numeric_cast<int32_t>(rhs.data)};}
  RDFTTYPE operator()(rdf::LInt32  lhs, rdf::LDouble rhs)const{return rdf::LInt32{lhs.data+boost::numeric_cast<int32_t>(rhs.data)};}

  RDFTTYPE operator()(rdf::LUInt32 lhs, rdf::LInt32  rhs)const{return rdf::LUInt32{lhs.data+boost::numeric_cast<uint32_t>(rhs.data)};}
  RDFTTYPE operator()(rdf::LUInt32 lhs, rdf::LUInt32 rhs)const{return rdf::LUInt32{lhs.data+rhs.data};}
  RDFTTYPE operator()(rdf::LUInt32 lhs, rdf::LInt64  rhs)const{return rdf::LUInt32{lhs.data+boost::numeric_cast<uint32_t>(rhs.data)};}
  RDFTTYPE operator()(rdf::LUInt32 lhs, rdf::LUInt64 rhs)const{return rdf::LUInt32{lhs.data+boost::numeric_cast<uint32_t>(rhs.data)};}
  RDFTTYPE operator()(rdf::LUInt32 lhs, rdf::LDouble rhs)const{return rdf::LUInt32{lhs.data+boost::numeric_cast<uint32_t>(rhs.data)};}

  RDFTTYPE operator()(rdf::LInt64  lhs, rdf::LInt32  rhs)const{return rdf::LInt64{lhs.data+boost::numeric_cast<int64_t>(rhs.data)};}
  RDFTTYPE operator()(rdf::LInt64  lhs, rdf::LUInt32 rhs)const{return rdf::LInt64{lhs.data+boost::numeric_cast<int64_t>(rhs.data)};}
  RDFTTYPE operator()(rdf::LInt64  lhs, rdf::LInt64  rhs)const{return rdf::LInt64{lhs.data+rhs.data};}
  RDFTTYPE operator()(rdf::LInt64  lhs, rdf::LUInt64 rhs)const{return rdf::LInt64{lhs.data+boost::numeric_cast<int64_t>(rhs.data)};}
  RDFTTYPE operator()(rdf::LInt64  lhs, rdf::LDouble rhs)const{return rdf::LInt64{lhs.data+boost::numeric_cast<int64_t>(rhs.data)};}

  RDFTTYPE operator()(rdf::LUInt64 lhs, rdf::LInt32  rhs)const{return rdf::LUInt64{lhs.data+boost::numeric_cast<uint64_t>(rhs.data)};}
  RDFTTYPE operator()(rdf::LUInt64 lhs, rdf::LUInt32 rhs)const{return rdf::LUInt64{lhs.data+boost::numeric_cast<uint64_t>(rhs.data)};}
  RDFTTYPE operator()(rdf::LUInt64 lhs, rdf::LInt64  rhs)const{return rdf::LUInt64{lhs.data+boost::numeric_cast<uint64_t>(rhs.data)};}
  RDFTTYPE operator()(rdf::LUInt64 lhs, rdf::LUInt64 rhs)const{return rdf::LUInt64{lhs.data+rhs.data};}
  RDFTTYPE operator()(rdf::LUInt64 lhs, rdf::LDouble rhs)const{return rdf::LUInt64{lhs.data+boost::numeric_cast<uint64_t>(rhs.data)};}

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
  RDFTTYPE operator()(rdf::LString lhs, rdf::LDouble rhs)const{return rdf::LString{lhs.data+std::to_string(rhs.data)};}
  RDFTTYPE operator()(rdf::LString lhs, rdf::LString rhs)const{return rdf::LString{lhs.data+rhs.data};}
  RDFTTYPE operator()(rdf::LString lhs, rdf::LDate   rhs)const{return rdf::LString{lhs.data+rdf::to_string(rhs.data)};}
  RDFTTYPE operator()(rdf::LString lhs, rdf::LDatetime rhs)const{return rdf::LString{lhs.data+rdf::to_string(rhs.data)};}

  // -------------------------------------------------------------------------------------------
  RDFTTYPE operator()(rdf::LInt32        lhs, rdf::LDate       rhs)const{return rdf::LDate{rdf::add_days(std::move(rhs.data), lhs.data)};}
  RDFTTYPE operator()(rdf::LUInt32       lhs, rdf::LDate       rhs)const{return rdf::LDate{rdf::add_days(std::move(rhs.data), lhs.data)};}
  RDFTTYPE operator()(rdf::LInt64        lhs, rdf::LDate       rhs)const{return rdf::LDate{rdf::add_days(std::move(rhs.data), lhs.data)};}
  RDFTTYPE operator()(rdf::LUInt64       lhs, rdf::LDate       rhs)const{return rdf::LDate{rdf::add_days(std::move(rhs.data), lhs.data)};}
  // -------------------------------------------------------------------------------------------
  RDFTTYPE operator()(rdf::LInt32        lhs, rdf::LDatetime   rhs)const{return rdf::LDatetime{rdf::datetime{rdf::add_days(rhs.data.date(), lhs.data), rhs.data.time_of_day()}};}
  RDFTTYPE operator()(rdf::LUInt32       lhs, rdf::LDatetime   rhs)const{return rdf::LDatetime{rdf::datetime{rdf::add_days(rhs.data.date(), lhs.data), rhs.data.time_of_day()}};}
  RDFTTYPE operator()(rdf::LInt64        lhs, rdf::LDatetime   rhs)const{return rdf::LDatetime{rdf::datetime{rdf::add_days(rhs.data.date(), lhs.data), rhs.data.time_of_day()}};}
  RDFTTYPE operator()(rdf::LUInt64       lhs, rdf::LDatetime   rhs)const{return rdf::LDatetime{rdf::datetime{rdf::add_days(rhs.data.date(), lhs.data), rhs.data.time_of_day()}};}
  // -------------------------------------------------------------------------------------------
  RDFTTYPE operator()(rdf::LDate       lhs, rdf::LInt32        rhs)const{return rdf::LDate{rdf::add_days(std::move(lhs.data), rhs.data)};}
  RDFTTYPE operator()(rdf::LDate       lhs, rdf::LUInt32       rhs)const{return rdf::LDate{rdf::add_days(std::move(lhs.data), rhs.data)};}
  RDFTTYPE operator()(rdf::LDate       lhs, rdf::LInt64        rhs)const{return rdf::LDate{rdf::add_days(std::move(lhs.data), rhs.data)};}
  RDFTTYPE operator()(rdf::LDate       lhs, rdf::LUInt64       rhs)const{return rdf::LDate{rdf::add_days(std::move(lhs.data), rhs.data)};}
  
  RDFTTYPE operator()(rdf::LDatetime   lhs, rdf::LInt32        rhs)const{return rdf::LDatetime{rdf::datetime{rdf::add_days(lhs.data.date(), rhs.data), lhs.data.time_of_day()}};}
  RDFTTYPE operator()(rdf::LDatetime   lhs, rdf::LUInt32       rhs)const{return rdf::LDatetime{rdf::datetime{rdf::add_days(lhs.data.date(), rhs.data), lhs.data.time_of_day()}};}
  RDFTTYPE operator()(rdf::LDatetime   lhs, rdf::LInt64        rhs)const{return rdf::LDatetime{rdf::datetime{rdf::add_days(lhs.data.date(), rhs.data), lhs.data.time_of_day()}};}
  RDFTTYPE operator()(rdf::LDatetime   lhs, rdf::LUInt64       rhs)const{return rdf::LDatetime{rdf::datetime{rdf::add_days(lhs.data.date(), rhs.data), lhs.data.time_of_day()}};}
  ReteSession * rs;
  BetaRow const* br;
};

// SubsVisitor
// --------------------------------------------------------------------------------------
struct SubsVisitor: public boost::static_visitor<RDFTTYPE>, public NoCallbackNeeded
{
  SubsVisitor(ReteSession * rs, BetaRow const* br): rs(rs), br(br) {}
  template<class T, class U> RDFTTYPE operator()(T lhs, U rhs) const {if(br==nullptr) return rdf::Null(); else RETE_EXCEPTION("Invalid arguments for substraction: ("<<lhs<<", "<<rhs<<")");};

  RDFTTYPE operator()(rdf::LInt32  lhs, rdf::LInt32  rhs)const{return rdf::LInt32{lhs.data-rhs.data};}
  RDFTTYPE operator()(rdf::LInt32  lhs, rdf::LUInt32 rhs)const{return rdf::LInt32{lhs.data-boost::numeric_cast<int32_t>(rhs.data)};}
  RDFTTYPE operator()(rdf::LInt32  lhs, rdf::LInt64  rhs)const{return rdf::LInt32{lhs.data-boost::numeric_cast<int32_t>(rhs.data)};}
  RDFTTYPE operator()(rdf::LInt32  lhs, rdf::LUInt64 rhs)const{return rdf::LInt32{lhs.data-boost::numeric_cast<int32_t>(rhs.data)};}
  RDFTTYPE operator()(rdf::LInt32  lhs, rdf::LDouble rhs)const{return rdf::LInt32{lhs.data-boost::numeric_cast<int32_t>(rhs.data)};}

  RDFTTYPE operator()(rdf::LUInt32 lhs, rdf::LInt32  rhs)const{return rdf::LUInt32{lhs.data-boost::numeric_cast<uint32_t>(rhs.data)};}
  RDFTTYPE operator()(rdf::LUInt32 lhs, rdf::LUInt32 rhs)const{return rdf::LUInt32{lhs.data-rhs.data};}
  RDFTTYPE operator()(rdf::LUInt32 lhs, rdf::LInt64  rhs)const{return rdf::LUInt32{lhs.data-boost::numeric_cast<uint32_t>(rhs.data)};}
  RDFTTYPE operator()(rdf::LUInt32 lhs, rdf::LUInt64 rhs)const{return rdf::LUInt32{lhs.data-boost::numeric_cast<uint32_t>(rhs.data)};}
  RDFTTYPE operator()(rdf::LUInt32 lhs, rdf::LDouble rhs)const{return rdf::LUInt32{lhs.data-boost::numeric_cast<uint32_t>(rhs.data)};}

  RDFTTYPE operator()(rdf::LInt64  lhs, rdf::LInt32  rhs)const{return rdf::LInt64{lhs.data-boost::numeric_cast<int64_t>(rhs.data)};}
  RDFTTYPE operator()(rdf::LInt64  lhs, rdf::LUInt32 rhs)const{return rdf::LInt64{lhs.data-boost::numeric_cast<int64_t>(rhs.data)};}
  RDFTTYPE operator()(rdf::LInt64  lhs, rdf::LInt64  rhs)const{return rdf::LInt64{lhs.data-rhs.data};}
  RDFTTYPE operator()(rdf::LInt64  lhs, rdf::LUInt64 rhs)const{return rdf::LInt64{lhs.data-boost::numeric_cast<int64_t>(rhs.data)};}
  RDFTTYPE operator()(rdf::LInt64  lhs, rdf::LDouble rhs)const{return rdf::LInt64{lhs.data-boost::numeric_cast<int64_t>(rhs.data)};}

  RDFTTYPE operator()(rdf::LUInt64 lhs, rdf::LInt32  rhs)const{return rdf::LUInt64{lhs.data-boost::numeric_cast<uint64_t>(rhs.data)};}
  RDFTTYPE operator()(rdf::LUInt64 lhs, rdf::LUInt32 rhs)const{return rdf::LUInt64{lhs.data-boost::numeric_cast<uint64_t>(rhs.data)};}
  RDFTTYPE operator()(rdf::LUInt64 lhs, rdf::LInt64  rhs)const{return rdf::LUInt64{lhs.data-boost::numeric_cast<uint64_t>(rhs.data)};}
  RDFTTYPE operator()(rdf::LUInt64 lhs, rdf::LUInt64 rhs)const{return rdf::LUInt64{lhs.data-boost::numeric_cast<uint64_t>(rhs.data)};}
  RDFTTYPE operator()(rdf::LUInt64 lhs, rdf::LDouble rhs)const{return rdf::LUInt64{lhs.data-boost::numeric_cast<uint64_t>(rhs.data)};}

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
  template<class T, class U> RDFTTYPE operator()(T lhs, U rhs) const {if(br==nullptr) return rdf::Null(); else RETE_EXCEPTION("Invalid arguments for Multiplication: ("<<lhs<<", "<<rhs<<")");};

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
struct DivVisitor: public boost::static_visitor<RDFTTYPE>, public NoCallbackNeeded
{
  DivVisitor(ReteSession * rs, BetaRow const* br): rs(rs), br(br) {}
  template<class T, class U> RDFTTYPE operator()(T lhs, U rhs) const {if(br==nullptr) return rdf::Null(); else RETE_EXCEPTION("Invalid arguments for Division: ("<<lhs<<", "<<rhs<<")");};

  RDFTTYPE operator()(rdf::LInt32  lhs, rdf::LInt32  rhs)const{return rdf::LInt32{lhs.data/rhs.data};}
  RDFTTYPE operator()(rdf::LInt32  lhs, rdf::LUInt32 rhs)const{return rdf::LInt32{lhs.data/boost::numeric_cast<int32_t>(rhs.data)};}
  RDFTTYPE operator()(rdf::LInt32  lhs, rdf::LInt64  rhs)const{return rdf::LInt32{lhs.data/boost::numeric_cast<int32_t>(rhs.data)};}
  RDFTTYPE operator()(rdf::LInt32  lhs, rdf::LUInt64 rhs)const{return rdf::LInt32{lhs.data/boost::numeric_cast<int32_t>(rhs.data)};}
  RDFTTYPE operator()(rdf::LInt32  lhs, rdf::LDouble rhs)const{return rdf::LDouble{boost::numeric_cast<double>(lhs.data)/rhs.data};}

  RDFTTYPE operator()(rdf::LUInt32 lhs, rdf::LInt32  rhs)const{return rdf::LUInt32{lhs.data/boost::numeric_cast<uint32_t>(rhs.data)};}
  RDFTTYPE operator()(rdf::LUInt32 lhs, rdf::LUInt32 rhs)const{return rdf::LUInt32{lhs.data/rhs.data};}
  RDFTTYPE operator()(rdf::LUInt32 lhs, rdf::LInt64  rhs)const{return rdf::LUInt32{lhs.data/boost::numeric_cast<uint32_t>(rhs.data)};}
  RDFTTYPE operator()(rdf::LUInt32 lhs, rdf::LUInt64 rhs)const{return rdf::LUInt32{lhs.data/boost::numeric_cast<uint32_t>(rhs.data)};}
  RDFTTYPE operator()(rdf::LUInt32 lhs, rdf::LDouble rhs)const{return rdf::LDouble{boost::numeric_cast<double>(lhs.data)/rhs.data};}

  RDFTTYPE operator()(rdf::LInt64  lhs, rdf::LInt32  rhs)const{return rdf::LInt64{lhs.data/boost::numeric_cast<int64_t>(rhs.data)};}
  RDFTTYPE operator()(rdf::LInt64  lhs, rdf::LUInt32 rhs)const{return rdf::LInt64{lhs.data/boost::numeric_cast<int64_t>(rhs.data)};}
  RDFTTYPE operator()(rdf::LInt64  lhs, rdf::LInt64  rhs)const{return rdf::LInt64{lhs.data/rhs.data};}
  RDFTTYPE operator()(rdf::LInt64  lhs, rdf::LUInt64 rhs)const{return rdf::LInt64{lhs.data/boost::numeric_cast<int64_t>(rhs.data)};}
  RDFTTYPE operator()(rdf::LInt64  lhs, rdf::LDouble rhs)const{return rdf::LDouble{boost::numeric_cast<double>(lhs.data)/rhs.data};}

  RDFTTYPE operator()(rdf::LUInt64 lhs, rdf::LInt32  rhs)const{return rdf::LUInt64{lhs.data/boost::numeric_cast<uint64_t>(rhs.data)};}
  RDFTTYPE operator()(rdf::LUInt64 lhs, rdf::LUInt32 rhs)const{return rdf::LUInt64{lhs.data/boost::numeric_cast<uint64_t>(rhs.data)};}
  RDFTTYPE operator()(rdf::LUInt64 lhs, rdf::LInt64  rhs)const{return rdf::LUInt64{lhs.data/boost::numeric_cast<uint64_t>(rhs.data)};}
  RDFTTYPE operator()(rdf::LUInt64 lhs, rdf::LUInt64 rhs)const{return rdf::LUInt64{lhs.data/boost::numeric_cast<uint64_t>(rhs.data)};}
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
  template<class T> RDFTTYPE operator()(T lhs) const {if(br==nullptr) return rdf::Null(); else RETE_EXCEPTION("Invalid arguments for Abs: ("<<lhs<<")");};

  RDFTTYPE operator()(rdf::LInt32  lhs)const{return rdf::LInt32{abs(lhs.data)};}
  RDFTTYPE operator()(rdf::LUInt32 lhs)const{return lhs;}
  RDFTTYPE operator()(rdf::LInt64  lhs)const{return rdf::LInt64{abs(lhs.data)};}
  RDFTTYPE operator()(rdf::LUInt64 lhs)const{return lhs;}
  RDFTTYPE operator()(rdf::LDouble lhs)const{return rdf::LDouble{fabs(lhs.data)};}

  ReteSession * rs;
  BetaRow const* br;
};

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
  // Determine which mode the operator is to be used
  auto * rdf_session_p = rs->rdf_session();
  auto const* jr = rdf_session_p->rmgr()->jets();
  auto p_filter  = rdf_session_p->get_object(rhs, jr->jets__value_property);
  if( p_filter == nullptr) {
    // Mode is min/max of a multi value property
    p_filter = rhs;
  }
  if(not p_filter) {
    return 0;
  }

  auto cb = create_rete_callback_for_visitors(rs, vertex, p_filter);
  rdf_session_p->asserted_graph()->register_callback(cb);
  rdf_session_p->inferred_graph()->register_callback(cb);
  return 0;
}

struct MaxOfVisitor: public boost::static_visitor<RDFTTYPE>
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
struct MinOfVisitor: public boost::static_visitor<RDFTTYPE>
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
struct SortedHeadVisitor: public boost::static_visitor<RDFTTYPE>
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

    auto * rdf_session_p = rs->rdf_session();
    auto const* jr = rdf_session_p->rmgr()->jets();
    auto p_filter  = rdf_session_p->get_object(get_resource(rs, std::forward<ExprBase::ExprDataType>(rhs)), 
      jr->jets__value_property);
    if(not p_filter) {
      return 0;
    }

    auto cb = create_rete_callback_for_visitors(rs, vertex, p_filter);
    rdf_session_p->asserted_graph()->register_callback(cb);
    rdf_session_p->inferred_graph()->register_callback(cb);
    return 0;
  }

  ReteSession * rs;
  BetaRow const* br;
};

// SUM VALUES ==========================================================================

struct ApplySumValuesVisitor
{
  ApplySumValuesVisitor(ReteSession * rs): rs(rs) {}

  // Apply the visitor to find:
  //  - case objp is nullptr: the sum of ?v in (s, datap, ?v)
  //  - case datap is not nullptr: the sum of ?v in (s, objp, ?o).(?o, datap, ?v)
  RDFTTYPE operator()(rdf::r_index s, rdf::r_index objp, rdf::r_index datap)const
  {
    AddVisitor visitor;
    RDFTTYPE res;
    if(not datap) {
      RETE_EXCEPTION("Invalid arguments for ApplySumValuesVisitor, cannot have null datap");
    }
    if(objp == nullptr) {
      // make sum ?v: (s, datap, ?v)
      auto itor = rs->rdf_session()->find(s, datap);
      bool is_first = true;
      while(!itor.is_end()) {
        auto val = itor.get_object();
        if(not val) {
          RETE_EXCEPTION("BUG in ApplySumValuesVisitor: unexpected null value");
        }
        if(is_first) {
          res = *val;
          is_first = false;
        } else {
          if(val->which() != rdf::rdf_null_t) {
            res = boost::apply_visitor(visitor, res, *val);
          }
        }
        itor.next();
      }
    } else {
      // make sum ?v: (s, objp, ?o).(?o, datap, ?v)
      auto itor = rs->rdf_session()->find(s, objp);
      bool is_first = true;
      while(!itor.is_end()) {
        auto val = rs->rdf_session()->get_object(itor.get_object(), datap);
        if(val) {
          if(is_first) {
            res = *val;
            is_first = false;
          } else {
            if(val->which() != rdf::rdf_null_t) {
              res = boost::apply_visitor(visitor, res, *val);
            }
          }
        }
        itor.next();
      }
    }
    return res;
  }

  ReteSession * rs;
};

// SumValuesVisitor * Add truth maintenance
// --------------------------------------------------------------------------------------
struct SumValuesVisitor: public boost::static_visitor<RDFTTYPE>
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

    auto * rdf_session_p = rs->rdf_session();
    auto const* jr = rdf_session_p->rmgr()->jets();
    auto p_filter  = rdf_session_p->get_object(get_resource(rs, std::forward<ExprBase::ExprDataType>(rhs)), 
      jr->jets__value_property);
    if(not p_filter) {
      return 0;
    }

    auto cb = create_rete_callback_for_visitors(rs, vertex, p_filter);
    rdf_session_p->asserted_graph()->register_callback(cb);
    rdf_session_p->inferred_graph()->register_callback(cb);
    return 0;
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
  RDFTTYPE operator()(rdf::LDouble   lhs)const{return rdf::LString{std::to_string(lhs.data)};}
  RDFTTYPE operator()(rdf::LString   lhs)const{return lhs;}
  RDFTTYPE operator()(rdf::LDate     lhs)const{return rdf::LString{rdf::to_string(lhs.data)};}
  RDFTTYPE operator()(rdf::LDatetime lhs)const{return rdf::LString{rdf::to_string(lhs.data)};}
  RDFTTYPE operator()(rdf::NamedResource lhs)const{return rdf::LString{lhs.name};}
  ReteSession * rs;
  BetaRow const* br;
};



} // namespace jets::rete
#endif // JETS_RETE_EXPR_OP_ARITHMETICS_H
