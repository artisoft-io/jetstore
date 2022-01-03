#include <cstddef>
#include <cstdint>
#include <iostream>
#include <string>
#include <memory>

#include <gtest/gtest.h>

#include "expr.h"
#include "jets/rdf/rdf_types.h"
#include "jets/rete/rete_types.h"

namespace jets::rete {
namespace {

TEST(ExprTest, ExprCstTest) {
    auto xtrue = create_expr_cst(rdf::RdfAstType(rdf::LInt32(1)));
    EXPECT_TRUE(rdf::to_bool(xtrue->eval(nullptr, nullptr)));

    auto xfalse = create_expr_cst(rdf::RdfAstType(rdf::LInt32(0)));
    EXPECT_FALSE(rdf::to_bool(xfalse->eval(nullptr, nullptr)));
}

TEST(ExprTest, ExprBindedVar1Test) {
  // rdf resource manager
  rdf::RManager rmanager;
  auto r0 = rmanager.create_resource("r0");
  auto beta_row = create_beta_row(nullptr, 1);
  EXPECT_EQ(beta_row->put(0, r0), 0);

  auto xbv = create_expr_binded_var(0);
  EXPECT_TRUE( xbv->eval(nullptr, beta_row.get())==*r0 );
}

TEST(ExprTest, ExprBindedVar2Test) {
  // rdf resource manager
  rdf::RManager rmanager;
  auto r0 = rmanager.create_resource("r0");
  auto r1 = rmanager.create_resource("r1");
  auto beta_row = create_beta_row(nullptr, 2);
  EXPECT_EQ(beta_row->put(0, r0), 0);
  EXPECT_EQ(beta_row->put(1, r1), 0);

  auto xbv = create_expr_binded_var(0);
  EXPECT_TRUE( xbv->eval(nullptr, beta_row.get())==*r0 );

  xbv = create_expr_binded_var(1);
  EXPECT_TRUE( xbv->eval(nullptr, beta_row.get())==*r1 );
  EXPECT_FALSE( xbv->eval(nullptr, beta_row.get())==*r0 );
}

TEST(ExprTest, ExprConjunctionTest) {
  auto xtrue = create_expr_cst(rdf::RdfAstType(rdf::LInt32(1)));
  EXPECT_TRUE(rdf::to_bool(xtrue->eval(nullptr, nullptr)));

  ExprConjunction::data_type data;
  auto conj = create_expr_conjunction(data);
  EXPECT_FALSE(rdf::to_bool(conj->eval(nullptr, nullptr)));

  data.clear();
  data.push_back(xtrue);
  conj = create_expr_conjunction(data);
  EXPECT_TRUE(rdf::to_bool(conj->eval(nullptr, nullptr)));

  data.clear();
  data.push_back(xtrue);
  data.push_back(xtrue);
  conj = create_expr_conjunction(data);
  EXPECT_TRUE(rdf::to_bool(conj->eval(nullptr, nullptr)));

  data.clear();
  data.push_back(xtrue);
  data.push_back(xtrue);
  data.push_back(xtrue);
  conj = create_expr_conjunction(data);
  EXPECT_TRUE(rdf::to_bool(conj->eval(nullptr, nullptr)));

  auto xfalse = create_expr_cst(rdf::RdfAstType(rdf::LInt32(0)));
  data.clear();
  data.push_back(xtrue);
  data.push_back(xfalse);
  data.push_back(xtrue);
  conj = create_expr_conjunction(data);
  EXPECT_FALSE(rdf::to_bool(conj->eval(nullptr, nullptr)));
}

TEST(ExprTest, ExprDisjunctionTest) {
  auto xtrue = create_expr_cst(rdf::RdfAstType(rdf::LInt32(1)));
  EXPECT_TRUE(rdf::to_bool(xtrue->eval(nullptr, nullptr)));

  auto xfalse = create_expr_cst(rdf::RdfAstType(rdf::LInt32(0)));
  EXPECT_FALSE(rdf::to_bool(xfalse->eval(nullptr, nullptr)));

  ExprDisjunction::data_type data;
  auto disj = create_expr_disjunction(data);
  EXPECT_FALSE(rdf::to_bool(disj->eval(nullptr, nullptr)));

  data.clear();
  data.push_back(xtrue);
  disj = create_expr_disjunction(data);
  EXPECT_TRUE(rdf::to_bool(disj->eval(nullptr, nullptr)));

  data.clear();
  data.push_back(xtrue);
  data.push_back(xtrue);
  disj = create_expr_disjunction(data);
  EXPECT_TRUE(rdf::to_bool(disj->eval(nullptr, nullptr)));

  data.clear();
  data.push_back(xfalse);
  disj = create_expr_disjunction(data);
  EXPECT_FALSE(rdf::to_bool(disj->eval(nullptr, nullptr)));

  data.clear();
  data.push_back(xfalse);
  data.push_back(xtrue);
  disj = create_expr_disjunction(data);
  EXPECT_TRUE(rdf::to_bool(disj->eval(nullptr, nullptr)));

  data.clear();
  data.push_back(xfalse);
  data.push_back(xfalse);
  data.push_back(xtrue);
  disj = create_expr_disjunction(data);
  EXPECT_TRUE(rdf::to_bool(disj->eval(nullptr, nullptr)));

  data.clear();
  data.push_back(xfalse);
  data.push_back(xtrue);
  data.push_back(xfalse);
  data.push_back(xtrue);
  disj = create_expr_disjunction(data);
  EXPECT_TRUE(rdf::to_bool(disj->eval(nullptr, nullptr)));

  data.clear();
  data.push_back(xfalse);
  data.push_back(xfalse);
  data.push_back(xfalse);
  disj = create_expr_disjunction(data);
  EXPECT_FALSE(rdf::to_bool(disj->eval(nullptr, nullptr)));
}

TEST(ExprTest, AddVisitor1Test) {
  auto x1 = create_expr_cst(rdf::RdfAstType(rdf::LInt32(1)));
  auto x2 = create_expr_cst(rdf::RdfAstType(rdf::LInt32(2)));
  auto op = create_expr_binary_operator<AddVisitor>(x1, x2);

  rdf::RManager rmanager;
  auto v1 = rmanager.create_literal<int32_t>(1);
  auto v4 = rmanager.create_literal<int32_t>(4);
  EXPECT_TRUE(op->eval(nullptr, nullptr)==*v1 );
  EXPECT_FALSE(op->eval(nullptr, nullptr)==*v4 );
}

TEST(ExprTest, AddVisitor2Test) {
  auto x1 = create_expr_cst(rdf::RdfAstType(rdf::LInt32(1)));
  auto x2 = create_expr_cst(rdf::RdfAstType(rdf::NamedResource("r2")));
  auto op = create_expr_binary_operator<AddVisitor>(x1, x2);

  rdf::RManager rmanager;
  auto vnull = rmanager.get_null();
  auto v4 = rmanager.create_literal<int32_t>(4);
  EXPECT_TRUE(op->eval(nullptr, nullptr)==*vnull );
  EXPECT_FALSE(op->eval(nullptr, nullptr)==*v4 );
}

}   // namespace
}   // namespace jets::rete