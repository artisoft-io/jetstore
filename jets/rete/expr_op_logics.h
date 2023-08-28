#ifndef JETS_RETE_EXPR_OP_LOGICS_H
#define JETS_RETE_EXPR_OP_LOGICS_H

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

// This file contains basic operator used in rule expression 
// see ExprUnaryOp and ExprBinaryOp classes.
namespace jets::rete {

using RDFTTYPE = rdf::RdfAstType;

// EqVisitor
// --------------------------------------------------------------------------------------
struct EqVisitor: public boost::static_visitor<RDFTTYPE>, public NoCallbackNeeded
{
  // Fully expanded example to serve as template
  EqVisitor(ReteSession * rs, BetaRow const* br): rs(rs), br(br) {}
  EqVisitor(): rs(nullptr), br(nullptr) {} // for use by other operators
  RDFTTYPE operator()(rdf::RDFNull       lhs, rdf::RDFNull       rhs)const{return rdf::True();}
  RDFTTYPE operator()(rdf::RDFNull       lhs, rdf::BlankNode     rhs)const{return rdf::False();}
  RDFTTYPE operator()(rdf::RDFNull       lhs, rdf::NamedResource rhs)const{return rdf::False();}
  RDFTTYPE operator()(rdf::RDFNull       lhs, rdf::LInt32        rhs)const{return rdf::False();}
  RDFTTYPE operator()(rdf::RDFNull       lhs, rdf::LUInt32       rhs)const{return rdf::False();}
  RDFTTYPE operator()(rdf::RDFNull       lhs, rdf::LInt64        rhs)const{return rdf::False();}
  RDFTTYPE operator()(rdf::RDFNull       lhs, rdf::LUInt64       rhs)const{return rdf::False();}
  RDFTTYPE operator()(rdf::RDFNull       lhs, rdf::LDouble       rhs)const{return rdf::False();}
  RDFTTYPE operator()(rdf::RDFNull       lhs, rdf::LString       rhs)const{return rdf::False();}

  RDFTTYPE operator()(rdf::BlankNode     lhs, rdf::RDFNull       rhs)const{return rdf::False();}
  RDFTTYPE operator()(rdf::BlankNode     lhs, rdf::BlankNode     rhs)const{return rdf::LInt32{lhs.key == rhs.key};}
  RDFTTYPE operator()(rdf::BlankNode     lhs, rdf::NamedResource rhs)const{return rdf::False();}
  RDFTTYPE operator()(rdf::BlankNode     lhs, rdf::LInt32        rhs)const{return rdf::False();}
  RDFTTYPE operator()(rdf::BlankNode     lhs, rdf::LUInt32       rhs)const{return rdf::False();}
  RDFTTYPE operator()(rdf::BlankNode     lhs, rdf::LInt64        rhs)const{return rdf::False();}
  RDFTTYPE operator()(rdf::BlankNode     lhs, rdf::LUInt64       rhs)const{return rdf::False();}
  RDFTTYPE operator()(rdf::BlankNode     lhs, rdf::LDouble       rhs)const{return rdf::False();}
  RDFTTYPE operator()(rdf::BlankNode     lhs, rdf::LString       rhs)const{return rdf::False();}

  RDFTTYPE operator()(rdf::NamedResource lhs, rdf::RDFNull       rhs)const{return rdf::False();}
  RDFTTYPE operator()(rdf::NamedResource lhs, rdf::BlankNode     rhs)const{return rdf::False();}
  RDFTTYPE operator()(rdf::NamedResource lhs, rdf::NamedResource rhs)const{return rdf::LInt32{lhs.name == rhs.name};}
  RDFTTYPE operator()(rdf::NamedResource lhs, rdf::LInt32        rhs)const{return rdf::False();}
  RDFTTYPE operator()(rdf::NamedResource lhs, rdf::LUInt32       rhs)const{return rdf::False();}
  RDFTTYPE operator()(rdf::NamedResource lhs, rdf::LInt64        rhs)const{return rdf::False();}
  RDFTTYPE operator()(rdf::NamedResource lhs, rdf::LUInt64       rhs)const{return rdf::False();}
  RDFTTYPE operator()(rdf::NamedResource lhs, rdf::LDouble       rhs)const{return rdf::False();}
  RDFTTYPE operator()(rdf::NamedResource lhs, rdf::LString       rhs)const{return rdf::False();}

  RDFTTYPE operator()(rdf::LInt32        lhs, rdf::RDFNull       rhs)const{return rdf::False();}
  RDFTTYPE operator()(rdf::LInt32        lhs, rdf::BlankNode     rhs)const{return rdf::False();}
  RDFTTYPE operator()(rdf::LInt32        lhs, rdf::NamedResource rhs)const{return rdf::False();}
  RDFTTYPE operator()(rdf::LInt32        lhs, rdf::LInt32        rhs)const{return rdf::LInt32{std::cmp_equal(lhs.data, rhs.data)};}
  RDFTTYPE operator()(rdf::LInt32        lhs, rdf::LUInt32       rhs)const{return rdf::LInt32{std::cmp_equal(lhs.data, rhs.data)};}
  RDFTTYPE operator()(rdf::LInt32        lhs, rdf::LInt64        rhs)const{return rdf::LInt32{std::cmp_equal(lhs.data, rhs.data)};}
  RDFTTYPE operator()(rdf::LInt32        lhs, rdf::LUInt64       rhs)const{return rdf::LInt32{std::cmp_equal(lhs.data, rhs.data)};}
  RDFTTYPE operator()(rdf::LInt32        lhs, rdf::LDouble       rhs)const{return rdf::LInt32{lhs.data == boost::numeric_cast<int32_t>(rhs.data)};}
  RDFTTYPE operator()(rdf::LInt32        lhs, rdf::LString       rhs)const{return rdf::False();}

  RDFTTYPE operator()(rdf::LUInt32       lhs, rdf::RDFNull       rhs)const{return rdf::False();}
  RDFTTYPE operator()(rdf::LUInt32       lhs, rdf::BlankNode     rhs)const{return rdf::False();}
  RDFTTYPE operator()(rdf::LUInt32       lhs, rdf::NamedResource rhs)const{return rdf::False();}
  RDFTTYPE operator()(rdf::LUInt32       lhs, rdf::LInt32        rhs)const{return rdf::LInt32{std::cmp_equal(lhs.data, rhs.data)};}
  RDFTTYPE operator()(rdf::LUInt32       lhs, rdf::LUInt32       rhs)const{return rdf::LInt32{std::cmp_equal(lhs.data, rhs.data)};}
  RDFTTYPE operator()(rdf::LUInt32       lhs, rdf::LInt64        rhs)const{return rdf::LInt32{std::cmp_equal(lhs.data, rhs.data)};}
  RDFTTYPE operator()(rdf::LUInt32       lhs, rdf::LUInt64       rhs)const{return rdf::LInt32{std::cmp_equal(lhs.data, rhs.data)};}
  RDFTTYPE operator()(rdf::LUInt32       lhs, rdf::LDouble       rhs)const{return rdf::LInt32{lhs.data == boost::numeric_cast<uint32_t>(rhs.data)};}
  RDFTTYPE operator()(rdf::LUInt32       lhs, rdf::LString       rhs)const{return rdf::False();}

