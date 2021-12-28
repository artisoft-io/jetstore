#include <iostream>
#include <memory>

#include <gtest/gtest.h>

#include "jets/rdf/rdf_types.h"

#include "jets/rete/node_vertex.h"
#include "jets/rete/beta_relation.h"

namespace jets::rete {
namespace {
// Simple test

struct AlphaConnector {

};

// The suite fixture for node_vertex
class NodeVertexTest : public ::testing::Test {
 protected:
  NodeVertexTest() : ri0(), ri1(), nv0(), nv1() {
      ri0 = create_row_initializer(3);
      ri1 = create_row_initializer(5);
      nv0 = create_node_vertex(nullptr, 0, false, 0, 10, ri0, {});
      nv1 = create_node_vertex(nv0.get(), 0, false, 0, 20, ri1, {});
  }

  BetaRowInitializerPtr ri0;
  BetaRowInitializerPtr ri1;
  NodeVertexPtr nv0;
  NodeVertexPtr nv1;
};

// Define the tests
TEST_F(NodeVertexTest, FirstTest) {
    EXPECT_EQ(ri0->get_size(), 3);
    EXPECT_EQ(ri1->get_size(), 5);

    EXPECT_EQ(ri0->put(0, 10), 0);
    EXPECT_EQ(ri0->put(1, 11), 0);
    EXPECT_EQ(ri0->put(2, 13), 0);

    EXPECT_EQ(ri1->put(0, 20), 0);
    EXPECT_EQ(ri1->put(1, 21), 0);
    EXPECT_EQ(ri1->put(2, 22), 0);
    EXPECT_EQ(ri1->put(3, 23), 0);
    EXPECT_EQ(ri1->put(4, 24), 0);

    EXPECT_EQ(nv1->parent_node_vertex, nv0.get());
}

}   // namespace
}   // namespace jets::rdf