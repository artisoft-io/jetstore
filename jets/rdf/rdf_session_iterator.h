#ifndef JETS_RDF_SESSION_ITERATOR_H
#define JETS_RDF_SESSION_ITERATOR_H

#include <string>
#include <utility>

#include "absl/hash/hash.h"
#include "jets/rdf/rdf_err.h"
#include "jets/rdf/base_graph.h"
#include "jets/rdf/rdf_ast.h"
#include "jets/rdf/w_node.h"

namespace jets::rdf {

/////////////////////////////////////////////////////////////////////////////////////////
// class RDFSessionIterator
//
/////////////////////////////////////////////////////////////////////////////////////////
/**
 * The unified iterator over the triple graph structured as:
 *
 * 	map(u, map(v, set(w))), where the triple (u, v, w) can either be:
 * 		- (subject, predicate, object), known as spo
 * 		- (predicate, object, subject), known as pos
 * 		- (object, subject, predicate), known as osp
 * 
 * Example:
 * \code {.cpp}
 * while(not itor.is_end()) {
 *   auto s = itor.get_subject();
 *   std::cout<< get_name(s) << std::endl;
 *   itor.next();
 * }  
 * \endcode
 */
template <class Iterator>
struct RDFSessionIterator {

  inline RDFSessionIterator(Iterator const& meta_itor, Iterator const& asserted_itor, Iterator const& inferred_itor)
      : meta_itor_(meta_itor),
        asserted_itor_(asserted_itor),
        inferred_itor_(inferred_itor) 
      {}

  inline RDFSessionIterator(Iterator && meta_itor, Iterator && asserted_itor, Iterator && inferred_itor)
      : meta_itor_    (std::forward<Iterator>(meta_itor)),
        asserted_itor_(std::forward<Iterator>(asserted_itor)),
        inferred_itor_(std::forward<Iterator>(inferred_itor)) 
      {}

  RDFSessionIterator() = delete;

  inline RDFSessionIterator(RDFSessionIterator const& rhs) = default;
  inline RDFSessionIterator(RDFSessionIterator && rhs) = default;

  inline RDFSessionIterator& operator=(RDFSessionIterator const& rhs) = default;

  /**
   * Test if the iterator has exhausted all triples in his collection.
   *
   * @return true if the iterator has no more triples to return
   */
  inline bool is_end() const 
  {
    return meta_itor_.is_end() and asserted_itor_.is_end() and inferred_itor_.is_end();
  }

  /**
   * Advance the iterator to the next triple
   *
   * @return false if the iterator has no more triples to return
   */
  inline bool next() 
  {
    if(meta_itor_.is_end()) {
      if(asserted_itor_.is_end()) {
        return inferred_itor_.next();
      } else {
        return asserted_itor_.next();
      }
    } else {
      return meta_itor_.next(); 
    }
  }

  inline r_index get_subject() const 
  {
    if(meta_itor_.is_end()) {
      if(asserted_itor_.is_end()) {
        return inferred_itor_.get_subject();
      } else {
        return asserted_itor_.get_subject();
      }
    } else {
      return meta_itor_.get_subject(); 
    }
  }

  inline r_index get_predicate() const 
  {
    if(meta_itor_.is_end()) {
      if(asserted_itor_.is_end()) {
        return inferred_itor_.get_predicate();
      } else {
        return asserted_itor_.get_predicate();
      }
    } else {
      return meta_itor_.get_predicate(); 
    }
  }

  inline r_index get_object() const 
  {
    if(meta_itor_.is_end()) {
      if(asserted_itor_.is_end()) {
        return inferred_itor_.get_object();
      } else {
        return asserted_itor_.get_object();
      }
    } else {
      return meta_itor_.get_object();
    }
  }

  inline r_index * get_triple(r_index * t3) const 
  {    
    if(meta_itor_.is_end()) {
      if(asserted_itor_.is_end()) {
        t3[0] = inferred_itor_.get_object(); 
        t3[1] = inferred_itor_.get_predicate(); 
        t3[2] = inferred_itor_.get_object(); 
      } else {
        t3[0] = asserted_itor_.get_object(); 
        t3[1] = asserted_itor_.get_predicate(); 
        t3[2] = asserted_itor_.get_object(); 
      }
    } else {
      t3[0] = meta_itor_.get_object(); 
      t3[1] = meta_itor_.get_predicate(); 
      t3[2] = meta_itor_.get_object(); 
    }
    return t3;
  }

  inline Triple as_triple() const 
  {    
    if(meta_itor_.is_end()) {
      if(asserted_itor_.is_end()) {
        return Triple(inferred_itor_.get_object(), 
          inferred_itor_.get_predicate(), 
          inferred_itor_.get_object());
      } else {
        return Triple(asserted_itor_.get_object(), 
          asserted_itor_.get_predicate(), 
          asserted_itor_.get_object());
      }
    } else {
        return Triple(meta_itor_.get_object(), 
          meta_itor_.get_predicate(), 
          meta_itor_.get_object());
    }
  }

 private:
  Iterator meta_itor_;
  Iterator asserted_itor_;
  Iterator inferred_itor_;
};

}  // namespace jets::rdf
#endif  // JETS_RDF_SESSION_ITERATOR_H
