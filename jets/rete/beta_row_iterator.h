#ifndef JETS_RETE_BETA_ROW_ITERATOR_H
#define JETS_RETE_BETA_ROW_ITERATOR_H

#include <cassert>
#include <string>
#include <memory>
#include <utility>
#include <list>
#include <tuple>
#include <iterator>
#include <scoped_allocator>
#include <type_traits>
#include <vector>

#include "absl/hash/hash.h"
#include "absl/container/flat_hash_map.h"
#include "absl/container/inlined_vector.h"

#include "../rete/beta_row.h"

namespace jets::rete {
// Declaration of Container Types for BetaRelation Class
/////////////////////////////////////////////////////////////////////////////////////////
// Container for indexes on BetaRow
using BetaRowIndxKey2 = std::tuple<rdf::r_index, rdf::r_index>;
using BetaRowIndxKey3 = std::tuple<rdf::r_index, rdf::r_index, rdf::r_index>;

/**
 * An index of a beta relation's rows: the rows by the values a child's antecedent
 * looks them up by, several rows per key.
 *
 * A flat hash map from the key to the rows under it, in place of the
 * std::unordered_multimap it was, and allocating from the relation's arena. It keeps
 * the two properties of the multimap the engine relies on:
 *  - erase(key) removes every row under the key, not only one (I-85, kept as is);
 *  - the rows under a key are returned newest first, which is the order libstdc++'s
 *    unordered_multimap returned equal keys in (a new element goes before the first
 *    equal one, and a rehash keeps equal elements in order).
 *
 * The map and the lists under it move when they grow, so an iteration over a key's
 * rows must not survive a change to the index: BetaRelation freezes its live scans
 * before it changes one (see BetaRowScan).
 */
template<class K>
class BetaRowIndex {
 public:
  using allocator_type = ArenaAllocator<BetaRow const*>;
  using row_list = absl::InlinedVector<BetaRow const*, 2, ArenaAllocator<BetaRow const*>>;
  using map_type = absl::flat_hash_map<K, row_list, absl::Hash<K>, std::equal_to<K>,
    ArenaAllocator<std::pair<K const, row_list>>>;

  // The rows under a key, newest first: [first, last) read backwards
  struct Rows {
    BetaRow const* const* first;
    BetaRow const* const* last;
  };

  BetaRowIndex() : map_() {}
  explicit BetaRowIndex(allocator_type const& alloc)
    : map_(typename map_type::allocator_type(alloc)) {}
  BetaRowIndex(BetaRowIndex && rhs) = default;
  BetaRowIndex(BetaRowIndex && rhs, allocator_type const& alloc)
    : map_(std::move(rhs.map_), typename map_type::allocator_type(alloc)) {}
  BetaRowIndex(BetaRowIndex const&) = delete;
  BetaRowIndex & operator=(BetaRowIndex const&) = delete;

  inline void
  insert(std::pair<K, BetaRow const*> const& key_row)
  {
    auto res = map_.try_emplace(key_row.first, allocator_type(map_.get_allocator()));
    res.first->second.push_back(key_row.second);
  }

  // Removes every row under `key` (I-85) and returns how many, as the multimap's
  // erase(key) did
  inline std::size_t
  erase(K const& key)
  {
    auto itor = map_.find(key);
    if(itor == map_.end()) return 0;
    std::size_t n = itor->second.size();
    map_.erase(itor);
    return n;
  }

  inline Rows
  rows(K const& key)const
  {
    auto itor = map_.find(key);
    if(itor == map_.end()) return {nullptr, nullptr};
    auto const& list = itor->second;
    return {list.data(), list.data() + list.size()};
  }

  // Number of keys
  inline std::size_t
  size()const
  {
    return map_.size();
  }

  inline allocator_type
  get_allocator()const
  {
    return allocator_type(map_.get_allocator());
  }

