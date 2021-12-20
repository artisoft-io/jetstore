#ifndef JETS_RETE_BETA_RELATION_H
#define JETS_RETE_BETA_RELATION_H

#include <string>
#include <memory>
#include <list>

#include "absl/hash/hash.h"
#include "absl/container/flat_hash_set.h"

#include "jets/rete/node_vertex.h"
#include "jets/rete/beta_row_initializer.h"
#include "jets/rete/beta_row.h"
#include "jets/rete/beta_row_iterator.h"

// Component to manage all the rdf resources and literals of a graph
namespace jets::rete {
// //////////////////////////////////////////////////////////////////////////////////////
// BetaRelation class -- main class for the rete network
// --------------------------------------------------------------------------------------
class BetaRelation;
using BetaRelationPtr = std::shared_ptr<BetaRelation>;

// container for holding all beta_rows
using beta_row_set = absl::flat_hash_set<BetaRowPtr>;

// Compute the hash of BetaRow
template <typename H>
H AbslHashValue(H h, BetaRowPtr const& s) {
  if(s->get_size() == 0) return h;
  auto itor = s->begin();
  auto end = s->end();
  for(; itor !=end; itor++) {
      h = H::combine(std::move(h), *itor);
  }
  return h;
}

inline bool 
operator==(BetaRowPtr const& lhs, BetaRowPtr const& rhs) 
{ 
  auto sz = lhs->get_size();
  if(sz != rhs->get_size()) return false;

  for(int i=0; i<sz; i++) {
    if(lhs->get(i) != rhs->get(i)) return false;
  }
  return true; 
}

// queue of new beta_row for descendent nodes
using beta_row_list = std::list<BetaRowPtr>;

// BetaRelation making the rete network
class BetaRelation {
 public:
  BetaRelation()
    : node_vertex_(nullptr),
      is_activated_(false),
      all_beta_rows_(),
      pending_beta_rows_()
    {}

  explicit BetaRelation(b_index node_vertex) 
    : node_vertex_(node_vertex),
      is_activated_(false),
      all_beta_rows_(),
      pending_beta_rows_()
    {}

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
    pending_beta_rows_.clear();
  }

 protected:

 private:
  // friend class find_visitor<RDFGraph>;
  // friend class RDFSession<RDFGraph>;

  b_index         node_vertex_;
  bool            is_activated_;
  beta_row_set    all_beta_rows_;
  beta_row_list   pending_beta_rows_;
};

inline BetaRelationPtr create_beta_node(b_index node_vertex)
{
  return std::make_shared<BetaRelation>(node_vertex);
}

} // namespace jets::rete
#endif // JETS_RETE_BETA_RELATION_H
