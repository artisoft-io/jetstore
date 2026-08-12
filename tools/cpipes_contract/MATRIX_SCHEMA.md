# The cpipes applicability matrix — schema

**Task B.1 · drafted 2026-08-11 · awaiting review**

This is the definition of the matrix, not the matrix. The rows currently in `matrix/` are a seed:
nine types and seventy-two fields, hand-extracted while designing the schema, kept because a schema
with no rows in it has not been shown to carry anything. B.2–B.6 fill the rest.

The reasoning behind the matrix — why reflection over `pipes_model.go` recovers nothing about
applicability, and why the schema is a projection of the matrix rather than a thing built directly —
is in the Phase 0 plan (`plan/1_phase0_plan.md` §5.2 of the `jetstore_agentic_ai` repo). This file
documents the columns. Where it departs from the plan's §5.2.1 it says so, and the argument for each
departure is issue **I-11** in that repo's `plan/tracking/3_phase0_tasks_issues_risks.md`.

## Three tables, not one

| File | One row per | What it is for |
|---|---|---|
| `matrix/types.csv` | **addressable type** — a Go struct paired with one value of its discriminator | The unit of a `$defs` entry, of a Pydantic subclass, and of a fragment-library part |
| `matrix/fields.csv` | **field of an addressable type** | The matrix proper: applicability, requirement, default, evidence |
| `matrix/constraints.csv` | **requirement spanning more than one field** | What `(applicable, required)` cannot express |

The plan asks for one CSV. It became three because the discriminator vocabulary, the corpus instance
count and the exemplar are properties of a *type* rather than of a field: carried on every field row
they would be repeated a dozen times and could disagree row to row, which is the two-sources-for-one-
fact problem decision 8 exists to object to. `fields.csv` remains the review artifact; the other two
are small (a few dozen rows each when complete).

## Conventions

- **Every cell is filled.** `-` means "none / not applicable". An empty cell is a schema error, so an
  unfilled row is always distinguishable from a row reviewed and found empty.
- **`*` is the type token of a struct with no discriminator** — `OllamaSpec/*` means every instance
  of `OllamaSpec`. A blank token would collide with "not yet determined".
- **Citations are `path:line` relative to the JetStore repo root**, the same convention the plan uses:
  `jets/compute_pipes/pipes_model.go:1152`.
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
| `corpus_instances` | Occurrences across the 71 `.pc.json`. |
| `exemplar_file`, `exemplar_path` | One real occurrence. Set exactly when `corpus_instances > 0`, and `check --corpus` resolves every one of them — a fragment library whose exemplars do not resolve is a catalogue of things that may not exist. |
| `doc_ref` | `file:line` of the type definition. |
| `description`, `notes` | Prose. `description` is destined for the Pydantic model; `notes` is not. |

The token vocabulary of a struct is the set of its rows here. It is deliberately not a column
anywhere, so it cannot drift from the rows it describes.

## `fields.csv`

Ten of these columns are the plan's §5.2.1 list. The other ten are marked **+**.

| Column | Rule |
|---|---|
| `go_struct`, `type_token` | Foreign key into `types.csv`. |
| `field_name` | The Go field name. |
| `json_key` | The wire key. |
| **+** `declared_in` | The struct that declares the field. Differs from `go_struct` when the field is promoted through an embed, which keeps the field inventory single-sourced while letting applicability be per host and per token — `FileConfig.Delimiter` is defaulted for `input_channel` type `stage` and not for the others. |
| `go_type` | The Go type as written, pointer and all. |
| **+** `container` | `scalar`, `object`, `array`, `map`, `raw_json`, `any`. |
| **+** `ref_struct` | The struct the value is, for objects, arrays of objects and maps of objects; `-` otherwise. Without it the matrix is a flat field list and nothing can compose fragments: this and `container` are what make the same rows serve the parts library. |
| **+** `values` | A closed value set, pipe-separated — `pass_through\|drop\|fail`. `-` on the discriminator field itself, whose vocabulary is the `types.csv` rows. Enums are a third of what a schema constrains and §5.2.1 has no column for them. |
| `applicable` | Does this field mean anything for this type token? `applicable=no` is the row that drives the `if`/`then` overlay's prohibitions. |
| `required` | **Three states, not two:** `yes`, `no`, `conditional`, plus `na` when the field is inapplicable. |
| **+** `required_when` | The condition, set exactly when `required` is `conditional`. `absent(output_channel.schema_provider)`. |
| `default` | The literal applied when the field is absent, or `-`. Empty when the field is inapplicable — a value forced by the builder goes in `notes`, not here. |
| **+** `default_by` | `validator`, `builder` or `none`. This decides who can observe the default: the config validator mutates its input, so a default it applies is already in the config by the time the harness looks; one the operator builder applies is invisible both at config level and to the schema. `pool_size` is applied by both. |
| `evidence` | `validator`, `builder`, `corpus`, `generator`, `comment` — the plan's authority order. **`reviewed` and `unreviewed` are not values here**; see `review`. |
| **+** `evidence_ref` | `file:line`, relative to the JetStore repo root. Required for every evidence kind except `corpus`, where `corpus_count` is the citation. A source kind with no location is not checkable, and the plan's own standard is that a claim without a citation does not belong. `check --code` resolves every one of them and requires the cited line to name the field — see below. |
| `corpus_count` | Occurrences of this field across instances of this type. Strong evidence for presence, weak for absence — a row claiming `applicable=no` with a non-zero count is rejected outright. |
| **+** `harness` | `pass`, `fail`, `untestable`, `pending` — the B.7 result for this row. The mitigation for R-1 is that reviewing a row means reading a test result, which is only true if the result is on the row. `untestable` is the honest state for builder-evidence rows the config validator never sees. |
| **+** `review` | `unreviewed`, `reviewed`, `disputed`. A separate axis from `evidence`: a row can be corpus-derived and reviewed, or validator-derived and unreviewed. R-1's number to watch — rows still unreviewed when B.9 wants to start — is not countable unless these are two columns. `disputed` is for rows where the sources disagree. |
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

