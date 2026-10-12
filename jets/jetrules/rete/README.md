# `jets/jetrules/rete` — package notes

What you cannot see from the code in front of you. Newest first.

## Every rule fires at the salience it declares — 2026-10-11

**A vertex has one salience, and rules with the same antecedents used to share their last vertex.**
So a vertex could carry rules with different saliences, and the two engines settled it differently:
- **Go** took the lowest (`slices.Min`, `NewReteMetaStoreFactory`, over the list in `.rete.json`).
- **C++** took the first rule's: it reads one number, `workspace.db`'s `rete_nodes.salience`, and
  the compiler wrote `Salience[0]` there.

Either way, **a declared salience could be overruled with no error.** Raising a rule's salience did
nothing if a rule with the same antecedents sat lower. Lowering an aggregate below its producer
dragged the producer down with it when the two shared antecedents. The compiler's aggregate salience
check compared declared saliences, so it missed both.

Measured on a three-rule case in the rules unit test harness: a producer declared at s=100 sharing
its antecedents with an s=5 rule, feeding an aggregate at s=50. It compiled. C++ ran the vertex at
100 and the aggregate counted the producer's value; Go ran it at 5 and counted nothing
(`jetstore_agentic_ai` `rules_unit_test_harness` `I-25`).

**Fixed in the compiler's network builder** (`terminalReteNode`,
`jets/compilerv2/compiler/jet_rule_listener_rete.go`): rules share their last vertex only when their
saliences are equal, and a rule with another salience gets a vertex of its own; the antecedents
before it are still shared. So every vertex fires at one salience, the declared one, on both
engines, and the aggregate check's declared saliences are the ones that run. The same three-rule
case now counts the value on both engines.

**One way to defeat a declared salience remains, and the compiler warns about it**
(`WarnSameConsequentAtTwoSaliences`, same file). Two rules with the same antecedents and different
saliences that infer the same triple: the triple exists from when the higher one fires, and the
lower one only adds a reference, so its salience changes nothing about that triple. That is what is
left when a rule was copied to change its salience and the original kept. It is a warning, not an
error, because the pair can differ in other consequents. ~~A warning reaches the process log and
not the compile output a workspace compile returns on success.~~ **Since 2026-10-12 a successful
compile's output carries its warnings** under *Compilation warnings:* (`compileWarnings`,
`jets/workspace/compile_workspace_v2.go`), so View Last Log in the Workspace IDE shows them; until
then only the warnings the compiler also writes to the parse log reached it.

**Nothing in production moved**, recompiled 2026-10-11 against the previous compiler:
- **`usi_ws`, `walrus_ws` and `cedargate_ws`** at the pins `jetstore_agentic_ai` records compile to
  identical networks, node for node (11,363 rete nodes in `usi_ws`).
- **USI's and Walrus's own copies** have no vertex carrying two saliences.
- **`jets_ws` gains one vertex**: the generated class-inheritance rule
  `ci_cintel:Briefing_Pharmacy_Event` stops sharing `BP_RxFillDates10`'s vertex and fires at its own
  s=100 instead of 10. It infers only `rdf:type owl:Thing`, which no rule reads.

## Text read as a bool: one specification on both engines — 2026-10-11

**Until 2026-10-11 the two engines read a string as a bool the opposite way round.** C++
(`rdf::to_bool`) took any text but a few spellings of false as true; Go (`rdf.ParseBool` for
literals, `Node.Bool` for a filter) took any text but a few spellings of true as false, and
`Node.Bool` called `"false"` true. Rule configuration read the first letter, so `tomato` was true.
None of the four workspaces reached the difference on that day: no rule file wrote `bool("…")` or
`bool x = "…";`, and no filter tested a bare variable. This is `rules_unit_test_harness` `I-45`.

**Now one parser, `rdf.ParseBool` in Go and `rdf::parse_bool` in C++, on the same sets**: trimmed,
in any case, `true, t, yes, y, 1` and `false, f, no, n, 0`; empty is **null**; anything else is an
error. The parser decides nothing about what an error means. Each caller does, and that is the
specification:

