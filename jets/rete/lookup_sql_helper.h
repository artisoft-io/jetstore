#ifndef JETS_RETE_LOOKUP_SQL_HELPER_H
#define JETS_RETE_LOOKUP_SQL_HELPER_H

#include <cctype>
#include <cstdint>
#include <type_traits>
#include <iostream>
#include <sstream>
#include <algorithm>
#include <string>
#include <memory>
#include <utility>
#include <mutex>
#include <list>
#include <vector>
#include <unordered_map>

#include "sqlite3.h"

#include "../rdf/rdf_types.h"
#include "../rete/rete_err.h"
#include "../rete/beta_row.h"
#include "../rete/rete_session.h"

// This file contains helper class to lookup multi_lookup operators
// SQLite callback for lookup column info
static int lookup_column_cb(void *data, int argc, char **argv, char **azColName);

namespace jets::rete {

using RDFTTYPE = rdf::RdfAstType;

// Utility Functions
// --------------------------------------------------------------------------------------
inline std::string to_table_name(std::string const& resource_name)
{
  std::string str;
  str.reserve(resource_name.size()+5);
  for(auto c: resource_name) {
    if(c == ':') str.append("__");
    else str.push_back(c);
  }
  boost::to_lower(str);
  return str;
}

/////////////////////////////////////////////////////////////////////////////////////////
class LookupTable;
using LookupTablePtr = std::shared_ptr<LookupTable>;

struct LookupConnection {
  sqlite3 *     db;
  sqlite3_stmt* stmt;
};
using LCPool = std::list<LookupConnection>;

// LookupTable - Component for each lookup table
// ------------------------------------------------------------------------------------
class LookupTable {
 public:
  using ColumnInfo = std::pair<rdf::r_index, std::string>; // column name as resource, range type
  using LookupInfoV = std::vector<ColumnInfo>;
  LookupTable(rdf::RDFGraph * meta_graph, int lookup_key, std::string_view lookup_name, std::string_view lookup_db_path)
    : meta_graph_(meta_graph),
    lookup_key_(lookup_key),
    lookup_name_(lookup_name),
    lookup_db_path_(lookup_db_path),
    mutex_(),
    pool_(),
    columns_()
  {}

  int initialize(sqlite3 * workspace_db)
  {
    if(not meta_graph_ or not workspace_db) {
      LOG(ERROR) << "LookupTable::initialize: ERROR: Arguments meta_graph and workspace_db are required";
      return -1;
    }
    int err = 0;
    char * err_msg = 0;
    std::string sql = "SELECT name, type, as_array from lookup_columns WHERE lookup_table_key = ";
    sql += std::to_string(this->lookup_key_);
    err = sqlite3_exec(workspace_db, sql.c_str(), ::lookup_column_cb, (void*)this, &err_msg);
    if( err != SQLITE_OK ) {
      LOG(ERROR) << "LookupTable::initialize: SQL error while reading column details: " << err_msg;
      sqlite3_free(err_msg);
      return err;
    }

    // Create statement and cnx pool
    std::ostringstream sqlb("SELECT ", std::ios_base::ate);
    bool is_first = true;
    for(auto const& colinfo: this->columns_) {
      if(not is_first) sqlb << ", ";
      is_first = false;
      sqlb << rdf::get_name(colinfo.first);
    }
    sqlb << " FROM " << to_table_name(this->lookup_name_);
    sqlb << " WHERE jets__key = ?";
    sql = sqlb.str();
    //*
    std::cout <<"LOOKUP SQL: " << sql << std::endl;
    // setup the first connection
    sqlite3 *   db;
    sqlite3_stmt* stmt;
    err = sqlite3_open(this->lookup_db_path_.c_str(), &db);
    if( err ) {
      LOG(ERROR) << "LookupTable::initialize: ERROR: Can't open database: '" <<
        this->lookup_db_path_<<"' as lookup_db_path, error:" << sqlite3_errmsg(db);
      return err;
    }
    err = sqlite3_prepare_v2( db, sql.c_str(), -1, &stmt, 0 );
    if ( err != SQLITE_OK ) {
      return err;
    }
    this->pool_.push_back({db, stmt});
    return 0;
  }

  int lookup_column_cb(int argc, char **argv, char **colnm)
  {
    // name             0  STRING NOT NULL,
    // type             1  STRING NOT NULL,
    // as_array         2  BOOL, (not implemented)
    auto rmgr = this->meta_graph_->rmgr();
    this->columns_.push_back({rmgr->create_resource(argv[0]), argv[1]});
    return 0;
  }

  // close connection pool
  int terminate()
  {
    //*
    std::cout <<"LOOKUP TERMINATE CALLED" << std::endl;
    int err = 0;
    for(auto info: this->pool_) {
      sqlite3_finalize( info.stmt );
      int xerr = sqlite3_close_v2( info.db );
      if ( xerr != SQLITE_OK ) {
        err = xerr;
        LOG(ERROR) << "LookupTable::terminate: ERROR while closing rete_db connection, code: "<<err;
      }
    }
    return err;
  }

  RDFTTYPE lookup(std::string_view key)
  {
    return {};
  }