  RDFTTYPE operator()(rdf::LInt64        lhs, rdf::RDFNull       rhs)const{return rdf::False();}
  RDFTTYPE operator()(rdf::LInt64        lhs, rdf::BlankNode     rhs)const{return rdf::False();}
  RDFTTYPE operator()(rdf::LInt64        lhs, rdf::NamedResource rhs)const{return rdf::False();}
  RDFTTYPE operator()(rdf::LInt64        lhs, rdf::LInt32        rhs)const{return rdf::LInt32{std::cmp_equal(lhs.data, rhs.data)};}
  RDFTTYPE operator()(rdf::LInt64        lhs, rdf::LUInt32       rhs)const{return rdf::LInt32{std::cmp_equal(lhs.data, rhs.data)};}
  RDFTTYPE operator()(rdf::LInt64        lhs, rdf::LInt64        rhs)const{return rdf::LInt32{std::cmp_equal(lhs.data, rhs.data)};}
  RDFTTYPE operator()(rdf::LInt64        lhs, rdf::LUInt64       rhs)const{return rdf::LInt32{std::cmp_equal(lhs.data, rhs.data)};}
  RDFTTYPE operator()(rdf::LInt64        lhs, rdf::LDouble       rhs)const{return rdf::LInt32{lhs.data == boost::numeric_cast<int64_t>(rhs.data)};}
  RDFTTYPE operator()(rdf::LInt64        lhs, rdf::LString       rhs)const{return rdf::False();}

  RDFTTYPE operator()(rdf::LUInt64       lhs, rdf::RDFNull       rhs)const{return rdf::False();}
  RDFTTYPE operator()(rdf::LUInt64       lhs, rdf::BlankNode     rhs)const{return rdf::False();}
  RDFTTYPE operator()(rdf::LUInt64       lhs, rdf::NamedResource rhs)const{return rdf::False();}
  RDFTTYPE operator()(rdf::LUInt64       lhs, rdf::LInt32        rhs)const{return rdf::LInt32{std::cmp_equal(lhs.data, rhs.data)};}
  RDFTTYPE operator()(rdf::LUInt64       lhs, rdf::LUInt32       rhs)const{return rdf::LInt32{std::cmp_equal(lhs.data, rhs.data)};}
  RDFTTYPE operator()(rdf::LUInt64       lhs, rdf::LInt64        rhs)const{return rdf::LInt32{std::cmp_equal(lhs.data, rhs.data)};}
  RDFTTYPE operator()(rdf::LUInt64       lhs, rdf::LUInt64       rhs)const{return rdf::LInt32{std::cmp_equal(lhs.data, rhs.data)};}
  RDFTTYPE operator()(rdf::LUInt64       lhs, rdf::LDouble       rhs)const{return rdf::LInt32{lhs.data == boost::numeric_cast<uint32_t>(rhs.data)};}
  RDFTTYPE operator()(rdf::LUInt64       lhs, rdf::LString       rhs)const{return rdf::False();}

  RDFTTYPE operator()(rdf::LDouble       lhs, rdf::RDFNull       rhs)const{return rdf::False();}
  RDFTTYPE operator()(rdf::LDouble       lhs, rdf::BlankNode     rhs)const{return rdf::False();}
  RDFTTYPE operator()(rdf::LDouble       lhs, rdf::NamedResource rhs)const{return rdf::False();}
  RDFTTYPE operator()(rdf::LDouble       lhs, rdf::LInt32        rhs)const{return rdf::LInt32{lhs.data == boost::numeric_cast<int32_t>(rhs.data)};}
  RDFTTYPE operator()(rdf::LDouble       lhs, rdf::LUInt32       rhs)const{return rdf::LInt32{lhs.data == boost::numeric_cast<uint32_t>(rhs.data)};}
  RDFTTYPE operator()(rdf::LDouble       lhs, rdf::LInt64        rhs)const{return rdf::LInt32{lhs.data == boost::numeric_cast<int64_t>(rhs.data)};}
  RDFTTYPE operator()(rdf::LDouble       lhs, rdf::LUInt64       rhs)const{return rdf::LInt32{lhs.data == boost::numeric_cast<uint64_t>(rhs.data)};}
  RDFTTYPE operator()(rdf::LDouble       lhs, rdf::LDouble       rhs)const{return rdf::LInt32{is_eq(lhs, rhs)};}
  RDFTTYPE operator()(rdf::LDouble       lhs, rdf::LString       rhs)const{return rdf::False();}

  RDFTTYPE operator()(rdf::LString       lhs, rdf::RDFNull       rhs)const{return rdf::False();}
  RDFTTYPE operator()(rdf::LString       lhs, rdf::BlankNode     rhs)const{return rdf::False();}
  RDFTTYPE operator()(rdf::LString       lhs, rdf::NamedResource rhs)const{return rdf::False();}
  RDFTTYPE operator()(rdf::LString       lhs, rdf::LInt32        rhs)const{return rdf::False();}
  RDFTTYPE operator()(rdf::LString       lhs, rdf::LUInt32       rhs)const{return rdf::False();}
  RDFTTYPE operator()(rdf::LString       lhs, rdf::LInt64        rhs)const{return rdf::False();}
  RDFTTYPE operator()(rdf::LString       lhs, rdf::LUInt64       rhs)const{return rdf::False();}
  RDFTTYPE operator()(rdf::LString       lhs, rdf::LDouble       rhs)const{return rdf::False();}
  RDFTTYPE operator()(rdf::LString       lhs, rdf::LString       rhs)const{return rdf::LInt32{lhs.data == rhs.data};}

  // -------------------------------------------------------------------------------------------
  RDFTTYPE operator()(rdf::RDFNull       lhs, rdf::LDate       rhs)const{return rdf::False();}
  RDFTTYPE operator()(rdf::BlankNode     lhs, rdf::LDate       rhs)const{return rdf::False();}
  RDFTTYPE operator()(rdf::NamedResource lhs, rdf::LDate       rhs)const{return rdf::False();}
  RDFTTYPE operator()(rdf::LInt32        lhs, rdf::LDate       rhs)const{return rdf::False();}
  RDFTTYPE operator()(rdf::LUInt32       lhs, rdf::LDate       rhs)const{return rdf::False();}
  RDFTTYPE operator()(rdf::LInt64        lhs, rdf::LDate       rhs)const{return rdf::False();}
  RDFTTYPE operator()(rdf::LUInt64       lhs, rdf::LDate       rhs)const{return rdf::False();}
  RDFTTYPE operator()(rdf::LDouble       lhs, rdf::LDate       rhs)const{return rdf::False();}
  RDFTTYPE operator()(rdf::LString       lhs, rdf::LDate       rhs)const{return rdf::False();}
  // -------------------------------------------------------------------------------------------
  RDFTTYPE operator()(rdf::RDFNull       lhs, rdf::LDatetime   rhs)const{return rdf::False();}
  RDFTTYPE operator()(rdf::BlankNode     lhs, rdf::LDatetime   rhs)const{return rdf::False();}
  RDFTTYPE operator()(rdf::NamedResource lhs, rdf::LDatetime   rhs)const{return rdf::False();}
  RDFTTYPE operator()(rdf::LInt32        lhs, rdf::LDatetime   rhs)const{return rdf::False();}
  RDFTTYPE operator()(rdf::LUInt32       lhs, rdf::LDatetime   rhs)const{return rdf::False();}
  RDFTTYPE operator()(rdf::LInt64        lhs, rdf::LDatetime   rhs)const{return rdf::False();}
  RDFTTYPE operator()(rdf::LUInt64       lhs, rdf::LDatetime   rhs)const{return rdf::False();}
  RDFTTYPE operator()(rdf::LDouble       lhs, rdf::LDatetime   rhs)const{return rdf::False();}
  RDFTTYPE operator()(rdf::LString       lhs, rdf::LDatetime   rhs)const{return rdf::False();}
  // -------------------------------------------------------------------------------------------
  RDFTTYPE operator()(rdf::LDate       lhs, rdf::RDFNull       rhs)const{return rdf::False();}
  RDFTTYPE operator()(rdf::LDate       lhs, rdf::BlankNode     rhs)const{return rdf::False();}
  RDFTTYPE operator()(rdf::LDate       lhs, rdf::NamedResource rhs)const{return rdf::False();}
  RDFTTYPE operator()(rdf::LDate       lhs, rdf::LInt32        rhs)const{return rdf::False();}
  RDFTTYPE operator()(rdf::LDate       lhs, rdf::LUInt32       rhs)const{return rdf::False();}
  RDFTTYPE operator()(rdf::LDate       lhs, rdf::LInt64        rhs)const{return rdf::False();}
  RDFTTYPE operator()(rdf::LDate       lhs, rdf::LUInt64       rhs)const{return rdf::False();}
  RDFTTYPE operator()(rdf::LDate       lhs, rdf::LDouble       rhs)const{return rdf::False();}
  RDFTTYPE operator()(rdf::LDate       lhs, rdf::LString       rhs)const{return rdf::False();}
  RDFTTYPE operator()(rdf::LDate       lhs, rdf::LDate         rhs)const{return rdf::LInt32{lhs.data == rhs.data};}
  RDFTTYPE operator()(rdf::LDate       lhs, rdf::LDatetime     rhs)const{return rdf::LInt32{rdf::to_datetime(std::move(lhs.data)) == rhs.data};}
  
