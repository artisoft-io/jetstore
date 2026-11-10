---
title: "The retired Go engine"
description: "Why JetStore has one rule engine, what the Go engine did differently, and what was settled before it was retired."
kind: explanation
sidebar:
  order: 5
---

The history of JetStore's second rule engine, written while both engines ran, newest first. What it
says of the Go engine is history, and a Go file or test it names is at the tag `go-engine-final`, not
in the current tree. What it says of the C++ engine is current unless a section says otherwise.

## The Go engine is retired — decided 2026-10-22

A benchmark of both engines on a client's claim processes fixed and settled their differences
(the next section), and on 2026-10-22 it was decided to keep one engine, the C++ one, computing what
it computed. The last
`jets_ai` commit with the Go engine is tagged **`go-engine-final`**; `git checkout go-engine-final`
brings it back.

- **What stays of the Go side is the rule *model***, not an engine: `rete_meta_store_model.go`,
  `workspace_control.go` and `retraction_mode.go` in `jets/jetrules/rete` (`retraction_mode.go`
  deleted with the mode on 2026-10-24: see [Retraction and truth maintenance](/docs/developer/engine/rule-engine/retraction/)), and `ast.go` and
  `datetime_parser.go` in `jets/jetrules/rdf`. The compiler writes a workspace with them and the
  pipelines read their configuration through them.
- **A configuration that asks for the Go engine is refused**, with a message naming the
  retirement: `use_jet_rules_go: true` in a pipeline's or a bench spec's `jetrules_config`, and
  `use_jet_rules_go`, `compare_engines` or an engine of `go` in a rules unit test.
  `use_jet_rules_go: false` is accepted, so existing configurations need no change.
- **The shared tables are the C++ engine's specification.** Each of the tables the next section
  names lived in a Go test with a C++ copy; the C++ copies stay, in `jets/rete/expr_op_test.cc`:
  `DoubleToText.ShortestDigitsNeverAnExponent`, `ArithmeticTypes.TheSharedTypingTable`,
  `Comparisons.TheSharedTable` and `ArithmeticTypes.DivisionByZero`.
- **The compiler checks operator names against the C++ engine** (the last section, superseded):
  `jets/compilerv2/compiler/operators.txt` lists every operator `create_binary_expr` and
  `create_unary_expr` register, the compiler refuses a rule calling any other, and
  `jets/rete/operator_list_test.cc` keeps the list equal to `expr_operator_factory.h`. It also
  marks the operators a filter may not call because they write to the graph, which the Go
  engine's operator types used to tell the compiler.
- **What was still different at the retirement** (*Still different, recorded*, below) is now simply
  what the engine does.

## The engines' differences settled — 2026-10-22

The two engines were compared value by value, and each difference was decided on 2026-10-22.
This entry said what **both engines
did** from then until the Go engine's retirement, and since which pull request. Go: jetstore#2330,
#2331, #2334, #2336. C++: #2332, #2333, #2335, #2336. The end-to-end rule test is `jets_ws` `jet_rules/unit_tests/reentry_error_path/`.
The older entries below that these settle say so in place. **Shared tables** held both
engines to the same values: each table lived in a Go test, and a C++ test checks the same rows. The
C++ tests are now the specification (see the entry above).

What was settled is described on the pages it concerns: values, operators and lookups in
[Values and operators](/docs/developer/engine/rule-engine/values-and-operators/); a filter's re-entry, index removal and live iterators in
[The rete network at run time](/docs/developer/engine/rule-engine/rete-network/); and `Retract` in
[Retraction and truth maintenance](/docs/developer/engine/rule-engine/retraction/).

### Still different, recorded

Recorded, not changed. Since the Go engine's
retirement each is simply what the C++ engine does; what Go did is kept for the record.

- **A date against a datetime.** C++'s `EqVisitor` can call a date equal to a datetime (the date as
  its midnight). Go never did.
- **`numeric_cast` throws.** C++ mixed-integer arithmetic converts the right operand to the left's
  type with `boost::numeric_cast`, which throws out of range (int32 + a large int64, uint32 + -1).
  Go returned no result. *(Null since 2026-10-23.)*
- **`INT_MIN / -1`** traps in C++'s `DivVisitor`, for int32 and int64, and through `numeric_cast`
  in the mixed integer cases. Go gave the minimum int. *(Null since 2026-10-23.)*
