#include <iostream>
#include <memory>
#include <filesystem>

#include <gtest/gtest.h>

#include "rete_meta_store_factory.h"
#include "sqlite3.h"

#include "jets/rdf/rdf_types.h"
#include "jets/rete/rete_types.h"

namespace fs = std::filesystem;
namespace jets::rete {
namespace {
// Simple test

static int callback(void *data, int argc, char **argv, char **azColName) {
  if (data) {
    std::cout << (const char*)data;
  }
   int i;
   for(i = 0; i<argc; i++) {
      std::cout << azColName[i] << " = " << (argv[i] ? argv[i] : "NULL") << std::endl;
   }
   std::cout << std::endl;
   return 0;
}


class SQLiteTest : public ::testing::Test {
 protected:
  SQLiteTest() : db_name("ms_test.db") {

    sqlite3 *db;
    char *zErrMsg = 0;
    int rc;
    char const* sql;

    /* Open database */
      std::filesystem::path p(db_name);
      std::cout << "Current path is " << fs::current_path() << '\n';
      std::cout << "Absolute path for " << p << " is " 
                << std::filesystem::absolute(p) << '\n';

      std::cout << "Path exist? " << std::filesystem::exists(p)  << '\n';
      if(std::filesystem::exists(p) and std::filesystem::is_regular_file(p)) {
        std::filesystem::remove(p);
      }

    //  rc = sqlite3_open(std::filesystem::absolute(p).c_str(), &db);
    rc = sqlite3_open(this->db_name.c_str(), &db);
    if( rc ) {
        std::cout<< "Can't open database: %s\n" << sqlite3_errmsg(db) << std::endl;
    } else {
        std::cout<< "Open database\n"  << std::endl;
    }
    EXPECT_EQ(rc, SQLITE_OK);

    /* Create SQL statement */
    sql = "CREATE TABLE COMPANY("  \
        "ID INT PRIMARY KEY     NOT NULL," \
        "NAME           TEXT    NOT NULL," \
        "AGE            INT     NOT NULL," \
        "ADDRESS        CHAR(50)," \
        "SALARY         REAL );";

    /* Execute SQL statement */
    rc = sqlite3_exec(db, sql, callback, 0, &zErrMsg);
    if( rc != SQLITE_OK ){
        fprintf(stderr, "SQL CREATE error: %s\n", zErrMsg);
        sqlite3_free(zErrMsg);
    } else {
        fprintf(stdout, "Table created successfully\n");
    }
    EXPECT_EQ(rc, SQLITE_OK);   

   /* Create SQL statement */
   sql = "INSERT INTO COMPANY (ID,NAME,AGE,ADDRESS,SALARY) "  \
         "VALUES (1, 'Paul', 32, 'California', 20000.00 ); " \
         "INSERT INTO COMPANY (ID,NAME,AGE,ADDRESS,SALARY) "  \
         "VALUES (2, 'Allen', 25, 'Texas', 15000.00 ); "     \
         "INSERT INTO COMPANY (ID,NAME,AGE,ADDRESS,SALARY)" \
         "VALUES (3, 'Teddy', 23, 'Norway', 20000.00 );" \
         "INSERT INTO COMPANY (ID,NAME,AGE,ADDRESS,SALARY)" \
         "VALUES (4, 'Mark', 25, 'Rich-Mond ', 65000.00 );";

   /* Execute SQL statement */
   rc = sqlite3_exec(db, sql, callback, 0, &zErrMsg);   
   if( rc != SQLITE_OK ){
      fprintf(stderr, "SQL INSERT error: %s\n", zErrMsg);
      sqlite3_free(zErrMsg);
   } else {
      fprintf(stdout, "Records created successfully\n");
   }
    EXPECT_EQ(rc, SQLITE_OK);   

   rc = sqlite3_close(db);
   if( rc != SQLITE_OK ){
      fprintf(stderr, "CLOSE error: %s\n", zErrMsg);
      sqlite3_free(zErrMsg);
   } else {
      fprintf(stdout, "DB closed successfully\n");
   }
   EXPECT_EQ(rc, SQLITE_OK);   

  }