  RDFTTYPE operator()(rdf::LDatetime   lhs, rdf::RDFNull       rhs)const{return rdf::False();}
  RDFTTYPE operator()(rdf::LDatetime   lhs, rdf::BlankNode     rhs)const{return rdf::False();}
  RDFTTYPE operator()(rdf::LDatetime   lhs, rdf::NamedResource rhs)const{return rdf::False();}
  RDFTTYPE operator()(rdf::LDatetime   lhs, rdf::LInt32        rhs)const{return rdf::False();}
  RDFTTYPE operator()(rdf::LDatetime   lhs, rdf::LUInt32       rhs)const{return rdf::False();}
  RDFTTYPE operator()(rdf::LDatetime   lhs, rdf::LInt64        rhs)const{return rdf::False();}
  RDFTTYPE operator()(rdf::LDatetime   lhs, rdf::LUInt64       rhs)const{return rdf::False();}
  RDFTTYPE operator()(rdf::LDatetime   lhs, rdf::LDouble       rhs)const{return rdf::False();}
  RDFTTYPE operator()(rdf::LDatetime   lhs, rdf::LString       rhs)const{return rdf::False();}
  RDFTTYPE operator()(rdf::LDatetime   lhs, rdf::LDate         rhs)const{return rdf::LInt32{lhs.data == rdf::to_datetime(std::move(rhs.data))};}
  RDFTTYPE operator()(rdf::LDatetime   lhs, rdf::LDatetime     rhs)const{return rdf::LInt32{lhs.data == rhs.data};}
  
  ReteSession * rs;
  BetaRow const* br;
};

// NeVisitor
// --------------------------------------------------------------------------------------
struct NeVisitor: public boost::static_visitor<RDFTTYPE>, public NoCallbackNeeded
{
  NeVisitor(ReteSession * rs, BetaRow const* br): rs(rs), br(br) {}
  NeVisitor(): rs(nullptr), br(nullptr) {} // for use by other operators
  template<class T, class U> RDFTTYPE operator()(T lhs, U rhs) const {return rdf::True();};
  RDFTTYPE operator()(rdf::RDFNull       lhs, rdf::RDFNull       rhs)const{return rdf::False();}
  RDFTTYPE operator()(rdf::BlankNode     lhs, rdf::BlankNode     rhs)const{return rdf::LInt32{lhs.key != rhs.key};}
  RDFTTYPE operator()(rdf::NamedResource lhs, rdf::NamedResource rhs)const{return rdf::LInt32{lhs.name != rhs.name};}

  RDFTTYPE operator()(rdf::LInt32        lhs, rdf::LInt32        rhs)const{return rdf::LInt32{std::cmp_not_equal(lhs.data, rhs.data)};}
  RDFTTYPE operator()(rdf::LInt32        lhs, rdf::LUInt32       rhs)const{return rdf::LInt32{std::cmp_not_equal(lhs.data, rhs.data)};}
  RDFTTYPE operator()(rdf::LInt32        lhs, rdf::LInt64        rhs)const{return rdf::LInt32{std::cmp_not_equal(lhs.data, rhs.data)};}
  RDFTTYPE operator()(rdf::LInt32        lhs, rdf::LUInt64       rhs)const{return rdf::LInt32{std::cmp_not_equal(lhs.data, rhs.data)};}
  RDFTTYPE operator()(rdf::LInt32        lhs, rdf::LDouble       rhs)const{return rdf::LInt32{boost::numeric_cast<double>(lhs.data) != rhs.data};}

  RDFTTYPE operator()(rdf::LUInt32       lhs, rdf::LInt32        rhs)const{return rdf::LInt32{std::cmp_not_equal(lhs.data, rhs.data)};}
  RDFTTYPE operator()(rdf::LUInt32       lhs, rdf::LUInt32       rhs)const{return rdf::LInt32{std::cmp_not_equal(lhs.data, rhs.data)};}
  RDFTTYPE operator()(rdf::LUInt32       lhs, rdf::LInt64        rhs)const{return rdf::LInt32{std::cmp_not_equal(lhs.data, rhs.data)};}
  RDFTTYPE operator()(rdf::LUInt32       lhs, rdf::LUInt64       rhs)const{return rdf::LInt32{std::cmp_not_equal(lhs.data, rhs.data)};}
  RDFTTYPE operator()(rdf::LUInt32       lhs, rdf::LDouble       rhs)const{return rdf::LInt32{boost::numeric_cast<double>(lhs.data) != rhs.data};}

  RDFTTYPE operator()(rdf::LInt64        lhs, rdf::LInt32        rhs)const{return rdf::LInt32{std::cmp_not_equal(lhs.data, rhs.data)};}
  RDFTTYPE operator()(rdf::LInt64        lhs, rdf::LUInt32       rhs)const{return rdf::LInt32{std::cmp_not_equal(lhs.data, rhs.data)};}
  RDFTTYPE operator()(rdf::LInt64        lhs, rdf::LInt64        rhs)const{return rdf::LInt32{std::cmp_not_equal(lhs.data, rhs.data)};}
  RDFTTYPE operator()(rdf::LInt64        lhs, rdf::LUInt64       rhs)const{return rdf::LInt32{std::cmp_not_equal(lhs.data, rhs.data)};}
  RDFTTYPE operator()(rdf::LInt64        lhs, rdf::LDouble       rhs)const{return rdf::LInt32{boost::numeric_cast<double>(lhs.data) != rhs.data};}

  RDFTTYPE operator()(rdf::LUInt64       lhs, rdf::LInt32        rhs)const{return rdf::LInt32{std::cmp_not_equal(lhs.data, rhs.data)};}
  RDFTTYPE operator()(rdf::LUInt64       lhs, rdf::LUInt32       rhs)const{return rdf::LInt32{std::cmp_not_equal(lhs.data, rhs.data)};}
  RDFTTYPE operator()(rdf::LUInt64       lhs, rdf::LInt64        rhs)const{return rdf::LInt32{std::cmp_not_equal(lhs.data, rhs.data)};}
  RDFTTYPE operator()(rdf::LUInt64       lhs, rdf::LUInt64       rhs)const{return rdf::LInt32{std::cmp_not_equal(lhs.data, rhs.data)};}
  RDFTTYPE operator()(rdf::LUInt64       lhs, rdf::LDouble       rhs)const{return rdf::LInt32{boost::numeric_cast<double>(lhs.data) != rhs.data};}

  RDFTTYPE operator()(rdf::LDouble       lhs, rdf::LInt32        rhs)const{return rdf::LInt32{lhs.data != boost::numeric_cast<double>(rhs.data)};}
  RDFTTYPE operator()(rdf::LDouble       lhs, rdf::LUInt32       rhs)const{return rdf::LInt32{lhs.data != boost::numeric_cast<double>(rhs.data)};}
  RDFTTYPE operator()(rdf::LDouble       lhs, rdf::LInt64        rhs)const{return rdf::LInt32{lhs.data != boost::numeric_cast<double>(rhs.data)};}
  RDFTTYPE operator()(rdf::LDouble       lhs, rdf::LUInt64       rhs)const{return rdf::LInt32{lhs.data != boost::numeric_cast<double>(rhs.data)};}
  RDFTTYPE operator()(rdf::LDouble       lhs, rdf::LDouble       rhs)const{return rdf::LInt32{!is_eq(lhs, rhs)};}