- **28 near-power-of-ten double pairs.** Go reproduced glibc's `log10` and `pow`, and agreed with the
  compiled C++ comparison on 600,017 of 600,045 tested pairs. The 28 others lie within a few ulps of
  a power of ten, where glibc's `log` and Go's `math.Log` round differently. The Go port mirrored
  x86-64 glibc 2.39; another glibc, or an aarch64 build that fuses into an FMA, may differ again.
- **A negation's free variable.** Deleting one of several triples that match a negated
  antecedent with a free variable re-satisfies the negation (it did in both engines), although another
  still matches. With `not(?s blocked ?b)` and both `(s blocked b1)` and `(s blocked b2)`,
  retracting `(s blocked b1)` infers the consequent; asserting `(s blocked b2)` alone does not. It is
  an order dependence. Since the re-entry fix of 2026-10-22, a row inserted this way after a re-entry is rejected. The fix
  would change what the engine infers, so it waits for its own change.
- **`ClearPendingRows` during the filter replay.** A nested visit clears a child's pending
  rows during `TripleUpdatedForFilter`'s replay, so rows the replay inserted and has not yet passed
  on cannot reach that child's children. It predates the comparison and was read, not run, on Go; C++'s
  `rete_session.cc` reads as having the same shape, also not run.
- **Text plus a named resource.** C++ has no such case and throws (or gives null in a filter); Go
  concatenated the name. *(Null since 2026-10-23; the name is not concatenated.)*
- **`join_values` over a blank node** writes `bn(<k>)` on C++; Go wrote `BN<k>`.
- **Regex dialects.** C++'s `literal_regex` compiles ECMAScript, so a lookahead or a backreference
  compiles; Go's compiled RE2 and panicked on them.
- **`lookup` and `multi_lookup` on one table and key share a subject and a cache triple**, as
  they did on both engines. Whichever runs first in a session decides what the other returns. None of the
  four workspaces calls both on one table (counted 2026-10-22).

## The Go and C++ engines are independently maintained and they disagree — 2026-09-09

*History: written while both engines ran. The C++ facts in it stand; the Go side was settled on
2026-10-22 (above) or ended with the retirement.*

Found while adding `join_values` (`JoinValuesOp`, `expr_operator_math_join_values.go` at `go-engine-final`; the C++ half
is `JoinValuesVisitor`, `jets/rete/expr_op_arithmetics.h`). Every operator here has a C++ twin and
nothing checks that the two behave alike, so **a rule can compile against one engine and not the
other, or — worse — run on both and produce different values**. Three divergences were measured on
`sum_values` while writing its sibling. ~~None is fixed here; they~~ **They** are recorded so the
next operator author does not rediscover them.

**A fourth entry was added later the same day**, by the change that fixed 2, and it is a different
animal: **item 4 was not a divergence when it was found** — both engines had it — and it became one
because only C++ was fixed. It is filed here anyway because this is where a reader looking for
"why is this aggregate wrong" will be standing.

**Items 2 and 4 were fixed in the C++ engine on 2026-09-09**, and each entry says so in place
rather than being struck out, because what a fix leaves behind matters: 4 is fixed on one side
only, and both are reachable by far less of the rule corpus than they look — see the paragraph
after 4, which is the one to read first if you arrived here from a wrong aggregate in production.

**1. A config carrying `jets:value_property` alone works in C++ and silently yielded nothing in Go.**
`SumValuesVisitor` resolves `datap = (rhs, jets:value_property, ?)` and falls back to `rhs` itself
(`jets/rete/expr_op_arithmetics.h`); `SumValuesOp.InitializeOperator`
(`expr_operator_math_sum_values.go` at `go-engine-final`) only looked at `jets:value_property` when
`jets:entity_property` was present, and otherwise used `rhs` as the data property — so the config
resource was walked as if it were a property of the subject, matching nothing. The C++
`expr_op_specialty_test.cc` `SumValuesVisitor1` case is exactly this form, and it had no Go
counterpart. `join_values` accepted the form on **both** sides, deliberately.

**2. ~~C++ registers no truth-maintenance callback for the direct-property form.~~ FIXED in C++
2026-09-09.** `SumValuesVisitor::register_callback` returned 0 when the rhs carried no
`jets:value_property`, while `SumValuesOp.RegisterCallback` fell back to watching `objProperty`. So
`(?s sum_values someMultiValuedProperty)` was recomputed on change under the Go engine and was not
under the C++ one. `join_values` registered in both cases on both sides. The C++ side now goes
through `register_callbacks_for_aggregate` (`jets/rete/expr_op_arithmetics.h`), which carries that
fallback for every operator that accepts the form. `SumValuesRecomputesOnDirectMultiValuedProperty`
in `jets/rete/expr_op_specialty_test.cc` fails without it.

