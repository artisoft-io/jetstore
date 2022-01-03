#include <cstddef>
#include <iostream>
#include <string>
#include <memory>

#include <gtest/gtest.h>

#include "jets/rdf/rdf_types.h"
#include "jets/rete/rete_types.h"

namespace jets::rete {
namespace {
using namespace jets::rete;
class AlphaNodeStub: public AlphaNode {
 public:
  AlphaNodeStub(b_index node_vertex, bool is_antecedent)
    : AlphaNode(node_vertex, is_antecedent) {}

  int
  register_callback(ReteSession * rete_session)const override
  {
    return 0;
  }

  typename AlphaNode::Iterator
  find_matching_triples(rdf::RDFSession * rdf_session, 
    BetaRow const* parent_row)const override
  {
    return rdf_session->find();
  }
  BetaRowIteratorPtr
  find_matching_rows(BetaRelation * beta_relation, 
    rdf::r_index s, rdf::r_index p, rdf::r_index o)const override
  {
      return {};
  }
  rdf::Triple
  compute_consequent_triple(BetaRow * beta_row)const override
  {
    return {};
  }
};
AlphaNodePtr create_alpha_node(b_index node_vertex, bool is_antecedent)
{
  return std::make_shared<AlphaNodeStub>(node_vertex, is_antecedent);
}

// Simple test
// The suite fixture for ReteMetaStore
//parent_node:                           v0()
//child nodes(consequent nodes):   v1(c4)   v2()
//child nodes(consequent nodes):             v3(c5,c6)
// we have 2 paths:
// v0->v1(c4)
// v0->v2->v3(c5,c6)
// Difference between good and bad: the bad configuration have a consequent term at vertex 2
class ReteMetaStoreTest : public ::testing::Test {
 protected:
  ReteMetaStoreTest() : good_alpha_nodes(), bad_alpha_nodes(), node_vertexes() {
      // 4 NodeVertex corresponding to Beta nodes
      node_vertexes.reserve(4);
      node_vertexes.push_back(create_node_vertex(nullptr, 0, false, 0, 10, {}, {}));
      node_vertexes.push_back(create_node_vertex(node_vertexes[0].get(), 1, false, 0, 10, {}, {}));
      node_vertexes.push_back(create_node_vertex(node_vertexes[0].get(), 2, false, 0, 10, {}, {}));
      node_vertexes.push_back(create_node_vertex(node_vertexes[2].get(), 3, false, 0, 10, {}, {}));
      // The good 7 AlphaNode
      good_alpha_nodes.reserve(7);
      good_alpha_nodes.push_back(create_alpha_node(node_vertexes[0].get(), true));
      good_alpha_nodes.push_back(create_alpha_node(node_vertexes[1].get(), true));
      good_alpha_nodes.push_back(create_alpha_node(node_vertexes[2].get(), true));
      good_alpha_nodes.push_back(create_alpha_node(node_vertexes[3].get(), true));
      good_alpha_nodes.push_back(create_alpha_node(node_vertexes[1].get(), false));
      good_alpha_nodes.push_back(create_alpha_node(node_vertexes[3].get(), false));
      good_alpha_nodes.push_back(create_alpha_node(node_vertexes[3].get(), false));
      // The bad 6 AlphaNode (having a consequent node at vertex 2)
      bad_alpha_nodes.reserve(7);
      bad_alpha_nodes.push_back(create_alpha_node(node_vertexes[0].get(), true));
      bad_alpha_nodes.push_back(create_alpha_node(node_vertexes[1].get(), true));
      bad_alpha_nodes.push_back(create_alpha_node(node_vertexes[2].get(), false)); //<- bad!
      bad_alpha_nodes.push_back(create_alpha_node(node_vertexes[3].get(), true));
      bad_alpha_nodes.push_back(create_alpha_node(node_vertexes[1].get(), false));
      bad_alpha_nodes.push_back(create_alpha_node(node_vertexes[3].get(), false));
      bad_alpha_nodes.push_back(create_alpha_node(node_vertexes[3].get(), false));
  }

  ReteMetaStore::AlphaNodeVector good_alpha_nodes;
  ReteMetaStore::AlphaNodeVector bad_alpha_nodes;
  NodeVertexVector node_vertexes;
};

// Define the tests
TEST_F(ReteMetaStoreTest, GoodMetaStoreTest0) {
    b_index b0 = node_vertexes[0].get();
    b_index b1 = node_vertexes[1].get();
    b_index b2 = node_vertexes[2].get();
    b_index b3 = node_vertexes[3].get();
    ReteMetaStorePtr good_meta = create_rete_meta_store(good_alpha_nodes, {}, node_vertexes);
    EXPECT_EQ(good_meta->initialize(), 0);
    EXPECT_EQ(b0->has_consequent_terms(), false);
    EXPECT_EQ(b0->child_nodes.size(), 2);
    EXPECT_TRUE(b0->child_nodes.find(b1) != b0->child_nodes.end());
    EXPECT_TRUE(b0->child_nodes.find(b2) != b0->child_nodes.end());
    EXPECT_TRUE(b0->child_nodes.find(b3) == b0->child_nodes.end());
}

TEST_F(ReteMetaStoreTest, GoodMetaStoreTest1) {
    b_index b1 = node_vertexes[1].get();
    ReteMetaStorePtr good_meta = create_rete_meta_store(good_alpha_nodes, {}, node_vertexes);
    EXPECT_EQ(good_meta->initialize(), 0);
    EXPECT_EQ(b1->has_consequent_terms(), true);
    EXPECT_EQ(b1->child_nodes.size(), 0);
    EXPECT_EQ(b1->consequent_alpha_vertexes.size(), 1);
    EXPECT_TRUE(b1->consequent_alpha_vertexes.find(4) != b1->consequent_alpha_vertexes.end());
}

TEST_F(ReteMetaStoreTest, GoodMetaStoreTest2) {
    b_index b2 = node_vertexes[2].get();
    b_index b3 = node_vertexes[3].get();
    ReteMetaStorePtr good_meta = create_rete_meta_store(good_alpha_nodes, {}, node_vertexes);
    EXPECT_EQ(good_meta->initialize(), 0);
    EXPECT_EQ(b2->has_consequent_terms(), false);
    EXPECT_EQ(b2->child_nodes.size(), 1);
    EXPECT_TRUE(b2->child_nodes.find(b3) != b2->child_nodes.end());
}

TEST_F(ReteMetaStoreTest, GoodMetaStoreTest3) {
    b_index b3 = node_vertexes[3].get();
    ReteMetaStorePtr good_meta = create_rete_meta_store(good_alpha_nodes, {}, node_vertexes);
    EXPECT_EQ(good_meta->initialize(), 0);
    EXPECT_EQ(b3->has_consequent_terms(), true);
    EXPECT_EQ(b3->child_nodes.size(), 0);
    EXPECT_EQ(b3->consequent_alpha_vertexes.size(), 2);
    EXPECT_TRUE(b3->consequent_alpha_vertexes.find(5) != b3->consequent_alpha_vertexes.end());
    EXPECT_TRUE(b3->consequent_alpha_vertexes.find(6) != b3->consequent_alpha_vertexes.end());
}

TEST_F(ReteMetaStoreTest, BadMetaStoreTest0) {
    ReteMetaStorePtr meta = create_rete_meta_store(bad_alpha_nodes, {}, node_vertexes);
    EXPECT_EQ(meta->initialize(), -1);
}

}   // namespace
}   // namespace jets::rdf