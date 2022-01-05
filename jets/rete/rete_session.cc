#include <cstddef>
#include <iostream>
#include <memory>
#include <vector>

#include <glog/logging.h>
// #include <gflags/gflags.h>

#include "beta_row_initializer.h"
#include "jets/rdf/rdf_types.h"
#include "jets/rete/rete_types.h"
#include "node_vertex.h"
#include "rete_err.h"
#include "rete_meta_store.h"

// DEFINE_bool(big_menu, true, "Include 'advanced' options in the menu listing");
// DEFINE_string(languages, "english,french,german",
//               "comma-separated list of languages to offer in the 'lang' menu");

namespace jets::rete {

  int 
  ReteSession::initialize(ReteMetaStorePtr rule_ms)
  {
    if(not rule_ms) {
      RETE_EXCEPTION("ReteSession::Initialize requires a valid ReteMetaStore as argument");
    }
    this->rule_ms_ = rule_ms;
    std::cout<<"ReteSession::initialize init beta relations..."<<std::endl;
    beta_relations_.reserve(this->rule_ms_->node_vertexes_.size());
    // Initialize BetaRelationVector beta_relations_
    for(size_t ipos=0; ipos<this->rule_ms_->node_vertexes_.size(); ++ipos) {
      auto const* meta_node = this->rule_ms_->node_vertexes_[ipos].get();
      auto bn = create_beta_node(meta_node);
      bn->initialize();
      if(meta_node->is_head_vertice()) {
        // put an empty BetaRow to kick start the propagation in the rete network
        bn->insert_beta_row(this, create_beta_row(meta_node, 0));
      }
      beta_relations_.push_back(bn);
    }
    std::cout<<"ReteSession::initialize BetaRelations initalized -- now calbacks"<<std::endl;
    this->set_graph_callbacks();
    return 0;
  }

  int 
  ReteSession::set_graph_callbacks()
  {
    // Check if has any AlphaNode (to support test mode)
    if(this->rule_ms_->alpha_nodes_.empty()) {
      LOG(WARNING) << "ReteSession::set_graph_callbacks: ReteMetaStore does not "
        "have AlphNodes, skipping grapg callback setup)";
      return -1;
    }
    // Preparing the list of callbacks from the AlphaNodes
    for(size_t ipos=0; ipos<this->rule_ms_->node_vertexes_.size(); ++ipos) {

      // Register GraphCallbackManager using antecedent AlphaNode adaptor
      // Taking into consideration that antecedent AlphaNodes have the
      // same index as NodeVertex
      this->rule_ms_->alpha_nodes_[ipos]->register_callback(this);
    }
    return 0;
  }

  int 
  ReteSession::remove_graph_callbacks()
  {
    this->rdf_session()->inferred_graph()->remove_all_callbacks();
    return 0;
  }

  int 
  ReteSession::execute_rules()
  {
    // This is the only place we call execute_rule with compute_consequent = true
    return execute_rules(0, true, true);
  }

  int 
  ReteSession::execute_rules(int from_vertex, bool is_inferring, bool compute_consequents)
  {
    std::cout<<"ReteSession::execute_rules called, starting at "<<from_vertex<<std::endl;

    // Visit the beta nodes
    int err = visit_rete_graph(from_vertex, is_inferring);
    if(err < 0) {
      LOG(ERROR) << "ReteSession::execute_rules: error returned from "
        "visit_rete_graph(from_vertex="<<from_vertex<<", is_inferring="<<is_inferring<<")";
      return err;
    }

    if(compute_consequents) {
      err = compute_consequent_triples();
      if(err < 0) {
        LOG(ERROR) << "ReteSession::execute_rules: error returned from "
          "compute_consequent_triples() called with: from_vertex="<<from_vertex<<", is_inferring="<<is_inferring<<".";
        return err;
      }
    }

    return 0;
  }

