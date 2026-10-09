#include <memory>
#include <string>

#include <gtest/gtest.h>

#include "../rdf/rdf_types.h"
#include "../rete/rete_types.h"
#include "../rete/rete_types_impl.h"
#include "../rete/expr_op_logics.h"
#include "../rete/expr_op_resources.h"

// _0:no_truth_main_on_exist -- A PRODUCTION FLAG WITH NO TEST.
//
// The string appeared in no _test.cc and no _test.go until this file. It is not a
// laboratory switch: workspaces/usi_ws sets it, in
// pipes_config/usiclaim_processes.pc.json, under jetrules_config.rule_config as
// {"_0:no_truth_main_on_exist": {"value": "1", "type": "int"}}, because that client's
// rules predate rule filters participating in truth maintenance (jetstore, 2023-04-26,
// "Fixes #699"). One customer's rules behave differently depending on this flag and
// nothing asserted what it does.
//
// THE MECHANISM, in three places:
//
//   - ReteSession::initialize (jets/rete/rete_session.cc:32-45) looks up the resource
//     _0:no_truth_main_on_exist and reads (jets:iState, that, ?v) out of the rule_config
//     graph. The rule_config triples land in the META graph -- InsertRuleConfig calls
//     insert_meta_graph (jets/bridge/bridge.go:331) -- which is why the fixture below
//     puts them there rather than asserting them.
//
//   - The default is 0, TRUTH MAINTENANCE ON, and it is the default twice over: the
//     member is initialised to 0 (jets/rete/rete_session.h:66) and initialize leaves it
//     alone when the resource is absent, and sets it to 0 again when the value is
//     present but is not an int32.
//
//   - ExistVisitor::register_callback and ExistNotVisitor::register_callback
//     (jets/rete/expr_op_resources.h:145 and :176) return 0 without registering
//     anything when it is 1.
//
// THE TESTS ARE BEHAVIOURAL, not member reads. Reading get_no_truth_main_on_exist back
// would assert that initialize parsed a triple, which is the easy half; what the flag
// CLAIMS is that a rule whose filter tests exist / exist_not stops being replayed when
// the graph moves under it. So the cases below run rules over a small rete network and
// look at what was inferred. The three member-read cases at the end cover the parsing
// branches that no behaviour distinguishes.
//
// TWO PROPERTIES OF THE FIXTURE ARE LOAD BEARING, and they are the same two that
// AggregateTruthMaintenanceTest in expr_op_specialty_test.cc documents:
//
//   - THE OPERATOR IS IN A FILTER. ReteSession::set_graph_callbacks registers a node
//     vertex's antecedent alpha node and its filter expression and nothing else, so an
//     exist in a consequent never registers a callback and this flag could not reach it.
//
//   - THE GRAPH IS CHANGED BY A RULE, not by the test. Callbacks fire into the
//     consequent queue that compute_consequent_triples is draining; a triple inserted
//     after execute_rules has returned replays the vertex with no drain left to run and
//     would show nothing either way. That is what the `pending` rule below is for.
namespace jets::rete {
namespace {

// Absent means the resource is created but no (jets:iState, ...) triple is written,
// which is the state of every workspace except usi_ws.
enum FlagState { kFlagAbsent, kFlagOne, kFlagZero, kFlagNotAnInt };

class NoTruthMainOnExistTest : public ::testing::Test {
 protected:
  ~NoTruthMainOnExistTest() {
    if(this->rete_session) this->rete_session->terminate();
  }

