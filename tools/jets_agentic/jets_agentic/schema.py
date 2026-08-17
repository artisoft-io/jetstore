"""The domain-model JSON Schema emitter (item 2a) — a projection, not a dump.

Two products from the one Pydantic source (§5.1):

- `emit()` — `jets_agentic.schema.json`, the committed bundle: every entity
  and every vocabulary as a `$defs` entry, the `$ref` targets item 3's
  glossary and tool signatures point into.
- `schema_for(entity, allowed_fields)` — the per-agent projection. What an
  agent emits is never a whole entity, so a field the agent may not set is
  *absent* from the schema it is decoded against: under guided decoding the
  field is unreachable rather than merely rejected (§5.1, A§5.6). The
  projection selects fields; it never touches what values a selected field
  may take — restricting *which* fields is the point, loosening a
  vocabulary's values is not (A2a.4).

The post-processing pass is §3.8's known, deliberate cost (A2a.2): Pydantic's
`model_json_schema()` output is cleaned of `title` noise, `anyOf`-with-null
optionals are collapsed to the bare type (optionality already lives in
`required`), null defaults are dropped, and the sidecar-only extras that
`prop()` plants (`key`, `data_classification`) are stripped — the schema's
one job is constraining a decode; property metadata belongs to the sidecar.

The vocabularies are where the schema earns the most (§5.1): each `StrEnum`
survives as a `$defs` entry with an `enum` keyword that entity properties
`$ref` — the shape §A.2.8 writes by hand — and under guided decoding an
out-of-taxonomy value is unreachable, which is the single constraint the
compiler cannot enforce later.
"""

from __future__ import annotations

import json
from typing import Any

from pydantic import BaseModel, ConfigDict, create_model
from pydantic.json_schema import models_json_schema

from . import header
from . import model as M

DIALECT = "https://json-schema.org/draft/2020-12/schema"
SCHEMA_ID = "jets_agentic.schema.json"

# prop()'s json_schema_extra keys: sidecar metadata, not decode constraints.
_SIDECAR_EXTRAS = ("key", "data_classification")


def _clean(node: Any) -> Any:
    """The A2a.2 post-processing pass, applied bottom-up."""
    if isinstance(node, list):
        return [_clean(item) for item in node]
    if not isinstance(node, dict):
        return node
    node = {k: _clean(v) for k, v in node.items()}
    node.pop("title", None)
    for extra in _SIDECAR_EXTRAS:
        node.pop(extra, None)
    if node.get("default", ...) is None:
        del node["default"]
    any_of = node.get("anyOf")
    if (
        isinstance(any_of, list)
        and len(any_of) == 2
        and {"type": "null"} in any_of
    ):
        inner = next(m for m in any_of if m != {"type": "null"})
        del node["anyOf"]
        # Siblings on the anyOf (description, default) stay put and win.
        node = {**inner, **node}
    return node


def emit() -> str:
    """The committed bundle: all ten entities and every reachable vocabulary
    under one `$defs`, each entity independently reachable as
    `jets_agentic.schema.json#/$defs/<Entity>`."""
    _, definitions = models_json_schema(
        [(entity, "validation") for entity in M.ENTITIES],
        ref_template="#/$defs/{model}",
    )
    bundle = {
        "$schema": DIALECT,
        "$id": SCHEMA_ID,
        "$comment": header.json_comment(
            "jets_agentic.schema.json",
            "`jets-agentic generate`, from "
            "tools/jets_agentic/jets_agentic/model.py.",
        ),
        "description": (
            f"The jets_agentic domain model, version {M.MODEL_VERSION}. "
            "Decode against `jets-agentic schema` projections, not whole entities."
        ),
        "$defs": _clean(definitions["$defs"]),
    }
    return json.dumps(bundle, indent=2, sort_keys=False) + "\n"


def projection_model(entity: type[BaseModel], allowed_fields: list[str]) -> type[BaseModel]:
    """The Pydantic model of a projection — also the validator for what comes
    back from a decode constrained by `schema_for` (A2a.3's check uses it)."""
    unknown = [f for f in allowed_fields if f not in entity.model_fields]
    if unknown:
        raise KeyError(
            f"{entity.__name__} has no field(s) {unknown}; "
            f"it has {sorted(entity.model_fields)}"
        )
    if not allowed_fields:
        raise ValueError("an empty projection decodes to nothing an agent may set")
    fields: dict[str, Any] = {
        name: (entity.model_fields[name].annotation, entity.model_fields[name])
        for name in allowed_fields
    }
    projected = create_model(
        entity.__name__,
        __config__=ConfigDict(extra="forbid"),
        __doc__=entity.__doc__,
        **fields,
    )
    return projected


def schema_for(entity: type[BaseModel], allowed_fields: list[str]) -> dict:
    """§5.1's interface. The returned dict is self-contained (its `$defs`
    carry the reachable vocabularies and value objects) and is passed as-is
    to Ollama `format` or vLLM `guided_json`."""
    return _clean(projection_model(entity, allowed_fields).model_json_schema())


def entity_by_name(name: str) -> type[BaseModel]:
    for entity in M.ENTITIES:
        if entity.__name__ == name:
            return entity
    raise KeyError(f"no entity named {name}; the model has "
                   + ", ".join(e.__name__ for e in M.ENTITIES))
