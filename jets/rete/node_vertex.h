#ifndef JETS_RETE_NODE_VERTEX_H
#define JETS_RETE_NODE_VERTEX_H

#include <atomic>
#include <cstdint>
#include <string>
#include <memory>
#include <list>
#include <vector>

#include "absl/container/btree_set.h"

#include "../rete/beta_row_initializer.h"

// This file contains metadata data structure for BetaRelations
namespace jets::rete {
// //////////////////////////////////////////////////////////////////////////////////////
// NodeVertex class -- metadata obj describing a BetaRelation node in the rete graph
// 
// `NodeVertex::antecedent_query_key` attribute is to specify the query key of the antecedent
// which is required to query the matching row when a triple is added / deleted triple
// from the inferred graph.
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
using b_index_set = absl::btree_set<b_index>;

// Set<int> representing the set of consequent AlphaNode's vertex for each NodeVertex
// This is set by the ReteMetaStore::initialize() method
using consequent_set = absl::btree_set<int>;

/**
 * @brief NodeVertex holding metadata information about a BetaRelation node
 * 
 * Note the child_nodes and consequent_alpha_vertexes properties are set after 
 * construction by ReteMetaStore as part of metadata initialization routine.
 */
struct NodeVertex {

  NodeVertex()
    : key(0),
      parent_node_vertex(nullptr),
      child_nodes(),
      consequent_alpha_vertexes(),
      vertex(0),
      is_negation(false),
      salience(0),
      filter_expr(),
      normalized_label(),
      beta_row_initializer(),
      antecedent_query_key(0),
      records_consequents(false)
  {}

  NodeVertex(
    b_index parent_node_vertex, 
    int key,
    int vertex, 
    bool is_negation, 
    int salience, 
    ExprBasePtr filter_expr,
    std::string_view normalized_label,
    BetaRowInitializerPtr beta_row_initializer) 
    : key(key),
      parent_node_vertex(parent_node_vertex),
      child_nodes(),
      consequent_alpha_vertexes(),
      vertex(vertex),
      is_negation(is_negation),
      salience(salience),
      filter_expr(filter_expr),
      normalized_label(normalized_label),
      beta_row_initializer(beta_row_initializer),
      antecedent_query_key(0),
      tid_(0),
      records_consequents(false)
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

  inline std::string const&
  get_normalized_label()const
  {
    return normalized_label;
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

  // Whether the rows of this vertex count their supports (support_key_positions)
  inline bool
  counts_supports()const
  {
    return not support_key_positions.empty();
  }

  inline int
  get_next_tid()const
  {
    return ++this->tid_;
  }

  int                      key;
  b_index                  parent_node_vertex;
  b_index_set              child_nodes;
  consequent_set           consequent_alpha_vertexes;
  int                      vertex;
  bool                     is_negation;
  int                      salience;
  ExprBasePtr              filter_expr;
  std::string              normalized_label;
  BetaRowInitializerPtr    beta_row_initializer;
  int                      antecedent_query_key;     // set by ReteMetaStore::initialize
  mutable int              tid_;
  // Set by ReteMetaStore::initialize when a consequent term of this vertex mints a
  // resource (AlphaNode::mints_resource). A beta row of such a vertex records the
  // triples it inferred, and retraction replays that record rather than recomputing
  // the consequents -- recomputing create_entity 0 mints a second entity and inserts
  // a stray jets:key, and recomputing a keyed one re-inserts its jets:key.
  bool                     records_consequents;
  // Set by ReteMetaStore::initialize: the positions of the parent's row that this
  // vertex's row does not carry. The compiler leaves out of a vertex's row every
  // variable bound above it that neither its triple nor anything below reads (the
  // pruned variables), so parent rows that differ only there make one row of this
  // vertex. Their values are what tells those derivations apart, and a row of a vertex
  // with any keeps the set of them that support it (BetaRelation's support sets), so
  // that withdrawing one derivation leaves a row another still makes. Empty for a
  // vertex whose row carries its parent's whole row, which is one derivation per row.
  std::vector<int>         support_key_positions;
  // The peak row count of this vertex's beta relation in the last rete session that
  // released one, a size hint for the next session's relation (BetaRelation::
  // reserve_from_hint). The meta store is shared by the sessions of every pool worker:
  // a relaxed atomic, read once per session at the relation's first visit and stored
  // once at the session's end. Which session stored last can vary between runs; it
  // only sizes a table.
  mutable std::atomic<std::uint32_t> beta_rows_hint{0};
};

inline std::ostream & operator<<(std::ostream & out, b_index node)
{
  if(not node) out << "NULL";
  else {
    int parent_vertex = node->parent_node_vertex?node->parent_node_vertex->vertex:0;
    out << "NodeVertex: key "<< node->key <<
      ", vertex "<<node->vertex <<", parent vertex "<<parent_vertex <<
      ", "<<node->normalized_label <<
      ", negation? "<<node->is_negation <<", salience "<<node->salience<<
      ", antecedent_query_key "<<node->antecedent_query_key<<
      ", children {";
    bool is_first = true;
    for(auto child: node->child_nodes) {
      if(not is_first) out << ", ";
      is_first = false;
      out << child->vertex;
    }
    out << "}, consequents {";
    is_first = true;
    for(auto consequent_vertex: node->consequent_alpha_vertexes) {
      if(not is_first) out << ", ";
      is_first = false;
      out << consequent_vertex;
    }
    out << "}";
  }
  return out;
}

inline std::ostream & operator<<(std::ostream & out, NodeVertexPtr node)
{
  if(not node) out << "NULL";
  else {
    out << node.get();
  }
  return out;
}

inline 
NodeVertexPtr create_node_vertex(
  b_index parent_node_vertex, int key, int vertex, bool is_negation, int salience, ExprBasePtr filter,
  std::string_view normalized_label, BetaRowInitializerPtr beta_row_initializer)
{
  return std::make_shared<NodeVertex>(parent_node_vertex, key, vertex, 
    is_negation, salience, filter, normalized_label, beta_row_initializer);
}

} // namespace jets::rete
#endif // JETS_RETE_NODE_VERTEX_H
