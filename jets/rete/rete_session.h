#ifndef JETS_RETE_RETE_SESSION_H
#define JETS_RETE_RETE_SESSION_H

#include <queue>
#include <string>
#include <memory>
#include <list>
#include <vector>

#include "absl/hash/hash.h"
#include "absl/container/flat_hash_set.h"

#include "../rdf/rdf_types.h"
#include "../rete/node_vertex.h"
#include "../rete/beta_row_initializer.h"
#include "../rete/beta_row.h"
#include "../rete/beta_row_iterator.h"
#include "../rete/beta_relation.h"
#include "../rete/graph_callback_mgr_impl.h"
#include "../rete/alpha_node.h"
#include "../rete/expr.h"
#include "../rete/rete_meta_store.h"


// Component to manage all the rdf resources and literals of a graph
namespace jets::rete {
// //////////////////////////////////////////////////////////////////////////////////////
// ReteSession class -- main session class for the rete network
// --------------------------------------------------------------------------------------
class ReteSession;
using ReteSessionPtr = std::shared_ptr<ReteSession>;

using BetaRelationVector = std::vector<BetaRelationPtr>;
using AlphaNodeVector = std::vector<AlphaNodePtr>;

struct BetaRowPriorityCompare {
  inline bool
  operator()(BetaRowPtr const& lhs, BetaRowPtr const& rhs) {
    auto const& lv = lhs->get_node_vertex();
    auto const& rv = rhs->get_node_vertex();
    if(lv->salience == rv->salience) {
      return lv->vertex > rv->vertex;
    }
    return lv->salience < rv->salience;
  }
};
using BetaRowPriorityQueue = std::priority_queue<BetaRowPtr, std::vector<BetaRowPtr>, BetaRowPriorityCompare>;

/**
 * @brief ReteSession making the rete network
 * 
 */
class ReteSession {
 public:
  using Iterator = rdf::RDFSession::Iterator;

  ReteSession(ReteMetaStorePtr rule_ms, rdf::RDFSession * rdf_session) 
    : rule_ms_(rule_ms),
      rdf_session_(rdf_session),
      beta_relations_(),
      pending_compute_consequent_beta_rows_(),
      err_msg_()
    {}

  inline rdf::RDFSession *
  rdf_session()
  {
    return rdf_session_;
  }

  inline ReteMetaStore const*
  rule_ms()const
  {
    return rule_ms_.get();
  }

  inline BetaRelation *
  get_beta_relation(int vertex)
  {
    if(vertex<0 or vertex>= static_cast<int>(beta_relations_.size())) return nullptr;
    return beta_relations_[vertex].get();
  }

  inline BetaRelationVector const&
  beta_relations()const
  {
    return beta_relations_;
  }
  
  int 
  execute_rules();

  char const* execute_rules2(int*v);

  /**
   * @brief Notification function called when a triple is added to the inferred graph
   * 
   * @param vertex key of NodeVertex that registered the call back
   * @param s triple's subject inserted
   * @param p triple's predicate inserted
   * @param o triple's object inserted
   * @return int 
   */
  inline int
  triple_inserted(int vertex, rdf::r_index s, rdf::r_index p, rdf::r_index o)
  {
    return triple_updated(vertex, s, p, o, true);
  }

  /**
   * @brief Notification function called when a triple is deleted from the inferred graph
   * 
   * @param vertex key of NodeVertex that registered the call back
   * @param s triple's subject deleted
   * @param p triple's predicate deleted
   * @param o triple's object deleted
   * @return int 
   */
  inline int
  triple_deleted(int vertex, rdf::r_index s, rdf::r_index p, rdf::r_index o)
  {
    return triple_updated(vertex, s, p, o, false);
  }

  /**
   * @brief Notification function called when a triple is added to the inferred graph
   * 
   * This is called by callback manager created for rule term filter
   * 
   * @param vertex key of NodeVertex that registered the call back
   * @param s triple's subject inserted
   * @param p triple's predicate inserted
   * @param o triple's object inserted
   * @return int 
   */
  inline int
  triple_inserted_for_filters(int vertex, rdf::r_index s, rdf::r_index p, rdf::r_index o)
  {
    return triple_updated_for_filters(vertex, s, p, o, true);
  }

