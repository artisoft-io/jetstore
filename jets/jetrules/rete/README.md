# `jets/jetrules/rete` — package notes

What you cannot see from the code in front of you. Newest first.

## The Go and C++ engines are independently maintained and they disagree — 2026-09-09

Found while adding `join_values` (`JoinValuesOp`, `expr_operator_math_join_values.go`; the C++ half
is `JoinValuesVisitor`, `jets/rete/expr_op_arithmetics.h`). Every operator here has a C++ twin and
nothing checks that the two behave alike, so **a rule can compile against one engine and not the
other, or — worse — run on both and produce different values**. Three divergences were measured on
`sum_values` while writing its sibling. None is fixed here; they are recorded so the next operator
author does not rediscover them.

**1. A config carrying `jets:value_property` alone works in C++ and silently yields nothing in Go.**
`SumValuesVisitor` resolves `datap = (rhs, jets:value_property, ?)` and falls back to `rhs` itself
(`jets/rete/expr_op_arithmetics.h`); `SumValuesOp.InitializeOperator`
(`expr_operator_math_sum_values.go:20`) only looks at `jets:value_property` when
`jets:entity_property` is present, and otherwise uses `rhs` as the data property — so the config
resource is walked as if it were a property of the subject, matching nothing. The C++
`expr_op_specialty_test.cc` `SumValuesVisitor1` case is exactly this form, and it has no Go
counterpart. `join_values` accepts the form on **both** sides, deliberately.

**2. C++ registers no truth-maintenance callback for the direct-property form.**
`SumValuesVisitor::register_callback` returns 0 when the rhs carries no `jets:value_property`,
while `SumValuesOp.RegisterCallback` falls back to watching `objProperty`. So
`(?s sum_values someMultiValuedProperty)` is recomputed on change under the Go engine and is not
under the C++ one. `join_values` registers in both cases on both sides.

**3. A double *literal* is quantised to 15 bits of mantissa by the Go engine and not by the C++ one.**
`ResourceManager.NewDoubleLiteral` (`jets/jetrules/rdf/resource_manager.go:287`) does
`big.NewFloat(x).SetPrec(15).Float64()` — 15 **bits**, not digits — so `0.891089` is stored as
`0.891082763671875`. Computed doubles do not take that path (`rdf.F`, `jets/jetrules/rdf/ast.go:367`,
stores the value as given), so the same number reaches the graph differently depending on whether a
rule wrote it down or worked it out. Measured 2026-09-09 by a rendering test that expected the two
engines to agree on `0.891089` and got `0.891083` from Go.

**The consequence for anyone adding an operator: write the tests in pairs.**
`expr_operator_math_join_values_test.go` and the `JoinValues*` cases in
`jets/rete/expr_op_specialty_test.cc` are deliberately the same cases in the same order, and the
rendering test in the Go file exists only to pin agreement with the C++ `JoinTextVisitor`.
`Node.String()` is **not** that rendering: it formats an `LDate` with `%v` on the struct
(`{2025-06-15 00:00:00 +0000 UTC}`), where C++ emits ISO-8601.

## Operator names are not registered anywhere but the two factories — 2026-09-09

Searched while adding `join_values`. A binary operator name reaches the engine as an opaque string:
the ANTLR lexer has no token for `sum_values` or any of its siblings
(`jets/compilerv2/parser/JetRuleLexer.tokens` carries only punctuation and structural keywords), the
compiler carries the name through as `ExpressionNode.Op` and writes it to `workspace.db` unexamined
(`workspace_db_helper.go`), and no validator whitelists it. So **adding an operator is exactly two
registrations** — a `case` in `CreateBinaryOperator` (`expr_operator_factory.go`) and a line in
`create_binary_expr` (`jets/rete/expr_operator_factory.h`) — plus the implementation.

The cost of that design is that **an unknown operator is not a compile error**. It compiles, and
fails at rule-execution time: `create_binary_expr` throws, `CreateBinaryOperator` returns nil.
