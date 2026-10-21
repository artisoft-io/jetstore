// Micro-benchmark of the RDF graph: insert, find by s / sp / spo / po, and iteration.
//
// Standalone: google benchmark is not a dependency of this repository, and this file is
// not in a CMake target. It uses only RDFGraph's public interface, so the same file
// compiles against any commit's headers, which is what comparing two commits needs.
// To measure a commit, copy this file into that commit's tree at jets/rdf/ (its includes
// are resolved beside it) and build it against that tree's Release build directory <B>:
//
//   g++ -std=gnu++20 -O3 -DNDEBUG jets/rdf/base_graph_benchmark.cc -o base_graph_benchmark \
//     <B>/jets/libjets_static.a -Wl,--start-group /usr/local/lib/libabsl_*.a -Wl,--end-group \
//     -lglog -lgflags -lboost_date_time -lsqlite3 -lpthread
//   taskset -c 0-5 ./base_graph_benchmark [repetitions]
//
// The graph's shape is a session's: S subjects ("claims"), each with P predicates; one
// object per (s, p) except a few multi-valued predicates, a typed subject per class,
// and objects shared across subjects (codes, dates). Each operation is timed over the
// whole graph; the output is one line per case: the median over repetitions of
// nanoseconds per operation, and the operation count.
#include <algorithm>
#include <chrono>
#include <cstdint>
#include <cstdio>
#include <cstdlib>
#include <string>
#include <vector>

#include "../rdf/rdf_types.h"

using namespace jets::rdf;

namespace {

struct Shape {
  int subjects;
  int predicates;         // single-valued predicates per subject
  int multi_predicates;   // multi-valued predicates per subject
  int multi_values;       // objects per multi-valued predicate
  int shared_objects;     // distinct objects shared across subjects
};

struct Fixture {
  RDFGraphPtr g;
  std::vector<r_index> s, p, o;
  r_index rdf_type{nullptr}, klass{nullptr};
};

// Deterministic: the same graph on every run and every commit
void build(Fixture & f, Shape const& sh)
{
  f.g = create_rdf_graph(nullptr, 2);
  auto m = f.g->get_rmgr();
  for(int i=0; i<sh.subjects; i++) f.s.push_back(m->create_resource("s" + std::to_string(i)));
  for(int i=0; i<sh.predicates + sh.multi_predicates; i++) f.p.push_back(m->create_resource("p" + std::to_string(i)));
  for(int i=0; i<sh.shared_objects; i++) f.o.push_back(m->create_literal(i));
  f.rdf_type = m->create_resource("rdf:type");
  f.klass = m->create_resource("Claim");
}

void insert_all(Fixture & f, Shape const& sh, std::uint64_t & ops)
{
  std::uint64_t x = 88172645463325252ull;
  auto rnd = [&x]() { x ^= x << 13; x ^= x >> 7; x ^= x << 17; return x; };
  for(int i=0; i<sh.subjects; i++) {
    f.g->insert(f.s[i], f.rdf_type, f.klass); ops++;
    for(int j=0; j<sh.predicates; j++) {
      f.g->insert(f.s[i], f.p[j], f.o[rnd() % f.o.size()]); ops++;
    }
    for(int j=0; j<sh.multi_predicates; j++) {
      for(int k=0; k<sh.multi_values; k++) {
        f.g->insert(f.s[i], f.p[sh.predicates+j], f.o[rnd() % f.o.size()]); ops++;
      }
    }
  }
}

using Clock = std::chrono::steady_clock;

template<typename F>
double time_ns(F && fn, std::uint64_t & ops)
{
  ops = 0;
  auto t0 = Clock::now();
  fn(ops);
  auto t1 = Clock::now();
  return std::chrono::duration<double, std::nano>(t1 - t0).count();
}

volatile std::uintptr_t sink = 0;

}  // namespace

