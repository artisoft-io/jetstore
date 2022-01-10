#ifndef JETS_RDF_AST_H
#define JETS_RDF_AST_H

#include <cinttypes>
#include <type_traits>
#include <string>
#include <string_view>
#include <algorithm>
#include <memory>
#include <utility>
#include <iostream>

#include "absl/hash/hash.h"
#include "boost/variant.hpp"

#include "jets/rdf/other/fcmp.h"

namespace jets::rdf {

/**
 * @brief rdf ast
 * Define the rdf data types as a typed variant so to be able to construct
 * efficient data structures.
 * The rdf types are:
 *   - named_resource (resource)
 *   - unnamed_resource (blank_node)
 *   - literals (string, int32, uint32, int64, uint64, bool, double, date, date_time, duration)
 *   - null
 * 
 */
struct RDFNull {
  using is_non_resource = std::true_type;
  using is_non_literal = std::true_type;
  RDFNull() = default;
  RDFNull(RDFNull const&) = default;
  RDFNull(RDFNull &&) = default;
  inline RDFNull& operator=(RDFNull const&){return *this;}
  inline bool operator==(RDFNull  const& rhs) const{return true;}
  template<typename W>
  inline bool operator==(W const& rhs)const{return false;}
  inline bool operator!=(RDFNull  const& rhs) const{return false;}
};

inline std::ostream & operator<<(std::ostream & out, RDFNull bn)
{
  out <<"NULL";
  return out;
}

// BlankNode is the default-construct of the RdfAst, see (https://www.boost.org/doc/libs/1_77_0/doc/html/variant/tutorial.html)
// it must have a default-construct constructor.
struct BlankNode {
  using is_resource = std::true_type;
  using is_non_literal = std::true_type;
  BlankNode() = default;
  explicit BlankNode(int32_t n):key(n){}
  BlankNode(BlankNode const&) = default;
  BlankNode(BlankNode &&) = default;
  inline BlankNode& operator=(BlankNode const&){return *this;}
  inline bool operator==(BlankNode  const& rhs) const{return this->key == rhs.key;}
  template<typename W>
  inline bool operator==(W const& rhs)const{return false;}
  inline bool operator!=(BlankNode  const& rhs) const{return this->key != rhs.key;}
  inline bool operator< (BlankNode  const& rhs) const{return this->key <  rhs.key;}
  int32_t key;
};

inline std::ostream & operator<<(std::ostream & out, BlankNode bn)
{
  out <<"bn"<<"<"<<bn.key<<">";
  return out;
}

// NamedResource -- the common rdf named resource
struct NamedResource {
  using is_resource = std::true_type;
  using is_non_literal = std::true_type;
  NamedResource() = default;
  explicit NamedResource(std::string const& n):name(n){}
  explicit NamedResource(char const* n):name(n){}
  explicit NamedResource(std::string && n):name(std::forward<std::string>(n)){}
  NamedResource(NamedResource const& rhs):name(rhs.name){};
  NamedResource(NamedResource && rhs):name(std::forward<std::string>(rhs.name)){};
  inline NamedResource& operator=(NamedResource const& rhs){this->name = rhs.name; return *this;}
  inline NamedResource& operator=(NamedResource && rhs){this->name = std::forward<std::string>(rhs.name); return *this;}
  inline bool operator==(NamedResource  const& rhs) const{return this->name == rhs.name;}
  template<typename W>
  inline bool operator==(W const& rhs)const{return false;}
  inline bool operator!=(NamedResource  const& rhs) const{return not this->operator==(rhs);}
  inline bool operator< (NamedResource  const& rhs) const{return this->name < rhs.name;}
  std::string name;
};

inline std::ostream & operator<<(std::ostream & out, NamedResource const& r)
{
  out <<r.name;
  return out;
}

// Literal -- for each supported literal type
template<class T>
struct Literal {
  using is_non_resource = std::true_type;
  using is_literal = std::true_type;
  Literal() = default;
  explicit Literal(T const&v):data(v){}
  explicit Literal(T &&v):data(std::forward<T>(v)){}
  Literal(Literal const&) = default;
  Literal(Literal &&) = default;
  inline Literal& operator=(Literal const& rhs){this->data = rhs.data; return *this;}
  inline Literal& operator=(Literal && rhs){this->data = std::forward<T>(rhs.data); return *this;}
  inline bool operator==(Literal const& rhs)const{return this->data == rhs.data;}
  inline bool operator!=(Literal  const& rhs) const{return not this->operator==(rhs);}
  inline bool operator< (Literal  const& rhs) const{return this->data < rhs.data;}
  T data;
};

template<class T>
inline std::ostream & operator<<(std::ostream & out, Literal<T> const& r)
{
  out <<r.data;
  return out;
}

// ======================================================================================
// Literal Defined
// sizeof(int): 4
// sizeof(std::int32_t): 4
// sizeof(std::int64_t): 8
// sizeof(double): 8
// --------------------------------------------------------------------------------------
using LInt32  = Literal<std::int32_t>;
using LUInt32 = Literal<std::uint32_t>;
using LInt64  = Literal<std::int64_t>;
using LUInt64 = Literal<std::uint64_t>;
using LDouble = Literal<double>;
using LString = Literal<std::string>;

// Functions to make double literals comparable
// --------------------------------------------------------------------------------------
inline double
round_to_digits(double value, int digits)
{
  if(value == 0.0) return 0.0;
  if(not std::isfinite(value)) return value;
  double factor = pow(10.0, digits - ceil(log10(fabs(value))));
  return round(value * factor) / factor;
}

inline bool
is_eq(LDouble const& lhs, LDouble const& rhs)
{
  if(std::isnan(lhs.data) or std::isnan(rhs.data)) return false;
  if(std::isinf(lhs.data) or std::isinf(rhs.data)) return false;
  return fcmp(round_to_digits(lhs.data, 15), round_to_digits(rhs.data, 15), 
    std::numeric_limits<double>::epsilon()) == 0;
}

inline bool
is_gt(LDouble const& lhs, LDouble const& rhs)
{
  if(std::isnan(lhs.data) or std::isnan(rhs.data)) return false;
  if(std::isinf(lhs.data) or std::isinf(rhs.data)) return false;
  return fcmp(round_to_digits(lhs.data, 15), round_to_digits(rhs.data, 15), 
    std::numeric_limits<double>::epsilon()) > 0;
}

inline bool
is_ge(LDouble const& lhs, LDouble const& rhs)
{
  if(std::isnan(lhs.data) or std::isnan(rhs.data)) return false;
  if(std::isinf(lhs.data) or std::isinf(rhs.data)) return false;
  return fcmp(round_to_digits(lhs.data, 15), round_to_digits(rhs.data, 15), 
    std::numeric_limits<double>::epsilon()) >= 0;
}

inline bool
is_lt(LDouble const& lhs, LDouble const& rhs)
{
  if(std::isnan(lhs.data) or std::isnan(rhs.data)) return false;
  if(std::isinf(lhs.data) or std::isinf(rhs.data)) return false;
  return fcmp(round_to_digits(lhs.data, 15), round_to_digits(rhs.data, 15), 
    std::numeric_limits<double>::epsilon()) < 0;
}

inline bool
is_le(LDouble const& lhs, LDouble const& rhs)
{
  if(std::isnan(lhs.data) or std::isnan(rhs.data)) return false;
  if(std::isinf(lhs.data) or std::isinf(rhs.data)) return false;
  return fcmp(round_to_digits(lhs.data, 15), round_to_digits(rhs.data, 15), 
    std::numeric_limits<double>::epsilon()) <= 0;
}

// ======================================================================================
// Utility Functions
// -----------------------------------------------------------------------------
inline std::string 
trim(std::string const& str)
{
  static constexpr char kWhitespaces[] = " \t\n\r";
  if(str.empty()) return {};
  auto p1 = str.find_first_not_of(&kWhitespaces[0], 0, sizeof(kWhitespaces));
  if(p1 == std::string::npos) return {};
  auto p2 = str.find_last_not_of(&kWhitespaces[0], std::string::npos, sizeof(kWhitespaces));
  if(p2 == std::string::npos) return {};
  return str.substr(p1, p2-p1+1);
}

// ======================================================================================
// Main AST Class
// -----------------------------------------------------------------------------
enum rdf_ast_which_order {
    rdf_null_t                       = 0 ,
    rdf_blank_node_t                 = 1 ,
    rdf_named_resource_t             = 2 ,
    rdf_literal_int32_t              = 3 ,
    rdf_literal_uint32_t             = 4 ,
    rdf_literal_int64_t              = 5 ,
    rdf_literal_uint64_t             = 6 ,
    rdf_literal_double_t             = 7 ,
    rdf_literal_string_t             = 8 
};

//* NOTE: If updated, MUST update ast_which_order and possibly ast_sort_order
// ======================================================================================
using RdfAstType = boost::variant< 
    RDFNull,
    BlankNode, 
    NamedResource,
    LInt32,
    LUInt32,
    LInt64,
    LUInt64,
    LDouble,
    LString >;

// ======================================================================================
// r_index
// -----------------------------------------------------------------------------
using r_index = RdfAstType const *;
inline std::ostream & operator<<(std::ostream & out, r_index const& r)
{
  if(r) {
    out <<*r;
  } else {
    out << "NULL";
  }
  return out;
}

// ======================================================================================
// Rptr
// -----------------------------------------------------------------------------
using Rptr = std::shared_ptr<RdfAstType>;
inline r_index to_r_index(Rptr r)
{
  return r.get();
}
inline std::ostream & operator<<(std::ostream & out, Rptr const& r)
{
  if(r) {
    out <<*r;
  } else {
    out << "NULL";
  }
  return out;
}

// ======================================================================================
// triple template class
// -----------------------------------------------------------------------------
template<typename T>
struct TripleBase {
  TripleBase() :subject(), predicate(), object() {}
  TripleBase(T const&s, T const&p, T const&o) : subject(s), predicate(p), object(o) {}
  TripleBase(T &&s, T &&p, T &&o) : subject(std::forward<T>(s)), predicate(std::forward<T>(p)), object(std::forward<T>(o)) {}
  TripleBase(TripleBase const&) = default;
  TripleBase(TripleBase &&) = default;
  inline TripleBase& operator=(TripleBase const& rhs)
  {
    this->subject = rhs.subject; 
    this->predicate = rhs.predicate; 
    this->object = rhs.object; 
    return *this;
  }
  inline T get(int pos)const 
  {
    if(pos < 0 or pos > 2) return nullptr;
    switch (pos) {
    case 0   : return subject;
    case 1   : return predicate;
    case 2   : return object;
    default: return nullptr;
    }
  }
  inline bool operator==(TripleBase  const& rhs) const
  {
    if(this->subject != rhs.subject) return false;
    if(this->predicate != rhs.predicate) return false;
    if(this->object != rhs.object) return false;
    return true;
  }
  T subject;
  T predicate;
  T object;
};

// rdf::Triple class for convenience in some api
using Triple = TripleBase<r_index>;
inline std::ostream & operator<<(std::ostream & out, Triple const& t3)
{
  out << "("<<t3.subject<<","<<t3.predicate<<","<<t3.object<<")";
  return out;
}

inline std::string
to_string(Triple const& t)
{
  std::ostringstream out;
  out << t;
  return out.str();
}

// Function to compute hash value for rdf data
// ======================================================================================
template <typename H>
H AbslHashValue(H h, const Rptr& rptr) 
{
  auto &m = *rptr;
  switch (m.which()) {
  case rdf_null_t             : return H::combine(std::move(h), 0);
  case rdf_blank_node_t       : return H::combine(std::move(h), boost::get<BlankNode    >(m).key);
  case rdf_named_resource_t   : return H::combine(std::move(h), boost::get<NamedResource>(m).name);
  case rdf_literal_int32_t    : return H::combine(std::move(h), boost::get<LInt32       >(m).data);
  case rdf_literal_uint32_t   : return H::combine(std::move(h), boost::get<LUInt32      >(m).data);
  case rdf_literal_int64_t    : return H::combine(std::move(h), boost::get<LInt64       >(m).data);
  case rdf_literal_uint64_t   : return H::combine(std::move(h), boost::get<LUInt64      >(m).data);
  case rdf_literal_double_t   : return H::combine(std::move(h), boost::get<LDouble      >(m).data);
  case rdf_literal_string_t   : return H::combine(std::move(h), boost::get<LString      >(m).data);
  default: return H::combine(std::move(h), 0);
  }
}

inline bool 
operator==(const Rptr& lhs, const Rptr& rhs) {
  return *lhs == *rhs;
}

// ======================================================================================
// RdfAstType visitors
// -----------------------------------------------------------------------------
struct get_key_visitor: public boost::static_visitor<int32_t>
{
  int32_t operator()(RDFNull       const& )const{return 0;}
  int32_t operator()(BlankNode     const&v)const{return v.key;}
  int32_t operator()(NamedResource const& )const{return 0;}
  int32_t operator()(LInt32        const& )const{return 0;}
  int32_t operator()(LUInt32       const& )const{return 0;}
  int32_t operator()(LInt64        const& )const{return 0;}
  int32_t operator()(LUInt64       const& )const{return 0;}
  int32_t operator()(LDouble       const& )const{return 0;}
  int32_t operator()(LString       const& )const{return 0;}
};
inline int32_t 
get_key(r_index r)
{
  if(not r) return 0;
  return boost::apply_visitor(get_key_visitor(), *r);
}

struct get_name_visitor: public boost::static_visitor<std::string>
{
  std::string operator()(RDFNull       const& )const{return {};}
  std::string operator()(BlankNode     const&v)const{return {"bn("+std::to_string(v.key)+")"};}
  std::string operator()(NamedResource const&v)const{return v.name;}
  std::string operator()(LInt32        const& )const{return {};}
  std::string operator()(LUInt32       const& )const{return {};}
  std::string operator()(LInt64        const& )const{return {};}
  std::string operator()(LUInt64       const& )const{return {};}
  std::string operator()(LDouble       const& )const{return {};}
  std::string operator()(LString       const& )const{return {};}
};
inline std::string 
get_name(r_index r)
{
  if(not r) return {"NULL"};
  return boost::apply_visitor(get_name_visitor(), *r);
}

struct to_bool_visitor: public boost::static_visitor<bool>
{
  static constexpr char kFalse[] = "false";

