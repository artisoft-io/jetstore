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
// Found by a client workspace's assessment (jetstore_agentic_ai, its Q-37 / R-13),
// where one rule's create_entity was keyed as a workaround.
//
// THE FIX: a node vertex with a consequent that mints a resource -- create_entity,
// create_uuid_resource, range, lookup_rand, multi_lookup_rand
// (NodeVertex::records_consequents, set by ReteMetaStore::initialize from
// AlphaNode::mints_resource) -- records on the beta row every triple its consequents
// inferred, the jets:key triple create_entity inserts included, and retraction
// retracts that record and evaluates nothing.
//
// TWO NEIGHBOURING BEHAVIOURS FOLLOW FROM THE SAME RULE. A consequent that reads the
// graph (an aggregate, exist, size_of) records too, so retraction replays it rather
// than recomputing it against a graph that has moved; and a withdrawn row is erased
// from its relation, so an equal row arriving later fires again. Both were kept as
// LEGACY defects from 2026-10-09, selectable off for the rules unit test harness only,
// and became the engine's only behaviour in 2026-10, when the mode was deleted (see
// loop_through_negation_test.cc and jets/rete/README.md). Four cases were renamed then,
// from the defect they pinned to the behaviour they assert: AggregateConsequentRecords,
// CreateEntityKeyedCyclesRefire, ConsequentAggregateRetractionReplays and
// PureConsequentRefires.
//
// The fixture follows the shape of no_truth_main_on_exist_test.cc: THE RETRACTION IS
// DRIVEN BY A RULE. Vertex 1 carries [(?s exist_not done) == 1], so when vertex 2 puts
// (main1 done 1) in the graph during inference, the exist_not callback replays vertex 1
// and retracts its row inside the same compute_consequent_triples drain. The repeated
// cycle cases drive it from the test instead, by inserting (main1 done 1) into the
// inferred graph and retracting it between execute_rules calls; the callback queues the
// work and the next execute_rules drains it. Not insert and erase: retract is the rete's
// own retraction, which these cases exercise. (RDFGraph::erase did not notify the graph
// callbacks until 2026-10; it does now, and EraseTellsTheReteTest below covers it.)
//
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
// resource makes the vertex record, one that mints nothing and reads nothing never does.
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

// An aggregate reads the graph and mints nothing, and its vertex records: recomputed
// on retraction, it would retract a triple that was never inferred (LEGACY defect 1
// until 2026-10).
TEST_F(RetractRecordedConsequentsTest, AggregateConsequentRecords) {
  this->build(kSumValues);
  EXPECT_TRUE(this->rete_meta_store->get_node_vertex(1)->records_consequents);
}

// The two predicates, on the consequents the fixture builds.
TEST_F(RetractRecordedConsequentsTest, ConsequentPredicates) {
  struct { ConsequentKind kind; bool mints; bool reads; } cases[] = {
    {kCreateEntityUuid, true, false}, {kCreateEntityKeyed, true, false},
    {kSumValues, false, true}, {kPure, false, false}};
  for(auto const& c: cases) {
    this->build(c.kind);
    // the consequent under test is the first alpha node past the antecedents
    auto const* an = this->rete_meta_store->get_alpha_node(4);
    EXPECT_EQ(an->mints_resource(), c.mints) << "kind " << c.kind;
    EXPECT_EQ(an->reads_graph(), c.reads) << "kind " << c.kind;
    this->rete_session->terminate();
    this->rete_session.reset();
  }
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

// Repeated assert / retract cycles on the keyed entity. Withdrawn, nothing stands.
// Re-admitted, the withdrawn row was erased and an equal one fires, so the link and one
// jets:key are back on every cycle. Until 2026-10 (LEGACY defect 2) the withdrawn row
// stayed in the relation and an equal row was taken for it, so the row was never
// inferred again; before the record was kept empty, each later withdrawal recomputed
// create_entity("K1") and the jets:key count climbed by one per cycle.
TEST_F(RetractRecordedConsequentsTest, CreateEntityKeyedCyclesRefire) {
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
    EXPECT_EQ(this->count_links(), 1) << "cycle " << cycle << ", re-admitted";
    EXPECT_EQ(this->k1_key_ref_count(), 1) << "cycle " << cycle << ", re-admitted";
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
    EXPECT_EQ(this->count_links(), 1) << "cycle " << cycle << ", re-admitted";
    EXPECT_EQ(this->count_inferred_keys(), 1) << "cycle " << cycle << ", re-admitted";
  }
}

