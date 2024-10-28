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
  RDFTTYPE operator()(rdf::LDate         lhs, rdf::LString       rhs)const{boost::format fmt(rhs.data); fmt.exceptions(boost::io::no_error_bits); fmt % lhs.data.year() % lhs.data.month().as_number() % lhs.data.day(); return rdf::LString(fmt.str());};
  RDFTTYPE operator()(rdf::LDatetime     lhs, rdf::LString       rhs)const{boost::format fmt(rhs.data); fmt.exceptions(boost::io::no_error_bits); fmt % lhs.data.date().year() % lhs.data.date().month().as_number() % lhs.data.date().day() % lhs.data.time_of_day().hours() % lhs.data.time_of_day().minutes() % lhs.data.time_of_day().seconds() % lhs.data.time_of_day().fractional_seconds();	return rdf::LString(fmt.str());};

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

// EndsWithVisitor
// --------------------------------------------------------------------------------------
struct EndsWithVisitor: public boost::static_visitor<RDFTTYPE>, public NoCallbackNeeded
{
  EndsWithVisitor(ReteSession * rs, BetaRow const* br): rs(rs), br(br) {}
  EndsWithVisitor(): rs(nullptr), br(nullptr) {}
  template<class T, class U> RDFTTYPE operator()(T lhs, U rhs)const{if(br==nullptr) return rdf::Null(); else RETE_EXCEPTION("Invalid arguments for starts_with: ("<<lhs<<", "<<rhs<<")");};

  RDFTTYPE operator()(rdf::LString       lhs, rdf::LString       rhs)const
  {
    return rdf::LInt32{ boost::ends_with(lhs.data, rhs.data) };
  }

  ReteSession * rs;
  BetaRow const* br;
};

// SubstringOfVisitor
// --------------------------------------------------------------------------------------
struct SubstringOfVisitor: public boost::static_visitor<RDFTTYPE>, public NoCallbackNeeded
{
  SubstringOfVisitor(ReteSession * rs, BetaRow const* br): rs(rs), br(br) {}
  SubstringOfVisitor(): rs(nullptr), br(nullptr) {}
  template<class T, class U> RDFTTYPE operator()(T lhs, U rhs)const{if(br==nullptr) return rdf::Null(); else RETE_EXCEPTION("Invalid arguments for substring_of: ("<<lhs<<", "<<rhs<<")");};

  RDFTTYPE operator()(rdf::NamedResource lhs, rdf::LString rhs)const
  {
    // lhs argument is a config resource with data properties:
    //    - jets:from int value for start position of substring
    //    - jets:length int value for length of substring.
    // Note: if jets:from + jets:length > rhs.data.size() then return the available characters of rhs.data
    // Note: if jets:length < 0 then remove jets:length from the end of the string
    // Get from and length from the config object
    auto * sess = rs->rdf_session();
    auto rmgr = sess->rmgr();
    auto config = rmgr->get_resource(lhs.name);
    auto const* jr = rmgr->jets();
    auto from_obj  = sess->get_object(config, jr->jets__from);
    auto length_obj  = sess->get_object(config, jr->jets__length);

    // if obj == null or not int, then raise an error
    if (from_obj == nullptr || from_obj->which() != rdf::rdf_literal_int32_t) {
      sess->insert(jr->jets__istate, jr->jets__exception, rmgr->create_literal("error: invalid jets:from property for operator substring_of "+rhs.data));
      return rdf::Null();
    }
    if (length_obj == nullptr || length_obj->which() != rdf::rdf_literal_int32_t) {
      sess->insert(jr->jets__istate, jr->jets__exception, rmgr->create_literal("error: invalid jets:length property for operator substring_of "+rhs.data));
      return rdf::Null();
    }
    auto from = boost::get<rdf::LInt32>(from_obj)->data;
    auto length = boost::get<rdf::LInt32>(length_obj)->data;
    if (length < 0) {
      auto sz = rhs.data.size();
      length = sz + length - from;
    }
    return rdf::LString{ rhs.data.substr(from, length) };
  }

  ReteSession * rs;
  BetaRow const* br;
};

// ReplaceCharOfVisitor
// --------------------------------------------------------------------------------------
struct ReplaceCharOfVisitor: public boost::static_visitor<RDFTTYPE>, public NoCallbackNeeded
{
  ReplaceCharOfVisitor(ReteSession * rs, BetaRow const* br): rs(rs), br(br) {}
  ReplaceCharOfVisitor(): rs(nullptr), br(nullptr) {}
  template<class T, class U> RDFTTYPE operator()(T lhs, U rhs)const{if(br==nullptr) return rdf::Null(); else RETE_EXCEPTION("Invalid arguments for substring_of: ("<<lhs<<", "<<rhs<<")");};

