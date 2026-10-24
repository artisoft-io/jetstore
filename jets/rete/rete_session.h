#ifndef JETS_RETE_RETE_SESSION_H
#define JETS_RETE_RETE_SESSION_H

#include <queue>
#include <string>
#include <memory>
#include <list>
#include <vector>

#include "absl/hash/hash.h"
#include "absl/container/flat_hash_set.h"

#include "../rdf/rdf_types.h"
#include "../rete/node_vertex.h"
#include "../rete/beta_row_arena.h"
#include "../rete/beta_row_initializer.h"
#include "../rete/beta_row.h"
#include "../rete/beta_row_iterator.h"
#include "../rete/beta_relation.h"
#include "../rete/graph_callback_mgr_impl.h"
#include "../rete/alpha_node.h"
#include "../rete/expr.h"
#include "../rete/rete_meta_store.h"


// Component to manage all the rdf resources and literals of a graph
namespace jets::rete {
// //////////////////////////////////////////////////////////////////////////////////////
// ReteSession class -- main session class for the rete network
// --------------------------------------------------------------------------------------
class ReteSession;
using ReteSessionPtr = std::shared_ptr<ReteSession>;

// The relations live in the session's arena, which owns them (see ~ReteSession)
using BetaRelationVector = std::vector<BetaRelation *>;
using VertexVisitsVector = std::vector<std::pair<int, int>>;
using AlphaNodeVector = std::vector<AlphaNodePtr>;

struct BetaRowPriorityCompare {
  inline bool
  operator()(BetaRowPtr const& lhs, BetaRowPtr const& rhs) {
    auto const& lv = lhs->get_node_vertex();
    auto const& rv = rhs->get_node_vertex();
    if(lv->salience == rv->salience) {
      return lv->vertex > rv->vertex;
    }
    return lv->salience < rv->salience;
  }
};
using BetaRowPriorityQueue = std::priority_queue<BetaRowPtr, std::vector<BetaRowPtr>, BetaRowPriorityCompare>;

/**
 * @brief ReteSession making the rete network
 * 
 */
class ReteSession {
 public:
  using Iterator = rdf::RDFSession::Iterator;

  // The vertex-visit limit of a session whose meta graph does not set
  // (jets:iState jets:max_vertex_visits <n>): without one, a rule that never
  // settles ran until the pipeline timed out, with no message (Q-83, 2026-10-23).
  // A process that sets the triple gets its own value, and a value of 0 or less
  // turns the limit off, as before.
  static constexpr int kDefaultMaxVertexVisits = 10000;

  ReteSession(ReteMetaStorePtr rule_ms, rdf::RDFSession * rdf_session) 
    : arena_(),
      rule_ms_(rule_ms),
      rdf_session_(rdf_session),
      beta_relations_(),
      vertex_visits_(),
      pending_compute_consequent_beta_rows_(),
      err_msg_(),
      max_vertex_visit_(0),
      max_vertex_visit_is_default_(false),
      recording_row_(nullptr),
      scratch_rows_(),
      scratch_depth_(0),
      held_rows_(),
      reentries_(0)
    {}

  ReteSession(ReteSession const&) = delete;
  ReteSession & operator=(ReteSession const&) = delete;

  /**
   * @brief Release the session's beta relations in bulk
   *
   * The relations, their rows and the nodes and buckets of their containers are all
   * in arena_, and none of them is destroyed: arena_ frees its blocks, declared first
   * so that it is destroyed last. Destroying them one by one cost a large session
   * about a fifth of its time. What holds a row outside the arena, the consequent
   * queue, is destroyed after this body, as a member declared after arena_, and
   * releases its rows into the arena as before.
   *
   * In a debug or AddressSanitizer build, every relation's containers are first
   * checked to be in arena_: one that is not would never be freed. At verbosity 1
   * (glog VLOG) the arena's blocks are reported against its live bytes, which is the
   * memory its free lists and block tails held (see beta_row_arena.h).
   */
  ~ReteSession()
  {
#if !defined(NDEBUG) || defined(JETS_ARENA_ASAN)
    for(auto const* relation: beta_relations_) {
      if(not relation->containers_in(&arena_)) {
        LOG(FATAL) << "~ReteSession: a container of the beta relation at vertex "
          << relation->get_node_vertex()->vertex << " is not in the session's arena";
      }
    }
#endif
    // Each visited relation's peak sizes the next session's (reserve_from_hint)
    for(auto const* relation: beta_relations_) {
      if(relation->is_activated()) relation->store_size_hint();
    }
    VLOG(1) << "ReteSession released: arena block_bytes " << arena_.block_bytes()
      << " live_bytes " << arena_.live_bytes() << " nbr_blocks " << arena_.nbr_blocks()
      << " large_bytes " << arena_.large_bytes() << " nbr_large " << arena_.nbr_large();
  }

