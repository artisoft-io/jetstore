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
#include <random>

#include "sqlite3.h"

#include "../rdf/rdf_types.h"
#include "../rete/rete_err.h"
#include "../rete/beta_row.h"

// This file contains helper class to lookup multi_lookup operators
// SQLite callback for lookup column info
static int lookup_column_cb(void *data, int argc, char **argv, char **azColName);

namespace jets::rete {
class ReteSession;

using RDFTTYPE = rdf::RdfAstType;

// Utility Classes & Functions
// --------------------------------------------------------------------------------------
inline 
std::string to_table_name(std::string const& resource_name)
{
  // std::string str;
  // str.reserve(resource_name.size()+5);
  // for(auto c: resource_name) {
  //   if(c == ':') str.append("__");
  //   else str.push_back(c);
  // }
  // boost::to_lower(str);
  // return str;
  return resource_name;
}

inline
int run_count_statement( sqlite3 * db, std::string sql )
{
  sqlite3_stmt* stmt;
  int res = sqlite3_prepare_v2( db, sql.data(), -1, &stmt, 0 );
  if ( res != SQLITE_OK )
    return -1;

  res = sqlite3_step( stmt );
  if ( res != SQLITE_ROW )
    return -1;

  int count = sqlite3_column_int( stmt, 0 );
  sqlite3_finalize( stmt );
  return count;
}


// DBConnectionPool - Manage a connection pool with 2 compiled statements, stmt1 & stmt2
// ------------------------------------------------------------------------------------
struct DBConnection {
  sqlite3 *     db{nullptr};
  sqlite3_stmt* stmt1{nullptr};
  sqlite3_stmt* stmt2{nullptr};
};
using LCPool = std::list<DBConnection>;

class DBConnectionPool {
 public:
  explicit
  DBConnectionPool(std::string_view db_path)
    : db_path_(db_path),
    sql1_(),
    sql2_(),
    mutex_(),
    pool_()
  {}

  int initialize(std::string sql1, std::string sql2)
  {
    if(not sql1.empty()) {
      this->sql1_ = std::move(sql1);
    }
    if(not sql2.empty()) {
      this->sql2_ = std::move(sql2);
    }
    VLOG(20) << "Lookup Connection Pool initialize with statements " << this->sql1_
      <<(this->sql2_.empty()?"":" and ") << this->sql2_ ;
    return 0;
  }

  size_t size()const
  {
    return this->pool_.size();
  }

  DBConnection get_connection()
  {
    std::lock_guard<std::mutex> lock(mutex_);
    if(this->pool_.empty()) {
      // setup a new connection
      DBConnection lc;
      lc.stmt1 = nullptr;
      lc.stmt2 = nullptr;
      int err = sqlite3_open(this->db_path_.c_str(), &lc.db);
      if( err ) {
        LOG(ERROR) << "DBConnection::get_connection: ERROR: Can't open database: '" <<
          this->db_path_<<"' as db_path, error:" << sqlite3_errmsg(lc.db);
        return {};
      }
      if(not this->sql1_.empty()) {
        err = sqlite3_prepare_v2( lc.db, this->sql1_.c_str(), -1, &lc.stmt1, 0 );
        if ( err != SQLITE_OK ) {
          LOG(ERROR) << "DBConnection::get_connection: ERROR: Can't prepare sql1: " << this->sql1_;
          return {};
        }
      }
      if(not this->sql2_.empty()) {
        err = sqlite3_prepare_v2( lc.db, this->sql2_.c_str(), -1, &lc.stmt2, 0 );
        if ( err != SQLITE_OK ) {
          LOG(ERROR) << "DBConnection::get_connection: ERROR: Can't prepare sql2: " << this->sql2_;
          return {};
        }
      }
      return lc;
    }
    auto lc = this->pool_.front();
    this->pool_.pop_front();
    return lc;
  }

  void put_connection(DBConnection lc)
  {
    std::lock_guard<std::mutex> lock(mutex_);
    this->pool_.push_back(std::move(lc));
  }

  // close connection pool
  int terminate()
  {
    int err = 0;
    VLOG(20)<<"DB Pool Terminate called, pool size is "<<this->pool_.size()<<" for statement "<<this->sql1_<<std::endl;
    for(auto info: this->pool_) {
      if(not this->sql1_.empty()) {
        sqlite3_finalize( info.stmt1 );
      }
      if(not this->sql2_.empty()) {
        sqlite3_finalize( info.stmt2 );
      }
      int xerr = sqlite3_close_v2( info.db );
      if ( xerr != SQLITE_OK ) {
        err = xerr;
        LOG(ERROR) << "LookupTable::terminate: ERROR while closing rete_db connection, code: "<<err;
      }
    }
    return err;
  }

