#include <cstddef>
#include <limits>
#include <iostream>
#include <string>
#include <memory>
#include <vector>

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

    // Create the rdf_session and the rete_session and initialize them
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
TEST_F(ExprOpTest, ApplyFormatVisitor2) {
  ApplyFormatVisitor op(this->rete_session.get(), nullptr);
  rdf::LDate  lhs(rdf::date(2020,3,19));
  rdf::LString rhs("The answer is: %04d%02d%02d");
  auto res = boost::apply_visitor(op, rdf::RdfAstType(lhs), rdf::RdfAstType(rhs));
  EXPECT_EQ(res, rdf::RdfAstType(rdf::LString("The answer is: 20200319")));
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
  EXPECT_EQ(res, rdf::RdfAstType(rdf::LDouble(0)));   // an LInt32 until 2026-10
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
  EXPECT_EQ(res, rdf::RdfAstType(rdf::LDouble(3)));   // an LInt32 until 2026-10
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
// String operator: substring_of
TEST_F(ExprOpTest, SubstringOfVisitor1) {
  auto sess = this->rete_session->rdf_session();
  auto rmgr = sess->rmgr();
  rdf::r_index config = rmgr->create_resource("config");
  sess->insert(config, rmgr->jets()->jets__from, rmgr->create_literal(2));
  sess->insert(config, rmgr->jets()->jets__length, rmgr->create_literal(4));
  SubstringOfVisitor op(this->rete_session.get(), nullptr);
  rdf::NamedResource lhs("config");
  rdf::LString rhs("==Allo===");
  auto res = boost::apply_visitor(op, rdf::RdfAstType(lhs), rdf::RdfAstType(rhs));
  EXPECT_EQ(res, rdf::RdfAstType(rdf::LString("Allo")));
}
TEST_F(ExprOpTest, SubstringOfVisitor3) {
  auto sess = this->rete_session->rdf_session();
  auto rmgr = sess->rmgr();
  rdf::r_index config = rmgr->create_resource("config");
  sess->insert(config, rmgr->jets()->jets__from, rmgr->create_literal(2));
  sess->insert(config, rmgr->jets()->jets__length, rmgr->create_literal(-3));
  SubstringOfVisitor op(this->rete_session.get(), nullptr);
  rdf::NamedResource lhs("config");
  rdf::LString rhs("==Allo===");
  auto res = boost::apply_visitor(op, rdf::RdfAstType(lhs), rdf::RdfAstType(rhs));
  EXPECT_EQ(res, rdf::RdfAstType(rdf::LString("Allo")));
}
TEST_F(ExprOpTest, SubstringOfVisitor2) {
  auto sess = this->rete_session->rdf_session();
  auto rmgr = sess->rmgr();
  rdf::r_index config = rmgr->create_resource("config");
  sess->insert(config, rmgr->jets()->jets__from, rmgr->create_literal(2));
  sess->insert(config, rmgr->jets()->jets__length, rmgr->create_literal(20));
  SubstringOfVisitor op(this->rete_session.get(), nullptr);
  rdf::NamedResource lhs("config");
  rdf::LString rhs("==Allo");
  auto res = boost::apply_visitor(op, rdf::RdfAstType(lhs), rdf::RdfAstType(rhs));
  EXPECT_EQ(res, rdf::RdfAstType(rdf::LString("Allo")));
}
TEST_F(ExprOpTest, CharAtVisitor1) {
  CharAtVisitor op(this->rete_session.get(), nullptr);
  rdf::LString lhs("==Allo");
  rdf::LInt32 rhs(2);
  auto res = boost::apply_visitor(op, rdf::RdfAstType(lhs), rdf::RdfAstType(rhs));
  EXPECT_EQ(res, rdf::RdfAstType(rdf::LString("A")));
}
TEST_F(ExprOpTest, ReplaceCharOfVisitor1) {
  auto sess = this->rete_session->rdf_session();
  auto rmgr = sess->rmgr();
  rdf::r_index config = rmgr->create_resource("config");
  sess->insert(config, rmgr->jets()->jets__replace_chars, rmgr->create_literal("`' "));
  sess->insert(config, rmgr->jets()->jets__replace_with, rmgr->create_literal("_"));
  ReplaceCharOfVisitor op(this->rete_session.get(), nullptr);
  rdf::NamedResource lhs("config");
  rdf::LString rhs("something`that'different or else");
  auto res = boost::apply_visitor(op, rdf::RdfAstType(lhs), rdf::RdfAstType(rhs));
  EXPECT_EQ(res, rdf::RdfAstType(rdf::LString("something_that_different_or_else")));
}
TEST_F(ExprOpTest, ReplaceCharOfVisitor2) {
  auto sess = this->rete_session->rdf_session();
  auto rmgr = sess->rmgr();
  rdf::r_index config = rmgr->create_resource("config");
  sess->insert(config, rmgr->jets()->jets__replace_chars, rmgr->create_literal("`' "));
  sess->insert(config, rmgr->jets()->jets__replace_with, rmgr->create_literal("_"));
  ReplaceCharOfVisitor op(this->rete_session.get(), nullptr);
  rdf::NamedResource lhs("config");
  rdf::LString rhs("something`that");
  auto res = boost::apply_visitor(op, rdf::RdfAstType(lhs), rdf::RdfAstType(rhs));
  EXPECT_EQ(res, rdf::RdfAstType(rdf::LString("something_that")));
}

TEST_F(ExprOpTest, RangeVisitor1) {
  auto sess = this->rete_session->rdf_session();
  auto rmgr = sess->rmgr();
  RangeVisitor op(this->rete_session.get(), nullptr);
  rdf::LInt32 lhs(1);
  rdf::LInt32 rhs(3);
  auto res = boost::apply_visitor(op, rdf::RdfAstType(lhs), rdf::RdfAstType(rhs));

  // std::cout << sess << std::endl;
  rdf::r_index s = rmgr->get_bnode(get_key(&res));
  rdf::r_index p = rmgr->create_resource("jets:range_value");
  EXPECT_FALSE(sess->contains(s, p, rmgr->create_literal(0)));
  EXPECT_TRUE(sess->contains(s, p, rmgr->create_literal(1)));
  EXPECT_TRUE(sess->contains(s, p, rmgr->create_literal(2)));
  EXPECT_TRUE(sess->contains(s, p, rmgr->create_literal(3)));
  EXPECT_FALSE(sess->contains(s, p, rmgr->create_literal(4)));
}

TEST_F(ExprOpTest, RangeVisitor2) {
  auto sess = this->rete_session->rdf_session();
  auto rmgr = sess->rmgr();
  RangeVisitor op(this->rete_session.get(), nullptr);
  rdf::LInt32 lhs(0);
  rdf::LInt32 rhs(3);
  auto res = boost::apply_visitor(op, rdf::RdfAstType(lhs), rdf::RdfAstType(rhs));
  // std::cout << sess << std::endl;
  rdf::r_index s = rmgr->get_bnode(get_key(&res));
  rdf::r_index p = rmgr->create_resource("jets:range_value");
  EXPECT_TRUE(sess->contains(s, p, rmgr->create_literal(0)));
  EXPECT_TRUE(sess->contains(s, p, rmgr->create_literal(1)));
  EXPECT_TRUE(sess->contains(s, p, rmgr->create_literal(2)));
  EXPECT_FALSE(sess->contains(s, p, rmgr->create_literal(3)));
  EXPECT_FALSE(sess->contains(s, p, rmgr->create_literal(4)));
}

// sum_values over a property whose values a hash set holds, so the order they are met
// in is the set's. Many subjects, each with values of its own, inserted from a different
// one for each subject: some subject meets each kind of value first, and the sum must
// not depend on which.
class SumValuesOrderTest : public ExprOpTest {
 protected:
  // (s_i, amount, v) for each v of values(i); the sum, and the expected sum, per subject
  template<class Values, class Expected>
  void expect_sums(Values values, Expected expected, int subjects = 64)
  {
    auto * sess = this->rete_session->rdf_session();
    auto rmgr = sess->rmgr();
    auto p = rmgr->create_resource("amount");
    for(int i=0; i<subjects; i++) {
      auto name = "subject" + std::to_string(i) + "_" + std::to_string(this->batch);
      auto s = rmgr->create_resource(name);
      // inserted from a different value for each subject: a small table can keep the
      // order of insertion
      auto vs = values(rmgr, i);
      for(size_t k=0; k<vs.size(); k++) sess->insert(s, p, vs[(k + i) % vs.size()]);
      auto first = sess->find(s, p);
      if(not first.is_end() and first.get_object()->which() == rdf::rdf_null_t) this->null_first++;
      SumValuesVisitor op(this->rete_session.get(), nullptr);
      auto sum = boost::apply_visitor(op, rdf::RdfAstType(rdf::NamedResource(name)),
        rdf::RdfAstType(rdf::NamedResource("amount")));
      EXPECT_EQ(sum, expected(i)) << "subject " << i << ": " << sum;
    }
    this->batch++;
  }
  int batch = 0;
  int null_first = 0;   // subjects whose first value met is null
};

using Values = std::vector<rdf::r_index>;

TEST_F(SumValuesOrderTest, ANullIsSkippedWhereverItComes) {
  expect_sums([](auto rmgr, int i) { return Values{rmgr->get_null(), 
      rmgr->create_literal(1.5 + i), rmgr->create_literal(0.25 + i)}; },
    [](int i) { return RDFTTYPE(rdf::LDouble(1.75 + 2*i)); });
  expect_sums([](auto rmgr, int i) { return Values{rmgr->get_null(), 
      rmgr->create_literal(int32_t(100 + i)), rmgr->create_literal(int32_t(1000 + i))}; },
    [](int i) { return RDFTTYPE(rdf::LInt32(1100 + 2*i)); });
  EXPECT_GT(this->null_first, 0) << "no subject met its null first: the case is not exercised";
  // only nulls: null
  expect_sums([](auto rmgr, int) { return Values{rmgr->get_null()}; },
    [](int) { return RDFTTYPE(rdf::RDFNull()); }, 4);
}

// A value that is not a number cannot be summed: the sum is null, wherever it comes.
// Until 2026-10 text concatenated, and 5, "x", 2 gave "x25" or no answer by order.
TEST_F(SumValuesOrderTest, AValueThatIsNotANumberMakesItNull) {
  expect_sums([](auto rmgr, int i) { return Values{rmgr->create_literal(int32_t(5 + i)),
      rmgr->create_literal("x" + std::to_string(i)), rmgr->create_literal(int32_t(2))}; },
    [](int) { return RDFTTYPE(rdf::RDFNull()); });
  expect_sums([](auto rmgr, int i) { return Values{rmgr->create_literal("a" + std::to_string(i)),
      rmgr->create_literal("b" + std::to_string(i))}; },
    [](int) { return RDFTTYPE(rdf::RDFNull()); });
  expect_sums([](auto rmgr, int i) { return Values{rmgr->create_literal(uint32_t(10 + i)),
      rmgr->create_literal(uint32_t(2)), rmgr->create_literal(uint32_t(4))}; },
    [](int i) { return RDFTTYPE(rdf::LUInt32(16 + i)); });
}

TEST_F(SumValuesOrderTest, ADoubleMakesTheSumADouble) {
  expect_sums([](auto rmgr, int i) { return Values{rmgr->create_literal(int32_t(i)), 
      rmgr->create_literal(0.5 + i)}; },
    [](int i) { return RDFTTYPE(rdf::LDouble(0.5 + 2*i)); });
  expect_sums([](auto rmgr, int i) { return Values{rmgr->create_literal(int32_t(i)), 
      rmgr->create_literal(int64_t(1000 + i)), rmgr->create_literal(0.25 + i), rmgr->get_null()}; },
    [](int i) { return RDFTTYPE(rdf::LDouble(1000.25 + 3*i)); });
}

TEST_F(SumValuesOrderTest, IntegersTakeTheWidestTypePresent) {
  expect_sums([](auto rmgr, int i) { return Values{rmgr->create_literal(int32_t(i)), 
      rmgr->create_literal(int64_t(5000000000 + i))}; },
    [](int i) { return RDFTTYPE(rdf::LInt64(5000000000 + 2*i)); });
  expect_sums([](auto rmgr, int i) { return Values{rmgr->create_literal(int32_t(-1 - i)), 
      rmgr->create_literal(uint32_t(10 + 2*i))}; },
    [](int i) { return RDFTTYPE(rdf::LUInt32(9 + i)); });
}

// I-16: a double as text is its shortest round-trip digits, in plain notation. This is
// the Go engine's table, DoubleTextCases in jets/jetrules/rdf/text_test.go, as is, so both
// engines write the same text.
struct DoubleText { double v; char const* text; };
std::vector<DoubleText> const& double_text_table()
{
  static std::vector<DoubleText> const t = {
    {991, "991"},
    {-991, "-991"},
    {0, "0"},
    {-0.0, "-0"},
    {1, "1"},
    {0.5, "0.5"},
    {-2.75, "-2.75"},
    {1.5, "1.5"},
    {100.1, "100.1"},
    {12.34, "12.34"},
    {0.891089, "0.891089"},
    {1234567.5, "1234567.5"},
    {1234567.89, "1234567.89"},
    {1e6, "1000000"},
    {0.00001, "0.00001"},
    {1e-7, "0.0000001"},
    {0.30000000000000004, "0.30000000000000004"},
    {0.3333333333333333, "0.3333333333333333"},
    {0.6666666666666666, "0.6666666666666666"},
    {123456789.123, "123456789.123"},
    {9007199254740992.0, "9007199254740992"},
    {123456789012345678.0, "123456789012345680"},
    {1e21, "1000000000000000000000"},
    {1e23, "99999999999999991611392"},
    {-1e23, "-99999999999999991611392"},
    {std::numeric_limits<double>::quiet_NaN(), "nan"},
    {std::numeric_limits<double>::infinity(), "inf"},
    {-std::numeric_limits<double>::infinity(), "-inf"},
  };
  return t;
}

TEST(DoubleToText, ShortestDigitsNeverAnExponent) {
  for(auto const& c: double_text_table()) {
    EXPECT_EQ(rdf::double_to_text(c.v), c.text);
    EXPECT_EQ(boost::apply_visitor(JoinTextVisitor(), RDFTTYPE(rdf::LDouble(c.v))), c.text);
  }
  // a NaN's sign is dropped: on x86 0.0/0.0 is a negative NaN
  EXPECT_EQ(rdf::double_to_text(-std::numeric_limits<double>::quiet_NaN()), "nan");
}

TEST_F(ExprOpTest, EveryOperatorWritesADoubleTheSameWay) {
  for(auto const& c: double_text_table()) {
    AddVisitor add(this->rete_session.get(), nullptr);
    EXPECT_EQ(boost::apply_visitor(add, RDFTTYPE(rdf::LString("x=")), RDFTTYPE(rdf::LDouble(c.v))),
      RDFTTYPE(rdf::LString(std::string("x=") + c.text)));
    ToTextVisitor to_text(this->rete_session.get(), nullptr);
    EXPECT_EQ(boost::apply_visitor(to_text, RDFTTYPE(rdf::LDouble(c.v))), RDFTTYPE(rdf::LString(c.text)));
  }
  // create_entity names the entity from a non-zero double
  CreateEntityVisitor ce(this->rete_session.get(), nullptr);
  EXPECT_EQ(boost::apply_visitor(ce, RDFTTYPE(rdf::LDouble(1234567.5))), 
    RDFTTYPE(rdf::NamedResource("1234567.5")));
}

// I-33: sorted_head on equal values picks the object first in the tie-break order
// (tie_break_less), not the first the graph meets. Equal weights, inserted from a
// different object for each subject, so that some subject meets the later name first.
TEST_F(ExprOpTest, SortedHeadBreaksATieByName) {
  auto * sess = this->rete_session->rdf_session();
  auto rmgr = sess->rmgr();
  auto const* jr = rmgr->jets();
  auto list = rmgr->create_resource("list");
  auto weight = rmgr->create_resource("weight");
  for(auto op: {"<", ">"}) {
    auto cfg = rmgr->create_resource(std::string("cfg") + op);
    sess->insert(cfg, jr->jets__entity_property, list);
    sess->insert(cfg, jr->jets__value_property, weight);
    sess->insert(cfg, jr->jets__operator, rmgr->create_literal(std::string(op)));
  }
  int later_first = 0;
  for(int i=0; i<64; i++) {
    for(auto op: {"<", ">"}) {
      auto name = std::string("s") + op + std::to_string(i);
      auto s = rmgr->create_resource(name);
      // the two tied objects, and one that loses either way
      bool is_min = std::string(op) == "<";
      std::vector<std::pair<std::string, int>> objs = {
        {name + "_a", 5}, {name + "_b", 5}, {name + "_c", is_min ? 9 : 1}};
      for(size_t k=0; k<objs.size(); k++) {
        auto const& o = objs[(k + i) % objs.size()];
        auto obj = rmgr->create_resource(o.first);
        sess->insert(s, list, obj);
        sess->insert(obj, weight, rmgr->create_literal(int32_t(o.second)));
      }
      for(auto itor = sess->find(s, list); not itor.is_end(); itor.next()) {
        auto n = rdf::get_name(itor.get_object());
        if(n == name + "_a") break;
        if(n == name + "_b") { later_first++; break; }
      }
      SortedHeadVisitor head(this->rete_session.get(), nullptr);
      auto res = boost::apply_visitor(head, RDFTTYPE(rdf::NamedResource(name)),
        RDFTTYPE(rdf::NamedResource(std::string("cfg") + op)));
      EXPECT_EQ(res, RDFTTYPE(rdf::NamedResource(name + "_a"))) << name;
    }
  }
  EXPECT_GT(later_first, 0) << "no subject met the later name first: the case is not exercised";
}

// A tie is exact equality (tie_equal), not "neither is greater": an int 5 and a double
// 5.0 tie, so the name decides; a NaN ties with nothing, so the first object met wins.
class SortedHeadTieTest : public ExprOpTest {
 protected:
  // sorted_head "<" over two objects, _a and _b, with these weights, inserted from a
  // different one for each subject; for each subject the result and the first object met
  template<class F>
  void run(rdf::r_index wa, rdf::r_index wb, F && check)
  {
    auto * sess = this->rete_session->rdf_session();
    auto rmgr = sess->rmgr();
    auto const* jr = rmgr->jets();
    auto list = rmgr->create_resource("tlist");
    auto weight = rmgr->create_resource("tweight");
    auto cfg = rmgr->create_resource("tcfg" + std::to_string(batch));
    sess->insert(cfg, jr->jets__entity_property, list);
    sess->insert(cfg, jr->jets__value_property, weight);
    sess->insert(cfg, jr->jets__operator, rmgr->create_literal(std::string("<")));
    for(int i=0; i<64; i++) {
      auto name = "t" + std::to_string(batch) + "_" + std::to_string(i);
      auto s = rmgr->create_resource(name);
      std::vector<std::pair<std::string, rdf::r_index>> objs = {{name + "_a", wa}, {name + "_b", wb}};
      for(size_t k=0; k<objs.size(); k++) {
        auto const& o = objs[(k + i) % objs.size()];
        auto obj = rmgr->create_resource(o.first);
        sess->insert(s, list, obj);
        sess->insert(obj, weight, o.second);
      }
      auto first = rdf::get_name(sess->find(s, list).get_object());
      SortedHeadVisitor head(this->rete_session.get(), nullptr);
      auto res = boost::apply_visitor(head, RDFTTYPE(rdf::NamedResource(name)),
        RDFTTYPE(rdf::NamedResource("tcfg" + std::to_string(batch))));
      check(name, boost::get<rdf::NamedResource>(res).name, first);
    }
    batch++;
  }
  int batch = 0;
};

TEST_F(SortedHeadTieTest, AnIntAndAnEqualDoubleTie) {
  auto rmgr = this->rete_session->rdf_session()->rmgr();
  int b_first = 0;
  run(rmgr->create_literal(5.0), rmgr->create_literal(int32_t(5)),
    [&](std::string const& name, std::string const& res, std::string const& first) {
      if(first == name + "_b") b_first++;
      EXPECT_EQ(res, name + "_a");
    });
  EXPECT_GT(b_first, 0) << "no subject met _b first: the case is not exercised";
}

// An int 5 and a double 5.7 are not equal. EqVisitor says they are (it truncates the
// double), which would let the name pick 5.7 as the minimum.
TEST_F(SortedHeadTieTest, AnIntAndADifferentDoubleDoNotTie) {
  auto rmgr = this->rete_session->rdf_session()->rmgr();
  int b_first = 0;
  run(rmgr->create_literal(5.7), rmgr->create_literal(int32_t(5)),
    [&](std::string const& name, std::string const& res, std::string const& first) {
      if(first == name + "_b") b_first++;
      EXPECT_EQ(res, name + "_b");
    });
  EXPECT_GT(b_first, 0) << "no subject met _b first: the case is not exercised";
}

TEST_F(SortedHeadTieTest, ANaNTiesWithNothing) {
  auto rmgr = this->rete_session->rdf_session()->rmgr();
  int b_first = 0, a_first = 0;
  run(rmgr->create_literal(std::numeric_limits<double>::quiet_NaN()), rmgr->create_literal(2.0),
    [&](std::string const& name, std::string const& res, std::string const& first) {
      (first == name + "_b" ? b_first : a_first)++;
      EXPECT_EQ(res, first) << "not a tie: the first object met wins, as before";
    });
  EXPECT_GT(a_first, 0);
  EXPECT_GT(b_first, 0);
}

TEST(TieBreak, ResourcesThenBlankNodesThenLiterals) {
  auto r_a = rdf::mkResource("a"), r_b = rdf::mkResource("b");
  auto b1 = rdf::mkBlankNode(1), b2 = rdf::mkBlankNode(2);
  auto i10 = rdf::mkLiteral(int32_t(10)), i9 = rdf::mkLiteral(int32_t(9));
  auto d10 = rdf::mkLiteral(10.0), s10 = rdf::mkLiteral("10");
  EXPECT_TRUE(tie_break_less(r_a.get(), r_b.get()));
  EXPECT_FALSE(tie_break_less(r_b.get(), r_a.get()));
  EXPECT_TRUE(tie_break_less(r_b.get(), b1.get()));
  EXPECT_TRUE(tie_break_less(b1.get(), b2.get()));
  EXPECT_TRUE(tie_break_less(b2.get(), i9.get()));
  EXPECT_TRUE(tie_break_less(i10.get(), i9.get()));     // by text: "10" < "9"
  EXPECT_TRUE(tie_break_less(i10.get(), d10.get()));    // equal text "10": int32 first
  auto l10 = rdf::mkLiteral(int64_t(10)), u10 = rdf::mkLiteral(uint32_t(10));
  EXPECT_TRUE(tie_break_less(l10.get(), u10.get()));    // signed before unsigned
  EXPECT_FALSE(tie_break_less(u10.get(), l10.get()));
  EXPECT_TRUE(tie_break_less(d10.get(), s10.get()));    // then double, then string
  EXPECT_FALSE(tie_break_less(r_a.get(), r_a.get()));
}

// + - * /: a double operand makes the result a double, on either side. Until 2026-10 an
// integer first truncated the double in + and - (2 + 0.5 was 2).
template<class V>
void expect_double_result(char const* op, double (*f)(double, double))
{
  V v(nullptr, nullptr);
  std::vector<RDFTTYPE> ints = {rdf::LInt32(7), rdf::LUInt32(7), rdf::LInt64(7), rdf::LUInt64(7)};
  for(auto const& n: ints) {
    EXPECT_EQ(boost::apply_visitor(v, n, RDFTTYPE(rdf::LDouble(0.5))), RDFTTYPE(rdf::LDouble(f(7, 0.5))))
      << n << " " << op << " 0.5";
    EXPECT_EQ(boost::apply_visitor(v, RDFTTYPE(rdf::LDouble(0.5)), n), RDFTTYPE(rdf::LDouble(f(0.5, 7))))
      << "0.5 " << op << " " << n;
  }
}

TEST(ArithmeticTypes, ADoubleOperandMakesADouble) {
  expect_double_result<AddVisitor>("+", [](double a, double b) { return a + b; });
  expect_double_result<SubsVisitor>("-", [](double a, double b) { return a - b; });
  expect_double_result<MultVisitor>("*", [](double a, double b) { return a * b; });
  expect_double_result<DivVisitor>("/", [](double a, double b) { return a / b; });
}

// The typing table shared with the Go engine's TestArithmeticTyping
// (jets/jetrules/rdf/ast_math_operators_test.go). Go's int and uint are 64-bit, so they are
// int64 and uint64 here. Not here: Go's "uint + negative int" and "int + uint past int",
// which give no result on Go and throw in numeric_cast here, a recorded difference.
TEST(ArithmeticTypes, TheSharedTypingTable) {
  AddVisitor add(nullptr, nullptr);
  SubsVisitor sub(nullptr, nullptr);
  MultVisitor mul(nullptr, nullptr);
  DivVisitor div(nullptr, nullptr);
  auto I = [](int64_t v) { return RDFTTYPE(rdf::LInt64(v)); };
  auto UI = [](uint64_t v) { return RDFTTYPE(rdf::LUInt64(v)); };
  auto F = [](double v) { return RDFTTYPE(rdf::LDouble(v)); };
  auto DD = [](int y, int m, int d) { return RDFTTYPE(rdf::LDate(rdf::date(y, m, d))); };
  auto ap = [](auto & op, RDFTTYPE a, RDFTTYPE b) { return boost::apply_visitor(op, a, b); };
  EXPECT_EQ(ap(add, I(5), F(0.5)), F(5.5));
  EXPECT_EQ(ap(add, F(0.5), I(5)), F(5.5));
  EXPECT_EQ(ap(sub, I(5), F(0.5)), F(4.5));
  EXPECT_EQ(ap(sub, F(0.5), I(5)), F(-4.5));
  EXPECT_EQ(ap(mul, I(5), F(0.5)), F(2.5));
  EXPECT_EQ(ap(mul, F(0.5), I(5)), F(2.5));
  EXPECT_EQ(ap(div, I(5), F(2)), F(2.5));
  EXPECT_EQ(ap(div, I(5), I(2)), I(2));
  EXPECT_EQ(ap(add, I(5), I(2)), I(7));
  EXPECT_EQ(ap(add, I(5), UI(3)), I(8));
  EXPECT_EQ(ap(add, UI(5), I(3)), UI(8));
  EXPECT_EQ(ap(add, UI(5), UI(3)), UI(8));
  EXPECT_EQ(ap(sub, UI(5), I(3)), UI(2));
  EXPECT_EQ(ap(mul, UI(5), I(3)), UI(15));
  EXPECT_EQ(ap(div, UI(7), I(2)), UI(3));
  EXPECT_EQ(ap(mul, UI(5), F(0.5)), F(2.5));
  EXPECT_EQ(ap(add, I(5), RDFTTYPE(rdf::LString("x"))).which(), rdf::rdf_null_t);
  EXPECT_EQ(ap(add, DD(2025, 1, 1), I(1)), DD(2025, 1, 2));
}

TEST(ArithmeticTypes, IntegersKeepTheLeftOperandsType) {
  AddVisitor add(nullptr, nullptr);
  EXPECT_EQ(boost::apply_visitor(add, RDFTTYPE(rdf::LInt32(2)), RDFTTYPE(rdf::LInt64(3))), RDFTTYPE(rdf::LInt32(5)));
  EXPECT_EQ(boost::apply_visitor(add, RDFTTYPE(rdf::LInt64(2)), RDFTTYPE(rdf::LInt32(3))), RDFTTYPE(rdf::LInt64(5)));
  DivVisitor div(nullptr, nullptr);
  EXPECT_EQ(boost::apply_visitor(div, RDFTTYPE(rdf::LInt32(7)), RDFTTYPE(rdf::LInt32(2))), RDFTTYPE(rdf::LInt32(3)));
}

}   // namespace
}   // namespace jets::rdf