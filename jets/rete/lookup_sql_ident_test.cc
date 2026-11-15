#include <filesystem>
#include <string>
#include <vector>

#include <sqlite3.h>
#include <unistd.h>

#include <gtest/gtest.h>

#include "../rdf/rdf_types.h"
#include "../rete/rete_types.h"
#include "../rete/rete_meta_store_factory.h"
#include "../rete/lookup_sql_helper.h"

// The lookup reader quotes lookup.db's table and column names as the packager
// writes them (sqliteIdent, jets/workspace/lookup_tables.go): in double quotes,
// an embedded double quote doubled. Same fixture as lookup_rand_test.cc.

namespace jets::rete {
namespace {

TEST(LookupSqlIdentTest, QuotesAnIdentifier) {
  EXPECT_EQ(sql_ident("acme:ProcedureLookup"), "\"acme:ProcedureLookup\"");
  EXPECT_EQ(sql_ident(""), "\"\"");
  EXPECT_EQ(sql_ident("a\"b"), "\"a\"\"b\"");
  EXPECT_EQ(sql_ident("\""), "\"\"\"\"");
  EXPECT_EQ(sql_ident("a\\b"), "\"a\\b\"");
}

// For a name with no double quote the statements are the text the reader built
// before it had sql_ident, byte for byte: '"' + name + '"'. These literals are
// that text for the fixture's table and columns.
TEST(LookupSqlIdentTest, StatementsAreUnchangedForAnOrdinaryName) {
  EXPECT_EQ(lookup_key_bound_sql("MIN", "acme:ProcedureLookup"),
    "SELECT MIN(__key__) FROM \"acme:ProcedureLookup\"");
  EXPECT_EQ(lookup_key_bound_sql("MAX", "acme:ProcedureLookup"),
    "SELECT MAX(__key__) FROM \"acme:ProcedureLookup\"");
  EXPECT_EQ(lookup_select_sql("acme:ProcedureLookup", {"EVENT_DURATION", "PROC_RID", "FROM_DATE", "EXCL"}),
    "SELECT \"EVENT_DURATION\", \"PROC_RID\", \"FROM_DATE\", \"EXCL\", _rowid_ FROM \"acme:ProcedureLookup\" WHERE ");
  EXPECT_EQ(lookup_select_sql("acme:ProcedureLookup", {}),
    "SELECT _rowid_ FROM \"acme:ProcedureLookup\" WHERE ");
  // and the general case: any name without a double quote is wrapped as it was
  for(std::string name: {"x", "jets:key", "a b", "a\\b", "a'b", "a;b", "acme:Lookup_v2"}) {
    EXPECT_EQ(sql_ident(name), "\"" + name + "\"") << name;
  }
}

std::filesystem::path scratch(std::string const& name)
{
  return std::filesystem::temp_directory_path() /
    (name + "_" + std::to_string(getpid()) + ".db");
}

int exec(std::filesystem::path const& path, std::string const& sql)
{
  sqlite3 * db = nullptr;
  int err = sqlite3_open(path.c_str(), &db);
  if(err == SQLITE_OK) err = sqlite3_exec(db, sql.c_str(), nullptr, nullptr, nullptr);
  sqlite3_close(db);
  return err;
}

// A lookup table and a column whose names hold a double quote and a backslash,
// declared beside the fixture's table and packaged as the packager writes them,
// are read: the key range, a lookup by jets:key, and a random lookup by __key__.
TEST(LookupSqlIdentTest, ReadsATableAndColumnNamedWithAQuoteAndABackslash) {
  std::string const table = "acme:Odd\"Name\\Lookup";
  std::string const column = "VALUE\"\\COL";

  auto ws = scratch("lookup_ident_ws");
  auto data = scratch("lookup_ident_data");
  std::filesystem::copy_file("test_data/lookup_helper_test_workspace.db", ws,
    std::filesystem::copy_options::overwrite_existing);
  std::filesystem::copy_file("test_data/lookup_helper_test_data.db", data,
    std::filesystem::copy_options::overwrite_existing);

  // The workspace db names the table and its column as data, bound as literals
  ASSERT_EQ(exec(ws,
    "INSERT INTO lookup_tables (key, name, table_name, csv_file, lookup_key, lookup_resources, source_file_key) "
    "VALUES (7, 'acme:Odd\"Name\\Lookup', '', 'odd.csv', 'K', 'K', 1);"
    "INSERT INTO lookup_columns (lookup_table_key, name, type, as_array) "
    "VALUES (7, 'VALUE\"\\COL', 'text', 0);"), SQLITE_OK);
  // lookup.db, created as the packager's CREATE statement spells it
  ASSERT_EQ(exec(data,
    "CREATE TABLE " + sql_ident(table) + " (__key__ INTEGER, \"jets:key\" TEXT NOT NULL, "
    + sql_ident(column) + " TEXT);"
    "INSERT INTO " + sql_ident(table) + " VALUES (1, 'k1', 'first'), (2, 'k2', 'second');"), SQLITE_OK);

  {
    ReteMetaStoreFactory factory;
    ASSERT_EQ(factory.load_database(ws.string(), data.string()), 0);
    auto meta_store = factory.get_rete_meta_store("lookup_helper_test_workspace.jr");
    ASSERT_TRUE(meta_store);
    auto rdf_session = rdf::create_rdf_session(rdf::create_rdf_graph(factory.get_rmgr()));
    auto rete_session = create_rete_session(meta_store, rdf_session.get());
    rete_session->initialize();
    auto helper = meta_store->get_lookup_sql_helper();
    auto * rmgr = rdf_session->rmgr();

    rdf::RdfAstType out;
    ASSERT_EQ(helper->lookup(rete_session.get(), table, "k2", &out), 0);
    ASSERT_NE(rdf::get_type(&out), rdf::rdf_null_t);
    EXPECT_EQ(rdf::get_name(&out), "jets:" + table + ":k2");
    // the column resources belong to the meta store, so the column is matched by name
    std::string value;
    auto row = rmgr->create_resource("jets:" + table + ":k2");
    for(auto itor = rdf_session->find(row, rdf::make_any(), rdf::make_any()); not itor.is_end(); itor.next()) {
      if(rdf::get_name(itor.get_predicate()) == column) value = boost::get<rdf::LString>(*itor.get_object()).data;
    }
    EXPECT_EQ(value, "second");

    rdf::RdfAstType rand;
    ASSERT_EQ(helper->lookup_rand(rete_session.get(), table, &rand), 0);
    EXPECT_NE(rdf::get_type(&rand), rdf::rdf_null_t);

    // the fixture's own table still reads
    rdf::RdfAstType fixture;
    ASSERT_EQ(helper->lookup(rete_session.get(), "acme:ProcedureLookup", "100", &fixture), 0);
    EXPECT_NE(rdf::get_type(&fixture), rdf::rdf_null_t);
    EXPECT_EQ(helper->terminate(), 0);
  }
  std::filesystem::remove(ws);
  std::filesystem::remove(data);
}

}   // namespace
}   // namespace jets::rete