  // build stands up a rete network carrying two rules, both with `?s rdf:type
  // MainEntity` as their antecedent:
  //
  //   vertex 1  [(?s exist gotValue) == 1]      -> (?s existFlag 1)
  //   vertex 2  [(?s exist_not gotValue) == 1]  -> (?s existNotFlag 1)
  //   vertex 3  (?s pendingValue ?o)            -> (?s gotValue ?o)
  //
  // Vertices 1 and 2 have the higher salience, so both are evaluated before vertex 3's
  // consequent puts gotValue in the graph. What happens next is the whole question:
  // with truth maintenance the gotValue triple replays them, so existFlag is asserted
  // and existNotFlag is retracted; with the flag set to 1 no callback exists, so
  // neither vertex is ever looked at again.
  void build(FlagState flag)
  {
    auto meta_graph = rdf::create_rdf_graph();
    auto * rmgr = meta_graph->rmgr();
    rmgr->initialize();
    auto const* jets = rmgr->jets();

    auto me            = rmgr->create_resource("MainEntity");
    auto got_value     = rmgr->create_resource("gotValue");
    auto pending_value = rmgr->create_resource("pendingValue");
    auto exist_flag    = rmgr->create_resource("existFlag");
    auto exist_not_flag = rmgr->create_resource("existNotFlag");
    auto one           = rmgr->create_literal<int>(1);

    // The flag itself. The resource is created in every arm because
    // ReteSession::initialize calls get_resource, not create_resource: an arm that did
    // not create it would be testing that the lookup misses rather than that the value
    // is read.
    auto no_tm = rmgr->create_resource("_0:no_truth_main_on_exist");
    switch(flag) {
      case kFlagAbsent:    break;
      case kFlagOne:       meta_graph->insert(jets->jets__istate, no_tm, 1); break;
      case kFlagZero:      meta_graph->insert(jets->jets__istate, no_tm, 0); break;
      case kFlagNotAnInt:  meta_graph->insert(jets->jets__istate, no_tm, std::string("1")); break;
    }

    auto ri1 = []() {
      auto ri = create_row_initializer(1);
      ri->put(0, 0 | brc_triple, "?s");
      return ri;
    };
    auto ri2 = []() {
      auto ri = create_row_initializer(2);
      ri->put(0, 0 | brc_triple, "?s");
      ri->put(1, 2 | brc_triple, "?o");
      return ri;
    };

    // [(?s exist gotValue) == 1]
    auto exist_filter = create_expr_binary_operator<EqVisitor>(0,
      create_expr_binary_operator<ExistVisitor>(0,
        create_expr_binded_var(0),
        create_expr_cst(rdf::RdfAstType(rdf::NamedResource("gotValue")))),
      create_expr_cst(rdf::RdfAstType(rdf::LInt32(1))));

    // [(?s exist_not gotValue) == 1]
    auto exist_not_filter = create_expr_binary_operator<EqVisitor>(0,
      create_expr_binary_operator<ExistNotVisitor>(0,
        create_expr_binded_var(0),
        create_expr_cst(rdf::RdfAstType(rdf::NamedResource("gotValue")))),
      create_expr_cst(rdf::RdfAstType(rdf::LInt32(1))));

    NodeVertexVector node_vertexes;
    node_vertexes.push_back(create_node_vertex(nullptr, 0, 0, false, 100, {}, "", {}));
    node_vertexes.push_back(create_node_vertex(node_vertexes[0].get(), 0, 1, false, 10, exist_filter, "", ri1()));
    node_vertexes.push_back(create_node_vertex(node_vertexes[0].get(), 0, 2, false, 10, exist_not_filter, "", ri1()));
    node_vertexes.push_back(create_node_vertex(node_vertexes[0].get(), 0, 3, false, 5, {}, "", ri2()));

    ReteMetaStore::AlphaNodeVector alpha_nodes;
    // Antecedent alpha nodes, one per node vertex and in the same order:
    // set_graph_callbacks indexes them by position.
    alpha_nodes.push_back(create_alpha_node<F_var, F_var, F_var>(node_vertexes[0].get(), 0, true, "",
      F_var("*"), F_var("*"), F_var("*") ));
    for(int v=1; v<=2; ++v) {
      alpha_nodes.push_back(create_alpha_node<F_var, F_cst, F_cst>(node_vertexes[v].get(), 0, true, "",
        F_var("?s"), F_cst(jets->rdf__type), F_cst(me) ));
    }
    alpha_nodes.push_back(create_alpha_node<F_var, F_cst, F_var>(node_vertexes[3].get(), 0, true, "",
      F_var("?s"), F_cst(pending_value), F_var("?o") ));

    // Consequent alpha nodes.
    alpha_nodes.push_back(create_alpha_node<F_binded, F_cst, F_cst>(node_vertexes[1].get(), 0, false, "",
      F_binded(0), F_cst(exist_flag), F_cst(one) ));
    alpha_nodes.push_back(create_alpha_node<F_binded, F_cst, F_cst>(node_vertexes[2].get(), 0, false, "",
      F_binded(0), F_cst(exist_not_flag), F_cst(one) ));
    alpha_nodes.push_back(create_alpha_node<F_binded, F_cst, F_binded>(node_vertexes[3].get(), 0, false, "",
      F_binded(0), F_cst(got_value), F_binded(1) ));

    this->rete_meta_store = create_rete_meta_store({}, alpha_nodes, node_vertexes);
    this->rete_meta_store->initialize();
    rmgr->set_locked();
    meta_graph->set_locked();

    this->rdf_session = rdf::create_rdf_session(meta_graph);
    this->rete_session = create_rete_session(this->rete_meta_store, this->rdf_session.get());
    this->rete_session->initialize();

    auto srmgr = this->rdf_session->rmgr();
    auto main1 = srmgr->create_resource("main1");
    this->rdf_session->insert(main1, jets->rdf__type, me);
  }

