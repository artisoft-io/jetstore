#include <iostream>
#include <memory>

#include <gtest/gtest.h>

#include "jets/rdf/rdf_types.h"

namespace jets::rdf {
namespace {
// Simple test
TEST(RdfAstTest, BaseAstComposition) {
    
    // subjects
    auto bn1_s = mkBlankNode(1);
    auto bn2_s = mkBlankNode(2);
    // predicates
    std::string str1("mange");
    auto mange_s      = mkResource(str1);
    auto nbr_items_s  = mkResource("nbr_items");
    // objects
    auto banane_s = mkResource("banane");
    auto pomme_s  = mkResource("pomme");
    auto fraise_s = mkResource("fraise");
    auto five_s   = mkLiteral(std::int32_t{5});
    auto eps_s    = mkLiteral(double{0.01});

    // Convert to r_index by taking raw pointers
    r_index bn1=bn1_s.get(), bn2=bn2_s.get();
    r_index mange=mange_s.get(), nbr_items=nbr_items_s.get();
    r_index banane=banane_s.get(), pomme=pomme_s.get(), fraise=fraise_s.get(), five=five_s.get(), eps=eps_s.get();

    // std::cout << "So let's start with this **" << std::endl;
    // std::cout << "  (" << bn1 << ", " << mange << ", " << pomme << ")" << std::endl;
    // std::cout << "  (" << bn1 << ", " << mange << ", " << banane << ")" << std::endl;
    // std::cout << "  (" << bn1 << ", " << nbr_items << ", " << five << ")" << std::endl;
    // std::cout << "  (" << bn2 << ", " << mange << ", " << banane << ")" << std::endl;
    // std::cout << "  (" << bn2 << ", " << mange << ", " << fraise << ")" << std::endl;
    // std::cout << "  (" << bn2 << ", " << nbr_items << ", " << five << ")" << std::endl;
    // std::cout << "  (" << bn2 << ", " << nbr_items << ", " << eps << ")" << std::endl;

    auto graph_p = create_stl_base_graph('s');
    graph_p->insert(bn1, mange, pomme);
    graph_p->insert(bn1, mange, banane);
    graph_p->insert(bn1, nbr_items, five);
    graph_p->insert(bn2, mange, banane);
    graph_p->insert(bn2, mange, fraise);
    graph_p->insert(bn2, nbr_items, five);
    graph_p->insert(bn2, nbr_items, eps);

    // std::cout<<"So let's see if we get back expected triple count per individual!"<<std::endl;
    int count = 0;
    auto itor = graph_p->find(bn1);
    // std::cout<<"For, "<<bn1<<" we have:"<<std::endl;
    while(not itor.is_end()) {
        // std::cout << "   ("<<itor.get_subject() << ", "<<itor.get_predicate() << ", " << itor.get_object()<<")"<<std::endl;
        count += 1;
        itor.next();
    }
    EXPECT_EQ(count, 3);

    count = 0;
    itor = graph_p->find(bn2);
    // std::cout<<"For, "<<bn2<<" we have:"<<std::endl;
    while(not itor.is_end()) {
        // std::cout << "   ("<<itor.get_subject() << ", "<<itor.get_predicate() << ", " << itor.get_object()<<")"<<std::endl;
        count += 1;
        itor.next();
    }
    EXPECT_EQ(count, 4);
}
}   // namespace
}   // namespace jets::rdf