#ifndef JETS_RDF_RDF_GRAPH_H
#define JETS_RDF_RDF_GRAPH_H

#include <string>
#include <memory>
#include <list>
#include <ostream>

#include <boost/variant/multivisitors.hpp>

#include "absl/hash/hash.h"
#include <glog/logging.h>

#include "../rdf/rdf_err.h"
#include "../rdf/rdf_ast.h"
#include "../rdf/base_graph_iterator.h"
#include "../rdf/base_graph.h"
#include "../rdf/graph_callback_mgr.h"
#include "../rdf/r_manager.h"

namespace jets::rdf {
// ======================================================================================
// find argument ast as variant<StarMatch, r_index>
// -----------------------------------------------------------------------------
struct StarMatch {};
inline std::ostream & operator<<(std::ostream & out, StarMatch const& r)
{
  out <<"*";
  return out;
}

enum star_idx_ast_which_order {
    rdf_star_t        = 0 ,
    rdf_r_index_t     = 1 
};

//* NOTE: If updated, MUST update ast_which_order and possibly ast_sort_order
// ======================================================================================
using AllOrRIndex = boost::variant< 
        StarMatch,
        r_index >;

inline AllOrRIndex make_any(){return StarMatch();}

// SearchTriple class for convenience in printing the seach criteria
using SearchTriple = TripleBase<AllOrRIndex>;
inline std::ostream & operator<<(std::ostream & out, SearchTriple const& t3)
{
  out << "("<<t3.subject<<","<<t3.predicate<<","<<t3.object<<")";
  return out;
}

inline std::string
to_string(SearchTriple const& t)
{
  std::ostringstream out;
  out << t;
  return out.str();
}

// find visitor defined after RDFGraph
struct find_visitor;

/////////////////////////////////////////////////////////////////////////////////////////
// RDFGraph
class RDFGraph;
using RDFGraphPtr = std::shared_ptr<RDFGraph>;

// RDFSession -- decl
class RDFSession;
using RDFSessionPtr = std::shared_ptr<RDFSession>;

// //////////////////////////////////////////////////////////////////////////////////////
/**
 * @brief RDFGraph is a fully indexed rdf graph with type (r_index, r_index, r_index)
 * 
 */
class RDFGraph {
 public:
  using Iterator = BaseGraph::Iterator;

  RDFGraph() 
    : size_(0),
      is_locked_(false),
      r_mgr_p(create_rmanager()), 
      spo_graph_('s'), 
      pos_graph_('p'), 
      osp_graph_('o'),
      graph_callback_mgr_(create_graph_callback_mgr())
    {}

  RDFGraph(RManagerPtr meta_mgr) 
    : size_(0),
      is_locked_(false),
      r_mgr_p(create_rmanager(meta_mgr)), 
      spo_graph_('s'), 
      pos_graph_('p'), 
      osp_graph_('o'),
      graph_callback_mgr_(create_graph_callback_mgr())
    {}

  /**
   * @brief the number of triples in the graph
   * 
   * @return int the nbr of triples in the graph
   */
  inline int size() const
  {
    return size_;
  }

  inline bool
  is_locked() const
  {
    return is_locked_;
  }

  inline void set_locked()
  {
    this->is_locked_ = true;
  }

  /**
   * @brief Get the `RManager` shared ptr
   * 
   * @return RManagerPtr 
   */
  inline RManagerPtr 
  get_rmgr()const 
  {
    return r_mgr_p;
  }

  /**
   * @brief Get the `RManager` raw ptr
   * 
   * @return RManager const*
   */
  inline RManager const*
  rmgr()const 
  {
    return r_mgr_p.get();
  }

  /**
   * @brief Get the `RManager` raw ptr
   * 
   * @return RManager *
   */
  inline RManager *
  rmgr()
  {
    return r_mgr_p.get();
  }

  inline bool 
  contains(r_index s, r_index p, r_index o) const 
  {
    return spo_graph_.contains(s, p, o);
  }
  
