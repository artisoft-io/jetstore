#ifndef JETS_RETE_ALPHA_NODE_IMPL_H
#define JETS_RETE_ALPHA_NODE_IMPL_H

#include <string>
#include <memory>
#include <list>

#include "jets/rdf/rdf_types.h"
#include "node_vertex.h"
#include "jets/rete/beta_row_iterator.h"
#include "jets/rete/alpha_node.h"
#include "jets/rete/graph_callback_mgr_impl.h"

// This file contains the implementation classes for AlphaNode
namespace jets::rete {
// //////////////////////////////////////////////////////////////////////////////////////
// AlphaNode Implementation Class
// --------------------------------------------------------------------------------------
template<class T, class Fu, class Fv, class Fw>
class AlphaNodeImpl: public AlphaNode<T> {
 public:
  using AlphaNode<T>::RDFSession ;
  using AlphaNode<T>::RDFSessionPtr;
  using AlphaNode<T>::Iterator;
  using AlphaNode<T>::RDFGraph;
  using AlphaNode<T>::RDFGraphPtr;

  AlphaNodeImpl() = delete;
  // AlphaNodeImpl()
  //   : AlphaNode<T>(),fu_(),fv_(),fw_()
  // {}

  // AlphaNodeImpl(b_index node_vertex, bool is_antecedent,
  //   Fu fu, Fv fv, Fw fw) 
  //   : AlphaNode<T>(node_vertex, is_antecedent),fu_(fu),fv_(fv),fw_(fw)
  // {}

  AlphaNodeImpl(b_index node_vertex, bool is_antecedent,
    Fu const&fu, Fv const&fv, Fw const&fw) 
    : AlphaNode<T>(node_vertex, is_antecedent),fu_(fu),fv_(fv),fw_(fw)
  {}

  AlphaNodeImpl(b_index node_vertex, bool is_antecedent,
    Fu &&fu, Fv &&fv, Fw &&fw) 
    : AlphaNode<T>(node_vertex, is_antecedent),
      fu_(std::forward<Fu>(fu)),fv_(std::forward<Fv>(fv)),fw_(std::forward<Fw>(fw))
  {}

  virtual ~AlphaNodeImpl() 
  {}

  int
  register_callback(ReteSession<T> * rete_session, ReteCallBackList<T> * callbacks)const override
  {
    assert(rete_session);
    assert(callbacks);
    rdf::r_index s = fu_.to_cst();
    rdf::r_index p = fv_.to_cst();
    rdf::r_index o = fw_.to_cst();
    if(not s or not p or not o) {
      callbacks->push_back(ReteCallBack<T>{rete_session, this->get_node_vertex()->vertex});
    }
    return 0;
  }

  // Call to get all triples from rdf session matching `parent_row`
  // Applicable to antecedent terms only, call during initial graph visit only
  // Will throw if called on a consequent term
  /**
   * @brief Get all triples from rdf session matching `parent_row`
   * 
   * Applicable to antecedent terms only, call during initial graph visit only
   * @param rdf_session 
   * @param parent_row 
   * @return AlphaNode<T>::Iterator 
   */
  typename AlphaNode<T>::Iterator
  find_matching_triples(typename AlphaNode<T>::RDFSession * rdf_session, 
    BetaRow const* parent_row)const override
  {
    return rdf_session->find(fu_.eval(parent_row), fv_.eval(parent_row), fw_.eval(parent_row));
  }

  // Called to query rows matching `triple`, 
  // case merging with new triples from inferred graph
  // Applicable to antecedent terms only
  // Will throw if called on a consequent term
  /**
   * @brief Called to query rows matching `triple`, case merging with new triples from inferred graph
   * 
   * Applicable to antecedent terms only, therefore the Antecedent query is taken from beta node
   * @param beta_relation 
   * @param triple 
   * @return BetaRowIteratorPtr 
   */
  BetaRowIteratorPtr
  find_matching_rows(BetaRelation * beta_relation, rdf::r_index s, rdf::r_index p, rdf::r_index o)const override
  {
    b_index meta_vertex = beta_relation->get_node_vertex();
    // Get QuerySpec from beta_relation
    AntecedentQuerySpecPtr const& query_spec = meta_vertex->antecedent_query_spec;
    rdf::r_index u, v, w;
    rdf::lookup_spo2uvw(query_spec->spin, s, p, o, u, v, w);
    switch (query_spec->type) {
    case AntecedentQueryType::kQTu: 
      return beta_relation->get_idx1_rows_iterator(query_spec->key, u);

    case AntecedentQueryType::kQTuv:
      return beta_relation->get_idx2_rows_iterator(query_spec->key, u, v);

    case AntecedentQueryType::kQTuvw:
      return beta_relation->get_idx3_rows_iterator(query_spec->key, u, v, w);

    case AntecedentQueryType::kQTAll:
      return beta_relation->get_all_rows_iterator();
    }
  }

  // Return consequent `triple` for BetaRow
  // Applicable to consequent terms only
  // Will throw if called on an antecedent term
  /**
   * @brief Return consequent `triple` for BetaRow
   * 
   * Applicable to consequent terms only,
   * Will throw if called on an antecedent term
   * @param beta_row to apply index retrieval
   * @return rdf::Triple 
   */
  rdf::Triple
  compute_consequent_triple(BetaRow * beta_row)const override
  {
    return {fu_.eval(beta_row), fv_.eval(beta_row), fw_.eval(beta_row)};
  }

 private:
  Fu fu_;
  Fv fv_;
  Fw fw_;
};

// template<class T>
// AlphaNodePtr<T> create_alpha_node(b_index node_vertex)
// {
//   return std::make_shared<AlphaNode<T>>(node_vertex);
// }

} // namespace jets::rete
#endif // JETS_RETE_ALPHA_NODE_IMPL_H
