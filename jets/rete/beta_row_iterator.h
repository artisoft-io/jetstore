#ifndef JETS_RETE_BETA_ROW_ITERATOR_H
#define JETS_RETE_BETA_ROW_ITERATOR_H

#include <string>
#include <memory>
#include <utility>

#include "jets/rete/beta_row.h"

namespace jets::rete {

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

  inline BetaRowIterator() {}
  virtual ~BetaRowIterator() {}

  // provide implementation for specific classes
  // BetaRowIterator(BetaRowIterator const& rhs) = delete;
  // BetaRowIterator(BetaRowIterator && rhs) = delete;
  // BetaRowIterator& operator=(BetaRowIterator const& rhs) = delete;

  /**
   * Test if the iterator has exhausted all triples in his collection.
   *
   * @return true if the iterator has no more triples to return
   */
  virtual bool is_end()const=0;

  /**
   * Advance the iterator to the next triple
   *
   * @return false if the iterator has no more triples to return
   */
  virtual bool next()=0;

  virtual BetaRowPtr get_row() const=0; 
};

}  // namespace jets::rete
#endif  // JETS_RETE_BETA_ROW_ITERATOR_H
