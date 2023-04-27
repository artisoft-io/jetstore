#ifndef JETS_RETE_BETA_ROW_ITERATOR_H
#define JETS_RETE_BETA_ROW_ITERATOR_H

#include <string>
#include <memory>
#include <utility>
#include <list>
#include <tuple>
#include <unordered_map>
#include <vector>

#include "absl/hash/hash.h"
#include "absl/container/flat_hash_set.h"

#include "../rete/beta_row.h"

namespace jets::rete {
// Declaration of Container Types for BetaRelation Class
/////////////////////////////////////////////////////////////////////////////////////////
// Container for indexes on BetaRow
using BetaRowIndxKey2 = std::tuple<rdf::r_index, rdf::r_index>;
using BetaRowIndxKey3 = std::tuple<rdf::r_index, rdf::r_index, rdf::r_index>;

using BetaRowIndexes1 = std::unordered_multimap<rdf::r_index, BetaRow const*, absl::Hash<rdf::r_index>>;
using BetaRowIndexes2 = std::unordered_multimap<BetaRowIndxKey2, BetaRow const*, absl::Hash<BetaRowIndxKey2> >;
using BetaRowIndexes3 = std::unordered_multimap<BetaRowIndxKey3, BetaRow const*, absl::Hash<BetaRowIndxKey3> >;
// Container for sets of indexes on BetaRow
// Parent beta node have multiple child node, each having an antecedent term pointing to the parent node.
// We can have multiple indexes of each type. Therefore we need to have a vector of indexes, keyed by position, for each type.
using BetaRowIndxVec1 = std::vector<BetaRowIndexes1>;
using BetaRowIndxVec2 = std::vector<BetaRowIndexes2>;
using BetaRowIndxVec3 = std::vector<BetaRowIndexes3>;

// queue of new beta_row for descendent nodes
using beta_row_list = std::list<BetaRowPtr>;
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

// Provide specific classes
// --------------------------------------------------------------------------------------
template<class I>
class BaseIterator: public BetaRowIterator {
 public:
  using Iterator = I;

  inline 
  BaseIterator(Iterator begin, Iterator end)
    : BetaRowIterator(),
      itor_(begin),
      end_(end)
  {
  }

  virtual ~BaseIterator() {}

  // No need for those, will use shared_ptr
  BaseIterator(BaseIterator const& rhs) = delete;
  BaseIterator(BaseIterator && rhs) = delete;
  BaseIterator& operator=(BaseIterator const& rhs) = delete;

  /**
   * Test if the iterator has exhausted all items in his collection.
   *
   * @return true if the iterator has no more items to return
   */
  bool is_end()const override
  {
    return itor_ == end_;
  }

  /**
   * Advance the iterator to the next item
   *
   * @return false if the iterator has no more item to return
   */
  bool next() override
  {
    if(is_end()) return false;
    ++itor_;
    return not is_end();
  }

  BetaRow const* get_row() const override
  {
    if(is_end()) return nullptr;
    return itor_->second;
  } 

 private:
  Iterator itor_;
  Iterator end_;
};

// Case Using container with std::shared_ptr<T>
// as it is the case for the BetaRowSet
template<class I>
class PtrAdaptIterator: public BetaRowIterator {
 public:
  using Iterator = I;

  inline 
  PtrAdaptIterator(Iterator begin, Iterator end)
    : BetaRowIterator(),
      itor_(begin),
      end_(end)
  {
  }

  virtual ~PtrAdaptIterator() {}

  // No need for those, will use shared_ptr
  PtrAdaptIterator(PtrAdaptIterator const& rhs) = delete;
  PtrAdaptIterator(PtrAdaptIterator && rhs) = delete;
  PtrAdaptIterator& operator=(PtrAdaptIterator const& rhs) = delete;

  /**
   * Test if the iterator has exhausted all items in his collection.
   *
   * @return true if the iterator has no more items to return
   */
  bool is_end()const override
  {
    return itor_ == end_;
  }

  /**
   * Advance the iterator to the next item
   *
   * @return false if the iterator has no more item to return
   */
  bool next() override
  {
    if(is_end()) return false;
    ++itor_;
    return not is_end();
  }

  BetaRow const* get_row() const override
  {
    if(is_end()) return nullptr;
    return (*itor_).get();    // exposing the raw ptr from the shared_ptr
  } 

 private:
  Iterator itor_;
  Iterator end_;
};

inline BetaRowIteratorPtr 
create_all_rows_iterator(beta_row_set::const_iterator begin, beta_row_set::const_iterator end)
{
  return std::make_shared<PtrAdaptIterator<beta_row_set::const_iterator>>(begin, end);
}

inline BetaRowIteratorPtr 
create_pending_rows_iterator(beta_row_list::const_iterator begin, beta_row_list::const_iterator end)
{
  return std::make_shared<PtrAdaptIterator<beta_row_list::const_iterator>>(begin, end);
}

inline BetaRowIteratorPtr 
create_idx1_rows_iterator(BetaRowIndexes1::const_iterator begin, BetaRowIndexes1::const_iterator end)
{
  return std::make_shared<BaseIterator<BetaRowIndexes1::const_iterator>>(begin, end);
}

inline BetaRowIteratorPtr 
create_idx2_rows_iterator(BetaRowIndexes2::const_iterator begin, BetaRowIndexes2::const_iterator end)
{
  return std::make_shared<BaseIterator<BetaRowIndexes2::const_iterator>>(begin, end);
}

inline BetaRowIteratorPtr 
create_idx3_rows_iterator(BetaRowIndexes3::const_iterator begin, BetaRowIndexes3::const_iterator end)
{
  return std::make_shared<BaseIterator<BetaRowIndexes3::const_iterator>>(begin, end);
}

// Alternate iterator, returning all BetaRowPtr
class BetaRowPtrIterator;
using BetaRowPtrIteratorPtr = std::shared_ptr<BetaRowPtrIterator>;

class BetaRowPtrIterator {
 public:
  using Iterator = beta_row_set::const_iterator;

  inline 
  BetaRowPtrIterator(Iterator begin, Iterator end)
    : itor_(begin),
      end_(end)
  {
  }

  virtual ~BetaRowPtrIterator() {}

  // No need for those, will use shared_ptr
  BetaRowPtrIterator(BetaRowPtrIterator const& rhs) = delete;
  BetaRowPtrIterator(BetaRowPtrIterator && rhs) = delete;
  BetaRowPtrIterator& operator=(BetaRowPtrIterator const& rhs) = delete;

  /**
   * Test if the iterator has exhausted all items in his collection.
   *
   * @return true if the iterator has no more items to return
   */
  bool is_end()const
  {
    return itor_ == end_;
  }

  /**
   * Advance the iterator to the next item
   *
   * @return false if the iterator has no more item to return
   */
  bool next()
  {
    if(is_end()) return false;
    ++itor_;
    return not is_end();
  }

  BetaRowPtr get_row_ptr() const
  {
    if(is_end()) return nullptr;
    return *itor_;
  } 

 private:
  Iterator itor_;
  Iterator end_;
};

inline BetaRowPtrIteratorPtr 
create_all_rows_ptr_iterator(beta_row_set::const_iterator begin, beta_row_set::const_iterator end)
{
  return std::make_shared<BetaRowPtrIterator>(begin, end);
}

}  // namespace jets::rete
#endif  // JETS_RETE_BETA_ROW_ITERATOR_H