  RDFTTYPE operator()(rdf::LString       lhs, rdf::LString       rhs)const{return rdf::LInt32{lhs.data != rhs.data};}

  // -------------------------------------------------------------------------------------------
  RDFTTYPE operator()(rdf::LDate       lhs, rdf::LDate         rhs)const{return rdf::LInt32{lhs.data != rhs.data};}
  RDFTTYPE operator()(rdf::LDate       lhs, rdf::LDatetime     rhs)const{return rdf::LInt32{rdf::to_datetime(std::move(lhs.data)) != rhs.data};}  
  RDFTTYPE operator()(rdf::LDatetime   lhs, rdf::LDate         rhs)const{return rdf::LInt32{lhs.data != rdf::to_datetime(std::move(rhs.data))};}
  RDFTTYPE operator()(rdf::LDatetime   lhs, rdf::LDatetime     rhs)const{return rdf::LInt32{lhs.data != rhs.data};}
  
  ReteSession * rs;
  BetaRow const* br;
};

// LeVisitor
// --------------------------------------------------------------------------------------
struct LeVisitor: public boost::static_visitor<RDFTTYPE>, public NoCallbackNeeded
{
  LeVisitor(ReteSession * rs, BetaRow const* br): rs(rs), br(br) {}
  LeVisitor(): rs(nullptr), br(nullptr) {} // for use by other operators
  template<class T, class U> RDFTTYPE operator()(T lhs, U rhs) const {return rdf::False();};
  RDFTTYPE operator()(rdf::RDFNull       lhs, rdf::RDFNull       rhs)const{return rdf::True();}
  RDFTTYPE operator()(rdf::BlankNode     lhs, rdf::BlankNode     rhs)const{return rdf::LInt32{lhs.key <= rhs.key};}
  RDFTTYPE operator()(rdf::NamedResource lhs, rdf::NamedResource rhs)const{return rdf::LInt32{lhs.name <= rhs.name};}

  RDFTTYPE operator()(rdf::LInt32        lhs, rdf::LInt32        rhs)const{return rdf::LInt32{std::cmp_less_equal(lhs.data, rhs.data)};}
  RDFTTYPE operator()(rdf::LInt32        lhs, rdf::LUInt32       rhs)const{return rdf::LInt32{std::cmp_less_equal(lhs.data, rhs.data)};}
  RDFTTYPE operator()(rdf::LInt32        lhs, rdf::LInt64        rhs)const{return rdf::LInt32{std::cmp_less_equal(lhs.data, rhs.data)};}
  RDFTTYPE operator()(rdf::LInt32        lhs, rdf::LUInt64       rhs)const{return rdf::LInt32{std::cmp_less_equal(lhs.data, rhs.data)};}
  RDFTTYPE operator()(rdf::LInt32        lhs, rdf::LDouble       rhs)const{return rdf::LInt32{boost::numeric_cast<double>(lhs.data) <= rhs.data};}

  RDFTTYPE operator()(rdf::LUInt32       lhs, rdf::LInt32        rhs)const{return rdf::LInt32{std::cmp_less_equal(lhs.data, rhs.data)};}
  RDFTTYPE operator()(rdf::LUInt32       lhs, rdf::LUInt32       rhs)const{return rdf::LInt32{std::cmp_less_equal(lhs.data, rhs.data)};}
  RDFTTYPE operator()(rdf::LUInt32       lhs, rdf::LInt64        rhs)const{return rdf::LInt32{std::cmp_less_equal(lhs.data, rhs.data)};}
  RDFTTYPE operator()(rdf::LUInt32       lhs, rdf::LUInt64       rhs)const{return rdf::LInt32{std::cmp_less_equal(lhs.data, rhs.data)};}
  RDFTTYPE operator()(rdf::LUInt32       lhs, rdf::LDouble       rhs)const{return rdf::LInt32{boost::numeric_cast<double>(lhs.data) <= rhs.data};}

  RDFTTYPE operator()(rdf::LInt64        lhs, rdf::LInt32        rhs)const{return rdf::LInt32{std::cmp_less_equal(lhs.data, rhs.data)};}
  RDFTTYPE operator()(rdf::LInt64        lhs, rdf::LUInt32       rhs)const{return rdf::LInt32{std::cmp_less_equal(lhs.data, rhs.data)};}
  RDFTTYPE operator()(rdf::LInt64        lhs, rdf::LInt64        rhs)const{return rdf::LInt32{std::cmp_less_equal(lhs.data, rhs.data)};}
  RDFTTYPE operator()(rdf::LInt64        lhs, rdf::LUInt64       rhs)const{return rdf::LInt32{std::cmp_less_equal(lhs.data, rhs.data)};}
  RDFTTYPE operator()(rdf::LInt64        lhs, rdf::LDouble       rhs)const{return rdf::LInt32{boost::numeric_cast<double>(lhs.data) <= rhs.data};}

  RDFTTYPE operator()(rdf::LUInt64       lhs, rdf::LInt32        rhs)const{return rdf::LInt32{std::cmp_less_equal(lhs.data, rhs.data)};}
  RDFTTYPE operator()(rdf::LUInt64       lhs, rdf::LUInt32       rhs)const{return rdf::LInt32{std::cmp_less_equal(lhs.data, rhs.data)};}
  RDFTTYPE operator()(rdf::LUInt64       lhs, rdf::LInt64        rhs)const{return rdf::LInt32{std::cmp_less_equal(lhs.data, rhs.data)};}
  RDFTTYPE operator()(rdf::LUInt64       lhs, rdf::LUInt64       rhs)const{return rdf::LInt32{std::cmp_less_equal(lhs.data, rhs.data)};}
  RDFTTYPE operator()(rdf::LUInt64       lhs, rdf::LDouble       rhs)const{return rdf::LInt32{boost::numeric_cast<double>(lhs.data) <= rhs.data};}

  RDFTTYPE operator()(rdf::LDouble       lhs, rdf::LInt32        rhs)const{return rdf::LInt32{lhs.data <= boost::numeric_cast<double>(rhs.data)};}
  RDFTTYPE operator()(rdf::LDouble       lhs, rdf::LUInt32       rhs)const{return rdf::LInt32{lhs.data <= boost::numeric_cast<double>(rhs.data)};}
  RDFTTYPE operator()(rdf::LDouble       lhs, rdf::LInt64        rhs)const{return rdf::LInt32{lhs.data <= boost::numeric_cast<double>(rhs.data)};}
  RDFTTYPE operator()(rdf::LDouble       lhs, rdf::LUInt64       rhs)const{return rdf::LInt32{lhs.data <= boost::numeric_cast<double>(rhs.data)};}
  RDFTTYPE operator()(rdf::LDouble       lhs, rdf::LDouble       rhs)const{return rdf::LInt32{is_le(lhs, rhs)};}

  RDFTTYPE operator()(rdf::LString       lhs, rdf::LString       rhs)const{return rdf::LInt32{lhs.data <= rhs.data};}

  // -------------------------------------------------------------------------------------------
  RDFTTYPE operator()(rdf::LDate       lhs, rdf::LDate         rhs)const{return rdf::LInt32{lhs.data <= rhs.data};}
  RDFTTYPE operator()(rdf::LDate       lhs, rdf::LDatetime     rhs)const{return rdf::LInt32{rdf::to_datetime(std::move(lhs.data)) <= rhs.data};}  
  RDFTTYPE operator()(rdf::LDatetime   lhs, rdf::LDate         rhs)const{return rdf::LInt32{lhs.data <= rdf::to_datetime(std::move(rhs.data))};}
  RDFTTYPE operator()(rdf::LDatetime   lhs, rdf::LDatetime     rhs)const{return rdf::LInt32{lhs.data <= rhs.data};}

