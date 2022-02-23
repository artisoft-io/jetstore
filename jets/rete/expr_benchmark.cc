#include <benchmark/benchmark.h>
#include "boost/variant.hpp"

#include "jets/rdf/rdf_types.h"
#include "jets/rete/rete_types.h"
#include "jets/rete/rete_types_impl.h"

using namespace jets::rdf;
using namespace jets::rete;

static void BM_expr(benchmark::State& state) {
  ExprDisjunction::data_type data;
  data.push_back(
    create_expr_binary_operator<EqVisitor>(0, 
      create_expr_binary_operator<RegexVisitor>(0, 
        create_expr_cst(RdfAstType(LString("(\\d)"))), 
        create_expr_binary_operator<AddVisitor>(0, 
          create_expr_cst(RdfAstType(LString("Hello "))),
          create_expr_binary_operator<AddVisitor>(0, 
            create_expr_cst(RdfAstType(LInt32(1))), 
            create_expr_cst(RdfAstType(LUInt64(2)))
          )
        )
      ), 
      create_expr_cst(RdfAstType(LString("1")))
    )
  );
  // The next would throw since Regex is used on an int argument
  // but this test return false at first term since it's a conjunction
  data.push_back(
    create_expr_binary_operator<EqVisitor>(0, 
      create_expr_binary_operator<RegexVisitor>(0, 
        create_expr_cst(RdfAstType(LString("(\\d)"))), 
        create_expr_binary_operator<AddVisitor>(0, 
          create_expr_cst(RdfAstType(LString("Hello "))),
          create_expr_binary_operator<AddVisitor>(0, 
            create_expr_cst(RdfAstType(LInt32(1))), 
            create_expr_cst(RdfAstType(LUInt64(2)))
          )
        )
      ), 
      create_expr_cst(RdfAstType(LString("2")))
  ));
  data.push_back(
    create_expr_binary_operator<EqVisitor>(0, 
      create_expr_binary_operator<RegexVisitor>(0, 
        create_expr_cst(RdfAstType(LString("(\\d)"))), 
        create_expr_binary_operator<AddVisitor>(0, 
          create_expr_cst(RdfAstType(LString("Hello "))),
          create_expr_binary_operator<AddVisitor>(0, 
            create_expr_cst(RdfAstType(LInt32(1))), 
            create_expr_cst(RdfAstType(LUInt64(2)))
          )
        )
      ), 
      create_expr_cst(RdfAstType(LString("3")))
  ));
  auto expr = create_expr_disjunction(data);

  for (auto _ : state) {
    if(not to_bool(expr->eval(nullptr, nullptr))) std::cout<<"ERROR!!!!"<<std::endl;
  }

}
// Register the function as a benchmark
BENCHMARK(BM_expr);

BENCHMARK_MAIN();