  RDFTTYPE operator()(rdf::NamedResource lhs, rdf::LString rhs)const
  {
    // lhs argument is a config resource with data properties:
    //    - jets:replace_chars string: list of characters to replace
    //    - jets:replace_with string: character(s) to replace with
    // Get replace_chars and replace_with from the config object
    auto * sess = rs->rdf_session();
    auto rmgr = sess->rmgr();
    auto config = rmgr->get_resource(lhs.name);
    auto const* jr = rmgr->jets();
    auto replace_chars_obj  = sess->get_object(config, jr->jets__replace_chars);
    auto replace_with_obj  = sess->get_object(config, jr->jets__replace_with);

    // if obj == null or not int, then raise an error
    if (replace_chars_obj == nullptr || replace_chars_obj->which() != rdf::rdf_literal_string_t) {
      sess->insert(jr->jets__istate, jr->jets__exception, rmgr->create_literal("error: invalid jets:replace_chars property for operator replace_char_of "+rhs.data));
      return rdf::Null();
    }
    if (replace_with_obj == nullptr || replace_with_obj->which() != rdf::rdf_literal_string_t) {
      sess->insert(jr->jets__istate, jr->jets__exception, rmgr->create_literal("error: invalid jets:replace_with property for operator replace_char_of "+rhs.data));
      return rdf::Null();
    }
    auto replace_chars = boost::get<rdf::LString>(replace_chars_obj)->data;
    auto replace_with = boost::get<rdf::LString>(replace_with_obj)->data;
    auto str = std::move(rhs.data);
    for(auto i=0; i<replace_chars.size(); i++) {
      boost::replace_all(str, std::string(1, replace_chars[i]), replace_with);
    }
    return rdf::LString{ str };
  }

  ReteSession * rs;
  BetaRow const* br;
};

// CharAtVisitor
// --------------------------------------------------------------------------------------
struct CharAtVisitor: public boost::static_visitor<RDFTTYPE>, public NoCallbackNeeded
{
  CharAtVisitor(ReteSession * rs, BetaRow const* br): rs(rs), br(br) {}
  CharAtVisitor(): rs(nullptr), br(nullptr) {}
  template<class T, class U> RDFTTYPE operator()(T lhs, U rhs)const{if(br==nullptr) return rdf::Null(); else RETE_EXCEPTION("Invalid arguments for char_at: ("<<lhs<<", "<<rhs<<")");};

  RDFTTYPE operator()(rdf::LString       lhs, rdf::LInt32       rhs)const
  {
    if(lhs.data.size() <= rhs.data) {
      return rdf::Null();
    }
    return rdf::LString{ std::string(1, lhs.data[rhs.data]) };
  }

  ReteSession * rs;
  BetaRow const* br;
};

// CreateNamedMd5UUIDVisitor
// --------------------------------------------------------------------------------------
// Operator: uuid_md5
// Create a text literal containing a name-base uuid using MD5 hashing
struct CreateNamedMd5UUIDVisitor: public boost::static_visitor<RDFTTYPE>, public NoCallbackNeeded
{
  CreateNamedMd5UUIDVisitor(ReteSession * rs, BetaRow const* br): rs(rs), br(br) {}
  template<class T>RDFTTYPE operator()(T lhs)const{if(br==nullptr) return rdf::Null(); else RETE_EXCEPTION("Invalid arguments for uuid_md5: ("<<lhs<<")");};
  RDFTTYPE operator()(rdf::LString lhs)const
  {
    return rdf::LString(rdf::create_md5_uuid(std::move(lhs.data)));
  }
  ReteSession * rs;
  BetaRow const* br;
};

// CreateNamedSha1UUIDVisitor
// --------------------------------------------------------------------------------------
// Operator: uuid_sha1
// Create a text literal containing a name-base uuid using SHA1 hashing
struct CreateNamedSha1UUIDVisitor: public boost::static_visitor<RDFTTYPE>, public NoCallbackNeeded
{
  CreateNamedSha1UUIDVisitor(ReteSession * rs, BetaRow const* br): rs(rs), br(br) {}
  template<class T>RDFTTYPE operator()(T lhs)const{if(br==nullptr) return rdf::Null(); else RETE_EXCEPTION("Invalid arguments for uuid_md5: ("<<lhs<<")");};
  RDFTTYPE operator()(rdf::LString lhs)const
  {
    return rdf::LString(rdf::create_sha1_uuid(std::move(lhs.data)));
  }
  ReteSession * rs;
  BetaRow const* br;
};

} // namespace jets::rete
#endif // JETS_RETE_EXPR_OP_STRINGS_H
