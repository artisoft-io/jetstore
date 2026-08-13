# The cpipes applicability matrix — schema

**Task B.1 · drafted 2026-08-11 · revised 2026-08-12 after first review · awaiting review**

This is the definition of the matrix, not the matrix. The rows currently in `matrix/` are a seed:
twelve types and eighty-eight fields, hand-extracted while designing the schema, kept because a schema
with no rows in it has not been shown to carry anything. B.2–B.6 fill the rest.

The reasoning behind the matrix — why reflection over `pipes_model.go` recovers nothing about
applicability, and why the schema is a projection of the matrix rather than a thing built directly —
is in the Phase 0 plan (`plan/1_phase0_plan.md` §5.2 of the `jetstore_agentic_ai` repo). This file
documents the columns. Where it departs from the plan's §5.2.1 it says so, and the argument for each
departure is issue **I-11** in that repo's `plan/tracking/3_phase0_tasks_issues_risks.md`.

## Three tables, not one

| File | One row per | What it is for |
|---|---|---|
| `matrix/types.csv` | **addressable type** — a Go struct, paired with one value of its discriminator when it has one | The unit of a `$defs` entry, of a Pydantic subclass, and of a fragment-library part |
| `matrix/fields.csv` | **field of an addressable type** | The matrix proper: applicability, requirement, default, evidence |
| `matrix/constraints.csv` | **requirement spanning more than one field** | What `(applicable, required)` cannot express |

The plan asks for one CSV. It became three because the discriminator vocabulary, the corpus instance
count and the exemplar are properties of a *type* rather than of a field: carried on every field row
they would be repeated a dozen times and could disagree row to row, which is the two-sources-for-one-
fact problem decision 8 exists to object to. `fields.csv` remains the review artifact.

### How exhaustive `types.csv` has to be

**Exhaustive.** A type row is not merely a place to put a `defs_name` that the Go name cannot carry —
it is the parent of its field rows, the anchor of the fragment library's exemplar, and the node the
corpus walker descends through. So:

> **Every struct reachable as a *value* gets a row: one per `(go_struct, type_token)` pair, and `*`
> is a full-standing token, not a placeholder.** A struct with no discriminator gets exactly one row.

Reachable *as a value* means some field points at it through `ref_struct`, plus the document root.
Two consequences that are easy to get wrong at extraction time:

- **Embedded structs get no row.** `FileConfig` is only ever embedded, never a named field, so
  nothing references it — its 43 fields appear on the *host's* rows with `declared_in=FileConfig`.
  A `FileConfig/*` row would have no parent field, no exemplar and no corpus count.
- **Undiscriminated structs are not optional.** `OllamaSpec/*` is where the only `ollama` exemplar in
  the corpus lives, and it is the parent of 22 field rows. `defs_name` happens to be mechanical there;
  the other nine columns are not.

This is enforced, not merely intended. `check` fails with `no types row for X/Y` for any orphaned
field row; the corpus walker reports a struct with no row as `unreachable` and stops descending, which
is how `TransformationColumnSpec` (1446 nodes) and `OutputChannelConfig` (389) currently head the
coverage worklist; and `--strict` makes every unresolved `ref_struct` a failure.

Size, then: `pipes_model.go` declares 69 structs, of which about fourteen discriminate into roughly
fifty tokens between them — `TransformationSpec` alone has fifteen. So expect **on the order of a
hundred rows**, not a few dozen. `constraints.csv` stays small.

## Conventions

- **Every cell is filled.** `-` means "none / not applicable". An empty cell is a schema error, so an
  unfilled row is always distinguishable from a row reviewed and found empty.
- **`*` is the type token of a struct with no discriminator** — `OllamaSpec/*` means every instance
  of `OllamaSpec`. A blank token would collide with "not yet determined".
- **Citations are `path:line` relative to the JetStore repo root**, the same convention the plan uses:
  `jets/compute_pipes/pipes_model.go:1152`.
