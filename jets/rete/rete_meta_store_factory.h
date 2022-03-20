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

#include "alpha_functors.h"
#include "alpha_node_impl.h"
#include "beta_row_initializer.h"
#include "expr.h"
#include "rete_session.h"
#include "sqlite3.h"

#include "../rdf/rdf_types.h"
#include "../rete/node_vertex.h"
#include "../rete/alpha_node.h"
#include "../rete/rete_meta_store.h"
#include "../rete/expr_operator_factory.h"
#include "../rete/rete_meta_store_factory_helper.h"
#include "../rete/alpha_node_impl.h"

// Factory class to create and configure ReteMetaStore objects
static int read_resources_cb(void *data, int argc, char **argv, char **azColName);
namespace jets::rete {
// //////////////////////////////////////////////////////////////////////////////////////
// ReteMetaStoreFactory class -- Class constructing instances of ReteMetaStore using jetstore_rete.db
// --------------------------------------------------------------------------------------
struct var_info {
  var_info(std::string_view id, bool is_binded, int vertex, int row_pos)
    : id(id), is_binded(is_binded), vertex(vertex), row_pos(row_pos) {}
  std::string id;
  bool is_binded;
  int vertex;
  int row_pos;
};
/**
 * @brief Factory class for ReteMetaStore, using jetrule_rete.db
 * 
 */
class ReteMetaStoreFactory {
 public:
  using ResourceLookup = std::unordered_map<int, rdf::r_index>;
  using ReteSessionLookup = std::unordered_map<ReteSession *, ReteSessionPtr>;
  // key  ->  <var name, is_binded>
  using VariableLookup = std::unordered_map<int, var_info>;
  using MainRuleUriLookup = std::unordered_map<std::string, int>;
  using MetaStoreLookup = std::unordered_map<int, ReteMetaStorePtr>;

  ~ReteMetaStoreFactory()
  {
    this->reset();
  }

  ReteMetaStoreFactory();

  inline int
  reset()
  {
    if(this->db_) {
      auto db = this->db_;
      this->db_ = nullptr;

      if(this->node_vertexes_stmt_) sqlite3_finalize( this->node_vertexes_stmt_ );
      if(this->alpha_nodes_stmt_) sqlite3_finalize( this->alpha_nodes_stmt_ );
      if(this->expr_stmt_) sqlite3_finalize( this->expr_stmt_ );
      if(this->br_stmt_) sqlite3_finalize( this->br_stmt_ );
      this->node_vertexes_stmt_ = nullptr;
      this->alpha_nodes_stmt_ = nullptr;
      this->expr_stmt_ = nullptr;
      this->br_stmt_ = nullptr;
      int res = sqlite3_close_v2(db);
      if ( res != SQLITE_OK ) {
        LOG(ERROR) << "ReteMetaStoreFactory::create_rete_meta_store: ERROR while closing rete_db connection, code: "<<res;
      }
      return res;
    }
    return 0;
  }

  rdf::RDFGraph const*
  meta_graph()
  {
    return this->meta_graph_.get();
  }

  rdf::RDFGraphPtr
  get_meta_graph()
  {
    return this->meta_graph_;
  }

  ReteMetaStorePtr
  get_rete_meta_store(std::string const& main_rule) const
  {
    auto itor = this->jr_map_.find(main_rule);
    if(itor == this->jr_map_.end()) {
      LOG(WARNING) << "ReteMetaStoreFactory::create_rete_meta_store: WARNING main_rule file "<<main_rule<<" not found";
      return {};
    }
    auto mitor = this->ms_map_.find(itor->second);
    if(mitor == this->ms_map_.end()) {
      LOG(ERROR) << "ReteMetaStoreFactory::create_rete_meta_store: ERROR ReteMetaStore not found for main_rule file "<<
        main_rule;
      return {};
    }
    return mitor->second;
  }

  ReteSessionPtr
  create_rete_session(std::string const& main_rule)
  {
    auto ms = this->get_rete_meta_store(main_rule);
    if(not ms) {
      LOG(ERROR) << "ReteMetaStoreFactory::create_rete_session: ERROR ReteMetaStore not found for main_rule file "<<
        main_rule;
      return {};
    }
    auto rdf_session = rdf::create_rdf_session(this->meta_graph_);
    auto rete_session = rete::create_rete_session(ms, rdf_session);
    rete_session->initialize();
    this->rs_map_.insert({rete_session.get(), rete_session});
    return rete_session;
  }

