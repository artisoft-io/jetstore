#include <memory>
#include <string>

#include <gtest/gtest.h>

#include "../rdf/rdf_types.h"
#include "../rete/rete_types.h"
#include "../rete/rete_types_impl.h"
#include "../rete/expr_op_logics.h"
#include "../rete/expr_op_arithmetics.h"
#include "../rete/expr_op_resources.h"

// RETRACTION REPLAYS WHAT A BETA ROW INFERRED -- it does not recompute it.
//
// Until 2026-10-08 ReteSession::compute_consequent_triples retracted a beta row by
// evaluating its consequent terms a second time, against the withdrawn row, and
// retracting whatever came out. That is right only when the consequent is a pure
// function of the row. It is not for:
//
//   - create_entity 0, which mints a UUID: the retraction minted a SECOND entity,
//     inserted a stray jets:key triple for it, and retracted a link to it that did not
//     exist, leaving the real link in place.
//   - a keyed create_entity, which re-inserted jets:key on every retraction, so its
//     reference count never came back to zero.
//   - an aggregate, which read the graph as it stood at retraction: once its input
//     had changed, the triple retracted was not the triple inferred.
//
// Found by the usi_ws workspace assessment (jetstore_agentic_ai,
// projects/workspace_assessments/02_usi_ws_assessment, Q-37 / R-13), where rule ckd30
// was keyed as a workaround.
//
// THE FIX: a node vertex whose consequent terms are not row pure
// (NodeVertex::records_consequents, set by ReteMetaStore::initialize from
// AlphaNode::is_row_pure) records on the beta row every triple its consequents
// inferred, the jets:key triple create_entity inserts included, and retraction
// retracts that record and evaluates nothing.
//
// The fixture follows the shape of no_truth_main_on_exist_test.cc: THE RETRACTION IS
// DRIVEN BY A RULE. Vertex 1 carries [(?s exist_not done) == 1], so when vertex 2 puts
// (main1 done 1) in the graph during inference, the exist_not callback replays vertex 1
// and retracts its row inside the same compute_consequent_triples drain. The repeated
// cycle case also drives it from the test, by asserting and erasing (main1 done 1)
// between execute_rules calls; the callback queues the work and the next execute_rules
// drains it.
namespace jets::rete {
namespace {

enum ConsequentKind {
  kCreateEntityUuid,    // (?s link create_entity(0))
  kCreateEntityKeyed,   // (?s link create_entity("K1"))
  kSumValues,           // (?s total (?s sum_values sumConfig))
  kPure,                // (?s link (?s))  -- no expression that is not row pure
};

class RetractRecordedConsequentsTest : public ::testing::Test {
 protected:
  ~RetractRecordedConsequentsTest() {
    if(this->rete_session) this->rete_session->terminate();
  }