 private:
  map_type map_;
};

using BetaRowIndexes1 = BetaRowIndex<rdf::r_index>;
using BetaRowIndexes2 = BetaRowIndex<BetaRowIndxKey2>;
using BetaRowIndexes3 = BetaRowIndex<BetaRowIndxKey3>;
// Container for sets of indexes on BetaRow
// Parent beta node have multiple child node, each having an antecedent term pointing to the parent node.
// We can have multiple indexes of each type. Therefore we need to have a vector of indexes, keyed by position, for each type.
// The scoped adaptor hands the vector's arena to each index it holds, including one
// pushed as a default-constructed temporary.
template<class I>
using BetaRowIndxVec = std::vector<I, std::scoped_allocator_adaptor<ArenaAllocator<I>>>;
using BetaRowIndxVec1 = BetaRowIndxVec<BetaRowIndexes1>;
using BetaRowIndxVec2 = BetaRowIndxVec<BetaRowIndexes2>;
using BetaRowIndxVec3 = BetaRowIndxVec<BetaRowIndexes3>;

// queue of new beta_row for descendent nodes
using beta_row_list = std::list<BetaRowPtr, ArenaAllocator<BetaRowPtr>>;
/////////////////////////////////////////////////////////////////////////////////////////
// class BetaRowIterator
//
/////////////////////////////////////////////////////////////////////////////////////////
class BetaRowIterator;
using BetaRowIteratorPtr = std::shared_ptr<BetaRowIterator>;

/**
 * The unified iterator over the BetaRow managed by a BetaRelation
 *
 * 	The interator unify the iteration over:
 * 		- All rows contained in the BetaRelation
 * 		- The activated rows (add/delete) resulting from last merge
 * 		- Selected row based on query in response for a triple added or removed from the
 *      inferred graph.
 *
 * Implemented using an abstract base class with specialized sub classes.
 *
 * The iterator api follow the same api as for the rdf graph:
 * \code {.cpp}
 * while(not itor.is_end()) {
 *   auto row = itor.get_row();
 *   ...
 *   itor.next();
 * }
 * \endcode
 */
class BetaRowIterator {
 public:
  inline BetaRowIterator() {}
  virtual ~BetaRowIterator() {}

  // No need for those, will use shared_ptr
  BetaRowIterator(BetaRowIterator const& rhs) = delete;
  BetaRowIterator(BetaRowIterator && rhs) = delete;
  BetaRowIterator& operator=(BetaRowIterator const& rhs) = delete;

  /**
   * Test if the iterator has exhausted all items in his collection.
   *
   * @return true if the iterator has no more items to return
   */
  virtual bool is_end()const=0;

  /**
   * Advance the iterator to the next item
   *
   * @return false if the iterator has no more item to return
   */
  virtual bool next()=0;

  virtual BetaRow const* get_row() const=0;
};

/////////////////////////////////////////////////////////////////////////////////////////
// Scans that survive a change to what they scan
/////////////////////////////////////////////////////////////////////////////////////////
class BetaRowScan;

/**
 * The live scans of one beta relation, by what they scan
 *
 * A scan iterates its container in place. The engine can change a relation while one
 * of its scans is live: a filter's error path, or a create_entity in a filter,
 * inserts into a graph whose callbacks insert into and remove from beta relations
 * (I-90). A flat container's iterators do not survive an insert that grows it, so
 * the relation freezes its scans of a container before it changes that container:
 * each copies the rows it has yet to return and continues over the copy. This is the
 * treatment the RDF graph's iterators have (BaseGraph::freeze_iterators).
 *
 * A frozen scan holds the rows it has yet to return, so a remove meanwhile does not
 * free one under it. A frozen scan of the row set or of an index skips a row that is
 * no longer in the relation when it gets to it, as an iteration of the node-based
 * set skipped an erased element; a frozen scan of an index also skips a row that has
 * left the index, a removed row kept in the set as kDeleted, which the index no longer
 * returns. A frozen scan of the pending list skips nothing, since a removed row is put
 * on the pending list to notify the children.
 *
 * A scan leaves the registry when it is exhausted, so one that outlives its relation
 * (an exhausted BetaRowIteratorPtr) does not touch the relation when it is destroyed.
 *
 * With no live scan of the container being changed, the cost is one test of a pointer.
 */
struct BetaRowScanRegistry {
  enum Kind { kAllRows = 0, kPendingRows = 1, kIndexRows = 2, kNbrKinds = 3 };

  BetaRowScanRegistry() = default;
  BetaRowScanRegistry(BetaRowScanRegistry const&) = delete;
  BetaRowScanRegistry & operator=(BetaRowScanRegistry const&) = delete;

  inline void freeze(Kind kind);

  BetaRowScan * head[kNbrKinds] = {nullptr, nullptr, nullptr};
};

class BetaRowScan : public BetaRowIterator {
 public:
  using Kind = BetaRowScanRegistry::Kind;

  BetaRowScan(BetaRowScanRegistry * registry, Kind kind)
    : BetaRowIterator(), registry_(registry), kind_(kind), prev_(nullptr), next_(nullptr)
  {
    if(registry_) {
      next_ = registry_->head[kind_];
      if(next_) next_->prev_ = this;
      registry_->head[kind_] = this;
    }
  }

  ~BetaRowScan() override
  {
    unlink();
  }

  /**
   * Copy the rows this scan has yet to return, holding them, continue over the copy,
   * and leave the registry.
   */
  virtual void freeze()=0;

 protected:
  inline void
  unlink()
  {
    if(not registry_) return;
    if(prev_) prev_->next_ = next_;
    else registry_->head[kind_] = next_;
    if(next_) next_->prev_ = prev_;
    registry_ = nullptr;
    prev_ = next_ = nullptr;
  }

