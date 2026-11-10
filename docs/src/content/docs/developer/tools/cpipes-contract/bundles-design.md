---
title: Why bundles exist
description: What the compute pipes contract's operator bundles are for, why nesting column operators are sub-typed rather than ranged, and what the layer was measured to save.
kind: explanation
sidebar:
  order: 5
---

The columns of the bundle files are in [the operator bundles](/docs/developer/tools/cpipes-contract/bundles/).
This page gives the reasons. The bundles were seeded on 2026-08-19 and authored and reviewed on
2026-08-20; the token figures below were measured then, over the fifteen operators of the day, and
are not re-measured here.

## One bundle per operator

**One bundle per operator is a deliberate choice, and it makes the `TransformationSpec` tier one to
one with the leaves**: those rows group nothing, they carry a range and a description. It is the
right shape when the *template* names the operator at each hole, which keeps a hole's scope as small
as it can be. What it forecloses is a hole that offers a **choice** among related operators; that
would need a second tier grouping the operators, and `role` (`theme` / `substrate`) is retained for
it even though nothing is `substrate` today. The semantic grouping does real work on the column side.

## What the layer is for

`TransformationSpec` is a flat union of operators, and a JSON Schema addressed at it measured 41,040
tokens against an inference-server budget of 24,576 (`jets/agentic/prompt` documents the
measurement). Narrowing the union does not fix that on its own: a union of *one* operator still
measured 28,700 tokens, because every leaf carries `conditional_config` →
`ConditionalTransformationSpec` → `TransformationSpecOverride`, which re-admits every operator's
config. The closure of a cycle does not care how the entry points are partitioned.

The fix is two changes that only work together: an abstraction layer between the root union and the
concrete leaves, **and** a range on each leaf's `columns` and `conditional_config` narrowing them to
that leaf's own bundle. Measured over three hand-ranged bundles: 16,985, 19,147 and 24,708 tokens,
the first two comfortably inside budget and the third at the line. Measured over all fifteen bundles:
**every bundle fits**, worst 15,562 (`AnalyzePipe`, 63% of budget), mean 10,671, best 7,271
(`DistinctPipe`).

**Those figures size the JSON Schema, and the JSON Schema never occupies the context.** Ollama
compiles `format` into a sampling grammar, at a measured 2 to 3 prompt tokens whether the schema is
5,252 or 41,029. The prompt is the constraint, and by that measure the flat union is about 9,090
tokens and **does** fit. Measured as assembled prompts *with* their few-shot examples, every bundle
still fits comfortably: worst `AnalyzePipe` at about 11,818 estimated and 13,501 measured, against
24,576. The layer's value is therefore semantic scoping and a two- to six-fold prompt saving, not
fit. Nothing in the mechanism changes; the justification does.

**A bundle is semantic, and the corpus cannot propose one.** With 45 live configs, `high_freq`,
`ollama`, `anonymize`, `distinct` and `shuffling` appeared in one or two files each, and
`clustering`, `avrg`, `max`, `min` and `multi_select` never appeared directly. At those counts the
corpus cannot distinguish *semantically excluded* from *nobody has written one yet*: it gives cases,
not rates. So bundles are authored from the operators' meaning, and the corpus is used only to
**falsify**: a bundle contradicted by a live config is wrong, but a bundle unattested by the corpus is
merely unattested.

## What it changes, and what it does not

Nothing on the runtime path. The Go side is one flat struct, `TransformationSpec` in
`jets/compute_pipes/pipes_model.go`, carrying every operator's config field, `ConditionalConfig` and
`When` together, and the Pydantic leaves are already a projection of the matrix rather than a mirror
of the Go types. The validator, the builder in `jets/compute_pipes/actions_start_common.go` and the
JSON wire format are untouched.

Nor does `jets/agentic/prompt` change: `Subschema` resolves any `$defs` entry, so a bundle is simply
another entry to address.

`_MATRIX_KEYS` in `cpipes_model.py` needed one new case. Every bundle *leaf* still maps to
`("TransformationSpec", <token>)`; only the abstract bundle nodes are new, and they have no Go struct
behind them, the same shape as the `embeds` rows in `types.csv`.

## The corpus, and the deleted clustering config

`clustering_test.pc.json` was the only config that ever used the `clustering` operator, and it was
deleted on 2026-08-16 because its `clustering_config` carried keys from a *retired revision* of
`ClusteringSpec`. So `clustering`'s zero is not "nobody has written one": it is "the only one ever
written targeted a spec that no longer exists", which means the operator's current shape has never
been exercised by any config. A bundle containing `clustering` is authored on semantics alone and
cannot be falsified by the corpus; its `notes` say so.

## Nesting column operators must be sub-typed, not merely ranged

A `TransformationColumnSpec` is reachable from **four** places, not one:

| Site | Field |
|---|---|
| an operator's payload | `TransformationSpec*.columns` |
| a case leg | `CaseExpression.then` |
| a case fallthrough | `TransformationColumnSpecCase.else_expr` |
| the two map-reduce phases | `TransformationColumnSpecMapReduce.apply_map`, `.apply_reduce` |