- **The corpus is `workspaces/*/pipes_config/**` and nothing else** — 49 files. The `.pc.json` under
  `workspaces/*/data/` are developer notes and reference material that JetStore never loads; they are
  not counted, and `check --corpus` refuses one as an exemplar. See *The corpus* below.
- **The corpus is authored documents.** The root type also serialises a *runtime* shape that
  JetStore writes for itself, and the schema must describe only the authored one — see the fourth
  finding below.
- **`test/` is a tier, not an exclusion.** Configs under `workspaces/*/pipes_config/*/test/` are real
  and must validate, so they stay in the corpus and in every count — but they are also counted
  separately, because a field attested only by a test config is weaker evidence than one a production
  pipeline depends on, and exemplars prefer a production occurrence.
- **The validator wins.** It runs on every execution, so a shape it rejects cannot be in service. A
  config that contradicts it is not evidence against it, it is evidence of its own age. Corpus
  evidence stays strong for presence and weak for absence, and is worth nothing against the validator.
- **Corpus paths are relative to the repo holding `workspaces/`**, and the path into the file is dot
  notation with numeric indices — the same notation `OllamaMappingSpec.Path` already uses, so there is
  one path convention rather than two.
- **Pipe-separated lists** (`values`, `members`); comma-separated only for `embeds`.
- Descriptions are flattened to one line on write, so the files stay diffable and open cleanly in a
  spreadsheet.

## `types.csv`

| Column | Rule |
|---|---|
| `go_struct` | The Go struct name, as declared. |
| `type_token` | One value of that struct's discriminator, or `*` when it has none. |
| `defs_name` | Mechanical: `CamelCase(type_token) + go_struct`, or `go_struct` when the token is `*`. `OllamaTransformationSpec`, `MergeFilesPipeSpec`, `StageInputChannelConfig`. The check enforces it, so the `$defs` key, the Pydantic class name and the fragment-library entry are one name rather than three conventions. |
| `discriminator` | The **json key** of the discriminating field, or `-`. Not always `type`: `PartitionWriterSpec` discriminates on `device_writer_type`. |
| `embeds` | Structs embedded anonymously, whose fields are promoted onto this type on the wire. `InputChannelConfig` embeds `FileConfig`. |
| `fragment` | Whether this type can be authored and validated standing alone (plan criteria 6 and 7). Expected to be `yes` almost everywhere; a `no` must say why in `notes`. |
| `deprecated` | Superseded but still valid; as in `fields.csv`. |
| `corpus_instances`, `corpus_prod_instances` | **Measured.** Occurrences across the live corpus, and the subset outside a `test/` directory. Written by `corpus --apply`, never by hand. |
| `exemplar_file`, `exemplar_path` | One real occurrence, from a live config, preferring a production one over a `test/` one. Set exactly when `corpus_instances > 0`, and `check --corpus` resolves every one of them and refuses a retired one — a fragment library whose exemplars do not resolve is a catalogue of things that may not exist. |
| `doc_ref` | `file:line` of the type definition. |
| `description`, `notes` | Prose. `description` is destined for the Pydantic model; `notes` is not. |

The token vocabulary of a struct is the set of its rows here. It is deliberately not a column
anywhere, so it cannot drift from the rows it describes.

## `fields.csv`

Ten of these columns are the plan's §5.2.1 list. The other twelve are marked **+**.

