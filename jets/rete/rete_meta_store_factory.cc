

#include <iostream>
#include <ostream>
#include <string>
#include <string_view>

#include "beta_row_initializer.h"
#include "../rete/rete_meta_store_factory.h"
#include "node_vertex.h"

namespace jets::rete {


ReteMetaStoreFactory::ReteMetaStoreFactory()
  : jetrule_rete_db_(), 
  meta_graph_(), 
  r_map_(),
  v_map_(),
  jr_map_(),
  ms_map_(),
  rs_map_(),
  db_(nullptr),
  node_vertexes_stmt_(nullptr),
  alpha_nodes_stmt_(nullptr),
  expr_stmt_(nullptr),
  br_stmt_(nullptr)
{}


int
ReteMetaStoreFactory::load_database(std::string const& jetrule_rete_db)
{
  //*
  // VLOG(1) << "Current path is " << std::filesystem::current_path() << std::endl;
  std::cout << "load_database: Current path is " << std::filesystem::current_path() << std::endl;
  // Open database -- check that db exists
  this->jetrule_rete_db_ = jetrule_rete_db;
  std::filesystem::path p(this->jetrule_rete_db_);
  if(not std::filesystem::exists(p)) {
    LOG(ERROR) << "ReteMetaStoreFactory::create_rete_meta_store: ERROR: Invalid argument jetrule_rete_db: '" <<
      this->jetrule_rete_db_<<"' database does not exists.";
    std::cout << "ReteMetaStoreFactory::create_rete_meta_store: ERROR: Invalid argument jetrule_rete_db: '" <<
      this->jetrule_rete_db_<<"' database does not exists."<<std::endl;
    return -1;
  }
  std::cout << "*** ReteMetaStoreFactory::create_rete_meta_store: calling open on sqlite3 for '" <<
    this->jetrule_rete_db_<<"' ..."<<std::endl;

  int err = 0;
  err = sqlite3_open(this->jetrule_rete_db_.c_str(), &this->db_);
  std::cout << "*** ReteMetaStoreFactory::create_rete_meta_store: sqlite3_open called, ret code "<<err<<std::endl;
  if( err ) {
    std::cout << "***>>> ReteMetaStoreFactory::create_rete_meta_store: sqlite3_open called ERROR '" << sqlite3_errmsg(this->db_) <<std::endl;
    LOG(ERROR) << "ReteMetaStoreFactory::create_rete_meta_store: ERROR: Can't open database: '" <<
      this->jetrule_rete_db_<<"', error:" << sqlite3_errmsg(this->db_);
    return err;
  }

  // Load all resources
  err = this->load_resources();
  if(err) return err;

  // load the rete config for main_rule
  this->load_workspace_control();

  // load RetaMetaStores configurations
  this->ms_map_.clear();
  // Prepared statement
  // --------------------------------------------------------------
  // Prepared statement for MetaStore node vertexes
  auto const* node_vertexes_sql = "SELECT * FROM rete_nodes "
                    "WHERE source_file_key is ? AND type is 'antecedent' ORDER BY vertex ASC";
  int res = sqlite3_prepare_v2( this->db_, node_vertexes_sql, -1, &this->node_vertexes_stmt_, 0 );
  if ( res != SQLITE_OK ) {
    return res;
  }
  // Prepared statement for MetaStore alpha nodes
  auto const* alpha_nodes_sql = "SELECT * FROM rete_nodes "
                    "WHERE source_file_key is ? ORDER BY key ASC";
  res = sqlite3_prepare_v2( this->db_, alpha_nodes_sql, -1, &this->alpha_nodes_stmt_, 0 );
  if ( res != SQLITE_OK ) {
    return res;
  }
  // Prepare the statement for expressions table
  auto const* expr_sql = "SELECT * FROM expressions WHERE key = ?";
  res = sqlite3_prepare_v2( this->db_, expr_sql, -1, &this->expr_stmt_, 0 );
  if ( res != SQLITE_OK ) {
    return res;
  }
  // Prepare the statement for beta_row_config table
  auto const* br_sql = 
    "SELECT * FROM beta_row_config WHERE vertex = ? AND source_file_key = ? ORDER BY seq ASC";
  res = sqlite3_prepare_v2( this->db_, br_sql, -1, &this->br_stmt_, 0 );
  if ( res != SQLITE_OK ) {
    return res;
  }

  // Load each main rule file as a ReteMetaStore
  for(auto const& item: this->jr_map_) {
    VLOG(1)<< "Loading file key: "<<item.second;
    int file_key = item.second;

    //*
    VLOG(1) << "Loading vertexes for file_key "<< file_key;

    // Load the node_vertexes
    NodeVertexVector node_vertexes;
    res = this->load_node_vertexes(file_key, node_vertexes);
    if ( res != SQLITE_OK ) {
      return res;
    }

    //*
    VLOG(1) << "Loading alpha nodes for file_key "<< file_key;

    // Load the alpha nodes
    AlphaNodeVector alpha_nodes;
    res = this->load_alpha_nodes(file_key, node_vertexes, alpha_nodes);
    if ( res != SQLITE_OK ) {
      return res;
    }

    // Create the ReteMetaStore
    // create & initalize the meta store
    auto rete_meta_store = rete::create_rete_meta_store(this->meta_graph_, alpha_nodes, node_vertexes);
    rete_meta_store->initialize();
    this->ms_map_.insert({file_key, rete_meta_store});
  }

  // All good!, release the stmts and db connection
  // VLOG(1)<< "All Done! Contains "<<this->r_map_.size()<<" resource definitions";
  std::cout<< "All Done! Contains "<<this->r_map_.size()<<" resource definitions"<<std::endl;;
  return this->reset();
}

int 
ReteMetaStoreFactory::read_resources_cb(int argc, char **argv, char **colnm)
{
  // resources table
  // key              0  INTEGER PRIMARY KEY,
  // type             1  STRING NOT NULL,
  // id               2  STRING,
  // value            3  STRING,
  // symbol           4  STRING,
  // is_binded        5  BOOL,     -- for var type only
  // inline           6  BOOL,
  // source_file_key  7  INTEGER NOT NULL,
  // vertex           8  INTEGER,  -- for var type only, var for vertex
  // row_pos          9  INTEGER   -- for var type only, pos in beta row
  //
  int key = std::stoi(argv[0]);
  char * type     =  argv[1];
  char * id       =  argv[2];
  char * value    =  argv[3];
  char * symbol   =  argv[4];
  char * binded   =  argv[5];
  char * vx       =  argv[8];
  char * pos      =  argv[9];

  // Capture var as we'll need them for the rete_nodes
  if( strcmp(type, "var") == 0 ) {
    bool is_binded = std::stoi(binded);
    int vertex = std::stoi(vx);
    int row_pos = 0;
    if(pos) row_pos = std::stoi(pos);
    this->v_map_.insert({key, var_info(id, is_binded, vertex, row_pos)});
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
    this->r_map_.insert({key, this->meta_graph_->rmgr()->create_literal(std::stoi(value))});
    return SQLITE_OK;
  }
  
  if( strcmp(type, "uint") == 0) {
    auto v = std::stoul(value);
    std::uint32_t u = v;
    if(u != v) {
      LOG(ERROR) << "ReteMetaStoreFactory::create_rete_meta_store: ERROR: unsignd int overflow, use a unsigned long literal for resource with id: "<<(id?std::string(id):"NULL");
      return SQLITE_ERROR;
    }
    this->r_map_.insert({key, this->meta_graph_->rmgr()->create_literal(u)});
    return SQLITE_OK;
  }
  
  if( strcmp(type, "long") == 0) {
    this->r_map_.insert({key, this->meta_graph_->rmgr()->create_literal(std::stol(value))});
    return SQLITE_OK;
  }
  
  if( strcmp(type, "ulong") == 0) {
    this->r_map_.insert({key, this->meta_graph_->rmgr()->create_literal(std::stoul(value))});
    return SQLITE_OK;
  }
  
  if( strcmp(type, "double") == 0) {
    this->r_map_.insert({key, this->meta_graph_->rmgr()->create_literal(std::stod(value))});
    return SQLITE_OK;
  }
  
  if( strcmp(type, "text") == 0) {
    this->r_map_.insert({key, this->meta_graph_->rmgr()->create_literal(value)});
    return SQLITE_OK;
  }

  LOG(ERROR) << "ReteMetaStoreFactory::create_rete_meta_store: ERROR: unknown type: "<<std::string(type);
  return SQLITE_ERROR;
}

int
ReteMetaStoreFactory::load_node_vertexes(int file_key, NodeVertexVector & node_vertexes)
{
  // CREATE TABLE rete_nodes (
  //     0  key                INTEGER NOT NULL,
  //     1  vertex             INTEGER NOT NULL,
  //     2  type               STRING NOT NULL,
  //     3  subject_key        INTEGER,
  //     4  predicate_key      INTEGER,
  //     5  object_key         INTEGER,
  //     6  obj_expr_key       INTEGER,
  //     7  filter_expr_key    INTEGER,
  //     8  normalizedLabel    STRING,
  //     9  parent_vertex      INTEGER,
  //    10  source_file_key    STRING,
  //    11  is_negation        STRING,
  //    12  salience           INTEGER NOT NULL,
  //    13  consequent_seq     INTEGER,
  //  UNIQUE (vertex, type, consequent_seq, source_file_key)
  //       )
  int res = sqlite3_reset(this->node_vertexes_stmt_);
  if( res != SQLITE_OK ) return res;

  res = sqlite3_bind_int(this->node_vertexes_stmt_, 1, file_key);
  if ( res != SQLITE_OK ) {
    return res;
  }
  // The query retains only antecedent node, add head node manually
  // Also make sure node_vertexes is clean
  node_vertexes.clear();
  node_vertexes.push_back(create_node_vertex(nullptr, 0, 0, false, 0, {}, "(* * *)", {}));

  bool is_done = false;
  while(not is_done) {
    res = sqlite3_step( this->node_vertexes_stmt_ );
    if ( res == SQLITE_DONE ) {
      is_done = true;
      continue;
    }
    if(res != SQLITE_ROW) {
      LOG(ERROR) << "ReteMetaStoreFactory::create_rete_meta_store: " <<
        "SQL error while reading rete_nodes table: " << res;
      return res;
    }
    // Get the data out of the row
    int key                = get_column_int_value( this->node_vertexes_stmt_, 0  );   //  INTEGER NOT NULL,
    int vertex             = get_column_int_value( this->node_vertexes_stmt_, 1  );   //  INTEGER NOT NULL,
    int filter_expr_key    = get_column_int_value( this->node_vertexes_stmt_, 7  );   //  INTEGER,
    int parent_vertex      = get_column_int_value( this->node_vertexes_stmt_, 9  );   //  INTEGER,
    int is_negation        = get_column_int_value( this->node_vertexes_stmt_, 11 );   //  INTEGER,
    int salience           = get_column_int_value( this->node_vertexes_stmt_, 12 );   //  INTEGER,

    //*
    VLOG(1) << "Loading vertex: "<< vertex <<", with key "<<key << std::endl;
    
    // validation
    if(vertex<0 or parent_vertex<0) {
      LOG(ERROR) << "ReteMetaStoreFactory::create_rete_meta_store: "<<
        "Invalid NodeVertex in rete_db, got vertex: " << vertex << 
        ", parent_vertex: "<<parent_vertex;
      res = -1;
      return res;
    }
    if(is_negation < 0) is_negation = 0;
    if(salience < 0) salience = 100;      // default value (should have been set in python)

    // Check if we have the head_node
    if(vertex == 0) {
      // got it covered already
      continue;
    }

    std::string type     ((char const*)sqlite3_column_text( this->node_vertexes_stmt_, 2 ));   //  STRING NOT NULL,
    char const* nlabel = (char const*)sqlite3_column_text( this->node_vertexes_stmt_, 8 );
    std::string_view normalized_label("");
    if(nlabel) normalized_label = nlabel;

    //*
    if(filter_expr_key >= 0) VLOG(1) << "Creating filter with key: "<< filter_expr_key << std::endl;

    // Create Filter
    ExprBasePtr filter{};
    res = this->create_expr(filter_expr_key, filter);
    if(res != SQLITE_OK) {
      LOG(ERROR) << "ReteMetaStoreFactory::create_rete_meta_store: " <<
        "SQL error while reading expressions table: " << res;
      return res;
    }

    //*
    VLOG(1) << "Creating beta row initializer, vertex "<< vertex << std::endl;

    // Create BetaRowInitializer
    // load all seq for (vertex, file_key)
    BetaRowInitializerPtr beta_row_initializer;
    res = this->create_beta_row_initializer(vertex, file_key, beta_row_initializer);
    if(res != 0) {
      LOG(ERROR) << "ReteMetaStoreFactory::create_rete_meta_store: " <<
        "Error while loading beta row initializer for vertex " << vertex << 
        ", file key "<<file_key;
      return res;
    }

    //*
    VLOG(1) << "Creating NodeVertex @ "<<key<<", vertex "<< vertex << ", parent vertex "<< parent_vertex << std::endl;
    if(node_vertexes.size()<1) {
      LOG(ERROR) << "ReteMetaStoreFactory::create_rete_meta_store: " <<
        "Error node_vertexes.size()<1 for vertex " << vertex << 
        ", parent vertex "<<parent_vertex<<", file key "<<file_key;
      return -1;
    }
    auto parent = node_vertexes.at(parent_vertex);
    b_index parent_index = node_vertexes[parent_vertex].get();

    // Create the NodeVertex
    node_vertexes.push_back(
      create_node_vertex(parent_index, key, vertex, 
        is_negation, salience, filter, normalized_label, beta_row_initializer));
  }
  //*
  VLOG(1) << "Got "<<node_vertexes.size()<<" NodeVertexes " << std::endl;
  return SQLITE_OK;
}

int
ReteMetaStoreFactory::load_alpha_nodes(int file_key, NodeVertexVector const& node_vertexes, AlphaNodeVector & alpha_nodes)
{
  // CREATE TABLE rete_nodes (
  //     0  key                INTEGER NOT NULL,
  //     1  vertex             INTEGER NOT NULL,
  //     2  type               STRING NOT NULL,
  //     3  subject_key        INTEGER,
  //     4  predicate_key      INTEGER,
  //     5  object_key         INTEGER,
  //     6  obj_expr_key       INTEGER,
  //     7  filter_expr_key    INTEGER,
  //     8  normalizedLabel    STRING,
  //     9  parent_vertex      INTEGER,
  //    10  source_file_key    STRING,
  //    11  is_negation        STRING,
  //    12  salience           INTEGER NOT NULL,
  //    13  consequent_seq     INTEGER,
  //  UNIQUE (vertex, type, consequent_seq, source_file_key)
  //       )
  int res = sqlite3_reset(this->alpha_nodes_stmt_);
  if( res != SQLITE_OK ) return res;

  res = sqlite3_bind_int(this->alpha_nodes_stmt_, 1, file_key);
  if ( res != SQLITE_OK ) {
    return res;
  }
  // The query retains only antecedent node, add head node manually
  // Also make sure alpha_nodes is clean
  alpha_nodes.clear();
  alpha_nodes.push_back(create_alpha_node<F_var, F_var, F_var>(node_vertexes[0].get(), 0, true, "(* * *)",
      F_var("*"), F_var("*"), F_var("*") ));

  bool is_done = false;
  while(not is_done) {
    res = sqlite3_step( this->alpha_nodes_stmt_ );
    if ( res == SQLITE_DONE ) {
      is_done = true;
      continue;
    }
    if(res != SQLITE_ROW) {
      LOG(ERROR) << "ReteMetaStoreFactory::create_rete_meta_store: " <<
        "SQL error while reading rete_nodes table: " << res;
      return res;
    }
    // Get the data out of the row
    int key                = get_column_int_value( this->alpha_nodes_stmt_,      0  );   //  INTEGER NOT NULL,
    int vertex             = get_column_int_value( this->alpha_nodes_stmt_,      1  );   //  INTEGER NOT NULL,
    std::string type ((char const*)sqlite3_column_text( this->alpha_nodes_stmt_, 2 ));   //  STRING NOT NULL,
    int subject_key        = get_column_int_value( this->alpha_nodes_stmt_,      3  );   //  INTEGER,
    int predicate_key      = get_column_int_value( this->alpha_nodes_stmt_,      4  );   //  INTEGER,
    int object_key         = get_column_int_value( this->alpha_nodes_stmt_,      5  );   //  INTEGER,
    int obj_expr_key       = get_column_int_value( this->alpha_nodes_stmt_,      6  );   //  INTEGER,
    char const* nlabel = (char const*)sqlite3_column_text( this->alpha_nodes_stmt_, 8 );
    std::string_view normalized_label("");
    if(nlabel) normalized_label = nlabel;

    // Check if we have the head_node
    if(vertex == 0) {
      // got it covered already
      continue;
    }

    bool is_antecedent = false;
    if( type == "antecedent") is_antecedent = true;

    // Prepare to create the AlphaNode
    if(subject_key < 0 or predicate_key < 0) {
      LOG(ERROR) << "ERROR subject_key or predicate_key is null in load alpha_node";
      return -1;
    }

    VLOG(1)<<"Creating AlphaNode: "<<type<<" is_antecedent?"<<is_antecedent<<" ("<<subject_key<<", "<<predicate_key<<", "<<object_key<<")";

    auto fu = this->create_func_factory(subject_key);
    auto fv = this->create_func_factory(predicate_key);
    FuncFactoryPtr fw;      
    if(object_key >= 0) {
      fw = this->create_func_factory(object_key);
    }  else if(obj_expr_key >= 0) {
      fw = this->create_func_expr_factory(obj_expr_key);
    } else {
      LOG(ERROR) << "ERROR object_key and object_expr_key are null in load alpha_node";
      return -1;
    }

    // Create the AlphaNode
    if(fu->get_func_type() == f_cst_e) {
      if(fv->get_func_type() == f_cst_e) {
        if(fw->get_func_type() == f_cst_e) {
          alpha_nodes.push_back(create_alpha_node<F_cst, F_cst, F_cst>(node_vertexes[vertex].get(), key, is_antecedent, normalized_label, 
            fu->get_f_cst(), fv->get_f_cst(), fw->get_f_cst()));
        } else if(fw->get_func_type() == f_binded_e) {
          alpha_nodes.push_back(create_alpha_node<F_cst, F_cst, F_binded>(node_vertexes[vertex].get(), key, is_antecedent, normalized_label, 
            fu->get_f_cst(), fv->get_f_cst(), fw->get_f_binded()));
        } else if(fw->get_func_type() == f_var_e) {
          alpha_nodes.push_back(create_alpha_node<F_cst, F_cst, F_var>(node_vertexes[vertex].get(), key, is_antecedent, normalized_label, 
            fu->get_f_cst(), fv->get_f_cst(), fw->get_f_var()));
        } else if(fw->get_func_type() == f_expr_e) {
          alpha_nodes.push_back(create_alpha_node<F_cst, F_cst, F_expr>(node_vertexes[vertex].get(), key, is_antecedent, normalized_label, 
            fu->get_f_cst(), fv->get_f_cst(), fw->get_f_expr()));
        } else {
          LOG(ERROR) << "ERROR Create the AlphaNode fw is uknown type";
          return -1;
        }
      } else if (fv->get_func_type() == f_binded_e) {
        if(fw->get_func_type() == f_cst_e) {
          alpha_nodes.push_back(create_alpha_node<F_cst, F_binded, F_cst>(node_vertexes[vertex].get(), key, is_antecedent, normalized_label, 
            fu->get_f_cst(), fv->get_f_binded(), fw->get_f_cst()));
        } else if(fw->get_func_type() == f_binded_e) {
          alpha_nodes.push_back(create_alpha_node<F_cst, F_binded, F_binded>(node_vertexes[vertex].get(), key, is_antecedent, normalized_label, 
            fu->get_f_cst(), fv->get_f_binded(), fw->get_f_binded()));
        } else if(fw->get_func_type() == f_var_e) {
          alpha_nodes.push_back(create_alpha_node<F_cst, F_binded, F_var>(node_vertexes[vertex].get(), key, is_antecedent, normalized_label, 
            fu->get_f_cst(), fv->get_f_binded(), fw->get_f_var()));
        } else if(fw->get_func_type() == f_expr_e) {
          alpha_nodes.push_back(create_alpha_node<F_cst, F_binded, F_expr>(node_vertexes[vertex].get(), key, is_antecedent, normalized_label, 
            fu->get_f_cst(), fv->get_f_binded(), fw->get_f_expr()));
        } else {
          LOG(ERROR) << "ERROR Create the AlphaNode fw is uknown type";
          return -1;
        }

      } else if (fv->get_func_type() == f_var_e) {
        if(fw->get_func_type() == f_cst_e) {
          alpha_nodes.push_back(create_alpha_node<F_cst, F_var, F_cst>(node_vertexes[vertex].get(), key, is_antecedent, normalized_label, 
            fu->get_f_cst(), fv->get_f_var(), fw->get_f_cst()));
        } else if(fw->get_func_type() == f_binded_e) {
          alpha_nodes.push_back(create_alpha_node<F_cst, F_var, F_binded>(node_vertexes[vertex].get(), key, is_antecedent, normalized_label, 
            fu->get_f_cst(), fv->get_f_var(), fw->get_f_binded()));
        } else if(fw->get_func_type() == f_var_e) {
          alpha_nodes.push_back(create_alpha_node<F_cst, F_var, F_var>(node_vertexes[vertex].get(), key, is_antecedent, normalized_label, 
            fu->get_f_cst(), fv->get_f_var(), fw->get_f_var()));
        } else if(fw->get_func_type() == f_expr_e) {
          alpha_nodes.push_back(create_alpha_node<F_cst, F_var, F_expr>(node_vertexes[vertex].get(), key, is_antecedent, normalized_label, 
            fu->get_f_cst(), fv->get_f_var(), fw->get_f_expr()));
        } else {
          LOG(ERROR) << "ERROR Create the AlphaNode fw is uknown type";
          return -1;
        }

      } else {
        LOG(ERROR) << "ERROR Create the AlphaNode fv is uknown type";
        return -1;
      }
    } else if(fu->get_func_type() == f_binded_e) {
      if(fv->get_func_type() == f_cst_e) {
        if(fw->get_func_type() == f_cst_e) {
          alpha_nodes.push_back(create_alpha_node<F_binded, F_cst, F_cst>(node_vertexes[vertex].get(), key, is_antecedent, normalized_label, 
            fu->get_f_binded(), fv->get_f_cst(), fw->get_f_cst()));
        } else if(fw->get_func_type() == f_binded_e) {
          alpha_nodes.push_back(create_alpha_node<F_binded, F_cst, F_binded>(node_vertexes[vertex].get(), key, is_antecedent, normalized_label, 
            fu->get_f_binded(), fv->get_f_cst(), fw->get_f_binded()));
        } else if(fw->get_func_type() == f_var_e) {
          alpha_nodes.push_back(create_alpha_node<F_binded, F_cst, F_var>(node_vertexes[vertex].get(), key, is_antecedent, normalized_label, 
            fu->get_f_binded(), fv->get_f_cst(), fw->get_f_var()));
        } else if(fw->get_func_type() == f_expr_e) {
          alpha_nodes.push_back(create_alpha_node<F_binded, F_cst, F_expr>(node_vertexes[vertex].get(), key, is_antecedent, normalized_label, 
            fu->get_f_binded(), fv->get_f_cst(), fw->get_f_expr()));
        } else {
          LOG(ERROR) << "ERROR Create the AlphaNode fw is uknown type";
          return -1;
        }
      } else if (fv->get_func_type() == f_binded_e) {
        if(fw->get_func_type() == f_cst_e) {
          alpha_nodes.push_back(create_alpha_node<F_binded, F_binded, F_cst>(node_vertexes[vertex].get(), key, is_antecedent, normalized_label, 
            fu->get_f_binded(), fv->get_f_binded(), fw->get_f_cst()));
        } else if(fw->get_func_type() == f_binded_e) {
          alpha_nodes.push_back(create_alpha_node<F_binded, F_binded, F_binded>(node_vertexes[vertex].get(), key, is_antecedent, normalized_label, 
            fu->get_f_binded(), fv->get_f_binded(), fw->get_f_binded()));
        } else if(fw->get_func_type() == f_var_e) {
          alpha_nodes.push_back(create_alpha_node<F_binded, F_binded, F_var>(node_vertexes[vertex].get(), key, is_antecedent, normalized_label, 
            fu->get_f_binded(), fv->get_f_binded(), fw->get_f_var()));
        } else if(fw->get_func_type() == f_expr_e) {
          alpha_nodes.push_back(create_alpha_node<F_binded, F_binded, F_expr>(node_vertexes[vertex].get(), key, is_antecedent, normalized_label, 
            fu->get_f_binded(), fv->get_f_binded(), fw->get_f_expr()));
        } else {
          LOG(ERROR) << "ERROR Create the AlphaNode fw is uknown type";
          return -1;
        }

      } else if (fv->get_func_type() == f_var_e) {
        if(fw->get_func_type() == f_cst_e) {
          alpha_nodes.push_back(create_alpha_node<F_binded, F_var, F_cst>(node_vertexes[vertex].get(), key, is_antecedent, normalized_label, 
            fu->get_f_binded(), fv->get_f_var(), fw->get_f_cst()));
        } else if(fw->get_func_type() == f_binded_e) {
          alpha_nodes.push_back(create_alpha_node<F_binded, F_var, F_binded>(node_vertexes[vertex].get(), key, is_antecedent, normalized_label, 
            fu->get_f_binded(), fv->get_f_var(), fw->get_f_binded()));
        } else if(fw->get_func_type() == f_var_e) {
          alpha_nodes.push_back(create_alpha_node<F_binded, F_var, F_var>(node_vertexes[vertex].get(), key, is_antecedent, normalized_label, 
            fu->get_f_binded(), fv->get_f_var(), fw->get_f_var()));
        } else if(fw->get_func_type() == f_expr_e) {
          alpha_nodes.push_back(create_alpha_node<F_binded, F_var, F_expr>(node_vertexes[vertex].get(), key, is_antecedent, normalized_label, 
            fu->get_f_binded(), fv->get_f_var(), fw->get_f_expr()));
        } else {
          LOG(ERROR) << "ERROR Create the AlphaNode fw is uknown type";
          return -1;
        }

      } else {
        LOG(ERROR) << "ERROR Create the AlphaNode fv is uknown type";
        return -1;
      }

    } else if(fu->get_func_type() == f_var_e) {
      if(fv->get_func_type() == f_cst_e) {
        if(fw->get_func_type() == f_cst_e) {
          alpha_nodes.push_back(create_alpha_node<F_var, F_cst, F_cst>(node_vertexes[vertex].get(), key, is_antecedent, normalized_label, 
            fu->get_f_var(), fv->get_f_cst(), fw->get_f_cst()));
        } else if(fw->get_func_type() == f_binded_e) {
          alpha_nodes.push_back(create_alpha_node<F_var, F_cst, F_binded>(node_vertexes[vertex].get(), key, is_antecedent, normalized_label, 
            fu->get_f_var(), fv->get_f_cst(), fw->get_f_binded()));
        } else if(fw->get_func_type() == f_var_e) {
          alpha_nodes.push_back(create_alpha_node<F_var, F_cst, F_var>(node_vertexes[vertex].get(), key, is_antecedent, normalized_label, 
            fu->get_f_var(), fv->get_f_cst(), fw->get_f_var()));
        } else if(fw->get_func_type() == f_expr_e) {
          alpha_nodes.push_back(create_alpha_node<F_var, F_cst, F_expr>(node_vertexes[vertex].get(), key, is_antecedent, normalized_label, 
            fu->get_f_var(), fv->get_f_cst(), fw->get_f_expr()));
        } else {
          LOG(ERROR) << "ERROR Create the AlphaNode fw is uknown type";
          return -1;
        }
      } else if (fv->get_func_type() == f_binded_e) {
        if(fw->get_func_type() == f_cst_e) {
          alpha_nodes.push_back(create_alpha_node<F_var, F_binded, F_cst>(node_vertexes[vertex].get(), key, is_antecedent, normalized_label, 
            fu->get_f_var(), fv->get_f_binded(), fw->get_f_cst()));
        } else if(fw->get_func_type() == f_binded_e) {
          alpha_nodes.push_back(create_alpha_node<F_var, F_binded, F_binded>(node_vertexes[vertex].get(), key, is_antecedent, normalized_label, 
            fu->get_f_var(), fv->get_f_binded(), fw->get_f_binded()));
        } else if(fw->get_func_type() == f_var_e) {
          alpha_nodes.push_back(create_alpha_node<F_var, F_binded, F_var>(node_vertexes[vertex].get(), key, is_antecedent, normalized_label, 
            fu->get_f_var(), fv->get_f_binded(), fw->get_f_var()));
        } else if(fw->get_func_type() == f_expr_e) {
          alpha_nodes.push_back(create_alpha_node<F_var, F_binded, F_expr>(node_vertexes[vertex].get(), key, is_antecedent, normalized_label, 
            fu->get_f_var(), fv->get_f_binded(), fw->get_f_expr()));
        } else {
          LOG(ERROR) << "ERROR Create the AlphaNode fw is uknown type";
          return -1;
        }

      } else if (fv->get_func_type() == f_var_e) {
        if(fw->get_func_type() == f_cst_e) {
          alpha_nodes.push_back(create_alpha_node<F_var, F_var, F_cst>(node_vertexes[vertex].get(), key, is_antecedent, normalized_label, 
            fu->get_f_var(), fv->get_f_var(), fw->get_f_cst()));
        } else if(fw->get_func_type() == f_binded_e) {
          alpha_nodes.push_back(create_alpha_node<F_var, F_var, F_binded>(node_vertexes[vertex].get(), key, is_antecedent, normalized_label, 
            fu->get_f_var(), fv->get_f_var(), fw->get_f_binded()));
        } else if(fw->get_func_type() == f_var_e) {
          alpha_nodes.push_back(create_alpha_node<F_var, F_var, F_var>(node_vertexes[vertex].get(), key, is_antecedent, normalized_label, 
            fu->get_f_var(), fv->get_f_var(), fw->get_f_var()));
        } else if(fw->get_func_type() == f_expr_e) {
          alpha_nodes.push_back(create_alpha_node<F_var, F_var, F_expr>(node_vertexes[vertex].get(), key, is_antecedent, normalized_label, 
            fu->get_f_var(), fv->get_f_var(), fw->get_f_expr()));
        } else {
          LOG(ERROR) << "ERROR Create the AlphaNode fw is uknown type";
          return -1;
        }

      } else {
        LOG(ERROR) << "ERROR Create the AlphaNode fv is uknown type";
        return -1;
      }
    } else {
      LOG(ERROR) << "ERROR Create the AlphaNode fu is uknown type";
      return -1;
    }
  }
  return SQLITE_OK;
}

int
ReteMetaStoreFactory::create_expr(int expr_key, ExprBasePtr & expr)
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

