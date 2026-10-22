#ifndef JETS_RDF_BASE_GRAPH_H
#define JETS_RDF_BASE_GRAPH_H

#include <string>
#include <memory>
#include "absl/hash/hash.h"
#include <glog/logging.h>

#include "../rdf/rdf_ast.h"
#include "../rdf/w_node.h"
#include "../rdf/base_graph_iterator.h"
#include "../rdf/graph_callback_mgr.h"

namespace jets::rdf {
// //////////////////////////////////////////////////////////////////////////////////////
class BaseGraph;
using BaseGraphPtr = std::shared_ptr<BaseGraph>;

/**
 * @brief Class BaseGraph is an rdf graph
 * 
 * Class to manage a triple graph. The natural indexing to the graph is (u, v, w)
 * which is it's natural order. The natural indexing allow to iterate the element
 * according to: (u, v, *), (u, *, *), (*, *, *). 
 * In order to have the complementary indexes, a spin property indicate the maping 
 * of the indexes:
 * 
 *  's': (u, v, w)  =>  (s, p, o) 
 *  'p': (u, v, w)  =>  (p, o, s) 
 *  'o': (u, v, w)  =>  (o, s, p) 
 *
 * The graph structure, representing the triples: 
 *    (u, v, w) implemented as MAP(u, MAP(v, SET(WNode)))
 */
class BaseGraph {
 public:
  using Iterator = BaseGraphIterator;
  BaseGraph() = delete;
  /**
   * Constructor for BaseGraph. 
   * Spin indicate the triple rotation index, possible
   * values are 's', 'p', and 'o'
   *
   * @param spin indicates spin scheme
   * @param gtype indicates the type of graph (0: meta_graph, 1:asserted_graph, 2:inferred_graph)
   */
  inline BaseGraph(char const spin, int gtype)
    : spin_(spin),
      umap_data_(),
      gtype_(gtype),
      registry_()
  {}

  // An iterator that outlives its graph, such as one Go still holds when its session
  // is released, is detached and left at its end, so that using it reads nothing freed.
  inline ~BaseGraph()
  {
    auto * itor = registry_.head;
    while(itor) {
      auto * next = itor->next_;
      itor->registry_ = nullptr;
      itor->prev_ = itor->next_ = nullptr;
      itor->u_ = itor->v_ = itor->w_ = nullptr;
      itor->frozen_.reset();
      itor->frozen_pos_ = 0;
      itor = next;
    }
    registry_.head = nullptr;
  }

  inline void clear() 
  { 
    freeze_iterators();
    umap_data_.clear(); 
  }

  static char const* get_gtype(int gtype)
  {
    switch (gtype) {
    case 0: return "META";
    case 1: return "ASSERTED";
    case 2: return "INFERRED";
    default: return "ERROR";
    }
  }

  /**
   * @return true if (u, v, w) exist, false otherwise.
   */
  bool contains(r_index u, r_index v, r_index w) const 
  {
    auto utor = umap_data_.find(u);
    if (utor == umap_data_.end()) return false;

    auto vtor = utor->second.find(v);
    if (vtor == utor->second.end()) return false;

    return vtor->second.contains(WSetType::value_type{w});
  }

  /**
   * @return true if (s, p, o) exist with (spo => uvw) mapping, false otherwise.
   */
  inline bool contains_spo(r_index s, r_index p, r_index o) const 
  {
    r_index u, v, w;
    lookup_spo2uvw(spin_, s, p, o, u, v, w);
    return contains(u, v, w);
  }

  /**
   * @return true if (u, v, *) exist, false otherwise.
   */
  bool contains(r_index u, r_index v) const 
  {
    auto utor = umap_data_.find(u);
    if (utor == umap_data_.end()) return false;

    return utor->second.contains(v);
  }

  /**
   * @return an Iterator over all the triples in the graph
   *
   */
  Iterator find() const 
  {
    Iterator itor;
    itor.spin_ = spin_;
    itor.mode_ = Iterator::kAll;
    itor.utor_ = umap_data_.begin();
    itor.uend_ = umap_data_.end();
    for(; itor.utor_ != itor.uend_; ++itor.utor_) {
      if(itor.enter_u()) {
        itor.start(registry());
        return itor;
      }
    }
    return end_iterator();
  }

