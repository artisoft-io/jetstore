#include <fstream>
#include <map>
#include <set>
#include <sstream>
#include <string>
#include <vector>

#include <gtest/gtest.h>

#include "../rdf/rdf_types.h"
#include "../rete/rete_types.h"
#include "../rete/rete_types_impl.h"
// Not expr_operator_factory.h, which DEFINES create_binary_expr and create_unary_expr:
// see expr_operator_factory_test.cc. The definitions link out of libjets_static.
#include "../rete/rete_meta_store_factory.h"

// THE writes_graph MARKS OF THE COMPILER'S OPERATOR LIST ARE THE ENGINE'S (I-65).
//
// The compiler refuses a filter that calls an operator marked writes_graph in
// jets/compilerv2/compiler/operators.txt (jet_rule_listener_filter_writes.go), so a
// filter never inserts into the session while the session iterates. The marks were
// set by hand from reading the visitors; this test sets them from what the visitors
// do. Every listed operator is evaluated, in a session on the lookup test workspace
// (one lookup table, acme:ProcedureLookup), on every pair of a few operands (every
// one, for a unary operator), and an operator writes the graph when one of those
// evaluations grows the session's inferred graph, which is where insert_inferred
// puts a triple. Then:
//   - every operator that wrote is marked writes_graph;
//   - every operator marked writes_graph wrote for one of the operands.
// The jets:exception an operator inserts on a bad argument goes to the asserted
// graph, by insert, and is not a write in this sense: operators.txt says why.
//
// An operator that writes only for an operand none of these is would pass unmarked.
// The operands are chosen for the operators that write today: a lookup table's name
// and a key it holds, small integers for range, text for create_entity.
namespace jets::rete {
namespace {

#ifndef JETS_SOURCE_DIR
#error "JETS_SOURCE_DIR must name the repository root (jets/CMakeLists.txt defines it)"
#endif

std::string const kOperatorList = std::string(JETS_SOURCE_DIR) + "/jets/compilerv2/compiler/operators.txt";

struct Listed {
  bool binary;
  bool writes_graph;
};

// operators.txt, as operator_list_test.cc reads it; that test checks its form
std::map<std::string, Listed> read_operator_list(std::string const& path)
{
  std::map<std::string, Listed> ops;
  std::ifstream in(path);
  EXPECT_TRUE(in.good()) << "cannot read " << path;
  std::string line;
  while(std::getline(in, line)) {
    std::istringstream fields(line);
    std::string kind, name, flag;
    fields >> kind;
    if(kind.empty() or kind[0] == '#') continue;
    fields >> name >> flag;
    // a name listed as binary and as unary is two operators
    ops[kind + " " + name] = {kind == "binary", flag == "writes_graph"};
  }
  return ops;
}

struct TestableFactory : public ReteMetaStoreFactory {
  using ReteMetaStoreFactory::create_binary_expr;
  using ReteMetaStoreFactory::create_unary_expr;
};

std::vector<RDFTTYPE> operands()
{
  return {
    rdf::NamedResource("acme:ProcedureLookup"),
    rdf::LString("100"),
    rdf::LString("x"),
    rdf::LInt32(1),
    rdf::LInt32(3),
    rdf::LDouble(2.5),
    rdf::LDate(rdf::date(2024, 1, 1)),
    rdf::NamedResource("s"),
  };
}

class OperatorWritesGraph : public ::testing::Test {
 protected:
  void SetUp() override
  {
    ASSERT_EQ(factory.load_database("test_data/lookup_helper_test_workspace.db",
      "test_data/lookup_helper_test_data.db"), 0);
    meta_store = factory.get_rete_meta_store("lookup_helper_test_workspace.jr");
    ASSERT_TRUE(meta_store);
    listed = read_operator_list(kOperatorList);
    ASSERT_GT(listed.size(), 50u) << kOperatorList << " read as near empty";
  }

  // Whether evaluating `op` on one of the operands grew the inferred graph
  bool writes(std::string const& name, bool binary)
  {
    auto rdf_session = rdf::create_rdf_session(rdf::create_rdf_graph(factory.get_rmgr()));
    auto rete_session = create_rete_session(meta_store, rdf_session.get());
    EXPECT_EQ(rete_session->initialize(), 0);
    auto const& inferred = rdf_session->get_inferred_graph();
    bool wrote = false;
    auto eval = [&](ExprBasePtr expr) {
      auto before = inferred->size();
      try {
        expr->eval(rete_session.get(), nullptr);
      } catch(...) {
        // a refused operand; what was inserted before the throw still counts
      }
      if(inferred->size() > before) wrote = true;
    };
    for(auto const& a: operands()) {
      if(binary) {
        for(auto const& b: operands()) {
          eval(factory.create_binary_expr(0, create_expr_cst(a), name, create_expr_cst(b)));
        }
      } else {
        eval(factory.create_unary_expr(0, name, create_expr_cst(a)));
      }
    }
    rete_session->terminate();
    return wrote;
  }

  TestableFactory factory;
  ReteMetaStorePtr meta_store;
  std::map<std::string, Listed> listed;
};

TEST_F(OperatorWritesGraph, TheMarksAreWhatTheVisitorsDo)
{
  std::set<std::string> wrote, marked;
  for(auto const& [key, info]: listed) {
    auto name = key.substr(key.find(' ') + 1);
    if(writes(name, info.binary)) wrote.insert(key);
    if(info.writes_graph) marked.insert(key);
  }
  for(auto const& op: wrote) {
    EXPECT_TRUE(marked.count(op)) << op << " inserts into the inferred graph and operators.txt does not mark it "
      "writes_graph: the compiler lets a filter call it";
  }
  for(auto const& op: marked) {
    EXPECT_TRUE(wrote.count(op)) << op << " is marked writes_graph and inserted nothing for any operand tried";
  }
  // the operators that write today, so that a change in what this test sees is a
  // decision rather than a surprise
  EXPECT_EQ(wrote, (std::set<std::string>{
    "binary lookup", "binary multi_lookup", "binary range",
    "unary create_entity", "unary lookup_rand", "unary multi_lookup_rand"}));
}

}   // namespace
}   // namespace jets::rete