| Column | Rule |
|---|---|
| `go_struct`, `type_token` | Foreign key into `types.csv`. |
| `field_name` | The Go field name. |
| `json_key` | The wire key. |
| **+** `declared_in` | The struct that declares the field. Differs from `go_struct` when the field is promoted through an embed, which keeps the field inventory single-sourced while letting applicability be per host and per token — `FileConfig.Delimiter` is defaulted for `input_channel` type `stage` and not for the others. |
| `go_type` | The Go type as written, pointer and all. |
| **+** `container` | `scalar`, `object`, `array`, `array2`, `map`, `raw_json`, `any`. `array2` is `[][]T`, which three fields of the model need — `reducing_pipes_config`, `date_formats`, `other_date_formats`. A vocabulary without it would have forced a wrong row at B.2, though the first of the three is deprecated (I-14), so the shape may end up carried only by the two date fields. |
| **+** `ref_struct` | The struct the value is, for objects, arrays of objects and maps of objects; `-` otherwise. Without it the matrix is a flat field list and nothing can compose fragments: this and `container` are what make the same rows serve the parts library. |
| **+** `values` | A closed value set, pipe-separated — `pass_through\|drop\|fail`. `-` on the discriminator field itself, whose vocabulary is the `types.csv` rows. Enums are a third of what a schema constrains and §5.2.1 has no column for them. |
| `applicable` | Does this field mean anything for this type token? `applicable=no` is the row that drives the `if`/`then` overlay's prohibitions. |
| **+** `deprecated` | Superseded but still valid. **A third axis, not a state of `applicable`**: a deprecated field is applicable, is present in the corpus, and must still validate — three things `applicable=no` denies. What it must not do is appear in the fragment library or in anything the model is prompted to produce. `reducing_pipes_config` is the known case (I-14); the check rejects `deprecated=yes` with `applicable=no` as two different claims conflated. |
| `required` | **Three states, not two:** `yes`, `no`, `conditional`, plus `na` when the field is inapplicable. |
| **+** `required_when` | The condition, set exactly when `required` is `conditional`. `absent(output_channel.schema_provider)`. |
| `default` | The literal applied when the field is absent, or `-`. A value the builder *forces* — writing it whatever the author put there — is not a default and belongs in `notes`; the row is `applicable=no`, and `TransformationSpec/ollama.new_record` is the case. |
| **+** `default_by` | `validator`, `builder` or `none`. This decides who can observe the default: the config validator mutates its input, so a default it applies is already in the config by the time the harness looks; one the operator builder applies is invisible both at config level and to the schema. `pool_size` is applied by both. |
| `evidence` | `validator`, `builder`, `corpus`, `generator`, `comment` — the plan's authority order. **`reviewed` and `unreviewed` are not values here**; see `review`. |
| **+** `evidence_ref` | `file:line`, relative to the JetStore repo root. Required for every evidence kind except `corpus`, where `corpus_count` is the citation. A source kind with no location is not checkable, and the plan's own standard is that a claim without a citation does not belong. `check --code` resolves every one of them and requires the cited line to name the field — see below. |
| `corpus_count` | **Measured.** Occurrences of this field across instances of this type. Strong evidence for presence, weak for absence — a row claiming `applicable=no` with a non-zero count is rejected outright. |
| **+** `corpus_prod_count` | **Measured.** Of those, the ones outside a `test/` directory. A field attested only by test configs is weaker evidence than one a production pipeline depends on, and `corpus_count == n, corpus_prod_count == 0` is the pattern the review should reach early (R-1 orders by evidence strength). Costs nothing to carry, since both are written by `corpus --apply` and never typed. |
| **+** `harness` | **Written by the machine.** `pass`, `fail`, `untestable`, `pending` — the B.7 result for this row, written back by the harness. The mitigation for R-1 is that reviewing a row means reading a test result, which is only true if the result is on the row. `untestable` is the honest state for builder-evidence rows the config validator never sees. |
| **+** `review` | **Written by you.** `unreviewed`, `reviewed`, `disputed` — the human sign-off, set by hand in B.8 and by nothing else. A separate axis from `evidence`: a row can be corpus-derived and reviewed, or validator-derived and unreviewed. R-1's number to watch — rows still unreviewed when B.9 wants to start — is not countable unless these are two columns. `disputed` is for rows where you judge the sources to disagree; it is a verdict, not a measurement. |

