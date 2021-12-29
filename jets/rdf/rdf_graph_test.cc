#include <iostream>
#include <memory>

#include <gtest/gtest.h>

#include "jets/rdf/rdf_types.h"

namespace jets::rdf {
namespace {
// Test cases for RDFGraph class
// The suite fixture for RDFGraph
class RDFGraphStlTest : public ::testing::Test {
 protected:
  RDFGraphStlTest()
    : meta_graph_p(),
      rdf_graph0_p(),
      rdf_graph1_p(),
      rdf_graph2_p()
  {
    // std::cout<<"Creating Meta Graph"<<std::endl;
    meta_graph_p = create_rdf_graph();
    auto meta_mgr_p = meta_graph_p->get_rmgr();
    init_graph(meta_graph_p);
    meta_mgr_p->set_locked();

    // std::cout<<"Creating Graph 0"<<std::endl;
    rdf_graph0_p = create_rdf_graph();
    init_graph(rdf_graph0_p);

    // std::cout<<"Creating Graph 1"<<std::endl;
    rdf_graph1_p = create_rdf_graph(meta_mgr_p);
    init_graph(rdf_graph1_p);

    // std::cout<<"Creating Graph 2"<<std::endl;
    rdf_graph2_p = create_rdf_graph(meta_mgr_p);
    init_graph(rdf_graph2_p);
  }

  // meta graph, owns meta_mgr
  RDFGraphPtr meta_graph_p;

  // simple graph without meta_mgr
  RDFGraphPtr rdf_graph0_p;

  // rdf_graph with meta_mgr
  RDFGraphPtr rdf_graph1_p;

  // another rdf_graph with same meta_mgr
  RDFGraphPtr rdf_graph2_p;

