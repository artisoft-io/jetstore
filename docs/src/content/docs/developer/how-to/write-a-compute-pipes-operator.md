---
title: Write a compute-pipes operator
description: Add a new type of transformation step to the compute-pipes engine, from its configuration struct and its evaluator to the registration sites, the contract matrix and the tests.
kind: how-to
sidebar:
  order: 3
---

An operator is one `type` of a pipeline step: the value of `type` in a `TransformationSpec`
(`jets/compute_pipes/pipes_model.go`), with a configuration block of its own beside it. Adding one
touches the Go package in a fixed set of places and the compute-pipes contract in
`tools/cpipes_contract/`.

These steps follow the most recent operator JetStore added, `render`: commit `68b8b07e` added the
operator and its registration sites, and `8102d41b` its rows in the contract. On 2026-11-11 the
render tests and the contract checks below were run on a checkout of Release 4.1.6; the results are
given with each step. No operator was added.

**Two other routes may suit better.** An inference operator shares the inference plumbing, and
[Add an inference backend](/docs/developer/compute-pipes/how-to/add-an-inference-backend/) is its
checklist. An operator that only one installation needs can be a **site operator**, registered by
that installation's own node Lambda through `compute_pipes.WithOperators` and configured under
`site_config`, with no change to JetStore. Read
[A site operator runs in one of three processes](/docs/developer/compute-pipes/notes/site-operators/)
before choosing it; [Write a Lambda for a workspace](/docs/developer/how-to/write-a-workspace-lambda/)
covers the node Lambda.

## 1. Declare the configuration

In `jets/compute_pipes/pipes_model.go`:

- add the token to the comment above `TransformationSpec.Type`, which lists the range;
- add a pointer field for the operator's configuration to `TransformationSpec`, named
  `<Name>Config` with the json key `<name>_config` and `omitzero`, as
  `` RenderConfig *RenderSpec `json:"render_config,omitzero"` `` is;
- declare the configuration struct (`RenderSpec`) with a doc comment for each field.

**If the operator reports row-level failures**, give its struct an `ErrorChannel` and a
`MaxErrorCount`, as `RenderSpec` has: the error-channel synthesis writes to them (step 3).

**If the operator needs a new top-level element of the configuration**, as `render` needed
`text_templates`, add it to `ComputePipesConfig` and assign it in the node configuration the two
starters build (`actions_start_sharding_cp.go` and `actions_start_reducing_cp.go`; render's is
`TextTemplates`). A top-level element missing from either reaches no worker node, silently.
`TestTextTemplatesReachAWorkerNode` is the test that holds this for render.

## 2. Write the operator

Put it in its own file, `jets/compute_pipes/pipe_transformation_<name>.go`. Declare the token as a
constant (`RenderOperatorType`), a type that implements `PipeTransformationEvaluator`
(`jets/compute_pipes/pipesmodel/model.go`: `Apply`, `Done` and `Finally`), and its constructor on
`BuilderContext`:

```go
func (ctx *BuilderContext) NewRenderTransformationPipe(source *InputChannel, outputCh *OutputChannel,
	spec *TransformationSpec) (*RenderTransformationPipe, error)
```

`Apply` receives one input record and sends what it produces on the output channel; `Done` runs when
the input is exhausted; `Finally` runs in every case, to release what the operator holds. **Report
every failure that the configuration alone decides from the constructor**, not from `Apply`: a
missing column or an unknown template is then a configuration error at the start of the step instead
of a failure on the first record.

## 3. Register the token

Each of these is a `switch` on the operator type, and the new token needs an arm in each. They are
the sites the render commit touched:

| Function | File | What the arm does |
|---|---|---|
| `BuildPipeTransformationEvaluator` | `pipes_runtime_model.go` | calls the constructor; this is the dispatch |
| `ValidatePipeSpecConfig` | `actions_start_common.go` | the checks that need no channels, run once at startup |
| `errorChannelConfig` | `actions_start_common.go` | returns the operator's `ErrorChannel` |
| `reportsRowLevelFailures` | `error_channel_default.go` | true when the operator reports failed rows |
| `setErrorChannelConfig` | `error_channel_default.go` | places a synthesised error channel on the operator's config |
| `setDefaultMaxErrorCount` | `error_channel_default.go` | applies the deployment's default cap |

Then add the token to `builtinOperatorTypes` in `site_operators.go`, the set of tokens a site
operator may not take. `TestBuiltinOperatorTypesMatchesTheDispatch` parses the dispatch's `switch`
out of `pipes_runtime_model.go` and fails when the two disagree.

**`errorChannelConfig` and `reportsRowLevelFailures` must name the same operators**, or an operator
gets an error channel nothing finds, or finds one nothing gave it. An operator that never reports a
failed row has no arm in either, nor in the two setters.

