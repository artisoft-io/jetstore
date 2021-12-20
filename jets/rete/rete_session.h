#ifndef JETS_RETE_RETE_SESSION_H
#define JETS_RETE_RETE_SESSION_H

#include <string>
#include <memory>
#include <list>
#include <vector>

#include "absl/hash/hash.h"
#include "absl/container/flat_hash_set.h"

#include "jets/rdf/rdf_types.h"
#include "jets/rete/node_vertex.h"
#include "jets/rete/beta_row_initializer.h"
#include "jets/rete/beta_row.h"
#include "jets/rete/beta_row_iterator.h"
#include "jets/rete/beta_relation.h"
#include "jets/rete/alpha_node.h"
#include "jets/rete/rete_meta_store.h"


// Component to manage all the rdf resources and literals of a graph
namespace jets::rete {
// //////////////////////////////////////////////////////////////////////////////////////
// ReteSession class -- main session class for the rete network
// --------------------------------------------------------------------------------------
template<class T>
class ReteSession;
template<class T>
using ReteSessionPtr = std::shared_ptr<ReteSession<T>>;

using BetaRelationVector = std::vector<BetaRelationPtr>;

// ReteSession making the rete network
template<class T>
class ReteSession {
 public:
  using RDFSession = T;
  using RDFSessionPtr = std::shared_ptr<T>;
  using Iterator = typename T::Iterator;
  using RDFGraph = typename T::RDFGraph;
  using RDFGraphPtr = std::shared_ptr<RDFGraph>;

  ReteSession()
    : rule_ms_(nullptr),
      rdf_session_(nullptr),
      beta_relations_()
    {}

  ReteSession(ReteMetaStore<T> const* rule_ms, RDFSession * rdf_session) 
    : rule_ms_(rule_ms),
      rdf_session_(rdf_session),
      beta_relations_()
    {}

  inline RDFSession *
  rdf_session()
  {
    return rdf_session_;
  }

  inline ReteMetaStore<T> const*
  rule_ms()const
  {
    return rule_ms_;
  }

  inline BetaRelation *
  get_beta_relation(int vertex)
  {
    if(vertex<0 or vertex>= beta_relations_.size()) return nullptr;
    return beta_relations_[vertex].get();
  }
  
  int 
  execute_rules();

  int
  initialize();

 protected:

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
   * @brief Visit rete graph using dfs to activate beta nodes
   * 
   * @param from_vertex start vertex, 0 is root (head) node
   * @param is_inferring to forward chaining, otherwise retract inferrence
   * @return int 0 for normal, -1 if error
   */
  int 
  visit_rete_graph(int from_vertex, bool is_inferring);

  /**
   * @brief Compute consequent triples from activated rules
   *
   * Inferred triples are added to inferred graph of rdf session.
   * 
   * @return int 0 for normal, -1 if error
   */
  int
  compute_consequent_triples();

 private:
  // friend class find_visitor<RDFGraph>;
  // friend class RDFSession<RDFGraph>;

  ReteMetaStore<T> const*  rule_ms_;
  RDFSession  *            rdf_session_;
  BetaRelationVector       beta_relations_; //* TODO initialize()
};

template<class T>
inline ReteSessionPtr<T> create_rete_session(b_index node_vertex)
{
  return std::make_shared<ReteSession<T>>(node_vertex);
}

} // namespace jets::rete
#endif // JETS_RETE_RETE_SESSION_H
