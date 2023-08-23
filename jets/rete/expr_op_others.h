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
#include "../rete/lookup_sql_helper.h"

// This file contains basic operator used in rule expression 
// see ExprUnaryOp and ExprBinaryOp classes.
namespace jets::rete {

using RDFTTYPE = rdf::RdfAstType;

// LookupVisitor
// --------------------------------------------------------------------------------------
struct LookupVisitor: public boost::static_visitor<RDFTTYPE>, public NoCallbackNeeded
{
  // This operator is used as: lookup_uri lookup key where lookup_uri is a resource and key is a text literal or a resource
  LookupVisitor(ReteSession * rs, BetaRow const* br): rs(rs), br(br) {}
  template<class T, class U> RDFTTYPE operator()(T lhs, U rhs) const {if(br==nullptr) return rdf::Null(); else RETE_EXCEPTION("Invalid arguments for lookup: ("<<lhs<<", "<<rhs<<")");};
  RDFTTYPE operator()(rdf::NamedResource lhs, rdf::NamedResource rhs)const{return this->lookup(lhs.name, rhs.name);}
  RDFTTYPE operator()(rdf::NamedResource lhs, rdf::LInt32        rhs)const{return this->lookup(lhs.name, std::to_string(rhs.data));}
  RDFTTYPE operator()(rdf::NamedResource lhs, rdf::LUInt32       rhs)const{return this->lookup(lhs.name, std::to_string(rhs.data));}
  RDFTTYPE operator()(rdf::NamedResource lhs, rdf::LInt64        rhs)const{return this->lookup(lhs.name, std::to_string(rhs.data));}
  RDFTTYPE operator()(rdf::NamedResource lhs, rdf::LUInt64       rhs)const{return this->lookup(lhs.name, std::to_string(rhs.data));}
  RDFTTYPE operator()(rdf::NamedResource lhs, rdf::LString       rhs)const{return this->lookup(lhs.name, rhs.data);}

  RDFTTYPE operator()(rdf::LString       lhs, rdf::NamedResource rhs)const{return this->lookup(lhs.data, rhs.name);}
  RDFTTYPE operator()(rdf::LString       lhs, rdf::LInt32        rhs)const{return this->lookup(lhs.data, std::to_string(rhs.data));}
  RDFTTYPE operator()(rdf::LString       lhs, rdf::LUInt32       rhs)const{return this->lookup(lhs.data, std::to_string(rhs.data));}
  RDFTTYPE operator()(rdf::LString       lhs, rdf::LInt64        rhs)const{return this->lookup(lhs.data, std::to_string(rhs.data));}
  RDFTTYPE operator()(rdf::LString       lhs, rdf::LUInt64       rhs)const{return this->lookup(lhs.data, std::to_string(rhs.data));}
  RDFTTYPE operator()(rdf::LString       lhs, rdf::LString       rhs)const{return this->lookup(lhs.data, rhs.data);}
  
  RDFTTYPE lookup(std::string const& lookup_tbl, std::string const& key)const
  {
    RDFTTYPE out;
    auto helper = this->rs->rule_ms()->get_lookup_sql_helper();
    if(not helper) {
      RETE_EXCEPTION("Invalid lookup helper! Arguments: ("<<lookup_tbl<<", "<<key<<")");
    }
    if(helper->lookup(this->rs, lookup_tbl, key, &out)) {
      RETE_EXCEPTION("ERROR while calling lookup with arguments: ("<<lookup_tbl<<", "<<key<<")");
      return rdf::Null();
    }
    return out;
  }

  ReteSession * rs;
  BetaRow const* br;
};

// LookupRandVisitor
// --------------------------------------------------------------------------------------
// Visitor used to lookup table by random key
struct LookupRandVisitor: public boost::static_visitor<RDFTTYPE>, public NoCallbackNeeded
{
  explicit
  LookupRandVisitor(ReteSession * rs, BetaRow const* br): rs(rs){}
  template<class T> RDFTTYPE operator()(T) const{return rdf::Null();}
  RDFTTYPE operator()(rdf::NamedResource const&v)const{return this->lookup(v.name);}
  RDFTTYPE operator()(rdf::LString       const&v)const{return this->lookup(v.data);}
  
