#ifndef JETS_RETE_RETE_META_STORE_H
#define JETS_RETE_RETE_META_STORE_H

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
#include "jets/rete/expr.h"


// Component to manage all the rdf resources and literals of a graph
namespace jets::rete {
// //////////////////////////////////////////////////////////////////////////////////////
// ReteMetaStore class -- main session class for the rete network
// --------------------------------------------------------------------------------------
template<class T>
class ReteMetaStore;
template<class T>
using ReteMetaStorePtr = std::shared_ptr<ReteMetaStore<T>>;

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
template<class T>
class ReteMetaStore {
 public:
  using RDFSession = T;
  using RDFSessionPtr = std::shared_ptr<T>;
  using Iterator = typename T::Iterator;
  using RDFGraph = typename T::RDFGraph;
  using RDFGraphPtr = std::shared_ptr<RDFGraph>;

  using AlphaNodeVector = std::vector<AlphaNodePtr<T>>;
  using ExprVector = std::vector<ExprBasePtr<T>>;

  ReteMetaStore()
    : alpha_nodes_(),
      exprs_(),
      node_vertexes_()
  {}
  // ReteMetaStore(AlphaNodeVector alpha_nodes, ExprVector exprs, NodeVertexVector node_vertexes)
  //   : alpha_nodes_(alpha_nodes), 
  //     exprs_(exprs),
  //     node_vertexes_(node_vertexes)
  // {}
  ReteMetaStore(AlphaNodeVector const& alpha_nodes, ExprVector const& exprs, 
    NodeVertexVector const& node_vertexes)
    : alpha_nodes_(alpha_nodes), 
      exprs_(exprs),
      node_vertexes_(node_vertexes)
  {}
  ReteMetaStore(AlphaNodeVector && alpha_nodes, ExprVector && exprs, 
      NodeVertexVector && node_vertexes)
    : alpha_nodes_(std::forward<AlphaNodeVector>(alpha_nodes)), 
      exprs_(std::forward<ExprVector>(exprs)), 
      node_vertexes_(std::forward<NodeVertexVector>(node_vertexes))
  {}

  /**
   * @brief Get the alpha node object by key
   * 
   * The alpha node key correspond to the beta relation key
   * for antecedent alpha nodes. Consequent alpha nodes have 
   * their key post anatecedent nodes
   * @param vertex 
   * @return AlphaNode<T> const* 
   */
  inline AlphaNode<T> const*
  get_alpha_node(int vertex)const
  {
    if(vertex<0 or vertex >= alpha_nodes_.size()) return {};
    return alpha_nodes_[vertex].get();
  }

  inline ExprBase<T> const*
  get_expr(int vertex)const
  {
    if(vertex<0 or vertex >= exprs_.size()) return {};
    return exprs_[vertex].get();
  }

  inline b_index
  get_node_vertex(int vertex)const
  {
    if(vertex<0 or vertex >= node_vertexes_.size()) return {};
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
      // remember the root node does not have a parent node!!
      if(not node->parent_node_vertex) continue;
      auto & parent_node = this->node_vertexes_[node->parent_node_vertex->vertex];
      parent_node->child_nodes.insert(node);
    }

    // Assign consequent terms vertex (AlphaNode) to NodeVertex
    int sz = static_cast<int>(this->alpha_nodes_.size());
    for(int ipos=0; ipos<sz; ++ipos) {
      // validate that alpha node at ipos < nbr_vertices are antecedent nodes
      auto & alpha_ptr =  this->alpha_nodes_[ipos];
      if(ipos<this->nbr_vertices() and not alpha_ptr->is_antecedent() ) {
        LOG(ERROR) << "ReteMetaStore::initialize: error AlphaNode "
          "with vertex < nbr_vertices that is NOT an antecedent term!";
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
  friend class ReteSession<T>;
  AlphaNodeVector  alpha_nodes_;
  ExprVector       exprs_;
  NodeVertexVector node_vertexes_;
};

template<class T>
inline ReteMetaStorePtr<T> create_rete_meta_store(
  typename ReteMetaStore<T>::AlphaNodeVector const& alpha_nodes,
  typename ReteMetaStore<T>::ExprVector const& exprs,
  NodeVertexVector const& node_vertexes)
{
  return std::make_shared<ReteMetaStore<T>>(alpha_nodes, exprs, node_vertexes);
}

template<class T>
inline ReteMetaStorePtr<T> create_rete_meta_store(
  typename ReteMetaStore<T>::AlphaNodeVector && alpha_nodes,
  typename ReteMetaStore<T>::ExprVector && exprs,
  NodeVertexVector && node_vertexes)
{
  return std::make_shared<ReteMetaStore<T>>(
    std::forward<typename ReteMetaStore<T>::AlphaNodeVector>(alpha_nodes),
    std::forward<typename ReteMetaStore<T>::ExprVector>(alpha_nodes),
    std::forward<NodeVertexVector>(alpha_nodes) );
}

} // namespace jets::rete
#endif // JETS_RETE_RETE_META_STORE_H
