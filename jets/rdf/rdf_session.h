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
    : meta_graph_p_(meta_graph), 
      asserted_graph_p_(), 
      inferred_graph_p_()
    {
      auto meta_mgr_p = meta_graph_p_->get_rmgr();
      asserted_graph_p_ = std::make_shared<Graph>(meta_mgr_p);
      auto r_mgr_p = asserted_graph_p_->get_rmgr();
      inferred_graph_p_ = std::make_shared<Graph>();
      inferred_graph_p_->set_rmgr(r_mgr_p);
    }

  /**
   * @brief the number of triples in all graphs
   * 
   * @return int the meta + asserted + inferred graph size
   */
  inline int size() const{
    return meta_graph_p_->size() + asserted_graph_p_->size() + inferred_graph_p_->size();
  }

  /**
   * @brief Get the `RManager` shared ptr
   * 
   * @return RManagerPtr 
   */
  inline typename Graph::RManagerPtr get_rmgr()const {
    return asserted_graph_p_->get_rmgr();
  }

  /**
   * @brief Get the meta graph shared ptr
   */
  inline RDFGraphPtr get_meta_graph()const {
    return meta_graph_p_;
  }

  /**
   * @brief Get the asserted graph shared ptr
   */
  inline RDFGraphPtr get_asserted_graph()const {
    return asserted_graph_p_;
  }

  /**
   * @brief Get the inferred graph shared ptr
   */
  inline RDFGraphPtr get_inferred_graph()const {
    return inferred_graph_p_;
  }

  inline bool contains(r_index s, r_index p, r_index o) const {
    return 
        asserted_graph_p_->contains(s, p, o) or
        inferred_graph_p_->contains(s, p, o) or
        meta_graph_p_->contains(s, p, o) ;
  }
  
  inline bool contains_sp(r_index s, r_index p) const {
    return 
        asserted_graph_p_->contains_sp(s, p) or
        inferred_graph_p_->contains_sp(s, p) or
        meta_graph_p_->contains_sp(s, p) ;
  }

  // find methods
  // -----------------------------------------------------------------------
  inline Iterator find() const 
  {
    return Iterator(
      asserted_graph_p_->find(),
      inferred_graph_p_->find(),
      meta_graph_p_->find()
    );
  }

  inline Iterator find(r_index s) const 
  {
    return Iterator(
      asserted_graph_p_->find(s),
      inferred_graph_p_->find(s),
      meta_graph_p_->find(s)
    );
  }

  inline Iterator find(r_index s, r_index p) const 
  {
    return Iterator(
      asserted_graph_p_->find(s, p),
      inferred_graph_p_->find(s, p),
      meta_graph_p_->find(s, p)
    );
  }

  inline Iterator find(AllOrRIndex const&s, AllOrRIndex const&p, AllOrRIndex const&o) 
  {
    return Iterator(
      asserted_graph_p_->find(s, p, o),
      inferred_graph_p_->find(s, p, o),
      meta_graph_p_->find(s, p, o)
    );
  }

  // insert methods
  template<typename L>
  inline typename literal_restrictor<L, int>::result
  insert(r_index s, r_index p, L const& v)
  {
    if(!s or !p) {
      LOG(ERROR) << "RDFSession::insert: trying to insert a triple with a null s or p index: (" 
                 << get_name(s) << ", " << get_name(p) <<")";
      return 0;
    }
    return asserted_graph_p_->insert(s, p, v);
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
    return asserted_graph_p_->insert(s, p, std::forward<L>(v));
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
    return asserted_graph_p_->insert(s, p, o);
  }

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
    return inferred_graph_p_->insert(s, p, v);
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
    return inferred_graph_p_->insert(s, p, std::forward<L>(v));
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
    return inferred_graph_p_->insert(s, p, o);
  }

  // erase triple (s, p, o) from asserted and inferred graphs, return 1 if erased
  inline int
  erase(r_index s, r_index p, r_index o, bool notify_listners=true)
  {
    if(!s or !p or !o) {
      LOG(ERROR) << "RDFSession::erase: trying to erase a triple with a null index (" 
                 << get_name(s) << ", " << get_name(p) << ", " << get_name(o) <<")";
      return 0;
    }
    bool erased = asserted_graph_p_.erase(s, p, o, notify_listners);
    erased = inferred_graph_p_.erase(s, p, o, notify_listners) or erased;
    return erase;
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
    bool erased = asserted_graph_p_.retract(s, p, o, notify_listners);
    erased = inferred_graph_p_.retract(s, p, o, notify_listners) or erased;
    return erase;
  }

 protected:

 private:
  // friend class find_visitor<RDFSession>;

  RDFGraphPtr meta_graph_p_;
  RDFGraphPtr asserted_graph_p_;
  RDFGraphPtr inferred_graph_p_;
};

template<class Graph>
RDFSessionPtr<Graph> create_rdf_session(std::shared_ptr<Graph> g)
{
  return std::make_shared<RDFSession<Graph>>(g);
}

} // namespace jets::rdf
#endif // JETS_RDF_SESSION_H
