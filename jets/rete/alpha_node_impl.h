#ifndef JETS_RETE_ALPHA_NODE_IMPL_H
#define JETS_RETE_ALPHA_NODE_IMPL_H

#include <string>
#include <memory>

#include <boost/variant/multivisitors.hpp>

#include "jets/rdf/rdf_types.h"
#include "jets/rete/node_vertex.h"
#include "jets/rete/beta_row.h"
#include "jets/rete/beta_row_iterator.h"
#include "jets/rete/beta_relation.h"
#include "jets/rete/graph_callback_mgr_impl.h"
#include "jets/rete/alpha_node.h"
#include "jets/rete/rete_session.h"
#include "jets/rete/antecedent_query_visitor.h"

// This file contains the implementation classes for AlphaNode
namespace jets::rete {
// //////////////////////////////////////////////////////////////////////////////////////
// AlphaNode Implementation Class
// --------------------------------------------------------------------------------------
template<class Fu, class Fv, class Fw>
class AlphaNodeImpl: public AlphaNode {
 public:
  using AlphaNode::Iterator;

  AlphaNodeImpl() = delete;

  AlphaNodeImpl(b_index node_vertex, int key, bool is_antecedent,
    Fu const&fu, Fv const&fv, Fw const&fw) 
    : AlphaNode(node_vertex, key, is_antecedent),fu_(fu),fv_(fv),fw_(fw)
  {}

  AlphaNodeImpl(b_index node_vertex, int key, bool is_antecedent,
    Fu &&fu, Fv &&fv, Fw &&fw) 
    : AlphaNode(node_vertex, key, is_antecedent),
      fu_(std::forward<Fu>(fu)),fv_(std::forward<Fv>(fv)),fw_(std::forward<Fw>(fw))
  {}

  virtual ~AlphaNodeImpl() 
  {}

  /**
   * @brief Register ReteCallBack functions to inferred rdf graph
   * 
   * Register ReteCallBack functions to inferred rdf graph
   * 
   * To regester the callback functions, need to spell out each case:
   * (*, *, *)
   * (*, r, *)
   * (*, r, r)
   * (*, i, r)
   * (*, i, i)
   * etc.
   * @param rete_session 
   * @return int 
   */
  int
  register_callback(ReteSession * rete_session)const override
  {
    if(this->get_node_vertex()->is_head_vertice()) return 0;
    
    int vertex = this->get_node_vertex()->vertex;
    rdf::r_index u = fu_.to_cst();
    rdf::r_index v = fv_.to_cst();
    rdf::r_index w = fw_.to_cst();
    std::cout<<"AlphaNode::register callback @ alpha node "<<get_key()<<" for vertex "<<vertex<<" with pattern "<<rdf::Triple(u, v, w)<<std::endl;
    rete_session->rdf_session()->inferred_graph()->register_callback(
      create_rete_callback(rete_session, vertex, u, v, w));
    return 0;
  }

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
  typename AlphaNode::Iterator
  find_matching_triples(rdf::RDFSession * rdf_session, 
    BetaRow const* parent_row)const override
  {
    if(not this->is_antecedent()) {
      RETE_EXCEPTION("AlphaNodeImpl::find_matching_triples: Called on alpha node "<<
        this->get_key()<<" that is NOT an antecedent term, vertex: "<<
        this->get_node_vertex()->vertex);
    }
    return rdf_session->find(fu_.to_AllOrRIndex(parent_row), fv_.to_AllOrRIndex(parent_row), 
      fw_.to_AllOrRIndex(parent_row));
  }

  /**
   * @brief Index beta_row in beta_relation indexes according to the functors template arguments
   * 
   * @param beta_relation BetaRelation with the indexes
   * @param beta_row  BetaRow to index
   */
  void
  index_beta_row(BetaRelation * beta_relation, BetaRow const* beta_row)const override
  {
    AQVIndexBetaRowsVisitor visitor(beta_relation, beta_relation->get_node_vertex(), beta_row);
    return boost::apply_visitor(visitor, fu_.to_AQV(), fv_.to_AQV(), fv_.to_AQV());
  }