  inline bool 
  contains_sp(r_index s, r_index p) const 
  {
    return spo_graph_.contains(s, p);
  }

  // find methods
  inline Iterator 
  find() const 
  {
    return spo_graph_.find();
  }

  inline Iterator 
  find(r_index s) const 
  {
    return spo_graph_.find(s);
  }

  inline Iterator 
  find(r_index s, r_index p) const 
  {
    return spo_graph_.find(s, p);
  }

  // defined below after the find_visitor definition
  inline Iterator 
  find(AllOrRIndex const&s, AllOrRIndex const&p, AllOrRIndex const&o)const;

  /**
   * @brief DO not use! for benchmarking only
   * 
   * @param s 
   * @param p 
   * @param o 
   * @return Iterator 
   */
  inline Iterator 
  find_idx(r_index s, r_index p, r_index o) const 
  {
    if(s) {
      if(p) {
        if(o) {
          // case (r, r, r)
          return this->spo_graph_.find(s, p, o);
        } else {
          // case (r, r, *)
          return this->spo_graph_.find(s, p);
        }
      } else {
        if(o) {
          // case (r, *, r)
          return this->osp_graph_.find(o, s);
        } else {
          // case (r, *, *)
          return this->spo_graph_.find(s);
        }
      }
    } else {
      if(p) {
        if(o) {
          // case (*, r, r)
          return this->pos_graph_.find(p, o);
        } else {
          // case (*, r, *)
          return this->pos_graph_.find(p);
        }
      } else {
        if(o) {
          // case (*, *, r)
          return this->osp_graph_.find(o);
        } else {
          // case (*, *, *)
          return this->spo_graph_.find();
        }
      }
    }
  }

  // insert methods
  template<typename L>
  inline typename literal_restrictor<L, int>::result
  insert(r_index s, r_index p, L const& v)
  {
    if(is_locked_) throw rdf_exception("rdf graph locked, cannot mutate. "
      "This is probably a meta graph and you want to mutate the asserted"
      " of inferred graph of the redf session.");
    auto o = r_mgr_p->create_literal(v);
    return insert(s, p, o);
  }

  template<typename L>
  inline typename literal_restrictor<L, int>::result
  insert(r_index s, r_index p, L && v)
  {
    if(is_locked_) throw rdf_exception("rdf graph locked, cannot mutate. "
      "This is probably a meta graph and you want to mutate the asserted"
      " of inferred graph of the redf session.");
    auto o = r_mgr_p->create_literal(std::forward<L>(v));
    return insert(s, p, o);
  }

  // insert triple (s, p, o), returns 1 if inserted zero otherwise
  inline int
  insert(r_index s, r_index p, r_index o, bool notify_listners=true)
  {
    if(is_locked_) throw rdf_exception("rdf graph locked, cannot mutate. "
      "This is probably a meta graph and you want to mutate the asserted"
      " of inferred graph of the redf session.");
    if(!s or !p or !o) {
      LOG(ERROR) << "rdf_graph::insert: trying to insert a triple with a null index (" 
                 << get_name(s) << ", " << get_name(p) << ", " << get_name(o) <<")";
      return 0;
    }
    bool inserted = spo_graph_.insert(s, p, o);
    if(inserted) {
      pos_graph_.insert(p, o, s);
      osp_graph_.insert(o, s, p);
      size_+= 1;
      if(notify_listners) this->graph_callback_mgr_->triple_inserted(s, p, o);
      return 1;
    }
    return 0;
  }

  // erase triple (s, p, o) from graph, return 1 if erased
  inline int
  erase(r_index s, r_index p, r_index o, bool notify_listners=true)
  {
    if(is_locked_) throw rdf_exception("rdf graph locked, cannot mutate. "
      "This is probably a meta graph and you want to mutate the asserted"
      " of inferred graph of the redf session.");
    if(!s or !p or !o) {
      LOG(ERROR) << "rdf_graph::erase: trying to erase a triple with a null index (" 
                 << get_name(s) << ", " << get_name(p) << ", " << get_name(o) <<")";
      return 0;
    }
    bool erased = spo_graph_.erase(s, p, o);
    if(erased) {
      pos_graph_.erase(p, o, s);
      osp_graph_.erase(o, s, p);
      size_-= 1;
      if(notify_listners) this->graph_callback_mgr_->triple_deleted(s, p, o);
      return 1;
    }
    return 0;
  }

