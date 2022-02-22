#include <algorithm>
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
  AlphaNodeStub(b_index node_vertex, int key, bool is_antecedent)
    : AlphaNode(node_vertex, key, is_antecedent) {}

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
  rdf::SearchTriple
  compute_find_triple(BetaRow const* parent_row)const override
  {
    return {};
  }
  rdf::Triple
  compute_consequent_triple(ReteSession * rete_session, BetaRow const* beta_row)const override
  {
    return {};
  }
  void
  index_beta_row(BetaRelation * parent_beta_relation, b_index child_node_vertex, BetaRow const* beta_row)const override
  {}

  /**
   * @brief Remove index beta_row in beta_relation indexes according to the functors template arguments
   * 
   * @param beta_relation BetaRelation with the indexes
   * @param beta_row  BetaRow to index
   */
  void
  remove_index_beta_row(BetaRelation * parent_beta_relation, b_index child_node_vertex, BetaRow const* beta_row)const override
  {}

  /**
   * @brief Initialize BetaRelation indexes for this child AlphaNode
   * 
   * @param beta_relation BetaRelation with the indexes
   */
  void
  initialize_indexes(BetaRelation * parent_beta_relation, b_index child_node_vertex)const override
  {}

  std::ostream & 
  describe(std::ostream & out)const override
  {
    return out;
  }

};
AlphaNodePtr create_alpha_node(b_index node_vertex, int key, bool is_antecedent)
{
  return std::make_shared<AlphaNodeStub>(node_vertex, key, is_antecedent);
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
  ReteMetaStoreTest() : good_alpha_nodes(), bad_alpha_nodes(), node_vertexes(), meta_graph() {
      meta_graph = rdf::create_rdf_graph();
      // 4 NodeVertex corresponding to Beta nodes
      node_vertexes.reserve(4);
      node_vertexes.push_back(create_node_vertex(nullptr, 0, 0, false, 10, {}, {}));
      node_vertexes.push_back(create_node_vertex(node_vertexes[0].get(), 0, 1, false, 10, {}, {}));
      node_vertexes.push_back(create_node_vertex(node_vertexes[0].get(), 0, 2, false, 10, {}, {}));
      node_vertexes.push_back(create_node_vertex(node_vertexes[2].get(), 0, 3, false, 10, {}, {}));
      // The good 7 AlphaNode
      good_alpha_nodes.reserve(7);
      good_alpha_nodes.push_back(create_alpha_node(node_vertexes[0].get(), 0, true));
      good_alpha_nodes.push_back(create_alpha_node(node_vertexes[1].get(), 0, true));
      good_alpha_nodes.push_back(create_alpha_node(node_vertexes[2].get(), 0, true));
      good_alpha_nodes.push_back(create_alpha_node(node_vertexes[3].get(), 0, true));
      good_alpha_nodes.push_back(create_alpha_node(node_vertexes[1].get(), 0, false));
      good_alpha_nodes.push_back(create_alpha_node(node_vertexes[3].get(), 0, false));
      good_alpha_nodes.push_back(create_alpha_node(node_vertexes[3].get(), 0, false));
      // The bad 6 AlphaNode (having a consequent node at vertex 2)
      bad_alpha_nodes.reserve(7);
      bad_alpha_nodes.push_back(create_alpha_node(node_vertexes[0].get(), 0, true));
      bad_alpha_nodes.push_back(create_alpha_node(node_vertexes[1].get(), 0, true));
      bad_alpha_nodes.push_back(create_alpha_node(node_vertexes[2].get(), 0, false)); //<- bad!
      bad_alpha_nodes.push_back(create_alpha_node(node_vertexes[3].get(), 0, true));
      bad_alpha_nodes.push_back(create_alpha_node(node_vertexes[1].get(), 0, false));
      bad_alpha_nodes.push_back(create_alpha_node(node_vertexes[3].get(), 0, false));
      bad_alpha_nodes.push_back(create_alpha_node(node_vertexes[3].get(), 0, false));
  }

  ReteMetaStore::AlphaNodeVector good_alpha_nodes;
  ReteMetaStore::AlphaNodeVector bad_alpha_nodes;
  NodeVertexVector node_vertexes;
  rdf::RDFGraphPtr meta_graph;
};

// Define the tests
TEST_F(ReteMetaStoreTest, GoodMetaStoreTest0) {
    b_index b0 = node_vertexes[0].get();
    b_index b1 = node_vertexes[1].get();
    b_index b2 = node_vertexes[2].get();
    b_index b3 = node_vertexes[3].get();
    ReteMetaStorePtr good_meta = create_rete_meta_store(meta_graph, good_alpha_nodes, node_vertexes);
    EXPECT_EQ(good_meta->initialize(), 0);
    EXPECT_EQ(b0->has_consequent_terms(), false);
    EXPECT_EQ(b0->child_nodes.size(), 2);
    EXPECT_TRUE(b0->child_nodes.find(b1) != b0->child_nodes.end());
    EXPECT_TRUE(b0->child_nodes.find(b2) != b0->child_nodes.end());
    EXPECT_TRUE(b0->child_nodes.find(b3) == b0->child_nodes.end());
}

TEST_F(ReteMetaStoreTest, GoodMetaStoreTest1) {
    b_index b1 = node_vertexes[1].get();
    ReteMetaStorePtr good_meta = create_rete_meta_store(meta_graph, good_alpha_nodes, node_vertexes);
    EXPECT_EQ(good_meta->initialize(), 0);
    EXPECT_EQ(b1->has_consequent_terms(), true);
    EXPECT_EQ(b1->child_nodes.size(), 0);
    EXPECT_EQ(b1->consequent_alpha_vertexes.size(), 1);
    EXPECT_TRUE(b1->consequent_alpha_vertexes.find(4) != b1->consequent_alpha_vertexes.end());
}

TEST_F(ReteMetaStoreTest, GoodMetaStoreTest2) {
    b_index b2 = node_vertexes[2].get();
    b_index b3 = node_vertexes[3].get();
    ReteMetaStorePtr good_meta = create_rete_meta_store(meta_graph, good_alpha_nodes, node_vertexes);
    EXPECT_EQ(good_meta->initialize(), 0);
    EXPECT_EQ(b2->has_consequent_terms(), false);
    EXPECT_EQ(b2->child_nodes.size(), 1);
    EXPECT_TRUE(b2->child_nodes.find(b3) != b2->child_nodes.end());
}

TEST_F(ReteMetaStoreTest, GoodMetaStoreTest3) {
    b_index b3 = node_vertexes[3].get();
    ReteMetaStorePtr good_meta = create_rete_meta_store(meta_graph, good_alpha_nodes, node_vertexes);
    EXPECT_EQ(good_meta->initialize(), 0);
    EXPECT_EQ(b3->has_consequent_terms(), true);
    EXPECT_EQ(b3->child_nodes.size(), 0);
    EXPECT_EQ(b3->consequent_alpha_vertexes.size(), 2);
    EXPECT_TRUE(b3->consequent_alpha_vertexes.find(5) != b3->consequent_alpha_vertexes.end());
    EXPECT_TRUE(b3->consequent_alpha_vertexes.find(6) != b3->consequent_alpha_vertexes.end());
}

TEST_F(ReteMetaStoreTest, BadMetaStoreTest0) {
    ReteMetaStorePtr meta = create_rete_meta_store(meta_graph, bad_alpha_nodes, node_vertexes);
    EXPECT_EQ(meta->initialize(), -1);
}

}   // namespace
}   // namespace jets::rdf