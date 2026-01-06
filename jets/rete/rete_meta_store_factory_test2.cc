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

class MSFactoryLoadV2 : public ::testing::Test {
 protected:
  MSFactoryLoadV2()
      : workspace_db_name("test_data/usi_workspace_v2.db"),
        lookup_db_name("test_data/usi_lookup_v2.db") {
    google::InitGoogleLogging("MSFactoryLoadV2");
    workspacePath = std::filesystem::path(workspace_db_name);
    lookupPath = std::filesystem::path(lookup_db_name);
    std::cout << "Current path is " << fs::current_path() << '\n';
    std::cout << "Absolute path for workspace db v2 " << workspacePath << " is "
              << std::filesystem::absolute(workspacePath) << '\n';
    std::cout << "Absolute path for lookup db v2 " << lookupPath << " is "
              << std::filesystem::absolute(lookupPath) << '\n';
  }
  std::string workspace_db_name;
  std::string lookup_db_name;
  std::filesystem::path workspacePath;
  std::filesystem::path lookupPath;
};

// Define the tests
TEST_F(MSFactoryLoadV2, Test1) {
  if (not std::filesystem::exists(workspacePath)) {
    std::cout << "workspace db v2 file not found!, skipping test" << std::endl;
    GTEST_SKIP();
  }
  if (not std::filesystem::exists(lookupPath)) {
    std::cout << "lookup db v2 file not found!, skipping test" << std::endl;
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

TEST_F(MSFactoryLoadV2, Test2) {
  if (not std::filesystem::exists(workspacePath)) {
    std::cout << "workspace db v2 file not found!, skipping test" << std::endl;
    GTEST_SKIP();
  }
  if (not std::filesystem::exists(lookupPath)) {
    std::cout << "lookup db v2 file not found!, skipping test" << std::endl;
    GTEST_SKIP();
  }
  // Create the factory
  auto* factory = new ReteMetaStoreFactory();

  int res = 0;
  EXPECT_NO_THROW(res = factory->load_database(workspacePath.string(),
                                               lookupPath.string()));
  EXPECT_EQ(res, 0);

  // Load map_authorization_main.jr rule set meta triples
  factory->load_meta_triples("jet_rules/authorization/map_authorization_main.jr",
                             /*is_rule_set=*/1);
  std::cout << "Meta graph size for map_authorization_main.jr: "
            << factory->get_meta_graph()->size() << std::endl;
  EXPECT_GT(factory->get_meta_graph()->size(), 0);

  // authorization_main.jr metastore
  std::cout << "map_authorization_main alpha nodes size: "
            << factory
                   ->get_rete_meta_store(
                       "jet_rules/authorization/map_authorization_main.jr")
                   ->alpha_nodes()
                   .size()
            << std::endl;

  res = factory->reset();
  EXPECT_EQ(res, 0);
  delete factory;
  google::ShutdownGoogleLogging();
}

TEST_F(MSFactoryLoadV2, Test3) {
  if (not std::filesystem::exists(workspacePath)) {
    std::cout << "workspace db v2 file not found!, skipping test" << std::endl;
    GTEST_SKIP();
  }
  if (not std::filesystem::exists(lookupPath)) {
    std::cout << "lookup db v2 file not found!, skipping test" << std::endl;
    GTEST_SKIP();
  }
  // Create the factory
  auto* factory = new ReteMetaStoreFactory();

  int res = 0;
  EXPECT_NO_THROW(res = factory->load_database(workspacePath.string(),
                                               lookupPath.string()));
  EXPECT_EQ(res, 0);

  // Load MSK rule sequence meta triples
  factory->load_meta_triples("MSK", /*is_rule_set=*/0);
  std::cout << "Meta graph size for MSK: " << factory->get_meta_graph()->size()
            << std::endl;
  EXPECT_GT(factory->get_meta_graph()->size(), 0);

  // metastore
  std::cout << "jet_rules/main/MSK/1_MSK_Mapping_SM_Main1.jr alpha nodes size: "
            << factory->get_rete_meta_store("jet_rules/main/MSK/1_MSK_Mapping_SM_Main1.jr")->alpha_nodes().size()
            << std::endl;
  std::cout << "jet_rules/main/MSK/1_MSK_Mapping_SM_Main2.jr alpha nodes size: "
            << factory->get_rete_meta_store("jet_rules/main/MSK/1_MSK_Mapping_SM_Main2.jr")->alpha_nodes().size()
            << std::endl;
  std::cout << "jet_rules/main/MSK/1_MSK_Mapping_SM_Main3.jr alpha nodes size: "
            << factory->get_rete_meta_store("jet_rules/main/MSK/1_MSK_Mapping_SM_Main3.jr")->alpha_nodes().size()
            << std::endl;

  res = factory->reset();
  EXPECT_EQ(res, 0);
  delete factory;
  google::ShutdownGoogleLogging();
}

TEST_F(MSFactoryLoadV2, Test4) {
  if (not std::filesystem::exists(workspacePath)) {
    std::cout << "workspace db v2 file not found!, skipping test" << std::endl;
    GTEST_SKIP();
  }
  if (not std::filesystem::exists(lookupPath)) {
    std::cout << "lookup db v2 file not found!, skipping test" << std::endl;
    GTEST_SKIP();
  }
  // Create the factory
  auto* factory = new ReteMetaStoreFactory();

  int res = 0;
  EXPECT_NO_THROW(res = factory->load_database(workspacePath.string(),
                                               lookupPath.string()));
  EXPECT_EQ(res, 0);

  // Load CM rule sequence meta triples
  factory->load_meta_triples("CM", /*is_rule_set=*/0);
  std::cout << "Meta graph size for CM: " << factory->get_meta_graph()->size()
            << std::endl;
  EXPECT_GT(factory->get_meta_graph()->size(), 0);

  // metastore
  std::cout << "jet_rules/main/CM/1_CM01_Mapping_SM_Main1.jr alpha nodes size: "
            << factory->get_rete_meta_store("jet_rules/main/CM/1_CM01_Mapping_SM_Main1.jr")->alpha_nodes().size()
            << std::endl;
  std::cout << "jet_rules/main/CM/1_CM01_Mapping_SM_Main2.jr alpha nodes size: "
            << factory->get_rete_meta_store("jet_rules/main/CM/1_CM01_Mapping_SM_Main2.jr")->alpha_nodes().size()
            << std::endl;
  std::cout << "jet_rules/main/CM/1_CM01_Mapping_SM_Main3.jr alpha nodes size: "
            << factory->get_rete_meta_store("jet_rules/main/CM/1_CM01_Mapping_SM_Main3.jr")->alpha_nodes().size()
            << std::endl;
  std::cout << "jet_rules/main/CM/2_CM02_Matching_SM_Main1.jr alpha nodes size: "
            << factory->get_rete_meta_store("jet_rules/main/CM/2_CM02_Matching_SM_Main1.jr")->alpha_nodes().size()
            << std::endl;

  res = factory->reset();
  EXPECT_EQ(res, 0);
  delete factory;
  google::ShutdownGoogleLogging();
}

TEST_F(MSFactoryLoadV2, Test5) {
  if (not std::filesystem::exists(workspacePath)) {
    std::cout << "workspace db v2 file not found!, skipping test" << std::endl;
    GTEST_SKIP();
  }
  if (not std::filesystem::exists(lookupPath)) {
    std::cout << "lookup db v2 file not found!, skipping test" << std::endl;
    GTEST_SKIP();
  }
  // Create the factory
  auto* factory = new ReteMetaStoreFactory();

  int res = 0;
  EXPECT_NO_THROW(res = factory->load_database(workspacePath.string(),
                                               lookupPath.string()));
  EXPECT_EQ(res, 0);

  // Load MSK rule sequence meta triples
  factory->load_meta_triples("IM", /*is_rule_set=*/0);
  std::cout << "Meta graph size for IM: " << factory->get_meta_graph()->size()
            << std::endl;
  EXPECT_GT(factory->get_meta_graph()->size(), 0);

  // metastore
  std::cout << "jet_rules/main/IM/1_IM01_Mapping_SM_Main1.jr alpha nodes size: "
            << factory->get_rete_meta_store("jet_rules/main/IM/1_IM01_Mapping_SM_Main1.jr")->alpha_nodes().size()
            << std::endl;
  std::cout << "jet_rules/main/IM/2_IM02_Matching_SM_Main1.jr alpha nodes size: "
            << factory->get_rete_meta_store("jet_rules/main/IM/2_IM02_Matching_SM_Main1.jr")->alpha_nodes().size()
            << std::endl;
  std::cout << "jet_rules/main/IM/3_IM03_Export_Main1.jr alpha nodes size: "
            << factory->get_rete_meta_store("jet_rules/main/IM/3_IM03_Export_Main1.jr")->alpha_nodes().size()
            << std::endl;

  res = factory->reset();
  EXPECT_EQ(res, 0);
  delete factory;
  google::ShutdownGoogleLogging();
}

}  // namespace
}  // namespace jets::rete