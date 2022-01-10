#include <iostream>
#include <memory>

#include <gtest/gtest.h>

#include "jets/rdf/rdf_types.h"
#include "jets/rete/rete_types.h"

namespace jets::rete {
namespace {
// Simple test
TEST(BetaRowInitializerTest, SimpleTest) {

    auto initializer_p = create_row_initializer(3);
    EXPECT_EQ(initializer_p->get_size(), 3);

    int e = initializer_p->put(0, 10, "a");
    EXPECT_EQ(e, 0);

    e = initializer_p->put(1, 11, "b");
    EXPECT_EQ(e, 0);

    e = initializer_p->put(2, 12, "c");
    EXPECT_EQ(e, 0);

    e = initializer_p->get(0);
    EXPECT_EQ(e, 10);

    e = initializer_p->get(1);
    EXPECT_EQ(e, 11);

    e = initializer_p->get(2);
    EXPECT_EQ(e, 12);

    e = initializer_p->get(3);
    EXPECT_EQ(e, -1);

    EXPECT_EQ(initializer_p->get_label(0), std::string_view("a"));
    EXPECT_EQ(initializer_p->get_label(1), std::string_view("b"));
    EXPECT_EQ(initializer_p->get_label(2), std::string_view("c"));
    EXPECT_EQ(initializer_p->get_label(3), std::string_view(""));
}

TEST(BetaRowInitializerTest, IteratorTest) {

    auto initializer_p = create_row_initializer(3);
    int e = initializer_p->put(0, 10, "");
    EXPECT_EQ(e, 0);
    e = initializer_p->put(1, 11, "");
    EXPECT_EQ(e, 0);
    e = initializer_p->put(2, 12, "");
    EXPECT_EQ(e, 0);

    auto itor = initializer_p->begin();
    auto end = initializer_p->end();
    int expect = 10;
    for(; itor !=end; itor++) {
        EXPECT_EQ(*itor, expect);
        expect++;
    }
}

// Simple test
// The suite fixture for node_vertex
class NodeVertexTest : public ::testing::Test {
 protected:
  NodeVertexTest() : ri0(), ri1(), nv0(), nv1() {
      ri0 = create_row_initializer(3);
      ri1 = create_row_initializer(5);
      nv0 = create_node_vertex(nullptr, 0, false, 10, {}, ri0);
      nv1 = create_node_vertex(nv0.get(), 1, false, 20, {}, ri1);
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

    EXPECT_EQ(ri0->put(0, 10, ""), 0);
    EXPECT_EQ(ri0->put(1, 11, ""), 0);
    EXPECT_EQ(ri0->put(2, 13, ""), 0);

    EXPECT_EQ(ri1->put(0, 20, ""), 0);
    EXPECT_EQ(ri1->put(1, 21, ""), 0);
    EXPECT_EQ(ri1->put(2, 22, ""), 0);
    EXPECT_EQ(ri1->put(3, 23, ""), 0);
    EXPECT_EQ(ri1->put(4, 24, ""), 0);

    EXPECT_EQ(nv1->parent_node_vertex, nv0.get());
}

}   // namespace
}   // namespace jets::rete