 private:
  std::string         db_path_;
  std::string         sql1_;
  std::string         sql2_;
  mutable std::mutex  mutex_;
  LCPool              pool_;
};

/////////////////////////////////////////////////////////////////////////////////////////
class LookupTable;
using LookupTablePtr = std::shared_ptr<LookupTable>;

// LookupTable - Component for each lookup table
// ------------------------------------------------------------------------------------
class LookupTable {
 public:
  using ColumnInfo = std::pair<rdf::r_index, int>; // column name as resource, range type (which code)
  using LookupInfoV = std::vector<ColumnInfo>;
  LookupTable(rdf::RManager * rmgr, int lookup_key, std::string_view lookup_name, std::string_view lookup_db_path)
    : rmgr_(rmgr),
    lookup_key_(lookup_key),
    lookup_name_(lookup_name),
    cache_uri_(nullptr),
    subject_prefix_("jets:"),
    subject_rand_prefix_("jets:rand:"),
    db_pool_(lookup_db_path),
    columns_(),
    max_key_(0),
    rand_eng_(),
    uniform_dist_()
  {
    this->cache_uri_ = this->rmgr_->create_resource(this->lookup_name_);
    this->subject_prefix_.append(this->lookup_name_).push_back(':');
    this->subject_rand_prefix_.append(this->lookup_name_).push_back(':');
  }

  int initialize(sqlite3 * workspace_db, sqlite3 * lookup_db)
  {
    if(not this->rmgr_ or not workspace_db) {
      LOG(ERROR) << "LookupTable::initialize: ERROR: Arguments rmgr and workspace_db are required";
      return -1;
    }
    int err = 0;
    char * err_msg = 0;
    auto lookup_table_name = to_table_name(this->lookup_name_);
    // Get the columns info
    std::string sql = "SELECT name, type, as_array from lookup_columns WHERE lookup_table_key = ";
    sql += std::to_string(this->lookup_key_);
    err = sqlite3_exec(workspace_db, sql.c_str(), ::lookup_column_cb, (void*)this, &err_msg);
    if( err != SQLITE_OK ) {
      LOG(ERROR) << "LookupTable::initialize: SQL error while reading column details: " << err_msg;
      sqlite3_free(err_msg);
      return err;
    }

    // Get table's max key
    sql = "SELECT MAX(__key__) FROM \"" + lookup_table_name+"\"";
    this->max_key_ = run_count_statement(lookup_db, sql);
    if(this->max_key_ < 0) {
      LOG(ERROR) << "LookupTable::initialize: ERROR while getting last key of lookup table "<<lookup_table_name;
      return -1;
    }
    VLOG(30)<<"Lookup table "<<this->lookup_name_<<" max __key__ is "<<this->max_key_<<std::endl;

    // Prepare for random lookup
    this->uniform_dist_ = std::uniform_int_distribution<int>(0, this->max_key_);

    // Create statement and cnx pool
    std::ostringstream sqlb("SELECT ", std::ios_base::ate);
    bool is_first = true;
    for(auto const& colinfo: this->columns_) {
      if(not is_first) sqlb << ", ";
      is_first = false;
      sqlb << "\""<< rdf::get_name(colinfo.first) << "\"";
    }
    sqlb << " FROM \"" << lookup_table_name << "\" WHERE ";
    auto str = sqlb.str();
    err = this->db_pool_.initialize(str+"\"jets:key\" = ?", str+"__key__ = ?");
    if(err) {
      LOG(ERROR) << "LookupTable::initialize: ERROR while initializing DBConnectionPool";
      return err;
    }
    auto lc = this->db_pool_.get_connection();
    if(not lc.db or (not lc.stmt1 and not lc.stmt2)) {
      LOG(ERROR) << "LookupTable::initialize: ERROR while initializing first connection in DBConnectionPool";
      return -100;
    }
    this->db_pool_.put_connection(lc);
    LOG(INFO) << "LookupTable '" << this->lookup_name_ <<"' initialized";
    return 0;
  }

  // close connection pool
  inline
  int terminate()
  {
    return this->db_pool_.terminate();
  }

  inline
  int lookup(ReteSession * rete_session, std::string const& key, RDFTTYPE * out)
  {
    return this->lookup_internal(rete_session, false, key, out);
  }

