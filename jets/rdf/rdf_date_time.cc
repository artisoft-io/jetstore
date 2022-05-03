
#include <string>
#include <glog/logging.h>

#include "../rdf/rdf_err.h"
#include "../rdf/rdf_date_time.h"


namespace jets::rdf {

int month_2_int(std::string const& month_str) {
  if (month_str.size() == 3) {
    // std::string str = boost::to_upper_copy<std::string>(month_str);
    if (month_str == "JAN") return boost::gregorian::Jan;
    if (month_str == "FEB") return boost::gregorian::Feb;
    if (month_str == "MAR") return boost::gregorian::Mar;
    if (month_str == "APR") return boost::gregorian::Apr;
    if (month_str == "MAY") return boost::gregorian::May;
    if (month_str == "JUN") return boost::gregorian::Jun;
    if (month_str == "JUL") return boost::gregorian::Jul;
    if (month_str == "AUG") return boost::gregorian::Aug;
    if (month_str == "SEP") return boost::gregorian::Sep;
    if (month_str == "OCT") return boost::gregorian::Oct;
    if (month_str == "NOV") return boost::gregorian::Nov;
    if (month_str == "DEC") return boost::gregorian::Dec;
    return boost::gregorian::NotAMonth;
  } else {
    return boost::lexical_cast<int>(month_str);
  }
}

std::regex const internal::date_regex(
    R"X((\d{1,4})-?\/?(JAN|FEB|MAR|APR|MAY|JUN|JUL|AUG|SEP|OCT|NOV|DEC|\d{1,2})-?\/?(\d{1,4}))X");

date parse_date_internal(std::string&& text) {
  try {
    std::smatch what;
    if (std::regex_search(text, what, internal::date_regex)) {
      int ntok = what.size();
      if (ntok < 4) {
        LOG(ERROR) <<"parse_date: Argument is not a date: " << text;
        return date(1400, 1, 1);
      }

      // get the date portion
      std::string const& tok1 = what[1];
      std::string const& tok2 = what[2];
      std::string const& tok3 = what[3];
      int y, m, d;

      if (tok1.size() == 4) {
        // year, month, day
        y = boost::lexical_cast<int>(tok1);
        m = month_2_int(tok2);
        d = boost::lexical_cast<int>(tok3);
      } else {
        // month, day, year
        y = boost::lexical_cast<int>(tok3);
        m = month_2_int(tok1);
        d = boost::lexical_cast<int>(tok2);
      }
      if(y < 1400 or y > 9999) {
        LOG(ERROR) <<"parse_date: Argument date has not a valid year: " << text << ", year parsed: "<<y;
        return date(1400, 1, 1);
      }
      if(m < 1 or m > 12) {
        LOG(ERROR) <<"parse_date: Argument date has not a valid month: " << text << ", month parsed: "<<m;
        return date(1400, 1, 1);
      }
      if(d < 1 or d > 31) {
        LOG(ERROR) <<"parse_date: Argument date has not a valid day: " << text << ", day parsed: "<<m;
        return date(1400, 1, 1);
      }
      return date(y, m, d);
    } else {
        LOG(ERROR) <<"parse_date: Argument is not a date: " << text;
        return date(1400, 1, 1);
    }
  } catch(...) {
    LOG(ERROR) <<"parse_date: Argument is not a date: " << text;
    return date(1400, 1, 1);
  }
}

date parse_date(std::string const& text0) {
  std::string str = boost::to_upper_copy<std::string>(text0);
  boost::to_upper<std::string>(str);
  return parse_date_internal(std::forward<std::string>(str));
}

date parse_date(std::string&& str) {
  boost::to_upper<std::string>(str);
  return parse_date_internal(std::forward<std::string>(str));
}

std::regex const internal::date_time_regex(
    R"X((\d{1,4})-?\/?(JAN|FEB|MAR|APR|MAY|JUN|JUL|AUG|SEP|OCT|NOV|DEC|\d{1,2})-?\/?(\d{1,4})[T-]?\s?(\d{1,2})?[:.]?(\d{1,2})?[:.]?(\d{1,2})?[.,]?(\d+)?\s?([+-])?(\d{1,2})?[:.]?(\d{1,2})?)X");

datetime parse_time_internal(std::string&& text) {
  try {
    std::smatch what;
    if (std::regex_search(text, what, internal::date_time_regex)) {
      int ntok = what.size();
      if (ntok < 4) {
        LOG(ERROR) <<"parse_date: Argument is not a datetime: " << text;
        return datetime();
      }

      // get the date portion
      std::string tok1 = what[1];
      std::string tok2 = what[2];
      std::string tok3 = what[3];

      date ddate;
      if (tok1.size() == 4) {
        // year, month, day
        ddate = date(boost::lexical_cast<int>(tok1), month_2_int(tok2),
                    boost::lexical_cast<int>(tok3));
      } else {
        // month, day, year
        ddate = date(boost::lexical_cast<int>(tok3), month_2_int(tok1),
                    boost::lexical_cast<int>(tok2));
      }

      // get the duration portion
      time_duration offset;
      std::string hour_str;
      if (ntok > 4) hour_str = what[4];

      if (hour_str.empty()) {
        offset = time_duration(0, 0, 0, 0);
      } else {
        int hours = boost::lexical_cast<int>(hour_str), mins = 0, secs = 0,
            frac = 0;
        if (ntok > 5) {
          std::string min_str = what[5];
          if (not min_str.empty()) mins = boost::lexical_cast<int>(min_str);
        }
        if (ntok > 6) {
          std::string sec_str = what[6];
          if (not sec_str.empty()) {
            secs = boost::lexical_cast<int>(sec_str);
            if (ntok > 7) {
              std::string frac_str = what[7];
              if (not frac_str.empty()) {
                int digits = static_cast<int>(frac_str.size());
                int precision = time_duration::num_fractional_digits();
                if (digits >= precision) {
                  // drop excess digits
                  frac = boost::lexical_cast<int>(frac_str.substr(0, precision));
                } else {
                  frac = boost::lexical_cast<int>(frac_str);
                }
                if (digits < precision) {
                  // trailing zeros get dropped from the string,
                  // "1:01:01.1" would yield .000001 instead of .100000
                  // the power() compensates for the missing decimal
                  // places
                  frac *= std::pow(10, precision - digits);
                }
              }
            }
          }

          // timezone
          if (ntok > 9) {
            int hrs = 0, mns = 0;
            std::string sign_str = what[8];
            std::string hr_str = what[9];
            if (not hr_str.empty()) hrs = boost::lexical_cast<int>(hr_str);
            if (ntok > 10) {
              std::string mn_str = what[10];
              if (not mn_str.empty()) mns = boost::lexical_cast<int>(mn_str);
            }
            if (sign_str[0] == '-') {
              hours += hrs;
              mins += mns;
            } else {
              hours -= hrs;
              mins -= mns;
            }
          }
        }

        offset = time_duration(hours, mins, secs, frac);

        // std::cout<<" -- time_duration(hours=<<"<<hours<<",
        // mins="<<mins<<", secs="<<secs<<", frac="<<frac<<")"<<std::endl;
        // std::ostringstream buf;
        // buf << what[4]<<":"<<what[5]<<":"<<what[6]<<"."<<what[7];
        // offset =
        // boost::date_time::parse_delimited_time_duration<time_duration>(buf.str());
      }
      auto result = datetime(ddate, offset);
      //		   std::cout<<"  -- result:
      //'"<<to_string(result)<<"'"<<std::endl;
      return result;
    } else {
      LOG(ERROR) <<"parse_datetime: Argument is not a datetime: " << text;
    }
  } catch(...) {
    LOG(ERROR) <<"parse_datetime: Argument is not a datetime: " << text;
  }
  return datetime();
}

datetime parse_datetime(std::string const& text0) {
  std::string str = boost::to_upper_copy<std::string>(text0);
  return parse_time_internal(std::forward<std::string>(str));
}

datetime parse_datetime(std::string&& str) {
  boost::to_upper<std::string>(str);
  return parse_time_internal(std::forward<std::string>(str));
}

}  // namespace jets::rdf
