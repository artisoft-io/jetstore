#ifndef JETS_RETE_RETE_META_STORE_FACTORY_H
#define JETS_RETE_RETE_META_STORE_FACTORY_H

#include <algorithm>
#include <cstring>
#include <string>
#include <memory>
#include <filesystem>
#include <unordered_map>

#include <glog/logging.h>
#include <unordered_map>

#include <pqxx/pqxx>
#include "sqlite3.h"

#include "jets/rdf/rdf_types.h"
#include "jets/rete/node_vertex.h"
#include "jets/rete/alpha_node.h"
#include "jets/rete/rete_meta_store.h"

// Factory class to create and configure ReteMetaStore objects
static int read_resources_cb(void *data, int argc, char **argv, char **azColName);
namespace jets::rete {
// //////////////////////////////////////////////////////////////////////////////////////
// ReteMetaStoreFactory class -- Class constructing instances of ReteMetaStore using jetstore_rete.db
// --------------------------------------------------------------------------------------
/**
 * @brief Factory class for ReteMetaStore, using jetrule_rete.db
 * 
 */
class ReteMetaStoreFactory {
 public:
  using ResourceLookup = std::unordered_map<int, rdf::r_index>;

  ReteMetaStoreFactory(std::string const& jetrule_rete_db)
    : jetrule_rete_db_(jetrule_rete_db), 
    meta_graph_(), 
    r_map_() 
  {
    // Open database -- check that db exists
    std::filesystem::path p(this->jetrule_rete_db_);
    //*
    std::cout << "Current path is " << std::filesystem::current_path() << std::endl;
    std::cout << "Absolute path is " << std::filesystem::absolute(p) << std::endl;
    std::cout << "jetrule_rete_db is " << p << std::endl;
    if(not std::filesystem::exists(p)) {
      LOG(ERROR) << "ReteMetaStoreFactory::create_rete_meta_store: ERROR: Invalid argument jetrule_rete_db: '" <<
        this->jetrule_rete_db_<<"' database does not exists.";
      RETE_EXCEPTION("ReteMetaStoreFactory::create_rete_meta_store: ERROR: Invalid argument jetrule_rete_db: '" <<
        this->jetrule_rete_db_<<"' database does not exists.");
    }
  }

  rdf::RDFGraph const*
  meta_graph()
  {
    return this->meta_graph_.get();
  }

  ReteMetaStorePtr
  create_rete_meta_store(std::string const& main_rule)
  {
    // Open database -- check that db exists
    std::filesystem::path p(this->jetrule_rete_db_);
    if(not std::filesystem::exists(p)) {
      LOG(ERROR) << "ReteMetaStoreFactory::create_rete_meta_store: ERROR: Invalid argument jetrule_rete_db: '" <<
        this->jetrule_rete_db_<<"' database does not exists.";
      return {};
    }

    sqlite3 *db;
    int err = 0;
    char * err_msg = 0;
    err = sqlite3_open(this->jetrule_rete_db_.c_str(), &db);
    if( err ) {
      LOG(ERROR) << "ReteMetaStoreFactory::create_rete_meta_store: ERROR: Can't open database: '" <<
        this->jetrule_rete_db_<<"', error:" << sqlite3_errmsg(db);
      return {};
    }

    std::cout<< "** Database Opened"  << std::endl;

    // Load all resources
    err = this->load_resources(db);
    if(err) return {};

    // load the rete config for main_rule


    return {};
  }

