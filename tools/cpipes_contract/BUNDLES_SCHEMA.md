# The cpipes operator bundles — schema

**Seeded 2026-08-19 · authored and reviewed by Michel 2026-08-20 · three files in `matrix/`, two authored and one measured**

The layer is authored. `bundles.csv` carries ~~**seventeen**~~ ~~**twenty-two**~~ **twenty-three** bundles: one per
`TransformationSpec` operator, plus `ColumnMapping`, `ColumnAggregation`, and the two map-reduce
phase bundles. *(Seventeen was right on 2026-08-20 and the invariant it states is what moved the
number: `infer`, `vllm` and `embed` were built afterwards, and one bundle per operator means three
more bundles. See* The three infer-family bundles *at the end.)*

**One bundle per operator is a deliberate choice and it makes the `TransformationSpec` tier 1:1 with
the leaves** — those rows group nothing, they carry a range and a description. It is the right shape
when the *template* names the operator at each hole, which keeps a hole's scope as small as it can
be. What it forecloses is a hole that offers a **choice** among related operators; that would need a
second tier grouping these fifteen, and `role` (`theme` / `substrate`) is retained for it even though
nothing is `substrate` today. The semantic grouping does real work on the column side.

Measured over all fifteen, against the 24,576-token budget: **every bundle fits**, worst 15,562
(`AnalyzePipe`, 63% of budget), mean 10,671, best 7,271 (`DistinctPipe`). The flat
`TransformationSpec` union it replaces was 41,040 and fitted nowhere.

> **Correction, 2026-08-20 (Q-15).** Those figures size the *JSON Schema*, and the JSON Schema never
> occupies the context: Ollama compiles `format` into a sampling grammar, at a measured 2–3 prompt
> tokens whether the schema is 5,252 or 41,029. The prompt is the constraint, and by that measure the
> flat union is ~9,090 tokens and **does** fit — so "fitted nowhere" is wrong as stated, and the
> section below argues from a premise that did not hold. Measured as assembled prompts *with* their
> few-shot examples, every bundle still fits comfortably: worst `AnalyzePipe` at ~11,818 estimated
> and 13,501 measured, against 24,576. The layer's value is therefore semantic scoping and a two- to
> six-fold prompt saving, not fit. Nothing in the mechanism changes; the justification does.

`-` marks not-applicable throughout, consistent with the other matrix sheets.

## What the layer is for

`TransformationSpec` is a flat union of fifteen operators, and a schema addressed at it is 41,040
tokens against an infer-server budget of 24,576 (`jets/agentic/prompt` documents the measurement).
Narrowing the union does not fix that on its own: a union of *one* operator still measures 28,700
tokens, because every leaf carries `conditional_config` → `ConditionalTransformationSpec` →
`TransformationSpecOverride`, which re-admits every operator's config. The closure of a cycle does
not care how the entry points are partitioned.

The fix is two changes that only work together — an abstraction layer between the root union and the
concrete leaves, **and** a range on each leaf's `columns` and `conditional_config` narrowing them to
that leaf's own bundle. Measured over three hand-ranged bundles: 16,985, 19,147 and 24,708 tokens,
the first two comfortably inside budget and the third at the line.

**A bundle is semantic, and the corpus cannot propose one.** With 45 live configs, `high_freq`,
`ollama`, `anonymize`, `distinct` and `shuffling` appear in one or two files each, and `clustering`,
`avrg`, `max`, `min` and `multi_select` never appear at all. At those counts the corpus cannot
distinguish *semantically excluded* from *nobody has written one yet* — which is Decision 13's
cases-not-rates rule arriving in a new place. So bundles are authored from the operators' meaning,
and the corpus is used only to **falsify**: a bundle contradicted by a live config is wrong, but a
bundle unattested by the corpus is merely unattested.

## What this changes, and what it does not

Nothing on the runtime path. The Go side is one flat struct — `TransformationSpec` at
`jets/compute_pipes/pipes_model.go:638` carries all sixteen config fields, `ConditionalConfig` and
`When` together — and the Pydantic leaves are already a projection of the matrix rather than a mirror
of the Go types. The validator, the builder at `jets/compute_pipes/actions_start_common.go:692` and
the JSON wire format are untouched.

Nor does `jets/agentic/prompt` change: `Subschema` resolves any `$defs` entry, so a bundle is simply
another entry to address.

