---
title: Why the contract is a matrix
description: Why the compute pipes contract is extracted into a matrix rather than reflected from the Go structs, the corpus it is checked against, and the two documents and two artefacts it describes.
kind: explanation
sidebar:
  order: 1
---

## Applicability is the whole contract

The `omitzero` and `omitempty` tags in `jets/compute_pipes/pipes_model.go` mean **applicable**, not
**required**, so reflection over the Go structs recovers the field inventory and nothing about
applicability, and applicability is the whole contract. A JSON Schema listing fourteen optional
config objects under every `type` is not a weak schema, it is a useless one: it permits precisely the
configs a model is most likely to invent.

So the contract is extracted from the code and the corpus into a reviewable matrix, and the schema
becomes a projection of it.

## Which file is the source of truth

The source of truth for the *contract claims*, which fields exist per token (`applicable`),
`required`, `values`, `default` and `description`, is `cpipes_model.py`, the Pydantic model that was
first generated from the matrix and then adopted. Those columns of `fields.csv` are regenerated from
the model by `python -m cpipes_contract reflect`, and `reflect --check` is the divergence guard.

Everything else stays authoritative in the CSV files:

- the audit trail: evidence, citations, corpus counts, harness verdicts, review stamps, notes;
- the Go binding: `go_type`, `container`, `ref_struct`, `declared_in`;
- `types.csv` and `constraints.csv`;
- the rows the model cannot express: the merged `ExpressionNode` and `ContextSpec` claims, and
  fields inapplicable on every token of their struct.

The `generate` command is the bootstrap that first produced the model. Running it again overwrites
model edits.

## Three tables, not one

| File | One row per | What it is for |
|---|---|---|
| `matrix/types.csv` | **addressable type**: a Go struct, paired with one value of its discriminator when it has one | The unit of a `$defs` entry, of a Pydantic subclass, and of a fragment-library part |
| `matrix/fields.csv` | **field of an addressable type** | The matrix proper: applicability, requirement, default, evidence |
| `matrix/constraints.csv` | **requirement spanning more than one field** | What `(applicable, required)` cannot express |

The discriminator vocabulary, the corpus instance count and the exemplar are properties of a *type*
rather than of a field. Carried on every field row they would be repeated a dozen times and could
disagree row to row, which is two sources for one fact. `fields.csv` remains the review artefact.

### How exhaustive `types.csv` has to be

**Exhaustive.** A type row is not merely a place to put a `defs_name` that the Go name cannot carry:
it is the parent of its field rows, the anchor of the fragment library's exemplar, and the node the
corpus walker descends through. So:

> **Every struct reachable as a *value* gets a row: one per `(go_struct, type_token)` pair, and `*`
> is a full-standing token, not a placeholder.** A struct with no discriminator gets exactly one row.

Reachable *as a value* means some field points at it through `ref_struct`, plus the document root.
Two consequences that are easy to get wrong at extraction time:

- **Embedded structs get no row.** `FileConfig` is only ever embedded, never a named field, so
  nothing references it: its fields appear on the *host's* rows with `declared_in=FileConfig`. A
  `FileConfig/*` row would have no parent field, no exemplar and no corpus count.
- **Undiscriminated structs are not optional.** `OllamaSpec/*` is where the `ollama` exemplar lives,
  and it is the parent of 22 field rows. `defs_name` happens to be mechanical there; the other nine
  columns are not.

This is enforced, not merely intended. `check` fails with `no types row for X/Y` for any orphaned
field row; the corpus walker reports a struct with no row as `unreachable` and stops descending; and
`--strict` makes every unresolved `ref_struct` a failure.

`pipes_model.go` declares about seventy structs, of which about fourteen discriminate into roughly
fifty tokens between them, so expect **on the order of a hundred rows**, not a few dozen.
`constraints.csv` stays small.

## The corpus

The corpus, for this package, is **every `.pc.json` under `workspaces/` whose path has a
`pipes_config` directory in it**, at any depth (`config_files` in `cpipes_contract/corpus.py`), and
it subtracts nothing. Anything else under `workspaces/` is counted as retired and not walked: the
`.pc.json` files under `workspaces/*/data/` are developer reference material JetStore never loads,
and counting them manufactured contradictions with the validator.

A second corpus definition exists in the tree, in `jets/agentic/eval`, and the two answer different
questions:

| Tool | Rule | Files, 2026-11-10 |
|---|---|---:|
| `cpipes-contract corpus`, `check --corpus` (`config_files`) | every `.pc.json` under a `pipes_config` directory in `workspaces/` | **51** |
| `jets/agentic/eval` (`LoadCorpus`, `jetstoreOwnedAssets`) | `workspaces/*/pipes_config/**`, less the names in that directory's `jets_assets_manifest.json` | **46** |

**The five files between them are installed copies of JetStore's own two configs**
(`embed_input_parts.pc.json`, `jets_loader.pc.json`), which `install_workspace_assets` writes into
each workspace and the workspace repositories do not track. So this package's number depends on
whether the installer has run in the checkout: on a fresh clone, with nothing installed, the two rules
agree at the tracked count. The corpus grows from commits in the workspace repositories, so read any
count as a dated measurement; `cpipes-contract corpus` prints today's.

## Two documents, and only one of them is the schema

`ComputePipesConfig` is the *authored* document: what an author writes and what `check --corpus`
walks. `ComputePipesRuntimeConfig` is what a worker node is handed: the same fields plus
`common_runtime_args` and `pipes_config`, which the starters fill in and no author writes.

The matrix carries both as rows of one `go_struct` and marks the second pair `applicable=no`,
because `pipes_model.go` has one struct for both shapes. The emitted schema is the authored
projection only, and a case in `negative_suite.json` holds it to that.

The number of fields on each is a property of a struct under active development, so
`tests_runtime_document.py` asserts the *relation* rather than the counts: every json tag is a
runtime field, and what the runtime model adds is what the matrix calls inapplicable.

Until the runtime model existed, no class here accepted a runtime document at all: `extra="forbid"`
refused every document a node had ever been given, and the corpus walk could not see it, because a
node is never handed an authored document. **A green check over the wrong half of a document space**
is the failure to take from this, not a two-field oversight.

## Two artefacts, one contract

The package emits two readers of one contract, and `tests_schema.py` is where they are held to each
other. `cpipes_model.py` is the Pydantic model and the source of truth for the claims;
`cpipes_schema.json` is the projection a Go consumer and every typed template hole reads. They can
disagree silently, and did: the complement branch for site operators (see
[How the matrix was built](/docs/developer/tools/cpipes-contract/matrix-findings/#a-discrimination-that-is-a-complement))
reached the schema before it reached the model's `TransformationSpec` alias, and no document asked
both until one naming a site operator was authored.

A union carrying an `unlisted(...)` token is now emitted as an ordered union of the tagged members
and that branch, and `schema.splice_complement_branches` folds Pydantic's `anyOf` rendering of it
back into the single discriminated `oneOf`, so the emitted file is unchanged and the model validates
what the schema accepts.
