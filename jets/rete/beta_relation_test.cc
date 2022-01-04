#include <cstddef>
#include <iostream>
#include <memory>

#include <gtest/gtest.h>

#include "alpha_functors.h"
#include "beta_row.h"
#include "jets/rdf/rdf_types.h"
#include "jets/rete/rete_types.h"

namespace jets::rete {
namespace {
// The suite fixture for beta relation
class BetaRelationTest : public ::testing::Test {
 protected:
  BetaRelationTest() 
    : node_vertexes(), beta_relations(), rete_session(),
      rete_meta_store(), rdf_session() 
  {
    // create antecedent query spec for the child vertices 1, 2, 3
    AntecedentQuerySpecPtr aqspec1 = create_antecedent_query_spec(0, AntecedentQueryType::kQTu, 's', 0, -1, -1);
    AntecedentQuerySpecPtr aqspec2 = create_antecedent_query_spec(0, AntecedentQueryType::kQTuv, 's', 0, 1, -1);
    AntecedentQuerySpecPtr aqspec3 = create_antecedent_query_spec(0, AntecedentQueryType::kQTuvw, 's', 0, 1, 2);

    // we have 1 paths:
    // v0->v1
    // v0 row: [p1, p2, p3]
    // v1 row: [p1, p2, t2]
    auto ri0 = create_row_initializer(3);
    ri0->put(0, 0 | brc_triple);
    ri0->put(1, 1 | brc_triple);
    ri0->put(1, 2 | brc_triple);
    auto ri1 = create_row_initializer(3);
    ri1->put(0, 0 | brc_parent_node);
    ri1->put(1, 1 | brc_parent_node);
    ri1->put(2, 2 | brc_triple);
    node_vertexes.push_back(create_node_vertex(nullptr, 0, false, 0, 10, ri0, {}));
    node_vertexes.push_back(create_node_vertex(node_vertexes[0].get(), 1, false, 0, 10, ri1, aqspec1));
    node_vertexes.push_back(create_node_vertex(node_vertexes[0].get(), 2, false, 0, 10, ri1, aqspec2));
    node_vertexes.push_back(create_node_vertex(node_vertexes[0].get(), 3, false, 0, 10, ri1, aqspec3));

    // create & initalize the meta store
    rete_meta_store = create_rete_meta_store({}, {}, node_vertexes);
    rete_meta_store->initialize();

    // create & initialize the beta relation entities
    for(size_t i=0; i<node_vertexes.size(); ++i) {
      auto bn = create_beta_node(node_vertexes[i].get());
      bn->initialize();
      beta_relations.push_back(bn);
    }
    rdf_session = rdf::create_rdf_session(rdf::create_rdf_graph());
    rete_session = create_rete_session(rdf_session.get());
    rete_session->initialize(rete_meta_store.get());
  }

  BetaRowPtr 
  create_beta_row(b_index node_vertex, BetaRowPtr parent_row, rdf::Triple triple) 
  {
    BetaRowPtr beta_row = ::jets::rete::create_beta_row(node_vertex, node_vertex->beta_row_initializer->get_size());
    beta_row->initialize(node_vertex->beta_row_initializer.get(), parent_row.get(), &triple);
    return beta_row;
  }