`_MATRIX_KEYS` in `cpipes_model.py` needs one new case. Every bundle *leaf* still maps to
`("TransformationSpec", <token>)` exactly as now; only the abstract bundle nodes are new, and they
have no Go struct behind them — the same shape as the twelve `embeds` rows already in `types.csv`.

## Three files

| File | One row per | Authored or measured |
|---|---|---|
| `matrix/bundles.csv` | bundle | **authored** |
| `matrix/bundle_members.csv` | (bundle, operator) pair | **authored** |
| `matrix/bundle_evidence.csv` | operator | **measured** — regenerated from the corpus, never hand-edited |

The split matters for the same reason the matrix's does: the authored files carry a claim about
meaning that no tool can recover, and the measured file is what checks it. Per the B.10 flip, the
model is the source of truth for contract *claims*; the bundle layer is CSV-authoritative because it
expresses semantics the model cannot express, exactly as `constraints.csv` does.

## `bundles.csv`

| Column | Meaning |
|---|---|
| `bundle` | The abstract type's name. Becomes a `$defs` entry between the root union and the leaves. |
| `applies_to` | `TransformationSpec` or `TransformationColumnSpec`. |
| `description` | **The column only you can fill.** What intent the bundle serves — the sentence that says why these operators belong together and the others do not. It becomes the abstract type's schema `description`, so a model generating into a hole reads it. |
| `status` | `example` (seeded, delete), `todo`, or `authored`. |
| `notes` | Audit trail: why this boundary and not another; anything the corpus says that argues against it. |

## `bundle_members.csv`

| Column | Meaning |
|---|---|
| `bundle` | Names a row of `bundles.csv`. |
| `type_token` | The operator's discriminator value — `map_record`, `aggregate`, `count`. Must exist in `types.csv`. |
| `role` | `theme` or `substrate`. Substrate is an operator that belongs to a bundle without being what the bundle is *about*: `map_record` and `partition_writer` are in 38 and 35 of 45 files and will be in nearly every bundle. Keeping the distinction is what stops "the bundle is about mapping" from being diluted by the two operators that are in everything. |
| `columns_range` | The `TransformationColumnSpec` bundle this operator's `columns` admits, or `-` where `columns` is inapplicable. `bundle_evidence.csv` pre-fills which those are — six of fifteen. |
| `conditional_range` | The `TransformationSpec` bundle this operator's `conditional_config` may override to. Usually the operator's own bundle; that is the change that breaks the cycle. |
| `status` | `example`, `todo`, `authored`. |
| `notes` | Audit trail. |

An operator appears once per bundle it joins, so `map_record` will have several rows.

## `bundle_evidence.csv` (measured — do not hand-edit)

Measured 2026-08-19 over the 45 live configs, `workspaces/*/pipes_config/**` per
`cpipes_contract/corpus.py`. The `.pc.json` under `workspaces/*/data/` are excluded: JetStore never
loads them, and counting them manufactures contradictions with the validator.

**The corpus was 49 when `README.md` was written and is 45 now**, and the difference is four
deliberate deletions on 2026-08-16, all flagged by the B.7 corpus baseline as unloadable (I-15):
`clustering_test.pc.json` and `csv_test.pc.json` from `cedargate_ws` (`d253af30`), and
`test/jetrules_test.pc.json` and `test/jetrules_grouped_test.pc.json` from `usi_ws` (`7ea23f0`). The
README's count is stale, not wrong-at-the-time.

**One of those deletions changes how a row of this table reads.** `clustering_test.pc.json` was the
only config that ever used the `clustering` operator, and it was deleted because its
`clustering_config` carried keys from a *retired revision* of `ClusteringSpec`. So `clustering`'s
zero is not "nobody has written one" — it is "the only one ever written targeted a spec that no
longer exists", which means the operator's current shape has never been exercised by any config.
A bundle containing `clustering` is authored on semantics alone and cannot be falsified by the
corpus at all; say so in its `notes`.

The other zeroes are unaffected by the deletions. The four removed files used only `select` and
`value` between them, both heavily attested elsewhere.

