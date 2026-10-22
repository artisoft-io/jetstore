#ifndef JETS_RDF_BASE_GRAPH_ITERATOR_H
#define JETS_RDF_BASE_GRAPH_ITERATOR_H

#include <array>
#include <cstdint>
#include <memory>
#include <string>
#include <vector>

#include "absl/hash/hash.h"

#include "../rdf/rdf_err.h"
#include "../rdf/rdf_ast.h"
#include "../rdf/w_node.h"
#include "../rdf/containers_type.h"

namespace jets::rdf {

class BaseGraph;
class BaseGraphIterator;

namespace bgi_internal {
/**
 * The live iterators of one BaseGraph, as an intrusive doubly-linked list.
 *
 * A graph that is copied or assigned does not take the iterators of the graph it
 * came from: they iterate that graph, not this one.
 */
struct IteratorRegistry {
  IteratorRegistry() = default;
  IteratorRegistry(IteratorRegistry const&) : head(nullptr) {}
  IteratorRegistry& operator=(IteratorRegistry const&) { return *this; }

  BaseGraphIterator* head{nullptr};
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
 * Semantic of the iterator:
 * \code {.cpp}
 *  itor = graph.find();
 *  while(not itor.is_end()) {
 *    auto s = itor.get_subject();
 *    itor.next();
 *  }
 * \endcode
 *
 * The current triple is held by value, so reading it never touches the graph.
 *
 * Changing the graph while an iterator is live. A rule's filter does not write to the
 * graph, but writes do reach a session graph while some of its iterators are live:
 *  - the error path (raise_exception, and the substring_of and replace_char_of
 *    errors) records (jets:iState, jets:exception, msg) while a filter is evaluated,
 *    under the iterators of every rule being visited. RDFSession::insert first erases
 *    the same triple from the inferred graph, under the inferred graph's iterators;
 *  - consequent operators insert inferred triples as they compute: lookups
 *    (lookup_sql_helper.cc), range (expr_op_others.h) and create_entity's jets:key
 *    (expr_op_resources.h);
 *  - an iterator handed to Go through jets_rete_cwrapper lives as long as Go holds
 *    it, across any insert Go makes.
 * The graph's tables are open-addressing tables whose iterators an insert can
 * invalidate, so the graph freezes its live iterators before it changes structure:
 * each one copies the triples it has yet to return (`freeze`) and continues over that
 * copy. A frozen iterator returns what the graph held when it was frozen, including a
 * triple erased afterwards, and nothing inserted afterwards; this is how the Go
 * engine's cursor behaves (base_graph_iterator.go). Before 2026-10, with node-based
 * tables, such an iteration was undefined: a rehash could skip or repeat elements,
 * and the erase from the inferred graph could free the set a live iterator was in.
 *
 * A session graph is single-threaded, even when it is only read: find(), copying an
 * iterator and destroying one write the graph's registry of live iterators, so two
 * threads reading one session graph at once would corrupt it.
 *
 * A graph that is not tracked (the meta graph, which every session shares across
 * threads and which does not change once rules run) registers no iterator, and must
 * not be changed while one of its iterators is live.
 *
 * An iterator that outlives its graph is detached and left at its end.
 */
class BaseGraphIterator {
 public:
  using Registry = bgi_internal::IteratorRegistry;

  /** An iterator at its end. */
  inline BaseGraphIterator()
    : spin_('s'), mode_(kUVW), u_(nullptr), v_(nullptr), w_(nullptr),
      utor_(), uend_(), vtor_(), vend_(), wtor_(), wend_(),
      frozen_(), frozen_pos_(0), registry_(nullptr), prev_(nullptr), next_(nullptr)
  {}

  inline BaseGraphIterator(BaseGraphIterator const& rhs)
    : spin_(rhs.spin_), mode_(rhs.mode_), u_(rhs.u_), v_(rhs.v_), w_(rhs.w_),
      utor_(rhs.utor_), uend_(rhs.uend_), vtor_(rhs.vtor_), vend_(rhs.vend_),
      wtor_(rhs.wtor_), wend_(rhs.wend_),
      frozen_(rhs.frozen_), frozen_pos_(rhs.frozen_pos_),
      registry_(nullptr), prev_(nullptr), next_(nullptr)
  {
    if(rhs.registry_) link(rhs.registry_);
  }

