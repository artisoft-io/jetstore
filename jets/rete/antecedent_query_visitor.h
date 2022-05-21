#ifndef JETS_RETE_ANTECEDENT_QUERY_VISITOR_H
#define JETS_RETE_ANTECEDENT_QUERY_VISITOR_H

#include <string>
#include <memory>

#include <boost/variant/multivisitors.hpp>

#include "../rdf/rdf_types.h"
#include "../rete/node_vertex.h"
#include "../rete/beta_row.h"
#include "../rete/beta_row_iterator.h"
#include "../rete/beta_relation.h"
#include "../rete/graph_callback_mgr_impl.h"
#include "../rete/alpha_node.h"
#include "../rete/rete_session.h"

// This file contains the visitor used to index BetaRow and query them from BetaRelation
namespace jets::rete {

struct AQIndex {
  AQIndex(int d): data(d) {}
  int data;

  inline
  rdf::r_index
  f(rdf::r_index r)const
  {
    return r;
  }

  inline
  rdf::r_index
  to_r(BetaRow const* row)const
  {
    return row->get(this->data);
  }

};
inline std::ostream & operator<<(std::ostream & out, AQIndex const& i)
{
  out <<"["<<i.data<<"]";
  return out;
}

struct AQOther {
  inline
  rdf::r_index
  f(rdf::r_index r)const
  {
    return nullptr;
  }