  /**
   * @brief Notification function called when a triple is deleted from the inferred graph
   * 
   * This is called by callback manager created for rule term filter
   * 
   * @param vertex key of NodeVertex that registered the call back
   * @param s triple's subject deleted
   * @param p triple's predicate deleted
   * @param o triple's object deleted
   * @return int 
   */
  inline int
  triple_deleted_for_filters(int vertex, rdf::r_index s, rdf::r_index p, rdf::r_index o)
  {
    return triple_updated_for_filters(vertex, s, p, o, false);
  }

  /**
   * @brief Initialize ReteSession using ReteMetaStore
   *
   *  - Initialize BetaRelationVector beta_relations_ such that
   *    `beta_relations_[ipos] = create_beta_node(rule_ms_->node_vertexes_[ipos]);`
   *  - Register GraphCallbackManager using antecedent AlphaNode adaptor
   * 
   * @param rule_ms ReteMetaStore to use for the ReteSession
   * 
   * @return int 
   */
  int
  initialize();

  /**
   * @brief Terminate the ReteSession, remove dependency on RdfSession
   * 
   * @return int 0 if normal
   */
  int
  terminate();

 protected:
  int
  set_graph_callbacks();

  int
  remove_graph_callbacks();

  /**
   * @brief Notification for triple inserted/deleted
   * 
   * @param vertex key of NodeVertex that registered the call back
   * @param s triple's subject 
   * @param p triple's predicate 
   * @param o triple's object 
   * @param is_inserted true if inserted, false for deleted triple
   * @return int 
   */
  int
  triple_updated(int vertex, rdf::r_index s, rdf::r_index p, rdf::r_index o, bool is_inserted);

  /**
   * @brief Notification for triple inserted/deleted for rule filter terms
   * 
   * @param vertex key of NodeVertex that registered the call back
   * @param s triple's subject 
   * @param p triple's predicate 
   * @param o triple's object 
   * @param is_inserted true if inserted, false for deleted triple
   * @return int 
   */
  int
  triple_updated_for_filters(int vertex, rdf::r_index s, rdf::r_index p, rdf::r_index o, bool is_inserted);

  /**
   * @brief Execute inferrence on rete graph
   * 
   * @param from_vertex start vertex node
   * @param is_inferring apply forward chaining if true, otherwise retract inferrence
   * @param compute_consequents add inferred triples to rdf session
   * @return int 0 for normal, -1 if error
   */
  int
  execute_rules(int from_vertex, bool is_inferring, bool compute_consequents);

/**
 * @brief Visit the Rete Graph and apply inferrence or retactation of inferred facts
 * 
 * Perform DFS graph visitation starting at node `from_vertex`
 *
 * @param from_vertex Starting point of graph visitation
 * @param is_inferring apply inferrence if true, retract inferrence if false
 * @return int 0 if normal, -1 if error
 */
  int 
  visit_rete_graph(int from_vertex, bool is_inferring);

  /**
   * @brief Schedule consequent terms of the rule associated with the `beta_row`
   * 
   * @param beta_row Inferred or retracted BetaRow
   * @return int 0 if normal, -1 if error
   */
  int
  schedule_consequent_terms(BetaRowPtr beta_row);

  /**
   * @brief Compute consequent triples from scheduled consequent terms
   *
   * Scheduled consequent terms are processed using a priority queue with
   * a priority based on rule salience.
   * Inferred triples are added to inferred graph of rdf session which
   * trigger rule having antecedent matching the inferred triple to
   * activate and in turn to infer or retract triples.
   *
   * TODO Add logging to trach which rule inferre which triple
   * to be able to explain how a triple got inferred.
   * 
   * @return int 0 for normal, -1 if error
   */
  int
  compute_consequent_triples();

 private:
 friend class BetaRelation;

