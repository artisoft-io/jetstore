#include <fstream>
#include <string>
#include <vector>

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

// OPERATOR NAMES MUST RESOLVE IN THE ENGINE THAT SHIPS.
//
// An operator name reaches the engine as an opaque string: the JetRule grammar reduces
// both binaryOp and unaryOp to Identifier. Until rules_benchmark Phase 6 the compiler wrote the name
// to workspace.db unexamined, so a typo in a .jr file COMPILED CLEANLY and raised here,
// at rule-execution time, out of create_binary_expr below.
//
// Since then the compiler refuses an operator that jets/compilerv2/compiler/operators.txt
// does not list, and operator_list_test.cc checks that list against this factory in both
// directions. That is the check now; this file is the earlier, narrower one, kept.
//
// WHERE THE NAMES COME FROM. C++ cannot compile a .jr file, so the corpus names were
// collected by a Go test that could, TestCorpusOperatorNamesResolve, which compiled
// every rule set of every workspace and wrote test_data/corpus_operators.txt. It went
// with the Go engine (rules_benchmark Phase 6), so the manifest is the corpus's
// operator names as last collected, and is no longer regenerated.
namespace jets::rete {
namespace {

// The manifest is found relative to the working directory, which for ctest is the
// build's jets/ directory -- jets/CMakeLists.txt copies jets/rete/test_data next to the
// jets_test binary after every build. The other candidates let the binary be run from
// the repository root or from the build root by hand.
std::string find_manifest()
{
  for(auto const& candidate : {
      "test_data/corpus_operators.txt",
      "jets/test_data/corpus_operators.txt",
      "jets/rete/test_data/corpus_operators.txt",
      "../jets/rete/test_data/corpus_operators.txt"}) {
    std::ifstream in(candidate);
    if(in.good()) return candidate;
  }
  return {};
}

struct OperatorNames {
  std::vector<std::string> binary;
  std::vector<std::string> unary;
};

OperatorNames read_manifest(std::string const& path)
{
  OperatorNames names;
  std::ifstream in(path);
  std::string line;
  while(std::getline(in, line)) {
    if(line.empty() or line[0] == '#') continue;
    auto sp = line.find(' ');
    if(sp == std::string::npos) continue;
    auto kind = line.substr(0, sp);
    auto op = line.substr(sp + 1);
    while(not op.empty() and (op.back() == '\r' or op.back() == ' ')) op.pop_back();
    if(op.empty()) continue;
    if(kind == "binary") names.binary.push_back(op);
    else if(kind == "unary") names.unary.push_back(op);
  }
  return names;
}

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

// The corpus check. Every operator name the rules actually use must resolve.
TEST_F(OperatorFactoryTest, CorpusOperatorNames) {
  auto path = find_manifest();
  ASSERT_FALSE(path.empty())
    << "test_data/corpus_operators.txt was not found from the working directory. "
       "ctest runs this binary from the build's jets/ directory, where CMake copies "
       "jets/rete/test_data after every build; run it through ctest, or from that "
       "directory.";
  auto names = read_manifest(path);

  // A manifest that failed to parse would make every assertion below vacuous, which is
  // the one way this test could pass while checking nothing.
  ASSERT_GT(names.binary.size(), 20u) << "manifest " << path << " parsed as near empty";
  ASSERT_GT(names.unary.size(), 5u) << "manifest " << path << " parsed as near empty";

  for(auto const& op : names.binary) {
    EXPECT_NO_THROW({
      auto expr = this->factory.create_binary_expr(0, this->operand(), op, this->operand());
      EXPECT_TRUE(expr) << "binary operator '" << op << "' returned a null expression";
    }) << "binary operator '" << op << "' is used by the rule corpus and is not "
          "registered in create_binary_expr (jets/rete/expr_operator_factory.h)";
  }
  for(auto const& op : names.unary) {
    EXPECT_NO_THROW({
      auto expr = this->factory.create_unary_expr(0, op, this->operand());
      EXPECT_TRUE(expr) << "unary operator '" << op << "' returned a null expression";
    }) << "unary operator '" << op << "' is used by the rule corpus and is not "
          "registered in create_unary_expr (jets/rete/expr_operator_factory.h)";
  }
}

// THE CONTROL CASE, and it is here because the test above is a list of things that
// pass. If create_binary_expr ever stopped raising on an unknown name -- returning
// null, say -- every assertion above would still pass and the typo it exists to catch
// would go through silently.
TEST_F(OperatorFactoryTest, UnknownOperatorNameRaises) {
  EXPECT_THROW(
    this->factory.create_binary_expr(0, this->operand(), "sum_value", this->operand()),
    jets::rete_exception)
    << "an unknown binary operator must raise; 'sum_value' is the typo of sum_values "
       "that compiles cleanly";
  EXPECT_THROW(
    this->factory.create_unary_expr(0, "to_upper_case", this->operand()),
    jets::rete_exception)
    << "an unknown unary operator must raise";
}

}   // namespace
}   // namespace jets::rete