// A consequent aggregate whose input changes between inference and retraction. The
// row infers (main1 total 6); vertex 3 then links support4, so the sum is 10 by the
// time vertex 2 retracts the row. The row replays its record and 6 goes. Until 2026-10
// (LEGACY defect 1) retraction recomputed, retracted (main1 total 10), which was never
// inferred, and left (main1 total 6) standing.
TEST_F(RetractRecordedConsequentsTest, ConsequentAggregateRetractionReplays) {
  this->build(kSumValues);
  this->stage_support4();
  this->stage_done();
  ASSERT_EQ(this->rete_session->execute_rules(), 0);
  auto rmgr = this->rdf_session->rmgr();
  ASSERT_TRUE(this->rdf_session->contains(r("main1"), r("hasSupport"), r("support4")))
    << "the aggregate's input changed before the retraction";
  EXPECT_FALSE(this->rdf_session->contains(r("main1"), r("total"), rmgr->create_literal<int>(6)))
    << "the total inferred was retracted";
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

// Cycles on a consequent the recording does not touch: the withdrawn row is erased, and
// an equal row fires on every cycle. Until 2026-10 (LEGACY defect 2) the row was left
// in its relation marked kProcessed -- remove_beta_row returns at once for a row
// already marked kDeleted -- so an equal row was taken for one already inferred and
// never fired.
TEST_F(RetractRecordedConsequentsTest, PureConsequentRefires) {
  this->build(kPure);
  auto one = this->rdf_session->rmgr()->create_literal(1);
  ASSERT_EQ(this->rete_session->execute_rules(), 0);
  ASSERT_EQ(this->count_links(), 1);
  for(int cycle=0; cycle<5; ++cycle) {
    this->rdf_session->insert_inferred(r("main1"), r("done"), one);
    ASSERT_EQ(this->rete_session->execute_rules(), 0);
    EXPECT_EQ(this->count_links(), 0) << "cycle " << cycle << ", withdrawn";
    this->rdf_session->retract(r("main1"), r("done"), one);
    ASSERT_EQ(this->rete_session->execute_rules(), 0);
    EXPECT_EQ(this->count_links(), 1) << "cycle " << cycle << ", re-admitted";
  }
}

// The session's erase is a delete of an asserted fact, and the rete hears it: the rows
// built on the erased triple are withdrawn and their consequents retracted, as for the
// Go engine's Erase. The rule is (?s p o) -> (?s q 1).
class EraseTellsTheReteTest : public ::testing::Test {
 protected:
  ~EraseTellsTheReteTest() {
    if(this->rete_session) this->rete_session->terminate();
  }

  void build()
  {
    auto meta_graph = rdf::create_rdf_graph();
    auto * rmgr = meta_graph->rmgr();
    rmgr->initialize();
    p = rmgr->create_resource("p");
    o = rmgr->create_resource("o");
    q = rmgr->create_resource("q");
    one = rmgr->create_literal(int32_t(1));
    auto ri = create_row_initializer(1);
    ri->put(0, 0|brc_triple, "?s");
    NodeVertexVector nv;
    nv.push_back(create_node_vertex(nullptr, 0, 0, false, 100, {}, "", {}));
    nv.push_back(create_node_vertex(nv[0].get(), 0, 1, false, 100, {}, "", ri));
    ReteMetaStore::AlphaNodeVector an;
    an.push_back(create_alpha_node<F_var, F_var, F_var>(nv[0].get(), 0, true, "", F_var("*"), F_var("*"), F_var("*")));
    an.push_back(create_alpha_node<F_var, F_cst, F_cst>(nv[1].get(), 0, true, "", F_var("?s"), F_cst(p), F_cst(o)));
    an.push_back(create_alpha_node<F_binded, F_cst, F_cst>(nv[1].get(), 0, false, "", F_binded(0), F_cst(q), F_cst(one)));
    this->rete_meta_store = create_rete_meta_store({}, an, nv);
    this->rete_meta_store->initialize();
    rmgr->set_locked();
    meta_graph->set_locked();
    this->rdf_session = rdf::create_rdf_session(meta_graph);
    new_rete_session();
    s = this->rdf_session->rmgr()->create_resource("s");
  }

  // As the pool worker does at each rule set: release the rete session, start another
  void new_rete_session()
  {
    if(this->rete_session) this->rete_session->terminate();
    this->rete_session = create_rete_session(this->rete_meta_store, this->rdf_session.get());
    this->rete_session->initialize();
  }

  rdf::r_index p{}, o{}, q{}, one{}, s{};
  ReteSessionPtr     rete_session;
  ReteMetaStorePtr   rete_meta_store;
  rdf::RDFSessionPtr rdf_session;
};

TEST_F(EraseTellsTheReteTest, AnErasedFactWithdrawsWhatItInferred) {
  build();
  rdf_session->insert(s, p, o);
  ASSERT_EQ(rete_session->execute_rules(), 0);
  ASSERT_TRUE(rdf_session->contains(s, q, one));
  EXPECT_EQ(rdf_session->erase(s, p, o), 1);
  ASSERT_EQ(rete_session->execute_rules(), 0);
  EXPECT_FALSE(rdf_session->contains(s, p, o));
  EXPECT_FALSE(rdf_session->contains(s, q, one)) << "the rete was not told of the erase";
  // Asserting the fact again infers (s q 1) again: the withdrawn row was erased from its
  // relation. Until 2026-10 it stayed (LEGACY defect 2), and nothing was inferred.
  rdf_session->insert(s, p, o);
  ASSERT_EQ(rete_session->execute_rules(), 0);
  EXPECT_TRUE(rdf_session->contains(s, q, one));
}

// The pool worker erases (jets:iState jets:loop *) and (jets:iState jets:completed *)
// at the start of each rule set, after the new rete session registered its callbacks
// and before its first execute_rules: the rete has no rows yet, and nothing changes.
TEST_F(EraseTellsTheReteTest, AnEraseBeforeTheFirstExecuteChangesNothing) {
  build();
  rdf_session->insert(s, p, o);
  ASSERT_EQ(rete_session->execute_rules(), 0);
  ASSERT_TRUE(rdf_session->contains(s, q, one));
  new_rete_session();
  EXPECT_EQ(rdf_session->erase(s, p, o), 1);
  ASSERT_EQ(rete_session->execute_rules(), 0);
  // the new rete never saw (s p o), so it infers nothing and withdraws nothing:
  // (s q 1) was inferred under the previous rete session and stays
  EXPECT_TRUE(rdf_session->contains(s, q, one));
  EXPECT_FALSE(rdf_session->contains(s, p, o));
}

// Asserting a triple the rules inferred moves it from the inferred graph to the asserted
// one: it never leaves the session, and the rete is not told (rdf_session.h, insert).
TEST_F(EraseTellsTheReteTest, AssertingAnInferredTripleMovesItSilently) {
  build();
  rdf_session->insert(s, p, o);
  ASSERT_EQ(rete_session->execute_rules(), 0);
  ASSERT_TRUE(rdf_session->get_inferred_graph()->contains(s, q, one));
  rdf_session->insert(s, q, one);
  EXPECT_FALSE(rdf_session->get_inferred_graph()->contains(s, q, one));
  EXPECT_TRUE(rdf_session->contains(s, q, one));
  ASSERT_EQ(rete_session->execute_rules(), 0);
  EXPECT_TRUE(rdf_session->contains(s, q, one));
}

}   // namespace
}   // namespace jets::rete
