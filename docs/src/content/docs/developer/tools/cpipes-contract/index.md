---
title: The compute pipes contract
description: What tools/cpipes_contract holds, the applicability matrix, its schema and the checks over it, and where to start reading.
kind: overview
sidebar:
  label: Overview
---

`tools/cpipes_contract` is the contract of the compute pipes configuration, the `.pc.json` file: the
applicability matrix, its schema, and the checks over it.

The contract is extracted from the code and from the workspaces into a reviewable matrix, and the
schema becomes a projection of it. This package holds the matrix and the machinery that keeps it
honest. The [compute pipes configuration reference](/docs/developer/reference/generated/pipeline-configuration/)
is generated from the same matrix, from `matrix/types.csv` and `matrix/fields.csv`.

```
matrix/types.csv        one row per addressable type (Go struct + discriminator value)
matrix/fields.csv       one row per field of one of those types - the matrix proper
matrix/constraints.csv  requirements spanning more than one field
cpipes_contract/        the schema as Pydantic models, the checks, the corpus walker, the harness
harness/                the Go runner: feeds synthesized configs through ValidatePipeSpecConfig
```

It is developer tooling: it is not copied into any image, and nothing on the compute pipes runtime
path depends on it.

## Where to start

- [The applicability matrix](/docs/developer/tools/cpipes-contract/matrix/) is the column reference,
  and the page to read first.
- [Why the contract is a matrix](/docs/developer/tools/cpipes-contract/design/) explains why
  reflection over the Go structs is not enough, which corpus the matrix is checked against, and the
  two documents the model describes.
- [How the matrix was built](/docs/developer/tools/cpipes-contract/matrix-findings/) records what
  extracting it found, with the dates of each measurement.
- [The harness](/docs/developer/tools/cpipes-contract/harness/) turns the matrix's rows into test
  results, so that reviewing a row means reading what the validator actually did.
- [The operator bundles](/docs/developer/tools/cpipes-contract/bundles/) are an authored layer over
  the matrix that narrows the schema a template hole offers;
  [why bundles exist](/docs/developer/tools/cpipes-contract/bundles-design/) gives the reasons and
  the measurements.
- [Projections](/docs/developer/tools/cpipes-contract/projections/) describes the user-flow documents
  generated from a pipeline template.
- [Run the contract checks](/docs/developer/tools/cpipes-contract/run-the-checks/) has the commands.
