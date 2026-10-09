#include <memory>
#include <string>

#include <gtest/gtest.h>

#include "../rdf/rdf_types.h"
#include "../rete/rete_types.h"
#include "../rete/rete_types_impl.h"
#include "../rete/expr_op_logics.h"
#include "../rete/expr_op_arithmetics.h"
#include "../rete/expr_op_resources.h"

// THE usi_ws LOOP-CONTROL SHAPE, reduced to five rule terms.
//
// MSK_ADJ_LOOP010/LOOP020/MERGE_010 (MSK), adj11/adj12/adj20 (IM) and the topMatch /
// mergeList rules (CM) all have this form:
//
//   v3  (?s rdf:type State).(?s matchList ?m).not(?s mergeList ?m) -> (?s priorityList ?m)
//   v4  (?s rdf:type State).(?s loop ?n).[(?s exist priorityList)]
//         -> (?s adjLoop ?n).(?s topMatch (?s sorted_head cfg))
//   v5  (?s topMatch ?m) -> (?s mergeList ?m)
//
// The loop driver asserts (?s loop n) and runs the rules, n = 1, 2, ...
//
// topMatch supports mergeList, mergeList removes the priority list entry, and the empty
// priority list withdraws the v4 row that inferred topMatch: an odd loop through
// negation, which has no fixed point under correct truth maintenance. It terminates in
// this engine because of two known defects, kept deliberately by Michel's decision of
// 2026-10-09 (jets/jetrules/rete/README.md):
//
//   - an aggregate consequent is recomputed on retraction: sorted_head over the now
//     empty list gives null, so the topMatch inferred is not retracted;
//   - a withdrawn row is never erased from its relation, so the v4 row is not
//     re-admitted when its support comes back.
//
// Measured on 2026-10-09: replaying the sorted_head consequent AND erasing withdrawn
// rows makes this network hit the vertex visit limit (-10); replaying alone ends with
// every merge undone; erasing alone ends as below. usi_ws runs MSK_ADJ_LOOP010/020 +
// MERGE_010, adj11/adj12/adj20 and mcp1/mcp2/csc20 in this shape, and with
// _0:no_truth_main_on_exist off they cycled without end under the full fix.
//
// THIS IS A GUARD, NOT A SPECIFICATION. It pins that the shape terminates with both
// items merged, as the engine at 45dd7851 does. If a change to truth maintenance makes
// it fail, that change needs option (b) or (c) of the README entry, not a new
// expectation here.
namespace jets::rete {
namespace {

class LoopThroughNegationTest : public ::testing::Test {
 protected:
  ~LoopThroughNegationTest() {
    if(this->rete_session) this->rete_session->terminate();
  }

