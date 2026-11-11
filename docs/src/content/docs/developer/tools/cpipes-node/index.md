---
title: The Python compute pipes node
description: What tools/cpipes_node is, the three-field seam it implements, the contract model it imports, and the subset of operators it declares.
kind: overview
sidebar:
  label: Overview
---

`tools/cpipes_node` is a compute pipes node in Python: the entry, the config load, the declared
operator scope, the channel graph, and a local driver.

**The seam is three fields.** `ComputePipesNodeArgs` is `{id, jp, pe}`
(`jets/compute_pipes/actions_common_model.go`), and everything else a node needs it reads from
`jetsapi.cpipes_execution_status` and from S3. That is what makes a second implementation of the
*runtime* tractable where a port of the engine would not be: what must hold is the contract and the
side effects.

**The contract model is not re-derived.** `tools/cpipes_contract/cpipes_model.py` is the source of
truth for the whole `.pc.json` and has a drift guard behind it (see
[the compute pipes contract](/docs/developer/tools/cpipes-contract/)); this package imports it, and
**no longer widens it anywhere**. It once carried four widenings: three for the site operator the
contract's transformation union omitted, deleted when the omission was closed in the contract, and
one for the two fields a *starter* fills in, retired when the contract gained
`ComputePipesRuntimeConfig`. A widening here is a second reader of one rule, so the test pinning each
was written to go red and name what to delete. The runtime-fields guard was the exception: it pinned
the *difference* between the two models, and the repair kept that difference on purpose, so the
guard stayed green through the change it was written to announce. It now asserts the ownership rather
than the difference (`cpipes_node/contract.py`, `tests_config.py`).

## The declared scope

**This node implements a subset and says so.** A `.pc.json` naming an operator outside the subset
aborts at startup naming the token and the scope searched, never silently skipping it.

| kind | tokens |
|---|---|
| input channel | `generator` ✓, `memory` ✓ |
| pipe | `fan_out` ✓, `merge_files` ✓ |
| transformation | `map_record` ✓, `filter` ✓, `partition_writer` ✓ |
| transformation, by registration | whatever a deployment registers |

✓ is implemented; anything else declared is owed, and `cpipes-node scope` prints what owes each one.
**Implemented is derived and not declared**: a token is implemented exactly when its class defines
`build`, so a stub cannot claim to be finished and a finished operator cannot be left marked as a
stub.

Seven declared tokens of the contract's twenty-six, plus the site operators a deployment registers.
**The declaration is the producer's own**: an in-scope token is one `Operator` subclass under
`cpipes_node/operators/`, and `declared_scope()` walks the registry those classes put themselves into.
There is no list of tokens for an edit to widen. Whether the subset is ever a parity commitment is
not settled.

## Status

Nothing on the compute pipes runtime path depends on this package, and the Go engine is untouched by
it. It becomes a deployment's runtime when the CDK deployment can select a Python worker.

## In this section

- [Use the node](/docs/developer/tools/cpipes-node/use-the-node/): check a document against the
  scope, measure the token diff, register a deployment's own operator, run without AWS, and run the
  tests.
- [The token diff](/docs/developer/tools/cpipes-node/token-diff/): how a built-in JetStore adds
  becomes visible here.
- [The execution model](/docs/developer/tools/cpipes-node/execution-model/): the channel graph, the
  `when` subset, `conditional_config` and `merge_files`.
- [Outputs and side effects](/docs/developer/tools/cpipes-node/outputs-and-side-effects/): where a
  file lands, under which partition and name, and what the node writes to the database.
