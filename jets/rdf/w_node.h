#ifndef JETS_RDF_W_NODE_H
#define JETS_RDF_W_NODE_H

#include <cstdint>
#include <utility>

#include "absl/hash/hash.h"

#include "../rdf/rdf_ast.h"

namespace jets::rdf {

/**
 * @brief Class WNode to wrap `r_index` with mechanism to keep track of reference count to it.
 *
 * This structure is used in the `BaseGraph` graph. Keeping track on
 * the reference count is used to support backtracking (truth maintenance) in
 * inference engine.
 *
 * The reference count is a mutable member to ensure we can increase it's
 * count while it is in a collection as a const object.
 *
 * The reference count is not used in the calculation of the hash of the
 * individuals of this structure.
 */
class WNode {
 public:

  WNode() : index_(nullptr), ref_count_(0){};

  WNode(r_index w_index, int count = 1)
      : index_(w_index), ref_count_(count){};

  WNode(WNode const &rhs) = default;
  WNode &operator=(WNode const &rhs) = default;

  // Compute the hash excluding the ref_count
  template <typename H>
  friend H AbslHashValue(H h, const WNode& s) {
    return H::combine(std::move(h), s.index_);
  }

  bool operator==(WNode const &rhs) const { return index_ == rhs.index_; }
  bool operator!=(WNode const &rhs) const { return !operator==(rhs); }

  r_index get_index() const { return index_; }

  int get_ref_count() const { return ref_count_; }

  int add_ref_count(int count = 1) const {
    ref_count_ += count;
    return ref_count_;
  }

  int del_ref_count(int count = 1) const {
    ref_count_ -= count;
    return ref_count_;
  }

 private:
  r_index index_;
  mutable int ref_count_;
};

} // namespace jets::rdf
#endif // JETS_RDF_W_NODE_H
