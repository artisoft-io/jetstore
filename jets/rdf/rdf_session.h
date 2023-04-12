#ifndef JETS_RDF_SESSION_H
#define JETS_RDF_SESSION_H

#include <string>
#include <memory>
#include <list>
#include <ostream>

#include <boost/variant/multivisitors.hpp>

#include "absl/hash/hash.h"
#include <glog/logging.h>

#include "../rdf/rdf_err.h"
#include "../rdf/rdf_ast.h"
#include "../rdf/r_manager.h"
#include "../rdf/rdf_graph.h"
#include "../rdf/rdf_session_iterator.h"

namespace jets::rdf {
/////////////////////////////////////////////////////////////////////////////////////////
/**
 * @brief RDFSession is the working memory used by the rule engine
 * 
 * rdf session is composed of 3 rdf graphs:
 *    - meta graph that is read-only and shared among sessions
 *    - asserted graph containing the triples comming from the input source.
 *    - inferred graph containing the inferred triples.
 */
std::ostream & operator<<(std::ostream & out, RDFSession const* g);
class RDFSession {
 public:
  using Iterator = RDFSessionIterator;

  RDFSession() = delete;
 protected:
  /**
   * @brief Construct a new RDFSession object
   * Create asserted graph using the `RManager` of the meta graph as root mgr.
   * Create the inferred graph sharing the same `RManager` of the asserted
   * graph
   * 
   * @param meta_graph 
   */
  inline
  RDFSession(RDFGraphPtr meta_graph) 
    : meta_graph_(meta_graph), 
      asserted_graph_(), 
      inferred_graph_(),
      graph_buf_()
    {
      auto meta_mgr = meta_graph_->get_rmgr();
      asserted_graph_ = create_rdf_graph(meta_mgr, 1);
      auto r_mgr_p = asserted_graph_->get_rmgr();
      inferred_graph_ = create_rdf_graph(2);
      inferred_graph_->set_rmgr(r_mgr_p);
    }

 public:
  static RDFSessionPtr create(RDFGraphPtr meta_graph)
  {
    if(meta_graph) {
      meta_graph->set_locked();
    } else {
      LOG(ERROR) << "create_rdf_session: meta_graph argument is required and cannot be null";
      RDF_EXCEPTION("create_rdf_session: meta_graph argument is required and cannot be null");
    }
    struct make_shared_enabler: public RDFSession {
      make_shared_enabler(RDFGraphPtr meta_graph): RDFSession(meta_graph){}
    };
    return std::make_shared<make_shared_enabler>(meta_graph);
  }

  static RDFSession * create_raw_ptr(RDFGraphPtr meta_graph)
  {
    if(meta_graph) {
      meta_graph->set_locked();
    } else {
      LOG(ERROR) << "create_rdf_session: meta_graph argument is required and cannot be null";
      return nullptr;
    }
    return new RDFSession(meta_graph);
  }

  /**
   * @brief the number of triples in all graphs
   * 
   * @return int the meta + asserted + inferred graph size
   */
  inline int 
  size() const
  {
    return meta_graph_->size() + asserted_graph_->size() + inferred_graph_->size();
  }

  /**
   * @brief Get the `RManager` shared ptr
   * 
   * @return RManagerPtr 
   */
  inline RManagerPtr 
  get_rmgr()const 
  {
    return asserted_graph_->get_rmgr();
  }

  /**
   * @brief Get the `RManager` raw ptr
   * 
   * @return RManager const *
   */
  inline RManager const*
  rmgr()const 
  {
    return asserted_graph_->rmgr();
  }

  /**
   * @brief Get the `RManager` raw ptr
   * 
   * @return RManager *
   */
  inline RManager *
  rmgr()
  {
    return asserted_graph_->rmgr();
  }

  /**
   * @brief Get the meta graph shared ptr
   */
  inline RDFGraphPtr 
  get_meta_graph()const 
  {
    return meta_graph_;
  }

  /**
   * @brief Get the asserted graph shared ptr
   */
  inline RDFGraphPtr 
  get_asserted_graph()const 
  {
    return asserted_graph_;
  }

  /**
   * @brief Get the inferred graph shared ptr
   */
  inline RDFGraphPtr 
  get_inferred_graph()const 
  {
    return inferred_graph_;
  }

  inline bool 
  contains(r_index s, r_index p, r_index o) const 
  {
    return 
        asserted_graph_->contains(s, p, o) or
        inferred_graph_->contains(s, p, o) or
        meta_graph_->contains(s, p, o) ;
  }

