#ifndef JETS_RETE_BETA_RELATION_H
#define JETS_RETE_BETA_RELATION_H

#include <string>
#include <memory>
#include <utility>
#include <list>
#include <tuple>
#include <unordered_map>
#include <vector>

#include "expr.h"
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
class AlphaNode;
class ReteSession;

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
      beta_row_idx1_(),
      beta_row_idx2_(),
      beta_row_idx3_()
    {}

  explicit BetaRelation(b_index node_vertex) 
    : node_vertex_(node_vertex),
      is_activated_(false),
      all_beta_rows_(),
      pending_beta_rows_(),
      beta_row_idx1_(),
      beta_row_idx2_(),
      beta_row_idx3_()
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

  inline BetaRowIteratorPtr
  get_all_rows_iterator()const
  {
    return create_all_rows_iterator(all_beta_rows_.begin(), all_beta_rows_.end());
  }

  inline BetaRowIteratorPtr
  get_pending_rows_iterator()const
  {
    return create_pending_rows_iterator(pending_beta_rows_.begin(), pending_beta_rows_.end());
  }

/**
 * @brief Insert BetaRow into BetaRelation and transfer ownsership of the row as well.
 * 
 * @tparam T RDFSession used
 * @param rete_session Current session that owns the BetaRelation
 * @param beta_row the row to insert
 * @return int 0 if normal, -1 if error
 */
  // Defined in rete_session.h
  int
  insert_beta_row(ReteSession * rete_session, BetaRowPtr beta_row);

  // Defined in rete_session.h
  /**
   * @brief Remove `beta_row` from beta relation if fount in beta relation
   * 
   * @param rete_session 
   * @param beta_row BetaRow to remove
   * @return int 
   */
  int
  remove_beta_row(ReteSession * rete_session, BetaRowPtr beta_row);

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
    auto result = this->beta_row_idx1_[key].equal_range( u ); 
    return create_idx1_rows_iterator(result.first, result.second);
  }

  inline BetaRowIteratorPtr
  get_idx2_rows_iterator(int key, rdf::r_index u, rdf::r_index v)const
  {
    auto result = this->beta_row_idx2_[key].equal_range( {u, v} ); 
    return create_idx2_rows_iterator(result.first, result.second);
  }

  inline BetaRowIteratorPtr
  get_idx3_rows_iterator(int key, rdf::r_index u, rdf::r_index v, rdf::r_index w)const
  {
    auto result = this->beta_row_idx3_[key].equal_range( {u, v, w} ); 
    return create_idx3_rows_iterator(result.first, result.second);
  }


 protected:
  int
  add_indexes(BetaRowPtr & beta_row)
  {
    for(auto const& b_index: node_vertex_->child_nodes) {
      AntecedentQuerySpecPtr const& query_spec = b_index->antecedent_query_spec;
      switch (query_spec->type) {
      case AntecedentQueryType::kQTu: 
        // idx_mm is a multimap r_index, beta_row*
        beta_row_idx1_[query_spec->key].insert(
          {beta_row->get(query_spec->u_pos), beta_row.get()}
        ); 
        break;
      case AntecedentQueryType::kQTuv:
        // idx_mm is a multimap pair<r_index,r_index>, beta_row*
        beta_row_idx2_[query_spec->key].insert(
          {{beta_row->get(query_spec->u_pos), 
            beta_row->get(query_spec->v_pos)}, beta_row.get()}
        ); 
        break;
      case AntecedentQueryType::kQTuvw:
        // idx_mm is a multimap tuple<r_index,r_index, r_index>, beta_row*
        beta_row_idx3_[query_spec->key].insert(
          {{beta_row->get(query_spec->u_pos), 
            beta_row->get(query_spec->v_pos), 
            beta_row->get(query_spec->w_pos)}, beta_row.get()}
        ); 
        break;
      case AntecedentQueryType::kQTAll:
      break;
      }
    }
    return 0;
  }

  int
  remove_indexes(BetaRowPtr & beta_row)
  {
    for(auto const& b_index: node_vertex_->child_nodes) {
      AntecedentQuerySpecPtr const& query_spec = b_index->antecedent_query_spec;
      switch (query_spec->type) {
      case AntecedentQueryType::kQTu: 
        // idx_mm is a multimap r_index, beta_row*
        beta_row_idx1_[query_spec->key].erase(
          {beta_row->get(query_spec->u_pos)}
        ); 
        break;
      case AntecedentQueryType::kQTuv:
        // idx_mm is a multimap pair<r_index,r_index>, beta_row*
        beta_row_idx2_[query_spec->key].erase(
          {beta_row->get(query_spec->u_pos), 
            beta_row->get(query_spec->v_pos)}
        ); 
        break;
      case AntecedentQueryType::kQTuvw:
        // idx_mm is a multimap tuple<r_index,r_index, r_index>, beta_row*
        beta_row_idx3_[query_spec->key].erase(
          {beta_row->get(query_spec->u_pos), 
            beta_row->get(query_spec->v_pos), 
            beta_row->get(query_spec->w_pos)}
        ); 
        break;
      case AntecedentQueryType::kQTAll:
      break;
      }
    }
    return 0;
  }


 private:
  friend class AlphaNode;

  b_index         node_vertex_;
  bool            is_activated_;
  beta_row_set    all_beta_rows_;
  beta_row_list   pending_beta_rows_;
  BetaRowIndxVec1 beta_row_idx1_;
  BetaRowIndxVec2 beta_row_idx2_;
  BetaRowIndxVec3 beta_row_idx3_;
};

inline BetaRelationPtr 
create_beta_node(b_index node_vertex)
{
  return std::make_shared<BetaRelation>(node_vertex);
}

} // namespace jets::rete
#endif // JETS_RETE_BETA_RELATION_H
