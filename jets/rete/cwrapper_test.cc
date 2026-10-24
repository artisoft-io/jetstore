#include <string>

#include <gtest/gtest.h>

#include "../rdf/rdf_types.h"
#include "../rete/jets_rete_cwrapper.h"

namespace jets::rete {
namespace {

// I-162: the four ISO getters returned c_str() of a temporary, which the caller
// read after it was freed. A datetime's text is longer than the small-string
// buffer, so the read was of freed heap; under AddressSanitizer each of these
// reports heap-use-after-free at the base.
TEST(CWrapperIsoGetters, DatetimeTextOutlivesTheCall)
{
  auto dt = rdf::mkLiteral(rdf::parse_datetime("2024-03-15T10:20:30.123456"));
  int status = -1;
  char const* text = get_datetime_iso_string2(dt.get(), &status);
  ASSERT_EQ(status, 0);
  ASSERT_NE(text, nullptr);
  EXPECT_STREQ(text, "2024-03-15T10:20:30.123456");

  HSTR out = nullptr;
  ASSERT_EQ(get_datetime_iso_string(dt.get(), &out), 0);
  EXPECT_STREQ(static_cast<char const*>(out), "2024-03-15T10:20:30.123456");
}

TEST(CWrapperIsoGetters, DateTextOutlivesTheCall)
{
  auto d = rdf::mkLiteral(rdf::parse_date("2024-03-15"));
  int status = -1;
  char const* text = get_date_iso_string2(d.get(), &status);
  ASSERT_EQ(status, 0);
  ASSERT_NE(text, nullptr);
  EXPECT_STREQ(text, "2024-03-15");

  HSTR out = nullptr;
  ASSERT_EQ(get_date_iso_string(d.get(), &out), 0);
  EXPECT_STREQ(static_cast<char const*>(out), "2024-03-15");
}

// The buffer is the calling thread's: a later call replaces the text, it does not
// free the storage an earlier pointer names.
TEST(CWrapperIsoGetters, ALaterCallReusesTheBuffer)
{
  auto a = rdf::mkLiteral(rdf::parse_datetime("2024-03-15T10:20:30"));
  auto b = rdf::mkLiteral(rdf::parse_datetime("2025-12-31T23:59:59"));
  int status = -1;
  char const* first = get_datetime_iso_string2(a.get(), &status);
  ASSERT_EQ(status, 0);
  std::string kept(first);
  char const* second = get_datetime_iso_string2(b.get(), &status);
  ASSERT_EQ(status, 0);
  EXPECT_EQ(kept, "2024-03-15T10:20:30");
  EXPECT_STREQ(second, "2025-12-31T23:59:59");
  EXPECT_EQ(first, second);
}

// The getters jets/bridge calls write into the caller's buffer, so no other call can
// change the text afterwards
TEST(CWrapperIsoGetters, TheCallersBufferGetsTheText)
{
  auto dt = rdf::mkLiteral(rdf::parse_datetime("2024-03-15T10:20:30.123456"));
  auto d = rdf::mkLiteral(rdf::parse_date("2024-03-15"));
  char buf[64];
  ASSERT_EQ(get_datetime_iso_string3(dt.get(), buf, sizeof(buf)), 0);
  EXPECT_STREQ(buf, "2024-03-15T10:20:30.123456");
  char other[64];
  ASSERT_EQ(get_date_iso_string3(d.get(), other, sizeof(other)), 0);
  EXPECT_STREQ(other, "2024-03-15");
  EXPECT_STREQ(buf, "2024-03-15T10:20:30.123456");
}

TEST(CWrapperIsoGetters, TheCallersBufferMustFit)
{
  auto d = rdf::mkLiteral(rdf::parse_date("2024-03-15"));
  char buf[11];
  EXPECT_EQ(get_date_iso_string3(d.get(), buf, 10), -4);   // 10 characters and the zero
  EXPECT_STREQ(buf, "");
  ASSERT_EQ(get_date_iso_string3(d.get(), buf, 11), 0);
  EXPECT_STREQ(buf, "2024-03-15");
  EXPECT_EQ(get_date_iso_string3(d.get(), buf, 0), -4);
  EXPECT_EQ(get_date_iso_string3(d.get(), nullptr, 11), -1);
  EXPECT_EQ(get_date_iso_string3(nullptr, buf, 11), -1);
  auto nad = rdf::mkLiteral(rdf::date(boost::date_time::not_a_date_time));
  EXPECT_EQ(get_date_iso_string3(nad.get(), buf, 11), -2);
  auto i = rdf::mkLiteral(int32_t(5));
  EXPECT_EQ(get_date_iso_string3(i.get(), buf, 11), -1);
  EXPECT_EQ(get_datetime_iso_string3(d.get(), buf, 11), -1);
}

TEST(CWrapperIsoGetters, NotADateGivesNoText)
{
  auto nad = rdf::mkLiteral(rdf::date(boost::date_time::not_a_date_time));
  int status = 0;
  EXPECT_EQ(get_date_iso_string2(nad.get(), &status), nullptr);
  EXPECT_EQ(status, -2);
  auto i = rdf::mkLiteral(int32_t(5));
  EXPECT_EQ(get_date_iso_string2(i.get(), &status), nullptr);
  EXPECT_EQ(status, -1);
}

}   // namespace
}   // namespace jets::rete
