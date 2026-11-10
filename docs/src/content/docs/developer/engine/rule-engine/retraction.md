---
title: "Retraction and truth maintenance"
description: "How the rule engine retracts what a withdrawn row inferred, and the retraction defects and switch it once kept."
kind: explanation
sidebar:
  order: 1
---

How the rule engine withdraws what a row inferred when the row loses its support, newest first. The
older sections are the history of how the engine reached its one retraction behaviour, and say so.

## One retraction behaviour; `_0:no_truth_main_on_exist` deleted — 2026-10-24

Decided on 2026-10-24. **The engine has one retraction behaviour, the one the `LEGACY` mode
called `fixed`**, and the mode is deleted, as the Go engine was. This closes the section of
2026-10-08 below and the `_0:no_truth_main_on_exist` entry of 2026-09-09, which are kept as the
history of how the engine got here.

- **A consequent that reads the graph records and is replayed**, as a minting one always was: an
  aggregate, `exist`, `exist_not`, `size_of`, `lookup`. `ReteMetaStore::initialize` sets
  `records_consequents` on a vertex when a consequent `mints_resource()` or `reads_graph()`
  (`jets/rete/rete_meta_store.h`). So the triple retracted is the triple inferred, however the
  graph has moved since. *(This was `LEGACY` defect 1, kept from 2026-10-09.)*
- **A withdrawn row is erased** from its relation once its consequents are retracted
  (`BetaRelation::erase_retracted_beta_row`, from `compute_consequent_triples`), so an equal row
  arriving later fires again. *(This was `LEGACY` defect 2.)*
- **An `exist` or `exist_not` filter always registers its callback**, so it is re-evaluated when
  the graph moves under it. `_0:no_truth_main_on_exist` set to 1 turned that off; the switch is
  deleted (the member, its accessors and the two early returns in `expr_op_resources.h`).

**Why now.** A comparison ran the fixed mode against legacy on the rules benchmark's nine inputs, a
client workspace's rules and the workspaces' unit tests, and found them
computing the same, but for the tests built to pin the two defects. The client workspace that
relied on the defects and the switch had stopped doing so on 2026-10-11 (the corrections at the end
of the 2026-10-08 section); its deployed rules no longer set the switch, which was tested at
deployment, and no other client was known to rely on either.

**What was deleted**, C++: `RetractionMode` (`rete_meta_store.h`) and the mode parameter of
`ReteMetaStore::initialize` and `ReteMetaStoreFactory::load_database`; the C entry point
`create_jetstore_hdl_with_mode` (`create_jetstore_hdl` is unchanged in signature, and is now the
only way to build the networks); the `remove_beta_row` branch of retraction; `no_truth_main_on_exist_`
and `set_`/`get_no_truth_main_on_exist`. Go: `rete.RetractionMode` (`jets/jetrules/rete/retraction_mode.go`),
`compute_pipes.RetractionMode`, `EngineOption`, `WithRetractionMode` and `ApplyEngineOptions`
(`NewJetRuleEngine` has its signature from before the mode), `bridge.LoadJetRulesWithMode`, the native
adaptor's `RetractionMode()`, `harness.RetractionModeOf`, and `rulesbench.RetractionModeOf`.

**What is refused, naming the change**, so that a configuration that asked for the deleted
behaviour is told rather than run on the one that remains:

- **`_0:no_truth_main_on_exist` in a rule configuration**, by `ReteSession::initialize`, where
  every session starts, so every caller gets it: the session does not start, and its error says
  `the rule configuration sets _0:no_truth_main_on_exist to 1 (int): that switch was deleted with the
  LEGACY retraction mode (2026-10), and exist and exist_not filters always take part in truth
  maintenance; remove it`. An integer 0 asks for what remains and is accepted; any other value,
  text `"1"` included, is refused. The message reaches Go: `create_rete_session` keeps the handle
  of a session whose `initialize` failed, the new C entry point `get_rete_session_err_msg` reads
  its error, and `bridge.NewReteSession` returns it and releases the session (it returned a bare
  return code, and leaked the session, before). A rules unit test or a bench spec that sets it is
  refused earlier, by validation, in the words of the next item.
