#include <cstddef>
#include <iostream>
#include <string>
#include <memory>

#include <gtest/gtest.h>

#include "../rdf/rdf_types.h"
#include "../rete/rete_types.h"
#include "../rete/expr_op_logics.h"
#include "../rete/expr_op_arithmetics.h"
#include "../rete/expr_op_strings.h"
#include "../rete/expr_op_resources.h"
#include "../rete/expr_op_others.h"
// This file contains test cases for the specialty operators: 
//  sorted_head, max_of, min_of, sum_values
namespace jets::rete {
namespace {
// Arithmetic operators test
// The suite fixture for expr operator tests
class ExprOpSpecialtyTest : public ::testing::Test {
 protected:
  ExprOpSpecialtyTest() 
    : rete_session(), rete_meta_store(), rdf_session() 
    {
    auto meta_graph = rdf::create_rdf_graph();
    auto * rmgr = meta_graph->rmgr();
    rmgr->initialize();

    // ReteMetaStore
    NodeVertexVector   node_vertexes;
    ReteMetaStore::AlphaNodeVector alpha_nodes;
    // create & initalize the meta store
    rete_meta_store = create_rete_meta_store({}, alpha_nodes, node_vertexes);
    rete_meta_store->initialize();
    rmgr->set_locked();
    meta_graph->set_locked();

    // Create the rdf_session and the rete_session and initialize them
    // Initialize the rete_session now that the rule base is ready
    this->rdf_session = rdf::create_rdf_session(meta_graph);
    this->rete_session = create_rete_session(rete_meta_store, this->rdf_session.get());
    this->rete_session->initialize();

    // initialize the rdf session
    this->init_test();
  }

  void init_test()
  {
    // Create a set of test data in the rdf session
    auto rmgr = this->rdf_session->rmgr();
    auto const* jets = rmgr->jets();
    // schema
    rdf::r_index me = rmgr->create_resource("MainEntity");
    rdf::r_index se = rmgr->create_resource("SupportEntity");
    rdf::r_index has = rmgr->create_resource("hasSupport");
    rdf::r_index value = rmgr->create_resource("value");
    rdf::r_index values = rmgr->create_resource("values");
    rdf::r_index name = rmgr->create_resource("name");

    // config for min/max
    rdf::r_index config1 = rmgr->create_resource("config1");
    this->rdf_session->insert(config1, jets->jets__entity_property, has);
    this->rdf_session->insert(config1, jets->jets__value_property, value);

    // instance
    rdf::r_index main1 = rmgr->create_resource("main1");
    this->rdf_session->insert(main1, jets->rdf__type, me);
    this->rdf_session->insert(main1, values, 10);
    this->rdf_session->insert(main1, values, 20);
    this->rdf_session->insert(main1, values, 30);
    {
      rdf::r_index s1 = rmgr->create_resource("support1");
      this->rdf_session->insert(s1, jets->rdf__type, se);
      this->rdf_session->insert(main1, has, s1);
      this->rdf_session->insert(s1, name, std::string("name1"));
      this->rdf_session->insert(s1, value, 1);
    }
    {
      rdf::r_index s2 = rmgr->create_resource("support2");
      this->rdf_session->insert(s2, jets->rdf__type, se);
      this->rdf_session->insert(main1, has, s2);
      this->rdf_session->insert(s2, name, std::string("name2"));
      this->rdf_session->insert(s2, value, 2);
    }
    {
      rdf::r_index s3 = rmgr->create_resource("support3");
      this->rdf_session->insert(s3, jets->rdf__type, se);
      this->rdf_session->insert(main1, has, s3);
      this->rdf_session->insert(s3, name, std::string("name3"));
      this->rdf_session->insert(s3, value, 3);
    }
  }

