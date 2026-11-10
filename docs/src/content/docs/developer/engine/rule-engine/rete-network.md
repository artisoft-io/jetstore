---
title: "The rete network at run time"
description: "Salience, a filter's re-entry, index removal, live iterators, and what a vertex holds, observed."
kind: explanation
sidebar:
  order: 3
---

How the compiled network behaves while a session runs: what a vertex can be observed to hold, how a
filter that writes to the graph is reconciled, how rows leave an index, what a live iterator sees,
and why every rule fires at the salience it declares. Several sections were settled while a Go
engine also ran, and say what it did; it is retired (see [The retired Go engine](/docs/developer/engine/rule-engine/go-engine/)).

## What a vertex holds, observed — 2026-10-24

**`get_vertex_observations`**
(`jets/rete/jets_rete_cwrapper.h`) reads, for every vertex of a rete session's network, its beta
relation's rows now (`BetaRelation::size`) and at their most in the session (`peak_size`), and the
times its consequents were computed and retracted (`ReteSession::vertex_visits`). Go reads it as
`bridge.ReteSession.ObserveVertices`, and `compute_pipes.VertexObserver` on the native adaptor's rete
session; `rules_bench -observe` writes it per session (see the `rules_bench` command's documentation).

**It needs no observation flag, and changes no hot path**: the peak has been kept since the reserve
hint (`b0136f18`, 2026-10-22), which leaves each session's peak for the next session's relation, and
the visits are kept for the vertex visit limit, which counts the inferred ones. Rows now and at peak
count the relation's row set, withdrawn rows not yet erased included, which is the number memory
follows. So a session that is never observed runs exactly the
code it ran before; the reader is the whole of the change.

## A filter's re-entry is reconciled before its rows are kept — 2026-10-22

C++ jetstore#2332, #2335; Go jetstore#2330, #2334.

A filter can write to the graph. In a compiled rule set the only write left is the `jets:exception` a
failed truth test raises (the compiler refuses graph-writing operators in filters). The write's
callbacks re-enter the session at once, while a scan is still building candidates. Candidates were
then kept from parent rows that no longer existed, or with filters no longer true, and never
withdrawn, so the result depended on the order the triples arrived in.

The first fix (#2332, #2330) dropped a candidate whose *own* filter's write removed its support. The
review found that check too narrow in three ways, all closed by #2334 and #2335:
1. **A later candidate of a re-entered scan.** An earlier candidate's write removes a later
   candidate's parent row, or the absence a negation tests, just as surely, and the later candidate's
   own filter may write nothing new. In C++ a frozen pending scan could also see the removed row
   twice.
2. **A relation's first scan.** A candidate kept before the write is already inserted into a relation
   no callback reaches yet. The removal's nested visit walks the parent's remaining rows, not the
   removed one, and the filter replay does not run.
3. **A filter whose answer flips.** For example, `exist_not` of the `jets:exception` that it, or a
   later candidate, raises: its answer is out of date as soon as it is given.

C++ also did not count a callback it ignored for a relation not yet activated. Go counted it before
its activation check, and C++ now does too (`count_ignored_reentry`).

Both engines did the same three things from then on, and C++ does them now:
- **A relation's first scan, at a vertex with a filter, holds its candidates.** If the session was
  re-entered during the scan, it drops them and scans again from the parent's rows as they are now,
  then inserts them. The hold is limited to *filtered* first scans: an unfiltered scan writes
  nothing, so it cannot be re-entered, and keeps its single insert.
- **An activated relation inserts each candidate at once.** It first re-evaluates a filter whose
  evaluation re-entered, then re-checks the parent row and triple once the re-entry count has moved
  (Go `stillHolds`, C++ `candidate_still_supported`). A later re-entry still reaches the row: the
  parent row's removal withdraws it, and a graph-reading filter has `TripleUpdatedForFilter` replay
  the relation. Holding here would let the replay rebuild the relation without the held rows.
- **Both retries stop at 16** and raise an error.

Without a re-entry the cost is one comparison per scan and per kept candidate. A filtered first scan
also holds its rows. C++'s `BetaRelation::holds` asks for the same row instance, where Go compared
rows by equality. The engines still converged, because a re-inserted equal row's own pending visit
makes the same candidate.

Tests: `CandidateRowReentryTest` (C++, `jets/rete/rete_session_test.cc`); on Go, `TestRegression_I91_*` in `defects_rbm_i91_test.go` at `go-engine-final`.
End to end, the `jets_ws` rule `[(jets:iState exist_not jets:exception) and ((not ?v) != 1)]` runs
through the rules unit-test harness (`reentry_error_path`). It failed on both engines before these
fixes and passed on both after them (`compare_engines`), and it runs on C++ alone since the
retirement.

## Removing a row from an index removes only that row — 2026-10-22

C++ jetstore#2332.

When a row left a C++ beta relation, `AQVRemoveIndexBetaRowsVisitor` erased its index entries with
`erase(key)`, which removed **every** row under the key. The rows sharing the key stayed in the
relation but could no longer be joined, so a triple arriving later found nothing to join them to.
The Go engine removed the row alone.

`BetaRowIndex::erase(key, row)` (`jets/rete/beta_row_iterator.h`) now removes that row. The others keep their order (newest first),
and the key goes with its last row. A removal searches the rows under its key; on the benchmark's
workloads that is about one row, and three at most.

## A live iterator sees the graph as it was when frozen — 2026-10-23

C++ jetstore#2322.

Since the graph's tables became open-addressing tables (#2322), an insert can invalidate their
iterators, so the graph freezes its live iterators before it changes structure: each copies the
triples it has yet to return and continues over that copy (`freeze`,
`jets/rdf/base_graph_iterator.h`). **A live C++ iterator returns the graph as it was when it was
frozen**, a triple erased afterwards included and nothing inserted afterwards, which is how the Go
engine's cursor behaved: one difference between the engines fewer, recorded here on 2026-10-23.
Before, with node-based tables, such an iteration was undefined. The meta graph, shared across
threads and not changed once rules run, registers no iterator and must not change under one.

## Every rule fires at the salience it declares — 2026-10-11

**A vertex has one salience, and rules with the same antecedents used to share their last vertex.**
So a vertex could carry rules with different saliences, and the two engines settled it differently:
- **Go** took the lowest (`slices.Min`, in its `NewReteMetaStoreFactory`, over the list in `.rete.json`).
- **C++** took, and would still take, the first rule's: it reads one number, `workspace.db`'s `rete_nodes.salience`, and
  the compiler wrote `Salience[0]` there.

Either way, **a declared salience could be overruled with no error.** Raising a rule's salience did
nothing if a rule with the same antecedents sat lower. Lowering an aggregate below its producer
dragged the producer down with it when the two shared antecedents. The compiler's aggregate salience
check compared declared saliences, so it missed both.

Measured on a three-rule case in the rules unit test harness: a producer declared at s=100 sharing
its antecedents with an s=5 rule, feeding an aggregate at s=50. It compiled. C++ ran the vertex at
100 and the aggregate counted the producer's value; Go ran it at 5 and counted nothing
.

**Fixed in the compiler's network builder** (`terminalReteNode`,
`jets/compilerv2/compiler/jet_rule_listener_rete.go`): rules share their last vertex only when their
saliences are equal, and a rule with another salience gets a vertex of its own; the antecedents
before it are still shared. So every vertex fires at one salience, the declared one, on both
engines, and the aggregate check's declared saliences are the ones that run. The same three-rule
case then counted the value on both engines.

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
- **The three client workspaces** at the commits then in use compile to identical
  networks, node for node (11,363 rete nodes in the largest).
- **The clients' own copies of two of them** have no vertex carrying two saliences.
- **`jets_ws` gains one vertex**: the generated class-inheritance rule
  `ci_cintel:Briefing_Pharmacy_Event` stops sharing `BP_RxFillDates10`'s vertex and fires at its own
  s=100 instead of 10. It infers only `rdf:type owl:Thing`, which no rule reads.