  /**
   * @brief Remove index beta_row in beta_relation indexes according to the functors template arguments
   * 
   * @param beta_relation BetaRelation with the indexes
   * @param beta_row  BetaRow to index
   */
  void
  remove_index_beta_row(BetaRelation * beta_relation, BetaRow const* beta_row)const override
  {
    AQVRemoveIndexBetaRowsVisitor visitor(beta_relation, beta_relation->get_node_vertex(), beta_row);
    return boost::apply_visitor(visitor, fu_.to_AQV(), fv_.to_AQV(), fv_.to_AQV());
  }

  /**
   * @brief Initialize BetaRelation indexes for this child AlphaNode
   * 
   * @param beta_relation BetaRelation with the indexes
   */
  void
  initialize_indexes(BetaRelation * beta_relation)const override
  {
    AQVInitializeIndexesVisitor visitor(beta_relation, beta_relation->get_node_vertex());
    return boost::apply_visitor(visitor, fu_.to_AQV(), fv_.to_AQV(), fv_.to_AQV());
  }

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
  BetaRowIteratorPtr
  find_matching_rows(BetaRelation * parent_beta_relation, rdf::r_index s, rdf::r_index p, rdf::r_index o)const override
  {
    if(not this->is_antecedent()) {
      RETE_EXCEPTION("AlphaNodeImpl::find_matching_rows: Called on alpha node "<<
      this->get_key()<<" that is NOT an antecedent term, vertex: "<<
      this->get_node_vertex()->vertex);
    }
    AQVMatchingRowsVisitor visitor(parent_beta_relation, this->get_node_vertex(), s, p, o);
    return boost::apply_visitor(visitor, fu_.to_AQV(), fv_.to_AQV(), fv_.to_AQV());
  }

  /**
   * @brief Return consequent `triple` for BetaRow
   * 
   * Applicable to consequent terms only,
   * Will throw if called on an antecedent term
   * @param beta_row to apply index retrieval
   * @return rdf::Triple 
   */
  rdf::Triple
  compute_consequent_triple(ReteSession * rete_session, BetaRow const* beta_row)const override
  {
    if(this->is_antecedent()) {
      RETE_EXCEPTION("AlphaNodeImpl::compute_consequent_triple: Called on alpha node "<<
      this->get_key()<<" that is NOT a consequent term, vertex: "<<
      this->get_node_vertex()->vertex);
    }
    return {
      fu_.to_r_index(rete_session, beta_row), 
      fv_.to_r_index(rete_session, beta_row), 
      fw_.to_r_index(rete_session, beta_row)
    };
  }

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
  rdf::SearchTriple
  compute_find_triple(BetaRow const* parent_row)const override
  {
    if(not this->is_antecedent()) {
      RETE_EXCEPTION("AlphaNodeImpl::compute_find_triple: Called on alpha node "<<
      this->get_key()<<" that IS a consequent term, vertex: "<<
      this->get_node_vertex()->vertex);
    }
    return {
      fu_.to_AllOrRIndex(parent_row), 
      fv_.to_AllOrRIndex(parent_row), 
      fw_.to_AllOrRIndex(parent_row)
    };
  }

  std::ostream & 
  describe(std::ostream & out)const override
  {
    out << "AlphaNode: key "<< this->get_key() << 
      " "<<(this->is_antecedent()?"antecedent":"consequent") <<
      " ("<<this->fu_<<", "<<this->fv_<<", "<<this->fw_<<") ";
    return out;
  }

 private:
  Fu fu_;
  Fv fv_;
  Fw fw_;
};

template<class Fu, class Fv, class Fw>
AlphaNodePtr create_alpha_node(b_index node_vertex, int key, bool is_antecedent,
    Fu const& fu, Fv const& fv, Fw const& fw)
{
  return std::make_shared<AlphaNodeImpl<Fu,Fv,Fw>>(node_vertex, key, is_antecedent, fu, fv, fw);
}

template<class Fu, class Fv, class Fw>
AlphaNodePtr create_alpha_node(b_index node_vertex, int key, bool is_antecedent,
    Fu && fu, Fv && fv, Fw && fw)
{
  return std::make_shared<AlphaNodeImpl<Fu,Fv,Fw>>(node_vertex, key, is_antecedent, 
    std::forward<Fu>(fu), std::forward<Fv>(fv), std::forward<Fw>(fw));
}
} // namespace jets::rete
#endif // JETS_RETE_ALPHA_NODE_IMPL_H