| Column | Meaning |
|---|---|
| `go_struct`, `type_token` | The operator, keyed as `types.csv` keys it. |
| `corpus_files`, `corpus_prod_files` | Files using the operator; the second excludes `test/`. A bundle attested only under `test/` is weaker evidence (R-1). |
| `corpus_instances` | Instance count — the useful figure for column operators, where files are not the unit. |
| `instances_direct`, `instances_in_case`, `instances_in_map_reduce` | Where those instances sit. An operator attested only under a nesting site is a different claim from one attested in an operator's `columns`: `avrg`, `max` and `min` appear only inside a `case`, and a third of `count`'s instances are a map-reduce phase. |
| `nests` | The nested column-list fields this operator carries, or `-`. Only `case` and `map_reduce` are non-empty — these are the two needing a sub-type per bundle. |
| `observed_hosts` | For a column operator, which pipe operators host it and how often. The evidence for `columns_range`. |
| `observed_columns`, `observed_columns_in_case` | For a pipe operator, the column operators seen in its `columns`, and those seen *only* nested inside a `case` under it. |
| `columns_applicable` | From `fields.csv`. `no` means `columns_range` must be `-`. Six of fifteen operators are `no`; a range set on one of them is not merely redundant, it reaches the column bundle through `TransformationSpecOverride.columns` and costs about 4,000 tokens. |

**Read `observed_hosts` before ranging anything.** Six column operators are hosted by exactly one
pipe operator in every live instance — `count`, `sum`, `distinct_count` and `map_reduce` by
`aggregate`; `map` and `lookup` by `map_record`. Four more are genuinely shared and belong in a core
bundle. Read the nesting columns alongside them: an operator whose instances are all nested is
evidence about a nested range, not about an operator's `columns`.

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
defeats the narrowing exactly where the narrowing was the point — the same cycle as
`conditional_config`, one level further down.

**And it is semantic before it is arithmetic.** A `case` inside an `aggregate` should offer the
aggregation column operators, not the mapping ones; `apply_reduce` should offer what can combine
partial results, which is not what `apply_map` offers. A shared `TransformationColumnSpecCase` entry
cannot say either thing, because one entry cannot range differently per host. Hence a sub-type per
bundle rather than a range applied to a common type.

The corpus says the same. Measured over the 45 live configs, the four sites hold different things:

| Site | What it actually contains |
|---|---|
| `case.then` | value 166, max 3, select 2, eval 2, min 2 |
| `case.else_expr` | value 15, eval 10, min 1, max 1, avrg 1 |
| `map_reduce.apply_map` | distinct_count 6, sum 4, count 2 |
| `map_reduce.apply_reduce` | **count 12, and nothing else** |

`apply_reduce` is the cleanest evidence in the whole corpus that the flat range is wrong: one
operator, twelve instances, no exceptions — the reduce phase combines partial counts and does nothing
else.

**A walk that reads only `columns` undercounts by 115 of 2,073 instances** and reports three
operators at zero that are not. `avrg`, `max` and `min` are attested only inside a `case` under
`aggregate` — 1, 4 and 3 instances, all in
`workspaces/cedargate_ws/pipes_config/data_profiling.pc.json`, which selects `min`/`max` by
`minmax_type` and `avrg` by a distinct-count threshold. `count`, `sum` and `distinct_count` are
undercounted by 14, 4 and 6 through the map-reduce phases. **`multi_select` is the only column
operator with no live instance anywhere.**

What sub-typing is worth, measured on the Aggregation bundle against a 24,576-token budget:

| | ~tokens |
|---|---|
| flat `TransformationSpec` union, nothing ranged | 41,040 |
| bundled but nothing ranged | 30,606 |
| `columns` + `conditional_config` ranged, nesting open | 16,363 |
| **all four sites ranged** | **15,277** |

The last step is worth 1,086 tokens on this bundle and more on wider ones — but the reason to take it
is the semantic, not the 6.6%.

### How the CSVs express it

`columns_range` on a member row carries what that member admits *inside itself*. For a pipe operator
that is its `columns` property, so a bare bundle name suffices. For a nesting column operator the
fields are named explicitly, because `map_reduce`'s two phases differ:

```
ColumnAggregation,case,theme,then=ColumnAggregation;else_expr=ColumnAggregation,-,...
ColumnAggregation,map_reduce,theme,apply_map=ColumnMapPhase;apply_reduce=ColumnReducePhase,-,...
```

A bundle whose `case` legs range to that same bundle is recursive, which is intended and stays
bounded — the measured closure is 34 of 144 definitions.

