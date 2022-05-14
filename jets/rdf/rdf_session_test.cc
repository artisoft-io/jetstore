#include <iostream>
#include <memory>

#include <gtest/gtest.h>

#include "../rdf/rdf_types.h"

namespace jets::rdf {
namespace {
// Test cases for RDFSession class
// The suite fixture for RDFSession
// simple test for bug #154
//   duplicate triples when inferred and asserted
TEST(RDFSessionInsert, TestInsert1)
{
  auto meta_graph_p = create_rdf_graph();
  meta_graph_p->set_locked();
  auto rdf_session_p = create_rdf_session(meta_graph_p);

  auto r_mgr_p = rdf_session_p->get_rmgr();
  auto s=   r_mgr_p->create_resource("s");
  auto p=   r_mgr_p->create_resource("p");
  auto o=   r_mgr_p->create_resource("o");

  rdf_session_p->insert(s, p, o);
  EXPECT_EQ(rdf_session_p->size(), 1);

  rdf_session_p->insert_inferred(s, p, o);
  EXPECT_EQ(rdf_session_p->size(), 1);

}
TEST(RDFSessionInsert, TestInsert2)
{
  auto meta_graph_p = create_rdf_graph();
  meta_graph_p->set_locked();
  auto rdf_session_p = create_rdf_session(meta_graph_p);

  auto r_mgr_p = rdf_session_p->get_rmgr();
  auto s=   r_mgr_p->create_resource("s");
  auto p=   r_mgr_p->create_resource("p");
  auto o=   r_mgr_p->create_resource("o");

  rdf_session_p->insert_inferred(s, p, o);
  rdf_session_p->insert_inferred(s, p, o);
  EXPECT_EQ(rdf_session_p->size(), 1);

  rdf_session_p->retract(s, p, o);
  EXPECT_EQ(rdf_session_p->size(), 1);

  rdf_session_p->retract(s, p, o);
  EXPECT_EQ(rdf_session_p->size(), 0);
}
TEST(RDFSessionInsert, TestInsert3)
{
  auto meta_graph_p = create_rdf_graph();
  meta_graph_p->set_locked();
  auto rdf_session_p = create_rdf_session(meta_graph_p);

  auto r_mgr_p = rdf_session_p->get_rmgr();
  auto s=   r_mgr_p->create_resource("s");
  auto p=   r_mgr_p->create_resource("p");
  auto o=   r_mgr_p->create_resource("o");

  rdf_session_p->insert_inferred(s, p, o);
  rdf_session_p->insert_inferred(s, p, o);
  EXPECT_EQ(rdf_session_p->size(), 1);

  rdf_session_p->insert(s, p, o);
  EXPECT_EQ(rdf_session_p->size(), 1);

  rdf_session_p->retract(s, p, o);
  rdf_session_p->retract(s, p, o);
  EXPECT_EQ(rdf_session_p->size(), 1);
}
TEST(RDFSessionInsert, TestInsert4)
{
  auto meta_graph_p = create_rdf_graph();
  auto r_mgr_p = meta_graph_p->get_rmgr();
  auto ms=   r_mgr_p->create_resource("s");
  auto mp=   r_mgr_p->create_resource("p");
  auto mo=   r_mgr_p->create_resource("o");
  meta_graph_p->insert(ms, mp, mo);
  meta_graph_p->set_locked();
  auto rdf_session_p = create_rdf_session(meta_graph_p);

  r_mgr_p = rdf_session_p->get_rmgr();
  auto s=   r_mgr_p->create_resource("s");
  auto p=   r_mgr_p->create_resource("p");
  auto o=   r_mgr_p->create_resource("o");
  EXPECT_EQ(s, ms);
  EXPECT_EQ(p, mp);
  EXPECT_EQ(o, mo);

  EXPECT_EQ(meta_graph_p->size(), 1);

  rdf_session_p->insert_inferred(s, p, o);
  rdf_session_p->insert_inferred(s, p, o);
  EXPECT_EQ(rdf_session_p->size(), 2);

  rdf_session_p->insert(s, p, o);
  EXPECT_EQ(rdf_session_p->size(), 2);

  rdf_session_p->retract(s, p, o);
  rdf_session_p->retract(s, p, o);
  rdf_session_p->retract(s, p, o);
  EXPECT_EQ(rdf_session_p->size(), 2);

  rdf_session_p->erase(s, p, o);
  EXPECT_EQ(rdf_session_p->size(), 1);
}

TEST(RDFSessionFind, TestFind1)
{
  auto meta_graph_p = create_rdf_graph();
  meta_graph_p->set_locked();
  auto rdf_session_p = create_rdf_session(meta_graph_p);

  auto r_mgr_p = rdf_session_p->get_rmgr();
  auto s=   r_mgr_p->create_resource("s");
  auto p=   r_mgr_p->create_resource("p");
  auto o1=  r_mgr_p->create_resource("o1");
  auto o2=  r_mgr_p->create_resource("o2");

  rdf_session_p->insert(s, p, o1);
  rdf_session_p->insert_inferred(s, p, o2);
  EXPECT_EQ(rdf_session_p->size(), 2);
  // auto itor = rdf_session_p->find();
  // while(!itor.is_end()) {
  //   std::cout<<"find(): "<<itor.as_triple()<<" -- get_object "<<itor.get_object()<<std::endl;
  //   itor.next();
  // }
  // itor = rdf_session_p->find(s, p);
  // while(!itor.is_end()) {
  //   std::cout<<"find(s, p): "<<itor.as_triple()<<" -- get_object "<<itor.get_object()<<std::endl;
  //   itor.next();
  // }
  // std::cout<<"rdf_session::get_object(s, p) = "<<rdf_session_p->get_object(s, p)<<std::endl;
  // std::cout<<"rdf_session::get_object(s, o1) = "<<rdf_session_p->get_object(s, o1)<<std::endl;
  EXPECT_EQ(rdf_session_p->get_object(s, p), o1);
  EXPECT_EQ(rdf_session_p->get_object(s, o1), nullptr);
}

class RDFSessionStlTest : public ::testing::Test {
 protected:
  RDFSessionStlTest(): rdf_session_p()
  {
    // std::cout<<"Creating Meta Graph"<<std::endl;
    auto meta_graph_p = create_rdf_graph();
    init_graph(meta_graph_p, "r1", "John Smith");
    meta_graph_p->set_locked();

    rdf_session_p = create_rdf_session(meta_graph_p);
    auto graph_p = rdf_session_p->get_asserted_graph();
    init_graph(graph_p, "r2", "John Wayne");

    graph_p = rdf_session_p->get_inferred_graph();
    init_graph(graph_p, "r3", "Peter Pan");

    auto r_mgr_p = rdf_session_p->get_rmgr();
    r1 =          r_mgr_p->get_resource("r1");
    r2 =          r_mgr_p->get_resource("r2");
    r3 =          r_mgr_p->get_resource("r3");
    has_name =   r_mgr_p->get_resource("has_name");
    has_age =    r_mgr_p->get_resource("has_age");
    a35 =        r_mgr_p->get_literal(35);
    name1 =       r_mgr_p->get_literal("John Smith");
    name2 =       r_mgr_p->get_literal("John Wayne");
    name3 =       r_mgr_p->get_literal("Peter Pan");
  }

