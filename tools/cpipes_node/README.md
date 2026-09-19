# cpipes_node

A compute pipes node in Python: the entry, the config load, the declared
operator scope, and a local driver.

The seam is three fields. `ComputePipesNodeArgs` is `{id, jp, pe}`
(`jets/compute_pipes/actions_common_model.go:46`) and everything else a node
needs it reads from `jetsapi.cpipes_execution_status` and from S3. That is what
makes a second implementation of the *runtime* tractable where a port of the
engine would not be: what must hold is the contract and the side effects.

The contract model is not re-derived. `tools/cpipes_contract/cpipes_model.py`
is the source of truth for the whole `.pc.json` and has a drift guard behind
it; this package imports it. See `cpipes_node/contract.py` for the three places
that model had to be widened and for what each widening is asserted against.

## The declared scope

**This node implements a subset and says so.** A `.pc.json` naming an operator
outside the subset aborts at startup naming the token and the scope searched —
never silently skipping it.

```
$ cpipes-node scope
$ cpipes-node check --config path/to/pipeline.pc.json
```

`check` exits 0 clean, **1** when the document names a token outside the scope
and **2** when it names one this node declares and has not built. Two codes,
because the two have different repairs: the first is the author's or a scope
decision, the second is a task in this package.

The subset is:

| kind | tokens |
|---|---|
| input channel | `generator`, `memory` |
| pipe | `fan_out`, `merge_files` |
| transformation | `map_record`, `filter`, `partition_writer` |
| transformation, by registration | whatever a deployment registers |

Seven declared tokens of the contract's twenty-six, plus the site operators a
deployment registers. **The declaration is the producer's own**: an in-scope
token is one `Operator` subclass under `cpipes_node/operators/`, and
`declared_scope()` walks the registry those classes put themselves into. There
is no list of tokens for an edit to widen.

Whether the subset is ever a parity commitment is not settled here. It is
`P9-I04`, the decision number reserved for it is `D-206`, and the phase charter
says in terms that it is not this phase's to answer.

## A deployment's own operator

The same shape as Go's `WithOperators`, and the same three rules — a built-in
wins, a colliding registration is kept and logged rather than refused, and an
unreachable registration is dropped:

The deployed entry is `awslambda.Node`, composed rather than imported ready
made, so that a deployment's operators reach the node the way Go's do — as an
argument:

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

Nothing in this package is named after any deployment, and nothing in it should
become so: a corpus-shaped helper added *here* rather than in the deployment's
own operator is the point at which the node stops being a JetStore capability.

## Running without AWS

`coordinate` takes its configuration from a `ConfigSource` and its objects from
an `ObjectStore`, and each has a local implementation. The local path is the
same `coordinate` with different arguments — not a second engine, which is what
makes a local measurement evidence about a deployed run.

```
$ cpipes-node run --config pipeline.pc.json --store ./bucket --id 0 --pe 1
```

What a local run does **not** cover: the lambda invocation, the
`cpipes_execution_status` read, S3 itself and its KMS settings, the state
machine's Map over partitions, and the six side-effect tables (P9-T09).

## Checks

```
$ python -m pytest -q          # 75 tests
$ ruff check . && ruff format --check .
$ mypy cpipes_node --ignore-missing-imports
```

Four tests read Go source as their oracle rather than transcribing it — the
argument struct's json tags, the `ComputePipesConfig` struct's tags, the
config `SELECT`, and the authored `.pc.json` corpus — so a rename on the other
side of the seam is caught here rather than at a deployment.

Developer tooling status: nothing on the cpipes runtime path depends on this
package yet; it becomes runtime the moment `graph.run` is implemented.
