#include <cstddef>
#include <iostream>
#include <memory>
#include <vector>

#include <glog/logging.h>
// #include <gflags/gflags.h>

#include "../rdf/rdf_types.h"
#include "../rete/rete_types.h"
#include "../rete/rete_types_impl.h"

// DEFINE_bool(big_menu, true, "Include 'advanced' options in the menu listing");
// DEFINE_string(languages, "english,french,german",
//               "comma-separated list of languages to offer in the 'lang' menu");

namespace jets::rete {

  int 
  ReteSession::initialize()
  {
    if(not this->rule_ms_) {
      RETE_EXCEPTION("ReteSession::Initialize requires a valid ReteMetaStore as argument");
    }
    beta_relations_.reserve(this->rule_ms_->node_vertexes_.size());
    // Initialize BetaRelationVector beta_relations_
    for(size_t ipos=0; ipos<this->rule_ms_->node_vertexes_.size(); ++ipos) {
      auto const* meta_node = this->rule_ms_->node_vertexes_[ipos].get();
      auto bn = create_beta_node(meta_node);
      bn->initialize(this);
      if(meta_node->is_head_vertice()) {
        // put an empty BetaRow to kick start the propagation in the rete network
        bn->insert_beta_row(this, create_beta_row(meta_node, 0));
      }
      beta_relations_.push_back(bn);
    }
    auto ret = this->set_graph_callbacks();
    return ret;
  }

  int 
  ReteSession::terminate()
  {
    return this->remove_graph_callbacks();
  }

