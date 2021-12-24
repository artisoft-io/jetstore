#ifndef JETS_RETE_ALPHA_NODE_H
#define JETS_RETE_ALPHA_NODE_H

#include <string>
#include <memory>
#include <list>

#include "jets/rdf/rdf_types.h"
#include "jets/rete/node_vertex.h"
#include "jets/rete/beta_row.h"
#include "jets/rete/beta_relation.h"
#include "jets/rete/beta_row_iterator.h"

// Component to manage all the rdf resources and literals of a graph
namespace jets::rete {
// Give an alias name to rdf::AllOrRIndex for use by rete engine
using rstar = rdf::AllOrRIndex;

// Triple class for convenience in some api -- using a different name for clarity
using RStar3 = rdf::TripleBase<rstar>;

// forward declaration
class BetaRelation;

// ======================================================================================
// Variant to query BetaRelation  as variant<PosMatch, r_index>
// -----------------------------------------------------------------------------
// Hold a column position
struct PosMatch 
{
  PosMatch(int pos): pos(pos) {}
  int pos;
};
inline std::ostream & operator<<(std::ostream & out, PosMatch const& r)
{
  out <<"<pos"<<r.pos<<">";
  return out;
}

enum pos_idx_ast_which_order {
    br_pos_t         = 0 ,
    br_r_index_t     = 1 
};

//* NOTE: If updated, MUST update ast_which_order and possibly ast_sort_order
// ======================================================================================
using PosOrRIndex = boost::variant< 
        PosMatch,
        rdf::r_index >;

inline PosOrRIndex make_pos(int pos){return PosMatch(pos);}
using RPos3 = rdf::TripleBase<PosOrRIndex>;

// //////////////////////////////////////////////////////////////////////////////////////
// AlphaNode class -- is a connector to the rdf graph for a antecedent or consequent term
//                     in the for of a triple (u, v, w) where
//                        - u and v can be ?s, or a constant
//                        - w can be ?s, constant, or and expression
// AlphaNode is a virtual base class, sub class are parametrized by functor: <Fu, Fv, Fw>
// --------------------------------------------------------------------------------------
template<class T>
class AlphaNode;

template<class T>
using AlphaNodePtr = std::shared_ptr<AlphaNode<T>>;

//
// AlphaNode making the rete network
template<class T>
class AlphaNode {
 public:
  using RDFSession = T;
  using RDFSessionPtr = std::shared_ptr<T>;
  using Iterator = typename T::Iterator;
  using RDFGraph = typename T::RDFGraph;
  using RDFGraphPtr = std::shared_ptr<RDFGraph>;

  AlphaNode()
    : node_vertex_(nullptr)
  {}

  explicit AlphaNode(b_index node_vertex) 
    : node_vertex_(node_vertex)
  {}

  virtual ~AlphaNode() 
  {}

  inline b_index
  get_node_vertex()const
  {
    return node_vertex_;
  }

  virtual bool
  is_antecedent()const=0;

  // Call to get all triples from rdf session matching `parent_row`
  // Applicable to antecedent terms only, call during initial graph visit only
  virtual Iterator
  find_matching_triples(RDFSession * rdf_session, BetaRow const* parent_row)const=0;

  // Called to query rows matching `triple`, 
  // case merging with new triples from inferred graph
  // Applicable to antecedent terms only
  virtual BetaRowIteratorPtr
  find_matching_rows(BetaRelation * beta_relation,  rdf::Triple * triple)const=0;

  // Return consequent `triple` for BetaRow
  // Applicable to consequent terms only
  virtual rdf::Triple
  compute_consequent_triple(BetaRow * beta_row)const=0;

 private:
  b_index         node_vertex_;
};

template<class T>
AlphaNodePtr<T> create_alpha_node(b_index node_vertex)
{
  return std::make_shared<AlphaNode<T>>(node_vertex);
}

} // namespace jets::rete
#endif // JETS_RETE_ALPHA_NODE_H