  int
  delete_rete_session(ReteSession * rs)
  {
    if(not rs) {
      LOG(ERROR) << "ReteMetaStoreFactory::create_rete_session: ERROR ReteSession argument cannot be NULL";
      return -1;
    }
    this->rs_map_.erase(rs);
    return 0;
  }

  int
  load_database(std::string const& jetrule_rete_db);

  int 
  read_resources_cb(int argc, char **argv, char **colnm);

 protected:
  int
  load_resources()
  {
    if(not this->r_map_.empty()) return 0;  // already loaded    
    int err = 0;
    char * err_msg = 0;

    if(not this->meta_graph_ ) {
      this->meta_graph_ = rdf::create_rdf_graph();
      this->meta_graph_->rmgr()->initialize();
    }
    auto const* sql = "SELECT * from resources";
    err = sqlite3_exec(this->db_, sql, ::read_resources_cb, (void*)this, &err_msg);    
    if( err != SQLITE_OK ) {
      LOG(ERROR) << "ReteMetaStoreFactory::create_rete_meta_store: SQL error while reading resources: " << err_msg;
      sqlite3_free(err_msg);
      return -1;
    }
    return 0;
  }

  int
  load_workspace_control()
  {
    this->jr_map_.clear();
    auto const* sql = "SELECT key, source_file_name from workspace_control WHERE is_main = 1";

    sqlite3_stmt* stmt;
    int res = sqlite3_prepare_v2( this->db_, sql, -1, &stmt, 0 );
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
      this->jr_map_.insert({rule_uri, key});
    }
    sqlite3_finalize( stmt );
    return 0;
  }

  int
  load_node_vertexes(int file_key, NodeVertexVector & node_vertexes);

  int
  load_alpha_nodes(int file_key, NodeVertexVector const& node_vertexes, AlphaNodeVector & alpha_nodes);

  FuncFactoryPtr
  create_func_factory(int key)
  {
    auto itor = this->r_map_.find(key);
    if(itor != this->r_map_.end()) {
      // F_cst
      auto r = itor->second;
      return std::make_shared<FcstFunc>(r);
    }

    auto v_vitor = this->v_map_.find(key);
    if(v_vitor != this->v_map_.end()) {
      auto const& vinfo = v_vitor->second;
      if(vinfo.is_binded) {
        // F_binded 
        return std::make_shared<FbindedFunc>(vinfo.row_pos);

      } else {
        // F_var
        return std::make_shared<FvarFunc>(vinfo.id);
      }
    }
    RETE_EXCEPTION("ERROR create_func_factor for key "<<key);
  }

  FuncFactoryPtr
  create_func_expr_factory(int key)
  {
    ExprBasePtr expr;
    int res = create_expr(key, expr);
    if(res) {
      RETE_EXCEPTION("ERROR create_func_expr_factor for key "<<key<<", err "<<res);
    }
    return std::make_shared<FexprFunc>(expr);
  }

  int
  create_expr(int expr_key, ExprBasePtr & expr);

  int
  create_beta_row_initializer(int vertex, int file_key, BetaRowInitializerPtr & bri);

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

int 
run_count_stmt(const char* sql )
{
  sqlite3_stmt* stmt;
  int res = sqlite3_prepare_v2( this->db_, sql, -1, &stmt, 0 );
  if ( res != SQLITE_OK ) {
    return -1;
  }

  res = sqlite3_step( stmt );
  if ( res != SQLITE_ROW ) {
    return -1;
  }

  int count = sqlite3_column_int( stmt, 0 );
  sqlite3_finalize( stmt );
  return count;
}

  std::string jetrule_rete_db_;
  rdf::RDFGraphPtr meta_graph_;

  ResourceLookup r_map_;
  VariableLookup v_map_;
  MainRuleUriLookup jr_map_;
  MetaStoreLookup ms_map_;
  ReteSessionLookup rs_map_;

  sqlite3 *     db_;
  sqlite3_stmt* node_vertexes_stmt_;
  sqlite3_stmt* alpha_nodes_stmt_;
  sqlite3_stmt* expr_stmt_;
  sqlite3_stmt* br_stmt_;

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
