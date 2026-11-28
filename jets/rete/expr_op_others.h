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
struct LookupVisitor: public boost::static_visitor<RDFTTYPE>, public NoCallbackNeeded, public ReadsGraph
{
  // This operator is used as: lookup_uri lookup key where lookup_uri is a resource and key is a text literal or a resource
  LookupVisitor(ReteSession * rs, BetaRow const* br): rs(rs), br(br) {}
  // A table or a key of a type it does not take is null, and looks nothing up
  // (2026-10-24): a key that is null, a blank node, a double or a date, or a table named by
  // anything but a resource or text. Each threw in a session's rule until then, and a null
  // or blank-node key was made null first.
  template<class T, class U> RDFTTYPE operator()(T, U) const {return rdf::Null();}
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
struct LookupRandVisitor: public boost::static_visitor<RDFTTYPE>, public NoCallbackNeeded, public MintsResource
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
struct MultiLookupVisitor: public boost::static_visitor<RDFTTYPE>, public NoCallbackNeeded, public ReadsGraph
{
  // This operator is used as: lookup_uri lookup key where lookup_uri is a resource and key is a text literal or a resource
  MultiLookupVisitor(ReteSession * rs, BetaRow const* br): rs(rs), br(br) {}
  // A table or a key of a type it does not take is null, and looks nothing up
  // (2026-10-24): a key that is null, a blank node, a double or a date, or a table named by
  // anything but a resource or text. Each threw in a session's rule until then, and a null
  // or blank-node key was made null first.
  template<class T, class U> RDFTTYPE operator()(T, U) const {return rdf::Null();}
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
struct MultiLookupRandVisitor: public boost::static_visitor<RDFTTYPE>, public NoCallbackNeeded, public MintsResource
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
//
// age_as_of and age_in_months_as_of of operands they do not take are null, with a row
// or without (2026-11-27): anything but two dates, and for age_in_months_as_of also
// text that does not read as a date. So are + - * / and abs of a type mismatch, and
// to_int and to_double of what they cannot read (expr_op_arithmetics.h). Until then
// each threw in a session's rule, which failed the session, and was null only with no
// beta row; text that is not a date threw with no row too.
//
// A date that is not a date (not-a-date, or an infinity) is null too, either operand
// (2026-11-27), as date arithmetic on one is (date_plus_days in expr_op_arithmetics.h):
// boost threw std::out_of_range from year() of it, which failed the session.
// --------------------------------------------------------------------------------------
struct AgeAsOfVisitor: public boost::static_visitor<RDFTTYPE>, public NoCallbackNeeded
{
  AgeAsOfVisitor(ReteSession * rs, BetaRow const* br): rs(rs), br(br) {}
  AgeAsOfVisitor(): rs(nullptr), br(nullptr) {}
  template<class T, class U> RDFTTYPE operator()(T, U)const{return rdf::Null();}   // a type mismatch

  RDFTTYPE operator()(rdf::LDate lhs, rdf::LDate rhs)const
  {
    if(lhs.data.is_special() or rhs.data.is_special()) return rdf::Null();
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
  template<class T, class U> RDFTTYPE operator()(T, U)const{return rdf::Null();}   // a type mismatch

  RDFTTYPE operator()(rdf::LString lhs, rdf::LDate rhs)const
  {
    // text that does not read as a date is null
    auto birthday = rdf::parse_date(lhs.data);
    if(birthday.is_special() or rhs.data.is_special()) return rdf::Null();
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

  RDFTTYPE operator()(rdf::LDate lhs, rdf::LDate rhs)const
  {
    if(lhs.data.is_special() or rhs.data.is_special()) return rdf::Null();
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

// to_timestamp, month_period_of, week_period_of and day_period_of
// --------------------------------------------------------------------------------------
// Each takes a date. The three periods are those the platform computes for a session's
// source period (CalculatePeriod in jets/datatable/source_period.go, which fills
// source_period, session_registry's periods and $DAY_PERIOD), so a rule can compare its
// dates with the session's period:
//   month_period_of = (year - 1970) * 12 + month
//   week_period_of  = unix time / 604800 + 1     (7 * 86400 seconds a week)
//   day_period_of   = unix time / 86400 + 1      (24 * 60 * 60 seconds a day)
// the division truncating toward zero in both languages, so 1970-01-01 is month 1, week 1
// and day 1, and the day before it is day 0. Until 2026-11-27 day_period_of divided by
// 84400, a typing slip: it ran ahead of the platform's day period by one more day about
// every 42 days from 1970, skipping an integer each time, so that 2026-11-27 was 21277
// against the platform's 20785; only 1969-11-20 to 1970-02-12 agreed.
//
// A date that is not a date (not-a-date, or an infinity) is null since 2026-11-27, as date
// arithmetic on one is: to_timestamp, week_period_of and day_period_of of it gave a
// meaningless number (to_timestamp 9223372036854 for not-a-date), and month_period_of
// threw from boost, which failed the session. Anything but a date still throws in a
// session's rule; until 2026-11-27 three of the four named month_period_of in the message.

// ToTimestampVisitor
// --------------------------------------------------------------------------------------
struct ToTimestampVisitor: public boost::static_visitor<RDFTTYPE>, public NoCallbackNeeded
{
  ToTimestampVisitor(ReteSession * rs, BetaRow const* br): rs(rs), br(br) {}
  ToTimestampVisitor(): rs(nullptr), br(nullptr) {}
  template<class T>RDFTTYPE operator()(T lhs)const{if(br==nullptr) return rdf::Null(); else RETE_EXCEPTION("Invalid arguments for to_timestamp: ("<<lhs<<")");};
  RDFTTYPE operator()(rdf::LDate lhs)const
  {
    if(lhs.data.is_special()) return rdf::Null();
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
    if(lhs.data.is_special()) return rdf::Null();
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
  template<class T>RDFTTYPE operator()(T lhs)const{if(br==nullptr) return rdf::Null(); else RETE_EXCEPTION("Invalid arguments for week_period_of: ("<<lhs<<")");};
  RDFTTYPE operator()(rdf::LDate lhs)const
  {
    // secPerDay = 24 * 60 * 60 = 86400
    // secPerWeek = 7 * secPerDay = 604800
    // weekPeriod = int(unixTime/secPerWeek + 1)
    if(lhs.data.is_special()) return rdf::Null();
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
  template<class T>RDFTTYPE operator()(T lhs)const{if(br==nullptr) return rdf::Null(); else RETE_EXCEPTION("Invalid arguments for day_period_of: ("<<lhs<<")");};
  RDFTTYPE operator()(rdf::LDate lhs)const
  {
    // secPerDay = 24 * 60 * 60 = 86400
    // dayPeriod = int(unixTime/secPerDay + 1)
    if(lhs.data.is_special()) return rdf::Null();
    auto timestamp = rdf::to_timestamp(lhs.data);
    return rdf::LInt32(int((timestamp / 86400L) + 1L));
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
    case rdf::rdf_literal_string_t   : return rdf::LString(rdf::double_to_text(v.data));
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
struct RangeVisitor: public boost::static_visitor<RDFTTYPE>, public NoCallbackNeeded, public MintsResource
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