  RDFTTYPE lookup(std::string const& lookup_tbl)const
  {
    RDFTTYPE out;
    auto helper = this->rs->rule_ms()->get_lookup_sql_helper();
    if(not helper) {
      RETE_EXCEPTION("Invalid lookup helper! Arguments: ("<<lookup_tbl<<")");
    }
    if(helper->lookup_rand(this->rs, lookup_tbl, &out)) {
      RETE_EXCEPTION("ERROR while calling lookup_rand with arguments: ("<<lookup_tbl<<")");
      return rdf::Null();
    }
    return out;
  }

  ReteSession * rs;
};

// MultiLookupVisitor
// --------------------------------------------------------------------------------------
struct MultiLookupVisitor: public boost::static_visitor<RDFTTYPE>, public NoCallbackNeeded
{
  // This operator is used as: lookup_uri lookup key where lookup_uri is a resource and key is a text literal or a resource
  MultiLookupVisitor(ReteSession * rs, BetaRow const* br): rs(rs), br(br) {}
  template<class T, class U> RDFTTYPE operator()(T lhs, U rhs) const {if(br==nullptr) return rdf::Null(); else RETE_EXCEPTION("Invalid arguments for lookup rand: ("<<lhs<<", "<<rhs<<")");};
  RDFTTYPE operator()(rdf::NamedResource lhs, rdf::NamedResource rhs)const{return this->multi_lookup(lhs.name, rhs.name);}
  RDFTTYPE operator()(rdf::NamedResource lhs, rdf::LInt32        rhs)const{return this->multi_lookup(lhs.name, std::to_string(rhs.data));}
  RDFTTYPE operator()(rdf::NamedResource lhs, rdf::LUInt32       rhs)const{return this->multi_lookup(lhs.name, std::to_string(rhs.data));}
  RDFTTYPE operator()(rdf::NamedResource lhs, rdf::LInt64        rhs)const{return this->multi_lookup(lhs.name, std::to_string(rhs.data));}
  RDFTTYPE operator()(rdf::NamedResource lhs, rdf::LUInt64       rhs)const{return this->multi_lookup(lhs.name, std::to_string(rhs.data));}
  RDFTTYPE operator()(rdf::NamedResource lhs, rdf::LString       rhs)const{return this->multi_lookup(lhs.name, rhs.data);}

  RDFTTYPE operator()(rdf::LString       lhs, rdf::NamedResource rhs)const{return this->multi_lookup(lhs.data, rhs.name);}
  RDFTTYPE operator()(rdf::LString       lhs, rdf::LInt32        rhs)const{return this->multi_lookup(lhs.data, std::to_string(rhs.data));}
  RDFTTYPE operator()(rdf::LString       lhs, rdf::LUInt32       rhs)const{return this->multi_lookup(lhs.data, std::to_string(rhs.data));}
  RDFTTYPE operator()(rdf::LString       lhs, rdf::LInt64        rhs)const{return this->multi_lookup(lhs.data, std::to_string(rhs.data));}
  RDFTTYPE operator()(rdf::LString       lhs, rdf::LUInt64       rhs)const{return this->multi_lookup(lhs.data, std::to_string(rhs.data));}
  RDFTTYPE operator()(rdf::LString       lhs, rdf::LString       rhs)const{return this->multi_lookup(lhs.data, rhs.data);}
  
  RDFTTYPE multi_lookup(std::string const& lookup_tbl, std::string const& key)const
  {
    RDFTTYPE out;
    auto helper = this->rs->rule_ms()->get_lookup_sql_helper();
    if(not helper) {
      RETE_EXCEPTION("Invalid (multi)lookup helper! Arguments: ("<<lookup_tbl<<", "<<key<<")");
    }
    if(helper->multi_lookup(this->rs, lookup_tbl, key, &out)) {
      RETE_EXCEPTION("ERROR while calling multi_lookup with arguments: ("<<lookup_tbl<<", "<<key<<")");
      return rdf::Null();
    }
    return out;
  }

  ReteSession * rs;
  BetaRow const* br;
};

// MultiLookupRandVisitor
// --------------------------------------------------------------------------------------
struct MultiLookupRandVisitor: public boost::static_visitor<RDFTTYPE>, public NoCallbackNeeded
{
  // This operator is used as: lookup_uri lookup key where lookup_uri is a resource and key is a text literal or a resource
  MultiLookupRandVisitor(ReteSession * rs, BetaRow const* ): rs(rs) {}
  template<class T> RDFTTYPE operator()(T) const{return rdf::Null();}
  RDFTTYPE operator()(rdf::NamedResource const&v)const{return this->lookup(v.name);}
  RDFTTYPE operator()(rdf::LString       const&v)const{return this->lookup(v.data);}
  
