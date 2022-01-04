#ifndef JETS_RETE_EXPR_OPERATORS_H
#define JETS_RETE_EXPR_OPERATORS_H

#include <cctype>
#include <cstdint>
#include <type_traits>
#include <algorithm>
#include <string>
#include <memory>
#include <utility>
#include <regex>

#include <boost/numeric/conversion/cast.hpp>

#include "jets/rdf/rdf_types.h"
#include "jets/rete/rete_err.h"
#include "jets/rete/beta_row.h"
#include "jets/rete/rete_session.h"

// This file contains basic operator used in rule expression 
// see ExprUnaryOp and ExprBinaryOp classes.
namespace jets::rete {

using RDFTTYPE = rdf::RdfAstType;
// AddVisitor
// --------------------------------------------------------------------------------------
struct AddVisitor: public boost::static_visitor<RDFTTYPE>
{
  AddVisitor(ReteSession * rs, BetaRow const* br): rs(rs), br(br) {}
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
  RDFTTYPE operator()(rdf::LUInt64 lhs, rdf::LUInt64 rhs)const{return rdf::LUInt64{lhs.data+boost::numeric_cast<uint64_t>(rhs.data)};}
  RDFTTYPE operator()(rdf::LUInt64 lhs, rdf::LDouble rhs)const{return rdf::LUInt64{lhs.data+boost::numeric_cast<uint64_t>(rhs.data)};}

  RDFTTYPE operator()(rdf::LDouble lhs, rdf::LInt32  rhs)const{return rdf::LDouble{lhs.data+boost::numeric_cast<double>(rhs.data)};}
  RDFTTYPE operator()(rdf::LDouble lhs, rdf::LUInt32 rhs)const{return rdf::LDouble{lhs.data+boost::numeric_cast<double>(rhs.data)};}
  RDFTTYPE operator()(rdf::LDouble lhs, rdf::LInt64  rhs)const{return rdf::LDouble{lhs.data+boost::numeric_cast<double>(rhs.data)};}
  RDFTTYPE operator()(rdf::LDouble lhs, rdf::LUInt64 rhs)const{return rdf::LDouble{lhs.data+boost::numeric_cast<double>(rhs.data)};}
  RDFTTYPE operator()(rdf::LDouble lhs, rdf::LDouble rhs)const{return rdf::LDouble{lhs.data+boost::numeric_cast<double>(rhs.data)};}

  RDFTTYPE operator()(rdf::LString lhs, rdf::LInt32  rhs)const{return rdf::LString{lhs.data+std::to_string(rhs.data)};}
  RDFTTYPE operator()(rdf::LString lhs, rdf::LUInt32 rhs)const{return rdf::LString{lhs.data+std::to_string(rhs.data)};}
  RDFTTYPE operator()(rdf::LString lhs, rdf::LInt64  rhs)const{return rdf::LString{lhs.data+std::to_string(rhs.data)};}
  RDFTTYPE operator()(rdf::LString lhs, rdf::LUInt64 rhs)const{return rdf::LString{lhs.data+std::to_string(rhs.data)};}
  RDFTTYPE operator()(rdf::LString lhs, rdf::LDouble rhs)const{return rdf::LString{lhs.data+std::to_string(rhs.data)};}
  RDFTTYPE operator()(rdf::LString lhs, rdf::LString rhs)const{return rdf::LString{lhs.data+rhs.data};}
  ReteSession * rs;
  BetaRow const* br;
};

// EqVisitor
// --------------------------------------------------------------------------------------
struct EqVisitor: public boost::static_visitor<RDFTTYPE>
{
  // Fully expanded example to serve as template
  EqVisitor(ReteSession * rs, BetaRow const* br): rs(rs), br(br) {}
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
  ReteSession * rs;
  BetaRow const* br;
};

// RegexVisitor
// --------------------------------------------------------------------------------------
struct RegexVisitor: public boost::static_visitor<RDFTTYPE>
{
  RegexVisitor(ReteSession * rs, BetaRow const* br): rs(rs), br(br) {}
  template<class T, class U> RDFTTYPE operator()(T lhs, U rhs)const{RETE_EXCEPTION("Invalid arguments for REGEX: ("<<lhs<<", "<<rhs<<")");};
  RDFTTYPE operator()(rdf::LString lhs, rdf::LString rhs)const
  {
    std::regex expr_regex(lhs.data);
    std::smatch match;
    if(std::regex_search(rhs.data, match, expr_regex)) {
      return rdf::LString(match[1]);
    }
    return rdf::RDFNull();
  }
  ReteSession * rs;
  BetaRow const* br;
};

// ToUpperVisitor
// --------------------------------------------------------------------------------------
struct ToUpperVisitor: public boost::static_visitor<RDFTTYPE>
{
  ToUpperVisitor(ReteSession * rs, BetaRow const* br): rs(rs), br(br) {}
  template<class T>RDFTTYPE operator()(T lhs)const{RETE_EXCEPTION("Invalid arguments for TOUPPER: ("<<lhs<<")");};
  RDFTTYPE operator()(rdf::LString lhs)const
  {
    std::transform(lhs.data.begin(), lhs.data.end(), lhs.data.begin(),
      [](unsigned char c){return std::toupper(c);});
    return lhs;
  }
  ReteSession * rs;
  BetaRow const* br;
};

// ToLowerVisitor
// --------------------------------------------------------------------------------------
struct ToLowerVisitor: public boost::static_visitor<RDFTTYPE>
{
  ToLowerVisitor(ReteSession * rs, BetaRow const* br): rs(rs), br(br) {}
  template<class T>RDFTTYPE operator()(T lhs)const{RETE_EXCEPTION("Invalid arguments for TOLOWER: ("<<lhs<<")");};
  RDFTTYPE operator()(rdf::LString lhs)const
  {
    std::transform(lhs.data.begin(), lhs.data.end(), lhs.data.begin(),
      [](unsigned char c){return std::tolower(c);});
    return lhs;
  }
  ReteSession * rs;
  BetaRow const* br;
};

// TrimVisitor
// --------------------------------------------------------------------------------------
struct TrimVisitor: public boost::static_visitor<RDFTTYPE>
{
  TrimVisitor(ReteSession * rs, BetaRow const* br): rs(rs), br(br) {}
  template<class T>RDFTTYPE operator()(T lhs)const{RETE_EXCEPTION("Invalid arguments for TRIM: ("<<lhs<<")");};
  RDFTTYPE operator()(rdf::LString lhs)const
  {
    return rdf::LString(rdf::trim(lhs.data));
  }
  ReteSession * rs;
  BetaRow const* br;
};

} // namespace jets::rete
#endif // JETS_RETE_EXPR_OPERATORS_H
