# tasks_ts

A minimal TypeScript sample that sends a prompt to a **local Ollama server** and
turns the model's response into strongly-typed `Task` objects. The domain model
is defined with [**TypeBox**](https://github.com/sinclairzx81/typebox), and the
same schema is used both to constrain Ollama's structured output and to validate
the response on the client side.

This is the TypeScript twin of [`../tasks_py`](../tasks_py) (Pydantic version) —
both do exactly the same thing.

## What it does

1. Reads a prompt file containing a `System:` and a `User:` section
   (defaults to `initial_test_prompt.md` used in the `jets_ws` workspace).
2. Calls the local Ollama `/api/chat` endpoint, passing the TypeBox schema as
   the `format` field so the model returns JSON matching the `Task[]` shape.
3. Parses and validates the response against the TypeBox schema, printing either
   the validated tasks or the validation errors.

## Domain model

```ts
type Status = "todo" | "in_progress" | "done";

type Task =
  | { type: "email";     id: string; title: string; status: Status; recipient: string }
  | { type: "review";    id: string; title: string; status: Status; reviewer: string }
  | { type: "batch_job"; id: string; title: string; status: Status; recordCount: number };
```

See [src/schema.ts](src/schema.ts) for the TypeBox definitions.

## Requirements

- Node.js 18+ (uses the built-in `fetch`)
- A running [Ollama](https://ollama.com) server with a model that supports
  structured output (e.g. `gemma4:latest`)

## Install & run

```bash
cd tools/sample_projects/tasks_ts
npm install
npm run dev            # compiles and runs
```

Or in two steps:

```bash
npm run build
npm start
```

### Passing a different prompt file

```bash
npm run build
node dist/index.js /path/to/your_prompt.md
```

## Configuration

| Env var        | Default                    | Description                     |
| -------------- | -------------------------- | ------------------------------- |
| `OLLAMA_HOST`  | `http://localhost:11434`   | Base URL of the Ollama server   |
| `OLLAMA_MODEL` | `gemma4:latest`            | Model name to use               |

Example:

```bash
OLLAMA_MODEL=gemma4:e2b npm start
```

## Example output

```
Model      : gemma4:latest
Ollama host: http://localhost:11434
Prompt file: .../initial_test_prompt.md

Validated 2 task(s):

[
  { "type": "email",  "id": "1", "title": "Email summary to dana@plan.com", "status": "todo", "recipient": "dana@plan.com" },
  { "type": "review", "id": "2", "title": "Priya reviews the summary",      "status": "todo", "reviewer": "priya" }
]
```
