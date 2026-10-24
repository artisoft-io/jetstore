#include <cstdint>
#include <vector>

#include <gtest/gtest.h>

#include "../rdf/rdf_types.h"
#include "../rete/rete_types.h"
#include "../rete/rete_types_impl.h"
#include "../rete/jets_rete_cwrapper.h"

// WHAT A VERTEX HOLDS, READ THROUGH get_vertex_observations (rules_benchmark Q-98,
// BU.1): per vertex, its beta relation's rows now and at their most, and the times its
// consequents were computed and retracted. Every counter is one the session keeps
// anyway -- BetaRelation::peak_size for the reserve hint, vertex_visits_ for the vertex
// visit limit -- so observing costs nothing until the entry point is called.
//
// The rule is (?s p o) -> (?s q 1): vertex 1 holds one row per subject with (?s p o).
namespace jets::rete {
namespace {

class VertexObservationsTest : public ::testing::Test {
 protected:
  ~VertexObservationsTest() {
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
    this->rete_session = create_rete_session(this->rete_meta_store, this->rdf_session.get());
    this->rete_session->initialize();
  }

  rdf::r_index s(int i)
  {
    return this->rdf_session->rmgr()->create_resource("s" + std::to_string(i));
  }

  struct Observed {
    int count;
    std::vector<int64_t> now, peak, inferred, retracted;
  };

  Observed observe()
  {
    Observed ob;
    ob.count = get_vertex_observations(this->rete_session.get(), 0, nullptr, nullptr, nullptr, nullptr);
    ob.now.resize(ob.count); ob.peak.resize(ob.count); ob.inferred.resize(ob.count); ob.retracted.resize(ob.count);
    int again = get_vertex_observations(this->rete_session.get(), ob.count,
      ob.now.data(), ob.peak.data(), ob.inferred.data(), ob.retracted.data());
    EXPECT_EQ(again, ob.count);
    return ob;
  }

  rdf::r_index p{}, o{}, q{}, one{};
  ReteSessionPtr     rete_session;
  ReteMetaStorePtr   rete_meta_store;
  rdf::RDFSessionPtr rdf_session;
};

TEST_F(VertexObservationsTest, RowsNowAndAtTheirMostAndVisits) {
  build();
  for(int i=0; i<3; ++i) rdf_session->insert(s(i), p, o);
  ASSERT_EQ(rete_session->execute_rules(), 0);
  auto ob = observe();
  ASSERT_EQ(ob.count, 2) << "the head vertex and the rule's";
  EXPECT_EQ(ob.now[1], 3);
  EXPECT_EQ(ob.peak[1], 3);
  EXPECT_EQ(ob.inferred[1], 3);
  EXPECT_EQ(ob.retracted[1], 0);
  EXPECT_EQ(ob.now[0], 1) << "the head vertex holds the one empty row";

  // Two subjects withdrawn: their rows are erased, the peak stays, and the
  // retractions are counted.
  EXPECT_EQ(rdf_session->erase(s(0), p, o), 1);
  EXPECT_EQ(rdf_session->erase(s(1), p, o), 1);
  ASSERT_EQ(rete_session->execute_rules(), 0);
  ob = observe();
  EXPECT_EQ(ob.now[1], 1);
  EXPECT_EQ(ob.peak[1], 3);
  EXPECT_EQ(ob.inferred[1], 3);
  EXPECT_EQ(ob.retracted[1], 2);
}

// A short buffer is filled as far as it goes, and the count is the network's.
TEST_F(VertexObservationsTest, AShortBufferIsFilledAsFarAsItGoes) {
  build();
  rdf_session->insert(s(0), p, o);
  ASSERT_EQ(rete_session->execute_rules(), 0);
  int64_t now[1] = {-1}, peak[1] = {-1}, inferred[1] = {-1}, retracted[1] = {-1};
  EXPECT_EQ(get_vertex_observations(rete_session.get(), 1, now, peak, inferred, retracted), 2);
  EXPECT_EQ(now[0], 1);
  EXPECT_EQ(get_vertex_observations(nullptr, 0, nullptr, nullptr, nullptr, nullptr), -1);
  EXPECT_EQ(get_vertex_observations(rete_session.get(), 1, nullptr, peak, inferred, retracted), -1);
}

}   // namespace
}   // namespace jets::rete
