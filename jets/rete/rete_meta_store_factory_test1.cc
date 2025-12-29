#include <glog/logging.h>
#include <gtest/gtest.h>

#include <filesystem>
#include <iostream>
#include <memory>

#include "../rdf/rdf_types.h"
#include "../rete/rete_meta_store_factory.h"
#include "../rete/rete_types.h"

namespace fs = std::filesystem;
namespace jets::rete {
namespace {

class MSFactoryLoadV1 : public ::testing::Test {
 protected:
  MSFactoryLoadV1()
      : workspace_db_name("test_data/usi_workspace_v1.db"),
        lookup_db_name("test_data/usi_lookup_v1.db") {
    google::InitGoogleLogging("MSFactoryLoadV1");
  }
  std::string workspace_db_name;
  std::string lookup_db_name;
};

// Define the tests
TEST_F(MSFactoryLoadV1, Test1) {
  /* Open database */
  std::filesystem::path workspacePath(workspace_db_name);
  std::filesystem::path lookupPath(lookup_db_name);
  std::cout << "Current path is " << fs::current_path() << '\n';
  std::cout << "Absolute path for workspace db v1 " << workspacePath << " is "
            << std::filesystem::absolute(workspacePath) << '\n';
  std::cout << "Path exist? " << std::filesystem::exists(workspacePath) << '\n';
  std::cout << "Absolute path for lookup db v1 " << lookupPath << " is "
            << std::filesystem::absolute(lookupPath) << '\n';
  std::cout << "Path exist? " << std::filesystem::exists(lookupPath) << '\n';

  if (not std::filesystem::exists(workspacePath)) {
    std::cout << "workspace db v1 file not found!, skipping test" << std::endl;
    GTEST_SKIP();
  }
  if (not std::filesystem::exists(lookupPath)) {
    std::cout << "lookup db v1 file not found!, skipping test" << std::endl;
    GTEST_SKIP();
  }
  // Create the factory
  auto* factory = new ReteMetaStoreFactory();

  int res = 0;
  EXPECT_NO_THROW(res = factory->load_database(workspacePath.string(),
                                               lookupPath.string()));
  EXPECT_EQ(res, 0);

  // Load map_eligibility_main.jr rule set meta triples
  factory->load_meta_triples("jet_rules/eligibility/map_eligibility_main.jr",
                              /*is_rule_set=*/1);
  std::cout << "Meta graph size for map_eligibility_main.jr: "
            << factory->get_meta_graph()->size() << std::endl;
  EXPECT_GT(factory->get_meta_graph()->size(), 0);

  // eligibility_main.jr metastore
  std::cout << "map_eligibility_main alpha nodes size: "
            << factory
                   ->get_rete_meta_store(
                       "jet_rules/eligibility/map_eligibility_main.jr")
                   ->alpha_nodes()
                   .size()
            << std::endl;

  res = factory->reset();
  EXPECT_EQ(res, 0);
  delete factory;
  google::ShutdownGoogleLogging();
}

TEST_F(MSFactoryLoadV1, Test2) {
  /* Open database */
  std::filesystem::path workspacePath(workspace_db_name);
  std::filesystem::path lookupPath(lookup_db_name);
  std::cout << "Current path is " << fs::current_path() << '\n';
  std::cout << "Absolute path for workspace db v1 " << workspacePath << " is "
            << std::filesystem::absolute(workspacePath) << '\n';
  std::cout << "Path exist? " << std::filesystem::exists(workspacePath) << '\n';
  std::cout << "Absolute path for lookup db v1 " << lookupPath << " is "
            << std::filesystem::absolute(lookupPath) << '\n';
  std::cout << "Path exist? " << std::filesystem::exists(lookupPath) << '\n';

  if (not std::filesystem::exists(workspacePath)) {
    std::cout << "workspace db v1 file not found!, skipping test" << std::endl;
    GTEST_SKIP();
  }
  if (not std::filesystem::exists(lookupPath)) {
    std::cout << "lookup db v1 file not found!, skipping test" << std::endl;
    GTEST_SKIP();
  }
  // Create the factory
  auto* factory = new ReteMetaStoreFactory();

  int res = 0;
  EXPECT_NO_THROW(res = factory->load_database(workspacePath.string(),
                                               lookupPath.string()));
  EXPECT_EQ(res, 0);

  // Load map_eligibility_main.jr rule set meta triples
  factory->load_meta_triples("jet_rules/eligibility/map_eligibility_main.jr",
                              /*is_rule_set=*/1);
  std::cout << "Meta graph size for map_eligibility_main.jr: "
            << factory->get_meta_graph()->size() << std::endl;
  EXPECT_GT(factory->get_meta_graph()->size(), 0);

  // eligibility_main.jr metastore
  std::cout << "map_eligibility_main alpha nodes size: "
            << factory
                   ->get_rete_meta_store(
                       "jet_rules/eligibility/map_eligibility_main.jr")
                   ->alpha_nodes()
                   .size()
            << std::endl;

  res = factory->reset();
  EXPECT_EQ(res, 0);
  delete factory;
  google::ShutdownGoogleLogging();
}

}  // namespace
}  // namespace jets::rete