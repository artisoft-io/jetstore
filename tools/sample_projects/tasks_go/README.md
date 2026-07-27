# tasks_go

A minimal Go sample that sends a prompt to a **local Ollama server** and
turns the model's response into `Task` objects. A single **JSON Schema**
([`task_schema.json`](task_schema.json)) is used both to constrain Ollama's
structured output (the request `format` field) and to validate the response
client-side with [santhosh-tekuri/jsonschema](https://github.com/santhosh-tekuri/jsonschema).

This is the Go twin of [`../tasks_py`](../tasks_py) (Pydantic) and
[`../tasks_ts`](../tasks_ts) (TypeBox) — all three do the same thing.

## What it does

1. Reads a prompt file containing a `System:` and a `User:` section
   (defaults to `initial_test_prompt.md` used in the `jets_ws` workspace).
2. Calls the local Ollama `/api/chat` endpoint, passing the JSON Schema as the
   `format` field so the model returns JSON matching the `[]Task` shape.
3. Parses the response and validates it against the same JSON Schema, printing
   either the validated tasks or the validation errors.

## Domain model

The schema in [`task_schema.json`](task_schema.json) mirrors this TypeScript
type (see [`../task_schema_from_ts.json`](../task_schema_from_ts.json)):

```ts
type Status = "todo" | "in_progress" | "done";

type Task =
  | { type: "email";     id: string; title: string; status: Status; recipient: string }
  | { type: "review";    id: string; title: string; status: Status; reviewer: string }
  | { type: "batch_job"; id: string; title: string; status: Status; recordCount: number };
```

The three variants form a discriminated union keyed on `type`.

## Requirements

- Go 1.26+ (uses the parent module at
  [`../../../go.mod`](../../../go.mod) — there is **no** separate `go.mod` here)
- A running [Ollama](https://ollama.com) server with a model that supports
  structured output (e.g. `gemma4:latest`)

The only extra dependency is
`github.com/santhosh-tekuri/jsonschema/v6`, already added to the parent module.

## Run

From the repository root (`jetstore/`):

```bash
go run ./tools/sample_projects/tasks_go/
```

Or build a binary:

```bash
go build -o /tmp/tasks_go ./tools/sample_projects/tasks_go/
/tmp/tasks_go
```

### Passing a different prompt file

```bash
go run ./tools/sample_projects/tasks_go/ /path/to/your_prompt.md
```

## Configuration

| Env var        | Default                    | Description                     |
| -------------- | -------------------------- | ------------------------------- |
| `OLLAMA_HOST`  | `http://localhost:11434`   | Base URL of the Ollama server   |
| `OLLAMA_MODEL` | `gemma4:latest`            | Model name to use               |

Example:

```bash
OLLAMA_MODEL=gemma4:e2b go run ./tools/sample_projects/tasks_go/
```

## Example output

The debug payload is written to **stderr**; the result is written to **stdout**.

```text
Model      : gemma4:latest
Ollama host: http://localhost:11434
Prompt file: .../initial_test_prompt.md

Validated 2 task(s):

[
  { "type": "email",  "id": "1", "title": "Email summary to dana@plan.com", "status": "todo", "recipient": "dana@plan.com" },
  { "type": "review", "id": "2", "title": "Priya reviews the summary",      "status": "todo", "reviewer": "priya" }
]
```

If the model returns JSON that does not match the schema, validation fails and
the raw output is printed so you can see what the model produced:

```text
model output failed schema validation:
jsonschema validation failed with 'task_schema.json#'
- at '/0': 'anyOf' failed
...

Raw output:
[ { "task": "email the summary to dana@plan.com", "status": "todo" } ]
```

This is expected behaviour when a model does not honour the structured-output
`format` — the client-side JSON Schema validation is what catches it.

## Notes

- The schema is `go:embed`ed so the sample is self-contained and always
  validates against the exact schema it sends to Ollama.
- `task_schema.json` reuses the label `"$id": "Status"` in each variant (as
  emitted by the TypeScript-to-JSON-Schema converter). Ollama tolerates this,
  but a strict draft 2020-12 validator rejects the duplicate anchors, so the
  sample strips `$id` keys before compiling the schema for validation.