int main(int argc, char ** argv)
{
  int reps = argc > 1 ? std::atoi(argv[1]) : 11;
  // About a 100- and a 1,000-claim session's asserted and inferred graph
  std::vector<std::pair<std::string, Shape>> shapes = {
    {"small",  {2000, 30, 3, 4, 5000}},
    {"large",  {20000, 30, 3, 4, 50000}},
  };
  std::printf("case\tshape\tns_per_op\tops\n");
  for(auto const& [name, sh]: shapes) {
    std::vector<std::string> cases = {"insert", "find_s", "find_sp", "find_spo_hit", "find_spo_miss",
      "find_po_class", "contains", "iterate_all", "retract_all", "destroy"};
    std::vector<std::vector<double>> ns(cases.size());
    std::vector<std::uint64_t> nops(cases.size());
    for(int r=0; r<reps; r++) {
      Fixture f;
      build(f, sh);
      std::uint64_t ops;
      size_t c = 0;
      // insert
      ns[c].push_back(time_ns([&](std::uint64_t & n){ insert_all(f, sh, n); }, ops) / ops); nops[c++] = ops;
      // find(s): every triple of every subject
      ns[c].push_back(time_ns([&](std::uint64_t & n){
        for(auto s: f.s) { auto it = f.g->find(s); while(not it.is_end()) { sink += (std::uintptr_t)it.get_object(); it.next(); } n++; }
      }, ops) / ops); nops[c++] = ops;
      // find(s, p)
      ns[c].push_back(time_ns([&](std::uint64_t & n){
        for(auto s: f.s) for(auto p: f.p) { auto it = f.g->find(s, p); while(not it.is_end()) { sink += (std::uintptr_t)it.get_object(); it.next(); } n++; }
      }, ops) / ops); nops[c++] = ops;
      // find(s, p, o), present: what a fully bound antecedent does
      ns[c].push_back(time_ns([&](std::uint64_t & n){
        for(auto s: f.s) { auto it = f.g->find(s, f.rdf_type, f.klass); while(not it.is_end()) { sink += (std::uintptr_t)it.get_object(); it.next(); } n++; }
      }, ops) / ops); nops[c++] = ops;
      // find(s, p, o), absent
      ns[c].push_back(time_ns([&](std::uint64_t & n){
        for(auto s: f.s) { auto it = f.g->find(s, f.p[0], f.klass); while(not it.is_end()) { it.next(); } n++; }
      }, ops) / ops); nops[c++] = ops;
      // find(*, rdf:type, Claim): every entity of a class, through the pos index
      ns[c].push_back(time_ns([&](std::uint64_t & n){
        for(int k=0; k<10; k++) { auto it = f.g->find(make_any(), f.rdf_type, f.klass); while(not it.is_end()) { sink += (std::uintptr_t)it.get_subject(); it.next(); n++; } }
      }, ops) / ops); nops[c++] = ops;
      // contains(s, p, o)
      ns[c].push_back(time_ns([&](std::uint64_t & n){
        for(auto s: f.s) for(int j=0; j<sh.predicates; j++) { sink += f.g->contains(s, f.p[j], f.o[j % f.o.size()]); n++; }
      }, ops) / ops); nops[c++] = ops;
      // find(): every triple
      ns[c].push_back(time_ns([&](std::uint64_t & n){
        auto it = f.g->find(); while(not it.is_end()) { sink += (std::uintptr_t)it.get_predicate(); it.next(); n++; }
      }, ops) / ops); nops[c++] = ops;
      // retract every triple of half the subjects, then destroy the graph
      std::vector<Triple> half;
      for(size_t i=0; i<f.s.size(); i+=2) { auto it = f.g->find(f.s[i]); while(not it.is_end()) { half.push_back(it.as_triple()); it.next(); } }
      ns[c].push_back(time_ns([&](std::uint64_t & n){
        for(auto const& t: half) { f.g->retract(t.subject, t.predicate, t.object); n++; }
      }, ops) / ops); nops[c++] = ops;
      std::uint64_t size = f.g->size();
      ns[c].push_back(time_ns([&](std::uint64_t & n){ f.g.reset(); n = size; }, ops) / std::max<std::uint64_t>(ops, 1)); nops[c++] = ops;
    }
    for(size_t c=0; c<cases.size(); c++) {
      auto v = ns[c];
      std::sort(v.begin(), v.end());
      std::printf("%s\t%s\t%.2f\t%llu\n", cases[c].c_str(), name.c_str(), v[v.size()/2], (unsigned long long)nops[c]);
    }
  }
  return 0;
}
