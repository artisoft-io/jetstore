#include <cstddef>
#include <iostream>
#include <string>
#include <memory>

#include <gtest/gtest.h>

#include "../rdf/rdf_types.h"
#include "../rete/rete_types.h"
#include "../rete/rete_meta_store_factory.h"
#include "../rete/lookup_sql_helper.h"

namespace jets::rete {
namespace {
// Simple test

TEST(LookupSqlHelperTest, LookupTest1) {


  ReteMetaStoreFactory factory;
  int res = factory.load_database("test_data/lookup_helper_test_workspace.db", "test_data/lookup_helper_test_data.db");
  EXPECT_EQ(res, 0);

  std::cout<<"ReteMetaStoreFactory Loaded!"<<std::endl;

  auto meta_graph = factory.get_meta_graph();
  meta_graph->rmgr()->initialize();

  // Get the Rete Meta Store
  auto meta_store = factory.get_rete_meta_store("lookup_helper_test_workspace.jr");  
  EXPECT_TRUE(meta_store);

  auto helper = meta_store->get_lookup_sql_helper();

  std::cout<<"Initialize Completed!"<<std::endl;


  // Create the rdf_session and the rete_session and initialize them
  // Initialize the rete_session now that the rule base is ready
  auto rdf_session = rdf::create_rdf_session(factory.get_meta_graph());
  auto rete_session = create_rete_session(meta_store, rdf_session);
  rete_session->initialize();

  std::cout<<"Rete Session Initialize Completed!"<<std::endl;

  rdf::RdfAstType out;

  // Lookup
  EXPECT_EQ(helper->lookup(rete_session.get(), "acme:ProcedureLookup", "100", &out), 0);

  std::cout<<"Lookup GOT: "<<rdf::get_name(&out)<<std::endl;

  // Verifying we pull stuff correctly from lookup table
  rdf::r_index s = rdf_session->rmgr()->create_resource("jets:acme:ProcedureLookup:100");
  auto itor = rdf_session->find(s, rdf::make_any(), rdf::make_any());
  while(not itor.is_end()) {
    auto predicate = rdf::get_name(itor.get_predicate());
    if(predicate == "PROC_RID") {
      EXPECT_EQ(std::string(rdf::get_type_name(itor.get_object())), std::string("long"));
      EXPECT_EQ(boost::get<rdf::LInt64>(*itor.get_object()).data, 12345678901L);
    } else if(predicate == "FROM_DATE") {
      EXPECT_EQ(std::string(rdf::get_type_name(itor.get_object())), std::string("date"));
    } else if(predicate == "EXCL") {
      EXPECT_EQ(std::string(rdf::get_type_name(itor.get_object())), std::string("int"));
    } else if(predicate == "EVENT_DURATION") {
      EXPECT_EQ(std::string(rdf::get_type_name(itor.get_object())), std::string("int"));
    } else {
      FAIL();
    }
    itor.next();
  }
  EXPECT_EQ(helper->terminate(), 0);
}

TEST(LookupSqlHelperTest, MultiLookupTest1) {


  ReteMetaStoreFactory factory;
  int res = factory.load_database("test_data/lookup_helper_test_workspace.db", "test_data/lookup_helper_test_data.db");
  EXPECT_EQ(res, 0);

  std::cout<<"ReteMetaStoreFactory Loaded!"<<std::endl;

  auto meta_graph = factory.get_meta_graph();
  meta_graph->rmgr()->initialize();

  // Get the Rete Meta Store
  auto meta_store = factory.get_rete_meta_store("lookup_helper_test_workspace.jr");  
  EXPECT_TRUE(meta_store);

  auto helper = meta_store->get_lookup_sql_helper();

  std::cout<<"Initialize Completed!"<<std::endl;

  // Create the rdf_session and the rete_session and initialize them
  // Initialize the rete_session now that the rule base is ready
  auto rdf_session = rdf::create_rdf_session(factory.get_meta_graph());
  auto rete_session = create_rete_session(meta_store, rdf_session);
  rete_session->initialize();

  std::cout<<"Rete Session Initialize Completed!"<<std::endl;

  rdf::RdfAstType out;

  // Multi Lookup
  EXPECT_EQ(helper->multi_lookup(rete_session.get(), "acme:ProcedureLookup", "100", &out), 0);

  std::cout<<"MULTI Lookup GOT: "<<rdf::get_name(&out)<<std::endl;

  // Verifying we pull stuff correctly from lookup table
  rdf::r_index s = rdf_session->rmgr()->create_resource("jets:acme:ProcedureLookup:100");
  rdf::r_index p = rdf_session->rmgr()->create_resource("jets:lookup_multi_rows");
  auto itor = rdf_session->find(rdf_session->get_object(s, p), rdf::make_any(), rdf::make_any());  
  while(not itor.is_end()) {
    std::cout<<"   "<<itor.as_triple()<<std::endl;
    auto predicate = rdf::get_name(itor.get_predicate());
    if(predicate == "PROC_RID") {
      EXPECT_EQ(std::string(rdf::get_type_name(itor.get_object())), std::string("long"));
      EXPECT_EQ(boost::get<rdf::LInt64>(*itor.get_object()).data, 12345678901L);
    } else if(predicate == "FROM_DATE") {
      EXPECT_EQ(std::string(rdf::get_type_name(itor.get_object())), std::string("date"));
    } else if(predicate == "EXCL") {
      EXPECT_EQ(std::string(rdf::get_type_name(itor.get_object())), std::string("int"));
    } else if(predicate == "EVENT_DURATION") {
      EXPECT_EQ(std::string(rdf::get_type_name(itor.get_object())), std::string("int"));
    } else {
      FAIL();
    }
    itor.next();
  }

  EXPECT_EQ(helper->terminate(), 0);
}

}   // namespace
}   // namespace jets::rete