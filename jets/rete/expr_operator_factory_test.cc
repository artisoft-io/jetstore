#include <string>

#include <gtest/gtest.h>

#include "../rdf/rdf_types.h"
#include "../rete/rete_types.h"
#include "../rete/rete_types_impl.h"
// Deliberately NOT expr_operator_factory.h: that header carries the out-of-class
// DEFINITIONS of create_binary_expr and create_unary_expr rather than declarations, so
// including it in a second translation unit is a duplicate-symbol link error. The
// declarations come from rete_meta_store_factory.h and the definitions link out of
// libjets_static, which is also the honest arrangement for a test -- it exercises the
// code the library ships rather than its own copy.
#include "../rete/rete_meta_store_factory.h"

// AN UNKNOWN OPERATOR NAME RAISES IN THE ENGINE THAT SHIPS.
//
// An operator name reaches the engine as an opaque string: the JetRule grammar reduces
// both binaryOp and unaryOp to Identifier. Until rules_benchmark Phase 6 the compiler
// wrote the name to workspace.db unexamined, so a typo in a .jr file COMPILED CLEANLY
// and raised here, at rule-execution time, out of create_binary_expr below.
//
// Since then the compiler refuses an operator that jets/compilerv2/compiler/operators.txt
// does not list, and operator_list_test.cc checks that list against this factory in both
// directions. That is the check on the names now. This file held the earlier, narrower
// one, CorpusOperatorNames, which read the corpus's operator names from a manifest
// (test_data/corpus_operators.txt) written by a Go test deleted with the Go engine; the
// frozen manifest and that test were removed in rules_benchmark Phase 7 (I-163), and
// both are at the tag go-engine-final. What stays is the control: the factory raises on
// a name it does not register.
namespace jets::rete {
namespace {

// create_binary_expr and create_unary_expr are PROTECTED members of
// ReteMetaStoreFactory, so the test reaches them through a subclass rather than by
// widening the production interface for a test's benefit.
//
// They touch none of the factory's state -- each dispatches on the name and hands the
// operands to create_expr_binary_operator, which only wraps them in a shared_ptr. So a
// default-constructed factory is enough and NO DATABASE IS NEEDED, which is what makes
// this test cheap enough to run with the rest.
struct TestableFactory : public ReteMetaStoreFactory {
  using ReteMetaStoreFactory::create_binary_expr;
  using ReteMetaStoreFactory::create_unary_expr;
};

class OperatorFactoryTest : public ::testing::Test {
 protected:
  TestableFactory factory;

  ExprBasePtr operand()const
  {
    return create_expr_cst(rdf::RdfAstType(rdf::LInt32(1)));
  }
};

// An unknown name raises rather than giving a null expression, which a rule would
// evaluate as nothing: the engine's own refusal, behind the compiler's.
TEST_F(OperatorFactoryTest, UnknownOperatorNameRaises) {
  EXPECT_THROW(
    this->factory.create_binary_expr(0, this->operand(), "sum_value", this->operand()),
    jets::rete_exception)
    << "an unknown binary operator must raise; 'sum_value' is the typo of sum_values "
       "that compiled cleanly before the compiler's operator list";
  EXPECT_THROW(
    this->factory.create_unary_expr(0, "to_upper_case", this->operand()),
    jets::rete_exception)
    << "an unknown unary operator must raise";
}

}   // namespace
}   // namespace jets::rete
