#include <iostream>
#include <memory>

#include <gtest/gtest.h>

#include "jets/rdf/rdf_types.h"

#include "jets/rete/beta_row_initializer.h"

namespace jets::rete {
namespace {
// Simple test

struct AlphaConnector {

};

TEST(BetaRowInitializerTest, SimpleTest) {

    auto initializer_p = create_row_initializer(3);
    EXPECT_EQ(initializer_p->get_size(), 3);

    int e = initializer_p->put(0, 10);
    EXPECT_EQ(e, 0);

    e = initializer_p->put(1, 11);
    EXPECT_EQ(e, 0);

    e = initializer_p->put(2, 12);
    EXPECT_EQ(e, 0);

    e = initializer_p->get(0);
    EXPECT_EQ(e, 10);

    e = initializer_p->get(1);
    EXPECT_EQ(e, 11);

    e = initializer_p->get(2);
    EXPECT_EQ(e, 12);

    e = initializer_p->get(3);
    EXPECT_EQ(e, -1);
}

TEST(BetaRowInitializerTest, IteratorTest) {

    auto initializer_p = create_row_initializer(3);
    int e = initializer_p->put(0, 10);
    EXPECT_EQ(e, 0);
    e = initializer_p->put(1, 11);
    EXPECT_EQ(e, 0);
    e = initializer_p->put(2, 12);
    EXPECT_EQ(e, 0);

    auto itor = initializer_p->begin();
    auto end = initializer_p->end();
    int expect = 10;
    for(; itor !=end; itor++) {
        EXPECT_EQ(*itor, expect);
        expect++;
    }
}

}   // namespace
}   // namespace jets::rete