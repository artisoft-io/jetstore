#ifndef JETS_RDF_BASE_GRAPH_H
#define JETS_RDF_BASE_GRAPH_H

#include <string>
#include <memory>
#include <unordered_set>
#include <unordered_map>

#include "absl/hash/hash.h"

#include "jets/rdf/rdf_err.h"
#include "jets/rdf/rdf_ast.h"
#include "jets/rdf/w_node.h"

namespace jets::rdf {
/**
 * Map (u, v, w) ==> (s, p, o) according to \c m_spin code.
 *
   *  - (u, v, w) => 's' => (u, v, w) <=> (s, p, o)
   *  - (u, v, w) => 'p' => (u, v, w) <=> (p, o, s)
   *  - (u, v, w) => 'o' => (u, v, w) <=> (o, s, p)
 *
 * @param[in] u incoming index
 * @param[in] v incoming index
 * @param[in] w incoming index
 * @param[out] s outgoing index
 * @param[out] p outgoing index
 * @param[out] o outgoing index
 */
inline void
lookup_uvw2spo(char const spin, r_index  const& u, r_index  const& v, r_index  const& w, r_index  &s, r_index  &p, r_index  &o)
{
  if(spin == 's') {							// case 'spo'  <==> "uvw'
    s = u;
    p = v;
    o = w;
  } else if(spin == 'p') {					// case 'pos'  <==> "uvw'
    s = w;
    p = u;
    o = v;
  } else {									// case 'osp'  <==> "uvw'
    s = v;
    p = w;
    o = u;
  }
}

/**
 * Map (s, p, o) ==> (u, v, w) according to \c m_spin code.
 *
   *  - (s, p, o) => 's' => (s, p, o) <=> (u, v, w)
   *  - (s, p, o) => 'p' => (p, o, s) <=> (u, v, w)
   *  - (s, p, o) => 'o' => (o, s, p) <=> (u, v, w)
 *
 * @param[in] s incoming index
 * @param[in] p incoming index
 * @param[in] o incoming index
 * @param[out] u outgoing index
 * @param[out] v outgoing index
 * @param[out] w outgoing index
 */
inline void
lookup_spo2uvw(char const spin, r_index  const& s, r_index  const& p, r_index  const& o, r_index  &u, r_index  &v, r_index  &w)
{
  if(spin == 's') {							// case 'spo'  <==> "uvw'
    u = s;
    v = p;
    w = o;
  } else if(spin == 'p') {					// case 'pos'  <==> "uvw'
    w = s;
    u = p;
    v = o;
  } else {									// case 'osp'  <==> "uvw'
    v = s;
    w = p;
    u = o;
  }
}
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
template <class U_MAP, class V_MAP, class W_SET, class ITOR>
class BaseGraph {
 public:
  using Iterator = ITOR;
  /**
   * Constructor for BaseGraph. 
   * Spin indicate the triple rotation index, possible
   * values are 's', 'p', and 'o'
   *
   * @param spin indicates spin scheme
   */
  inline BaseGraph(char const spin)
      : m_spin(spin),
        m_umap_data(),
        m_v_end(),
        m_w_end(){};
        //* m_session_p(nullptr),
        //* m_index_triple_cback_mgr_p(nullptr){};

  inline void clear() { m_umap_data.clear(); };

  /**
   * @return true if (u, v, w) exist, false otherwise.
   */
  inline bool contains(r_index u, r_index v, r_index w) const {
    auto utor = m_umap_data.find(u);
    if (utor == m_umap_data.end()) return false;

    auto vtor = utor->second.find(v);
    if (vtor == utor->second.end()) return false;

    auto wtor = vtor->second.find(typename W_SET::value_type{w});
    if (wtor == vtor->second.end()) return false;
    return true;
  }

  /**
   * @return true if (s, p, o) exist with (spo => uvw) mapping, false otherwise.
   */
  inline bool contains_spo(r_index s, r_index p, r_index o) const {
    r_index u, v, w;
    lookup_spo2uvw(m_spin, s, p, o, u, v, w);
    return contains(u, v, w);
  }

  /**
   * @return true if (u, v, *) exist, false otherwise.
   */
  inline bool contains(r_index u, r_index v) const {
    auto utor = m_umap_data.find(u);
    if (utor == m_umap_data.end()) return false;

    auto vtor = utor->second.find(v);
    if (vtor == utor->second.end()) return false;

    return true;
  }

