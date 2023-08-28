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
#include "sqlite3.h"

#include "../rdf/rdf_types.h"
#include "beta_row_initializer.h"
#include "../rete/node_vertex.h"
#include "../rete/alpha_node.h"
#include "alpha_functors.h"
#include "alpha_node_impl.h"
#include "expr.h"
#include "rete_session.h"

#include "../rete/rete_meta_store.h"
#include "../rete/rete_meta_store_factory_helper.h"

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
    // this->reset();
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

  rdf::RManager const*
  rmgr()
  {
    return this->rmgr_.get();
  }

  rdf::RManagerPtr
  get_rmgr()
  {
    return this->rmgr_;
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

  int
  load_database(std::string const& jetrule_rete_db, std::string const& lookup_data_db);

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
      this->rmgr_ = this->meta_graph_->get_rmgr();
      this->rmgr_->initialize();
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

  public:
  int
  load_meta_triples(std::string const& process_name, int is_rule_set)
  {
    auto const* sqlSequence = "SELECT t3.subject_key, t3.predicate_key, t3.object_key FROM triples t3, rule_sequences rs, main_rule_sets mrs WHERE t3.source_file_key = mrs.ruleset_file_key AND mrs.rule_sequence_key = rs.key AND rs.name = ?";
    auto const* sqlRuleSet  = "SELECT t3.subject_key, t3.predicate_key, t3.object_key FROM triples t3, workspace_control wc WHERE t3.source_file_key = wc.key AND wc.source_file_name = ?";

    sqlite3_stmt* stmt;
    int res;
    if(is_rule_set) {
      res = sqlite3_prepare_v2( this->db_, sqlRuleSet, -1, &stmt, 0 );
    } else {
      res = sqlite3_prepare_v2( this->db_, sqlSequence, -1, &stmt, 0 );
    }
    if ( res != SQLITE_OK ) {
      LOG(ERROR) << "ReteMetaStoreFactory::load_meta_triples: SQL error while sqlite3_prepare_v2: " << res;
      return res;
    }

    res = sqlite3_bind_text(stmt, 1, process_name.c_str(), process_name.size(), nullptr);
    if ( res != SQLITE_OK ) {
      LOG(ERROR) << "ReteMetaStoreFactory::load_meta_triples: SQL error while sqlite3_bind_text: " << res;
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
        LOG(ERROR) << "ReteMetaStoreFactory::create_rete_meta_store: SQL error while load_meta_triples: " << res;
        return res;
      }
      // Get the data out of the row and in the meta_graph
      int skey = sqlite3_column_int( stmt, 0 );
      int pkey = sqlite3_column_int( stmt, 1 );
      int okey = sqlite3_column_int( stmt, 2 );
      auto stor = this->r_map_.find(skey);
      if (stor == this->r_map_.end()) {
        LOG(ERROR) << "ReteMetaStoreFactory::load_meta_triples: subject key not found in resource map, key: " << skey;
        return -1;
      }
      auto ptor = this->r_map_.find(pkey);
      if (ptor == this->r_map_.end()) {
        LOG(ERROR) << "ReteMetaStoreFactory::load_meta_triples: predicate key not found in resource map, key: " << pkey;
        return -1;
      }
      auto otor = this->r_map_.find(okey);
      if (otor == this->r_map_.end()) {
        LOG(ERROR) << "ReteMetaStoreFactory::load_meta_triples: object key not found in resource map, key: " << okey;
        return -1;
      }
      this->meta_graph_->insert(stor->second, ptor->second, otor->second);
    }
    sqlite3_finalize( stmt );
    return 0;
  }

 protected:
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

  ExprBasePtr
  create_binary_expr(int key, ExprBasePtr lhs, std::string const& op, ExprBasePtr rhs);

  ExprBasePtr
  create_unary_expr(int key, std::string const& op, ExprBasePtr arg);

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
  std::string lookup_data_db_;
  rdf::RManagerPtr rmgr_;
  rdf::RDFGraphPtr meta_graph_;

  ResourceLookup r_map_;
  VariableLookup v_map_;
  MainRuleUriLookup jr_map_;
  MetaStoreLookup ms_map_;

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