  // The moved-from iterator is left at its end, and this one takes its place in the registry
  inline BaseGraphIterator(BaseGraphIterator && rhs) noexcept
    : spin_(rhs.spin_), mode_(rhs.mode_), u_(rhs.u_), v_(rhs.v_), w_(rhs.w_),
      utor_(rhs.utor_), uend_(rhs.uend_), vtor_(rhs.vtor_), vend_(rhs.vend_),
      wtor_(rhs.wtor_), wend_(rhs.wend_),
      frozen_(std::move(rhs.frozen_)), frozen_pos_(rhs.frozen_pos_),
      registry_(nullptr), prev_(nullptr), next_(nullptr)
  {
    take_place_of(rhs);
  }

  inline BaseGraphIterator& operator=(BaseGraphIterator const& rhs)
  {
    if(this == &rhs) return *this;
    unlink();
    copy_state(rhs);
    frozen_ = rhs.frozen_;
    if(rhs.registry_) link(rhs.registry_);
    return *this;
  }

  inline BaseGraphIterator& operator=(BaseGraphIterator && rhs) noexcept
  {
    if(this == &rhs) return *this;
    unlink();
    copy_state(rhs);
    frozen_ = std::move(rhs.frozen_);
    take_place_of(rhs);
    return *this;
  }

  inline ~BaseGraphIterator()
  {
    unlink();
  }

  /**
   * Test if the iterator has exhausted all triples in his collection.
   *
   * @return true if the iterator has no more triples to return
   */
  inline bool is_end() const
  {
    return w_ == nullptr;
  }

  /**
   * Advance the iterator to the next triple
   *
   * @return false if the iterator has no more triples to return
   */
  inline bool next()
  {
    if(is_end()) return false;
    if(frozen_) {
      if(frozen_pos_ < frozen_->size()) {
        auto const& t3 = (*frozen_)[frozen_pos_++];
        u_ = t3[0];
        v_ = t3[1];
        w_ = t3[2];
        return true;
      }
      return set_end();
    }
    if(step()) return true;
    return set_end();
  }

  inline r_index get_subject() const
  {
    if (is_end())
      throw rdf_exception(
          "BaseGraphIterator::get_object: ERROR: called past end of "
          "iterator!");
    r_index s, p, o;
    lookup_uvw2spo(spin_, u_, v_, w_, s, p, o);
    return s;
  }

  inline r_index get_predicate() const
  {
    if (is_end())
      throw rdf_exception(
          "BaseGraphIterator::get_predicate: ERROR: called past end of "
          "iterator!");
    r_index s, p, o;
    lookup_uvw2spo(spin_, u_, v_, w_, s, p, o);
    return p;
  }

  inline r_index get_object() const
  {
    if (is_end())
      throw rdf_exception(
          "BaseGraphIterator::get_object: ERROR: called past end of "
          "iterator!");
    r_index s, p, o;
    lookup_uvw2spo(spin_, u_, v_, w_, s, p, o);
    return o;
  }

  inline Triple
  as_triple() const
  {
    if (is_end())
      throw rdf_exception(
          "BaseGraphIterator::as_triple: ERROR: called past end of "
          "iterator!");
    r_index s, p, o;
    lookup_uvw2spo(spin_, u_, v_, w_, s, p, o);
    return Triple(s, p, o);
  }

 private:
  friend class BaseGraph;
  using Frozen = std::vector<std::array<r_index, 3>>;

  // Which levels of map(u, map(v, set(w))) the iterator walks
  enum Mode : std::uint8_t {
    kAll,     // (*, *, *): every u, every v, every w
    kU,       // (u, *, *): every v of one u, every w
    kUV,      // (u, v, *): every w of one (u, v)
    kUVW      // (u, v, w): the one triple, or none
  };

  // Called by BaseGraph only, after positioning: registers the iterator if it is
  // not at its end and still walks a table
  inline void start(Registry * registry)
  {
    if(registry and not is_end() and mode_ != kUVW) link(registry);
  }

  // Positions on vtor_, which is not vend_; false if its set is empty
  inline bool enter_v()
  {
    v_ = vtor_->first;
    wtor_ = vtor_->second.begin();
    wend_ = vtor_->second.end();
    if(wtor_ == wend_) return false;
    w_ = wtor_->get_index();
    return true;
  }

