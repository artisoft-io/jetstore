#include <iostream>
#include <random>
#include <iomanip>
#include <cstring>
#include <cmath>
#include <memory>

#include <gtest/gtest.h>

#include "../rdf/rdf_types.h"
#include "../rdf/rdf_date_time.h"
#include "rdf_ast.h"

#include <boost/date_time/posix_time/posix_time.hpp>
#include <boost/date_time/gregorian/gregorian.hpp>

namespace jets::rdf {
namespace {
// Simple test
TEST(RdfAstTest, BaseAstComposition) 
{

  boost::gregorian::date d{2014, 1, 31};
  std::cout << "DATE: "<<d.year() << '\n';
  std::cout << d.month() << '\n';
  std::cout << d.day() << '\n';
  std::cout << d.day_of_week() << '\n';
  std::cout << d.end_of_month() << '\n';
  std::cout << "size of date: "<<sizeof(d) << '\n';

using namespace boost::posix_time;
using namespace boost::gregorian;

  ptime pt{date{2014, 5, 12}, time_duration{12, 0, 0}};
  date dd = pt.date();
  std::cout << "FROM PTIME\nGot date part: "<<dd << '\n';
  time_duration td = pt.time_of_day();
  std::cout << "and now time of day: "<<td << '\n';
  std::cout << "size of ptime: "<<sizeof(pt)<<", while size of time_duration is "<<sizeof(td) << '\n';

  // subjects
  auto bn1_s = mkBlankNode(1);
  auto bn2_s = mkBlankNode(2);
  // predicates
  std::string str1("mange");
  auto mange_s      = mkResource(str1);
  auto nbr_items_s  = mkResource("nbr_items");
  // objects
  auto banane_s = mkResource("banane");
  auto pomme_s  = mkResource("pomme");
  auto fraise_s = mkResource("fraise");
  auto five_s   = mkLiteral(std::int32_t{5});
  auto eps_s    = mkLiteral(double{0.01});

  // Convert to r_index by taking raw pointers
  r_index bn1=bn1_s.get(), bn2=bn2_s.get();
  r_index mange=mange_s.get(), nbr_items=nbr_items_s.get();
  r_index banane=banane_s.get(), pomme=pomme_s.get(), fraise=fraise_s.get(), five=five_s.get(), eps=eps_s.get();

  // std::cout << "So let's start with this **" << std::endl;
  // std::cout << "  (" << bn1 << ", " << mange << ", " << pomme << ")" << std::endl;
  // std::cout << "  (" << bn1 << ", " << mange << ", " << banane << ")" << std::endl;
  // std::cout << "  (" << bn1 << ", " << nbr_items << ", " << five << ")" << std::endl;
  // std::cout << "  (" << bn2 << ", " << mange << ", " << banane << ")" << std::endl;
  // std::cout << "  (" << bn2 << ", " << mange << ", " << fraise << ")" << std::endl;
  // std::cout << "  (" << bn2 << ", " << nbr_items << ", " << five << ")" << std::endl;
  // std::cout << "  (" << bn2 << ", " << nbr_items << ", " << eps << ")" << std::endl;

  auto graph_p = create_base_graph('s');
  graph_p->insert(bn1, mange, pomme);
  graph_p->insert(bn1, mange, banane);
  graph_p->insert(bn1, nbr_items, five);
  graph_p->insert(bn2, mange, banane);
  graph_p->insert(bn2, mange, fraise);
  graph_p->insert(bn2, nbr_items, five);
  graph_p->insert(bn2, nbr_items, eps);

  // std::cout<<"So let's see if we get back expected triple count per individual!"<<std::endl;
  int count = 0;
  auto itor = graph_p->find(bn1);
  // std::cout<<"For, "<<bn1<<" we have:"<<std::endl;
  while(not itor.is_end()) {
      // std::cout << "   ("<<itor.get_subject() << ", "<<itor.get_predicate() << ", " << itor.get_object()<<")"<<std::endl;
      count += 1;
      itor.next();
  }
  EXPECT_EQ(count, 3);

  count = 0;
  itor = graph_p->find(bn2);
  // std::cout<<"For, "<<bn2<<" we have:"<<std::endl;
  while(not itor.is_end()) {
      // std::cout << "   ("<<itor.get_subject() << ", "<<itor.get_predicate() << ", " << itor.get_object()<<")"<<std::endl;
      count += 1;
      itor.next();
  }
  EXPECT_EQ(count, 4);
}

TEST(RdfAstTest, ToBool) 
{
  // some resources
  auto null_s = mkNull();
  auto bn1_s = mkBlankNode(1);
  auto banane_s = mkResource("banane");
  auto false_s  = mkLiteral("false");
  auto f0lse_s  = mkLiteral("f0lse");
  auto FALSE_s  = mkLiteral("FALSE");
  auto TRUE_s  = mkLiteral("TRUE");
  auto f_s  = mkLiteral("f");
  auto F_s  = mkLiteral("F");
  auto t_s  = mkLiteral("t");
  auto T_s  = mkLiteral("T");
  auto zero_s  = mkLiteral("0");
  auto one_s  = mkLiteral("1");
  auto zero_i   = mkLiteral(std::int32_t{0});
  auto one_i   = mkLiteral(std::int32_t{1});
  auto five_i   = mkLiteral(std::int32_t{5});
  auto eps_d    = mkLiteral(double{0.01});
  auto zero_d    = mkLiteral(double{0});

  EXPECT_EQ(to_bool(null_s.get()), false);
  EXPECT_EQ(to_bool(bn1_s.get()), true);
  EXPECT_EQ(to_bool(banane_s.get()), true);
  EXPECT_EQ(to_bool(false_s.get()), false);
  // "f0lse" was true until I-45 (2026-10-11): any text but a spelling of false.
  // It is outside the accepted spellings now, so to_bool, which has nowhere to
  // report it, reads it as not true; a rule testing it raises a jets:exception.
  EXPECT_EQ(to_bool(f0lse_s.get()), false);
  EXPECT_EQ(to_bool(FALSE_s.get()), false);
  EXPECT_EQ(to_bool(TRUE_s.get()), true);
  EXPECT_EQ(to_bool(f_s.get()), false);
  EXPECT_EQ(to_bool(F_s.get()), false);
  EXPECT_EQ(to_bool(t_s.get()), true);
  EXPECT_EQ(to_bool(T_s.get()), true);
  EXPECT_EQ(to_bool(zero_s.get()), false);
  EXPECT_EQ(to_bool(one_s.get()), true);
  EXPECT_EQ(to_bool(zero_i.get()), false);
  EXPECT_EQ(to_bool(one_i.get()), true);
  EXPECT_EQ(to_bool(five_i.get()), true);
  EXPECT_EQ(to_bool(eps_d.get()), true);
  EXPECT_EQ(to_bool(zero_d.get()), false);
}

TEST(RdfAstTest, DateTime)
{
  EXPECT_EQ(time_duration::num_fractional_digits(), 6);

  auto d00 = date(2019, 3, 7);
  EXPECT_EQ(parse_date("2019-03-07"), d00);

  EXPECT_EQ(parse_datetime("20120727T233718.000000-00:00  (time zone part "
                           "optional and ignored)"),
            parse_datetime("2012-07-27 23:37:18"));

  // format used for fbl encoding (see resource_manager::create_literal(int
  // type, char *data, int size))
  EXPECT_EQ(to_string(parse_datetime("20170101T000000,150")),
            "2017-01-01T00:00:00.150000");

  // Additional tests
  EXPECT_EQ(parse_datetime("20170101T000000,000002"),
            datetime(date(2017, 1, 1), boost::posix_time::microseconds(2)));
  EXPECT_EQ(parse_datetime("20170101T010100,000002+01:01"),
            datetime(date(2017, 1, 1), time_duration(0, 0, 0, 2)));
  EXPECT_EQ(parse_datetime("20170101T000000,000002-01:01"),
            datetime(date(2017, 1, 1), time_duration(1, 1, 0, 2)));
  EXPECT_EQ(parse_datetime("20170101T000000,123"),
            datetime(date(2017, 1, 1), boost::posix_time::millisec(123)));

  EXPECT_EQ(parse_datetime("20170101"),
            datetime(date(2017, 1, 1), time_duration(0, 0, 0, 0)));
  EXPECT_EQ(parse_datetime("2017-01-01 00:00:00"),
            datetime(date(2017, 1, 1), time_duration(0, 0, 0, 0)));
  EXPECT_EQ(parse_datetime("2017-01-01 12:34"),
            datetime(date(2017, 1, 1), time_duration(12, 34, 0, 0)));

  EXPECT_EQ(parse_datetime("2017-01-01 12:34:00+01:23"),
            datetime(date(2017, 1, 1), time_duration(11, 11, 0, 0)));
  EXPECT_EQ(parse_datetime("2017-01-01 12:34:00 +01:23"),
            datetime(date(2017, 1, 1), time_duration(11, 11, 0, 0)));
  EXPECT_EQ(parse_datetime("2017-01-01 12:34:00.123456+01:23"),
            datetime(date(2017, 1, 1), time_duration(11, 11, 0, 123456)));
  EXPECT_EQ(parse_datetime("2017-01-01 12:34:00.123456 +01:23"),
            datetime(date(2017, 1, 1), time_duration(11, 11, 0, 123456)));

  EXPECT_EQ(parse_datetime("2017-01-01 11:11:00-01:23"),
            datetime(date(2017, 1, 1), time_duration(12, 34, 0, 0)));
  EXPECT_EQ(parse_datetime("2017-01-01 11:11:00 -01:23"),
            datetime(date(2017, 1, 1), time_duration(12, 34, 0, 0)));
  EXPECT_EQ(parse_datetime("2017-01-01 12:34+01:23"),
            datetime(date(2017, 1, 1), time_duration(11, 11, 0, 0)));
  EXPECT_EQ(parse_datetime("2017-01-01 11:11-01:23"),
            datetime(date(2017, 1, 1), time_duration(12, 34, 0, 0)));
  EXPECT_EQ(parse_datetime("2017-01-01 11-01:34"),
            datetime(date(2017, 1, 1), time_duration(12, 34, 0, 0)));
  EXPECT_EQ(parse_datetime("2017-01-01 11-01"),
            datetime(date(2017, 1, 1), time_duration(12, 0, 0, 0)));

  EXPECT_EQ(parse_datetime("2017-01-01 00:00:00,1"),
            datetime(date(2017, 1, 1), boost::posix_time::millisec(100)));
  EXPECT_EQ(parse_datetime("2017-01-01 00:00:00,12"),
            datetime(date(2017, 1, 1), boost::posix_time::millisec(120)));
  EXPECT_EQ(parse_datetime("2017-01-01 00:00:00,123"),
            datetime(date(2017, 1, 1), boost::posix_time::millisec(123)));
  EXPECT_EQ(
      parse_datetime("2017-01-01 00:00:00,1234"),
      datetime(date(2017, 1, 1), boost::posix_time::microseconds(123400)));
  EXPECT_EQ(
      parse_datetime("2017-01-01 00:00:00.12345"),
      datetime(date(2017, 1, 1), boost::posix_time::microseconds(123450)));
  EXPECT_EQ(
      parse_datetime("2017-01-01 00:00:00.123456"),
      datetime(date(2017, 1, 1), boost::posix_time::microseconds(123456)));
  EXPECT_EQ(
      parse_datetime("2017-01-01 00:00:00.1234567"),
      datetime(date(2017, 1, 1), boost::posix_time::microseconds(123456)));
  EXPECT_EQ(
      parse_datetime("2017-01-01 00:00:00.12345678"),
      datetime(date(2017, 1, 1), boost::posix_time::microseconds(123456)));

  // auto d = date();
  // std::cout <<"GOT:"<<d<<std::endl;
  // EXPECT_EQ(parse_datetime("NOT VALID DATETIME"), datetime(date(), time_duration(0, 0, 0, 0)));

  // std::cout <<"TESTING not valid literal date:"<<std::endl;
  // auto rptr = mkLiteral(date());
  // r_index r = rptr.get();
  // std::cout <<"TESTING THIS SHOULD NOT BE A VALID DATE:"<<r<<std::endl;


  EXPECT_EQ(parse_date("NOT VALID DATE"), date());
  EXPECT_EQ(parse_date("2022-55-55"), date());

  EXPECT_EQ(
      parse_date(
          "20120727T233718.000000-00:00  (time part optional and ignored)"),
      parse_date("2012-07-27"));
  EXPECT_EQ(parse_date("07/27/2012 12:59:33.1"),
            boost_date_from_string("2012-07-27"));
  EXPECT_EQ(parse_date("201279"), boost_date_from_string("2012-07-09"));
  EXPECT_EQ(parse_date("2012-Jul-27"), boost_date_from_string("2012-07-27"));
  EXPECT_EQ(parse_date("2012-07-27 (default)"),
            boost_date_from_string("2012-07-27"));
  EXPECT_EQ(parse_date("2012-07-27 23:37:18 (default)"),
            boost_date_from_string("2012-07-27"));
  EXPECT_EQ(parse_date("20120727"), boost_date_from_string("2012-07-27"));
  EXPECT_EQ(parse_date("07/27/2012"), boost_date_from_string("2012-07-27"));
  EXPECT_EQ(parse_date("07/27/2012 12:59:33.1"),
            boost_date_from_string("2012-07-27"));
  EXPECT_EQ(parse_date("7/27/2012"), boost_date_from_string("2012-07-27"));
  EXPECT_EQ(parse_date("7/9/2012"), boost_date_from_string("2012-07-09"));
  EXPECT_EQ(parse_date("2012-07-9"), boost_date_from_string("2012-07-09"));
  EXPECT_EQ(parse_date("2012-7-9"), boost_date_from_string("2012-07-09"));
  EXPECT_EQ(parse_date("2012-7-09"), boost_date_from_string("2012-07-09"));
  EXPECT_EQ(parse_date("2012-Jul-09"), boost_date_from_string("2012-07-09"));
  EXPECT_EQ(parse_date("2012-JUL-09"), boost_date_from_string("2012-07-09"));
  EXPECT_EQ(parse_date("7-9-2012"), boost_date_from_string("2012-07-09"));
  EXPECT_EQ(parse_date("07/9/2012"), boost_date_from_string("2012-07-09"));
  EXPECT_EQ(parse_date("7/09/2012 12:59:33.1"),
            boost_date_from_string("2012-07-09"));

  EXPECT_EQ(parse_date("20170101T000000,000002"), date(2017, 1, 1));
  EXPECT_EQ(parse_date("20170101T000000,000002+01:01"), date(2017, 1, 1));
  EXPECT_EQ(parse_date("20170101T010100,000002-01:01"), date(2017, 1, 1));
  EXPECT_EQ(parse_date("20170101T000000,123"), date(2017, 1, 1));
  EXPECT_EQ(parse_date("20170101"), date(2017, 1, 1));
  EXPECT_EQ(parse_date("2017-01-01 00:00:00"), date(2017, 1, 1));
  EXPECT_EQ(parse_date("2017-01-01 12:34"), date(2017, 1, 1));
  EXPECT_EQ(parse_date("2017-01-01 12:34:00-01:23"), date(2017, 1, 1));
  EXPECT_EQ(parse_date("2017-01-01 11:11:00+01:23"), date(2017, 1, 1));
  EXPECT_EQ(parse_date("2017-01-01 12:34-01:23"), date(2017, 1, 1));
  EXPECT_EQ(parse_date("2017-01-01 11:11+01:23"), date(2017, 1, 1));
  EXPECT_EQ(parse_date("2017-01-01 11+01:34"), date(2017, 1, 1));
  EXPECT_EQ(parse_date("2017-01-01 11+01"), date(2017, 1, 1));
  EXPECT_EQ(parse_date("2017-01-01 00:00:00,1"), date(2017, 1, 1));
  EXPECT_EQ(parse_date("2017-01-01 00:00:00,12"), date(2017, 1, 1));
  EXPECT_EQ(parse_date("2017/01/01T00:00:00,123"), date(2017, 1, 1));
  EXPECT_EQ(parse_date("2017/01/01T00:00:00.123456"), date(2017, 1, 1));
  EXPECT_EQ(parse_date("01-01-2017 00:00:00,12"), date(2017, 1, 1));
  EXPECT_EQ(parse_date("01/01/2017T00:00:00,123"), date(2017, 1, 1));
}

// compare15's fast path gives compare15_slow's result: 10^6 pairs, most of them close
// enough to need the rule, near powers of ten, a few ulps apart, tiny, and money-like.
TEST(Compare15, TheFastPathGivesTheRulesResult) {
  std::mt19937_64 rng(20261022);
  std::uniform_real_distribution<double> unit(0.0, 1.0);
  auto pow10 = [&](double lo, double hi) { return std::pow(10.0, lo + (hi - lo) * unit(rng)); };
  auto sign = [&]() { return unit(rng) < 0.5 ? -1.0 : 1.0; };
  int mismatches = 0, close = 0;
  auto check = [&](double a, double b) {
    int fast = compare15(a, b), slow = compare15_slow(a, b);
    if(std::isfinite(a) and std::isfinite(b) and a != b and
       std::fabs(a - b) <= 2e-14 * std::fmax(std::fabs(a), std::fabs(b))) close++;
    if(fast != slow and mismatches++ < 10) {
      ADD_FAILURE() << std::setprecision(17) << a << " vs " << b << ": fast " << fast << ", rule " << slow;
    }
  };
  int const n = 1000000 / 6;
  for(int k=0; k<n; k++) {
    // any finite double, from random bits
    uint64_t bits[2] = {rng(), rng()};
    double x[2];
    std::memcpy(x, bits, sizeof(x));
    if(std::isfinite(x[0]) and std::isfinite(x[1])) check(x[0], x[1]);
    // within a few 1e-14 of each other, at any magnitude
    double a = sign() * pow10(-300, 300);
    check(a, a * (1 + (unit(rng) - 0.5) * 8e-14));
    // near a power of ten, where the digits counted change
    double p = std::pow(10.0, std::floor(-300 + 600 * unit(rng)));
    check(p * (1 + (unit(rng) - 0.5) * 2e-13), p * (1 + (unit(rng) - 0.5) * 2e-13));
    // a few ulps apart
    double c = sign() * pow10(-20, 20), d = c;
    for(int j = 1 + int(unit(rng) * 64); j > 0; j--) d = std::nextafter(d, 2 * c);
    check(c, d);
    // tiny, where 15-digit rounding gives way to the value itself
    double t = pow10(-320, -280);
    check(t, t * (1 + (unit(rng) - 0.5) * 1e-13));
    // money: cents
    double m = std::round(unit(rng) * 1e9) / 100;
    check(m, m + (std::round(unit(rng) * 4) - 2) / 100 + (unit(rng) - 0.5) * 1e-12);
  }
  EXPECT_EQ(mismatches, 0);
  EXPECT_GT(close, n) << "too few pairs close enough to need the rule";
}

}   // namespace
}   // namespace jets::rdf