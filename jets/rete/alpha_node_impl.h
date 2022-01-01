#ifndef JETS_RETE_ALPHA_NODE_IMPL_H
#define JETS_RETE_ALPHA_NODE_IMPL_H

#include <string>
#include <memory>

#include "jets/rdf/rdf_types.h"
#include "node_vertex.h"
#include "jets/rete/beta_row.h"
#include "jets/rete/beta_row_iterator.h"
#include "jets/rete/beta_relation.h"
#include "jets/rete/graph_callback_mgr_impl.h"
#include "jets/rete/alpha_node.h"
#include "jets/rete/rete_session.h"

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

  AlphaNodeImpl(b_index node_vertex, bool is_antecedent,
    Fu const&fu, Fv const&fv, Fw const&fw) 
    : AlphaNode(node_vertex, is_antecedent),fu_(fu),fv_(fv),fw_(fw)
  {}

  AlphaNodeImpl(b_index node_vertex, bool is_antecedent,
    Fu &&fu, Fv &&fv, Fw &&fw) 
    : AlphaNode(node_vertex, is_antecedent),
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
    if(fu_.is_var()) {
      if(fv_.is_var()) {
        if(fw_.is_var()) {
          // case (*, *, *)
          // register to spo for all triples
          rete_session->rdf_session()->inferred_graph()->register_callback('s', vertex, nullptr, nullptr, nullptr);
        } else if(fw_.is_index()) {
          // case (*, *, i)
          rete_session->rdf_session()->inferred_graph()->register_callback('s', vertex, nullptr, nullptr, nullptr);
        } else {
          // case (*, *, r)
          rete_session->rdf_session()->inferred_graph()->register_callback('o', vertex, fw_.get_cst(), nullptr, nullptr);
        }
      } else if(fv_.is_index()) {
        if(fw_.is_var()) {
          // case (*, i, *)
          rete_session->rdf_session()->inferred_graph()->register_callback('s', vertex, nullptr, nullptr, nullptr);
        } else if(fw_.is_index()) {
          // case (*, i, i)
          rete_session->rdf_session()->inferred_graph()->register_callback('s', vertex, nullptr, nullptr, nullptr);
        } else {
          // case (*, i, r)
          rete_session->rdf_session()->inferred_graph()->register_callback('o', vertex, fw_.get_cst(), nullptr, nullptr);
        }
      } else {
        if(fw_.is_var()) {
          // case (*, r, *)
          rete_session->rdf_session()->inferred_graph()->register_callback('p', vertex, fv_.get_cst(), nullptr, nullptr);
        } else if(fw_.is_index()) {
          // case (*, r, i)
          rete_session->rdf_session()->inferred_graph()->register_callback('p', vertex, fv_.get_cst(), nullptr, nullptr);
        } else {
          // case (*, r, r)
          rete_session->rdf_session()->inferred_graph()->register_callback('p', vertex, fv_.get_cst(), fw_.get_cst(), nullptr);
        }
      }

    } else if(fu_.is_index()) {
      if(fv_.is_var()) {
        if(fw_.is_var()) {
          // case (i, *, *)
          rete_session->rdf_session()->inferred_graph()->register_callback('s', vertex, nullptr, nullptr, nullptr);
        } else if(fw_.is_index()) {
          // case (i, *, i)
          rete_session->rdf_session()->inferred_graph()->register_callback('s', vertex, nullptr, nullptr, nullptr);
        } else {
          // case (i, *, r)
          rete_session->rdf_session()->inferred_graph()->register_callback('o', vertex, fw_.get_cst(), nullptr, nullptr);
        }
      } else if(fv_.is_index()) {
        if(fw_.is_var()) {
          // case (i, i, *)
          rete_session->rdf_session()->inferred_graph()->register_callback('s', vertex, nullptr, nullptr, nullptr);
        } else if(fw_.is_index()) {
          // case (i, i, i)
          rete_session->rdf_session()->inferred_graph()->register_callback('s', vertex, nullptr, nullptr, nullptr);
        } else {
          // case (i, i, r)
          rete_session->rdf_session()->inferred_graph()->register_callback('o', vertex, fw_.get_cst(), nullptr, nullptr);
        }
      } else {
        if(fw_.is_var()) {
          // case (i, r, *)
          rete_session->rdf_session()->inferred_graph()->register_callback('p', vertex, fv_.get_cst(), nullptr, nullptr);
        } else if(fw_.is_index()) {
          // case (i, r, i)
          rete_session->rdf_session()->inferred_graph()->register_callback('p', vertex, fv_.get_cst(), nullptr, nullptr);
        } else {
          // case (i, r, r)
          rete_session->rdf_session()->inferred_graph()->register_callback('p', vertex, fv_.get_cst(), fw_.get_cst(), nullptr);
        }
      }
    } else {
      if(fv_.is_var()) {
        if(fw_.is_var()) {
          // case (r, *, *)
          rete_session->rdf_session()->inferred_graph()->register_callback('s', vertex, fu_.get_cst(), nullptr, nullptr);
        } else if(fw_.is_index()) {
          // case (r, *, i)
          rete_session->rdf_session()->inferred_graph()->register_callback('s', vertex, fu_.get_cst(), nullptr, nullptr);
        } else {
          // case (r, *, r)
          rete_session->rdf_session()->inferred_graph()->register_callback('o', vertex, fw_.get_cst(), fu_.get_cst(), nullptr);
        }
      } else if(fv_.is_index()) {
        if(fw_.is_var()) {
          // case (r, i, *)
          rete_session->rdf_session()->inferred_graph()->register_callback('s', vertex, fu_.get_cst(), nullptr, nullptr);
        } else if(fw_.is_index()) {
          // case (r, i, i)
          rete_session->rdf_session()->inferred_graph()->register_callback('s', vertex, fu_.get_cst(), nullptr, nullptr);
        } else {
          // case (r, i, r)
          rete_session->rdf_session()->inferred_graph()->register_callback('o', vertex, fw_.get_cst(), fu_.get_cst(), nullptr);
        }
      } else {
        if(fw_.is_var()) {
          // case (r, r, *)
          rete_session->rdf_session()->inferred_graph()->register_callback('s', vertex, fu_.get_cst(), fv_.get_cst(), nullptr);
        } else if(fw_.is_index()) {
          rete_session->rdf_session()->inferred_graph()->register_callback('s', vertex, fu_.get_cst(), fv_.get_cst(), nullptr);
          // case (r, r, i)
        } else {
          // case (r, r, r)
          rete_session->rdf_session()->inferred_graph()->register_callback('s', vertex, fu_.get_cst(), fv_.get_cst(), fw_.get_cst());
        }
      }
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
   * @return AlphaNode::Iterator 
   */
  typename AlphaNode::Iterator
  find_matching_triples(rdf::RDFSession * rdf_session, 
    BetaRow const* parent_row)const override
  {
    return rdf_session->find(fu_.eval(parent_row), fv_.eval(parent_row), fw_.eval(parent_row));
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
      RETE_EXCEPTION("AlphaNodeImpl::find_matching_rows: Called on alpha node that is NOT an antecedent term, vertex: "<<this->get_node_vertex()->vertex);
    }
    // Get QuerySpec from NodeVertex
    AntecedentQuerySpecPtr const& query_spec = this->get_node_vertex()->antecedent_query_spec;
    rdf::r_index u, v, w;
    rdf::lookup_spo2uvw(query_spec->spin, s, p, o, u, v, w);
    switch (query_spec->type) {
    case AntecedentQueryType::kQTu: 
      return parent_beta_relation->get_idx1_rows_iterator(query_spec->key, u);

    case AntecedentQueryType::kQTuv:
      return parent_beta_relation->get_idx2_rows_iterator(query_spec->key, u, v);

    case AntecedentQueryType::kQTuvw:
      return parent_beta_relation->get_idx3_rows_iterator(query_spec->key, u, v, w);

    case AntecedentQueryType::kQTAll:
      return parent_beta_relation->get_all_rows_iterator();
    }
    return {};
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

template<class Fu, class Fv, class Fw>
AlphaNodePtr create_alpha_node(b_index node_vertex, bool is_antecedent,
    Fu const& fu, Fv const& fv, Fw const& fw)
{
  return std::make_shared<AlphaNodeImpl<Fu,Fv,Fw>>(node_vertex, is_antecedent, fu, fv, fw);
}

template<class Fu, class Fv, class Fw>
AlphaNodePtr create_alpha_node(b_index node_vertex, bool is_antecedent,
    Fu && fu, Fv && fv, Fw && fw)
{
  return std::make_shared<AlphaNodeImpl<Fu,Fv,Fw>>(node_vertex, is_antecedent, 
    std::forward<Fu>(fu), std::forward<Fv>(fv), std::forward<Fw>(fw));
}

} // namespace jets::rete
#endif // JETS_RETE_ALPHA_NODE_IMPL_H