  int 
  ReteSession::set_graph_callbacks()
  {
    // Check if has any AlphaNode (to support test mode)
    if(this->rule_ms_->alpha_nodes_.empty()) {
      LOG(WARNING) << "ReteSession::set_graph_callbacks: ReteMetaStore does not "
        "have AlphaNodes, skipping graph callback setup)";
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
    if(not this->rdf_session_) return -1;
    this->rdf_session_->asserted_graph()->remove_all_callbacks();
    this->rdf_session_->inferred_graph()->remove_all_callbacks();
    return 0;
  }

  int 
  ReteSession::execute_rules()
  {
    // This is the only place we call execute_rule with compute_consequent = true
    return execute_rules(0, true, true);
  }

  // this version is called from the go client is the way forward
  char const* 
  ReteSession::execute_rules2(int*v)
  {
    if(not v) return nullptr;
    // Visit the beta nodes
    try {
      int ret = this->execute_rules(0, true, true);
      if(ret < 0) {
        this->err_msg_ = std::string("execute rules returned error code ") +
        std::to_string(ret);
        *v = ret;
        return this->err_msg_.data();
      }
    } catch (std::exception& err) {
      LOG(ERROR) << "ReteSession::execute_rules: error:"<<err.what();
      this->err_msg_ = std::string(err.what());
      *v = -1;
      return this->err_msg_.data();
    } catch (...) {
      LOG(ERROR) << "ReteSession::execute_rules: unknown error";
      *v = -1;
      this->err_msg_ = std::string("unknown error in executing rules");
      return this->err_msg_.data();
    }
    *v = 0;
    return nullptr;
  }

  int 
  ReteSession::execute_rules(int from_vertex, bool is_inferring, bool compute_consequents)
  {
    VLOG(5)<<"ReteSession::execute_rules called, starting at "<<from_vertex;

    // Visit the beta nodes
    int err = visit_rete_graph(from_vertex, is_inferring);
    if(err < 0) {
      LOG(ERROR) << "ReteSession::execute_rules: error returned from "
        "visit_rete_graph(from_vertex="<<from_vertex<<", is_inferring="<<is_inferring<<")";
      return err;
    }

    if(compute_consequents) {
      VLOG(5)<<"execute_rules: COMPUTING CONSEQUENT TERMS";
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
    VLOG(5)<<"ReteSession::visit_rete_graph called, starting at "<<from_vertex<<", is_inferring? "<<is_inferring;
    std::vector<int> stack;
    stack.reserve(rule_ms_->nbr_vertices());
    
    stack.push_back({from_vertex});
    
    while(!stack.empty()) {
      int parent_vertex = stack.back();
      stack.pop_back();

      b_index parent_node = this->rule_ms_->get_node_vertex(parent_vertex);
      for(auto const* cmeta_node: parent_node->child_nodes) {
        VLOG(5)<<"  @ parent node "<<parent_vertex<<" | child node "<<cmeta_node->vertex<<">";

        // Compute beta relation between `parent_vertex` and `vertex`
        int current_vertex = cmeta_node->vertex;
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
        auto const* beta_row_initializer = cmeta_node->get_beta_row_initializer();
        while(!parent_row_itor->is_end()) {

          // for each triple from the rdf graph matching the AlphaNode
          // compute the BetaRow to infer or retract
          auto const* parent_row = parent_row_itor->get_row();
          auto t3_itor = alpha_node->find_matching_triples(this->rdf_session(), parent_row);
          // Process the returned iterator t3_itor accordingly if AlphaNode is a negation
          if(cmeta_node->is_negation and t3_itor.is_end()) {
            // create the beta row
            auto beta_row = create_beta_row(cmeta_node, static_cast<int>(beta_row_initializer->get_size()));
            // initialize the beta row with parent_row and place holder for t3
            rdf::Triple triple;
            beta_row->initialize(beta_row_initializer, parent_row, &triple);

            VLOG(5)<<"    Parent Row "<<parent_row<<"  +  not"<<
              alpha_node->compute_find_triple(parent_row)<<"  =>  Row "<<beta_row;

            // evaluate the current_relation filter if any
            bool keepit = true;
            if(cmeta_node->has_expr()) {
              keepit = cmeta_node->filter_expr->eval_filter(this, beta_row.get());
              VLOG(5)<<"    Applying Filter ... "<<(keepit?"keep row":"reject row");
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
          } else {
            while(not t3_itor.is_end()) {
              // create the beta row
              auto beta_row = create_beta_row(cmeta_node, static_cast<int>(beta_row_initializer->get_size()));
              // initialize the beta row with parent_row and t3
              rdf::Triple triple = t3_itor.as_triple();
              beta_row->initialize(beta_row_initializer, parent_row, &triple);

              VLOG(5)<<"    Parent Row "<<parent_row<<"  +  "<<triple<<"  =>  Row "<<beta_row;

              // evaluate the current_relation filter if any
              bool keepit = true;
              if(cmeta_node->has_expr()) {
                keepit = cmeta_node->filter_expr->eval_filter(this, beta_row.get());
                VLOG(5)<<"    Applying Filter ... "<<(keepit?"keep row":"reject row");
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
          }
          parent_row_itor->next();
        }

        // mark current beta node as activated and push it on the stack so to visit it's childrens
        if(need_all_rows) current_relation->set_activated(true);
        stack.push_back(current_vertex);
      }
    }
    VLOG(5)<<"OK done for visit_rete_graph";
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
        VLOG(5)<<"compute_consequent_triples: row already processed: "<<beta_row<<", skipping";
        continue;
      }
      VLOG(5)<<"ReteSession::compute_consequent_triples for vertex "<<beta_row->get_node_vertex()->vertex<<", with row "<<beta_row;

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
          // Mark row as Processed
          beta_row->set_status(BetaRowStatus::kProcessed);
          for(int consequent_vertex: meta_node->consequent_alpha_vertexes) {
            auto const* consequent_node = this->rule_ms_->get_alpha_node(consequent_vertex);
            //*
            VLOG(5)<<"    Inferring triple: "<<consequent_node->compute_consequent_triple(this, beta_row.get());
            this->rdf_session_->insert_inferred(consequent_node->compute_consequent_triple(this, beta_row.get()));
          }
        } else {
          // beta_row status must be kDeleted, meaning retracting mode
          if(not beta_row->is_deleted()) {
            LOG(ERROR) << "compute_consequent_triples: Invalid beta_row at vertex "
                  <<meta_node->vertex<<": error expecting status deleted, got "<<beta_row->get_status();
            RETE_EXCEPTION("compute_consequent_triples: Invalid beta_row at vertex "
                  <<meta_node->vertex<<": error expecting status deleted, got "<<beta_row->get_status());
          }
          // Mark row as Processed
          beta_row->set_status(BetaRowStatus::kProcessed);
          for(int consequent_vertex: meta_node->consequent_alpha_vertexes) {
            auto const* consequent_node = this->rule_ms_->get_alpha_node(consequent_vertex);
            //*
            VLOG(5)<<"    Retracting triple: "<<consequent_node->compute_consequent_triple(this, beta_row.get());
            this->rdf_session_->retract(consequent_node->compute_consequent_triple(this, beta_row.get()));
          }
        }
      }
    }
    return 0;
  }

  int
  ReteSession::triple_updated(int vertex, rdf::r_index s, rdf::r_index p, rdf::r_index o, bool is_inserted)
  {
    VLOG(5)<<"     *  ReteSession::triple_updated called "<<rdf::Triple(s, p, o)<<", vertex "<<vertex<<", inserted? "<<is_inserted << " reteSession::rule_ms_ "<<this->rule_ms_.get();
    b_index cmeta_node = this->rule_ms_->get_node_vertex(vertex);

    // make sure this is not the rete head node
    if(cmeta_node->is_head_vertice()) return 0;

    auto * parent_beta_relation = this->get_beta_relation(cmeta_node->parent_node_vertex->vertex);
    auto * current_relation = this->get_beta_relation(vertex);
    if(not parent_beta_relation or not current_relation) {
      LOG(ERROR) << "ReteSession::triple_updated @ vertex "
        <<vertex<<": error parent_beta_relation or beta_relation is null!";
      RETE_EXCEPTION("ReteSession::triple_updated @ vertex "
        <<vertex<<": error parent_beta_relation or beta_relation is null!");
    }

    // Get an iterator over all applicable rows from the parent beta node
    // which is provided by the alpha node adaptor
    auto const* alpha_node = this->rule_ms_->get_alpha_node(vertex);
    BetaRowIteratorPtr parent_row_itor = alpha_node->find_matching_rows(parent_beta_relation, s, p, o);

    // for each BetaRow of parent beta node, 
    // compute the inferred/retracted BetaRow for the added/retracted triple (s, p, o)
    rdf::Triple t3(s, p, o);
    auto const* beta_row_initializer = cmeta_node->get_beta_row_initializer();
    while(!parent_row_itor->is_end()) {

      // Clear the pending rows in current_relation, we propagate for each parent row
      current_relation->clear_pending_rows();

      // create the beta row to add/retract
      auto beta_row = create_beta_row(cmeta_node, beta_row_initializer->get_size());

      // initialize the beta row with parent_row and t3
      auto const* parent_row = parent_row_itor->get_row();

      VLOG(5)<<"            Parent Row "<<parent_row<<"  +  "<<t3<<"  =>  Row ..."<</*beta_row<<*/std::endl;

      beta_row->initialize(beta_row_initializer, parent_row, &t3);

      // evaluate the current_relation filter if any
      bool keepit = true;
      if(cmeta_node->has_expr()) {
        keepit = cmeta_node->filter_expr->eval_filter(this, beta_row.get());
      }

      // insert or remove the row from current_relation based on is_inserted
      if(keepit) {
        // Add/Remove row to current beta relation (current_relation)
        if(is_inserted) {
          if(not cmeta_node->is_negation) {
            VLOG(5)<<"                1.INSERTING ROW "<<beta_row<<" @ vertex "<<beta_row->get_node_vertex()->vertex;
            current_relation->insert_beta_row(this, beta_row);
            // Propagate down the rete network
            if(current_relation->has_pending_rows()) {
              auto err = this->visit_rete_graph(vertex, true);
              if(err) return err;
            }
          } else {
            VLOG(5)<<"                2.REMOVING ROW "<<beta_row<<" @ vertex "<<beta_row->get_node_vertex()->vertex;
            current_relation->remove_beta_row(this, beta_row);
            // Propagate down the rete network
            if(current_relation->has_pending_rows()) {
              auto err = this->visit_rete_graph(vertex, false);
              if(err) return err;
            }
          }
        } else {
          if(not cmeta_node->is_negation) {
            VLOG(5)<<"                3.REMOVING ROW "<<beta_row<<" @ vertex "<<beta_row->get_node_vertex()->vertex;
            current_relation->remove_beta_row(this, beta_row);
            // Propagate down the rete network
            if(current_relation->has_pending_rows()) {
              auto err = this->visit_rete_graph(vertex, false);
              if(err) return err;
            }
          } else {
            VLOG(5)<<"                4.INSERTING ROW "<<beta_row<<" @ vertex "<<beta_row->get_node_vertex()->vertex;
            current_relation->insert_beta_row(this, beta_row);
            // Propagate down the rete network
            if(current_relation->has_pending_rows()) {
              auto err = this->visit_rete_graph(vertex, true);
              if(err) return err;
            }
          }
        }
      }
      parent_row_itor->next();
    }
    return 0;
  }

}  // namespace jets::rete