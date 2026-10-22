// rules_unit_test_harness I-45 on the C++ engine, 2026-10-11.
//
// rdf::parse_bool accepts exactly the spellings Go's rdf.ParseBool accepts
// (jets/jetrules/rdf/parse_bool_test.go is the same table). A rule testing text
// for truth reads it with parse_bool: a value outside the spellings adds
// (jets:iState jets:exception "...") to the session and makes the expression
// null, and empty text is null with no exception.
#include <algorithm>
#include <string>
#include <vector>

#include <gtest/gtest.h>

#include "../rdf/rdf_types.h"
#include "../rete/rete_types.h"
#include "../rete/expr_op_logics.h"
#include "../rete/rete_types_impl.h"

namespace jets::rete {
namespace {

TEST(ParseBoolTest, TheTwoSetsAndNothingElse) {
  for(auto v: {"true", "TRUE", "True", "t", "T", "yes", "YES", "y", "Y", "1", " true ", "\tY\n"}) {
    EXPECT_EQ(rdf::parse_bool(v), rdf::ParsedBool::True) << v;
  }
  for(auto v: {"false", "FALSE", "f", "F", "no", "NO", "n", "N", "0", " no "}) {
    EXPECT_EQ(rdf::parse_bool(v), rdf::ParsedBool::False) << v;
  }
  for(auto v: {"", "  ", "\t"}) {
    EXPECT_EQ(rdf::parse_bool(v), rdf::ParsedBool::Null) << "[" << v << "]";
  }
  for(auto v: {"maybe", "tomato", "nope", "yess", "2", "-1", "10", "on", "off", "\"true\""}) {
    EXPECT_EQ(rdf::parse_bool(v), rdf::ParsedBool::Invalid) << v;
  }
  // to_bool, with nowhere to report an error, is true only for true.
  EXPECT_TRUE(rdf::to_bool(std::string_view("Yes")));
  EXPECT_FALSE(rdf::to_bool(std::string_view("maybe")));
  EXPECT_FALSE(rdf::to_bool(std::string_view("")));
}

class TruthTest : public ::testing::Test {
 protected:
  TruthTest() {
    auto meta_graph = rdf::create_rdf_graph();
    auto * rmgr = meta_graph->rmgr();
    rmgr->initialize();
    NodeVertexVector node_vertexes;
    ReteMetaStore::AlphaNodeVector alpha_nodes;
    rete_meta_store = create_rete_meta_store({}, alpha_nodes, node_vertexes);
    rete_meta_store->initialize();
    rmgr->set_locked();
    meta_graph->set_locked();
    rdf_session = rdf::create_rdf_session(meta_graph);
    rete_session = create_rete_session(rete_meta_store, rdf_session.get());
    rete_session->initialize();
  }

  std::vector<std::string> exceptions() {
    auto * sess = rete_session->rdf_session();
    auto const* jr = sess->rmgr()->jets();
    std::vector<std::string> out;
    auto itor = sess->find(jr->jets__istate, jr->jets__exception);
    while(not itor.is_end()) {
      out.push_back(rdf::to_string(itor.get_object()));
      itor.next();
    }
    return out;
  }

