#include <cstddef>
#include <iostream>
#include <string>
#include <memory>

#include <gtest/gtest.h>

#include "beta_row_initializer.h"
#include "jets/rdf/rdf_types.h"

#include "jets/rete/node_vertex.h"
#include "jets/rete/beta_row.h"

namespace jets::rete {
namespace {
// Simple test

TEST(StandingBetaRowTest, SimpleTest) {
    // auto node_vertex_p = create_node_vertex(nullptr, 0, false, false, false);
    // b_index node_vertex = node_vertex_p.get();
    // auto bn = create_beta_node<AlphaConnector>(node_vertex);
    // EXPECT_EQ(bn->get_node_vertex()->vertex, 0);
}

// The suite fixture for node_vertex
class BetaRowTest : public ::testing::Test {
 protected:
  BetaRowTest() : br0(), ri0(), nv0() {
      int row_size = 3;
      ri0 = create_row_initializer(row_size);
      nv0 = create_node_vertex(nullptr, 0, false, false, false, 10, ri0);
      br0 = create_beta_row(nv0.get(), row_size);
  }

  BetaRowPtr br0;
  BetaRowInitializerPtr ri0;
  NodeVertexPtr nv0;
};

// Define the tests
TEST_F(BetaRowTest, RowInitializerTest) {
    EXPECT_EQ(ri0->get_size(), 3);

    EXPECT_EQ(ri0->put(0, 0 | brc_parent_node), 0);
    EXPECT_EQ(ri0->put(1, 0 | brc_triple), 0);
    EXPECT_EQ(ri0->put(2, 1 | brc_parent_node), 0);

    EXPECT_EQ(ri0->get(0) & brc_low_mask, 0);
    EXPECT_EQ(ri0->get(1) & brc_low_mask, 0);
    EXPECT_EQ(ri0->get(2) & brc_low_mask, 1);
}

TEST_F(BetaRowTest, BetaRowTest) {
    EXPECT_EQ(br0->get_size(), 3);

    // rdf resource manager
    rdf::RManager<rdf::LD2RIndexMap> rmanager;

    // subjects
    std::string s0("r0"), s1("r1"), s2("r2");
    auto r0 = rmanager.create_resource(s0);
    auto r1 = rmanager.create_resource(s1);
    auto r2 = rmanager.create_resource(s2);

    EXPECT_EQ(br0->put(0, r0), 0);
    EXPECT_EQ(br0->put(1, r1), 0);
    EXPECT_EQ(br0->put(2, r2), 0);

    EXPECT_EQ(br0->get(0), r0);
    EXPECT_EQ(br0->get(1), r1);
    EXPECT_EQ(br0->get(2), r2);
}

}   // namespace
}   // namespace jets::rdf