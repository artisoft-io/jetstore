// Micro-benchmark: building and releasing a session-like population of beta rows,
// on the global heap with every relation destroyed (how a ReteSession was released
// before its BetaRowArena) and in an arena released in bulk (how it is released now).
//
// Not part of the CMake build, like beta_row_benchmark.cc. Self-contained (no
// benchmark library). From the repository root, with a Release build in build/:
//
//   g++ -O3 -DNDEBUG -std=gnu++20 -Ijets jets/rete/beta_row_teardown_benchmark.cc \
//     -Lbuild/jets -ljets_static -lglog -lgflags -labsl_hash -labsl_city \
//     -labsl_raw_hash_set -lboost_date_time -lsqlite3 -lpthread -o /tmp/beta_row_teardown_benchmark
//   /tmp/beta_row_teardown_benchmark [rows=1000000] [relations=200] [repetitions=5]
//
// Per repetition and arm it prints the time to build the population and the time to
// release it, in ms and in ns per row. The population mimics a large session: rows of
// 6 values with a tid past the short-string limit, held by their relation's row set,
// half of them in its pending list, and indexed by one to three child indexes.
#include <chrono>
#include <cstdio>
#include <cstdlib>
#include <memory>
#include <vector>

#include "../rete/beta_relation.h"

namespace {
using namespace jets::rete;
using Clock = std::chrono::steady_clock;

// Stand-in values: rows only store and hash them
inline jets::rdf::r_index
value(std::size_t i)
{
  return reinterpret_cast<jets::rdf::r_index>(static_cast<std::uintptr_t>(16 * (i + 1)));
}

// What BetaRelation's containers hold, built directly so no ReteSession is needed
struct Relation {
  explicit Relation(BetaRowArena * arena)
    : rows(ArenaAllocator<BetaRowPtr>(arena)),
      pending(ArenaAllocator<BetaRowPtr>(arena)),
      idx1(BetaRowIndxVec1::allocator_type(ArenaAllocator<BetaRowIndexes1>(arena))),
      idx2(BetaRowIndxVec2::allocator_type(ArenaAllocator<BetaRowIndexes2>(arena))),
      idx3(BetaRowIndxVec3::allocator_type(ArenaAllocator<BetaRowIndexes3>(arena)))
  {
    idx1.push_back({});
    idx2.push_back({});
    idx3.push_back({});
  }
  beta_row_set    rows;
  beta_row_list   pending;
  BetaRowIndxVec1 idx1;
  BetaRowIndxVec2 idx2;
  BetaRowIndxVec3 idx3;
};

// One node vertex for every row, and a parent row three vertices deep: a row
// initialized from it gets a tid of 21 characters, past the short-string limit, as the
// rows of a deep rete network do
struct Prototype {
  Prototype()
    : ri(create_row_initializer(6)),
      nv(create_node_vertex(nullptr, 0, 1000, false, 10, {}, "", ri)),
      parent(create_beta_row(nv.get(), 6))
  {
    for(int k=0; k<6; k++) {
      ri->put(k, k | brc_parent_node, "");
      parent->put(k, value(k));
    }
    for(int depth=0; depth<3; depth++) {
      auto row = create_beta_row(nv.get(), 6);
      row->initialize(ri.get(), parent.get(), nullptr);
      parent = row;
    }
  }
  BetaRowInitializerPtr ri;
  NodeVertexPtr nv;
  BetaRowPtr parent;
};

void
populate(std::vector<Relation*> const& relations, BetaRowArena * arena, std::size_t nbr_rows)
{
  static Prototype const proto;
  std::size_t const nrel = relations.size();
  b_index nv = proto.nv.get();
  for(std::size_t i=0; i<nbr_rows; i++) {
    Relation * r = relations[i % nrel];
    auto row = arena ? create_beta_row(arena, nv, 6) : create_beta_row(nv, 6);
    row->initialize(proto.ri.get(), proto.parent.get(), nullptr);
    for(int k=0; k<6; k++) row->put(k, value(i * 7 + k));
    r->rows.insert(row);
    if(i % 2 == 0) r->pending.push_back(row);
    r->idx1[0].insert({value(i % 1000), row.get()});
    if(i % 3 != 0) r->idx2[0].insert({{value(i % 1000), value(i % 13)}, row.get()});
    if(i % 3 == 2) r->idx3[0].insert({{value(i % 1000), value(i % 13), value(i % 5)}, row.get()});
  }
}

double
ms(Clock::time_point a, Clock::time_point b)
{
  return std::chrono::duration<double, std::milli>(b - a).count();
}

void
run(bool use_arena, std::size_t nbr_rows, std::size_t nbr_relations, int rep)
{
  auto t0 = Clock::now();
  auto arena = use_arena ? std::make_unique<BetaRowArena>() : nullptr;
  std::vector<Relation*> relations;
  for(std::size_t i=0; i<nbr_relations; i++) {
    if(arena) {
      relations.push_back(::new (arena->allocate(sizeof(Relation))) Relation(arena.get()));
    } else {
      relations.push_back(new Relation(nullptr));
    }
  }
  populate(relations, arena.get(), nbr_rows);
  auto t1 = Clock::now();
  if(arena) {
    arena.reset();                      // the relations are never destroyed
  } else {
    for(auto * r: relations) delete r;
  }
  auto t2 = Clock::now();
  std::printf("%-5s rep %d rows %zu relations %zu: build %9.1f ms (%6.1f ns/row)  release %9.1f ms (%6.1f ns/row)\n",
    use_arena ? "arena" : "heap", rep, nbr_rows, nbr_relations,
    ms(t0, t1), 1e6 * ms(t0, t1) / nbr_rows, ms(t1, t2), 1e6 * ms(t1, t2) / nbr_rows);
}
}  // namespace

int
main(int argc, char ** argv)
{
  std::size_t nbr_rows = argc > 1 ? std::strtoull(argv[1], nullptr, 10) : 1000000;
  std::size_t nbr_relations = argc > 2 ? std::strtoull(argv[2], nullptr, 10) : 200;
  int repetitions = argc > 3 ? std::atoi(argv[3]) : 5;
  for(int rep=0; rep<repetitions; rep++) {
    // interleaved, heap first then arena, so drift hits both arms
    run(false, nbr_rows, nbr_relations, rep);
    run(true, nbr_rows, nbr_relations, rep);
  }
  return 0;
}
