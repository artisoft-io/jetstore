// Micro-benchmark of comparing doubles, and an integer with a double, as rules do.
//
// Standalone, like base_graph_benchmark.cc: not in a CMake target, and using only what any
// commit has (rdf::is_eq, is_lt and the rete's EqVisitor and LtVisitor), so the same file
// compares two commits. Copy it into the tree to measure at jets/rdf/, and build it against
// that tree's Release build directory <B>:
//
//   g++ -std=gnu++20 -O3 -DNDEBUG jets/rdf/compare_benchmark.cc -o compare_benchmark \
//     <B>/jets/libjets_static.a -Wl,--start-group /usr/local/lib/libabsl_*.a -Wl,--end-group \
//     -lglog -lgflags -lboost_date_time -lsqlite3 -lpthread
//   taskset -c 0-5 ./compare_benchmark [repetitions]
//
// One line per case: the median over repetitions of nanoseconds per comparison.
#include <algorithm>
#include <chrono>
#include <cstdio>
#include <cstdlib>
#include <functional>
#include <random>
#include <string>
#include <vector>

#include "../rdf/rdf_types.h"
#include "../rete/rete_types.h"
#include "../rete/expr_op_logics.h"

using namespace jets;

namespace {
volatile int sink = 0;

template<class F>
double ns_per(F && f, size_t n)
{
  auto t0 = std::chrono::steady_clock::now();
  for(size_t i=0; i<n; i++) sink += f(i);
  auto t1 = std::chrono::steady_clock::now();
  return std::chrono::duration<double, std::nano>(t1 - t0).count() / n;
}
}  // namespace

int main(int argc, char ** argv)
{
  int reps = argc > 1 ? std::atoi(argv[1]) : 11;
  size_t const n = 1 << 20;
  std::mt19937_64 rng(7);
  std::uniform_real_distribution<double> u(0, 1000);
  // far apart (most comparisons in rules), and equal to 15 digits but not bit-equal
  std::vector<rdf::RdfAstType> far_a, far_b, near_a, near_b, ints;
  for(size_t i=0; i<n; i++) {
    double a = u(rng);
    far_a.push_back(rdf::LDouble(a));
    far_b.push_back(rdf::LDouble(u(rng)));
    near_a.push_back(rdf::LDouble(a));
    near_b.push_back(rdf::LDouble(a * (1 + 1e-16)));
    ints.push_back(rdf::LInt32(static_cast<int32_t>(u(rng))));
  }
  rete::EqVisitor eq(nullptr, nullptr);
  rete::LtVisitor lt(nullptr, nullptr);
  auto d = [](rdf::RdfAstType const& v) { return boost::get<rdf::LDouble>(v); };
  std::vector<std::pair<std::string, std::function<int(size_t)>>> cases = {
    {"is_eq_far",        [&](size_t i) { return int(rdf::is_eq(d(far_a[i]), d(far_b[i]))); }},
    {"is_lt_far",        [&](size_t i) { return int(rdf::is_lt(d(far_a[i]), d(far_b[i]))); }},
    {"is_eq_near",       [&](size_t i) { return int(rdf::is_eq(d(near_a[i]), d(near_b[i]))); }},
    {"eq_int_double",    [&](size_t i) { return int(rdf::to_bool(boost::apply_visitor(eq, ints[i], far_b[i]))); }},
    {"lt_int_double",    [&](size_t i) { return int(rdf::to_bool(boost::apply_visitor(lt, ints[i], far_b[i]))); }},
    {"eq_double_double", [&](size_t i) { return int(rdf::to_bool(boost::apply_visitor(eq, far_a[i], far_b[i]))); }},
  };
  std::printf("case\tns_per_op\n");
  for(auto & [name, f]: cases) {
    std::vector<double> v;
    for(int r=0; r<reps; r++) v.push_back(ns_per(f, n));
    std::sort(v.begin(), v.end());
    std::printf("%s\t%.2f\n", name.c_str(), v[v.size()/2]);
  }
  return 0;
}