  bool operator()(RDFNull       const& )const{return false;}
  bool operator()(BlankNode     const&v)const{return true;}
  bool operator()(NamedResource const&v)const{return true;}
  bool operator()(LInt32        const&v)const{return v.data;}
  bool operator()(LUInt32       const&v)const{return v.data;}
  bool operator()(LInt64        const&v)const{return v.data;}
  bool operator()(LUInt64       const&v)const{return v.data;}
  bool operator()(LDouble       const&v)const{return v.data;}
  bool operator()(LString       const&v)const
  {
    if(not v.data.empty()) {
      if(v.data.size() == 1) {
        if(v.data[0] == '0') return false;
        if(std::tolower(v.data[0]) == 'f') return false;
        return true;
      } else {
        if(v.data.size() == sizeof(kFalse)-1) {
          return not std::equal(
            v.data.begin(), v.data.end(), &kFalse[0], 
            [](char const& c1, char const& c2) {return std::tolower(c1) == std::tolower(c2); } 
          );
        }
        return true;
      }
    }
    return false;
  }
};
inline bool
to_bool(r_index r)
{
  if(not r) return false;
  return boost::apply_visitor(to_bool_visitor(), *r);
}

inline bool
to_bool(RdfAstType v)
{
  return boost::apply_visitor(to_bool_visitor(), v);
}

inline RdfAstType True() { return LInt32(1);}
inline RdfAstType False() { return LInt32(0);}

inline RdfAstType Null() { return RDFNull();}

// ==================================================================================
// Resource and Literals Factory constructors
// ----------------------------------------------------------------------------------
inline Rptr mkNull()                        { return std::make_shared<RdfAstType>(RDFNull()); }
inline Rptr mkBlankNode(int key)            { return std::make_shared<RdfAstType>(BlankNode(key)); }

inline Rptr mkResource(std::string n)       
{ 
  NamedResource r;
  std::swap(r.name, n);
  return std::make_shared<RdfAstType>(r); 
}

inline Rptr mkResource(std::string_view n)
{ 
  NamedResource r;
  r.name = n;
  return std::make_shared<RdfAstType>(r); 
}

inline Rptr mkResource(const char * nptr)
{ 
  NamedResource r;
  r.name = nptr;
  return std::make_shared<RdfAstType>(r); 
}

inline Rptr mkLiteral(std::int32_t v)       { return std::make_shared<RdfAstType>(LInt32(v)); }
inline Rptr mkLiteral(std::uint32_t v)      { return std::make_shared<RdfAstType>(LUInt32(v)); }
inline Rptr mkLiteral(std::int64_t v)       { return std::make_shared<RdfAstType>(LInt64(v)); }
inline Rptr mkLiteral(std::uint64_t v)      { return std::make_shared<RdfAstType>(LUInt64(v)); }
inline Rptr mkLiteral(double v)             { return std::make_shared<RdfAstType>(LDouble(v)); }

inline Rptr mkLiteral(std::string n)       
{ 
  LString l;
  std::swap(l.data, n);
  return std::make_shared<RdfAstType>(l); 
}

inline Rptr mkLiteral(std::string_view n)       
{ 
  LString l;
  l.data = n;
  return std::make_shared<RdfAstType>(l); 
}

inline Rptr mkLiteral(const char * nptr)       
{ 
  LString l;
  l.data = nptr;
  return std::make_shared<RdfAstType>(l); 
}
// ----------------------------------------------------------------------------------

// // ==================================================================================
// // Template to restrict arg to functions taking rdf data as argument
// // typename R is the return type of the function while enumared type T are
// // the acceptable literal data type
// // ----------------------------------------------------------------------------------	
// template<typename T, typename R> struct literal_restrictor                  {                 };
// template<typename R>           struct literal_restrictor<BlankNode, R>      {typedef R result;};
// template<typename R>           struct literal_restrictor<NamedResource, R>  {typedef R result;};
// template<typename R>           struct literal_restrictor<LInt32, R>         {typedef R result;};
// template<typename R>           struct literal_restrictor<LUInt32, R>        {typedef R result;};
// template<typename R>           struct literal_restrictor<LInt64, R>         {typedef R result;};
// template<typename R>           struct literal_restrictor<LUInt64, R>        {typedef R result;};
// template<typename R>           struct literal_restrictor<LDouble, R>        {typedef R result;};
// template<typename R>           struct literal_restrictor<LString, R>        {typedef 

// ==================================================================================
// Template to restrict arg to functions taking rdf data as argument
// typename R is the return type of the function while enumared type T are
// the acceptable literal data type and V is the signature of the literal data type
// ----------------------------------------------------------------------------------	
template<class T, class R> struct literal_restrictor{};
template<class R> struct literal_restrictor< void*, R>         {typedef R result;};
template<class R> struct literal_restrictor< int32_t, R>       {typedef R result;};
template<class R> struct literal_restrictor< uint32_t,  R>     {typedef R result;};
template<class R> struct literal_restrictor< int64_t, R>       {typedef R result;};
template<class R> struct literal_restrictor< uint64_t, R>      {typedef R result;};
template<class R> struct literal_restrictor< double, R>        {typedef R result;};
template<class R> struct literal_restrictor< std::string, R>   {typedef R result;};
// ----------------------------------------------------------------------------------
template<class T, class R> struct resource_restrictor{};
template<class R> struct resource_restrictor< void*, R>       {typedef R result;};
template<class R> struct resource_restrictor< int32_t, R>     {typedef R result;};
template<class R> struct resource_restrictor< std::string, R> {typedef R result;};
// ----------------------------------------------------------------------------------

} // namespace jets::rdf
#endif // JETS_RDF_AST_H
