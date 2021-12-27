#ifndef JETS_RETE_RETE_META_STORE_H
#define JETS_RETE_RETE_META_STORE_H

#include <string>
#include <memory>
#include <list>
#include <vector>
#include <unordered_map>
#include <unordered_set>

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
// NodeVertexAdjency represent adjency graph using map<parent vertex, set<child vertexes>>
using NodeVertexAdjency = std::unordered_map<int, std::unordered_set<int>>;

// ReteMetaStore making the rete network
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
  using PairIntItor = std::pair<
                        std::unordered_set<int>::const_iterator,
                        std::unordered_set<int>::const_iterator>;
  // NodeVertexConsequentTerms is the set of consequent terms associated with a NodeVertex 
  // using map<node vertex, set<consequent terms>> these are identified as AlphaNodes that 
  // are consequent terms associated with NodeVertex
  using AlphaNodeSet = std::unordered_set<AlphaNode<T> const*>;
  using NodeVertexConsequentTerms = std::unordered_map<int, AlphaNodeSet>;

  ReteMetaStore()
    : alpha_nodes_(),
      node_vertexes_(),
      node_vertex_adj_(),
      consequent_terms_()
  {}
  ReteMetaStore(AlphaNodeVector alpha_nodes, ExprVector exprs, 
      NodeVertexVector node_vertexes, NodeVertexAdjency node_vertex_adj)
    : alpha_nodes_(alpha_nodes), 
      exprs_(exprs),
      node_vertexes_(node_vertexes),
      node_vertex_adj_(node_vertex_adj),
      consequent_terms_()
  {
    // Set consequent_terms_ mapping, which is a reverse lookup
    initialize();
  }
  ReteMetaStore(AlphaNodeVector const& alpha_nodes, ExprVector const& exprs, 
      NodeVertexVector const& node_vertexes, NodeVertexAdjency node_vertex_adj)
    : alpha_nodes_(alpha_nodes), 
      exprs_(exprs),
      node_vertexes_(node_vertexes),
      node_vertex_adj_(node_vertex_adj),
      consequent_terms_()
  {
    // Set consequent_terms_ mapping, which is a reverse lookup
    initialize();
  }
  ReteMetaStore(AlphaNodeVector && alpha_nodes, ExprVector && exprs, 
      NodeVertexVector && node_vertexes, NodeVertexAdjency && node_vertex_adj)
    : alpha_nodes_(std::forward<AlphaNodeVector>(alpha_nodes)), 
      exprs_(std::forward<ExprVector>(exprs)), 
      node_vertexes_(std::forward<NodeVertexVector>(node_vertexes)),
      node_vertex_adj_(std::forward<NodeVertexAdjency>(node_vertex_adj)),
      consequent_terms_()
  {
    // Set consequent_terms_ mapping, which is a reverse lookup
    initialize();
  }

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

  inline AlphaNodeSet const*
  get_consequent_nodes(int vertex)const
  {
    auto itor = consequent_terms_.find(vertex);    
    if(itor != consequent_terms_.end()) {
      return &(itor->second);
    }
    return nullptr;
  }

  inline PairIntItor
  get_adj_node_vertexes(int vertex)const
  {
    auto itor = node_vertex_adj_.find(vertex);    
    if(itor != node_vertex_adj_.end()) {
      return {itor->second.begin(), itor->second.end()};
    }
    return {};
  }

  inline int
  nbr_vertices()const
  {
    return static_cast<int>(node_vertexes_.size());
  }

 protected:
  void
  initialize()
  {
    for(auto const& node: this->alpha_nodes_)
    {
      int vertex = node->get_node_vertex()->vertex;
      auto itor = consequent_terms_.find(vertex);
      if(itor == consequent_terms_.end()) {
        auto ret = consequent_terms_.insert({vertex, {}});
        ret.first->second.insert(node.get());
      } else {
        itor->second.insert(node.get());
      }
    }
  }

 private:
  friend class ReteSession<T>;
  AlphaNodeVector  alpha_nodes_;
  ExprVector       exprs_;
  NodeVertexVector node_vertexes_;
  // NodeVertexAdjency is map<parent node vertex>, <set of child node vertex>
  NodeVertexAdjency node_vertex_adj_;
  // NodeVertexConsequentTerms is the set of consequent terms associated with a NodeVertex 
  // using map<node vertex, set<consequent terms>> these are identified as AlphaNodes that 
  NodeVertexConsequentTerms consequent_terms_;
};

template<class T>
inline ReteMetaStorePtr<T> create_rete_meta_store()
{
  return std::make_shared<ReteMetaStore<T>>();
}

template<class T>
inline ReteMetaStorePtr<T> create_rete_meta_store(
  typename ReteMetaStore<T>::AlphaNodeVector alpha_nodes, 
  typename ReteMetaStore<T>::ExprVector exprs, 
  NodeVertexVector node_vertexes,
  NodeVertexAdjency node_adjency)
{
  return std::make_shared<ReteMetaStore<T>>(alpha_nodes, exprs, 
    node_vertexes, node_adjency);
}

template<class T>
inline ReteMetaStorePtr<T> create_rete_meta_store(
  typename ReteMetaStore<T>::AlphaNodeVector const& alpha_nodes,
  typename ReteMetaStore<T>::ExprVector const& exprs,
  NodeVertexVector const& node_vertexes,
  NodeVertexAdjency const& node_adjency)
{
  return std::make_shared<ReteMetaStore<T>>(alpha_nodes, exprs, 
    node_vertexes, node_adjency);
}

template<class T>
inline ReteMetaStorePtr<T> create_rete_meta_store(
  typename ReteMetaStore<T>::AlphaNodeVector && alpha_nodes,
  typename ReteMetaStore<T>::ExprVector && exprs,
  NodeVertexVector && node_vertexes,
  NodeVertexAdjency && node_adjency)
{
  return std::make_shared<ReteMetaStore<T>>(
    std::forward<typename ReteMetaStore<T>::AlphaNodeVector>(alpha_nodes),
    std::forward<typename ReteMetaStore<T>::ExprVector>(alpha_nodes),
    std::forward<NodeVertexVector>(alpha_nodes),
    std::forward<NodeVertexAdjency>(node_adjency) );
}

} // namespace jets::rete
#endif // JETS_RETE_RETE_META_STORE_H
