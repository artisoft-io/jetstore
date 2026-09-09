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
//  sorted_head, max_of, min_of, sum_values, join_values
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


// JOIN VALUES
// -----------------------------------------------------------------------------------
// The Go engine carries the same cases in
// jets/jetrules/rete/expr_operator_math_join_values_test.go; the two implementations
// are independently maintained, so the tests are deliberately parallel.

// Form 3: rhs is itself the non functional data property, default separator.
TEST_F(ExprOpSpecialtyTest, JoinValuesVisitor1) {
  // test join ?v in (s, p, ?v)
  JoinValuesVisitor op(this->rete_session.get(), nullptr);
  rdf::NamedResource lhs("main1");
  rdf::NamedResource rhs("values");
  auto res = boost::apply_visitor(op, rdf::RdfAstType(lhs), rdf::RdfAstType(rhs));
  EXPECT_EQ(res, rdf::RdfAstType(rdf::LString(std::string("10, 20, 30"))));
}

// Form 2: config carrying jets:value_property alone, with an explicit separator.
TEST_F(ExprOpSpecialtyTest, JoinValuesVisitor2) {
  auto rmgr = this->rdf_session->rmgr();
  auto const* jets = rmgr->jets();
  rdf::r_index config = rmgr->create_resource("configJ2");
  rdf::r_index values = rmgr->create_resource("values");
  this->rdf_session->insert(config, jets->jets__value_property, values);
  this->rdf_session->insert(config, jets->jets__separator, std::string("-"));

  JoinValuesVisitor op(this->rete_session.get(), nullptr);
  rdf::NamedResource lhs("main1");
  rdf::NamedResource rhs("configJ2");
  auto res = boost::apply_visitor(op, rdf::RdfAstType(lhs), rdf::RdfAstType(rhs));
  EXPECT_EQ(res, rdf::RdfAstType(rdf::LString(std::string("10-20-30"))));
}

// Form 1: jets:entity_property + jets:value_property, default separator.
TEST_F(ExprOpSpecialtyTest, JoinValuesVisitor3) {
  auto rmgr = this->rdf_session->rmgr();
  auto const* jets = rmgr->jets();
  rdf::r_index config = rmgr->create_resource("configJ3");
  rdf::r_index has = rmgr->create_resource("hasSupport");
  rdf::r_index name = rmgr->create_resource("name");
  this->rdf_session->insert(config, jets->jets__entity_property, has);
  this->rdf_session->insert(config, jets->jets__value_property, name);

  JoinValuesVisitor op(this->rete_session.get(), nullptr);
  rdf::NamedResource lhs("main1");
  rdf::NamedResource rhs("configJ3");
  auto res = boost::apply_visitor(op, rdf::RdfAstType(lhs), rdf::RdfAstType(rhs));
  EXPECT_EQ(res, rdf::RdfAstType(rdf::LString(std::string("name1, name2, name3"))));
}

// The output is sorted, not in insertion order: the fill dates go in unsorted and come
// out chronological, which is what ISO-8601 lexical order buys.
TEST_F(ExprOpSpecialtyTest, JoinValuesVisitorSorted) {
  auto rmgr = this->rdf_session->rmgr();
  auto const* jets = rmgr->jets();
  rdf::r_index config = rmgr->create_resource("configJS");
  rdf::r_index hasFill = rmgr->create_resource("hasFill");
  rdf::r_index fillDate = rmgr->create_resource("fillDate");
  this->rdf_session->insert(config, jets->jets__entity_property, hasFill);
  this->rdf_session->insert(config, jets->jets__value_property, fillDate);

  rdf::r_index med = rmgr->create_resource("lisinopril");
  char const* dates[] = {"2025-08-24", "2025-06-15", "2025-07-20"};
  for(int i=0; i<3; ++i) {
    auto f = rmgr->create_resource(std::string("fill")+std::to_string(i));
    this->rdf_session->insert(med, hasFill, f);
    this->rdf_session->insert(f, fillDate, rdf::boost_date_from_string(dates[i]));
  }

  JoinValuesVisitor op(this->rete_session.get(), nullptr);
  rdf::NamedResource lhs("lisinopril");
  rdf::NamedResource rhs("configJS");
  auto res = boost::apply_visitor(op, rdf::RdfAstType(lhs), rdf::RdfAstType(rhs));
  EXPECT_EQ(res, rdf::RdfAstType(
    rdf::LString(std::string("2025-06-15, 2025-07-20, 2025-08-24"))));
}

// A single value carries no separator.
TEST_F(ExprOpSpecialtyTest, JoinValuesVisitorSingleValue) {
  auto rmgr = this->rdf_session->rmgr();
  auto s = rmgr->create_resource(std::string("sJ1"));
  auto p = rmgr->create_resource(std::string("pJ1"));
  this->rdf_session->insert(s, p, std::string("2025-07-02"));

  JoinValuesVisitor op(this->rete_session.get(), nullptr);
  rdf::NamedResource lhs("sJ1");
  rdf::NamedResource rhs("pJ1");
  auto res = boost::apply_visitor(op, rdf::RdfAstType(lhs), rdf::RdfAstType(rhs));
  EXPECT_EQ(res, rdf::RdfAstType(rdf::LString(std::string("2025-07-02"))));
}

