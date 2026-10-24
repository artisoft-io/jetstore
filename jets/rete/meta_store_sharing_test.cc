#include <set>
#include <string>
#include <thread>
#include <vector>

#include <gtest/gtest.h>

#include "../rdf/rdf_types.h"
#include "../rete/rete_types.h"
#include "../rete/rete_types_impl.h"

// What the sessions of one meta store share, and what a session reads of the
// meta graph: the antecedent query keys (I-86) and the vertex-visit limit (Q-83).

namespace jets::rete {
namespace {

// rule2> (?s p ?o).(?s q ?x) -> (?s out2 ?x)
// rule3> (?s p ?o).(?s r ?y) -> (?s out3 ?y)
//   vertex 0 head; 1 (?s p ?o); 2 (?s q ?x) and 3 (?s r ?y), both children of 1,
//   each querying vertex 1's relation by its one bound term, ?s: two indexes of
//   the same kind, idx1, at positions 0 and 1
struct TwoChildren {
  rdf::RDFGraphPtr meta_graph;
  ReteMetaStorePtr meta_store;
  NodeVertexVector nv;

  TwoChildren()
  {
    meta_graph = rdf::create_rdf_graph();
    auto * rmgr = meta_graph->rmgr();
    rmgr->initialize();
    auto p = rmgr->create_resource("p");
    auto q = rmgr->create_resource("q");
    auto r = rmgr->create_resource("r");
    auto out2 = rmgr->create_resource("out2");
    auto out3 = rmgr->create_resource("out3");

    auto ri1 = create_row_initializer(2);
    ri1->put(0, 0 | brc_triple, "?s");
    ri1->put(1, 2 | brc_triple, "?o");
    auto ri2 = create_row_initializer(3);
    ri2->put(0, 0 | brc_parent_node, "?s");
    ri2->put(1, 1 | brc_parent_node, "?o");
    ri2->put(2, 2 | brc_triple, "?x");

    nv.push_back(create_node_vertex(nullptr, 0, 0, false, 10, {}, "", {}));
    nv.push_back(create_node_vertex(nv[0].get(), 0, 1, false, 10, {}, "", ri1));
    nv.push_back(create_node_vertex(nv[1].get(), 0, 2, false, 10, {}, "", ri2));
    nv.push_back(create_node_vertex(nv[1].get(), 0, 3, false, 10, {}, "", ri2));

    ReteMetaStore::AlphaNodeVector an;
    an.push_back(create_alpha_node<F_var, F_var, F_var>(nv[0].get(), 0, true, "",
      F_var("*"), F_var("*"), F_var("*")));
    an.push_back(create_alpha_node<F_var, F_cst, F_var>(nv[1].get(), 0, true, "",
      F_var("?s"), F_cst(p), F_var("?o")));
    an.push_back(create_alpha_node<F_binded, F_cst, F_var>(nv[2].get(), 0, true, "",
      F_binded(0), F_cst(q), F_var("?x")));
    an.push_back(create_alpha_node<F_binded, F_cst, F_var>(nv[3].get(), 0, true, "",
      F_binded(0), F_cst(r), F_var("?x")));
    an.push_back(create_alpha_node<F_binded, F_cst, F_binded>(nv[2].get(), 0, false, "",
      F_binded(0), F_cst(out2), F_binded(2)));
    an.push_back(create_alpha_node<F_binded, F_cst, F_binded>(nv[3].get(), 0, false, "",
      F_binded(0), F_cst(out3), F_binded(2)));

    meta_store = create_rete_meta_store({}, an, nv);
  }

