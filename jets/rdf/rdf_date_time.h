#ifndef JETS_RDF_DATE_TIME_H
#define JETS_RDF_DATE_TIME_H

#include <memory>
#include <string>
#include <regex>

#include <boost/date_time/gregorian/gregorian.hpp>
#include <boost/date_time/posix_time/posix_time.hpp>

namespace jets::rdf {

using date = boost::gregorian::date;
using datetime = boost::posix_time::ptime;
using time_duration = boost::posix_time::time_duration;

namespace internal {
extern std::regex const date_regex;
extern std::regex const date_time_regex;
}  // namespace internal

/**
 * Function to extract the date portion of the time structure.
 *
 * Delegates to Boost.
 * @param t time from which to get the date from.
 * @return date portion of the argument t.
 */
inline date to_date(datetime t) { return t.date(); };

/**
 * Function to get the number of days between 2 dates, can be negative
 *
 * Delegates to Boost.
 *
 * @param t0 date from which to get the duration.
 * @param t1 date to which to get the duration.
 * @return nbr of days, aka t0 - t1, positive if t0 > t1 (t0 is after t1)
 */
inline int days(date t0, date t1) {
  boost::gregorian::date_duration duration = t0 - t1;
  return duration.days();
}
/**
 * Function to add the number of days to a date, can be negative
 *
 * Delegates to Boost.
 *
 * @param t0 date from which to get the duration.
 * @param days to add to t0.
 * @return new date
 */
inline date add_days(date t0, int days) {
  return t0 + boost::gregorian::date_duration(days);
}

/**
 * Function to convert a date to datetime (at midnight).
 *
 * Delegates to Boost.
 * @param t date from which to convert
 * @return datetime at midnight of t.
 */
inline datetime to_datetime(date t) {
  return boost::posix_time::ptime(t, boost::posix_time::time_duration(0, 0, 0, 0));
}

/**
 * Function to convert a date to unix timestamp (nbr of sec since 1/1/1970)
 *
 * Delegates to Boost.
 * @param t date from which to convert
 * @return int the timestamp equivalent to the date
 */
inline long int to_timestamp(date d) {
	using namespace boost::posix_time;
	static ptime epoch(boost::gregorian::date(1970, 1, 1));
	time_duration::sec_type secs = (ptime(d, seconds(0)) - epoch).total_seconds();
	return secs;
}

/**
 * String representation of date. Delegates to iso_extended_string of boost.
 *
 * Format: 	To YYYY-MM-DD where all components are integers.
 * example: 2002-01-31
 *
 * @param t
 * @return
 */
inline std::string to_string(date d) {
  return boost::gregorian::to_iso_extended_string(d);
}

/**
 * String representation of time. Delegates to to_simple_string of boost.
 *
 * Format: Convert to form YYYY-MM-DDTHH:MM:SS,fffffffff where T is the date-time separator
 * example: 2002-01-31T10:00:01,123456
 *
 * @param t
 * @return
 */
inline std::string
to_string(datetime t)
{
  return boost::posix_time::to_iso_extended_string(t);
}

/**
 * Create date from string representation. Delegates to from_string of boost.
 *
 * @param s
 * @return
 */
inline date boost_date_from_string(std::string const& s) {
  return boost::gregorian::from_string(s);
}

// Date -- here to have access to literal_data definition
int month_2_int(std::string const& month_str);

date parse_date(std::string const& text);

date parse_date(std::string&& text);

datetime parse_datetime(std::string const& text);

datetime parse_datetime(std::string&& text);

}  // namespace jets::rdf
#endif  // JETS_RDF_DATE_TIME_H
