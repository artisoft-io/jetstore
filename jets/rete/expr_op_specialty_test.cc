#include <cstddef>
#include <iostream>
#include <string>
#include <memory>

#include <gtest/gtest.h>

#include "../rdf/rdf_types.h"
#include "../rete/rete_types.h"
#include "../rete/rete_types_impl.h"
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

// TRUTH MAINTENANCE FOR THE AGGREGATE OPERATORS
// -----------------------------------------------------------------------------------
// Everything above calls a visitor directly, so none of it says anything about whether
// the engine RE-computes an aggregate when the graph changes underneath it. That is a
// separate mechanism: ExprBase::register_callback, reached from
// ReteSession::set_graph_callbacks (jets/rete/rete_session.cc), which registers a graph
// callback on ONE property and replays the rule term when a triple carrying that
// property is inserted or deleted.
//
// The cases below need a real, if small, rete network, because that mechanism cannot be
// reached by calling a visitor. Two properties of the fixture are load bearing:
//
//  - THE AGGREGATE IS IN A FILTER, not in a consequent. set_graph_callbacks registers
//    callbacks for a node vertex's filter expression only, so a consequent aggregate
//    never registers one. AggregateInConsequentIsNotMaintained at the end pins that,
//    because it is a much bigger claim than these two defects and it is easy to read
//    the cases below as covering it.
//
//  - THE GRAPH IS CHANGED BY A RULE, not by the test. Consequent triples are computed
//    in ReteSession::compute_consequent_triples, which drains a queue: a triple it
//    infers fires the callbacks, the callbacks replay their vertex and push more rows
//    onto that same queue, and the loop keeps going. A triple inserted by the test
//    after execute_rules has returned replays the vertex but has no drain left to run,
//    so it would show nothing either way. The `pending*` rules below exist for that
//    reason and for no other.
class AggregateTruthMaintenanceTest : public ::testing::Test {
 protected:
  ~AggregateTruthMaintenanceTest() {
    this->rete_session->terminate();
  }

