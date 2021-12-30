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
  AntecedentQuerySpec(int key, AntecedentQueryType type, 
    char spin, int upos, int vpos, int wpos)
    : key(key), type(type), spin(spin), u_pos(upos), v_pos(vpos), w_pos(wpos)
  {}
  int key;                    // key to register the query with BetaRelation indexes, key start a 0 per type
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

inline 
AntecedentQuerySpecPtr create_antecedent_query_spec(
  int key, AntecedentQueryType type, 
  char spin, int upos, int vpos, int wpos)
{
  return std::make_shared<AntecedentQuerySpec>(key, type, spin, upos, vpos, wpos);
}

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
      expr_vertex(-1),
      salience(0),
      beta_row_initializer(),
      antecedent_query_spec()
  {}

  NodeVertex(
    b_index parent_node_vertex, 
    int vertex, 
    bool is_negation, 
    int expr_vertex, 
    int salience, 
    BetaRowInitializerPtr beta_row_initializer,
    AntecedentQuerySpecPtr antecedent_query_spec) 
    : parent_node_vertex(parent_node_vertex),
      child_nodes(),
      consequent_alpha_vertexes(),
      vertex(vertex),
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
  int                      expr_vertex;
  int                      salience;
  BetaRowInitializerPtr    beta_row_initializer;
  AntecedentQuerySpecPtr   antecedent_query_spec;
};

inline 
NodeVertexPtr create_node_vertex(
  b_index parent_node_vertex, int vertex, bool is_negation, int expr_vertex, int salience,
  BetaRowInitializerPtr beta_row_initializer, AntecedentQuerySpecPtr antecedent_query_spec)
{
  return std::make_shared<NodeVertex>(parent_node_vertex, vertex, 
    is_negation, expr_vertex, salience, beta_row_initializer, antecedent_query_spec);
}

} // namespace jets::rete
#endif // JETS_RETE_NODE_VERTEX_H