  // The arena the session's beta relations and rows are allocated from
  inline BetaRowArena *
  arena()
  {
    return &arena_;
  }

  /**
   * @brief A candidate beta row, built in the session's scratch storage
   *
   * A candidate is initialized, filtered, then inserted into or removed from its
   * relation (BetaRelation::insert_candidate, remove_candidate), which allocates a row
   * only when it is kept. The scratch rows are reused, one per nesting level: a filter
   * can re-enter the session (the error path inserts into the asserted graph, whose
   * callbacks build candidates of their own) while the outer candidate is live. They
   * are in the arena and released with it.
   */
  class CandidateRow {
   public:
    CandidateRow(ReteSession * rete_session, b_index node_vertex, int size)
      : rete_session_(rete_session)
    {
      auto & rows = rete_session->scratch_rows_;
      auto depth = rete_session->scratch_depth_;
      if(depth == rows.size()) {
        void * p = rete_session->arena_.allocate(sizeof(BetaRow), alignof(BetaRow));
        rows.push_back(::new (p) BetaRow(nullptr, 0, &rete_session->arena_));
      }
      row_ = rows[depth];
      row_->reset(node_vertex, size);
      // Last: what precedes can throw (bad_alloc), and the destructor does not run then
      rete_session->scratch_depth_ = depth + 1;
    }
    ~CandidateRow() { --rete_session_->scratch_depth_; }
    CandidateRow(CandidateRow const&) = delete;
    CandidateRow & operator=(CandidateRow const&) = delete;

    inline BetaRow * get()const { return row_; }

   private:
    ReteSession * rete_session_;
    BetaRow *     row_;
  };

  /**
   * @brief The candidates a relation's first scan holds until it ends (I-91)
   *
   * A filter can write to the graph, and the write's callbacks re-enter the session
   * at once: they can remove a parent row the scan built candidates from, or change
   * what another candidate's filter reads, before or after that candidate. A relation
   * that is not activated yet is reached by no callback, so neither the removal nor a
   * filter's replay (triple_updated_for_filters) would reach a row inserted from such
   * a candidate. Its first scan, when its vertex has a filter (nothing else in a scan
   * writes to the graph), therefore holds its candidates, as the rows that would
   * be inserted (allocated as insert_candidate would allocate them), in the session's
   * held_rows_ above where the scan began. If the session was re-entered meanwhile it
   * drops them and scans again; the rows are inserted once a scan completes with no
   * re-entry. A scan started by a re-entry holds above its caller's and ends first.
   * Whatever is still held when the scan leaves, by an error, is dropped.
   */
  class HeldCandidates {
   public:
    explicit HeldCandidates(ReteSession * rete_session)
      : rete_session_(rete_session), base_(rete_session->held_rows_.size()) {}
    ~HeldCandidates() { drop(); }
    HeldCandidates(HeldCandidates const&) = delete;
    HeldCandidates & operator=(HeldCandidates const&) = delete;

    inline void
    hold(BetaRow const& candidate)
    {
      rete_session_->held_rows_.push_back(create_beta_row(&rete_session_->arena_, candidate));
    }

    inline void
    drop()
    {
      auto & rows = rete_session_->held_rows_;
      if(rows.size() > base_) rows.erase(rows.begin() + static_cast<std::ptrdiff_t>(base_), rows.end());
    }