  AggregateTruthMaintenanceTest()
    : rete_session(), rete_meta_store(), rdf_session()
  {
    auto meta_graph = rdf::create_rdf_graph();
    auto * rmgr = meta_graph->rmgr();
    rmgr->initialize();
    auto const* jets = rmgr->jets();

    auto me       = rmgr->create_resource("MainEntity");
    auto has      = rmgr->create_resource("hasSupport");
    auto value    = rmgr->create_resource("value");
    auto values   = rmgr->create_resource("values");
    auto p_has    = rmgr->create_resource("pendingSupport");
    auto p_value  = rmgr->create_resource("pendingValue");
    auto p_values = rmgr->create_resource("pendingValues");
    auto sum_flag = rmgr->create_resource("sumFlag");
    auto max_flag = rmgr->create_resource("maxFlag");
    auto min_flag = rmgr->create_resource("minFlag");
    auto dir_flag = rmgr->create_resource("directFlag");
    auto join_flag = rmgr->create_resource("joinFlag");
    auto head_flag = rmgr->create_resource("headFlag");
    auto total    = rmgr->create_resource("total");
    auto one      = rmgr->create_literal<int>(1);

    // The operator configs go in the META graph, as they do in a rule file, where
    // triple(config, jets:entity_property, ...) compiles into the meta store. That is
    // not cosmetic here: callbacks are registered during rete_session->initialize(),
    // before anything has been asserted, so a config living in the asserted graph would
    // not be resolvable at the moment registration reads it.
    auto sum_config = rmgr->create_resource("sumConfig");
    meta_graph->insert(sum_config, jets->jets__entity_property, has);
    meta_graph->insert(sum_config, jets->jets__value_property, value);

    auto mm_config = rmgr->create_resource("mmConfig");
    meta_graph->insert(mm_config, jets->jets__entity_property, has);
    meta_graph->insert(mm_config, jets->jets__value_property, value);

    auto sh_config = rmgr->create_resource("shConfig");
    meta_graph->insert(sh_config, jets->jets__entity_property, has);
    meta_graph->insert(sh_config, jets->jets__value_property, value);
    meta_graph->insert(sh_config, jets->jets__operator, std::string(">"));

    // The aggregate rule terms bind ?s alone; the pending rules bind ?s and ?o.
    auto ri1 = []() {
      auto ri = create_row_initializer(1);
      ri->put(0, 0 | brc_triple, "?s");
      return ri;
    };
    auto ri2 = []() {
      auto ri = create_row_initializer(2);
      ri->put(0, 0 | brc_triple, "?s");
      ri->put(1, 2 | brc_triple, "?o");
      return ri;
    };

    // vertex 1: [(?s sum_values sumConfig) > 6]  -- entity_property + value_property
    auto sum_filter = create_expr_binary_operator<GtVisitor>(0,
      create_expr_binary_operator<SumValuesVisitor>(0,
        create_expr_binded_var(0),
        create_expr_cst(rdf::RdfAstType(rdf::NamedResource("sumConfig")))),
      create_expr_cst(rdf::RdfAstType(rdf::LInt32(6))));

    // vertex 2: [(?s max_of mmConfig) > 3]
    auto max_filter = create_expr_binary_operator<GtVisitor>(0,
      create_expr_binary_operator<MaxOfVisitor>(0,
        create_expr_binded_var(0),
        create_expr_cst(rdf::RdfAstType(rdf::NamedResource("mmConfig")))),
      create_expr_cst(rdf::RdfAstType(rdf::LInt32(3))));

    // vertex 3: [(?s sum_values values) > 60]  -- the direct multi valued property form
    auto dir_filter = create_expr_binary_operator<GtVisitor>(0,
      create_expr_binary_operator<SumValuesVisitor>(0,
        create_expr_binded_var(0),
        create_expr_cst(rdf::RdfAstType(rdf::NamedResource("values")))),
      create_expr_cst(rdf::RdfAstType(rdf::LInt32(60))));

    // vertex 5: [(?s min_of mmConfig) < 1]
    auto min_filter = create_expr_binary_operator<LtVisitor>(0,
      create_expr_binary_operator<MinOfVisitor>(0,
        create_expr_binded_var(0),
        create_expr_cst(rdf::RdfAstType(rdf::NamedResource("mmConfig")))),
      create_expr_cst(rdf::RdfAstType(rdf::LInt32(1))));

    // vertex 9: [(?s join_values mmConfig) == "1, 2, 3, 4"]
    auto join_filter = create_expr_binary_operator<EqVisitor>(0,
      create_expr_binary_operator<JoinValuesVisitor>(0,
        create_expr_binded_var(0),
        create_expr_cst(rdf::RdfAstType(rdf::NamedResource("mmConfig")))),
      create_expr_cst(rdf::RdfAstType(rdf::LString(std::string("1, 2, 3, 4")))));

    // vertex 10: [(?s sorted_head shConfig) == support4]
    auto head_filter = create_expr_binary_operator<EqVisitor>(0,
      create_expr_binary_operator<SortedHeadVisitor>(0,
        create_expr_binded_var(0),
        create_expr_cst(rdf::RdfAstType(rdf::NamedResource("shConfig")))),
      create_expr_cst(rdf::RdfAstType(rdf::NamedResource("support4"))));

    NodeVertexVector node_vertexes;
    node_vertexes.push_back(create_node_vertex(nullptr, 0, 0, false, 100, {}, "", {}));
    node_vertexes.push_back(create_node_vertex(node_vertexes[0].get(), 0, 1, false, 10, sum_filter, "", ri1()));
    node_vertexes.push_back(create_node_vertex(node_vertexes[0].get(), 0, 2, false, 10, max_filter, "", ri1()));
    node_vertexes.push_back(create_node_vertex(node_vertexes[0].get(), 0, 3, false, 10, dir_filter, "", ri1()));
    // vertex 4 carries no filter: the aggregate is in its consequent instead. Its high
    // salience makes its consequent the first one computed, so that it reads the graph
    // as it stood before the pending rules changed it -- which is what makes the case
    // at the end a measurement rather than a race.
    node_vertexes.push_back(create_node_vertex(node_vertexes[0].get(), 0, 4, false, 90, {}, "", ri1()));
    node_vertexes.push_back(create_node_vertex(node_vertexes[0].get(), 0, 5, false, 10, min_filter, "", ri1()));
    // vertexes 6 to 8: the rules that change the graph during inference.
    node_vertexes.push_back(create_node_vertex(node_vertexes[0].get(), 0, 6, false, 5, {}, "", ri2()));
    node_vertexes.push_back(create_node_vertex(node_vertexes[0].get(), 0, 7, false, 5, {}, "", ri2()));
    node_vertexes.push_back(create_node_vertex(node_vertexes[0].get(), 0, 8, false, 5, {}, "", ri2()));
    node_vertexes.push_back(create_node_vertex(node_vertexes[0].get(), 0, 9, false, 10, join_filter, "", ri1()));
    node_vertexes.push_back(create_node_vertex(node_vertexes[0].get(), 0, 10, false, 10, head_filter, "", ri1()));

    ReteMetaStore::AlphaNodeVector alpha_nodes;
    // Antecedent alpha nodes, one per node vertex and in the same order, which is what
    // ReteSession::set_graph_callbacks assumes when it indexes them by position.
    alpha_nodes.push_back(create_alpha_node<F_var, F_var, F_var>(node_vertexes[0].get(), 0, true, "",
      F_var("*"), F_var("*"), F_var("*") ));
    for(int v=1; v<=5; ++v) {
      alpha_nodes.push_back(create_alpha_node<F_var, F_cst, F_cst>(node_vertexes[v].get(), 0, true, "",
        F_var("?s"), F_cst(jets->rdf__type), F_cst(me) ));
    }
    alpha_nodes.push_back(create_alpha_node<F_var, F_cst, F_var>(node_vertexes[6].get(), 0, true, "",
      F_var("?s"), F_cst(p_has), F_var("?o") ));
    alpha_nodes.push_back(create_alpha_node<F_var, F_cst, F_var>(node_vertexes[7].get(), 0, true, "",
      F_var("?s"), F_cst(p_value), F_var("?o") ));
    alpha_nodes.push_back(create_alpha_node<F_var, F_cst, F_var>(node_vertexes[8].get(), 0, true, "",
      F_var("?s"), F_cst(p_values), F_var("?o") ));
    for(int v=9; v<=10; ++v) {
      alpha_nodes.push_back(create_alpha_node<F_var, F_cst, F_cst>(node_vertexes[v].get(), 0, true, "",
        F_var("?s"), F_cst(jets->rdf__type), F_cst(me) ));
    }

    // Consequent alpha nodes.
    alpha_nodes.push_back(create_alpha_node<F_binded, F_cst, F_cst>(node_vertexes[1].get(), 0, false, "",
      F_binded(0), F_cst(sum_flag), F_cst(one) ));
    alpha_nodes.push_back(create_alpha_node<F_binded, F_cst, F_cst>(node_vertexes[2].get(), 0, false, "",
      F_binded(0), F_cst(max_flag), F_cst(one) ));
    alpha_nodes.push_back(create_alpha_node<F_binded, F_cst, F_cst>(node_vertexes[3].get(), 0, false, "",
      F_binded(0), F_cst(dir_flag), F_cst(one) ));
    {
      // vertex 4: (?s total (?s sum_values sumConfig)) -- the shape CE_RxDateRange10 in
      // workspaces/jets_ws uses, kept here to pin what it does and does not do.
      auto total_expr = create_expr_binary_operator<SumValuesVisitor>(0,
        create_expr_binded_var(0),
        create_expr_cst(rdf::RdfAstType(rdf::NamedResource("sumConfig"))));
      alpha_nodes.push_back(create_alpha_node<F_binded, F_cst, F_expr>(node_vertexes[4].get(), 0, false, "",
        F_binded(0), F_cst(total), F_expr(total_expr) ));
    }
    alpha_nodes.push_back(create_alpha_node<F_binded, F_cst, F_cst>(node_vertexes[5].get(), 0, false, "",
      F_binded(0), F_cst(min_flag), F_cst(one) ));
    alpha_nodes.push_back(create_alpha_node<F_binded, F_cst, F_binded>(node_vertexes[6].get(), 0, false, "",
      F_binded(0), F_cst(has), F_binded(1) ));
    alpha_nodes.push_back(create_alpha_node<F_binded, F_cst, F_binded>(node_vertexes[7].get(), 0, false, "",
      F_binded(0), F_cst(value), F_binded(1) ));
    alpha_nodes.push_back(create_alpha_node<F_binded, F_cst, F_binded>(node_vertexes[8].get(), 0, false, "",
      F_binded(0), F_cst(values), F_binded(1) ));
    alpha_nodes.push_back(create_alpha_node<F_binded, F_cst, F_cst>(node_vertexes[9].get(), 0, false, "",
      F_binded(0), F_cst(join_flag), F_cst(one) ));
    alpha_nodes.push_back(create_alpha_node<F_binded, F_cst, F_cst>(node_vertexes[10].get(), 0, false, "",
      F_binded(0), F_cst(head_flag), F_cst(one) ));

    rete_meta_store = create_rete_meta_store({}, alpha_nodes, node_vertexes);
    rete_meta_store->initialize();
    rmgr->set_locked();
    meta_graph->set_locked();

    this->rdf_session = rdf::create_rdf_session(meta_graph);
    this->rete_session = create_rete_session(rete_meta_store, this->rdf_session.get());
    this->rete_session->initialize();

    // The instance data: main1 with three support entities valued 1, 2 and 3, and a
    // multi valued `values` property holding 10, 20 and 30.
    auto srmgr = this->rdf_session->rmgr();
    auto main1 = srmgr->create_resource("main1");
    this->rdf_session->insert(main1, jets->rdf__type, me);
    this->rdf_session->insert(main1, values, 10);
    this->rdf_session->insert(main1, values, 20);
    this->rdf_session->insert(main1, values, 30);
    for(int i=1; i<=3; ++i) {
      auto s = srmgr->create_resource(std::string("support")+std::to_string(i));
      this->rdf_session->insert(main1, has, s);
      this->rdf_session->insert(s, value, i);
    }
    // support0 is linked and carries NO value. Every aggregate here skips an object
    // with no value property, so it contributes nothing until one is given to it --
    // which is what the value-property control case below turns on.
    this->rdf_session->insert(main1, has, srmgr->create_resource("support0"));
  }

