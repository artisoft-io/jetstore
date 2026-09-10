# `jets/jetrules/rete` — package notes

What you cannot see from the code in front of you. Newest first.

## `_0:no_truth_main_on_exist` — a production flag, and where its test is — 2026-09-09

C++ only; there is no Go counterpart, so a rule set that needs it cannot run on the Go engine at all.
`workspaces/usi_ws` sets it in `pipes_config/usiclaim_processes.pc.json` because its rules predate
rule filters participating in truth maintenance, and it makes `ExistVisitor::register_callback` and
`ExistNotVisitor::register_callback` (`jets/rete/expr_op_resources.h`) register nothing.

**The two directions fail differently, which is worth knowing before reading a wrong result.** With
the flag on, an `exist` filter that becomes true is never noticed, and an `exist_not` filter that
becomes false keeps whatever it inferred — the second is a triple that should have been retracted and
was not. `NoTruthMainOnExistTest` (`jets/rete/no_truth_main_on_exist_test.cc`) pins both, and the
default: **absent means truth maintenance ON**, and so does a value that is present and is not an
int32, which is how a workspace writing `"1"` as text gets the opposite of what it asked for with
nothing logged.

## The Go and C++ engines are independently maintained and they disagree — 2026-09-09

Found while adding `join_values` (`JoinValuesOp`, `expr_operator_math_join_values.go`; the C++ half
is `JoinValuesVisitor`, `jets/rete/expr_op_arithmetics.h`). Every operator here has a C++ twin and
nothing checks that the two behave alike, so **a rule can compile against one engine and not the
other, or — worse — run on both and produce different values**. Three divergences were measured on
`sum_values` while writing its sibling. ~~None is fixed here; they~~ **They** are recorded so the
next operator author does not rediscover them.

**A fourth entry was added later the same day**, by the session that fixed 2, and it is a different
animal: **item 4 was not a divergence when it was found** — both engines had it — and it became one
because only C++ was fixed. It is filed here anyway because this is where a reader looking for
"why is this aggregate wrong" will be standing.

**Items 2 and 4 were fixed in the C++ engine on 2026-09-09**, and each entry says so in place
rather than being struck out, because what a fix leaves behind matters: 4 is fixed on one side
only, and both are reachable by far less of the rule corpus than they look — see the paragraph
after 4, which is the one to read first if you arrived here from a wrong aggregate in production.

**1. A config carrying `jets:value_property` alone works in C++ and silently yields nothing in Go.**
`SumValuesVisitor` resolves `datap = (rhs, jets:value_property, ?)` and falls back to `rhs` itself
(`jets/rete/expr_op_arithmetics.h`); `SumValuesOp.InitializeOperator`
(`expr_operator_math_sum_values.go:20`) only looks at `jets:value_property` when
`jets:entity_property` is present, and otherwise uses `rhs` as the data property — so the config
resource is walked as if it were a property of the subject, matching nothing. The C++
`expr_op_specialty_test.cc` `SumValuesVisitor1` case is exactly this form, and it has no Go
counterpart. `join_values` accepts the form on **both** sides, deliberately.

**2. ~~C++ registers no truth-maintenance callback for the direct-property form.~~ FIXED in C++
2026-09-09.** `SumValuesVisitor::register_callback` returned 0 when the rhs carried no
`jets:value_property`, while `SumValuesOp.RegisterCallback` falls back to watching `objProperty`. So
`(?s sum_values someMultiValuedProperty)` was recomputed on change under the Go engine and was not
under the C++ one. `join_values` registered in both cases on both sides. The C++ side now goes
through `register_callbacks_for_aggregate` (`jets/rete/expr_op_arithmetics.h`), which carries that
fallback for every operator that accepts the form. `SumValuesRecomputesOnDirectMultiValuedProperty`
in `jets/rete/expr_op_specialty_test.cc` fails without it.

**3. A double *literal* is quantised to 15 bits of mantissa by the Go engine and not by the C++ one.**
`ResourceManager.NewDoubleLiteral` (`jets/jetrules/rdf/resource_manager.go:287`) does
`big.NewFloat(x).SetPrec(15).Float64()` — 15 **bits**, not digits — so `0.891089` is stored as
`0.891082763671875`. Computed doubles do not take that path (`rdf.F`, `jets/jetrules/rdf/ast.go:367`,
stores the value as given), so the same number reaches the graph differently depending on whether a
rule wrote it down or worked it out. Measured 2026-09-09 by a rendering test that expected the two
engines to agree on `0.891089` and got `0.891083` from Go.

**4. No aggregate operator watched `jets:entity_property`, on either side — FIXED in C++
2026-09-09, and still open in Go.** An aggregate configured with `jets:entity_property` **and** `jets:value_property` walks
`(s, entityP, ?o).(?o, valueP, ?v)` and therefore depends on both properties, and every one of the
five operators registered its callback on the value property alone — `sum_values`, `min_of`,
`max_of`, `sorted_head` and `join_values`, in both engines. **Only a change to an already-linked
child was ever seen.** Linking a further child does not touch the value property, so an entity
materialised with its value and then attached — which is the ordinary order for an entity built by
rules — moves the aggregate without anything noticing. The C++ side now registers on both
(`register_callbacks_for_aggregate`, `jets/rete/expr_op_arithmetics.h`); `MinMaxOp`, `SumValuesOp`
and `JoinValuesOp` in this package still do not.