  ReteSession * rs;
  BetaRow const* br;
};

// LtVisitor
// --------------------------------------------------------------------------------------
struct LtVisitor: public boost::static_visitor<RDFTTYPE>, public NoCallbackNeeded
{
  LtVisitor(ReteSession * rs, BetaRow const* br): rs(rs), br(br) {}
  LtVisitor(): rs(nullptr), br(nullptr) {} // for use by other operators
  template<class T, class U> RDFTTYPE operator()(T lhs, U rhs) const {return rdf::False();};
  RDFTTYPE operator()(rdf::RDFNull       lhs, rdf::RDFNull       rhs)const{return rdf::False();}
  RDFTTYPE operator()(rdf::BlankNode     lhs, rdf::BlankNode     rhs)const{return rdf::LInt32{lhs.key < rhs.key};}
  RDFTTYPE operator()(rdf::NamedResource lhs, rdf::NamedResource rhs)const{return rdf::LInt32{lhs.name < rhs.name};}

  RDFTTYPE operator()(rdf::LInt32        lhs, rdf::LInt32        rhs)const{return rdf::LInt32{std::cmp_less(lhs.data, rhs.data)};}
  RDFTTYPE operator()(rdf::LInt32        lhs, rdf::LUInt32       rhs)const{return rdf::LInt32{std::cmp_less(lhs.data, rhs.data)};}
  RDFTTYPE operator()(rdf::LInt32        lhs, rdf::LInt64        rhs)const{return rdf::LInt32{std::cmp_less(lhs.data, rhs.data)};}
  RDFTTYPE operator()(rdf::LInt32        lhs, rdf::LUInt64       rhs)const{return rdf::LInt32{std::cmp_less(lhs.data, rhs.data)};}
  RDFTTYPE operator()(rdf::LInt32        lhs, rdf::LDouble       rhs)const{return rdf::LInt32{boost::numeric_cast<double>(lhs.data) < rhs.data};}

  RDFTTYPE operator()(rdf::LUInt32       lhs, rdf::LInt32        rhs)const{return rdf::LInt32{std::cmp_less(lhs.data, rhs.data)};}
  RDFTTYPE operator()(rdf::LUInt32       lhs, rdf::LUInt32       rhs)const{return rdf::LInt32{std::cmp_less(lhs.data, rhs.data)};}
  RDFTTYPE operator()(rdf::LUInt32       lhs, rdf::LInt64        rhs)const{return rdf::LInt32{std::cmp_less(lhs.data, rhs.data)};}
  RDFTTYPE operator()(rdf::LUInt32       lhs, rdf::LUInt64       rhs)const{return rdf::LInt32{std::cmp_less(lhs.data, rhs.data)};}
  RDFTTYPE operator()(rdf::LUInt32       lhs, rdf::LDouble       rhs)const{return rdf::LInt32{boost::numeric_cast<double>(lhs.data) < rhs.data};}

  RDFTTYPE operator()(rdf::LInt64        lhs, rdf::LInt32        rhs)const{return rdf::LInt32{std::cmp_less(lhs.data, rhs.data)};}
  RDFTTYPE operator()(rdf::LInt64        lhs, rdf::LUInt32       rhs)const{return rdf::LInt32{std::cmp_less(lhs.data, rhs.data)};}
  RDFTTYPE operator()(rdf::LInt64        lhs, rdf::LInt64        rhs)const{return rdf::LInt32{std::cmp_less(lhs.data, rhs.data)};}
  RDFTTYPE operator()(rdf::LInt64        lhs, rdf::LUInt64       rhs)const{return rdf::LInt32{std::cmp_less(lhs.data, rhs.data)};}
  RDFTTYPE operator()(rdf::LInt64        lhs, rdf::LDouble       rhs)const{return rdf::LInt32{boost::numeric_cast<double>(lhs.data) < rhs.data};}

  RDFTTYPE operator()(rdf::LUInt64       lhs, rdf::LInt32        rhs)const{return rdf::LInt32{std::cmp_less(lhs.data, rhs.data)};}
  RDFTTYPE operator()(rdf::LUInt64       lhs, rdf::LUInt32       rhs)const{return rdf::LInt32{std::cmp_less(lhs.data, rhs.data)};}
  RDFTTYPE operator()(rdf::LUInt64       lhs, rdf::LInt64        rhs)const{return rdf::LInt32{std::cmp_less(lhs.data, rhs.data)};}
  RDFTTYPE operator()(rdf::LUInt64       lhs, rdf::LUInt64       rhs)const{return rdf::LInt32{std::cmp_less(lhs.data, rhs.data)};}
  RDFTTYPE operator()(rdf::LUInt64       lhs, rdf::LDouble       rhs)const{return rdf::LInt32{boost::numeric_cast<double>(lhs.data) < rhs.data};}

  RDFTTYPE operator()(rdf::LDouble       lhs, rdf::LInt32        rhs)const{return rdf::LInt32{lhs.data < boost::numeric_cast<double>(rhs.data)};}
  RDFTTYPE operator()(rdf::LDouble       lhs, rdf::LUInt32       rhs)const{return rdf::LInt32{lhs.data < boost::numeric_cast<double>(rhs.data)};}
  RDFTTYPE operator()(rdf::LDouble       lhs, rdf::LInt64        rhs)const{return rdf::LInt32{lhs.data < boost::numeric_cast<double>(rhs.data)};}
  RDFTTYPE operator()(rdf::LDouble       lhs, rdf::LUInt64       rhs)const{return rdf::LInt32{lhs.data < boost::numeric_cast<double>(rhs.data)};}
  RDFTTYPE operator()(rdf::LDouble       lhs, rdf::LDouble       rhs)const{return rdf::LInt32{is_lt(lhs, rhs)};}

  RDFTTYPE operator()(rdf::LString       lhs, rdf::LString       rhs)const{return rdf::LInt32{lhs.data < rhs.data};}

  // -------------------------------------------------------------------------------------------
  RDFTTYPE operator()(rdf::LDate       lhs, rdf::LDate         rhs)const{return rdf::LInt32{lhs.data < rhs.data};}
  RDFTTYPE operator()(rdf::LDate       lhs, rdf::LDatetime     rhs)const{return rdf::LInt32{rdf::to_datetime(std::move(lhs.data)) < rhs.data};}  
  RDFTTYPE operator()(rdf::LDatetime   lhs, rdf::LDate         rhs)const{return rdf::LInt32{lhs.data < rdf::to_datetime(std::move(rhs.data))};}
  RDFTTYPE operator()(rdf::LDatetime   lhs, rdf::LDatetime     rhs)const{return rdf::LInt32{lhs.data < rhs.data};}

  ReteSession * rs;
  BetaRow const* br;
};

// GeVisitor
// --------------------------------------------------------------------------------------
struct GeVisitor: public boost::static_visitor<RDFTTYPE>, public NoCallbackNeeded
{
  GeVisitor(ReteSession * rs, BetaRow const* br): rs(rs), br(br) {}
  GeVisitor(): rs(nullptr), br(nullptr) {} // for use by other operators
  template<class T, class U> RDFTTYPE operator()(T lhs, U rhs) const {return rdf::False();};
  RDFTTYPE operator()(rdf::RDFNull       lhs, rdf::RDFNull       rhs)const{return rdf::True();}
  RDFTTYPE operator()(rdf::BlankNode     lhs, rdf::BlankNode     rhs)const{return rdf::LInt32{lhs.key >= rhs.key};}
  RDFTTYPE operator()(rdf::NamedResource lhs, rdf::NamedResource rhs)const{return rdf::LInt32{lhs.name >= rhs.name};}

