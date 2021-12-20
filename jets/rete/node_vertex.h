#ifndef JETS_RETE_NODE_VERTEX_H
#define JETS_RETE_NODE_VERTEX_H

#include <string>
#include <memory>
#include <list>

#include "jets/rete/beta_row_initializer.h"

// Metadata information for BetaRelations
namespace jets::rete {
// //////////////////////////////////////////////////////////////////////////////////////
// NodeVertex class -- metadata obj describing a BetaRelation node in the rete graph
// --------------------------------------------------------------------------------------
struct NodeVertex;
using NodeVertexPtr = std::shared_ptr<NodeVertex>;
using b_index = NodeVertex const *;

// Forward definition, see alpha_node.h
template<class T>
class AlphaNode;

template<class T>
using AlphaNodePtr = std::shared_ptr<AlphaNode<T>>;

template<class T>
using alpha_node_list = std::list<AlphaNodePtr<T>>;

// NodeVertex holding metadata information about a BetaRelation node
struct NodeVertex {

  NodeVertex()
    : parent_node_vertex(nullptr),
      vertex(0),
      is_consequent(false),
      is_negation(false),
      has_filter(false),
      salience(0),
      beta_row_initializer()
  {}

  NodeVertex(
    b_index parent_node_vertex, int vertex, bool is_consequent, bool is_negation, 
    bool has_filter, int salience, BetaRowInitializerPtr beta_row_initializer) 
    : parent_node_vertex(parent_node_vertex),
      vertex(vertex),
      is_consequent(is_consequent),
      is_negation(is_negation),
      has_filter(has_filter),
      salience(salience),
      beta_row_initializer(beta_row_initializer)
  {}

  inline b_index
  get_parent_node_vertex()const
  {
    return parent_node_vertex;
  }

  inline BetaRowInitializerPtr
  get_beta_row_initializer()const
  {
    return beta_row_initializer;
  }

  b_index               parent_node_vertex;
  int                   vertex;
  bool                  is_consequent;
  bool                  is_negation;
  bool                  has_filter;
  int                   salience;
  BetaRowInitializerPtr beta_row_initializer;
};

inline 
NodeVertexPtr create_node_vertex(b_index parent_node_vertex, int vertex, bool is_consequent, bool is_negation, bool has_filter, int salience, BetaRowInitializerPtr beta_row_initializer_p)
{
  return std::make_shared<NodeVertex>(parent_node_vertex, vertex, is_consequent, is_negation, has_filter, salience, beta_row_initializer_p);
}

} // namespace jets::rete
#endif // JETS_RETE_NODE_VERTEX_H