  int 
  read_resources_cb(int argc, char **argv, char **colnm)
  {
    int key = pqxx::from_string<int>(argv[0]);
    // int key = std::stoi(argv[0]);
    char * type =   argv[1];
    char * value =  argv[3];
    char * symbol = argv[4];

    if( strcmp(type, "var") == 0 ) return SQLITE_OK;

    //*
    std::cout<<"** read_resources_cb: "<<key<<", type "<<std::string(type)<<", value "<<(value?std::string(value):std::string("NULL"))<<", symbol "<<(symbol?std::string(symbol):std::string("NULL"))<<std::endl;

    if(not value and not symbol) {
      LOG(ERROR) << "ReteMetaStoreFactory::create_rete_meta_store: ERROR: read_resources_cb: resource has no value and no symbol!!";
      return SQLITE_ERROR;
    } 

    if( strcmp(type, "resource") == 0 ) {
      //*
      std::cout<<">> read_resources_cb: "<<key<<", GOT resource -- "<<std::string(type)<<", value "<<(value?std::string(value):std::string("NULL"))<<", symbol "<<(symbol?std::string(symbol):std::string("NULL"))<<std::endl;

      if(value) {
        auto item = this->r_map_.insert({key, this->meta_graph_->rmgr()->create_resource(value)});
        std::cout<<std::string("@@ inserted: ")<<item.first->second<<std::endl;

      } else {

        if( strcmp(symbol, "null") == 0) {
          this->r_map_.insert({key, this->meta_graph_->rmgr()->get_null()});

        } else if( strcmp(symbol, "create_uuid_resource()") == 0) {
          // this->r_map_.insert({key, this->meta_graph_->rmgr()->create_uuid_resource()});
        } else {
          LOG(ERROR) << "ReteMetaStoreFactory::create_rete_meta_store: ERROR: read_resources_cb: unknown symbol: "<<std::string(symbol);
          return SQLITE_ERROR;
        }
      }
      return SQLITE_OK;
    } 
    
    if( strcmp(type, "volatile_resource") == 0) {
      //*
      std::cout<<">> read_resources_cb: "<<key<<", GOT volatile_resource -- "<<std::string(type)<<", value "<<(value?std::string(value):std::string("NULL"))<<", symbol "<<(symbol?std::string(symbol):std::string("NULL"))<<std::endl;
      if(value) {
        std::string v("_0:");
        v =+ value;
        this->r_map_.insert({key, this->meta_graph_->rmgr()->create_resource(v)});
        return SQLITE_OK;
      }
      LOG(ERROR) << "ReteMetaStoreFactory::create_rete_meta_store: ERROR: read_resources_cb: volatile_resource with no value!";
      return SQLITE_ERROR;
    }

    // It's a literal

    return SQLITE_OK;
  }

 protected:
  int
  load_resources(sqlite3 *db)
  {
    if(not this->r_map_.empty()) return 0;  // already loaded
    int err = 0;
    char * err_msg = 0;

    if(not this->meta_graph_ ) this->meta_graph_ = rdf::create_rdf_graph();    
    auto const* sql = "SELECT * from resources";
    err = sqlite3_exec(db, sql, ::read_resources_cb, (void*)this, &err_msg);    
    if( err != SQLITE_OK ) {
      LOG(ERROR) << "ReteMetaStoreFactory::create_rete_meta_store: SQL error while reading resources: " << err_msg;
      sqlite3_free(err_msg);
      return -1;
    }
    return 0;
  }

 private:

  std::string jetrule_rete_db_;
  rdf::RDFGraphPtr meta_graph_;
  ResourceLookup r_map_;
};

} // namespace jets::rete
// ======================================================================================
// CALLBACK FUNCTIONS
// --------------------------------------------------------------------------------------
/**
 * @brief Callback for reading resources from sqlite3.exec
 * 
 * @param data      Data provided in the 4th argument of sqlite3_exec() 
 * @param argc      The number of columns in row 
 * @param argv      An array of strings representing fields in the row 
 * @param azColName An array of strings representing column names  
 * @return int 
 */
static int read_resources_cb(void *data, int argc, char **argv, char **colname) {
  jets::rete::ReteMetaStoreFactory * factory = nullptr;
  if (data) {
    factory = (jets::rete::ReteMetaStoreFactory *)data;
  }
  if(not factory) {
    LOG(ERROR) << "ReteMetaStoreFactory::create_rete_meta_store: ERROR: read_resources_cb have no factory!!";
    return SQLITE_ERROR;
  }
  if(argc < 8) {
    LOG(ERROR) << "ReteMetaStoreFactory::create_rete_meta_store: ERROR: read_resources_cb invalid nbr of columns!!";
    return SQLITE_ERROR;
  }
  return factory->read_resources_cb(argc, argv, colname);
}

#endif // JETS_RETE_RETE_META_STORE_FACTORY_H