  inline int
  erase(r_index s, r_index p, r_index o) const 
  {
    return 
        asserted_graph_->erase(s, p, o) +
        inferred_graph_->erase(s, p, o);
  }
  
  inline bool 
  contains_sp(r_index s, r_index p) const 
  {
    return 
        asserted_graph_->contains_sp(s, p) or
        inferred_graph_->contains_sp(s, p) or
        meta_graph_->contains_sp(s, p) ;
  }
  // ------------------------------------------------------------------------------------
  // get_object methods
  // ------------------------------------------------------------------------------------
  inline r_index
  get_object(r_index s, r_index p) const 
  {
    auto itor = this->find(s, p);
    if(!itor.is_end()) return itor.get_object();
    return nullptr;
  }
  // ------------------------------------------------------------------------------------
  // find methods
  // ------------------------------------------------------------------------------------
  inline Iterator 
  find() const 
  {
    return Iterator(
      asserted_graph_->find(),
      inferred_graph_->find(),
      meta_graph_->find()
    );
  }

  inline Iterator *
  new_find() const 
  {
    return new Iterator(
      asserted_graph_->find(),
      inferred_graph_->find(),
      meta_graph_->find()
    );
  }

  inline Iterator *
  new_find(r_index s, r_index p, r_index o) const 
  {
    AllOrRIndex s_, p_, o_;
    if(s) s_ = s;
    if(p) p_ = p;
    if(o) o_ = o;
    return new Iterator(
      asserted_graph_->find(s_, p_, o_),
      inferred_graph_->find(s_, p_, o_),
      meta_graph_->find(s_, p_, o_)
    );
  }

  inline Iterator *
  new_find(r_index s) const 
  {
    return new Iterator(
      asserted_graph_->find(s),
      inferred_graph_->find(s),
      meta_graph_->find(s)
    );
  }

  inline Iterator *
  new_find(r_index s, r_index p) const 
  {
    return new Iterator(
      asserted_graph_->find(s, p),
      inferred_graph_->find(s, p),
      meta_graph_->find(s, p)
    );
  }

  inline Iterator 
  find(r_index s) const 
  {
    return Iterator(
      asserted_graph_->find(s),
      inferred_graph_->find(s),
      meta_graph_->find(s)
    );
  }

  inline Iterator 
  find(r_index s, r_index p) const 
  {
    return Iterator(
      asserted_graph_->find(s, p),
      inferred_graph_->find(s, p),
      meta_graph_->find(s, p)
    );
  }

  inline Iterator 
  find(AllOrRIndex const&s, AllOrRIndex const&p, AllOrRIndex const&o) const
  {
    // std::cout<<"    RdfSession::find ("<<s<<", "<<p<<", "<<o<<")"<<std::endl;
    return Iterator(
      asserted_graph_->find(s, p, o),
      inferred_graph_->find(s, p, o),
      meta_graph_->find(s, p, o)
    );
  }

