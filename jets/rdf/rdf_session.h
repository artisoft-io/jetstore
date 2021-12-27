#ifndef JETS_RDF_SESSION_H
#define JETS_RDF_SESSION_H

#include <string>
#include <memory>

#include <boost/variant/multivisitors.hpp>

#include "absl/hash/hash.h"
#include <glog/logging.h>

#include "jets/rdf/rdf_err.h"
#include "jets/rdf/r_manager.h"
#include "jets/rdf/rdf_graph.h"
#include "jets/rdf/rdf_session_iterator.h"
#include "rdf_ast.h"

namespace jets::rdf {
/////////////////////////////////////////////////////////////////////////////////////////
// rdf session is composed of 3 rdf graphs:
//    - meta graph that is read-only and shared among sessions
//    - asserted graph containing the triples comming from the input source.
//    - inferred graph containing the inferred triples.
/////////////////////////////////////////////////////////////////////////////////////////
// RDFSession 
template<class Graph>
class RDFSession {
 public:
  using RDFGraph = Graph;
  using RDFGraphPtr = std::shared_ptr<Graph>;
  using Iterator = RDFSessionIterator<typename Graph::Iterator>;

  RDFSession() = delete;

  /**
   * @brief Construct a new RDFSession object
   * Create asserted graph using the `RManager` of the meta graph as root mgr.
   * Create the inferred graph sharing the same `RManager` of the asserted
   * graph
   * 
   * @param meta_graph 
   */
  RDFSession(RDFGraphPtr meta_graph) 
    : meta_graph_(meta_graph), 
      asserted_graph_(), 
      inferred_graph_()
    {
      auto meta_mgr_p = meta_graph_->get_rmgr();
      asserted_graph_ = std::make_shared<Graph>(meta_mgr_p);
      auto r_mgr_p = asserted_graph_->get_rmgr();
      inferred_graph_ = std::make_shared<Graph>();
      inferred_graph_->set_rmgr(r_mgr_p);
    }

  /**
   * @brief the number of triples in all graphs
   * 
   * @return int the meta + asserted + inferred graph size
   */
  inline int size() const{
    return meta_graph_->size() + asserted_graph_->size() + inferred_graph_->size();
  }

  /**
   * @brief Get the `RManager` shared ptr
   * 
   * @return RManagerPtr 
   */
  inline typename Graph::RManagerPtr get_rmgr()const {
    return asserted_graph_->get_rmgr();
  }

  /**
   * @brief Get the meta graph shared ptr
   */
  inline RDFGraphPtr get_meta_graph()const {
    return meta_graph_;
  }

  /**
   * @brief Get the asserted graph shared ptr
   */
  inline RDFGraphPtr get_asserted_graph()const {
    return asserted_graph_;
  }

  /**
   * @brief Get the inferred graph shared ptr
   */
  inline RDFGraphPtr get_inferred_graph()const {
    return inferred_graph_;
  }

  inline bool contains(r_index s, r_index p, r_index o) const {
    return 
        asserted_graph_->contains(s, p, o) or
        inferred_graph_->contains(s, p, o) or
        meta_graph_->contains(s, p, o) ;
  }
  
  inline bool contains_sp(r_index s, r_index p) const {
    return 
        asserted_graph_->contains_sp(s, p) or
        inferred_graph_->contains_sp(s, p) or
        meta_graph_->contains_sp(s, p) ;
  }
  // ------------------------------------------------------------------------------------
  // find methods
  // ------------------------------------------------------------------------------------
  inline Iterator find() const 
  {
    return Iterator(
      asserted_graph_->find(),
      inferred_graph_->find(),
      meta_graph_->find()
    );
  }

  inline Iterator find(r_index s) const 
  {
    return Iterator(
      asserted_graph_->find(s),
      inferred_graph_->find(s),
      meta_graph_->find(s)
    );
  }

  inline Iterator find(r_index s, r_index p) const 
  {
    return Iterator(
      asserted_graph_->find(s, p),
      inferred_graph_->find(s, p),
      meta_graph_->find(s, p)
    );
  }

  inline Iterator find(AllOrRIndex const&s, AllOrRIndex const&p, AllOrRIndex const&o) 
  {
    return Iterator(
      asserted_graph_->find(s, p, o),
      inferred_graph_->find(s, p, o),
      meta_graph_->find(s, p, o)
    );
  }
  // ------------------------------------------------------------------------------------
  // insert methods
  // ------------------------------------------------------------------------------------
  template<typename L>
  inline typename literal_restrictor<L, int>::result
  insert(r_index s, r_index p, L const& v)
  {
    if(!s or !p) {
      LOG(ERROR) << "RDFSession::insert: trying to insert a triple with a null s or p index: (" 
                 << get_name(s) << ", " << get_name(p) <<")";
      return 0;
    }
    return asserted_graph_->insert(s, p, v);
  }

