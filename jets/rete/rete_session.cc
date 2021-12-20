#include <iostream>
#include <memory>
#include <vector>

#include <glog/logging.h>
// #include <gflags/gflags.h>

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
            LOG(ERROR) << "visit_rete_graph from_vertex "<<from_vertex<<": error beta_relation is null!";
            return -1;
          }
          bool need_all_rows = !current_relation->is_activated();
          // HERE: parent_relation->get_rows(need_all_rows) that returns the unified beta_row_iterator
          
          // compute_rows here

					// m_action(session_p, u, v);
					// session_p->get_beta_relation(v).set_has_fired(true);
					
					if(keep_vertex(session_p, v) and !terminate(session_p, v)) {
						stack.push_back(stack_elm(v, boost::adjacent_vertices(v, graph)));
					}
				}
			}

    return 0;
  }


}  // namespace jets::rete