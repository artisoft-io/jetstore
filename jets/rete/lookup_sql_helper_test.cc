#include <cstddef>
#include <iostream>
#include <string>
#include <memory>

#include <gtest/gtest.h>

#include "../rdf/rdf_types.h"
#include "../rete/rete_types.h"
#include "../rete/lookup_sql_helper.h"

namespace jets::rete {
namespace {
// Simple test

TEST(LookupSqlHelperTest, SimpleTest1) {

  auto meta_graph = rdf::create_rdf_graph();
  meta_graph->rmgr()->initialize();

  auto helper = create_lookup_sql_helper("test_data/lookup_helper_test_workspace.db", "test_data/lookup_helper_test_workspace.db");
  EXPECT_EQ(helper->initialize(meta_graph.get()), 0);

  std::cout<<"Initialize Completed!"<<std::endl;

  EXPECT_EQ(helper->terminate(), 0);
}

}   // namespace
}   // namespace jets::rete