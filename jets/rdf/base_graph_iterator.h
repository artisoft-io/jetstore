#ifndef JETS_RDF_BASE_GRAPH_ITERATOR_H
#define JETS_RDF_BASE_GRAPH_ITERATOR_H

#include <string>

#include "absl/hash/hash.h"
#include "jets/rdf/rdf_err.h"
#include "jets/rdf/base_graph.h"
#include "jets/rdf/rdf_ast.h"
#include "jets/rdf/w_node.h"

namespace jets::rdf {
namespace bgi_internal {

// Semantic of the iterators:
// --------------------------
//  itor = graph.find();
//  while(not itor.is_end()) {
//    auto s = itor.get_s();
//    itor.next();
//  }
//
template <class T>  // T is the W_SET parameter of `BaseGraph`
struct w_itor_set {
  inline w_itor_set() : m_itor(), m_end() {}

  inline w_itor_set(typename T::const_iterator const& itor_,
                    typename T::const_iterator const& end_)
      : m_itor(itor_), m_end(end_) {}

  w_itor_set(const w_itor_set& rhs) = default;

  w_itor_set& operator=(const w_itor_set& rhs) = default;

  inline void set_itor(typename T::const_iterator const& itor,
                       typename T::const_iterator const& end) {
    m_itor = itor;
    m_end = end;
  }

  inline r_index get_index() const {
    if (is_end()) return nullptr;
    return m_itor->get_index();
  }

  inline bool is_end() const { return m_itor == m_end; }

  inline bool next() {
    if (is_end()) return false;
    ++m_itor;
    return !is_end();
  }

 private:
  typename T::const_iterator m_itor;
  typename T::const_iterator m_end;
};

/**
 * Struct to provide the familiar iteration semantics to u_map_type and
 * `v_map_type` collections.
 *
 * Provide iteration over the keys of the template class collection.
 */
template <class T>  // T is either u_map_type or v_map_type
class itor_map {
 public:
  inline itor_map() : m_v(0), m_itor(), m_end(){};

  inline itor_map(r_index v_, typename T::const_iterator const& itor_,
                  typename T::const_iterator const& end_)
      : m_v(v_), m_itor(itor_), m_end(end_) {
    if (m_v == nullptr) m_itor = m_end;
  }

  inline itor_map(const itor_map& rhs)
      : m_v(rhs.m_v), m_itor(rhs.m_itor), m_end(rhs.m_end){};

  /**
   * Assign operator
   */
  inline itor_map& operator=(const itor_map& rhs) = default;

  inline void set_itor(typename T::const_iterator const& itor,
                       typename T::const_iterator const& end) {
    m_itor = itor;
    m_end = end;
  }

  inline r_index get_index() const { return m_v; }

  inline bool is_end() const { return m_itor == m_end; }

  /**
   * Move to the next key in the collection.
   *
   * The class argument \c W is either of type \c itor_map<v_map_type> or \c
   * w_itor_set which is the iterator of the next level. Meaning:
   *
   * 		- If \c T is of type \c u_map_type, then \c W is \c
   * itor_map<v_map_type>
   * 		- If \c T is of type \c v_map_type, then \c W is \c w_itor_set
   *
   * When \c m_itor is advanced to the next key of the collection, the iterator
   * \c W is reset to the iterator over the values associated to the new key of
   * \c m_itor.
   */
  template <class W>  // W is either itor_map<v_map_type> or w_itor_set
  inline bool next(W& w_itor) {
    if (m_itor != m_end) ++m_itor;
    if (m_itor == m_end) {
      m_v = nullptr;
      return false;
    }
    this->set_position(w_itor);
    return true;
  }

  /**
   * Set the param \c w_itor to iterate over the values (m_itor->second)
   * associated with the key (m_itor->first).
   *
   * @see next()
   */
  template <class W>  // W is either itor_map<v_map_type> or w_itor_set
  inline void set_position(W& w_itor) {
    if (m_itor != m_end) {
      m_v = m_itor->first;
      w_itor.set_itor(m_itor->second.begin(), m_itor->second.end());
    }
  }

 private:
  r_index m_v;
  typename T::const_iterator m_itor;
  typename T::const_iterator m_end;
};
}   // namespace bgi_internal

/////////////////////////////////////////////////////////////////////////////////////////
// class BaseGraphIterator
//
/////////////////////////////////////////////////////////////////////////////////////////
/**
 * The unified iterator over the triple graph structured as:
 *
 * 	map(u, map(v, set(w))), where the triple (u, v, w) can either be:
 * 		- (subject, predicate, object), known as spo
 * 		- (predicate, object, subject), known as pos
 * 		- (object, subject, predicate), known as osp
 *
 */
template <class U, class V, class W>
struct BaseGraphIterator {
    using U_ITOR = bgi_internal::itor_map<U>;
    using V_ITOR = bgi_internal::itor_map<V>;
    using W_ITOR = bgi_internal::w_itor_set<W>;

  inline BaseGraphIterator(char const lookup, U_ITOR u_itor_,V_ITOR v_itor_, W_ITOR w_itor_)
      : m_lookup(lookup),
        m_u_itor(u_itor_),
        m_v_itor(v_itor_),
        m_w_itor(w_itor_) {}

  inline BaseGraphIterator()
      : m_lookup('s'), m_u_itor(), m_v_itor(), m_w_itor() {}

  inline BaseGraphIterator(BaseGraphIterator const& rhs) = default;

  inline BaseGraphIterator& operator=(BaseGraphIterator const& rhs) = default;

  /**
   * Test if the iterator has exhausted all triples in his collection.
   *
   * @return true if the iterator has no more triples to return
   */
  inline bool is_end() const {
    return m_u_itor.is_end() and m_v_itor.is_end() and m_w_itor.is_end();
  }

  /**
   * Advance the iterator to the next triple
   *
   * @return false if the iterator has no more triples to return
   */
  inline bool next() {
    if (is_end()) return false;

    if (!m_w_itor.next()) {
      if (!m_v_itor.next(m_w_itor)) {
        if (m_u_itor.next(m_v_itor)) {
          m_v_itor.set_position(m_w_itor);
        }
      }
    }
    return !is_end();
  }

  inline r_index get_subject() const {
    if (is_end())
      throw rdf_exception(
          "BaseGraphIterator::get_object: ERROR: called past end of "
          "iterator!");
    r_index s, p, o;
    lookup_uvw2spo(m_lookup, m_u_itor.get_index(), m_v_itor.get_index(),
                   m_w_itor.get_index(), s, p, o);
    return s;
  }

  inline r_index get_predicate() const {
    if (is_end())
      throw rdf_exception(
          "BaseGraphIterator::get_predicate: ERROR: called past end of "
          "iterator!");
    r_index s, p, o;
    lookup_uvw2spo(m_lookup, m_u_itor.get_index(), m_v_itor.get_index(),
                   m_w_itor.get_index(), s, p, o);
    return p;
  }

  inline r_index get_object() const {
    if (is_end())
      throw rdf_exception(
          "BaseGraphIterator::get_object: ERROR: called past end of "
          "iterator!");
    r_index s, p, o;
    lookup_uvw2spo(m_lookup, m_u_itor.get_index(), m_v_itor.get_index(),
                   m_w_itor.get_index(), s, p, o);
    return o;
  }

 private:
  char m_lookup;
  U_ITOR m_u_itor;
  V_ITOR m_v_itor;
  W_ITOR m_w_itor;
};

}  // namespace jets::rdf
#endif  // JETS_RDF_BASE_GRAPH_ITERATOR_H