  int res = sqlite3_reset(this->expr_stmt_);
  if( res != SQLITE_OK ) return res;

  res = sqlite3_bind_int(this->expr_stmt_, 1, expr_key);
  if( res != SQLITE_OK ) return res;

  res = sqlite3_step( this->expr_stmt_ );
  if(res != SQLITE_ROW) return res;

  char const* type = (char const*)sqlite3_column_text( this->expr_stmt_, 1  );  
  int arg0_key     = get_column_int_value( this->expr_stmt_, 2  );   //  INTEGER,
  int arg1_key     = get_column_int_value( this->expr_stmt_, 3  );   //  INTEGER,
  // int arg2_key     = get_column_int_value( this->expr_stmt_, 4  );   //  INTEGER,
  // int arg3_key     = get_column_int_value( this->expr_stmt_, 5  );   //  INTEGER,
  // int arg4_key     = get_column_int_value( this->expr_stmt_, 6  );   //  INTEGER,
  // int arg5_key     = get_column_int_value( this->expr_stmt_, 7  );   //  INTEGER,
  char const* opc   = (char const*)sqlite3_column_text( this->expr_stmt_, 8  );
  std::string op;
  if(opc) op = opc;
  if(not type) return -1;

  if( strcmp(type, "binary") == 0) {
    // do not use type pass this (this function is recursive)
    if(op.empty()) return -1;
    if(arg0_key<0 or arg1_key<0) return -1;

    ExprBasePtr lhs{}, rhs{};
    res = this->create_expr(arg0_key, lhs);
    if( res != SQLITE_OK ) return res;
    res = this->create_expr(arg1_key, rhs);
    if( res != SQLITE_OK ) return res;

    expr = create_binary_expr(expr_key, lhs, op, rhs);
    return SQLITE_OK;
  }
  if( strcmp(type, "unary") == 0) {
    // do not use type pass this (this function is recursive)
    if(op.empty()) return -1;
    if(arg0_key<0) return -1;

    ExprBasePtr arg{};
    res = this->create_expr(arg0_key, arg);
    if( res != SQLITE_OK ) return res;

    expr = create_unary_expr(expr_key, op, arg);
    return SQLITE_OK;

  }
  if( strcmp(type, "function") == 0) {
    // do not use type pass this (this function is recursive)
    return -1;
  } 
  if( strcmp(type, "resource") == 0) {
    // do not use type pass this (this function is recursive)
    if(arg0_key<0) return -1;
    auto itor = this->r_map_.find(arg0_key);
    if(itor != this->r_map_.end()) {
      auto r = itor->second;
      expr = create_expr_cst(*r);
      return SQLITE_OK;
    }
    auto vitor = this->v_map_.find(arg0_key);
    if(vitor != this->v_map_.end()) {
      auto const& vinfo = vitor->second;
      expr = create_expr_binded_var(vinfo.row_pos);
      return SQLITE_OK;
    }
  }