  int 
  ReteSession::visit_rete_graph(int from_vertex, bool is_inferring)
  {
    std::cout<<"ReteSession::visit_rete_graph called, starting at "<<from_vertex<<", is_inferring "<<is_inferring<<std::endl;
			std::vector<int> stack;
			stack.reserve(rule_ms_->nbr_vertices());
			
			stack.push_back({from_vertex});
			
			while(!stack.empty()) {
			
				int parent_vertex = stack.back();
				stack.pop_back();

        std::cout<<"ReteSession::visit_rete_graph stack pop `parent_vertex` "<<parent_vertex<<std::endl;

        b_index parent_node = this->rule_ms_->get_node_vertex(parent_vertex);
        if(not parent_node) {
          std::cout<<"We've found a problem!"<<std::endl;
        }
        std::cout<<"ReteSession::visit_rete_graph stack pop *(2) `parent_vertex` "<<parent_vertex<<" :: "<<parent_node->vertex<<std::endl;
        if(parent_node->child_nodes.empty()) continue;
        std::cout<<"ReteSession::visit_rete_graph stack pop *(3) `parent_vertex` "<<parent_vertex<<" :: "<<parent_node->vertex<<std::endl;
				auto itor = parent_node->child_nodes.begin();
				auto end  = parent_node->child_nodes.end();
				for(; itor!=end; ++itor) {
        std::cout<<"ReteSession::visit_rete_graph stack pop (4) `parent_vertex` "<<parent_vertex<<" :: "<<parent_node->vertex<<std::endl;

          // Compute beta relation between `parent_vertex` and `vertex`
					int current_vertex = (*itor)->vertex;
        std::cout<<"ReteSession::visit_rete_graph stack pop (5) `parent_vertex` "<<parent_vertex<<" :: current vertex "<<current_vertex<<std::endl;

          std::cout<<"ReteSession::visit_rete_graph Compute beta relation between `parent_vertex` "<<parent_vertex<<", and `current_vertex`  "<<current_vertex<<std::endl;

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

          // process rows from parent beta node:
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

              std::cout<<"ReteSession::visit_rete_graph Compute beta relation between `row` "<<parent_row<<", and `t3`  "<<t3_itor.as_triple()<<std::endl;

              // create the beta row
              auto beta_row = create_beta_row(cmeta_node, static_cast<int>(beta_row_initializer->get_size()));
              // initialize the beta row with parent_row and t3
              rdf::Triple triple = t3_itor.as_triple();
              beta_row->initialize(beta_row_initializer, parent_row, &triple);

              // evaluate the current_relation filter if any
              bool keepit = true;
              if(cmeta_node->has_expr()) {
                auto const* expr = this->rule_ms_->get_expr(cmeta_node->expr_vertex);
                keepit = expr->eval_filter(this, beta_row.get());
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

          // mark current beta node as activated and push it on the stack so to visit it's childrens
          if(need_all_rows) current_relation->set_activated(true);
          stack.push_back(current_vertex);
				}
			}
    return 0;
  }

  int 
  ReteSession::schedule_consequent_terms(BetaRowPtr beta_row)
  {
    assert(beta_row);
    //* TODO Check for max visit allowed for a vertex
    this->pending_beta_rows_.push(beta_row);
    return 0;
  }

  int 
  ReteSession::compute_consequent_triples()
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

      if(meta_node->has_consequent_terms()) {
        if(beta_row->is_inserted()) {
          for(int consequent_vertex: meta_node->consequent_alpha_vertexes) {
            auto const* consequent_node = this->rule_ms_->get_alpha_node(consequent_vertex);
            this->rdf_session_->insert_inferred(consequent_node->compute_consequent_triple(beta_row.get()));
          }
        } else {
          // beta_row status must be kDeleted, meaning retracting mode
          if(not beta_row->is_deleted()) {
            LOG(ERROR) << "compute_consequent_triples: Invalid beta_row at vertex "
                  <<meta_node->vertex<<": error expecting status deleted, got "<<beta_row->get_status();
            RETE_EXCEPTION("compute_consequent_triples: Invalid beta_row at vertex "
                  <<meta_node->vertex<<": error expecting status deleted, got "<<beta_row->get_status());
          }
          for(int consequent_vertex: meta_node->consequent_alpha_vertexes) {
            auto const* consequent_node = this->rule_ms_->get_alpha_node(consequent_vertex);
            this->rdf_session_->retract(consequent_node->compute_consequent_triple(beta_row.get()));
          }
        }
      }
    }
    return 0;
  }

  int
  ReteSession::triple_updated(int vertex, rdf::r_index s, rdf::r_index p, rdf::r_index o, bool is_inserted)
  {
    b_index meta_node = this->rule_ms_->get_node_vertex(vertex);

    // make sure this is not the rete head node
    if(meta_node->is_head_vertice()) return 0;

    auto * parent_beta_relation = this->get_beta_relation(meta_node->parent_node_vertex->vertex);
    auto * current_relation = this->get_beta_relation(vertex);
    if(not parent_beta_relation or not current_relation) {
      LOG(ERROR) << "ReteSession::triple_updated @ vertex "
        <<vertex<<": error parent_beta_relation or beta_relation is null!";
      RETE_EXCEPTION("ReteSession::triple_updated @ vertex "
        <<vertex<<": error parent_beta_relation or beta_relation is null!");
    }

    // Clear the pending rows in current_relation, since they were for the last pass
    current_relation->clear_pending_rows();

    // Get an iterator over all applicable rows from the parent beta node
    // which is provided by the alpha node adaptor
    auto const* alpha_node = this->rule_ms_->get_alpha_node(vertex);
    BetaRowIteratorPtr parent_row_itor = alpha_node->find_matching_rows(parent_beta_relation, s, p, o);

    // for each BetaRow of parent beta node, 
    // compute the inferred/retracted BetaRow for the added/retracted triple (s, p, o)
    rdf::Triple t3(s, p, o);
    b_index cmeta_node = current_relation->get_node_vertex();
    auto const* beta_row_initializer = cmeta_node->get_beta_row_initializer();
    while(!parent_row_itor->is_end()) {

      // create the beta row to add/retract
      auto beta_row = create_beta_row(cmeta_node, static_cast<int>(beta_row_initializer->get_size()));

      // initialize the beta row with parent_row and t3
      auto const* parent_row = parent_row_itor->get_row();
      beta_row->initialize(beta_row_initializer, parent_row, &t3);

      // evaluate the current_relation filter if any
      bool keepit = true;
      if(cmeta_node->has_expr()) {
        auto const* expr = this->rule_ms_->get_expr(cmeta_node->expr_vertex);
        keepit = expr->eval_filter(this, beta_row.get());
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
    
      parent_row_itor->next();
    }
    return 0;
  }

}  // namespace jets::rete