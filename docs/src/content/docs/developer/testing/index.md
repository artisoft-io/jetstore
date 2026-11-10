---
title: Testing
description: How JetStore's rules, rule engine and web application are tested, the rules unit-test harness, the rules benchmark and its graph-dump diff, and the web app's development and tests.
kind: overview
sidebar:
  label: Overview
---

This section covers the test and measurement tools that sit beside JetStore's code.

## Rules unit tests

A rules unit test is a `.tst.json` file in a workspace: a rule process, input sessions of CSV
entities, and the triples the session must and must not hold after the rules ran. The apiserver runs
them from the Workspaces screen; `jets/cmds/rules_unit_test` runs one from a shell.

- [Rules unit tests](/docs/developer/testing/rules-unit-tests/): what a test is and how it runs.
- [Run a rules unit test from a shell](/docs/developer/testing/rules-unit-tests/run-from-a-shell/).
- [Rules unit test reference](/docs/developer/testing/rules-unit-tests/reference/): the command
  line, the test definition, the checks, the results and the input CSVs.
- [Writing rules unit tests](/docs/developer/testing/rules-unit-tests/writing-tests/) and
  [inside the harness](/docs/developer/testing/rules-unit-tests/harness-internals/).

## The rules benchmark

`jets/cmds/rules_bench` runs a set of rule sessions on the C++ engine along the path a pipeline takes,
and writes one JSON result per run; `jets/cmds/rules_bench_diff` compares the graph dumps two runs
wrote.

- [The rules benchmark](/docs/developer/testing/rules-benchmark/): what it measures.
- [Build and run the benchmark](/docs/developer/testing/rules-benchmark/build-and-run/): building
  `libjets.so` and the command, the run protocol, profiling, and checking the harness.
- [Rules benchmark reference](/docs/developer/testing/rules-benchmark/reference/): the command line,
  the bench spec, and what a result holds.
- [How the benchmark measures](/docs/developer/testing/rules-benchmark/measurement/): the two
  releases, the counters, concurrency and profiling.
- [Compare graph dumps](/docs/developer/testing/rules-benchmark/compare-graph-dumps/) and the
  [graph-dump diff reference](/docs/developer/testing/rules-benchmark/graph-dump-diff/).

## The web application

`jetsclient_ide` is JetStore's web application, React and TypeScript, served by the apiserver.

- [The web application](/docs/developer/testing/web-app/): what it is and how it is laid out.
- [Develop and test the web app](/docs/developer/testing/web-app/develop-and-test/): build, test,
  and run it against a local apiserver.
- [Web app reference](/docs/developer/testing/web-app/reference/): the source layout and the server
  contract.
- [How the web app is served and built](/docs/developer/testing/web-app/serving/).
- [Validating action documents](/docs/developer/testing/web-app/action-documents/): what the
  checks on a `.ua.json` guarantee, and what they do not.

## Other tests

The C++ engine's tests and the Go tests are run with CMake's `ctest` and `go test`, as the
repository's `CLAUDE.md` describes. A test that reads a workspace, or any folder outside its Go
module, must run with `-count=1`: the test cache does not see a change there.
