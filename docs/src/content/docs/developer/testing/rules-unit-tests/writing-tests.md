---
title: Writing rules unit tests
description: What a rules unit test author needs to know about how the engine asserts data, names test resources, fires rules by salience, and breaks ties.
kind: explanation
sidebar:
  order: 2
---

- **A `resource`-typed data property is asserted as text**, in production and therefore here. Write
  the expected object with type `text`. This is recorded, not fixed.
- **Name test resources so they cannot meet production ones.** A test's rule file is listed in
  `workspace_control.json`, so it compiles into the production `workspace.db` and ships with the
  workspace. Declare test-only classes, properties and resources under a prefix of the test's own,
  such as `ut_<test>:`. Never declare a production name in a test rule file.
- **`(jets:iState rdf:type jets:State)`** comes from `data_model/jets_model.jr`, as a `triple(...)`.
  A looping rule file that does not import that model must declare it itself.
- **A rule fires at the salience it declares.** Until 2026-10-11 rules with the same antecedents
  shared their last vertex and its one salience, so a declared salience could be overruled. The
  compiler now gives a rule with a different salience a vertex of its own, and warns when two such
  rules infer the same triple, because the lower one's salience then changes nothing about it.
- **Never let a `sorted_head` tie decide an outcome.** Ties break non-deterministically on the C++
  engine, as they did on Go. Give every candidate a distinct weight.
- **`is_debug`**:
  - logs each asserted row;
  - restarts the process with `GLOG_v=35` for the C++ trace (`INFER Vertex …` lines).

  The apiserver keeps the tail of the log.

The format of a test is in the [rules unit test reference](/docs/developer/testing/rules-unit-tests/reference/).