  RDFTTYPE operator()(rdf::LInt32        lhs, rdf::LInt32        rhs)const{return rdf::LInt32{std::cmp_greater_equal(lhs.data, rhs.data)};}
  RDFTTYPE operator()(rdf::LInt32        lhs, rdf::LUInt32       rhs)const{return rdf::LInt32{std::cmp_greater_equal(lhs.data, rhs.data)};}
  RDFTTYPE operator()(rdf::LInt32        lhs, rdf::LInt64        rhs)const{return rdf::LInt32{std::cmp_greater_equal(lhs.data, rhs.data)};}
  RDFTTYPE operator()(rdf::LInt32        lhs, rdf::LUInt64       rhs)const{return rdf::LInt32{std::cmp_greater_equal(lhs.data, rhs.data)};}
  RDFTTYPE operator()(rdf::LInt32        lhs, rdf::LDouble       rhs)const{return rdf::LInt32{boost::numeric_cast<double>(lhs.data) >= rhs.data};}

  RDFTTYPE operator()(rdf::LUInt32       lhs, rdf::LInt32        rhs)const{return rdf::LInt32{std::cmp_greater_equal(lhs.data, rhs.data)};}
  RDFTTYPE operator()(rdf::LUInt32       lhs, rdf::LUInt32       rhs)const{return rdf::LInt32{std::cmp_greater_equal(lhs.data, rhs.data)};}
  RDFTTYPE operator()(rdf::LUInt32       lhs, rdf::LInt64        rhs)const{return rdf::LInt32{std::cmp_greater_equal(lhs.data, rhs.data)};}
  RDFTTYPE operator()(rdf::LUInt32       lhs, rdf::LUInt64       rhs)const{return rdf::LInt32{std::cmp_greater_equal(lhs.data, rhs.data)};}
  RDFTTYPE operator()(rdf::LUInt32       lhs, rdf::LDouble       rhs)const{return rdf::LInt32{boost::numeric_cast<double>(lhs.data) >= rhs.data};}

  RDFTTYPE operator()(rdf::LInt64        lhs, rdf::LInt32        rhs)const{return rdf::LInt32{std::cmp_greater_equal(lhs.data, rhs.data)};}
  RDFTTYPE operator()(rdf::LInt64        lhs, rdf::LUInt32       rhs)const{return rdf::LInt32{std::cmp_greater_equal(lhs.data, rhs.data)};}
  RDFTTYPE operator()(rdf::LInt64        lhs, rdf::LInt64        rhs)const{return rdf::LInt32{std::cmp_greater_equal(lhs.data, rhs.data)};}
  RDFTTYPE operator()(rdf::LInt64        lhs, rdf::LUInt64       rhs)const{return rdf::LInt32{std::cmp_greater_equal(lhs.data, rhs.data)};}
  RDFTTYPE operator()(rdf::LInt64        lhs, rdf::LDouble       rhs)const{return rdf::LInt32{boost::numeric_cast<double>(lhs.data) >= rhs.data};}

  RDFTTYPE operator()(rdf::LUInt64       lhs, rdf::LInt32        rhs)const{return rdf::LInt32{std::cmp_greater_equal(lhs.data, rhs.data)};}
  RDFTTYPE operator()(rdf::LUInt64       lhs, rdf::LUInt32       rhs)const{return rdf::LInt32{std::cmp_greater_equal(lhs.data, rhs.data)};}
  RDFTTYPE operator()(rdf::LUInt64       lhs, rdf::LInt64        rhs)const{return rdf::LInt32{std::cmp_greater_equal(lhs.data, rhs.data)};}
  RDFTTYPE operator()(rdf::LUInt64       lhs, rdf::LUInt64       rhs)const{return rdf::LInt32{std::cmp_greater_equal(lhs.data, rhs.data)};}
  RDFTTYPE operator()(rdf::LUInt64       lhs, rdf::LDouble       rhs)const{return rdf::LInt32{boost::numeric_cast<double>(lhs.data) >= rhs.data};}

  RDFTTYPE operator()(rdf::LDouble       lhs, rdf::LInt32        rhs)const{return rdf::LInt32{lhs.data >= boost::numeric_cast<double>(rhs.data)};}
  RDFTTYPE operator()(rdf::LDouble       lhs, rdf::LUInt32       rhs)const{return rdf::LInt32{lhs.data >= boost::numeric_cast<double>(rhs.data)};}
  RDFTTYPE operator()(rdf::LDouble       lhs, rdf::LInt64        rhs)const{return rdf::LInt32{lhs.data >= boost::numeric_cast<double>(rhs.data)};}
  RDFTTYPE operator()(rdf::LDouble       lhs, rdf::LUInt64       rhs)const{return rdf::LInt32{lhs.data >= boost::numeric_cast<double>(rhs.data)};}
  RDFTTYPE operator()(rdf::LDouble       lhs, rdf::LDouble       rhs)const{return rdf::LInt32{is_ge(lhs, rhs)};}

  RDFTTYPE operator()(rdf::LString       lhs, rdf::LString       rhs)const{return rdf::LInt32{lhs.data >= rhs.data};}

  // -------------------------------------------------------------------------------------------
  RDFTTYPE operator()(rdf::LDate       lhs, rdf::LDate         rhs)const{return rdf::LInt32{lhs.data >= rhs.data};}
  RDFTTYPE operator()(rdf::LDate       lhs, rdf::LDatetime     rhs)const{return rdf::LInt32{rdf::to_datetime(std::move(lhs.data)) >= rhs.data};}  
  RDFTTYPE operator()(rdf::LDatetime   lhs, rdf::LDate         rhs)const{return rdf::LInt32{lhs.data >= rdf::to_datetime(std::move(rhs.data))};}
  RDFTTYPE operator()(rdf::LDatetime   lhs, rdf::LDatetime     rhs)const{return rdf::LInt32{lhs.data >= rhs.data};}

  ReteSession * rs;
  BetaRow const* br;
};

// GtVisitor
// --------------------------------------------------------------------------------------
struct GtVisitor: public boost::static_visitor<RDFTTYPE>, public NoCallbackNeeded
{
  GtVisitor(ReteSession * rs, BetaRow const* br): rs(rs), br(br) {}
  GtVisitor(): rs(nullptr), br(nullptr) {} // for use by other operators
  template<class T, class U> RDFTTYPE operator()(T lhs, U rhs) const {return rdf::False();};
  RDFTTYPE operator()(rdf::RDFNull       lhs, rdf::RDFNull       rhs)const{return rdf::False();}
  RDFTTYPE operator()(rdf::BlankNode     lhs, rdf::BlankNode     rhs)const{return rdf::LInt32{lhs.key > rhs.key};}
  RDFTTYPE operator()(rdf::NamedResource lhs, rdf::NamedResource rhs)const{return rdf::LInt32{lhs.name > rhs.name};}

  RDFTTYPE operator()(rdf::LInt32        lhs, rdf::LInt32        rhs)const{return rdf::LInt32{std::cmp_greater(lhs.data, rhs.data)};}
  RDFTTYPE operator()(rdf::LInt32        lhs, rdf::LUInt32       rhs)const{return rdf::LInt32{std::cmp_greater(lhs.data, rhs.data)};}
  RDFTTYPE operator()(rdf::LInt32        lhs, rdf::LInt64        rhs)const{return rdf::LInt32{std::cmp_greater(lhs.data, rhs.data)};}
  RDFTTYPE operator()(rdf::LInt32        lhs, rdf::LUInt64       rhs)const{return rdf::LInt32{std::cmp_greater(lhs.data, rhs.data)};}
  RDFTTYPE operator()(rdf::LInt32        lhs, rdf::LDouble       rhs)const{return rdf::LInt32{boost::numeric_cast<double>(lhs.data) > rhs.data};}

