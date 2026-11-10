---
title: The token diff
description: How the Python node keeps the difference between its operators and JetStore's measurable, why the check is pinned to a baseline set, and what a green result does not cover.
kind: explanation
sidebar:
  order: 1
---

**JetStore adds a built-in; the Python node does not have it, and nothing says so.** The cheap half of
that risk is a set subtraction: both sides enumerate their tokens in one place each, JetStore in
`jets/compute_pipes/cpipes_contract_data.go` and the node in `scope._REGISTRY`, so the difference is
measurable before either engine is deployed anywhere. `cpipes-node tokens` measures it
([Use the node](/docs/developer/tools/cpipes-node/use-the-node/#measure-the-token-diff)).

| kind | JetStore only | node only | both |
|---|---|---|---|
| input channel | 2 | 0 | 2 |
| pipe | 1 | 0 | 2 |
| transformation | **16** | **0** | **3** |

*Measured 2026-09-21. The table is documentation; the baseline file is the assertion.*

The exit codes are three and not two because a parser that stopped matching returns empty sets that
look exactly like JetStore having dropped every token it has, and that must not read as drift.

## Why it lives in this package

**Because of the dependency direction.** The subtraction needs both sides in one process: JetStore's
enumeration, which `cpipes_contract` can reach, and this node's registry, which it cannot.
`cpipes_node` depends on `cpipes_contract`, and the reverse would make the contract tooling need this
node installed in order to run at all. So this side of that arrow is the only place the measurement
can be made.

## Pinned to a baseline

**Neither of the two obvious postures works.** A check that failed on any difference would be red on
its first run over a gap of sixteen that is deliberate: a gate nobody can run green. A check that only
reports never sees the actual risk, which is not the gap but the gap *growing*. So the difference is
pinned, and `--check` fails only when it moves without the baseline moving with it. The pattern is the
documentation build's missing-glyph baseline, whose argument is that *a build that is red every day
for a known reason is a build people stop reading*.

**The baseline is a set and not a count**, which is where it departs from that pattern deliberately.
A count moves in one direction and can be satisfied by the wrong token: `aggregate` leaving JetStore
on the day `transmogrify` joined it nets to zero. A set moves in both directions and the failure can
name what moved, which is the only thing a reader can act on. The message is correspondingly a
**routing warning** rather than an assertion:

```
  transformation: `transmogrify` entered `JetStore only`
      JetStore declares it and this node does not. A step naming it must not
      be routed to the Python node: a `use_python_node_when` expression that
      evaluates true over such a step aborts the run at startup, naming
      the token. Either take it on -- one Operator subclass under
      cpipes_node/operators/ -- or keep it out of the Python arm's `when`
      expression.
```

## What a green result does not cover

**It is printed on every run**, by `token_diff.coverage()`, and that is the design rather than an
apology. A named mitigation is a claim about what a test detects, and this one's claim is narrow: it
compares token *names*, and says nothing about whether two implementations of one token agree. Both
engines implement `partition_writer`, and they once wrote different bytes for one document (see
[Outputs and side effects](/docs/developer/tools/cpipes-node/outputs-and-side-effects/)). Whoever reads
a green result is exactly the person who needs to know that. Comparing what the two engines write is
a byte-for-byte differential run, two engines, one document, a deployed run and a diff, which is a
project rather than a check.

Two further limits:

- **The JetStore side is the *contract's* universe**, what an author may write, which is one token
  wider than the engine's dispatch: `infer` is rewritten to `ollama` or `vllm` by
  `ResolveInferBackend` before the graph is built, so no dispatch case exists for it. That the
  dispatch and the contract agree on the rest is JetStore's own
  `TestBuiltinOperatorTypesMatchesTheDispatch`, not this check.
- **A site operator is in scope by registration on both sides**, so it appears here on neither.

## The mutation tests

**They are the point of `tests_token_diff.py`.** A baseline check that has never been seen to fail is
a claim about what it would detect. So a token is added to a copy of the Go enumeration, a token is
removed, a token in `both` is removed, and this node is made to take one on: four movements in three
directions, each asserted to name what moved. Three more assert that a token *count* is insensitive
to unrelated edits of the same Go file: adding field keys to another struct's entry, removing the
`CsvSourceSpec/csv_file` discriminator, and gofmt re-padding a field block.
