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

namespace jets::rete {
namespace {
// Arithmetic operators test
// The suite fixture for expr operator tests
class ExprOpTest : public ::testing::Test {
 protected:
  ExprOpTest() 
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

    // Cretae the rdf_session and the rete_session and initialize them
    // Initialize the rete_session now that the rule base is ready
    this->rdf_session = rdf::create_rdf_session(meta_graph);
    this->rete_session = create_rete_session(rete_meta_store, this->rdf_session.get());
    this->rete_session->initialize();
  }

  ReteSessionPtr  rete_session;
  ReteMetaStorePtr rete_meta_store;
  rdf::RDFSessionPtr   rdf_session;
};

// Define the tests
// -----------------------------------------------------------------------------------
TEST_F(ExprOpTest, SizeOfVisitor1) {
  auto sess = this->rete_session->rdf_session();
  auto rmgr = sess->rmgr();
  rdf::r_index subject = rmgr->create_resource("subject");
  rdf::r_index predicate = rmgr->create_resource("predicate");
  sess->insert(subject, predicate, rmgr->create_literal("object1"));
  sess->insert(subject, predicate, rmgr->create_literal("object2"));
  sess->insert(subject, predicate, rmgr->create_literal("object3"));
  SizeOfVisitor op(this->rete_session.get(), nullptr);
  rdf::NamedResource lhs("subject");
  rdf::NamedResource rhs("predicate");
  auto res = boost::apply_visitor(op, rdf::RdfAstType(lhs), rdf::RdfAstType(rhs));
  EXPECT_EQ(res, rdf::RdfAstType(rdf::LInt32(3)));
}
TEST_F(ExprOpTest, SizeOfVisitor2) {
  auto sess = this->rete_session->rdf_session();
  auto rmgr = sess->rmgr();
  rdf::r_index subject = rmgr->create_resource("subject");
  rdf::r_index predicate = rmgr->create_resource("predicate");
  SizeOfVisitor op(this->rete_session.get(), nullptr);
  rdf::NamedResource lhs("subject");
  rdf::NamedResource rhs("predicate");
  auto res = boost::apply_visitor(op, rdf::RdfAstType(lhs), rdf::RdfAstType(rhs));
  EXPECT_EQ(res, rdf::RdfAstType(rdf::LInt32(0)));
}

TEST_F(ExprOpTest, CreateEntityVisitor1) {
  auto sess = this->rete_session->rdf_session();
  auto rmgr = sess->rmgr();
  CreateEntityVisitor op(this->rete_session.get(), nullptr);
  rdf::LString lhs("instance_1");
  auto res = boost::apply_visitor(op, rdf::RdfAstType(lhs));
  EXPECT_EQ(res, rdf::RdfAstType(rdf::NamedResource("instance_1")));
  auto obj = sess->get_object(
    rmgr->create_resource("instance_1"),
    rmgr->jets()->jets__key
  );
  EXPECT_EQ(*obj, rdf::RdfAstType(rdf::LString("instance_1")));
}
TEST_F(ExprOpTest, ExistNotVisitor1) {
  auto sess = this->rete_session->rdf_session();
  auto rmgr = sess->rmgr();
  rdf::r_index subject = rmgr->create_resource("subject");
  rdf::r_index predicate = rmgr->create_resource("predicate");
  ExistNotVisitor op(this->rete_session.get(), nullptr);
  rdf::NamedResource lhs("subject");
  rdf::NamedResource rhs("predicate");
  auto res = boost::apply_visitor(op, rdf::RdfAstType(lhs), rdf::RdfAstType(rhs));
  EXPECT_EQ(res, rdf::RdfAstType(rdf::LInt32(1)));
}
TEST_F(ExprOpTest, ExistVisitor1) {
  auto sess = this->rete_session->rdf_session();
  auto rmgr = sess->rmgr();
  rdf::r_index subject = rmgr->create_resource("subject");
  rdf::r_index predicate = rmgr->create_resource("predicate");
  sess->insert(
    subject, 
    predicate,
    rmgr->create_literal("object")
  );
  ExistVisitor op(this->rete_session.get(), nullptr);
  rdf::NamedResource lhs("subject");
  rdf::NamedResource rhs("predicate");
  auto res = boost::apply_visitor(op, rdf::RdfAstType(lhs), rdf::RdfAstType(rhs));
  EXPECT_EQ(res, rdf::RdfAstType(rdf::LInt32(1)));
}
TEST_F(ExprOpTest, RegexVisitor1) {
  RegexVisitor op(this->rete_session.get(), nullptr);
  rdf::LString lhs("12345");
  rdf::LString rhs("(\\d*)");
  auto res = boost::apply_visitor(op, rdf::RdfAstType(lhs), rdf::RdfAstType(rhs));
  EXPECT_EQ(res, rdf::RdfAstType(rdf::LString("12345")));
}
TEST_F(ExprOpTest, ApplyFormatVisitor1) {
  ApplyFormatVisitor op(this->rete_session.get(), nullptr);
  rdf::LInt32 lhs(5);
  rdf::LString rhs("The answer is: %d");
  auto res = boost::apply_visitor(op, rdf::RdfAstType(lhs), rdf::RdfAstType(rhs));
  EXPECT_EQ(res, rdf::RdfAstType(rdf::LString("The answer is: 5")));
}
TEST_F(ExprOpTest, StartsWithVisitor1) {
  StartsWithVisitor op(this->rete_session.get(), nullptr);
  rdf::LString lhs("Hello");
  rdf::LString rhs("Hel");
  auto res = boost::apply_visitor(op, rdf::RdfAstType(lhs), rdf::RdfAstType(rhs));
  EXPECT_EQ(res, rdf::RdfAstType(rdf::LInt32(1)));
}
TEST_F(ExprOpTest, ContainsVisitor1) {
  ContainsVisitor op(this->rete_session.get(), nullptr);
  rdf::LString lhs("Hello");
  rdf::LString rhs("ll");
  auto res = boost::apply_visitor(op, rdf::RdfAstType(lhs), rdf::RdfAstType(rhs));
  EXPECT_EQ(res, rdf::RdfAstType(rdf::LInt32(1)));
}
TEST_F(ExprOpTest, GeVisitor1) {
  GeVisitor op(this->rete_session.get(), nullptr);
  {
    rdf::LString lhs("Hello");
    rdf::LString rhs("Hello");
    auto res = boost::apply_visitor(op, rdf::RdfAstType(lhs), rdf::RdfAstType(rhs));
    EXPECT_EQ(res, rdf::RdfAstType(rdf::LInt32(1)));
  }
  {
    rdf::LDouble lhs(22.1);
    rdf::LDouble rhs(22.0);
    auto res = boost::apply_visitor(op, rdf::RdfAstType(lhs), rdf::RdfAstType(rhs));
    EXPECT_EQ(res, rdf::RdfAstType(rdf::LInt32(1)));
  }
  {
    rdf::LDouble lhs(22.0);
    rdf::LDouble rhs(22.1);
    auto res = boost::apply_visitor(op, rdf::RdfAstType(lhs), rdf::RdfAstType(rhs));
    EXPECT_EQ(res, rdf::RdfAstType(rdf::LInt32(0)));
  }
  {
    rdf::LDouble lhs(22.0);
    rdf::LDouble rhs(22.0);
    auto res = boost::apply_visitor(op, rdf::RdfAstType(lhs), rdf::RdfAstType(rhs));
    EXPECT_EQ(res, rdf::RdfAstType(rdf::LInt32(1)));
  }
  {
    rdf::LDouble lhs(22.0);
    rdf::LInt32 rhs(21);
    auto res = boost::apply_visitor(op, rdf::RdfAstType(lhs), rdf::RdfAstType(rhs));
    EXPECT_EQ(res, rdf::RdfAstType(rdf::LInt32(1)));
  }
  {
    rdf::LDouble lhs(21.0);
    rdf::LInt32 rhs(22);
    auto res = boost::apply_visitor(op, rdf::RdfAstType(lhs), rdf::RdfAstType(rhs));
    EXPECT_EQ(res, rdf::RdfAstType(rdf::LInt32(0)));
  }
  {
    rdf::LDouble lhs(22.0);
    rdf::LInt32 rhs(22);
    auto res = boost::apply_visitor(op, rdf::RdfAstType(lhs), rdf::RdfAstType(rhs));
    EXPECT_EQ(res, rdf::RdfAstType(rdf::LInt32(1)));
  }
}
TEST_F(ExprOpTest, GtVisitor1) {
  GtVisitor op(this->rete_session.get(), nullptr);
  rdf::LString lhs("Hello1");
  rdf::LString rhs("Hello");
  auto res = boost::apply_visitor(op, rdf::RdfAstType(lhs), rdf::RdfAstType(rhs));
  EXPECT_EQ(res, rdf::RdfAstType(rdf::LInt32(1)));
}
TEST_F(ExprOpTest, LeVisitor1) {
  LeVisitor op(this->rete_session.get(), nullptr);
  {
    rdf::LString lhs("Hello");
    rdf::LString rhs("Hello");
    auto res = boost::apply_visitor(op, rdf::RdfAstType(lhs), rdf::RdfAstType(rhs));
    EXPECT_EQ(res, rdf::RdfAstType(rdf::LInt32(1)));
  }
  {
    rdf::LDouble lhs(22.1);
    rdf::LDouble rhs(22.0);
    auto res = boost::apply_visitor(op, rdf::RdfAstType(lhs), rdf::RdfAstType(rhs));
    EXPECT_EQ(res, rdf::RdfAstType(rdf::LInt32(0)));
  }
  {
    rdf::LDouble lhs(22.0);
    rdf::LDouble rhs(22.1);
    auto res = boost::apply_visitor(op, rdf::RdfAstType(lhs), rdf::RdfAstType(rhs));
    EXPECT_EQ(res, rdf::RdfAstType(rdf::LInt32(1)));
  }
  {
    rdf::LDouble lhs(22.0);
    rdf::LDouble rhs(22.0);
    auto res = boost::apply_visitor(op, rdf::RdfAstType(lhs), rdf::RdfAstType(rhs));
    EXPECT_EQ(res, rdf::RdfAstType(rdf::LInt32(1)));
  }
  {
    rdf::LDouble lhs(22.0);
    rdf::LInt32 rhs(21);
    auto res = boost::apply_visitor(op, rdf::RdfAstType(lhs), rdf::RdfAstType(rhs));
    EXPECT_EQ(res, rdf::RdfAstType(rdf::LInt32(0)));
  }
  {
    rdf::LDouble lhs(21.0);
    rdf::LInt32 rhs(22);
    auto res = boost::apply_visitor(op, rdf::RdfAstType(lhs), rdf::RdfAstType(rhs));
    EXPECT_EQ(res, rdf::RdfAstType(rdf::LInt32(1)));
  }
  {
    rdf::LDouble lhs(22.0);
    rdf::LInt32 rhs(22);
    auto res = boost::apply_visitor(op, rdf::RdfAstType(lhs), rdf::RdfAstType(rhs));
    EXPECT_EQ(res, rdf::RdfAstType(rdf::LInt32(1)));
  }
}
TEST_F(ExprOpTest, LtVisitor1) {
  LtVisitor op(this->rete_session.get(), nullptr);
  rdf::LString lhs("Hell");
  rdf::LString rhs("Hello");
  auto res = boost::apply_visitor(op, rdf::RdfAstType(lhs), rdf::RdfAstType(rhs));
  EXPECT_EQ(res, rdf::RdfAstType(rdf::LInt32(1)));
}
TEST_F(ExprOpTest, NeVisitor4) {
  NeVisitor op(this->rete_session.get(), nullptr);
  rdf::LString lhs("Hello1");
  rdf::LString rhs("Hello2");
  auto res = boost::apply_visitor(op, rdf::RdfAstType(lhs), rdf::RdfAstType(rhs));
  EXPECT_EQ(res, rdf::RdfAstType(rdf::LInt32(1)));
}
TEST_F(ExprOpTest, NeVisitor1) {
  NeVisitor op(this->rete_session.get(), nullptr);
  rdf::NamedResource lhs("Hello1");
  rdf::NamedResource rhs("Hello2");
  auto res = boost::apply_visitor(op, rdf::RdfAstType(lhs), rdf::RdfAstType(rhs));
  EXPECT_EQ(res, rdf::RdfAstType(rdf::LInt32(1)));
}
TEST_F(ExprOpTest, NeVisitor2) {
  NeVisitor op(this->rete_session.get(), nullptr);
  rdf::NamedResource lhs("Hello1");
  rdf::NamedResource rhs("Hello2");
  auto res = boost::apply_visitor(op, rdf::RdfAstType(lhs), rdf::RdfAstType(rhs));
  EXPECT_EQ(res, rdf::RdfAstType(rdf::LInt32(1)));
}
TEST_F(ExprOpTest, NeVisitor3) {
  NeVisitor op(this->rete_session.get(), nullptr);
  rdf::NamedResource lhs("Hello1");
  rdf::NamedResource rhs("Hello1");
  auto res = boost::apply_visitor(op, rdf::RdfAstType(lhs), rdf::RdfAstType(rhs));
  EXPECT_EQ(res, rdf::RdfAstType(rdf::LInt32(0)));
}
TEST_F(ExprOpTest, EqVisitor1) {
  EqVisitor op(this->rete_session.get(), nullptr);
  rdf::LInt32 lhs(5);
  rdf::LDouble rhs(5);
  auto res = boost::apply_visitor(op, rdf::RdfAstType(lhs), rdf::RdfAstType(rhs));
  EXPECT_EQ(res, rdf::RdfAstType(rdf::LInt32(1)));
}
TEST_F(ExprOpTest, AndVisitor1) {
  AndVisitor op(this->rete_session.get(), nullptr);
  rdf::LInt32 lhs(1);
  rdf::LInt32 rhs(1);
  auto res = boost::apply_visitor(op, rdf::RdfAstType(lhs), rdf::RdfAstType(rhs));
  EXPECT_EQ(res, rdf::RdfAstType(rdf::LInt32(1)));
}
TEST_F(ExprOpTest, AndVisitor2) {
  AndVisitor op(this->rete_session.get(), nullptr);
  rdf::LInt32 lhs(1);
  rdf::LInt32 rhs(0);
  auto res = boost::apply_visitor(op, rdf::RdfAstType(lhs), rdf::RdfAstType(rhs));
  EXPECT_EQ(res, rdf::RdfAstType(rdf::LInt32(0)));
}
TEST_F(ExprOpTest, OrVisitor1) {
  OrVisitor op(this->rete_session.get(), nullptr);
  rdf::LInt32 lhs(1);
  rdf::LInt32 rhs(1);
  auto res = boost::apply_visitor(op, rdf::RdfAstType(lhs), rdf::RdfAstType(rhs));
  EXPECT_EQ(res, rdf::RdfAstType(rdf::LInt32(1)));
}
TEST_F(ExprOpTest, OrVisitor2) {
  OrVisitor op(this->rete_session.get(), nullptr);
  rdf::LInt32 lhs(1);
  rdf::LInt32 rhs(0);
  auto res = boost::apply_visitor(op, rdf::RdfAstType(lhs), rdf::RdfAstType(rhs));
  EXPECT_EQ(res, rdf::RdfAstType(rdf::LInt32(1)));
}
TEST_F(ExprOpTest, MultVisitor1) {
  MultVisitor op(this->rete_session.get(), nullptr);
  rdf::LInt32 lhs(4);
  rdf::LInt32 rhs(2);
  auto res = boost::apply_visitor(op, rdf::RdfAstType(lhs), rdf::RdfAstType(rhs));
  EXPECT_EQ(res, rdf::RdfAstType(rdf::LInt32(8)));
}
TEST_F(ExprOpTest, MultVisitor2) {
  MultVisitor op(this->rete_session.get(), nullptr);
  rdf::LInt32 lhs(4);
  rdf::LDouble rhs(2);
  auto res = boost::apply_visitor(op, rdf::RdfAstType(lhs), rdf::RdfAstType(rhs));
  EXPECT_EQ(res, rdf::RdfAstType(rdf::LDouble(8)));
}
TEST_F(ExprOpTest, DivVisitor1) {
  DivVisitor op(this->rete_session.get(), nullptr);
  rdf::LInt32 lhs(4);
  rdf::LInt32 rhs(2);
  auto res = boost::apply_visitor(op, rdf::RdfAstType(lhs), rdf::RdfAstType(rhs));
  EXPECT_EQ(res, rdf::RdfAstType(rdf::LInt32(2)));
}
TEST_F(ExprOpTest, DivVisitor2) {
  DivVisitor op(this->rete_session.get(), nullptr);
  rdf::LInt32 lhs(4);
  rdf::LDouble rhs(2);
  auto res = boost::apply_visitor(op, rdf::RdfAstType(lhs), rdf::RdfAstType(rhs));
  EXPECT_EQ(res, rdf::RdfAstType(rdf::LDouble(2)));
}

TEST_F(ExprOpTest, SubsVisitor1) {
  SubsVisitor op(this->rete_session.get(), nullptr);
  rdf::LInt32 lhs(1);
  rdf::LInt32 rhs(-2);
  auto res = boost::apply_visitor(op, rdf::RdfAstType(lhs), rdf::RdfAstType(rhs));
  EXPECT_EQ(res, rdf::RdfAstType(rdf::LInt32(3)));
}
TEST_F(ExprOpTest, SubsVisitor2) {
  SubsVisitor op(this->rete_session.get(), nullptr);
  rdf::LInt32 lhs(1);
  rdf::LDouble rhs(1);
  auto res = boost::apply_visitor(op, rdf::RdfAstType(lhs), rdf::RdfAstType(rhs));
  EXPECT_EQ(res, rdf::RdfAstType(rdf::LInt32(0)));
}
TEST_F(ExprOpTest, SubsVisitor3) {
  SubsVisitor op(this->rete_session.get(), nullptr);
  rdf::LInt32 lhs(1);
  rdf::LDate  rhs(rdf::date(2020,3,20));
  auto res = boost::apply_visitor(op, rdf::RdfAstType(lhs), rdf::RdfAstType(rhs));
  EXPECT_EQ(res, rdf::RdfAstType(rdf::Null()));
}
TEST_F(ExprOpTest, SubsVisitor4) {
  SubsVisitor op(this->rete_session.get(), nullptr);
  rdf::LDate  lhs(rdf::date(2020,3,20));
  rdf::LInt32 rhs(1);
  auto res = boost::apply_visitor(op, rdf::RdfAstType(lhs), rdf::RdfAstType(rhs));
  EXPECT_EQ(res, rdf::RdfAstType(rdf::LDate(rdf::date(2020, 3, 19))));
}
TEST_F(ExprOpTest, SubsVisitor5) {
  SubsVisitor op(this->rete_session.get(), nullptr);
  rdf::LDate  lhs(rdf::date(2020,3,20));
  rdf::LInt32 rhs(-1);
  auto res = boost::apply_visitor(op, rdf::RdfAstType(lhs), rdf::RdfAstType(rhs));
  EXPECT_EQ(res, rdf::RdfAstType(rdf::LDate(rdf::date(2020, 3, 21))));
}
TEST_F(ExprOpTest, SubsVisitor6) {
  SubsVisitor op(this->rete_session.get(), nullptr);
  rdf::LDate  lhs(rdf::date(2020,3,20));
  rdf::LDate  rhs(rdf::date(2020,3,19));
  auto res = boost::apply_visitor(op, rdf::RdfAstType(lhs), rdf::RdfAstType(rhs));
  EXPECT_EQ(res, rdf::RdfAstType(rdf::LInt32(1)));
}
TEST_F(ExprOpTest, SubsVisitor7) {
  SubsVisitor op(this->rete_session.get(), nullptr);
  rdf::LDate  lhs(rdf::date(2020,3,19));
  rdf::LDate  rhs(rdf::date(2020,3,20));
  auto res = boost::apply_visitor(op, rdf::RdfAstType(lhs), rdf::RdfAstType(rhs));
  EXPECT_EQ(res, rdf::RdfAstType(rdf::LInt32(-1)));
}

TEST_F(ExprOpTest, AddVisistorTest1) {
  AddVisitor op(this->rete_session.get(), nullptr);
  rdf::LInt32 lhs(1);
  rdf::LInt32 rhs(-2);
  auto res = boost::apply_visitor(op, rdf::RdfAstType(lhs), rdf::RdfAstType(rhs));
  EXPECT_EQ(res, rdf::RdfAstType(rdf::LInt32(-1)));
}
TEST_F(ExprOpTest, AddVisistorTest2) {
  AddVisitor op(this->rete_session.get(), nullptr);
  rdf::LInt32 lhs(1);
  rdf::LUInt32 rhs(2);
  auto res = boost::apply_visitor(op, rdf::RdfAstType(lhs), rdf::RdfAstType(rhs));
  EXPECT_EQ(res, rdf::RdfAstType(rdf::LInt32(3)));
}
TEST_F(ExprOpTest, AddVisistorTest3) {
  AddVisitor op(this->rete_session.get(), nullptr);
  rdf::LInt32 lhs(1);
  rdf::LInt64 rhs(2);
  auto res = boost::apply_visitor(op, rdf::RdfAstType(lhs), rdf::RdfAstType(rhs));
  EXPECT_EQ(res, rdf::RdfAstType(rdf::LInt32(3)));
}
TEST_F(ExprOpTest, AddVisistorTest4) {
  AddVisitor op(this->rete_session.get(), nullptr);
  rdf::LInt32 lhs(1);
  rdf::LUInt64 rhs(2);
  auto res = boost::apply_visitor(op, rdf::RdfAstType(lhs), rdf::RdfAstType(rhs));
  EXPECT_EQ(res, rdf::RdfAstType(rdf::LInt32(3)));
}
TEST_F(ExprOpTest, AddVisistorTest5) {
  AddVisitor op(this->rete_session.get(), nullptr);
  rdf::LInt32 lhs(1);
  rdf::LDouble rhs(2.0);
  auto res = boost::apply_visitor(op, rdf::RdfAstType(lhs), rdf::RdfAstType(rhs));
  EXPECT_EQ(res, rdf::RdfAstType(rdf::LInt32(3)));
}
TEST_F(ExprOpTest, AddVisistorTest6) {
  AddVisitor op(this->rete_session.get(), nullptr);
  rdf::LInt32 lhs(1);
  rdf::LDate rhs(rdf::date(2019, 3, 7));
  auto res = boost::apply_visitor(op, rdf::RdfAstType(lhs), rdf::RdfAstType(rhs));
  EXPECT_EQ(res, rdf::RdfAstType(rdf::LDate(rdf::date(2019, 3, 8))));
}
TEST_F(ExprOpTest, AddVisistorTest7) {
  AddVisitor op(this->rete_session.get(), nullptr);
  rdf::LInt32 lhs(1);
  rdf::LDate rhs(rdf::date(2019, 3, 7));
  auto res = boost::apply_visitor(op, rdf::RdfAstType(rhs), rdf::RdfAstType(lhs));
  EXPECT_EQ(res, rdf::RdfAstType(rdf::LDate(rdf::date(2019, 3, 8))));
}
TEST_F(ExprOpTest, AddVisistorTest8) {
  AddVisitor op(this->rete_session.get(), nullptr);
  rdf::LInt32 lhs(-1);
  rdf::LDate rhs(rdf::date(2019, 3, 7));
  auto res = boost::apply_visitor(op, rdf::RdfAstType(lhs), rdf::RdfAstType(rhs));
  EXPECT_EQ(res, rdf::RdfAstType(rdf::LDate(rdf::date(2019, 3, 6))));
}

// Tests for age_as_of and age_in_months_as_of
TEST_F(ExprOpTest, AgeVisistorTest1) {
  AgeAsOfVisitor op(this->rete_session.get(), nullptr);
  rdf::LDate lhs(rdf::date(2000, 7, 27));
  rdf::LDate rhs(rdf::date(2020, 8, 27));
  auto res = boost::apply_visitor(op, rdf::RdfAstType(lhs), rdf::RdfAstType(rhs));
  EXPECT_EQ(res, rdf::RdfAstType(rdf::LInt32(20)));
}
TEST_F(ExprOpTest, AgeVisistorTest2) {
  AgeAsOfVisitor op(this->rete_session.get(), nullptr);
  rdf::LDate lhs(rdf::date(2000, 7, 27));
  rdf::LDate rhs(rdf::date(2020, 6, 27));
  auto res = boost::apply_visitor(op, rdf::RdfAstType(lhs), rdf::RdfAstType(rhs));
  EXPECT_EQ(res, rdf::RdfAstType(rdf::LInt32(19)));
}
TEST_F(ExprOpTest, AgeVisistorTest3) {
  AgeInMonthsAsOfVisitor op(this->rete_session.get(), nullptr);
  rdf::LDate lhs(rdf::date(2000, 7, 27));
  rdf::LDate rhs(rdf::date(2020, 9, 27));
  auto res = boost::apply_visitor(op, rdf::RdfAstType(lhs), rdf::RdfAstType(rhs));
  EXPECT_EQ(res, rdf::RdfAstType(rdf::LInt32(20*12 + 2)));
}
TEST_F(ExprOpTest, AgeVisistorTest4) {
  AgeInMonthsAsOfVisitor op(this->rete_session.get(), nullptr);
  rdf::LDate lhs(rdf::date(2000, 7, 27));
  rdf::LDate rhs(rdf::date(2020, 4, 27));
  auto res = boost::apply_visitor(op, rdf::RdfAstType(lhs), rdf::RdfAstType(rhs));
  EXPECT_EQ(res, rdf::RdfAstType(rdf::LInt32(19*12 + 4)));
}
TEST_F(ExprOpTest, MonthPeriodOfTest1) {
  MonthPeriodVisitor op(this->rete_session.get(), nullptr);
  rdf::LDate lhs(rdf::date(2000, 7, 27));
  auto res = boost::apply_visitor(op, rdf::RdfAstType(lhs));
  EXPECT_EQ(res, rdf::RdfAstType(rdf::LInt32((2000-1970)*12 + 7)));
}

}   // namespace
}   // namespace jets::rdf