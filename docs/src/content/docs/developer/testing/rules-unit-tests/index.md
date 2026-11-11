---
title: Rules unit tests
description: What a rules unit test is, how the apiserver and the rules_unit_test command run one, and where the code is.
kind: overview
sidebar:
  label: Overview
---

A rules unit test is a `.tst.json` file under a workspace's `jet_rules/unit_tests/`. It names a rule
process, one or more input sessions of CSV entities, and the triples the RDF session must and must not
hold after the rules ran. The `rules_unit_test` command runs **one** test and writes its **result
file**.

The apiserver runs it once per test, from a pool, and records the result
(`rules_unit_test_execution`, `rules_unit_test_check`; see the
[system tables](/docs/developer/reference/generated/system-tables/)). A rule author can run the same
binary from a shell against a checked-out workspace, with no apiserver and no database.

- **It opens no database connection.** It reads the compiled workspace from
  `$WORKSPACES_HOME/$WORKSPACE` and writes a file.
- **It builds one engine per run, in its own process**, and never shares an engine between tests.
  The meta graph locks when the first RDF session opens, so `rule_config` cannot change afterwards.

The types, validation and result file are in `jets/rulestest`. The runner is in
`jets/rulestest/harness`, and the command in `jets/cmds/rules_unit_test`.

## In this section

- [Run a rules unit test from a shell](/docs/developer/testing/rules-unit-tests/run-from-a-shell/).
- [Rules unit test reference](/docs/developer/testing/rules-unit-tests/reference/): the command line,
  the environment, the test definition, the checks, the results and the input CSVs.
- [Writing rules unit tests](/docs/developer/testing/rules-unit-tests/writing-tests/): what a test
  author needs to know about the engine.
- [Inside the harness](/docs/developer/testing/rules-unit-tests/harness-internals/): the order the
  harness does things in, and why.