`harness` and `review` are the answer to "who fills this in": the columns marked **Measured** are
written by `corpus --apply`, the rest are extracted from the code by hand, `harness` is written by
the test run, and `review` is the one column that is yours. Nothing in the toolchain writes it — `check` and `corpus` only ever read it, so
a `reviewed` mark can never be manufactured by a re-run.

What the toolchain does *not* yet do is invalidate a `reviewed` mark when the row underneath it
changes, which it should: a row reviewed in B.8 and re-measured in a later `corpus --apply` keeps its
tick. Cheapest fix is a hash of the row's evidence-bearing columns stored alongside the mark, and it
belongs with B.8 rather than here.
| `description` | The Go doc comment, carried across by B.6. Destined for the Pydantic field description, after which the Go comments stop being the source. Last column, so a spreadsheet review can hide it. |
| `notes` | Everything else, including why a weak row is weak. |

### Coherence rules the check enforces

- `required = na` exactly when `applicable = no`.
- `required_when` set exactly when `required = conditional`.
- `default_by = none` exactly when `default = -`.
- An inapplicable field has no default.
- An object field names a `ref_struct`; a scalar or `raw_json` field does not.
- Non-corpus evidence carries an `evidence_ref`.
- `applicable = no` with `corpus_count > 0` is a contradiction.
- `declared_in ≠ go_struct` requires the type row to embed it.

## `constraints.csv`

`(applicable, required)` says a field is needed; it cannot say *exactly one of these two*, *this one
wins when both are set*, or *required unless that one is present*. All three are in the code.

| Column | Rule |
|---|---|
| `go_struct`, `type_token` | The **innermost type from which every member is reachable**. Members below it are dotted paths — `ollama_config.output_mapping`. |
| `kind` | `one_of`, `at_least_one`, `mutually_exclusive`, `precedence` (all may be set, the first member wins), `requires`, `forbids`, `external`. |
| `members` | Pipe-separated json keys, in significant order for `precedence`. For `external`, the first member is the field and the rest name something outside the type — another section of the document, or the pipe's position. |
| `enforce` | `schema` (expressible as an `if`/`then` overlay), `validator` (needs the Go validator or the harness), `prompt` (expressible only as an instruction to the model). Knowing which constraints fall to `prompt` is knowing where the contract stops applying. |
| `evidence`, `evidence_ref`, `review` | As in `fields.csv`. |
| `notes` | Prose. |

## The corpus

**49 files, not 71.** `workspaces/*/pipes_config/**` is what JetStore loads. The `.pc.json` under
`workspaces/*/data/` are notes and reference material for developers, never read by the engine, and
preserving config shapes that have since been retired. Counting them does not add noise, it
manufactures contradictions: *every* apparent disagreement between the validator and the corpus in
the first draft of this seed came from that directory, and all of them dissolve once it is excluded.

| | live `*/pipes_config/**` | retired `*/data/**` |
|---|---:|---:|
| files | 49 | 9 |
| pipes | 333 | 68 |
| transformations | 468 | 186 |
| `splitter` without `splitter_config` | **0** | 18 |
| `partition_writer` without `partition_writer_config` | **0** | 0 |

Those zeroes are the point, and they were not zero when this seed was drafted. The stale sources —
`data/automated_mapping/initial_pipes_config/` and the one live file copied from it,
`cedargate_ws/pipes_config/hf_medicalclaim_extract.pc.json` — were deleted on 2026-08-12. The live
corpus now agrees with the validator without exception: `partition_writer_config` is present on
143/143 instances, `splitter_config` on 89/89.

Counting is not done by hand. `cpipes-contract corpus` walks the live corpus **driven by the matrix
itself** — a field row's `ref_struct` says where a child of that type is found, the child's own
discriminator says which row it is — and `--apply` writes the measured counts and a live exemplar
back onto the rows. So `corpus_instances`, `corpus_count` and every exemplar are measured or they are
not written, which is the same discipline `--code` imposes on citations and for the same reason.

