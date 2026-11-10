---
title: Inside the harness
description: The order the rules unit-test harness creates resources and asserts input in, why it shares the pipeline's loop, and how its checks read the session.
kind: explanation
sidebar:
  order: 3
---

For whoever changes `jets/rulestest/harness`:

- **Order matters.** The harness calls `engine.JetResources()` before the first RDF session opens,
  because the meta graph locks then and a new meta resource is refused afterwards with an error.
  Input is asserted through `RunRuleSets`' `assertInput`, the pipeline's order. **Until 2026-10-11 the
  C++ engine was stricter**: a meta resource made after a session opened aborted the process, and so
  did asserting input before the first rete session, because the native session made its nodes
  through the meta graph until one existed.
- **The loop is the pipeline's**, `compute_pipes.RunRuleSets`, so a change to the looping protocol
  reaches both.
- **The checks read a snapshot of the session.** `DecodeNode` normalises the adaptors' type names, so
  a check never builds a node in the session it checks.
- **The `LEGACY` switches were passed to the engine constructor** until October 2026
  (`compute_pipes.WithRetractionMode`), so that a test chose its retraction mode; the mode, the option
  and `create_jetstore_hdl_with_mode` are deleted, and every engine is built the one way.
  `legacy_mode_test.go` runs a test with no `legacy` object, with its switches `false`, and with one
  `true`, which is refused.