  void init_graph(RDFGraphPtr graph_p) 
  {
    auto r_mgr_p = graph_p->get_rmgr();
    auto r1 =         r_mgr_p->create_resource("r1");
    auto r2 =         r_mgr_p->create_resource("r2");
    auto has_name =   r_mgr_p->create_resource("has_name");
    auto has_age =    r_mgr_p->create_resource("has_age");
    auto has_friend = r_mgr_p->create_resource("has_friend");
    graph_p->insert(r1, has_name, std::string("John Smith"));
    graph_p->insert(r1, has_age, 35);
    graph_p->insert(r2, has_name, std::string("John Wayne"));
    graph_p->insert(r2, has_age, 41);
    graph_p->insert(r2, has_friend, r1);
  }
};

TEST_F(RDFGraphStlTest, EnsuringMetaGraphIsLocked) 
{
  // std::cout<<"Starting EnsuringMetaGraphIsLocked"<<std::endl;
  // validating the meta graph
  auto r_mgr_p = meta_graph_p->get_rmgr();
  auto r1 =         r_mgr_p->get_resource("r1");
  auto r2 =         r_mgr_p->get_resource("r2");
  auto has_name =   r_mgr_p->get_resource("has_name");
  auto has_age =    r_mgr_p->get_resource("has_age");
  auto has_friend = r_mgr_p->get_resource("has_friend");
  auto a35 =        r_mgr_p->get_literal(35);
  auto a41 =        r_mgr_p->get_literal(41);
  auto John_Smith = r_mgr_p->get_literal("John Smith");
  auto John_Wayne = r_mgr_p->get_literal("John Wayne");

  auto itor = meta_graph_p->find();
  int count = 0;
  while(not itor.is_end()) {
    count += 1;
    auto s = itor.get_subject();
    auto p = itor.get_predicate();
    auto o = itor.get_object();
    EXPECT_TRUE(s==r1 or s==r2); 
    EXPECT_TRUE(p==has_name or p==has_age or p==has_friend);
    EXPECT_TRUE(o==a35 or o==a41 or o==John_Smith or o==John_Wayne or o==r1);
    itor.next();
  }
  EXPECT_EQ(count, meta_graph_p->size());

  // ensuring the meta graph is properly locked
  EXPECT_THROW(r_mgr_p->create_resource("r4"), rdf_exception);
}

// Testing an rdf graph without a meta resource mgr
TEST_F(RDFGraphStlTest, SimpleGraphTest) 
{
  // std::cout<<"Starting SimpleGraphTest"<<std::endl;
  auto r_mgr_p = rdf_graph0_p->get_rmgr();
  auto r1 =         r_mgr_p->get_resource("r1");
  auto has_name =   r_mgr_p->get_resource("has_name");
  auto has_age =    r_mgr_p->get_resource("has_age");
  auto a35 =        r_mgr_p->get_literal(35);
  auto John_Smith = r_mgr_p->get_literal("John Smith");

  auto itor = rdf_graph0_p->find(r1);
  int count = 0;
  while(not itor.is_end()) {
    count += 1;
    auto s = itor.get_subject();
    auto p = itor.get_predicate();
    auto o = itor.get_object();
    EXPECT_TRUE(s==r1); 
    EXPECT_TRUE(p==has_name or p==has_age);
    EXPECT_TRUE(o==a35 or o==John_Smith);
    itor.next();
  }
  EXPECT_EQ(count, 2);

  // ensuring the r_manager is not locked
  auto r3 = r_mgr_p->create_resource("r3");
  auto size = rdf_graph0_p->size();
  rdf_graph0_p->insert(r3, has_name, std::string("John Smith"));
  EXPECT_EQ(rdf_graph0_p->size(), size+1);
}

// Testing a shared meta_mgr with 2 graphs
TEST_F(RDFGraphStlTest, SharedMetaMgrTest) 
{
  // std::cout<<"Starting SharedMetaMgrTest"<<std::endl;
  auto r_mgr_p = meta_graph_p->get_rmgr();
  auto r1 =         r_mgr_p->get_resource("r1");
  auto r2 =         r_mgr_p->get_resource("r2");
  auto has_age =    r_mgr_p->get_resource("has_age");
  auto a35 =        r_mgr_p->get_literal(35);
  auto a41 =        r_mgr_p->get_literal(41);

  auto any = make_any();

  // adding a triple to rdf_graph1_p, make sure it's not in rdf_graph2_p
  auto g1_mgr_p = rdf_graph1_p->get_rmgr();
  auto r3 = g1_mgr_p->create_resource("r3");
  auto xx = g1_mgr_p->create_resource("has_age");
  EXPECT_TRUE(has_age == xx);
  rdf_graph1_p->insert(r3, has_age, a35);
  // test (*, r, *)
  auto itor = rdf_graph1_p->find(any, has_age, any);
  int count = 0;
  while(not itor.is_end()) {
    count += 1;
    auto s = itor.get_subject();
    auto p = itor.get_predicate();
    auto o = itor.get_object();
    EXPECT_TRUE(s==r1 or s==r2 or s==r3 ); 
    EXPECT_TRUE(p==has_age);
    EXPECT_TRUE(o==a35 or o==a41);
    itor.next();
  }
  EXPECT_EQ(count, 3);

  // make sure it did not affect rdf_graph2_p
  auto g2_mgr_p = rdf_graph2_p->get_rmgr();
  auto rr3 = g2_mgr_p->get_resource("r3");
  EXPECT_TRUE(rr3 == nullptr);

  // test (*, *, r)
  auto jtor = rdf_graph2_p->find(any, any, a35);
  count = 0;
  while(not jtor.is_end()) {
    count += 1;
    auto s = jtor.get_subject();
    auto p = jtor.get_predicate();
    auto o = jtor.get_object();
    EXPECT_TRUE(s==r1); 
    EXPECT_TRUE(p==has_age);
    EXPECT_TRUE(o==a35);
    jtor.next();
  }
  EXPECT_EQ(count, 1);
}

// Testing various find argument combinations
TEST_F(RDFGraphStlTest, AllOrRIndexFindTest) 
{
  // std::cout<<"Starting SharedMetaMgrTest"<<std::endl;
  auto r_mgr_p = rdf_graph1_p->get_rmgr();
  auto r1 =         r_mgr_p->get_resource("r1");
  auto r2 =         r_mgr_p->get_resource("r2");
  auto has_name =   r_mgr_p->get_resource("has_name");
  auto has_age =    r_mgr_p->get_resource("has_age");
  auto has_friend = r_mgr_p->get_resource("has_friend");
  auto a35 =        r_mgr_p->get_literal(35);
  auto a41 =        r_mgr_p->get_literal(41);
  auto John_Smith = r_mgr_p->get_literal("John Smith");
  auto John_Wayne = r_mgr_p->get_literal("John Wayne");

  auto any = make_any();

  // test (*, *, *)
  auto itor = rdf_graph1_p->find(any, any, any);
  int count = 0;
  while(not itor.is_end()) {
    count += 1;
    auto s = itor.get_subject();
    auto p = itor.get_predicate();
    auto o = itor.get_object();
    EXPECT_TRUE(s==r1 or s==r2); 
    EXPECT_TRUE(p==has_name or p==has_age or p==has_friend);
    EXPECT_TRUE(o==a35 or o==a41 or o==John_Smith or o==John_Wayne or o==r1);
    itor.next();
  }
  EXPECT_EQ(count, rdf_graph1_p->size());

  // test (r, *, *)
  itor = rdf_graph1_p->find(r1, any, any);
  count = 0;
  while(not itor.is_end()) {
    count += 1;
    auto s = itor.get_subject();
    auto p = itor.get_predicate();
    auto o = itor.get_object();
    EXPECT_TRUE(s==r1 or s==r2); 
    EXPECT_TRUE(p==has_name or p==has_age or p==has_friend);
    EXPECT_TRUE(o==a35 or o==a41 or o==John_Smith or o==John_Wayne or o==r1);
    itor.next();
  }
  EXPECT_EQ(count, 2);

  // test (r, r, *)
  itor = rdf_graph1_p->find(r1, has_name, any);
  count = 0;
  while(not itor.is_end()) {
    count += 1;
    auto s = itor.get_subject();
    auto p = itor.get_predicate();
    auto o = itor.get_object();
    EXPECT_TRUE(s==r1 or s==r2); 
    EXPECT_TRUE(p==has_name or p==has_age or p==has_friend);
    EXPECT_TRUE(o==a35 or o==a41 or o==John_Smith or o==John_Wayne or o==r1);
    itor.next();
  }
  EXPECT_EQ(count, 1);

  // test (r, r, r)
  itor = rdf_graph1_p->find(r1, has_name, John_Smith);
  count = 0;
  while(not itor.is_end()) {
    count += 1;
    auto s = itor.get_subject();
    auto p = itor.get_predicate();
    auto o = itor.get_object();
    EXPECT_TRUE(s==r1 or s==r2); 
    EXPECT_TRUE(p==has_name or p==has_age or p==has_friend);
    EXPECT_TRUE(o==a35 or o==a41 or o==John_Smith or o==John_Wayne or o==r1);
    itor.next();
  }
  EXPECT_EQ(count, 1);

  // test (*, r, r)
  itor = rdf_graph1_p->find(any, has_name, John_Smith);
  count = 0;
  while(not itor.is_end()) {
    count += 1;
    auto s = itor.get_subject();
    auto p = itor.get_predicate();
    auto o = itor.get_object();
    EXPECT_TRUE(s==r1 or s==r2); 
    EXPECT_TRUE(p==has_name or p==has_age or p==has_friend);
    EXPECT_TRUE(o==a35 or o==a41 or o==John_Smith or o==John_Wayne or o==r1);
    itor.next();
  }
  EXPECT_EQ(count, 1);

  // test (r, *, r)
  itor = rdf_graph1_p->find(r1, any, John_Smith);
  count = 0;
  while(not itor.is_end()) {
    count += 1;
    auto s = itor.get_subject();
    auto p = itor.get_predicate();
    auto o = itor.get_object();
    EXPECT_TRUE(s==r1 or s==r2); 
    EXPECT_TRUE(p==has_name or p==has_age or p==has_friend);
    EXPECT_TRUE(o==a35 or o==a41 or o==John_Smith or o==John_Wayne or o==r1);
    itor.next();
  }
  EXPECT_EQ(count, 1);
}

}   // namespace
}   // namespace jets::rdf