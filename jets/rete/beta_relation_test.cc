#include <cstddef>
#include <iostream>
#include <memory>

#include <gtest/gtest.h>

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
  std::cout<<"**BetaRelationTest CONST)**1"<<std::endl;
    // we have 1 paths:
    // v0->v1
    // v0 row: [p1, p2]
    // v1 row: [p1, p2, t2]
    auto ri0 = create_row_initializer(2);
    ri0->put(0, 0 | brc_triple);
    ri0->put(1, 1 | brc_triple);
    auto ri1 = create_row_initializer(3);
    ri1->put(0, 0 | brc_parent_node);
    ri1->put(1, 1 | brc_parent_node);
    ri1->put(2, 2 | brc_triple);
    node_vertexes.push_back(create_node_vertex(nullptr, 0, false, 0, 10, ri0, {}));
    node_vertexes.push_back(create_node_vertex(node_vertexes[0].get(), 1, false, 0, 10, ri1, {}));

  std::cout<<"**BetaRelationTest CONST)**2"<<std::endl;
    // create the beta relation entities
    for(size_t i=0; i<node_vertexes.size(); ++i) {
        beta_relations.push_back(create_beta_node(node_vertexes[i].get()));
    }
  std::cout<<"**BetaRelationTest CONST)**3"<<std::endl;
    rdf_session = rdf::create_rdf_session(rdf::create_rdf_graph());
    rete_meta_store = create_rete_meta_store({}, {}, node_vertexes);
    rete_meta_store->initialize();
  std::cout<<"**BetaRelationTest CONST)**4"<<std::endl;
    rete_session = create_rete_session(rete_meta_store.get(), rdf_session.get());
  std::cout<<"**BetaRelationTest CONST)**5"<<std::endl;
    rete_session->initialize();
  std::cout<<"**BetaRelationTest CONST)**30"<<std::endl;
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
  std::cout<<"**(BetaRelationTest, InsertBetaRow)**1"<<std::endl;
  // rdf resource manager
  rdf::RManager rmanager;
  auto p0 = rmanager.create_resource("p0");
  auto p1 = rmanager.create_resource("p1");
  auto p2 = rmanager.create_resource("p2");
  std::cout<<"**(BetaRelationTest, InsertBetaRow)**2"<<std::endl;
  BetaRowPtr beta_row = ::jets::rete::create_beta_row(node_vertexes[1].get(), 3);
  beta_row->put(0, p0);
  beta_row->put(1, p1);
  beta_row->put(2, p2);

  std::cout<<"**(BetaRelationTest, InsertBetaRow)**10"<<std::endl;
  EXPECT_EQ(beta_relations[1]->insert_beta_row(rete_session.get(), beta_row), 0);
  EXPECT_TRUE(beta_row->is_inserted());
  std::cout<<"**(BetaRelationTest, InsertBetaRow)**20"<<std::endl;
}

}   // namespace
}   // namespace jets::rdf