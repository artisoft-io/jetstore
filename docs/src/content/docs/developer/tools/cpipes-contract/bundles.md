---
title: The operator bundles
description: The columns of the compute pipes contract's bundle files, bundles.csv, bundle_members.csv and bundle_evidence.csv, and how a range on a nesting operator is written.
kind: reference
sidebar:
  order: 4
---

The bundle layer is an authored layer of the compute pipes contract, in three files beside the
[applicability matrix](/docs/developer/tools/cpipes-contract/matrix/). `bundles.csv` carries
twenty-three bundles: one per `TransformationSpec` operator, plus `ColumnMapping`,
`ColumnAggregation`, and the two map-reduce phase bundles. Why the layer exists, and what it was
measured to save, is in [Why bundles exist](/docs/developer/tools/cpipes-contract/bundles-design/).

`-` marks not-applicable throughout, consistent with the other matrix sheets.

## Three files

| File | One row per | Authored or measured |
|---|---|---|
| `matrix/bundles.csv` | bundle | **authored** |
| `matrix/bundle_members.csv` | (bundle, operator) pair | **authored** |
| `matrix/bundle_evidence.csv` | operator | **measured**: a snapshot of the corpus, never hand-edited |

The split matters for the same reason the matrix's does: the authored files carry a claim about
meaning that no tool can recover, and the measured file is what checks it. The model is the source of
truth for contract *claims*; the bundle layer is authoritative in its CSV files because it expresses
semantics the model cannot express, as `constraints.csv` does.

## `bundles.csv`

| Column | Meaning |
|---|---|
| `bundle` | The abstract type's name. Becomes a `$defs` entry between the root union and the leaves. |
| `applies_to` | `TransformationSpec` or `TransformationColumnSpec`. |
| `description` | **The column only a person can fill.** What intent the bundle serves: the sentence that says why these operators belong together and the others do not. It becomes the abstract type's schema `description`, so a model generating into a hole reads it. |
| `status` | See *Status* below. |
| `notes` | Audit trail: why this boundary and not another; anything the corpus says that argues against it. |

## `bundle_members.csv`

| Column | Meaning |
|---|---|
| `bundle` | Names a row of `bundles.csv`. |
| `type_token` | The operator's discriminator value: `map_record`, `aggregate`, `count`. Must exist in `types.csv`. |
| `role` | `theme` or `substrate`. Substrate is an operator that belongs to a bundle without being what the bundle is *about*: `map_record` and `partition_writer` were in 38 and 35 of 45 files on 2026-08-19, and would be in nearly every bundle. Keeping the distinction is what stops "the bundle is about mapping" from being diluted by the two operators that are in everything. Nothing is `substrate` today. |
| `columns_range` | The `TransformationColumnSpec` bundle this operator's `columns` admits, or `-` where `columns` is inapplicable. |
| `conditional_range` | The `TransformationSpec` bundle this operator's `conditional_config` may override to. Usually the operator's own bundle; that is the change that breaks the cycle. |
| `status` | See *Status* below. |
| `notes` | Audit trail. |

An operator appears once per bundle it joins, so `map_record` may have several rows.

### Status

The column was designed to take `example` (seeded, delete), `todo` or `authored`. **The rows in both
files take `reviewed` or `unreviewed`.** Neither file carries a `reviewed_hash`, so nothing here is
self-certifying a review: the stamp mechanism covers `types.csv`, `fields.csv` and `constraints.csv`
alone.

## `bundle_evidence.csv` (measured, do not hand-edit)

Measured on 2026-08-19 over the 45 live configs of the day, `workspaces/*/pipes_config/**` per
`cpipes_contract/corpus.py`. The `.pc.json` files under `workspaces/*/data/` are excluded: JetStore
never loads them, and counting them manufactures contradictions with the validator.

**No command reads or writes this file**, so it is a hand-seeded snapshot of 2026-08-19 that has not
been refreshed, and it has no row for the operators added since (`infer`, `vllm`, `embed`, `render`).
Adding rows by hand would contradict its own header; a generator for it does not exist yet.

| Column | Meaning |
|---|---|
| `go_struct`, `type_token` | The operator, keyed as `types.csv` keys it. |
| `corpus_files`, `corpus_prod_files` | Files using the operator; the second excludes `test/`. A bundle attested only under `test/` is weaker evidence. |
| `corpus_instances` | Instance count: the useful figure for column operators, where files are not the unit. |
| `instances_direct`, `instances_in_case`, `instances_in_map_reduce` | Where those instances sit. An operator attested only under a nesting site is a different claim from one attested in an operator's `columns`: `avrg`, `max` and `min` appear only inside a `case`, and a third of `count`'s instances are a map-reduce phase. |
| `nests` | The nested column-list fields this operator carries, or `-`. Only `case` and `map_reduce` are non-empty: these are the two needing a sub-type per bundle. |
| `observed_hosts` | For a column operator, which pipe operators host it and how often. The evidence for `columns_range`. |
| `observed_columns`, `observed_columns_in_case` | For a pipe operator, the column operators seen in its `columns`, and those seen *only* nested inside a `case` under it. |
| `columns_applicable` | From `fields.csv`. `no` means `columns_range` must be `-`. A range set on such an operator is not merely redundant: it reaches the column bundle through `TransformationSpecOverride.columns` and costs about 4,000 tokens. |

**Read `observed_hosts` before ranging anything.** In the 2026-08-19 snapshot, six column operators
were hosted by exactly one pipe operator in every live instance (`count`, `sum`, `distinct_count`
and `map_reduce` by `aggregate`; `map` and `lookup` by `map_record`), and four more were genuinely
shared and belong in a core bundle. Read the nesting columns alongside them: an operator whose
instances are all nested is evidence about a nested range, not about an operator's `columns`.

## How a nesting range is written

`columns_range` on a member row carries what that member admits *inside itself*. For a pipe operator
that is its `columns` property, so a bare bundle name suffices. For a nesting column operator the
fields are named explicitly, because `map_reduce`'s two phases differ:

```
ColumnAggregation,case,theme,then=ColumnAggregation;else_expr=ColumnAggregation,-,...
ColumnAggregation,map_reduce,theme,apply_map=ColumnMapPhase;apply_reduce=ColumnReducePhase,-,...
```

A bundle whose `case` legs range to that same bundle is recursive, which is intended and stays
bounded.

## The checks

- **`cpipes-contract bundles`** checks the corpus against the authoring: for every live config, every
  (pipe operator, column operator) pair must be admitted by the authoring, and every operator must
  belong to at least one bundle. A violation means the authoring is wrong or the config is, and
  `corpus_prod_files` says which of those to believe first.
- **`test_every_admitted_operator_has_a_bundle`** in `tests_bundles.py` compares the discriminator
  mapping against `bundle_members.csv` and fails on any token in no bundle, which the corpus check
  cannot do for an operator no config uses yet.
- **`test_every_bundle_is_authorable`** in `tests_template.py` reads `bundles.csv` and re-checks the
  prompt-size budget over every bundle on every run.
