---
title: Use the node
description: Check a pipeline configuration against the Python node's scope, measure the token diff, register a deployment's own operator, run a pipeline without AWS, and run the node's tests.
kind: how-to
sidebar:
  order: 0
---

The commands run from `tools/cpipes_node`.

## Check a document against the scope

```
$ cpipes-node scope
$ cpipes-node check --config path/to/pipeline.pc.json
```

`scope` prints the declared tokens and which are implemented. `check` exits **0** clean, **1** when
the document names a token outside the scope and **2** when it names one this node declares and has
not built. Two codes, because the two have different repairs: the first is the author's or a scope
decision, the second is work in this package.

## Measure the token diff

```
$ cd tools/cpipes_node && uv run cpipes-node tokens            # the three sets
$ cd tools/cpipes_node && uv run cpipes-node tokens --check    # ...against the baseline
```

One command, no arguments: the JetStore tree is found from this package's own location, and the
baseline ships beside the code at `cpipes_node/token_diff_baseline.json`. Exit **0** clean, **4** when
the difference has moved, **3** when the instrument could not measure. Move the baseline deliberately,
with `--write-baseline YYYY-MM-DD` and the reason in the commit message. What the diff does and does
not cover is in [the token diff](/docs/developer/tools/cpipes-node/token-diff/).

## Register a deployment's own operator

The same shape as Go's `WithOperators`, and the same three rules: a built-in wins, a colliding
registration is kept and logged rather than refused, and an unreachable registration is dropped.

The deployed entry is `awslambda.Node`, composed rather than imported ready made, so that a
deployment's operators reach the node the way Go's do, as an argument:

```python
from cpipes_node import Registry
from cpipes_node.awslambda import Node

node = Node(site_operators=Registry().with_operators({"my_operator": my_factory}))
handler = node.handler
```

and the same call, spelled out:

```python
from cpipes_node import NodeArgs, Registry
from cpipes_node.config import ExecutionStatusConfigSource
from cpipes_node.node import coordinate
from cpipes_node.store import S3

coordinate(
    NodeArgs(**event),
    ExecutionStatusConfigSource(connection),
    store=S3(bucket=settings.bucket, region=settings.region),
    site_operators=Registry().with_operators({"my_operator": my_factory}),
)
```

Nothing in this package is named after any deployment, and nothing in it should become so: a helper
shaped for one deployment's data added *here* rather than in the deployment's own operator is the
point at which the node stops being a JetStore capability.

## Run without AWS

`coordinate` takes its configuration from a `ConfigSource` and its objects from an `ObjectStore`, and
each has a local implementation. The local path is the same `coordinate` with different arguments,
not a second engine, which is what makes a local measurement evidence about a deployed run.

```
$ cpipes-node run --config pipeline.pc.json --store ./bucket --id 0 --pe 1
$ cpipes-node run ... --bucket corpus-out=./other-bucket   # repeatable
```

`--bucket NAME=DIR` is a directory standing in for an **external** bucket, the one a document names
in `output_channel.bucket`. It is separate from `--store` and not a default for it, because a local
run that quietly wrote another account's bucket under this one would pass every byte comparison while
hiding the single thing a deployed run gets wrong. A document naming a bucket with no mapping for it
is **refused**, naming the bucket and the mapping.

It prints the run's figures **per channel rather than as a total**, because a total is satisfied by
the right number of rows in the wrong channels, which is exactly the failure a twelve-output step can
have.

**The `run` subcommand registers no site operators** (`site.EMPTY`), so a document naming a
deployment's own operator can only be *refused*. A document made of `map_record` and `filter` runs to
completion through it. A driver that runs a pipeline using site operators has to be handed a
registry; the composition to copy is the `coordinate(...)` call above.

What a local run does **not** cover: the Lambda invocation, the `cpipes_execution_status` read, S3
itself and its KMS settings, the state machine's Map over partitions, and **any side-effect row**:
`run` passes no connection, so `side_effects.NONE` records the run and records nothing.

## Run the checks

```
$ python -m pytest -q
$ ruff check . && ruff format --check .
$ mypy cpipes_node --ignore-missing-imports
```

**Many tests read Go source as their oracle rather than transcribing it**: the argument struct's json
tags, the `ComputePipesConfig` struct's tags, the config `SELECT`, the authored `.pc.json` corpus,
the `OperatorEnv` interface, the operator table, the expression leaf-type switch, the csv quoting
rule's clauses, the partition file name, the header condition, the merge's six-arm header switch, the
SQL statements, the `jetstore_bucket` sentinel, `OutputFileSpec`'s accessors and more. So a rename on
the other side of the seam is caught here rather than at a deployment. They call
`conftest.go_source`; how many there are is a count no test asserts yet, so any figure in prose goes
stale.

**One of them is asserted against a Go test that fails**, deliberately: `TestEncodeRdfTypeToTxt`
expects `2006-01-02T00:00:00` where `encodeRdfTypeToTxt` returns `2006-01-02`, so the *test* is stale
about the *function*. The function is what the engine runs and is what this node mirrors; the
disagreement is pinned from both sides, so whichever one somebody repairs, this goes red saying which.