  template<typename L>
  inline typename literal_restrictor<L, int>::result
  insert(r_index s, r_index p, L && v)
  {
    if(!s or !p) {
      LOG(ERROR) << "RDFSession::insert: trying to insert a triple with a null s or p index: (" 
                 << get_name(s) << ", " << get_name(p) <<")";
      return 0;
    }
    return asserted_graph_->insert(s, p, std::forward<L>(v));
  }

  // insert triple (s, p, o), returns 1 if inserted zero otherwise
  inline int
  insert(r_index s, r_index p, r_index o)
  {
    if(!s or !p or !o) {
      LOG(ERROR) << "RDFSession::insert: trying to insert a triple with a null index (" 
                 << get_name(s) << ", " << get_name(p) << ", " << get_name(o) <<")";
      return 0;
    }
    return asserted_graph_->insert(s, p, o);
  }

  // insert triple (Triple(s, p, o)), returns 1 if inserted zero otherwise
  inline int
  insert(Triple t3)
  {
    if(!t3.subject or !t3.predicate or !t3.object) {
      LOG(ERROR) << "RDFSession::insert: trying to insert a triple with a null index (" 
                 << get_name(t3.subject) << ", " << get_name(t3.predicate) << ", " << get_name(t3.object) <<")";
      return 0;
    }
    return asserted_graph_->insert(t3.subject, t3.predicate, t3.object);
  }

  // insert triple (Triple(s, p, o)), returns 1 if inserted zero otherwise
  inline int
  insert(Triple const& t3)
  {
    if(!t3.subject or !t3.predicate or !t3.object) {
      LOG(ERROR) << "RDFSession::insert: trying to insert a triple with a null index (" 
                 << get_name(t3.subject) << ", " << get_name(t3.predicate) << ", " << get_name(t3.object) <<")";
      return 0;
    }
    return asserted_graph_->insert(t3.subject, t3.predicate, t3.object);
  }

  // insert triple (Triple(s, p, o)), returns 1 if inserted zero otherwise
  inline int
  insert(Triple && t3)
  {
    if(!t3.subject or !t3.predicate or !t3.object) {
      LOG(ERROR) << "RDFSession::insert: trying to insert a triple with a null index (" 
                 << get_name(t3.subject) << ", " << get_name(t3.predicate) << ", " << get_name(t3.object) <<")";
      return 0;
    }
    return asserted_graph_->insert(t3.subject, t3.predicate, t3.object);
  }
  // ------------------------------------------------------------------------------------
  // insert_inferred
  // ------------------------------------------------------------------------------------
  // insert triple (s, p, o) in graph containing inferred triples 
  template<typename L>
  inline typename literal_restrictor<L, int>::result
  insert_inferred(r_index s, r_index p, L const& v)
  {
    if(!s or !p) {
      LOG(ERROR) << "RDFSession::insert: trying to insert a triple with a null s or p index: (" 
                 << get_name(s) << ", " << get_name(p) <<")";
      return 0;
    }
    return inferred_graph_->insert(s, p, v);
  }

  // insert triple (s, p, o) in graph containing inferred triples 
  template<typename L>
  inline typename literal_restrictor<L, int>::result
  insert_inferred(r_index s, r_index p, L && v)
  {
    if(!s or !p) {
      LOG(ERROR) << "RDFSession::insert: trying to insert a triple with a null s or p index: (" 
                 << get_name(s) << ", " << get_name(p) <<")";
      return 0;
    }
    return inferred_graph_->insert(s, p, std::forward<L>(v));
  }

  // insert triple (s, p, o), returns 1 if inserted zero otherwise
  inline int
  insert_inferred(r_index s, r_index p, r_index o)
  {
    if(!s or !p or !o) {
      LOG(ERROR) << "RDFSession::insert: trying to insert a triple with a null index (" 
                 << get_name(s) << ", " << get_name(p) << ", " << get_name(o) <<")";
      return 0;
    }
    return inferred_graph_->insert(s, p, o);
  }

  // insert triple (Triple(s, p, o)), returns 1 if inserted zero otherwise
  inline int
  insert_inferred(Triple t3)
  {
    if(!t3.subject or !t3.predicate or !t3.object) {
      LOG(ERROR) << "RDFSession::insert: trying to insert a triple with a null index (" 
                 << get_name(t3.subject) << ", " << get_name(t3.predicate) << ", " << get_name(t3.object) <<")";
      return 0;
    }
    return inferred_graph_->insert(t3.subject, t3.predicate, t3.object);
  }

