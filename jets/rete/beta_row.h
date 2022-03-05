#ifndef JETS_RETE_BETA_ROW_H
#define JETS_RETE_BETA_ROW_H

#include <string>
#include <memory>
#include <ostream>
#include <unordered_set>

#include "absl/hash/hash.h"

#include "../rdf/rdf_types.h"
#include "../rete/rete_err.h"
#include "../rete/beta_row_initializer.h"
#include "../rete/node_vertex.h"


// Component to manage all the rdf resources and literals of a graph
namespace jets::rete {
// //////////////////////////////////////////////////////////////////////////////////////
// BetaRow class is a row in the BetaRelation table
// --------------------------------------------------------------------------------------
class BetaRow;
using BetaRowPtr = std::shared_ptr<BetaRow>;

enum BetaRowStatus {
  kNone = 0,
  kInserted = 1,
  kDeleted = 2,
  kProcessed = 3,
};

// BetaRow is a row in the BetaRelation table
class BetaRow {
 public:
  using const_iterator = rdf::r_index const*;

  BetaRow() : data_(nullptr), size_(0), node_vertex_(nullptr) {}
  BetaRow(b_index node_vertex, int size) 
    : status_(kNone),
      data_(nullptr),
      size_(size),
      node_vertex_(node_vertex)
  {
    if(size_ > 0) data_ = new rdf::r_index[size_ + 1]; // +1 for end()
  }

  virtual inline
  ~BetaRow()
  {
    if(data_) delete [] data_;
  }

  BetaRow(BetaRow const&) = delete;
  BetaRow & operator=(BetaRow const&) = delete;

  inline int
  get_size()const
  {
    return size_;
  }

  inline void
  set_status(BetaRowStatus status);

  inline BetaRowStatus
  get_status()const
  {
    return status_;
  }

  inline bool
  is_deleted()const
  {
    return status_ == kDeleted;
  }

  inline bool
  is_inserted()const
  {
    return status_ == kInserted;
  }

  inline bool
  is_processed()const
  {
    return status_ == kProcessed;
  }

  inline b_index
  get_node_vertex()const
  {
    return node_vertex_;
  }

  // Method to initialize the row with a BetaRowInitializer
  int
  initialize(BetaRowInitializer const* initializer, BetaRow const* parent_node, rdf::Triple const* triple)
  {
    int pos = 0;
    auto itor = initializer->begin();
    auto end = initializer->end();
    rdf::r_index value;
    for(; itor != end; itor++) {
      int index = *itor;
      if(index & brc_parent_node) {
        value = parent_node->get(index & brc_low_mask);
      } else {
        value = triple->get(index & brc_low_mask);
      }
      if(not value or this->put(pos, value)<0) {
        LOG(ERROR) << "BetaRow::initialize: invalid index to lookup r_index";
        return -1;
      }
      pos++;
    }
    return 0;
  }

  // Method to initialize data_ 
  // return -1 if called with invalid pos
  inline int 
  put(int pos, rdf::r_index val)
  {
    if(pos < 0 or pos >= size_) return -1;
    data_[pos] = val;
    return 0;
  }

  // Method to get data_[pos] 
  // return -1 if called with invalid pos
  inline rdf::r_index
  get(int pos)const
  {
    if(pos < 0 or pos >= size_) return nullptr;
    return data_[pos];
  }

  // const_iterator used to initialize BetaRow upon row creation
  inline const_iterator
  begin()const
  {
    if(data_) return &data_[0];
    return nullptr;
  }

  inline const_iterator
  end()const
  {
    if(data_) return &data_[size_];
    return nullptr;
  }


inline bool 
operator==(BetaRow const& rhs)const
{ 
  VLOG(1)<<"** operator BetaRow == BetaRow called";
  auto sz = this->get_size();
  if(sz != rhs.get_size()) return false;

  for(int i=0; i<sz; i++) {
    if(this->get(i) != rhs.get(i)) return false;
  }
  return true; 
}

 protected:

 private:
  // To track when rows get inferred and then retracted
  BetaRowStatus   status_;
  rdf::r_index *  data_;
  int             size_;
  b_index         node_vertex_;
};

inline BetaRowPtr create_beta_row(b_index node_vertex, int size)
{
  return std::make_shared<BetaRow>(node_vertex, size);
}

inline std::ostream & operator<<(std::ostream & out, BetaRow const& row)
{
  if(row.get_node_vertex() and row.get_node_vertex()->get_beta_row_initializer()) {
    auto ri = row.get_node_vertex()->get_beta_row_initializer();
    out << "[";
    for(int i=0; i<row.get_size(); i++) {
      if(i > 0) out << ", ";
      out << ri->get_label(i);
    }
    out << "]";
  }
  out << "(";
  for(int i=0; i<row.get_size(); i++) {
    if(i > 0) out << ", ";
    out << row.get(i);
  }
  out << ")";
  return out;
}

inline std::ostream & operator<<(std::ostream & out, BetaRow * row)
{
  if(not row) out << "NULL";
  else {
    out << *row;
  }
  return out;
}

inline std::ostream & operator<<(std::ostream & out, BetaRow const* row)
{
  if(not row) out << "NULL";
  else {
    out << *row;
  }
  return out;
}

inline std::ostream & operator<<(std::ostream & out, BetaRowPtr const& r)
{
  out << r.get();
  return out;
}

// ======================================================================================
// Making BetaRow Hashable
// --------------------------------------------------------------------------------------
inline void
BetaRow::set_status(BetaRowStatus status)
{
  status_ = status;
}

// Compute the hash and equality of BetaRow
template <typename H>
H AbslHashValue(H h, BetaRow * s) {
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
  if(sz != rhs->get_size()) {
    return false;
  }

  for(int i=0; i<sz; i++) {
    if(lhs->get(i) != rhs->get(i)) {
      return false;
    }
  }
  return true; 
}

// using beta_row_set = absl::flat_hash_set<BetaRowPtr, absl::Hash<BetaRowPtr>>;
using beta_row_set = std::unordered_set<BetaRowPtr, absl::Hash<BetaRowPtr>>;


} // namespace jets::rete
#endif // JETS_RETE_BETA_ROW_H