  RDFTTYPE operator()(rdf::LUInt32       lhs, rdf::LInt32        rhs)const{return rdf::LInt32{std::cmp_greater(lhs.data, rhs.data)};}
  RDFTTYPE operator()(rdf::LUInt32       lhs, rdf::LUInt32       rhs)const{return rdf::LInt32{std::cmp_greater(lhs.data, rhs.data)};}
  RDFTTYPE operator()(rdf::LUInt32       lhs, rdf::LInt64        rhs)const{return rdf::LInt32{std::cmp_greater(lhs.data, rhs.data)};}
  RDFTTYPE operator()(rdf::LUInt32       lhs, rdf::LUInt64       rhs)const{return rdf::LInt32{std::cmp_greater(lhs.data, rhs.data)};}
  RDFTTYPE operator()(rdf::LUInt32       lhs, rdf::LDouble       rhs)const{return rdf::LInt32{boost::numeric_cast<double>(lhs.data) > rhs.data};}

  RDFTTYPE operator()(rdf::LInt64        lhs, rdf::LInt32        rhs)const{return rdf::LInt32{std::cmp_greater(lhs.data, rhs.data)};}
  RDFTTYPE operator()(rdf::LInt64        lhs, rdf::LUInt32       rhs)const{return rdf::LInt32{std::cmp_greater(lhs.data, rhs.data)};}
  RDFTTYPE operator()(rdf::LInt64        lhs, rdf::LInt64        rhs)const{return rdf::LInt32{std::cmp_greater(lhs.data, rhs.data)};}
  RDFTTYPE operator()(rdf::LInt64        lhs, rdf::LUInt64       rhs)const{return rdf::LInt32{std::cmp_greater(lhs.data, rhs.data)};}
  RDFTTYPE operator()(rdf::LInt64        lhs, rdf::LDouble       rhs)const{return rdf::LInt32{boost::numeric_cast<double>(lhs.data) > rhs.data};}

  RDFTTYPE operator()(rdf::LUInt64       lhs, rdf::LInt32        rhs)const{return rdf::LInt32{std::cmp_greater(lhs.data, rhs.data)};}
  RDFTTYPE operator()(rdf::LUInt64       lhs, rdf::LUInt32       rhs)const{return rdf::LInt32{std::cmp_greater(lhs.data, rhs.data)};}
  RDFTTYPE operator()(rdf::LUInt64       lhs, rdf::LInt64        rhs)const{return rdf::LInt32{std::cmp_greater(lhs.data, rhs.data)};}
  RDFTTYPE operator()(rdf::LUInt64       lhs, rdf::LUInt64       rhs)const{return rdf::LInt32{std::cmp_greater(lhs.data, rhs.data)};}
  RDFTTYPE operator()(rdf::LUInt64       lhs, rdf::LDouble       rhs)const{return rdf::LInt32{boost::numeric_cast<double>(lhs.data) > rhs.data};}

  RDFTTYPE operator()(rdf::LDouble       lhs, rdf::LInt32        rhs)const{return rdf::LInt32{lhs.data > boost::numeric_cast<double>(rhs.data)};}
  RDFTTYPE operator()(rdf::LDouble       lhs, rdf::LUInt32       rhs)const{return rdf::LInt32{lhs.data > boost::numeric_cast<double>(rhs.data)};}
  RDFTTYPE operator()(rdf::LDouble       lhs, rdf::LInt64        rhs)const{return rdf::LInt32{lhs.data > boost::numeric_cast<double>(rhs.data)};}
  RDFTTYPE operator()(rdf::LDouble       lhs, rdf::LUInt64       rhs)const{return rdf::LInt32{lhs.data > boost::numeric_cast<double>(rhs.data)};}
  RDFTTYPE operator()(rdf::LDouble       lhs, rdf::LDouble       rhs)const{return rdf::LInt32{is_gt(lhs, rhs)};}

  RDFTTYPE operator()(rdf::LString       lhs, rdf::LString       rhs)const{return rdf::LInt32{lhs.data > rhs.data};}

  // -------------------------------------------------------------------------------------------
  RDFTTYPE operator()(rdf::LDate       lhs, rdf::LDate         rhs)const{return rdf::LInt32{lhs.data > rhs.data};}
  RDFTTYPE operator()(rdf::LDate       lhs, rdf::LDatetime     rhs)const{return rdf::LInt32{rdf::to_datetime(std::move(lhs.data)) > rhs.data};}  
  RDFTTYPE operator()(rdf::LDatetime   lhs, rdf::LDate         rhs)const{return rdf::LInt32{lhs.data > rdf::to_datetime(std::move(rhs.data))};}
  RDFTTYPE operator()(rdf::LDatetime   lhs, rdf::LDatetime     rhs)const{return rdf::LInt32{lhs.data > rhs.data};}

  ReteSession * rs;
  BetaRow const* br;
};

// AndVisitor
// --------------------------------------------------------------------------------------
struct AndVisitor: public boost::static_visitor<RDFTTYPE>, public NoCallbackNeeded
{
  AndVisitor(ReteSession * rs, BetaRow const* br): rs(rs), br(br) {}
  AndVisitor(): rs(nullptr), br(nullptr) {} // for use by other operators
  template<class T, class U> RDFTTYPE operator()(T lhs, U rhs) const {return rdf::False();};
  RDFTTYPE operator()(rdf::LInt32        lhs, rdf::LInt32        rhs)const{return rdf::LInt32{lhs.data and rhs.data};}
  RDFTTYPE operator()(rdf::LInt32        lhs, rdf::LUInt32       rhs)const{return rdf::LInt32{lhs.data and rhs.data};}
  RDFTTYPE operator()(rdf::LInt32        lhs, rdf::LInt64        rhs)const{return rdf::LInt32{lhs.data and rhs.data};}
  RDFTTYPE operator()(rdf::LInt32        lhs, rdf::LUInt64       rhs)const{return rdf::LInt32{lhs.data and rhs.data};}
  RDFTTYPE operator()(rdf::LInt32        lhs, rdf::LDouble       rhs)const{return rdf::LInt32{lhs.data and rhs.data};}

  RDFTTYPE operator()(rdf::LUInt32       lhs, rdf::LInt32        rhs)const{return rdf::LInt32{lhs.data and rhs.data};}
  RDFTTYPE operator()(rdf::LUInt32       lhs, rdf::LUInt32       rhs)const{return rdf::LInt32{lhs.data and rhs.data};}
  RDFTTYPE operator()(rdf::LUInt32       lhs, rdf::LInt64        rhs)const{return rdf::LInt32{lhs.data and rhs.data};}
  RDFTTYPE operator()(rdf::LUInt32       lhs, rdf::LUInt64       rhs)const{return rdf::LInt32{lhs.data and rhs.data};}
  RDFTTYPE operator()(rdf::LUInt32       lhs, rdf::LDouble       rhs)const{return rdf::LInt32{lhs.data and rhs.data};}

  RDFTTYPE operator()(rdf::LInt64        lhs, rdf::LInt32        rhs)const{return rdf::LInt32{lhs.data and rhs.data};}
  RDFTTYPE operator()(rdf::LInt64        lhs, rdf::LUInt32       rhs)const{return rdf::LInt32{lhs.data and rhs.data};}
  RDFTTYPE operator()(rdf::LInt64        lhs, rdf::LInt64        rhs)const{return rdf::LInt32{lhs.data and rhs.data};}
  RDFTTYPE operator()(rdf::LInt64        lhs, rdf::LUInt64       rhs)const{return rdf::LInt32{lhs.data and rhs.data};}
  RDFTTYPE operator()(rdf::LInt64        lhs, rdf::LDouble       rhs)const{return rdf::LInt32{lhs.data and rhs.data};}

