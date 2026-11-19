#ifndef JETS_RETE_BETA_RELATION_H
#define JETS_RETE_BETA_RELATION_H

#include <algorithm>
#include <cassert>
#include <cstdint>
#include <string>
#include <memory>
#include <utility>
#include <list>
#include <tuple>
#include <vector>

#include "absl/container/flat_hash_map.h"
#include "absl/container/flat_hash_set.h"
#include "absl/container/inlined_vector.h"

#include "../rdf/rdf_types.h"
#include "../rete/node_vertex.h"
#include "../rete/beta_row_initializer.h"
#include "../rete/beta_row.h"
#include "../rete/beta_row_iterator.h"

// Component to manage all the rdf resources and literals of a graph
namespace jets::rete {
// //////////////////////////////////////////////////////////////////////////////////////
// BetaRelation class -- main class for the rete network
// --------------------------------------------------------------------------------------
// Forward declaration
class AlphaNode;
class ReteSession;
struct AQVIndexBetaRowsVisitor;
struct AQVRemoveIndexBetaRowsVisitor;

class BetaRelation;
using BetaRelationPtr = std::shared_ptr<BetaRelation>;

// The supports of a row of a vertex that counts them (NodeVertex::counts_supports):
// for each derivation that makes the row, the values of the parent row at the
// positions the row leaves out (NodeVertex::support_key_positions). A derivation is
// a parent row and the triple it joined, and every variable of the triple is in the
// row, so those values tell the derivations of one row apart. A set, not a count: a
// derivation offered twice, by a scan and by a callback that both reach it, is one
// support, and withdrawing it twice withdraws it once.
using SupportKey = absl::InlinedVector<rdf::r_index, 2, ArenaAllocator<rdf::r_index>>;
using SupportSet = absl::flat_hash_set<SupportKey, absl::Hash<SupportKey>, std::equal_to<SupportKey>,
  ArenaAllocator<SupportKey>>;

// One row's supports. Nearly every row has one (measured on rule sets of several
// hundred thousand such rows, about one in two thousand to one in fourteen had a
// second), so the first is held in place and a set is allocated, in the row's arena,
// only for the others: a set per row was most of what keeping supports cost.
class RowSupports {
 public:
  explicit RowSupports(SupportKey first) : first_(std::move(first)), rest_(nullptr) {}
  RowSupports(RowSupports && other) noexcept
    : first_(std::move(other.first_)), rest_(std::exchange(other.rest_, nullptr)) {}
  RowSupports & operator=(RowSupports && other) noexcept
  {
    if(this != &other) {
      release();
      first_ = std::move(other.first_);
      rest_ = std::exchange(other.rest_, nullptr);
    }
    return *this;
  }
  RowSupports(RowSupports const&) = delete;
  RowSupports & operator=(RowSupports const&) = delete;
  ~RowSupports() { release(); }

  inline std::size_t
  size()const
  {
    return 1 + (rest_ ? rest_->size() : 0);
  }

  inline void
  add(SupportKey const& support)
  {
    if(support == first_) return;
    if(not rest_) {
      ArenaAllocator<SupportSet> alloc(first_.get_allocator());
      rest_ = alloc.allocate(1);
      ::new (static_cast<void*>(rest_)) SupportSet(SupportSet::allocator_type(alloc));
    }
    rest_->insert(support);
  }

  // Withdraws `support`; false when none is left
  inline bool
  withdraw(SupportKey const& support)
  {
    if(support == first_) {
      if(not rest_ or rest_->empty()) return false;
      auto next = rest_->begin();
      first_ = *next;
      rest_->erase(next);
      return true;
    }
    if(rest_) rest_->erase(support);
    return true;
  }

  // The arena the supports are in, for ~ReteSession's checks
  inline bool
  in(BetaRowArena const* arena)const
  {
    return first_.get_allocator().arena() == arena and (not rest_ or rest_->get_allocator().arena() == arena);
  }

 private:
  inline void
  release()
  {
    if(not rest_) return;
    ArenaAllocator<SupportSet> alloc(first_.get_allocator());
    rest_->~SupportSet();
    alloc.deallocate(rest_, 1);
    rest_ = nullptr;
  }