  std::string db_name;
};

// Define the tests
TEST_F(SQLiteTest, FirstTest) {
  sqlite3 *db;
  char *zErrMsg = 0;
  int rc;
  char const*sql;
  const char* data = "Callback function called";

  /* Open database */
  rc = sqlite3_open(this->db_name.c_str(), &db);
  if( rc ) {
      fprintf(stderr, "Can't open database: %s\n", sqlite3_errmsg(db));
  } else {
      fprintf(stderr, "Opened database successfully\n");
  }
  EXPECT_EQ(rc, SQLITE_OK);   

  /* Create SQL statement */
  sql = "SELECT * from COMPANY";
  /* Execute SQL statement */
  rc = sqlite3_exec(db, sql, callback, (void*)data, &zErrMsg);    
  if( rc != SQLITE_OK ) {
      fprintf(stderr, "SQL error: %s\n", zErrMsg);
      sqlite3_free(zErrMsg);
  } else {
      fprintf(stdout, "Operation done successfully\n");
  }
  EXPECT_EQ(rc, SQLITE_OK);   

  rc = sqlite3_close(db);
  if( rc != SQLITE_OK ){
    fprintf(stderr, "CLOSE error: %s\n", zErrMsg);
    sqlite3_free(zErrMsg);
  } else {
    fprintf(stdout, "DB closed successfully\n");
  }
  EXPECT_EQ(rc, SQLITE_OK);
}

TEST(ReteMetaStoreFactoryTest, FactoryTest1) {

  ReteMetaStoreFactory factory;
  int res = factory.load_database("jets/test_data/jetrule_rete_test.db");
  EXPECT_EQ(res, 0);

  auto const* meta_graph = factory.meta_graph();

  auto r = meta_graph->get_rmgr()->get_resource("rdf:type");
  EXPECT_EQ(rdf::get_name(r), "rdf:type");
  // EXPECT_EQ(meta_graph->get_rmgr()->size(), 3);

  // Get the Rete Meta Store
  auto meta_store = factory.get_rete_meta_store("ms_factory_test1.jr");  
  EXPECT_TRUE(meta_store);

  // Create the rdf_session and the rete_session and initialize them
  // Initialize the rete_session now that the rule base is ready
  auto rdf_session = rdf::create_rdf_session(factory.get_meta_graph());
  auto rete_session = create_rete_session(meta_store, rdf_session);
  rete_session->initialize();
  auto mgr = rdf_session->get_rmgr();
  rdf::r_index iclaim = mgr->create_resource("iclaim");
  rdf::r_index rdf_type = mgr->create_resource("rdf:type");
  rdf::r_index hc_Claim = mgr->create_resource("hc:Claim");
  rdf::r_index hc_BaseClaim = mgr->create_resource("hc:BaseClaim");
  rdf_session->insert(iclaim, rdf_type, hc_Claim);
  rete_session->execute_rules();

  EXPECT_TRUE(rdf_session->contains(iclaim, rdf_type, hc_BaseClaim));
}

TEST(ReteMetaStoreFactoryTest, FactoryTest2) {

  ReteMetaStoreFactory factory;
  int res = factory.load_database("jets/test_data/jetrule_rete_test.db");
  EXPECT_EQ(res, 0);

  auto const* meta_graph = factory.meta_graph();

  auto r = meta_graph->get_rmgr()->get_resource("rdf:type");
  EXPECT_EQ(rdf::get_name(r), "rdf:type");
  // EXPECT_EQ(meta_graph->get_rmgr()->size(), 3);

  // Get the Rete Meta Store
  auto meta_store = factory.get_rete_meta_store("ms_factory_test2.jr");  
  EXPECT_TRUE(meta_store);

  // Create the rdf_session and the rete_session and initialize them
  // Initialize the rete_session now that the rule base is ready
  auto rdf_session = rdf::create_rdf_session(factory.get_meta_graph());
  auto rete_session = create_rete_session(meta_store, rdf_session);
  rete_session->initialize();  
  auto mgr = rdf_session->get_rmgr();

  // Print nodes configuration
  std::cout<<"Beta Nodes: "<<rete_session->beta_relations().size()<<std::endl;
  for(auto item: rete_session->beta_relations()){
    std::cout<<"    "<<item<<std::endl;
  }
  std::cout<<"Node Vertexes: "<<meta_store->nbr_vertices()<<std::endl;
  for(auto item: meta_store->node_vertexes()){
    std::cout<<"    "<<item<<std::endl;
  }
  std::cout<<"Alpha Nodes:"<<std::endl;
  for(auto const&item: meta_store->alpha_nodes()){
    std::cout<<"    "<<item.get()<<std::endl;
  }
  std::cout<<"========="<<std::endl;


  // Context Obj
  rdf::r_index iJetCtx = mgr->create_resource("iJetCtx");
  rdf::r_index jet_Context = mgr->create_resource("jet:Context");
  rdf::r_index first_name_rx = mgr->create_resource("first_name_rx");
  rdf::r_index last_name_rx = mgr->create_resource("last_name_rx");

  rdf::r_index iclaim = mgr->create_resource("iclaim");
  rdf::r_index rdf_type = mgr->create_resource("rdf:type");
  rdf::r_index hc_Claim = mgr->create_resource("hc:Claim");
  rdf::r_index hc_BaseClaim = mgr->create_resource("hc:BaseClaim");
  rdf::r_index hc_code1 = mgr->create_resource("hc:code1");
  rdf::r_index hc_code2 = mgr->create_resource("hc:code2");
  rdf::r_index hc_raw_name = mgr->create_resource("hc:raw_name");
  rdf_session->insert(iJetCtx, rdf_type, jet_Context);
  rdf_session->insert(iJetCtx, first_name_rx, mgr->create_literal("\\s*(\\w*)\\s*"));
  rdf_session->insert(iJetCtx, last_name_rx, mgr->create_literal("\\s*\\w*\\s*(\\w*)\\s*"));
  rdf_session->insert(iclaim, rdf_type, hc_Claim);
  rdf_session->insert(iclaim, hc_code1, mgr->create_literal(2));
  rdf_session->insert(iclaim, hc_code2, mgr->create_literal(3));
  rdf_session->insert(iclaim, hc_raw_name, mgr->create_literal("John Smith"));
  rdf_session->insert(mgr->create_resource("s0"), mgr->create_resource("node1"), mgr->create_resource("n1"));
  rdf_session->insert(mgr->create_resource("s0"), mgr->create_resource("node10"), mgr->create_resource("n1"));

  {  
    std::cout << "ASSERTED FACTS" << std::endl;
    auto itor = rdf_session->find();
    while(not itor.is_end()) {
      std::cout << itor.as_triple() << std::endl;
      itor.next();
    }
  }
  rete_session->execute_rules();
  std::cout << "DONE EXECUTE RULES" << std::endl;
  
  {
    std::cout << "INFERRED FACTS" << std::endl;
    auto itor = rdf_session->get_inferred_graph()->find();
    while(not itor.is_end()) {
      std::cout << itor.as_triple() << std::endl;
      itor.next();
    }
  }
  EXPECT_TRUE(rdf_session->contains(iclaim, rdf_type, hc_BaseClaim));
  EXPECT_TRUE(rdf_session->contains(iclaim, mgr->create_resource("hc:full_name"), mgr->create_literal("John Smith")));
  EXPECT_TRUE(rdf_session->contains(iclaim, mgr->create_resource("hc:first_name"), mgr->create_literal("John")));
  EXPECT_TRUE(rdf_session->contains(iclaim, mgr->create_resource("hc:last_name"), mgr->create_literal("Smith")));
  EXPECT_TRUE(rdf_session->contains(iclaim, mgr->create_resource("hc:code5"), mgr->create_literal(5)));
  EXPECT_TRUE(rdf_session->contains(mgr->create_resource("s0"), mgr->create_resource("node2"), mgr->create_resource("n1")));
  EXPECT_FALSE(rdf_session->contains(mgr->create_resource("s0"), mgr->create_resource("node3"), mgr->create_resource("n1")));
  EXPECT_TRUE(rdf_session->contains(mgr->create_resource("s0"), mgr->create_resource("node20"), mgr->create_resource("n1")));
  EXPECT_FALSE(rdf_session->contains(mgr->create_resource("s0"), mgr->create_resource("node30"), mgr->create_resource("n1")));
}

//rete_meta_store_test.db
}   // namespace
}   // namespace jets::rete