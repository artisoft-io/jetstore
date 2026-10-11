#include <filesystem>
#include <set>
#include <string>

#include <unistd.h>

#include <gtest/gtest.h>

#include "../rdf/rdf_types.h"
#include "../rete/rete_types.h"
#include "../rete/rete_meta_store_factory.h"
#include "../rete/lookup_sql_helper.h"

// lookup_sql_helper_test.cc is not in the build, so this test of the random
// lookups lives on its own. It uses the same fixture.

namespace jets::rete {
namespace {

// lookup_rand and multi_lookup_rand draw __key__ uniformly from [MIN, MAX].
// They drew from [0, MAX], which suited the earlier packager's keys from 0 and
// asked for a key that does not exist once the Go packager numbered them
// from 1, returning Null for it. The fixture's keys are 0 and 1; the test runs
// on it and on a copy shifted to 1 and 2.
TEST(LookupRandTest, DrawsEveryKeyWhateverTheBase) {
  for(int shift: {0, 1}) {
    auto path = std::filesystem::temp_directory_path() /
      ("lookup_rand_test_" + std::to_string(getpid()) + "_" + std::to_string(shift) + ".db");
    std::filesystem::copy_file("test_data/lookup_helper_test_data.db", path,
      std::filesystem::copy_options::overwrite_existing);
    sqlite3 * db = nullptr;
    ASSERT_EQ(sqlite3_open(path.c_str(), &db), SQLITE_OK);
    auto sql = "UPDATE \"acme:ProcedureLookup\" SET __key__ = __key__ + " + std::to_string(shift);
    ASSERT_EQ(sqlite3_exec(db, sql.c_str(), nullptr, nullptr, nullptr), SQLITE_OK);
    sqlite3_close(db);

    ReteMetaStoreFactory factory;
    ASSERT_EQ(factory.load_database("test_data/lookup_helper_test_workspace.db", path.string()), 0);
    auto meta_store = factory.get_rete_meta_store("lookup_helper_test_workspace.jr");
    ASSERT_TRUE(meta_store);
    auto rdf_session = rdf::create_rdf_session(rdf::create_rdf_graph(factory.get_rmgr()));
    auto rete_session = create_rete_session(meta_store, rdf_session.get());
    rete_session->initialize();
    auto helper = meta_store->get_lookup_sql_helper();

    std::set<std::string> want{
      "jets:acme:ProcedureLookup:" + std::to_string(shift),
      "jets:acme:ProcedureLookup:" + std::to_string(shift + 1)};
    std::set<std::string> want_multi{
      "jets:rand:acme:ProcedureLookup:" + std::to_string(shift),
      "jets:rand:acme:ProcedureLookup:" + std::to_string(shift + 1)};
    std::set<std::string> seen, seen_multi;
    for(int i = 0; i < 200; i++) {
      rdf::RdfAstType out;
      ASSERT_EQ(helper->lookup_rand(rete_session.get(), "acme:ProcedureLookup", &out), 0);
      ASSERT_NE(rdf::get_type(&out), rdf::rdf_null_t) << "lookup_rand returned Null, keys from " << shift;
      seen.insert(rdf::get_name(&out));
      rdf::RdfAstType multi;
      ASSERT_EQ(helper->multi_lookup_rand(rete_session.get(), "acme:ProcedureLookup", &multi), 0);
      ASSERT_NE(rdf::get_type(&multi), rdf::rdf_null_t) << "multi_lookup_rand returned Null, keys from " << shift;
      seen_multi.insert(rdf::get_name(&multi));
    }
    EXPECT_EQ(seen, want) << "keys from " << shift;
    EXPECT_EQ(seen_multi, want_multi) << "keys from " << shift;
    EXPECT_EQ(helper->terminate(), 0);
    std::filesystem::remove(path);
  }
}

}   // namespace
}   // namespace jets::rete
