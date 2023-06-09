#include <cmath>
#include <cstddef>
#include <cstdint>
#include <iostream>
#include <string>
#include <memory>

#include <gtest/gtest.h>

#include "../rdf/rdf_types.h"
#include "../rete/rete_types.h"
#include "../rete/rete_types_impl.h"

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
  auto rmanager_p = rdf::RManager::create();
  auto rmanager = *rmanager_p;
  auto r0 = rmanager.create_resource("r0");
  auto beta_row = create_beta_row(nullptr, 1);
  EXPECT_EQ(beta_row->put(0, r0), 0);

  auto xbv = create_expr_binded_var(0);
  EXPECT_TRUE( xbv->eval(nullptr, beta_row.get())==*r0 );
}

TEST(ExprTest, ExprBindedVar2Test) {
  // rdf resource manager
  auto rmanager_p = rdf::RManager::create();
  auto rmanager = *rmanager_p;
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

// ExprConjunction Test -----------------------------------------------------------------------
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

// ExprDisjunction Test -----------------------------------------------------------------------
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

// AddVisitor Test -----------------------------------------------------------------------
TEST(ExprTest, AddVisitor1Test) {
  EXPECT_EQ(create_expr_binary_operator<AddVisitor>(0,
    create_expr_cst(rdf::RdfAstType(rdf::LInt32(1))), 
    create_expr_cst(rdf::RdfAstType(rdf::LInt32(2))))->eval(nullptr, nullptr), 
    rdf::RdfAstType(rdf::LInt32(3)) );

  EXPECT_NE(create_expr_binary_operator<AddVisitor>(0,
    create_expr_cst(rdf::RdfAstType(rdf::LInt32(1))), 
    create_expr_cst(rdf::RdfAstType(rdf::LInt32(2))))->eval(nullptr, nullptr), 
    rdf::RdfAstType(rdf::LInt32(4)) );
}

TEST(ExprTest, AddVisitor2Test) {
  EXPECT_EQ(create_expr_binary_operator<AddVisitor>(0,
    create_expr_cst(rdf::RdfAstType(rdf::LInt32(1))), 
    create_expr_cst(rdf::RdfAstType(rdf::NamedResource("r2"))))->eval(nullptr, nullptr), 
    rdf::Null() );
}

TEST(ExprTest, AddVisitor3Test) {
  EXPECT_EQ(create_expr_binary_operator<AddVisitor>(0,
    create_expr_cst(rdf::RdfAstType(rdf::LInt32(1))), 
    create_expr_cst(rdf::RdfAstType(rdf::LDouble(1.9))))->eval(nullptr, nullptr), 
    rdf::RdfAstType(rdf::LInt32(2)) );

  EXPECT_EQ(create_expr_binary_operator<AddVisitor>(0,
    create_expr_cst(rdf::RdfAstType(rdf::LString("Hello "))), 
    create_expr_cst(rdf::RdfAstType(rdf::LString("World"))))->eval(nullptr, nullptr), 
    rdf::RdfAstType(rdf::LString("Hello World")) );

  auto x1 = create_expr_cst(rdf::RdfAstType(rdf::LDouble(1.9)));
  auto x2 = create_expr_cst(rdf::RdfAstType(rdf::LInt32(1)));
  auto op = create_expr_binary_operator<AddVisitor>(0, x1, x2);
  EXPECT_FLOAT_EQ(boost::get<rdf::LDouble>(op->eval(nullptr, nullptr)).data , 2.9 );
}

// EqVisitor Test -----------------------------------------------------------------------
TEST(ExprTest, EqVisitor1Test) {
  EXPECT_EQ(create_expr_binary_operator<EqVisitor>(0, 
    create_expr_cst(rdf::RdfAstType(rdf::RDFNull())), 
    create_expr_cst(rdf::RdfAstType(rdf::RDFNull())))->eval(nullptr, nullptr), 
    rdf::True() );
    
  EXPECT_EQ(create_expr_binary_operator<EqVisitor>(0, 
    create_expr_cst(rdf::RdfAstType(rdf::RDFNull())), 
    create_expr_cst(rdf::RdfAstType(rdf::BlankNode(22))))->eval(nullptr, nullptr), 
    rdf::False() );
    
  EXPECT_EQ(create_expr_binary_operator<EqVisitor>(0, 
    create_expr_cst(rdf::RdfAstType(rdf::RDFNull())), 
    create_expr_cst(rdf::RdfAstType(rdf::LInt32(22))))->eval(nullptr, nullptr), 
    rdf::False() );
    
  EXPECT_EQ(create_expr_binary_operator<EqVisitor>(0, 
    create_expr_cst(rdf::RdfAstType(rdf::RDFNull())), 
    create_expr_cst(rdf::RdfAstType(rdf::LDouble(2.2))))->eval(nullptr, nullptr), 
    rdf::False() );
}

TEST(ExprTest, EqVisitor2Test) {
  EXPECT_EQ(create_expr_binary_operator<EqVisitor>(0, 
    create_expr_cst(rdf::RdfAstType(rdf::BlankNode(22))), 
    create_expr_cst(rdf::RdfAstType(rdf::RDFNull())))->eval(nullptr, nullptr), 
    rdf::False() );
    
  EXPECT_EQ(create_expr_binary_operator<EqVisitor>(0, 
    create_expr_cst(rdf::RdfAstType(rdf::BlankNode(22))), 
    create_expr_cst(rdf::RdfAstType(rdf::BlankNode(22))))->eval(nullptr, nullptr), 
    rdf::True() );
    
  EXPECT_EQ(create_expr_binary_operator<EqVisitor>(0, 
    create_expr_cst(rdf::RdfAstType(rdf::BlankNode(22))), 
    create_expr_cst(rdf::RdfAstType(rdf::BlankNode(0))))->eval(nullptr, nullptr), 
    rdf::False() );
    
  EXPECT_EQ(create_expr_binary_operator<EqVisitor>(0, 
    create_expr_cst(rdf::RdfAstType(rdf::BlankNode(22))), 
    create_expr_cst(rdf::RdfAstType(rdf::LInt32(22))))->eval(nullptr, nullptr), 
    rdf::False() );
    
  EXPECT_EQ(create_expr_binary_operator<EqVisitor>(0, 
    create_expr_cst(rdf::RdfAstType(rdf::BlankNode(22))), 
    create_expr_cst(rdf::RdfAstType(rdf::LDouble(2.2))))->eval(nullptr, nullptr), 
    rdf::False() );
}

TEST(ExprTest, EqVisitor3Test) {
  EXPECT_EQ(create_expr_binary_operator<EqVisitor>(0, 
    create_expr_cst(rdf::RdfAstType(rdf::NamedResource("r0"))), 
    create_expr_cst(rdf::RdfAstType(rdf::RDFNull())))->eval(nullptr, nullptr), 
    rdf::False() );
    
  EXPECT_EQ(create_expr_binary_operator<EqVisitor>(0, 
    create_expr_cst(rdf::RdfAstType(rdf::NamedResource("r0"))), 
    create_expr_cst(rdf::RdfAstType(rdf::BlankNode(22))))->eval(nullptr, nullptr), 
    rdf::False() );
    
  EXPECT_EQ(create_expr_binary_operator<EqVisitor>(0, 
    create_expr_cst(rdf::RdfAstType(rdf::NamedResource("r0"))), 
    create_expr_cst(rdf::RdfAstType(rdf::NamedResource("r0"))))->eval(nullptr, nullptr), 
    rdf::True() );
    
  EXPECT_EQ(create_expr_binary_operator<EqVisitor>(0, 
    create_expr_cst(rdf::RdfAstType(rdf::NamedResource("r0"))), 
    create_expr_cst(rdf::RdfAstType(rdf::NamedResource("r1"))))->eval(nullptr, nullptr), 
    rdf::False() );
    
  EXPECT_EQ(create_expr_binary_operator<EqVisitor>(0, 
    create_expr_cst(rdf::RdfAstType(rdf::NamedResource("r0"))), 
    create_expr_cst(rdf::RdfAstType(rdf::LInt32(22))))->eval(nullptr, nullptr), 
    rdf::False() );
    
  EXPECT_EQ(create_expr_binary_operator<EqVisitor>(0, 
    create_expr_cst(rdf::RdfAstType(rdf::NamedResource("r0"))), 
    create_expr_cst(rdf::RdfAstType(rdf::LDouble(2.2))))->eval(nullptr, nullptr), 
    rdf::False() );
}

TEST(ExprTest, EqVisitor4Test) {
  EXPECT_EQ(create_expr_binary_operator<EqVisitor>(0, 
    create_expr_cst(rdf::RdfAstType(rdf::LInt32(1))), 
    create_expr_cst(rdf::RdfAstType(rdf::RDFNull())))->eval(nullptr, nullptr), 
    rdf::False() );
    
  EXPECT_EQ(create_expr_binary_operator<EqVisitor>(0, 
    create_expr_cst(rdf::RdfAstType(rdf::LInt32(1))), 
    create_expr_cst(rdf::RdfAstType(rdf::BlankNode(22))))->eval(nullptr, nullptr), 
    rdf::False() );
    
  EXPECT_EQ(create_expr_binary_operator<EqVisitor>(0, 
    create_expr_cst(rdf::RdfAstType(rdf::LInt32(1))), 
    create_expr_cst(rdf::RdfAstType(rdf::NamedResource("r0"))))->eval(nullptr, nullptr), 
    rdf::False() );
    
  EXPECT_EQ(create_expr_binary_operator<EqVisitor>(0, 
    create_expr_cst(rdf::RdfAstType(rdf::LInt32(1))), 
    create_expr_cst(rdf::RdfAstType(rdf::LInt32(1))))->eval(nullptr, nullptr), 
    rdf::True() );
    
  EXPECT_EQ(create_expr_binary_operator<EqVisitor>(0, 
    create_expr_cst(rdf::RdfAstType(rdf::LInt32(1))), 
    create_expr_cst(rdf::RdfAstType(rdf::LInt32(22))))->eval(nullptr, nullptr), 
    rdf::False() );
    
  EXPECT_EQ(create_expr_binary_operator<EqVisitor>(0, 
    create_expr_cst(rdf::RdfAstType(rdf::LInt32(-1))), 
    create_expr_cst(rdf::RdfAstType(rdf::LUInt32(1))))->eval(nullptr, nullptr), 
    rdf::False() );
    
  EXPECT_EQ(create_expr_binary_operator<EqVisitor>(0, 
    create_expr_cst(rdf::RdfAstType(rdf::LInt32(1))), 
    create_expr_cst(rdf::RdfAstType(rdf::LUInt32(1))))->eval(nullptr, nullptr), 
    rdf::True() );
    
  EXPECT_EQ(create_expr_binary_operator<EqVisitor>(0, 
    create_expr_cst(rdf::RdfAstType(rdf::LInt32(1))), 
    create_expr_cst(rdf::RdfAstType(rdf::LUInt64(1))))->eval(nullptr, nullptr), 
    rdf::True() );
    
  EXPECT_EQ(create_expr_binary_operator<EqVisitor>(0, 
    create_expr_cst(rdf::RdfAstType(rdf::LInt32(1))), 
    create_expr_cst(rdf::RdfAstType(rdf::LDouble(1.1))))->eval(nullptr, nullptr), 
    rdf::True() );
    
  EXPECT_EQ(create_expr_binary_operator<EqVisitor>(0, 
    create_expr_cst(rdf::RdfAstType(rdf::LInt32(-1))), 
    create_expr_cst(rdf::RdfAstType(rdf::LDouble(1.1))))->eval(nullptr, nullptr), 
    rdf::False() );
}

TEST(ExprTest, EqVisitor5Test) {
  EXPECT_EQ(create_expr_binary_operator<EqVisitor>(0, 
    create_expr_cst(rdf::RdfAstType(rdf::LDouble(1))), 
    create_expr_cst(rdf::RdfAstType(rdf::RDFNull())))->eval(nullptr, nullptr), 
    rdf::False() );
    
  EXPECT_EQ(create_expr_binary_operator<EqVisitor>(0, 
    create_expr_cst(rdf::RdfAstType(rdf::LDouble(1))), 
    create_expr_cst(rdf::RdfAstType(rdf::BlankNode(22))))->eval(nullptr, nullptr), 
    rdf::False() );
    
  EXPECT_EQ(create_expr_binary_operator<EqVisitor>(0, 
    create_expr_cst(rdf::RdfAstType(rdf::LDouble(1))), 
    create_expr_cst(rdf::RdfAstType(rdf::NamedResource("r0"))))->eval(nullptr, nullptr), 
    rdf::False() );
    
  EXPECT_EQ(create_expr_binary_operator<EqVisitor>(0, 
    create_expr_cst(rdf::RdfAstType(rdf::LDouble(1))), 
    create_expr_cst(rdf::RdfAstType(rdf::LInt32(1))))->eval(nullptr, nullptr), 
    rdf::True() );
    
  EXPECT_EQ(create_expr_binary_operator<EqVisitor>(0, 
    create_expr_cst(rdf::RdfAstType(rdf::LDouble(1.1))), 
    create_expr_cst(rdf::RdfAstType(rdf::LInt32(1))))->eval(nullptr, nullptr), 
    rdf::False() );
    
  EXPECT_EQ(create_expr_binary_operator<EqVisitor>(0, 
    create_expr_cst(rdf::RdfAstType(rdf::LDouble(1))), 
    create_expr_cst(rdf::RdfAstType(rdf::LUInt32(1))))->eval(nullptr, nullptr), 
    rdf::True() );
    
  EXPECT_EQ(create_expr_binary_operator<EqVisitor>(0, 
    create_expr_cst(rdf::RdfAstType(rdf::LDouble(-1))), 
    create_expr_cst(rdf::RdfAstType(rdf::LInt32(-1))))->eval(nullptr, nullptr), 
    rdf::True() );
    
  EXPECT_EQ(create_expr_binary_operator<EqVisitor>(0, 
    create_expr_cst(rdf::RdfAstType(rdf::LDouble(1))), 
    create_expr_cst(rdf::RdfAstType(rdf::LUInt64(1))))->eval(nullptr, nullptr), 
    rdf::True() );
    
  EXPECT_EQ(create_expr_binary_operator<EqVisitor>(0, 
    create_expr_cst(rdf::RdfAstType(rdf::LDouble(1.1/3))), 
    create_expr_cst(rdf::RdfAstType(rdf::LDouble(1.1/3))))->eval(nullptr, nullptr), 
    rdf::True() );
    
  EXPECT_EQ(create_expr_binary_operator<EqVisitor>(0, 
    create_expr_cst(rdf::RdfAstType(rdf::LDouble(2.0/3.0))), 
    create_expr_cst(rdf::RdfAstType(rdf::LDouble(1.0/3+1.0/3))))->eval(nullptr, nullptr), 
    rdf::True() );

  EXPECT_EQ(create_expr_binary_operator<EqVisitor>(0, 
    create_expr_cst(rdf::RdfAstType(rdf::LDouble(1))), 
    create_expr_cst(rdf::RdfAstType(rdf::LDouble(1.1))))->eval(nullptr, nullptr), 
    rdf::False() );
    
  EXPECT_EQ(create_expr_binary_operator<EqVisitor>(0, 
    create_expr_cst(rdf::RdfAstType(rdf::LDouble(NAN))), 
    create_expr_cst(rdf::RdfAstType(rdf::LDouble(1.1))))->eval(nullptr, nullptr), 
    rdf::False() );
    
  EXPECT_EQ(create_expr_binary_operator<EqVisitor>(0, 
    create_expr_cst(rdf::RdfAstType(rdf::LDouble(NAN))), 
    create_expr_cst(rdf::RdfAstType(rdf::LDouble(NAN))))->eval(nullptr, nullptr), 
    rdf::False() );
    
  EXPECT_EQ(create_expr_binary_operator<EqVisitor>(0, 
    create_expr_cst(rdf::RdfAstType(rdf::LDouble(INFINITY))), 
    create_expr_cst(rdf::RdfAstType(rdf::LDouble(1.1))))->eval(nullptr, nullptr), 
    rdf::False() );
    
  EXPECT_EQ(create_expr_binary_operator<EqVisitor>(0, 
    create_expr_cst(rdf::RdfAstType(rdf::LDouble(INFINITY))), 
    create_expr_cst(rdf::RdfAstType(rdf::LDouble(NAN))))->eval(nullptr, nullptr), 
    rdf::False() );
    
  EXPECT_EQ(create_expr_binary_operator<EqVisitor>(0, 
    create_expr_cst(rdf::RdfAstType(rdf::LDouble(INFINITY))), 
    create_expr_cst(rdf::RdfAstType(rdf::LDouble(INFINITY))))->eval(nullptr, nullptr), 
    rdf::False() );
}

// RegexVisitor Test -----------------------------------------------------------------------
TEST(ExprTest, RegexVisitor1Test) {
  EXPECT_EQ(create_expr_binary_operator<RegexVisitor>(0, 
    create_expr_cst(rdf::RdfAstType(rdf::NamedResource("r2"))),
    create_expr_cst(rdf::RdfAstType(rdf::LInt32(1)))
    )->eval(nullptr, nullptr), 
    rdf::Null() );
    
  EXPECT_EQ(create_expr_binary_operator<RegexVisitor>(0, 
    create_expr_cst(rdf::RdfAstType(rdf::BlankNode(22))),
    create_expr_cst(rdf::RdfAstType(rdf::RDFNull()))
    )->eval(nullptr, nullptr), 
    rdf::Null() );
    
  EXPECT_EQ(create_expr_binary_operator<RegexVisitor>(0, 
      create_expr_cst(rdf::RdfAstType(rdf::LString("Hello World"))),
      create_expr_cst(rdf::RdfAstType(rdf::LString("(Hello)")))
    )->eval(nullptr, nullptr), 
    rdf::RdfAstType(rdf::LString("Hello")) );
    
  EXPECT_EQ(create_expr_binary_operator<RegexVisitor>(0, 
    create_expr_cst(rdf::RdfAstType(rdf::LString("1 2 123 4"))),
    create_expr_cst(rdf::RdfAstType(rdf::LString("(\\d\\d\\d)")))
    )->eval(nullptr, nullptr), 
    rdf::RdfAstType(rdf::LString("123")) );
    
  EXPECT_EQ(create_expr_binary_operator<RegexVisitor>(0, 
    create_expr_cst(rdf::RdfAstType(rdf::LString("John Smith"))),
    create_expr_cst(rdf::RdfAstType(rdf::LString("\\s*(\\w*)\\s*")))
    )->eval(nullptr, nullptr), 
    rdf::RdfAstType(rdf::LString("John")) );
    
  EXPECT_EQ(create_expr_binary_operator<RegexVisitor>(0, 
    create_expr_cst(rdf::RdfAstType(rdf::LString("John Smith"))),
    create_expr_cst(rdf::RdfAstType(rdf::LString("\\s*\\w*\\s*(\\w*)\\s*")))
    )->eval(nullptr, nullptr), 
    rdf::RdfAstType(rdf::LString("Smith")) );
}

// To_upperVisitor Test -----------------------------------------------------------------------
TEST(ExprTest, to_upperVisitor1Test) {
  EXPECT_EQ(create_expr_unary_operator<To_upperVisitor>(0, 
    create_expr_cst(rdf::RdfAstType(rdf::NamedResource("r2"))))->eval(nullptr, nullptr), 
    rdf::Null() );
    
  EXPECT_EQ(create_expr_unary_operator<To_upperVisitor>(0, 
    create_expr_cst(rdf::RdfAstType(rdf::BlankNode(22))))->eval(nullptr, nullptr), 
    rdf::Null() );
    
  BetaRow br;
  EXPECT_THROW(create_expr_unary_operator<To_upperVisitor>(0, 
    create_expr_cst(rdf::RdfAstType(rdf::BlankNode(22))))->eval(nullptr, &br), 
    rete_exception );
    
  EXPECT_EQ(create_expr_unary_operator<To_upperVisitor>(0, 
    create_expr_cst(rdf::RdfAstType(rdf::LString("Hello World"))))->eval(nullptr, nullptr), 
    rdf::RdfAstType(rdf::LString("HELLO WORLD")) );
    
  EXPECT_EQ(create_expr_unary_operator<To_upperVisitor>(0, 
    create_expr_cst(rdf::RdfAstType(rdf::LString("1 2xWc 123 4"))))->eval(nullptr, nullptr), 
    rdf::RdfAstType(rdf::LString("1 2XWC 123 4")) );
}

// To_lowerVisitor Test -----------------------------------------------------------------------
TEST(ExprTest, ToLowerVisitor1Test) {
  BetaRow br;
  EXPECT_THROW(create_expr_unary_operator<To_lowerVisitor>(0, 
    create_expr_cst(rdf::RdfAstType(rdf::NamedResource("r2"))))->eval(nullptr, &br), 
    rete_exception );
    
  EXPECT_THROW(create_expr_unary_operator<To_lowerVisitor>(0, 
    create_expr_cst(rdf::RdfAstType(rdf::BlankNode(22))))->eval(nullptr, &br), 
    rete_exception );
    
  EXPECT_EQ(create_expr_unary_operator<To_lowerVisitor>(0, 
    create_expr_cst(rdf::RdfAstType(rdf::LString("Hello World"))))->eval(nullptr, nullptr), 
    rdf::RdfAstType(rdf::LString("hello world")) );
    
  EXPECT_EQ(create_expr_unary_operator<To_lowerVisitor>(0, 
    create_expr_cst(rdf::RdfAstType(rdf::LString("1 2xWc 123 4"))))->eval(nullptr, nullptr), 
    rdf::RdfAstType(rdf::LString("1 2xwc 123 4")) );
}

// TrimVisitor Test -----------------------------------------------------------------------
TEST(ExprTest, TrimVisitor1Test) {
  BetaRow br;
  EXPECT_THROW(create_expr_unary_operator<TrimVisitor>(0, 
    create_expr_cst(rdf::RdfAstType(rdf::NamedResource("r2"))))->eval(nullptr, &br), 
    rete_exception );
    
  EXPECT_THROW(create_expr_unary_operator<TrimVisitor>(0, 
    create_expr_cst(rdf::RdfAstType(rdf::BlankNode(22))))->eval(nullptr, &br), 
    rete_exception );
    
  EXPECT_EQ(create_expr_unary_operator<TrimVisitor>(0, 
    create_expr_cst(rdf::RdfAstType(rdf::LString("Hello World"))))->eval(nullptr, nullptr), 
    rdf::RdfAstType(rdf::LString("Hello World")) );
    
  EXPECT_EQ(create_expr_unary_operator<TrimVisitor>(0, 
    create_expr_cst(rdf::RdfAstType(rdf::LString(" W "))))->eval(nullptr, nullptr), 
    rdf::RdfAstType(rdf::LString("W")) );
    
  EXPECT_EQ(create_expr_unary_operator<TrimVisitor>(0, 
    create_expr_cst(rdf::RdfAstType(rdf::LString("W "))))->eval(nullptr, nullptr), 
    rdf::RdfAstType(rdf::LString("W")) );
    
  EXPECT_EQ(create_expr_unary_operator<TrimVisitor>(0, 
    create_expr_cst(rdf::RdfAstType(rdf::LString(" W"))))->eval(nullptr, nullptr), 
    rdf::RdfAstType(rdf::LString("W")) );
    
  EXPECT_EQ(create_expr_unary_operator<TrimVisitor>(0, 
    create_expr_cst(rdf::RdfAstType(rdf::LString("\t 123 \n"))))->eval(nullptr, nullptr), 
    rdf::RdfAstType(rdf::LString("123")) );
    
  EXPECT_EQ(create_expr_unary_operator<TrimVisitor>(0, 
    create_expr_cst(rdf::RdfAstType(rdf::LString("\t1 123 \n\r"))))->eval(nullptr, nullptr), 
    rdf::RdfAstType(rdf::LString("1 123")) );
}

// TrimVisitor Test -----------------------------------------------------------------------
TEST(ExprTest, Expression1Test) {
  // expression: ("(\\d)" regex ("Hello " + (1 + 2))) == "1" or
  // expression: ("(\\d)" regex (1 + 2)) == "2" or
  // expression: ("(\\d)" regex (1 + 2)) == "3"
  // The first branch should returns false
  ExprConjunction::data_type data;
  data.push_back(
    create_expr_binary_operator<EqVisitor>(0, 
      create_expr_binary_operator<RegexVisitor>(0, 
        create_expr_cst(rdf::RdfAstType(rdf::LString("(\\d)"))), 
        create_expr_binary_operator<AddVisitor>(0, 
          create_expr_cst(rdf::RdfAstType(rdf::LString("Hello "))),
          create_expr_binary_operator<AddVisitor>(0, 
            create_expr_cst(rdf::RdfAstType(rdf::LInt32(1))), 
            create_expr_cst(rdf::RdfAstType(rdf::LUInt64(2)))
          )
        )
      ), 
      create_expr_cst(rdf::RdfAstType(rdf::LString("1")))
    )
  );
  // The next would throw since Regex is used on an int argument
  // but this test return false at first term since it's a conjunction
  data.push_back(
    create_expr_binary_operator<EqVisitor>(0, 
      create_expr_binary_operator<RegexVisitor>(0, 
        create_expr_cst(rdf::RdfAstType(rdf::LString("(\\d)"))), 
        create_expr_binary_operator<AddVisitor>(0, 
          create_expr_cst(rdf::RdfAstType(rdf::LInt32(1))), 
          create_expr_cst(rdf::RdfAstType(rdf::LUInt64(2)))
        )
      ), 
      create_expr_cst(rdf::RdfAstType(rdf::LString("2")))
  ));
  data.push_back(
    create_expr_binary_operator<EqVisitor>(0, 
      create_expr_binary_operator<RegexVisitor>(0, 
        create_expr_cst(rdf::RdfAstType(rdf::LString("(\\d)"))), 
        create_expr_binary_operator<AddVisitor>(0, 
          create_expr_cst(rdf::RdfAstType(rdf::LInt32(1))), 
          create_expr_cst(rdf::RdfAstType(rdf::LUInt64(2)))
        )
      ), 
      create_expr_cst(rdf::RdfAstType(rdf::LString("3")))
  ));
  auto conj = create_expr_conjunction(data);
  EXPECT_FALSE(rdf::to_bool(conj->eval(nullptr, nullptr)));
}

TEST(ExprTest, Expression2Test) {
  // expression: ("(\\d)" regex ("Hello " + (1 + 2))) == "1" or
  // expression: ("(\\d)" regex (1 + 2)) == "2" or
  // expression: ("(\\d)" regex (1 + 2)) == "3"
  // The second branch will throw
  ExprDisjunction::data_type data;
  data.push_back(
    create_expr_binary_operator<EqVisitor>(0, 
      create_expr_binary_operator<RegexVisitor>(0, 
        create_expr_cst(rdf::RdfAstType(rdf::LString("(\\d)"))), 
        create_expr_binary_operator<AddVisitor>(0, 
          create_expr_cst(rdf::RdfAstType(rdf::LString("Hello "))),
          create_expr_binary_operator<AddVisitor>(0, 
            create_expr_cst(rdf::RdfAstType(rdf::LInt32(1))), 
            create_expr_cst(rdf::RdfAstType(rdf::LUInt64(2)))
          )
        )
      ), 
      create_expr_cst(rdf::RdfAstType(rdf::LString("1")))
    )
  );
  // The next would throw since Regex is used on an int argument
  // but this test return false at first term since it's a conjunction
  data.push_back(
    create_expr_binary_operator<EqVisitor>(0, 
      create_expr_binary_operator<RegexVisitor>(0, 
        create_expr_cst(rdf::RdfAstType(rdf::LString("(\\d)"))), 
        create_expr_binary_operator<AddVisitor>(0, 
          create_expr_cst(rdf::RdfAstType(rdf::LInt32(1))), 
          create_expr_cst(rdf::RdfAstType(rdf::LUInt64(2)))
        )
      ), 
      create_expr_cst(rdf::RdfAstType(rdf::LString("2")))
  ));
  data.push_back(
    create_expr_binary_operator<EqVisitor>(0, 
      create_expr_binary_operator<RegexVisitor>(0, 
        create_expr_cst(rdf::RdfAstType(rdf::LString("(\\d)"))), 
        create_expr_binary_operator<AddVisitor>(0, 
          create_expr_cst(rdf::RdfAstType(rdf::LInt32(1))), 
          create_expr_cst(rdf::RdfAstType(rdf::LUInt64(2)))
        )
      ), 
      create_expr_cst(rdf::RdfAstType(rdf::LString("3")))
  ));
  auto expr = create_expr_disjunction(data);
  BetaRow br;
  EXPECT_THROW(rdf::to_bool(expr->eval(nullptr, &br)), rete_exception);
}

TEST(ExprTest, Expression3Test) {
  // expression: ("(\\d)" regex ("Hello " + (1 + 2))) == 1 or
  // expression: ("(\\d)" regex ("Hello " + (1 + 2))) == "2" or
  // expression: ("(\\d)" regex ("Hello " + (1 + 2))) == "3"
  // The last branch will return true
  ExprDisjunction::data_type data;
  data.push_back(
    create_expr_binary_operator<EqVisitor>(0, 
      create_expr_binary_operator<RegexVisitor>(0, 
        create_expr_binary_operator<AddVisitor>(0, 
          create_expr_cst(rdf::RdfAstType(rdf::LString("Hello "))),
          create_expr_binary_operator<AddVisitor>(0, 
            create_expr_cst(rdf::RdfAstType(rdf::LInt32(1))), 
            create_expr_cst(rdf::RdfAstType(rdf::LUInt64(2)))
          )
        ),
        create_expr_cst(rdf::RdfAstType(rdf::LString("(\\d)")))
      ), 
      create_expr_cst(rdf::RdfAstType(rdf::LString("1")))
    )
  );
  // The next would throw since Regex is used on an int argument
  // but this test return false at first term since it's a conjunction
  data.push_back(
    create_expr_binary_operator<EqVisitor>(0, 
      create_expr_binary_operator<RegexVisitor>(0, 
        create_expr_binary_operator<AddVisitor>(0, 
          create_expr_cst(rdf::RdfAstType(rdf::LString("Hello "))),
          create_expr_binary_operator<AddVisitor>(0, 
            create_expr_cst(rdf::RdfAstType(rdf::LInt32(1))), 
            create_expr_cst(rdf::RdfAstType(rdf::LUInt64(2)))
          )
        ),
        create_expr_cst(rdf::RdfAstType(rdf::LString("(\\d)")))
      ), 
      create_expr_cst(rdf::RdfAstType(rdf::LString("2")))
  ));
  data.push_back(
    create_expr_binary_operator<EqVisitor>(0, 
      create_expr_binary_operator<RegexVisitor>(0, 
        create_expr_binary_operator<AddVisitor>(0, 
          create_expr_cst(rdf::RdfAstType(rdf::LString("Hello "))),
          create_expr_binary_operator<AddVisitor>(0, 
            create_expr_cst(rdf::RdfAstType(rdf::LInt32(1))), 
            create_expr_cst(rdf::RdfAstType(rdf::LUInt64(2)))
          )
        ),
        create_expr_cst(rdf::RdfAstType(rdf::LString("(\\d)")))
      ), 
      create_expr_cst(rdf::RdfAstType(rdf::LString("3")))
  ));
  auto expr = create_expr_disjunction(data);
  EXPECT_TRUE(rdf::to_bool(expr->eval(nullptr, nullptr)));
}
}   // namespace
}   // namespace jets::rete