  void build()
  {
    auto meta_graph = rdf::create_rdf_graph();
    auto * rmgr = meta_graph->rmgr();
    rmgr->initialize();
    auto const* jets = rmgr->jets();

    auto state     = rmgr->create_resource("State");
    auto match     = rmgr->create_resource("matchList");
    auto merge     = rmgr->create_resource("mergeList");
    auto priority  = rmgr->create_resource("priorityList");
    auto loop      = rmgr->create_resource("loop");
    auto adj_loop  = rmgr->create_resource("adjLoop");
    auto top       = rmgr->create_resource("topMatch");
    auto weight    = rmgr->create_resource("weight");

    auto cfg = rmgr->create_resource("cfg");
    meta_graph->insert(cfg, jets->jets__entity_property, priority);
    meta_graph->insert(cfg, jets->jets__value_property, weight);
    meta_graph->insert(cfg, jets->jets__operator, std::string("<"));
    // stop a runaway rather than hang the test
    meta_graph->insert(jets->jets__istate, jets->jets__max_vertex_visits, 1000);

    auto ri = [](std::initializer_list<std::pair<int, char const*>> cols) {
      auto r = create_row_initializer(static_cast<int>(cols.size()));
      int i = 0;
      for(auto const& c: cols) r->put(i++, c.first, c.second);
      return r;
    };

    // [(?s exist priorityList) == 1]
    auto has_priority = create_expr_binary_operator<EqVisitor>(0,
      create_expr_binary_operator<ExistVisitor>(0,
        create_expr_binded_var(0),
        create_expr_cst(rdf::RdfAstType(rdf::NamedResource("priorityList")))),
      create_expr_cst(rdf::RdfAstType(rdf::LInt32(1))));

    NodeVertexVector nv;
    nv.push_back(create_node_vertex(nullptr, 0, 0, false, 100, {}, "", {}));
    // v1 (?s rdf:type State)
    nv.push_back(create_node_vertex(nv[0].get(), 0, 1, false, 100, {}, "", ri({{0|brc_triple, "?s"}})));
    // v2 (?s matchList ?m)
    nv.push_back(create_node_vertex(nv[1].get(), 0, 2, false, 100, {}, "",
      ri({{0|brc_parent_node, "?s"}, {2|brc_triple, "?m"}})));
    // v3 not(?s mergeList ?m) -> (?s priorityList ?m)
    nv.push_back(create_node_vertex(nv[2].get(), 0, 3, true, 100, {}, "",
      ri({{0|brc_parent_node, "?s"}, {1|brc_parent_node, "?m"}})));
    // v4 (?s loop ?n).[(?s exist priorityList)] -> (?s adjLoop ?n).(?s topMatch sorted_head)
    nv.push_back(create_node_vertex(nv[1].get(), 0, 4, false, 60, has_priority, "",
      ri({{0|brc_parent_node, "?s"}, {2|brc_triple, "?n"}})));
    // v5 (?s topMatch ?m) -> (?s mergeList ?m)
    nv.push_back(create_node_vertex(nv[0].get(), 0, 5, false, 100, {}, "",
      ri({{0|brc_triple, "?s"}, {2|brc_triple, "?m"}})));

    ReteMetaStore::AlphaNodeVector an;
    an.push_back(create_alpha_node<F_var, F_var, F_var>(nv[0].get(), 0, true, "", F_var("*"), F_var("*"), F_var("*")));
    an.push_back(create_alpha_node<F_var, F_cst, F_cst>(nv[1].get(), 0, true, "", F_var("?s"), F_cst(jets->rdf__type), F_cst(state)));
    an.push_back(create_alpha_node<F_binded, F_cst, F_var>(nv[2].get(), 0, true, "", F_binded(0), F_cst(match), F_var("?m")));
    an.push_back(create_alpha_node<F_binded, F_cst, F_binded>(nv[3].get(), 0, true, "", F_binded(0), F_cst(merge), F_binded(1)));
    an.push_back(create_alpha_node<F_binded, F_cst, F_var>(nv[4].get(), 0, true, "", F_binded(0), F_cst(loop), F_var("?n")));
    an.push_back(create_alpha_node<F_var, F_cst, F_var>(nv[5].get(), 0, true, "", F_var("?s"), F_cst(top), F_var("?m")));
    // consequents
    an.push_back(create_alpha_node<F_binded, F_cst, F_binded>(nv[3].get(), 0, false, "", F_binded(0), F_cst(priority), F_binded(1)));
    an.push_back(create_alpha_node<F_binded, F_cst, F_binded>(nv[4].get(), 0, false, "", F_binded(0), F_cst(adj_loop), F_binded(1)));
    {
      auto head = create_expr_binary_operator<SortedHeadVisitor>(0,
        create_expr_binded_var(0),
        create_expr_cst(rdf::RdfAstType(rdf::NamedResource("cfg"))));
      an.push_back(create_alpha_node<F_binded, F_cst, F_expr>(nv[4].get(), 0, false, "", F_binded(0), F_cst(top), F_expr(head)));
    }
    an.push_back(create_alpha_node<F_binded, F_cst, F_binded>(nv[5].get(), 0, false, "", F_binded(0), F_cst(merge), F_binded(1)));

    this->rete_meta_store = create_rete_meta_store({}, an, nv);
    this->rete_meta_store->initialize();
    rmgr->set_locked();
    meta_graph->set_locked();

    this->rdf_session = rdf::create_rdf_session(meta_graph);
    this->rete_session = create_rete_session(this->rete_meta_store, this->rdf_session.get());
    this->rete_session->initialize();

    auto s = r("s");
    this->rdf_session->insert(s, jets->rdf__type, state);
    for(int i=1; i<=2; ++i) {
      auto m = r((std::string("m")+std::to_string(i)).c_str());
      this->rdf_session->insert(s, match, m);
      this->rdf_session->insert(m, weight, i);
    }
  }

  rdf::r_index r(char const* name)
  {
    return this->rdf_session->rmgr()->create_resource(std::string(name));
  }

  bool has(char const* p, char const* o)
  {
    return this->rdf_session->contains(r("s"), r(p), r(o));
  }

  // The jetrules_pool_worker loop, without the jets:completed rule: it runs a fixed
  // number of loops, which is enough to empty the two-item priority list.
  int run_loops(int n)
  {
    auto rmgr = this->rdf_session->rmgr();
    int err = this->rete_session->execute_rules();
    for(int i=1; i<=n and err==0; ++i) {
      this->rdf_session->insert(r("s"), r("loop"), rmgr->create_literal<int>(i));
      err = this->rete_session->execute_rules();
    }
    return err;
  }

  ReteSessionPtr     rete_session;
  ReteMetaStorePtr   rete_meta_store;
  rdf::RDFSessionPtr rdf_session;
};

TEST_F(LoopThroughNegationTest, LoopControlRulesTerminate) {
  this->build();
  ASSERT_EQ(this->run_loops(3), 0) << "the loop rules cycled: -10 is the vertex visit limit";
  EXPECT_TRUE(this->has("topMatch", "m1"));
  EXPECT_TRUE(this->has("topMatch", "m2"));
  EXPECT_TRUE(this->has("mergeList", "m1")) << "a merge was undone";
  EXPECT_TRUE(this->has("mergeList", "m2")) << "a merge was undone";
  EXPECT_FALSE(this->has("priorityList", "m1"));
  EXPECT_FALSE(this->has("priorityList", "m2"));
}

}   // namespace
}   // namespace jets::rete
