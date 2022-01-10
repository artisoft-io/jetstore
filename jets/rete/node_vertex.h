#ifndef JETS_RETE_NODE_VERTEX_H
#define JETS_RETE_NODE_VERTEX_H

#include <string>
#include <memory>
#include <list>

#include "absl/container/flat_hash_set.h"

#include "jets/rete/beta_row_initializer.h"

// This file contains metadata data structure for BetaRelations
namespace jets::rete {
// //////////////////////////////////////////////////////////////////////////////////////
// NodeVertex class -- metadata obj describing a BetaRelation node in the rete graph
// 
// `NodeVertex::antecedent_query_key` attribute is to specify the query the antecedent
// requires to query the matching row for an added / deleted triple to the inferred 
// graph.
// Note the BetaRelation will have has many query specs as there are child beta nodes.
// It is possible that 2 child beta node have the same query spec, that would be the
// case if two child nodes are different only by the filter term.
// --------------------------------------------------------------------------------------
// Forward definition
class ExprBase;
using ExprBasePtr = std::shared_ptr<ExprBase>;

struct NodeVertex;
using NodeVertexPtr = std::shared_ptr<NodeVertex>;
using b_index = NodeVertex const *;

// Reversed lookup for descendent nodes to speed up insert/delete in indexes struct
using b_index_set = absl::flat_hash_set<b_index>;

// Set<int> representing the set of consequent AlphaNode's vertex for each NodeVertex
// This is set by the ReteMetaStore::initialize() method
using consequent_set = absl::flat_hash_set<int>;

/**
 * @brief NodeVertex holding metadata information about a BetaRelation node
 * 
 * Note the child_nodes and consequent_alpha_vertexes properties are set after 
 * construction by ReteMetaStore as part of metadata initialization routine.
 */
struct NodeVertex {

  NodeVertex()
    : parent_node_vertex(nullptr),
      child_nodes(),
      consequent_alpha_vertexes(),
      vertex(0),
      is_negation(false),
      salience(0),
      filter_expr(),
      beta_row_initializer(),
      antecedent_query_key(0)
  {}

  NodeVertex(
    b_index parent_node_vertex, 
    int vertex, 
    bool is_negation, 
    int salience, 
    ExprBasePtr filter_expr,
    BetaRowInitializerPtr beta_row_initializer) 
    : parent_node_vertex(parent_node_vertex),
      child_nodes(),
      consequent_alpha_vertexes(),
      vertex(vertex),
      is_negation(is_negation),
      salience(salience),
      filter_expr(filter_expr),
      beta_row_initializer(beta_row_initializer),
      antecedent_query_key(0)
  {}

  inline bool
  is_head_vertice()const
  {
    return vertex == 0;
  }

  inline bool
  has_expr()const
  {
    return filter_expr.use_count() > 0;
  }

  inline BetaRowInitializer const*
  get_beta_row_initializer()const
  {
    if(not beta_row_initializer) return nullptr;
    return beta_row_initializer.get();
  }

  inline bool 
  has_consequent_terms()const
  {
    return not consequent_alpha_vertexes.empty();
  }

  b_index                  parent_node_vertex;
  b_index_set              child_nodes;
  consequent_set           consequent_alpha_vertexes;
  int                      vertex;
  bool                     is_negation;
  int                      salience;
  ExprBasePtr              filter_expr;
  BetaRowInitializerPtr    beta_row_initializer;
  mutable int              antecedent_query_key;
};

inline 
NodeVertexPtr create_node_vertex(
  b_index parent_node_vertex, int vertex, bool is_negation, int salience, ExprBasePtr filter,
  BetaRowInitializerPtr beta_row_initializer)
{
  return std::make_shared<NodeVertex>(parent_node_vertex, vertex, 
    is_negation, salience, filter, beta_row_initializer);
}

} // namespace jets::rete
#endif // JETS_RETE_NODE_VERTEX_H