 private:
  BetaRowScanRegistry * registry_;
  Kind                  kind_;
  BetaRowScan *         prev_;
  BetaRowScan *         next_;
};

inline void
BetaRowScanRegistry::freeze(Kind kind)
{
  // each freeze unlinks its scan
  while(head[kind]) head[kind]->freeze();
}

/**
 * A scan over [begin, end) of a relation container: its row set (BetaRowPtr), its
 * pending list (BetaRowPtr) or the rows under one index key (BetaRow const*, read
 * backwards). `end` is read on every step, as a container's iterator compares to it,
 * so a scan of the pending list returns a row appended to the list while it runs, as
 * an iteration of the list did.
 *
 * `relation_rows` is the relation's row set, given to a scan of the set or of an
 * index: once frozen, the scan holds an index's rows by looking them up there, and
 * skips a row that has left the set (and, for an index, a row marked kDeleted, which
 * has left the index). Null for a scan of the pending list.
 */
template<class I>
class BetaRowRangeScan final : public BetaRowScan {
 public:
  BetaRowRangeScan(BetaRowScanRegistry * registry, Kind kind, I begin, I end,
    beta_row_set const* relation_rows)
    : BetaRowScan(registry, kind), itor_(begin), end_(end), relation_rows_(relation_rows),
      skip_deleted_(kind == BetaRowScanRegistry::kIndexRows),
      frozen_(false), done_(false), returned_(false), pos_(0)
  {
    if(itor_ == end_) finish();
  }

  bool is_end()const override
  {
    if(done_) return true;
    if(frozen_) {
      skip_rows_gone();
      return pos_ >= rows_.size();
    }
    return itor_ == end_;
  }

  bool next() override
  {
    if(is_end()) return false;
    if(frozen_) ++pos_;
    else ++itor_;
    returned_ = false;
    if(is_end()) {
      finish();
      return false;
    }
    return true;
  }

  BetaRow const* get_row()const override
  {
    if(is_end()) return nullptr;
    returned_ = true;
    return frozen_ ? rows_[pos_].get() : row_of(*itor_);
  }

  void freeze() override
  {
    if(not frozen_) {
      rows_.reserve(static_cast<std::size_t>(std::distance(itor_, end_)));
      for(I i = itor_; i != end_; ++i) rows_.push_back(hold(*i));
      frozen_ = true;
      pos_ = 0;
    }
    this->unlink();
  }

 private:
  static inline BetaRow const* row_of(BetaRowPtr const& row) { return row.get(); }
  static inline BetaRow const* row_of(BetaRow const* row) { return row; }

  inline BetaRowPtr hold(BetaRowPtr const& row)const { return row; }

  // An index holds raw pointers to rows of the set: the set holds the row
  inline BetaRowPtr
  hold(BetaRow const* row)const
  {
    if(relation_rows_) {
      auto found = relation_rows_->find(*row);
      if(found != relation_rows_->end() and found->get() == row) return *found;
    }
    // An indexed row is in the set: an index entry is removed before its row leaves
    // the set. Kept, without ownership, should that ever not hold
    assert(false and "BetaRowRangeScan: an indexed row is not in its relation");
    return BetaRowPtr(BetaRowPtr(), const_cast<BetaRow*>(row));
  }

  // Once frozen, a row of the set or of an index that has left the set is skipped,
  // and a row of an index marked kDeleted, which has left the index; but not the row
  // already returned, which its caller may be processing
  inline void
  skip_rows_gone()const
  {
    if(not relation_rows_ or returned_) return;
    while(pos_ < rows_.size()) {
      auto const* row = rows_[pos_].get();
      if(not (skip_deleted_ and row->is_deleted())) {
        auto found = relation_rows_->find(*row);
        if(found != relation_rows_->end() and found->get() == row) return;
      }
      ++pos_;
    }
  }

  // Exhausted: no container is read again, so the scan leaves the registry
  inline void
  finish()
  {
    done_ = true;
    this->unlink();
  }

  I itor_;
  I end_;
  beta_row_set const* relation_rows_;
  bool skip_deleted_;                  // a scan of an index
  bool frozen_;
  bool done_;                          // exhausted
  mutable bool returned_;              // get_row returned the row at the position
  mutable std::size_t pos_;
  std::vector<BetaRowPtr> rows_;       // once frozen: what was left to return, held
};

using BetaRowSetScan = BetaRowRangeScan<beta_row_set::const_iterator>;
using BetaRowListScan = BetaRowRangeScan<beta_row_list::const_iterator>;
using BetaRowIndexScan = BetaRowRangeScan<std::reverse_iterator<BetaRow const* const*>>;

}  // namespace jets::rete
#endif  // JETS_RETE_BETA_ROW_ITERATOR_H