**3. ~~A double *literal* is quantised to 15 bits of mantissa by the Go engine and not by the C++ one.~~
SETTLED 2026-10-22 (jetstore#2331): `NewDoubleLiteral` stores the double given,
on both engines; see [Values and operators](/docs/developer/engine/rule-engine/values-and-operators/).** As it was:
`ResourceManager.NewDoubleLiteral` (`jets/jetrules/rdf/resource_manager.go` at `go-engine-final`) does
`big.NewFloat(x).SetPrec(15).Float64()` — 15 **bits**, not digits — so `0.891089` is stored as
`0.891082763671875`. Computed doubles do not take that path (`rdf.F`, `jets/jetrules/rdf/ast.go`,
stores the value as given), so the same number reaches the graph differently depending on whether a
rule wrote it down or worked it out. Measured 2026-09-09 by a rendering test that expected the two
engines to agree on `0.891089` and got `0.891083` from Go.

**4. No aggregate operator watched `jets:entity_property`, on either side — FIXED in C++
2026-09-09, and open in Go until its retirement.** An aggregate configured with `jets:entity_property` **and** `jets:value_property` walks
`(s, entityP, ?o).(?o, valueP, ?v)` and therefore depends on both properties, and every one of the
five operators registered its callback on the value property alone — `sum_values`, `min_of`,
`max_of`, `sorted_head` and `join_values`, in both engines. **Only a change to an already-linked
child was ever seen.** Linking a further child does not touch the value property, so an entity
materialised with its value and then attached — which is the ordinary order for an entity built by
rules — moves the aggregate without anything noticing. The C++ side now registers on both
(`register_callbacks_for_aggregate`, `jets/rete/expr_op_arithmetics.h`); Go's `MinMaxOp`, `SumValuesOp`
and `JoinValuesOp` never did.

**And a fifth thing, which is not a divergence and is the one that actually bites.** All of the
above is about `register_callback`, and **`register_callback` is never called for a consequent
expression.** `ReteSession::set_graph_callbacks` (`jets/rete/rete_session.cc`) registers a node
vertex's *antecedent* alpha node and its *filter* expression, and nothing else; Go's `ReteSession`
initialisation in `rete_session.go` at `go-engine-final` did the same and called `InitializeExpression` alone on a
consequent. So **an aggregate in a consequent has no truth maintenance** (it had none in either engine), whatever
its config, and the four fixes above cannot reach one.

That is not a hypothetical shape. **All 89 uses of these five operators across the four workspaces
then examined are in consequents; none is in a filter** (counted 2026-09-09). The visible case
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

**The consequence for anyone adding an operator, while there were two engines: write the tests in
pairs.** `expr_operator_math_join_values_test.go` at `go-engine-final` and the `JoinValues*` cases in
`jets/rete/expr_op_specialty_test.cc` were deliberately the same cases in the same order, and the
rendering test in the Go file existed only to pin agreement with the C++ `JoinTextVisitor`. The C++
cases are now `join_values`' specification. Go's `Node.String()` was **not** that rendering: it formatted an `LDate` with `%v` on the struct
(`{2025-06-15 00:00:00 +0000 UTC}`), where C++ emits ISO-8601. **Since 2026-10-22 (jetstore#2331,
#2333)** every operator that writes a value into text, `join_values`
included, shared one rendering on both engines (`rdf.TextOf` in Go), and a double in `join_values`
is no longer `%g` with six digits but the shortest round-trip digits. See [Values and operators](/docs/developer/engine/rule-engine/values-and-operators/).

**And write a rule as well as a pair of unit tests, because a visitor test cannot see the name.**
`join_values` shipped with matched unit tests on both engines and no `.jr` file anywhere used it, so
nothing exercised compile → metastore → execution — the leg on which an operator name is an opaque
string. `jets/jetrules/test_ws/jet_rules/test_join_values_main.jr` is that rule; it was run on Go by
`TestJoinValuesCompilesAndRuns` (`join_values_rule_test.go` at `go-engine-final`), which compiled a scratch copy of
`test_ws` with `workspace.CompileWorkspace`, asset validation skipped, and asserted the joined strings. **Two of the three config forms
are in it deliberately**: the `jets:entity_property` + `jets:value_property` form with an explicit
`jets:separator`, and the `jets:value_property`-alone form with the default — which is item 1 above,
the form `sum_values` got wrong in Go, so the rule is what said `join_values` did not.

## Operator names are not registered anywhere but the two factories — 2026-09-09

**Superseded 2026-10-22, with the Go engine's retirement.** The compiler now refuses an operator
the C++ engine does not register: the names are in `jets/compilerv2/compiler/operators.txt`, and
`jets/rete/operator_list_test.cc` checks them against `expr_operator_factory.h` in both directions.
The Go tests this entry names were deleted with the Go engine. `corpus_operators.txt` and
`OperatorFactoryTest.CorpusOperatorNames` were deleted on 2026-10-23: with
no Go half left to collect the names, the compiler's `operators.txt` and `operator_list_test.cc` are
what check them, and the manifest as last collected is at `go-engine-final`. The six names the
table below lists as one-sided are now a compile error, or, for `raise_exception`, an operator
like any other. The entry is kept as written.

Searched while adding `join_values`. A binary operator name reaches the engine as an opaque string:
the ANTLR lexer has no token for `sum_values` or any of its siblings
(`jets/compilerv2/parser/JetRuleLexer.tokens` carries only punctuation and structural keywords), the
compiler carries the name through as `ExpressionNode.Op` and writes it to `workspace.db` unexamined
(`workspace_db_helper.go`), and no validator whitelists it. So **adding an operator is exactly two
registrations** — a `case` in `CreateBinaryOperator` (`expr_operator_factory.go` at `go-engine-final`) and a line in
`create_binary_expr` (`jets/rete/expr_operator_factory.h`) — plus the implementation.

The cost of that design is that **an unknown operator is not a compile error**. It compiles, and
fails at rule-execution time: `create_binary_expr` throws, `CreateBinaryOperator` returns nil.

**That is now checked at test time instead, and the check is in two halves because no one process can
do both.** `operator_registration_test.go` at `go-engine-final` in this package compiles every rule set of every
workspace — 50 of them, from each `workspace_control.json`'s `rule_sets` rather than from a
`*_main.jr` glob, which finds 2 of the largest workspace's 34 — collects every `ExpressionNode.Op` and asserts each
one resolves in `CreateBinaryOperator` / `CreateUnaryOperator`. It writes the names it found to
`jets/rete/test_data/corpus_operators.txt`, and `OperatorFactoryTest.CorpusOperatorNames`
(`jets/rete/expr_operator_factory_test.cc`) asserts the same list against `create_binary_expr` /
`create_unary_expr` — **the C++ engine cannot compile a `.jr` file, so it cannot collect the names
for itself**, and the manifest is the only thing keeping the two halves looking at the same list. It
is checked in and the Go test fails when it drifts. *(Both halves are gone since 2026-10-23, the
manifest and its C++ test kept at `go-engine-final`; the names are checked by `operators.txt` and
`operator_list_test.cc`, as the note above says.)*

**And the *two registrations* claim is itself now a test, which is how the next entry got measured.**
`TestBothFactoriesRegisterTheSameOperators` reads both factories as text — `case "name":` and
`if(op == "name")`, C++ comments stripped so the two commented-out `to_type_of` / `cast_to` lines do
not count — and fails when a name is in one and not the other.

**Six names are, and none of them is in the corpus, which is the only reason nobody has paid for
it.** Measured 2026-09-09 and listed in `knownFactoryDivergences` so a seventh fails the test rather
than joining them:

| Name | | |
|---|---|---|
| `apply_regex` | binary | Go only, an alias for `literal_regex`; C++ has `literal_regex` alone. Since 2026-10-22 (jetstore#2331) both names mean what C++'s does: capture group 1, or null |
| `min_head_of`, `max_head_of` | binary | Go only, `NewMinMaxOp(_, true)`; C++ has no head form |
| `to_date`, `to_datetime` | unary | Go only; C++ has `to_timestamp` and neither of these |
| `raise_exception` | unary | C++ only, `RaiseExceptionVisitor`; the Go engine has no counterpart |

**Read that as five ways to write a rule that runs on the Go engine and throws on the deployed one**,
and one the other way round. `cpipes_native_server` is what ships (`dockerfiles/Dockerfile.cpipes` copies it) and its
default factory is `jetrules_native_adaptor`, so the Go-only column is the dangerous one — a rule
using `to_date` passes every test a developer runs with `use_jet_rules_go` and fails in production.