  RDFSessionPtr rdf_session_p;
  r_index r1 ;
  r_index r2 ;
  r_index r3 ;
  r_index has_name ;
  r_index has_age ;
  r_index a35 ;
  r_index name1 ;
  r_index name2 ;
  r_index name3 ;

  void init_graph(RDFGraphPtr graph_p, std::string ss, std::string name)
  {
      auto r_mgr_p = graph_p->get_rmgr();
      auto s =          r_mgr_p->create_resource(ss);
      auto has_name =   r_mgr_p->create_resource("has_name");
      auto has_age =    r_mgr_p->create_resource("has_age");
      graph_p->insert(s, has_name, name);
      graph_p->insert(s, has_age, 35);
  }
};

TEST_F(RDFSessionStlTest, EnsuringMetaGraphIsLocked) 
{
  auto itor = rdf_session_p->find();
  int count = 0;
  while(not itor.is_end()) {
      count += 1;
      auto s = itor.get_subject();
      auto p = itor.get_predicate();
      auto o = itor.get_object();
      // std::cout << "*** ("<<s<<", "<<p<<", "<<o<<")"<<std::endl;
      EXPECT_TRUE(s==r1 or s==r2 or s==r3); 
      EXPECT_TRUE(p==has_name or p==has_age);
      EXPECT_TRUE(o==a35  or o==name1 or o==name2 or o==name3);
      itor.next();
  }
  EXPECT_EQ(count, rdf_session_p->size());

  // ensuring the meta graph is properly locked
  auto meta_graph_p = rdf_session_p->get_meta_graph();
  EXPECT_THROW(meta_graph_p->insert(r1, has_name, name1), rdf_exception);
  EXPECT_THROW(meta_graph_p->erase(r1, has_name, name1), rdf_exception);
  EXPECT_THROW(meta_graph_p->retract(r1, has_name, name1), rdf_exception);
}

TEST_F(RDFSessionStlTest, TestFindSSS) 
{
  auto any = make_any();
  auto itor = rdf_session_p->find(any, any, any);
  int count = 0;
  while(not itor.is_end()) {
      count += 1;
      auto s = itor.get_subject();
      auto p = itor.get_predicate();
      auto o = itor.get_object();
      // std::cout << "*** ("<<s<<", "<<p<<", "<<o<<")"<<std::endl;
      EXPECT_TRUE(s==r1 or s==r2 or s==r3); 
      EXPECT_TRUE(p==has_name or p==has_age);
      EXPECT_TRUE(o==a35  or o==name1 or o==name2 or o==name3);
      itor.next();
  }
  EXPECT_EQ(count, rdf_session_p->size());
}

TEST_F(RDFSessionStlTest, TestFindRSS) 
{
  auto any = make_any();
  auto itor = rdf_session_p->find(r1, any, any);
  int count = 0;
  while(not itor.is_end()) {
      count += 1;
      auto s = itor.get_subject();
      auto p = itor.get_predicate();
      auto o = itor.get_object();
      // std::cout << "*** ("<<s<<", "<<p<<", "<<o<<")"<<std::endl;
      EXPECT_TRUE(s==r1); 
      EXPECT_TRUE(p==has_name or p==has_age);
      EXPECT_TRUE(o==a35  or o==name1);
      itor.next();
  }
  EXPECT_EQ(count, 2);
}

TEST_F(RDFSessionStlTest, TestFindSRS) 
{
  auto any = make_any();
  auto itor = rdf_session_p->find(any, has_age, any);
  int count = 0;
  while(not itor.is_end()) {
      count += 1;
      auto s = itor.get_subject();
      auto p = itor.get_predicate();
      auto o = itor.get_object();
      // std::cout << "*** ("<<s<<", "<<p<<", "<<o<<")"<<std::endl;
      EXPECT_TRUE(s==r1 or s==r2 or s==r3); 
      EXPECT_TRUE(p==has_age);
      EXPECT_TRUE(o==a35);
      itor.next();
  }
  EXPECT_EQ(count, 3);
}

TEST_F(RDFSessionStlTest, TestFindRSR) 
{
  auto any = make_any();
  auto itor = rdf_session_p->find(r1, any, a35);
  int count = 0;
  while(not itor.is_end()) {
      count += 1;
      auto s = itor.get_subject();
      auto p = itor.get_predicate();
      auto o = itor.get_object();
      // std::cout << "*** ("<<s<<", "<<p<<", "<<o<<")"<<std::endl;
      EXPECT_TRUE(s==r1); 
      EXPECT_TRUE(p==has_age);
      EXPECT_TRUE(o==a35);
      itor.next();
  }
  EXPECT_EQ(count, 1);
}

TEST_F(RDFSessionStlTest, TestFindSRR) 
{
  auto any = make_any();
  auto itor = rdf_session_p->find(any, has_age, a35);
  int count = 0;
  while(not itor.is_end()) {
      count += 1;
      auto s = itor.get_subject();
      auto p = itor.get_predicate();
      auto o = itor.get_object();
      // std::cout << "*** ("<<s<<", "<<p<<", "<<o<<")"<<std::endl;
      EXPECT_TRUE(s==r1 or s==r2 or s==r3); 
      EXPECT_TRUE(p==has_age);
      EXPECT_TRUE(o==a35);
      itor.next();
  }
  EXPECT_EQ(count, 3);
}

TEST_F(RDFSessionStlTest, TestFindRRS) 
{
  auto any = make_any();
  auto itor = rdf_session_p->find(r1, has_name, any);
  int count = 0;
  while(not itor.is_end()) {
      count += 1;
      auto s = itor.get_subject();
      auto p = itor.get_predicate();
      auto o = itor.get_object();
      // std::cout << "*** ("<<s<<", "<<p<<", "<<o<<")"<<std::endl;
      EXPECT_TRUE(s==r1); 
      EXPECT_TRUE(p==has_name);
      EXPECT_TRUE(o==name1);
      itor.next();
  }
  EXPECT_EQ(count, 1);
}

}   // namespace
}   // namespace jets::rdf