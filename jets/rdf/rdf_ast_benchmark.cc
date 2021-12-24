#include <benchmark/benchmark.h>
#include "boost/variant.hpp"

#include "jets/rdf/rdf_types.h"
#include "rdf_ast.h"

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

static void BM_To_BOOLOp(benchmark::State& state) {

  auto banane_s = jets::rdf::mkResource("banane");
  auto false_s  = jets::rdf::mkLiteral("false");
  auto f0lse_s  = jets::rdf::mkLiteral("f0lse");
  auto FALSE_s  = jets::rdf::mkLiteral("FALSE");
  auto TRUE_s  = jets::rdf::mkLiteral("TRUE");
  auto f_s  =    jets::rdf::mkLiteral("f");
  auto T_s  =    jets::rdf::mkLiteral("T");
  auto zero_s  = jets::rdf::mkLiteral("0");
  auto one_s  = jets::rdf::mkLiteral("1");


  int k=0;
  for (auto _ : state) {
    auto b0 = jets::rdf::to_bool(banane_s.get());
    auto b1 = jets::rdf::to_bool(false_s.get());
    auto b2 = jets::rdf::to_bool(f0lse_s.get());
    auto b3 = jets::rdf::to_bool(FALSE_s.get());
    auto b4 = jets::rdf::to_bool(TRUE_s.get());
    auto b5 = jets::rdf::to_bool(f_s.get());
    auto b6 = jets::rdf::to_bool(T_s.get());
    auto b7 = jets::rdf::to_bool(zero_s.get());
    auto b8 = jets::rdf::to_bool(one_s.get());
    k += b0 or b1 or b2 or b3 or b4 or b5 or b6 or b7 or b8;
  }

}
// Register the function as a benchmark
BENCHMARK(BM_To_BOOLOp);

BENCHMARK_MAIN();