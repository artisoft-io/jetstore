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

The corpus is `workspaces/*/pipes_config/**` - 49 files. The `.pc.json` under `workspaces/*/data/`
are developer reference material JetStore never loads, and counting them manufactures contradictions
with the validator; see `cpipes_contract/corpus.py`.

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

The matrix is extracted (B.2) and under review; the harness (B.7) turns its rows into test results
so the review reads what the validator actually did. The plan it executes is
`projects/agentic_ai/plan/phase0_plan.md` §5.2 in the `jetstore_agentic_ai` repo, tasks B.1–B.8.