So `case` and `map_reduce` are **nesting** column operators: each re-admits the full column union
from inside whatever bundle contains it. Ranging an operator's `columns` while leaving those open
defeats the narrowing exactly where the narrowing was the point: the same cycle as
`conditional_config`, one level further down.

**And it is semantic before it is arithmetic.** A `case` inside an `aggregate` should offer the
aggregation column operators, not the mapping ones; `apply_reduce` should offer what can combine
partial results, which is not what `apply_map` offers. A shared `TransformationColumnSpecCase` entry
cannot say either thing, because one entry cannot range differently per host. Hence a sub-type per
bundle rather than a range applied to a common type.

The corpus says the same. Measured over the 45 live configs on 2026-08-19, the four sites held
different things:

| Site | What it actually contains |
|---|---|
| `case.then` | value 166, max 3, select 2, eval 2, min 2 |
| `case.else_expr` | value 15, eval 10, min 1, max 1, avrg 1 |
| `map_reduce.apply_map` | distinct_count 6, sum 4, count 2 |
| `map_reduce.apply_reduce` | **count 12, and nothing else** |

`apply_reduce` is the cleanest evidence in the corpus that the flat range is wrong: one operator,
twelve instances, no exceptions. The reduce phase combines partial counts and does nothing else.

**A walk that reads only `columns` undercounts by 115 of 2,073 instances** and reports three
operators at zero that are not. `avrg`, `max` and `min` were attested only inside a `case` under
`aggregate`, 1, 4 and 3 instances, all in one workspace's data-profiling pipeline, which selects
`min` or `max` by a `minmax_type` and `avrg` by a distinct-count threshold. `count`, `sum` and
`distinct_count` were undercounted by 14, 4 and 6 through the map-reduce phases. **`multi_select` was
the only column operator with no live instance anywhere.**

What sub-typing is worth, measured on the Aggregation bundle against a 24,576-token budget:

| | ~tokens |
|---|---|
| flat `TransformationSpec` union, nothing ranged | 41,040 |
| bundled but nothing ranged | 30,606 |
| `columns` + `conditional_config` ranged, nesting open | 16,363 |
| **all four sites ranged** | **15,277** |

The last step is worth 1,086 tokens on this bundle and more on wider ones, but the reason to take it
is the semantic one, not the 6.6%. With every site ranged, a recursive bundle's measured closure was
34 of 144 definitions.

Run against the authored files on 2026-08-20, the corpus check reported **45 configs, 2,073 column
instances, 0 violations**, every pipe operator in a bundle and every column operator in a bundle.

## The bundles added since

The `InferPipe` bundle came with the `infer` operator on 2026-09-05, and `VllmPipe` and `EmbedPipe`
on 2026-09-10, after which every token `TransformationSpec`'s discriminator admits had a bundle.

**What the gap was.** The *flat* tier admitted `vllm` and `embed` from the day each operator's leaf
rows were extracted: both are in `TransformationSpec`'s discriminator mapping, and `validate` was
green over the corpus throughout. What admitted neither was the **bundle** tier, which is the tier an
authoring hole binds (`Hole.schema_ref` names a bundle) and the tier `bundles` checks the corpus
against. So the operators were authorable in a hand-written `.pc.json` and unreachable from a
template hole.

**It was invisible until an asset install made it visible.** `bundles` can only falsify: it walks
live configs, so an operator no `.pc.json` uses is an operator it never asks about. `embed` became
visible on 2026-09-08, when JetStore's own `pipes_config/embed_input_parts.pc.json` was installed into
the workspaces and the command began reporting `operator 'embed' is in no bundle`. `vllm` was still
unused and would have stayed invisible. So `tests_bundles.py` holds the rule the corpus cannot:
`test_every_admitted_operator_has_a_bundle` would have caught both on the day their leaf rows landed.
The rest of that file is the negative half: a bundle is a `deepcopy` of its leaf with two properties
retargeted, so the way it goes wrong is by admitting *more* than the leaf did, and a bundle that
admitted everything would pass both the corpus check and the authorability check.

**`RenderPipe` is the twenty-third, added on 2026-09-11 with the `render` operator.** It is the first
bundle whose `columns_range` is `-` for a reason other than the operator having no `columns` field in
Go: the render operator carries the key and the matrix marks it inapplicable, because the operator
writes one column named by `render_config.output_column` and runs no column transformations. The
emitter treats the two the same, popping `columns` from the bundle and from its override, so the
distinction is one for a reader of `fields.csv` rather than one the schema can see.

The check that found it owed a row was `cpipes-contract bundles`, by name: a live `render` fragment
in a pipeline template reported *operator 'render' is in no bundle*. Nothing in `check` or `schema`
would have: the emitter fails on a bundle naming an unknown token and not on a token named by no
bundle, so *one bundle per operator* is an invariant the corpus check and the tests enforce and the
emitter does not.