  ReteSessionPtr  rete_session;
  ReteMetaStorePtr rete_meta_store;
  rdf::RDFSessionPtr   rdf_session;
};

// Define the tests
// -----------------------------------------------------------------------------------
TEST_F(ExprOpSpecialtyTest, MinOfVisitor1) {
  // test min ?v in (s, p, ?v)
  MinOfVisitor op(this->rete_session.get(), nullptr);
  rdf::NamedResource lhs("main1");
  rdf::NamedResource rhs("values");
  auto res = boost::apply_visitor(op, rdf::RdfAstType(lhs), rdf::RdfAstType(rhs));
  EXPECT_EQ(res, rdf::RdfAstType(rdf::LInt32(10)));
}
TEST_F(ExprOpSpecialtyTest, MinOfVisitorX1) {
  // test min ?v in (s, p, ?v)
  auto rmgr = this->rdf_session->rmgr();
  auto s = rmgr->create_resource(std::string("s"));
  auto p = rmgr->create_resource(std::string("p"));
  this->rdf_session->insert(s, p, rmgr->create_literal(int(1)));
  this->rdf_session->insert(s, p, rmgr->create_literal(int(2)));
  this->rdf_session->insert(s, p, rmgr->create_literal(int(3)));
  MinOfVisitor op(this->rete_session.get(), nullptr);
  rdf::NamedResource lhs("s");
  rdf::NamedResource rhs("p");
  auto res = boost::apply_visitor(op, rdf::RdfAstType(lhs), rdf::RdfAstType(rhs));
  EXPECT_EQ(res, rdf::RdfAstType(rdf::LInt32(1)));
}
TEST_F(ExprOpSpecialtyTest, MinOfVisitorX2) {
  // test min ?v in (s, p, ?v)
  auto rmgr = this->rdf_session->rmgr();
  auto s = rmgr->create_resource(std::string("s"));
  auto p = rmgr->create_resource(std::string("p"));
  this->rdf_session->insert(s, p, rmgr->create_literal(rdf::boost_date_from_string("2023-06-21")));
  this->rdf_session->insert(s, p, rmgr->create_literal(rdf::boost_date_from_string("2023-06-22")));
  this->rdf_session->insert(s, p, rmgr->create_literal(rdf::boost_date_from_string("2023-06-23")));
  MinOfVisitor op(this->rete_session.get(), nullptr);
  rdf::NamedResource lhs("s");
  rdf::NamedResource rhs("p");
  auto res = boost::apply_visitor(op, rdf::RdfAstType(lhs), rdf::RdfAstType(rhs));
  EXPECT_EQ(res, rdf::RdfAstType(rdf::LDate(rdf::boost_date_from_string("2023-06-21"))));
}
TEST_F(ExprOpSpecialtyTest, MinOfVisitorX3) {
  // test min ?v in (s, p, ?v)
  auto rmgr = this->rdf_session->rmgr();
  auto s = rmgr->create_resource(std::string("s"));
  auto p = rmgr->create_resource(std::string("p"));
  this->rdf_session->insert(s, p, rmgr->create_literal(rdf::boost_date_from_string("2023-06-01")));
  this->rdf_session->insert(s, p, rmgr->create_literal(rdf::boost_date_from_string("2023-06-06")));
  MinOfVisitor op(this->rete_session.get(), nullptr);
  rdf::NamedResource lhs("s");
  rdf::NamedResource rhs("p");
  auto res = boost::apply_visitor(op, rdf::RdfAstType(lhs), rdf::RdfAstType(rhs));
  EXPECT_EQ(res, rdf::RdfAstType(rdf::LDate(rdf::boost_date_from_string("2023-06-01"))));
}
TEST_F(ExprOpSpecialtyTest, MaxOfVisitorX3) {
  // test min ?v in (s, p, ?v)
  auto rmgr = this->rdf_session->rmgr();
  auto s = rmgr->create_resource(std::string("s"));
  auto p = rmgr->create_resource(std::string("p"));
  this->rdf_session->insert(s, p, rmgr->create_literal(rdf::boost_date_from_string("2023-06-01")));
  this->rdf_session->insert(s, p, rmgr->create_literal(rdf::boost_date_from_string("2023-06-06")));
  MaxOfVisitor op(this->rete_session.get(), nullptr);
  rdf::NamedResource lhs("s");
  rdf::NamedResource rhs("p");
  auto res = boost::apply_visitor(op, rdf::RdfAstType(lhs), rdf::RdfAstType(rhs));
  EXPECT_EQ(res, rdf::RdfAstType(rdf::LDate(rdf::boost_date_from_string("2023-06-06"))));
}
TEST_F(ExprOpSpecialtyTest, MinOfVisitor2) {
  // test min ?v in (s, objp, ?o).(?o, datap, ?v)
  MinOfVisitor op(this->rete_session.get(), nullptr);
  rdf::NamedResource lhs("main1");
  rdf::NamedResource rhs("config1");
  auto res = boost::apply_visitor(op, rdf::RdfAstType(lhs), rdf::RdfAstType(rhs));
  EXPECT_EQ(res, rdf::RdfAstType(rdf::LInt32(1)));
}
TEST_F(ExprOpSpecialtyTest, MaxOfVisitor1) {
  // test max ?v in (s, p, ?v)
  MaxOfVisitor op(this->rete_session.get(), nullptr);
  rdf::NamedResource lhs("main1");
  rdf::NamedResource rhs("values");
  auto res = boost::apply_visitor(op, rdf::RdfAstType(lhs), rdf::RdfAstType(rhs));
  EXPECT_EQ(res, rdf::RdfAstType(rdf::LInt32(30)));
}
TEST_F(ExprOpSpecialtyTest, MaxOfVisitor2) {
  // test max ?v in (s, objp, ?o).(?o, datap, ?v)
  MaxOfVisitor op(this->rete_session.get(), nullptr);
  rdf::NamedResource lhs("main1");
  rdf::NamedResource rhs("config1");
  auto res = boost::apply_visitor(op, rdf::RdfAstType(lhs), rdf::RdfAstType(rhs));
  EXPECT_EQ(res, rdf::RdfAstType(rdf::LInt32(3)));
}

TEST_F(ExprOpSpecialtyTest, SortedHeadVisitor1) {
  // test get ?o such that max ?v in (s, objp, ?o).(?o datap ?v)
  // This test case has the following config:
  //  - sort operator: >
  //  - entity property (obj property): has 
  //  - value property (data property): value
  auto rmgr = this->rdf_session->rmgr();
  auto const* jets = rmgr->jets();
  // operator config obj
  rdf::r_index config = rmgr->create_resource("config");
  rdf::r_index has = rmgr->create_resource("hasSupport");
  rdf::r_index value = rmgr->create_resource("value");
  this->rdf_session->insert(config, jets->jets__entity_property, has);
  this->rdf_session->insert(config, jets->jets__value_property, value);
  this->rdf_session->insert(config, jets->jets__operator, std::string("<"));

  SortedHeadVisitor op(this->rete_session.get(), nullptr);
  rdf::NamedResource lhs("main1");
  rdf::NamedResource rhs("config");
  auto res = boost::apply_visitor(op, rdf::RdfAstType(lhs), rdf::RdfAstType(rhs));
  EXPECT_EQ(res, rdf::RdfAstType(rdf::NamedResource("support1")));
}

TEST_F(ExprOpSpecialtyTest, SortedHeadVisitor2) {
  // test get ?o such that max ?v in (s, objp, ?o).(?o datap ?v)
  // This test case has the following config:
  //  - sort operator: >
  //  - entity property (obj property): has 
  //  - value property (data property): value
  auto rmgr = this->rdf_session->rmgr();
  auto const* jets = rmgr->jets();
  // operator config obj
  rdf::r_index config = rmgr->create_resource("config");
  rdf::r_index has = rmgr->create_resource("hasSupport");
  rdf::r_index value = rmgr->create_resource("value");
  this->rdf_session->insert(config, jets->jets__entity_property, has);
  this->rdf_session->insert(config, jets->jets__value_property, value);
  this->rdf_session->insert(config, jets->jets__operator, std::string(">"));

  SortedHeadVisitor op(this->rete_session.get(), nullptr);
  rdf::NamedResource lhs("main1");
  rdf::NamedResource rhs("config");
  auto res = boost::apply_visitor(op, rdf::RdfAstType(lhs), rdf::RdfAstType(rhs));
  EXPECT_EQ(res, rdf::RdfAstType(rdf::NamedResource("support3")));
}

TEST_F(ExprOpSpecialtyTest, SumValuesVisitor1) {
  // test sum ?o in (s, p, ?o)
  auto rmgr = this->rdf_session->rmgr();
  auto const* jets = rmgr->jets();
  // operator config obj
  rdf::r_index config = rmgr->create_resource("config");
  rdf::r_index values = rmgr->create_resource("values");
  this->rdf_session->insert(config, jets->jets__value_property, values);

  SumValuesVisitor op(this->rete_session.get(), nullptr);
  rdf::NamedResource lhs("main1");
  rdf::NamedResource rhs("config");
  auto res = boost::apply_visitor(op, rdf::RdfAstType(lhs), rdf::RdfAstType(rhs));
  EXPECT_EQ(res, rdf::RdfAstType(rdf::LInt32(60)));
}

TEST_F(ExprOpSpecialtyTest, SumValuesVisitor2) {
  // test sum ?v in (s, objp, ?o).(?o, datap, ?v)
  auto rmgr = this->rdf_session->rmgr();
  auto const* jets = rmgr->jets();
  // operator config obj
  rdf::r_index config = rmgr->create_resource("config");
  rdf::r_index has = rmgr->create_resource("hasSupport");
  rdf::r_index value = rmgr->create_resource("value");
  this->rdf_session->insert(config, jets->jets__entity_property, has);
  this->rdf_session->insert(config, jets->jets__value_property, value);

  SumValuesVisitor op(this->rete_session.get(), nullptr);
  rdf::NamedResource lhs("main1");
  rdf::NamedResource rhs("config");
  auto res = boost::apply_visitor(op, rdf::RdfAstType(lhs), rdf::RdfAstType(rhs));
  EXPECT_EQ(res, rdf::RdfAstType(rdf::LInt32(6)));
}

}   // namespace
}   // namespace jets::rdf