# cpipes_contract

The cpipes config contract: the applicability matrix, its schema, and the checks over it.

`pipes_model.go`'s `omitzero` and `omitempty` tags mean **applicable**, not **required**, so
reflection over the Go structs recovers the field inventory and nothing about applicability — and
applicability is the whole contract. A JSON Schema listing fourteen optional config objects under
every `type` is not a weak schema, it is a useless one: it permits precisely the configs a model is
most likely to invent.

So the contract is extracted from the code and the corpus into a reviewable matrix, and the schema
becomes a projection of it. This package holds the matrix and the machinery that keeps it honest.

```
matrix/types.csv        one row per addressable type (Go struct + discriminator value)
matrix/fields.csv       one row per field of one of those types - the matrix proper
matrix/constraints.csv  requirements spanning more than one field
cpipes_contract/        the schema as Pydantic models, the checks, the corpus walker, the harness
harness/                the Go runner: feeds synthesized configs through ValidatePipeSpecConfig
```

The corpus, for this package, is **every `.pc.json` under `workspaces/` whose path has a
`pipes_config` directory in it**, at any depth (`config_files` in `cpipes_contract/corpus.py`), and
it subtracts nothing. Anything else under `workspaces/` is counted as retired and not walked: the
`.pc.json` that used to sit under `workspaces/*/data/` are developer reference material JetStore
never loads, and counting them manufactured contradictions with the validator.

**Until 2026-11-10 this paragraph said the corpus was that set *minus what each directory's
`jets_assets_manifest.json` names*. No module here reads that manifest** (`git log -S manifest`
over this package finds no commit that ever made one read it). The
subtraction is the other corpus definition in the tree, `jets/agentic/eval`'s, and this sentence
described it rather than the walker it sits beside. The two coexist and answer different
questions:

| Tool | Rule | Files, 2026-11-10 |
|---|---|---:|
| `cpipes-contract corpus`, `check --corpus` (`config_files`) | every `.pc.json` under a `pipes_config` directory in `workspaces/` | **51** |
| `jets/agentic/eval` (`LoadCorpus`, `jetstoreOwnedAssets`) | `workspaces/*/pipes_config/**`, less the names in that directory's `jets_assets_manifest.json` | **46** |

**The five files between them are installed copies of JetStore's own two configs**
(`embed_input_parts.pc.json`, `jets_loader.pc.json`), which `install_workspace_assets` writes into
each workspace and the workspace repositories do not track. So this package's number depends on
whether the installer has run in the checkout, and how recently: on a fresh clone, with nothing installed, the two rules agree at the
tracked count, 46. Measured 2026-11-10 over the four workspaces at the commits the integration
checkout pinned that day, with the installer's output present, by calling `config_files` and by
running `TestAgainstTheRealCorpus` with `-count=1` (*46 live files of 51 on disk*); 0 retired. The
figures this paragraph carried before (53 on 2026-09-19, 42 on 2026-09-11, 49 before 2026-09-08)
were stated under the manifest rule while this walker applied its own, and the corpus grows from
commits in the workspace repositories, so read them as dated measurements rather than as a series.

**[MATRIX_SCHEMA.md](MATRIX_SCHEMA.md) is the column reference** and the document to read first.

Developer tooling: it is not copied into any image, and nothing on the cpipes runtime path depends
on it.

```bash
python3 -m venv .venv && .venv/bin/pip install -e .
.venv/bin/cpipes-contract check --code ../.. --corpus ../../..   # coherence, citations, exemplars
.venv/bin/cpipes-contract corpus --corpus ../../..               # recorded counts vs measured
.venv/bin/cpipes-contract harness --code ../..                   # every row becomes a test result
.venv/bin/cpipes-contract stamp                                  # certify what the review marked
```

The test suite is `tests_*.py` and needs that pattern declared, which `pyproject.toml` now does —
`pytest` here collected **nothing** and exited 0 until 2026-09-19. Run it with
`uv run --extra dev pytest -q`.

**The model is two projections, and only one of them is the schema.** `ComputePipesConfig` is the
*authored* document - what an author writes and what `check --corpus` walks - and
`ComputePipesRuntimeConfig` is what a worker node is handed: the same fields plus
`common_runtime_args` and `pipes_config`, which the starters fill in and no author writes. The
matrix carries both as rows of one `go_struct` and marks the second pair `applicable=no`, because
`pipes_model.go` has one struct for both shapes; the emitted schema is the authored projection only,
and `negative_suite.json`'s *root pipes_config (I-14 runtime shape)* case is what holds it to that.

**Measured by enumeration on 2026-09-21 at `d1df2e9f`: 15 json tags on the Go struct, 13 fields on
`ComputePipesConfig`, 15 on `ComputePipesRuntimeConfig`.** It is the fourth independent measurement
of the first two numbers and the first of the third. The numbers are a property of a struct under
active development, so `tests_runtime_document.py` asserts the *relation* - every json tag is a
runtime field, and what the runtime model adds is what the matrix calls inapplicable - and this
paragraph records the measurement with its date rather than standing in for the check.

**Until 2026-09-21 no class here accepted a runtime document at all**, which is
`healthcare_corpus`'s `P9-I28` (read 2026-09-20): `extra="forbid"` refused every document a node
has ever been given, and the corpus walk could not see it because a node is never handed an
authored document. `tools/cpipes_node` carried a local widening for it, and that widening is now
retired. **A green check over the wrong half of a document space is the failure to take from this**,
not a two-field oversight.

**The two artefacts this package emits are two readers of one contract, and `tests_schema.py` is
where they are held to each other.** `cpipes_model.py` is the Pydantic model and the source of truth
for the claims; `cpipes_schema.json` is the projection a Go consumer and every typed hole reads. They
can disagree silently, and did: gap 2b (`I-778`) put the `~site` complement branch into the schema
and not into the model's `TransformationSpec` alias, and no document asked both until one naming a
site operator was authored. A union carrying an `unlisted(...)` token is now emitted as an ordered
union of the tagged members and that branch, and `schema.splice_complement_branches` folds Pydantic's
`anyOf` rendering of it back into the single discriminated `oneOf` — so the emitted file is
byte-identical and the model validates what the schema accepts.

The matrix is extracted (B.2) and under review; the harness (B.7) turns its rows into test results
so the review reads what the validator actually did. The plan it executes is
`projects/agentic_ai/plan/phase0_plan.md` §5.2 in the `jetstore_agentic_ai` repo, tasks B.1–B.8.
