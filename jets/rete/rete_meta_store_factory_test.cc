#include <iostream>
#include <memory>

#include <gtest/gtest.h>

#include "jets/rdf/rdf_types.h"
#include "jets/rete/rete_types.h"

namespace jets::rete {
namespace {
// Simple test
TEST(ReteMetaStoreFactoryTest, SQLiteTest) {

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

}   // namespace
}   // namespace jets::rete