  // Stage a fourth support entity, valued `v`, to be LINKED BY A RULE during inference.
  //
  // The child is given its value before it is attached, which is the order the pharmacy
  // rules in workspaces/jets_ws produce -- a claim entity is materialised with its own
  // data and then attached to the event -- and it is the order under which watching the
  // value property alone is not enough, because the value triple lands while the child
  // is still unreachable from main1.
  void stage_support(char const* name, int v)
  {
    auto rmgr = this->rdf_session->rmgr();
    auto s = rmgr->create_resource(std::string(name));
    this->rdf_session->insert(s, rmgr->create_resource("value"), v);
    this->rdf_session->insert(
      rmgr->create_resource("main1"), rmgr->create_resource("pendingSupport"), s);
  }

  void stage(char const* subject, char const* property, int v)
  {
    auto rmgr = this->rdf_session->rmgr();
    this->rdf_session->insert(
      rmgr->create_resource(std::string(subject)),
      rmgr->create_resource(std::string(property)),
      v);
  }

  bool has_flag(char const* flag)
  {
    auto rmgr = this->rdf_session->rmgr();
    return this->rdf_session->contains(
      rmgr->create_resource("main1"),
      rmgr->create_resource(std::string(flag)),
      rmgr->create_literal<int>(1));
  }