  /**
   * @return an Iterator over the triples identified as (u, *, *)
   */
  Iterator find(r_index u) const {
    auto utor = umap_data_.find(u);
    if (utor == umap_data_.end()) return end_iterator();

    Iterator itor;
    itor.spin_ = spin_;
    itor.mode_ = Iterator::kU;
    itor.u_ = u;
    itor.vtor_ = utor->second.begin();
    itor.vend_ = utor->second.end();
    for(; itor.vtor_ != itor.vend_; ++itor.vtor_) {
      if(itor.enter_v()) {
        itor.start(registry());
        return itor;
      }
    }
    return end_iterator();
  }

  /**
   * @return an Iterator over the triples identified as (s, p, *)
   */
  Iterator find(r_index u, r_index v) const {
    auto utor = umap_data_.find(u);
    if (utor == umap_data_.end()) return end_iterator();

    auto vtor = utor->second.find(v);
    if (vtor == utor->second.end()) return end_iterator();

    Iterator itor;
    itor.spin_ = spin_;
    itor.mode_ = Iterator::kUV;
    itor.u_ = u;
    itor.vtor_ = vtor;
    if(not itor.enter_v()) return end_iterator();
    itor.start(registry());
    return itor;
  }

  /**
   * The returned Iterator will have at most one item.
   *
   * It had more until 2026-10: it walked the w set from (u, v, w) to the set's end,
   * so it returned every w stored after w in the set's order. A rule's fully bound
   * antecedent saw (u, v, w) and then the other objects of (u, v), each making a beta
   * row equal to the first (the row takes nothing from a bound triple), which the
   * beta relation discarded as a duplicate. Results are unchanged; the work is not.
   *
   * @return an Iterator with the triple (u, v, w) if it exist in the graph.
   */
  Iterator find(r_index u, r_index v, r_index w) const {
    if(not contains(u, v, w)) return end_iterator();

    Iterator itor;
    itor.spin_ = spin_;
    itor.mode_ = Iterator::kUVW;
    itor.u_ = u;
    itor.v_ = v;
    itor.w_ = w;
    return itor;
  }

  /**
   * @return an Iterator with the triple (s, p, o) using (spo => uvw) mapping if it exist in the graph.
   */
  inline Iterator find_spo(r_index s, r_index p, r_index o) const 
  {
    r_index u, v, w;
    lookup_spo2uvw(spin_, s, p, o, u, v, w);
    return find(u, v, w);
  }

  /**
   * Used by `rule_term` to determine if an inferred triple will
   * be removed as result of retract call.
   *
   * @return the reference count associated with the triple (u, v, w)
   */
  int get_ref_count(r_index u, r_index v, r_index w) const 
  {
    auto utor = umap_data_.find(u);
    if (utor == umap_data_.end()) return 0;

    auto vtor = utor->second.find(v);
    if (vtor == utor->second.end()) return 0;

    auto wtor = vtor->second.find(WSetType::value_type{w});
    if (wtor == vtor->second.end()) return 0;

    return wtor->get_ref_count();
  }

  /**
   * Insert triple (s, p, o) into graph.
   *
   * Reference count is increased by 1 if the triple already exist in graph.
   *
   * @param u subject
   * @param v predicate
   * @param w object
   * @return true if triple was actually inserted (did not already exist in
   * graph)
   */
  bool insert(r_index u, r_index v, r_index w) 
  {
    // A new triple changes structure; the tables may rehash under a live iterator
    if(registry_.head and not contains(u, v, w)) freeze_iterators();

    auto & vmap = umap_data_.try_emplace(u).first->second;
    auto & wset = vmap.try_emplace(v).first->second;

		// If not inserted, then increase the ref_count by 1
    int rc = 1;
    auto pair = wset.insert(WSetType::value_type{w});
    if (!pair.second) {
      rc = pair.first->add_ref_count();
    }
    if(this->spin_ == 's') {
      VLOG(40)<<"INSERT "<<get_gtype(this->gtype_)<<" "<<rc<<" ("<< u <<", "<< v <<", " << w <<")";
    }
    return pair.second;
  }

  inline bool insert_spo(r_index s, r_index p, r_index o) 
  {
    r_index u, v, w;
    lookup_spo2uvw(spin_, s, p, o, u, v, w);
    return insert(u, v, w);
  }