| Where | Empty | Not empty and not in the sets |
|---|---|---|
| Rule-file bool literal | null | compile error naming file and line (`checkBoolLiteral`, compilerv2); refused at load on both engines |
| Lookup bool column (packager, `jets/workspace/lookup_tables.go`) | SQL `NULL`, served as rdf null on both engines | compile error naming lookup, CSV row, column and value |
| Rule configuration (`ParseRdfNodeValue`) | null | pipeline fails at start |
| Text a rule tests for truth: filter, `and`, `or`, `not` | null; a filter does not pass; no exception | `(jets:iState jets:exception "…")` and the expression is null; the rule set carries on and the pipeline reports it under `on_error` |
| `map_record` bool cell | null (an empty cell is never cast) | record-level error under the operator's `on_error` |
| Typed file read (`castToRdfTypeFromTxt`) | null | the read fails, as for a bad date or int: JetStore wrote the file |
| Rules unit test expected bool | null | the check errors |

**Three things that were wrong on the way, and are fixed with it.** `bool x = "false";` kept its
quotes, so both engines received `"false"` with the quote characters: true on C++, false on Go.
C++ `not` on any text raised `Invalid arguments for not`, failing the session, even for `"false"`.
And C++ read a SQL `NULL` in an int or double lookup column as `0` where Go served null
(`lookup_sql_helper.cc`), which mattered the moment an empty bool cell became `NULL`.

**The run-time exception is the C++ `substring_of` pattern** (`expr_op_strings.h`), now on Go as
`ReteSession.RaiseException`: a rule-time error is data in the session, not a failed
`ExecuteRules`, so one bad value in one row does not stop the others.

## A minting consequent is retracted by replay; two retraction defects are kept — 2026-10-08, revised 2026-10-09