  NodeVertexVector   node_vertexes;
  BetaRelationVector beta_relations;
  ReteSessionPtr  rete_session;
  ReteMetaStorePtr rete_meta_store;
  rdf::RDFSessionPtr   rdf_session;
};

// Define the tests
TEST_F(BetaRelationTest, InsertBetaRow) 
{
  // rdf resource manager
  rdf::RManager rmanager;
  auto p0 = rmanager.create_resource("p0");
  auto p1 = rmanager.create_resource("p1");
  auto p2 = rmanager.create_resource("p2");
  BetaRowPtr beta_row = ::jets::rete::create_beta_row(node_vertexes[1].get(), 3);
  beta_row->put(0, p0);
  beta_row->put(1, p1);
  beta_row->put(2, p2);

  EXPECT_EQ(beta_relations[1]->insert_beta_row(rete_session.get(), beta_row), 0);
  EXPECT_FALSE(beta_row->get_node_vertex()->has_consequent_terms());
  EXPECT_FALSE(beta_row->is_inserted());
  EXPECT_TRUE(beta_row->is_processed());
}

TEST_F(BetaRelationTest, AntecedentQuerySpecTest) 
{
  // rdf resource manager
  rdf::RManager rmanager;
  BetaRowPtr row0 = ::jets::rete::create_beta_row(node_vertexes[1].get(), 3);
  row0->put(0, rmanager.create_resource("s0"));
  row0->put(1, rmanager.create_resource("p0"));
  row0->put(2, rmanager.create_resource("o0"));
  BetaRowPtr row1 = ::jets::rete::create_beta_row(node_vertexes[2].get(), 3);
  row1->put(0, rmanager.create_resource("s0"));
  row1->put(1, rmanager.create_resource("p0"));
  row1->put(2, rmanager.create_resource("o1"));
  BetaRowPtr row2 = ::jets::rete::create_beta_row(node_vertexes[3].get(), 3);
  row2->put(0, rmanager.create_resource("s2"));
  row2->put(1, rmanager.create_resource("p2"));
  row2->put(2, rmanager.create_resource("o2"));

  EXPECT_EQ(beta_relations[0]->insert_beta_row(rete_session.get(), row0), 0);
  EXPECT_EQ(beta_relations[0]->insert_beta_row(rete_session.get(), row1), 0);
  EXPECT_EQ(beta_relations[0]->insert_beta_row(rete_session.get(), row2), 0);

  {
    auto o0 = rmanager.create_resource("o0");
    auto o1 = rmanager.create_resource("o1");
    auto itor = beta_relations[0]->get_idx1_rows_iterator(0, rmanager.create_resource("s0"));
    int cnt=0;
    while(not itor->is_end()) {
      auto row = itor->get_row();
      EXPECT_EQ(row->get(0), rmanager.create_resource("s0"));
      EXPECT_EQ(row->get(1), rmanager.create_resource("p0"));
      EXPECT_TRUE(row->get(2)==o0 or row->get(2)==o1);
      ++cnt;
      itor->next();
    }
    EXPECT_EQ(cnt, 2);
  }

  {
    auto o0 = rmanager.create_resource("o0");
    auto o1 = rmanager.create_resource("o1");
    auto itor = beta_relations[0]->get_idx2_rows_iterator(0, rmanager.create_resource("s0"), rmanager.create_resource("p0"));
    int cnt=0;
    while(not itor->is_end()) {
      auto row = itor->get_row();
      EXPECT_EQ(row->get(0), rmanager.create_resource("s0"));
      EXPECT_EQ(row->get(1), rmanager.create_resource("p0"));
      EXPECT_TRUE(row->get(2)==o0 or row->get(2)==o1);
      ++cnt;
      itor->next();
    }
    EXPECT_EQ(cnt, 2);
  }

  {
    auto s2 = rmanager.create_resource("s2");
    auto p2 = rmanager.create_resource("p2");
    auto o2 = rmanager.create_resource("o2");
    auto itor = beta_relations[0]->get_idx3_rows_iterator(0, s2, p2, o2);
    while(not itor->is_end()) {
      auto row = itor->get_row();
      EXPECT_EQ(row->get(0), s2);
      EXPECT_EQ(row->get(1), p2);
      EXPECT_EQ(row->get(2), o2);
      itor->next();
    }
  }
}

TEST_F(BetaRelationTest, AlphaFunctor1Test) 
{
  auto * rmgr = rdf_session->rmgr();
  auto s0 = rmgr->create_resource("s0");
  auto s1 = rmgr->create_resource("s1");
  rdf_session->insert(s0, rmgr->create_resource("p0"), rmgr->create_resource("o0"));
  rdf_session->insert(s1, rmgr->create_resource("p0"), rmgr->create_resource("o0"));
  rdf_session->insert(s1, rmgr->create_resource("p1"), rmgr->create_resource("o0"));
  AlphaNodePtr nd = create_alpha_node<F_var, F_cst, F_cst>(node_vertexes[0].get(), true,
    F_var("?s"), F_cst(rmgr->create_resource("p0")), F_cst(rmgr->create_resource("o0")) );
  auto row = ::jets::rete::create_beta_row(node_vertexes[0].get(), 3);
  auto itor = nd->find_matching_triples(rdf_session.get(), row.get());
  while(not itor.is_end()) {
    auto t3 = itor.as_triple();
    // std::cout << "** TRIPLE ** ("<<t3.subject<<", "<<t3.predicate<<", "<<t3.object<<")"<<std::endl;
    EXPECT_TRUE(t3.subject==s0 or t3.subject==s1);
    EXPECT_EQ(t3.predicate, rmgr->create_resource("p0"));
    EXPECT_EQ(t3.object, rmgr->create_resource("o0"));
    itor.next();
  }
}

TEST_F(BetaRelationTest, AlphaFunctor2Test) 
{
  auto * rmgr = rdf_session->rmgr();
  auto s0 = rmgr->create_resource("s0");
  AlphaNodePtr nd = create_alpha_node<F_binded, F_cst, F_cst>(node_vertexes[0].get(), false,
    F_binded(0), F_cst(rmgr->create_resource("p0")), F_cst(rmgr->create_resource("o0")) );
  auto row = ::jets::rete::create_beta_row(node_vertexes[0].get(), 3);
  EXPECT_EQ(row->put(0, s0), 0);
  auto t3 = nd->compute_consequent_triple(row.get());
  EXPECT_EQ(t3.subject, s0);
  EXPECT_EQ(t3.predicate, rmgr->create_resource("p0"));
  EXPECT_EQ(t3.object, rmgr->create_resource("o0"));
}

}   // namespace
}   // namespace jets::rdf