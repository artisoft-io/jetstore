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
  int key;                    // key to register the query with BetaRelation
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
    b_index parent_node_vertex, 
    int vertex, 
    bool is_consequent, 
    bool is_negation, 
    bool has_filter, 
    int salience, 
    BetaRowInitializerPtr beta_row_initializer,
    AntecedentQuerySpecPtr antecedent_query_spec) 
    : parent_node_vertex(parent_node_vertex),
      vertex(vertex),
      is_consequent(is_consequent),
      is_negation(is_negation),
      has_filter(has_filter),
      salience(salience),
      beta_row_initializer(beta_row_initializer),
      antecedent_query_spec(antecedent_query_spec)
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

  b_index                  parent_node_vertex;
  int                      vertex;
  bool                     is_consequent;
  bool                     is_negation;
  bool                     has_filter;
  int                      salience;
  BetaRowInitializerPtr    beta_row_initializer;
  AntecedentQuerySpecPtr   antecedent_query_spec;
};

inline 
NodeVertexPtr create_node_vertex(
  b_index parent_node_vertex, int vertex, bool is_consequent, bool is_negation, bool has_filter, int salience, 
  BetaRowInitializerPtr beta_row_initializer, AntecedentQuerySpecPtr antecedent_query_spec)
{
  return std::make_shared<NodeVertex>(parent_node_vertex, vertex, is_consequent, is_negation, has_filter, salience, 
                                      beta_row_initializer, antecedent_query_spec);
}

} // namespace jets::rete
#endif // JETS_RETE_NODE_VERTEX_H