  RDFTTYPE lookup(std::string const& lookup_tbl)const
  {
    RDFTTYPE out;
    auto helper = this->rs->rule_ms()->get_lookup_sql_helper();
    if(not helper) {
      RETE_EXCEPTION("Invalid lookup helper! Arguments: ("<<lookup_tbl<<")");
    }
    if(helper->multi_lookup_rand(this->rs, lookup_tbl, &out)) {
      RETE_EXCEPTION("ERROR while calling multi_lookup_rand with arguments: ("<<lookup_tbl<<")");
      return rdf::Null();
    }
    return out;
  }

  ReteSession * rs;
};

// AgeAsOfVisitor
// Calculate the age (in years), typical use:  (dob age_as_of serviceDate)
// where dob and serviceDate are date literals
// --------------------------------------------------------------------------------------
struct AgeAsOfVisitor: public boost::static_visitor<RDFTTYPE>, public NoCallbackNeeded
{
  AgeAsOfVisitor(ReteSession * rs, BetaRow const* br): rs(rs), br(br) {}
  AgeAsOfVisitor(): rs(nullptr), br(nullptr) {}
  template<class T, class U> RDFTTYPE operator()(T lhs, U rhs)const{if(br==nullptr) return rdf::Null(); else RETE_EXCEPTION("Invalid arguments for age_as_of: ("<<lhs<<", "<<rhs<<")");};

  RDFTTYPE operator()(rdf::LDate lhs, rdf::LDate rhs)const
  {
    auto birthday = lhs.data;
    auto asOf = rhs.data;
    int age = asOf.year() - birthday.year();
    if(asOf.day_of_year() < birthday.day_of_year()) age -= 1;
    return rdf::LInt32{ age };
  }

  ReteSession * rs;
  BetaRow const* br;
};

struct AgeInMonthsAsOfVisitor: public boost::static_visitor<RDFTTYPE>, public NoCallbackNeeded
{
  AgeInMonthsAsOfVisitor(ReteSession * rs, BetaRow const* br): rs(rs), br(br) {}
  AgeInMonthsAsOfVisitor(): rs(nullptr), br(nullptr) {}
  template<class T, class U> RDFTTYPE operator()(T lhs, U rhs)const{if(br==nullptr) return rdf::Null(); else RETE_EXCEPTION("Invalid arguments for age_in_months_as_of: ("<<lhs<<", "<<rhs<<")");};

  RDFTTYPE operator()(rdf::LDate lhs, rdf::LDate rhs)const
  {
    auto birthday = lhs.data;
    auto asOf = rhs.data;
    int years = asOf.year() - birthday.year();
    int months = 0;
    // Add the number of months in the last year
    if(asOf.day_of_year() <= birthday.day_of_year()) {
      years -= 1;
      months += asOf.month().as_number();
    } else {
      months += asOf.month().as_number() - birthday.month().as_number();
    }
    months += years * 12;
    return rdf::LInt32{ months };
  }