  inline Iterator 
  find_idx(r_index s, r_index p, r_index o) const 
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
      LOG(ERROR) << "RDFSession::insert: trying to insert a triple with a NULL ptr s or p index: (" 
                 << s << ", " << p << ")";
      RDF_EXCEPTION("RDFSession::insert: trying to insert a triple with a NULL ptr index (see logs)");
    }
    auto o = rmgr()->create_literal(v);
    if(this->meta_graph()->contains(s, p, o)) return 0;
    if (this->inferred_graph()) {
      this->inferred_graph()->erase(s, p, o);
    }
    return asserted_graph_->insert(s, p, o);
  }

  template<typename L>
  inline typename literal_restrictor<L, int>::result
  insert(r_index s, r_index p, L && v)
  {
    if(!s or !p) {
      LOG(ERROR) << "RDFSession::insert: trying to insert a triple with a NULL ptr s or p index: (" 
                 << s << ", " << p << ")";
      RDF_EXCEPTION("RDFSession::insert: trying to insert a triple with a NULL ptr index (see logs)");
    }
    auto o = rmgr()->create_literal(v);
    if(this->meta_graph()->contains(s, p, o)) return 0;
    if (this->inferred_graph()) {
      this->inferred_graph()->erase(s, p, o);
    }
    return asserted_graph_->insert(s, p, o);
  }

  // insert triple (s, p, o), returns 1 if inserted zero otherwise
  inline int
  insert(r_index s, r_index p, r_index o)
  {
    if(!s or !p or !o) {
      LOG(ERROR) << "RDFSession::insert: trying to insert a triple with a NULL ptr index (" 
                 << s << ", " << p << ", " << o <<")";
      RDF_EXCEPTION("RDFSession::insert: trying to insert a triple with a NULL ptr index (see logs)");
    }
    if(this->meta_graph()->contains(s, p, o)) return 0;
    if (this->inferred_graph()) {
      this->inferred_graph()->erase(s, p, o);
    }
    return asserted_graph_->insert(s, p, o);
  }

  // insert triple (Triple(s, p, o)), returns 1 if inserted zero otherwise
  inline int
  insert(Triple t3)
  {
    if(!t3.subject or !t3.predicate or !t3.object) {
      LOG(ERROR) << "RDFSession::insert: trying to insert a triple with a NULL ptr index (" 
                 << t3.subject << ", " << t3.predicate << ", " << t3.object <<")";
      RDF_EXCEPTION("RDFSession::insert: trying to insert a triple with a NULL ptr index (see logs)");
    }
    if(this->meta_graph()->contains(t3.subject, t3.predicate, t3.object)) return 0;
    if (this->inferred_graph()) {
      this->inferred_graph()->erase(t3.subject, t3.predicate, t3.object);
    }
    return asserted_graph_->insert(t3.subject, t3.predicate, t3.object);
  }

  // insert triple (Triple(s, p, o)), returns 1 if inserted zero otherwise
  inline int
  insert(Triple const& t3)
  {
    if(!t3.subject or !t3.predicate or !t3.object) {
      LOG(ERROR) << "RDFSession::insert: trying to insert a triple with a NULL ptr index (" 
                 << t3.subject << ", " << t3.predicate << ", " << t3.object <<")";
      RDF_EXCEPTION("RDFSession::insert: trying to insert a triple with a NULL ptr index (see logs)");
    }
    if(this->meta_graph()->contains(t3.subject, t3.predicate, t3.object)) return 0;
    if (this->inferred_graph()) {
      this->inferred_graph()->erase(t3.subject, t3.predicate, t3.object);
    }
    return asserted_graph_->insert(t3.subject, t3.predicate, t3.object);
  }

  // insert triple (Triple(s, p, o)), returns 1 if inserted zero otherwise
  inline int
  insert(Triple && t3)
  {
    if(!t3.subject or !t3.predicate or !t3.object) {
      LOG(ERROR) << "RDFSession::insert: trying to insert a triple with a NULL ptr index (" 
                 << t3.subject << ", " << t3.predicate << ", " << t3.object <<")";
      RDF_EXCEPTION("RDFSession::insert: trying to insert a triple with a NULL ptr index (see logs)");
    }
    if(this->meta_graph()->contains(t3.subject, t3.predicate, t3.object)) return 0;
    if (this->inferred_graph()) {
      this->inferred_graph()->erase(t3.subject, t3.predicate, t3.object);
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
      LOG(ERROR) << "RDFSession::insert: trying to insert a triple with a NULL ptr s or p index: (" 
                 << s << ", " << p <<")";
      RDF_EXCEPTION("RDFSession::insert: trying to insert a triple with a NULL ptr index (see logs)");
    }
    auto o = rmgr()->create_literal(v);
    if(this->meta_graph()->contains(s, p, o)) return 0;
    if(this->asserted_graph()->contains(s, p, o)) return 0;
    return inferred_graph_->insert(s, p, o);
  }

  // insert triple (s, p, o) in graph containing inferred triples 
  template<typename L>
  inline typename literal_restrictor<L, int>::result
  insert_inferred(r_index s, r_index p, L && v)
  {
    if(!s or !p) {
      LOG(ERROR) << "RDFSession::insert: trying to insert a triple with a NULL ptr s or p index: (" 
                 << s << ", " << p <<")";
      RDF_EXCEPTION("RDFSession::insert: trying to insert a triple with a NULL ptr index (see logs)");
    }
    auto o = rmgr()->create_literal(std::forward<L>(v));
    if(this->meta_graph()->contains(s, p, o)) return 0;
    if(this->asserted_graph()->contains(s, p, o)) return 0;
    return inferred_graph_->insert(s, p, o);
  }

  // insert triple (s, p, o), returns 1 if inserted zero otherwise
  inline int
  insert_inferred(r_index s, r_index p, r_index o)
  {
    if(!s or !p or !o) {
      LOG(ERROR) << "RDFSession::insert: trying to insert a triple with a NULL ptr index (" 
                 << s << ", " << p << ", " << o <<")";
      RDF_EXCEPTION("RDFSession::insert: trying to insert a triple with a NULL ptr index (see logs)");
    }
    if(this->meta_graph()->contains(s, p, o)) return 0;
    if(this->asserted_graph()->contains(s, p, o)) return 0;
    return inferred_graph_->insert(s, p, o);
  }

  // insert triple (Triple(s, p, o)), returns 1 if inserted zero otherwise
  inline int
  insert_inferred(Triple const& t3)
  {
    if(!t3.subject or !t3.predicate or !t3.object) {
      LOG(ERROR) << "RDFSession::insert: trying to insert a triple with a NULL ptr index (" 
                 << t3.subject << ", " << t3.predicate << ", " << t3.object <<")";
      RDF_EXCEPTION("RDFSession::insert: trying to insert a triple with a NULL ptr index (see logs)");
    }
    // std::cout<<"    RdfSession::insert_inferred "<<t3<<std::endl;
    if(this->meta_graph()->contains(t3.subject, t3.predicate, t3.object)) return 0;
    if(this->asserted_graph()->contains(t3.subject, t3.predicate, t3.object)) return 0;
    return inferred_graph_->insert(t3.subject, t3.predicate, t3.object);
  }

  // insert triple (Triple(s, p, o)), returns 1 if inserted zero otherwise
  inline int
  insert_inferred(Triple && t3)
  {
    if(!t3.subject or !t3.predicate or !t3.object) {
      LOG(ERROR) << "RDFSession::insert: trying to insert a triple with a NULL ptr index (" 
                 << t3.subject << ", " << t3.predicate << ", " << t3.object <<")";
      RDF_EXCEPTION("RDFSession::insert: trying to insert a triple with a NULL ptr index (see logs)");
    }
    // std::cout<<"    RdfSession::insert_inferred&& "<<t3<<std::endl;
    if(this->meta_graph()->contains(t3.subject, t3.predicate, t3.object)) return 0;
    if(this->asserted_graph()->contains(t3.subject, t3.predicate, t3.object)) return 0;
    return inferred_graph_->insert(t3.subject, t3.predicate, t3.object);
  }
  // ------------------------------------------------------------------------------------
  // erase/retract methods
  // ------------------------------------------------------------------------------------
  // erase triple (s, p, o) from asserted and inferred graphs, 
  // return count erased or -1 if error
  // Note triples in meta graph are never erased or inserted from the rete session, this
  // graph is read only.
  inline int
  erase(r_index s, r_index p, r_index o)
  {
    int res = asserted_graph_->erase(s, p, o) + inferred_graph_->erase(s, p, o);
    return res;
  }

  // retract triple (s, p, o) from inferred graph, reducing the reference count,
  //  return 1 if actually erased (ref count == 0)
  inline int
  retract(r_index s, r_index p, r_index o)
  {
    if(!s or !p or !o) {
      LOG(ERROR) << "RDFSession::erase: trying to erase a triple with a NULL ptr index (" 
                 << s << ", " << p << ", " << o <<")";
      RDF_EXCEPTION("RDFSession::insert: trying to insert a triple with a NULL ptr index (see logs)");
    }
    return inferred_graph_->retract(s, p, o);
  }

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

  // Access to graph_buf_, used by go client
  inline char const*
  get_graph_buf(int*v)
  {
    if(not v) return nullptr;
    try {
      std::ostringstream buf;
      buf << this << std::endl;
      this->graph_buf_ = buf.str();
    } catch (std::exception& err) {
      LOG(ERROR) << "RDFSession::get_graph_buf: error:"<<err.what();
      *v = -1;
      return nullptr;
    } catch (...) {
      LOG(ERROR) << "ReteSession::get_graph_buf: unknown error";
      *v = -1;
      return nullptr;
    }
    *v = 0;
    return graph_buf_.data();
  }

 protected:

 private:
  RDFGraphPtr meta_graph_;
  RDFGraphPtr asserted_graph_;
  RDFGraphPtr inferred_graph_;
  std::string graph_buf_;     // used by go to pull out the rdf session as text
};

inline RDFSessionPtr 
create_rdf_session(RDFGraphPtr g)
{
  return RDFSession::create(g);
}

inline std::ostream & operator<<(std::ostream & out, RDFSession const* g)
{
  if(not g) out << "NULL";
  else {
    std::list<std::string> triples;
    auto itor = g->find();
    while(not itor.is_end()) {
      triples.push_back(to_string(itor.as_triple()));
      itor.next();
    }
    triples.sort();
    for(auto const& item: triples) {
      out << item << std::endl;
    }
  }
  return out;
}

inline std::ostream & operator<<(std::ostream & out, RDFSessionPtr const& r)
{
  out << r.get();
  return out;
}

} // namespace jets::rdf
#endif // JETS_RDF_SESSION_H