## The check, and what it reports today

Run against the authored files on 2026-08-20: **45 configs, 2,073 column instances, 0 violations**,
every pipe operator in a bundle and every column operator in a bundle.

Once the authored files have rows, the corpus check is: for every live config, every
(pipe operator, column operator) pair must be admitted by the authoring, and every operator must
belong to at least one bundle. A violation means the authoring is wrong or the config is — and
`corpus_prod_files` says which of those to believe first.


## The three infer-family bundles — 2026-09-10

**Added at `agentic_ai`'s `AW.1`, for `I-512`.** `InferPipe` landed on 2026-09-05 with the `infer`
token; `VllmPipe` and `EmbedPipe` land now, and with them every token
`TransformationSpec`'s discriminator admits has a bundle.

**What the gap actually was, measured rather than described.** The *flat* tier admitted `vllm` and
`embed` from the day each operator's leaf rows were extracted — both are in
`TransformationSpec`'s discriminator mapping, and `validate` has been green over the corpus
throughout. What admitted neither was the **bundle** tier, which is the tier an authoring hole binds
(`Hole.schema_ref` names a bundle) and the tier `bundles` checks the corpus against. So the operators
were authorable in a hand-written `.pc.json` and unreachable from a template hole.

**It was invisible until an asset install made it visible.** `bundles` can only falsify: it walks
live configs, so an operator no `.pc.json` uses is an operator it never asks about. `embed` became
visible on 2026-09-08, when the JetStore-owned `pipes_config/embed_input_parts.pc.json` was installed
into all four workspaces and the command began reporting `operator 'embed' is in no bundle` four
times over. `vllm` is still unattested and would have stayed invisible indefinitely.

**So `tests_bundles.py` holds the rule the corpus cannot.** `test_every_admitted_operator_has_a_bundle`
compares the discriminator mapping against `bundle_members.csv` and fails on any token in no bundle,
which is the check that would have caught both on the day their leaf rows landed. The rest of that
file is the negative half: a bundle is a `deepcopy` of its leaf with two properties retargeted, so
the way it goes wrong is by admitting *more* than the leaf did — and a bundle that admitted
everything would pass both the corpus check and the authorability check.

**Two things in this document are stale for the same reason and are left as the dated measurements
they are.** *Fifteen operators* (§*What the layer is for*, and the two range tables) was the count on
2026-08-20 and is ~~eighteen~~ **nineteen** today (`render`, 2026-09-11); the token figures below were measured over those fifteen and are
not re-measured here. `tests_template.py`'s `test_every_bundle_is_authorable` reads `bundles.csv`
rather than a literal, so the budget claim is re-checked over all ~~twenty-two~~ **twenty-three** on
every run and does not depend on the prose.

**`RenderPipe` is the twenty-third, added 2026-09-11 with the `render` operator.** It is the first
bundle whose `columns_range` is `-` for a reason other than the operator having no `columns` field in
Go: the render operator carries the key and the matrix marks it inapplicable, because the operator
writes one column named by `render_config.output_column` and runs no column transformations. The
emitter treats the two the same — it pops `columns` from the bundle and from its override — so the
distinction is one for a reader of `fields.csv` rather than one the schema can see.

**The check that found it owed a row is `cpipes-contract bundles`, and it found it by name**: a live
`render` fragment in `patient_profile_template.pc.json` reported *operator 'render' is in no bundle*.
Nothing in `check` or `schema` would have — the emitter fails on a bundle naming an unknown token and
not on a token named by no bundle — so *one bundle per operator* is an invariant the corpus check
enforces and the emitter does not.

**`bundle_evidence.csv` has no row for any of the three, and that is not an omission of this change.**
The file is documented above as *regenerated from the corpus, never hand-edited* — and nothing
regenerates it: no command reads or writes it, so it is a hand-seeded snapshot of 2026-08-19 that has
never been refreshed. Adding rows by hand would contradict its own header; writing the generator is a
separate piece of work.

**`status` in these two files takes `reviewed` / `unreviewed`, not the `example` / `todo` /
`authored` the column reference above names.** The new rows are `unreviewed`, following `InferPipe`.
Neither file carries a `reviewed_hash`, so nothing here is self-certifying a review: the stamp
mechanism covers `types.csv`, `fields.csv` and `constraints.csv` alone.
