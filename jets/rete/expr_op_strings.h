#ifndef JETS_RETE_EXPR_OP_STRINGS_H
#define JETS_RETE_EXPR_OP_STRINGS_H

#include <cctype>
#include <cstdint>
#include <type_traits>
#include <algorithm>
#include <string>
#include <memory>
#include <utility>
#include <regex>

#include <boost/format.hpp>
#include <boost/algorithm/string.hpp>
#include <boost/numeric/conversion/cast.hpp>

#include "../rdf/rdf_types.h"
#include "../rete/rete_err.h"
#include "../rete/beta_row.h"
#include "../rete/rete_session.h"

// This file contains basic operator used in rule expression 
// see ExprUnaryOp and ExprBinaryOp classes.
namespace jets::rete {

using RDFTTYPE = rdf::RdfAstType;

// RegexVisitor
// --------------------------------------------------------------------------------------
struct RegexVisitor: public boost::static_visitor<RDFTTYPE>, public NoCallbackNeeded
{
  RegexVisitor(ReteSession * rs, BetaRow const* br): rs(rs), br(br) {}
  RegexVisitor(): rs(nullptr), br(nullptr) {}
  template<class T, class U> RDFTTYPE operator()(T lhs, U rhs)const{if(br==nullptr) return rdf::Null(); else RETE_EXCEPTION("Invalid arguments for REGEX: ("<<lhs<<", "<<rhs<<")");};
  RDFTTYPE operator()(rdf::LString lhs, rdf::LString rhs)const
  {
    std::regex expr_regex(rhs.data);
    std::smatch match;
    if(std::regex_search(lhs.data, match, expr_regex)) {
      return rdf::LString(match[1]);
    }
    return rdf::Null();
  }
  ReteSession * rs;
  BetaRow const* br;
};

// To_upperVisitor
// --------------------------------------------------------------------------------------
struct To_upperVisitor: public boost::static_visitor<RDFTTYPE>, public NoCallbackNeeded
{
  To_upperVisitor(ReteSession * rs, BetaRow const* br): rs(rs), br(br) {}
  To_upperVisitor(): rs(nullptr), br(nullptr) {}
  template<class T>RDFTTYPE operator()(T lhs)const{if(br==nullptr) return rdf::Null(); else RETE_EXCEPTION("Invalid arguments for to_upper: ("<<lhs<<")");};
  RDFTTYPE operator()(rdf::LString lhs)const
  {
    std::transform(lhs.data.begin(), lhs.data.end(), lhs.data.begin(),
      [](unsigned char c){return std::toupper(c);});
    return lhs;
  }
  ReteSession * rs;
  BetaRow const* br;
};

// To_lowerVisitor
// --------------------------------------------------------------------------------------
struct To_lowerVisitor: public boost::static_visitor<RDFTTYPE>, public NoCallbackNeeded
{
  To_lowerVisitor(ReteSession * rs, BetaRow const* br): rs(rs), br(br) {}
  To_lowerVisitor(): rs(nullptr), br(nullptr) {}
  template<class T>RDFTTYPE operator()(T lhs)const{if(br==nullptr) return rdf::Null(); else RETE_EXCEPTION("Invalid arguments for TOLOWER: ("<<lhs<<")");};
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
struct TrimVisitor: public boost::static_visitor<RDFTTYPE>, public NoCallbackNeeded
{
  TrimVisitor(ReteSession * rs, BetaRow const* br): rs(rs), br(br) {}
  TrimVisitor(): rs(nullptr), br(nullptr) {}
  template<class T>RDFTTYPE operator()(T lhs)const{if(br==nullptr) return rdf::Null(); else RETE_EXCEPTION("Invalid arguments for TRIM: ("<<lhs<<")");};
  RDFTTYPE operator()(rdf::LString lhs)const
  {
    return rdf::LString(rdf::trim(lhs.data));
  }
  ReteSession * rs;
  BetaRow const* br;
};

// ContainsVisitor
// --------------------------------------------------------------------------------------
struct ContainsVisitor: public boost::static_visitor<RDFTTYPE>, public NoCallbackNeeded
{
  ContainsVisitor(ReteSession * rs, BetaRow const* br): rs(rs), br(br) {}
  ContainsVisitor(): rs(nullptr), br(nullptr) {}
  template<class T, class U> RDFTTYPE operator()(T lhs, U rhs)const{if(br==nullptr) return rdf::Null(); else RETE_EXCEPTION("Invalid arguments for contains: ("<<lhs<<", "<<rhs<<")");};
  RDFTTYPE operator()(rdf::LString       lhs, rdf::LString       rhs)const
  {
    return rdf::LInt32{ lhs.data.find(rhs.data) != std::string::npos };
  }
  ReteSession * rs;
  BetaRow const* br;
};

// LengthOfVisitor
// --------------------------------------------------------------------------------------
struct LengthOfVisitor: public boost::static_visitor<RDFTTYPE>, public NoCallbackNeeded
{
  LengthOfVisitor(ReteSession * rs, BetaRow const* br): rs(rs), br(br) {}
  template<class T>RDFTTYPE operator()(T lhs)const{if(br==nullptr) return rdf::Null(); else RETE_EXCEPTION("Invalid arguments for length_of: ("<<lhs<<")");};
  RDFTTYPE operator()(rdf::LString lhs)const
  {
    return rdf::LInt32(lhs.data.size());
  }
  ReteSession * rs;
  BetaRow const* br;
};

// ParseUsdCurrencyVisitor
// --------------------------------------------------------------------------------------
struct ParseUsdCurrencyVisitor: public boost::static_visitor<RDFTTYPE>, public NoCallbackNeeded
{
  ParseUsdCurrencyVisitor(ReteSession * rs, BetaRow const* br): rs(rs), br(br) {}
  ParseUsdCurrencyVisitor(): rs(nullptr), br(nullptr) {}
  template<class T>RDFTTYPE operator()(T lhs)const{if(br==nullptr) return rdf::Null(); else RETE_EXCEPTION("Invalid arguments for TRIM: ("<<lhs<<")");};
  RDFTTYPE operator()(rdf::LString lhs)const
  {
    if(lhs.data.empty()) return rdf::LDouble(0.0);
    std::string currency_amt;
    auto sz = lhs.data.size();
    currency_amt.reserve(sz);
    std::string::size_type ipos = 0;
    while(ipos < sz) {
        char c = lhs.data.at(ipos);
        if(c == '(' or c == '-') {
            currency_amt.push_back('-');
        } else if(std::isdigit(c, std::locale::classic()) or c=='.') {
            currency_amt.push_back(c);
        }
        ipos++;
    }
    if(currency_amt.empty()) return rdf::LDouble(0.0);
    return rdf::LDouble(boost::lexical_cast<double_t>(currency_amt));
  }
  ReteSession * rs;
  BetaRow const* br;
};

// ApplyFormatVisitor
// --------------------------------------------------------------------------------------
struct ApplyFormatVisitor: public boost::static_visitor<RDFTTYPE>, public NoCallbackNeeded
{
  ApplyFormatVisitor(ReteSession * rs, BetaRow const* br): rs(rs), br(br) {}
  ApplyFormatVisitor(): rs(nullptr), br(nullptr) {}
  template<class T, class U> RDFTTYPE operator()(T lhs, U rhs)const{if(br==nullptr) return rdf::Null(); else RETE_EXCEPTION("Invalid arguments for apply_format: ("<<lhs<<", "<<rhs<<")");};