  ReteMetaStorePtr        rule_ms_;
  rdf::RDFSession *       rdf_session_;
  BetaRelationVector      beta_relations_;
  BetaRowPriorityQueue    pending_compute_consequent_beta_rows_;
  std::string             err_msg_;
};

inline ReteSessionPtr create_rete_session(ReteMetaStorePtr rule_ms, 
  rdf::RDFSession * rdf_session)
{
  return std::make_shared<ReteSession>(rule_ms, rdf_session);
}

// //////////////////////////////////////////////////////////////////////////////////////
// BetaRelation methods
// --------------------------------------------------------------------------------------
inline int
BetaRelation::insert_beta_row(ReteSession * rete_session, BetaRowPtr beta_row)
{
  auto iret = this->all_beta_rows_.insert(beta_row);
  if(iret.second) {
    // beta_row inserted in set
    VLOG(56)<<"BetaRow insert row "<<beta_row;
    // schedule the consequent terms
    if(beta_row->get_node_vertex()->has_consequent_terms()) {
      // Flag row as new and pending to infer triples
      beta_row->set_status(BetaRowStatus::kInserted);
      rete_session->schedule_consequent_terms(beta_row);
    } else {

      // Mark row as done
      beta_row->set_status(BetaRowStatus::kProcessed);
    }

  } else {
    beta_row = *(iret.first);  
    VLOG(56)<<"BetaRow insert (again) row "<<beta_row;
    // Check if row was maked as deleted
    if(beta_row->is_deleted()) {
      // Mark it as processed so it does not get retracted
      beta_row->set_status(BetaRowStatus::kProcessed);
    } else {
      // VLOG(50)<<"    Insert Beta Row at vertex "<<
      //   this->get_node_vertex()->vertex<<" ALREADY INSERTED, row "<<beta_row<<
      //   ", status is "<< beta_row->get_status()<<" skipping. . .";
      return 0;
    }    
  }

  if(this->get_node_vertex()->child_nodes.empty()) return 0;

  // Add row to pending queue to notify child nodes
  this->pending_beta_rows_.push_back(beta_row);
  // Add/Restore row indexes
  if(this->node_vertex_->is_head_vertice()) return 0;
  for(auto const& child_node_vertex: node_vertex_->child_nodes) {
    auto alpha_node = rete_session->rule_ms()->get_alpha_node(child_node_vertex->vertex);
    alpha_node->index_beta_row(this, child_node_vertex, beta_row.get());
  }
  return 0;
}

inline int
BetaRelation::remove_beta_row(ReteSession * rete_session, BetaRowPtr beta_row)
{
  auto itor = this->all_beta_rows_.find(beta_row);
  if(itor==this->all_beta_rows_.end()) {
    // Already deleted!
    // VLOG(50)<<"    BetaRowPtr not found, must be already deleted.(D01)";
    return 0;
  }
  // make sure we point to the right instance
  beta_row = *itor;
  VLOG(56)<<"BetaRow remove row "<<beta_row;
  
  // Check if row was already deleted
  if(beta_row->is_deleted()) {
    // VLOG(50)<<"    Marked as deleted already";
    return 0;
  }

  // Check for consequent terms
  if(beta_row->get_node_vertex()->has_consequent_terms()) {
    // Check if status kInserted
    if(beta_row->is_inserted()) {
      // Row was marked kInserted, not inferred yet
      // Cancel row insertion **
      // VLOG(50)<<"    Row marked kInserted, not inferred yet ** Cancel row insertion **";
      beta_row->set_status(BetaRowStatus::kProcessed);

      // Put the row in the pending queue to notify children that this row is being deleted
      if(not this->get_node_vertex()->child_nodes.empty()) {
        this->pending_beta_rows_.push_back(beta_row);
        this->remove_indexes(rete_session, beta_row);
      }

      this->all_beta_rows_.erase(beta_row);
      return 0;
    }

    // VLOG(50)<<"    Row marked kProcessed, need to put it for delete/retract";
    // Row must be in kProcessed state -- need to put it for delete/retract
    beta_row->set_status(BetaRowStatus::kDeleted);

    // Put the row in the pending queue to notify children that this row is being deleted
    if(not this->get_node_vertex()->child_nodes.empty()) {
      this->pending_beta_rows_.push_back(beta_row);
      this->remove_indexes(rete_session, beta_row);
    }

    rete_session->schedule_consequent_terms(beta_row);

  } else {
    // No consequent terms, remove and propagate down
    beta_row->set_status(BetaRowStatus::kProcessed);

    // Put the row in the pending queue to notify children that this row is being deleted
    if(not this->get_node_vertex()->child_nodes.empty()) {
      this->pending_beta_rows_.push_back(beta_row);
      this->remove_indexes(rete_session, beta_row);
    }

    this->all_beta_rows_.erase(beta_row);
  }
  return 0;
}
  /**
   * @brief Initialize the BetaRelation object indexes
   * 
   * Allocate and initialize all 3 indexes
   *    - BetaRowIndxVec1
   *    - BetaRowIndxVec2
   *    - BetaRowIndxVec3
   * @return int 
   */
  // Defined in rete_session.h
  inline int
  BetaRelation::initialize(ReteSession * rete_session)
  {
    beta_row_idx1_.clear();
    beta_row_idx2_.clear();
    beta_row_idx3_.clear();
    for(auto const& child_node_vertex: this->node_vertex_->child_nodes) {
      auto child_alpha_node = rete_session->rule_ms()->get_alpha_node(child_node_vertex->vertex);
      child_alpha_node->initialize_indexes(this, child_node_vertex);
    }
    return 0;
  }