  inline
  rdf::r_index
  to_r(BetaRow const* row)const
  {
    return nullptr;
  }
};
inline std::ostream & operator<<(std::ostream & out, AQOther const& r)
{
  out <<"*";
  return out;
}

using AQV = boost::variant< AQIndex,AQOther >;

// AQVTriple class for convenience in printing the seach criteria
using AQVTriple = rdf::TripleBase<AQV>;
inline std::ostream & operator<<(std::ostream & out, AQVTriple const& t3)
{
  out << "("<<t3.subject<<","<<t3.predicate<<","<<t3.object<<")";
  return out;
}

inline std::string
to_string(AQVTriple const& t)
{
  std::ostringstream out;
  out << t;
  return out.str();
}

// visitor
struct AQVMatchingRowsVisitor: public boost::static_visitor<BetaRowIteratorPtr>
{
  using I = AQIndex;
  using O = AQOther;
  using R = BetaRowIteratorPtr;
  AQVMatchingRowsVisitor(BetaRelation * g, b_index m, rdf::r_index s, rdf::r_index p, rdf::r_index o) 
    : parent_beta_relation(g), node_vertex(m), t3(s, p, o){}
  R operator()(I const&s, I const&p, I const&o){return parent_beta_relation->get_idx3_rows_iterator(node_vertex->antecedent_query_key, s.f(t3.subject), p.f(t3.predicate), o.f(t3.object));}
  R operator()(O const&s, I const&p, I const&o){return parent_beta_relation->get_idx2_rows_iterator(node_vertex->antecedent_query_key, p.f(t3.predicate), o.f(t3.object));}
  R operator()(O const&s, O const&p, I const&o){return parent_beta_relation->get_idx1_rows_iterator(node_vertex->antecedent_query_key, o.f(t3.object));}
  R operator()(O const&s, O const&p, O const&o){return parent_beta_relation->get_all_rows_iterator();}
  R operator()(I const&s, O const&p, I const&o){return parent_beta_relation->get_idx2_rows_iterator(node_vertex->antecedent_query_key, s.f(t3.subject), o.f(t3.object));}
  R operator()(I const&s, O const&p, O const&o){return parent_beta_relation->get_idx1_rows_iterator(node_vertex->antecedent_query_key, s.f(t3.subject));}
  R operator()(I const&s, I const&p, O const&o){return parent_beta_relation->get_idx2_rows_iterator(node_vertex->antecedent_query_key, s.f(t3.subject), p.f(t3.predicate));}
  R operator()(O const&s, I const&p, O const&o){return parent_beta_relation->get_idx1_rows_iterator(node_vertex->antecedent_query_key, p.f(t3.predicate));}
  BetaRelation * parent_beta_relation;
  b_index node_vertex;
  rdf::Triple t3;
};

// visitor
struct AQVIndexBetaRowsVisitor: public boost::static_visitor<>
{
  using I = AQIndex;
  using O = AQOther;
  AQVIndexBetaRowsVisitor(BetaRelation * g, b_index m, BetaRow const* r) 
    : beta_relation(g), node_vertex(m), row(r), key(m->antecedent_query_key) {}
  void operator()(I const&s, I const&p, I const&o){ beta_relation->beta_row_idx3_[key].insert( {{s.to_r(row), p.to_r(row), o.to_r(row)}, row} );}
  void operator()(O const&s, I const&p, I const&o){ beta_relation->beta_row_idx2_[key].insert( {{p.to_r(row), o.to_r(row)}, row} );}
  void operator()(O const&s, O const&p, I const&o){ beta_relation->beta_row_idx1_[key].insert( {o.to_r(row), row} );}
  void operator()(O const&, O const&, O const&){ }
  void operator()(I const&s, O const&p, I const&o){ beta_relation->beta_row_idx2_[key].insert( {{s.to_r(row), o.to_r(row)}, row} );}
  void operator()(I const&s, O const&p, O const&o){ beta_relation->beta_row_idx1_[key].insert( {s.to_r(row), row} );}
  void operator()(I const&s, I const&p, O const&o){ beta_relation->beta_row_idx2_[key].insert( {{s.to_r(row), p.to_r(row)}, row} );}
  void operator()(O const&s, I const&p, O const&o){ beta_relation->beta_row_idx1_[key].insert( {p.to_r(row), row} );}
  BetaRelation * beta_relation;
  b_index node_vertex;
  BetaRow const* row;
  int key;
};

// visitor
struct AQVRemoveIndexBetaRowsVisitor: public boost::static_visitor<>
{
  using I = AQIndex;
  using O = AQOther;
  AQVRemoveIndexBetaRowsVisitor(BetaRelation * g, b_index m, BetaRow const* r) 
    : beta_relation(g), node_vertex(m), row(r), key(m->antecedent_query_key) {}
  void operator()(I const&s, I const&p, I const&o){ beta_relation->beta_row_idx3_[key].erase( {s.to_r(row), p.to_r(row), o.to_r(row)} );}
  void operator()(O const&s, I const&p, I const&o){ beta_relation->beta_row_idx2_[key].erase( {p.to_r(row), o.to_r(row)} );}
  void operator()(O const&s, O const&p, I const&o){ beta_relation->beta_row_idx1_[key].erase( o.to_r(row) );}
  void operator()(O const&, O const&, O const&){ }
  void operator()(I const&s, O const&p, I const&o){ beta_relation->beta_row_idx2_[key].erase( {s.to_r(row), o.to_r(row)} );}
  void operator()(I const&s, O const&p, O const&o){ beta_relation->beta_row_idx1_[key].erase( s.to_r(row) );}
  void operator()(I const&s, I const&p, O const&o){ beta_relation->beta_row_idx2_[key].erase( {s.to_r(row), p.to_r(row)} );}
  void operator()(O const&s, I const&p, O const&o){ beta_relation->beta_row_idx1_[key].erase( p.to_r(row) );}
  BetaRelation * beta_relation;
  b_index node_vertex;
  BetaRow const* row;
  int key;
};

// visitor
struct AQVInitializeIndexesVisitor: public boost::static_visitor<>
{
  using I = AQIndex;
  using O = AQOther;
  AQVInitializeIndexesVisitor(BetaRelation * g, b_index m) 
    : beta_relation(g), node_vertex(m) {}
  void operator()(I const&, I const&, I const&){ this->node_vertex->antecedent_query_key = this->add_query3(); VLOG(50)<<"Q3 parent beta row vertex "<<this->beta_relation->node_vertex_->vertex<<" | Child NodeVertex "<<this->node_vertex->vertex<<", antecedent_query_key "<<this->node_vertex->antecedent_query_key<<">";}
  void operator()(O const&, I const&, I const&){ this->node_vertex->antecedent_query_key = this->add_query2();VLOG(50)<<"Q2 parent beta row vertex "<<this->beta_relation->node_vertex_->vertex<<" | Child NodeVertex "<<this->node_vertex->vertex<<", antecedent_query_key "<<this->node_vertex->antecedent_query_key<<">";}
  void operator()(O const&, O const&, I const&){ this->node_vertex->antecedent_query_key = this->add_query1();VLOG(50)<<"Q1 parent beta row vertex "<<this->beta_relation->node_vertex_->vertex<<" | Child NodeVertex "<<this->node_vertex->vertex<<", antecedent_query_key "<<this->node_vertex->antecedent_query_key<<">";}
  void operator()(O const&, O const&, O const&){ }
  void operator()(I const&, O const&, I const&){ this->node_vertex->antecedent_query_key = this->add_query2();VLOG(50)<<"Q2 parent beta row vertex "<<this->beta_relation->node_vertex_->vertex<<" | Child NodeVertex "<<this->node_vertex->vertex<<", antecedent_query_key "<<this->node_vertex->antecedent_query_key<<">";}
  void operator()(I const&, O const&, O const&){ this->node_vertex->antecedent_query_key = this->add_query1();VLOG(50)<<"Q1 parent beta row vertex "<<this->beta_relation->node_vertex_->vertex<<" | Child NodeVertex "<<this->node_vertex->vertex<<", antecedent_query_key "<<this->node_vertex->antecedent_query_key<<">";}
  void operator()(I const&, I const&, O const&){ this->node_vertex->antecedent_query_key = this->add_query2();VLOG(50)<<"Q2 parent beta row vertex "<<this->beta_relation->node_vertex_->vertex<<" | Child NodeVertex "<<this->node_vertex->vertex<<", antecedent_query_key "<<this->node_vertex->antecedent_query_key<<">";}
  void operator()(O const&, I const&, O const&){ this->node_vertex->antecedent_query_key = this->add_query1();VLOG(50)<<"Q1 parent beta row vertex "<<this->beta_relation->node_vertex_->vertex<<" | Child NodeVertex "<<this->node_vertex->vertex<<", antecedent_query_key "<<this->node_vertex->antecedent_query_key<<">";}

  inline int
  add_query1() {beta_relation->beta_row_idx1_.push_back({}); return (int)(beta_relation->beta_row_idx1_.size()-1);}
  inline int
  add_query2() {beta_relation->beta_row_idx2_.push_back({}); return (int)(beta_relation->beta_row_idx2_.size()-1);}
  inline int
  add_query3() {beta_relation->beta_row_idx3_.push_back({}); return (int)(beta_relation->beta_row_idx3_.size()-1);}
  BetaRelation * beta_relation;
  b_index node_vertex;
};


} // namespace jets::rete
#endif // JETS_RETE_ANTECEDENT_QUERY_VISITOR_H