  /**
   * @return an Iterator over all the triples in the graph
   *
   */
  inline Iterator find() const {
    auto utor_begin = m_umap_data.begin();
    auto utor_end = m_umap_data.end();
    if (utor_begin == utor_end) {
      return Iterator(m_spin, typename Iterator::U_ITOR(0, utor_end, utor_end),
                      typename Iterator::V_ITOR(0, m_v_end, m_v_end), 
											typename Iterator::W_ITOR(m_w_end, m_w_end) );
    }

    auto vtor_begin = utor_begin->second.begin();
    auto vtor_end = utor_begin->second.end();
    if (vtor_begin == vtor_end) {
      return Iterator(m_spin, typename Iterator::U_ITOR(0, utor_end, utor_end),
                      typename Iterator::V_ITOR(0, vtor_end, vtor_end),
                      typename Iterator::W_ITOR(m_w_end, m_w_end));
    }

    return Iterator(
        m_spin, typename Iterator::U_ITOR(utor_begin->first, utor_begin, utor_end),
        typename Iterator::V_ITOR(vtor_begin->first, vtor_begin, vtor_end),
        typename Iterator::W_ITOR(vtor_begin->second.begin(), vtor_begin->second.end()));
  }

  /**
   * @return an Iterator over the triples identified as (u, *, *)
   */
  inline Iterator find(r_index u) const {
    auto utor = m_umap_data.find(u);
    if (utor == m_umap_data.end()) {
      return Iterator(m_spin, typename Iterator::U_ITOR(0, m_umap_data.end(), m_umap_data.end()),
                      typename Iterator::V_ITOR(0, m_v_end, m_v_end),
                      typename Iterator::W_ITOR(m_w_end, m_w_end));
    }

    auto vtor_begin = utor->second.begin();
    auto vtor_end = utor->second.end();
    if (vtor_begin == vtor_end) {
      return Iterator(m_spin, typename Iterator::U_ITOR(0, m_umap_data.end(), m_umap_data.end()),
                      typename Iterator::V_ITOR(0, vtor_end, vtor_end),
                      typename Iterator::W_ITOR(m_w_end, m_w_end));
    }

    return Iterator(
        m_spin, typename Iterator::U_ITOR(u, m_umap_data.end(), m_umap_data.end()),
        typename Iterator::V_ITOR(vtor_begin->first, vtor_begin, vtor_end),
        typename Iterator::W_ITOR(vtor_begin->second.begin(), vtor_begin->second.end()));
  }

  /**
   * @return an Iterator over the triples identified as (s, p, *)
   */
  Iterator find(r_index u, r_index v) const {
    auto utor = m_umap_data.find(u);
    if (utor == m_umap_data.end()) {
      return Iterator(m_spin, typename Iterator::U_ITOR(0, m_umap_data.end(), m_umap_data.end()),
                      typename Iterator::V_ITOR(0, m_v_end, m_v_end),
                      typename Iterator::W_ITOR(m_w_end, m_w_end));
    }

    auto vtor = utor->second.find(v);
    if (vtor == utor->second.end()) {
      return Iterator(m_spin, typename Iterator::U_ITOR(0, m_umap_data.end(), m_umap_data.end()),
                      typename Iterator::V_ITOR(0, m_v_end, m_v_end),
                      typename Iterator::W_ITOR(m_w_end, m_w_end));
    }

    return Iterator(m_spin, typename Iterator::U_ITOR(u, m_umap_data.end(), m_umap_data.end()),
                    typename Iterator::V_ITOR(v, m_v_end, m_v_end),
                    typename Iterator::W_ITOR(vtor->second.begin(), vtor->second.end()));
  }

  /**
   * The returned Iterator will have at most one item.
   *
   * @return an Iterator with the triple (u, v, w) if it exist in the graph.
   */
  inline Iterator find(r_index u, r_index v, r_index w) const {
    auto utor = m_umap_data.find(u);
    if (utor == m_umap_data.end()) {
      return Iterator(m_spin, typename Iterator::U_ITOR(0, m_umap_data.end(), m_umap_data.end()),
                      typename Iterator::V_ITOR(0, m_v_end, m_v_end),
                      typename Iterator::W_ITOR(m_w_end, m_w_end));
    }

    auto vtor = utor->second.find(v);
    if (vtor == utor->second.end()) {
      return Iterator(m_spin, typename Iterator::U_ITOR(0, m_umap_data.end(), m_umap_data.end()),
                      typename Iterator::V_ITOR(0, m_v_end, m_v_end),
                      typename Iterator::W_ITOR(m_w_end, m_w_end));
    }

    auto wtor = vtor->second.find(typename W_SET::value_type{w});
    if (wtor == vtor->second.end()) {
      return Iterator(m_spin, typename Iterator::U_ITOR(0, m_umap_data.end(), m_umap_data.end()),
                      typename Iterator::V_ITOR(0, m_v_end, m_v_end),
                      typename Iterator::W_ITOR(m_w_end, m_w_end));
    }

    return Iterator(m_spin, typename Iterator::U_ITOR(u, m_umap_data.end(), m_umap_data.end()),
                    typename Iterator::V_ITOR(v, m_v_end, m_v_end), typename Iterator::W_ITOR(wtor, vtor->second.end()));
  }

