#include <iostream>
#include <memory>
#include <vector>

#include <glog/logging.h>
// #include <gflags/gflags.h>

#include "beta_row.h"
#include "jets/rdf/rdf_types.h"
#include "jets/rete/rete_types.h"

// DEFINE_bool(big_menu, true, "Include 'advanced' options in the menu listing");
// DEFINE_string(languages, "english,french,german",
//               "comma-separated list of languages to offer in the 'lang' menu");

namespace jets::rete {

template<class T>
  int 
  ReteSession<T>::initialize()
  {
    //* TODO init beta relation vector
    return 0;
  }

template<class T>
  int 
  ReteSession<T>::execute_rules()
  {
    // This is the only place we call execute_rule with compute_consequent = true
    return execute_rules(0, true, true);
  }

template<class T>
  int 
  ReteSession<T>::execute_rules(int from_vertex, bool is_inferring, bool compute_consequents)
  {
    // Visit the beta nodes
    int err = visit_rete_graph(from_vertex, is_inferring);
    if(err < 0) {
      LOG(ERROR) << "ReteSession<T>::execute_rules: error returned from "
        "visit_rete_graph(from_vertex="<<from_vertex<<", is_inferring="<<is_inferring<<")";
      return err;
    }

    if(compute_consequents) {
      err = compute_consequent_triples();
      if(err < 0) {
        LOG(ERROR) << "ReteSession<T>::execute_rules: error returned from "
          "compute_consequent_triples() called with: from_vertex="<<from_vertex<<", is_inferring="<<is_inferring<<".";
        return err;
      }
    }

    return 0;
  }

/**
 * @brief Visit the Rete Graph and apply inferrence or retactation of inferred facts
 * 
 * Perform DFS graph visitation starting at node `from_vertex`
 *
 * @tparam T ReteSession template parameter corresponding to the RDFSession type
 * @param from_vertex Starting point of graph visitation
 * @param is_inferring apply inferrence if true, retract inferrence if false
 * @return int 0 if normal, -1 if error
 */
template<class T>
  int 
  ReteSession<T>::visit_rete_graph(int from_vertex, bool is_inferring)
  {
			std::vector<int> stack;
			stack.reserve(rule_ms_->nbr_vertices());
			
			stack.push_back({from_vertex});
			
			while(!stack.empty()) {
			
				int parent_vertex = stack.back();
				stack.pop_back();

        auto adj_vertices = this->rule_ms_->get_adj_node_vertexes(parent_vertex);
				auto itor = adj_vertices.first;
				auto end  = adj_vertices.second;
				for(; itor!=end; ++itor) {

          // Compute beta relation between `parent_vertex` and `vertex`
					int current_vertex = *itor;
          auto * parent_beta_relation = this->get_beta_relation(parent_vertex);
          auto * current_relation = this->get_beta_relation(current_vertex);
          if(not parent_beta_relation or not current_relation) {
            LOG(ERROR) << "visit_rete_graph from_vertex "
                       <<from_vertex<<": error beta_relation is null!";
            return -1;
          }

          // Clear the pending rows in current_relation, since they were for the last pass
          current_relation->clear_pending_rows();

          // Get an iterator over all applicable rows from the parent beta node
          BetaRowIteratorPtr parent_row_itor;
          bool need_all_rows = !current_relation->is_activated();
          if(need_all_rows) {
            parent_row_itor = parent_beta_relation->get_all_rows_iterator();
          } else {
            parent_row_itor = parent_beta_relation->get_pending_rows_iterator();
          }
          
          // for each BetaRow of parent beta node, 
          // compute the inferred/retracted BetaRow for current_relation
          auto const* alpha_node = this->rule_ms_->get_alpha_node(current_vertex);
          b_index cmeta_node = current_relation->get_node_vertex();
          auto const* beta_row_initializer = cmeta_node->get_beta_row_initializer();
          while(!parent_row_itor->is_end()) {

            // for each triple from the rdf graph matching the AlphaNode
            // compute the BetaRow to infer or retract
            auto const* parent_row = parent_row_itor->get_row();
            auto t3_itor = alpha_node->find_matching_triples(this->rdf_session(), parent_row);
            while(not t3_itor.is_end()) {

              // create the beta row
              auto beta_row = create_beta_row(cmeta_node, beta_row_initializer->get_size());
              // initialize the beta row with parent_row and t3
              rdf::r_index t3[3];
              initialize_beta_row(beta_row, beta_row_initializer, 
                                  parent_row, t3_itor.get_triple(&t3[0]));

              // evaluate the current_relation filter if any
              bool keepit = true;
              if(cmeta_node->has_expr()) {
                auto const* expr = this->rule_ms_->get_expr(cmeta_node->expr_vertex);
                keepit = expr->eval_filter(this, beta_row);
              }

              // insert or remove the row from current_relation based on is_inferring
              if(keepit) {
                if(is_inferring) {
                  // Add row to current beta relation (current_relation)
                  current_relation->insert_beta_row(this, beta_row);
                } else {
                  // Remove row from current beta relation (current_relation)
                  current_relation->remove_beta_row(this, beta_row);
                }
              }
              t3_itor.next();
            }
            parent_row_itor->next();
          }
          if(need_all_rows) current_relation->set_activated(true);
          stack.push_back(current_vertex);
				}
			}

    return 0;
  }

template<class T>
  int 
  ReteSession<T>::schedule_consequent_terms(BetaRowPtr beta_row)
  {
    assert(beta_row);
    XXX;

    return 0;
  }

}  // namespace jets::rete