  SupportKey   first_;
  SupportSet * rest_;
};

using SupportMap = absl::flat_hash_map<BetaRow const*, RowSupports, absl::Hash<BetaRow const*>,
  std::equal_to<BetaRow const*>, ArenaAllocator<std::pair<BetaRow const* const, RowSupports>>>;

// The support key of the derivation of a row of `node_vertex` from `parent_row`, in
// `arena`: empty when the vertex does not count supports
inline SupportKey
make_support_key(b_index node_vertex, BetaRow const* parent_row, BetaRowArena * arena)
{
  SupportKey key{ArenaAllocator<rdf::r_index>(arena)};
  if(not node_vertex or not parent_row) return key;
  for(int pos: node_vertex->support_key_positions) key.push_back(parent_row->get(pos));
  return key;
}

// container for holding all beta_rows
// Forward declaration in beta_row_iterator.h

// BetaRelation making the rete network
class BetaRelation {
 public:
  BetaRelation()
    : node_vertex_(nullptr),
      is_activated_(false),
      all_beta_rows_(),
      pending_beta_rows_(),
      beta_row_idx1_(),
      beta_row_idx2_(),
      beta_row_idx3_(),
      supports_(),
      arena_(nullptr)
    {}

  // `arena` is the owning ReteSession's, null outside a session (global heap). A
  // relation in an arena is never destroyed: the session releases the arena.
  explicit BetaRelation(b_index node_vertex, BetaRowArena * arena = nullptr)
    : node_vertex_(node_vertex),
      is_activated_(false),
      all_beta_rows_(ArenaAllocator<BetaRowPtr>(arena)),
      pending_beta_rows_(ArenaAllocator<BetaRowPtr>(arena)),
      beta_row_idx1_(BetaRowIndxVec1::allocator_type(ArenaAllocator<BetaRowIndexes1>(arena))),
      beta_row_idx2_(BetaRowIndxVec2::allocator_type(ArenaAllocator<BetaRowIndexes2>(arena))),
      beta_row_idx3_(BetaRowIndxVec3::allocator_type(ArenaAllocator<BetaRowIndexes3>(arena))),
      supports_(SupportMap::allocator_type(arena)),
      arena_(arena)
    {}

  inline BetaRowArena *
  arena()const
  {
    return arena_;
  }

  // True when this relation and every container it owns allocate from `arena`.
  // Checked by ~ReteSession in debug and AddressSanitizer builds, not on any hot path.
  inline bool
  containers_in(BetaRowArena const* arena)const
  {
    if(arena_ != arena) return false;
    if(all_beta_rows_.get_allocator().arena() != arena) return false;
    if(pending_beta_rows_.get_allocator().arena() != arena) return false;
    if(supports_.get_allocator().arena() != arena) return false;
    for(auto const& [row, supports]: supports_) {
      if(not supports.in(arena)) return false;
    }
    auto in_arena = [arena](auto const& vec) {
      if(vec.get_allocator().arena() != arena) return false;
      for(auto const& idx: vec) {
        if(idx.get_allocator().arena() != arena) return false;
      }
      return true;
    };
    return in_arena(beta_row_idx1_) and in_arena(beta_row_idx2_) and in_arena(beta_row_idx3_);
  }

  inline b_index
  get_node_vertex()const
  {
    return node_vertex_;
  }

  inline bool
  is_activated()const
  {
    return is_activated_;
  }

  inline void
  set_activated(bool b)
  {
    is_activated_ = b;
  }

  inline void
  clear_pending_rows()
  {
    scans_.freeze(BetaRowScanRegistry::kPendingRows);
    pending_beta_rows_.clear();
  }

  inline bool
  has_pending_rows()
  {
    return not pending_beta_rows_.empty();
  }

  // Scans of the relation's rows and of its pending rows, made where they are used
  // (no allocation) and frozen if the relation changes what they scan
  inline BetaRowSetScan
  scan_all_rows()const
  {
    return BetaRowSetScan(&scans_, BetaRowScanRegistry::kAllRows, all_beta_rows_.begin(), all_beta_rows_.end(),
      &all_beta_rows_);
  }

  inline BetaRowListScan
  scan_pending_rows()const
  {
    return BetaRowListScan(&scans_, BetaRowScanRegistry::kPendingRows,
      pending_beta_rows_.begin(), pending_beta_rows_.end(), nullptr);
  }