  inline
  int lookup_rand(ReteSession * rete_session, RDFTTYPE * out)
  {
    return this->lookup_internal_rand(rete_session, false, out);
  }

  inline
  int multi_lookup(ReteSession * rete_session, std::string const& key, RDFTTYPE * out)
  {
    return this->lookup_internal(rete_session, true, key, out);
  }

  inline
  int multi_lookup_rand(ReteSession * rete_session, RDFTTYPE * out)
  {
    return this->lookup_internal_rand(rete_session, true, out);
  }

  inline
  int lookup_column_cb(int argc, char **argv, char **colnm)
  {
    // name             0  STRING NOT NULL,
    // type             1  STRING NOT NULL,
    // as_array         2  BOOL, (not implemented)
    this->columns_.push_back({this->rmgr_->create_resource(argv[0]), rdf::type_name2which(argv[1])});
    return 0;
  }

 protected:

  int lookup_internal(ReteSession * rete_session, bool is_multi, std::string const& key, RDFTTYPE * out);
  int lookup_internal_rand(ReteSession * rete_session, bool is_multi, RDFTTYPE * out);
  int lookup_internal_c(ReteSession * rete_session, bool is_multi, rdf::r_index subject, std::string const& key, sqlite3_stmt* stmt, RDFTTYPE * out);

 private:

  rdf::RManager *                    rmgr_;
  int                                lookup_key_;
  std::string                        lookup_name_;
  rdf::r_index                       cache_uri_;
  std::string                        subject_prefix_;
  std::string                        subject_rand_prefix_;
  DBConnectionPool                   db_pool_;
  LookupInfoV                        columns_;
  int                                max_key_;
  std::default_random_engine         rand_eng_;
  std::uniform_int_distribution<int> uniform_dist_;
};

inline LookupTablePtr 
create_lookup_table(rdf::RManager * rmgr, int lookup_key, std::string_view lookup_name, std::string_view lookup_db_path)
{
  return std::make_shared<LookupTable>(rmgr, lookup_key, lookup_name, lookup_db_path);
}

// //////////////////////////////////////////////////////////////////////////////////////
// TypeOf - Get domain property range's type for casting purpose, used by ToTypeOfVisitor
// ------------------------------------------------------------------------------------
/////////////////////////////////////////////////////////////////////////////////////////
class TypeOf;
using TypeOfPtr = std::shared_ptr<TypeOf>;

class TypeOf {
 public:
  TypeOf(std::string_view workspace_db_path)
    : db_pool_(workspace_db_path)
  {
  }

  int initialize(sqlite3 * )
  {
    // Create statement and cnx pool
    this->db_pool_.initialize("SELECT type, as_array FROM data_properties WHERE name = ?", "");

    // setup the first connection to make sure we can open it
    auto lc = this->db_pool_.get_connection();
    if(not lc.db or not lc.stmt1) return -100;
    this->db_pool_.put_connection(lc);
    return 0;
  }

  // close connection pool
  int terminate()
  {
    return this->db_pool_.terminate();
  }

  // return rdf_ast_which_order as rdf type or -1 if error
  int type_of(ReteSession *, std::string const& data_property)
  {
    // Get the db connection and bind it to the key
    auto lc = this->db_pool_.get_connection();
    int err = sqlite3_reset(lc.stmt1);
    if( err != SQLITE_OK ) return -100;

    err = sqlite3_bind_text(lc.stmt1, 1, data_property.c_str(), data_property.size(), nullptr);
    if( err != SQLITE_OK ) return -1;

    err = sqlite3_step( lc.stmt1 );
    if ( err != SQLITE_ROW ) {
      LOG(ERROR)<<"TypeOf::type_of: ERROR Unknown Data Property: "<<data_property;
      return -1;
    }

    int type = rdf::type_name2which((char*)sqlite3_column_text(lc.stmt1, 0));
    this->db_pool_.put_connection(lc);
    return type;
  }

