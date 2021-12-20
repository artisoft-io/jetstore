#ifndef JETS_RDF_W_NODE_H
#define JETS_RDF_W_NODE_H

#include <cstdint>
#include <utility>

#include "absl/hash/hash.h"

#include "jets/rdf/rdf_ast.h"

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
template<typename T=r_index>
class WNode {
 public:

  WNode() : m_index(nullptr), m_ref_count(0){};

  WNode(T w_index, int count = 1)
      : m_index(w_index), m_ref_count(count){};

  WNode(WNode const &rhs)
      : m_index(rhs.m_index), m_ref_count(rhs.m_ref_count){};

  inline WNode &operator=(WNode const &rhs) {
    m_index = rhs.m_index;
    m_ref_count = rhs.m_ref_count;
    return *this;
  }

  // Compute the hash excluding the ref_count
  template <typename H>
  friend H AbslHashValue(H h, const WNode& s) {
    return H::combine(std::move(h), s.m_index);
  }

  bool operator==(WNode const &rhs) const { return m_index == rhs.m_index; }
  bool operator!=(WNode const &rhs) const { return !operator==(rhs); }

  T get_index() const { return m_index; }

  int get_ref_count() const { return m_ref_count; }

  int add_ref_count(int count = 1) const {
    m_ref_count += count;
    return m_ref_count;
  }

  int del_ref_count(int count = 1) const {
    m_ref_count -= count;
    return m_ref_count;
  }

 private:

  T m_index;
  mutable int m_ref_count;
};

} // namespace jets::rdf
#endif // JETS_RDF_W_NODE_H
