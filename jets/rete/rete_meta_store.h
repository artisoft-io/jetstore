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

#include "jets/rdf/rdf_types.h"
#include "jets/rete/node_vertex.h"
#include "jets/rete/alpha_node.h"


// Component to manage all the rdf resources and literals of a graph
namespace jets::rete {
// //////////////////////////////////////////////////////////////////////////////////////
// ReteMetaStore class -- main session class for the rete network
// --------------------------------------------------------------------------------------
class ReteMetaStore;
using ReteMetaStorePtr = std::shared_ptr<ReteMetaStore>;

using NodeVertexVector = std::vector<NodeVertexPtr>;

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
    : meta_graph_(),
      alpha_nodes_(),
      node_vertexes_()
  {}  
  ReteMetaStore(rdf::RDFGraphPtr meta_graph, AlphaNodeVector const& alpha_nodes, NodeVertexVector const& node_vertexes)
    : meta_graph_(meta_graph),
      alpha_nodes_(alpha_nodes), 
      node_vertexes_(node_vertexes)
  {}
  ReteMetaStore(rdf::RDFGraphPtr meta_graph, AlphaNodeVector && alpha_nodes, NodeVertexVector && node_vertexes)
    : meta_graph_(meta_graph),
      alpha_nodes_(std::forward<AlphaNodeVector>(alpha_nodes)), 
      node_vertexes_(std::forward<NodeVertexVector>(node_vertexes))
  {}

  inline rdf::RDFGraph const*
  get_meta_graph()const
  {
    return this->meta_graph_.get();
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

  int
  initialize()
  {
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
      }
    }
    return 0;
  }

 private:
  friend class ReteSession;
  rdf::RDFGraphPtr meta_graph_;
  AlphaNodeVector  alpha_nodes_;
  NodeVertexVector node_vertexes_;
};

inline ReteMetaStorePtr create_rete_meta_store(
  rdf::RDFGraphPtr meta_graph,
  ReteMetaStore::AlphaNodeVector const& alpha_nodes,
  NodeVertexVector const& node_vertexes)
{
  return std::make_shared<ReteMetaStore>(meta_graph, alpha_nodes, node_vertexes);
}

inline ReteMetaStorePtr create_rete_meta_store(
  rdf::RDFGraphPtr meta_graph,
  ReteMetaStore::AlphaNodeVector && alpha_nodes,
  NodeVertexVector && node_vertexes)
{
  return std::make_shared<ReteMetaStore>(
    meta_graph,
    std::forward<ReteMetaStore::AlphaNodeVector>(alpha_nodes),
    std::forward<NodeVertexVector>(node_vertexes) 
  );
}

} // namespace jets::rete
#endif // JETS_RETE_RETE_META_STORE_H
