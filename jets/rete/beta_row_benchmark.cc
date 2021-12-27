#include <benchmark/benchmark.h>

#include "jets/rdf/rdf_types.h"
#include "jets/rete/beta_row.h"

static void BM_SimpleBetaRow(benchmark::State& state) {

  // rdf resource manager
  jets::rdf::RManager<jets::rdf::LD2RIndexMap> rmanager;

  // subjects
  std::string s0("r0"), s1("r1"), s2("r2");
  auto r0 = rmanager.create_resource(s0);
  auto r1 = rmanager.create_resource(s1);
  auto r2 = rmanager.create_resource(s2);

  int row_size = 3;
  jets::rete::b_index_set set0;
  for (auto _ : state) {
    jets::rete::BetaRowInitializerPtr ri0 = jets::rete::create_row_initializer(row_size);
    jets::rete::NodeVertexPtr nv0 = jets::rete::create_node_vertex(nullptr, set0, 0, false, false, 0, 10, ri0, {});
    jets::rete::BetaRowPtr br0    = jets::rete::create_beta_row(nv0.get(), row_size);
    ri0->put(0, 0 | jets::rete::brc_parent_node);
    ri0->put(1, 0 | jets::rete::brc_triple);
    ri0->put(2, 1 | jets::rete::brc_parent_node);
    br0->put(0, r0);
    br0->put(1, r1);
    br0->put(2, r2);
  }
}
// Register the function as a benchmark
BENCHMARK(BM_SimpleBetaRow);

BENCHMARK_MAIN();