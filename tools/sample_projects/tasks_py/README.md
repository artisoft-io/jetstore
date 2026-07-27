# tasks_py

A minimal Python sample that sends a prompt to a **local Ollama server** and
turns the model's response into strongly-typed `Task` objects. The domain model
is defined with [**Pydantic**](https://docs.pydantic.dev), and the same schema is
used both to constrain Ollama's structured output and to validate the response
on the client side.

This is the Python twin of [`../tasks_ts`](../tasks_ts) (TypeBox version) —
both do exactly the same thing.

## What it does

1. Reads a prompt file containing a `System:` and a `User:` section
   (defaults to `initial_test_prompt.md` used in the `jets_ws` workspace).
2. Calls the local Ollama `/api/chat` endpoint, passing the Pydantic-derived
   JSON Schema as the `format` field so the model returns JSON matching the
   `list[Task]` shape.
3. Parses and validates the response against the Pydantic model, printing either
   the validated tasks or the validation errors.

## Domain model

```python
Status = Literal["todo", "in_progress", "done"]

class EmailTask(BaseModel):    type: Literal["email"];     id: str; title: str; status: Status; recipient: str
class ReviewTask(BaseModel):   type: Literal["review"];    id: str; title: str; status: Status; reviewer: str
class BatchJobTask(BaseModel): type: Literal["batch_job"]; id: str; title: str; status: Status; recordCount: int
```

See [tasks_py/schema.py](tasks_py/schema.py) for the full definitions. The three
variants form a discriminated union keyed on `type`.

## Requirements

- Python 3.10+
- A running [Ollama](https://ollama.com) server with a model that supports
  structured output (e.g. `gemma4:latest`)

Only third-party dependency is `pydantic` (the Ollama call uses the standard
library `urllib`).

## Install & run

```bash
cd tools/sample_projects/tasks_py
python -m venv .venv
source .venv/bin/activate
pip install -r requirements.txt

python -m tasks_py.main
```

Alternatively, install as a package to get the `tasks-py` console script:

```bash
pip install -e .
tasks-py
```

### Passing a different prompt file

```bash
python -m tasks_py.main /path/to/your_prompt.md
```

## Configuration

| Env var        | Default                    | Description                     |
| -------------- | -------------------------- | ------------------------------- |
| `OLLAMA_HOST`  | `http://localhost:11434`   | Base URL of the Ollama server   |
| `OLLAMA_MODEL` | `gemma4:latest`            | Model name to use               |

Example:

```bash
OLLAMA_MODEL=gemma4:e2b python -m tasks_py.main
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
