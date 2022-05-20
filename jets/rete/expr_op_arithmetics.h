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

using RDFTTYPE = rdf::RdfAstType;

// AddVisitor
// --------------------------------------------------------------------------------------
struct AddVisitor: public boost::static_visitor<RDFTTYPE>
{
  AddVisitor(ReteSession * rs, BetaRow const* br): rs(rs), br(br) {}
  AddVisitor(): rs(nullptr), br(nullptr) {}
  template<class T, class U> RDFTTYPE operator()(T lhs, U rhs) const {RETE_EXCEPTION("Invalid arguments for ADD: ("<<lhs<<", "<<rhs<<")");};

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
struct SubsVisitor: public boost::static_visitor<RDFTTYPE>
{
  SubsVisitor(ReteSession * rs, BetaRow const* br): rs(rs), br(br) {}
  template<class T, class U> RDFTTYPE operator()(T lhs, U rhs) const {RETE_EXCEPTION("Invalid arguments for substraction: ("<<lhs<<", "<<rhs<<")");};

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
struct MultVisitor: public boost::static_visitor<RDFTTYPE>
{
  MultVisitor(ReteSession * rs, BetaRow const* br): rs(rs), br(br) {}
  template<class T, class U> RDFTTYPE operator()(T lhs, U rhs) const {RETE_EXCEPTION("Invalid arguments for Multiplication: ("<<lhs<<", "<<rhs<<")");};

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
struct DivVisitor: public boost::static_visitor<RDFTTYPE>
{
  DivVisitor(ReteSession * rs, BetaRow const* br): rs(rs), br(br) {}
  template<class T, class U> RDFTTYPE operator()(T lhs, U rhs) const {RETE_EXCEPTION("Invalid arguments for Division: ("<<lhs<<", "<<rhs<<")");};

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
struct AbsVisitor: public boost::static_visitor<RDFTTYPE>
{
  AbsVisitor(ReteSession * rs, BetaRow const* br): rs(rs), br(br) {}
  template<class T> RDFTTYPE operator()(T lhs) const {RETE_EXCEPTION("Invalid arguments for Abs: ("<<lhs<<")");};

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
  ApplyMinMaxVisitor(ReteSession * rs, bool is_min): rs(rs), is_min(is_min) {}

  // Apply the visitor to fins the min/max value for the triple (s, objp, ?o).(?o, datap, ?v)
  // if datap == null, then return ?o otherwise return ?v
  RDFTTYPE operator()(rdf::r_index s, rdf::r_index objp, rdf::r_index datap=nullptr)const
  {
    GtVisitor visitor;
    auto itor = rs->rdf_session()->find(s, objp);
    bool is_first = true;
    RDFTTYPE res;
    rdf::r_index lhs, rhs;
    while(!itor.is_end()) {
      auto obj = itor.get_object();
      auto val = obj;
      if(datap) {
        val = rs->rdf_session()->get_object(obj, datap);
      }
      if(is_first) {
        res = *val;
        is_first = false;
      } else {
        // visitor is for: lhs > rhs
        // to have min, do if res > val, then res = val 
        // to have max, do if val > res, then res = val 
        if(this->is_min) {
          lhs = &res;
          rhs = val;
        } else {
          lhs = val;
          rhs = &res;
        }
        if(rdf::to_bool(boost::apply_visitor(visitor, *lhs, *rhs))) res = *val; 
      }
      itor.next();
    }
    return res;
  }

  ReteSession * rs;
  bool is_min;
};

// MaxOfVisitor * Add truth maintenance
// --------------------------------------------------------------------------------------
struct MaxOfVisitor: public boost::static_visitor<RDFTTYPE>
{
  MaxOfVisitor(ReteSession * rs, BetaRow const* br): rs(rs), br(br) {}
  template<class T, class U> RDFTTYPE operator()(T lhs, U rhs) const {RETE_EXCEPTION("Invalid arguments for max_of: ("<<lhs<<", "<<rhs<<")");};

  RDFTTYPE operator()(rdf::NamedResource lhs, rdf::NamedResource rhs)const
  {
    auto * sess = this->rs->rdf_session();
    auto pr = get_resources(sess->rmgr(), std::move(lhs.name), std::move(rhs.name));
    if(not pr.first or not pr.second) return rdf::Null();
    ApplyMinMaxVisitor av(this->rs, false);
    return av(pr.first, pr.second);
  }

  ReteSession * rs;
  BetaRow const* br;
};

// MinOfVisitor * Add truth maintenance
// --------------------------------------------------------------------------------------
struct MinOfVisitor: public boost::static_visitor<RDFTTYPE>
{
  MinOfVisitor(ReteSession * rs, BetaRow const* br): rs(rs), br(br) {}
  template<class T, class U> RDFTTYPE operator()(T lhs, U rhs) const {RETE_EXCEPTION("Invalid arguments for min_of: ("<<lhs<<", "<<rhs<<")");};

