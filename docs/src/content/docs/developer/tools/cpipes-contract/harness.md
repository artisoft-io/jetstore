---
title: The harness
description: How the compute pipes contract harness turns each row of the matrix into a test result against the Go validator, what it cannot see, and what its first runs found.
kind: explanation
sidebar:
  order: 3
---

The harness was built before the matrix was reviewed rather than after, so that reviewing a row means
reading what the validator actually did. It writes the `harness` columns of `types.csv` and
`fields.csv` ([the applicability matrix](/docs/developer/tools/cpipes-contract/matrix/)). It has two
halves.

## The Go runner

**`harness/` (Go)** is deliberately dumb: a JSON array of `{id, config}` documents on standard input,
a verdict per document on standard output, produced by running each one through `GetComputePipes`,
`ApplyAllConditionalTransformationSpec` and `ValidatePipeSpecConfig` exactly as
`jets/compute_pipes/actions_start_sharding_cp.go` and `actions_start_reducing_cp.go` do, including a
fresh unmarshal per step, because the validator mutates its input. It holds no opinion about the
matrix, so the contract under test is the one the engine enforces.

A `decode` failure (the document `json.Unmarshal` refuses) is reported separately from a `validate`
rejection, because the first means the matrix's `go_type` or the synthesis is wrong and the second is
the observation the harness exists to make.

## The synthesis

**`cpipes_contract/harness.py`** synthesises, per `(go_struct, type_token)`, a **minimal authored
config from the matrix alone**:

- the target's required fields, and the conditional ones whose `required_when` holds in the minimal
  context;
- values taken from `values`, `default` or a neutral fill;
- embedded at a position found by walking the matrix's own `ref_struct` graph from
  `ComputePipesConfig/*`, the same graph the corpus walker descends, in the other direction.

The host chain is itself minimal. References are wired to the sibling entities the validator resolves
them against (`output_file` to an `output_files` entry, `output_table_key` to an `output_tables`
entry with a `channel_spec_name`, a jetrules `spec_name` to a `channels` entry), and every document
gets at least one live step, since a config the validator runs zero steps over is accepted vacuously.
Then, for each required field of the target, the same document minus that one field must be rejected.

Three kinds of knowledge live in the harness rather than the matrix, each a small named table with its
reason:

- which token an intermediate takes (`PREFERRED_TOKEN`: the cheapest valid instance, `map_record` for
  a transformation, `value` for an expression);
- which token is position-bound (`TOKEN_ONLY_VIA`: `~override` means override only at
  `conditional_config.N.then`; anywhere else a type-less spec is just invalid);
- the reference wiring above.

A hint never substitutes for a missing `required` claim: when the matrix under-declares a type, its
minimal config is rejected and that is reported as a finding, not papered over. The findings *are* the
review worklist, in the order of the validator's authority.

## What it cannot see

The config validator sees a fraction of the real contract; the rest lives in the operator builders,
at the time the pipeline's graph is built. So `pass` on a type the validator never inspects says only
"decodable, and nothing contradicted"; builder-evidence requirements come back `untestable` rather
than proven; and prohibitions are entirely the emitted schema's to enforce. The expression grammar is
the extreme case: an `ExpressionNode` is carried, not compiled, at config level, so all seven of its
variants accept vacuously. Their contract becomes testable only where an expression is actually
built.

## What the first run found, 2026-08-15

45 findings, none of them an artefact of the harness, in three classes:

1. **Requirements the validator enforces that the matrix under- or mis-declared.** Eleven
   transformation tokens (`aggregate` through `clustering`) carried no claim on `output_channel`,
   and the validator rejects every one of them (`output_channel.name must not be empty`): this one
   gap failed about 25 types, because everything reachable only through those tokens inherited it.
   Worse than unfilled: `anonymize_config`, `jetrules_config`, `clustering_config`,
   `correlation_output_channel` and `JetrulesSpec.output_channels` all said **`required=no` on comment
   evidence**, and `write_step_id`, `file_key` and `format` said the same on `OutputChannelConfig`:
   claims the validator rejects outright. The validator outranks the comment and the corpus, and the
   harness is what made the disagreement visible. All were corrected with `evidence=validator` and
   the rejecting line cited. The stage channel's `write_step_id`-or-`file_key` alternative became
   `required=conditional` on both rows plus an `at_least_one` constraint, and `format` on the `output`
   token is conditional on `absent(schema_provider)`, since the schema-provider sync can supply it.
   `TransformationSpec/jetrules.output_channel` went the other way: `applicable=no`, because the
   validator discards it in favour of `jetrules_config.output_channels`, and none of the 13 corpus
   instances carried one.