## What the seed rows demonstrate

Nine types, seventy-two fields, nine constraints, chosen to exercise the parts of the schema that
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
- **`PipeSpec/merge_files.apply`** — `applicable=no` on 0/11 corpus occurrences and nothing else.
  Correct-looking and weakly evidenced, marked `unreviewed`, and exactly the kind of row the review
  is ordered to reach first.
- **`TransformationSpec/partition_writer.partition_writer_config`** — the only `disputed` row in the
  seed, and the most useful one. See below.
- **`InputChannelConfig/stage`** — the embedded-`FileConfig` case, four validator-applied defaults,
  and `bucket`, which the validator *overwrites* rather than defaults.

## Two findings from seeding it

**The validator and the corpus disagree about `partition_writer_config`.** The validator rejects a
`partition_writer` without one (`actions_start_common.go:908`), and 8 of the 181 corpus instances
have none. Those 8 carry `device_writer_type`, `partition_size` and `write_headers` flat on the
transformation instead, and `write_headers` exists nowhere in `jets/`. They are stale configs written
against an older shape, silently tolerated because Go's decoder ignores unknown keys — which is the
dead-key audit B.14 was told to run *before* turning on `additionalProperties: false`, arriving early
and unasked. It also qualifies exit criterion 6: "if a real config fails, the schema is wrong, not the
config" needs the exception these 8 files are.

**The corpus has moved since it was counted.** Measured 2026-08-11 it holds 678 transformation
instances, against the 676 the analysis's table gives (and the 674 its prose gives) as of 2026-08-06.
The 71-file count is unchanged; the difference is two instances added to
`workspaces/jets_ws/pipes_config/patient_profile.pc.json` on 2026-08-08. One of them is an `ollama`
transformation — so **`ollama` is no longer at zero**, and the corpus now contains a real exemplar for
the operator that had none. Recorded as I-12.

## Running the checks

```bash
python3 -m venv .venv && .venv/bin/pip install -e .

.venv/bin/cpipes-contract check                        # coherence, while the matrix is partial
.venv/bin/cpipes-contract check --code ../..           # every citation resolves and names its field
.venv/bin/cpipes-contract check --corpus ../../..      # every exemplar resolves
.venv/bin/cpipes-contract check --strict --code ../.. --corpus ../../..
```

One command, one exit code — there is no CI service to host it (I-9). `--strict` adds the checks that
only hold once extraction is complete, and its failures are the worklist: every `ref_struct` with no
`types.csv` row is a type B.2–B.6 has yet to reach. There are 16 against the seed, which is a fair
statement of how much of `pipes_model.go` remains.

**`--code` exists because I wrote 27 bad citations into the seed.** Drafting the rows, I inferred
line numbers from the shape of the file instead of reading them, and the doc-comment references were
mostly a line or two off — pointing at the neighbouring field's declaration, which is the failure mode
a reader would take at face value. The check requires the cited line itself to name the field, by its
Go name or its json key; the seed's citations are now computed by locating the field in the source
rather than typed, and **B.2–B.6 should do the same**. A citation that a program cannot follow is one
nobody will follow either.
