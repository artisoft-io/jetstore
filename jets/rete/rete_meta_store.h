#ifndef JETS_RETE_RETE_META_STORE_H
#define JETS_RETE_RETE_META_STORE_H

#include <algorithm>
#include <string>
#include <memory>
#include <list>
#include <vector>
#include <unordered_map>
#include <unordered_set>

#include <glog/logging.h>

#include "absl/hash/hash.h"
#include "absl/container/flat_hash_set.h"

#include "../rdf/rdf_types.h"
#include "../rete/node_vertex.h"
#include "../rete/alpha_node.h"
#include "../rete/lookup_sql_helper.h"


// Component to manage all the rdf resources and literals of a graph
namespace jets::rete {
// //////////////////////////////////////////////////////////////////////////////////////
// ReteMetaStore class -- main session class for the rete network
// --------------------------------------------------------------------------------------
class ReteMetaStore;
using ReteMetaStorePtr = std::shared_ptr<ReteMetaStore>;

using NodeVertexVector = std::vector<NodeVertexPtr>;

/**
 * @brief The two LEGACY retraction defects, as a mode fixed when the network is built
 *
 * Both engines keep two retraction defects by Michel's decision of 2026-10-09, because
 * usi_ws's loop rules and CM04 op01 terminate only because of them
 * (jets/jetrules/rete/README.md, entry of 2026-10-08, revised 2026-10-09):
 *
 *  1. a consequent that reads the graph (ExprBase::reads_graph) is recomputed on
 *     retraction, so once its input has moved the triple retracted is not the triple
 *     inferred;
 *  2. a withdrawn row never fires again: compute_consequent_triples calls
 *     remove_beta_row on a row already marked kDeleted, which returns without erasing
 *     it, so an equal row arriving later is taken for one already inferred.
 *
 * This is README option (b), built for the rules unit test harness only: the harness
 * reaches it through create_jetstore_hdl_with_mode. create_jetstore_hdl, which every
 * pipeline calls, builds the default -- legacy for both. No rule_config key reaches it.
 * The fixed behaviours are the ones 1d993844 built and c2de6953 narrowed. The Go engine
 * has the same struct, rete.RetractionMode in jets/jetrules/rete/retraction_mode.go.
 */
struct RetractionMode {
  // Defect 1 off: a vertex whose consequent reads the graph records what its rows
  // inferred, and retraction replays the record.
  bool replay_graph_reading_consequents{false};
  // Defect 2 off: a row is erased from its relation once its consequents are
  // retracted (BetaRelation::erase_retracted_beta_row), so an equal row fires again.
  bool erase_withdrawn_rows{false};

  inline bool
  is_legacy()const
  {
    return not replay_graph_reading_consequents and not erase_withdrawn_rows;
  }

  // "legacy", "fixed-1", "fixed-2" or "fixed", as rete.RetractionMode.Label
  inline char const*
  label()const
  {
    if(replay_graph_reading_consequents and erase_withdrawn_rows) return "fixed";
    if(replay_graph_reading_consequents) return "fixed-1";
    if(erase_withdrawn_rows) return "fixed-2";
    return "legacy";
  }
};

/**
 * @brief Main metadata store regarding the rete network
 *
 * The ReteMetaStore correspond to a complete rule set
 * organized as a rete network.
 *
 * Initialize routine perform important connection between the
 * metadata entities, such as reverse lookup of the consequent terms
 * and children lookup for each NodeVertex.
 *
 * Initialize must be call before use with ReteSession
 * 
 * @tparam T 
 */
class ReteMetaStore {
 public:
  using Iterator = rdf::RDFSession::Iterator;

  using AlphaNodeVector = std::vector<AlphaNodePtr>;
  using ExprVector = std::vector<ExprBasePtr>;

  ReteMetaStore()
    : lookup_sql_helper_(),
      alpha_nodes_(),
      node_vertexes_()
  {}  
  ReteMetaStore(LookupSqlHelperPtr lookup_sql_helper, AlphaNodeVector const& alpha_nodes, NodeVertexVector const& node_vertexes)
    : lookup_sql_helper_(lookup_sql_helper),
      alpha_nodes_(alpha_nodes), 
      node_vertexes_(node_vertexes)
  {}
  ReteMetaStore(LookupSqlHelperPtr lookup_sql_helper, AlphaNodeVector && alpha_nodes, NodeVertexVector && node_vertexes)
    : lookup_sql_helper_(lookup_sql_helper),
      alpha_nodes_(std::forward<AlphaNodeVector>(alpha_nodes)), 
      node_vertexes_(std::forward<NodeVertexVector>(node_vertexes))
  {}

