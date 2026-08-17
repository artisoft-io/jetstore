"""The constrained-decoding check (A2a.3, A2a.4) — one command, exit code.

Three gates, in rising order of realism:

- **Shape (A2a.4):** every vocabulary survives into the emitted bundle as a
  `$defs` entry with an `enum` of exactly the `StrEnum`'s values, every
  vocabulary-typed entity property reaches it by `$ref` (the §A.2.8
  hand-written shape), and no projection can widen an enum — each
  vocabulary's values in a projected schema equal the source's exactly.
- **vLLM `guided_json` (A2a.3):** the projected schema compiles under
  `xgrammar`, the structured-output backend vLLM uses — the schema is turned
  into an actual decoding grammar, not merely accepted as JSON. Needs
  `pip install xgrammar` in the tool venv; it is a check-only dependency and
  deliberately not in `pyproject.toml`.
- **Ollama `format` (A2a.3):** with `--ollama-model`, a live request passes
  `schema_for(...)` as the `format` field with no translation step, and the
  response must validate against the projection model — including its enum
  constraints, which is where guided decoding earns its keep (§5.1).

The reference projection is §5.1's own example: a triage agent setting four
of `Incident`'s fields.
"""

from __future__ import annotations

import argparse
import json
import os
import urllib.error
import urllib.request

from . import model as M
from . import schema
from .jr import analyze, collect_vocabularies

TRIAGE_FIELDS = ["incident_id", "classification", "severity", "status"]


def _enums_in(node, found: dict[str, list]) -> None:
    if isinstance(node, dict):
        for key, sub in node.items():
            if key == "$defs" and isinstance(sub, dict):
                for name, defn in sub.items():
                    if isinstance(defn, dict) and "enum" in defn:
                        found[name] = defn["enum"]
            _enums_in(sub, found)
    elif isinstance(node, list):
        for sub in node:
            _enums_in(sub, found)


def check_shape(problems: list[str]) -> None:
    bundle = json.loads(schema.emit())
    defs = bundle["$defs"]
    vocab_values = {cls.__name__: [m.value for m in cls] for cls, _ in collect_vocabularies()}

    # Every vocabulary is a $defs enum of exactly the StrEnum's values.
    for name, values in vocab_values.items():
        entry = defs.get(name)
        if not isinstance(entry, dict) or entry.get("enum") != values:
            problems.append(f"$defs.{name} is not an enum of exactly {name}'s values")

    # Every vocabulary-typed property reaches its enum by $ref.
    for entity in M.ENTITIES:
        for fname, field in entity.model_fields.items():
            _, _, cls = analyze(field.annotation)
            if cls is None or cls.__name__ not in vocab_values:
                continue
            prop_schema = defs[entity.__name__]["properties"][fname]
            if isinstance(prop_schema.get("items"), dict):
                prop_schema = prop_schema["items"]
            if prop_schema.get("$ref") != f"#/$defs/{cls.__name__}":
                problems.append(
                    f"{entity.__name__}.{fname} does not $ref "
                    f"#/$defs/{cls.__name__}: {prop_schema}"
                )

    # No projection can widen (or narrow) an enum: project every entity over
    # all its fields and every single field, and compare each enum that
    # appears against the source vocabulary.
    for entity in M.ENTITIES:
        field_sets = [list(entity.model_fields)] + [[f] for f in entity.model_fields]
        for fields in field_sets:
            found: dict[str, list] = {}
            _enums_in(schema.schema_for(entity, fields), found)
            for name, values in found.items():
                if values != vocab_values.get(name):
                    problems.append(
                        f"projection {entity.__name__}{fields} carries {name} "
                        f"as {values}, source says {vocab_values.get(name)}"
                    )


def check_xgrammar(problems: list[str]) -> None:
    try:
        import xgrammar
    except ImportError:
        problems.append(
            "xgrammar is not installed in this venv - "
            "`pip install xgrammar` (check-only dependency)"
        )
        return
    targets = [("Incident/triage", schema.schema_for(schema.entity_by_name("Incident"), TRIAGE_FIELDS))]
    targets += [(e.__name__, schema.schema_for(e, list(e.model_fields))) for e in M.ENTITIES]
    for name, projected in targets:
        try:
            xgrammar.Grammar.from_json_schema(json.dumps(projected))
        except Exception as exc:  # noqa: BLE001 - the report is the point
            problems.append(f"xgrammar rejects the {name} schema: {exc}")


def check_ollama(model_name: str, problems: list[str]) -> None:
    host = os.environ.get("OLLAMA_HOST", "http://localhost:11434")
    entity = schema.entity_by_name("Incident")
    projected = schema.schema_for(entity, TRIAGE_FIELDS)
    body = json.dumps({
        "model": model_name,
        "stream": False,
        "format": projected,  # the schema, as-is - the no-translation claim
        "messages": [{
            "role": "user",
            "content": (
                "A nightly claims file arrived 400MB smaller than every prior "
                "delivery and the loader parsed zero records. Triage this as "
                "an incident: give it an id, classify it, set severity and "
                "status."
            ),
        }],
        "options": {"temperature": 0},
    }).encode()
    req = urllib.request.Request(
        f"{host}/api/chat", data=body, headers={"Content-Type": "application/json"}
    )
    try:
        with urllib.request.urlopen(req, timeout=600) as resp:
            content = json.loads(resp.read())["message"]["content"]
    except (urllib.error.URLError, OSError, KeyError) as exc:
        problems.append(f"ollama request failed ({host}, model {model_name}): {exc}")
        return
    try:
        decoded = schema.projection_model(entity, TRIAGE_FIELDS).model_validate_json(content)
    except Exception as exc:  # noqa: BLE001
        problems.append(f"ollama output does not validate against the projection: {exc}\n  output: {content!r}")
        return
    print(f"  ollama decoded, validates against the projection: {decoded.model_dump()}")


def run(args: argparse.Namespace) -> int:
    problems: list[str] = []
    check_shape(problems)
    shape_problems = len(problems)
    print(f"shape (A2a.4): {'FAILED' if shape_problems else 'ok'} - "
          f"{len(collect_vocabularies())} vocabularies as $defs enums, projections cannot widen")
    check_xgrammar(problems)
    print(f"vLLM guided_json via xgrammar (A2a.3): "
          f"{'FAILED' if len(problems) > shape_problems else 'ok - all 10 entities + the triage projection compile to grammars'}")
    if args.ollama_model:
        before = len(problems)
        check_ollama(args.ollama_model, problems)
        print(f"ollama format (A2a.3): {'FAILED' if len(problems) > before else 'ok - live decode, schema passed as-is'}")
    else:
        print("ollama format (A2a.3): not run - pass --ollama-model with a reachable server")

    if problems:
        print(f"decode check: {len(problems)} problem(s):")
        for p in problems:
            print(" ", p)
        return 1
    print("decode check: clean")
    return 0