  RDFTTYPE operator()(rdf::NamedResource lhs, rdf::NamedResource rhs)const
  {
    auto * sess = this->rs->rdf_session();
    auto pr = get_resources(sess->rmgr(), std::move(lhs.name), std::move(rhs.name));
    if(not pr.first or not pr.second) return rdf::Null();
    ApplyMinMaxVisitor av(this->rs, true);
    return av(pr.first, pr.second);
  }

  ReteSession * rs;
  BetaRow const* br;
};

// SortedHeadVisitor * Add truth maintenance
// --------------------------------------------------------------------------------------
struct SortedHeadVisitor: public boost::static_visitor<RDFTTYPE>
{
  SortedHeadVisitor(ReteSession * rs, BetaRow const* br): rs(rs), br(br) {}
  template<class T, class U> RDFTTYPE operator()(T lhs, U rhs) const {RETE_EXCEPTION("Invalid arguments for sorted_head: ("<<lhs<<", "<<rhs<<")");};

  RDFTTYPE operator()(rdf::NamedResource lhs, rdf::NamedResource rhs)const
  {
    auto * sess = this->rs->rdf_session();
    auto pr = get_resources(sess->rmgr(), std::move(lhs.name), std::move(rhs.name));
    if(not pr.first or not pr.second) return rdf::Null();
    ApplyMinMaxVisitor av(this->rs, false);
    auto const* jr = sess->rmgr()->jets();
    auto objp = sess->get_object(pr.second, jr->jets__entity_property);
    auto datap = sess->get_object(pr.second, jr->jets__value_property);
    return av(pr.first, objp, datap);
  }

  ReteSession * rs;
  BetaRow const* br;
};

// SUM VALUES ==========================================================================

struct ApplySumValuesVisitor
{
  ApplySumValuesVisitor(ReteSession * rs): rs(rs) {}

  // Apply the visitor to fins the min/max value for the triple (s, objp, ?o).(?o, datap, ?v)
  // if datap == null, then return ?o otherwise return ?v
  RDFTTYPE operator()(rdf::r_index s, rdf::r_index objp, rdf::r_index datap=nullptr)const
  {
    AddVisitor visitor;
    auto itor = rs->rdf_session()->find(s, objp);
    bool is_first = true;
    RDFTTYPE res;
    while(!itor.is_end()) {
      auto obj = itor.get_object();
      auto val = obj;
      if(datap) {
        val = rs->rdf_session()->get_object(obj, datap);
      }
      if(is_first) {
        res = *val;
        is_first = false;
      } else {
        // visitor is for: lhs > rhs
        // to have min, do if res > val, then res = val 
        // to have max, do if val > res, then res = val 
        res = boost::apply_visitor(visitor, res, *val);
      }
      itor.next();
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
  template<class T, class U> RDFTTYPE operator()(T lhs, U rhs) const {RETE_EXCEPTION("Invalid arguments for sum_values: ("<<lhs<<", "<<rhs<<")");};

  RDFTTYPE operator()(rdf::NamedResource lhs, rdf::NamedResource rhs)const
  {
    auto * sess = this->rs->rdf_session();
    auto pr = get_resources(sess->rmgr(), std::move(lhs.name), std::move(rhs.name));
    if(not pr.first or not pr.second) return rdf::Null();
    ApplySumValuesVisitor av(this->rs);
    auto const* jr = sess->rmgr()->jets();
    auto objp = sess->get_object(pr.second, jr->jets__entity_property);
    auto datap = sess->get_object(pr.second, jr->jets__value_property);
    return av(pr.first, objp, datap);
  }

  ReteSession * rs;
  BetaRow const* br;
};

// ToIntVisitor
// --------------------------------------------------------------------------------------
struct ToIntVisitor: public boost::static_visitor<RDFTTYPE>
{
  ToIntVisitor(ReteSession * rs, BetaRow const* br): rs(rs), br(br) {}
  template<class T> RDFTTYPE operator()(T lhs) const {RETE_EXCEPTION("Invalid arguments for to_int: ("<<lhs<<")");};

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
struct ToDoubleVisitor: public boost::static_visitor<RDFTTYPE>
{
  ToDoubleVisitor(ReteSession * rs, BetaRow const* br): rs(rs), br(br) {}
  template<class T> RDFTTYPE operator()(T lhs) const {RETE_EXCEPTION("Invalid arguments for to_double: ("<<lhs<<")");};

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



} // namespace jets::rete
#endif // JETS_RETE_EXPR_OP_ARITHMETICS_H