  // A copy of the relation's rows, for a caller that changes the relation as it goes
  inline std::vector<BetaRowPtr>
  copy_all_rows()const
  {
    return std::vector<BetaRowPtr>(all_beta_rows_.begin(), all_beta_rows_.end());
  }

  inline BetaRowIteratorPtr
  get_all_rows_iterator()const
  {
    return std::make_shared<BetaRowSetScan>(&scans_, BetaRowScanRegistry::kAllRows,
      all_beta_rows_.begin(), all_beta_rows_.end(), &all_beta_rows_);
  }

  inline BetaRowIteratorPtr
  get_pending_rows_iterator()const
  {
    return std::make_shared<BetaRowListScan>(&scans_, BetaRowScanRegistry::kPendingRows,
      pending_beta_rows_.begin(), pending_beta_rows_.end(), nullptr);
  }

  inline std::size_t
  size()const
  {
    return all_beta_rows_.size();
  }

  // Whether `row` itself is in the relation and not withdrawn (marked kDeleted).
  // The same instance, where the Go engine compared rows by equality: a parent row
  // removed and an equal one inserted again during a filter drops the candidate here,
  // and the engines still converge, because the re-inserted row is pending and its own
  // visit of the child makes the same candidate.
  inline bool
  holds(BetaRow const* row)const
  {
    if(row->is_deleted()) return false;
    auto itor = all_beta_rows_.find(*row);
    return itor != all_beta_rows_.end() and itor->get() == row;
  }

  // The row set's capacity, for tests and reports
  inline std::size_t
  capacity()const
  {
    return all_beta_rows_.capacity();
  }

  // The largest number of rows the relation held in this session
  inline std::size_t
  peak_size()const
  {
    return peak_rows_;
  }

  // At most this many rows are reserved from a hint: it keeps nearly all of what the
  // hint saves on small sessions, and bounds a table sized for a larger session than
  // this one to about 8,000 slots
  static constexpr std::size_t kMaxReservedRows = 4096;

  /**
   * @brief Size the row set from the last session's peak for this vertex, before the
   * relation's first visit fills it
   *
   * Half the hint, at most kMaxReservedRows: a table grows by doubling, so half the
   * peak saves the growths below it without reserving past what a smaller session
   * needs.
   *
   * Called at every visit of the relation before it is activated, and only the first
   * call reserves. A later one can come while the first visit is still running: a
   * filter there re-enters the session, its insert reaches an activated
   * ancestor, and that ancestor's visit walks its whole subtree, this relation and the
   * rows it already holds included. At the first call the set is empty and unscanned
   * (a relation receives rows only from its first visit or once activated); a reserve
   * rehashes, so live scans are frozen first all the same.
   */
  inline void
  reserve_from_hint()
  {
    if(hint_consumed_ or not node_vertex_) return;
    hint_consumed_ = true;
    std::size_t hint = node_vertex_->beta_rows_hint.load(std::memory_order_relaxed) / 2;
    if(hint < 2) return;
    assert(all_beta_rows_.empty() and "BetaRelation::reserve_from_hint: rows before the first visit");
    if(not all_beta_rows_.empty()) return;
    scans_.freeze(BetaRowScanRegistry::kAllRows);
    all_beta_rows_.reserve(std::min(hint, kMaxReservedRows));
  }

  // Leave this session's peak as the hint for the next session's relation
  inline void
  store_size_hint()const
  {
    if(not node_vertex_) return;
    auto peak = std::min<std::size_t>(peak_rows_, UINT32_MAX);
    node_vertex_->beta_rows_hint.store(static_cast<std::uint32_t>(peak), std::memory_order_relaxed);
  }

  /**
   * @brief Insert BetaRow into BetaRelation and transfer ownsership of the row as well.
   * 
   * @param rete_session Current session that owns the BetaRelation
   * @param beta_row the row to insert
   * @param support the derivation's support key (make_support_key), when the vertex
   *        counts supports
   * @return int 0 if normal, -1 if error
   */
  // Defined in rete_session.h
  int
  insert_beta_row(ReteSession * rete_session, BetaRowPtr beta_row, SupportKey const* support = nullptr);

  /**
   * @brief Remove `beta_row` from beta relation if fount in beta relation, whatever
   * supports it: every one is dropped (a filter's replay, which then inserts again
   * what its parent rows still make)
   * 
   * @param rete_session 
   * @param beta_row BetaRow to remove
   * @return int 
   */
  // Defined in rete_session.h
  int
  remove_beta_row(ReteSession * rete_session, BetaRowPtr beta_row);

