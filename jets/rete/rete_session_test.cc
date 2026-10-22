#include <algorithm>
#include <cstddef>
#include <iostream>
#include <memory>
#include <set>
#include <unordered_map>

#include <gtest/gtest.h>

#include "../rdf/rdf_types.h"
#include "../rete/rete_types.h"
#include "../rete/rete_types_impl.h"

namespace jets::rete {
namespace {
/**
 * @brief The integrated suite fixture for ReteSession and ReteMetaStore
 *
 * Testing 2 scenarios. Scenario 1:
 * (head node0).(?s has_node ?n1) -> (?s1 plus1_node expr(?n1 + 1))
 * 
 */
class ReteSessionTest : public ::testing::Test {
 protected:
  ~ReteSessionTest() {
    this->rete_session->terminate();
  }
  ReteSessionTest() 
    : rete_session(), rete_meta_store(), rdf_session() 
  {
    auto meta_graph = rdf::create_rdf_graph();
    auto * rmgr = meta_graph->rmgr();
    auto has_node = rmgr->create_resource("has_node");
    auto plus1_node = rmgr->create_resource("plus1_node");
    auto plus2_node = rmgr->create_resource("plus2_node");
    auto node1 = rmgr->create_resource("node1");
    auto node2 = rmgr->create_resource("node2");
    auto node3 = rmgr->create_resource("node3");
    auto node10 = rmgr->create_resource("node10");
    auto node20 = rmgr->create_resource("node20");
    auto node30 = rmgr->create_resource("node30");
    auto fnode = rmgr->create_resource("fnode");
    auto f2node = rmgr->create_resource("f2node");

    //rule1> (head node0).(?s has_node ?n1) -> (?s plus1_node expr(?n1 + 1))
    //rule2> (head node0).(?s has_node ?n1).(?s has_node ?n2) -> (?s plus2_node expr(?n1 + ?n2))
    //        node 0        node 1            node 2

    //rule3> (head node0).(?s fnode ?n1).(?s fnode ?n2).[?n1 <= ?n2] -> (?s f2node expr(?n1 + ?n2))
    //        node 0        node 3            node 4

    //rule4> (head node0).(?s node1 ?n1) -> (?s node2 ?n1)                    :: s=10
    //rule5> (head node0).(?s node1 ?n1).not(?s node2 ?n1) -> (?s node3 ?n1)  :: s=20
    //        node 0        node 5            node 6

    //rule6> (head node0).(?s node10 ?n1) .(?s node2 ?n1) -> (?s node20 ?n1)                     :: s=10
    //rule7> (head node0).(?s node10 ?n1) .(?s node2 ?n1).not(?s node20 ?n1).(?s node1 ?n1) -> (?s node30 ?n1)  :: s=5
    //        node 0        node 7            node 8         node 9           node 10
    // ----------------------------------------------------------------------------------
    // No need for AntecedentQuerySpec since the only vertex reads from the graph

    // BetaRowInitializer --BetaRelation 1 row: [?s, ?n1]
    auto ri1 = create_row_initializer(2);
    ri1->put(0, 0 | brc_triple, "?s");
    ri1->put(1, 2 | brc_triple, "?n1");
    
    // BetaRowInitializer --BetaRelation 2 row: [?s, ?n1, ?n2]
    auto ri2 = create_row_initializer(3);
    ri2->put(0, 0 | brc_parent_node, "?s");
    ri2->put(1, 1 | brc_parent_node, "?n1");
    ri2->put(2, 2 | brc_triple, "?n2");
    
    // BetaRowInitializer --BetaRelation 3 row: [?s, ?n1]
    auto ri3 = create_row_initializer(2);
    ri3->put(0, 0 | brc_triple, "?s");
    ri3->put(1, 2 | brc_triple, "?n1");
    
    // BetaRowInitializer --BetaRelation 4 row: [?s, ?n1, ?n2]
    auto ri4 = create_row_initializer(3);
    ri4->put(0, 0 | brc_parent_node, "?s");
    ri4->put(1, 1 | brc_parent_node, "?n1");
    ri4->put(2, 2 | brc_triple, "?n2");

    // BetaRowInitializer --BetaRelation 5 row: [?s, ?n1]
    auto ri5 = create_row_initializer(2);
    ri5->put(0, 0 | brc_triple, "?s");
    ri5->put(1, 2 | brc_triple, "?n1");

    // BetaRowInitializer --BetaRelation 6 row: [?s, ?n1]
    auto ri6 = create_row_initializer(2);
    ri6->put(0, 0 | brc_parent_node, "?s");
    ri6->put(1, 1 | brc_parent_node, "?n1");

    // Expression terms for Filters
    // node 4: [?n1 <= ?n2]
    auto lhs = create_expr_binded_var(1);
    auto rhs = create_expr_binded_var(2);
    auto expr_filter4 = create_expr_binary_operator<LeVisitor>(0, lhs, rhs);

    // NodeVertex
    NodeVertexVector   node_vertexes;
    // node 0: (head node0)
    node_vertexes.push_back(create_node_vertex(nullptr, 0, 0, false, 10, {}, "", {}));
    // node 1: (?s has_node ?n1)
    node_vertexes.push_back(create_node_vertex(node_vertexes[0].get(), 0, 1, false, 20, {}, "", ri1));
    // node 2: (?s has_node ?n2)
    node_vertexes.push_back(create_node_vertex(node_vertexes[1].get(), 0, 2, false, 10, {}, "", ri2));
    // node 3: (?s fnode ?n1)
    node_vertexes.push_back(create_node_vertex(node_vertexes[0].get(), 0, 3, false, 20, {}, "", ri3));
    // node 4: (?s fnode ?n2).[?n1 <= ?n2]
    node_vertexes.push_back(create_node_vertex(node_vertexes[3].get(), 0, 4, false, 10, expr_filter4, "", ri4));
    // node 5: (?s node1 ?n1)
    node_vertexes.push_back(create_node_vertex(node_vertexes[0].get(), 0, 5, false, 10, {}, "", ri5));
    // node 6: not(?s node2 ?n2)
    node_vertexes.push_back(create_node_vertex(node_vertexes[5].get(), 0, 6, true, 20, {}, "", ri6));
    // node 7: (?s node10 ?n1)
    node_vertexes.push_back(create_node_vertex(node_vertexes[0].get(), 0, 7, false, 10, {}, "", ri5));
    // node 8: (?s node2 ?n1)
    node_vertexes.push_back(create_node_vertex(node_vertexes[7].get(), 0, 8, false, 10, {}, "", ri6));
    // node 9: not(?s node20 ?n1)
    node_vertexes.push_back(create_node_vertex(node_vertexes[8].get(), 0, 9, true, 0, {}, "", ri6));
    // node 10: (?s node1 ?n1)
    node_vertexes.push_back(create_node_vertex(node_vertexes[9].get(), 0, 10, false, 5, {}, "", ri6));

    // AlphaNodes
    ReteMetaStore::AlphaNodeVector alpha_nodes;

    // Add Antecedent term on vertex 0 -- head vertices
    // this AlphaNode is not used, it's a place holder since we need one AlphaNode for each NodeVertex
    alpha_nodes.push_back(create_alpha_node<F_var, F_var, F_var>(node_vertexes[0].get(), 0, true, "",
      F_var("*"), F_var("*"), F_var("*") ));

    // Add Antecedent term on vertex 1: (?s has_node ?n1)
    alpha_nodes.push_back(create_alpha_node<F_var, F_cst, F_var>(node_vertexes[1].get(), 0, true, "",
      F_var("?s"), F_cst(has_node), F_var("?n1") ));

    // Add Antecedent term on vertex 2: (?s has_node ?n2)
    alpha_nodes.push_back(create_alpha_node<F_binded, F_cst, F_var>(node_vertexes[2].get(), 0, true, "",
      F_binded(0), F_cst(has_node), F_var("?n2") ));

    // Add Antecedent term on vertex 3: (?s fnode ?n1)
    alpha_nodes.push_back(create_alpha_node<F_var, F_cst, F_var>(node_vertexes[3].get(), 0, true, "",
      F_var("?s"), F_cst(fnode), F_var("?n1") ));

    // Add Antecedent term on vertex 4: (?s fnode ?n2)
    alpha_nodes.push_back(create_alpha_node<F_binded, F_cst, F_var>(node_vertexes[4].get(), 0, true, "",
      F_binded(0), F_cst(fnode), F_var("?n2") ));

    // Add Antecedent term on vertex 5: (?s node1 ?n1)
    alpha_nodes.push_back(create_alpha_node<F_var, F_cst, F_var>(node_vertexes[5].get(), 0, true, "",
      F_var("?s"), F_cst(node1), F_var("?n1") ));

    // Add Antecedent term on vertex 6: (?s node2 ?n1)
    alpha_nodes.push_back(create_alpha_node<F_binded, F_cst, F_binded>(node_vertexes[6].get(), 0, true, "",
      F_binded(0), F_cst(node2), F_binded(1) ));

    // Add Antecedent term on vertex 7: (?s node10 ?n1)
    alpha_nodes.push_back(create_alpha_node<F_var, F_cst, F_var>(node_vertexes[7].get(), 0, true, "",
      F_var("?s"), F_cst(node10), F_var("?n1") ));

    // Add Antecedent term on vertex 8: (?s node2 ?n1)
    alpha_nodes.push_back(create_alpha_node<F_binded, F_cst, F_binded>(node_vertexes[8].get(), 0, true, "",
      F_binded(0), F_cst(node2), F_binded(1) ));

    // Add Antecedent term on vertex 9: not(?s node20 ?n1)
    alpha_nodes.push_back(create_alpha_node<F_binded, F_cst, F_binded>(node_vertexes[9].get(), 0, true, "",
      F_binded(0), F_cst(node20), F_binded(1) ));

    // Add Antecedent term on vertex 10: (?s node1 ?n1)
    alpha_nodes.push_back(create_alpha_node<F_binded, F_cst, F_binded>(node_vertexes[10].get(), 0, true, "",
      F_binded(0), F_cst(node1), F_binded(1) ));

    {
      // Add Consequent term on vertex 1: (?s1 plus1_node expr(?n1 + 1))
      auto lhs = create_expr_binded_var(1);
      auto rhs = create_expr_cst(rdf::RdfAstType(rdf::LInt32(1)));
      auto expr = create_expr_binary_operator<AddVisitor>(0, lhs, rhs);
      alpha_nodes.push_back(create_alpha_node<F_binded, F_cst, F_expr>(node_vertexes[1].get(), 0, false, "",
        F_binded(0), F_cst(plus1_node), F_expr(expr) ));
    }
    {
      // Add Consequent term on vertex 2: (?s1 plus2_node expr(?n1 + ?n2))
      auto lhs = create_expr_binded_var(1);
      auto rhs = create_expr_binded_var(2);
      auto expr = create_expr_binary_operator<AddVisitor>(0, lhs, rhs);
      alpha_nodes.push_back(create_alpha_node<F_binded, F_cst, F_expr>(node_vertexes[2].get(), 0, false, "",
        F_binded(0), F_cst(plus2_node), F_expr(expr) ));
    }
    {
      // Add Consequent term on vertex 4: (?s f2node expr(?n1 + ?n2))
      auto lhs = create_expr_binded_var(1);
      auto rhs = create_expr_binded_var(2);
      auto expr = create_expr_binary_operator<AddVisitor>(0, lhs, rhs);
      alpha_nodes.push_back(create_alpha_node<F_binded, F_cst, F_expr>(node_vertexes[4].get(), 0, false, "",
        F_binded(0), F_cst(f2node), F_expr(expr) ));
    }
    // Add Consequent term on vertex 5:(?s node2 ?n1)
    alpha_nodes.push_back(create_alpha_node<F_binded, F_cst, F_binded>(node_vertexes[5].get(), 0, false, "",
      F_binded(0), F_cst(node2), F_binded(1) ));
    // Add Consequent term on vertex 6: (?s node3 ?n1)
    alpha_nodes.push_back(create_alpha_node<F_binded, F_cst, F_binded>(node_vertexes[6].get(), 0, false, "",
      F_binded(0), F_cst(node3), F_binded(1) ));
    // Add Consequent term on vertex 8:(?s node20 ?n1)
    alpha_nodes.push_back(create_alpha_node<F_binded, F_cst, F_binded>(node_vertexes[8].get(), 0, false, "",
      F_binded(0), F_cst(node20), F_binded(1) ));
    // Add Consequent term on vertex 10: (?s node30 ?n1)
    alpha_nodes.push_back(create_alpha_node<F_binded, F_cst, F_binded>(node_vertexes[10].get(), 0, false, "",
      F_binded(0), F_cst(node30), F_binded(1) ));

    // ReteMetaStore
    // create & initalize the meta store -- TODO have an expression builder with meta store
    rete_meta_store = create_rete_meta_store({}, alpha_nodes, node_vertexes);
    rete_meta_store->initialize();

    // Cretae the rdf_session and the rete_session and initialize them
    // Initialize the rete_session now that the rule base is ready
    this->rdf_session = rdf::create_rdf_session(meta_graph);
    this->rete_session = create_rete_session(rete_meta_store, this->rdf_session.get());
    this->rete_session->initialize();
  }

