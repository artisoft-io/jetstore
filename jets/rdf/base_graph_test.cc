#include <iostream>
#include <functional>
#include <memory>
#include <set>
#include <string>
#include <tuple>
#include <vector>

#include <gtest/gtest.h>

#include "../rdf/rdf_types.h"

namespace jets::rdf {
namespace {
// The suite fixture for base_graph
class BaseGraphTest : public ::testing::Test {
 protected:
  BaseGraphTest()
    : s1(), s2(), s3(), s4(), g0(), g1(), g2(), g3()
  {
    s1 = mkResource("r1");
    s2 = mkResource("r2");
    s3 = mkResource("r3");
    s4 = mkResource("r4");
    g0 = create_base_graph('s');
    g1 = create_base_graph('s');
    g2 = create_base_graph('p');
    g3 = create_base_graph('o');
    
    // Make all graphs to have the same triples as g1 (the 's' one)
    // Note that insert(s, p, o) rotates the triple according to spin
    // while contains(u, v, w) method check for triple as is (i.e. u, v, w)
    r_index r1=s1.get(), r2=s2.get(), r3=s3.get(), r4=s4.get();

    // spo natural order
    g1->insert(r1, r2, r3);
    g1->insert(r4, r2, r3);
    g1->insert(r1, r4, r3);
    g1->insert(r1, r2, r4);

    // pos order
    g2->insert(r1, r2, r3);
    g2->insert(r4, r2, r3);
    g2->insert(r1, r4, r3);
    g2->insert(r1, r2, r4);

    // osp order
    g3->insert(r1, r2, r3);
    g3->insert(r4, r2, r3);
    g3->insert(r1, r4, r3);
    g3->insert(r1, r2, r4);
  }