  // retract triple (s, p, o) from graph, return 1 if actually erased
  inline int
  retract(r_index s, r_index p, r_index o, bool notify_listners=true)
  {
    if(is_locked_) throw rdf_exception("rdf graph locked, cannot mutate. "
      "This is probably a meta graph and you want to mutate the asserted"
      " of inferred graph of the redf session.");
    if(!s or !p or !o) {
      LOG(ERROR) << "rdf_graph::erase: trying to erase a triple with a null index (" 
                 << get_name(s) << ", " << get_name(p) << ", " << get_name(o) <<")";
      return 0;
    }
    bool erased = spo_graph_.retract(s, p, o);
    if(erased) {
      pos_graph_.retract(p, o, s);
      osp_graph_.retract(o, s, p);
      size_-= 1;
      if(notify_listners) this->graph_callback_mgr_->triple_deleted(s, p, o);
      return 1;
    }
    return 0;
  }

  inline int
  register_callback(ReteCallBackPtr cb) 
  {
    this->graph_callback_mgr_->add_callback(cb);
    return 0;
  }

  inline void
  remove_all_callbacks()
  {
    this->graph_callback_mgr_->clear_callbacks();
  }

 protected:
 // set the `RManager`. This is called by `RDFSession` to ensure the asserted and inferred graph
 // share the same `RManager`.
inline void
set_rmgr(RManagerPtr p)
{
  r_mgr_p = p;
}

 private:
  friend struct find_visitor;
  friend class RDFSession;

  int      size_;
  bool     is_locked_;
  RManagerPtr r_mgr_p;
  BaseGraph spo_graph_;
  BaseGraph pos_graph_;
  BaseGraph osp_graph_;
  GraphCallbackManagerPtr graph_callback_mgr_;
};

// find visitor
struct find_visitor: public boost::static_visitor<RDFGraph::Iterator>
{
  using S = StarMatch;
  using R = r_index;
  using I = RDFGraph::Iterator;
  find_visitor(RDFGraph const* g) : g(g){}
  I operator()(S const&s, S const&p, S const&o){return g->spo_graph_.find();}
  I operator()(R const&s, S const&p, S const&o){return g->spo_graph_.find(s);}
  I operator()(R const&s, R const&p, S const&o){return g->spo_graph_.find(s, p);}
  I operator()(R const&s, R const&p, R const&o){return g->spo_graph_.find(s, p, o);}
  I operator()(S const&s, R const&p, S const&o){return g->pos_graph_.find(p);}
  I operator()(S const&s, R const&p, R const&o){return g->pos_graph_.find(p, o);}
  I operator()(S const&s, S const&p, R const&o){return g->osp_graph_.find(o);}
  I operator()(R const&s, S const&p, R const&o){return g->osp_graph_.find(o, s);}
  RDFGraph const*g;
};

RDFGraph::Iterator 
RDFGraph::find(AllOrRIndex const&s, AllOrRIndex const&p, AllOrRIndex const&o)const
{
  find_visitor v(this);
  return  boost::apply_visitor(v, s, p, o);
}

inline RDFGraphPtr 
create_rdf_graph(RManagerPtr meta_mgr=nullptr)
{
  return std::make_shared<RDFGraph>(meta_mgr);
}

inline std::ostream & operator<<(std::ostream & out, RDFGraph const* g)
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

inline std::ostream & operator<<(std::ostream & out, RDFGraphPtr const& r)
{
  out << r.get();
  return out;
}

} // namespace jets::rdf
#endif // JETS_RDF_RDF_GRAPH_H
