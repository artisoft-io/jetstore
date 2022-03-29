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

// This file contains basic operator used in rule expression 
// see ExprUnaryOp and ExprBinaryOp classes.
namespace jets::rete {

using RDFTTYPE = rdf::RdfAstType;

// LookupVisitor
// --------------------------------------------------------------------------------------
struct LookupVisitor: public boost::static_visitor<RDFTTYPE>
{
  // Fully expanded example to serve as template
  LookupVisitor(ReteSession * rs, BetaRow const* br, rdf::r_index lhs, rdf::r_index rhs): rs(rs), br(br), lhs_(lhs), rhs_(rhs) {}
  LookupVisitor(): rs(nullptr), br(nullptr), lhs_(nullptr), rhs_(nullptr) {} // for use by other operators
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
  RDFTTYPE operator()(rdf::LDate       lhs, rdf::LDatetime     rhs)const{return rdf::LInt32{rdf::to_time(std::move(lhs.data)) == rhs.data};}
  
  RDFTTYPE operator()(rdf::LDatetime   lhs, rdf::RDFNull       rhs)const{return rdf::False();}
  RDFTTYPE operator()(rdf::LDatetime   lhs, rdf::BlankNode     rhs)const{return rdf::False();}
  RDFTTYPE operator()(rdf::LDatetime   lhs, rdf::NamedResource rhs)const{return rdf::False();}
  RDFTTYPE operator()(rdf::LDatetime   lhs, rdf::LInt32        rhs)const{return rdf::False();}
  RDFTTYPE operator()(rdf::LDatetime   lhs, rdf::LUInt32       rhs)const{return rdf::False();}
  RDFTTYPE operator()(rdf::LDatetime   lhs, rdf::LInt64        rhs)const{return rdf::False();}
  RDFTTYPE operator()(rdf::LDatetime   lhs, rdf::LUInt64       rhs)const{return rdf::False();}
  RDFTTYPE operator()(rdf::LDatetime   lhs, rdf::LDouble       rhs)const{return rdf::False();}
  RDFTTYPE operator()(rdf::LDatetime   lhs, rdf::LString       rhs)const{return rdf::False();}
  RDFTTYPE operator()(rdf::LDatetime   lhs, rdf::LDate         rhs)const{return rdf::LInt32{lhs.data == rdf::to_time(std::move(rhs.data))};}
  RDFTTYPE operator()(rdf::LDatetime   lhs, rdf::LDatetime     rhs)const{return rdf::LInt32{lhs.data == rhs.data};}
  
  ReteSession * rs;
  BetaRow const* br;
  rdf::r_index lhs_;  // Note: This is the lhs as an r_index, may not exist in r_manager if this is
  rdf::r_index rhs_;  //       transitory resource
};

} // namespace jets::rete
#endif // JETS_RETE_EXPR_OP_OTHERS_H