    // Inserting a row evaluates nothing and builds no candidate
    inline int insert_into(BetaRelation * relation);

   private:
    ReteSession * rete_session_;
    std::size_t   base_;
  };

  // How many times in a row a filter is evaluated again, or a first scan made again,
  // because the session was re-entered meanwhile. Each retry needs a new write by a
  // filter, so a rule set that reaches it writes without end: an error, not a result
  static constexpr int kMaxReentryRetries = 16;

  inline rdf::RDFSession *
  rdf_session()
  {
    return rdf_session_;
  }

  // The last error initialize or execute_rules2 reported, empty if none
  inline std::string const&
  get_err_msg()const
  {
    return err_msg_;
  }

  inline ReteMetaStore const*
  rule_ms()const
  {
    return rule_ms_.get();
  }

  inline BetaRelation *
  get_beta_relation(int vertex)
  {
    if(vertex<0 or vertex>= static_cast<int>(beta_relations_.size())) return nullptr;
    return beta_relations_[vertex];
  }

  inline BetaRelationVector const&
  beta_relations()const
  {
    return beta_relations_;
  }
  
  int 
  execute_rules();

  char const* execute_rules2(int*v);

  /**
   * @brief Notification function called when a triple is added to the inferred graph
   * 
   * @param vertex key of NodeVertex that registered the call back
   * @param s triple's subject inserted
   * @param p triple's predicate inserted
   * @param o triple's object inserted
   * @return int 
   */
  inline int
  triple_inserted(int vertex, rdf::r_index s, rdf::r_index p, rdf::r_index o)
  {
    return triple_updated(vertex, s, p, o, true);
  }

  /**
   * @brief Notification function called when a triple is deleted from the inferred graph
   * 
   * @param vertex key of NodeVertex that registered the call back
   * @param s triple's subject deleted
   * @param p triple's predicate deleted
   * @param o triple's object deleted
   * @return int 
   */
  inline int
  triple_deleted(int vertex, rdf::r_index s, rdf::r_index p, rdf::r_index o)
  {
    return triple_updated(vertex, s, p, o, false);
  }

  /**
   * @brief Notification function called when a triple is added to the inferred graph
   * 
   * This is called by callback manager created for rule term filter
   * 
   * @param vertex key of NodeVertex that registered the call back
   * @param s triple's subject inserted
   * @param p triple's predicate inserted
   * @param o triple's object inserted
   * @return int 
   */
  inline int
  triple_inserted_for_filters(int vertex, rdf::r_index s, rdf::r_index p, rdf::r_index o)
  {
    return triple_updated_for_filters(vertex, s, p, o, true);
  }

  /**
   * @brief Notification function called when a triple is deleted from the inferred graph
   * 
   * This is called by callback manager created for rule term filter
   * 
   * @param vertex key of NodeVertex that registered the call back
   * @param s triple's subject deleted
   * @param p triple's predicate deleted
   * @param o triple's object deleted
   * @return int 
   */
  inline int
  triple_deleted_for_filters(int vertex, rdf::r_index s, rdf::r_index p, rdf::r_index o)
  {
    return triple_updated_for_filters(vertex, s, p, o, false);
  }

  /**
   * @brief Initialize ReteSession using ReteMetaStore
   *
   *  - Initialize BetaRelationVector beta_relations_ such that
   *    `beta_relations_[ipos] = create_beta_node(&arena_, rule_ms_->node_vertexes_[ipos]);`,
   *    a relation constructed in the session's arena
   *  - Register GraphCallbackManager using antecedent AlphaNode adaptor
   *  - Initialize VertexVisitsVector with zero values
   * 
   * @param rule_ms ReteMetaStore to use for the ReteSession
   * 
   * @return int 
   */
  int
  initialize();

  /**
   * @brief Terminate the ReteSession, remove dependency on RdfSession
   * 
   * @return int 0 if normal
   */
  int
  terminate();

