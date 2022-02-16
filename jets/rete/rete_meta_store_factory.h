#ifndef JETS_RETE_RETE_META_STORE_FACTORY_H
#define JETS_RETE_RETE_META_STORE_FACTORY_H

#include <cstdint>
#include <algorithm>
#include <cstring>
#include <string>
#include <memory>
#include <filesystem>
#include <unordered_map>

#include <glog/logging.h>
#include <unordered_map>

#include <pqxx/pqxx>
#include "beta_row_initializer.h"
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
  // key  ->  <var name, is_binded>
  using VariableLookup = std::unordered_map<int, std::tuple<std::string, bool>>;
  using MainRuleUriLookup = std::unordered_map<int, std::string>;
  using MetaStoreLookup = std::unordered_map<int, ReteMetaStorePtr>;

  ReteMetaStoreFactory()
    : jetrule_rete_db_(), 
    meta_graph_(), 
    r_map_(),
    v_map(),
    jr_map(),
    ms_map()
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
    return {};
  }

  int
  load_database(std::string const& jetrule_rete_db)
  {
    // Open database -- check that db exists
    this->jetrule_rete_db_ = jetrule_rete_db;
    std::filesystem::path p(this->jetrule_rete_db_);
    if(not std::filesystem::exists(p)) {
      LOG(ERROR) << "ReteMetaStoreFactory::create_rete_meta_store: ERROR: Invalid argument jetrule_rete_db: '" <<
        this->jetrule_rete_db_<<"' database does not exists.";
      return -1;
    }

    sqlite3 *db;
    int err = 0;
    err = sqlite3_open(this->jetrule_rete_db_.c_str(), &db);
    if( err ) {
      LOG(ERROR) << "ReteMetaStoreFactory::create_rete_meta_store: ERROR: Can't open database: '" <<
        this->jetrule_rete_db_<<"', error:" << sqlite3_errmsg(db);
      return err;
    }

    // Load all resources
    err = this->load_resources(db);
    if(err) return err;

    // load the rete config for main_rule
    this->load_workspace_control(db);

    // load RetaMetaStores configurations

    return -1;
  }

  int 
  read_resources_cb(int argc, char **argv, char **colnm)
  {
    int key = pqxx::from_string<int>(argv[0]);
    // int key = std::stoi(argv[0]);
    char * type   =  argv[1];
    char * id     =  argv[2];
    char * value  =  argv[3];
    char * symbol =  argv[4];
    char * binded =  argv[5];

    // Capture var as we'll need them for the rete_nodes
    if( strcmp(type, "var") == 0 ) {
      //                         v_map:   key -> (id, is_binded)
      this->v_map.insert({key, {std::string(id), pqxx::from_string<int>(binded)}});
      return SQLITE_OK;
    }

    if(not value and not symbol) {
      LOG(ERROR) << "ReteMetaStoreFactory::create_rete_meta_store: ERROR: read_resources_cb: resource has no value and no symbol!!";
      return SQLITE_ERROR;
    } 

    if( strcmp(type, "resource") == 0 ) {
      if(value) {
        
        // main case, use the value to create the resource
        this->r_map_.insert({key, this->meta_graph_->rmgr()->create_resource(value)});
      } else {

        // special case, use symbol or operator/function
        if( strcmp(symbol, "null") == 0) {
          this->r_map_.insert({key, this->meta_graph_->rmgr()->get_null()});

        } else if( strcmp(symbol, "create_uuid_resource()") == 0) {
          this->r_map_.insert({key, this->meta_graph_->rmgr()->create_uuid_resource()});

        } else {
          LOG(ERROR) << "ReteMetaStoreFactory::create_rete_meta_store: ERROR: read_resources_cb: unknown symbol: "<<std::string(symbol);
          return SQLITE_ERROR;
        }
      }
      return SQLITE_OK;
    } 
    
    if( strcmp(type, "volatile_resource") == 0) {
      if(value) {
        std::string v("_0:");
        v += value;
        this->r_map_.insert({key, this->meta_graph_->rmgr()->create_resource(v)});
        return SQLITE_OK;
      }
      LOG(ERROR) << "ReteMetaStoreFactory::create_rete_meta_store: ERROR: read_resources_cb: volatile_resource with no value!";
      return SQLITE_ERROR;
    }

    // It's a literal -- no symbol allowed
    if(not value) {
      LOG(ERROR) << "ReteMetaStoreFactory::create_rete_meta_store: ERROR: read_resources_cb: literal with no value!!";
      return SQLITE_ERROR;
    }
    
    if( strcmp(type, "int") == 0) {
      this->r_map_.insert({key, this->meta_graph_->rmgr()->create_literal(pqxx::from_string<int_fast32_t>(value))});
      return SQLITE_OK;
    }
    
    if( strcmp(type, "uint") == 0) {
      this->r_map_.insert({key, this->meta_graph_->rmgr()->create_literal(pqxx::from_string<uint_fast32_t>(value))});
      return SQLITE_OK;
    }
    
    if( strcmp(type, "long") == 0) {
      this->r_map_.insert({key, this->meta_graph_->rmgr()->create_literal(pqxx::from_string<int_fast64_t>(value))});
      return SQLITE_OK;
    }
    
    if( strcmp(type, "ulong") == 0) {
      this->r_map_.insert({key, this->meta_graph_->rmgr()->create_literal(pqxx::from_string<uint_fast64_t>(value))});
      return SQLITE_OK;
    }
    
    if( strcmp(type, "double") == 0) {
      this->r_map_.insert({key, this->meta_graph_->rmgr()->create_literal(pqxx::from_string<double>(value))});
      return SQLITE_OK;
    }
    
    if( strcmp(type, "text") == 0) {
      this->r_map_.insert({key, this->meta_graph_->rmgr()->create_literal(value)});
      return SQLITE_OK;
    }

    LOG(ERROR) << "ReteMetaStoreFactory::create_rete_meta_store: ERROR: unknown type: "<<std::string(type);
    return SQLITE_ERROR;
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

  int
  load_workspace_control(sqlite3 *db)
  {
    this->jr_map.clear();
    auto const* sql = "SELECT key, source_file_name from workspace_control WHERE is_main = 1";

    sqlite3_stmt* stmt;
    int res = sqlite3_prepare_v2( db, sql, -1, &stmt, 0 );
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
        LOG(ERROR) << "ReteMetaStoreFactory::create_rete_meta_store: SQL error while load_workspace_control: " << res;
        return res;
      }
      // Get the data out of the row and in the lookup map
      int key = sqlite3_column_int( stmt, 0 );
      std::string rule_uri((char const*)sqlite3_column_text( stmt, 1 ));
      this->jr_map.insert({key, rule_uri});
    }
    sqlite3_finalize( stmt );
    return 0;
  }

  int
  load_rete_nodes(sqlite3 *db)
  {
    // CREATE TABLE rete_nodes (
    //         vertex             INTEGER NOT NULL,
    //         type               STRING NOT NULL,
    //         subject_key        INTEGER,
    //         predicate_key      INTEGER,
    //         object_key         INTEGER,
    //         obj_expr_key       INTEGER,
    //         filter_expr_key    INTEGER,
    //         normalizedLabel    STRING,
    //         parent_vertex      INTEGER,
    //         beta_relation_vars STRING,
    //         pruned_var         STRING,
    //         source_file_key    INTEGER NOT NULL,
    //         PRIMARY KEY (vertex, type, source_file_key)
    //       )
    this->ms_map.clear();
    auto const* sql = "SELECT vertex, type, subject_key, predicate_key, object_key, "
                       "obj_expr_key, filter_expr_key, normalizedLabel, parent_vertex, beta_relation_vars, "
                       "pruned_var from workspace_control WHERE source_file_key is ? ORDER BY vertex ASC";

    sqlite3_stmt* stmt;
    int res = sqlite3_prepare_v2( db, sql, -1, &stmt, 0 );
    if ( res != SQLITE_OK ) {
      return res;
    }
    // Load each main rule file as a ReteMetaStore
    for(auto const& item: this->jr_map) {
      std::cout<< "Loading "<<item.second<<std::endl;
      int file_key = item.first;
      res = sqlite3_bind_int(stmt, 1, file_key);
      if ( res != SQLITE_OK ) {
        return res;
      }
      NodeVertexVector   node_vertexes;
      bool is_done = false;
      while(not is_done) {
        res = sqlite3_step( stmt );
        if ( res == SQLITE_DONE ) {
          is_done = true;
          continue;
        }
        if(res != SQLITE_ROW) {
          LOG(ERROR) << "ReteMetaStoreFactory::create_rete_meta_store: SQL error while load_workspace_control: " << res;
          return res;
        }
        // Get the data out of the row
        int vertex             = get_column_int_value( stmt, 0  );   //  INTEGER NOT NULL,
        int subject_key        = get_column_int_value( stmt, 2  );   //  INTEGER,
        int predicate_key      = get_column_int_value( stmt, 3  );   //  INTEGER,
        int object_key         = get_column_int_value( stmt, 4  );   //  INTEGER,
        int obj_expr_key       = get_column_int_value( stmt, 5  );   //  INTEGER,
        int filter_expr_key    = get_column_int_value( stmt, 6  );   //  INTEGER,
        int parent_vertex      = get_column_int_value( stmt, 8  );   //  INTEGER,
        
        // validation
        if(vertex<0 or parent_vertex<0) {
          LOG(ERROR) << "ReteMetaStoreFactory::create_rete_meta_store: "<<
            "Invalid NodeVertex in rete_db, got vertex: " << vertex << 
            ", parent_vertex: "<<parent_vertex;
          return -1;
        }

        // Check if we have the head_node
        if(vertex == 0) {
          node_vertexes.push_back(create_node_vertex(nullptr, 0, false, 0, {}, {}));
          continue;
        }

        std::string type              ((char const*)sqlite3_column_text( stmt, 1 ));   //  STRING NOT NULL,
        std::string normalizedLabel   ((char const*)sqlite3_column_text( stmt, 7 ));   //  STRING,
        std::string beta_relation_vars((char const*)sqlite3_column_text( stmt, 9 ));   //  STRING,
        std::string pruned_var        ((char const*)sqlite3_column_text( stmt, 10));   //  STRING,

        // Create Filter
        ExprBasePtr filter = create_expr(filter_expr_key);

        // Create BetaRowInitializer
        auto rowi = this->create_beta_row_initializer(beta_relation_vars);

        // Create the NodeVertex
        node_vertexes.push_back(create_node_vertex(node_vertexes[0].get(), 1, false, 20, {}, ri1));

      }
      // Create the ReteMetaStore
    }
    sqlite3_finalize( stmt );
    return 0;
  }

  ExprBasePtr
  create_expr(int expr_key)
  {
    return {};
  }

  BetaRowInitializerPtr
  create_beta_row_initializer(std::string const& beta_relation_vars)
  {
    return {};
  }

 private:

 inline int
 get_column_int_value(sqlite3_stmt * stmt, int col)
 {
   int res = sqlite3_column_type(stmt, col);
   if (res == SQLITE_NULL) return -1;
   if(res != SQLITE_INTEGER) {
    LOG(ERROR) << "ReteMetaStoreFactory::create_rete_meta_store: ERROR: "<<
      "get_column_value: Incorrect column type, expecting int, got" << res;
    RETE_EXCEPTION("ReteMetaStoreFactory::create_rete_meta_store: ERROR: "<<
      "get_column_value: Incorrect column type, expecting int, got" << res);
   }
   return sqlite3_column_int( stmt, col);
 }

  std::string jetrule_rete_db_;
  rdf::RDFGraphPtr meta_graph_;

  ResourceLookup r_map_;
  VariableLookup v_map;
  MainRuleUriLookup jr_map;
  MetaStoreLookup ms_map;
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