  // One session: (sN p o), (sN q xN), (sN r yN) for N in [0, n); true when it
  // infers exactly (sN out2 xN) and (sN out3 yN)
  bool run_session(int n)
  {
    auto rdf_session = rdf::create_rdf_session(meta_graph);
    auto rete_session = create_rete_session(meta_store, rdf_session.get());
    if(rete_session->initialize() != 0) return false;
    auto * rmgr = rdf_session->rmgr();
    auto p = rmgr->create_resource("p");
    auto q = rmgr->create_resource("q");
    auto r = rmgr->create_resource("r");
    auto o = rmgr->create_resource("o");
    for(int i = 0; i < n; i++) {
      auto s = rmgr->create_resource("s" + std::to_string(i));
      rdf_session->insert(s, p, o);
      rdf_session->insert(s, q, rmgr->create_literal(i));
      rdf_session->insert(s, r, rmgr->create_literal(-i));
    }
    if(rete_session->execute_rules() != 0) return false;
    auto out2 = rmgr->create_resource("out2");
    auto out3 = rmgr->create_resource("out3");
    bool ok = true;
    for(int i = 0; i < n; i++) {
      auto s = rmgr->create_resource("s" + std::to_string(i));
      ok = ok and rdf_session->contains(s, out2, rmgr->create_literal(i));
      ok = ok and rdf_session->contains(s, out3, rmgr->create_literal(-i));
    }
    rete_session->terminate();
    return ok;
  }
};

// I-86: the keys are set when the meta store is built, before any session. Each
// session's BetaRelation::initialize used to write them into the shared node
// vertexes, so until the first session they were all 0.
TEST(MetaStoreSharingTest, QueryKeysAreSetWhenTheMetaStoreIsBuilt)
{
  TwoChildren t;
  ASSERT_EQ(t.meta_store->initialize(), 0);
  std::set<int> keys{t.nv[2]->antecedent_query_key, t.nv[3]->antecedent_query_key};
  EXPECT_EQ(keys, (std::set<int>{0, 1}));
  auto before2 = t.nv[2]->antecedent_query_key;
  EXPECT_TRUE(t.run_session(3));
  EXPECT_EQ(t.nv[2]->antecedent_query_key, before2);
}

// Sessions on one meta store, at once, as a pool runs them. Under
// ThreadSanitizer this reported the sessions' writes of antecedent_query_key.
TEST(MetaStoreSharingTest, ConcurrentSessionsOnOneMetaStore)
{
  TwoChildren t;
  ASSERT_EQ(t.meta_store->initialize(), 0);
  // The first session locks the meta graph; a pool opens it before its workers
  ASSERT_TRUE(t.run_session(1));
  constexpr int kThreads = 4;
  std::vector<int> ok(kThreads, 0);
  std::vector<std::thread> threads;
  for(int i = 0; i < kThreads; i++) {
    threads.emplace_back([&t, &ok, i]() {
      bool all = true;
      for(int k = 0; k < 20; k++) all = all and t.run_session(10);
      ok[i] = all ? 1 : 0;
    });
  }
  for(auto & th: threads) th.join();
  for(int i = 0; i < kThreads; i++) EXPECT_EQ(ok[i], 1) << "thread " << i;
}

// rule> (?s counter ?n).[?n < 50000] -> (?s counter expr(?n + 1))
// Each inferred triple is a new row of vertex 1, so the rule visits its vertex
// once per value of ?n: 50,000 times when nothing stops it.
struct Counter {
  rdf::RDFGraphPtr meta_graph;
  ReteMetaStorePtr meta_store;

  Counter()
  {
    meta_graph = rdf::create_rdf_graph();
    auto * rmgr = meta_graph->rmgr();
    rmgr->initialize();
    auto counter = rmgr->create_resource("counter");

    auto ri1 = create_row_initializer(2);
    ri1->put(0, 0 | brc_triple, "?s");
    ri1->put(1, 2 | brc_triple, "?n");
    auto below = create_expr_binary_operator<LtVisitor>(0,
      create_expr_binded_var(1), create_expr_cst(rdf::RdfAstType(rdf::LInt32(50000))));

    NodeVertexVector nv;
    nv.push_back(create_node_vertex(nullptr, 0, 0, false, 10, {}, "", {}));
    nv.push_back(create_node_vertex(nv[0].get(), 0, 1, false, 10, below, "", ri1));

    ReteMetaStore::AlphaNodeVector an;
    an.push_back(create_alpha_node<F_var, F_var, F_var>(nv[0].get(), 0, true, "",
      F_var("*"), F_var("*"), F_var("*")));
    an.push_back(create_alpha_node<F_var, F_cst, F_var>(nv[1].get(), 0, true, "",
      F_var("?s"), F_cst(counter), F_var("?n")));
    auto plus1 = create_expr_binary_operator<AddVisitor>(0,
      create_expr_binded_var(1), create_expr_cst(rdf::RdfAstType(rdf::LInt32(1))));
    an.push_back(create_alpha_node<F_binded, F_cst, F_expr>(nv[1].get(), 0, false, "",
      F_binded(0), F_cst(counter), F_expr(plus1)));

    meta_store = create_rete_meta_store({}, an, nv);
    meta_store->initialize();
  }