 private:
  rdf::RDFGraph *     meta_graph_;
  int                 lookup_key_;
  std::string         lookup_name_;
  std::string         lookup_db_path_;
  mutable std::mutex  mutex_;
  LCPool              pool_;
  LookupInfoV         columns_;
};

inline LookupTablePtr 
create_lookup_table(rdf::RDFGraph * meta_graph, int lookup_key, std::string_view lookup_name, std::string_view lookup_db_path)
{
  return std::make_shared<LookupTable>(meta_graph, lookup_key, lookup_name, lookup_db_path);
}

/////////////////////////////////////////////////////////////////////////////////////////
class LookupSqlHelper;
using LookupSqlHelperPtr = std::shared_ptr<LookupSqlHelper>;

// LookupSqlHelper class to manage the lookup table as sqlite3 tables
// --------------------------------------------------------------------------------------
class LookupSqlHelper {
 public:
  using LookupInfo = std::pair<int, std::string>; // lookup key, lookup name
  using LookupInfoList = std::list<LookupInfo>;
  using LookupTableMap = std::unordered_map<rdf::r_index, LookupTablePtr>;

  LookupSqlHelper() = delete;

  LookupSqlHelper(std::string_view workspace_db_path, std::string_view lookup_db_path) 
    : workspace_db_path_(workspace_db_path),
    workspace_db_(nullptr),
    lookup_db_path_(lookup_db_path),
    lookup_tbl_info_(),
    lookup_tbl_map_()
    {}

  /**
   * @brief Initialize the helper, open database connections
   */
  inline int initialize(rdf::RDFGraph * meta_graph)
  {
    int err = 0;
    err = sqlite3_open(this->workspace_db_path_.c_str(), &this->workspace_db_);
    if( err ) {
      LOG(ERROR) << "LookupSqlHelper::initialize: ERROR: Can't open database: '" <<
        this->workspace_db_path_<<"' as workspace_db_path, error:" << sqlite3_errmsg(this->workspace_db_);
      return err;
    }

    // Need to get the list of all lookup tables from workspace_db
    err = this->load_lookup_table_info();
    if( err ) return err;

    // Prepare the LookupTable that will cast the retured columns
    err = 0;
    auto rmgr = meta_graph->rmgr();
    for(auto const& info: this->lookup_tbl_info_) {
      auto r = rmgr->create_resource(info.second);
      auto l = create_lookup_table(meta_graph, info.first, info.second, this->lookup_db_path_);
      this->lookup_tbl_map_.insert({r, l});
      int xerr = l->initialize(this->workspace_db_);
      if(xerr) {
        err = xerr;
        LOG(ERROR) << "LookupSqlHelper::initialize: ERROR while initializing LookupTable: " << r;
      }
    }
    // All good!
    return err;
  }

  // close connection pools
  int terminate()
  {
    int err = 0;
    for(auto &info: this->lookup_tbl_map_) {
      int xerr = info.second->terminate();
      if( xerr ) {
        err = xerr;
        LOG(ERROR) << "LookupSqlHelper::terminate: ERROR while terminating LookupTable: " << info.first;
      }
    }
    return err;
  }

 protected:

  int
  load_lookup_table_info()
  {
    this->lookup_tbl_info_.clear();
    auto const* sql = "SELECT key, name from lookup_tables";
    sqlite3_stmt* stmt;
    int res = sqlite3_prepare_v2( this->workspace_db_, sql, -1, &stmt, 0 );
    if ( res != SQLITE_OK ) {
      return res;
    }

    bool is_done = false;
    while(not is_done) {
      res = sqlite3_step( stmt );
      if ( res == SQLITE_DONE ) {
        is_done = true;
        continue;
      }
      if(res != SQLITE_ROW) {
        LOG(ERROR) << "ReteMetaStoreFactory::load_lookup_table_info: SQL error while load_workspace_control: " << res;
        return res;
      }
      // Get the data out of the row and in the lookup map
      int key = sqlite3_column_int( stmt, 0 );
      std::string name((char const*)sqlite3_column_text( stmt, 1 ));
      this->lookup_tbl_info_.push_back({key, name});
    }
    sqlite3_finalize( stmt );
    return 0;
  }

 private:

  std::string workspace_db_path_;
  sqlite3 *   workspace_db_;
  std::string lookup_db_path_;
  LookupInfoList lookup_tbl_info_;
  LookupTableMap lookup_tbl_map_;
};

inline LookupSqlHelperPtr 
create_lookup_sql_helper(std::string_view workspace_db_path, std::string_view lookup_db_path)
{
  return std::make_shared<LookupSqlHelper>(workspace_db_path, lookup_db_path);
}

} // namespace jets::rete

// ======================================================================================
// CALLBACK FUNCTIONS
// --------------------------------------------------------------------------------------
/**
 * @brief Callback for reading lookup columns from sqlite3.exec
 * 
 * @param data      Data provided in the 4th argument of sqlite3_exec() 
 * @param argc      The number of columns in row 
 * @param argv      An array of strings representing fields in the row 
 * @param azColName An array of strings representing column names  
 * @return int 
 */
static int lookup_column_cb(void *data, int argc, char **argv, char **colname) {
  jets::rete::LookupTable * factory = nullptr;
  if (data) {
    factory = (jets::rete::LookupTable *)data;
  }
  if(not factory) {
    LOG(ERROR) << "LookupTable::initialize: ERROR: lookup_column_cb have no factory!!";
    return SQLITE_ERROR;
  }
  if(argc != 3) {
    LOG(ERROR) << "LookupTable::initialize: ERROR: lookup_column_cb invalid nbr of columns!!";
    return SQLITE_ERROR;
  }
  return factory->lookup_column_cb(argc, argv, colname);
}
#endif // JETS_RETE_LOOKUP_SQL_HELPER_H
