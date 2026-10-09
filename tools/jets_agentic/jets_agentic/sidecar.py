"""The sidecar emitter (A1.6) — `jets_agentic.meta.json`.

Emitter-only metadata that never reaches the RETE engine (§3.5): descriptions,
defaults, requiredness, the key markers, and the vocabulary values. Item 2a's
JSON Schema emitter and item 3's glossary/signature emitters read this file
(and, for client-authored classes, the workspace's triples table — one
emitter, two sources).
"""

from __future__ import annotations

import json
from enum import StrEnum

from pydantic import BaseModel

from . import header
from . import model as M
from .jr import analyze, collect_vocabularies, qname


def emit() -> str:
    entities: dict[str, dict] = {}
    for entity in M.ENTITIES:
        props: dict[str, dict] = {}
        for fname, field in entity.model_fields.items():
            jr_type, is_array, cls = analyze(field.annotation)
            extra = field.json_schema_extra if isinstance(field.json_schema_extra, dict) else {}
            entry: dict = {
                "type": jr_type,
                "array": is_array,
                "required": field.is_required(),
                "description": field.description or "",
            }
            if cls is not None and issubclass(cls, StrEnum):
                entry["vocabulary"] = cls.__name__
            if cls is not None and issubclass(cls, BaseModel):
                entry["entity"] = qname(cls.__name__)
                entry["type"] = "resource"
            if not field.is_required() and field.default is not None:
                entry["default"] = field.default
            if extra.get("data_classification"):
                entry["data_classification"] = extra["data_classification"]
            if extra.get("key"):
                entry["key"] = True
            props[qname(fname)] = entry
        entities[qname(entity.__name__)] = {
            "description": " ".join((entity.__doc__ or "").split()),
            "base": entity.jr_base,
            "as_table": entity.jr_as_table,
            "properties": props,
        }

    vocabularies = {
        cls.__name__: {
            "properties": sorted(set(props)),
            "values": [m.value for m in cls],
            "description": " ".join((cls.__doc__ or "").split()),
        }
        for cls, props in collect_vocabularies()
    }

    return json.dumps(
        {
            "jetstore-owned-asset": header.json_object(
                "jets_agentic.meta.json",
                "`jets-agentic generate`, from "
                "tools/jets_agentic/jets_agentic/model.py.",
            ),
            "model": "jets_agentic",
            "version": M.MODEL_VERSION,
            "prefix": M.PREFIX,
            "entities": entities,
            "vocabularies": vocabularies,
        },
        indent=2,
        sort_keys=False,
    ) + "\n"
