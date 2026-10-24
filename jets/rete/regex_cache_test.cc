#include <regex>
#include <string>
#include <thread>
#include <vector>

#include <gtest/gtest.h>

#include "../rdf/rdf_types.h"
#include "../rete/rete_types.h"
#include "../rete/expr_op_strings.h"

// literal_regex's cache of compiled patterns (C15): one per thread, bounded
namespace jets::rete {
namespace {

RDFTTYPE regex(std::string const& text, std::string const& pattern)
{
  return boost::apply_visitor(RegexVisitor(nullptr, nullptr),
    RDFTTYPE(rdf::LString(text)), RDFTTYPE(rdf::LString(pattern)));
}

TEST(RegexCacheTest, APatternIsCompiledOnce)
{
  thread_regex_cache().patterns.clear();
  auto const* first = &thread_regex_cache().get("(\\d+)");
  auto const* second = &thread_regex_cache().get("(\\d+)");
  EXPECT_EQ(first, second);
  EXPECT_EQ(thread_regex_cache().patterns.size(), 1u);
  EXPECT_EQ(regex("ab123", "(\\d+)"), RDFTTYPE(rdf::LString("123")));
  EXPECT_EQ(regex("ab", "(\\d+)").which(), rdf::rdf_null_t);
  EXPECT_EQ(thread_regex_cache().patterns.size(), 1u);
}

// A pattern from the data cannot grow it past its capacity
TEST(RegexCacheTest, ItIsBounded)
{
  thread_regex_cache().patterns.clear();
  for(std::size_t i = 0; i < 3 * kRegexCacheCapacity; i++) {
    auto n = std::to_string(i);
    EXPECT_EQ(regex("x" + n + "y", "x(" + n + ")y"), RDFTTYPE(rdf::LString(n)));
    EXPECT_LE(thread_regex_cache().patterns.size(), kRegexCacheCapacity);
  }
  EXPECT_GE(thread_regex_cache().patterns.size(), 1u);
}

// A pattern that does not compile throws each time, as it did, and is not kept
TEST(RegexCacheTest, ABadPatternThrowsAndIsNotKept)
{
  thread_regex_cache().patterns.clear();
  EXPECT_THROW(regex("abc", "(a"), std::regex_error);
  EXPECT_THROW(regex("abc", "(a"), std::regex_error);
  EXPECT_TRUE(thread_regex_cache().patterns.empty());
}

// Each thread has its own cache, so sessions on several threads share no pattern
TEST(RegexCacheTest, EachThreadHasItsOwn)
{
  thread_regex_cache().patterns.clear();
  auto const* mine = &thread_regex_cache().get("(b+)");
  constexpr int kThreads = 4;
  std::vector<int> ok(kThreads, 0);
  std::vector<std::regex const*> theirs(kThreads, nullptr);
  std::vector<std::thread> threads;
  for(int t = 0; t < kThreads; t++) {
    threads.emplace_back([&ok, &theirs, t]() {
      bool all = true;
      for(int k = 0; k < 2000; k++) {
        auto n = std::to_string(k % 10);
        all = all and regex("a" + n + "bb", "a(" + n + ")b") == RDFTTYPE(rdf::LString(n));
        all = all and regex("abbc", "(b+)") == RDFTTYPE(rdf::LString("bb"));
      }
      theirs[t] = &thread_regex_cache().get("(b+)");
      ok[t] = all and thread_regex_cache().patterns.size() == 11 ? 1 : 0;
    });
  }
  for(auto & th: threads) th.join();
  for(int t = 0; t < kThreads; t++) {
    EXPECT_EQ(ok[t], 1) << "thread " << t;
    EXPECT_NE(theirs[t], mine) << "thread " << t;
  }
  EXPECT_EQ(thread_regex_cache().patterns.size(), 1u);
}

}   // namespace
}   // namespace jets::rete
