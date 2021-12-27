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
    beta_relations_.reserve(this->rule_ms_->node_vertexes_.size());
    for(int ipos=0; ipos<this->rule_ms_->node_vertexes_.size(); ++ipos) {

      // Initialize BetaRelationVector beta_relations_
      beta_relations_[ipos] = create_beta_node(this->rule_ms_->node_vertexes_[ipos]);
    }
    this->set_graph_callbacks();
    return 0;
  }

template<class T>
  int 
  ReteSession<T>::set_graph_callbacks()
  {
    // Preparing the list of callbacks from the AlphaNodes
    ReteCallBackList<T> callbacks;
    for(int ipos=0; ipos<this->rule_ms_->node_vertexes_.size(); ++ipos) {

      // Register GraphCallbackManager using antecedent AlphaNode adaptor
      // Taking into consideration that antecedent AlphaNodes are nodes:
      // ReteMetaStore::alpha_nodes_[i], i=0, ReteMetaStore::node_vertexes_.size()
      this->rule_ms_->alpha_nodes_[ipos]->register_callback(this, &callbacks);
    }
    auto graph_callback_mgr = create_graph_callback(std::move(callbacks));
    this->rdf_session()->inferred_graph()->set_graph_callback_manager(graph_callback_mgr);
    return 0;
  }

template<class T>
  int 
  ReteSession<T>::remove_graph_callbacks()
  {
    this->rdf_session()->inferred_graph()->set_graph_callback_manager({});
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

          // process rows from parent beta node
          auto const* alpha_node = this->rule_ms_->get_alpha_node(current_vertex);
          this->process_parent_rows(current_relation, alpha_node, parent_row_itor.get(), is_inferring);

          // mark current beta node as activated and push it on the stack so to visit it's childrens
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
    //* TODO Check for max visit allowed for a vertex
    this->pending_beta_rows_.push(beta_row);
    return 0;
  }

template<class T>
  int 
  ReteSession<T>::compute_consequent_triples()
  {
    while(not this->pending_beta_rows_.empty()) {
      BetaRowPtr beta_row = this->pending_beta_rows_.top();
      this->pending_beta_rows_.pop();
      if(beta_row->is_processed()) {
        //*
        std::cout<<"compute_consequent_triples: row already processed: "<<beta_row<<", skipping"<<std::endl;
        continue;
      }

      // get the beta node and the vertex_node associated with the beta_row
      b_index meta_node = beta_row->get_node_vertex();
      BetaRelation * beta_relation = this->get_beta_relation(meta_node->vertex);
      if(not beta_relation) {
        LOG(ERROR) << "compute_consequent_triples: Invalid beta_relation at vertex "
                    <<meta_node->vertex<<": error beta_relation is null!";
        RETE_EXCEPTION("compute_consequent_triples: Invalid beta_relation at vertex "
                    <<meta_node->vertex<<": error beta_relation is null!");
      }

      //* TODO Log infer/retract event here to trace inferrence process (aka explain why)
      //* TODO Track how many times a rule infer/retract triples here (aka rule stat collector)

      auto const* consequent_nodes = this->rule_ms_->get_consequent_nodes(meta_node->vertex);
      if(consequent_nodes) {
        if(beta_row->is_inserted()) {
          for(auto const* consequent_node: *consequent_nodes) {
            this->rdf_session_->insert_inferred(consequent_node->compute_consequent_triple(beta_row));
          }
        } else {
          // beta_row status must be kDeleted, meaning retracting mode
          if(not beta_row->is_deleted()) {
            LOG(ERROR) << "compute_consequent_triples: Invalid beta_row at vertex "
                  <<meta_node->vertex<<": error expecting status deleted, got "<<beta_row->get_status();
            RETE_EXCEPTION("compute_consequent_triples: Invalid beta_row at vertex "
                  <<meta_node->vertex<<": error expecting status deleted, got "<<beta_row->get_status());
          }
          for(auto const* consequent_node: *consequent_nodes) {
            this->rdf_session_->retract(consequent_node->compute_consequent_triple(beta_row));
          }
        }
      }
    }
    return 0;
  }

template<class T>
  int
  ReteSession<T>::process_parent_rows(BetaRelation * current_relation, AlphaNode<T> const* alpha_node, 
    BetaRowIterator * parent_row_itor, bool is_inserted)
  {    
    // for each BetaRow of parent beta node, 
    // compute the inferred/retracted BetaRow for current_relation
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

        // insert or remove the row from current_relation based on is_inserted
        if(keepit) {
          if(is_inserted) {
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
  }

template<class T>
  int
  ReteSession<T>::triple_updated(int vertex, rdf::r_index s, rdf::r_index p, rdf::r_index o, bool is_inserted)
  {
    b_index meta_node = this->rule_ms_->get_node_vertex(vertex);
    auto * parent_beta_relation = this->get_beta_relation(meta_node->parent_node_vertex);
    auto * current_relation = this->get_beta_relation(vertex);
    if(not parent_beta_relation or not current_relation) {
      LOG(ERROR) << "ReteSession::triple_updated @ vertex "
                  <<vertex<<": error beta_relation is null!";
      return -1;
    }

    // Clear the pending rows in current_relation, since they were for the last pass
    current_relation->clear_pending_rows();

    // Get an iterator over all applicable rows from the parent beta node
    // which is provided by the alpha node adaptor
    auto const* alpha_node = this->rule_ms_->get_alpha_node(vertex);
    BetaRowIteratorPtr parent_row_itor = alpha_node->find_matching_rows(parent_beta_relation, s, p, o);
    return this->process_parent_rows(current_relation, alpha_node, parent_row_itor.get(), is_inserted);
  }

}  // namespace jets::rete