2. **Six `required=yes, evidence=validator` claims the validator did not enforce.** The validator
   *defaulted* an absent `input_channel` (to a memory channel) rather than requiring one
   (`PipeSpec/fan_out`, `/splitter`), and never checked that an input channel's `name` was non-empty
   (all four `InputChannelConfig` tokens). The claims stood, since the requirement is real, enforced
   where `GetInputChannel` (`jets/compute_pipes/pipes_runtime_model.go`) resolves the name when the
   graph is built, and the corpus was unanimous on presence (333 of 333 pipes, 343 of 343 names). So
   the *evidence* first moved to `builder`, and the rows went `fail` → `untestable`. The validator was
   then tightened the same day, as its own change: `ValidatePipeSpecConfig` rejects an unnamed or
   absent input channel, guarded by running all 49 live configs through it before and after with no
   verdict changing. The evidence moved back to `validator` and the six rows are `pass`: the harness
   proving the very check it caused to exist.
3. **Two types are unreachable by construction**: `ComputePipesCommonArgs` and
   `ClusterShardingInfo`, whose only referring fields are `applicable=no` because JetStore writes
   them, not the author. Permanently `pending` at config level, which is the correct state for
   runtime-only shapes.

After the corrections and the tightening the harness ran clean: **115 of 117 types `pass`, 0 `fail`,
the 2 above `pending`; fields 579 `pass`, 0 `fail`, 633 `untestable`, 580 `pending`**, the pending
fields being the rows whose `required` was still unfilled plus the rows of the two unreachable types.

## Closing the reachability gap, 2026-08-15

Ten structs referenced through `ref_struct` had no rows: seven named by `check --strict` and the three
their own fields pull in (`S3CopyFileSpec`, `DomainKeyInfo`, `InputSourceSpec`; `ReportCmdSpec` turned
out to be discriminated, `type` range `s3_copy_file`). Adding them brought the matrix to 127 types and
1,831 fields, every `ref_struct` resolving. The corpus walk was repointed to start at the document
root, so the root-section types (`channels`, `lookup_tables`, `schema_providers`, `output_tables`) are
measured rather than hand-attested. The harness then stood at **123 of 127 types `pass`, 0 `fail`, 4
`pending`**, the four being the runtime-only shapes (`ComputePipesCommonArgs`, `ClusterShardingInfo`,
and `SourcesConfigSpec` and `InputSourceSpec`, reachable only through `common_runtime_args`).

The repointed walk immediately found two engine-level defects at the top of the document, both
invisible before:

- **`ClusterSpec.MaxNbrPartitions`'s json tag was misspelled `max_nbr_partitons`.** Eight live
  configs wrote the correct spelling and Go ignored all eight, so their partition caps never took
  effect. Correcting the tag was safe for the corpus but made those caps *start working*, a runtime
  behaviour change. The tag in `jets/compute_pipes/pipes_model.go` now reads `max_nbr_partitions`.
- **Seven live configs set `use_ecs_tasks` on `cluster_sharding_config` entries**, where the model
  has no such field: the key belongs to `ConditionalPipeSpec`. It was the field's *old* home, and the
  seven leftovers, all `false`, were removed from the configs on 2026-08-16.

The corpus baseline that guarded the tightening showed the live corpus was *not* 49 of 49 through
`ValidatePipeSpecConfig`. Three configs need workspace environment variables the harness cannot know
(in `when` conditions), an environment limit rather than a config defect. Three more files would fail
at startup as they were, and were deleted on 2026-08-16 with a fourth (see
[How the matrix was built](/docs/developer/tools/cpipes-contract/matrix-findings/#the-corpus-as-measured-on-2026-08-16)).
Every remaining file decodes; the remaining rejections are the environment-variable limit, which is
the harness's, not the corpus's.