  return -1;
}

int
ReteMetaStoreFactory::create_beta_row_initializer(int vertex, int file_key, BetaRowInitializerPtr & bri)
{
  // -- --------------------
  // -- beta_row_config table
  // -- --------------------
  // key              0  INTEGER PRIMARY KEY,
  // vertex           1  INTEGER NOT NULL,
  // seq              2  INTEGER NOT NULL,
  // source_file_key  3  INTEGER NOT NULL,
  // row_pos          4  INTEGER NOT NULL,
  // is_binded        5  INTEGER,
  // id               6  STRING,
  // UNIQUE (vertex, seq, source_file_key)
  if(vertex < 0 or file_key<0) return -1;

  int res = sqlite3_reset(this->br_stmt_);
  if( res != SQLITE_OK ) return res;

  res = sqlite3_bind_int(this->br_stmt_, 1, vertex);
  if( res != SQLITE_OK ) return res;
  res = sqlite3_bind_int(this->br_stmt_, 2, file_key);
  if( res != SQLITE_OK ) return res;

  std::ostringstream buf;
  buf << "SELECT count(*) FROM beta_row_config WHERE vertex = " << vertex <<
    " AND source_file_key = " << file_key;
  std::string sql = buf.str();
  int br_sz = this->run_count_stmt(sql.c_str());

  bri = create_row_initializer(br_sz);

  bool is_done = false;
  while(not is_done) {
    res = sqlite3_step( this->br_stmt_ );
    if ( res == SQLITE_DONE ) {
      is_done = true;
      continue;
    }
    if(res != SQLITE_ROW) {
      LOG(ERROR) << "ReteMetaStoreFactory::create_rete_meta_store: " <<
        "SQL error while reading beta_row_config table: " << res;
      return res;
    }
    // Get the data out of the row
    int seq                   = get_column_int_value( this->br_stmt_, 2  );   //  INTEGER NOT NULL,
    int row_pos               = get_column_int_value( this->br_stmt_, 4  );   //  INTEGER NOT NULL,
    int is_binded             = get_column_int_value( this->br_stmt_, 5  );   //  INTEGER,
    std::string id ((char const*)sqlite3_column_text( this->br_stmt_, 6 ));   //  STRING NOT NULL,

    // Add the initializer row
    if(is_binded) {
      if(bri->put(seq, row_pos | brc_parent_node, id)) {
        LOG(ERROR) << "ReteMetaStoreFactory::create_rete_meta_store: " <<
          "ERROR while adding row to beta row initializer, is size ok?";
        return -1;
      }
    } else {
      if(bri->put(seq, row_pos | brc_triple, id)) {
        LOG(ERROR) << "ReteMetaStoreFactory::create_rete_meta_store: " <<
          "ERROR while adding row to beta row initializer, is size ok?";
        return -1;
      }
    }
  }
  return SQLITE_OK;
}

} // namespace jets::rete
