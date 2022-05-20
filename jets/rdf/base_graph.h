#ifndef JETS_RDF_BASE_GRAPH_H
#define JETS_RDF_BASE_GRAPH_H

#include <string>
#include <memory>
#include <unordered_set>
#include <unordered_map>

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
      v_end_(),
      w_end_(),
      gtype_(gtype)
  {}

  inline void clear() 
  { 
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

    auto wtor = vtor->second.find(WSetType::value_type{w});
    if (wtor == vtor->second.end()) return false;
    return true;
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

    auto vtor = utor->second.find(v);
    if (vtor == utor->second.end()) return false;

    return true;
  }

  /**
   * @return an Iterator over all the triples in the graph
   *
   */
  Iterator find() const 
  {
    auto utor_begin = umap_data_.begin();
    auto utor_end = umap_data_.end();
    if (utor_begin == utor_end) {
      return Iterator(spin_, Iterator::U_ITOR(0, utor_end, utor_end),
                      Iterator::V_ITOR(0, v_end_, v_end_), 
											Iterator::W_ITOR(w_end_, w_end_) );
    }

    auto vtor_begin = utor_begin->second.begin();
    auto vtor_end = utor_begin->second.end();
    if (vtor_begin == vtor_end) {
      return Iterator(spin_, Iterator::U_ITOR(0, utor_end, utor_end),
                      Iterator::V_ITOR(0, vtor_end, vtor_end),
                      Iterator::W_ITOR(w_end_, w_end_));
    }

    return Iterator(
        spin_, Iterator::U_ITOR(utor_begin->first, utor_begin, utor_end),
        Iterator::V_ITOR(vtor_begin->first, vtor_begin, vtor_end),
        Iterator::W_ITOR(vtor_begin->second.begin(), vtor_begin->second.end()));
  }

  /**
   * @return an Iterator over the triples identified as (u, *, *)
   */
  Iterator find(r_index u) const {
    auto utor = umap_data_.find(u);
    if (utor == umap_data_.end()) {
      return Iterator(spin_, Iterator::U_ITOR(0, umap_data_.end(), umap_data_.end()),
                      Iterator::V_ITOR(0, v_end_, v_end_),
                      Iterator::W_ITOR(w_end_, w_end_));
    }

    auto vtor_begin = utor->second.begin();
    auto vtor_end = utor->second.end();
    if (vtor_begin == vtor_end) {
      return Iterator(spin_, Iterator::U_ITOR(0, umap_data_.end(), umap_data_.end()),
                      Iterator::V_ITOR(0, vtor_end, vtor_end),
                      Iterator::W_ITOR(w_end_, w_end_));
    }

    return Iterator(
        spin_, Iterator::U_ITOR(u, umap_data_.end(), umap_data_.end()),
        Iterator::V_ITOR(vtor_begin->first, vtor_begin, vtor_end),
        Iterator::W_ITOR(vtor_begin->second.begin(), vtor_begin->second.end()));
  }

  /**
   * @return an Iterator over the triples identified as (s, p, *)
   */
  Iterator find(r_index u, r_index v) const {
    // std::cout<<"BaseGraph::find ("<<u<<", "<<v<<") spin "<<this->spin_<<std::endl;
    auto utor = umap_data_.find(u);
    if (utor == umap_data_.end()) {
      return Iterator(spin_, Iterator::U_ITOR(0, umap_data_.end(), umap_data_.end()),
                      Iterator::V_ITOR(0, v_end_, v_end_),
                      Iterator::W_ITOR(w_end_, w_end_));
    }

    auto vtor = utor->second.find(v);
    if (vtor == utor->second.end()) {
      return Iterator(spin_, Iterator::U_ITOR(0, umap_data_.end(), umap_data_.end()),
                      Iterator::V_ITOR(0, v_end_, v_end_),
                      Iterator::W_ITOR(w_end_, w_end_));
    }

    return Iterator(spin_, Iterator::U_ITOR(u, umap_data_.end(), umap_data_.end()),
                    Iterator::V_ITOR(v, v_end_, v_end_),
                    Iterator::W_ITOR(vtor->second.begin(), vtor->second.end()));
  }

  /**
   * The returned Iterator will have at most one item.
   *
   * @return an Iterator with the triple (u, v, w) if it exist in the graph.
   */
  Iterator find(r_index u, r_index v, r_index w) const {
    auto utor = umap_data_.find(u);
    if (utor == umap_data_.end()) {
      return Iterator(spin_, Iterator::U_ITOR(0, umap_data_.end(), umap_data_.end()),
                      Iterator::V_ITOR(0, v_end_, v_end_),
                      Iterator::W_ITOR(w_end_, w_end_));
    }

    auto vtor = utor->second.find(v);
    if (vtor == utor->second.end()) {
      return Iterator(spin_, Iterator::U_ITOR(0, umap_data_.end(), umap_data_.end()),
                      Iterator::V_ITOR(0, v_end_, v_end_),
                      Iterator::W_ITOR(w_end_, w_end_));
    }

    auto wtor = vtor->second.find(WSetType::value_type{w});
    if (wtor == vtor->second.end()) {
      return Iterator(spin_, Iterator::U_ITOR(0, umap_data_.end(), umap_data_.end()),
                      Iterator::V_ITOR(0, v_end_, v_end_),
                      Iterator::W_ITOR(w_end_, w_end_));
    }

    return Iterator(spin_, Iterator::U_ITOR(u, umap_data_.end(), umap_data_.end()),
                    Iterator::V_ITOR(v, v_end_, v_end_), Iterator::W_ITOR(wtor, vtor->second.end()));
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
    auto utor = umap_data_.find(u);
    if (utor == umap_data_.end()) {
      utor = umap_data_.insert({u, {} }).first;
    }

    auto vtor = utor->second.find(v);
    if (vtor == utor->second.end()) {
      vtor = utor->second.insert({v, {} }).first;
    }

		// If not inserted, then increase the ref_count by 1
    int rc = 1;
    auto pair = vtor->second.insert(WSetType::value_type{w});
    if (!pair.second) {
      rc = pair.first->add_ref_count();
    }
    VLOG(4)<<"INSERT "<<get_gtype(this->gtype_)<<" "<<rc<<" ("<< u <<", "<< v <<", " << w <<")";
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

    int count = vtor->second.erase(WSetType::value_type{w});
    if (vtor->second.empty()) {
      utor->second.erase(v);
      if (utor->second.empty()) {
        umap_data_.erase(u);
      }
    }
    if(count) {
      VLOG(4)<<"ERASE "<<get_gtype(this->gtype_)<<" 0 ("<< u <<", "<< v <<", " << w <<")";
    }
    return count;
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
      vtor->second.erase(wtor);
      if (vtor->second.empty()) {
        utor->second.erase(v);
        if (utor->second.empty()) {
          umap_data_.erase(u);
        }
      }
    }
    VLOG(4)<<"RETRACT "<<get_gtype(this->gtype_)<<" "<<rc<<" ("<< u <<", "<< v <<", " << w <<")";
    return rc == 0;
  }

  inline int retract_spo(r_index s, r_index p, r_index o) 
  {
    r_index u, v, w;
    lookup_spo2uvw(spin_, s, p, o, u, v, w);
    return retract(u, v, w);
  }

 private:
  char const spin_;
  UMapType umap_data_;
  // have empty iterators
  VMapType::const_iterator v_end_;
  WSetType::const_iterator w_end_;
  int gtype_;
};

inline 
BaseGraphPtr create_base_graph(char const spin, int gtype=0)
{
  return std::make_shared<BaseGraph>(spin, gtype);
}

}  // namespace jets::rdf
#endif  // JETS_RDF_BASE_GRAPH_H