  /**
   * @brief Insert a triple inferred by `beta_row`'s consequent terms
   *
   * When `beta_row` is the row compute_consequent_triples is currently recording
   * (its vertex is flagged records_consequents), a triple that reached the inferred
   * graph -- one whose reference count this insert raised -- is also recorded on the
   * row, so that retracting the row retracts exactly it. Called for the consequent
   * triple itself and by create_entity for the entity's jets:key triple. Called with
   * any other row, e.g. a candidate row a filter is evaluated on, it only inserts.
   *
   * @return what RDFSession::insert_inferred returns
   */
  inline int
  insert_inferred_for(BetaRow const* beta_row, rdf::Triple const& t3);

  /**
   * @brief Count a callback ignored because its relation is not activated yet
   *
   * As triple_updated and triple_updated_for_filters count the ones they receive, so
   * that every graph change a callback sees moves the count, as in the retired Go engine.
   */
  inline void
  count_ignored_reentry()
  {
    ++reentries_;
  }

 protected:
  int
  set_graph_callbacks();

  int
  remove_graph_callbacks();

  /**
   * @brief Notification for triple inserted/deleted
   * 
   * @param vertex key of NodeVertex that registered the call back
   * @param s triple's subject 
   * @param p triple's predicate 
   * @param o triple's object 
   * @param is_inserted true if inserted, false for deleted triple
   * @return int 
   */
  int
  triple_updated(int vertex, rdf::r_index s, rdf::r_index p, rdf::r_index o, bool is_inserted);

  /**
   * @brief Notification for triple inserted/deleted for rule filter terms
   * 
   * @param vertex key of NodeVertex that registered the call back
   * @param s triple's subject 
   * @param p triple's predicate 
   * @param o triple's object 
   * @param is_inserted true if inserted, false for deleted triple
   * @return int 
   */
  int
  triple_updated_for_filters(int vertex, rdf::r_index s, rdf::r_index p, rdf::r_index o, bool is_inserted);

  /**
   * @brief Execute inferrence on rete graph
   * 
   * @param from_vertex start vertex node
   * @param is_inferring apply forward chaining if true, otherwise retract inferrence
   * @param compute_consequents add inferred triples to rdf session
   * @return int 0 for normal, -1 if error
   */
  int
  execute_rules(int from_vertex, bool is_inferring, bool compute_consequents);

/**
 * @brief Visit the Rete Graph and apply inferrence or retactation of inferred facts
 * 
 * Perform DFS graph visitation starting at node `from_vertex`
 *
 * @param from_vertex Starting point of graph visitation
 * @param is_inferring apply inferrence if true, retract inferrence if false
 * @return int 0 if normal, -1 if error
 */
  int 
  visit_rete_graph(int from_vertex, bool is_inferring);

  /**
   * @brief Schedule consequent terms of the rule associated with the `beta_row`
   * 
   * @param beta_row Inferred or retracted BetaRow
   * @return int 0 if normal, -1 if error
   */
  int
  schedule_consequent_terms(BetaRowPtr beta_row);

  /**
   * @brief Compute consequent triples from scheduled consequent terms
   *
   * Scheduled consequent terms are processed using a priority queue with
   * a priority based on rule salience.
   * Inferred triples are added to inferred graph of rdf session which
   * trigger rule having antecedent matching the inferred triple to
   * activate and in turn to infer or retract triples.
   *
   * TODO Add logging to trach which rule inferre which triple
   * to be able to explain how a triple got inferred.
   * 
   * @return int 0 for normal, -1 if error
   */
  int
  compute_consequent_triples();