  ReteSessionPtr   rete_session;
  ReteMetaStorePtr rete_meta_store;
  rdf::RDFSessionPtr rdf_session;
};

// DEFECT 1 -- sum_values, the jets:entity_property + jets:value_property form.
// Linking a further child entity must recompute the aggregate. Before the fix
// SumValuesVisitor::register_callback watched jets:value_property only, so the
// (main1, hasSupport, support4) triple went unnoticed and the sum stayed at 6.
TEST_F(AggregateTruthMaintenanceTest, SumValuesRecomputesWhenChildEntityIsLinked) {
  this->stage_support("support4", 4);

  this->rete_session->execute_rules();

  EXPECT_TRUE(this->has_flag("sumFlag"))
    << "sum was 1+2+3 = 6 when the term was first evaluated and is 10 once support4 is "
       "linked; the filter is > 6";
}

// DEFECT 1 -- max_of, same form, same cause: registerCallback4MinMaxOf.
TEST_F(AggregateTruthMaintenanceTest, MaxOfRecomputesWhenChildEntityIsLinked) {
  this->stage_support("support4", 9);

  this->rete_session->execute_rules();

  EXPECT_TRUE(this->has_flag("maxFlag")) << "max was 3 and is 9 once support4 is linked";
}

// DEFECT 1 -- min_of.
TEST_F(AggregateTruthMaintenanceTest, MinOfRecomputesWhenChildEntityIsLinked) {
  this->stage_support("support4", 0);

  this->rete_session->execute_rules();

  EXPECT_TRUE(this->has_flag("minFlag")) << "min was 1 and is 0 once support4 is linked";
}