- **A `legacy` switch set `true`** in a rules unit test (`.tst.json`) or a `rules_bench` spec,
  and **`rules_bench -legacy`** with any value, with `the LEGACY retraction mode was deleted in
  October 2026; the C++ engine has one retraction behaviour, the one the mode called fixed`
  (`rulestest.LegacyRetractionDeleted`). **A `legacy` object whose every present switch is
  `false`** names the behaviour that remains and is accepted, selecting nothing, as
  `use_jet_rules_go: false` was at the Go engine's retirement: a client workspace's three strict
  unit tests carry one.
- **`legacy_mode` stays in results and in the database, and is always `fixed`.** It is a column of
  `unit_tests` and `rules_unit_test_execution` and of the Workspaces screen's two tables, and rows
  written before carry `legacy`, `fixed-1` or `fixed-2`; dropping it would lose what those rows
  say ran.

**The tests.** The `LEGACY` cases of `retract_recorded_consequents_test.cc` ran in four modes; they
run once, asserting the fixed behaviour, and the four whose names stated a defect were renamed:
`AggregateConsequentDoesNotRecord` is `AggregateConsequentRecords`,
`CreateEntityKeyedCyclesStayRetracted` is `CreateEntityKeyedCyclesRefire`,
`ConsequentAggregateRetractionRecomputes` is `ConsequentAggregateRetractionReplays`, and
`PureConsequentDoesNotRefire` is `PureConsequentRefires`. `LoopThroughNegationTest.LoopControlRulesTerminate`,
the guard that the five-term loop shape terminated, is
`LoopThroughNegationTest.LoopThroughNegationReachesTheVertexLimit`: **an odd loop through negation
has no fixed point, and the vertex visit limit is what stops it now**, as the 2026-10-08 section
measured with both defects off. A rule set written in that shape fails its session rather than
ending on what the defects left. `NoTruthMainOnExistTest` keeps its truth-maintenance cases and
pins the refusal. `jets_ws`'s `retraction_defect1`, `retraction_defect2` and `looping_test1` are their
former `_fixed` twins under the plain names, and the variant of `looping_test1` that set the switch
is deleted.

## `Retract` deletes, and an erase tells the rete — 2026-10-22

C++ jetstore#2333; both adaptors jetstore#2336. Settled with the engines' other differences (see
[The retired Go engine](/docs/developer/engine/rule-engine/go-engine/)).

**The adaptor's `Retract` deletes an asserted fact**, however many times it was inserted, and the
rete withdraws what depended on it at the next `ExecuteRules`.
- **C++** calls the session's erase. That erase removed the triple and told nothing: the notice was
  dropped in 2022 (`2507bf7c`).
- **Go** called `RdfSession.Erase` from #2336. Before, it called `RdfSession.Retract`, the
  inferred-only, one-support retraction the rete uses for consequents, which never removed an
  asserted triple.
- **`RDFGraph::erase` now sends `triple_deleted`** for each triple it removes, as Go's `Erase` did.
  That reaches `RDFSession::erase`, the bridge's `erase` and the native adaptor's `Retract`.
