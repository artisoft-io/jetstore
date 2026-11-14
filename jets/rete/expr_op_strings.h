#ifndef JETS_RETE_EXPR_OP_STRINGS_H
#define JETS_RETE_EXPR_OP_STRINGS_H

#include <cctype>
#include <cstdint>
#include <type_traits>
#include <algorithm>
#include <string>
#include <memory>
#include <utility>
#include <vector>
#include <regex>
#include <unordered_map>

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

// The compiled patterns of literal_regex (rules_benchmark C15). Each evaluation compiled
// its pattern, 1.7 to 2.3% of a production-shaped session, though a rule's pattern is
// nearly always a constant. A cache of the calling thread: the sessions of a pool run on
// their own threads, so nothing is shared and nothing is locked. It holds at most
// kRegexCacheCapacity patterns and is emptied when full, which bounds a rule whose pattern
// comes from the data. A pattern that does not compile throws, as it did, and is not kept.
constexpr std::size_t kRegexCacheCapacity = 256;

struct RegexCache {
  std::unordered_map<std::string, std::regex> patterns;

  std::regex const& get(std::string const& pattern)
  {
    auto itor = this->patterns.find(pattern);
    if(itor != this->patterns.end()) return itor->second;
    std::regex compiled(pattern);
    if(this->patterns.size() >= kRegexCacheCapacity) this->patterns.clear();
    return this->patterns.emplace(pattern, std::move(compiled)).first->second;
  }
};

inline RegexCache &
thread_regex_cache()
{
  thread_local RegexCache cache;
  return cache;
}

