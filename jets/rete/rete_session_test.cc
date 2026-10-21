#include <cstddef>
#include <iostream>
#include <memory>

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
  EXPECT_EQ(arena.allocate(33), p1);
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

TEST(BetaRowArenaTest, RelationRefusesARowFromAnotherArena)
{
  BetaRowArena a1, a2;
  auto * relation = create_beta_node(&a1, nullptr);
  EXPECT_THROW(relation->insert_beta_row(nullptr, create_beta_row(&a2, nullptr, 1)), rete_exception);
  EXPECT_THROW(relation->insert_beta_row(nullptr, create_beta_row(nullptr, 1)), rete_exception);
}

}   // namespace
}   // namespace jets::rdf