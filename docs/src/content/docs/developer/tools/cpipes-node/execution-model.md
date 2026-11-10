---
title: The execution model
description: How the Python node runs the channel graph, where it departs from Go and why, the when subset it evaluates, who applies conditional_config, and how a merge_files step runs.
kind: explanation
sidebar:
  order: 2
---

## The channel graph

`graph.run` is the run: the channel registry, the `generator` source, the `fan_out` executor with
`Apply` / `Done` / `Finally` and the channel closes around them, and `when` / `conditional_config`
evaluated before a factory is reached.

**The dispatch is the declaration.** There is no `if spec.type == "fan_out"` anywhere in `graph.py`;
`Operator.build` returns the handler and the graph calls it, so a channel type or pipe kind this node
grows is reached by its class being written rather than by an `if` being remembered.

**The execution model is the one place this departs from Go**, deliberately. Go runs every pipe of a
step in its own goroutine over unbuffered channels. This node runs one pipe at a time, in an order
derived from the document, draining after every source record. What that buys:

- **Determinism**: the order records reach a channel is a function of the `.pc.json` and not of a
  scheduler.
- **Bounded memory**: a channel holds what one source record produced, so the graph, like the
  operator, holds a partition's rows rather than a whole input's.
- **No deadlock to reason about**: every channel is closed by the pass that finishes its writer, and
  `run` asserts at the end that every channel a pipe read is closed.

What it costs is throughput, and any test of *concurrent* close semantics.

Two documents are refused at startup that Go accepts and then hangs on: a pipe whose source channel
no step writes, and a cycle. Both name the channel.

## The `when` subset

A guard is evaluated in a declared subset of the expression language (the comparisons, `AND` / `OR` /
`NOT`, `IS` / `IS NOT`, and `IN` over a static list), and **anything outside it is refused by name at
build time**. A `when` this node could not evaluate and defaulted either way would silently apply or
skip a step an author decided about.

## `conditional_config`

It is the **starter's**, not the node's: `ApplyAllConditionalTransformationSpec` runs in the start
actions (`jets/compute_pipes/actions_start_*_cp.go`). This node applies it only where it stands in
for a starter, which the document's own shape says: a flat `pipes_config` is a starter's work and
`conditional_pipes_config` alone is an authored document.

## `merge_files`

A merge is **a node mode and not a pipe of the channel graph**, which is the shape Go has:
`ProcessFilesAndReportStatus` branches on `ComputePipesArgs.MergeFiles` before `StartComputePipes` is
reached and calls `StartMergeFiles` instead of `LoadFiles`, so a merge registers no channel, builds no
evaluator and sees no record. `Pipe.drives_channel_graph` carries that per pipe kind and `graph.run`
reads it off the declaration, so there is no `if spec.type == "merge_files"` anywhere in that module.

Its input channel is typed `stage`, and **`stage` is not a channel type this node declares**: a
`fan_out` reading one would need an S3 reader and a record parser this node does not have. The type is
not an author's choice either: `ValidatePipeSpecConfig` refuses a merge reading anything else. So the
scope gate asserts the fixed type for this pipe kind instead of classifying a token, which is strictly
narrower than classifying it, and a merge reading `memory` is refused at *this node's* startup, where
Go refuses it only in the starter.

`header_plan` is `StartMergeFiles`' six-case switch arm for arm and **in its order**, because the arms
are not disjoint as predicates and Go's `switch` takes the first: a single csv part with
`first_partition_has_headers` satisfies two of them. The order is also what makes this node's single
path faithful: whenever `write_headers` and `skip_input_headers` are both false the merged file is the
part files unchanged, which is exactly what Go's S3 multipart copy produces. Getting it wrong produces
a merged CSV with a header line in the middle, which every downstream reader accepts.

Three inputs are refused by name: snappy compression, more than one parquet part (a single one is a
copy, which is Go's own condition), and xlsx, which the Go merge does not support either.

**Whether to merge a partitioned output at all is an open question.** A merge step is validated to run
on exactly **one** partition, and that partition set is derived by listing the previous step's stage
prefix, so a merge can only see part files one partition wrote. Merging an output a forty-node run
wrote therefore means funnelling all of it through one node first, which is the memory concentration
partitioning exists to remove. The case for merging is the consumer's (one file per table); the case
against is the producer's, and it is the one that scales with `nbr_nodes`. What would settle it is a
peak-memory measurement of that single-partition step at a realistic size, which nobody has made.
