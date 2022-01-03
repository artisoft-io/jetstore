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
#include "jets/rete/beta_row.h"
#include "jets/rete/rete_session.h"

// This file contains basic operator used in rule expression 
// see ExprUnaryOp and ExprBinaryOp classes.
namespace jets::rete {

template<class T, class R> struct xliteral{};
template<class R> struct xliteral< rdf::LInt32, R>   {typedef R result;};
template<class R> struct xliteral< rdf::LUInt32, R>  {typedef R result;};
template<class R> struct xliteral< rdf::LInt64, R>   {typedef R result;};
template<class R> struct xliteral< rdf::LUInt64, R>   {typedef R result;};
template<class R> struct xliteral< rdf::LDouble, R>   {typedef R result;};
template<class R> struct xliteral< rdf::LString, R>   {typedef R result;};

using RDFTTYPE = rdf::RdfAstType;
// AddVisitor
// --------------------------------------------------------------------------------------
struct AddVisitor: public boost::static_visitor<RDFTTYPE>
{
  AddVisitor(ReteSession * rs, BetaRow const* br): rs(rs), br(br) {}
  template<class T, class U> RDFTTYPE operator()(T lhs, U rhs) const {return rdf::RDFNull();};

  RDFTTYPE operator()(rdf::LInt32  lhs, rdf::LInt32  rhs)const{return rdf::LInt32{lhs.data+rhs.data};}
  RDFTTYPE operator()(rdf::LInt32  lhs, rdf::LUInt32 rhs)const{return rdf::LInt32{lhs.data+boost::numeric_cast<int32_t>(rhs.data)};}
  RDFTTYPE operator()(rdf::LInt32  lhs, rdf::LInt64  rhs)const{return rdf::LInt32{lhs.data+boost::numeric_cast<int32_t>(rhs.data)};}
  RDFTTYPE operator()(rdf::LInt32  lhs, rdf::LUInt64 rhs)const{return rdf::LInt32{lhs.data+boost::numeric_cast<int32_t>(rhs.data)};}
  RDFTTYPE operator()(rdf::LInt32  lhs, rdf::LDouble rhs)const{return rdf::LInt32{lhs.data+boost::numeric_cast<int32_t>(rhs.data)};}
  // RDFTTYPE operator()(rdf::LInt32  lhs, rdf::LString rhs){return {lhs.data+rhs.data};}

  RDFTTYPE operator()(rdf::LUInt32 lhs, rdf::LInt32  rhs)const{return rdf::LUInt32{lhs.data+boost::numeric_cast<uint32_t>(rhs.data)};}
  RDFTTYPE operator()(rdf::LUInt32 lhs, rdf::LUInt32 rhs)const{return rdf::LUInt32{lhs.data+rhs.data};}
  RDFTTYPE operator()(rdf::LUInt32 lhs, rdf::LInt64  rhs)const{return rdf::LUInt32{lhs.data+boost::numeric_cast<uint32_t>(rhs.data)};}
  RDFTTYPE operator()(rdf::LUInt32 lhs, rdf::LUInt64 rhs)const{return rdf::LUInt32{lhs.data+boost::numeric_cast<uint32_t>(rhs.data)};}
  RDFTTYPE operator()(rdf::LUInt32 lhs, rdf::LDouble rhs)const{return rdf::LUInt32{lhs.data+boost::numeric_cast<uint32_t>(rhs.data)};}
  // RDFTTYPE operator()(rdf::LUInt32 lhs, rdf::LString rhs){return {lhs.data+rhs.data};}

  RDFTTYPE operator()(rdf::LInt64  lhs, rdf::LInt32  rhs)const{return rdf::LInt64{lhs.data+boost::numeric_cast<int64_t>(rhs.data)};}
  RDFTTYPE operator()(rdf::LInt64  lhs, rdf::LUInt32 rhs)const{return rdf::LInt64{lhs.data+boost::numeric_cast<int64_t>(rhs.data)};}
  RDFTTYPE operator()(rdf::LInt64  lhs, rdf::LInt64  rhs)const{return rdf::LInt64{lhs.data+rhs.data};}
  RDFTTYPE operator()(rdf::LInt64  lhs, rdf::LUInt64 rhs)const{return rdf::LInt64{lhs.data+boost::numeric_cast<int64_t>(rhs.data)};}
  RDFTTYPE operator()(rdf::LInt64  lhs, rdf::LDouble rhs)const{return rdf::LInt64{lhs.data+boost::numeric_cast<int64_t>(rhs.data)};}
  // RDFTTYPE operator()(rdf::LInt64  lhs, rdf::LString rhs){return {lhs.data+rhs.data};}

  RDFTTYPE operator()(rdf::LUInt64 lhs, rdf::LInt32  rhs)const{return rdf::LUInt64{lhs.data+boost::numeric_cast<uint64_t>(rhs.data)};}
  RDFTTYPE operator()(rdf::LUInt64 lhs, rdf::LUInt32 rhs)const{return rdf::LUInt64{lhs.data+boost::numeric_cast<uint64_t>(rhs.data)};}
  RDFTTYPE operator()(rdf::LUInt64 lhs, rdf::LInt64  rhs)const{return rdf::LUInt64{lhs.data+boost::numeric_cast<uint64_t>(rhs.data)};}
  RDFTTYPE operator()(rdf::LUInt64 lhs, rdf::LUInt64 rhs)const{return rdf::LUInt64{lhs.data+boost::numeric_cast<uint64_t>(rhs.data)};}
  RDFTTYPE operator()(rdf::LUInt64 lhs, rdf::LDouble rhs)const{return rdf::LUInt64{lhs.data+boost::numeric_cast<uint64_t>(rhs.data)};}
  // RDFTTYPE operator()(rdf::LUInt64 lhs, rdf::LString rhs){return {lhs.data+rhs.data};}