  RDFTTYPE operator()(rdf::LUInt64       lhs, rdf::LInt32        rhs)const{return rdf::LInt32{lhs.data and rhs.data};}
  RDFTTYPE operator()(rdf::LUInt64       lhs, rdf::LUInt32       rhs)const{return rdf::LInt32{lhs.data and rhs.data};}
  RDFTTYPE operator()(rdf::LUInt64       lhs, rdf::LInt64        rhs)const{return rdf::LInt32{lhs.data and rhs.data};}
  RDFTTYPE operator()(rdf::LUInt64       lhs, rdf::LUInt64       rhs)const{return rdf::LInt32{lhs.data and rhs.data};}
  RDFTTYPE operator()(rdf::LUInt64       lhs, rdf::LDouble       rhs)const{return rdf::LInt32{lhs.data and rhs.data};}

  RDFTTYPE operator()(rdf::LDouble       lhs, rdf::LInt32        rhs)const{return rdf::LInt32{lhs.data and rhs.data};}
  RDFTTYPE operator()(rdf::LDouble       lhs, rdf::LUInt32       rhs)const{return rdf::LInt32{lhs.data and rhs.data};}
  RDFTTYPE operator()(rdf::LDouble       lhs, rdf::LInt64        rhs)const{return rdf::LInt32{lhs.data and rhs.data};}
  RDFTTYPE operator()(rdf::LDouble       lhs, rdf::LUInt64       rhs)const{return rdf::LInt32{lhs.data and rhs.data};}
  RDFTTYPE operator()(rdf::LDouble       lhs, rdf::LDouble       rhs)const{return rdf::LInt32{lhs.data and rhs.data};}

  RDFTTYPE operator()(rdf::LString       lhs, rdf::LString       rhs)const{return rdf::LInt32{!lhs.data.empty() and !rhs.data.empty()};}

  ReteSession * rs;
  BetaRow const* br;
};

// OrVisitor
// --------------------------------------------------------------------------------------
struct OrVisitor: public boost::static_visitor<RDFTTYPE>, public NoCallbackNeeded
{
  OrVisitor(ReteSession * rs, BetaRow const* br): rs(rs) {}
  OrVisitor(): rs(nullptr), br(nullptr) {} // for use by other operators
  template<class T, class U> RDFTTYPE operator()(T lhs, U rhs) const {return rdf::False();};
  RDFTTYPE operator()(rdf::LInt32        lhs, rdf::LInt32        rhs)const{return rdf::LInt32{lhs.data or rhs.data};}
  RDFTTYPE operator()(rdf::LInt32        lhs, rdf::LUInt32       rhs)const{return rdf::LInt32{lhs.data or rhs.data};}
  RDFTTYPE operator()(rdf::LInt32        lhs, rdf::LInt64        rhs)const{return rdf::LInt32{lhs.data or rhs.data};}
  RDFTTYPE operator()(rdf::LInt32        lhs, rdf::LUInt64       rhs)const{return rdf::LInt32{lhs.data or rhs.data};}
  RDFTTYPE operator()(rdf::LInt32        lhs, rdf::LDouble       rhs)const{return rdf::LInt32{lhs.data or rhs.data};}

  RDFTTYPE operator()(rdf::LUInt32       lhs, rdf::LInt32        rhs)const{return rdf::LInt32{lhs.data or rhs.data};}
  RDFTTYPE operator()(rdf::LUInt32       lhs, rdf::LUInt32       rhs)const{return rdf::LInt32{lhs.data or rhs.data};}
  RDFTTYPE operator()(rdf::LUInt32       lhs, rdf::LInt64        rhs)const{return rdf::LInt32{lhs.data or rhs.data};}
  RDFTTYPE operator()(rdf::LUInt32       lhs, rdf::LUInt64       rhs)const{return rdf::LInt32{lhs.data or rhs.data};}
  RDFTTYPE operator()(rdf::LUInt32       lhs, rdf::LDouble       rhs)const{return rdf::LInt32{lhs.data or rhs.data};}

  RDFTTYPE operator()(rdf::LInt64        lhs, rdf::LInt32        rhs)const{return rdf::LInt32{lhs.data or rhs.data};}
  RDFTTYPE operator()(rdf::LInt64        lhs, rdf::LUInt32       rhs)const{return rdf::LInt32{lhs.data or rhs.data};}
  RDFTTYPE operator()(rdf::LInt64        lhs, rdf::LInt64        rhs)const{return rdf::LInt32{lhs.data or rhs.data};}
  RDFTTYPE operator()(rdf::LInt64        lhs, rdf::LUInt64       rhs)const{return rdf::LInt32{lhs.data or rhs.data};}
  RDFTTYPE operator()(rdf::LInt64        lhs, rdf::LDouble       rhs)const{return rdf::LInt32{lhs.data or rhs.data};}

  RDFTTYPE operator()(rdf::LUInt64       lhs, rdf::LInt32        rhs)const{return rdf::LInt32{lhs.data or rhs.data};}
  RDFTTYPE operator()(rdf::LUInt64       lhs, rdf::LUInt32       rhs)const{return rdf::LInt32{lhs.data or rhs.data};}
  RDFTTYPE operator()(rdf::LUInt64       lhs, rdf::LInt64        rhs)const{return rdf::LInt32{lhs.data or rhs.data};}
  RDFTTYPE operator()(rdf::LUInt64       lhs, rdf::LUInt64       rhs)const{return rdf::LInt32{lhs.data or rhs.data};}
  RDFTTYPE operator()(rdf::LUInt64       lhs, rdf::LDouble       rhs)const{return rdf::LInt32{lhs.data or rhs.data};}

  RDFTTYPE operator()(rdf::LDouble       lhs, rdf::LInt32        rhs)const{return rdf::LInt32{lhs.data or rhs.data};}
  RDFTTYPE operator()(rdf::LDouble       lhs, rdf::LUInt32       rhs)const{return rdf::LInt32{lhs.data or rhs.data};}
  RDFTTYPE operator()(rdf::LDouble       lhs, rdf::LInt64        rhs)const{return rdf::LInt32{lhs.data or rhs.data};}
  RDFTTYPE operator()(rdf::LDouble       lhs, rdf::LUInt64       rhs)const{return rdf::LInt32{lhs.data or rhs.data};}
  RDFTTYPE operator()(rdf::LDouble       lhs, rdf::LDouble       rhs)const{return rdf::LInt32{lhs.data or rhs.data};}

  RDFTTYPE operator()(rdf::LString       lhs, rdf::LString       rhs)const{return rdf::LInt32{!lhs.data.empty() or !rhs.data.empty()};}

  ReteSession * rs;
  BetaRow const* br;
};

// NotVisitor
// --------------------------------------------------------------------------------------
struct NotVisitor: public boost::static_visitor<RDFTTYPE>, public NoCallbackNeeded
{
  NotVisitor(ReteSession * rs, BetaRow const* br): rs(rs), br(br) {}
  NotVisitor(): rs(nullptr), br(nullptr) {}
  template<class T> RDFTTYPE operator()(T lhs) const {if(br==nullptr) return rdf::Null(); else RETE_EXCEPTION("Invalid arguments for not: ("<<lhs<<")");};

  RDFTTYPE operator()(rdf::LInt32  lhs)const{return rdf::LInt32{boost::numeric_cast<int32_t>(not lhs.data)};}
  RDFTTYPE operator()(rdf::LUInt32 lhs)const{return rdf::LInt32{boost::numeric_cast<int32_t>(not lhs.data)};}
  RDFTTYPE operator()(rdf::LInt64  lhs)const{return rdf::LInt32{boost::numeric_cast<int32_t>(not lhs.data)};}
  RDFTTYPE operator()(rdf::LUInt64 lhs)const{return rdf::LInt32{boost::numeric_cast<int32_t>(not lhs.data)};}
  RDFTTYPE operator()(rdf::LDouble lhs)const{return rdf::LInt32{boost::numeric_cast<int32_t>(not lhs.data)};}

  ReteSession * rs;
  BetaRow const* br;
};

} // namespace jets::rete
#endif // JETS_RETE_EXPR_OP_LOGICS_H
