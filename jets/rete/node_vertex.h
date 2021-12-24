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
// `NodeVertex::antecedent_query_spec` attribute is to specify the query the antecedent
// requires to query the matching row for an added / deleted triple to the inferred 
// graph.
// Note the BetaRelation will have has many query specs as there are child beta nodes.
// It is possible that 2 child beta node have the same query spec, that would be the
// case if two child nodes are different only by the filter term.
// --------------------------------------------------------------------------------------
struct NodeVertex;
using NodeVertexPtr = std::shared_ptr<NodeVertex>;
using b_index = NodeVertex const *;

// AntecedentQuerySpec holding metadata to specify the query spec for the antecedent term
enum AntecedentQueryType {
  kQTAll    = 0,     // find()        -- equivalent to find all
  kQTu      = 1,     // find(u)
  kQTuv     = 2,     // find(u, v)
  kQTuvw    = 3,     // find(u, v, w) -- may return multiple BetaRow rows
};
struct AntecedentQuerySpec {
  int key;                    // key to register the query with BetaRelation indexes
  AntecedentQueryType type;   // Specify query function call to use and retained arguments
  char spin;                  // Specify rotation of arguments to be applied by antecedent: spo => uvw
  int u_pos;                  // BetaRow pos of u (in the uvw coordinates, i.e. rotated)
  int v_pos;                  // BetaRow pos of v (in the uvw coordinates, i.e. rotated)
  int w_pos;                  // BetaRow pos of w (in the uvw coordinates, i.e. rotated)
};
// Note u_pos is required for query type 1, 2, and 3
// Note v_pos is required for query type 2, and 3
// Note w_pos is required for query type 3 only
// Note u_pos, v_pos, and w_pos take value -1 when not specified
using AntecedentQuerySpecPtr = std::shared_ptr<AntecedentQuerySpec>;

// Reversed lookup for descendent nodes to speed up insert/delete in indexes struct
using b_index_set = absl::flat_hash_set<b_index>;

// NodeVertex holding metadata information about a BetaRelation node
struct NodeVertex {

  NodeVertex()
    : parent_node_vertex(nullptr),
    child_nodes(),
      vertex(0),
      has_consequent_terms(false),
      is_negation(false),
      expr_vertex(-1),
      salience(0),
      beta_row_initializer(),
      antecedent_query_spec()
  {}

  NodeVertex(
    b_index parent_node_vertex, 
    b_index_set child_nodes,
    int vertex, 
    bool has_consequent_terms, 
    bool is_negation, 
    int expr_vertex, 
    int salience, 
    BetaRowInitializerPtr beta_row_initializer,
    AntecedentQuerySpecPtr antecedent_query_spec) 
    : parent_node_vertex(parent_node_vertex),
      child_nodes(child_nodes),
      vertex(vertex),
      has_consequent_terms(has_consequent_terms),
      is_negation(is_negation),
      expr_vertex(expr_vertex),
      salience(salience),
      beta_row_initializer(beta_row_initializer),
      antecedent_query_spec(antecedent_query_spec)
  {}

  inline bool
  has_expr()const
  {
    return expr_vertex >= 0;
  }

  inline BetaRowInitializer const*
  get_beta_row_initializer()const
  {
    if(not beta_row_initializer) return nullptr;
    return beta_row_initializer.get();
  }

  b_index                  parent_node_vertex;
  b_index_set              child_nodes;
  int                      vertex;
  bool                     has_consequent_terms;
  bool                     is_negation;
  int                      expr_vertex;
  int                      salience;
  BetaRowInitializerPtr    beta_row_initializer;
  AntecedentQuerySpecPtr   antecedent_query_spec;
};

inline 
NodeVertexPtr create_node_vertex(
  b_index parent_node_vertex, int vertex, bool has_consequent_terms, bool is_negation, int expr_vertex, int salience, 
  BetaRowInitializerPtr beta_row_initializer, AntecedentQuerySpecPtr antecedent_query_spec)
{
  return std::make_shared<NodeVertex>(parent_node_vertex, vertex, has_consequent_terms, is_negation, expr_vertex, salience, 
                                      beta_row_initializer, antecedent_query_spec);
}

} // namespace jets::rete
#endif // JETS_RETE_NODE_VERTEX_H
