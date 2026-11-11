---
title: "The pipeline configuration contract is a matrix"
description: "Why the contract of a compute-pipes configuration is extracted into a reviewable matrix rather than reflected from the Go structs."
kind: explanation
---

*Decided 2026-08-11.*

## Context

A pipeline configuration, a `.pc.json`, is read into the Go structs of
`jets/compute_pipes/pipes_model.go`. Most of their fields carry `omitempty` or `omitzero`, which mean
*applicable*, not *required*, and which fields apply depends on the operator's `type`. Reflecting over
the structs recovers the list of fields and nothing about which apply where. A JSON Schema built that
way lists every optional object under every type, and so accepts exactly the configurations an author,
or a model writing one, is most likely to invent.

## Decision

Extract the contract from the code and from the configurations the workspaces hold into a reviewable
matrix, `tools/cpipes_contract/matrix/`: `types.csv` (one row per addressable type), `fields.csv`
(one row per field of a type: applicability, requirement, default, evidence) and `constraints.csv`
(requirements that span fields). The JSON Schema and the Pydantic model are projections of it.

## Consequences

- Each claim in the matrix carries its evidence, citations and corpus counts, so a row can be
  reviewed rather than trusted.
- The Pydantic model, `cpipes_model.py`, is the source of truth for the contract columns of
  `fields.csv`, and `reflect --check` guards the two against diverging; the audit trail and the Go
  binding stay authoritative in the CSV files.
- The generated [pipeline configuration reference](/docs/developer/reference/generated/pipeline-configuration/)
  is rendered from the matrix, so the documentation and the contract are the same file.
- A new operator type or field is not done until its matrix rows are.

## Where the reason is recorded

- [Why the contract is a matrix](/docs/developer/tools/cpipes-contract/design/), *Applicability is the
  whole contract*.
- Commit `195390a9`, *Add the cpipes contract toolchain and the applicability matrix schema*.