  Rptr s1;
  Rptr s2;
  Rptr s3;
  Rptr s4;
  BaseGraphPtr g0;
  BaseGraphPtr g1;
  BaseGraphPtr g2;
  BaseGraphPtr g3;
};

// Define the tests
TEST_F(BaseGraphTest, EmptyGraph) 
{
  r_index r1=s1.get(), r2=s2.get(), r3=s3.get();
  EXPECT_FALSE(g0->contains(r1, r2, r3));
  EXPECT_EQ(g0->retract(r1, r2, r3), 0);
  EXPECT_EQ(g0->erase(r1, r2, r3), 0);
  auto itor = g0->find();
  while(not itor.is_end()) {
      ADD_FAILURE() << "Expecting an empty graph, should not have returned triples";
      itor.next();
  }
}

TEST_F(BaseGraphTest, InsertUvwGraph) 
{
  r_index r1=s1.get(), r2=s2.get(), r3=s3.get();
  EXPECT_TRUE(g0->insert(r1, r2, r3));
  EXPECT_TRUE(g0->contains(r1, r2, r3));

  EXPECT_EQ(g0->retract(r1, r2, r3), 1);
  EXPECT_FALSE(g0->contains(r1, r2, r3));

  EXPECT_TRUE(g0->insert(r1, r2, r3));
  EXPECT_FALSE(g0->insert(r1, r2, r3));
  EXPECT_EQ(g0->retract(r1, r2, r3), 0); // returns 0 because triple is still in graph
  EXPECT_TRUE(g0->contains(r1, r2, r3));
  EXPECT_EQ(g0->retract(r1, r2, r3), 1);
  EXPECT_FALSE(g0->contains(r1, r2, r3));

  // insert again
  EXPECT_TRUE(g0->insert(r1, r2, r3));
  EXPECT_TRUE(g0->contains(r1, r2, r3));

  auto itor = g0->find();
  int count = 0;
  while(not itor.is_end()) {
      count += 1;
      EXPECT_EQ(itor.get_subject(), r1);
      EXPECT_NE(itor.get_subject(), r2);
      EXPECT_EQ(itor.get_predicate(), r2);
      EXPECT_NE(itor.get_predicate(), r3);
      EXPECT_EQ(itor.get_object(), r3);
      itor.next();
  }
  EXPECT_EQ(count, 1);
}

TEST_F(BaseGraphTest, SPOGraph) 
{
  r_index r1=s1.get(), r2=s2.get(), r3=s3.get(), r4=s4.get();
  // spo spin order
  EXPECT_TRUE(g1->contains_spo(r1, r2, r3));
  EXPECT_TRUE(g1->contains_spo(r4, r2, r3));
  EXPECT_TRUE(g1->contains_spo(r1, r4, r3));
  EXPECT_TRUE(g1->contains_spo(r1, r2, r4));

  // uvw
  EXPECT_TRUE(g1->contains(r1, r2, r3));
  EXPECT_TRUE(g1->contains(r4, r2, r3));
  EXPECT_TRUE(g1->contains(r1, r4, r3));
  EXPECT_TRUE(g1->contains(r1, r2, r4));

  auto itor = g1->find();
  int count = 0;
  while(not itor.is_end()) {
      count += 1;
      auto s = itor.get_subject();
      auto p = itor.get_predicate();
      auto o = itor.get_object();
      EXPECT_TRUE(s == r1 or s == r4);
      EXPECT_TRUE(p == r2 or p == r4);
      EXPECT_TRUE(o == r3 or o == r4);
      itor.next();
  }
  EXPECT_EQ(count, 4);
}

TEST_F(BaseGraphTest, POSGraph) 
{
  r_index r1=s1.get(), r2=s2.get(), r3=s3.get(), r4=s4.get();
  // uvw
  EXPECT_TRUE(g2->contains(r1, r2, r3));
  EXPECT_TRUE(g2->contains(r4, r2, r3));
  EXPECT_TRUE(g2->contains(r1, r4, r3));
  EXPECT_TRUE(g2->contains(r1, r2, r4));

  // spo spin order for 'p' spin
  g2->clear();    
  EXPECT_TRUE(g2->insert_spo(r1, r2, r3));
  EXPECT_TRUE(g2->insert_spo(r4, r2, r3));
  EXPECT_TRUE(g2->insert_spo(r1, r4, r3));
  EXPECT_TRUE(g2->insert_spo(r1, r2, r4));

  EXPECT_TRUE(g2->contains_spo(r1, r2, r3));
  EXPECT_TRUE(g2->contains_spo(r4, r2, r3));
  EXPECT_TRUE(g2->contains_spo(r1, r4, r3));
  EXPECT_TRUE(g2->contains_spo(r1, r2, r4));

  auto itor = g2->find();
  int count = 0;
  while(not itor.is_end()) {
      count += 1;
      auto s = itor.get_subject();
      auto p = itor.get_predicate();
      auto o = itor.get_object();
      EXPECT_TRUE(s == r1 or s == r4);
      EXPECT_TRUE(p == r2 or p == r4);
      EXPECT_TRUE(o == r3 or o == r4);
      itor.next();
  }
  EXPECT_EQ(count, 4);
}

TEST_F(BaseGraphTest, OSPGraph) 
{
  r_index r1=s1.get(), r2=s2.get(), r3=s3.get(), r4=s4.get();
  // uvw
  EXPECT_TRUE(g3->contains(r1, r2, r3));
  EXPECT_TRUE(g3->contains(r4, r2, r3));
  EXPECT_TRUE(g3->contains(r1, r4, r3));
  EXPECT_TRUE(g3->contains(r1, r2, r4));

  // spo spin order for 'o' spin
  g3->clear();    
  EXPECT_TRUE(g3->insert_spo(r1, r2, r3));
  EXPECT_TRUE(g3->insert_spo(r4, r2, r3));
  EXPECT_TRUE(g3->insert_spo(r1, r4, r3));
  EXPECT_TRUE(g3->insert_spo(r1, r2, r4));

  EXPECT_TRUE(g3->contains_spo(r1, r2, r3));
  EXPECT_TRUE(g3->contains_spo(r4, r2, r3));
  EXPECT_TRUE(g3->contains_spo(r1, r4, r3));
  EXPECT_TRUE(g3->contains_spo(r1, r2, r4));

  auto itor = g3->find();
  int count = 0;
  while(not itor.is_end()) {
      count += 1;
      auto s = itor.get_subject();
      auto p = itor.get_predicate();
      auto o = itor.get_object();
      EXPECT_TRUE(s == r1 or s == r4);
      EXPECT_TRUE(p == r2 or p == r4);
      EXPECT_TRUE(o == r3 or o == r4);
      itor.next();
  }
  EXPECT_EQ(count, 4);
}


// A graph changed while it is iterated: the iterators of a tracked graph (any but the
// meta graph) are frozen before the change, and return what the graph held then.
class BaseGraphChangeTest : public ::testing::Test {
 protected:
  BaseGraphChangeTest() : g(create_base_graph('s', 1))
  {
    for(int i=0; i<2000; i++) nodes.push_back(mkResource("n" + std::to_string(i)));
    // (n0, n1, *) has 50 objects, (n0, *, *) 10 predicates, the graph 20 subjects
    for(int s=0; s<20; s++) {
      for(int p=1; p<=10; p++) {
        int nobj = (s == 0 and p == 1) ? 50 : 3;
        for(int o=0; o<nobj; o++) g->insert(n(s), n(100+p), n(200+o));
      }
    }
  }
  r_index n(int i) const { return nodes[i].get(); }

