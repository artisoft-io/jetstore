#include <iostream>
#include <memory>

#include <gtest/gtest.h>

#include "jets/rdf/rdf_types.h"

namespace jets::rdf {
namespace {
// The suite fixture for base_graph
class BaseGraphTest : public ::testing::Test {
protected:
    BaseGraphTest()
        : s1(),
          s2(),
          s3(),
          s4(),
          g0(),
          g1(),
          g2(),
          g3()
            {
                s1 = mkResource("r1");
                s2 = mkResource("r2");
                s3 = mkResource("r3");
                s4 = mkResource("r4");
                g0 = create_stl_base_graph('s');
                g1 = create_stl_base_graph('s');
                g2 = create_stl_base_graph('p');
                g3 = create_stl_base_graph('o');
                
              // Make all graphs to have the same triples as g1 (the 's' one)
              // Note that insert(s, p, o) rotates the triple according to spin
              // while contains(u, v, w) method check for triple as is (i.e. u, v, w)
              r_index r1=s1.get(), r2=s2.get(), r3=s3.get(), r4=s4.get();

              // spo natural order
              g1->insert(r1, r2, r3);
              g1->insert(r4, r2, r3);
              g1->insert(r1, r4, r3);
              g1->insert(r1, r2, r4);

              // pos order
              g2->insert(r1, r2, r3);
              g2->insert(r4, r2, r3);
              g2->insert(r1, r4, r3);
              g2->insert(r1, r2, r4);

              // osp order
              g3->insert(r1, r2, r3);
              g3->insert(r4, r2, r3);
              g3->insert(r1, r4, r3);
              g3->insert(r1, r2, r4);
            }

