#ifndef JETS_RETE_ALPHA_NODE_H
#define JETS_RETE_ALPHA_NODE_H

#include <string>
#include <memory>
#include <list>

#include "../rdf/rdf_types.h"
#include "../rete/node_vertex.h"
#include "../rete/beta_row.h"
#include "../rete/beta_row_iterator.h"
#include "../rete/beta_relation.h"
#include "../rete/graph_callback_mgr_impl.h"

// Component representing the Alpha Node of a Rete Network
namespace jets::rete {
// Give an alias name to rdf::AllOrRIndex for use by rete engine
using rstar = rdf::AllOrRIndex;

// Triple class for convenience in some api -- using a different name for clarity
using RStar3 = rdf::TripleBase<rstar>;

// forward declaration
class BetaRelation;
class ReteSession;

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
//                        - w can be ?s, constant, or an expression
// AlphaNode is a virtual base class, sub class are parametrized by functor: <Fu, Fv, Fw>
// --------------------------------------------------------------------------------------
/**
 * @brief AphaNode is a connector to the rdf graph for a antecedents and consequents term.
 *
 * A beta relation is associated with a rule antecedent and possibly multiple
 * rule consequent terms.
 * Rule antecedent and consequent terms are represented by AlphaNodes.
 * As a result, there are more AlphaNodes than BetaRelations, however
 * each AlphaNode point to a NodeVertex corresponding to the associated
 * beta relation of the AlphaNode.
 *
 * Specialized AlphaNode classes exist to represent the various rule terms
 * the rete network must support. 
 * Two classes of AlphaNodes exists:
 *  - AlphaNodes representing Antecedent Terms
 *  - AlphaNodes representing Consequent Terms
 * In both cases, specialized AlphaNodes are parametrized with 3 functors:
 * AlphaNode<Fu, Fv, Fw>. The possible functor are different for 
 * antecedent and consequent terms.
 *
 * Possible functor for antecedent terms:
 *  - Fu: F_binded, F_var, F_cst
 *  - Fv: F_binded, F_var, F_cst
 *  - Fw: F_binded, F_var, F_cst
 *
 * Possible functor for consequent terms:
 *  - Fu: F_binded, F_cst
 *  - Fv: F_binded, F_cst
 *  - Fw: F_binded, F_cst, F_expr
 *
 * Description of each functor:
 *  - F_cst: Constant resource, such as rdf:type in: (?s rdf:type ?C)
 *  - F_var: A variable as ?s in: (?s rdf:type ?C)
 *  - F_binded: A binded variable to a previous term, such as ?C in: 
 *    (?s rdf:type ?C).(?C subclassOf Thing)
 *  - F_expr: An expression involving binded variables and constant terms.
 *
 * Note that consequent terms cannot have unbinded variable, so F_var
 * is not applicable.
 *
 * Note: AlphaNodes contains metadata information only and are managed by the
 *       ReteMetaStore.
 */
class AlphaNode;
using AlphaNodePtr = std::shared_ptr<AlphaNode>;

//
// AlphaNode making the rete network
class AlphaNode {
 public:
  using Iterator = rdf::RDFSession::Iterator;

  AlphaNode(b_index node_vertex, int key, bool is_antecedent, std::string_view normalized_label) 
    : node_vertex_(node_vertex), 
      key_(key), 
      is_antecedent_(is_antecedent),
      normalized_label_(normalized_label)
  {}

  virtual ~AlphaNode() 
  {}

  inline b_index
  get_node_vertex()const
  {
    return node_vertex_;
  }

  inline int
  get_key()const
  {
    return key_;
  }

  inline bool
  is_antecedent()const
  {
    return is_antecedent_;
  }

  inline std::string const&
  get_normalized_label()const
  {
    return normalized_label_;
  }

  /**
   * @brief Register callback with infer graph and construct AntecedentQuerySpec
   * 
   * @param rete_session 
   * @param callbacks 
   * @return int 
   */
  virtual int
  register_callback(ReteSession * rete_session)const=0;

  /**
   * @brief Get all triples from rdf session matching `parent_row`
   *
   * Invoking the functors to_AllOrRIndex methods, case:
   *  - F_cst: return the rdf resource of the functor (constant value)
   *  - F_binded: return the binded rdf resource from parent_row @ index of the functor.
   *  - F_var: return 'any' (StarMatch) to indicate a unbinded variable
   * 
   * Applicable to antecedent terms only, call during initial graph visit only
   * Will throw if called on a consequent term
   * @param rdf_session 
   * @param parent_row 
   * @return AlphaNode::Iterator 
   */
  virtual Iterator
  find_matching_triples(rdf::RDFSession * rdf_session, BetaRow const* parent_row)const=0;

  /**
   * @brief Return find statement as a `triple`
   * 
   * So far, this is used for diagnostics and printing.
   * This function is an alternative to find_matching_triples
   * Applicable to antecedent terms only,
   * Will throw if called on an antecedent term
   * @param parent_row BetaRow from parent beta node
   * @return SearchTriple
   */
  virtual rdf::SearchTriple
  compute_find_triple(BetaRow const* parent_row)const=0;

  /**
   * @brief Index beta_row in beta_relation indexes according to the functors template arguments
   * 
   * @param beta_relation BetaRelation with the indexes
   * @param beta_row  BetaRow to index
   */
  virtual void
  index_beta_row(BetaRelation * parent_beta_relation, b_index child_node_vertex, BetaRow const* beta_row)const=0;

  /**
   * @brief Remove index beta_row in beta_relation indexes according to the functors template arguments
   * 
   * @param beta_relation BetaRelation with the indexes
   * @param beta_row  BetaRow to index
   */
  virtual void
  remove_index_beta_row(BetaRelation * parent_beta_relation, b_index child_node_vertex, BetaRow const* beta_row)const=0;

  /**
   * @brief Initialize BetaRelation indexes for this child AlphaNode
   * 
   * @param beta_relation BetaRelation with the indexes
   */
  virtual void
  initialize_indexes(BetaRelation * parent_beta_relation, b_index child_node_vertex)const=0;

  /**
   * @brief Called to query rows from parent beta node matching `triple`, case merging with new triples from inferred graph
   *
   * The parent beta row is queried using the AntecedentQuerySpec from the current beta node,
   * that is the beta node with the same node vertex as the alpha node (since it's am antecedent term)
   * 
   * Applicable to antecedent terms only, will throw otherwise
   * @param parent_beta_relation 
   * @param triple 
   * @return BetaRowIteratorPtr 
   */
  virtual BetaRowIteratorPtr
  find_matching_rows(BetaRelation * parent_beta_relation,  rdf::r_index s, rdf::r_index p, rdf::r_index o)const=0;

  /**
   * @brief Return consequent `triple` for BetaRow
   * 
   * Applicable to consequent terms only,
   * Will throw if called on an antecedent term
   * @param beta_row to apply index retrieval
   * @return rdf::Triple 
   */
  virtual rdf::Triple
  compute_consequent_triple(ReteSession * rete_session, BetaRow const* beta_row)const=0;

  virtual std::ostream & describe(std::ostream & out)const=0;

 private:
  b_index     node_vertex_;
  int         key_;
  bool        is_antecedent_;
  std::string normalized_label_;
};

inline std::ostream & operator<<(std::ostream & out, AlphaNode const* node)
{
  if(not node) out << "NULL";
  else {
    node->describe(out);
  }
  return out;
}

} // namespace jets::rete
#endif // JETS_RETE_ALPHA_NODE_H