  /**
   * @brief Whether a candidate a filter kept is still supported (I-91)
   *
   * Called by keep_candidate, in a scan of an activated relation, when a callback has
   * seen a graph change since the scan began: a filter wrote a triple (create_entity,
   * or an operator's error path through a jets:exception) and its callbacks inserted
   * into or removed from relations, or were ignored because their relation is not
   * activated yet (count_ignored_reentry). The filter need not be the candidate's
   * own. Such a write can remove the candidate's support: its parent row, the triple
   * it joined, or the absence a negation tested. The removal was propagated at once,
   * before the candidate was in its relation, so inserting it now would leave a row,
   * and its consequents, that nothing retracts. It is dropped instead: the rule fires
   * as it does when the filter's triple was there from the start.
   *
   * A removed parent row is also appended to its relation's pending rows, and when a
   * scan of those rows is frozen it holds the row twice; the second pass makes
   * candidates from the removed row, which this drops as well.
   *
   * @param t3 the triple a positive antecedent joined; null for a negated one
   */
  bool
  candidate_still_supported(BetaRelation const* parent_relation, BetaRow const* parent_row,
    AlphaNode const* alpha_node, rdf::Triple const* t3);

  // The filter of `meta_node`, if any, evaluated once
  inline bool
  passes_filter(b_index meta_node, BetaRow const* row);

  /**
   * @brief The filter of `meta_node`, if any, as it is once its evaluation re-enters
   * the session no more
   *
   * A filter can read what its own write changed (exist_not of the jets:exception it
   * raised), so its answer is the one it gives when it writes nothing new. Throws
   * after kMaxReentryRetries evaluations that each re-entered.
   */
  bool
  current_filter(b_index meta_node, BetaRow const* row);

  /**
   * @brief Whether a candidate to insert into an activated relation is kept (I-91)
   *
   * Its filter's current answer and, when the session was re-entered since `mark`
   * (the count when the caller's scan began), candidate_still_supported. Without a
   * re-entry the cost is one compare. A re-entry after the insert reaches the row,
   * since the relation is activated: a parent row's removal withdraws it, and a
   * filter that reads the graph has triple_updated_for_filters replay the relation.
   */
  bool
  keep_candidate(b_index meta_node, BetaRow const* row, BetaRelation const* parent_relation,
    BetaRow const* parent_row, AlphaNode const* alpha_node, rdf::Triple const* t3, std::uint64_t mark);

 private:
 friend class BetaRelation;