  /**
   * @return an Iterator with the triple (s, p, o) using (spo => uvw) mapping if it exist in the graph.
   */
  inline Iterator find_spo(r_index s, r_index p, r_index o) const {
    r_index u, v, w;
    lookup_spo2uvw(m_spin, s, p, o, u, v, w);
    return find(u, v, w);
  }

  /**
   * Used by `rule_term` to determine if an inferred triple will
   * be removed as result of retract call.
   *
   * @return the reference count associated with the triple (u, v, w)
   */
  inline int get_ref_count(r_index u, r_index v, r_index w) const {
    auto utor = m_umap_data.find(u);
    if (utor == m_umap_data.end()) return 0;

    auto vtor = utor->second.find(v);
    if (vtor == utor->second.end()) return 0;

    auto wtor = vtor->second.find(typename W_SET::value_type{w});
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
  inline bool insert(r_index u, r_index v, r_index w) {
    auto utor = m_umap_data.find(u);
    if (utor == m_umap_data.end()) {
      utor = m_umap_data.insert({u, {} }).first;
    }

    auto vtor = utor->second.find(v);
    if (vtor == utor->second.end()) {
      vtor = utor->second.insert({v, {} }).first;
    }

		// If not inserted, then increase the ref_count by 1
    auto pair = vtor->second.insert(typename W_SET::value_type{w});
    if (!pair.second) pair.first->add_ref_count();
    return pair.second;
  }

  inline bool insert_spo(r_index s, r_index p, r_index o) {
    r_index u, v, w;
    lookup_spo2uvw(m_spin, s, p, o, u, v, w);
    return insert(u, v, w);
  }

  /**
   * Remove the triple (s, p, o).
   *
   * The registered callback functions are notified.
   *
   * @param u subject
   * @param v predicate
   * @param w object
   * @return 0 if was not found, 1 if removed.
   */
  inline int erase(r_index u, r_index v, r_index w) {
    auto utor = m_umap_data.find(u);
    if (utor == m_umap_data.end()) return 0;

    auto vtor = utor->second.find(v);
    if (vtor == utor->second.end()) return 0;

    int count = vtor->second.erase(typename W_SET::value_type{w});
    if (vtor->second.empty()) {
      utor->second.erase(v);
      if (utor->second.empty()) {
        m_umap_data.erase(u);
      }
    }
    return count;
  }

  inline int erase_spo(r_index s, r_index p, r_index o) {
    r_index u, v, w;
    lookup_spo2uvw(m_spin, s, p, o, u, v, w);
    return erase(u, v, w);
  }

  /**
   * Decrease the reference count of triple (s, p, o).
   *
   * The triple is removed if the reference count becomes zero.
   *
   * The registered callback functions are notified.
   *
   * @param u subject
   * @param v predicate
   * @param w object
   * @return 0 if not found or not removed, 1 if removed.
   */
  inline int retract(r_index u, r_index v, r_index w) {
    auto utor = m_umap_data.find(u);
    if (utor == m_umap_data.end()) return 0;

    auto vtor = utor->second.find(v);
    if (vtor == utor->second.end()) return 0;

    int count = 0;
    auto wtor = vtor->second.find(typename W_SET::value_type{w});
    if (wtor == vtor->second.end()) return 0;

    if (wtor->del_ref_count() == 0) {
      vtor->second.erase(wtor);
      if (vtor->second.empty()) {
        utor->second.erase(v);
        if (utor->second.empty()) {
          m_umap_data.erase(u);
        }
      }
      count = 1;
    }
    return count;
  }

  inline int retract_spo(r_index s, r_index p, r_index o) {
    r_index u, v, w;
    lookup_spo2uvw(m_spin, s, p, o, u, v, w);
    return retract(u, v, w);
  }

 private:

  char const m_spin;
  U_MAP m_umap_data;

  // have empty iterators
  typename V_MAP::const_iterator m_v_end;
  typename W_SET::const_iterator m_w_end;
};
}  // namespace jets::rdf
#endif  // JETS_RDF_BASE_GRAPH_H
