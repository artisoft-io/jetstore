#include <gtest/gtest.h>

#include "../rdf/rdf_types.h"
#include "../rete/beta_row_arena.h"
#include "../rete/beta_relation.h"

// RowSupports, a row's supports: the first in place, a set only for the others
// (beta_relation.h). The engine tests of the compiler drive it through the rete
// (jets/compilerv2/compiler/beta_row_pruning_engine_test.go); these pin the
// container itself, on the heap and in an arena.
namespace jets::rete {
namespace {

rdf::r_index res(std::uintptr_t i) { return reinterpret_cast<rdf::r_index>(i * 16); }

SupportKey key(BetaRowArena * arena, std::uintptr_t a, std::uintptr_t b)
{
  SupportKey k{ArenaAllocator<rdf::r_index>(arena)};
  k.push_back(res(a));
  k.push_back(res(b));
  return k;
}

void exercise(BetaRowArena * arena)
{
  RowSupports s(key(arena, 1, 1));
  EXPECT_EQ(s.size(), 1u);
  s.add(key(arena, 1, 1));                  // offered twice, one support
  EXPECT_EQ(s.size(), 1u);
  s.add(key(arena, 2, 1));
  s.add(key(arena, 3, 1));
  s.add(key(arena, 2, 1));
  EXPECT_EQ(s.size(), 3u);
  EXPECT_TRUE(s.in(arena));

  EXPECT_TRUE(s.withdraw(key(arena, 9, 9)));   // not one of its supports: withdraws nothing
  EXPECT_EQ(s.size(), 3u);
  EXPECT_TRUE(s.withdraw(key(arena, 1, 1)));   // the first, with others left
  EXPECT_EQ(s.size(), 2u);
  EXPECT_TRUE(s.withdraw(key(arena, 1, 1)));   // withdrawn twice, once
  EXPECT_EQ(s.size(), 2u);

  RowSupports moved(std::move(s));             // a map rehash moves its values
  EXPECT_EQ(moved.size(), 2u);
  EXPECT_TRUE(moved.withdraw(key(arena, 3, 1)));
  EXPECT_FALSE(moved.withdraw(key(arena, 2, 1)));   // the last
}

TEST(RowSupportsTest, OnTheHeap) { exercise(nullptr); }

TEST(RowSupportsTest, InAnArena)
{
  BetaRowArena arena;
  exercise(&arena);
}

TEST(RowSupportsTest, MapOfSupportsInAnArena)
{
  BetaRowArena arena;
  SupportMap m{SupportMap::allocator_type(&arena)};
  BetaRow const* rows[64];
  for(std::uintptr_t i = 0; i < 64; ++i) {
    rows[i] = reinterpret_cast<BetaRow const*>((i + 1) * 128);
    m.try_emplace(rows[i], key(&arena, i, 0));
    if(i % 2) m.find(rows[i])->second.add(key(&arena, i, 1));
  }
  for(auto const& [row, supports]: m) {
    EXPECT_TRUE(supports.in(&arena));
  }
  EXPECT_EQ(m.find(rows[3])->second.size(), 2u);
  EXPECT_EQ(m.find(rows[4])->second.size(), 1u);
}

}  // namespace
}  // namespace jets::rete
