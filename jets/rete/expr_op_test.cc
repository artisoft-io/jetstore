#include <cmath>
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
// The edges of substring_of: each row is jets:from, jets:length, the text and the
// result. A cut longer than the text gives an empty string, never the remainder and
// never an exception; a cut that runs past the end gives what is there.
TEST_F(ExprOpTest, SubstringOfVisitorEdges) {
  struct Row { std::int32_t from; std::int32_t length; std::string text; std::string want; };
  std::vector<Row> rows = {
    // a positive length within the text
    {0, 3, "abcdef", "abc"},
    {2, 2, "abcdef", "cd"},
    {1, 0, "abcdef", ""},
    // a negative length within the text drops characters from the end
    {0, -3, "abcdef", "abc"},
    {1, -2, "abcdef", "bcd"},
    {2, -4, "abcdef", ""},
    // a negative length on a text too short for the cut
    {0, -3, "ab", ""},
    {0, -3, "abc", ""},
    {2, -3, "abcd", ""},
    {0, -1, "", ""},
    {0, std::numeric_limits<std::int32_t>::min(), "abc", ""},
    // jets:from at the end of the text
    {3, 2, "abc", ""},
    {3, -1, "abc", ""},
    {0, 2, "", ""},
    // jets:from past the end of the text
    {4, 2, "abc", ""},
    {10, -1, "abc", ""},
    {std::numeric_limits<std::int32_t>::max(), 1, "abc", ""},
    // jets:from + jets:length past the end gives the available characters
    {1, 10, "abc", "bc"},
    {0, std::numeric_limits<std::int32_t>::max(), "abc", "abc"},
    {2, std::numeric_limits<std::int32_t>::max(), "abc", "c"},
    // a negative jets:from
    {-1, 2, "abc", ""},
    {-1, -1, "abc", ""},
    {std::numeric_limits<std::int32_t>::min(), 2, "abc", ""},
    // positions and lengths count characters (code points), not bytes: "é" is two
    // bytes in UTF-8 (C3 A9), "€" three (E2 82 AC), "😀" four (F0 9F 98 80)
    {0, 1, "\xc3\xa9t\xc3\xa9", "\xc3\xa9"},         // bytes split the "é"
    {0, 3, "\xc3\xa9t\xc3\xa9", "\xc3\xa9t\xc3\xa9"},
    {1, 1, "\xc3\xa9t\xc3\xa9", "t"},
    {2, 5, "\xc3\xa9t\xc3\xa9", "\xc3\xa9"},
    {3, 1, "\xc3\xa9t\xc3\xa9", ""},                  // past the 3 characters
    {0, -1, "\xc3\xa9t\xc3\xa9", "\xc3\xa9t"},
    {1, 1, "5\xe2\x82\xac!", "\xe2\x82\xac"},
    {0, -1, "5\xe2\x82\xac", "5"},
    {1, 1, "a\xf0\x9f\x98\x80" "b", "\xf0\x9f\x98\x80"},
    {2, 1, "a\xf0\x9f\x98\x80" "b", "b"},
    // no normalisation: "e" and a combining acute (CC 81) are two characters
    {0, 1, "e\xcc\x81t", "e"},
    {1, 2, "e\xcc\x81t", "\xcc\x81t"},
    // a byte that does not start a valid sequence is one character, kept as it is
    {1, 1, "a\xff" "b", "\xff"},
    {2, 1, "a\xff" "b", "b"},
    {0, 2, "\x80\x80", "\x80\x80"},                    // stray continuation bytes
    {1, 2, "a\xe2\x82", "\xe2\x82"},                   // a truncated sequence: two characters
    {2, 1, "a\xe2\x82", "\x82"},
    {0, 2, "\xc0\xafx", "\xc0\xaf"},                    // an overlong form: two characters
  };
  auto sess = this->rete_session->rdf_session();
  auto rmgr = sess->rmgr();
  SubstringOfVisitor op(this->rete_session.get(), nullptr);
  for(size_t i = 0; i < rows.size(); ++i) {
    auto const& r = rows[i];
    auto name = "config" + std::to_string(i);
    rdf::r_index config = rmgr->create_resource(name);
    sess->insert(config, rmgr->jets()->jets__from, rmgr->create_literal(r.from));
    sess->insert(config, rmgr->jets()->jets__length, rmgr->create_literal(r.length));
    RDFTTYPE res;
    try {
      res = boost::apply_visitor(op, RDFTTYPE(rdf::NamedResource(name)), RDFTTYPE(rdf::LString(r.text)));
    } catch(std::exception const& e) {
      res = rdf::LString(std::string("threw: ") + e.what());
    }
    EXPECT_EQ(res, RDFTTYPE(rdf::LString(r.want)))
      << "from " << r.from << ", length " << r.length << ", text \"" << r.text << "\"";
  }
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

// replace_char_of reads jets:replace_chars as UTF-8 characters (code points), not bytes.
// Each row is jets:replace_chars, jets:replace_with, the text and the result; a result of
// null is "<null>". "\xc3\xa9" is é and "\xc3\xa0" is à: they share the lead byte 0xC3, which
// a byte-by-byte replacement broke.
TEST_F(ExprOpTest, ReplaceCharOfVisitorCharacters) {
  struct Row { std::string chars; std::string with; std::string text; std::string want; };
  std::vector<Row> rows = {
    // ASCII, as it always was
    {"`' ", "_", "it's a `test`", "it_s_a__test_"},
    {"-.", "", "1-800.555", "1800555"},
    {"", "_", "unchanged text", "unchanged text"},
    // the characters are replaced in turn, each over what the earlier turns produced:
    // the "b"s inserted for "a" are then replaced by "bb" themselves
    {"ab", "bb", "a", "bbbb"},
    {"ba", "bb", "a", "bb"},
    // a character outside ASCII is replaced whole, and only it: à is left as it was
    {"\xc3\xa9", "e", "\xc3\xa9t\xc3\xa9 \xc3\xa0 Paris", "ete \xc3\xa0 Paris"},
    {"\xc3\xa9\xc3\xa0", "", "d\xc3\xa9j\xc3\xa0 vu", "dj vu"},
    // a replacement of more than one byte
    {"-", "\xe2\x80\x94", "a-b", "a\xe2\x80\x94" "b"},
    {"\xc3\xa9", "\xc3\xa8", "\xc3\xa9l\xc3\xa8ve", "\xc3\xa8l\xc3\xa8ve"},
    // three- and four-byte characters: \xe2\x82\xac is the euro sign, \xf0\x9f\x98\x80 an emoji
    {"\xe2\x82\xac\xf0\x9f\x98\x80", "?", "10\xe2\x82\xac \xf0\x9f\x98\x80!", "10? ?!"},
    // jets:replace_chars that is not valid UTF-8 is a configuration error: null
    {"\xc3", "e", "\xc3\xa9t\xc3\xa9", "<null>"},
    {"a\xa9", "e", "abc", "<null>"},
    {"\xc0\xaf", "e", "a/b", "<null>"},          // an overlong "/"
    {"\xed\xa0\x80", "e", "xyz", "<null>"},      // a surrogate, U+D800
  };
  auto sess = this->rete_session->rdf_session();
  auto rmgr = sess->rmgr();
  auto const* jr = rmgr->jets();
  ReplaceCharOfVisitor op(this->rete_session.get(), nullptr);
  for(size_t i = 0; i < rows.size(); ++i) {
    auto const& r = rows[i];
    auto name = "rcconfig" + std::to_string(i);
    rdf::r_index config = rmgr->create_resource(name);
    sess->insert(config, jr->jets__replace_chars, rmgr->create_literal(r.chars));
    sess->insert(config, jr->jets__replace_with, rmgr->create_literal(r.with));
    RDFTTYPE res;
    try {
      res = boost::apply_visitor(op, RDFTTYPE(rdf::NamedResource(name)), RDFTTYPE(rdf::LString(r.text)));
    } catch(std::exception const& e) {
      res = rdf::LString(std::string("threw: ") + e.what());
    }
    if(r.want == "<null>") {
      EXPECT_EQ(res, RDFTTYPE(rdf::Null())) << "row " << i;
    } else {
      EXPECT_EQ(res, RDFTTYPE(rdf::LString(r.want))) << "row " << i;
    }
  }
  // each invalid row inserted its error, which names the text (so the rows' texts differ)
  int errors = 0;
  for(auto itor = sess->find(jr->jets__istate, jr->jets__exception); not itor.is_end(); itor.next()) {
    auto const& v = *itor.get_object();
    if(v.which() == rdf::rdf_literal_string_t and
       boost::get<rdf::LString>(v).data.find("not valid UTF-8") != std::string::npos) errors++;
  }
  EXPECT_EQ(errors, 4);
}

TEST_F(ExprOpTest, SplitUtf8Chars) {
  std::vector<std::string> chars;
  EXPECT_TRUE(split_utf8_chars("", chars));
  EXPECT_TRUE(chars.empty());
  EXPECT_TRUE(split_utf8_chars("a\xc3\xa9\xe2\x82\xac\xf0\x9f\x98\x80", chars));
  EXPECT_EQ(chars, (std::vector<std::string>{"a", "\xc3\xa9", "\xe2\x82\xac", "\xf0\x9f\x98\x80"}));
  for(auto bad: {"\x80", "\xc3", "\xc3\x28", "\xc1\xbf", "\xe0\x80\xaf", "\xed\xbf\xbf",
                 "\xf0\x80\x80\xaf", "\xf4\x90\x80\x80", "\xf5\x80\x80\x80", "\xff"}) {
    std::vector<std::string> out;
    EXPECT_FALSE(split_utf8_chars(bad, out)) << "a " << std::string(bad).size() << "-byte case";
  }
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

// I-16: a double as text is its shortest round-trip digits, in plain notation. This table
// is the specification of rdf::double_to_text. It was written as the Go engine's table and
// shared with it, so that both engines wrote the same text, until the Go engine's retirement
// (decided 2026-10-22).
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

// A sorted_head config missing jets:operator, jets:entity_property or jets:value_property
// is the visitor's configuration error, a rete_exception. Until 2026-11-13 a config with
// no jets:operator dereferenced null and crashed the process.
TEST_F(ExprOpTest, SortedHeadConfigMissingAProperty) {
  auto * sess = this->rete_session->rdf_session();
  auto rmgr = sess->rmgr();
  auto const* jr = rmgr->jets();
  auto list = rmgr->create_resource("mlist");
  auto weight = rmgr->create_resource("mweight");
  auto s = rmgr->create_resource("msubject");
  auto obj = rmgr->create_resource("mobject");
  sess->insert(s, list, obj);
  sess->insert(obj, weight, rmgr->create_literal(int32_t(1)));
  struct Cfg { char const* name; bool entity; bool value; bool op; };
  for(auto c: {Cfg{"mcfg_no_op", true, true, false}, Cfg{"mcfg_no_entity", false, true, true},
               Cfg{"mcfg_no_value", true, false, true}, Cfg{"mcfg_empty", false, false, false}}) {
    auto cfg = rmgr->create_resource(c.name);
    if(c.entity) sess->insert(cfg, jr->jets__entity_property, list);
    if(c.value) sess->insert(cfg, jr->jets__value_property, weight);
    if(c.op) sess->insert(cfg, jr->jets__operator, rmgr->create_literal(std::string("<")));
    SortedHeadVisitor head(this->rete_session.get(), nullptr);
    try {
      boost::apply_visitor(head, RDFTTYPE(rdf::NamedResource("msubject")), RDFTTYPE(rdf::NamedResource(c.name)));
      ADD_FAILURE() << c.name << ": no exception";
    } catch(jets::rete_exception const& e) {
      EXPECT_NE(std::string(e.what()).find("jets:operator"), std::string::npos) << c.name << ": " << e.what();
    }
  }
}

// A sorted_head config whose jets:entity_property or jets:value_property is a literal
// rather than a resource is the same configuration error. Until 2026-11-14 the literal
// was passed to the graph's find, matched no triple, and the head was null, silently.
TEST_F(ExprOpTest, SortedHeadConfigWithALiteralProperty) {
  auto * sess = this->rete_session->rdf_session();
  auto rmgr = sess->rmgr();
  auto const* jr = rmgr->jets();
  auto list = rmgr->create_resource("llist");
  auto weight = rmgr->create_resource("lweight");
  auto s = rmgr->create_resource("lsubject");
  auto obj = rmgr->create_resource("lobject");
  sess->insert(s, list, obj);
  sess->insert(obj, weight, rmgr->create_literal(int32_t(1)));
  auto text = [&](char const* v) { return rmgr->create_literal(std::string(v)); };
  struct Cfg { char const* name; rdf::r_index entity; rdf::r_index value; };
  for(auto c: {Cfg{"lcfg_entity_text", text("llist"), weight},
               Cfg{"lcfg_value_text", list, text("lweight")},
               Cfg{"lcfg_value_int", list, rmgr->create_literal(int32_t(7))},
               Cfg{"lcfg_both_text", text("llist"), text("lweight")}}) {
    auto cfg = rmgr->create_resource(c.name);
    sess->insert(cfg, jr->jets__entity_property, c.entity);
    sess->insert(cfg, jr->jets__value_property, c.value);
    sess->insert(cfg, jr->jets__operator, text("<"));
    SortedHeadVisitor head(this->rete_session.get(), nullptr);
    try {
      boost::apply_visitor(head, RDFTTYPE(rdf::NamedResource("lsubject")), RDFTTYPE(rdf::NamedResource(c.name)));
      ADD_FAILURE() << c.name << ": no exception";
    } catch(jets::rete_exception const& e) {
      EXPECT_NE(std::string(e.what()).find("must be resources"), std::string::npos) << c.name << ": " << e.what();
    }
  }
  // the same config with resources gives the head
  auto cfg = rmgr->create_resource("lcfg_resources");
  sess->insert(cfg, jr->jets__entity_property, list);
  sess->insert(cfg, jr->jets__value_property, weight);
  sess->insert(cfg, jr->jets__operator, text("<"));
  SortedHeadVisitor head(this->rete_session.get(), nullptr);
  EXPECT_EQ(boost::apply_visitor(head, RDFTTYPE(rdf::NamedResource("lsubject")), RDFTTYPE(rdf::NamedResource("lcfg_resources"))),
    RDFTTYPE(rdf::NamedResource("lobject")));
}

// A tie is the rules' own == (EqVisitor), not "neither is greater": an int 5 and a double
// 5.0 tie, as do two doubles equal to 15 significant digits, so the name decides; a NaN
// ties with nothing, so the first object met wins.
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

// An int 5 and a double 5.7 are not equal. EqVisitor said they were until 2026-10 (it
// truncated the double), which would have let the name pick 5.7 as the minimum.
// Equal to 15 significant digits, as == says, though not bit-equal: a tie, as it was on Go
TEST_F(SortedHeadTieTest, DoublesEqualTo15DigitsTie) {
  auto rmgr = this->rete_session->rdf_session()->rmgr();
  int b_first = 0;
  run(rmgr->create_literal(0.3), rmgr->create_literal(0.30000000000000004),
    [&](std::string const& name, std::string const& res, std::string const& first) {
      if(first == name + "_b") b_first++;
      EXPECT_EQ(res, name + "_a");
    });
  EXPECT_GT(b_first, 0) << "no subject met _b first: the case is not exercised";
}

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

// A beta row, so that a visitor is evaluated as in a session's rule. With no row an
// operator's catch-all gives null for operands it does not take; with one, it threw, and
// the session failed (F517). Since Q-93 the catch-alls of + - * /, abs, to_int, to_double,
// lookup and multi_lookup give null either way; other operators' still differ. The rows
// below that name `in_session`, and the tables that take a SessionRow, are about that
// difference.
struct SessionRow {
  NodeVertexPtr vertex = create_node_vertex(nullptr, 0, 0, false, 10, {}, "", {});
  BetaRowPtr row = create_beta_row(vertex.get(), 0);
};

// A visitor's result, or the text "threw: ..." when it throws, so that each row of a table
// that throws fails on its own line rather than ending the test at the first.
template<class V>
RDFTTYPE or_threw(V & op, RDFTTYPE a)
{
  try { return boost::apply_visitor(op, a); }
  catch(std::exception const& e) { return rdf::LString(std::string("threw: ") + e.what()); }
}
template<class V>
RDFTTYPE or_threw(V & op, RDFTTYPE a, RDFTTYPE b)
{
  try { return boost::apply_visitor(op, a, b); }
  catch(std::exception const& e) { return rdf::LString(std::string("threw: ") + e.what()); }
}

// The typing table of + - * / : the specification of the arithmetic operators' result
// types. It was shared with the Go engine, since retired, whose int and uint were
// 64-bit, hence int64 and uint64 here. "uint + negative int" and "int + uint past int"
// gave no result on Go and threw in numeric_cast here, a recorded difference, until
// Q-85 (2026-10-23): a mixed-integer operation whose right operand does not fit the left
// operand's type is null (I-134), and so is text plus a named resource (I-109), which
// threw in a session. Each failed the session, and now the rule's consequent sees null.
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

  // Q-85, I-134: the right operand out of the left operand's range is null, in + - * /
  auto I32 = [](int32_t v) { return RDFTTYPE(rdf::LInt32(v)); };
  auto UI32 = [](uint32_t v) { return RDFTTYPE(rdf::LUInt32(v)); };
  auto null = [](RDFTTYPE r) { return r.which() == rdf::rdf_null_t; };
  int64_t const big = 3000000000;   // past int32, within uint32
  EXPECT_TRUE(null(ap(add, I32(5), I(big))));
  EXPECT_TRUE(null(ap(sub, I32(5), I(big))));
  EXPECT_TRUE(null(ap(mul, I32(5), I(big))));
  EXPECT_TRUE(null(ap(div, I32(5), I(big))));
  EXPECT_TRUE(null(ap(add, UI32(5), I32(-1))));
  EXPECT_TRUE(null(ap(sub, UI32(5), I32(-1))));
  EXPECT_TRUE(null(ap(mul, UI(5), I(-1))));
  EXPECT_TRUE(null(ap(div, UI(5), I(-1))));
  EXPECT_TRUE(null(ap(add, I(5), UI(std::numeric_limits<uint64_t>::max()))));
  EXPECT_TRUE(null(ap(add, I32(5), UI32(std::numeric_limits<uint32_t>::max()))));
  // and an operand that fits is converted as before
  EXPECT_EQ(ap(add, I32(5), I(3)), I32(8));
  EXPECT_EQ(ap(add, UI32(5), I(big)), UI32(static_cast<uint32_t>(5 + big)));
  EXPECT_EQ(ap(sub, I32(5), UI32(7)), I32(-2));

  // Q-85, I-109: text plus a named resource is null, in a session's rule as with no row
  SessionRow sr;
  AddVisitor add_in_session(nullptr, sr.row.get());
  EXPECT_TRUE(null(ap(add_in_session, RDFTTYPE(rdf::LString("x")), RDFTTYPE(rdf::NamedResource("r")))));
  EXPECT_TRUE(null(ap(add_in_session, RDFTTYPE(rdf::NamedResource("r")), RDFTTYPE(rdf::LString("x")))));
  EXPECT_TRUE(null(ap(add, RDFTTYPE(rdf::LString("x")), RDFTTYPE(rdf::NamedResource("r")))));
  // text plus a number still concatenates
  EXPECT_EQ(ap(add_in_session, RDFTTYPE(rdf::LString("x")), I32(5)), RDFTTYPE(rdf::LString("x5")));

  // Q-85 (BL.1b): text with a blank node, null with text and a double with text are
  // null in a session's rule, in either order and in each of + - * /; each threw. Text
  // plus null and text plus a double still concatenate, as they did
  SubsVisitor sub_in_session(nullptr, sr.row.get());
  MultVisitor mul_in_session(nullptr, sr.row.get());
  DivVisitor div_in_session(nullptr, sr.row.get());
  RDFTTYPE const text = rdf::LString("x"), bnode = rdf::BlankNode(3), nul = rdf::Null(), dbl = rdf::LDouble(2.5);
  auto mixes_are_null = [&](auto & op, char const* name, bool concatenates) {
    EXPECT_TRUE(null(ap(op, text, bnode))) << "text " << name << " blank node";
    EXPECT_TRUE(null(ap(op, bnode, text))) << "blank node " << name << " text";
    EXPECT_TRUE(null(ap(op, nul, text))) << "null " << name << " text";
    EXPECT_TRUE(null(ap(op, dbl, text))) << "double " << name << " text";
    if(not concatenates) {
      EXPECT_TRUE(null(ap(op, text, nul))) << "text " << name << " null";
      EXPECT_TRUE(null(ap(op, text, dbl))) << "text " << name << " double";
    }
  };
  mixes_are_null(add_in_session, "+", true);
  mixes_are_null(sub_in_session, "-", false);
  mixes_are_null(mul_in_session, "*", false);
  mixes_are_null(div_in_session, "/", false);
  EXPECT_EQ(ap(add_in_session, text, nul), RDFTTYPE(rdf::LString("xnull")));
  EXPECT_EQ(ap(add_in_session, text, dbl), RDFTTYPE(rdf::LString("x2.5")));
}

// Q-85 (BL.1b): to_int of a number out of int32's range is null; numeric_cast threw,
// in a session's rule as with no row. A number in range is truncated toward zero, as it was
TEST(ArithmeticTypes, ToIntOutOfRangeIsNull) {
  SessionRow sr;
  ToIntVisitor to_int(nullptr, sr.row.get());
  auto ti = [&](RDFTTYPE v) { return boost::apply_visitor(to_int, v); };
  auto null = [](RDFTTYPE r) { return r.which() == rdf::rdf_null_t; };
  double const inf = std::numeric_limits<double>::infinity();
  EXPECT_TRUE(null(ti(rdf::LUInt32(std::numeric_limits<uint32_t>::max()))));
  EXPECT_TRUE(null(ti(rdf::LInt64(3000000000))));
  EXPECT_TRUE(null(ti(rdf::LInt64(-3000000000))));
  EXPECT_TRUE(null(ti(rdf::LUInt64(std::numeric_limits<uint64_t>::max()))));
  EXPECT_TRUE(null(ti(rdf::LDouble(1e300))));
  EXPECT_TRUE(null(ti(rdf::LDouble(-1e300))));
  EXPECT_TRUE(null(ti(rdf::LDouble(inf))));
  EXPECT_EQ(ti(rdf::LInt64(-7)), RDFTTYPE(rdf::LInt32(-7)));
  EXPECT_EQ(ti(rdf::LUInt32(5)), RDFTTYPE(rdf::LInt32(5)));
  EXPECT_EQ(ti(rdf::LDouble(2.9)), RDFTTYPE(rdf::LInt32(2)));
  EXPECT_EQ(ti(rdf::LDouble(-2.9)), RDFTTYPE(rdf::LInt32(-2)));
  EXPECT_EQ(ti(rdf::LString(" 42 ")), RDFTTYPE(rdf::LInt32(42)));
}

// Q-93: every type mismatch of + - * / is null, in a session's rule as with no row: a
// number with text, a named resource, a blank node or null, either way round, and two
// operands neither of which the operator takes. Each threw in a session's rule, and the
// session failed (I-184). Text first in + is concatenation, by design, and is not a
// mismatch: "x" + 5 is "x5", while 5 + "x" is null.
TEST(ArithmeticTypes, EveryTypeMismatchIsNull) {
  SessionRow sr;
  AddVisitor add(nullptr, sr.row.get());
  SubsVisitor sub(nullptr, sr.row.get());
  MultVisitor mul(nullptr, sr.row.get());
  DivVisitor div(nullptr, sr.row.get());
  auto ap = [](auto & op, RDFTTYPE a, RDFTTYPE b) { return or_threw(op, a, b); };
  auto null = [](RDFTTYPE r) { return r.which() == rdf::rdf_null_t; };
  std::vector<RDFTTYPE> const numbers = {rdf::LInt32(5), rdf::LUInt32(5), rdf::LInt64(5), rdf::LUInt64(5), rdf::LDouble(2.5)};
  std::vector<RDFTTYPE> const others = {rdf::LString("x"), rdf::LString(""), rdf::NamedResource("r"), rdf::BlankNode(3), rdf::Null()};
  // two operands neither of which is a number the operator takes
  std::vector<std::pair<RDFTTYPE, RDFTTYPE>> const pairs = {
    {rdf::NamedResource("r"), rdf::NamedResource("s")},
    {rdf::BlankNode(3), rdf::BlankNode(4)},
    {rdf::Null(), rdf::Null()},
    {rdf::NamedResource("r"), rdf::Null()},
    {rdf::Null(), rdf::BlankNode(3)},
    {rdf::LDate(rdf::date(2025, 1, 1)), rdf::LString("x")},
    {rdf::LDate(rdf::date(2025, 1, 1)), rdf::NamedResource("r")},
    {rdf::LDatetime(rdf::parse_datetime("2025-01-01T10:00:00")), rdf::BlankNode(3)},
    {rdf::LDate(rdf::date(2025, 1, 1)), rdf::LDatetime(rdf::parse_datetime("2025-01-01T10:00:00"))},
  };
  auto mismatches_are_null = [&](auto & op, char const* name, bool text_first_concatenates) {
    for(auto const& n: numbers) {
      for(auto const& x: others) {
        EXPECT_TRUE(null(ap(op, n, x))) << n << " " << name << " " << x;
        bool const concatenates = text_first_concatenates and x.which() == rdf::rdf_literal_string_t;
        if(not concatenates) EXPECT_TRUE(null(ap(op, x, n))) << x << " " << name << " " << n;
      }
    }
    for(auto const& [l, r]: pairs) {
      EXPECT_TRUE(null(ap(op, l, r))) << l << " " << name << " " << r;
      if(not (text_first_concatenates and r.which() == rdf::rdf_literal_string_t)) {
        EXPECT_TRUE(null(ap(op, r, l))) << r << " " << name << " " << l;
      }
    }
  };
  mismatches_are_null(add, "+", true);
  mismatches_are_null(sub, "-", false);
  mismatches_are_null(mul, "*", false);
  mismatches_are_null(div, "/", false);
  // text first in + concatenates, as it did
  EXPECT_EQ(ap(add, RDFTTYPE(rdf::LString("x")), RDFTTYPE(rdf::LInt32(5))), RDFTTYPE(rdf::LString("x5")));
  EXPECT_EQ(ap(add, RDFTTYPE(rdf::LString("x")), RDFTTYPE(rdf::LDouble(2.5))), RDFTTYPE(rdf::LString("x2.5")));
  EXPECT_EQ(ap(add, RDFTTYPE(rdf::LString("x")), RDFTTYPE(rdf::LDate(rdf::date(2025, 1, 1)))), RDFTTYPE(rdf::LString("x2025-01-01")));
  // and the operations defined on dates are unchanged
  EXPECT_EQ(ap(sub, RDFTTYPE(rdf::LDate(rdf::date(2025, 1, 3))), RDFTTYPE(rdf::LDate(rdf::date(2025, 1, 1)))), RDFTTYPE(rdf::LInt32(2)));

  // abs of anything but a number is null
  AbsVisitor abs(nullptr, sr.row.get());
  for(auto const& x: others) {
    EXPECT_TRUE(null(or_threw(abs, x))) << "abs " << x;
  }
  EXPECT_TRUE(null(or_threw(abs, RDFTTYPE(rdf::LDate(rdf::date(2025, 1, 1))))));
  EXPECT_EQ(or_threw(abs, RDFTTYPE(rdf::LInt32(-5))), RDFTTYPE(rdf::LInt32(5)));
}

// Q-93: to_int and to_double of what they cannot read are null, in a session's rule as
// with no row: text that does not read as their type, empty or blank text, null, a named
// resource, a blank node or a date (I-185); and to_int of a NaN, which converted to
// int32 by undefined behaviour, numeric_cast not checking it (I-186). Each threw in a
// session's rule except the NaN, which gave whatever the conversion gave.
TEST(ArithmeticTypes, ToIntAndToDoubleOfWhatTheyCannotReadIsNull) {
  SessionRow sr;
  ToIntVisitor to_int(nullptr, sr.row.get());
  ToDoubleVisitor to_double(nullptr, sr.row.get());
  auto ti = [&](RDFTTYPE v) { return or_threw(to_int, v); };
  auto td = [&](RDFTTYPE v) { return or_threw(to_double, v); };
  auto null = [](RDFTTYPE r) { return r.which() == rdf::rdf_null_t; };
  std::vector<RDFTTYPE> const unreadable = {
    rdf::LString(""), rdf::LString("  "), rdf::LString("x"), rdf::LString("12abc"),
    rdf::Null(), rdf::NamedResource("r"), rdf::BlankNode(3), rdf::LDate(rdf::date(2025, 1, 1)),
    rdf::LDatetime(rdf::parse_datetime("2025-01-01T10:00:00")),
  };
  for(auto const& v: unreadable) {
    EXPECT_TRUE(null(ti(v))) << "to_int " << v;
    EXPECT_TRUE(null(td(v))) << "to_double " << v;
  }
  // text that reads as a number but not as an int32
  EXPECT_TRUE(null(ti(rdf::LString("2.5"))));
  EXPECT_TRUE(null(ti(rdf::LString("1e3"))));
  EXPECT_TRUE(null(ti(rdf::LString("99999999999"))));
  EXPECT_TRUE(null(ti(rdf::LDouble(std::numeric_limits<double>::quiet_NaN()))));
  // what they read is as it was
  EXPECT_EQ(ti(rdf::LString(" 42 ")), RDFTTYPE(rdf::LInt32(42)));
  EXPECT_EQ(ti(rdf::LString("-7")), RDFTTYPE(rdf::LInt32(-7)));
  EXPECT_EQ(td(rdf::LString(" 2.5 ")), RDFTTYPE(rdf::LDouble(2.5)));
  EXPECT_EQ(td(rdf::LString("1e3")), RDFTTYPE(rdf::LDouble(1000)));
  EXPECT_EQ(td(rdf::LInt32(3)), RDFTTYPE(rdf::LDouble(3)));
}

TEST(ArithmeticTypes, IntegersKeepTheLeftOperandsType) {
  AddVisitor add(nullptr, nullptr);
  EXPECT_EQ(boost::apply_visitor(add, RDFTTYPE(rdf::LInt32(2)), RDFTTYPE(rdf::LInt64(3))), RDFTTYPE(rdf::LInt32(5)));
  EXPECT_EQ(boost::apply_visitor(add, RDFTTYPE(rdf::LInt64(2)), RDFTTYPE(rdf::LInt32(3))), RDFTTYPE(rdf::LInt64(5)));
  DivVisitor div(nullptr, nullptr);
  EXPECT_EQ(boost::apply_visitor(div, RDFTTYPE(rdf::LInt32(7)), RDFTTYPE(rdf::LInt32(2))), RDFTTYPE(rdf::LInt32(3)));
}

// Q-68: == != < <= > >= over numbers. Integers with integers by value (std::cmp_*); any
// double operand compares as two doubles, by rdf::is_eq, is_lt, ... (15 significant
// digits); a NaN or an infinity is never equal or ordered. The table is the comparisons'
// specification; it was shared with the Go engine, the same rows in both, until its retirement.
struct ComparisonCase { RDFTTYPE lhs, rhs; char const* expected; };   // == != < <= > >=

TEST(Comparisons, TheSharedTable) {
  double const nan = std::numeric_limits<double>::quiet_NaN();
  double const inf = std::numeric_limits<double>::infinity();
  std::vector<ComparisonCase> const cases = {
    {rdf::LInt32(5), rdf::LDouble(5), "TFFTFT"},
    {rdf::LInt32(5), rdf::LDouble(5.5), "FTTTFF"},
    {rdf::LDouble(5.5), rdf::LInt32(5), "FTFFTT"},
    {rdf::LInt32(5), rdf::LUInt32(5), "TFFTFT"},
    {rdf::LInt32(-1), rdf::LUInt32(1), "FTTTFF"},
    {rdf::LUInt64(std::numeric_limits<uint64_t>::max()), rdf::LInt64(std::numeric_limits<int64_t>::max()), "FTFFTT"},
    {rdf::LInt32(3), rdf::LInt32(7), "FTTTFF"},
    {rdf::LDouble(0.30000000000000004), rdf::LDouble(0.3), "TFFTFT"},
    {rdf::LDouble(1), rdf::LDouble(1.000000000000001), "TFFTFT"},
    {rdf::LDouble(1), rdf::LDouble(1.000000000001), "FTTTFF"},
    {rdf::LDouble(nan), rdf::LDouble(nan), "FTFFFF"},
    {rdf::LDouble(nan), rdf::LInt32(5), "FTFFFF"},
    {rdf::LDouble(inf), rdf::LDouble(inf), "FTFFFF"},
    {rdf::LDouble(inf), rdf::LInt32(5), "FTFFFF"},
    {rdf::LInt32(5), rdf::LString("5"), "FTFFFF"},
    // a double too large for the integer's type compares, where it threw
    {rdf::LInt32(5), rdf::LDouble(1e12), "FTTTFF"},
    {rdf::LUInt64(5), rdf::LDouble(5), "TFFTFT"},
    // a tiny double is compared unrounded: rounding it to 15 digits overflowed to NaN,
    // which compared equal to anything
    {rdf::LDouble(1e-300), rdf::LInt32(5), "FTTTFF"},
    {rdf::LDouble(5e-324), rdf::LDouble(1e300), "FTTTFF"},
    {rdf::LDouble(1e-300), rdf::LInt32(-7), "FTFFTT"},
  };
  auto t = [](RDFTTYPE r) { return rdf::to_bool(r) ? 'T' : 'F'; };
  for(auto const& c: cases) {
    std::string got;
    got += t(boost::apply_visitor(EqVisitor(nullptr, nullptr), c.lhs, c.rhs));
    got += t(boost::apply_visitor(NeVisitor(nullptr, nullptr), c.lhs, c.rhs));
    got += t(boost::apply_visitor(LtVisitor(nullptr, nullptr), c.lhs, c.rhs));
    got += t(boost::apply_visitor(LeVisitor(nullptr, nullptr), c.lhs, c.rhs));
    got += t(boost::apply_visitor(GtVisitor(nullptr, nullptr), c.lhs, c.rhs));
    got += t(boost::apply_visitor(GeVisitor(nullptr, nullptr), c.lhs, c.rhs));
    EXPECT_EQ(got, c.expected) << c.lhs << " vs " << c.rhs;
  }
}

// Q-69: division by zero. An integer divided by an integer zero is null; it raised
// SIGFPE until 2026-10, which killed the process. A division that involves a double
// follows IEEE: +inf, -inf, and NaN for 0/0. The table is division's specification: seven
// rows shared with the Go engine until its retirement, then two mixed integer
// types and one nonzero division.
TEST(ArithmeticTypes, DivisionByZero) {
  DivVisitor div(nullptr, nullptr);
  auto q = [&](RDFTTYPE a, RDFTTYPE b) { return boost::apply_visitor(div, a, b); };
  EXPECT_EQ(q(rdf::LInt32(5), rdf::LInt32(0)).which(), rdf::rdf_null_t);
  EXPECT_EQ(q(rdf::LUInt32(5), rdf::LUInt32(0)).which(), rdf::rdf_null_t);
  EXPECT_EQ(q(rdf::LInt32(0), rdf::LInt32(0)).which(), rdf::rdf_null_t);
  double const inf = std::numeric_limits<double>::infinity();
  EXPECT_EQ(q(rdf::LDouble(5.0), rdf::LDouble(0.0)), RDFTTYPE(rdf::LDouble(inf)));
  EXPECT_EQ(q(rdf::LDouble(-5.0), rdf::LDouble(0.0)), RDFTTYPE(rdf::LDouble(-inf)));
  EXPECT_TRUE(std::isnan(boost::get<rdf::LDouble>(q(rdf::LDouble(0.0), rdf::LDouble(0.0))).data));
  EXPECT_EQ(q(rdf::LInt32(5), rdf::LDouble(0.0)), RDFTTYPE(rdf::LDouble(inf)));
  EXPECT_EQ(q(rdf::LInt64(7), rdf::LUInt32(0)).which(), rdf::rdf_null_t);
  EXPECT_EQ(q(rdf::LUInt64(7), rdf::LInt32(0)).which(), rdf::rdf_null_t);
  // and an integer divided by a nonzero integer is still integer division
  EXPECT_EQ(q(rdf::LInt32(7), rdf::LInt32(2)), RDFTTYPE(rdf::LInt32(3)));

  // Q-85, I-126: the minimum signed integer divided by -1 is null, as by zero. Its
  // quotient does not fit the type, and it raised SIGFPE, which killed the process, for
  // int32 and int64 and through the mixed integer types
  auto const i32min = std::numeric_limits<int32_t>::min();
  auto const i64min = std::numeric_limits<int64_t>::min();
  EXPECT_EQ(q(rdf::LInt32(i32min), rdf::LInt32(-1)).which(), rdf::rdf_null_t);
  EXPECT_EQ(q(rdf::LInt64(i64min), rdf::LInt64(-1)).which(), rdf::rdf_null_t);
  EXPECT_EQ(q(rdf::LInt32(i32min), rdf::LInt64(-1)).which(), rdf::rdf_null_t);
  EXPECT_EQ(q(rdf::LInt64(i64min), rdf::LInt32(-1)).which(), rdf::rdf_null_t);
  // and the neighbours divide
  EXPECT_EQ(q(rdf::LInt32(i32min), rdf::LInt32(1)), RDFTTYPE(rdf::LInt32(i32min)));
  EXPECT_EQ(q(rdf::LInt32(i32min + 1), rdf::LInt32(-1)), RDFTTYPE(rdf::LInt32(-(i32min + 1))));
  EXPECT_EQ(q(rdf::LInt32(-7), rdf::LInt32(-1)), RDFTTYPE(rdf::LInt32(7)));
  EXPECT_EQ(q(rdf::LInt64(i64min), rdf::LInt64(2)), RDFTTYPE(rdf::LInt64(i64min / 2)));
}

// Q-85, I-104: a null lookup key is null, and the session goes on; in a session's rule
// it threw. The table is not read: a null key looks nothing up.
// and so is a blank-node key (BL.1b), which threw likewise
TEST(Lookups, ANullKeyIsNull) {
  SessionRow sr;
  LookupVisitor lookup(nullptr, sr.row.get());
  MultiLookupVisitor multi_lookup(nullptr, sr.row.get());
  auto ap = [](auto & op, RDFTTYPE a, RDFTTYPE b) { return boost::apply_visitor(op, a, b); };
  RDFTTYPE const null_key = rdf::Null();
  EXPECT_EQ(ap(lookup, RDFTTYPE(rdf::NamedResource("acme:ProcedureLookup")), null_key).which(), rdf::rdf_null_t);
  EXPECT_EQ(ap(lookup, RDFTTYPE(rdf::LString("acme:ProcedureLookup")), null_key).which(), rdf::rdf_null_t);
  EXPECT_EQ(ap(multi_lookup, RDFTTYPE(rdf::NamedResource("acme:ProcedureLookup")), null_key).which(), rdf::rdf_null_t);
  EXPECT_EQ(ap(multi_lookup, RDFTTYPE(rdf::LString("acme:ProcedureLookup")), null_key).which(), rdf::rdf_null_t);
  RDFTTYPE const bnode_key = rdf::BlankNode(3);
  EXPECT_EQ(ap(lookup, RDFTTYPE(rdf::NamedResource("acme:ProcedureLookup")), bnode_key).which(), rdf::rdf_null_t);
  EXPECT_EQ(ap(lookup, RDFTTYPE(rdf::LString("acme:ProcedureLookup")), bnode_key).which(), rdf::rdf_null_t);
  EXPECT_EQ(ap(multi_lookup, RDFTTYPE(rdf::NamedResource("acme:ProcedureLookup")), bnode_key).which(), rdf::rdf_null_t);
  EXPECT_EQ(ap(multi_lookup, RDFTTYPE(rdf::LString("acme:ProcedureLookup")), bnode_key).which(), rdf::rdf_null_t);
}

// Q-93: lookup and multi_lookup with a key or a table of a type they do not take are null,
// in a session's rule as with no row, and look nothing up: a double, a date or a datetime
// key, and a table named by anything but a resource or text. Each threw in a session's
// rule (I-184). The rows need no lookup helper, since nothing is looked up.
TEST(Lookups, AKeyOrTableOfAnotherTypeIsNull) {
  SessionRow sr;
  LookupVisitor lookup(nullptr, sr.row.get());
  MultiLookupVisitor multi_lookup(nullptr, sr.row.get());
  auto ap = [](auto & op, RDFTTYPE a, RDFTTYPE b) { return or_threw(op, a, b); };
  auto null = [](RDFTTYPE r) { return r.which() == rdf::rdf_null_t; };
  std::vector<RDFTTYPE> const tables = {rdf::NamedResource("acme:ProcedureLookup"), rdf::LString("acme:ProcedureLookup")};
  std::vector<RDFTTYPE> const keys = {
    rdf::LDouble(2.5), rdf::LDate(rdf::date(2025, 1, 1)), rdf::LDatetime(rdf::parse_datetime("2025-01-01T10:00:00")),
  };
  for(auto const& t: tables) {
    for(auto const& k: keys) {
      EXPECT_TRUE(null(ap(lookup, t, k))) << "lookup " << t << " " << k;
      EXPECT_TRUE(null(ap(multi_lookup, t, k))) << "multi_lookup " << t << " " << k;
    }
  }
  std::vector<RDFTTYPE> const not_tables = {
    rdf::LInt32(5), rdf::LDouble(2.5), rdf::Null(), rdf::BlankNode(3), rdf::LDate(rdf::date(2025, 1, 1)),
  };
  for(auto const& t: not_tables) {
    EXPECT_TRUE(null(ap(lookup, t, RDFTTYPE(rdf::LString("k"))))) << "lookup " << t;
    EXPECT_TRUE(null(ap(multi_lookup, t, RDFTTYPE(rdf::LInt32(7))))) << "multi_lookup " << t;
  }
}

// char_at at the edges, evaluated as in a session's rule. The index counts characters
// (code points), as substring_of's positions and length_of do, and a byte that does not
// start a valid UTF-8 sequence is one character. An index outside the text, past the end
// or negative, is null,
// as it always was (substring_of gives "" there; char_at keeps null because a rule can
// tell the two apart). An index that is not an int, or a lhs that is not text, throws.
TEST(CharAt, TheEdges) {
  SessionRow sr;
  CharAtVisitor op(nullptr, sr.row.get());
  auto T = [](std::string v) { return RDFTTYPE(rdf::LString(std::move(v))); };
  auto I = [](std::int32_t v) { return RDFTTYPE(rdf::LInt32(v)); };
  auto const null = RDFTTYPE(rdf::Null());
  struct Row { std::string text; std::int32_t index; RDFTTYPE want; };
  std::vector<Row> const rows = {
    // within the text
    {"abc", 0, T("a")},
    {"abc", 2, T("c")},
    {" ", 0, T(" ")},
    // at or past the end
    {"abc", 3, null},
    {"abc", 10, null},
    {"", 0, null},
    {"abc", std::numeric_limits<std::int32_t>::max(), null},
    // negative
    {"abc", -1, null},
    {"", -1, null},
    {"abc", std::numeric_limits<std::int32_t>::min(), null},
    // the index counts characters (code points): "é" is the two bytes C3 A9 in UTF-8,
    // "€" the three E2 82 AC, "😀" the four F0 9F 98 80
    {"\xc3\xa9t\xc3\xa9", 0, T("\xc3\xa9")},
    {"\xc3\xa9t\xc3\xa9", 1, T("t")},
    {"\xc3\xa9t\xc3\xa9", 2, T("\xc3\xa9")},
    {"\xc3\xa9t\xc3\xa9", 3, null},
    {"\xc3\xa9t\xc3\xa9", 4, null},
    {"\xe2\x82\xac" "5", 0, T("\xe2\x82\xac")},
    {"\xe2\x82\xac" "5", 1, T("5")},
    {"\xf0\x9f\x98\x80", 0, T("\xf0\x9f\x98\x80")},
    {"\xf0\x9f\x98\x80", 1, null},
    // no normalisation: "e" and a combining acute (CC 81) are two characters
    {"e\xcc\x81", 1, T("\xcc\x81")},
    // a byte that does not start a valid sequence is one character, kept as it is
    {"a\xff" "b", 1, T("\xff")},
    {"a\xff" "b", 2, T("b")},
    {"\xa9", 0, T("\xa9")},
    {"a\xf0\x9f\x98", 1, T("\xf0")},                  // a truncated 4-byte sequence
    {"a\xf0\x9f\x98", 3, T("\x98")},
    {"a\xf0\x9f\x98", 4, null},
  };
  for(auto const& r: rows) {
    EXPECT_EQ(or_threw(op, T(r.text), I(r.index)), r.want) << "index " << r.index << ", text \"" << r.text << "\"";
  }
  // an index that is not an int, or a lhs that is not text, is the catch-all: it throws in
  // a session's rule, and is null with no row
  CharAtVisitor no_row(nullptr, nullptr);
  std::vector<std::pair<RDFTTYPE, RDFTTYPE>> const not_taken = {
    {T("abc"), RDFTTYPE(rdf::LInt64(1))}, {T("abc"), RDFTTYPE(rdf::LUInt32(1))},
    {T("abc"), RDFTTYPE(rdf::LDouble(1.0))}, {T("abc"), T("1")}, {T("abc"), null},
    {I(123), I(1)}, {null, I(0)},
  };
  for(auto const& [a, b]: not_taken) {
    auto res = or_threw(op, a, b);
    EXPECT_TRUE(res.which() == rdf::rdf_literal_string_t and
                boost::get<rdf::LString>(res).data.rfind("threw: ", 0) == 0) << a << " char_at " << b << ": " << res;
    EXPECT_EQ(boost::apply_visitor(no_row, a, b), null) << a << " char_at " << b;
  }
}

// length_of counts characters as TextUnits does, so that it agrees with substring_of and
// char_at: a code point of any length in UTF-8 is one, a byte that does not start a valid
// sequence is one, and there is no normalisation. Until 2026-11-14 it counted bytes.
TEST(LengthOf, CountsCharacters) {
  SessionRow sr;
  LengthOfVisitor op(nullptr, sr.row.get());
  std::vector<std::pair<std::string, std::int32_t>> const rows = {
    {"", 0},
    {"abc", 3},
    {"\xc3\xa9t\xc3\xa9", 3},                // "été", 5 bytes
    {"5\xe2\x82\xac", 2},                    // "5€", 4 bytes
    {"\xf0\x9f\x98\x80!", 2},                // "😀!", 5 bytes
    {"e\xcc\x81", 2},                        // "e" and a combining acute: no normalisation
    {"a\xff" "b", 3},                        // an invalid byte
    {"\x80\x80", 2},                         // stray continuation bytes
    {"a\xe2\x82", 3},                        // a truncated sequence: each byte
    {"\xed\xa0\x80", 3},                     // a surrogate: each byte
  };
  for(auto const& [text, want]: rows) {
    EXPECT_EQ(or_threw(op, RDFTTYPE(rdf::LString(text))), RDFTTYPE(rdf::LInt32(want))) << "\"" << text << "\"";
  }
}

// The three operators together, in the shape of a rule that drops a one-letter suffix
// from a name: (?e name ?v).(?e nameLength ?n) with ?n = (length_of ?v), and the filter
// [((?v char_at (?n - 2)) == " ")]. On a name whose last letter is not ASCII, counting
// bytes put ?n - 2 inside that letter and the suffix was missed; counting characters
// finds the space. And a cut of the first ?n - 2 characters keeps the name whole.
TEST(TextUnits, ANameSuffixAfterANonAsciiLetter) {
  SessionRow sr;
  LengthOfVisitor length_of(nullptr, sr.row.get());
  SubsVisitor minus(nullptr, sr.row.get());
  CharAtVisitor char_at(nullptr, sr.row.get());
  EqVisitor eq(nullptr, sr.row.get());
  auto suffix_space = [&](std::string const& name) {
    auto n = or_threw(length_of, RDFTTYPE(rdf::LString(name)));
    auto at = or_threw(minus, n, RDFTTYPE(rdf::LInt32(2)));
    auto c = or_threw(char_at, RDFTTYPE(rdf::LString(name)), at);
    return rdf::to_bool(or_threw(eq, c, RDFTTYPE(rdf::LString(" "))));
  };
  EXPECT_TRUE(suffix_space("Ana B"));
  EXPECT_TRUE(suffix_space("Ana \xc3\x89"));             // "Ana É"
  EXPECT_TRUE(suffix_space("Jos\xc3\xa9 \xc3\x89"));       // "José É"
  EXPECT_TRUE(suffix_space("Zo\xc3\xab \xc5\x81"));        // "Zoë Ł"
  EXPECT_FALSE(suffix_space("Jos\xc3\xa9"));              // "José": no suffix
  EXPECT_FALSE(suffix_space("A"));                        // index -1: null, not " "
}

}   // namespace
}   // namespace jets::rdf