  using T3 = std::tuple<r_index, r_index, r_index>;
  static std::set<T3> drain(BaseGraph::Iterator & itor, std::set<T3> seen = {})
  {
    while(not itor.is_end()) {
      EXPECT_TRUE(seen.insert({itor.get_subject(), itor.get_predicate(), itor.get_object()}).second)
        << "a triple was returned twice";
      itor.next();
    }
    return seen;
  }
  // Insert enough triples under (u, v), (u, *) and the graph to rehash every table
  void grow()
  {
    for(int i=0; i<1000; i++) {
      g->insert(n(0), n(101), n(1000+i));
      g->insert(n(0), n(1000+i), n(200));
      g->insert(n(1000+i), n(101), n(200));
    }
  }

  std::vector<Rptr> nodes;
  BaseGraphPtr g;
};

TEST_F(BaseGraphChangeTest, FindUvwReturnsOneTriple)
{
  auto itor = g->find(n(0), n(101), n(210));
  ASSERT_FALSE(itor.is_end());
  EXPECT_EQ(itor.get_object(), n(210));
  EXPECT_FALSE(itor.next());
  EXPECT_TRUE(itor.is_end());
  EXPECT_TRUE(g->find(n(0), n(101), n(999)).is_end());
}

TEST_F(BaseGraphChangeTest, InsertWhileIteratingReturnsWhatWasThere)
{
  // each find, how many triples it has before and how many grow() adds to it
  std::vector<std::tuple<std::function<BaseGraph::Iterator()>, size_t, size_t>> finds = {
    {[&]{ return g->find(); }, 647, 3000},
    {[&]{ return g->find(n(0)); }, 77, 2000},
    {[&]{ return g->find(n(0), n(101)); }, 50, 1000},
  };
  for(auto const& [find, count, added]: finds) {
    auto before = find();
    auto expected = drain(before);
    EXPECT_EQ(expected.size(), count);
    auto itor = find();
    std::set<T3> seen;
    seen.insert({itor.get_subject(), itor.get_predicate(), itor.get_object()});
    grow();
    itor.next();
    EXPECT_EQ(drain(itor, seen), expected);
    // the graph has the new triples, and a new iterator returns them
    auto after = find();
    EXPECT_EQ(drain(after).size(), count + added);
    // undo, for the next find
    for(int i=0; i<1000; i++) {
      g->erase(n(0), n(101), n(1000+i));
      g->erase(n(0), n(1000+i), n(200));
      g->erase(n(1000+i), n(101), n(200));
    }
  }
}

TEST_F(BaseGraphChangeTest, EraseWhileIteratingReturnsWhatWasThere)
{
  auto before = g->find(n(0));
  auto expected = drain(before);
  auto itor = g->find(n(0));
  // retract everything, emptying and erasing the inner tables under the iterator
  for(auto const& [s, p, o]: expected) EXPECT_EQ(g->retract(s, p, o), 1);
  EXPECT_FALSE(g->contains(n(0), n(101)));
  EXPECT_EQ(drain(itor), expected);
  EXPECT_TRUE(g->find(n(0)).is_end());
}

TEST_F(BaseGraphChangeTest, CopiesOfAFrozenIteratorAreIndependent)
{
  auto before = g->find(n(0), n(101));
  auto expected = drain(before);
  auto itor = g->find(n(0), n(101));
  std::set<T3> seen;
  seen.insert({itor.get_subject(), itor.get_predicate(), itor.get_object()});
  itor.next();
  seen.insert({itor.get_subject(), itor.get_predicate(), itor.get_object()});
  auto copy = itor;                     // live, registered twice
  BaseGraph::Iterator moved(std::move(itor));
  EXPECT_TRUE(itor.is_end());           // a moved-from iterator is at its end
  grow();                               // freezes both
  copy.next();
  moved.next();
  EXPECT_EQ(drain(copy, seen), expected);
  EXPECT_EQ(drain(moved, seen), expected);
}

TEST_F(BaseGraphChangeTest, ARefCountChangeDoesNotFreeze)
{
  auto itor = g->find(n(0), n(101));
  // an existing triple: its count goes up and down, the tables do not change
  EXPECT_FALSE(g->insert(n(0), n(101), n(200)));
  EXPECT_EQ(g->retract(n(0), n(101), n(200)), 0);
  EXPECT_EQ(drain(itor).size(), 50u);
}

}   // namespace
}   // namespace jets::rdf