- The engine's own retraction, the session's `retract` (Go's `RdfSession.Retract`), is unchanged.

**One erase stays silent, deliberately:** `RDFSession::insert`'s move of an inferred copy to
the asserted graph (`erase_without_notice`). The triple never leaves the session, so a notice would
only retract and rebuild rows. Production's one other erase, the pool worker clearing `jets:loop`
and `jets:completed` at a rule set's start (`jetrules_pool_worker.go`), runs before the new rete
session has rows, so its notice changes nothing.

## A minting consequent is retracted by replay; two retraction defects are kept — 2026-10-08, revised 2026-10-09

**Closed 2026-10-24: the two defects and the mode that kept them are deleted, and the fixed
behaviour is the engine's only one** (the first section). What follows is the history. Its test
names are as they were before the deletion; the tests it names as pinning a defect now assert the
fixed behaviour, under the names the first section gives.

Both engines. **Retraction recomputes a beta row's consequents** — it evaluates them again
against the withdrawn row and retracts whatever comes out (`ReteSession::compute_consequent_triples`, `jets/rete/rete_session.cc`; Go's
`ComputeConsequentTriples`, `rete_session_exec_rules.go` at `go-engine-final`). Found by the
assessment of a client workspace, where one rule's `create_entity` had been keyed as a workaround,
in that workspace's own repository.

### What is fixed: a consequent that mints a resource

- **`create_entity 0`** minted a second UUID on retraction, inserted a stray `jets:key` triple for
  it, and retracted a link to it that did not exist, so the real link stayed.
- **a keyed `create_entity`** re-inserted `jets:key` on every retraction, so its reference count
  never reached zero.

At rule load a node vertex is flagged when one of its consequent terms mints a resource:
`create_entity`, `create_uuid_resource`, `range`, `lookup_rand` or `multi_lookup_rand`. C++: the
`MintsResource` marker on those visitors (`jets/rete/expr.h`) and `ExprBase::mints_resource`,
read by `ReteMetaStore::initialize` (`jets/rete/rete_meta_store.h`). Go had `expr_mints_resource.go` at `go-engine-final`
and `AddConsequentTerm` (`node_vertex.go` at `go-engine-final`). A row of a flagged vertex records every triple its
consequents raised the reference count of — the consequent triple and `create_entity`'s `jets:key`,
both inserted through `insert_inferred_for` / `InsertInferredFor` (`jets/rete/rete_session.h`,
`rete_session.go` at `go-engine-final`) — and **retraction retracts that record and evaluates nothing**
(`beta_row->is_recording()` in `compute_consequent_triples`, `jets/rete/rete_session.cc`; `rete_session_exec_rules.go` at `go-engine-final`). The record is emptied rather than
released, because a withdrawn row stays in its relation (below) and is retracted again on later
withdrawals; released, the second retraction fell back to recomputing `create_entity` and
re-inserted `jets:key`. **A keyed entity's `jets:key` goes with its last support** — decided on
2026-10-08. Lookup cache rows and `range` values are not
recorded and still outlive the row.

### What is kept: two defects, and what depends on them

**The 2026-10-08 version of this entry fixed two more things, and both were reverted on 2026-10-09,
in favour of the narrow fix**, because with them a client workspace's rules never
terminate with `_0:no_truth_main_on_exist` off. *(That reason is gone since 2026-10-11: see the
correction of 2026-10-23 at the end of this entry. The defects are still what production runs.)* They are kept as **known defects**, pinned by the
`LEGACY` cases in `jets/rete/retract_recorded_consequents_test.cc` (and, until the retirement, in Go's
`retract_recorded_consequents_test.go` at `go-engine-final`):

1. **A consequent that reads the graph is recomputed on retraction** — the aggregates, `exist`,
   `size_of`, `lookup`. Once its input has moved, the triple retracted is not the triple inferred,
   and the inferred one stays.
2. **A withdrawn row never fires again.** After retracting a row's consequents the engine calls
   `remove_beta_row` (`jets/rete/rete_session.cc`; Go's `RemoveBetaRow`, `rete_session_exec_rules.go` at `go-engine-final`), which
   returns at once for a row marked `kDeleted`, which that row is. It stays in its relation as
   `kProcessed`, and an equal row arriving later is taken for one already inferred.

**Measured on the assessment's local harness, flag off, against the engine at `45dd7851`:**

- **The loop rules depend on both together.** The workspace's three claim processes each have a
  set of loop rules of one shape: an unmerged match is on the priority list, a loop rule guarded
  by `[exist priorityList]` infers `topMatch` by `sorted_head`, and `topMatch` puts the match on
  `mergeList`. That is an **odd loop through
  negation, with no fixed point under correct truth maintenance**. Replaying the `sorted_head`
  consequent retracts `topMatch` when the list empties, which un-merges, which restores the list,
  which re-admits the loop rule — and with defect 2 fixed, it fires again. Consequent visits went
  from tens to millions; one process's test run never finished, and another's members ended at the
  vertex visit limit.
  Recomputing gave null over the empty list and retracted nothing, and the stale row never
  re-fired, so the loop ended.
  `LoopThroughNegationTest` (`jets/rete/loop_through_negation_test.cc`), and on Go
  `TestLoopControlRulesTerminate` (`loop_through_negation_test.go` at `go-engine-final`), reproduce it in five rule terms
  and guard that it still terminates.
- **A self-guarded rule depends on defect 2 alone**, with pure consequents. Its guard is two
  `exist_not` tests on properties that its own chain of five rules goes on to infer, so the guard is
  falsified by that chain, the row is withdrawn and its consequents retracted, and the guard is true
  again. With defect 2 fixed it repeats until the vertex visit limit, a million times each on two
  vertexes of its rule set; on the pin it stops after one swing.
- **Fixing defect 1 alone** terminates but undoes every merge: all four members of each of two
  processes' test claims changed.
- **With the flag on**, as the client runs in production, no `exist` callback is registered, none
  of this happens, and the full fix gave the same member outcomes as the pin.

**The rule language cannot end these loops by itself**: a rule has the properties `o`, `s` and
`flag`, and nothing that makes a consequent non-retractable. *(Wrong, corrected 2026-10-11 below:
the workspace ended them in the rule language, by changing what the rules are guarded on.)* So if a
workspace ever needs correct truth maintenance on these shapes, the paths are:

- **(b)** put aggregate replay and row erasure behind an opt-in rule-config switch, default off —
  two code paths, no workspace changing behaviour unless it asks; or
- **(c)** fix both outright and add an engine primitive making a loop rule's consequents
  non-retractable, then rewrite the workspace's loop rules and its `exist_not` self-guards before running it
  with the flag off.

**2026-10-11: option (b) is built, for rules unit tests only, and no pipeline can reach it.**
The rules unit-test harness restored
`1d993844`'s two fixes behind a **retraction mode fixed when the network is built**, one switch
per defect, because each was measured above to do something different alone. Go had
`RetractionMode` (`retraction_mode.go`), passed as `WithRetractionMode` to
`NewReteMetaStoreFactory`; `AddConsequentTerm` recorded a graph-reading consequent
(`expr_reads_graph.go` at `go-engine-final`, the wide half of `1d993844`'s row purity) only with
`ReplayGraphReadingConsequents`, and `ComputeConsequentTriples` called `EraseRetractedBetaRow`
instead of `RemoveBetaRow` only with `EraseWithdrawnRows`. C++: `RetractionMode`
(`jets/rete/rete_meta_store.h`), passed to `ReteMetaStore::initialize` through
`ReteMetaStoreFactory::load_database` from the C entry point `create_jetstore_hdl_with_mode`;
`create_jetstore_hdl` is unchanged and builds the legacy mode. **The zero value is legacy for
both, in both engines** (the Go struct `rete.RetractionMode` stays, without the factory option:
it carries the mode to the bridge), and nothing a pipeline configures sets anything else: there is no
`rule_config` key and no `JetrulesSpec` field for it. The mode is set by a unit test definition
(its `legacy` object), which the harness reads before it builds the test's engine, so each test
runs in its own process in one mode. Production runs exactly what the decision of 2026-10-09
left it running. The `LEGACY` tests in both engines ran every case in the four modes, and C++'s still do:
`legacy` pins today's behaviour, and `fixed-1`, `fixed-2` and `fixed` assert what `1d993844`
asserted. On the five-term loop shape they reproduced the measurement above on both engines:
`fixed` ends at the vertex visit limit, `fixed-1` terminates with both merges undone, and
`fixed-2` terminates as `legacy` does.

**2026-10-11, later: the workspace depends on neither defect any more, nor on the switch.** Two
corrections to what is above, from the work that reduced these shapes to rules unit tests:

- **What held the shapes up in production was the switch, not the defects.** The measurements
  above were taken with `_0:no_truth_main_on_exist` off. With it on, as the client ran until 2026-10-11, an `exist` filter
  is evaluated once, when its row is made, and the compiler puts the loop rule's filter on the
  loop-0 `jets:State` row. So the guard never moved. With the switch on, the workspace at the time passed
  with both defects off as well. The defects held the shape up only with the switch off, which is
  what this entry measured. They do so in `jets_ws`'s `looping_test1`, which has no switch.
- **The rule language could end them; what was missing was the right guard, not an engine
  primitive.** A change in the workspace's own repository rewrote them without (c)'s
  non-retractable consequent:
  - Each process's loop rule is guarded by `size_of` on the match list, which no merge changes,
    instead of `exist` on the priority list, which every merge shrinks.
  - The loop completes on an empty priority list. `size_of` is maintained in every mode.
  - Two processes' adjustment loops run in their own rule sets, each with its own loop counter.
  - The self-guarded rules' chain now ends in an `_0:` property, which a rule of the next tier
    copies to the property the guard tested, where nothing tests for its absence.
  - Two exclusion rules test their own `_0:` flag instead of the property their own result
    reached.

  A second change then stopped setting the switch. The workspace's three loop unit tests pass in the
  legacy mode and with both defects off (`fixed`). With the switch off, two processes give the
  switch-on export on the assessment's 28 test claims.

**So the case for keeping the defects now rests on the client's own copy of the workspace**, which
still sets the switch and has neither change; it must take both together, because the switch removed
without the first change's rules brings the hangs back. Nothing in the workspaces then examined depends
on them except `jets_ws`'s `looping_test1`, which pins them on purpose, and the `LEGACY` tests.
Whether pipelines should run with them fixed is a decision this entry does not make.

**Corrected 2026-10-23: the client workspace that relied on them no longer does.** The paragraph
above was right on 2026-10-11 and stopped being right the same day, and it stood until the release
notes were checked. The client workspace that relied on the defects took both changes
together on 2026-10-11, its rewritten loops and the switch no longer set, and they were released
in Release 4.0.3. Its strict unit tests, which run with both defects off, pass with the legacy
tests' expectations. So no workspace depends on the defects or on the switch. **Production still
runs the legacy mode**: no pipeline can select another, and `create_jetstore_hdl` builds legacy.
Whether production moves to the fixed mode is measured first, legacy against fixed on the
benchmark's inputs and the workspaces' unit tests, and decided after that (it was, on 2026-10-24:
see the first section).

**Two divergences found on the way, neither fixed here.** C++ `RDFGraph::erase` notified no graph
callback (`erase_internal`, `jets/rdf/rdf_graph.h`), where Go's `RdfGraph.erase_internal` did —
so erasing an asserted triple drove truth maintenance in Go and not in C++, which is why the cycle
tests drive retraction with `insert_inferred` and `retract` instead. **Settled 2026-10-22
(jetstore#2333):** C++'s erase now tells the rete, as Go's did; only
`RDFSession::insert`'s move of an inferred copy stays silent. See *`Retract` deletes* above. ~~And Go `RdfGraph.Retract`
erases the triple from its `pos` and `osp` indexes even when the `spo` reference count is still
above zero, so a triple with two supports that loses one should stay `Contains`-true and stop
being found by predicate or object, read from the code, not measured.~~ **Fixed 2026-10-21**: Go's `RdfGraph.Retract` retracted all three indexes from then on, as C++
does, and `TestRetractKeepsATripleWithASupportLeftInEveryIndex` (`rdf_graph_test.go` at `go-engine-final`) held it.

## `_0:no_truth_main_on_exist` — a production flag, and where its test is — 2026-09-09

**Closed 2026-10-24: the flag is deleted, and a rule configuration that still turns it on is
refused at session start** (the first section). What follows is what it did.

C++ only; the Go engine had no counterpart, so a rule set that needed it could not run on the Go engine at all.
A client workspace set it in its pipeline configuration (until 2026-10-11; see the section above. Its
deployed rules no longer use it either, tested at deployment, as the first section says) because its rules predate
rule filters participating in truth maintenance, and it makes `ExistVisitor::register_callback` and
`ExistNotVisitor::register_callback` (`jets/rete/expr_op_resources.h`) register nothing.

**The two directions fail differently, which is worth knowing before reading a wrong result.** With
the flag on, an `exist` filter that becomes true is never noticed, and an `exist_not` filter that
becomes false keeps whatever it inferred — the second is a triple that should have been retracted and
was not. `NoTruthMainOnExistTest` (`jets/rete/no_truth_main_on_exist_test.cc`) pins both, and the
default: **absent means truth maintenance ON**, and so does a value that is present and is not an
int32, which is how a workspace writing `"1"` as text gets the opposite of what it asked for with
nothing logged.