  ReteSessionPtr  rete_session;
  ReteMetaStorePtr rete_meta_store;
  rdf::RDFSessionPtr   rdf_session;
};

// Define the tests
TEST_F(ReteSessionTest, TestRule1Rule2A) 
{
  // rdf resource manager
  rdf::RManager * rmanager = this->rdf_session->rmgr();
  auto s0 = rmanager->create_resource("s0");
  auto p0 = rmanager->create_resource("has_node");
  auto o0 = rmanager->create_literal<int>(1);
  this->rdf_session->insert(s0, p0, o0);
  this->rete_session->execute_rules();

  auto p1 = rmanager->create_resource("plus1_node");
  auto p2 = rmanager->create_resource("plus2_node");
  EXPECT_TRUE(this->rdf_session->contains(s0, p1, rmanager->create_literal<int>(2)));
  EXPECT_TRUE(this->rdf_session->contains(s0, p2, rmanager->create_literal<int>(2)));
  std::cout<<"TestRule1Rule2A RdfSession Contains:"<<std::endl;
  std::cout<<"---------------------"<<std::endl;
  std::cout<<this->rdf_session<<std::endl<<std::endl;
}

TEST_F(ReteSessionTest, TestRule1Rule2B) 
{
  // rdf resource manager
  rdf::RManager * rmanager = this->rdf_session->rmgr();
  auto s0 = rmanager->create_resource("s0");
  auto p0 = rmanager->create_resource("has_node");
  auto o0 = rmanager->create_literal<int>(1);
  this->rdf_session->insert(s0, p0, o0);
  auto p1 = rmanager->create_resource("has_node");
  auto o1 = rmanager->create_literal<int>(2);
  this->rdf_session->insert(s0, p1, o1);

  this->rete_session->execute_rules();

  auto p0r = rmanager->create_resource("plus1_node");
  EXPECT_TRUE(this->rdf_session->contains(s0, p0r, rmanager->create_literal<int>(2)));
  EXPECT_TRUE(this->rdf_session->contains(s0, p0r, rmanager->create_literal<int>(3)));

  auto p1r = rmanager->create_resource("plus2_node");
  EXPECT_TRUE(this->rdf_session->contains(s0, p1r, rmanager->create_literal<int>(2)));
  EXPECT_TRUE(this->rdf_session->contains(s0, p1r, rmanager->create_literal<int>(3)));
  EXPECT_TRUE(this->rdf_session->contains(s0, p1r, rmanager->create_literal<int>(4)));
  std::cout<<"TestRule1Rule2B RdfSession Contains:"<<std::endl;
  std::cout<<"-------------------------------------"<<std::endl;
  std::cout<<this->rdf_session<<std::endl<<std::endl;
}

TEST_F(ReteSessionTest, TestRule3) 
{
  // rdf resource manager
  rdf::RManager * rmanager = this->rdf_session->rmgr();
  auto s0 = rmanager->create_resource("s0");
  auto p0 = rmanager->create_resource("fnode");
  auto o0 = rmanager->create_literal<int>(1);
  this->rdf_session->insert(s0, p0, o0);
  auto p1 = rmanager->create_resource("fnode");
  auto o1 = rmanager->create_literal<int>(2);
  this->rdf_session->insert(s0, p1, o1);

  this->rete_session->execute_rules();

  auto p1r = rmanager->create_resource("f2node");
  EXPECT_TRUE(this->rdf_session->contains(s0, p1r, rmanager->create_literal<int>(2)));
  EXPECT_TRUE(this->rdf_session->contains(s0, p1r, rmanager->create_literal<int>(3)));
  EXPECT_TRUE(this->rdf_session->contains(s0, p1r, rmanager->create_literal<int>(4)));
  std::cout<<"TestRule3 RdfSession Contains:"<<std::endl;
  std::cout<<"-------------------------------------"<<std::endl;
  std::cout<<this->rdf_session<<std::endl<<std::endl;
}

TEST_F(ReteSessionTest, TestRule4Rule5)
{
  // rdf resource manager
  rdf::RManager * rmanager = this->rdf_session->rmgr();
  auto s0 = rmanager->create_resource("s0");
  auto p0 = rmanager->create_resource("node1");
  auto o0 = rmanager->create_resource("n1");
  this->rdf_session->insert(s0, p0, o0);

  this->rete_session->execute_rules();

  auto p1 = rmanager->create_resource("node2");
  auto p2 = rmanager->create_resource("node3");
  EXPECT_TRUE(this->rdf_session->contains(s0, p0, o0));
  EXPECT_TRUE(this->rdf_session->contains(s0, p1, o0));
  EXPECT_FALSE(this->rdf_session->contains(s0, p2, o0));
  std::cout<<"TestRule4Rule5 RdfSession Contains:"<<std::endl;
  std::cout<<"-------------------------------------"<<std::endl;
  std::cout<<this->rdf_session<<std::endl<<std::endl;
}

TEST_F(ReteSessionTest, TestRule6Rule7)
{
  // rdf resource manager
  rdf::RManager * rmanager = this->rdf_session->rmgr();
  auto s0 = rmanager->create_resource("s0");
  auto p0 = rmanager->create_resource("node1");
  auto p1 = rmanager->create_resource("node10");
  auto o0 = rmanager->create_resource("n1");
  this->rdf_session->insert(s0, p0, o0);
  this->rdf_session->insert(s0, p1, o0);

  this->rete_session->execute_rules();

  auto rp1 = rmanager->create_resource("node20");
  auto rp2 = rmanager->create_resource("node30");
  EXPECT_TRUE(this->rdf_session->contains(s0, p0, o0));
  EXPECT_TRUE(this->rdf_session->contains(s0, rp1, o0));
  EXPECT_FALSE(this->rdf_session->contains(s0, rp2, o0));
  std::cout<<"TestRule6Rule7 RdfSession Contains:"<<std::endl;
  std::cout<<"-------------------------------------"<<std::endl;
  std::cout<<this->rdf_session<<std::endl<<std::endl;
}

TEST_F(ReteSessionTest, TestAllRules)
{
  // rdf resource manager
  rdf::RManager * rmanager = this->rdf_session->rmgr();
  { // Rule1 Rule2
    auto s0 = rmanager->create_resource("s0");
    auto p0 = rmanager->create_resource("has_node");
    auto o0 = rmanager->create_literal<int>(1);
    this->rdf_session->insert(s0, p0, o0);
    auto p1 = rmanager->create_resource("has_node");
    auto o1 = rmanager->create_literal<int>(2);
    this->rdf_session->insert(s0, p1, o1);
  }
  { // Rule 3
    auto s0 = rmanager->create_resource("s0");
    auto p0 = rmanager->create_resource("fnode");
    auto o0 = rmanager->create_literal<int>(1);
    this->rdf_session->insert(s0, p0, o0);
    auto p1 = rmanager->create_resource("fnode");
    auto o1 = rmanager->create_literal<int>(2);
    this->rdf_session->insert(s0, p1, o1);
  }
  { // Rule 4 Rule 5
    auto s0 = rmanager->create_resource("s0");
    auto p0 = rmanager->create_resource("node1");
    auto o0 = rmanager->create_resource("n1");
    this->rdf_session->insert(s0, p0, o0);
  }
  { // Rule 6 Rule 7
    auto s0 = rmanager->create_resource("s0");
    auto p0 = rmanager->create_resource("node10");
    auto o0 = rmanager->create_resource("n1");
    this->rdf_session->insert(s0, p0, o0);
  }

  this->rete_session->execute_rules();

  { // Rule1 Rule2
    auto s0 = rmanager->create_resource("s0");
    auto p0 = rmanager->create_resource("has_node");
    auto o0 = rmanager->create_literal<int>(1);
    this->rdf_session->insert(s0, p0, o0);
    auto p1 = rmanager->create_resource("has_node");
    auto o1 = rmanager->create_literal<int>(2);
    this->rdf_session->insert(s0, p1, o1);
    auto p0r = rmanager->create_resource("plus1_node");
    EXPECT_TRUE(this->rdf_session->contains(s0, p0r, rmanager->create_literal<int>(2)));
    EXPECT_TRUE(this->rdf_session->contains(s0, p0r, rmanager->create_literal<int>(3)));

    auto p1r = rmanager->create_resource("plus2_node");
    EXPECT_TRUE(this->rdf_session->contains(s0, p1r, rmanager->create_literal<int>(2)));
    EXPECT_TRUE(this->rdf_session->contains(s0, p1r, rmanager->create_literal<int>(3)));
    EXPECT_TRUE(this->rdf_session->contains(s0, p1r, rmanager->create_literal<int>(4)));
  }
  { // Rule 3
    auto s0 = rmanager->create_resource("s0");
    auto p0 = rmanager->create_resource("fnode");
    auto o0 = rmanager->create_literal<int>(1);
    this->rdf_session->insert(s0, p0, o0);
    auto p1 = rmanager->create_resource("fnode");
    auto o1 = rmanager->create_literal<int>(2);
    this->rdf_session->insert(s0, p1, o1);
    auto p1r = rmanager->create_resource("f2node");
    EXPECT_TRUE(this->rdf_session->contains(s0, p1r, rmanager->create_literal<int>(2)));
    EXPECT_TRUE(this->rdf_session->contains(s0, p1r, rmanager->create_literal<int>(3)));
    EXPECT_TRUE(this->rdf_session->contains(s0, p1r, rmanager->create_literal<int>(4)));
  }
  { // Rule 4 Rule 5
    auto s0 = rmanager->create_resource("s0");
    auto p0 = rmanager->create_resource("node1");
    auto o0 = rmanager->create_resource("n1");
    this->rdf_session->insert(s0, p0, o0);
    auto p1 = rmanager->create_resource("node2");
    auto p2 = rmanager->create_resource("node3");
    EXPECT_TRUE(this->rdf_session->contains(s0, p0, o0));
    EXPECT_TRUE(this->rdf_session->contains(s0, p1, o0));
    EXPECT_FALSE(this->rdf_session->contains(s0, p2, o0));
  }
  { // Rule 6 Rule 7
    auto s0 = rmanager->create_resource("s0");
    auto p0 = rmanager->create_resource("node10");
    auto o0 = rmanager->create_resource("n1");
    this->rdf_session->insert(s0, p0, o0);
    auto p1 = rmanager->create_resource("node20");
    auto p2 = rmanager->create_resource("node30");
    EXPECT_TRUE(this->rdf_session->contains(s0, p0, o0));
    EXPECT_TRUE(this->rdf_session->contains(s0, p1, o0));
    EXPECT_FALSE(this->rdf_session->contains(s0, p2, o0));
  }

  std::cout<<"TestAllRules RdfSession Contains:"<<std::endl;
  std::cout<<"-------------------------------------"<<std::endl;
  std::cout<<this->rdf_session<<std::endl<<std::endl;
  std::cout<<"Done done!"<<std::endl;
}

// The session's beta relations and every row they hold are in the session's arena,
// which is what lets the session be released without destroying them one by one
TEST_F(ReteSessionTest, BetaRelationsAndRowsAreInTheSessionArena)
{
  rdf::RManager * rmanager = this->rdf_session->rmgr();
  auto s0 = rmanager->create_resource("s0");
  this->rdf_session->insert(s0, rmanager->create_resource("has_node"), rmanager->create_literal<int>(1));
  this->rdf_session->insert(s0, rmanager->create_resource("has_node"), rmanager->create_literal<int>(2));
  this->rdf_session->insert(s0, rmanager->create_resource("fnode"), rmanager->create_literal<int>(1));
  this->rdf_session->insert(s0, rmanager->create_resource("node1"), rmanager->create_resource("n1"));
  this->rete_session->execute_rules();

  auto * arena = this->rete_session->arena();
  EXPECT_GT(arena->live_bytes(), 0u);
  EXPECT_GT(arena->nbr_blocks(), 0u);
  EXPECT_LE(arena->live_bytes(), arena->block_bytes());
  int nbr_rows = 0;
  for(auto * relation: this->rete_session->beta_relations()) {
    ASSERT_NE(relation, nullptr);
    EXPECT_EQ(relation->arena(), arena);
    EXPECT_TRUE(relation->containers_in(arena));
    auto itor = relation->get_all_rows_iterator();
    while(not itor->is_end()) {
      EXPECT_EQ(itor->get_row()->arena(), arena);
      ++nbr_rows;
      itor->next();
    }
  }
  EXPECT_GT(nbr_rows, 5);
}

// A session released after a retraction, and a second session released while its rdf
// session lives on. Under AddressSanitizer these are the leak and use-after-free checks.
TEST_F(ReteSessionTest, SessionReleasedAfterRetraction)
{
  rdf::RManager * rmanager = this->rdf_session->rmgr();
  auto s0 = rmanager->create_resource("s0");
  auto p0 = rmanager->create_resource("has_node");
  auto o1 = rmanager->create_literal<int>(1);
  this->rdf_session->insert(s0, p0, o1);
  this->rdf_session->insert(s0, p0, rmanager->create_literal<int>(2));
  this->rete_session->execute_rules();
  this->rdf_session->erase(s0, p0, o1);
  this->rete_session->execute_rules();

  // A second session on the same rdf session, deleted while the rdf session lives on
  auto rs2 = create_rete_session(this->rete_meta_store, this->rdf_session.get());
  ASSERT_EQ(rs2->initialize(), 0);
  rs2->execute_rules();
  EXPECT_GT(rs2->arena()->live_bytes(), 0u);
  rs2->terminate();
  rs2.reset();
}

// A session leaves each visited relation's peak on its node vertex, and the next
// session's relation is reserved from half of it before its first visit, at most
// kMaxReservedRows; the rules infer the same
TEST_F(ReteSessionTest, PeakSizesTheNextSessionsRelation)
{
  rdf::RManager * rmanager = this->rdf_session->rmgr();
  auto p0 = rmanager->create_resource("has_node");
  for(int i=0; i<300; i++) {
    this->rdf_session->insert(rmanager->create_resource("s" + std::to_string(i)), p0, rmanager->create_literal<int>(i));
  }
  auto const* nv1 = this->rete_meta_store->get_node_vertex(1);
  nv1->beta_rows_hint.store(0);

  std::size_t peak = 0;
  {
    auto rs = create_rete_session(this->rete_meta_store, this->rdf_session.get());
    ASSERT_EQ(rs->initialize(), 0);
    ASSERT_EQ(rs->execute_rules(), 0);
    peak = rs->get_beta_relation(1)->peak_size();
    EXPECT_EQ(peak, 300u);
    EXPECT_EQ(nv1->beta_rows_hint.load(), 0u);       // stored when the session is released
    rs->terminate();
  }
  EXPECT_EQ(nv1->beta_rows_hint.load(), peak);

  {
    auto rs = create_rete_session(this->rete_meta_store, this->rdf_session.get());
    ASSERT_EQ(rs->initialize(), 0);
    auto * relation = rs->get_beta_relation(1);
    EXPECT_LE(relation->capacity(), 1u);             // empty (abseil keeps one inline slot)
    relation->reserve_from_hint();
    EXPECT_GE(relation->capacity(), peak / 2);
    EXPECT_LT(relation->capacity(), peak);
    ASSERT_EQ(rs->execute_rules(), 0);
    EXPECT_EQ(relation->size(), 300u);
    auto p1 = rmanager->create_resource("plus1_node");
    for(int i=0; i<300; i++) {
      EXPECT_TRUE(this->rdf_session->contains(rmanager->create_resource("s" + std::to_string(i)), p1,
        rmanager->create_literal<int>(i + 1)));
    }
    rs->terminate();
  }

  // the reserve is bounded whatever the hint
  nv1->beta_rows_hint.store(100000000u);
  {
    auto rs = create_rete_session(this->rete_meta_store, this->rdf_session.get());
    ASSERT_EQ(rs->initialize(), 0);
    auto * relation = rs->get_beta_relation(1);
    relation->reserve_from_hint();
    EXPECT_GE(relation->capacity(), BetaRelation::kMaxReservedRows);
    EXPECT_LT(relation->capacity(), 4 * BetaRelation::kMaxReservedRows);
    rs->terminate();
  }
}

TEST(BetaRowArenaTest, ReusesFreedMemoryBySizeClass)
{
  BetaRowArena arena;
  void * p1 = arena.allocate(40);
  void * p2 = arena.allocate(40);
  EXPECT_NE(p1, p2);
  // live bytes count whole 16-byte classes: 40 bytes is 48
  EXPECT_EQ(arena.live_bytes(), 96u);
  arena.deallocate(p1, 40);
  EXPECT_EQ(arena.live_bytes(), 48u);
  // 33 to 48 bytes is one class
#if !defined(JETS_ARENA_ASAN)
  EXPECT_EQ(arena.allocate(33), p1);       // under ASan a freed block is quarantined first
#endif
  // another class does not take it
  arena.deallocate(p2, 40);
  void * p3 = arena.allocate(16);
  EXPECT_NE(p3, p2);
  EXPECT_EQ(reinterpret_cast<std::uintptr_t>(p3) % BetaRowArena::kAlign, 0u);
}

TEST(BetaRowArenaTest, LargeAllocationsAreFreedOneByOneOrInBulk)
{
  BetaRowArena arena;
  void * p1 = arena.allocate(4096);
  void * p2 = arena.allocate(100000);
  EXPECT_EQ(arena.nbr_large(), 2u);
  EXPECT_EQ(arena.large_bytes(), 104096u);
  arena.deallocate(p1, 4096);
  EXPECT_EQ(arena.nbr_large(), 1u);
  EXPECT_EQ(arena.large_bytes(), 100000u);
  (void)p2;
  arena.allocate(8192);
  arena.allocate(64);
  arena.release();
  EXPECT_EQ(arena.nbr_large(), 0u);
  EXPECT_EQ(arena.nbr_blocks(), 0u);
  EXPECT_EQ(arena.live_bytes(), 0u);
  // usable again after a release
  EXPECT_NE(arena.allocate(64), nullptr);
}

// A row, its data and its inferred-triple record are in the arena, and freeing the
// row returns all of it
TEST(BetaRowArenaTest, RowAndRecordAreInTheArena)
{
  BetaRowArena arena;
  {
    auto row = create_beta_row(&arena, nullptr, 3);
    EXPECT_EQ(row->arena(), &arena);
    EXPECT_GT(arena.live_bytes(), 0u);
    row->start_recording();
    for(int i=0; i<100; i++) row->record_inferred(rdf::Triple());
    EXPECT_EQ(row->get_recorded()->size(), 100u);
    EXPECT_EQ(row->get_recorded()->get_allocator().arena(), &arena);
    auto recorded = row->take_recorded();
    EXPECT_EQ(recorded.size(), 100u);
    EXPECT_EQ(recorded.get_allocator().arena(), &arena);
    EXPECT_TRUE(row->is_recording());
    EXPECT_EQ(row->get_recorded()->size(), 0u);
  }
  EXPECT_EQ(arena.live_bytes(), 0u);
  EXPECT_EQ(arena.nbr_large(), 0u);
}

// The allocator does not propagate: assigning a heap container to an arena container
// moves the elements into the arena, and the container stays there
TEST(BetaRowArenaTest, AssignmentKeepsAContainerInItsArena)
{
  BetaRowArena arena;
  auto * relation = create_beta_node(&arena, nullptr);
  EXPECT_TRUE(relation->containers_in(&arena));
  auto heap_relation = create_beta_node(nullptr);
  EXPECT_FALSE(heap_relation->containers_in(&arena));
  EXPECT_TRUE(heap_relation->containers_in(nullptr));

  beta_row_list in_arena{ArenaAllocator<BetaRowPtr>(&arena)};
  beta_row_list on_heap;
  on_heap.push_back(create_beta_row(&arena, nullptr, 1));
  in_arena = std::move(on_heap);
  EXPECT_EQ(in_arena.get_allocator().arena(), &arena);
  EXPECT_EQ(in_arena.size(), 1u);
  in_arena = beta_row_list();
  EXPECT_EQ(in_arena.get_allocator().arena(), &arena);
  arena.release();
}

// What a relation holds is released with the arena without being destroyed: the
// containers are built here in arena storage and never destroyed. Under
// AddressSanitizer a node that escaped to the global heap is reported as a leak.
TEST(BetaRowArenaTest, ContainersReleasedWithoutDestruction)
{
  BetaRowArena arena;
  auto * relation = create_beta_node(&arena, nullptr);
  EXPECT_EQ(relation->arena(), &arena);
  auto * rows = ::new (arena.allocate(sizeof(beta_row_set))) beta_row_set(ArenaAllocator<BetaRowPtr>(&arena));
  auto * pending = ::new (arena.allocate(sizeof(beta_row_list))) beta_row_list(ArenaAllocator<BetaRowPtr>(&arena));
  auto * idx = ::new (arena.allocate(sizeof(BetaRowIndxVec1))) BetaRowIndxVec1(
    BetaRowIndxVec1::allocator_type(ArenaAllocator<BetaRowIndexes1>(&arena)));
  // as AQVInitializeIndexesVisitor adds an index: a default-constructed temporary
  idx->push_back({});
  idx->push_back({});
  EXPECT_EQ((*idx)[1].get_allocator().arena(), &arena);
  auto graph = rdf::create_rdf_graph();
  auto * rmanager = graph->rmgr();
  for(int i=0; i<2000; i++) {
    auto row = create_beta_row(&arena, nullptr, 2);
    row->put(0, rmanager->create_literal<int>(i));
    row->put(1, rmanager->create_literal<int>(i % 7));
    row->start_recording();
    row->record_inferred(rdf::Triple());
    rows->insert(row);
    pending->push_back(row);
    (*idx)[i % 2].insert({row->get(1), row.get()});
  }
  EXPECT_EQ(rows->size(), 2000u);
  EXPECT_GT(arena.nbr_large(), 0u);    // the bucket arrays
  arena.release();
}

// Candidates are built in scratch rows, one per nesting level, reused once released
TEST_F(ReteSessionTest, CandidateRowsNestAndAreReused)
{
  auto * rs = this->rete_session.get();
  auto const* nv = this->rete_meta_store->get_node_vertex(1);
  BetaRow * outer_row = nullptr;
  {
    ReteSession::CandidateRow outer(rs, nv, 2);
    outer_row = outer.get();
    outer_row->put(0, this->rdf_session->rmgr()->create_resource("x"));
    {
      ReteSession::CandidateRow inner(rs, nv, 3);
      EXPECT_NE(inner.get(), outer_row);
      EXPECT_EQ(inner.get()->get_size(), 3);
      EXPECT_EQ(inner.get()->arena(), rs->arena());
    }
    // the inner candidate left the outer one alone
    EXPECT_EQ(outer_row->get_size(), 2);
    EXPECT_NE(outer_row->get(0), nullptr);
  }
  ReteSession::CandidateRow again(rs, nv, 2);
  EXPECT_EQ(again.get(), outer_row);
  EXPECT_EQ(again.get()->get(0), nullptr);
  EXPECT_EQ(again.get()->get_tid(), "0");
  EXPECT_EQ(again.get()->get_status(), kNone);
}

// A filter that inserts a triple re-enters the session while its candidate is live:
//   rule1> (?s fnodeX ?n1).[create_entity(?n1)] -> (?s outP ?n1)
//   rule2> (?e jets:key ?k) -> (?e seenP ?k)
// Once both relations are activated, a new (?s fnodeX ?n1) reaches rule1 through the
// graph callback; its filter's create_entity inserts (?e jets:key ?k), which reaches
// rule2 through its own callback and builds a candidate of its own, nested in rule1's.
// rule1's candidate must come back with its values and be inserted.
TEST(CandidateRowReentryTest, FilterInsertingATripleKeepsTheOuterCandidate)
{
  auto meta_graph = rdf::create_rdf_graph();
  auto * rmgr = meta_graph->rmgr();
  rmgr->initialize();
  auto fnodeX = rmgr->create_resource("fnodeX");
  auto outP = rmgr->create_resource("outP");
  auto seenP = rmgr->create_resource("seenP");
  auto jets_key = rmgr->jets()->jets__key;

  // Rows of both vertices: [?s, ?n1] from the triple
  auto ri = create_row_initializer(2);
  ri->put(0, 0 | brc_triple, "?s");
  ri->put(1, 2 | brc_triple, "?n1");
  auto filter1 = create_expr_unary_operator<CreateEntityVisitor>(0, create_expr_binded_var(1));

  NodeVertexVector node_vertexes;
  node_vertexes.push_back(create_node_vertex(nullptr, 0, 0, false, 10, {}, "", {}));
  node_vertexes.push_back(create_node_vertex(node_vertexes[0].get(), 0, 1, false, 10, filter1, "", ri));
  node_vertexes.push_back(create_node_vertex(node_vertexes[0].get(), 0, 2, false, 10, {}, "", ri));

  ReteMetaStore::AlphaNodeVector alpha_nodes;
  alpha_nodes.push_back(create_alpha_node<F_var, F_var, F_var>(node_vertexes[0].get(), 0, true, "",
    F_var("*"), F_var("*"), F_var("*") ));
  alpha_nodes.push_back(create_alpha_node<F_var, F_cst, F_var>(node_vertexes[1].get(), 0, true, "",
    F_var("?s"), F_cst(fnodeX), F_var("?n1") ));
  alpha_nodes.push_back(create_alpha_node<F_var, F_cst, F_var>(node_vertexes[2].get(), 0, true, "",
    F_var("?e"), F_cst(jets_key), F_var("?k") ));
  alpha_nodes.push_back(create_alpha_node<F_binded, F_cst, F_binded>(node_vertexes[1].get(), 0, false, "",
    F_binded(0), F_cst(outP), F_binded(1) ));
  alpha_nodes.push_back(create_alpha_node<F_binded, F_cst, F_binded>(node_vertexes[2].get(), 0, false, "",
    F_binded(0), F_cst(seenP), F_binded(1) ));

  auto rete_meta_store = create_rete_meta_store({}, alpha_nodes, node_vertexes);
  rete_meta_store->initialize();
  auto rdf_session = rdf::create_rdf_session(meta_graph);
  auto rete_session = create_rete_session(rete_meta_store, rdf_session.get());
  ASSERT_EQ(rete_session->initialize(), 0);

  auto * srmgr = rdf_session->rmgr();
  auto s1 = srmgr->create_resource("s1");
  rdf_session->insert(s1, fnodeX, srmgr->create_literal<int>(1));
  ASSERT_EQ(rete_session->execute_rules(), 0);
  EXPECT_TRUE(rdf_session->contains(s1, outP, srmgr->create_literal<int>(1)));

  // Both relations are activated: this insert runs rule1's filter inside the callback
  auto s2 = srmgr->create_resource("s2");
  auto two = srmgr->create_literal<int>(2);
  rdf_session->insert(s2, fnodeX, two);
  ASSERT_EQ(rete_session->execute_rules(), 0);

  // The nested candidate went in: rule2 saw the entity create_entity minted
  auto entity2 = srmgr->create_resource("2");
  EXPECT_TRUE(rdf_session->contains(entity2, seenP, srmgr->create_literal(std::string("2"))));
  // and the outer candidate kept its values
  EXPECT_TRUE(rdf_session->contains(s2, outP, two));
  bool found = false;
  auto itor = rete_session->get_beta_relation(1)->get_all_rows_iterator();
  while(not itor->is_end()) {
    auto const* row = itor->get_row();
    if(row->get(0) == s2 and row->get(1) == two) found = true;
    EXPECT_NE(row->get(0), entity2);
    itor->next();
  }
  EXPECT_TRUE(found);
  rete_session->terminate();
}

// A visit scans a relation's rows while a filter re-enters the session and grows that
// very relation: the scan is frozen before the set grows, and goes on over its copy.
//   rule1> (?s jets:key ?k) -> nothing
//   rule2> (?s jets:key ?k).(?s fnodeY ?m).[create_entity(?m)] -> (?s outP ?m)
// Visiting rule2's vertex scans rule1's rows; every create_entity inserts a jets:key
// triple, which the callback inserts into rule1's relation during the scan. Without
// the freeze, the set rehashes under the scan (AddressSanitizer: use after free).
//
// With `hint`, the size hint left on rule2's vertex: the re-entrant visit of rule1, which
// walks rule1's subtree, reaches rule2's relation in the middle of its first visit, not
// yet activated and holding rows; only the first visit reserves from the hint.
void
run_scan_growing_under_it(std::uint32_t hint)
{
  auto meta_graph = rdf::create_rdf_graph();
  auto * rmgr = meta_graph->rmgr();
  rmgr->initialize();
  auto fnodeY = rmgr->create_resource("fnodeY");
  auto outP = rmgr->create_resource("outP");
  auto jets_key = rmgr->jets()->jets__key;

  auto ri1 = create_row_initializer(2);
  ri1->put(0, 0 | brc_triple, "?s");
  ri1->put(1, 2 | brc_triple, "?k");
  auto ri2 = create_row_initializer(3);
  ri2->put(0, 0 | brc_parent_node, "?s");
  ri2->put(1, 1 | brc_parent_node, "?k");
  ri2->put(2, 2 | brc_triple, "?m");
  auto filter2 = create_expr_unary_operator<CreateEntityVisitor>(0, create_expr_binded_var(2));

  NodeVertexVector node_vertexes;
  node_vertexes.push_back(create_node_vertex(nullptr, 0, 0, false, 10, {}, "", {}));
  node_vertexes.push_back(create_node_vertex(node_vertexes[0].get(), 0, 1, false, 10, {}, "", ri1));
  node_vertexes.push_back(create_node_vertex(node_vertexes[1].get(), 0, 2, false, 10, filter2, "", ri2));

  ReteMetaStore::AlphaNodeVector alpha_nodes;
  alpha_nodes.push_back(create_alpha_node<F_var, F_var, F_var>(node_vertexes[0].get(), 0, true, "",
    F_var("*"), F_var("*"), F_var("*") ));
  alpha_nodes.push_back(create_alpha_node<F_var, F_cst, F_var>(node_vertexes[1].get(), 0, true, "",
    F_var("?s"), F_cst(jets_key), F_var("?k") ));
  alpha_nodes.push_back(create_alpha_node<F_binded, F_cst, F_var>(node_vertexes[2].get(), 0, true, "",
    F_binded(0), F_cst(fnodeY), F_var("?m") ));
  alpha_nodes.push_back(create_alpha_node<F_binded, F_cst, F_binded>(node_vertexes[2].get(), 0, false, "",
    F_binded(0), F_cst(outP), F_binded(2) ));

  auto rete_meta_store = create_rete_meta_store({}, alpha_nodes, node_vertexes);
  rete_meta_store->initialize();
  node_vertexes[2]->beta_rows_hint.store(hint);
  auto rdf_session = rdf::create_rdf_session(meta_graph);
  auto rete_session = create_rete_session(rete_meta_store, rdf_session.get());
  ASSERT_EQ(rete_session->initialize(), 0);

  auto * srmgr = rdf_session->rmgr();
  auto a = srmgr->create_resource("a");
  rdf_session->insert(a, jets_key, srmgr->create_literal(std::string("ka")));
  int const n = 300;
  for(int i=1; i<=n; i++) rdf_session->insert(a, fnodeY, srmgr->create_literal<int>(i));
  ASSERT_EQ(rete_session->execute_rules(), 0);

  for(int i=1; i<=n; i++) {
    EXPECT_TRUE(rdf_session->contains(a, outP, srmgr->create_literal<int>(i))) << i;
  }
  // rule1's relation: a's row and one per entity minted during the scan
  EXPECT_EQ(rete_session->get_beta_relation(1)->size(), static_cast<std::size_t>(n + 1));
  if(hint / 2 >= 2) {
    EXPECT_GE(rete_session->get_beta_relation(2)->capacity(), std::min<std::size_t>(hint / 2, n));
  }
  rete_session->terminate();
}

TEST(BetaRowScanTest, ScanSurvivesTheRelationGrowingUnderIt)
{
  run_scan_growing_under_it(0);
}

// The same, with a size hint on the relation the re-entrant visit reaches in the middle
// of its first visit: the hint is taken once, at the first visit, and the second visit
// leaves the relation and its rows alone (a debug build asserted there before)
TEST(BetaRowScanTest, ReentrantVisitReachesARelationInItsFirstVisit)
{
  run_scan_growing_under_it(1000);
}

// I-85: removing a row from a relation removes its own index entries, not those of
// the other rows under the same key.
//   rule0> (?s inGroup ?g).not(?s blocked ?b).(?g q ?v) -> (?s outP ?v)
// The negation's relation holds [x1, g] and [x2, g], indexed under g for the last
// antecedent. Blocking x1 removes [x1, g]; a later (g q v) must still join [x2, g].
// Run A blocks x1 after the relation is built, run B before: the rules must end
// with the same graph, (x2 outP v) and not (x1 outP v).
struct IndexRemovalRun {
  bool x1_inferred;
  bool x2_inferred;
};

IndexRemovalRun
run_index_removal(bool block_first)
{
  auto meta_graph = rdf::create_rdf_graph();
  auto * rmgr = meta_graph->rmgr();
  rmgr->initialize();
  auto inGroup = rmgr->create_resource("inGroup");
  auto blocked = rmgr->create_resource("blocked");
  auto q = rmgr->create_resource("q");
  auto outP = rmgr->create_resource("outP");

  auto ri1 = create_row_initializer(2);
  ri1->put(0, 0 | brc_triple, "?s");
  ri1->put(1, 2 | brc_triple, "?g");
  auto ri2 = create_row_initializer(2);
  ri2->put(0, 0 | brc_parent_node, "?s");
  ri2->put(1, 1 | brc_parent_node, "?g");
  auto ri3 = create_row_initializer(3);
  ri3->put(0, 0 | brc_parent_node, "?s");
  ri3->put(1, 1 | brc_parent_node, "?g");
  ri3->put(2, 2 | brc_triple, "?v");

  NodeVertexVector node_vertexes;
  node_vertexes.push_back(create_node_vertex(nullptr, 0, 0, false, 10, {}, "", {}));
  node_vertexes.push_back(create_node_vertex(node_vertexes[0].get(), 0, 1, false, 10, {}, "", ri1));
  node_vertexes.push_back(create_node_vertex(node_vertexes[1].get(), 0, 2, true, 10, {}, "", ri2));
  node_vertexes.push_back(create_node_vertex(node_vertexes[2].get(), 0, 3, false, 10, {}, "", ri3));

  ReteMetaStore::AlphaNodeVector alpha_nodes;
  alpha_nodes.push_back(create_alpha_node<F_var, F_var, F_var>(node_vertexes[0].get(), 0, true, "",
    F_var("*"), F_var("*"), F_var("*") ));
  alpha_nodes.push_back(create_alpha_node<F_var, F_cst, F_var>(node_vertexes[1].get(), 0, true, "",
    F_var("?s"), F_cst(inGroup), F_var("?g") ));
  alpha_nodes.push_back(create_alpha_node<F_binded, F_cst, F_var>(node_vertexes[2].get(), 0, true, "",
    F_binded(0), F_cst(blocked), F_var("?b") ));
  alpha_nodes.push_back(create_alpha_node<F_binded, F_cst, F_var>(node_vertexes[3].get(), 0, true, "",
    F_binded(1), F_cst(q), F_var("?v") ));
  alpha_nodes.push_back(create_alpha_node<F_binded, F_cst, F_binded>(node_vertexes[3].get(), 0, false, "",
    F_binded(0), F_cst(outP), F_binded(2) ));

  auto rete_meta_store = create_rete_meta_store({}, alpha_nodes, node_vertexes);
  rete_meta_store->initialize();
  auto rdf_session = rdf::create_rdf_session(meta_graph);
  auto rete_session = create_rete_session(rete_meta_store, rdf_session.get());
  EXPECT_EQ(rete_session->initialize(), 0);

  auto * srmgr = rdf_session->rmgr();
  auto g = srmgr->create_resource("g");
  auto b = srmgr->create_resource("b");
  auto x1 = srmgr->create_resource("x1");
  auto x2 = srmgr->create_resource("x2");
  auto v = srmgr->create_resource("v");
  rdf_session->insert(x1, inGroup, g);
  rdf_session->insert(x2, inGroup, g);
  if(block_first) rdf_session->insert(x1, blocked, b);
  EXPECT_EQ(rete_session->execute_rules(), 0);
  if(not block_first) {
    rdf_session->insert(x1, blocked, b);    // removes [x1, g] from the relation the join reads
    EXPECT_EQ(rete_session->execute_rules(), 0);
  }
  rdf_session->insert(g, q, v);
  EXPECT_EQ(rete_session->execute_rules(), 0);

  IndexRemovalRun result{rdf_session->contains(x1, outP, v), rdf_session->contains(x2, outP, v)};
  rete_session->terminate();
  return result;
}

TEST(BetaRowIndexTest, RemovingARowKeepsItsSiblingsUnderTheKey)
{
  auto run_a = run_index_removal(false);
  auto run_b = run_index_removal(true);
  EXPECT_FALSE(run_b.x1_inferred);
  EXPECT_TRUE(run_b.x2_inferred);
  // the same graph whichever came first
  EXPECT_EQ(run_a.x1_inferred, run_b.x1_inferred);
  EXPECT_EQ(run_a.x2_inferred, run_b.x2_inferred);
}

// I-91: a candidate whose filter re-enters the session and removes the candidate's
// own support is not inserted.
//   rule0> (?s inGroup ?g).not(?s jets:key ?k).(?s q ?v).[create_entity(?v)] -> (?s outP ?v)
// or, with `negated_child`, the filter on a negated antecedent below:
//   rule0> (?s inGroup ?g).not(?s jets:key ?k).(?s q ?v).not(?s r ?z).[create_entity(?v)] -> (?s outP ?v)
// With (xi q "xi"), the filter of xi's candidate mints entity xi, whose jets:key
// triple removes xi's row from the not(jets:key) relation: the candidate's support
// is gone before its filter returns. Inserting the candidate anyway inferred
// (xi outP "xi"), which nothing retracted. Whatever the order the facts arrive in,
// the rules must end with the same graph as when the jets:key triples are there from
// the start, where the rule never fires.
enum class ReentryOrder { kAllAtOnce, kJoinAfterActivation, kKeysFirst };

struct ReentryRun {
  int inferred;          // (xi outP "xi") in the graph
  int keys;              // (xi jets:key "xi") in the graph
};

ReentryRun
run_reentry(ReentryOrder order, bool negated_child)
{
  auto meta_graph = rdf::create_rdf_graph();
  auto * rmgr = meta_graph->rmgr();
  rmgr->initialize();
  auto inGroup = rmgr->create_resource("inGroup");
  auto q = rmgr->create_resource("q");
  auto r = rmgr->create_resource("r");
  auto outP = rmgr->create_resource("outP");
  auto jets_key = rmgr->jets()->jets__key;

  auto ri1 = create_row_initializer(2);
  ri1->put(0, 0 | brc_triple, "?s");
  ri1->put(1, 2 | brc_triple, "?g");
  auto ri2 = create_row_initializer(2);
  ri2->put(0, 0 | brc_parent_node, "?s");
  ri2->put(1, 1 | brc_parent_node, "?g");
  auto ri3 = create_row_initializer(3);
  ri3->put(0, 0 | brc_parent_node, "?s");
  ri3->put(1, 1 | brc_parent_node, "?g");
  ri3->put(2, 2 | brc_triple, "?v");
  auto ri4 = create_row_initializer(3);
  ri4->put(0, 0 | brc_parent_node, "?s");
  ri4->put(1, 1 | brc_parent_node, "?g");
  ri4->put(2, 2 | brc_parent_node, "?v");
  auto filter = create_expr_unary_operator<CreateEntityVisitor>(0, create_expr_binded_var(2));

  NodeVertexVector node_vertexes;
  node_vertexes.push_back(create_node_vertex(nullptr, 0, 0, false, 10, {}, "", {}));
  node_vertexes.push_back(create_node_vertex(node_vertexes[0].get(), 0, 1, false, 10, {}, "", ri1));
  node_vertexes.push_back(create_node_vertex(node_vertexes[1].get(), 0, 2, true, 10, {}, "", ri2));
  node_vertexes.push_back(create_node_vertex(node_vertexes[2].get(), 0, 3, false, 10,
    negated_child ? ExprBasePtr() : filter, "", ri3));
  if(negated_child) {
    node_vertexes.push_back(create_node_vertex(node_vertexes[3].get(), 0, 4, true, 10, filter, "", ri4));
  }
  int last = negated_child ? 4 : 3;

  ReteMetaStore::AlphaNodeVector alpha_nodes;
  alpha_nodes.push_back(create_alpha_node<F_var, F_var, F_var>(node_vertexes[0].get(), 0, true, "",
    F_var("*"), F_var("*"), F_var("*") ));
  alpha_nodes.push_back(create_alpha_node<F_var, F_cst, F_var>(node_vertexes[1].get(), 0, true, "",
    F_var("?s"), F_cst(inGroup), F_var("?g") ));
  alpha_nodes.push_back(create_alpha_node<F_binded, F_cst, F_var>(node_vertexes[2].get(), 0, true, "",
    F_binded(0), F_cst(jets_key), F_var("?k") ));
  alpha_nodes.push_back(create_alpha_node<F_binded, F_cst, F_var>(node_vertexes[3].get(), 0, true, "",
    F_binded(0), F_cst(q), F_var("?v") ));
  if(negated_child) {
    alpha_nodes.push_back(create_alpha_node<F_binded, F_cst, F_var>(node_vertexes[4].get(), 0, true, "",
      F_binded(0), F_cst(r), F_var("?z") ));
  }
  alpha_nodes.push_back(create_alpha_node<F_binded, F_cst, F_binded>(node_vertexes[last].get(), 0, false, "",
    F_binded(0), F_cst(outP), F_binded(2) ));

  auto rete_meta_store = create_rete_meta_store({}, alpha_nodes, node_vertexes);
  rete_meta_store->initialize();
  auto rdf_session = rdf::create_rdf_session(meta_graph);
  auto rete_session = create_rete_session(rete_meta_store, rdf_session.get());
  EXPECT_EQ(rete_session->initialize(), 0);

  auto * srmgr = rdf_session->rmgr();
  auto g = srmgr->create_resource("g");
  int const n = 4;
  auto name = [](int i) { return std::string("x") + std::to_string(i); };
  for(int i=0; i<n; i++) {
    auto x = srmgr->create_resource(name(i));
    rdf_session->insert(x, inGroup, g);
    if(order == ReentryOrder::kKeysFirst) rdf_session->insert(x, jets_key, srmgr->create_literal(name(i)));
  }
  auto join = [&]() {
    for(int i=0; i<n; i++) rdf_session->insert(srmgr->create_resource(name(i)), q, srmgr->create_literal(name(i)));
  };
  if(order != ReentryOrder::kJoinAfterActivation) join();
  EXPECT_EQ(rete_session->execute_rules(), 0);
  if(order == ReentryOrder::kJoinAfterActivation) {
    join();
    EXPECT_EQ(rete_session->execute_rules(), 0);
  }

  ReentryRun result{0, 0};
  for(int i=0; i<n; i++) {
    auto x = srmgr->create_resource(name(i));
    auto v = srmgr->create_literal(name(i));
    if(rdf_session->contains(x, outP, v)) ++result.inferred;
    if(rdf_session->contains(x, jets_key, v)) ++result.keys;
  }
  rete_session->terminate();
  return result;
}

void
expect_same_graph_whatever_the_order(bool negated_child)
{
  auto reference = run_reentry(ReentryOrder::kKeysFirst, negated_child);
  EXPECT_EQ(reference.inferred, 0);
  EXPECT_EQ(reference.keys, 4);
  for(auto order: {ReentryOrder::kAllAtOnce, ReentryOrder::kJoinAfterActivation}) {
    auto run = run_reentry(order, negated_child);
    EXPECT_EQ(run.keys, reference.keys) << static_cast<int>(order);
    EXPECT_EQ(run.inferred, reference.inferred) << static_cast<int>(order);
  }
}

TEST(CandidateRowReentryTest, FilterRemovingItsOwnSupportInfersNothing)
{
  expect_same_graph_whatever_the_order(false);
}

TEST(CandidateRowReentryTest, FilterOnANegationRemovingItsOwnSupportInfersNothing)
{
  expect_same_graph_whatever_the_order(true);
}

// The same defect through the only write a compiled filter can make, the error path,
// in the case the Go engine's test runs too:
//   rule0> (?s inGroup ?g).not(jets:iState jets:exception ?e).(?g q ?v).[not(?v) != 1] -> (?s outP ?v)
// with (s inGroup g) and (g q "maybe"). not("maybe") raises a jets:exception (text
// that is not a bool) and gives null, and null != 1 is true: the filter keeps the
// candidate after its own jets:exception removed the candidate's parent row from the
// not(jets:exception) relation. Run A asserts the data only; run B asserts the same
// jets:exception first. Both must end without (s outP "maybe").
struct ErrorPathRun {
  bool inferred;                 // (s outP "maybe")
  std::string exception;         // the jets:exception text, empty if none
};

ErrorPathRun
run_error_path(std::string const& exception_first)
{
  auto meta_graph = rdf::create_rdf_graph();
  auto * rmgr = meta_graph->rmgr();
  rmgr->initialize();
  auto inGroup = rmgr->create_resource("inGroup");
  auto q = rmgr->create_resource("q");
  auto outP = rmgr->create_resource("outP");
  auto istate = rmgr->jets()->jets__istate;
  auto exception = rmgr->jets()->jets__exception;

  auto ri1 = create_row_initializer(2);
  ri1->put(0, 0 | brc_triple, "?s");
  ri1->put(1, 2 | brc_triple, "?g");
  auto ri2 = create_row_initializer(2);
  ri2->put(0, 0 | brc_parent_node, "?s");
  ri2->put(1, 1 | brc_parent_node, "?g");
  auto ri3 = create_row_initializer(3);
  ri3->put(0, 0 | brc_parent_node, "?s");
  ri3->put(1, 1 | brc_parent_node, "?g");
  ri3->put(2, 2 | brc_triple, "?v");
  auto filter = create_expr_binary_operator<NeVisitor>(0,
    create_expr_unary_operator<NotVisitor>(0, create_expr_binded_var(2)),
    create_expr_cst(rdf::RdfAstType(rdf::LInt32(1))));

  NodeVertexVector node_vertexes;
  node_vertexes.push_back(create_node_vertex(nullptr, 0, 0, false, 10, {}, "", {}));
  node_vertexes.push_back(create_node_vertex(node_vertexes[0].get(), 0, 1, false, 10, {}, "", ri1));
  node_vertexes.push_back(create_node_vertex(node_vertexes[1].get(), 0, 2, true, 10, {}, "", ri2));
  node_vertexes.push_back(create_node_vertex(node_vertexes[2].get(), 0, 3, false, 10, filter, "", ri3));

  ReteMetaStore::AlphaNodeVector alpha_nodes;
  alpha_nodes.push_back(create_alpha_node<F_var, F_var, F_var>(node_vertexes[0].get(), 0, true, "",
    F_var("*"), F_var("*"), F_var("*") ));
  alpha_nodes.push_back(create_alpha_node<F_var, F_cst, F_var>(node_vertexes[1].get(), 0, true, "",
    F_var("?s"), F_cst(inGroup), F_var("?g") ));
  alpha_nodes.push_back(create_alpha_node<F_cst, F_cst, F_var>(node_vertexes[2].get(), 0, true, "",
    F_cst(istate), F_cst(exception), F_var("?e") ));
  alpha_nodes.push_back(create_alpha_node<F_binded, F_cst, F_var>(node_vertexes[3].get(), 0, true, "",
    F_binded(1), F_cst(q), F_var("?v") ));
  alpha_nodes.push_back(create_alpha_node<F_binded, F_cst, F_binded>(node_vertexes[3].get(), 0, false, "",
    F_binded(0), F_cst(outP), F_binded(2) ));

  auto rete_meta_store = create_rete_meta_store({}, alpha_nodes, node_vertexes);
  rete_meta_store->initialize();
  auto rdf_session = rdf::create_rdf_session(meta_graph);
  auto rete_session = create_rete_session(rete_meta_store, rdf_session.get());
  EXPECT_EQ(rete_session->initialize(), 0);

  auto * srmgr = rdf_session->rmgr();
  auto s = srmgr->create_resource("s");
  auto g = srmgr->create_resource("g");
  auto maybe = srmgr->create_literal(std::string("maybe"));
  if(not exception_first.empty()) rdf_session->insert(istate, exception, srmgr->create_literal(exception_first));
  rdf_session->insert(s, inGroup, g);
  rdf_session->insert(g, q, maybe);
  EXPECT_EQ(rete_session->execute_rules(), 0);

  ErrorPathRun result{rdf_session->contains(s, outP, maybe), ""};
  auto e = rdf_session->get_object(istate, exception);
  if(e) {
    if(auto const* text = boost::get<rdf::LString>(e)) result.exception = text->data;
  }
  rete_session->terminate();
  return result;
}

TEST(CandidateRowReentryTest, ErrorPathRemovingItsOwnSupportInfersNothing)
{
  auto run_a = run_error_path("");
  ASSERT_FALSE(run_a.exception.empty());           // the filter raised
  auto run_b = run_error_path(run_a.exception);
  EXPECT_EQ(run_b.exception, run_a.exception);
  EXPECT_FALSE(run_b.inferred);
  // the same graph whichever came first
  EXPECT_EQ(run_a.inferred, run_b.inferred);
}

// The index returns a key's rows in the order std::unordered_multimap's equal_range
// did, newest first, through any number of rehashes; erase(key) removes all of them
TEST(BetaRowIndexTest, RowsAreInTheMultimapsOrder)
{
  BetaRowArena arena;
  BetaRowIndexes1 index{BetaRowIndexes1::allocator_type(&arena)};
  std::unordered_multimap<rdf::r_index, BetaRow const*, absl::Hash<rdf::r_index>> multimap;
  auto key_of = [](int k) { return reinterpret_cast<rdf::r_index>(static_cast<std::uintptr_t>(16 * (k + 1))); };
  auto row_of = [](int r) { return reinterpret_cast<BetaRow const*>(static_cast<std::uintptr_t>(16 * (r + 1))); };
  for(int r=0; r<5000; r++) {
    int k = (r * 7919) % 97;
    index.insert({key_of(k), row_of(r)});
    multimap.insert({key_of(k), row_of(r)});
  }
  EXPECT_EQ(index.get_allocator().arena(), &arena);
  for(int k=0; k<97; k++) {
    auto range = multimap.equal_range(key_of(k));
    std::vector<BetaRow const*> expected;
    for(auto it = range.first; it != range.second; ++it) expected.push_back(it->second);
    auto rows = index.rows(key_of(k));
    std::vector<BetaRow const*> got(std::make_reverse_iterator(rows.last), std::make_reverse_iterator(rows.first));
    EXPECT_EQ(got, expected) << "key " << k;
  }
  // one row out from under a key: the others keep the multimap's order
  for(int k: {3, 40}) {
    auto range = multimap.equal_range(key_of(k));
    auto middle = std::next(range.first, std::distance(range.first, range.second) / 2);
    auto const* row = middle->second;
    multimap.erase(middle);
    EXPECT_EQ(index.erase(key_of(k), row), 1u);
    EXPECT_EQ(index.erase(key_of(k), row), 0u);
    range = multimap.equal_range(key_of(k));
    std::vector<BetaRow const*> expected;
    for(auto it = range.first; it != range.second; ++it) expected.push_back(it->second);
    auto rows = index.rows(key_of(k));
    std::vector<BetaRow const*> got(std::make_reverse_iterator(rows.last), std::make_reverse_iterator(rows.first));
    EXPECT_EQ(got, expected) << "key " << k;
  }
  // the last row out takes the key with it
  auto single = row_of(100000);
  index.insert({key_of(1000), single});
  EXPECT_EQ(index.size(), 98u);
  EXPECT_EQ(index.erase(key_of(1000), single), 1u);
  auto none = index.rows(key_of(1000));
  EXPECT_EQ(none.first, none.last);
  EXPECT_EQ(index.size(), 97u);
}

// A frozen scan of the row set returns what was left when it was frozen, except a row
// that has left the set since; it holds those rows; and it has left the registry
TEST(BetaRowScanTest, FrozenSetScanSkipsRowsThatLeft)
{
  BetaRowArena arena;
  BetaRowScanRegistry registry;
  beta_row_set rows{ArenaAllocator<BetaRowPtr>(&arena)};
  auto graph = rdf::create_rdf_graph();
  auto * rmanager = graph->rmgr();
  for(int i=0; i<10; i++) {
    auto row = create_beta_row(&arena, nullptr, 1);
    row->put(0, rmanager->create_literal<int>(i));
    rows.insert(row);
  }
  auto value_of = [](BetaRow const* row) { return boost::get<rdf::LInt32>(*row->get(0)).data; };
  std::vector<int> returned;
  std::set<int> erased;
  {
    BetaRowSetScan scan(&registry, BetaRowScanRegistry::kAllRows, rows.begin(), rows.end(), &rows);
    EXPECT_EQ(registry.head[BetaRowScanRegistry::kAllRows], &scan);
    for(int i=0; i<4; i++) { returned.push_back(value_of(scan.get_row())); scan.next(); }
    registry.freeze(BetaRowScanRegistry::kAllRows);
    EXPECT_EQ(registry.head[BetaRowScanRegistry::kAllRows], nullptr);
    // two of the rows the scan has yet to return leave the set, which then grows
    int n = 0;
    for(auto itor = rows.begin(); itor != rows.end() and n < 2; ) {
      int v = value_of(itor->get());
      if(std::find(returned.begin(), returned.end(), v) == returned.end()) {
        erased.insert(v);
        rows.erase(itor++);
        ++n;
      } else {
        ++itor;
      }
    }
    for(int i=0; i<1000; i++) {
      auto row = create_beta_row(&arena, nullptr, 1);
      row->put(0, rmanager->create_literal<int>(1000 + i));
      rows.insert(row);
    }
    for(; not scan.is_end(); scan.next()) returned.push_back(value_of(scan.get_row()));
  }
  ASSERT_EQ(erased.size(), 2u);
  EXPECT_EQ(returned.size(), 8u);
  std::set<int> values(returned.begin(), returned.end());
  EXPECT_EQ(values.size(), 8u);
  for(int v: erased) EXPECT_EQ(values.count(v), 0u) << v;
  EXPECT_LT(*values.rbegin(), 10);
}

// A frozen scan of the pending list, which a nested visit clears, returns and holds
// what was left; a removed row is on the list to notify the children, so none is skipped
TEST(BetaRowScanTest, FrozenListScanHoldsItsRowsThroughAClear)
{
  BetaRowArena arena;
  BetaRowScanRegistry registry;
  beta_row_list pending{ArenaAllocator<BetaRowPtr>(&arena)};
  auto graph = rdf::create_rdf_graph();
  auto * rmanager = graph->rmgr();
  for(int i=0; i<10; i++) {
    auto row = create_beta_row(&arena, nullptr, 1);
    row->put(0, rmanager->create_literal<int>(i));
    pending.push_back(row);                           // the list alone holds the rows
  }
  std::set<int> values;
  auto value_of = [](BetaRow const* row) { return boost::get<rdf::LInt32>(*row->get(0)).data; };
  {
    BetaRowListScan scan(&registry, BetaRowScanRegistry::kPendingRows, pending.begin(), pending.end(), nullptr);
    for(int i=0; i<3; i++) { values.insert(value_of(scan.get_row())); scan.next(); }
    registry.freeze(BetaRowScanRegistry::kPendingRows);
    pending.clear();
    for(; not scan.is_end(); scan.next()) values.insert(value_of(scan.get_row()));
  }
  EXPECT_EQ(values.size(), 10u);
  EXPECT_EQ(registry.head[BetaRowScanRegistry::kPendingRows], nullptr);
}

// An index scan of a relation is live when a filter re-enters the session and a
// negated antecedent removes one of the rows it scans:
//   rule0> (?s inGroup ?g).not(?s jets:key ?k).(?g q ?v).[create_entity(?v)] -> (?s outP ?v)
// The relation of not(...) holds [x1, g] and [x2, g]. A triple (g q "x") reaches the
// last vertex, whose scan of that relation's index under g returns both rows. The
// first row's filter mints entity x, one of the rows' subject: (x jets:key "x")
// removes that row from the relation, and the nested visit drops the pending list's
// hold on it. The scan, frozen by the index change, holds the rows it has left.
struct NegationUnderIndexScan {
  std::size_t rows_left;          // in the negation's relation
  bool first_inferred;            // (first outP v)
  bool second_inferred;           // (second outP v)
};

// With `negation_infers`, the negation's vertex has a consequent, (?s marked ?g): a
// row removed from it is then marked kDeleted and kept in the relation's set, though
// it has left the index
NegationUnderIndexScan
run_negation_under_index_scan(bool remove_first, bool negation_infers = false)
{
  auto meta_graph = rdf::create_rdf_graph();
  auto * rmgr = meta_graph->rmgr();
  rmgr->initialize();
  auto inGroup = rmgr->create_resource("inGroup");
  auto q = rmgr->create_resource("q");
  auto outP = rmgr->create_resource("outP");
  auto marked = rmgr->create_resource("marked");
  auto jets_key = rmgr->jets()->jets__key;

  auto ri1 = create_row_initializer(2);
  ri1->put(0, 0 | brc_triple, "?s");
  ri1->put(1, 2 | brc_triple, "?g");
  auto ri2 = create_row_initializer(2);
  ri2->put(0, 0 | brc_parent_node, "?s");
  ri2->put(1, 1 | brc_parent_node, "?g");
  auto ri3 = create_row_initializer(3);
  ri3->put(0, 0 | brc_parent_node, "?s");
  ri3->put(1, 1 | brc_parent_node, "?g");
  ri3->put(2, 2 | brc_triple, "?v");
  auto filter3 = create_expr_unary_operator<CreateEntityVisitor>(0, create_expr_binded_var(2));

  NodeVertexVector node_vertexes;
  node_vertexes.push_back(create_node_vertex(nullptr, 0, 0, false, 10, {}, "", {}));
  node_vertexes.push_back(create_node_vertex(node_vertexes[0].get(), 0, 1, false, 10, {}, "", ri1));
  node_vertexes.push_back(create_node_vertex(node_vertexes[1].get(), 0, 2, true, 10, {}, "", ri2));
  node_vertexes.push_back(create_node_vertex(node_vertexes[2].get(), 0, 3, false, 10, filter3, "", ri3));

  ReteMetaStore::AlphaNodeVector alpha_nodes;
  alpha_nodes.push_back(create_alpha_node<F_var, F_var, F_var>(node_vertexes[0].get(), 0, true, "",
    F_var("*"), F_var("*"), F_var("*") ));
  alpha_nodes.push_back(create_alpha_node<F_var, F_cst, F_var>(node_vertexes[1].get(), 0, true, "",
    F_var("?s"), F_cst(inGroup), F_var("?g") ));
  alpha_nodes.push_back(create_alpha_node<F_binded, F_cst, F_var>(node_vertexes[2].get(), 0, true, "",
    F_binded(0), F_cst(jets_key), F_var("?k") ));
  alpha_nodes.push_back(create_alpha_node<F_binded, F_cst, F_var>(node_vertexes[3].get(), 0, true, "",
    F_binded(1), F_cst(q), F_var("?v") ));
  alpha_nodes.push_back(create_alpha_node<F_binded, F_cst, F_binded>(node_vertexes[3].get(), 0, false, "",
    F_binded(0), F_cst(outP), F_binded(2) ));
  if(negation_infers) {
    alpha_nodes.push_back(create_alpha_node<F_binded, F_cst, F_binded>(node_vertexes[2].get(), 0, false, "",
      F_binded(0), F_cst(marked), F_binded(1) ));
  }

  auto rete_meta_store = create_rete_meta_store({}, alpha_nodes, node_vertexes);
  rete_meta_store->initialize();
  auto rdf_session = rdf::create_rdf_session(meta_graph);
  auto rete_session = create_rete_session(rete_meta_store, rdf_session.get());
  EXPECT_EQ(rete_session->initialize(), 0);

  auto * srmgr = rdf_session->rmgr();
  auto g = srmgr->create_resource("g");
  auto x1 = srmgr->create_resource("x1");
  auto x2 = srmgr->create_resource("x2");
  rdf_session->insert(x1, inGroup, g);
  rdf_session->insert(x2, inGroup, g);
  EXPECT_EQ(rete_session->execute_rules(), 0);
  auto * negation = rete_session->get_beta_relation(2);
  EXPECT_EQ(negation->size(), 2u);

  // the rows in the order the scan returns them
  rdf::r_index first = nullptr, second = nullptr;
  {
    auto scan = negation->get_idx1_rows_iterator(node_vertexes[3]->antecedent_query_key, g);
    if(not scan->is_end()) first = scan->get_row()->get(0);
    scan->next();
    if(not scan->is_end()) second = scan->get_row()->get(0);
  }
  EXPECT_NE(first, nullptr);
  EXPECT_NE(second, nullptr);
  auto removed = remove_first ? first : second;
  auto v = srmgr->create_literal(std::string(removed == x1 ? "x1" : "x2"));
  rdf_session->insert(g, q, v);
  EXPECT_EQ(rete_session->execute_rules(), 0);

  NegationUnderIndexScan result{negation->size(),
    rdf_session->contains(first, outP, v), rdf_session->contains(second, outP, v)};
  rete_session->terminate();
  return result;
}

// The row removed is ahead of the scan: the scan, frozen, holds it, then skips it
// (AddressSanitizer: use after poison if it did not hold it)
TEST(BetaRowScanTest, IndexScanSurvivesANegationRemovingARowAheadOfIt)
{
  auto result = run_negation_under_index_scan(false);
  EXPECT_EQ(result.rows_left, 1u);
  EXPECT_TRUE(result.first_inferred);
  EXPECT_FALSE(result.second_inferred);
}

// The row removed ahead of the scan has consequents: it stays in the relation's set,
// marked kDeleted, but has left the index, which an unfrozen scan would not return it
// from; the frozen scan skips it too
TEST(BetaRowScanTest, IndexScanSkipsARowThatLeftTheIndexButNotTheSet)
{
  auto result = run_negation_under_index_scan(false, true);
  EXPECT_TRUE(result.first_inferred);
  EXPECT_FALSE(result.second_inferred);
}

// The row removed is the one the scan returned, whose filter is running: the scan
// does not skip it from under its caller, and goes on to the next row. The first row's
// candidate lost its parent row to its own filter, so it is not inserted (I-91; before
// the fix it was, and (first outP v) was inferred and never retracted)
TEST(BetaRowScanTest, IndexScanSurvivesANegationRemovingTheRowItReturned)
{
  auto result = run_negation_under_index_scan(true);
  EXPECT_EQ(result.rows_left, 1u);
  EXPECT_FALSE(result.first_inferred);
  EXPECT_TRUE(result.second_inferred);
}

// The row set's transparent hash is absl::Hash<BetaRowPtr>'s, value for value, so a
// candidate and its row hash alike; and equality is by values
TEST(BetaRowCandidateTest, HashAndEqualityMatchTheRowPtrs)
{
  auto graph = rdf::create_rdf_graph();
  auto * rmanager = graph->rmgr();
  BetaRowArena arena;
  BetaRowHash hash;
  BetaRowEqual equal;
  for(int n=0; n<200; n++) {
    int size = n % 7;
    auto row = create_beta_row(&arena, nullptr, size);
    BetaRow candidate(nullptr, size, &arena);
    for(int k=0; k<size; k++) {
      row->put(k, rmanager->create_literal<int>(n * 10 + k));
      candidate.put(k, rmanager->create_literal<int>(n * 10 + k));
    }
    EXPECT_EQ(hash(row), absl::Hash<BetaRowPtr>{}(row));
    EXPECT_EQ(hash(candidate), absl::Hash<BetaRowPtr>{}(row));
    EXPECT_TRUE(equal(row, candidate));
    EXPECT_TRUE(equal(candidate, row));
    if(size > 0) {
      candidate.put(0, rmanager->create_literal<int>(-1));
      EXPECT_FALSE(equal(row, candidate));
    }
  }
}

// The cached hash is the hash of the row's values: on a fresh row, after put() changes
// a value (which drops it), and on a kept row made from a candidate
TEST(BetaRowCandidateTest, CachedHashIsTheValuesHash)
{
  auto graph = rdf::create_rdf_graph();
  auto * rmanager = graph->rmgr();
  BetaRowArena arena;
  auto values_hash = [](BetaRow const& row) { return absl::Hash<BetaRow*>{}(const_cast<BetaRow*>(&row)); };
  for(int n=0; n<100; n++) {
    int size = 1 + n % 6;
    BetaRow candidate(nullptr, size, &arena);
    for(int k=0; k<size; k++) candidate.put(k, rmanager->create_literal<int>(n * 10 + k));
    EXPECT_EQ(candidate.hash(), values_hash(candidate));
    EXPECT_EQ(BetaRowHash{}(candidate), values_hash(candidate));
    auto kept = create_beta_row(&arena, candidate);
    EXPECT_EQ(kept->hash(), candidate.hash());
    EXPECT_EQ(BetaRowHash{}(kept), values_hash(*kept));
    // a value changed after the hash was taken
    auto before = candidate.hash();
    candidate.put(0, rmanager->create_literal<int>(-1 - n));
    EXPECT_EQ(candidate.hash(), values_hash(candidate));
    EXPECT_NE(candidate.hash(), before);
    EXPECT_FALSE(BetaRowEqual{}(kept, candidate));
    // reset drops it too
    candidate.reset(nullptr, size);
    EXPECT_EQ(candidate.hash(), values_hash(candidate));
  }
}

// A kept row copies the candidate's values and tid into the arena; the tid is
// "<parent tid>:<vertex>", as it was written with an ostringstream
TEST(BetaRowCandidateTest, KeptRowCopiesTheCandidate)
{
  auto graph = rdf::create_rdf_graph();
  auto * rmanager = graph->rmgr();
  BetaRowArena arena;
  auto ri = create_row_initializer(2);
  ri->put(0, 0 | brc_parent_node, "?a");
  ri->put(1, 2 | brc_triple, "?b");
  auto nv7 = create_node_vertex(nullptr, 0, 7, false, 10, {}, "", ri);
  auto nv12 = create_node_vertex(nullptr, 0, 12, false, 10, {}, "", ri);
  auto parent = create_beta_row(&arena, nv7.get(), 2);
  parent->put(0, rmanager->create_resource("p0"));
  parent->put(1, rmanager->create_resource("p1"));
  BetaRow candidate(nv12.get(), 2, &arena);
  rdf::Triple t3(rmanager->create_resource("s"), rmanager->create_resource("p"), rmanager->create_resource("o"));
  ASSERT_EQ(candidate.initialize(ri.get(), parent.get(), &t3), 0);
  EXPECT_EQ(candidate.get_tid(), "0:12");
  EXPECT_EQ(candidate.get(0), parent->get(0));
  EXPECT_EQ(candidate.get(1), t3.object);

  auto kept = create_beta_row(&arena, candidate);
  EXPECT_EQ(kept->arena(), &arena);
  EXPECT_EQ(kept->get_node_vertex(), nv12.get());
  EXPECT_EQ(kept->get_tid(), "0:12");
  EXPECT_TRUE(*kept == candidate);
  EXPECT_EQ(kept->get_status(), kNone);

  BetaRow grandchild(nv7.get(), 2, &arena);
  ASSERT_EQ(grandchild.initialize(ri.get(), kept.get(), &t3), 0);
  EXPECT_EQ(grandchild.get_tid(), "0:12:7");
}

TEST(BetaRowArenaTest, RelationRefusesARowFromAnotherArena)
{
  BetaRowArena a1, a2;
  auto * relation = create_beta_node(&a1, nullptr);
  EXPECT_THROW(relation->insert_beta_row(nullptr, create_beta_row(&a2, nullptr, 1)), rete_exception);
  EXPECT_THROW(relation->insert_beta_row(nullptr, create_beta_row(nullptr, 1)), rete_exception);
}

}   // namespace
}   // namespace jets::rdf