**And a fifth thing, which is not a divergence and is the one that actually bites.** All of the
above is about `register_callback`, and **`register_callback` is never called for a consequent
expression.** `ReteSession::set_graph_callbacks` (`jets/rete/rete_session.cc`) registers a node
vertex's *antecedent* alpha node and its *filter* expression, and nothing else; `ReteSession`'s
initialisation in `rete_session.go` does the same and calls `InitializeExpression` alone on a
consequent. So **an aggregate in a consequent has no truth maintenance in either engine**, whatever
its config, and the four fixes above cannot reach one.

That is not a hypothetical shape. **All 89 uses of these five operators across the four workspaces
under `workspaces/` are in consequents; none is in a filter** (counted 2026-09-09). The visible case
is `CE_RxDateRange10` (`workspaces/jets_ws/jet_rules/clinical_intel/common_events.jr`), whose
antecedent `(?entity tag hasPharmacyClaim)` is a *constant* triple that `AM_PCreateEvent40`
(`analysis_pharmacy_rules.jr`) asserts alongside the first claim it links. The rule term is created
once, at that first claim, and its `min_of`/`max_of`/`sum_values` consequents are computed over
whatever is linked when the row is drained from the consequent queue — which for equal salience is
heap order. A three-fill event aggregating exactly one claim is that, not any of the four
divergences above. `AggregateInConsequentIsNotMaintained` in
`jets/rete/expr_op_specialty_test.cc` pins the behaviour so that a reader who fixes
`register_callback` and expects that rule to change is told otherwise by a test rather than by a
production number.

**The consequence for anyone adding an operator: write the tests in pairs.**
`expr_operator_math_join_values_test.go` and the `JoinValues*` cases in
`jets/rete/expr_op_specialty_test.cc` are deliberately the same cases in the same order, and the
rendering test in the Go file exists only to pin agreement with the C++ `JoinTextVisitor`.
`Node.String()` is **not** that rendering: it formats an `LDate` with `%v` on the struct
(`{2025-06-15 00:00:00 +0000 UTC}`), where C++ emits ISO-8601.

**And write a rule as well as a pair of unit tests, because a visitor test cannot see the name.**
`join_values` shipped with matched unit tests on both engines and no `.jr` file anywhere used it, so
nothing exercised compile → metastore → execution — the leg on which an operator name is an opaque
string. `jets/jetrules/test_ws/jet_rules/test_join_values_main.jr` is that rule, run by
`TestJoinValuesCompilesAndRuns` (`join_values_rule_test.go`), which compiles a scratch copy of
`test_ws` with `compile_workspace` and asserts the joined strings. **Two of the three config forms
are in it deliberately**: the `jets:entity_property` + `jets:value_property` form with an explicit
`jets:separator`, and the `jets:value_property`-alone form with the default — which is item 1 above,
the form `sum_values` gets wrong in Go, so the rule is what says `join_values` does not.

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

**That is now checked at test time instead, and the check is in two halves because no one process can
do both.** `operator_registration_test.go` in this package compiles every rule set of every
workspace — 50 of them, from each `workspace_control.json`'s `rule_sets` rather than from a
`*_main.jr` glob, which finds 2 of usi_ws's 34 — collects every `ExpressionNode.Op` and asserts each
one resolves in `CreateBinaryOperator` / `CreateUnaryOperator`. It writes the names it found to
`jets/rete/test_data/corpus_operators.txt`, and `OperatorFactoryTest.CorpusOperatorNames`
(`jets/rete/expr_operator_factory_test.cc`) asserts the same list against `create_binary_expr` /
`create_unary_expr` — **the C++ engine cannot compile a `.jr` file, so it cannot collect the names
for itself**, and the manifest is the only thing keeping the two halves looking at the same list. It
is checked in and the Go test fails when it drifts.

**And the *two registrations* claim is itself now a test, which is how the next entry got measured.**
`TestBothFactoriesRegisterTheSameOperators` reads both factories as text — `case "name":` and
`if(op == "name")`, C++ comments stripped so the two commented-out `to_type_of` / `cast_to` lines do
not count — and fails when a name is in one and not the other.

**Six names are, and none of them is in the corpus, which is the only reason nobody has paid for
it.** Measured 2026-09-09 and listed in `knownFactoryDivergences` so a seventh fails the test rather
than joining them:

| Name | | |
|---|---|---|
| `apply_regex` | binary | Go only, an alias for `literal_regex`; C++ has `literal_regex` alone |
| `min_head_of`, `max_head_of` | binary | Go only, `NewMinMaxOp(_, true)`; C++ has no head form |
| `to_date`, `to_datetime` | unary | Go only; C++ has `to_timestamp` and neither of these |
| `raise_exception` | unary | C++ only, `RaiseExceptionVisitor`; the Go engine has no counterpart |

**Read that as five ways to write a rule that runs on the Go engine and throws on the deployed one**,
and one the other way round. `cpipes_native_server` is what ships (`Dockerfile.cpipes:46`) and its
default factory is `jetrules_native_adaptor`, so the Go-only column is the dangerous one — a rule
using `to_date` passes every test a developer runs with `use_jet_rules_go` and fails in production.