  BetaRowArena            arena_;                  // first: destroyed last, see ~ReteSession
  ReteMetaStorePtr        rule_ms_;
  rdf::RDFSession *       rdf_session_;
  BetaRelationVector      beta_relations_;
  VertexVisitsVector      vertex_visits_;
  BetaRowPriorityQueue    pending_compute_consequent_beta_rows_;
  std::string             err_msg_;
  int                     max_vertex_visit_;
  bool                    max_vertex_visit_is_default_; // the limit is kDefaultMaxVertexVisits, not set
  BetaRow *               recording_row_;          // row whose inferred triples are being recorded
  std::vector<BetaRow *>  scratch_rows_;           // CandidateRow's, in arena_, never destroyed
  std::size_t             scratch_depth_;          // CandidateRows live
  std::vector<BetaRowPtr> held_rows_;              // see HeldCandidates
  std::uint64_t           reentries_;              // callbacks seen, see candidate_still_supported
};

inline int
ReteSession::insert_inferred_for(BetaRow const* beta_row, rdf::Triple const& t3)
{
  auto * sess = this->rdf_session_;
  if(not t3.subject or not t3.predicate or not t3.object) {
    // insert_inferred reports the error
    return sess->insert_inferred(t3);
  }
  // Same test insert_inferred applies: a triple in the meta or asserted graph is not
  // inserted and its reference count is not raised, so there is nothing to retract.
  bool counted = not sess->meta_graph()->contains(t3.subject, t3.predicate, t3.object) and
                 not sess->asserted_graph()->contains(t3.subject, t3.predicate, t3.object);
  int ret = sess->insert_inferred(t3);
  if(counted and beta_row and beta_row == this->recording_row_) {
    this->recording_row_->record_inferred(t3);
  }
  return ret;
}

inline ReteSessionPtr create_rete_session(ReteMetaStorePtr rule_ms, 
  rdf::RDFSession * rdf_session)
{
  return std::make_shared<ReteSession>(rule_ms, rdf_session);
}

// The held rows are moved into the relation, not copied: each copy of a shared_ptr was
// an atomic increment and, when the copy went, a decrement (C16). drop() then releases
// the moved-from pointers, which hold nothing.
inline int
ReteSession::HeldCandidates::insert_into(BetaRelation * relation)
{
  auto & rows = rete_session_->held_rows_;
  for(auto i = base_; i < rows.size(); ++i) {
    auto err = relation->insert_beta_row(rete_session_, std::move(rows[i]));
    if(err) return err;
  }
  drop();
  return 0;
}

// Inline, and the filter evaluated only when the vertex has one (C16): most candidates
// are of vertexes without a filter, and for them this is the compare alone. The same
// answer as before: current_filter is true without a filter.
inline bool
ReteSession::keep_candidate(b_index meta_node, BetaRow const* row, BetaRelation const* parent_relation,
  BetaRow const* parent_row, AlphaNode const* alpha_node, rdf::Triple const* t3, std::uint64_t mark)
{
  if(meta_node->has_expr() and not this->current_filter(meta_node, row)) return false;
  return this->reentries_ == mark or this->candidate_still_supported(parent_relation, parent_row, alpha_node, t3);
}

inline bool
ReteSession::passes_filter(b_index meta_node, BetaRow const* row)
{
  return not meta_node->has_expr() or meta_node->filter_expr->eval_filter(this, row);
}

// //////////////////////////////////////////////////////////////////////////////////////
// BetaRelation methods
// --------------------------------------------------------------------------------------
inline int
BetaRelation::insert_beta_row(ReteSession * rete_session, BetaRowPtr beta_row)
{
  // A relation in an arena is released without being destroyed, so a row it holds
  // must be in the same arena, or the row would never be freed
  if(beta_row->arena() != this->arena_) {
    RETE_EXCEPTION("BetaRelation::insert_beta_row: row and relation are in different arenas");
  }
  // One lookup, which inserts if it finds no equal row (C16; it was a find, then an
  // insert). The set can grow, so its live scans are frozen first, found or not: it
  // costs nothing when none is live. A row taken is moved into the set; a row found
  // equal is left, and the set's element is used.
  this->scans_.freeze(BetaRowScanRegistry::kAllRows);
  auto [itor, is_new] = this->all_beta_rows_.insert(std::move(beta_row));
  if(is_new and this->all_beta_rows_.size() > this->peak_rows_) this->peak_rows_ = this->all_beta_rows_.size();
  // a reference to the set's element: row_inserted does not change the set
  return this->row_inserted(rete_session, *itor, is_new);
}

inline int
BetaRelation::insert_candidate(ReteSession * rete_session, BetaRow const& candidate)
{
  // One lookup, which inserts if it finds no equal row. The set can grow, so its live
  // scans are frozen first, found or not: it costs nothing when none is live
  this->scans_.freeze(BetaRowScanRegistry::kAllRows);
  bool is_new = false;
  auto itor = this->all_beta_rows_.lazy_emplace(candidate, [&](auto const& construct) {
    // Only a row that is kept is allocated
    is_new = true;
    construct(create_beta_row(this->arena_, candidate));
  });
  if(is_new and this->all_beta_rows_.size() > this->peak_rows_) this->peak_rows_ = this->all_beta_rows_.size();
  // a reference to the set's element: row_inserted does not change the set
  return this->row_inserted(rete_session, *itor, is_new);
}

inline int
BetaRelation::row_inserted(ReteSession * rete_session, BetaRowPtr const& beta_row, bool is_new)
{
  if(is_new) {
    // beta_row inserted in set
    VLOG(56)<<"BetaRow insert row "<<beta_row;
    // schedule the consequent terms
    if(beta_row->get_node_vertex()->has_consequent_terms()) {
      // Flag row as new and pending to infer triples
      beta_row->set_status(BetaRowStatus::kInserted);
      rete_session->schedule_consequent_terms(beta_row);
    } else {

      // Mark row as done
      beta_row->set_status(BetaRowStatus::kProcessed);
    }

  } else {
    VLOG(56)<<"BetaRow insert (again) row "<<beta_row;
    // Check if row was maked as deleted
    if(beta_row->is_deleted()) {
      // Mark it as processed so it does not get retracted
      beta_row->set_status(BetaRowStatus::kProcessed);
    } else {
      // VLOG(50)<<"    Insert Beta Row at vertex "<<
      //   this->get_node_vertex()->vertex<<" ALREADY INSERTED, row "<<beta_row<<
      //   ", status is "<< beta_row->get_status()<<" skipping. . .";
      return 0;
    }    
  }

  if(this->get_node_vertex()->child_nodes.empty()) return 0;

  // Add row to pending queue to notify child nodes
  this->pending_beta_rows_.push_back(beta_row);
  // Add/Restore row indexes
  if(this->node_vertex_->is_head_vertice()) return 0;
  this->scans_.freeze(BetaRowScanRegistry::kIndexRows);
  for(auto const& child_node_vertex: node_vertex_->child_nodes) {
    auto alpha_node = rete_session->rule_ms()->get_alpha_node(child_node_vertex->vertex);
    alpha_node->index_beta_row(this, child_node_vertex, beta_row.get());
  }
  return 0;
}

inline int
BetaRelation::remove_beta_row(ReteSession * rete_session, BetaRowPtr beta_row)
{
  return this->row_removed(rete_session, this->all_beta_rows_.find(beta_row));
}

inline int
BetaRelation::remove_candidate(ReteSession * rete_session, BetaRow const& candidate)
{
  return this->row_removed(rete_session, this->all_beta_rows_.find(candidate));
}

inline int
BetaRelation::row_removed(ReteSession * rete_session, beta_row_set::const_iterator itor)
{
  if(itor==this->all_beta_rows_.end()) {
    // Already deleted!
    // VLOG(50)<<"    BetaRowPtr not found, must be already deleted.(D01)";
    return 0;
  }
  // make sure we point to the right instance
  BetaRowPtr beta_row = *itor;
  VLOG(56)<<"BetaRow remove row "<<beta_row;
  
  // Check if row was already deleted
  if(beta_row->is_deleted()) {
    // VLOG(50)<<"    Marked as deleted already";
    return 0;
  }

  // Check for consequent terms
  if(beta_row->get_node_vertex()->has_consequent_terms()) {
    // Check if status kInserted
    if(beta_row->is_inserted()) {
      // Row was marked kInserted, not inferred yet
      // Cancel row insertion **
      // VLOG(50)<<"    Row marked kInserted, not inferred yet ** Cancel row insertion **";
      beta_row->set_status(BetaRowStatus::kProcessed);

      // Put the row in the pending queue to notify children that this row is being deleted
      if(not this->get_node_vertex()->child_nodes.empty()) {
        this->pending_beta_rows_.push_back(beta_row);
        this->remove_indexes(rete_session, beta_row);
      }

      this->scans_.freeze(BetaRowScanRegistry::kAllRows);
      this->all_beta_rows_.erase(beta_row);
      return 0;
    }

    // VLOG(50)<<"    Row marked kProcessed, need to put it for delete/retract";
    // Row must be in kProcessed state -- need to put it for delete/retract
    beta_row->set_status(BetaRowStatus::kDeleted);

    // Put the row in the pending queue to notify children that this row is being deleted
    if(not this->get_node_vertex()->child_nodes.empty()) {
      this->pending_beta_rows_.push_back(beta_row);
      this->remove_indexes(rete_session, beta_row);
    }

    rete_session->schedule_consequent_terms(beta_row);

  } else {
    // No consequent terms, remove and propagate down
    beta_row->set_status(BetaRowStatus::kProcessed);

    // Put the row in the pending queue to notify children that this row is being deleted
    if(not this->get_node_vertex()->child_nodes.empty()) {
      this->pending_beta_rows_.push_back(beta_row);
      this->remove_indexes(rete_session, beta_row);
    }

    this->scans_.freeze(BetaRowScanRegistry::kAllRows);
    this->all_beta_rows_.erase(beta_row);
  }
  return 0;
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
  inline int
  BetaRelation::initialize(ReteSession * rete_session)
  {
    scans_.freeze(BetaRowScanRegistry::kIndexRows);
    beta_row_idx1_.clear();
    beta_row_idx2_.clear();
    beta_row_idx3_.clear();
    for(auto const& child_node_vertex: this->node_vertex_->child_nodes) {
      auto child_alpha_node = rete_session->rule_ms()->get_alpha_node(child_node_vertex->vertex);
      child_alpha_node->initialize_indexes(this, child_node_vertex);
    }
    return 0;
  }

  inline void
  BetaRelation::remove_indexes(ReteSession * rete_session, BetaRowPtr beta_row)
  {
    // remove the indexes associated with the beta row
    this->scans_.freeze(BetaRowScanRegistry::kIndexRows);
    for(auto const& child_node_vertex: this->node_vertex_->child_nodes) {
      auto alpha_node = rete_session->rule_ms()->get_alpha_node(child_node_vertex->vertex);
      alpha_node->remove_index_beta_row(this, child_node_vertex, beta_row.get());
    }
  }

// Declaired in graph_callback_mgr_impl.h
inline void
ReteCallBackImpl::triple_inserted(rdf::r_index s, rdf::r_index p, rdf::r_index o)const
{
  if(this->s_filter_ and this->s_filter_!=s) return;
  if(this->p_filter_ and this->p_filter_!=p) return;
  if(this->o_filter_ and this->o_filter_!=o) return;
  // If beta node is not activated yet, ignore the notification, but count it: a
  // filter's write matching the antecedent being visited for the first time removes
  // a candidate's support as surely (see candidate_still_supported)
  if(not this->rete_session_->get_beta_relation(this->vertex_)->is_activated()) {
    this->rete_session_->count_ignored_reentry();
    return;
  }
  this->rete_session_->triple_inserted(this->vertex_, s, p, o);
}
// Declaired in graph_callback_mgr_impl.h
inline void
ReteCallBackImpl::triple_deleted(rdf::r_index s, rdf::r_index p, rdf::r_index o)const
{
  if(this->s_filter_!=nullptr and this->s_filter_!=s) return;
  if(this->p_filter_!=nullptr and this->p_filter_!=p) return;
  if(this->o_filter_!=nullptr and this->o_filter_!=o) return;
  // If beta node is not activated yet, ignore the notification, but count it: a
  // filter's write matching the antecedent being visited for the first time removes
  // a candidate's support as surely (see candidate_still_supported)
  if(not this->rete_session_->get_beta_relation(this->vertex_)->is_activated()) {
    this->rete_session_->count_ignored_reentry();
    return;
  }
  this->rete_session_->triple_deleted(this->vertex_, s, p, o);
}


// from graph_callback_mgr_impl.h
inline void
ReteCallBack4VisitorsImpl::triple_inserted(rdf::r_index s, rdf::r_index p, rdf::r_index o)const
{
  if(this->p_filter_ and this->p_filter_!=p) return;
  // If beta node is not activated yet, ignore the notification, but count it: a
  // filter's write matching the antecedent being visited for the first time removes
  // a candidate's support as surely (see candidate_still_supported)
  if(not this->rete_session_->get_beta_relation(this->vertex_)->is_activated()) {
    this->rete_session_->count_ignored_reentry();
    return;
  }
  this->rete_session_->triple_inserted_for_filters(this->vertex_, s, p, o);
}
// from graph_callback_mgr_impl.h
inline void
ReteCallBack4VisitorsImpl::triple_deleted(rdf::r_index s, rdf::r_index p, rdf::r_index o)const
{
  if(this->p_filter_!=nullptr and this->p_filter_!=p) return;
  // If beta node is not activated yet, ignore the notification, but count it: a
  // filter's write matching the antecedent being visited for the first time removes
  // a candidate's support as surely (see candidate_still_supported)
  if(not this->rete_session_->get_beta_relation(this->vertex_)->is_activated()) {
    this->rete_session_->count_ignored_reentry();
    return;
  }
  this->rete_session_->triple_deleted_for_filters(this->vertex_, s, p, o);
}

} // namespace jets::rete
#endif // JETS_RETE_RETE_SESSION_H