  RDFTTYPE operator()(rdf::NamedResource lhs, rdf::LString       rhs)const{boost::format fmt(rhs.data); fmt.exceptions(boost::io::no_error_bits); fmt % lhs.name;	return rdf::LString(fmt.str());};
  RDFTTYPE operator()(rdf::LInt32        lhs, rdf::LString       rhs)const{boost::format fmt(rhs.data); fmt.exceptions(boost::io::no_error_bits); fmt % lhs.data;	return rdf::LString(fmt.str());};
  RDFTTYPE operator()(rdf::LUInt32       lhs, rdf::LString       rhs)const{boost::format fmt(rhs.data); fmt.exceptions(boost::io::no_error_bits); fmt % lhs.data;	return rdf::LString(fmt.str());};
  RDFTTYPE operator()(rdf::LInt64        lhs, rdf::LString       rhs)const{boost::format fmt(rhs.data); fmt.exceptions(boost::io::no_error_bits); fmt % lhs.data;	return rdf::LString(fmt.str());};
  RDFTTYPE operator()(rdf::LUInt64       lhs, rdf::LString       rhs)const{boost::format fmt(rhs.data); fmt.exceptions(boost::io::no_error_bits); fmt % lhs.data;	return rdf::LString(fmt.str());};
  RDFTTYPE operator()(rdf::LDouble       lhs, rdf::LString       rhs)const{boost::format fmt(rhs.data); fmt.exceptions(boost::io::no_error_bits); fmt % lhs.data;	return rdf::LString(fmt.str());};
  RDFTTYPE operator()(rdf::LString       lhs, rdf::LString       rhs)const{boost::format fmt(rhs.data); fmt.exceptions(boost::io::no_error_bits); fmt % lhs.data;	return rdf::LString(fmt.str());};
  RDFTTYPE operator()(rdf::LDate         lhs, rdf::LString       rhs)const{boost::format fmt(rhs.data); fmt.exceptions(boost::io::no_error_bits); fmt % lhs.data.year() % lhs.data.month() % lhs.data.day(); return rdf::LString(fmt.str());};
  RDFTTYPE operator()(rdf::LDatetime     lhs, rdf::LString       rhs)const{boost::format fmt(rhs.data); fmt.exceptions(boost::io::no_error_bits); fmt % lhs.data.date().year() % lhs.data.date().month() % lhs.data.date().day() % lhs.data.time_of_day().hours() % lhs.data.time_of_day().minutes() % lhs.data.time_of_day().seconds() % lhs.data.time_of_day().fractional_seconds();	return rdf::LString(fmt.str());};

  ReteSession * rs;
  BetaRow const* br;
};

// StartsWithVisitor
// --------------------------------------------------------------------------------------
struct StartsWithVisitor: public boost::static_visitor<RDFTTYPE>, public NoCallbackNeeded
{
  StartsWithVisitor(ReteSession * rs, BetaRow const* br): rs(rs), br(br) {}
  StartsWithVisitor(): rs(nullptr), br(nullptr) {}
  template<class T, class U> RDFTTYPE operator()(T lhs, U rhs)const{if(br==nullptr) return rdf::Null(); else RETE_EXCEPTION("Invalid arguments for starts_with: ("<<lhs<<", "<<rhs<<")");};

  RDFTTYPE operator()(rdf::LString       lhs, rdf::LString       rhs)const
  {
    return rdf::LInt32{ boost::starts_with(lhs.data, rhs.data) };
  }

  ReteSession * rs;
  BetaRow const* br;
};

} // namespace jets::rete
#endif // JETS_RETE_EXPR_OP_STRINGS_H