  ReteSession * rs;
  BetaRow const* br;
};

// ToTimestampVisitor
// --------------------------------------------------------------------------------------
struct ToTimestampVisitor: public boost::static_visitor<RDFTTYPE>, public NoCallbackNeeded
{
  ToTimestampVisitor(ReteSession * rs, BetaRow const* br): rs(rs), br(br) {}
  ToTimestampVisitor(): rs(nullptr), br(nullptr) {}
  template<class T>RDFTTYPE operator()(T lhs)const{if(br==nullptr) return rdf::Null(); else RETE_EXCEPTION("Invalid arguments for month_period_of: ("<<lhs<<")");};
  RDFTTYPE operator()(rdf::LDate lhs)const
  {
    return rdf::LInt64(rdf::to_timestamp(lhs.data));
  }
  ReteSession * rs;
  BetaRow const* br;
};

// MonthPeriodVisitor
// --------------------------------------------------------------------------------------
struct MonthPeriodVisitor: public boost::static_visitor<RDFTTYPE>, public NoCallbackNeeded
{
  MonthPeriodVisitor(ReteSession * rs, BetaRow const* br): rs(rs), br(br) {}
  MonthPeriodVisitor(): rs(nullptr), br(nullptr) {}
  template<class T>RDFTTYPE operator()(T lhs)const{if(br==nullptr) return rdf::Null(); else RETE_EXCEPTION("Invalid arguments for month_period_of: ("<<lhs<<")");};
  RDFTTYPE operator()(rdf::LDate lhs)const
  {
    // monthPeriod = (year-1970)*12 + month
    auto date = lhs.data;
    auto ymd = date.year_month_day();
    int month = ymd.month.as_number();
    int year = ymd.year;
    return rdf::LInt32((year-1970)*12 + month);
  }
  ReteSession * rs;
  BetaRow const* br;
};

// WeekPeriodVisitor
// --------------------------------------------------------------------------------------
struct WeekPeriodVisitor: public boost::static_visitor<RDFTTYPE>, public NoCallbackNeeded
{
  WeekPeriodVisitor(ReteSession * rs, BetaRow const* br): rs(rs), br(br) {}
  WeekPeriodVisitor(): rs(nullptr), br(nullptr) {}
  template<class T>RDFTTYPE operator()(T lhs)const{if(br==nullptr) return rdf::Null(); else RETE_EXCEPTION("Invalid arguments for month_period_of: ("<<lhs<<")");};
  RDFTTYPE operator()(rdf::LDate lhs)const
  {
    // secPerDay = 24 * 60 * 60 = 84400
    // secPerWeek = 7 * secPerDay = 604800
    // weekPeriod = int(unixTime/secPerWeek + 1)
    auto timestamp = rdf::to_timestamp(lhs.data);
    return rdf::LInt32(int((timestamp / 604800L) + 1L));
  }
  ReteSession * rs;
  BetaRow const* br;
};

// DayPeriodVisitor
// --------------------------------------------------------------------------------------
struct DayPeriodVisitor: public boost::static_visitor<RDFTTYPE>, public NoCallbackNeeded
{
  DayPeriodVisitor(ReteSession * rs, BetaRow const* br): rs(rs), br(br) {}
  DayPeriodVisitor(): rs(nullptr), br(nullptr) {}
  template<class T>RDFTTYPE operator()(T lhs)const{if(br==nullptr) return rdf::Null(); else RETE_EXCEPTION("Invalid arguments for month_period_of: ("<<lhs<<")");};
  RDFTTYPE operator()(rdf::LDate lhs)const
  {
    // secPerDay = 24 * 60 * 60 = 84400
    // dayPeriod = int(unixTime/secPerDay + 1)
    auto timestamp = rdf::to_timestamp(lhs.data);
    return rdf::LInt32(int((timestamp / 84400L) + 1L));
  }
  ReteSession * rs;
  BetaRow const* br;
};

// ToTypeOfOperator
// --------------------------------------------------------------------------------------
// Visitor used by ToTypeOfOperator to determine the rhs data type (return -1 if not valid type)
struct DataTypeVisitor: public boost::static_visitor<int>, public NoCallbackNeeded
{
  DataTypeVisitor(ReteSession * rs, BetaRow const* ): rs(rs){}
  int operator()(rdf::RDFNull       const& )const{return rdf::rdf_null_t;}
  int operator()(rdf::BlankNode     const&v)const{return rdf::rdf_blank_node_t;}
  int operator()(rdf::NamedResource const&v)const{return this->rs->rule_ms()->get_lookup_sql_helper()->type_of(this->rs, v.name);}
  int operator()(rdf::LInt32        const&v)const{return v.data;}
  int operator()(rdf::LUInt32       const&v)const{return v.data;}
  int operator()(rdf::LInt64        const&v)const{return v.data;}
  int operator()(rdf::LUInt64       const&v)const{return v.data;}
  int operator()(rdf::LDouble       const& )const{return rdf::rdf_literal_double_t;}
  int operator()(rdf::LString       const&v)const{return rdf::type_name2which(v.data);}
  int operator()(rdf::LDate         const& )const{return rdf::rdf_literal_date_t;}
  int operator()(rdf::LDatetime     const& )const{return rdf::rdf_literal_datetime_t;}
  ReteSession * rs;
};
struct CastVisitor: public boost::static_visitor<RDFTTYPE>, public NoCallbackNeeded
{
  CastVisitor(ReteSession * rs, int type): rs(rs), type(type){}
  RDFTTYPE operator()(rdf::RDFNull       const&v)const{return v;}
  RDFTTYPE operator()(rdf::BlankNode     const&v)const{return type==rdf::rdf_blank_node_t ? v : rdf::Null();}
  RDFTTYPE operator()(rdf::NamedResource const&v)const
  {
    switch (this->type) {
    case rdf::rdf_named_resource_t   : return v;
    case rdf::rdf_literal_string_t   : return rdf::LString(v.name);
    default: return rdf::Null(); // return null by default
    }
  }
  RDFTTYPE operator()(rdf::LInt32        const&v)const
  {
    switch (this->type) {
    case rdf::rdf_literal_string_t   : return rdf::LString(std::to_string(v.data));
    case rdf::rdf_literal_int32_t    : return v;
    case rdf::rdf_literal_uint32_t   : return rdf::LUInt32(boost::numeric_cast<uint32_t>(v.data));
    case rdf::rdf_literal_int64_t    : return rdf::LInt64(v.data);
    case rdf::rdf_literal_uint64_t   : return rdf::LUInt64(boost::numeric_cast<uint64_t>(v.data));
    case rdf::rdf_literal_double_t   : return rdf::LDouble(boost::numeric_cast<double>(v.data));
    default: return rdf::Null(); // return null by default
    }
  }
  RDFTTYPE operator()(rdf::LUInt32       const&v)const
  {
    switch (this->type) {
    case rdf::rdf_literal_string_t   : return rdf::LString(std::to_string(v.data));
    case rdf::rdf_literal_int32_t    : return rdf::LInt32(boost::numeric_cast<int32_t>(v.data));
    case rdf::rdf_literal_uint32_t   : return v;
    case rdf::rdf_literal_int64_t    : return rdf::LInt64(boost::numeric_cast<int64_t>(v.data));
    case rdf::rdf_literal_uint64_t   : return rdf::LUInt64(boost::numeric_cast<uint64_t>(v.data));
    case rdf::rdf_literal_double_t   : return rdf::LDouble(boost::numeric_cast<double>(v.data));
    default: return rdf::Null(); // return null by default
    }
  }
  RDFTTYPE operator()(rdf::LInt64        const&v)const
  {
    switch (this->type) {
    case rdf::rdf_literal_string_t   : return rdf::LString(std::to_string(v.data));
    case rdf::rdf_literal_int32_t    : return rdf::LInt32(boost::numeric_cast<int32_t>(v.data));
    case rdf::rdf_literal_uint32_t   : return rdf::LUInt32(boost::numeric_cast<uint32_t>(v.data));
    case rdf::rdf_literal_int64_t    : return v;
    case rdf::rdf_literal_uint64_t   : return rdf::LUInt64(boost::numeric_cast<uint64_t>(v.data));
    case rdf::rdf_literal_double_t   : return rdf::LDouble(boost::numeric_cast<double>(v.data));
    default: return rdf::Null(); // return null by default
    }
  }
  RDFTTYPE operator()(rdf::LUInt64       const&v)const
  {
    switch (this->type) {
    case rdf::rdf_literal_string_t   : return rdf::LString(std::to_string(v.data));
    case rdf::rdf_literal_int32_t    : return rdf::LInt32(boost::numeric_cast<int32_t>(v.data));
    case rdf::rdf_literal_uint32_t   : return rdf::LUInt32(boost::numeric_cast<uint32_t>(v.data));
    case rdf::rdf_literal_int64_t    : return rdf::LInt64(boost::numeric_cast<int64_t>(v.data));
    case rdf::rdf_literal_uint64_t   : return v;
    case rdf::rdf_literal_double_t   : return rdf::LDouble(boost::numeric_cast<double>(v.data));
    default: return rdf::Null(); // return null by default
    }
  }
  RDFTTYPE operator()(rdf::LDouble       const&v)const
  {
    switch (this->type) {
    case rdf::rdf_literal_string_t   : return rdf::LString(std::to_string(v.data));
    case rdf::rdf_literal_int32_t    : return rdf::LInt32(boost::numeric_cast<int32_t>(v.data));
    case rdf::rdf_literal_uint32_t   : return rdf::LUInt32(boost::numeric_cast<uint32_t>(v.data));
    case rdf::rdf_literal_int64_t    : return rdf::LInt64(boost::numeric_cast<int64_t>(v.data));
    case rdf::rdf_literal_uint64_t   : return rdf::LUInt64(boost::numeric_cast<uint64_t>(v.data));
    case rdf::rdf_literal_double_t   : return v;
    default: return rdf::Null(); // return null by default
    }
  }
  RDFTTYPE operator()(rdf::LString       const&v)const
  {
    switch (this->type) {
    case rdf::rdf_literal_string_t   : return v;
    case rdf::rdf_literal_date_t     : return rdf::LDate(rdf::parse_date(v.data));
    case rdf::rdf_literal_datetime_t : return rdf::LDatetime(rdf::parse_datetime(v.data));
    }

    auto view = rdf::trim_view(v.data);
    if(view.empty()) return rdf::RDFNull();

    switch (this->type) {
    case rdf::rdf_literal_int32_t    : return rdf::LInt32(boost::lexical_cast<int32_t>(view));
    case rdf::rdf_literal_uint32_t   : return rdf::LUInt32(boost::lexical_cast<int32_t>(view));
    case rdf::rdf_literal_int64_t    : return rdf::LInt64(boost::lexical_cast<int64_t>(view));
    case rdf::rdf_literal_uint64_t   : return rdf::LUInt64(boost::lexical_cast<uint64_t>(view));
    case rdf::rdf_literal_double_t   : return rdf::LDouble(boost::lexical_cast<double>(view));
    default: return rdf::Null(); // return null by default
    }
  }
  RDFTTYPE operator()(rdf::LDate         const&v)const
  {
    switch (this->type) {
    case rdf::rdf_literal_string_t   : return rdf::LString(rdf::to_string(v.data));
    case rdf::rdf_literal_date_t     : return v;
    case rdf::rdf_literal_datetime_t : return rdf::LDatetime(rdf::to_datetime(v.data));
    default: return rdf::Null(); // return null by default
    }
  }
  RDFTTYPE operator()(rdf::LDatetime     const&v)const
  {
    switch (this->type) {
    case rdf::rdf_literal_string_t   : return rdf::LString(rdf::to_string(v.data));
    case rdf::rdf_literal_date_t     : return rdf::LDate(v.data.date());
    case rdf::rdf_literal_datetime_t : return v;
    default: return rdf::Null(); // return null by default
    }
  }
  ReteSession * rs;
  int type;
};
struct ToTypeOfOperator
{
  // This operator is used as: value to_type_of predicate where predicate is a data_property as a resource
  // This operator is used as: value cast_to type where type is a rdf data type as a text or int literal corresponding to the data type
  ToTypeOfOperator(ReteSession * rs, BetaRow const* br, rdf::r_index lhs, rdf::r_index rhs): rs(rs), br(br), lhs_(lhs), rhs_(rhs) 
  {
    DataTypeVisitor visitor(this->rs, br);
    this->type_ = boost::apply_visitor(visitor, *rhs);
  }

