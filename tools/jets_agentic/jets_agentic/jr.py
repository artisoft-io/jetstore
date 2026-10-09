"""The .jr emitter (A1.5) — deliberately the least clever component (R-3).

Two constructs, per the corpus idiom (§4): `class` declarations for the
entities, and named `text` literals for the vocabularies — a named literal
lands in `resources` with `type='text'`, `id` set and `inline` null, so rules
reference the constant by name and a mistyped value is a compile error.
Rule-visible metadata (`data_classification`) is emitted as `triple(...)`
facts (§3.5); everything emitter-only goes to the sidecar instead.
"""

from __future__ import annotations

import re
import types
import typing
from datetime import date, datetime
from enum import StrEnum

from pydantic import BaseModel

from . import header
from . import model as M

_SCALARS = {str: "text", int: "int", float: "double", bool: "bool",
            datetime: "datetime", date: "date"}


def unwrap_optional(annotation):
    if typing.get_origin(annotation) in (typing.Union, types.UnionType):
        args = [a for a in typing.get_args(annotation) if a is not type(None)]
        if len(args) == 1:
            return args[0]
    return annotation


def analyze(annotation) -> tuple[str, bool, type | None]:
    """Returns (jr_type, is_array, vocabulary-or-entity class)."""
    annotation = unwrap_optional(annotation)
    if typing.get_origin(annotation) is list:
        inner, _, cls = analyze(typing.get_args(annotation)[0])
        return inner, True, cls
    if isinstance(annotation, type) and issubclass(annotation, StrEnum):
        return "text", False, annotation
    if isinstance(annotation, type) and issubclass(annotation, BaseModel):
        return "resource", False, annotation
    jr = _SCALARS.get(annotation)
    if jr is None:
        raise TypeError(f"no .jr type for annotation {annotation!r}")
    return jr, False, None


def snake_upper(name: str) -> str:
    return re.sub(r"(?<!^)(?=[A-Z])", "_", name).upper()


def qname(name: str) -> str:
    return f"{M.PREFIX}:{name}"


def collect_vocabularies() -> list[type[StrEnum]]:
    """Every StrEnum reachable from an entity, in first-reached order — the
    rule Q-6 settled, since EvidenceSource is not in the proposal's §A.4."""
    seen: dict[type[StrEnum], list[str]] = {}
    for entity in M.ENTITIES:
        for fname, field in entity.model_fields.items():
            _, _, cls = analyze(field.annotation)
            if cls is not None and issubclass(cls, StrEnum):
                seen.setdefault(cls, []).append(qname(fname))
    return [(cls, props) for cls, props in seen.items()]  # type: ignore[return-value]


def emit() -> str:
    out: list[str] = []
    out.extend(
        header.comment_block(
            "jets_agentic.jr",
            "`jets-agentic generate`, from "
            f"tools/jets_agentic/jets_agentic/model.py (model version {M.MODEL_VERSION}).",
        )
    )
    out.append("# The schema-first authoring surface of decision 7: new agentic entities are")
    out.append("# authored in the Python source; existing data models and all rules stay in .jr.")
    out.append("")
    # Every entity derives from jets:Entity, which the platform base model
    # declares — and both files are installed into data_model/ by the same step
    # (A21.2), so this file can import it and be self-contained. Without it, a
    # rule set importing only this file fails with "base class jets:Entity not
    # found", which is where criterion 13's first run ended.
    out.append('import "data_model/jets_model.jr"')
    out.append("")

    triples: list[str] = []
    for entity in M.ENTITIES:
        data_props: list[str] = []
        object_props: list[str] = []
        for fname, field in entity.model_fields.items():
            jr_type, is_array, cls = analyze(field.annotation)
            name = qname(fname)
            if cls is not None and issubclass(cls, BaseModel):
                decl = f"{name} as array of resource" if is_array else f"{name} as resource"
                object_props.append(decl)
            else:
                decl = f"{name} as array of {jr_type}" if is_array else f"{name} as {jr_type}"
                data_props.append(decl)
            extra = field.json_schema_extra or {}
            classification = extra.get("data_classification") if isinstance(extra, dict) else None
            if classification:
                triples.append(
                    f'triple({name}, {M.PREFIX}:data_classification, "{classification}");'
                )

        doc = (entity.__doc__ or "").strip().split("\n")[0]
        out.append(f"# {doc}")
        out.append(f"class {qname(entity.__name__)} {{")
        out.append(f"  $base_classes = [{entity.jr_base}],")
        out.append("  $data_properties = [")
        out.append(",\n".join(f"    {d}" for d in data_props))
        if object_props:
            out.append("  ],")
            out.append("  $object_properties = [")
            out.append(",\n".join(f"    {d}" for d in object_props))
        out.append("  ],")
        out.append(f"  $as_table = {'true' if entity.jr_as_table else 'false'}")
        out.append("};")
        out.append("")

    for cls, props in collect_vocabularies():
        out.append(f"# {' / '.join(sorted(set(props)))} enum ({cls.__name__})")
        for member in cls:
            out.append(f'text {qname(snake_upper(cls.__name__) + "_" + member.name.upper())} = "{member.value}";')
        out.append("")

    if triples:
        out.append("# Rule-visible property metadata (§3.5): data_classification markers as")
        out.append("# triples, readable in working memory and at build time (workspace.db triples).")
        out.append(f'resource {M.PREFIX}:data_classification = "{M.PREFIX}:data_classification";')
        out.extend(triples)
        out.append("")

    return "\n".join(out)