  /**
   * Remove the triple (s, p, o).
   *
   * @param u subject
   * @param v predicate
   * @param w object
   * @return 1 if erased, 0 if was not there (not erased), -1 if error.
   */
  int erase(r_index u, r_index v, r_index w) 
  {
    if(!u or !v or !w) {
      LOG(ERROR) << "BaseGraph::erase: trying to erase a triple with a NULL ptr index (" 
                 << u << ", " << v << ", " << w <<")";
      return -1;
    }
    auto utor = umap_data_.find(u);
    if (utor == umap_data_.end()) return 0;

    auto vtor = utor->second.find(v);
    if (vtor == utor->second.end()) return 0;

    auto wtor = vtor->second.find(WSetType::value_type{w});
    if (wtor == vtor->second.end()) return 0;

    freeze_iterators();
    remove(utor, vtor, wtor);
    if(this->spin_=='s') {
      VLOG(40)<<"ERASE "<<get_gtype(this->gtype_)<<" 0 ("<< u <<", "<< v <<", " << w <<")";
    }
    return 1;
  }

  inline int erase_spo(r_index s, r_index p, r_index o) 
  {
    r_index u, v, w;
    lookup_spo2uvw(spin_, s, p, o, u, v, w);
    return erase(u, v, w);
  }

  /**
   * Decrease the reference count of triple (s, p, o).
   *
   * The triple is removed if the reference count becomes zero.
   *
   * @param u subject
   * @param v predicate
   * @param w object
   * @return 0 if not found or not removed, 1 if removed, -1 if error.
   */
  int retract(r_index u, r_index v, r_index w) 
  {
    if(!u or !v or !w) {
      LOG(ERROR) << "BaseGraph::retact: trying to retract a triple with a NULL ptr index (" 
                 << u << ", " << v << ", " << w <<")";
      return -1;
    }
    auto utor = umap_data_.find(u);
    if (utor == umap_data_.end()) return 0;

    auto vtor = utor->second.find(v);
    if (vtor == utor->second.end()) return 0;

    auto wtor = vtor->second.find(WSetType::value_type{w});
    if (wtor == vtor->second.end()) return 0;
    int rc = wtor->del_ref_count();
    if (rc == 0) {
      freeze_iterators();
      remove(utor, vtor, wtor);
    }
    if(this->spin_ == 's') {
      VLOG(40)<<"RETRACT "<<get_gtype(this->gtype_)<<" "<<rc<<" ("<< u <<", "<< v <<", " << w <<")";
    }
    return rc == 0;
  }

  inline int retract_spo(r_index s, r_index p, r_index o) 
  {
    r_index u, v, w;
    lookup_spo2uvw(spin_, s, p, o, u, v, w);
    return retract(u, v, w);
  }

 private:
  // The meta graph (gtype 0) is shared by the sessions of every thread and does not
  // change once rules run: registering its iterators would be a data race, and
  // there is nothing to protect them from.
  inline Iterator::Registry * registry() const
  {
    return gtype_ == 0 ? nullptr : &registry_;
  }

  inline Iterator end_iterator() const
  {
    Iterator itor;
    itor.spin_ = spin_;
    return itor;
  }

  /**
   * Freeze every live iterator of this graph. Called before any change of structure:
   * a triple inserted or removed, an inner table created or erased. See
   * BaseGraphIterator for why, and for what a frozen iterator returns.
   */
  inline void freeze_iterators()
  {
    while(registry_.head) {
      registry_.head->freeze();     // unlinks it
    }
  }

  // Remove (u, v, w), and the tables it leaves empty
  inline void remove(UMapType::iterator utor, VMapType::iterator vtor, WSetType::iterator wtor)
  {
    vtor->second.erase(wtor);
    if (vtor->second.empty()) {
      utor->second.erase(vtor);
      if (utor->second.empty()) {
        umap_data_.erase(utor);
      }
    }
  }

  char const spin_;
  UMapType umap_data_;
  int gtype_;
  mutable Iterator::Registry registry_;
};

inline 
BaseGraphPtr create_base_graph(char const spin, int gtype=0)
{
  return std::make_shared<BaseGraph>(spin, gtype);
}

}  // namespace jets::rdf
#endif  // JETS_RDF_BASE_GRAPH_H