  inline LookupSqlHelperPtr
  get_lookup_sql_helper()const
  {
    return this->lookup_sql_helper_;
  }

  inline AlphaNodeVector const&
  alpha_nodes()const
  {
    return this->alpha_nodes_;
  }

  inline NodeVertexVector const&
  node_vertexes()const
  {
    return this->node_vertexes_;
  }

  /**
   * @brief Get the alpha node object by key
   * 
   * The alpha node key correspond to the beta relation key
   * for antecedent alpha nodes. Consequent alpha nodes have 
   * their key post anatecedent nodes
   * @param vertex 
   * @return AlphaNode const* 
   */
  inline AlphaNode const*
  get_alpha_node(int vertex)const
  {
    if(vertex<0 or vertex >= static_cast<int>(alpha_nodes_.size())) return {};
    return alpha_nodes_[vertex].get();
  }

  inline b_index
  get_node_vertex(int vertex)const
  {
    if(vertex<0 or vertex >= static_cast<int>(node_vertexes_.size())) return {};
    return node_vertexes_[vertex].get();
  }

  inline int
  nbr_vertices()const
  {
    return static_cast<int>(node_vertexes_.size());
  }

  /**
   * @brief Connect the metadata entities, in a retraction mode
   *
   * The mode is fixed here, once, for every session built on this meta store: it
   * decides which node vertexes record their inferred triples, and
   * ReteSession::compute_consequent_triples reads it back through retraction_mode()
   * to decide whether a withdrawn row is erased. The default is legacy for both
   * defects, which is what every pipeline runs.
   */
  int
  initialize(RetractionMode mode = {})
  {
    this->retraction_mode_ = mode;
    // Perform reverse lookup of children NodeVertex using
    // NodeVertex parent_node property
    for(auto const& nodeptr: this->node_vertexes_) {
      b_index node = nodeptr.get();
      // remember the root node does not have a parent node!
      if(not node->parent_node_vertex) continue;
      auto & parent_node = this->node_vertexes_[node->parent_node_vertex->vertex];
      parent_node->child_nodes.insert(node);
    }

    // Assign consequent terms vertex (AlphaNode) to NodeVertex
    int sz = static_cast<int>(this->alpha_nodes_.size());
    for(int ipos=0; ipos<sz; ++ipos) {
      // validate that alpha node at ipos < nbr_vertices are antecedent nodes
      auto & alpha_ptr =  this->alpha_nodes_[ipos];
      if(ipos<(this->nbr_vertices()-1) and not alpha_ptr->is_antecedent() ) {
        LOG(ERROR) << "ReteMetaStore::initialize: error AlphaNode "
          "with vertex "<<ipos<<" < nbr_vertices "<<this->nbr_vertices()<<" that is NOT an antecedent term!";
        return -1;
      }
      if(not alpha_ptr->is_antecedent()) {
        b_index node = alpha_ptr->get_node_vertex();
        auto & current_node = this->node_vertexes_[node->vertex];
        current_node->consequent_alpha_vertexes.insert(ipos);
        // A minting consequent always records. One that reads the graph records only
        // with LEGACY defect 1 turned off; otherwise it is recomputed on retraction.
        if(alpha_ptr->mints_resource() or
            (mode.replay_graph_reading_consequents and alpha_ptr->reads_graph())) {
          current_node->records_consequents = true;
        }
      }
    }
    return 0;
  }

 inline RetractionMode const&
  retraction_mode()const
  {
    return this->retraction_mode_;
  }

 private:
  friend class ReteSession;
  LookupSqlHelperPtr lookup_sql_helper_;
  AlphaNodeVector  alpha_nodes_;
  NodeVertexVector node_vertexes_;
  RetractionMode   retraction_mode_;
};

inline ReteMetaStorePtr create_rete_meta_store(
  LookupSqlHelperPtr lookup_sql_helper,
  ReteMetaStore::AlphaNodeVector const& alpha_nodes,
  NodeVertexVector const& node_vertexes)
{
  return std::make_shared<ReteMetaStore>(lookup_sql_helper, alpha_nodes, node_vertexes);
}

inline ReteMetaStorePtr create_rete_meta_store(
  LookupSqlHelperPtr lookup_sql_helper,
  ReteMetaStore::AlphaNodeVector && alpha_nodes,
  NodeVertexVector && node_vertexes)
{
  return std::make_shared<ReteMetaStore>(
    lookup_sql_helper,
    std::forward<ReteMetaStore::AlphaNodeVector>(alpha_nodes),
    std::forward<NodeVertexVector>(node_vertexes) 
  );
}

} // namespace jets::rete
#endif // JETS_RETE_RETE_META_STORE_H
