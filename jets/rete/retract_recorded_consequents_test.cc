#include <memory>
#include <string>

#include <gtest/gtest.h>

#include "../rdf/rdf_types.h"
#include "../rete/rete_types.h"
#include "../rete/rete_types_impl.h"
#include "../rete/expr_op_logics.h"
#include "../rete/expr_op_arithmetics.h"
#include "../rete/expr_op_resources.h"

// RETRACTION REPLAYS WHAT A MINTING CONSEQUENT INFERRED -- and only that.
//
// Until 2026-10-08 ReteSession::compute_consequent_triples retracted a beta row by
// evaluating its consequent terms a second time, against the withdrawn row, and
// retracting whatever came out. For a consequent that mints a resource that is wrong:
//
//   - create_entity 0 minted a SECOND entity on retraction, inserted a stray jets:key
//     triple for it, and retracted a link to it that did not exist, leaving the real
//     link in place.
//   - a keyed create_entity re-inserted jets:key on every retraction, so its reference
//     count never came back to zero.
//
// Found by the usi_ws workspace assessment (jetstore_agentic_ai,
// projects/workspace_assessments/02_usi_ws_assessment, Q-37 / R-13), where rule ckd30
// was keyed as a workaround.
//
// THE FIX: a node vertex with a consequent that mints a resource -- create_entity,
// create_uuid_resource, range, lookup_rand, multi_lookup_rand
// (NodeVertex::records_consequents, set by ReteMetaStore::initialize from
// AlphaNode::mints_resource) -- records on the beta row every triple its consequents
// inferred, the jets:key triple create_entity inserts included, and retraction
// retracts that record and evaluates nothing.
//
// TWO NEIGHBOURING DEFECTS ARE KEPT, and the cases marked LEGACY pin them so that
// they are visible rather than assumed. A consequent aggregate is still recomputed on
// retraction, and a withdrawn row is never erased from its relation, so an equal row
// arriving later never fires. Fixing both was tried on 2026-10-08 and reverted on
// 2026-10-09 by Michel's decision: usi_ws's loop rules and CM04 op01 terminate only
// because of them (see loop_through_negation_test.cc and
// jets/jetrules/rete/README.md).
//
// The fixture follows the shape of no_truth_main_on_exist_test.cc: THE RETRACTION IS
// DRIVEN BY A RULE. Vertex 1 carries [(?s exist_not done) == 1], so when vertex 2 puts
// (main1 done 1) in the graph during inference, the exist_not callback replays vertex 1
// and retracts its row inside the same compute_consequent_triples drain. The repeated
// cycle cases drive it from the test instead, by inserting (main1 done 1) into the
// inferred graph and retracting it between execute_rules calls; the callback queues the
// work and the next execute_rules drains it. Not insert and erase: RDFGraph::erase does
// not notify the graph callbacks, only insert and retract do, so an erased triple is
// never seen by truth maintenance.
namespace jets::rete {
namespace {

enum ConsequentKind {
  kCreateEntityUuid,    // (?s link create_entity(0))
  kCreateEntityKeyed,   // (?s link create_entity("K1"))
  kSumValues,           // (?s total (?s sum_values sumConfig))
  kPure,                // (?s link ?s)  -- mints nothing
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

// The flag is set where it should be and nowhere else: a consequent that mints a
// resource makes the vertex record, one that does not, does not.
TEST_F(RetractRecordedConsequentsTest, OnlyVertexesWithMintingConsequentsRecord) {
  this->build(kCreateEntityUuid);
  EXPECT_TRUE(this->rete_meta_store->get_node_vertex(1)->records_consequents);
  EXPECT_FALSE(this->rete_meta_store->get_node_vertex(2)->records_consequents);
  EXPECT_FALSE(this->rete_meta_store->get_node_vertex(3)->records_consequents);
}

TEST_F(RetractRecordedConsequentsTest, KeyedCreateEntityRecords) {
  this->build(kCreateEntityKeyed);
  EXPECT_TRUE(this->rete_meta_store->get_node_vertex(1)->records_consequents);
}

TEST_F(RetractRecordedConsequentsTest, PureConsequentDoesNotRecord) {
  this->build(kPure);
  EXPECT_FALSE(this->rete_meta_store->get_node_vertex(1)->records_consequents);
}

// LEGACY: an aggregate reads the graph but mints nothing, and is deliberately left out
// of the set, so its consequent is recomputed on retraction.
TEST_F(RetractRecordedConsequentsTest, AggregateConsequentDoesNotRecord) {
  this->build(kSumValues);
  EXPECT_FALSE(this->rete_meta_store->get_node_vertex(1)->records_consequents);
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

// Repeated assert / retract cycles on the keyed entity. Two things are pinned at once:
//
//  - LEGACY: once the row is retracted it is never inferred again, though its support
//    comes back -- the withdrawn row stays in the relation and an equal row is taken
//    for it.
//  - THE FIX: every later withdrawal retracts that stale row again, and it replays its
//    now empty record. Before the record was kept empty, that second retraction fell
//    back to recomputing create_entity("K1") and re-inserted jets:key, so the count
//    climbed by one per cycle with no row standing at all.
TEST_F(RetractRecordedConsequentsTest, CreateEntityKeyedCyclesStayRetracted) {
  this->build(kCreateEntityKeyed);
  auto one = this->rdf_session->rmgr()->create_literal(1);
  ASSERT_EQ(this->rete_session->execute_rules(), 0);
  ASSERT_EQ(this->count_links(), 1);
  ASSERT_EQ(this->k1_key_ref_count(), 1);
  for(int cycle=0; cycle<5; ++cycle) {
    this->rdf_session->insert_inferred(r("main1"), r("done"), one);
    ASSERT_EQ(this->rete_session->execute_rules(), 0);
    EXPECT_EQ(this->count_links(), 0) << "cycle " << cycle << ", withdrawn";
    EXPECT_EQ(this->k1_key_ref_count(), 0) << "cycle " << cycle << ", withdrawn";

    this->rdf_session->retract(r("main1"), r("done"), one);
    ASSERT_EQ(this->rete_session->execute_rules(), 0);
    EXPECT_EQ(this->count_links(), 0) << "cycle " << cycle << ", re-admitted: LEGACY, does not re-fire";
    EXPECT_EQ(this->k1_key_ref_count(), 0) << "cycle " << cycle << ", re-admitted";
  }
}

// The same cycles with create_entity 0: no stray entity is ever minted.
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
    EXPECT_EQ(this->count_links(), 0) << "cycle " << cycle << ": LEGACY, does not re-fire";
    EXPECT_EQ(this->count_inferred_keys(), 0) << "cycle " << cycle;
  }
}

// LEGACY: a consequent aggregate whose input changes between inference and retraction.
// The row infers (main1 total 6); vertex 3 then links support4, so the sum is 10 by the
// time vertex 2 retracts the row. Retraction recomputes, retracts (main1 total 10),
// which was never inferred, and leaves (main1 total 6) standing. If this starts failing,
// aggregate consequents have been given replay -- read the README entry first.
TEST_F(RetractRecordedConsequentsTest, ConsequentAggregateRetractionRecomputes) {
  this->build(kSumValues);
  this->stage_support4();
  this->stage_done();
  ASSERT_EQ(this->rete_session->execute_rules(), 0);
  auto rmgr = this->rdf_session->rmgr();
  ASSERT_TRUE(this->rdf_session->contains(r("main1"), r("hasSupport"), r("support4")))
    << "the aggregate's input changed before the retraction";
  EXPECT_TRUE(this->rdf_session->contains(r("main1"), r("total"), rmgr->create_literal<int>(6)))
    << "LEGACY: the total inferred is still standing";
  EXPECT_FALSE(this->rdf_session->contains(r("main1"), r("total"), rmgr->create_literal<int>(10)));
}

// The pure path is unchanged: recomputing a consequent that mints nothing retracts
// what it inferred.
TEST_F(RetractRecordedConsequentsTest, PureConsequentStillRetracts) {
  this->build(kPure);
  this->stage_done();
  ASSERT_EQ(this->rete_session->execute_rules(), 0);
  EXPECT_EQ(this->count_links(), 0);
}

// LEGACY, on a consequent the recording does not touch: once a row has been retracted
// it is left in its beta relation marked kProcessed -- compute_consequent_triples calls
// remove_beta_row on a row already marked kDeleted, which returns without erasing it --
// so an equal row arriving later is taken for one already inferred and never fires.
TEST_F(RetractRecordedConsequentsTest, PureConsequentDoesNotRefire) {
  this->build(kPure);
  auto one = this->rdf_session->rmgr()->create_literal(1);
  ASSERT_EQ(this->rete_session->execute_rules(), 0);
  ASSERT_EQ(this->count_links(), 1);
  this->rdf_session->insert_inferred(r("main1"), r("done"), one);
  ASSERT_EQ(this->rete_session->execute_rules(), 0);
  EXPECT_EQ(this->count_links(), 0);
  this->rdf_session->retract(r("main1"), r("done"), one);
  ASSERT_EQ(this->rete_session->execute_rules(), 0);
  EXPECT_EQ(this->count_links(), 0) << "LEGACY: the support is back and the rule does not fire";
}

}   // namespace
}   // namespace jets::rete