  /**
   * @brief Insert, or remove, the derivation of the row equal to `candidate`, a row
   * built in scratch storage (ReteSession::CandidateRow) from `parent_row`
   *
   * The same as insert_beta_row and remove_beta_row, except that a row is allocated
   * only when the candidate is inserted and no equal row is in the relation; the
   * candidate itself is never held. When the vertex counts supports, an insert adds
   * the derivation's support to the row, and a remove withdraws it and removes the
   * row only when no other support is left.
   */
  // Defined in rete_session.h
  int
  insert_candidate(ReteSession * rete_session, BetaRow const& candidate, BetaRow const* parent_row);

  // Defined in rete_session.h
  int
  remove_candidate(ReteSession * rete_session, BetaRow const& candidate, BetaRow const* parent_row);

  // The number of derivations supporting `row` when its vertex counts supports, 0
  // otherwise or when the row is not in the relation; for tests and reports
  inline std::size_t
  support_count(BetaRow const* row)const
  {
    auto itor = supports_.find(row);
    return itor == supports_.end() ? 0 : itor->second.size();
  }

  /**
   * @brief Erase `beta_row` once its consequents have been retracted
   *
   * remove_beta_row marks a row with consequent terms kDeleted, removes its indexes and notifies the
   * child nodes, and leaves it in the relation until compute_consequent_triples has
   * retracted its consequents. This is the last step. Only that instance, still marked
   * kDeleted, is erased: a row re-inserted in the meantime was set back to kProcessed
   * by insert_beta_row and is never retracted. Restored from 1d993844, which c2de6953
   * removed.
   */
  inline void
  erase_retracted_beta_row(BetaRowPtr const& beta_row)
  {
    auto itor = this->all_beta_rows_.find(beta_row);
    if(itor != this->all_beta_rows_.end() and itor->get() == beta_row.get() and beta_row->is_deleted()) {
      this->supports_.erase(beta_row.get());
      this->scans_.freeze(BetaRowScanRegistry::kAllRows);
      this->all_beta_rows_.erase(itor);
    }
  }

  /**
   * @brief Get the idx1 rows iterator object
   * 
   * @param key the index key of the AntecedentQuery
   * @param r 
   * @return BetaRowIteratorPtr 
   */
  inline BetaRowIteratorPtr
  get_idx1_rows_iterator(int key, rdf::r_index u)const
  {
    return this->index_scan(this->beta_row_idx1_[key].rows(u));
  }

  inline BetaRowIteratorPtr
  get_idx2_rows_iterator(int key, rdf::r_index u, rdf::r_index v)const
  {
    return this->index_scan(this->beta_row_idx2_[key].rows({u, v}));
  }

  inline BetaRowIteratorPtr
  get_idx3_rows_iterator(int key, rdf::r_index u, rdf::r_index v, rdf::r_index w)const
  {
    return this->index_scan(this->beta_row_idx3_[key].rows({u, v, w}));
  }

  /**
   * @brief Initialize the BetaRelation object indexes
   * 
   * Allocate and initialize all 3 indexes
   *    - BetaRowIndxVec1
   *    - BetaRowIndxVec2
   *    - BetaRowIndxVec3
   * @return int 
   */
  // Defined in rete_session.h
  int
  initialize(ReteSession * rete_session);

 protected:

  // The rows under one index key, newest first
  template<class Rows>
  inline BetaRowIteratorPtr
  index_scan(Rows rows)const
  {
    using RI = std::reverse_iterator<BetaRow const* const*>;
    return std::make_shared<BetaRowIndexScan>(&scans_, BetaRowScanRegistry::kIndexRows,
      RI(rows.last), RI(rows.first), &all_beta_rows_);
  }

  // Defined in rete_session.h: the rest of an insert once the row is found or added
  // to all_beta_rows_, and of a remove once it is looked up
  int
  row_inserted(ReteSession * rete_session, BetaRowPtr const& beta_row, bool is_new);

  int
  row_removed(ReteSession * rete_session, beta_row_set::const_iterator itor);

