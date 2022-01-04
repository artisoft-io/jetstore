#include <cstddef>
#include <iostream>
#include <memory>

#include <gtest/gtest.h>

#include "expr_operators.h"
#include "jets/rdf/rdf_types.h"
#include "jets/rete/rete_types.h"

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

    //rule> (head node0).(?s has_node ?n1) -> (?s1 plus1_node expr(?n1 + 1))
    // ----------------------------------------------------------------------------------
    // No need for AntecedentQuerySpec since the only vertex reads from the graph
    std::cout<<"**ReteSessionTest initializing"<<std::endl;

    // BetaRowInitializer -- row: [?s, ?n1]
    auto ri0 = create_row_initializer(2);
    ri0->put(0, 0 | brc_triple);
    ri0->put(1, 2 | brc_triple);

    // AntecedentQuerySpec: there is NO AntecedentQuerySpec associated with node 0 
    // and all node having node 0 as parent.
    // In other words, the first BetaRelation of a rule does not have an parent for
    // the antecedent to query

    // NodeVertex
    node_vertexes.push_back(create_node_vertex(nullptr, 0, false, 0, 10, ri0, {}));
    node_vertexes.push_back(create_node_vertex(node_vertexes[0].get(), 1, false, 0, 10, ri0, {}));

    // AlphaNodes
    ReteMetaStore::AlphaNodeVector alpha_nodes;

    // Add Antecedent term on vertex 1
    alpha_nodes.push_back(create_alpha_node<F_var, F_cst, F_var>(node_vertexes[1].get(), true,
      F_var("?s"), F_cst(has_node), F_var("?n1") ));

    // Add Consequent term on vertex 1: (?s1 plus1_node expr(?n1 + 1))
    auto lhs = create_expr_binded_var(1);
    auto rhs = create_expr_cst(rdf::RdfAstType(rdf::LInt32(1)));
    auto expr = create_expr_binary_operator<AddVisitor>(lhs, rhs);
    alpha_nodes.push_back(create_alpha_node<F_binded, F_cst, F_expr>(node_vertexes[1].get(), false,
      F_binded(0), F_cst(plus1_node), F_expr(this->rete_session.get(), expr) ));

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
TEST_F(ReteSessionTest, ReteSessionExecuteRulesTest) 
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

}   // namespace
}   // namespace jets::rdf