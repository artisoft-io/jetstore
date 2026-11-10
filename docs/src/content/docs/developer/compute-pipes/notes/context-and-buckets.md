---
title: "Context overwrites the schema event, and an unresolved bucket writes successfully"
description: "How context entries override the deployment's environment, the default_value type that fills behind it, and the startup check on unresolved bucket names."
kind: explanation
sidebar:
  order: 8
---

**Established 2026-09-21, by measurement against the code and the four workspaces of the rule
corpus.** The two halves arrived as one report from a downstream consumer, read 2026-09-20, and
are separate mechanisms that happen to bite the same field.

## `context` assigns, it does not default

`prepareCpipesEnv` (`prepareCpipesEnv`, `actions_start_common.go`) makes the main input schema
provider's `Env` *be* `envSettings` and then walks `cpConfig.Context` over the top of it. So every
`context` entry the engine had until now — `value`, `file_key_component`,
`partfile_key_component` — **overrides what the deployment supplied**, and a document could not
carry a fallback for a variable the site is expected to set. That was the report's complaint and it
was right about the code.

**What it did not say is that the same function already contains the other rule, eleven lines
earlier**: `$DATE_FILE_KEY` is assigned only `if envSettings["$DATE_FILE_KEY"] == nil`
(`actions_start_common.go`), under the comment *Don't override with file key date if already
set via schema provider*. So a fill-behind `context` type is a generalisation of a rule this
function already makes rather than a new idea, and that is what `default_value` is. **The other
three types are unchanged and deliberately so** — a document that relies on overriding the event
keeps working.

**`types.csv` described the opposite and had since the matrix was written.** The `ContextSpec/value`
row read *"The value may be overriden by a schema provider"*, which is the order reversed; repaired
in the same change. A contract entry that states a precedence backwards is worse than one that
states none, because the reader who checks it stops.

## An unresolved `${...}` bucket is a bucket literally so named

Nothing between the document and the S3 call has an opinion about a bucket name.
`utils.ReplaceEnvVars` leaves a name it cannot substitute exactly as it found it, the six sites that
test for the JetStore bucket test `bucket == "" || bucket == "jetstore_bucket"` and say nothing
about anything else, and the writer addresses whatever it was handed.

**So the check is at startup**: `ValidateResolvedBuckets` (`bucket_validation.go`) is called from
both start actions after `ValidatePipeSpecConfig` and before either assembles the config its workers
run. Two properties worth knowing before changing it:

- **It walks by json key, not by type.** `FileConfig`'s `Bucket` reaches four specs by embedding and
  `ColumnFileSpec` declares its own, so an enumeration written from the embedders misses
  `anonymized_columns_output_file`. Matching the key `bucket` is what makes the walk complete and
  keeps it complete.
- **It checks the step about to run, not the document.** A conditional step's `addl_env` is applied
  only when that step is selected (`GetComputePipes`, `pipes_model.go`), so a later step's bucket
  may legitimately be unresolvable now; each step is checked when it starts. And `$SHARD_ID` and
  `$JETS_PARTITION_LABEL` are seeded, because `actions_coordinate_cp.go` assigns them at the
  worker, after startup has persisted the env.

**What it is not:** `src_bucket` and `dest_bucket` under a schema provider's `report_cmds` are
outside it. They are not on this path at all — `RunSchemaProviderReportsCmds` hands them to
`awsi.MultiPartCopy` verbatim (`jets/run_reports/delegate/run_commands.go`) with **no
substitution anywhere** — so what they want is a stricter check in that package, which nothing has
written.

## The corpus parameterises buckets in two spellings, and only one was being caught

Measured 2026-09-21 across the four workspaces of the rule corpus: **34 bucket values
carry a variable, in 5 documents** — 24 braced (`${CORPUS_OUT_BUCKET}`, `healthcare_corpus.pc.json`)
and 10 bare-dollar (two output-bucket keys, in four documents of client workspace C).
**Nine of the ten env keys `prepareCpipesEnv` writes are bare-dollar**, so the bare form is the
house spelling and the braced one is the exception.

`IsUnresolvedBucket` now refuses any dollar sign rather than `${` anywhere plus `$` at the head. An
S3 bucket name cannot contain a dollar, so the wider rule refuses no name that was ever a name — and
the narrower one left `corpus-out-$CLIENT` to be complained about by the bucket API, which is a
complaint made at the write, which is the thing the gate exists to move. **No corpus document uses
that mid-string form today**, so the widening is closing a hole rather than tightening on live
configuration; it is recorded here because the reasoning is the evidence and there is no instance.