  // Stage the triple that the pending rule turns into (main1, gotValue, 42) DURING
  // inference. main1 has no gotValue when the two filters are first evaluated.
  void stage_pending_value()
  {
    auto rmgr = this->rdf_session->rmgr();
    this->rdf_session->insert(
      rmgr->create_resource("main1"), rmgr->create_resource("pendingValue"), 42);
  }

  bool has_flag(char const* flag)
  {
    auto rmgr = this->rdf_session->rmgr();
    return this->rdf_session->contains(
      rmgr->create_resource("main1"),
      rmgr->create_resource(std::string(flag)),
      rmgr->create_literal<int>(1));
  }

  ReteSessionPtr     rete_session;
  ReteMetaStorePtr   rete_meta_store;
  rdf::RDFSessionPtr rdf_session;
};

// THE DEFAULT IS ON, and this is the case that says what the flag turns off.
// main1 has no gotValue when vertex 1 is evaluated, so the filter is false and nothing
// is inferred at that point. The pending rule then gives it one, the callback registered
// by ExistVisitor::register_callback replays vertex 1, and the consequent fires.
TEST_F(NoTruthMainOnExistTest, ExistIsMaintainedWhenTheFlagIsAbsent) {
  this->build(kFlagAbsent);
  this->stage_pending_value();

  this->rete_session->execute_rules();

  EXPECT_EQ(this->rete_session->get_no_truth_main_on_exist(), 0);
  EXPECT_TRUE(this->has_flag("existFlag"))
    << "(main1 exist gotValue) was false when the term was first evaluated and is true "
       "once the pending rule asserts gotValue";
}

// THE FLAG SET TO 1, the same graph, the opposite outcome. Nothing registers a callback
// on gotValue, so vertex 1 is evaluated once, against a graph in which main1 has no
// gotValue, and is never looked at again.
TEST_F(NoTruthMainOnExistTest, ExistIsNotMaintainedWhenTheFlagIsOne) {
  this->build(kFlagOne);
  this->stage_pending_value();

  this->rete_session->execute_rules();

  EXPECT_EQ(this->rete_session->get_no_truth_main_on_exist(), 1);
  EXPECT_FALSE(this->has_flag("existFlag"))
    << "with _0:no_truth_main_on_exist set to 1 the exist term is evaluated once and "
       "the gotValue triple that makes it true goes unnoticed";
}

// exist_not is the other half of the flag and it fails in the other direction: the
// filter is TRUE at first, so the consequent IS inferred, and truth maintenance is what
// takes it back when gotValue arrives.
TEST_F(NoTruthMainOnExistTest, ExistNotIsMaintainedWhenTheFlagIsAbsent) {
  this->build(kFlagAbsent);
  this->stage_pending_value();

  this->rete_session->execute_rules();

  EXPECT_FALSE(this->has_flag("existNotFlag"))
    << "(main1 exist_not gotValue) was true when the term was first evaluated and is "
       "false once gotValue is asserted, so the inferred triple is retracted";
}

TEST_F(NoTruthMainOnExistTest, ExistNotIsNotMaintainedWhenTheFlagIsOne) {
  this->build(kFlagOne);
  this->stage_pending_value();

  this->rete_session->execute_rules();

  EXPECT_TRUE(this->has_flag("existNotFlag"))
    << "with the flag set to 1 nothing retracts what exist_not inferred before gotValue "
       "arrived -- which is the legacy behaviour usi_ws asks for";
}

// THE CONTROL. Neither flag depends on the pending rule alone: without it the graph
// never moves, and both arms agree. If this case ever diverged, the two above would be
// measuring the fixture rather than the flag.
TEST_F(NoTruthMainOnExistTest, WithNoChangeToTheGraphBothArmsAgree) {
  this->build(kFlagAbsent);
  this->rete_session->execute_rules();
  EXPECT_FALSE(this->has_flag("existFlag"));
  EXPECT_TRUE(this->has_flag("existNotFlag"));
}

// The parsing branches of ReteSession::initialize. No behaviour distinguishes these
// from the absent case, so they are read back off the member -- which is the weaker
// assertion and is why it is only used where the stronger one says nothing.
TEST_F(NoTruthMainOnExistTest, AnExplicitZeroReadsAsZero) {
  this->build(kFlagZero);
  EXPECT_EQ(this->rete_session->get_no_truth_main_on_exist(), 0);
}

// A value that is not an int32 falls back to 0, TRUTH MAINTENANCE ON. The pipes config
// declares the type as "int"; a workspace that wrote "1" as text would get the default
// rather than the behaviour it asked for, and that is worth pinning because it fails
// silently -- the flag is only logged when it reads as 1.
TEST_F(NoTruthMainOnExistTest, ANonIntValueFallsBackToTruthMaintenanceOn) {
  this->build(kFlagNotAnInt);
  EXPECT_EQ(this->rete_session->get_no_truth_main_on_exist(), 0);
}

}   // namespace
}   // namespace jets::rete