  // vertex 1  (?s rdf:type MainEntity).[(?s exist_not done) == 1]  -> consequent under test
  // vertex 2  (?s pendingDone ?o)                                -> (?s done ?o)
  // vertex 3  (?s pendingSupport ?o)                             -> (?s hasSupport ?o)
  //
  // Salience 10, 5 and 7: vertex 1 infers first, vertex 3 then changes the aggregate's
  // input, and vertex 2 retracts vertex 1's row last.
  void build(ConsequentKind kind)
  {
    auto meta_graph = rdf::create_rdf_graph();
    auto * rmgr = meta_graph->rmgr();
    rmgr->initialize();
    auto const* jets = rmgr->jets();

    auto me              = rmgr->create_resource("MainEntity");
    auto done            = rmgr->create_resource("done");
    auto pending_done    = rmgr->create_resource("pendingDone");
    auto has             = rmgr->create_resource("hasSupport");
    auto value           = rmgr->create_resource("value");
    auto pending_support = rmgr->create_resource("pendingSupport");
    auto link            = rmgr->create_resource("link");
    auto total           = rmgr->create_resource("total");

    auto sum_config = rmgr->create_resource("sumConfig");
    meta_graph->insert(sum_config, jets->jets__entity_property, has);
    meta_graph->insert(sum_config, jets->jets__value_property, value);

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

    // [(?s exist_not done) == 1]
    auto not_done = create_expr_binary_operator<EqVisitor>(0,
      create_expr_binary_operator<ExistNotVisitor>(0,
        create_expr_binded_var(0),
        create_expr_cst(rdf::RdfAstType(rdf::NamedResource("done")))),
      create_expr_cst(rdf::RdfAstType(rdf::LInt32(1))));

    NodeVertexVector node_vertexes;
    node_vertexes.push_back(create_node_vertex(nullptr, 0, 0, false, 100, {}, "", {}));
    node_vertexes.push_back(create_node_vertex(node_vertexes[0].get(), 0, 1, false, 10, not_done, "", ri1()));
    node_vertexes.push_back(create_node_vertex(node_vertexes[0].get(), 0, 2, false, 5, {}, "", ri2()));
    node_vertexes.push_back(create_node_vertex(node_vertexes[0].get(), 0, 3, false, 7, {}, "", ri2()));

    ReteMetaStore::AlphaNodeVector alpha_nodes;
    // Antecedent alpha nodes, one per node vertex and in the same order.
    alpha_nodes.push_back(create_alpha_node<F_var, F_var, F_var>(node_vertexes[0].get(), 0, true, "",
      F_var("*"), F_var("*"), F_var("*") ));
    alpha_nodes.push_back(create_alpha_node<F_var, F_cst, F_cst>(node_vertexes[1].get(), 0, true, "",
      F_var("?s"), F_cst(jets->rdf__type), F_cst(me) ));
    alpha_nodes.push_back(create_alpha_node<F_var, F_cst, F_var>(node_vertexes[2].get(), 0, true, "",
      F_var("?s"), F_cst(pending_done), F_var("?o") ));
    alpha_nodes.push_back(create_alpha_node<F_var, F_cst, F_var>(node_vertexes[3].get(), 0, true, "",
      F_var("?s"), F_cst(pending_support), F_var("?o") ));

    // Consequent alpha nodes.
    switch(kind) {
      case kCreateEntityUuid:
      case kCreateEntityKeyed: {
        auto arg = kind == kCreateEntityUuid ?
          create_expr_cst(rdf::RdfAstType(rdf::LInt32(0))) :
          create_expr_cst(rdf::RdfAstType(rdf::LString(std::string("K1"))));
        auto expr = create_expr_unary_operator<CreateEntityVisitor>(0, arg);
        alpha_nodes.push_back(create_alpha_node<F_binded, F_cst, F_expr>(node_vertexes[1].get(), 0, false, "",
          F_binded(0), F_cst(link), F_expr(expr) ));
        break;
      }
      case kSumValues: {
        auto expr = create_expr_binary_operator<SumValuesVisitor>(0,
          create_expr_binded_var(0),
          create_expr_cst(rdf::RdfAstType(rdf::NamedResource("sumConfig"))));
        alpha_nodes.push_back(create_alpha_node<F_binded, F_cst, F_expr>(node_vertexes[1].get(), 0, false, "",
          F_binded(0), F_cst(total), F_expr(expr) ));
        break;
      }
      case kPure:
        alpha_nodes.push_back(create_alpha_node<F_binded, F_cst, F_binded>(node_vertexes[1].get(), 0, false, "",
          F_binded(0), F_cst(link), F_binded(0) ));
        break;
    }
    alpha_nodes.push_back(create_alpha_node<F_binded, F_cst, F_binded>(node_vertexes[2].get(), 0, false, "",
      F_binded(0), F_cst(done), F_binded(1) ));
    alpha_nodes.push_back(create_alpha_node<F_binded, F_cst, F_binded>(node_vertexes[3].get(), 0, false, "",
      F_binded(0), F_cst(has), F_binded(1) ));

    this->rete_meta_store = create_rete_meta_store({}, alpha_nodes, node_vertexes);
    this->rete_meta_store->initialize();
    rmgr->set_locked();
    meta_graph->set_locked();

    this->rdf_session = rdf::create_rdf_session(meta_graph);
    this->rete_session = create_rete_session(this->rete_meta_store, this->rdf_session.get());
    this->rete_session->initialize();

    // main1 with three supports valued 1, 2 and 3: the sum is 6.
    auto srmgr = this->rdf_session->rmgr();
    auto main1 = srmgr->create_resource("main1");
    this->rdf_session->insert(main1, jets->rdf__type, me);
    for(int i=1; i<=3; ++i) {
      auto s = srmgr->create_resource(std::string("support")+std::to_string(i));
      this->rdf_session->insert(main1, has, s);
      this->rdf_session->insert(s, value, i);
    }
  }

  rdf::r_index r(char const* name)
  {
    return this->rdf_session->rmgr()->create_resource(std::string(name));
  }

  // Stage (main1 pendingDone 1): vertex 2 turns it into (main1 done 1) DURING inference.
  void stage_done()
  {
    this->rdf_session->insert(r("main1"), r("pendingDone"), 1);
  }

  // Stage support4, valued 4, to be linked by vertex 3 during inference.
  void stage_support4()
  {
    this->rdf_session->insert(r("support4"), r("value"), 4);
    this->rdf_session->insert(r("main1"), r("pendingSupport"), r("support4"));
  }

