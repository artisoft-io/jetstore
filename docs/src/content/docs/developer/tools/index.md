---
title: Tools
description: The developer tools beside the code, the compute pipes configuration contract and the Python compute pipes node.
kind: overview
sidebar:
  label: Overview
---

The `tools/` folder of the `jetstore` repository holds developer tooling. None of it is copied into
an image, and nothing on the compute pipes runtime path depends on it.

## The compute pipes contract

`tools/cpipes_contract` is the contract of the `.pc.json` pipeline configuration: an applicability
matrix extracted from the code and the workspaces, the JSON Schema and Pydantic model projected from
it, and the checks that keep the three in step. The
[compute pipes configuration reference](/docs/developer/reference/generated/pipeline-configuration/)
is generated from the matrix when this site is built.

- [The compute pipes contract](/docs/developer/tools/cpipes-contract/): what the package holds, and
  where to start.
- [Why the contract is a matrix](/docs/developer/tools/cpipes-contract/design/): the reasoning, the
  corpus it is checked against, and the two documents it describes.
- [The applicability matrix](/docs/developer/tools/cpipes-contract/matrix/): every column of
  `types.csv`, `fields.csv` and `constraints.csv`.
- [How the matrix was built](/docs/developer/tools/cpipes-contract/matrix-findings/): the corpus, the
  seed rows, and what extracting and checking the matrix found.
- [The harness](/docs/developer/tools/cpipes-contract/harness/): how each row of the matrix becomes a
  test result.
- [The operator bundles](/docs/developer/tools/cpipes-contract/bundles/) and
  [why bundles exist](/docs/developer/tools/cpipes-contract/bundles-design/).
- [Projections](/docs/developer/tools/cpipes-contract/projections/): the user-flow documents the
  generator emits from a pipeline template.
- [Run the contract checks](/docs/developer/tools/cpipes-contract/run-the-checks/).

## The Python compute pipes node

`tools/cpipes_node` is a compute pipes node written in Python: a second implementation of the
runtime, for a declared subset of the operators, with a local driver.

- [The Python compute pipes node](/docs/developer/tools/cpipes-node/): the seam it implements and
  the subset it declares.
- [Use the node](/docs/developer/tools/cpipes-node/use-the-node/): the commands, registering a
  deployment's own operator, running without AWS, and the test suite.
- [The token diff](/docs/developer/tools/cpipes-node/token-diff/): how the node's operators are kept
  in view of JetStore's.
- [The execution model](/docs/developer/tools/cpipes-node/execution-model/): the channel graph, the
  `when` subset, `conditional_config` and `merge_files`.
- [Outputs and side effects](/docs/developer/tools/cpipes-node/outputs-and-side-effects/): where
  files land, under which partition and name, and what the node writes to the database.
