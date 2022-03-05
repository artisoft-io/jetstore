#include <cstddef>
#include <iostream>
#include <string>
#include <memory>

#include <gtest/gtest.h>

#include "../rdf/rdf_types.h"
#include "../rete/rete_types.h"

namespace jets::rete {
namespace {
// Simple test
TEST(BetaRowStatusTest, StatusTest) {
    int row_size = 3;
    auto ri0 = create_row_initializer(row_size);
    ri0->put(0, 0, "a");
    ri0->put(1, 0, "b");
    ri0->put(2, 0, "c");
    auto node_ptr = create_node_vertex(nullptr, 0, 0, false, 10, {}, "", ri0);
    BetaRowPtr row = create_beta_row(node_ptr.get(), row_size);
    row->put(0, nullptr);
    row->put(1, nullptr);
    row->put(2, nullptr);
    row->set_status(BetaRowStatus::kInserted);
    EXPECT_EQ(row->is_deleted(), false);
    EXPECT_EQ(row->is_inserted(), true);
    EXPECT_EQ(row->is_processed(), false);
    row->set_status(BetaRowStatus::kProcessed);
    EXPECT_EQ(row->is_deleted(), false);
    EXPECT_EQ(row->is_inserted(), false);
    EXPECT_EQ(row->is_processed(), true);
    row->set_status(BetaRowStatus::kDeleted);
    EXPECT_EQ(row->is_deleted(), true);
    EXPECT_EQ(row->is_inserted(), false);
    EXPECT_EQ(row->is_processed(), false);
}

// The suite fixture for node_vertex
class BetaRowTest : public ::testing::Test {
 protected:
  BetaRowTest() : br0(), br1(), ri0(), nv0() {
      int row_size = 3;
      ri0 = create_row_initializer(row_size);
      nv0 = create_node_vertex(nullptr, 0, 0, false, 10, {}, "", ri0);
      br0 = create_beta_row(nv0.get(), row_size);
      br1 = create_beta_row(nv0.get(), row_size);
  }

  BetaRowPtr br0;
  BetaRowPtr br1;
  BetaRowInitializerPtr ri0;
  NodeVertexPtr nv0;
};

// Define the tests
TEST_F(BetaRowTest, RowInitializerTest) {
    EXPECT_EQ(ri0->get_size(), 3);

    EXPECT_EQ(ri0->put(0, 0 | brc_parent_node, "a"), 0);
    EXPECT_EQ(ri0->put(1, 0 | brc_triple, "b"), 0);
    EXPECT_EQ(ri0->put(2, 1 | brc_parent_node, "c"), 0);

    EXPECT_EQ(ri0->get(0) & brc_low_mask, 0);
    EXPECT_EQ(ri0->get(1) & brc_low_mask, 0);
    EXPECT_EQ(ri0->get(2) & brc_low_mask, 1);
    EXPECT_EQ(ri0->get(3), -1);

    EXPECT_EQ(ri0->get_label(0), std::string_view("a"));
    EXPECT_EQ(ri0->get_label(1), std::string_view("b"));
    EXPECT_EQ(ri0->get_label(2), std::string_view("c"));
    EXPECT_EQ(ri0->get_label(3), std::string_view(""));
}

TEST_F(BetaRowTest, BetaRowTest) {
    EXPECT_EQ(br0->get_size(), 3);

    // rdf resource manager
    rdf::RManager rmanager;

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

TEST_F(BetaRowTest, BetaRowInitializeTest) {
    // setup the row initializer
    EXPECT_EQ(ri0->get_size(), 3);

    EXPECT_EQ(ri0->put(0, 0 | brc_parent_node, "a"), 0);
    EXPECT_EQ(ri0->put(1, 2 | brc_triple, "a"), 0);
    EXPECT_EQ(ri0->put(2, 1 | brc_parent_node, "a"), 0);

    // rdf resource manager
    rdf::RManager rmanager;
    auto r0 = rmanager.create_resource("r0");
    auto r1 = rmanager.create_resource("r1");
    auto r2 = rmanager.create_resource("r2");
    auto x0 = rmanager.create_resource("x0");
    auto x1 = rmanager.create_resource("x1");

    // setup the parent row
    EXPECT_EQ(br0->put(0, r0), 0);
    EXPECT_EQ(br0->put(1, r2), 0);
    EXPECT_EQ(br0->put(2, x1), 0);

    // setup the triple
    rdf::Triple t3(x0, x1, r1);

    // initialize the beta_row
    br1->initialize(ri0.get(), br0.get(), &t3);
    EXPECT_EQ(br1->get(0), r0);
    EXPECT_EQ(br1->get(1), r1);
    EXPECT_EQ(br1->get(2), r2);
    
}

}   // namespace
}   // namespace jets::rdf