#include <cstddef>
#include <iostream>
#include <string>
#include <memory>

#include <gtest/gtest.h>

#include "jets/rdf/rdf_types.h"
#include "jets/rete/rete_types.h"

namespace jets::rete {
namespace {
// Simple test

TEST(BasicNodeVertexTest, SimpleTest1) {
    auto node1 = jets::rete::create_node_vertex(nullptr, 0, 1, true, 10, {}, {});
    b_index b1 = node1.get();
    EXPECT_EQ(b1->parent_node_vertex, nullptr);
    EXPECT_EQ(b1->vertex, 1);
    EXPECT_EQ(b1->is_negation, true);
    EXPECT_EQ(b1->has_expr(), false);
    EXPECT_EQ(b1->salience, 10);
}

TEST(BasicNodeVertexTest, SimpleTest2) {
    auto node1 = jets::rete::create_node_vertex(nullptr, 0, 1, false, 10, {}, {});
    auto node2 = jets::rete::create_node_vertex(node1.get(), 0, 2, false, 10, {}, {});
    b_index b2 = node2.get();
    EXPECT_EQ(b2->parent_node_vertex, node1.get());
    EXPECT_EQ(b2->vertex, 2);
    EXPECT_EQ(b2->is_negation, false);
    EXPECT_EQ(b2->has_expr(), false);
    EXPECT_EQ(b2->salience, 10);

    EXPECT_FALSE(b2->has_consequent_terms());
    node2->consequent_alpha_vertexes.insert(0);
    EXPECT_TRUE(b2->has_consequent_terms());
}

}   // namespace
}   // namespace jets::rete