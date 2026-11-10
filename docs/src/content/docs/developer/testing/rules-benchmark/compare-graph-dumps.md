---
title: Compare graph dumps
description: Produce canonical graph dumps from a benchmark run or a rules unit test, and compare two trees of them with rules_bench_diff.
kind: how-to
sidebar:
  order: 3
---

## Produce the dumps

**From a benchmark run**: `rules_bench -dump_dir <dir>` writes them after each rule set of each
repetition (see the `-dump_dir` flag in the
[rules benchmark reference](/docs/developer/testing/rules-benchmark/reference/#command-line)). A run
with a dump is not a timing run.

**From a rules unit test**: `TestDumpRulesUnitTest` in `jets/rulesbench/graphdump` writes them, on the
C++ engine, through `RunRuleSets`:

```bash
LIB=<a build of libjets.so of this commit>/jets
CGO_CFLAGS="-I$PWD/jets" CGO_LDFLAGS="-L$LIB" go test -c -o graphdump.test ./jets/rulesbench/graphdump
LD_LIBRARY_PATH=$LIB WORKSPACES_HOME=<home> WORKSPACE=my_ws \
  GRAPHDUMP_TST=<home>/my_ws/jet_rules/unit_tests/my_test/my_test.tst.json \
  GRAPHDUMP_OUT=out/run1 ./graphdump.test -test.run '^TestDumpRulesUnitTest$'
```

## Compare two trees

Measure the noise floor first, run against run, then the change:

```bash
rules_bench_diff -a out/r01/native -b out/r02/native -scope self           # run against run
rules_bench_diff -a base/r01/native -b head/r01/native -json change.json   # before and after a change
```

The command exits `1` when a difference is unclassified or a file is in one tree only. Classify a
known difference by adding an entry to a list of your own and passing it with `-known`; the format,
and which differences belong in the built-in list, are in the
[graph-dump diff reference](/docs/developer/testing/rules-benchmark/graph-dump-diff/).
