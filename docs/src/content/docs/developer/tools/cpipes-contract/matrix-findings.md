---
title: How the matrix was built
description: The corpus the compute pipes matrix was measured against, the seed rows and what they demonstrate, and the findings from extracting the matrix, with the date of each.
kind: explanation
sidebar:
  order: 2
---

This page records what building the [applicability matrix](/docs/developer/tools/cpipes-contract/matrix/)
found. Each measurement carries its date and describes the code and the workspaces as they were that
day; `cpipes-contract corpus` prints today's figures.

## The corpus, as measured on 2026-08-16

**45 files on 2026-08-16, 49 until then and 71 originally; 51 on 2026-11-10** by the rule in
[Why the contract is a matrix](/docs/developer/tools/cpipes-contract/design/#the-corpus).
`workspaces/*/pipes_config/**` is what JetStore loads.

Four files were deleted on 2026-08-16 because they could not run as they stood: one workspace's
`csv_test.pc.json` (it did not decode: a string in an `int32` `delimiter`) and
`clustering_test.pc.json` (clustering keys from a retired revision of `ClusteringSpec`), and another
workspace's two `test/` jetrules configs (which predated the jetrules output-channel checks). A config
the engine rejects is not corpus, it is history. One consequence: `TransformationSpec/clustering`,
`ClusteringSpec` and `TargetColumnsLookupSpec` went back to zero live instances, supported by the code
and used by no config, the state of `SplitterSpec/ext_count`.

The `.pc.json` files under `workspaces/*/data/` are notes and reference material for developers,
never read by the engine, and they preserve config shapes that have since been retired. Counting them
does not add noise, it manufactures contradictions: *every* apparent disagreement between the
validator and the corpus in the first draft of the seed came from that directory, and all of them
dissolved once it was excluded.

| | live `*/pipes_config/**` | retired `*/data/**` |
|---|---:|---:|
| files | 45 | 9 |
| pipes | 324 | 68 |
| transformations | 458 | 186 |
| `splitter` without `splitter_config` | **0** | 18 |
| `partition_writer` without `partition_writer_config` | **0** | 0 |

Those zeroes are the point, and they were not zero when the seed was drafted. The stale sources, a
directory of initial pipeline configurations under one workspace's `data/` and the one live file
copied from it, were deleted on 2026-08-12. The live corpus then agreed with the validator without
exception: `partition_writer_config` was present on 143 of 143 instances, `splitter_config` on 89 of
89 (138 of 138 and 89 of 89 after the 2026-08-16 deletions).

Counting is not done by hand. `cpipes-contract corpus` walks the live corpus **driven by the matrix
itself**, starting at the document root, `ComputePipesConfig/*`: a field row's `ref_struct` says
where a child of that type is found, the child's own discriminator says which row it is, and
`--apply` writes the measured counts and a live exemplar back onto the rows. So `corpus_instances`,
`corpus_count` and every exemplar are measured or they are not written, which is the same discipline
`--code` imposes on citations and for the same reason. Typing a node tries the struct's `~virtual`
rows first, in their row order, before reading the discriminator, the same dispatch order as the
code the rows describe, and falls back to the discriminator's recorded default when the key is absent
from the wire.

Two things fall out of the walk for free. `unreachable` names the types the matrix cannot yet descend
into, ordered by how much of the corpus sits behind them, which is a coverage worklist ordered by
value rather than by struct order. `--unknown` names keys present in live configs that no field row
accounts for, which is the dead-key audit once the matrix is complete.

## What the seed rows demonstrate

The matrix began as a seed of twelve types, eighty-eight fields and eleven constraints, chosen to
exercise the parts of the schema that were in doubt rather than to make progress on coverage:

- **`OllamaSpec/*`, all 22 fields with their defaults and descriptions.** The richest set of
  builder-applied defaults in the model, and a full transfer of the Go doc comments on one struct,
  the standard the rest were held to.
- **`pool_size`**: a default applied twice, by the config validator in
  `jets/compute_pipes/actions_start_common.go` and again by `applyOllamaDefaults` in
  `jets/compute_pipes/pipe_transformation_ollama.go`. `default_by` earns its place on this row alone.
- **`device_writer_type`**: required unless the output channel names a schema provider, a condition
  on a field of a different struct. This is the row a boolean `required` cannot carry.
- **`prompt_template` / `prompt_template_name`**: exactly one, which is a constraint row rather than
  a field property.
- **`TransformationSpec/ollama.new_record`**: `applicable=no` because the builder *forces* it to
  false, not because it is meaningless. The distinction is in `notes`; the row keeps `default = -`.
- **`PipeSpec/merge_files.apply`**: `applicable=no` on 0 of 19 live occurrences and nothing else.
  Correct-looking and weakly evidenced, marked `unreviewed`, and exactly the kind of row the review
  is ordered to reach first.
- **`PipeSpec/splitter`, `SplitterSpec/standard` and `SplitterSpec/ext_count`**: `splitter_config`
  is required (90 of 90 live), and `SplitterSpec` carries three mutually substitutable fields of which
  at least one must be set, plus `partition_row_count`, which is `required=yes` under `ext_count` and
  `applicable=no` under `standard`. Five fields, two tokens, one constraint each: the density the
  schema was designed for.
- **`SplitterSpec/ext_count` has zero live instances.** A token the code supports and no config uses.
  The schema must still emit it, the fragment library has no part for it, and the review has only the
  code to go on: the first row where corpus evidence is not weak but absent.
- **`TransformationSpec/partition_writer.partition_writer_config`**: was the seed's only `disputed`
  row; the dispute was an artefact of counting retired configs. See below.
- **`InputChannelConfig/stage`**: the embedded-`FileConfig` case, four validator-applied defaults,
  and `bucket`, which the validator *overwrites* rather than defaults.

`PipeSpec/splitter`'s `defs_name` is **`SplitterPipeSpec`**, not `SplitterSpec`: `SplitterSpec` is
the name of the config struct it points at, a different type with its own row. The near-collision is
the argument *for* the mechanical `CamelCase(type_token) + go_struct` rule rather than against it:
picking names by hand is how two things end up sharing one.

## Five findings from seeding it

**The corpus was 71 files before the retired configs were excluded.** Under review, the `.pc.json`
files beneath `workspaces/*/data/` were identified as developer reference material JetStore never
loads. The consequences run through every number in the matrix and are set out under *The corpus*
above; the largest is that corpus evidence is now measured against configs that actually execute.

**The `partition_writer_config` dispute was an artefact of that.** The validator rejects a
`partition_writer` without one (`ValidatePipeSpecConfig`, `jets/compute_pipes/actions_start_common.go`);
8 of the then 181 instances had none. Seven of those 8 were retired, and the eighth was the stale live
file named above. The validator was right, and the general rule now stated under the matrix's
conventions, that the validator wins because it runs on every execution, is what the seed had
backwards. The dead-key observation survives: `write_headers` was in live config text and in no Go
source, so the dead-key audit still had something to find before `additionalProperties: false` went
on.

**A defaulted discriminator is usually absent from the config, which nearly broke the walk.**
`SplitterSpec` discriminates on `type`, and the builder defaults it to `standard`
(`jets/compute_pipes/pipe_executor_splitter.go`), so all 90 live `splitter_config` nodes were
`standard` and **not one of them wrote `type`**. Reading the token off the wire alone left every one
of them untyped. The fix needs no new column: the default is already on the discriminator's own field
row, so the walker falls back to it, and rows of one struct disagreeing about that default is reported
rather than resolved. `InputChannelConfig` is the same shape and larger: 171 of its live nodes omitted
`type`, against a validator default of `memory`. It also says something about the emitted schema: a
discriminated union whose discriminator is optional needs the default declared on the Pydantic field,
or nothing that consumes the schema can tell which branch an untagged object belongs to.

**The root type is two documents, and the schema must describe only one.** `ComputePipesConfig` has
three fields that can carry pipes. `conditional_pipes_config` and `reducing_pipes_config` are written
by the author; **`pipes_config` is written by JetStore, never by the author**: the startup actions
select one step's pipes and marshal a fresh `ComputePipesConfig` carrying them there
(`jets/compute_pipes/actions_start_sharding_cp.go`, `actions_start_reducing_cp.go`), and that per-step
document is what the whole runtime reads. In the live corpus of the day, 29 files used
`conditional_pipes_config`, 20 used `reducing_pipes_config`, none used both and none used
`pipes_config`. Reflection over the struct sees three alternatives and cannot see the split, so a
schema built from it would permit the runtime shape, which validates cleanly and cannot run.
`reducing_pipes_config` is also *superseded* rather than merely older, which is why the matrix has a
`deprecated` column.

**`ollama` was no longer at zero.** `workspaces/jets_ws/pipes_config/patient_profile.pc.json` gained
an `ollama` transformation on 2026-08-08, so the corpus holds a real exemplar for that operator.

## Two discriminations are structural

The token grammar first defined, a value of the discriminator or `*`, could not express two variants
the extraction ran into. The first attempt at recording them invented per-row notation (a `-` token
with `lhs` or `arg` standing in the `discriminator` column) that broke the very properties the schema
depends on: `-` acquired three meanings, and the rows of one struct stopped agreeing on their
discriminator. The correction is the `~virtual` token and its `variant_when` predicate, defined under
the matrix's conventions.

The two cases are different in kind. `ExpressionNode` is a **hybrid union**:
`BuildExprNodeEvaluator` (`jets/compute_pipes/eval_expression.go`) dispatches on `arg` being set,
then on `lhs`, and only then on the `type` value. So `~unary` and `~binary` are shape-selected while
`select`, `value`, `expr_proxy`, `function` and `static_list` (the mandatory right-hand side of `in`
and `in_no_case`) are value-selected, and the row order of the virtual tokens carries the dispatch
precedence. `TransformationSpec/~override` is **absence-selected**: a `conditional_config.N.then`
whose `type` is empty overrides fields of its host, while one with a `type` replaces it outright
(`MergeTransformationSpec`, `jets/compute_pipes/actions_start_common.go`). So every value token is
*also* legal at that position, and the schema for `then` is the whole union rather than one branch.
Neither case is the defaulted-discriminator shape above: there, an absent key still means one value
token; here, absence or presence *is* the discrimination.

## A discrimination that is a complement

Added on 2026-09-13, and the first union member the matrix had to describe as *everything else*. The
engine has a site operator registry: `BuildPipeTransformationEvaluator`'s `default:` branch
(`jets/compute_pipes/pipes_runtime_model.go`) hands a `type` that names no built-in to a factory the
deployment registered with `WithOperators`, and `TransformationSpec` has a `site_config` field to
configure it (`jets/compute_pipes/pipes_model.go`). The emitted contract refused all of it: a `oneOf`
over nineteen `"type": {"const": …}` branches with `additionalProperties: false` matches no site
token, and refuses `site_config` on every branch even if one did match. So JetStore accepted a shape
the schema called invalid.

**The membership predicate is `unlisted(type)` and not `present(site_config)`.** Three measurements
decided it, all taken on 2026-09-13:

- **`site_config` does not select the variant.** `siteOperatorArgs`
  (`jets/compute_pipes/site_operators.go`) returns early on a nil `SiteConfig`, so an operator needing
  no configuration is a shape the engine builds and runs. A branch keyed on the field's presence
  would refuse it.
- **It is not even *evidence* of the variant.** `validateSiteOperatorSpec`
  (`jets/compute_pipes/site_operators.go`) refuses a `site_config` on a built-in token, so its
  presence and the operator's identity are separate claims that the engine checks against each other.
- **The `oneOf` stays exclusive without it.** A built-in token matches its own branch and fails the
  site branch's `not: enum`; anything else matches the site branch alone.

**The enum is derived from `types.csv`, and that is why this is safe.** The tokens the site branch
excludes and the tokens the union's other branches carry are the same fact, and a second list of a
fact is how two lists drift apart. `test_the_complement_branch_excludes_exactly_its_union_s_own_tokens`
holds it. A token in the union and missing from the enum would match **two** branches, so the union
would stop being exclusive.

**`infer` is in the enum and is not in Go's `builtinOperatorTypes`, and both are right.** That map is
*the tokens the dispatch handles*, and `ResolveInferBackend`
(`jets/compute_pipes/actions_start_sharding_cp.go`) rewrites `infer` into `ollama` or `vllm` before
the validator or the dispatch sees it. So an authored `{"type": "infer", "site_config": …}` is
rewritten into a built-in carrying a `site_config` and refused: the schema excluding `infer` agrees
with the engine, and it is the *authored* vocabulary the schema describes.

**What the schema says, and what it cannot.** A site operator's `output_channel` is required,
because `validateOutputChConfig` runs for every transformation outside the operator switch. Its
built-in config pointers are inapplicable, because `siteOperatorArgs` reads none of them.
`site_config` is inapplicable at `conditional_config.N.then`, because `MergeTransformationSpec` has no
arm for it. What the schema cannot say is anything about the *contents* of `site_config.config`: it
is `json.RawMessage`, JetStore does not know its schema, and that is the design rather than a gap.

Every row added for it was `unreviewed`, and none of the 2,102 existing rows moved: `stamp` reported
`0 stamped, 0 cleared, 0 restamped`.

### The model, six days later

**The emitter change reached the JSON Schema and not the Pydantic model.** `generate.py` built a
union alias from the struct's *real* rows and emitted virtual tokens as free-standing classes, so
`TransformationSpec` stayed a nineteen-member tagged union and `ComputePipesConfig.model_validate`
refused every document naming a site operator, while `cpipes_schema.json`, which gets the branch by a
splice, validated the same document with no errors. Two readers of one contract, disagreeing. It was
found by the [Python compute pipes node](/docs/developer/tools/cpipes-node/)'s corpus test, on the
first authored document in the `jets_ws` corpus to contain a site operator.

**A union with a complement token is now emitted as an ordered union**:
`Annotated[Union[<the tagged nineteen>, <the branch>], union_mode="left_to_right"]`. Left-to-right
is the rule and not a preference: the branch's discriminator is a bare `str`, so under any other
ordering it is a candidate for every token, the built-ins included. That is the model's spelling of
`BuildPipeTransformationEvaluator`'s `default:` branch.

**And the complement's `type` refuses a built-in token at validation, not only in the schema.**
`json_schema_extra` is read when the schema is emitted and by nothing in Pydantic, so an ordered
union alone would have left the model admitting on the site branch exactly what the schema refuses by
`not: enum`. The direction of that disagreement is the harmful one: a *malformed* built-in fails its
own tagged branch, satisfies the site branch, and is reported as an unknown site operator rather than
as the malformed built-in it is. `_unlisted` closes it, taking the same emitted tuple the `not: enum`
takes, so the two remain one list. The empty string is refused with it.

**The emitted `cpipes_schema.json` did not move by a byte.** Pydantic writes the widened alias as
`anyOf: [<tagged oneOf>, <branch>]`, and `splice_complement_branches`, which used to *add* the
branch, now *folds* that back into the single discriminated `oneOf`. That is asserted on every run by
`tests_schema.py`, which also checks that the committed schema is what the current model emits.
