#include <filesystem>
#include <string>

#include <sqlite3.h>
#include <unistd.h>

#include <gtest/gtest.h>

#include "../rdf/rdf_types.h"
#include "../rete/rete_types.h"
#include "../rete/rete_meta_store_factory.h"
#include "../rete/lookup_sql_helper.h"

// What the engine opens of lookup.db and the workspace db, and that it closes it.
// The fixture is lookup_rand_test.cc's: one lookup table, acme:ProcedureLookup.

namespace jets::rete {
namespace {

std::filesystem::path scratch(std::string const& name)
{
  return std::filesystem::temp_directory_path() /
    (name + "_" + std::to_string(getpid()) + ".db");
}

// I-167: lookup.db was opened on every engine build, with sqlite3_open's default
// flags, so a workspace with no lookup table created an empty file at the path.
TEST(LookupDbOpenTest, NoLookupTableOpensNoLookupDb)
{
  auto ws = scratch("no_lookup_ws");
  std::filesystem::copy_file("test_data/lookup_helper_test_workspace.db", ws,
    std::filesystem::copy_options::overwrite_existing);
  sqlite3 * db = nullptr;
  ASSERT_EQ(sqlite3_open(ws.c_str(), &db), SQLITE_OK);
  ASSERT_EQ(sqlite3_exec(db, "DELETE FROM lookup_columns; DELETE FROM lookup_tables", nullptr, nullptr, nullptr), SQLITE_OK);
  sqlite3_close(db);

  auto lookup = scratch("no_lookup_data");
  std::filesystem::remove(lookup);
  {
    ReteMetaStoreFactory factory;
    EXPECT_EQ(factory.load_database(ws.string(), lookup.string()), 0);
  }
  EXPECT_FALSE(std::filesystem::exists(lookup)) << lookup << " was created";
  std::filesystem::remove(lookup);
  std::filesystem::remove(ws);
}

// I-167: with a lookup table declared, a missing lookup.db is an error and is not
// created; it used to be created empty, and the error came from the empty table.
TEST(LookupDbOpenTest, AMissingLookupDbIsAnErrorAndIsNotCreated)
{
  auto lookup = scratch("missing_lookup_data");
  std::filesystem::remove(lookup);
  {
    ReteMetaStoreFactory factory;
    EXPECT_NE(factory.load_database("test_data/lookup_helper_test_workspace.db", lookup.string()), 0);
  }
  EXPECT_FALSE(std::filesystem::exists(lookup)) << lookup << " was created";
  std::filesystem::remove(lookup);
}

// The engine reads the lookup db and never writes it: its connections are read-only
TEST(LookupDbOpenTest, LookupConnectionsAreReadOnly)
{
  ReteMetaStoreFactory factory;
  ASSERT_EQ(factory.load_database("test_data/lookup_helper_test_workspace.db", "test_data/lookup_helper_test_data.db"), 0);
  auto meta_store = factory.get_rete_meta_store("lookup_helper_test_workspace.jr");
  ASSERT_TRUE(meta_store);
  DBConnectionPool pool("test_data/lookup_helper_test_data.db");
  pool.initialize("SELECT 1", "");
  auto lc = pool.get_connection();
  ASSERT_NE(lc.db, nullptr);
  EXPECT_EQ(sqlite3_db_readonly(lc.db, "main"), 1);
  pool.put_connection(lc);
}

// One factory's life: load, a session, a lookup (which opens a pooled connection)
void run_one_factory()
{
  ReteMetaStoreFactory factory;
  ASSERT_EQ(factory.load_database("test_data/lookup_helper_test_workspace.db", "test_data/lookup_helper_test_data.db"), 0);
  auto meta_store = factory.get_rete_meta_store("lookup_helper_test_workspace.jr");
  ASSERT_TRUE(meta_store);
  auto rdf_session = rdf::create_rdf_session(rdf::create_rdf_graph(factory.get_rmgr()));
  auto rete_session = create_rete_session(meta_store, rdf_session.get());
  rete_session->initialize();
  rdf::RdfAstType out;
  ASSERT_EQ(meta_store->get_lookup_sql_helper()->lookup(rete_session.get(), "acme:ProcedureLookup", "100", &out), 0);
  ASSERT_GE(meta_store->get_lookup_sql_helper()->type_of(rete_session.get(), "acme:key"), -1);
}

// I-144, I-170: a factory that goes closes every connection it and its lookup
// helper opened, with no terminate() call. ~ReteMetaStoreFactory had its reset()
// commented out, LookupSqlHelper had no destructor, and the TypeOf pool was never
// closed, so each factory left its connections' memory behind. SQLite counts its
// own allocations, so this shows the leak without a sanitizer.
TEST(LookupDbOpenTest, AFactoryClosesWhatItOpened)
{
  run_one_factory();   // SQLite's own first-use allocations
  sqlite3_int64 before = 0, peak = 0;
  // the last argument resets the high-water mark to the current use
  ASSERT_EQ(sqlite3_status64(SQLITE_STATUS_MEMORY_USED, &before, &peak, 1), SQLITE_OK);
  for(int i = 0; i < 3; i++) run_one_factory();
  sqlite3_int64 after = 0;
  ASSERT_EQ(sqlite3_status64(SQLITE_STATUS_MEMORY_USED, &after, &peak, 0), SQLITE_OK);
  ASSERT_GT(peak, before) << "SQLite does not count its memory in this build";
  EXPECT_EQ(after, before) << "three factories left " << (after - before) << " bytes of SQLite memory";
}

}   // namespace
}   // namespace jets::rete