// The empty set yields null, so the rule asserts nothing.
TEST_F(ExprOpSpecialtyTest, JoinValuesVisitorEmptySet) {
  auto rmgr = this->rdf_session->rmgr();
  rmgr->create_resource(std::string("sJ0"));
  rmgr->create_resource(std::string("pJ0"));

  JoinValuesVisitor op(this->rete_session.get(), nullptr);
  rdf::NamedResource lhs("sJ0");
  rdf::NamedResource rhs("pJ0");
  auto res = boost::apply_visitor(op, rdf::RdfAstType(lhs), rdf::RdfAstType(rhs));
  EXPECT_EQ(res, rdf::RdfAstType(rdf::RDFNull()));
}

// An object carrying no value property contributes nothing, and leaves no empty slot.
TEST_F(ExprOpSpecialtyTest, JoinValuesVisitorSkipsAbsentValues) {
  auto rmgr = this->rdf_session->rmgr();
  auto const* jets = rmgr->jets();
  rdf::r_index config = rmgr->create_resource("configJ4");
  rdf::r_index has = rmgr->create_resource("hasThing");
  rdf::r_index val = rmgr->create_resource("thingValue");
  this->rdf_session->insert(config, jets->jets__entity_property, has);
  this->rdf_session->insert(config, jets->jets__value_property, val);

  rdf::r_index owner = rmgr->create_resource("ownerJ4");
  auto t1 = rmgr->create_resource("thing1");
  auto t2 = rmgr->create_resource("thing2");
  this->rdf_session->insert(owner, has, t1);
  this->rdf_session->insert(owner, has, t2);
  this->rdf_session->insert(t1, val, std::string("alpha"));
  // t2 carries no value property

  JoinValuesVisitor op(this->rete_session.get(), nullptr);
  rdf::NamedResource lhs("ownerJ4");
  rdf::NamedResource rhs("configJ4");
  auto res = boost::apply_visitor(op, rdf::RdfAstType(lhs), rdf::RdfAstType(rhs));
  EXPECT_EQ(res, rdf::RdfAstType(rdf::LString(std::string("alpha"))));
}

// Duplicates are kept: two fills on the same day are two fills.
TEST_F(ExprOpSpecialtyTest, JoinValuesVisitorKeepsDuplicates) {
  auto rmgr = this->rdf_session->rmgr();
  auto const* jets = rmgr->jets();
  rdf::r_index config = rmgr->create_resource("configJ5");
  rdf::r_index has = rmgr->create_resource("hasThing5");
  rdf::r_index val = rmgr->create_resource("thingValue5");
  this->rdf_session->insert(config, jets->jets__entity_property, has);
  this->rdf_session->insert(config, jets->jets__value_property, val);

  rdf::r_index owner = rmgr->create_resource("ownerJ5");
  auto t1 = rmgr->create_resource("thing51");
  auto t2 = rmgr->create_resource("thing52");
  this->rdf_session->insert(owner, has, t1);
  this->rdf_session->insert(owner, has, t2);
  this->rdf_session->insert(t1, val, std::string("2025-06-15"));
  this->rdf_session->insert(t2, val, std::string("2025-06-15"));

  JoinValuesVisitor op(this->rete_session.get(), nullptr);
  rdf::NamedResource lhs("ownerJ5");
  rdf::NamedResource rhs("configJ5");
  auto res = boost::apply_visitor(op, rdf::RdfAstType(lhs), rdf::RdfAstType(rhs));
  EXPECT_EQ(res, rdf::RdfAstType(rdf::LString(std::string("2025-06-15, 2025-06-15"))));
}

// jets:entity_property with no jets:value_property is a misconfiguration and is
// reported rather than silently yielding nothing.
TEST_F(ExprOpSpecialtyTest, JoinValuesVisitorMisconfigured) {
  auto rmgr = this->rdf_session->rmgr();
  auto const* jets = rmgr->jets();
  rdf::r_index config = rmgr->create_resource("configJ6");
  rdf::r_index has = rmgr->create_resource("hasThing6");
  this->rdf_session->insert(config, jets->jets__entity_property, has);

  JoinValuesVisitor op(this->rete_session.get(), nullptr);
  rdf::NamedResource lhs("main1");
  rdf::NamedResource rhs("configJ6");
  EXPECT_THROW(
    boost::apply_visitor(op, rdf::RdfAstType(lhs), rdf::RdfAstType(rhs)),
    jets::rete_exception);
}

// A non text jets:separator is reported rather than silently ignored.
TEST_F(ExprOpSpecialtyTest, JoinValuesVisitorBadSeparator) {
  auto rmgr = this->rdf_session->rmgr();
  auto const* jets = rmgr->jets();
  rdf::r_index config = rmgr->create_resource("configJ7");
  rdf::r_index values = rmgr->create_resource("values");
  this->rdf_session->insert(config, jets->jets__value_property, values);
  this->rdf_session->insert(config, jets->jets__separator, 3);

  JoinValuesVisitor op(this->rete_session.get(), nullptr);
  rdf::NamedResource lhs("main1");
  rdf::NamedResource rhs("configJ7");
  EXPECT_THROW(
    boost::apply_visitor(op, rdf::RdfAstType(lhs), rdf::RdfAstType(rhs)),
    jets::rete_exception);
}

}   // namespace
}   // namespace jets::rdf