    Rptr s1;
    Rptr s2;
    Rptr s3;
    Rptr s4;
    BaseGraphStlPtr g0;
    BaseGraphStlPtr g1;
    BaseGraphStlPtr g2;
    BaseGraphStlPtr g3;
};

// Define the tests
TEST_F(BaseGraphTest, EmptyGraph) {
    r_index r1=s1.get(), r2=s2.get(), r3=s3.get();
    EXPECT_FALSE(g0->contains(r1, r2, r3));
    EXPECT_EQ(g0->retract(r1, r2, r3), 0);
    EXPECT_EQ(g0->erase(r1, r2, r3), 0);
    auto itor = g0->find();
    while(not itor.is_end()) {
        ADD_FAILURE() << "Expecting an empty graph, should not have returned triples";
        itor.next();
    }
}

TEST_F(BaseGraphTest, InsertUvwGraph) {
    r_index r1=s1.get(), r2=s2.get(), r3=s3.get();
    EXPECT_TRUE(g0->insert(r1, r2, r3));
    EXPECT_TRUE(g0->contains(r1, r2, r3));

    EXPECT_EQ(g0->retract(r1, r2, r3), 1);
    EXPECT_FALSE(g0->contains(r1, r2, r3));

    EXPECT_TRUE(g0->insert(r1, r2, r3));
    EXPECT_FALSE(g0->insert(r1, r2, r3));
    EXPECT_EQ(g0->retract(r1, r2, r3), 0); // returns 0 because triple is still in graph
    EXPECT_TRUE(g0->contains(r1, r2, r3));
    EXPECT_EQ(g0->retract(r1, r2, r3), 1);
    EXPECT_FALSE(g0->contains(r1, r2, r3));

    // insert again
    EXPECT_TRUE(g0->insert(r1, r2, r3));
    EXPECT_TRUE(g0->contains(r1, r2, r3));

    auto itor = g0->find();
    int count = 0;
    while(not itor.is_end()) {
        count += 1;
        EXPECT_EQ(itor.get_subject(), r1);
        EXPECT_NE(itor.get_subject(), r2);
        EXPECT_EQ(itor.get_predicate(), r2);
        EXPECT_NE(itor.get_predicate(), r3);
        EXPECT_EQ(itor.get_object(), r3);
        itor.next();
    }
    EXPECT_EQ(count, 1);
}

TEST_F(BaseGraphTest, SPOGraph) {
    r_index r1=s1.get(), r2=s2.get(), r3=s3.get(), r4=s4.get();
    // spo spin order
    EXPECT_TRUE(g1->contains_spo(r1, r2, r3));
    EXPECT_TRUE(g1->contains_spo(r4, r2, r3));
    EXPECT_TRUE(g1->contains_spo(r1, r4, r3));
    EXPECT_TRUE(g1->contains_spo(r1, r2, r4));

    // uvw
    EXPECT_TRUE(g1->contains(r1, r2, r3));
    EXPECT_TRUE(g1->contains(r4, r2, r3));
    EXPECT_TRUE(g1->contains(r1, r4, r3));
    EXPECT_TRUE(g1->contains(r1, r2, r4));

    auto itor = g1->find();
    int count = 0;
    while(not itor.is_end()) {
        count += 1;
        auto s = itor.get_subject();
        auto p = itor.get_predicate();
        auto o = itor.get_object();
        EXPECT_TRUE(s == r1 or s == r4);
        EXPECT_TRUE(p == r2 or p == r4);
        EXPECT_TRUE(o == r3 or o == r4);
        itor.next();
    }
    EXPECT_EQ(count, 4);
}

TEST_F(BaseGraphTest, POSGraph) {
    r_index r1=s1.get(), r2=s2.get(), r3=s3.get(), r4=s4.get();
    // uvw
    EXPECT_TRUE(g2->contains(r1, r2, r3));
    EXPECT_TRUE(g2->contains(r4, r2, r3));
    EXPECT_TRUE(g2->contains(r1, r4, r3));
    EXPECT_TRUE(g2->contains(r1, r2, r4));

    // spo spin order for 'p' spin
    g2->clear();    
    EXPECT_TRUE(g2->insert_spo(r1, r2, r3));
    EXPECT_TRUE(g2->insert_spo(r4, r2, r3));
    EXPECT_TRUE(g2->insert_spo(r1, r4, r3));
    EXPECT_TRUE(g2->insert_spo(r1, r2, r4));

    EXPECT_TRUE(g2->contains_spo(r1, r2, r3));
    EXPECT_TRUE(g2->contains_spo(r4, r2, r3));
    EXPECT_TRUE(g2->contains_spo(r1, r4, r3));
    EXPECT_TRUE(g2->contains_spo(r1, r2, r4));

    auto itor = g2->find();
    int count = 0;
    while(not itor.is_end()) {
        count += 1;
        auto s = itor.get_subject();
        auto p = itor.get_predicate();
        auto o = itor.get_object();
        EXPECT_TRUE(s == r1 or s == r4);
        EXPECT_TRUE(p == r2 or p == r4);
        EXPECT_TRUE(o == r3 or o == r4);
        itor.next();
    }
    EXPECT_EQ(count, 4);
}

TEST_F(BaseGraphTest, OSPGraph) {
    r_index r1=s1.get(), r2=s2.get(), r3=s3.get(), r4=s4.get();
    // uvw
    EXPECT_TRUE(g3->contains(r1, r2, r3));
    EXPECT_TRUE(g3->contains(r4, r2, r3));
    EXPECT_TRUE(g3->contains(r1, r4, r3));
    EXPECT_TRUE(g3->contains(r1, r2, r4));

    // spo spin order for 'o' spin
    g3->clear();    
    EXPECT_TRUE(g3->insert_spo(r1, r2, r3));
    EXPECT_TRUE(g3->insert_spo(r4, r2, r3));
    EXPECT_TRUE(g3->insert_spo(r1, r4, r3));
    EXPECT_TRUE(g3->insert_spo(r1, r2, r4));

    EXPECT_TRUE(g3->contains_spo(r1, r2, r3));
    EXPECT_TRUE(g3->contains_spo(r4, r2, r3));
    EXPECT_TRUE(g3->contains_spo(r1, r4, r3));
    EXPECT_TRUE(g3->contains_spo(r1, r2, r4));

    auto itor = g3->find();
    int count = 0;
    while(not itor.is_end()) {
        count += 1;
        auto s = itor.get_subject();
        auto p = itor.get_predicate();
        auto o = itor.get_object();
        EXPECT_TRUE(s == r1 or s == r4);
        EXPECT_TRUE(p == r2 or p == r4);
        EXPECT_TRUE(o == r3 or o == r4);
        itor.next();
    }
    EXPECT_EQ(count, 4);
}

}   // namespace
}   // namespace jets::rdf