Both engines. **Retraction recomputes a beta row's consequents** — it evaluates them again
against the withdrawn row and retracts whatever comes out (`ReteSession::compute_consequent_triples`,
`jets/rete/rete_session.cc:343`; `ComputeConsequentTriples`, `rete_session_exec_rules.go:212`). Found
by the `usi_ws` workspace assessment (`jetstore_agentic_ai`,
`projects/workspace_assessments/02_usi_ws_assessment`, Q-37 and R-13), whose rule `ckd30` was keyed as
a workaround (artisoft-io/usi_ws#22).

### What is fixed: a consequent that mints a resource

- **`create_entity 0`** minted a second UUID on retraction, inserted a stray `jets:key` triple for
  it, and retracted a link to it that did not exist, so the real link stayed.
- **a keyed `create_entity`** re-inserted `jets:key` on every retraction, so its reference count
  never reached zero.

At rule load a node vertex is flagged when one of its consequent terms mints a resource:
`create_entity`, `create_uuid_resource`, `range`, `lookup_rand` or `multi_lookup_rand`. C++: the
`MintsResource` marker on those visitors (`jets/rete/expr.h:135`) and `ExprBase::mints_resource`,
read by `ReteMetaStore::initialize` (`jets/rete/rete_meta_store.h:201`). Go: `expr_mints_resource.go`
and `AddConsequentTerm` (`node_vertex.go:77`). A row of a flagged vertex records every triple its
consequents raised the reference count of — the consequent triple and `create_entity`'s `jets:key`,
both inserted through `insert_inferred_for` / `InsertInferredFor` (`jets/rete/rete_session.h:311`,
`rete_session.go:34`) — and **retraction retracts that record and evaluates nothing**
(`rete_session.cc:423`, `rete_session_exec_rules.go:270`). The record is emptied rather than
released, because a withdrawn row stays in its relation (below) and is retracted again on later
withdrawals; released, the second retraction fell back to recomputing `create_entity` and
re-inserted `jets:key`. **A keyed entity's `jets:key` goes with its last support** — decided by
Michel on 2026-10-08 ("retract the keyed entity"). Lookup cache rows and `range` values are not
recorded and still outlive the row.

### What is kept: two defects, and what depends on them

**The 2026-10-08 version of this entry fixed two more things, and both were reverted on 2026-10-09
by Michel's decision** ("go with (a), the narrow fix"), because with them `usi_ws`'s rules never
terminate with `_0:no_truth_main_on_exist` off. They are kept as **known defects**, pinned by the
`LEGACY` cases in `jets/rete/retract_recorded_consequents_test.cc` and
`retract_recorded_consequents_test.go`:

1. **A consequent that reads the graph is recomputed on retraction** — the aggregates, `exist`,
   `size_of`, `lookup`. Once its input has moved, the triple retracted is not the triple inferred,
   and the inferred one stays.
2. **A withdrawn row never fires again.** After retracting a row's consequents both engines call
   `remove_beta_row` / `RemoveBetaRow` (`rete_session.cc:456`, `rete_session_exec_rules.go:305`), which
   returns at once for a row marked `kDeleted`, which that row is. It stays in its relation as
   `kProcessed`, and an equal row arriving later is taken for one already inferred.

**Measured on the assessment's local harness, flag off, against the engine at `45dd7851`:**

- **The loop rules depend on both together.** `MSK_ADJ_LOOP010`/`LOOP020`/`MERGE_010` (MSK),
  `adj11`/`adj12`/`adj20` (IM) and `mcp1`/`mcp2`/`csc20` (CM) have one shape: an unmerged match is on
  the priority list, a loop rule guarded by `[exist priorityList]` infers `topMatch` by
  `sorted_head`, and `topMatch` puts the match on `mergeList`. That is an **odd loop through
  negation, with no fixed point under correct truth maintenance**. Replaying the `sorted_head`
  consequent retracts `topMatch` when the list empties, which un-merges, which restores the list,
  which re-admits the loop rule — and with defect 2 fixed, it fires again. Consequent visits went
  from tens to millions; the CM test run never finished, IM members ended at the vertex visit limit.
  Recomputing gave null over the empty list and retracted nothing, and the stale row never
  re-fired, so the loop ended.
  `LoopThroughNegationTest` (`jets/rete/loop_through_negation_test.cc`) and
  `TestLoopControlRulesTerminate` (`loop_through_negation_test.go`) reproduce it in five rule terms
  and guard that it still terminates.
- **CM04 `op01` depends on defect 2 alone**, with pure consequents. Its guard
  `[(?x01 exist_not hc_usi:totalAmountAllowed) and (?x01 exist_not hc_usi:alternateModifier)]`
  is falsified by its own chain (`cpt01` → `cpt10` → `gz01` → `gz02` → `rt01` infers
  `totalAmountAllowed`), the row is withdrawn and its consequents retracted, and the guard is true
  again. With defect 2 fixed it repeats until the vertex visit limit, a million times each on
  vertexes 57 and 59 of `4_CM04_Orphans_Main1`; on the pin it stops after one swing.
- **Fixing defect 1 alone** terminates but undoes every merge: all four CM and all four IM members
  of the test claims changed.
- **With the flag on**, as USI runs in production, no `exist` callback is registered, none of this
  happens, and the full fix gave the same member outcomes as the pin.

**The rule language cannot end these loops by itself**: a rule has the properties `o`, `s` and
`flag`, and nothing that makes a consequent non-retractable. *(Wrong, corrected 2026-10-11 below:
`usi_ws` ended them in the rule language, by changing what the rules are guarded on.)* So if a
workspace ever needs correct truth maintenance on these shapes, the paths are:

- **(b)** put aggregate replay and row erasure behind an opt-in rule-config switch, default off —
  two code paths, no workspace changing behaviour unless it asks; or
- **(c)** fix both outright and add an engine primitive making a loop rule's consequents
  non-retractable, then rewrite the `usi_ws` loop rules and its `exist_not` self-guards (`op01`,
  `op02` and their like) before running it with the flag off.

**2026-10-11: option (b) is built, for rules unit tests only, and no pipeline can reach it.**
`jetstore_agentic_ai`'s `rules_unit_test_harness` project (Phase 1, track `AF`) restored
`1d993844`'s two fixes behind a **retraction mode fixed when the network is built**, one switch
per defect, because each was measured above to do something different alone. Go:
`RetractionMode` (`retraction_mode.go`), passed as `WithRetractionMode` to
`NewReteMetaStoreFactory`; `AddConsequentTerm` records a graph-reading consequent
(`expr_reads_graph.go`, the wide half of `1d993844`'s row purity) only with
`ReplayGraphReadingConsequents`, and `ComputeConsequentTriples` calls `EraseRetractedBetaRow`
instead of `RemoveBetaRow` only with `EraseWithdrawnRows`. C++: `RetractionMode`
(`jets/rete/rete_meta_store.h`), passed to `ReteMetaStore::initialize` through
`ReteMetaStoreFactory::load_database` from the C entry point `create_jetstore_hdl_with_mode`;
`create_jetstore_hdl` is unchanged and builds the legacy mode. **The zero value is legacy for
both, in both engines**, and nothing a pipeline configures sets anything else: there is no
`rule_config` key and no `JetrulesSpec` field for it. The mode is set by a unit test definition
(its `legacy` object), which the harness reads before it builds the test's engine, so each test
runs in its own process in one mode. Production runs exactly what the decision of 2026-10-09
left it running. The `LEGACY` tests in both engines now run every case in the four modes:
`legacy` pins today's behaviour, and `fixed-1`, `fixed-2` and `fixed` assert what `1d993844`
asserted. On the five-term loop shape they reproduce the measurement above on both engines:
`fixed` ends at the vertex visit limit, `fixed-1` terminates with both merges undone, and
`fixed-2` terminates as `legacy` does.

**2026-10-11, later: `usi_ws` depends on neither defect any more, nor on the switch.** Two
corrections to what is above, from `jetstore_agentic_ai`'s `rules_unit_test_harness` (its `I-38` to
`I-40` and `I-44`), which reduced these shapes to unit tests:

- **What held the shapes up in production was the switch, not the defects.** The measurements
  above were taken with `_0:no_truth_main_on_exist` off. With it on, as USI runs, an `exist` filter
  is evaluated once, when its row is made, and the compiler puts the loop rule's filter on the
  loop-0 `jets:State` row. So the guard never moved. With the switch on, `usi_ws` at the time passed
  with both defects off as well. The defects held the shape up only with the switch off, which is
  what this entry measured. They do so in `jets_ws`'s `looping_test1`, which has no switch.
- **The rule language could end them; what was missing was the right guard, not an engine
  primitive.** `artisoft-io/usi_ws#31` rewrote them without (c)'s non-retractable consequent:
  - The loop rule (`mcp2` in CM, its IM and MSK twins) is guarded by `size_of` on the match list,
    which no merge changes, instead of `exist` on the priority list, which every merge shrinks.
  - The loop completes on an empty priority list. `size_of` is maintained in every mode.
  - CM02's and IM02's adjustment loops run in their own rule sets, each with its own loop counter.
  - `op01`/`op02`'s chain now ends in `_0:totalAmountAllowed`, which `rt10` copies to
    `hc_usi:totalAmountAllowed` in `4_CM04_Orphans_Main2`, the next tier, where nothing tests for
    its absence.
  - IM02's `MEREXCL011`/`012` test `_0:hasOwnExclusion` instead of `hc_usi:hasExclusion`, which
    their own result reached.

  `artisoft-io/usi_ws#32` then stopped setting the switch. The `usi_ws` unit tests `cm_loop`,
  `im_mercom` and `msk_loop` pass in the legacy mode and with both defects off (`fixed`). With the
  switch off, CM and IM give the switch-on export on the assessment's 28 test claims.

**So the case for keeping the defects now rests on USI's own copy of the workspace**, which still
sets the switch and has neither PR; it must take both together, because the switch removed without
`#31`'s rules brings the hangs back. Nothing in the workspaces `jetstore_agentic_ai` pins depends
on them except `jets_ws`'s `looping_test1`, which pins them on purpose, and the `LEGACY` tests.
Whether pipelines should run with them fixed is a decision this entry does not make.

**Two divergences found on the way, neither fixed here.** C++ `RDFGraph::erase` notifies no graph
callback (`erase_internal`, `jets/rdf/rdf_graph.h:301`), where Go's `RdfGraph.erase_internal` does —
so erasing an asserted triple drives truth maintenance in Go and not in C++, which is why the cycle
tests drive retraction with `insert_inferred` and `retract` instead. And Go `RdfGraph.Retract`
erases the triple from its `pos` and `osp` indexes even when the `spo` reference count is still
above zero (`Retract`, `jets/jetrules/rdf/rdf_graph.go:188`), so a triple with two supports that
loses one should stay `Contains`-true and stop being found by predicate or object — read from the
code, not measured.

## `_0:no_truth_main_on_exist` — a production flag, and where its test is — 2026-09-09

C++ only; there is no Go counterpart, so a rule set that needs it cannot run on the Go engine at all.
`workspaces/usi_ws` set it in `pipes_config/usiclaim_processes.pc.json` (until 2026-10-11,
`artisoft-io/usi_ws#32`; see the entry above; USI's own copy still does) because its rules predate
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
`test_ws` with `workspace.CompileWorkspace`, asset validation skipped, and asserts the joined strings. **Two of the three config forms
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
