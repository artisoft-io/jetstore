#include <cstddef>
#include <iostream>
#include <memory>

#include <gtest/gtest.h>

#include "alpha_functors.h"
#include "beta_row_initializer.h"
#include "expr_operators.h"
#include "jets/rdf/rdf_types.h"
#include "jets/rete/rete_types.h"
#include "node_vertex.h"

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
  ReteSessionTest() 
    : rete_session(), rete_meta_store(), rdf_session() 
  {
    NodeVertexVector   node_vertexes;
    this->rdf_session = rdf::create_rdf_session(rdf::create_rdf_graph());
    this->rete_session = create_rete_session(rdf_session.get());
    auto * rmgr = rdf_session->rmgr();
    auto has_node = rmgr->create_resource("has_node");
    auto plus1_node = rmgr->create_resource("plus1_node");
    auto plus2_node = rmgr->create_resource("plus2_node");

    //rule1> (head node0).(?s has_node ?n1) -> (?s1 plus1_node expr(?n1 + 1))
    //rule2> (head node0).(?s has_node ?n1).(?s has_node ?n2) -> (?s1 plus2_node expr(?n1 + ?n2))
    //        node 0        node 1            node 2
    // ----------------------------------------------------------------------------------
    // No need for AntecedentQuerySpec since the only vertex reads from the graph
    std::cout<<"**ReteSessionTest initializing"<<std::endl;

    // BetaRowInitializer --BetaRelation 1 row: [?s, ?n1]
    auto ri1 = create_row_initializer(2);
    ri1->put(0, 0 | brc_triple);
    ri1->put(1, 2 | brc_triple);
    // BetaRowInitializer --BetaRelation 2 row: [?s, ?n1, ?n2]
    auto ri2 = create_row_initializer(3);
    ri2->put(0, 0 | brc_parent_node);
    ri2->put(1, 1 | brc_parent_node);
    ri2->put(2, 2 | brc_triple);

    // AntecedentQuerySpec: there is NO AntecedentQuerySpec associated with node 0 
    // and all node having node 0 as parent.
    // In other words, the first BetaRelation of a rule does not have an parent for
    // the antecedent to query
    auto query2 = create_antecedent_query_spec(0, AntecedentQueryType::kQTu, 's', 0, -1, -1);

    // NodeVertex
    node_vertexes.push_back(create_node_vertex(nullptr, 0, false, -1, 10, {}, {}));
    node_vertexes.push_back(create_node_vertex(node_vertexes[0].get(), 1, false, -1, 10, ri1, {}));
    node_vertexes.push_back(create_node_vertex(node_vertexes[1].get(), 2, false, -1, 10, ri2, query2));

    // AlphaNodes
    ReteMetaStore::AlphaNodeVector alpha_nodes;

    // Add Antecedent term on vertex 0 -- head vertices
    // this AlphaNode is not used, it's a place holder since we need one AlphaNode for each NodeVertex
    alpha_nodes.push_back(create_alpha_node<F_var, F_var, F_var>(node_vertexes[0].get(), true,
      F_var("*"), F_var("*"), F_var("*") ));
    // Add Antecedent term on vertex 1
    alpha_nodes.push_back(create_alpha_node<F_var, F_cst, F_var>(node_vertexes[1].get(), true,
      F_var("?s"), F_cst(has_node), F_var("?n1") ));
    // Add Antecedent term on vertex 2
    alpha_nodes.push_back(create_alpha_node<F_binded, F_cst, F_var>(node_vertexes[2].get(), true,
      F_binded(0), F_cst(has_node), F_var("?n2") ));

    {
      // Add Consequent term on vertex 1: (?s1 plus1_node expr(?n1 + 1))
      auto lhs = create_expr_binded_var(1);
      auto rhs = create_expr_cst(rdf::RdfAstType(rdf::LInt32(1)));
      auto expr = create_expr_binary_operator<AddVisitor>(lhs, rhs);
      alpha_nodes.push_back(create_alpha_node<F_binded, F_cst, F_expr>(node_vertexes[1].get(), false,
        F_binded(0), F_cst(plus1_node), F_expr(this->rete_session.get(), expr) ));
    }
    {
      // Add Consequent term on vertex 2: (?s1 plus2_node expr(?n1 + ?n2))
      auto lhs = create_expr_binded_var(1);
      auto rhs = create_expr_binded_var(2);
      auto expr = create_expr_binary_operator<AddVisitor>(lhs, rhs);
      alpha_nodes.push_back(create_alpha_node<F_binded, F_cst, F_expr>(node_vertexes[1].get(), false,
        F_binded(0), F_cst(plus2_node), F_expr(this->rete_session.get(), expr) ));
    }

    std::cout<<"**ReteSessionTest AlphaNodes created"<<std::endl;

    // create & initalize the meta store -- TODO have an expression builder with meta store
    rete_meta_store = create_rete_meta_store(alpha_nodes, {}, node_vertexes);
    rete_meta_store->initialize();

    std::cout<<"**ReteSessionTest MetaStore initialized"<<std::endl;

    // Initialize the rete_session now that the rule base is ready
    this->rete_session->initialize(rete_meta_store);

    std::cout<<"**ReteSessionTest Initialize done!"<<std::endl;
  }

  ReteSessionPtr  rete_session;
  ReteMetaStorePtr rete_meta_store;
  rdf::RDFSessionPtr   rdf_session;
};

// Define the tests
TEST_F(ReteSessionTest, ExecuteRule1Test) 
{
  // rdf resource manager
  rdf::RManager * rmanager = this->rdf_session->rmgr();
  auto s0 = rmanager->create_resource("s0");
  auto p0 = rmanager->create_resource("has_node");
  auto o0 = rmanager->create_literal<int>(1);
  this->rdf_session->insert(s0, p0, o0);
  this->rete_session->execute_rules();

  auto p1 = rmanager->create_resource("plus1_node");
  auto o1 = rmanager->create_literal<int>(2);
  EXPECT_TRUE(this->rdf_session->contains(s0, p1, o1));
}

TEST_F(ReteSessionTest, ExecuteRule2Test) 
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
  auto o0r = rmanager->create_literal<int>(2);
  EXPECT_TRUE(this->rdf_session->contains(s0, p0r, o0r));

  auto p1r = rmanager->create_resource("plus2_node");
  auto o1r = rmanager->create_literal<int>(3);
  EXPECT_TRUE(this->rdf_session->contains(s0, p1r, o1r));
}

}   // namespace
}   // namespace jets::rdf