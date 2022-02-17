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
#include "expr.h"
#include "sqlite3.h"

#include "jets/rdf/rdf_types.h"
#include "jets/rete/node_vertex.h"
#include "jets/rete/alpha_node.h"
#include "jets/rete/rete_meta_store.h"
#include "jets/rete/expr_operator_factory.h"

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
  using VariableLookup = std::unordered_map<int, std::pair<std::string, bool>>;
  using MainRuleUriLookup = std::unordered_map<int, std::string>;
  using MetaStoreLookup = std::unordered_map<int, ReteMetaStorePtr>;

  ReteMetaStoreFactory()
    : jetrule_rete_db_(), 
    meta_graph_(), 
    r_map_(),
    v_map_(),
    jr_map_(),
    ms_map_()
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
      //                         v_map_:   key -> (id, is_binded)
      this->v_map_.insert({key, {std::string(id), pqxx::from_string<int>(binded)}});
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
    this->jr_map_.clear();
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
      this->jr_map_.insert({key, rule_uri});
    }
    sqlite3_finalize( stmt );
    return 0;
  }

  int
  load_rete_nodes(sqlite3 *db)
  {
    // CREATE TABLE rete_nodes (
    //     0     vertex             INTEGER NOT NULL,
    //     1     type               STRING NOT NULL,
    //     2     subject_key        INTEGER,
    //     3     predicate_key      INTEGER,
    //     4     object_key         INTEGER,
    //     5     obj_expr_key       INTEGER,
    //     6     filter_expr_key    INTEGER,
    //     7     normalizedLabel    STRING,
    //     8     parent_vertex      INTEGER,
    //     9     beta_relation_vars STRING,
    //    10     pruned_var         STRING,
    //    11     source_file_key    INTEGER NOT NULL,
    //    12     is_negation        INTEGER,
    //    13     salience           INTEGER,
    //          PRIMARY KEY (vertex, type, source_file_key)
    //       )
    this->ms_map_.clear();
    auto const* sql = "SELECT * FROM rete_nodes "
                      "WHERE source_file_key is ? ORDER BY vertex ASC";
    sqlite3_stmt* stmt;
    int res = sqlite3_prepare_v2( db, sql, -1, &stmt, 0 );
    if ( res != SQLITE_OK ) {
      return res;
    }
    // Prepare the statement for expressions table
    auto const* expr_sql = "SELECT * FROM expressions WHERE key = ?";
    sqlite3_stmt* expr_stmt;
    res = sqlite3_prepare_v2( db, expr_sql, -1, &expr_stmt, 0 );
    if ( res != SQLITE_OK ) {
      goto cleanup_and_exit;
    }

    // Load each main rule file as a ReteMetaStore
    for(auto const& item: this->jr_map_) {
      std::cout<< "Loading "<<item.second<<std::endl;
      int file_key = item.first;
      res = sqlite3_bind_int(stmt, 1, file_key);
      if ( res != SQLITE_OK ) {
        goto cleanup_and_exit;
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
          LOG(ERROR) << "ReteMetaStoreFactory::create_rete_meta_store: " <<
            "SQL error while reading rete_nodes table: " << res;
          goto cleanup_and_exit;
        }
        // Get the data out of the row
        int vertex             = get_column_int_value( stmt, 0  );   //  INTEGER NOT NULL,
        int subject_key        = get_column_int_value( stmt, 2  );   //  INTEGER,
        int predicate_key      = get_column_int_value( stmt, 3  );   //  INTEGER,
        int object_key         = get_column_int_value( stmt, 4  );   //  INTEGER,
        int obj_expr_key       = get_column_int_value( stmt, 5  );   //  INTEGER,
        int filter_expr_key    = get_column_int_value( stmt, 6  );   //  INTEGER,
        int parent_vertex      = get_column_int_value( stmt, 8  );   //  INTEGER,
        int is_negation        = get_column_int_value( stmt, 12 );   //  INTEGER,
        int salience           = get_column_int_value( stmt, 13 );   //  INTEGER,
        
        // validation
        if(vertex<0 or parent_vertex<0) {
          LOG(ERROR) << "ReteMetaStoreFactory::create_rete_meta_store: "<<
            "Invalid NodeVertex in rete_db, got vertex: " << vertex << 
            ", parent_vertex: "<<parent_vertex;
          res = -1;
          goto cleanup_and_exit;
        }
        if(is_negation < 0) is_negation = 0;
        if(salience < 0) salience = 100;      // default value (should have been set in python)

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
        ExprBasePtr filter{};
        res = create_expr(expr_stmt, beta_relation_vars, filter_expr_key, filter);
        if(res != SQLITE_ROW) {
          LOG(ERROR) << "ReteMetaStoreFactory::create_rete_meta_store: " <<
            "SQL error while reading expressions table: " << res;
          goto cleanup_and_exit;
        }

        // Create BetaRowInitializer
        auto rowi = this->create_beta_row_initializer(beta_relation_vars);

        // Create the NodeVertex
        node_vertexes.push_back(
          create_node_vertex(node_vertexes[parent_vertex].get(), vertex, 
            is_negation, salience, filter, rowi));

      }
      // Create the ReteMetaStore
    }

    cleanup_and_exit:
      if(stmt) sqlite3_finalize( stmt );
      if(expr_stmt) sqlite3_finalize( expr_stmt );
      return res;
  }

  int
  create_expr(sqlite3_stmt* expr_stmt, std::string const& brv, int expr_key, ExprBasePtr & expr)
  {
    // CREATE TABLE IF NOT EXISTS expressions (
    //   key              0  INTEGER PRIMARY KEY,
    //   type             1  STRING NOT NULL,
    //   arg0_key         2  INTEGER,
    //   arg1_key         3  INTEGER,
    //   arg2_key         4  INTEGER,
    //   arg3_key         5  INTEGER,
    //   arg4_key         6  INTEGER,
    //   arg5_key         7  INTEGER,
    //   op               8  STRING,
    //   source_file_key  9  INTEGER NOT NULL
    // );
    if(expr_key < 0) return SQLITE_OK;

    int res = sqlite3_bind_int(expr_stmt, 1, expr_key);
    if( res != SQLITE_OK ) return res;

    res = sqlite3_step( expr_stmt );
    if(res != SQLITE_ROW) return res;

    char const* type = (char const*)sqlite3_column_text( expr_stmt, 1  );  
    int arg0_key     = get_column_int_value( expr_stmt, 2  );   //  INTEGER,
    int arg1_key     = get_column_int_value( expr_stmt, 3  );   //  INTEGER,
    int arg2_key     = get_column_int_value( expr_stmt, 4  );   //  INTEGER,
    int arg3_key     = get_column_int_value( expr_stmt, 5  );   //  INTEGER,
    int arg4_key     = get_column_int_value( expr_stmt, 6  );   //  INTEGER,
    int arg5_key     = get_column_int_value( expr_stmt, 7  );   //  INTEGER,
    char const* op   = (char const*)sqlite3_column_text( expr_stmt, 8  );  
    if(not type) return -1;

    if( strcmp(type, "binary") == 0) {
      if(not op) return -1;
      if(arg0_key<0 or arg1_key<0) return -1;

      ExprBasePtr lhs{}, rhs{};
      res = this->create_expr(expr_stmt, brv, arg0_key, lhs);
      if( res != SQLITE_OK ) return res;
      res = this->create_expr(expr_stmt, brv, arg1_key, rhs);
      if( res != SQLITE_OK ) return res;

      expr = create_binary_expr(lhs, op, rhs);
      return SQLITE_OK;

    } else if( strcmp(type, "unary") == 0) {
      if(not op) return -1;
      if(arg0_key<0) return -1;

      ExprBasePtr arg{};
      res = this->create_expr(expr_stmt, brv, arg0_key, arg);
      if( res != SQLITE_OK ) return res;

      expr = create_unary_expr(op, arg);
      return SQLITE_OK;

    } else if( strcmp(type, "function") == 0) {
    } else if( strcmp(type, "resource") == 0) {
      if(arg0_key<0) return -1;
      auto itor = this->r_map_.find(arg0_key);
      if(itor == this->r_map_.end()) return -1;
      auto r = itor->second;
      expr = create_expr_cst(*r);
      return SQLITE_OK;

    } else if( strcmp(type, "var") == 0) {
      if(arg0_key<0) return -1;
      auto itor = this->v_map_.find(arg0_key);
      if(itor == this->v_map_.end()) return -1;
      auto var = itor->second.first;
      // find the pos of var in brv (beta_relation_variables)
      // get the array pos by counting ','
      std::size_t pos = brv.find(var);
      if (pos == std::string::npos) return -1;
      size_t n = std::count(brv.begin(), brv.begin()+pos, ',');
      expr = create_expr_binded_var(n);
      return SQLITE_OK;

    } else {
      // Unknown type
      return -1;
    }

    return SQLITE_OK;
  }

  BetaRowInitializerPtr
  create_beta_row_initializer(std::string const& beta_relation_vars)
  {
    // // beta_relation_vars format: "?x1,?x2,?x3"
    // // Iterate over the var (?x1 ?x2 ...) using ',' as delimiter
    // std::size_t pos = 0;
    // while(pos != std::string::npos) {
    //   std::size_t c = beta_relation_vars.find(',', pos);
    //   std::string var;
    //   if(c == std::string::npos) {
    //     var = beta_relation_vars.substr(pos);
    //   } else {
    //     var = beta_relation_vars.substr(pos, c);
    //     c++; // so that the next var is not prepended with ','
    //   }
    //   // lookup var to see if it's a binded variable

    //   pos = c;
    // }

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
  VariableLookup v_map_;
  MainRuleUriLookup jr_map_;
  MetaStoreLookup ms_map_;
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