  inline void
  BetaRelation::remove_indexes(ReteSession * rete_session, BetaRowPtr beta_row)
  {
    // remove the indexes associated with the beta row
    for(auto const& child_node_vertex: this->node_vertex_->child_nodes) {
      auto alpha_node = rete_session->rule_ms()->get_alpha_node(child_node_vertex->vertex);
      alpha_node->remove_index_beta_row(this, child_node_vertex, beta_row.get());
    }
  }

// Declaired in graph_callback_mgr_impl.h
inline void
ReteCallBackImpl::triple_inserted(rdf::r_index s, rdf::r_index p, rdf::r_index o)const
{
  if(this->s_filter_ and this->s_filter_!=s) return;
  if(this->p_filter_ and this->p_filter_!=p) return;
  if(this->o_filter_ and this->o_filter_!=o) return;
  // If beta node is not activated yet, ignore the notification
  if(not this->rete_session_->get_beta_relation(this->vertex_)->is_activated()) {
    return;
  }
  this->rete_session_->triple_inserted(this->vertex_, s, p, o);
}
// Declaired in graph_callback_mgr_impl.h
inline void
ReteCallBackImpl::triple_deleted(rdf::r_index s, rdf::r_index p, rdf::r_index o)const
{
  if(this->s_filter_!=nullptr and this->s_filter_!=s) return;
  if(this->p_filter_!=nullptr and this->p_filter_!=p) return;
  if(this->o_filter_!=nullptr and this->o_filter_!=o) return;
  // If beta node is not activated yet, ignore the notification
  if(not this->rete_session_->get_beta_relation(this->vertex_)->is_activated()) {
    return;
  }
  this->rete_session_->triple_deleted(this->vertex_, s, p, o);
}


// from graph_callback_mgr_impl.h
inline void
ReteCallBack4VisitorsImpl::triple_inserted(rdf::r_index s, rdf::r_index p, rdf::r_index o)const
{
  if(this->p_filter_ and this->p_filter_!=p) return;
  // If beta node is not activated yet, ignore the notification
  if(not this->rete_session_->get_beta_relation(this->vertex_)->is_activated()) {
    return;
  }
  this->rete_session_->triple_inserted_for_filters(this->vertex_, s, p, o);
}
// from graph_callback_mgr_impl.h
inline void
ReteCallBack4VisitorsImpl::triple_deleted(rdf::r_index s, rdf::r_index p, rdf::r_index o)const
{
  if(this->p_filter_!=nullptr and this->p_filter_!=p) return;
  // If beta node is not activated yet, ignore the notification
  if(not this->rete_session_->get_beta_relation(this->vertex_)->is_activated()) {
    return;
  }
  this->rete_session_->triple_deleted_for_filters(this->vertex_, s, p, o);
}

} // namespace jets::rete
#endif // JETS_RETE_RETE_SESSION_H