  // Every (?x link ?y) triple, in any graph.
  int count_links()
  {
    int n = 0;
    auto itor = this->rdf_session->find(r("main1"), r("link"));
    while(not itor.is_end()) { ++n; itor.next(); }
    return n;
  }

  // Every jets:key triple in the inferred graph: one per entity create_entity minted
  // and has not retracted.
  int count_inferred_keys()
  {
    auto jets_key = this->rdf_session->rmgr()->jets()->jets__key;
    int n = 0;
    auto itor = this->rdf_session->inferred_graph()->find();
    while(not itor.is_end()) {
      if(itor.get_predicate() == jets_key) ++n;
      itor.next();
    }
    return n;
  }

  int k1_key_ref_count()
  {
    auto rmgr = this->rdf_session->rmgr();
    return this->rdf_session->inferred_graph()->get_ref_count(
      r("K1"), rmgr->jets()->jets__key, rmgr->create_literal(std::string("K1")));
  }

  ReteSessionPtr     rete_session;
  ReteMetaStorePtr   rete_meta_store;
  rdf::RDFSessionPtr rdf_session;
};

// The flag is set where it should be and nowhere else: an expression that is not row
// pure makes the vertex record, a consequent with no such expression does not.
TEST_F(RetractRecordedConsequentsTest, OnlyVertexesWithImpureConsequentsRecord) {
  this->build(kCreateEntityUuid);
  EXPECT_TRUE(this->rete_meta_store->get_node_vertex(1)->records_consequents);
  EXPECT_FALSE(this->rete_meta_store->get_node_vertex(2)->records_consequents);
  EXPECT_FALSE(this->rete_meta_store->get_node_vertex(3)->records_consequents);
}

TEST_F(RetractRecordedConsequentsTest, PureConsequentDoesNotRecord) {
  this->build(kPure);
  EXPECT_FALSE(this->rete_meta_store->get_node_vertex(1)->records_consequents);
}

TEST_F(RetractRecordedConsequentsTest, AggregateConsequentRecords) {
  this->build(kSumValues);
  EXPECT_TRUE(this->rete_meta_store->get_node_vertex(1)->records_consequents);
}

// The control: nothing retracts, so the link and one jets:key are there.
TEST_F(RetractRecordedConsequentsTest, CreateEntityUuidInfersOneEntity) {
  this->build(kCreateEntityUuid);
  ASSERT_EQ(this->rete_session->execute_rules(), 0);
  EXPECT_EQ(this->count_links(), 1);
  EXPECT_EQ(this->count_inferred_keys(), 1);
}

// THE DEFECT. Before the fix this left the link in place (1) and two jets:key triples,
// one for the entity inferred and one for the entity the retraction minted.
TEST_F(RetractRecordedConsequentsTest, CreateEntityUuidRetractedLeavesNothing) {
  this->build(kCreateEntityUuid);
  this->stage_done();
  ASSERT_EQ(this->rete_session->execute_rules(), 0);
  EXPECT_TRUE(this->rdf_session->contains(r("main1"), r("done"), this->rdf_session->rmgr()->create_literal(1)))
    << "the retraction driver fired";
  EXPECT_EQ(this->count_links(), 0) << "the link inferred is the link retracted";
  EXPECT_EQ(this->count_inferred_keys(), 0)
    << "no second entity was minted, and the first entity's jets:key went with its row";
}

// A keyed entity: the jets:key triple is retracted with the row that inferred it,
// so its reference count reaches zero rather than being raised by the retraction.
TEST_F(RetractRecordedConsequentsTest, CreateEntityKeyedRetractedReleasesKey) {
  this->build(kCreateEntityKeyed);
  this->stage_done();
  ASSERT_EQ(this->rete_session->execute_rules(), 0);
  EXPECT_EQ(this->count_links(), 0);
  EXPECT_EQ(this->k1_key_ref_count(), 0);
}

// Repeated assert / retract cycles, driven from the test: retracting (main1 done 1)
// re-admits vertex 1's row, inserting it withdraws the row again. The reference count
// of the keyed entity's jets:key stays at 1 while the row stands and 0 while it does
// not, however many cycles run -- before the fix it went up by two per cycle.
TEST_F(RetractRecordedConsequentsTest, CreateEntityKeyedCyclesKeepRefCountBounded) {
  this->build(kCreateEntityKeyed);
  auto one = this->rdf_session->rmgr()->create_literal(1);
  ASSERT_EQ(this->rete_session->execute_rules(), 0);
  ASSERT_EQ(this->count_links(), 1);
  ASSERT_EQ(this->k1_key_ref_count(), 1);
  for(int cycle=0; cycle<5; ++cycle) {
    this->rdf_session->insert_inferred(r("main1"), r("done"), one);
    ASSERT_EQ(this->rete_session->execute_rules(), 0);
    EXPECT_EQ(this->count_links(), 0) << "cycle " << cycle << ", retracted";
    EXPECT_EQ(this->k1_key_ref_count(), 0) << "cycle " << cycle << ", retracted";

    this->rdf_session->retract(r("main1"), r("done"), one);
    ASSERT_EQ(this->rete_session->execute_rules(), 0);
    EXPECT_EQ(this->count_links(), 1) << "cycle " << cycle << ", re-inferred";
    EXPECT_EQ(this->k1_key_ref_count(), 1) << "cycle " << cycle << ", re-inferred";
  }
}

// The same cycles with create_entity 0: each re-inference mints a new entity, and each
// retraction takes that entity's jets:key with it, so at most one is ever standing.
TEST_F(RetractRecordedConsequentsTest, CreateEntityUuidCyclesLeaveNoStrayKeys) {
  this->build(kCreateEntityUuid);
  auto one = this->rdf_session->rmgr()->create_literal(1);
  ASSERT_EQ(this->rete_session->execute_rules(), 0);
  for(int cycle=0; cycle<5; ++cycle) {
    this->rdf_session->insert_inferred(r("main1"), r("done"), one);
    ASSERT_EQ(this->rete_session->execute_rules(), 0);
    EXPECT_EQ(this->count_links(), 0) << "cycle " << cycle;
    EXPECT_EQ(this->count_inferred_keys(), 0) << "cycle " << cycle;

    this->rdf_session->retract(r("main1"), r("done"), one);
    ASSERT_EQ(this->rete_session->execute_rules(), 0);
    EXPECT_EQ(this->count_links(), 1) << "cycle " << cycle;
    EXPECT_EQ(this->count_inferred_keys(), 1) << "cycle " << cycle;
  }
}

// A consequent aggregate whose input changes between inference and retraction. The
// row infers (main1 total 6); vertex 3 then links support4, so the sum is 10 by the
// time vertex 2 retracts the row. Recomputing retracted (main1 total 10), which was
// never inferred, and left (main1 total 6) standing.
TEST_F(RetractRecordedConsequentsTest, ConsequentAggregateRetractsWhatItInferred) {
  this->build(kSumValues);
  this->stage_support4();
  this->stage_done();
  ASSERT_EQ(this->rete_session->execute_rules(), 0);
  auto rmgr = this->rdf_session->rmgr();
  EXPECT_TRUE(this->rdf_session->contains(r("main1"), r("hasSupport"), r("support4")))
    << "the aggregate's input changed before the retraction";
  EXPECT_FALSE(this->rdf_session->contains(r("main1"), r("total"), rmgr->create_literal<int>(6)))
    << "the total inferred is retracted";
  EXPECT_FALSE(this->rdf_session->contains(r("main1"), r("total"), rmgr->create_literal<int>(10)))
    << "and no total is left standing";
}

// The pure path is unchanged: recomputing a consequent with no impure expression still
// retracts what it inferred.
TEST_F(RetractRecordedConsequentsTest, PureConsequentStillRetracts) {
  this->build(kPure);
  this->stage_done();
  ASSERT_EQ(this->rete_session->execute_rules(), 0);
  EXPECT_EQ(this->count_links(), 0);
}

// THE SAME CYCLES ON A PURE CONSEQUENT, which the recording does not touch. This
// failed before the recording was added as well: once a row had been retracted it was
// left in its beta relation marked kProcessed -- compute_consequent_triples called
// remove_beta_row on a row already marked kDeleted, which returns without erasing it --
// so an equal row arriving later was taken for one already inferred and never fired.
TEST_F(RetractRecordedConsequentsTest, PureConsequentCyclesReinfer) {
  this->build(kPure);
  auto one = this->rdf_session->rmgr()->create_literal(1);
  ASSERT_EQ(this->rete_session->execute_rules(), 0);
  ASSERT_EQ(this->count_links(), 1);
  for(int cycle=0; cycle<5; ++cycle) {
    this->rdf_session->insert_inferred(r("main1"), r("done"), one);
    ASSERT_EQ(this->rete_session->execute_rules(), 0);
    EXPECT_EQ(this->count_links(), 0) << "cycle " << cycle;

    this->rdf_session->retract(r("main1"), r("done"), one);
    ASSERT_EQ(this->rete_session->execute_rules(), 0);
    EXPECT_EQ(this->count_links(), 1) << "cycle " << cycle;
  }
}

}   // namespace
}   // namespace jets::rete