  // Positions on the first triple of utor_, which is not uend_; false if it has none
  inline bool enter_u()
  {
    u_ = utor_->first;
    vtor_ = utor_->second.begin();
    vend_ = utor_->second.end();
    for(; vtor_ != vend_; ++vtor_) {
      if(enter_v()) return true;
    }
    return false;
  }

  // Moves to the next triple in the tables; false when there is none, leaving the
  // current triple unspecified
  inline bool step()
  {
    switch(mode_) {
    case kUVW:
      return false;
    case kUV:
      if(++wtor_ != wend_) { w_ = wtor_->get_index(); return true; }
      return false;
    case kU:
    case kAll:
      if(++wtor_ != wend_) { w_ = wtor_->get_index(); return true; }
      while(++vtor_ != vend_) {
        if(enter_v()) return true;
      }
      if(mode_ == kAll) {
        while(++utor_ != uend_) {
          if(enter_u()) return true;
        }
      }
      return false;
    }
    return false;
  }

  inline bool set_end()
  {
    u_ = v_ = w_ = nullptr;
    frozen_.reset();
    frozen_pos_ = 0;
    unlink();
    return false;
  }

  /**
   * Called by BaseGraph before it changes structure, while this iterator walks its
   * tables: copy the triples left to return and continue over the copy, so that
   * nothing reads the tables again. The current triple is kept.
   */
  inline void freeze()
  {
    auto frozen = std::make_shared<Frozen>();
    r_index const u = u_, v = v_, w = w_;
    while(step()) {
      frozen->push_back({u_, v_, w_});
    }
    u_ = u; v_ = v; w_ = w;
    frozen_ = std::move(frozen);
    frozen_pos_ = 0;
    unlink();
  }

  inline void copy_state(BaseGraphIterator const& rhs)
  {
    spin_ = rhs.spin_;
    mode_ = rhs.mode_;
    u_ = rhs.u_;
    v_ = rhs.v_;
    w_ = rhs.w_;
    utor_ = rhs.utor_;
    uend_ = rhs.uend_;
    vtor_ = rhs.vtor_;
    vend_ = rhs.vend_;
    wtor_ = rhs.wtor_;
    wend_ = rhs.wend_;
    frozen_pos_ = rhs.frozen_pos_;
  }

  inline void link(Registry * registry)
  {
    registry_ = registry;
    prev_ = nullptr;
    next_ = registry->head;
    if(next_) next_->prev_ = this;
    registry->head = this;
  }

  inline void unlink()
  {
    if(not registry_) return;
    if(prev_) prev_->next_ = next_;
    else registry_->head = next_;
    if(next_) next_->prev_ = prev_;
    registry_ = nullptr;
    prev_ = next_ = nullptr;
  }

  inline void take_place_of(BaseGraphIterator & rhs)
  {
    if(rhs.registry_) {
      registry_ = rhs.registry_;
      prev_ = rhs.prev_;
      next_ = rhs.next_;
      if(prev_) prev_->next_ = this;
      else registry_->head = this;
      if(next_) next_->prev_ = this;
      rhs.registry_ = nullptr;
      rhs.prev_ = rhs.next_ = nullptr;
    }
    rhs.u_ = rhs.v_ = rhs.w_ = nullptr;
    rhs.frozen_.reset();
  }

  char spin_;
  Mode mode_;
  // the current triple, in (u, v, w); w_ is null at the end
  r_index u_;
  r_index v_;
  r_index w_;
  // where the iterator is in the tables, for the levels its mode walks
  UMapType::const_iterator utor_;
  UMapType::const_iterator uend_;
  VMapType::const_iterator vtor_;
  VMapType::const_iterator vend_;
  WSetType::const_iterator wtor_;
  WSetType::const_iterator wend_;
  // set once frozen: the triples left to return, shared by copies
  std::shared_ptr<Frozen const> frozen_;
  std::size_t frozen_pos_;
  // the registry of the graph this iterator walks, while it walks it
  Registry * registry_;
  BaseGraphIterator * prev_;
  BaseGraphIterator * next_;
};

}  // namespace jets::rdf
#endif  // JETS_RDF_BASE_GRAPH_ITERATOR_H
