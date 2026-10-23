#include <fstream>
#include <regex>
#include <set>
#include <sstream>
#include <string>

#include <gtest/gtest.h>

#include "../rdf/rdf_types.h"
#include "../rete/rete_types.h"
#include "../rete/rete_types_impl.h"
// Not expr_operator_factory.h, which DEFINES create_binary_expr and create_unary_expr:
// see expr_operator_factory_test.cc. The definitions link out of libjets_static.
#include "../rete/rete_meta_store_factory.h"

// THE COMPILER'S OPERATOR LIST IS THE FACTORY'S.
//
// The compiler refuses a rule that calls an operator not listed in
// jets/compilerv2/compiler/operators.txt (jet_rule_listener_operators.go), so that a
// misspelt name is a compile error rather than an exception when the engine builds the
// rule. That holds only while the list is exactly what create_binary_expr and
// create_unary_expr (jets/rete/expr_operator_factory.h) accept, under the same arity.
// This test is what keeps it so, in both directions:
//
//   - every listed operator resolves in the factory, at run time (ListedNamesResolve);
//   - every name the factory registers is listed (RegisteredNamesAreListed). There is
//     no way to ask a chain of ifs what it accepts, so this reads the header's source:
//     with its comments removed, every `op == "name"` in create_binary_expr's body is
//     a binary registration and every one in create_unary_expr's a unary one, whatever
//     the shape of the condition around it (`if(op == "a" or op == "b")`,
//     `else if(op == "x")`). A registration written another way, op.compare("x") say,
//     is not seen, and RegisteredNamesResolve cannot catch that; the factory is
//     written in the one form.
//
// JETS_SOURCE_DIR, the repository root, is defined for jets_test by jets/CMakeLists.txt.
namespace jets::rete {
namespace {

#ifndef JETS_SOURCE_DIR
#error "JETS_SOURCE_DIR must name the repository root (jets/CMakeLists.txt defines it)"
#endif

std::string const kOperatorList = std::string(JETS_SOURCE_DIR) + "/jets/compilerv2/compiler/operators.txt";
std::string const kFactory = std::string(JETS_SOURCE_DIR) + "/jets/rete/expr_operator_factory.h";

struct Names {
  std::set<std::string> binary;
  std::set<std::string> unary;
};

std::string read_file(std::string const& path)
{
  std::ifstream in(path);
  std::stringstream ss;
  ss << in.rdbuf();
  return ss.str();
}

// operators.txt: comments and blank lines skipped; every other line is an arity, a
// name and optionally writes_graph, which only the compiler reads. A line of any other
// form, or a name listed twice, is a failure.
Names read_operator_list(std::string const& path)
{
  Names names;
  std::ifstream in(path);
  EXPECT_TRUE(in.good()) << "cannot read " << path;
  std::string line;
  int n = 0;
  while(std::getline(in, line)) {
    ++n;
    std::istringstream fields(line);
    std::string kind, name, flag, extra;
    fields >> kind;
    if(kind.empty() or kind[0] == '#') continue;
    fields >> name >> flag >> extra;
    bool ok = (kind == "binary" or kind == "unary") and not name.empty() and
              (flag.empty() or flag == "writes_graph") and extra.empty();
    EXPECT_TRUE(ok) << path << ":" << n << ": want \"binary|unary <name> [writes_graph]\": " << line;
    if(not ok) continue;
    auto& set = kind == "binary" ? names.binary : names.unary;
    EXPECT_TRUE(set.insert(name).second) << path << ":" << n << ": " << kind << " " << name << " listed twice";
  }
  return names;
}

// The source with every comment replaced by spaces, keeping newlines, string and
// character literals: a commented-out registration is not a registration, and a "//"
// inside a string is not a comment.
std::string strip_comments(std::string const& src)
{
  enum { code, line_comment, block_comment, string_lit, char_lit } state = code;
  std::string out = src;
  for(size_t i = 0; i < src.size(); ++i) {
    char const c = src[i];
    char const next = i + 1 < src.size() ? src[i + 1] : '\0';
    switch(state) {
      case code:
        if(c == '/' and next == '/') { state = line_comment; out[i] = ' '; }
        else if(c == '/' and next == '*') { state = block_comment; out[i] = ' '; }
        else if(c == '"') state = string_lit;
        else if(c == '\'') state = char_lit;
        break;
      case line_comment:
        if(c == '\n') state = code; else out[i] = ' ';
        break;
      case block_comment:
        if(c == '*' and next == '/') { out[i] = out[i + 1] = ' '; ++i; state = code; }
        else if(c != '\n') out[i] = ' ';
        break;
      case string_lit:
        if(c == '\\') ++i;
        else if(c == '"') state = code;
        break;
      case char_lit:
        if(c == '\\') ++i;
        else if(c == '\'') state = code;
        break;
    }
  }
  return out;
}

// The names the factory's source registers: every `op == "name"` outside comments, in
// create_binary_expr and then in create_unary_expr.
Names read_factory_registrations(std::string const& path)
{
  Names names;
  std::string const src = strip_comments(read_file(path));
  EXPECT_FALSE(src.empty()) << "cannot read " << path;
  auto const binary_at = src.find("ReteMetaStoreFactory::create_binary_expr");
  auto const unary_at = src.find("ReteMetaStoreFactory::create_unary_expr");
  EXPECT_NE(binary_at, std::string::npos) << "create_binary_expr not found in " << path;
  EXPECT_NE(unary_at, std::string::npos) << "create_unary_expr not found in " << path;
  EXPECT_LT(binary_at, unary_at) << "create_binary_expr is expected before create_unary_expr in " << path;
  if(binary_at == std::string::npos or unary_at == std::string::npos or unary_at < binary_at) return names;

  std::regex const registration(R"re(\bop\s*==\s*"([^"]+)")re");
  auto scan = [&](std::string const& body, std::set<std::string>& out) {
    for(std::sregex_iterator m(body.begin(), body.end(), registration), end; m != end; ++m) {
      auto const name = (*m)[1].str();
      EXPECT_TRUE(out.insert(name).second) << name << " is registered twice in " << path;
    }
  };
  scan(src.substr(binary_at, unary_at - binary_at), names.binary);
  scan(src.substr(unary_at), names.unary);
  return names;
}

struct TestableFactory : public ReteMetaStoreFactory {
  using ReteMetaStoreFactory::create_binary_expr;
  using ReteMetaStoreFactory::create_unary_expr;
};

class OperatorList : public ::testing::Test {
 protected:
  void SetUp() override
  {
    listed = read_operator_list(kOperatorList);
    registered = read_factory_registrations(kFactory);
    // A file that read as near empty would make every assertion below vacuous.
    ASSERT_GT(listed.binary.size(), 20u) << kOperatorList << " read as near empty";
    ASSERT_GT(listed.unary.size(), 10u) << kOperatorList << " read as near empty";
    ASSERT_GT(registered.binary.size(), 20u) << kFactory << " read as near empty";
    ASSERT_GT(registered.unary.size(), 10u) << kFactory << " read as near empty";
  }

  ExprBasePtr operand()const
  {
    return create_expr_cst(rdf::RdfAstType(rdf::LInt32(1)));
  }

  TestableFactory factory;
  Names listed;
  Names registered;
};

TEST_F(OperatorList, ListedNamesResolve) {
  for(auto const& op : listed.binary) {
    EXPECT_NO_THROW({
      auto expr = factory.create_binary_expr(0, operand(), op, operand());
      EXPECT_TRUE(expr) << "binary operator '" << op << "' returned a null expression";
    }) << "binary " << op << " is listed in operators.txt and create_binary_expr does not register it";
  }
  for(auto const& op : listed.unary) {
    EXPECT_NO_THROW({
      auto expr = factory.create_unary_expr(0, op, operand());
      EXPECT_TRUE(expr) << "unary operator '" << op << "' returned a null expression";
    }) << "unary " << op << " is listed in operators.txt and create_unary_expr does not register it";
  }
}

TEST_F(OperatorList, RegisteredNamesAreListed) {
  for(auto const& op : registered.binary) {
    EXPECT_TRUE(listed.binary.count(op))
      << "create_binary_expr registers " << op << " and operators.txt does not list it as binary: "
         "the compiler refuses every rule that calls it";
  }
  for(auto const& op : registered.unary) {
    EXPECT_TRUE(listed.unary.count(op))
      << "create_unary_expr registers " << op << " and operators.txt does not list it as unary: "
         "the compiler refuses every rule that calls it";
  }
}

// The source scan reads what the factory runs: every name it finds resolves. Without
// this, a scan that matched the wrong lines could agree with the list and prove nothing.
TEST_F(OperatorList, RegisteredNamesResolve) {
  for(auto const& op : registered.binary) {
    EXPECT_NO_THROW(factory.create_binary_expr(0, operand(), op, operand())) << "binary " << op;
  }
  for(auto const& op : registered.unary) {
    EXPECT_NO_THROW(factory.create_unary_expr(0, op, operand())) << "unary " << op;
  }
}

// THE CONTROL CASE: a name the list does not have is not an operator, under either
// arity, and a name under the other arity is not one either.
TEST_F(OperatorList, UnlistedNamesRaise) {
  EXPECT_FALSE(listed.binary.count("sum_value"));
  EXPECT_THROW(factory.create_binary_expr(0, operand(), "sum_value", operand()), jets::rete_exception);
  EXPECT_FALSE(listed.unary.count("lookup"));
  EXPECT_THROW(factory.create_unary_expr(0, "lookup", operand()), jets::rete_exception);
}

}   // namespace
}   // namespace jets::rete