// The control case, and it is here because it PASSED before the fix: the callback on
// jets:value_property was the one half of the mechanism that did work. If a later
// change breaks this one, the fix has traded one half for the other.
TEST_F(AggregateTruthMaintenanceTest, SumValuesRecomputesWhenALinkedChildGetsItsValue) {
  this->stage("support0", "pendingValue", 40);

  this->rete_session->execute_rules();

  EXPECT_TRUE(this->has_flag("sumFlag"))
    << "support0 was already linked and valueless; once the rule gives it 40 the sum is 46";
}

// DEFECT 2 -- (?s sum_values someMultiValuedProperty), the direct property form.
// SumValuesVisitor::register_callback returned 0 when the rhs carried no
// jets:value_property, so this form had no truth maintenance at all in C++.
// registerCallback4MinMaxOf already fell back to the rhs, and so does the Go engine.
TEST_F(AggregateTruthMaintenanceTest, SumValuesRecomputesOnDirectMultiValuedProperty) {
  this->stage("main1", "pendingValues", 40);

  this->rete_session->execute_rules();

  EXPECT_TRUE(this->has_flag("directFlag"))
    << "sum was 10+20+30 = 60 and is 100 once the rule fires; the filter is > 60";
}

// DEFECT 1 -- join_values and sorted_head carried the same bug and are fixed by the same
// helper, so they get the same case. join_values is the newest of the five operators and
// was written against sum_values as the model, which is how the defect propagated.
TEST_F(AggregateTruthMaintenanceTest, JoinValuesRecomputesWhenChildEntityIsLinked) {
  this->stage_support("support4", 4);

  this->rete_session->execute_rules();

  EXPECT_TRUE(this->has_flag("joinFlag"))
    << R"(the join was "1, 2, 3" and is "1, 2, 3, 4" once support4 is linked)";
}

TEST_F(AggregateTruthMaintenanceTest, SortedHeadRecomputesWhenChildEntityIsLinked) {
  this->stage_support("support4", 4);

  this->rete_session->execute_rules();

  EXPECT_TRUE(this->has_flag("headFlag"))
    << "the head was support3 and is support4 once support4 is linked";
}

// NOT A DEFECT THESE TWO FIXES ADDRESS, AND THE REASON IT IS PINNED HERE.
//
// An aggregate in a CONSEQUENT gets no truth maintenance in either engine, because
// ReteSession::set_graph_callbacks registers callbacks for a node vertex's filter
// expression and for its antecedent alpha node, and for nothing else -- the Go engine
// does the same (ReteSession.initialize, jets/jetrules/rete/rete_session.go). So
// register_callback is never called for the consequent below, and the two fixes above
// cannot reach it.
//
// This is worth an assertion rather than a comment because the shape it describes is
// the one the rules in workspaces/jets_ws use: CE_RxDateRange10 puts min_of, max_of and
// sum_values in consequents. A reader who fixes register_callback and expects that rule
// to start aggregating every claim will be wrong, and this case says so out loud.
//
// If this test ever starts failing, that is not a regression: it means consequent
// expressions have been given truth maintenance, and this case should become the
// assertion that they now recompute.
TEST_F(AggregateTruthMaintenanceTest, AggregateInConsequentIsNotMaintained) {
  this->stage_support("support4", 4);

  this->rete_session->execute_rules();

  auto rmgr = this->rdf_session->rmgr();
  auto main1 = rmgr->create_resource("main1");
  auto total = rmgr->create_resource("total");
  EXPECT_TRUE(this->rdf_session->contains(main1, total, rmgr->create_literal<int>(6)))
    << "the consequent was computed over the three supports linked at the time";
  EXPECT_FALSE(this->rdf_session->contains(main1, total, rmgr->create_literal<int>(10)))
    << "and was never recomputed once support4 was linked";
}

}   // namespace
}   // namespace jets::rdf