**`ValidatePipeSpecConfig` runs in the starters, without the channels**, so it can check only what
the configuration says on its own (the `render` arm checks that `render_config`, `template_name` and
the two columns are present). Everything that needs the channels belongs in the constructor.

## 4. Add the operator to the contract

The contract describes every type and field a `.pc.json` may carry. It is what
`cpipes_schema.json` and the generated
[configuration reference](/docs/developer/reference/generated/pipeline-configuration/) are built from,
and a document using a token the contract does not have fails `cpipes-contract validate` at the
`oneOf` over `TransformationSpec`. In `tools/cpipes_contract/`:

1. **`matrix/types.csv`**: a row `TransformationSpec,<token>,TransformationSpec<Name>,type,...`, and a
   row for the configuration struct (`RenderSpec,*,RenderSpec,...`) and for any struct it reaches as
   a value.
2. **`matrix/fields.csv`**: a row for each field of the new structs, and a `<name>_config` row on
   **every** `TransformationSpec` token, applicable on the new one and not on the others. Render's
   cost 74 field rows.
3. **`cpipes_model.py`**, the Pydantic model, which is the source of truth for the claim columns
   (applicable, required, default, values, description): add the new types, then run `reflect` to
   write those columns back into `fields.csv`.
4. Regenerate the two artefacts: `cpipes-contract schema --out cpipes_schema.json` and
   `cpipes-contract gofile --out ../../jets/compute_pipes/cpipes_contract_data.go`. The second is
   checked in and carries one `TransformationSpec/<token>` entry per operator.

The columns and their rules are in [The applicability matrix](/docs/developer/tools/cpipes-contract/matrix/),
and why the model is the source of truth in
[Why the contract is a matrix](/docs/developer/tools/cpipes-contract/design/).

## 5. Run the checks and the tests

The contract checks, from `tools/cpipes_contract/` with the tool installed as
[Run the contract checks](/docs/developer/tools/cpipes-contract/run-the-checks/) describes:

```bash
.venv/bin/cpipes-contract check              # coherence of the three tables
.venv/bin/cpipes-contract reflect --check    # the model and the matrix agree
.venv/bin/cpipes-contract drift --code ../.. # the matrix's fields against pipes_model.go
.venv/bin/cpipes-contract harness --code ../.. # a synthesised config per type, through ValidatePipeSpecConfig
uv run --extra dev pytest -q
```

The harness builds a minimal configuration from each type's own rows and runs it through
`ValidatePipeSpecConfig`; a type from whose rows no valid configuration can be built fails it, which is
how render's first modelling of its template elements was caught.

The Go tests, from the repository root:

```bash
go test -count=1 -run 'Render|Builtin|Reserved|ErrorChannel|SiteOperator|Contract' ./jets/compute_pipes/
```

Write the operator's own tests beside it, `pipe_transformation_<name>_test.go`. Render's
(`pipe_transformation_render_test.go`) are the model: the records it writes, each failure it reports
and how often, the build failures (`TestRenderBuildFailures`), and two that test the registration
rather than the operator: `TestRenderIsWiredIntoTheErrorChannelSynthesis` and
`TestRenderIsValidatedAtStartup`.

What these gave on 2026-11-11, before any change:

| Command | Result |
|---|---|
| the Go tests above | pass |
| `check` | `ok: 142 types, 2170 fields, 46 constraints` |
| `reflect --check` | `model and matrix agree` |
| `harness --code ../..` | 137 types pass, 0 fail |
| `pytest` | 99 passed, 1 skipped |
| `schema` | identical to the checked-in `cpipes_schema.json` |
| `gofile` | **differs** from the checked-in `cpipes_contract_data.go` by one line: `cluster_config` of `ComputePipesConfig` is required in the matrix and not in the file |
| `drift --code ../..` | **6 differences**: six `response_format` and `config` fields declared `jsontext.Value` in Go and `json.RawMessage` in the matrix |
| `check --code ../..` | **2,012 problems**: citations in the matrix that no longer name their field |

**So read the last three as a baseline, not as a pass.** Run them before the change and after it,
and compare: a new operator should add nothing to the drift or to the citation problems, and its
regenerated `cpipes_contract_data.go` should differ from the checked-in one only by its own entries
and by the line above.

## 6. Document it

The operator's section of the
[configuration reference](/docs/developer/reference/generated/pipeline-configuration/) is generated
from its matrix rows. Add a page of its own under `developer/compute-pipes/operators/` when it needs
more than the reference says, as [render](/docs/developer/compute-pipes/operators/render/) has.

## See also

- [Pipeline configuration as a contract matrix](/docs/developer/architecture/decisions/contract-matrix/): why the matrix rows are part of the change.