// RegexVisitor
// --------------------------------------------------------------------------------------
struct RegexVisitor: public boost::static_visitor<RDFTTYPE>, public NoCallbackNeeded
{
  RegexVisitor(ReteSession * rs, BetaRow const* br): rs(rs), br(br) {}
  RegexVisitor(): rs(nullptr), br(nullptr) {}
  template<class T, class U> RDFTTYPE operator()(T lhs, U rhs)const{if(br==nullptr) return rdf::Null(); else RETE_EXCEPTION("Invalid arguments for REGEX: ("<<lhs<<", "<<rhs<<")");};
  RDFTTYPE operator()(rdf::LString lhs, rdf::LString rhs)const
  {
    std::regex const& expr_regex = thread_regex_cache().get(rhs.data);
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

// UTF-8 characters
// --------------------------------------------------------------------------------------
// The length in bytes of the valid UTF-8 sequence starting at text[i], 1 to 4, or 0 when
// none starts there: a stray continuation byte, a truncated sequence, an overlong form,
// a surrogate (U+D800 to U+DFFF), or a code point above U+10FFFF. i < text.size().
inline std::size_t utf8_char_len(std::string const& text, std::size_t i)
{
  auto const n = text.size();
  auto const b0 = static_cast<unsigned char>(text[i]);
  std::size_t len = 0;
  unsigned char lo = 0x80, hi = 0xBF;  // the range of the second byte
  if(b0 < 0x80)                     len = 1;
  else if(b0 >= 0xC2 && b0 <= 0xDF) len = 2;
  else if(b0 == 0xE0)               { len = 3; lo = 0xA0; }
  else if(b0 >= 0xE1 && b0 <= 0xEC) len = 3;
  else if(b0 == 0xED)               { len = 3; hi = 0x9F; }
  else if(b0 >= 0xEE && b0 <= 0xEF) len = 3;
  else if(b0 == 0xF0)               { len = 4; lo = 0x90; }
  else if(b0 >= 0xF1 && b0 <= 0xF3) len = 4;
  else if(b0 == 0xF4)               { len = 4; hi = 0x8F; }
  else return 0;
  if(i + len > n) return 0;
  for(std::size_t k = 1; k < len; k++) {
    auto const b = static_cast<unsigned char>(text[i + k]);
    if(k == 1 ? (b < lo || b > hi) : (b < 0x80 || b > 0xBF)) return 0;
  }
  return len;
}

// Splits text into its UTF-8 characters (code points), each as its byte sequence, in
// order. Returns false, with chars left partial, when the text is not valid UTF-8 (see
// utf8_char_len). replace_char_of reads jets:replace_chars with it, and refuses text
// that is not valid UTF-8.
inline bool split_utf8_chars(std::string const& text, std::vector<std::string> & chars)
{
  auto const n = text.size();
  std::size_t i = 0;
  while(i < n) {
    auto const len = utf8_char_len(text, i);
    if(len == 0) return false;
    chars.emplace_back(text, i, len);
    i += len;
  }
  return true;
}

// TextUnits
// --------------------------------------------------------------------------------------
// The unit in which length_of, substring_of and char_at count lengths and positions: the
// character, a Unicode code point, read from the text's UTF-8. This is the one place
// that decides it.
//  - a valid UTF-8 sequence, of 1 to 4 bytes, is one character;
//  - a byte that does not start a valid sequence (see utf8_char_len) is one character on
//    its own, kept as it is: text that is not valid UTF-8 never fails a rule and never
//    loses a byte;
//  - there is no normalisation: a letter followed by a combining accent (e, then U+0301)
//    is two characters, where the precomposed letter (U+00E9) is one.
// Until 2026-11-14 the unit was the byte, so on text that is not ASCII a length counted
// each byte of a multi-byte character and a cut could split one. ASCII is unchanged.
//  - size: the number of characters in text;
//  - slice: the bytes of the characters [from, from + count) of text, where the caller
//    has already checked 0 <= from, 0 <= count and from + count <= size(text).
struct TextUnits
{
  // The length in bytes of the character starting at text[i], i < text.size()
  static std::size_t char_len(std::string const& text, std::size_t i)
  {
    auto const len = utf8_char_len(text, i);
    return len == 0 ? 1 : len;
  }
  static std::int64_t size(std::string const& text)
  {
    std::int64_t count = 0;
    for(std::size_t i = 0; i < text.size(); i += char_len(text, i)) count++;
    return count;
  }
  static std::string slice(std::string const& text, std::int64_t from, std::int64_t count)
  {
    auto const n = text.size();
    std::size_t i = 0;
    for(std::int64_t k = 0; k < from && i < n; k++) i += char_len(text, i);
    std::size_t j = i;
    for(std::int64_t k = 0; k < count && j < n; k++) j += char_len(text, j);
    return text.substr(i, j - i);
  }
};

// LengthOfVisitor
// --------------------------------------------------------------------------------------
// (length_of ?text): the number of characters in ?text, counted as TextUnits counts them,
// so that length_of, substring_of and char_at agree. Until 2026-11-14 it was the number
// of bytes, the same on ASCII text.
struct LengthOfVisitor: public boost::static_visitor<RDFTTYPE>, public NoCallbackNeeded
{
  LengthOfVisitor(ReteSession * rs, BetaRow const* br): rs(rs), br(br) {}
  template<class T>RDFTTYPE operator()(T lhs)const{if(br==nullptr) return rdf::Null(); else RETE_EXCEPTION("Invalid arguments for length_of: ("<<lhs<<")");};
  RDFTTYPE operator()(rdf::LString lhs)const
  {
    return rdf::LInt32(static_cast<std::int32_t>(TextUnits::size(lhs.data)));
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
  template<class T, class U> RDFTTYPE operator()(T lhs, U rhs)const{if(br==nullptr) return rdf::Null(); else RETE_EXCEPTION("Invalid arguments for ends_with: ("<<lhs<<", "<<rhs<<")");};

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
    // Note: if jets:from + jets:length > the length of rhs.data then return the available characters of rhs.data
    // Note: if jets:length < 0 then remove jets:length from the end of the string
    // Note: an empty string is returned when the cut is longer than the text, that is
    //       when jets:from is past the end, when jets:from is negative, or when
    //       jets:length < 0 drops more than what remains after jets:from
    // Note: positions and lengths count characters (code points), as TextUnits counts
    //       them, so a cut never splits a multi-byte UTF-8 character. Until 2026-11-14
    //       they counted bytes
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
    // Signed 64-bit arithmetic throughout: neither the size of the text nor a
    // 32-bit from or length can wrap around in it
    std::int64_t const from = boost::get<rdf::LInt32>(from_obj)->data;
    std::int64_t const length = boost::get<rdf::LInt32>(length_obj)->data;
    std::int64_t const sz = TextUnits::size(rhs.data);
    if (from < 0 || from >= sz) {
      return rdf::LString{ std::string() };
    }
    std::int64_t const end = std::min(sz, length < 0 ? sz + length : from + length);
    if (end <= from) {
      return rdf::LString{ std::string() };
    }
    return rdf::LString{ TextUnits::slice(rhs.data, from, end - from) };
  }

  ReteSession * rs;
  BetaRow const* br;
};

// ReplaceCharOfVisitor
// --------------------------------------------------------------------------------------
// (config replace_char_of text): every occurrence in text of each character of
// jets:replace_chars is replaced by jets:replace_with.
//  - a character is a UTF-8 character (code point), replaced as its whole byte sequence,
//    so a character outside ASCII, such as "é", leaves the characters that share its
//    lead byte, such as "à", untouched. Until 2026-11-13 jets:replace_chars was read
//    byte by byte, which broke such characters and left invalid UTF-8 behind; ASCII
//    is unchanged, a character there being one byte. The retired Go engine
//    (ReplaceCharOp, at go-engine-final) worked on characters too;
//  - the characters are replaced in turn, in the order of jets:replace_chars, each over
//    the text the previous turns produced, so a later character is also replaced where
//    an earlier turn inserted it with jets:replace_with;
//  - an empty jets:replace_with removes the characters; an empty jets:replace_chars
//    returns the text unchanged;
//  - a jets:replace_chars or jets:replace_with that is missing or not text, or a
//    jets:replace_chars that is not valid UTF-8, inserts (jets:iState jets:exception
//    "...") and the expression is null.
struct ReplaceCharOfVisitor: public boost::static_visitor<RDFTTYPE>, public NoCallbackNeeded
{
  ReplaceCharOfVisitor(ReteSession * rs, BetaRow const* br): rs(rs), br(br) {}
  ReplaceCharOfVisitor(): rs(nullptr), br(nullptr) {}
  template<class T, class U> RDFTTYPE operator()(T lhs, U rhs)const{if(br==nullptr) return rdf::Null(); else RETE_EXCEPTION("Invalid arguments for replace_char_of: ("<<lhs<<", "<<rhs<<")");};

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
    auto const& replace_chars = boost::get<rdf::LString>(replace_chars_obj)->data;
    auto const& replace_with = boost::get<rdf::LString>(replace_with_obj)->data;
    std::vector<std::string> chars;
    if (not split_utf8_chars(replace_chars, chars)) {
      sess->insert(jr->jets__istate, jr->jets__exception, rmgr->create_literal("error: jets:replace_chars is not valid UTF-8 for operator replace_char_of "+rhs.data));
      return rdf::Null();
    }
    auto str = std::move(rhs.data);
    for(auto const& c: chars) {
      boost::replace_all(str, c, replace_with);
    }
    return rdf::LString{ str };
  }

  ReteSession * rs;
  BetaRow const* br;
};

// CharAtVisitor
// --------------------------------------------------------------------------------------
// (?text char_at ?index): the character of ?text at ?index, counted from 0, as a text of
// one character.
//  - characters are counted as TextUnits counts them, as substring_of's positions are:
//    a whole code point, however many bytes it takes in UTF-8, and a byte that does not
//    start a valid sequence on its own ((?text char_at 0) on "été" is "é"). Until
//    2026-11-14 the index counted bytes, and the result could be one byte of a
//    multi-byte character;
//  - an index at or past the end of the text, or negative, is null. substring_of gives
//    "" for a cut outside the text; char_at keeps null, which it has always given, as a
//    rule may already depend on it (is_literal, length_of, == "", or a consequent
//    writing the value all tell null from "");
//  - the index must be an int (32-bit) and the text a text: anything else is the
//    catch-all below, which throws in a session's rule.
// Signed 64-bit arithmetic, as in substring_of: a negative int32 cannot wrap around.
struct CharAtVisitor: public boost::static_visitor<RDFTTYPE>, public NoCallbackNeeded
{
  CharAtVisitor(ReteSession * rs, BetaRow const* br): rs(rs), br(br) {}
  CharAtVisitor(): rs(nullptr), br(nullptr) {}
  template<class T, class U> RDFTTYPE operator()(T lhs, U rhs)const{if(br==nullptr) return rdf::Null(); else RETE_EXCEPTION("Invalid arguments for char_at: ("<<lhs<<", "<<rhs<<")");};

  RDFTTYPE operator()(rdf::LString       lhs, rdf::LInt32       rhs)const
  {
    std::int64_t const index = rhs.data;
    if(index < 0 || index >= TextUnits::size(lhs.data)) {
      return rdf::Null();
    }
    return rdf::LString{ TextUnits::slice(lhs.data, index, 1) };
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
