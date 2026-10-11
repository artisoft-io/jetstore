#include <filesystem>
#include <map>
#include <string>

#include <sqlite3.h>
#include <unistd.h>

#include <gtest/gtest.h>

#include "../rdf/rdf_types.h"
#include "../rete/rete_types.h"
#include "../rete/rete_meta_store_factory.h"
#include "../rete/lookup_sql_helper.h"

// A served lookup row carries jets:row_id, the row's SQLite rowid: its
// position in the table, which tells apart rows sharing a jets:key. Same
// fixture as lookup_rand_test.cc: key 100 is row 1, key 200 is rows 2 and 3
// with EXCL 0 and 1.

namespace jets::rete {
namespace {

TEST(LookupRowIdTest, EveryServedRowCarriesItsRowId) {
  ReteMetaStoreFactory factory;
  ASSERT_EQ(factory.load_database("test_data/lookup_helper_test_workspace.db", "test_data/lookup_helper_test_data.db"), 0);
  auto meta_store = factory.get_rete_meta_store("lookup_helper_test_workspace.jr");
  ASSERT_TRUE(meta_store);
  auto rdf_session = rdf::create_rdf_session(rdf::create_rdf_graph(factory.get_rmgr()));
  auto rete_session = create_rete_session(meta_store, rdf_session.get());
  rete_session->initialize();
  auto helper = meta_store->get_lookup_sql_helper();
  auto * rmgr = rdf_session->rmgr();
  auto row_id = rmgr->jets()->jets__row_id;
  ASSERT_TRUE(row_id);

  rdf::RdfAstType out;
  ASSERT_EQ(helper->lookup(rete_session.get(), "acme:ProcedureLookup", "100", &out), 0);
  auto id = rdf_session->get_object(rmgr->create_resource("jets:acme:ProcedureLookup:100"), row_id);
  ASSERT_TRUE(id);
  EXPECT_EQ(boost::get<rdf::LInt32>(*id).data, 1);

  rdf::RdfAstType multi;
  ASSERT_EQ(helper->multi_lookup(rete_session.get(), "acme:ProcedureLookup", "200", &multi), 0);
  std::map<int, int> excl_by_row_id;
  auto itor = rdf_session->find(rmgr->create_resource("jets:acme:ProcedureLookup:200"),
    rmgr->jets()->jets__lookup_multi_rows, rdf::make_any());
  while(not itor.is_end()) {
    auto row = itor.get_object();
    auto rid = rdf_session->get_object(row, row_id);
    ASSERT_TRUE(rid);
    // the column resources belong to the meta store, so EXCL is matched by name
    int ex = -1;
    for(auto jtor = rdf_session->find(row, rdf::make_any(), rdf::make_any()); not jtor.is_end(); jtor.next()) {
      if(rdf::get_name(jtor.get_predicate()) == "EXCL") ex = rdf::to_bool(*jtor.get_object()) ? 1 : 0;
    }
    excl_by_row_id[boost::get<rdf::LInt32>(*rid).data] = ex;
    itor.next();
  }
  EXPECT_EQ(excl_by_row_id, (std::map<int, int>{{2, 0}, {3, 1}}));
  EXPECT_EQ(helper->terminate(), 0);
}

// A SQL NULL in a lookup column is served as rdf null, whatever the column's
// type (I-45). The packager writes an empty bool cell as NULL; the numeric arms
// read NULL as 0, so before this C++ served it as false where Go served null.
TEST(LookupRowIdTest, ANullColumnIsServedAsNull) {
  auto db = std::filesystem::temp_directory_path() /
    ("lookup_null_test_" + std::to_string(getpid()) + ".db");
  std::filesystem::copy_file("test_data/lookup_helper_test_data.db", db,
    std::filesystem::copy_options::overwrite_existing);
  sqlite3 * conn = nullptr;
  ASSERT_EQ(sqlite3_open(db.c_str(), &conn), SQLITE_OK);
  ASSERT_EQ(sqlite3_exec(conn, "UPDATE \"acme:ProcedureLookup\" SET \"EXCL\" = NULL WHERE \"jets:key\" = '100'",
    nullptr, nullptr, nullptr), SQLITE_OK);
  sqlite3_close(conn);

  ReteMetaStoreFactory factory;
  ASSERT_EQ(factory.load_database("test_data/lookup_helper_test_workspace.db", db.string()), 0);
  auto meta_store = factory.get_rete_meta_store("lookup_helper_test_workspace.jr");
  ASSERT_TRUE(meta_store);
  auto rdf_session = rdf::create_rdf_session(rdf::create_rdf_graph(factory.get_rmgr()));
  auto rete_session = create_rete_session(meta_store, rdf_session.get());
  rete_session->initialize();
  auto helper = meta_store->get_lookup_sql_helper();
  auto * rmgr = rdf_session->rmgr();

  rdf::RdfAstType out;
  ASSERT_EQ(helper->lookup(rete_session.get(), "acme:ProcedureLookup", "100", &out), 0);
  auto row = rmgr->create_resource("jets:acme:ProcedureLookup:100");
  int found = 0;
  for(auto itor = rdf_session->find(row, rdf::make_any(), rdf::make_any()); not itor.is_end(); itor.next()) {
    if(rdf::get_name(itor.get_predicate()) == "EXCL") {
      found++;
      EXPECT_EQ(itor.get_object()->which(), rdf::rdf_null_t);
    }
  }
  EXPECT_EQ(found, 1);
  EXPECT_EQ(helper->terminate(), 0);
  std::filesystem::remove(db);
}

}   // namespace
}   // namespace jets::rete
