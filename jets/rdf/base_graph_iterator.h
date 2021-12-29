#ifndef JETS_RDF_BASE_GRAPH_ITERATOR_H
#define JETS_RDF_BASE_GRAPH_ITERATOR_H

#include <string>

#include "absl/hash/hash.h"

#include "jets/rdf/rdf_err.h"
#include "jets/rdf/rdf_ast.h"
#include "jets/rdf/w_node.h"
#include "jets/rdf/containers_type.h"

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
struct w_itor_set {
  inline w_itor_set() : itor_(), end_() {}

  inline w_itor_set(WSetType::const_iterator const& itor_,
                    WSetType::const_iterator const& end_)
      : itor_(itor_), end_(end_) {}

  w_itor_set(const w_itor_set& rhs) = default;

  w_itor_set& operator=(const w_itor_set& rhs) = default;

  inline void set_itor(WSetType::const_iterator const& itor,
                       WSetType::const_iterator const& end) {
    itor_ = itor;
    end_ = end;
  }

  inline r_index get_index() const {
    if (is_end()) return nullptr;
    return itor_->get_index();
  }

  inline bool is_end() const { return itor_ == end_; }

  inline bool next() {
    if (is_end()) return false;
    ++itor_;
    return !is_end();
  }

 private:
  WSetType::const_iterator itor_;
  WSetType::const_iterator end_;
};

/**
 * Struct to provide iteration semantics to UMapType and VMapType
 *
 * Provide iteration over the keys of UMapType and VMapType collection.
 */
template <class T>  // T is either UMapType and VMapType
class itor_map {
 public:
  inline itor_map() : v_(0), itor_(), end_(){};

  inline itor_map(r_index v_, typename T::const_iterator const& itor,
                  typename T::const_iterator const& end)
    : v_(v_), itor_(itor), end_(end) 
  {
    if (v_ == nullptr) itor_ = end_;
  }

  inline itor_map(const itor_map& rhs)
    : v_(rhs.v_), itor_(rhs.itor_), end_(rhs.end_)
  {}

  inline itor_map& operator=(const itor_map& rhs) = default;

  inline void set_itor(typename T::const_iterator const& itor,
                       typename T::const_iterator const& end) 
  {
    itor_ = itor;
    end_ = end;
  }

  inline r_index get_index() const { return v_; }
  inline bool is_end() const { return itor_ == end_; }

  /**
   * Move to the next key in the collection.
   *
   * The template argument W is: 
   *  - itor_map<VMapType> when T=UMapType, or
   *  - WSetType           when T=VMapType
   *
   * When itor_ is advanced to the next key of the collection, the iterator
   * W is reset to the iterator over the values associated to the new key of
   * itor_.
   */
  template <class W> 
  inline bool next(W& w_itor) 
  {
    if (itor_ != end_) ++itor_;
    if (itor_ == end_) {
      v_ = nullptr;
      return false;
    }
    this->set_position(w_itor);
    return true;
  }

  /**
   * Set w_itor to iterate over the values (itor_->second)
   * associated with the key (itor_->first).
   *
   * The template argument W is: 
   *  - itor_map<VMapType> when T=UMapType, or
   *  - WSetType           when T=VMapType
   */
  template <class W>
  inline void set_position(W& w_itor) 
  {
    if (itor_ != end_) {
      v_ = itor_->first;
      w_itor.set_itor(itor_->second.begin(), itor_->second.end());
    }
  }