  ReteSessionPtr rete_session;
  ReteMetaStorePtr rete_meta_store;
  rdf::RDFSessionPtr rdf_session;
};

TEST_F(TruthTest, AFilterOnTextReadsItAsABool) {
  struct Case { char const* text; bool pass; bool raises; };
  for(auto c: std::vector<Case>{{"yes", true, false}, {" TRUE ", true, false}, {"false", false, false},
                                {"n", false, false}, {"", false, false}}) {
    auto expr = create_expr_cst(rdf::LString(c.text));
    EXPECT_EQ(expr->eval_filter(rete_session.get(), nullptr), c.pass) << c.text;
  }
  EXPECT_TRUE(exceptions().empty());

  auto expr = create_expr_cst(rdf::LString("maybe"));
  EXPECT_FALSE(expr->eval_filter(rete_session.get(), nullptr));
  auto ex = exceptions();
  ASSERT_EQ(ex.size(), 1u);
  EXPECT_NE(ex[0].find("\"maybe\" is not a bool"), std::string::npos) << ex[0];
}

TEST_F(TruthTest, NotAndConjunctionOnText) {
  NotVisitor not_op(rete_session.get(), nullptr);
  // not on text raised "Invalid arguments for not" until I-45, even for "false".
  EXPECT_TRUE(rdf::to_bool(boost::apply_visitor(not_op, rdf::RdfAstType(rdf::LString("false")))));
  EXPECT_FALSE(rdf::to_bool(boost::apply_visitor(not_op, rdf::RdfAstType(rdf::LString("Y")))));
  auto empty = boost::apply_visitor(not_op, rdf::RdfAstType(rdf::LString("")));
  EXPECT_EQ(empty.which(), rdf::rdf_null_t);
  EXPECT_TRUE(exceptions().empty());

  auto bad = boost::apply_visitor(not_op, rdf::RdfAstType(rdf::LString("nope")));
  EXPECT_EQ(bad.which(), rdf::rdf_null_t);
  EXPECT_EQ(exceptions().size(), 1u);

  auto conj = create_expr_conjunction(ExprConjunction::data_type{
      create_expr_cst(rdf::LString("yes")), create_expr_cst(rdf::LString("tomato"))});
  EXPECT_EQ(conj->eval(rete_session.get(), nullptr).which(), rdf::rdf_null_t);
  EXPECT_FALSE(conj->eval_filter(rete_session.get(), nullptr));
  EXPECT_EQ(exceptions().size(), 2u);  // one exception per string, and interned
}

// The error path writes to the graph under the iterators of the rule being visited:
// each distinct text raises a distinct (jets:iState jets:exception "...") while
// visit_rete_graph iterates (jets:iState ?p ?v), and the exceptions grow the very
// tables that iteration walks, rehashing them many times. The graph freezes its live
// iterators before each change (base_graph_iterator.h). Run it under AddressSanitizer
// to see a use of a stale iterator; here it checks every exception is raised once.
//
//   v1  (?x hasText ?t)
//   v2  (jets:iState ?p ?v).[?t]       -- ?t is text that is not a bool
TEST(TruthErrorPathTest, ExceptionsRaisedUnderALiveIterator) {
  constexpr int kTexts = 200;
  auto meta_graph = rdf::create_rdf_graph();
  auto * rmgr = meta_graph->rmgr();
  rmgr->initialize();
  auto const* jets = rmgr->jets();
  auto has_text = rmgr->create_resource("hasText");

  auto ri = [](std::initializer_list<std::pair<int, char const*>> cols) {
    auto r = create_row_initializer(static_cast<int>(cols.size()));
    int i = 0;
    for(auto const& c: cols) r->put(i++, c.first, c.second);
    return r;
  };
  NodeVertexVector nv;
  nv.push_back(create_node_vertex(nullptr, 0, 0, false, 100, {}, "", {}));
  nv.push_back(create_node_vertex(nv[0].get(), 0, 1, false, 100, {}, "",
    ri({{0|brc_triple, "?x"}, {2|brc_triple, "?t"}})));
  nv.push_back(create_node_vertex(nv[1].get(), 0, 2, false, 100, create_expr_binded_var(1), "",
    ri({{0|brc_parent_node, "?x"}, {1|brc_parent_node, "?t"}, {1|brc_triple, "?p"}, {2|brc_triple, "?v"}})));
  ReteMetaStore::AlphaNodeVector an;
  an.push_back(create_alpha_node<F_var, F_var, F_var>(nv[0].get(), 0, true, "", F_var("*"), F_var("*"), F_var("*")));
  an.push_back(create_alpha_node<F_var, F_cst, F_var>(nv[1].get(), 0, true, "", F_var("?x"), F_cst(has_text), F_var("?t")));
  an.push_back(create_alpha_node<F_cst, F_var, F_var>(nv[2].get(), 0, true, "", F_cst(jets->jets__istate), F_var("?p"), F_var("?v")));
  auto rete_meta_store = create_rete_meta_store({}, an, nv);
  rete_meta_store->initialize();
  rmgr->set_locked();
  meta_graph->set_locked();

  auto rdf_session = rdf::create_rdf_session(meta_graph);
  auto rete_session = create_rete_session(rete_meta_store, rdf_session.get());
  rete_session->initialize();
  auto smgr = rdf_session->rmgr();
  rdf_session->insert(jets->jets__istate, smgr->create_resource("cfg"), smgr->create_literal(1));
  for(int i=0; i<kTexts; i++) {
    rdf_session->insert(smgr->create_resource("x" + std::to_string(i)), has_text,
      smgr->create_literal("bad" + std::to_string(i)));
  }
  ASSERT_EQ(rete_session->execute_rules(), 0);

  std::vector<std::string> raised;
  auto itor = rdf_session->find(jets->jets__istate, jets->jets__exception);
  while(not itor.is_end()) {
    raised.push_back(rdf::to_string(itor.get_object()));
    itor.next();
  }
  ASSERT_EQ(raised.size(), static_cast<size_t>(kTexts));
  for(int i=0; i<kTexts; i++) {
    auto text = "\"bad" + std::to_string(i) + "\" is not a bool";
    EXPECT_EQ(std::count_if(raised.begin(), raised.end(), 
      [&](auto const& m) { return m.find(text) != std::string::npos; }), 1) << text;
  }
  rete_session->terminate();
}

}  // namespace
}  // namespace jets::rete
