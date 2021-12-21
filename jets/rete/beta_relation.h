#ifndef JETS_RETE_BETA_RELATION_H
#define JETS_RETE_BETA_RELATION_H

#include <string>
#include <memory>
#include <utility>
#include <list>
#include <tuple>
#include <unordered_map>
#include <vector>

#include "jets/rdf/rdf_types.h"
#include "jets/rete/node_vertex.h"
#include "jets/rete/beta_row_initializer.h"
#include "jets/rete/beta_row.h"
#include "jets/rete/beta_row_iterator.h"

// Component to manage all the rdf resources and literals of a graph
namespace jets::rete {
// //////////////////////////////////////////////////////////////////////////////////////
// BetaRelation class -- main class for the rete network
// --------------------------------------------------------------------------------------
// Forward declaration
template<class T>
class AlphaNode;

class BetaRelation;
using BetaRelationPtr = std::shared_ptr<BetaRelation>;

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
      beta_row_idx1(),
      beta_row_idx2(),
      beta_row_idx3()
    {}

  explicit BetaRelation(b_index node_vertex) 
    : node_vertex_(node_vertex),
      is_activated_(false),
      all_beta_rows_(),
      pending_beta_rows_(),
      beta_row_idx1(),
      beta_row_idx2(),
      beta_row_idx3()
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
  template<class W> friend class AlphaNode;

  b_index         node_vertex_;
  bool            is_activated_;
  beta_row_set    all_beta_rows_;
  beta_row_list   pending_beta_rows_;
  BetaRowIndxVec1 beta_row_idx1;
  BetaRowIndxVec2 beta_row_idx2;
  BetaRowIndxVec3 beta_row_idx3;
};

inline BetaRelationPtr create_beta_node(b_index node_vertex)
{
  return std::make_shared<BetaRelation>(node_vertex);
}

} // namespace jets::rete
#endif // JETS_RETE_BETA_RELATION_H
