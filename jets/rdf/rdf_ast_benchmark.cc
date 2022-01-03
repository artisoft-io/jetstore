#include <benchmark/benchmark.h>
#include "boost/variant.hpp"

#include "jets/rdf/rdf_types.h"
#include "rdf_ast.h"
#include "rdf_graph.h"

using namespace jets::rdf;

// class RSBase {
//   public:
//     RSBase() {}
//     virtual ~RSBase() {}
//     virtual int propagate()const=0;
// };

// class RSD1: public RSBase {
//   public:
//     RSD1(int id): RSBase(), id(id) {}
//     ~RSD1() override {}
//     int propagate()const override 
//     {
//         // std::cout<<"RSD1::propagate() called"<<std::endl;
//         return id;
//     }
//     int id;
// };

// class RSD2: public RSBase {
//   public:
//     RSD2(int id): RSBase(), id(id) {}
//     ~RSD2() override {}
//     int propagate()const override 
//     {
//         // std::cout<<"RSD2::propagate() called"<<std::endl;
//         return id;
//     }
//     int id;
// };

// static void BM_RSPropagate(benchmark::State& state) {
//   RSBase const* base1=nullptr;
//   RSBase const* base2=nullptr;
//   RSD1 d1(1);
//   base1 = static_cast<RSBase*>(&d1);
//   RSD2 d2(2);
//   base2 = static_cast<RSBase*>(&d2);
//   int k=0;

//   for (auto _ : state) {
//     int i = base1->propagate();
//     // benchmark::DoNotOptimize(i);

//     int j = base2->propagate();
//     // benchmark::DoNotOptimize(j);
//     k += i + j;
//   }
// }
// // Register the function as a benchmark
// BENCHMARK(BM_RSPropagate);

// struct RVD1 {int v()const {return 1;}};
// struct RVD2 {int v()const {return 2;}};
// inline std::ostream & operator<<(std::ostream & out, RVD1 const& r){ out <<std::string("*RVD1"); return out;}
// inline std::ostream & operator<<(std::ostream & out, RVD2 const& r){ out <<std::string("*RVD2"); return out;}

// using RVD = boost::variant< RVD1, RVD2>;

// // rvd visitor
// struct rvd_visitor: public boost::static_visitor<int>
// {
//   int operator()(RVD1 const&r)const{return r.v();}
//   int operator()(RVD2 const&r)const{return r.v();}
// };

// static void BM_RVDVisitor(benchmark::State& state) {

//   RVD rvd1 = RVD1();
//   RVD rvd2 = RVD2();
//   int k=0;
//   for (auto _ : state) {
//     int i = boost::apply_visitor( rvd_visitor(), rvd1 );
//     // benchmark::DoNotOptimize(i);

//     int j = boost::apply_visitor( rvd_visitor(), rvd2 );
//     // benchmark::DoNotOptimize(j);
//     k += i + j;
//   }
// }
// // Register the function as a benchmark
// BENCHMARK(BM_RVDVisitor);

static void BM_Find_Visitor(benchmark::State& state) {
  RDFSessionPtr rdf_session = create_rdf_session(create_rdf_graph());
  auto * rmgr = rdf_session->rmgr();
  auto r1 = rmgr->create_resource("r1");
  rdf_session->insert(r1, r1, r1);
  auto any = make_any();

  for (auto _ : state) {
    rdf_session->find(r1, r1, r1);
    rdf_session->find(r1, r1, any);
    rdf_session->find(r1, any, r1);
    rdf_session->find(r1, any, any);
    rdf_session->find(any, r1, r1);
    rdf_session->find(any, r1, any);
    rdf_session->find(any, any, r1);
    rdf_session->find(any, any, any);
  }

}
// Register the function as a benchmark
BENCHMARK(BM_Find_Visitor);

static void BM_Find_Index(benchmark::State& state) {
  RDFSessionPtr rdf_session = create_rdf_session(create_rdf_graph());
  auto * rmgr = rdf_session->rmgr();
  auto r1 = rmgr->create_resource("r1");
  rdf_session->insert(r1, r1, r1);
  r_index any = nullptr;

  for (auto _ : state) {
    rdf_session->find_idx(r1, r1, r1);
    rdf_session->find_idx(r1, r1, any);
    rdf_session->find_idx(r1, any, r1);
    rdf_session->find_idx(r1, any, any);
    rdf_session->find_idx(any, r1, r1);
    rdf_session->find_idx(any, r1, any);
    rdf_session->find_idx(any, any, r1);
    rdf_session->find_idx(any, any, any);
  }

}
// Register the function as a benchmark
BENCHMARK(BM_Find_Index);

BENCHMARK_MAIN();