  RDFTTYPE operator()(rdf::LDouble lhs, rdf::LInt32  rhs)const{return rdf::LDouble{lhs.data+boost::numeric_cast<double>(rhs.data)};}
  RDFTTYPE operator()(rdf::LDouble lhs, rdf::LUInt32 rhs)const{return rdf::LDouble{lhs.data+boost::numeric_cast<double>(rhs.data)};}
  RDFTTYPE operator()(rdf::LDouble lhs, rdf::LInt64  rhs)const{return rdf::LDouble{lhs.data+boost::numeric_cast<double>(rhs.data)};}
  RDFTTYPE operator()(rdf::LDouble lhs, rdf::LUInt64 rhs)const{return rdf::LDouble{lhs.data+boost::numeric_cast<double>(rhs.data)};}
  RDFTTYPE operator()(rdf::LDouble lhs, rdf::LDouble rhs)const{return rdf::LDouble{lhs.data+boost::numeric_cast<double>(rhs.data)};}
  // RDFTTYPE operator()(rdf::LDouble lhs, rdf::LString rhs){return {lhs.data+rhs.data};}

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
struct EqVisitor: public boost::static_visitor<rdf::RdfAstType>
{
  EqVisitor(ReteSession * rs, BetaRow const* br): rs(rs), br(br) {}
  template<class T, class U> rdf::RdfAstType operator()(T lhs, U rhs){return rdf::LInt32( std::cmp_equal(lhs.data, rhs.data) );};
  template<class T> rdf::RdfAstType operator()(T, rdf::RDFNull      ){return rdf::LInt32( 0 );};
  template<class T> rdf::RdfAstType operator()(T, rdf::BlankNode    ){return rdf::LInt32( 0 );};
  template<class T> rdf::RdfAstType operator()(T, rdf::NamedResource){return rdf::LInt32( 0 );};
  template<class T> rdf::RdfAstType operator()(rdf::RDFNull      , T){return rdf::LInt32( 0 );};
  template<class T> rdf::RdfAstType operator()(rdf::BlankNode    , T){return rdf::LInt32( 0 );};
  template<class T> rdf::RdfAstType operator()(rdf::NamedResource, T){return rdf::LInt32( 0 );};

  ReteSession * rs;
  BetaRow const* br;
};

// // RegexVisitor
// // --------------------------------------------------------------------------------------
// struct RegexVisitor: public boost::static_visitor<rdf::RdfAstType>
// {
//   RegexVisitor(ReteSession * rs, BetaRow const* br): rs(rs), br(br) {}
//   template<class T, class U> rdf::RdfAstType operator()(T, U){return rdf::RDFNull();};
//   rdf::RdfAstType operator()(rdf::LString lhs, rdf::LString rhs)
//   {
//     std::regex expr_regex(rhs.data);
//     std::smatch match;
//     if(std::regex_search(lhs.data, match, expr_regex)) {
//       return rdf::LString(match[1]);
//     }
//     return rdf::RDFNull();
//   }
//   ReteSession * rs;
//   BetaRow const* br;
// };

// // ToUpperVisitor
// // --------------------------------------------------------------------------------------
// struct ToUpperVisitor: public boost::static_visitor<rdf::RdfAstType>
// {
//   ToUpperVisitor(ReteSession * rs, BetaRow const* br): rs(rs), br(br) {}
//   template<class T>rdf::RdfAstType operator()(T){return rdf::RDFNull();};
//   rdf::RdfAstType operator()(rdf::LString lhs)
//   {
//     std::transform(lhs.data.begin(), lhs.data.end(), lhs.data.begin(),
//       [](unsigned char c){return std::toupper(c);});
//     return lhs;
//   }
//   ReteSession * rs;
//   BetaRow const* br;
// };

// // ToLowerVisitor
// // --------------------------------------------------------------------------------------
// struct ToLowerVisitor: public boost::static_visitor<rdf::RdfAstType>
// {
//   ToLowerVisitor(ReteSession * rs, BetaRow const* br): rs(rs), br(br) {}
//   template<class T>rdf::RdfAstType operator()(T){return rdf::RDFNull();};
//   rdf::RdfAstType operator()(rdf::LString lhs)
//   {
//     std::transform(lhs.data.begin(), lhs.data.end(), lhs.data.begin(),
//       [](unsigned char c){return std::tolower(c);});
//     return lhs;
//   }
//   ReteSession * rs;
//   BetaRow const* br;
// };

// // TrimVisitor
// // --------------------------------------------------------------------------------------
// struct TrimVisitor: public boost::static_visitor<rdf::RdfAstType>
// {
//   TrimVisitor(ReteSession * rs, BetaRow const* br): rs(rs), br(br) {}
//   template<class T>rdf::RdfAstType operator()(T){return rdf::RDFNull();};
//   rdf::RdfAstType operator()(rdf::LString lhs)
//   {
//     return rdf::LString(rdf::trim(lhs.data));
//   }
//   ReteSession * rs;
//   BetaRow const* br;
// };

} // namespace jets::rete
#endif // JETS_RETE_EXPR_OPERATORS_H