  void set_limit(int limit)
  {
    auto const* jets = meta_graph->rmgr()->jets();
    meta_graph->insert(jets->jets__istate, jets->jets__max_vertex_visits, limit);
  }

  struct Result {
    int code;
    std::string message;
    bool reached;          // (s0 counter 50000) is in the graph
  };

  Result run()
  {
    auto rdf_session = rdf::create_rdf_session(meta_graph);
    auto rete_session = create_rete_session(meta_store, rdf_session.get());
    EXPECT_EQ(rete_session->initialize(), 0);
    auto * rmgr = rdf_session->rmgr();
    auto s0 = rmgr->create_resource("s0");
    auto counter = rmgr->create_resource("counter");
    rdf_session->insert(s0, counter, rmgr->create_literal(0));
    Result res{0, {}, false};
    char const* msg = rete_session->execute_rules2(&res.code);
    if(msg) res.message = msg;
    res.reached = rdf_session->contains(s0, counter, rmgr->create_literal(50000));
    rete_session->terminate();
    return res;
  }
};

// Q-83: with no jets:max_vertex_visits, a session stops at 10,000 visits of a
// vertex, and its message names the limit and the setting. A session with no
// setting had no limit, so at the base this rule ran its 50,000 visits.
TEST(VertexVisitLimitTest, TheDefaultStopsAt10000)
{
  Counter c;
  auto res = c.run();
  EXPECT_EQ(res.code, -10);
  EXPECT_FALSE(res.reached);
  EXPECT_NE(res.message.find("Maximum vertex visits reached"), std::string::npos) << res.message;
  EXPECT_NE(res.message.find("\"limit\":10000"), std::string::npos) << res.message;
  EXPECT_NE(res.message.find("jets:max_vertex_visits"), std::string::npos) << res.message;
  EXPECT_NE(res.message.find("\"1\": [10000,0]"), std::string::npos) << res.message;
  EXPECT_EQ(ReteSession::kDefaultMaxVertexVisits, 10000);
}

// A process that sets the limit gets its own: past 10,000, and with the message
// it always had
TEST(VertexVisitLimitTest, ASetLimitIsKept)
{
  Counter c;
  c.set_limit(20000);
  auto res = c.run();
  EXPECT_EQ(res.code, -10);
  EXPECT_EQ(res.message, "{\"error\":\"Maximum vertex visits reached\",\"data\":{\"1\": [20000,0]}}");
}

TEST(VertexVisitLimitTest, ALimitAboveTheVisitsDoesNotStop)
{
  Counter c;
  c.set_limit(1000000);
  auto res = c.run();
  EXPECT_EQ(res.code, 0) << res.message;
  EXPECT_TRUE(res.reached);
}

// A limit of 0 or less turns the limit off, as it did
TEST(VertexVisitLimitTest, ZeroTurnsTheLimitOff)
{
  Counter c;
  c.set_limit(0);
  auto res = c.run();
  EXPECT_EQ(res.code, 0) << res.message;
  EXPECT_TRUE(res.reached);
}

// An integer of another width is read as the limit (an int64 arrives when a value
// does not fit in 32 bits); before, anything but an int32 meant no limit
TEST(VertexVisitLimitTest, AnInt64LimitIsRead)
{
  Counter c;
  auto const* jets = c.meta_graph->rmgr()->jets();
  c.meta_graph->insert(jets->jets__istate, jets->jets__max_vertex_visits,
    c.meta_graph->rmgr()->create_literal(std::int64_t(20000)));
  auto res = c.run();
  EXPECT_EQ(res.code, -10);
  EXPECT_NE(res.message.find("\"1\": [20000,0]"), std::string::npos) << res.message;
}

}   // namespace
}   // namespace jets::rete