  // Adds `support` to the row's supports, when its vertex counts them
  inline void
  add_support(BetaRow const* row, SupportKey const* support)
  {
    if(not support or not node_vertex_ or not node_vertex_->counts_supports()) return;
    auto [itor, is_new] = supports_.try_emplace(row, *support);
    if(not is_new) itor->second.add(*support);
  }

  // The same, taking the key: a new row's first support is moved into place
  inline void
  add_support(BetaRow const* row, SupportKey && support)
  {
    if(not node_vertex_ or not node_vertex_->counts_supports()) return;
    // try_emplace moves the key only when it inserts (one probe either way)
    auto [itor, is_new] = supports_.try_emplace(row, std::move(support));
    if(not is_new) itor->second.add(support);
  }

  // Withdraws `support` from the row and says whether the row is still supported, and
  // so stays. A key that is not one of the row's derivations (one a filter refused
  // when it was offered) withdraws nothing. A row of a vertex that does not count
  // supports, or that was given none, has one derivation, and goes
  inline bool
  withdraw_support(BetaRow const* row, SupportKey const& support)
  {
    if(not node_vertex_ or not node_vertex_->counts_supports()) return false;
    auto itor = supports_.find(row);
    if(itor == supports_.end()) return false;
    if(itor->second.withdraw(support)) return true;
    supports_.erase(itor);
    return false;
  }

  // Drops every support of `row`
  inline void
  drop_supports(BetaRow const* row)
  {
    supports_.erase(row);
  }

  // Erases `row` from all_beta_rows_ and drops its supports
  inline void
  erase_row(BetaRowPtr const& row)
  {
    this->supports_.erase(row.get());
    this->scans_.freeze(BetaRowScanRegistry::kAllRows);
    this->all_beta_rows_.erase(row);
  }

  // Defined in rete_session.h
  /**
   * @brief Remove indexes associated with argument beta_row
   * 
   * @param rete_session 
   * @param beta_row row of this beta node that is being removed
   * Argument are assumed to be not null
   */
  void
  remove_indexes(ReteSession * rete_session, BetaRowPtr beta_row);

 private:
  
  friend std::ostream & operator<<(std::ostream &, BetaRelation const*);
  friend class AlphaNode;
  friend struct AQVIndexBetaRowsVisitor;
  friend struct AQVRemoveIndexBetaRowsVisitor;
  friend struct AQVInitializeIndexesVisitor;

  b_index         node_vertex_;
  bool            is_activated_;
  beta_row_set    all_beta_rows_;
  beta_row_list   pending_beta_rows_;
  BetaRowIndxVec1 beta_row_idx1_;
  BetaRowIndxVec2 beta_row_idx2_;
  BetaRowIndxVec3 beta_row_idx3_;
  SupportMap      supports_;               // see SupportKey; rows of a vertex that counts supports
  BetaRowArena *  arena_;
  std::size_t     peak_rows_ = 0;           // see peak_size()
  bool            hint_consumed_ = false;   // see reserve_from_hint()
  mutable BetaRowScanRegistry scans_;      // live scans, frozen before a change they would not survive
};

inline std::ostream & operator<<(std::ostream & out, BetaRelation const* node)
{
  if(not node) out << "NULL";
  else {
    out << "BetaRelation: vertex "<<node->get_node_vertex()->vertex <<
      ", Q1 size "<< node->beta_row_idx1_.size()<<
      ", Q2 size "<< node->beta_row_idx2_.size()<<
      ", Q3 size "<< node->beta_row_idx3_.size();
  }
  return out;
}

inline std::ostream & operator<<(std::ostream & out, BetaRelationPtr node)
{
  if(not node) out << "NULL";
  else {
    out << node.get();
  }
  return out;
}

// A relation on the global heap, outside any ReteSession
inline BetaRelationPtr 
create_beta_node(b_index node_vertex)
{
  return std::make_shared<BetaRelation>(node_vertex);
}

// A relation constructed in `arena`, which owns it: it is never destroyed, and is
// gone, with its rows and indexes, when the arena is released
inline BetaRelation *
create_beta_node(BetaRowArena * arena, b_index node_vertex)
{
  void * p = arena->allocate(sizeof(BetaRelation), alignof(BetaRelation));
  return ::new (p) BetaRelation(node_vertex, arena);
}

} // namespace jets::rete
#endif // JETS_RETE_BETA_RELATION_H