Two things fall out of the walk for free. `unreachable` names the types the matrix cannot yet descend
into, ordered by how much of the corpus sits behind them — `TransformationColumnSpec` (1446 nodes),
`OutputChannelConfig` (389), `InputChannelConfig` with no `type` at all (171) — which is a coverage
worklist for B.2–B.6 ordered by value rather than by struct order. `--unknown` names keys present in
live configs that no field row accounts for, which is the B.14 dead-key audit once the matrix is
complete.

## What the seed rows demonstrate

Twelve types, eighty-eight fields, eleven constraints, chosen to exercise the parts of the schema that
were in doubt rather than to make progress on coverage:

- **`OllamaSpec/*`, all 22 fields with their defaults and descriptions.** The operator the agentic
  programme cares about, the richest set of builder-applied defaults in the model, and a full B.6
  description transfer on one struct — the standard R-2 asks the rest to be held to.
- **`pool_size`** — a default applied twice, by the validator at `actions_start_common.go:997` and
  again by `applyOllamaDefaults` at `pipe_transformation_ollama.go:911`. `default_by` earns its place
  on this row alone.
- **`device_writer_type`** — required unless the output channel names a schema provider, a condition
  on a field of a different struct. This is the row a boolean `required` cannot carry.
- **`prompt_template` / `prompt_template_name`** — exactly one, which is a constraint row rather than
  a field property.
- **`TransformationSpec/ollama.new_record`** — `applicable=no` because the builder *forces* it to
  false, not because it is meaningless. The distinction is in `notes`; the row keeps `default = -`.
- **`PipeSpec/merge_files.apply`** — `applicable=no` on 0/19 live occurrences and nothing else.
  Correct-looking and weakly evidenced, marked `unreviewed`, and exactly the kind of row the review
  is ordered to reach first.
- **`PipeSpec/splitter`, `SplitterSpec/standard` and `SplitterSpec/ext_count`** — added on review.
  `splitter_config` is required (90/90 live), and `SplitterSpec` carries three mutually-substitutable
  fields of which at least one must be set, plus `partition_row_count`, which is `required=yes` under
  `ext_count` and `applicable=no` under `standard`. Five fields, two tokens, one constraint each: the
  density the schema was designed for.
- **`SplitterSpec/ext_count` has zero live instances.** A token the code supports and no config uses.
  The schema must still emit it, the fragment library has no part for it, and the review has only the
  code to go on — the first row in the seed where corpus evidence is not weak but absent.
- **`TransformationSpec/partition_writer.partition_writer_config`** — was the seed's only `disputed`
  row; the dispute was an artifact of counting retired configs. See below.
- **`InputChannelConfig/stage`** — the embedded-`FileConfig` case, four validator-applied defaults,
  and `bucket`, which the validator *overwrites* rather than defaults.

`PipeSpec/splitter`'s `defs_name` is **`SplitterPipeSpec`**, not `SplitterSpec` — `SplitterSpec` is
the name of the config struct it points at, a different type with its own row. The near-collision is
the argument *for* the mechanical `CamelCase(type_token) + go_struct` rule rather than against it:
picking names by hand is how two things end up sharing one.

## Five findings from seeding it

**The corpus was 71 files and is 50.** Under review, the `.pc.json` beneath `workspaces/*/data/` were
identified as developer reference material JetStore never loads. The consequences run through every
number in the matrix and are set out under *The corpus* above; the largest is that corpus evidence is
now measured against configs that actually execute.

**The `partition_writer_config` dispute was an artifact of that.** The validator rejects a
`partition_writer` without one (`actions_start_common.go:908`); 8 of the then-181 instances had none.
Seven of those 8 are retired, and the eighth is the stale live file named above. The validator was
right, and the general rule now stated under *Conventions* — the validator wins, because it runs on
every execution — is what the seed had backwards. The row is no longer `disputed`. The dead-key
observation survives: `write_headers` is in live config text and in no Go source, so B.14's audit
still has something to find before `additionalProperties: false` goes on.

