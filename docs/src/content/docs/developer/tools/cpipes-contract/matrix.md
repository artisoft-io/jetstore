---
title: The applicability matrix
description: The conventions and every column of the compute pipes contract matrix, types.csv, fields.csv and constraints.csv, and the coherence rules the check enforces.
kind: reference
sidebar:
  order: 0
---

This is the definition of the matrix in `tools/cpipes_contract/matrix/`, not the matrix itself. The
[compute pipes configuration reference](/docs/developer/reference/generated/pipeline-configuration/)
is generated from it. Why it has three tables, and why every reachable struct gets a row, is in
[Why the contract is a matrix](/docs/developer/tools/cpipes-contract/design/#three-tables-not-one).

## Conventions

- **A cell has three states, and they never collapse into each other.** `-` is a claim: "none, and
  judged to be none". A **blank** is the absence of a claim: mechanically extracted, awaiting review.
  The inventory pass leaves every judgment column blank, so that a value in one of those columns is
  always someone's decision and never a generator's guess. A blank is legal only on the judgment
  columns of `fields.csv` and `constraints.csv`: identity columns (the struct, the field, the wire
  key, the container) come from the source and a blank one is a schema error, as is any blank on a
  row marked `reviewed`. `check` reports the blank count on every run, and `--strict` fails each
  one, because at end state the worklist must be empty. Without a legal blank, a mechanical
  extraction pass would have had to *invent* values for thousands of judgment cells just to satisfy
  the loader, manufacturing the unreviewed claims the rule exists to prevent.
- **`*` is the type token of a struct with no discriminator**: `OllamaSpec/*` means every instance
  of `OllamaSpec`. A blank token would collide with "not yet determined".
- **`~name` is a *virtual* token: a variant selected by the shape of the node rather than by a value
  of the discriminator.** Its membership test lives in `variant_when`: `present(key)` (the key is
  set and not null) or `absent(key)` (missing, null, or the empty string, the reading Go gives an
  omitted `string`). The engine contains two such discriminations. An `ExpressionNode` is `~unary`
  when `arg` is set and `~binary` when `lhs` is (`BuildExprNodeEvaluator` in
  `jets/compute_pipes/eval_expression.go` tests them in that order, before consulting `type`). A
  `TransformationSpec` at `conditional_config.N.then` with no `type` at all is `~override`, a
  field-level override of its host, where a `then` *with* a `type` is a full replacement
  (`MergeTransformationSpec` in `jets/compute_pipes/actions_start_common.go`). Virtual rows still
  carry the struct's real discriminator, so the one-discriminator-per-struct rule stays checkable;
  they are tested **in their row order, before any value token**, which is why `~unary` precedes
  `~binary` in the file: the row order mirrors the dispatch order of the code. The mechanical naming
  rule applies with the `~` dropped: `CamelCase(override) + TransformationSpec =
  OverrideTransformationSpec`. A union with virtual members is not a plain discriminated union: the
  JSON Schema branches become `if`/`then` overlays on key presence, and the Pydantic side needs a
  pre-validator rather than a discriminator field.
- **`unlisted(key)` is the third predicate, and a different kind of membership from the other two.**
  `present` and `absent` read the *shape* of the node; `unlisted` reads the discriminator and asks
  whether its value is one this struct has a row for. It is the matrix's spelling of a `default:`
  branch, and it exists because the engine has one: `BuildPipeTransformationEvaluator`
  (`jets/compute_pipes/pipes_runtime_model.go`) sends a `TransformationSpec` whose `type` names none
  of the built-in operators to the site operator registry, so the variant's membership is the
  **complement of the recorded token vocabulary** and no key's presence decides it.
  `TransformationSpec/~site` is the case and is the only one today. Three consequences:
  - An `unlisted` row **keeps its discriminator field**, alone among virtual rows: `~override` is
    selected by `type` being absent, so carrying it would contradict the membership, while `~site`
    is nothing without it, and the field is `required=yes`.
  - It **joins its union's `oneOf`**, which no other virtual token does, as a branch whose
    discriminator is a string with `"not": {"enum": [the value tokens]}` and `"minLength": 1`. The
    enum is derived from `types.csv` rather than listed, so the branch is the exact complement of the
    union's other branches and cannot drift from them.
  - An **absent** discriminator is *not* unlisted: a missing key is the defaulted-token case (the
    `standard` splitters that write no `type`), which the walker settles from the field row's
    `default`, so `unlisted` requires a value and is indifferent to row order against an
    `absent(...)` sibling. The empty string is likewise not unlisted: it is the `~override` shape,
    and `WithOperators` refuses to register an operator under an empty name.
- **Citations are `path:line` relative to the JetStore repository root**, for example
  `jets/compute_pipes/pipes_model.go:<line>`. A citation beginning with `workspaces/` resolves
  against the repository holding the corpus, the parent of the code root, because generator evidence
  (workspace Lambdas) lives in the workspaces beside the JetStore checkout, not inside it.
  `check --code` follows both.
- **The corpus is every `.pc.json` under a `pipes_config` directory in `workspaces/`**, with nothing
  subtracted (`config_files`, `cpipes_contract/corpus.py`). The `.pc.json` files under
  `workspaces/*/data/` are developer notes and reference material that JetStore never loads; they
  are not counted, and `check --corpus` refuses one as an exemplar.
- **The corpus is authored documents.** The root type also serialises a *runtime* shape that
  JetStore writes for itself, and the schema describes only the authored one.
- **`test/` is a tier, not an exclusion.** Configs under `workspaces/*/pipes_config/*/test/` are real
  and must validate, so they stay in the corpus and in every count. They are also counted
  separately, because a field attested only by a test config is weaker evidence than one a production
  pipeline depends on, and exemplars prefer a production occurrence.
- **The validator wins.** It runs on every execution, so a shape it rejects cannot be in service. A
  config that contradicts it is not evidence against it, it is evidence of its own age. Corpus
  evidence is strong for presence and weak for absence, and is worth nothing against the validator.
- **Corpus paths are relative to the repository holding `workspaces/`**, and the path into the file
  is dot notation with numeric indices, the same notation `OllamaMappingSpec.Path` uses, so there is
  one path convention rather than two.
- **Pipe-separated lists** (`values`, `members`); comma-separated only for `embeds`.
- Descriptions are flattened to one line on write, so the files stay diffable and open cleanly in a
  spreadsheet.

## `types.csv`

| Column | Rule |
|---|---|
| `go_struct` | The Go struct name, as declared. |
| `type_token` | One value of that struct's discriminator, `*` when it has none, or a `~virtual` token (see *Conventions*). |
| `defs_name` | Mechanical: `CamelCase(type_token) + go_struct`, or `go_struct` when the token is `*`. `OllamaTransformationSpec`, `MergeFilesPipeSpec`, `StageInputChannelConfig`. A virtual token drops its `~` first; hyphens split like underscores (`de-identification` → `DeIdentificationAnonymizeSpec`). Where the token repeats a word of the struct the name stutters, `OutputOutputChannelConfig`, `SqlLookupLookupSpec`, and the stutter stands: it is the price of a rule under which no hand-picked pair of names can collide. The check enforces the rule and also that every `defs_name` is unique, so the `$defs` key, the Pydantic class name and the fragment-library entry are one name rather than three conventions. |
| `discriminator` | The **json key** of the discriminating field, or `-`. Not always `type`: `PartitionWriterSpec` discriminates on `device_writer_type`. Virtual rows carry it too, so all rows of one struct can be checked to agree on it, **except where a struct discriminates only by shape, and then every row carries `-`**. `Element` is the case: a paragraph and a group are told apart by which of `text` and `elements` is present and by nothing else, so there is no key to name, and `variant_when` is where the membership is written. The check enforces *`-` exactly when the token is `*` or the struct's variants are all `~virtual`*. |
| `variant_when` | The membership predicate of a `~virtual` token, `present(key)`, `absent(key)` or `unlisted(key)`, and `-` on every other row. The corpus walker and the emitted schema both read it. `unlisted` names the struct's own discriminator and means *a value no row of this struct claims*. |
| `embeds` | Structs embedded anonymously, whose fields are promoted onto this type on the wire. `InputChannelConfig` embeds `FileConfig`. |
| `fragment` | Whether this type can be authored and validated standing alone. Expected to be `yes` almost everywhere; a `no` must say why in `notes`. |
| `deprecated` | Superseded but still valid; as in `fields.csv`. |
| `corpus_instances`, `corpus_prod_instances` | **Measured.** Occurrences across the live corpus, and the subset outside a `test/` directory. Written by `corpus --apply`, never by hand. |
| `exemplar_file`, `exemplar_path` | One real occurrence, from a live config, preferring a production one over a `test/` one. `check --corpus` resolves every one, refuses a retired one, and demands more than resolution: the node must be an **object that is an instance of this row**. A path landing on the containing array fails, and so does a node whose discriminator carries another row's token or that fails a virtual row's predicate. While extraction is partial, a row the walk cannot reach yet may carry a *hand-proposed* exemplar, which those checks verify and which `corpus --apply` replaces with a measured one as soon as the type becomes reachable; "set exactly when `corpus_instances > 0`" is therefore a `--strict` check. **`exemplar_path = -` means the exemplar is the whole file**, which is the root type's case and only its case; see below. |
| `doc_ref` | `file:line` of the type definition. `check --code` resolves it and requires the cited line to name the struct, the same discipline `evidence_ref` gets, and what catches a filename remembered rather than read. |
| **+** `harness` | **Written by the machine**, like its `fields.csv` namesake: did `ValidatePipeSpecConfig` accept the minimal config the harness synthesised for this type from the matrix's own claims? `pass`, `fail`, or `pending`. A type has no `untestable`, since every type can at least be embedded and offered. A `fail` here is the review's entry point for the whole type: its field rows stay `pending` until the type's own acceptance is settled, because removal tests against a rejected base prove nothing. See [the harness](/docs/developer/tools/cpipes-contract/harness/). |
| `description`, `notes` | Prose. `description` is destined for the Pydantic model; `notes` is not. |

The token vocabulary of a struct is the set of its rows here. It is deliberately not a column
anywhere, so it cannot drift from the rows it describes.

**The exemplar of the root type is the file itself, and its `exemplar_path` is `-`.** The dot path is
borrowed from `OllamaMappingSpec.Path`, where an *empty* path means the whole response (in
`jets/compute_pipes/pipe_transformation_ollama.go`, the path stays nil and the root object is used).
The `-` sentinel carries that meaning here: no path into a file is the file. The two columns
disambiguate each other:

| `exemplar_file` | `exemplar_path` | Means |
|---|---|---|
| `-` | `-` | No exemplar at all: `corpus_instances = 0`, as `SplitterSpec/ext_count` |
| a live config | `-` | The exemplar **is** that whole document: `ComputePipesConfig` |
| a live config | `conditional_pipes_config.6.pipes_config.3.apply.0` | That node inside it |

The fourth combination, a path with no file to resolve it against, is rejected by the check.

## `fields.csv`

The columns marked **+** were added to the original column list during the design.

| Column | Rule |
|---|---|
| `go_struct`, `type_token` | Foreign key into `types.csv`. |
| `field_name` | The Go field name. |
| `json_key` | The wire key. |
| **+** `declared_in` | The struct that declares the field. Differs from `go_struct` when the field is promoted through an embed, which keeps the field inventory single-sourced while letting applicability be per host and per token: `FileConfig.Delimiter` is defaulted for `input_channel` type `stage` and not for the others. |
| `go_type` | The Go type as written, pointer and all. |
| **+** `container` | `scalar`, `object`, `array`, `array2`, `map`, `raw_json`, `any`. `array2` is `[][]T`, which three fields of the model need: `reducing_pipes_config`, `date_formats`, `other_date_formats`. |
| **+** `ref_struct` | The struct the value is, for objects, arrays of objects and maps of objects; `-` otherwise. Without it the matrix is a flat field list and nothing can compose fragments: this and `container` are what make the same rows serve the parts library. |
| **+** `values` | A closed value set, pipe-separated: `pass_through\|drop\|fail`. `-` on the discriminator field itself, whose vocabulary is the `types.csv` rows. |
| `applicable` | Does this field mean anything for this type token? `applicable=no` is the row that drives the `if`/`then` overlay's prohibitions. |
| **+** `deprecated` | Superseded but still valid. **A third axis, not a state of `applicable`**: a deprecated field is applicable, is present in the corpus, and must still validate, three things `applicable=no` denies. What it must not do is appear in the fragment library or in anything a model is prompted to produce. `reducing_pipes_config` is the known case; the check rejects `deprecated=yes` with `applicable=no` as two different claims conflated. |
| `required` | **Three states, not two:** `yes`, `no`, `conditional`, plus `na` when the field is inapplicable. |
| **+** `required_when` | The condition, set exactly when `required` is `conditional`. `absent(output_channel.schema_provider)`. |
| `default` | The literal applied when the field is absent, or `-`. A value the builder *forces*, writing it whatever the author put there, is not a default and belongs in `notes`; the row is `applicable=no`, and `TransformationSpec/ollama.new_record` is the case. |
| **+** `default_by` | `validator`, `builder` or `none`. This decides who can observe the default: the config validator mutates its input, so a default it applies is already in the config by the time the harness looks; one the operator builder applies is invisible both at config level and to the schema. `pool_size` is applied by both. |
| `evidence` | `validator`, `builder`, `corpus`, `generator`, `comment`, in that order of authority. **`reviewed` and `unreviewed` are not values here**; see `review`. |
| **+** `evidence_ref` | `file:line`, relative to the JetStore repository root. Required for every evidence kind except `corpus`, where `corpus_count` is the citation. A source kind with no location is not checkable. `check --code` resolves every one of them and requires the cited line to name the field. |
| `corpus_count` | **Measured.** Occurrences of this field across instances of this type. Strong evidence for presence, weak for absence: a row claiming `applicable=no` with a non-zero count is rejected outright. On a type the corpus walk cannot reach (runtime-injected shapes), `corpus --apply` stamps `-` instead of leaving the cell to await review forever; a later run that reaches the type replaces the stamp with measured counts. |
| **+** `corpus_prod_count` | **Measured.** Of those, the ones outside a `test/` directory. A field attested only by test configs is weaker evidence than one a production pipeline depends on, and `corpus_count == n, corpus_prod_count == 0` is the pattern the review should reach early. Written by `corpus --apply` and never typed. |
| **+** `harness` | **Written by the machine.** `pass`, `fail`, `untestable`, `pending`: the harness result for this row. Reviewing a row means reading a test result, which is only true if the result is on the row. A `required=yes` (or satisfied `required_when`) row is `pass` when removing the field got the document rejected; a `required=no` row is `pass` when the type's minimal config, which omits every optional field, was accepted, absence tolerated being exactly that row's claim. `untestable` is the state for claims the config validator cannot see: a builder-enforced requirement whose removal was still accepted, a prohibition (`applicable=no`: the decoder ignores what it does not know), or a discriminator or `present()` membership key, whose removal *re-types* the node rather than invalidating it. `fail` is reserved for a claim the validator contradicts: `evidence=validator` and the removal was accepted anyway. See [the harness](/docs/developer/tools/cpipes-contract/harness/). |
| **+** `review` | **Written by you.** `unreviewed`, `reviewed`, `disputed`: the human sign-off, set by hand and by nothing else. A separate axis from `evidence`: a row can be corpus-derived and reviewed, or validator-derived and unreviewed. `disputed` is for rows where you judge the sources to disagree; it is a verdict, not a measurement. |
| **+** `reviewed_hash` | **Written by `stamp`.** The fingerprint of what a `reviewed` mark certifies, every column except `harness`, `review`, `notes` and the stamp itself, and `-` until stamped. A row reviewed and then changed by a re-extraction or a `corpus --apply` is reported by `check` as *changed since it was reviewed*, and only an explicit `stamp --restamp` re-certifies it. Mark a row `reviewed`, run `stamp`; the command never touches `review` itself, so a reviewed mark cannot be manufactured by a re-run. `harness` is deliberately outside the fingerprint: it re-runs freely and a regression there is caught on its own axis. |
| `description` | The Go doc comment, carried across into the Pydantic field description, after which the Go comments stop being the source. Last column, so a spreadsheet review can hide it. |
| `notes` | Everything else, including why a weak row is weak. |

**Who fills in each column**: the columns marked **Measured** are written by `corpus --apply`; the
rest are extracted from the code by hand; `harness` is written by the test run; `review` is the one
column that is yours, and nothing in the toolchain writes it; `reviewed_hash` is written by `stamp`,
which certifies the marks you made and clears the ones you withdrew.

The inventory pass fills only the identity columns (field name, wire key, declarer, Go type,
container, `ref_struct`, all read from the declarations), plus `harness=pending` and
`review=unreviewed`, which are defined initial states rather than judgments. Everything else starts
blank, `corpus --apply` fills the measured counts as the walk reaches each type, and the review turns
the rest into claims one row at a time.

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
| `go_struct`, `type_token` | The **innermost type from which every member is reachable**. Members below it are dotted paths: `ollama_config.output_mapping`. |
| `kind` | `one_of`, `at_least_one`, `mutually_exclusive`, `precedence` (all may be set, the first member wins), `requires`, `forbids`, `external`. |
| `members` | Pipe-separated json keys, in significant order for `precedence`. For `external`, the first member is the field and the rest name something outside the type: another section of the document, or the pipe's position. |
| `enforce` | `schema` (expressible as an `if`/`then` overlay), `validator` (needs the Go validator or the harness), `prompt` (expressible only as an instruction to a model). Knowing which constraints fall to `prompt` is knowing where the contract stops applying. |
| `evidence`, `evidence_ref`, `review`, `reviewed_hash` | As in `fields.csv`. |
| `notes` | Prose. |