  RDFTTYPE operator()()
  {
    CastVisitor visitor(this->rs, this->type_);
    return boost::apply_visitor(visitor, *this->lhs_);
  }

  ReteSession * rs;
  BetaRow const* br;
  rdf::r_index lhs_;  
  rdf::r_index rhs_;  
  int type_;
};

// RangeVisitor
// --------------------------------------------------------------------------------------
struct RangeVisitor: public boost::static_visitor<RDFTTYPE>, public NoCallbackNeeded
{
  // This operator is used as: (start_value range count)
  // It returns an iterator, i.e. it returns the subject (a blank node) of a set of triples:
  //      (subject, jets:range_value, value1)
  //      (subject, jets:range_value, value2)
  //                    . . .
  //      (subject, jets:range_value, valueN)
  // Where value1..N is: for(i=0; i<count; i++) start_value + i;
  RangeVisitor(ReteSession * rs, BetaRow const* br): rs(rs), br(br) {}
  template<class T, class U> RDFTTYPE operator()(T lhs, U rhs) const {if(br==nullptr) return rdf::Null(); else RETE_EXCEPTION("Invalid arguments for range: ("<<lhs<<", "<<rhs<<")");};
  RDFTTYPE operator()(rdf::LInt32 lhs, rdf::LInt32        rhs)const{return this->range(lhs.data, rhs.data);}
  
  RDFTTYPE range(int start_value, int count)const
  {
    // min of validation
    if(not rs) return rdf::Null();
    auto * rdf_session = rs->rdf_session();
    auto * rmgr = rdf_session->rmgr();

    // The subject resource for the triples to return
    rdf::r_index subject = rmgr->create_bnode();
    for(int i=0; i<count; i++) {
      rdf_session->insert_inferred(subject, rmgr->jets()->jets__range_value, start_value+i);
    }

    return *subject;
  }

  ReteSession * rs;
  BetaRow const* br;
};

} // namespace jets::rete
#endif // JETS_RETE_EXPR_OP_OTHERS_H