 private:
  r_index v_;
  typename T::const_iterator itor_;
  typename T::const_iterator end_;
};
}   // namespace bgi_internal
// //////////////////////////////////////////////////////////////////////////////////////
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
lookup_uvw2spo(char const spin, 
  r_index  const& u, r_index  const& v, r_index  const& w, 
  r_index  &s, r_index  &p, r_index  &o)
{
  if(spin == 's') {					// case 'spo'  <==> "uvw'
    s = u;
    p = v;
    o = w;
  } else if(spin == 'p') {	// case 'pos'  <==> "uvw'
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
lookup_spo2uvw(char const spin, 
  r_index  const& s, r_index  const& p, r_index  const& o, 
  r_index  &u, r_index  &v, r_index  &w)
{
  if(spin == 's') {					// case 'spo'  <==> "uvw'
    u = s;
    v = p;
    w = o;
  } else if(spin == 'p') {	// case 'pos'  <==> "uvw'
    w = s;
    u = p;
    v = o;
  } else {									// case 'osp'  <==> "uvw'
    v = s;
    w = p;
    u = o;
  }
}

/////////////////////////////////////////////////////////////////////////////////////////
// class BaseGraphIterator
/**
 * The unified iterator for BaseGraph
 *
 * BaseGraph is a collection of triples (u, v, w) structured as
 * map(u, map(v, set(w))). 
 *
 * The triple (u, v, w) can either be:
 *	- (subject, predicate, object), known as spo
 *	- (predicate, object, subject), known as pos
 *	- (object, subject, predicate), known as osp
 *
 */
struct BaseGraphIterator {
    using U_ITOR = bgi_internal::itor_map<UMapType>;
    using V_ITOR = bgi_internal::itor_map<VMapType>;
    using W_ITOR = bgi_internal::w_itor_set;

  inline BaseGraphIterator(char const lookup, U_ITOR u_itor_,V_ITOR v_itor_, W_ITOR w_itor_)
    : spin_(lookup),
      u_itor_(u_itor_),
      v_itor_(v_itor_),
      w_itor_(w_itor_) 
  {}

  inline BaseGraphIterator()
    : spin_('s'), u_itor_(), v_itor_(), w_itor_() 
  {}

  inline BaseGraphIterator(BaseGraphIterator const& rhs) = default;
  inline BaseGraphIterator& operator=(BaseGraphIterator const& rhs) = default;

  /**
   * Test if the iterator has exhausted all triples in his collection.
   *
   * @return true if the iterator has no more triples to return
   */
  inline bool is_end() const 
  {
    return u_itor_.is_end() and v_itor_.is_end() and w_itor_.is_end();
  }

  /**
   * Advance the iterator to the next triple
   *
   * @return false if the iterator has no more triples to return
   */
  inline bool next() 
  {
    if (is_end()) return false;

    if (!w_itor_.next()) {
      if (!v_itor_.next(w_itor_)) {
        if (u_itor_.next(v_itor_)) {
          v_itor_.set_position(w_itor_);
        }
      }
    }
    return !is_end();
  }

  inline r_index get_subject() const 
  {
    if (is_end())
      throw rdf_exception(
          "BaseGraphIterator::get_object: ERROR: called past end of "
          "iterator!");
    r_index s, p, o;
    lookup_uvw2spo(spin_, u_itor_.get_index(), v_itor_.get_index(),
                   w_itor_.get_index(), s, p, o);
    return s;
  }

  inline r_index get_predicate() const 
  {
    if (is_end())
      throw rdf_exception(
          "BaseGraphIterator::get_predicate: ERROR: called past end of "
          "iterator!");
    r_index s, p, o;
    lookup_uvw2spo(spin_, u_itor_.get_index(), v_itor_.get_index(),
                   w_itor_.get_index(), s, p, o);
    return p;
  }

  inline r_index get_object() const 
  {
    if (is_end())
      throw rdf_exception(
          "BaseGraphIterator::get_object: ERROR: called past end of "
          "iterator!");
    r_index s, p, o;
    lookup_uvw2spo(spin_, u_itor_.get_index(), v_itor_.get_index(),
                   w_itor_.get_index(), s, p, o);
    return o;
  }

 private:
  char spin_;
  U_ITOR u_itor_;
  V_ITOR v_itor_;
  W_ITOR w_itor_;
};

}  // namespace jets::rdf
#endif  // JETS_RDF_BASE_GRAPH_ITERATOR_H