**A defaulted discriminator is usually absent from the config, which nearly broke the walk.**
`SplitterSpec` discriminates on `type`, and the builder defaults it to `standard`
(`pipe_executor_splitter.go:100`) — so all 90 live `splitter_config` nodes are `standard` and **not
one of them writes `type`**. Reading the token off the wire alone leaves every one of them untyped.
The fix needs no new column: the default is already on the discriminator's own field row, so the
walker falls back to it, and rows of one struct disagreeing about that default is reported rather
than resolved. `InputChannelConfig` is the same shape and larger — 171 of its live nodes omit `type`,
against a validator default of `memory` (`actions_start_common.go:829`) — so B.2 inherits this
already handled rather than as a surprise. It also says something about the emitted schema: a
discriminated union whose discriminator is optional needs the default declared on the Pydantic field,
or nothing that consumes the schema can tell which branch an untagged object belongs to.

**The root type is two documents, and the schema must describe only one.** `ComputePipesConfig` has
three fields that can carry pipes. `conditional_pipes_config` and `reducing_pipes_config` are written
by the author; **`pipes_config` is written by JetStore, never by the author** — the startup actions
select one step's pipes and marshal a fresh `ComputePipesConfig` carrying them there
(`actions_start_sharding_cp.go:358`, `actions_start_reducing_cp.go:271`), and that per-step document
is what the whole runtime reads. The live corpus is unanimous: 29 files use
`conditional_pipes_config`, 20 use `reducing_pipes_config`, none uses both, none uses `pipes_config`.
Reflection over the struct sees three alternatives and cannot see the split, so a schema built from
it would permit the runtime shape — which validates cleanly and cannot run. Recorded as I-14, with
the further point that `reducing_pipes_config` is *superseded* rather than merely older, which the
matrix has no column for yet.

**`ollama` is no longer at zero.** The analysis and the plan both record it at zero as of 2026-08-06;
`workspaces/jets_ws/pipes_config/patient_profile.pc.json` gained an `ollama` transformation on
2026-08-08, so the corpus now holds a real exemplar for the operator the programme cares about most.
Recorded as I-12 — which also needs restating, since the transformation totals it quotes (678 against
the analysis's 676) were computed over all 71 files and are superseded by the live figures above.

## Running the checks

```bash
python3 -m venv .venv && .venv/bin/pip install -e .

.venv/bin/cpipes-contract check                        # coherence, while the matrix is partial
.venv/bin/cpipes-contract check --code ../..           # every citation resolves and names its field
.venv/bin/cpipes-contract check --corpus ../../..      # every exemplar resolves, and is a live config
.venv/bin/cpipes-contract check --strict --code ../.. --corpus ../../..

.venv/bin/cpipes-contract corpus --corpus ../../..           # recorded counts vs measured
.venv/bin/cpipes-contract corpus --corpus ../../.. --apply   # write the measured ones back
.venv/bin/cpipes-contract corpus --corpus ../../.. --unknown # keys no field row accounts for
```

One command, one exit code — there is no CI service to host it (I-9). `--strict` adds the checks that
only hold once extraction is complete, and its failures are the worklist: every `ref_struct` with no
`types.csv` row is a type B.2–B.6 has yet to reach. `corpus` without `--apply` is the drift check and
exits non-zero, so a corpus that moves under the matrix is caught rather than assumed.

**`--code` exists because I wrote 27 bad citations into the seed.** Drafting the rows, I inferred
line numbers from the shape of the file instead of reading them, and the doc-comment references were
mostly a line or two off — pointing at the neighbouring field's declaration, which is the failure mode
a reader would take at face value. The check requires the cited line itself to name the field, by its
Go name or its json key; the seed's citations are now computed by locating the field in the source
rather than typed, and **B.2–B.6 should do the same**. A citation that a program cannot follow is one
nobody will follow either.