  // insert triple (Triple(s, p, o)), returns 1 if inserted zero otherwise
  inline int
  insert_inferred(Triple const& t3)
  {
    if(!t3.subject or !t3.predicate or !t3.object) {
      LOG(ERROR) << "RDFSession::insert: trying to insert a triple with a null index (" 
                 << get_name(t3.subject) << ", " << get_name(t3.predicate) << ", " << get_name(t3.object) <<")";
      return 0;
    }
    return inferred_graph_->insert(t3.subject, t3.predicate, t3.object);
  }

  // insert triple (Triple(s, p, o)), returns 1 if inserted zero otherwise
  inline int
  insert_inferred(Triple && t3)
  {
    if(!t3.subject or !t3.predicate or !t3.object) {
      LOG(ERROR) << "RDFSession::insert: trying to insert a triple with a null index (" 
                 << get_name(t3.subject) << ", " << get_name(t3.predicate) << ", " << get_name(t3.object) <<")";
      return 0;
    }
    return inferred_graph_->insert(t3.subject, t3.predicate, t3.object);
  }
  // ------------------------------------------------------------------------------------
  // erase/retract methods
  // ------------------------------------------------------------------------------------
  // erase triple (s, p, o) from asserted and inferred graphs, return 1 if erased
  inline int
  erase(r_index s, r_index p, r_index o, bool notify_listners=true)
  {
    if(!s or !p or !o) {
      LOG(ERROR) << "RDFSession::erase: trying to erase a triple with a null index (" 
                 << get_name(s) << ", " << get_name(p) << ", " << get_name(o) <<")";
      return 0;
    }
    bool erased = asserted_graph_->erase(s, p, o, notify_listners);
    erased = inferred_graph_->erase(s, p, o, notify_listners) or erased;
    return erase;
  }

  // insert triple (Triple(s, p, o)), returns 1 if inserted zero otherwise
  inline int
  erase(Triple t3)
  {
    return erase(t3.subject, t3.predicate, t3.object);
  }

  // insert triple (Triple(s, p, o)), returns 1 if inserted zero otherwise
  inline int
  erase(Triple const& t3)
  {
    return erase(t3.subject, t3.predicate, t3.object);
  }

  // insert triple (Triple(s, p, o)), returns 1 if inserted zero otherwise
  inline int
  erase(Triple && t3)
  {
    return erase(t3.subject, t3.predicate, t3.object);
  }

  // retract triple (s, p, o) from graph, return 1 if actually erased
  inline int
  retract(r_index s, r_index p, r_index o, bool notify_listners=true)
  {
    if(!s or !p or !o) {
      LOG(ERROR) << "RDFSession::erase: trying to erase a triple with a null index (" 
                 << get_name(s) << ", " << get_name(p) << ", " << get_name(o) <<")";
      return 0;
    }
    bool erased = asserted_graph_->retract(s, p, o, notify_listners);
    erased = inferred_graph_->retract(s, p, o, notify_listners) or erased;
    return erase;
  }

  // insert triple (Triple(s, p, o)), returns 1 if inserted zero otherwise
  inline int
  retract(Triple t3)
  {
    return retract(t3.subject, t3.predicate, t3.object);
  }

  // insert triple (Triple(s, p, o)), returns 1 if inserted zero otherwise
  inline int
  retract(Triple const& t3)
  {
    return retract(t3.subject, t3.predicate, t3.object);
  }

  // insert triple (Triple(s, p, o)), returns 1 if inserted zero otherwise
  inline int
  retract(Triple && t3)
  {
    return retract(t3.subject, t3.predicate, t3.object);
  }

  // Access to specific graphs for ReteSession
  inline RDFGraph *
  meta_graph()
  {
    return this->meta_graph_.get();
  }
  inline RDFGraph *
  asserted_graph()
  {
    return this->asserted_graph_.get();
  }
  inline RDFGraph *
  inferred_graph()
  {
    return this->inferred_graph_.get();
  }

 protected:

 private:
  // friend class find_visitor<RDFSession>;

  RDFGraphPtr meta_graph_;
  RDFGraphPtr asserted_graph_;
  RDFGraphPtr inferred_graph_;
};

template<class Graph>
RDFSessionPtr<Graph> create_rdf_session(std::shared_ptr<Graph> g)
{
  return std::make_shared<RDFSession<Graph>>(g);
}

} // namespace jets::rdf
#endif // JETS_RDF_SESSION_H