 private:
  DBConnectionPool    db_pool_;
};

inline TypeOfPtr 
create_type_of(std::string_view workspace_db_path)
{
  return std::make_shared<TypeOf>(workspace_db_path);
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
  using LookupTableMap = std::unordered_map<std::string, LookupTablePtr>;

  LookupSqlHelper() = delete;

  LookupSqlHelper(std::string_view workspace_db_path, std::string_view lookup_db_path) 
    : workspace_db_path_(workspace_db_path),
    workspace_db_(nullptr),
    lookup_db_path_(lookup_db_path),
    lookup_tbl_info_(),
    lookup_tbl_map_(),
    type_of_()
    {}

  /**
   * @brief Initialize the helper, open database connections
   */
  inline int initialize(rdf::RManager * rmgr)
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
    // open a connection to lookup_db to get max(__key__) during initialization
    sqlite3 * lkdb;
    err = sqlite3_open(this->lookup_db_path_.c_str(), &lkdb);
    if( err ) {
      LOG(ERROR) << "LookupSqlHelper::initialize: ERROR: Can't open lookup database: '" <<
        this->lookup_db_path_<<"' as lookup_db, error:" << sqlite3_errmsg(lkdb);
      return err;
    }
    for(auto const& info: this->lookup_tbl_info_) {
      auto l = create_lookup_table(rmgr, info.first, info.second, this->lookup_db_path_);
      this->lookup_tbl_map_.insert({info.second, l});
      int xerr = l->initialize(this->workspace_db_, lkdb);
      if(xerr) {
        err = xerr;
        LOG(ERROR) << "LookupSqlHelper::initialize: ERROR while initializing LookupTable: " << info.second;
      }
    }
    int err2 = 0;
    if(lkdb) {
      err2 = sqlite3_close_v2( lkdb );
      if ( err2 != SQLITE_OK ) {
        LOG(ERROR) << "ERROR while closing lookup_db connection, code: "<<err;
      }
    }
    if( err2 ) return err2;
    if( err ) return err;

    // Prepare the type_of struct for casting
    this->type_of_ = create_type_of(this->workspace_db_path_);
    this->type_of_->initialize(this->workspace_db_);

    // All good!
    return 0;
  }

  inline
  int lookup(ReteSession * rete_session, std::string const& lookup_tbl, std::string const& key, RDFTTYPE * out) const
  {
    auto itor = this->lookup_tbl_map_.find(lookup_tbl);
    if(itor == this->lookup_tbl_map_.end()) {
      LOG(ERROR) << "LookupSqlHelper::lookup: ERROR LookupTable not found: " << lookup_tbl;
      return -1;
    }
    return itor->second->lookup(rete_session, key, out);
  }

  inline
  int lookup_rand(ReteSession * rete_session, std::string const& lookup_tbl, RDFTTYPE * out) const
  {
    auto itor = this->lookup_tbl_map_.find(lookup_tbl);
    if(itor == this->lookup_tbl_map_.end()) {
      LOG(ERROR) << "LookupSqlHelper::lookup: ERROR LookupTable not found: " << lookup_tbl;
      return -1;
    }
    return itor->second->lookup_rand(rete_session, out);
  }

  inline
  int multi_lookup(ReteSession * rete_session, std::string const& lookup_tbl, std::string const& key, RDFTTYPE * out) const
  {
    auto itor = this->lookup_tbl_map_.find(lookup_tbl);
    if(itor == this->lookup_tbl_map_.end()) {
      LOG(ERROR) << "LookupSqlHelper::lookup: ERROR LookupTable not found: " << lookup_tbl;
      return -1;
    }
    return itor->second->multi_lookup(rete_session, key, out);
  }

  inline
  int multi_lookup_rand(ReteSession * rete_session, std::string const& lookup_tbl, RDFTTYPE * out) const
  {
    auto itor = this->lookup_tbl_map_.find(lookup_tbl);
    if(itor == this->lookup_tbl_map_.end()) {
      LOG(ERROR) << "LookupSqlHelper::lookup: ERROR LookupTable not found: " << lookup_tbl;
      return -1;
    }
    return itor->second->multi_lookup_rand(rete_session, out);
  }

  inline
  int type_of(ReteSession * rete_session, std::string const& data_property)
  {
    return this->type_of_->type_of(rete_session, data_property);
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
    if(this->workspace_db_) {
      err = sqlite3_close_v2( this->workspace_db_ );
      if ( err != SQLITE_OK ) {
        LOG(ERROR) << "LookupTable::terminate: ERROR while closing rete_db connection, code: "<<err;
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
    int err = sqlite3_prepare_v2( this->workspace_db_, sql, -1, &stmt, 0 );
    if ( err != SQLITE_OK ) {
      return err;
    }

    bool is_done = false;
    while(not is_done) {
      err = sqlite3_step( stmt );
      if ( err == SQLITE_DONE ) {
        is_done = true;
        continue;
      }
      if(err != SQLITE_ROW) {
        LOG(ERROR) << "ReteMetaStoreFactory::load_lookup_table_info: SQL error while load_workspace_control: " << err;
        return err;
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
  TypeOfPtr type_of_;
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
