"""Generate `cpipes_model.py` — the Pydantic v2 model — from the reviewed matrix.

Post-B.10 this is the BOOTSTRAP direction only: the model is the source of
truth and `reflect` regenerates the matrix from it. Running `generate` again
overwrites edits made in the model — it exists to re-bootstrap, not to sync.

The design is the plan's §5.2.2: a base class per discriminated struct carrying
the fields applicable to every token, one subclass per `type` token carrying
only its own, and `Field(discriminator=...)` on the union — applicability stops
being a table lookup and becomes the class structure. The wire format does not
change: the model is hierarchical, the emitted JSON keeps the flat shape, and
all live corpus configs must continue to validate (the guard on the exercise).

Shapes the emitter treats specially, each grounded in the matrix itself:

- Unions whose discriminator has an engine default (`memory` on the channel
  configs, `standard` on the splitter, `anonymization` on anonymize) get a
  BeforeValidator that injects the default tag when the document omits it, so
  the discriminated union still validates the corpus's untyped instances.
- `ExpressionNode` (virtual `~unary`/`~binary` tokens, optional `type`) and
  `ContextSpec` (optional `type`, token-identical fields) are emitted as single
  merged classes — their variants are structural, not tag-carried.
- `TransformationSpec`'s `~override` token exists only at
  `conditional_config.N.then` and becomes its own class, referenced there.

The CSV remains the review artifact; this file is its projection. B.10 flips
the source of truth to the Python model and regenerates the CSV from it.
"""

from __future__ import annotations

import argparse
from collections import defaultdict
from pathlib import Path

from .matrix_schema import (
    ANY_TOKEN,
    NONE,
    FieldRow,
    Matrix,
    Required,
    TypeRow,
    YesNo,
)

# Structs emitted as one merged class instead of a discriminated union: the
# discriminator is optional in the wire and the variants are structural.
MERGED = {"ExpressionNode", "ContextSpec"}

# Virtual tokens (the `~` prefix) carry no literal tag and never join a union.
VIRTUAL_PREFIX = "~"

# The `~override` token is position-bound: it is the shape of
# `conditional_config.N.then` and nothing else (actions_start_common.go's
# MergeTransformationSpec). The edge below re-points that one reference.
REF_OVERRIDES = {
    ("ConditionalTransformationSpec", "then"): "TransformationSpecOverride",
}

_GO_BASE = {
    "string": "str",
    "bool": "bool",
    "int": "int",
    "int32": "int",
    "int64": "int",
    "uint64": "int",
    "byte": "int",
    "rune": "int",
    "float64": "float",
    "any": "Any",
    "json.RawMessage": "Any",
    "map[string]any": "dict[str, Any]",
    "map[string]string": "dict[str, str]",
}

HEADER = '''"""The cpipes contract - THE SOURCE OF TRUTH for the contract claims (B.10).

Since the B.10 flip, this Pydantic v2 model is where the contract is edited:
which fields exist per operator token, which are required, their value ranges,
and their descriptions. The matrix CSVs remain the review artifact and the
audit/Go-binding record, regenerated FROM this file:

    python -m cpipes_contract reflect          # sync the matrix claim columns
    python -m cpipes_contract reflect --check  # the divergence guard (CI)

The `generate` command is the bootstrap projection that first produced this
file from the reviewed matrix (B.9) - running it again OVERWRITES edits made
here. Engine-applied defaults are noted in the descriptions and deliberately
NOT materialised as Pydantic defaults, so a dumped document stays minimal and
the wire format unchanged.
"""

from __future__ import annotations

from typing import Annotated, Any, Literal, Union

from pydantic import BaseModel, BeforeValidator, ConfigDict, Field


def _tag_default(key: str, default: str):
    """Inject the engine's default discriminator value when the document
    omits it, so the discriminated union accepts untyped instances the way
    the engine does."""

    def inject(value):
        if isinstance(value, dict) and key not in value:
            return {**value, key: default}
        return value

    return inject


class _Base(BaseModel):
    model_config = ConfigDict(extra="forbid")

'''


def camel(token: str) -> str:
    parts = token.lstrip(VIRTUAL_PREFIX).replace("-", "_").split("_")
    return "".join(p[:1].upper() + p[1:] for p in parts if p)


class Emitter:
    def __init__(self, matrix: Matrix) -> None:
        self.types: dict[str, list[TypeRow]] = defaultdict(list)
        for t in matrix.types:
            self.types[t.go_struct].append(t)
        self.fields: dict[tuple[str, str], list[FieldRow]] = defaultdict(list)
        for f in matrix.fields_:
            self.fields[(f.go_struct, f.type_token)].append(f)

    # -- naming -------------------------------------------------------------

    def class_name(self, struct: str, token: str) -> str:
        if token == ANY_TOKEN or struct in MERGED:
            return struct
        if token.startswith(VIRTUAL_PREFIX):
            return struct + camel(token)
        return struct + camel(token)

    def is_union(self, struct: str) -> bool:
        return len(self.types[struct]) > 1 and struct not in MERGED

    def ref_name(self, struct: str) -> str:
        # A union's alias carries the bare struct name; single-token and
        # merged classes carry it directly. Either way a reference is the name.
        return struct

    # -- field typing -------------------------------------------------------

    def py_type(self, row: FieldRow, literal_token: str | None = None) -> str:
        if literal_token is not None:
            return f'Literal["{literal_token}"]'
        container = str(row.container)
        if container == "any":
            base = "int | str"
        elif container == "raw_json":
            base = "Any"
        elif row.ref_struct != NONE:
            target = REF_OVERRIDES.get((row.go_struct, row.json_key))
            base = target or self.ref_name(row.ref_struct)
        else:
            go = row.go_type.lstrip("*")
            while go.startswith("[]"):
                go = go[2:]
            if container == "map" and go.startswith("map[string]"):
                # the container column already says map; strip the go spelling
                # so the value type is not wrapped twice
                go = go.removeprefix("map[string]")
            base = _GO_BASE.get(go)
            if base is None:
                raise LookupError(
                    f"no Python type for go_type {row.go_type!r} on "
                    f"{row.go_struct}/{row.type_token}.{row.json_key}"
                )
            if base == "str" and row.values not in (NONE, None):
                opts = ", ".join(f'"{v}"' for v in row.values.split("|"))
                base = f"Literal[{opts}]"
        if container == "array":
            base = f"list[{base}]"
        elif container == "array2":
            base = f"list[list[{base}]]"
        elif container == "map":
            base = f"dict[str, {base}]"
        return base

    def field_line(self, row: FieldRow, literal_token: str | None = None) -> str:
        name = row.json_key
        typ = self.py_type(row, literal_token)
        desc = (row.description or "").strip()
        if row.deprecated is YesNo.YES:
            desc = "DEPRECATED. " + desc
        if row.required is Required.CONDITIONAL and row.required_when not in (NONE, None):
            desc += f" Required when: {row.required_when}."
        if row.default not in (NONE, None):
            desc += f" Engine default: {row.default} ({row.default_by})."
        desc = desc.replace("\\", "\\\\").replace('"', '\\"')
        if literal_token is not None:
            default = f' = "{literal_token}"' if row.required is not Required.YES else ""
            if desc:
                return f'    {name}: {typ} = Field({literal_token_default(row, literal_token)}description="{desc}")'
            return f"    {name}: {typ}{default}"
        required = row.required is Required.YES
        if desc:
            if required:
                return f'    {name}: {typ} = Field(description="{desc}")'
            return f'    {name}: {typ} | None = Field(default=None, description="{desc}")'
        if required:
            return f"    {name}: {typ}"
        return f"    {name}: {typ} | None = None"

    # -- class emission -----------------------------------------------------

    def signature(self, row: FieldRow) -> tuple:
        return (
            row.json_key,
            self.py_type(row),
            row.required is Required.YES,
            (row.description or "").strip(),
            str(row.default),
        )

    def emit_struct(self, struct: str, out: list[str]) -> None:
        tokens = self.types[struct]
        if not self.is_union(struct):
            self.emit_merged(struct, tokens, out)
            return
        disc = tokens[0].discriminator
        real = [t for t in tokens if not t.type_token.startswith(VIRTUAL_PREFIX)]
        virtual = [t for t in tokens if t.type_token.startswith(VIRTUAL_PREFIX)]

        # Fields applicable in every real token with an identical signature
        # move to the base class; the rest stay on the subclasses.
        per_token: dict[str, dict[str, FieldRow]] = {}
        for t in real:
            per_token[t.type_token] = {
                f.json_key: f
                for f in self.fields[(struct, t.type_token)]
                if f.applicable is YesNo.YES and f.json_key != disc
            }
        common: list[str] = []
        first = per_token[real[0].type_token]
        for key, row in first.items():
            sig = self.signature(row)
            if all(
                key in per_token[t.type_token]
                and self.signature(per_token[t.type_token][key]) == sig
                for t in real[1:]
            ):
                common.append(key)

        base_name = struct + "Base"
        out.append(f"class {base_name}(_Base):")
        doc = self.type_doc(tokens[0], base=True)
        if doc:
            out.append(f'    """{doc}"""')
        base_rows = [first[k] for k in common]
        if not base_rows and not doc:
            out.append("    pass")
        for row in sorted(base_rows, key=lambda r: r.json_key):
            out.append(self.field_line(row))
        out.append("")
        out.append("")

        for t in real:
            cname = self.class_name(struct, t.type_token)
            out.append(f"class {cname}({base_name}):")
            doc = self.type_doc(t)
            if doc:
                out.append(f'    """{doc}"""')
            disc_row = next(
                f for f in self.fields[(struct, t.type_token)] if f.json_key == disc
            )
            out.append(self.field_line(disc_row, literal_token=t.type_token))
            own = [
                per_token[t.type_token][k]
                for k in per_token[t.type_token]
                if k not in common
            ]
            for row in sorted(own, key=lambda r: r.json_key):
                out.append(self.field_line(row))
            out.append("")
            out.append("")

        for t in virtual:
            # A virtual token does not inherit the union base: its applicable
            # surface is its own (the matrix rules `when`/`conditional_config`
            # off the ~override shape, which the base would smuggle back in).
            cname = self.class_name(struct, t.type_token)
            out.append(f"class {cname}(_Base):")
            doc = self.type_doc(t)
            if doc:
                out.append(f'    """{doc}"""')
            rows = [
                f
                for f in self.fields[(struct, t.type_token)]
                if f.applicable is YesNo.YES and f.json_key != disc
            ]
            if not rows and not doc:
                out.append("    pass")
            for row in sorted(rows, key=lambda r: r.json_key):
                out.append(self.field_line(row))
            out.append("")
            out.append("")

    def emit_merged(self, struct: str, tokens: list[TypeRow], out: list[str]) -> None:
        # One class holding the union of the tokens' applicable fields. Where
        # tokens disagree on requiredness, the weaker claim (optional) wins:
        # the shape rules are structural and documented, not tag-enforced.
        merged: dict[str, FieldRow] = {}
        optional: set[str] = set()
        disc = tokens[0].discriminator if tokens[0].discriminator != NONE else None
        literal_values: list[str] = []
        for t in tokens:
            if disc and not t.type_token.startswith(VIRTUAL_PREFIX) and t.type_token != ANY_TOKEN:
                literal_values.append(t.type_token)
            for f in self.fields[(struct, t.type_token)]:
                if f.applicable is not YesNo.YES:
                    continue
                if f.json_key in merged:
                    if f.required is not Required.YES:
                        optional.add(f.json_key)
                else:
                    merged[f.json_key] = f
                    if f.required is not Required.YES:
                        optional.add(f.json_key)
        if len(tokens) > 1:
            # A field absent from some token is optional in the merged class.
            for key, row in merged.items():
                if any(
                    key
                    not in {
                        f.json_key
                        for f in self.fields[(struct, t.type_token)]
                        if f.applicable is YesNo.YES
                    }
                    for t in tokens
                ):
                    optional.add(key)
        out.append(f"class {struct}(_Base):")
        doc = self.type_doc(tokens[0], base=len(tokens) > 1)
        if doc:
            out.append(f'    """{doc}"""')
        if not merged and not doc:
            out.append("    pass")
        for key in sorted(merged):
            row = merged[key]
            if disc and key == disc and len(tokens) > 1:
                opts = ", ".join(f'"{v}"' for v in literal_values)
                out.append(
                    f'    {key}: Literal[{opts}] | None = Field(default=None, description="The node shape; unary and binary operator nodes carry no type.")'
                )
                continue
            if key in optional and row.required is Required.YES:
                # re-emit as optional without mutating the row
                typ = self.py_type(row)
                desc = (row.description or "").strip().replace("\\", "\\\\").replace('"', '\\"')
                if desc:
                    out.append(
                        f'    {key}: {typ} | None = Field(default=None, description="{desc}")'
                    )
                else:
                    out.append(f"    {key}: {typ} | None = None")
                continue
            out.append(self.field_line(row))
        out.append("")
        out.append("")

    def type_doc(self, t: TypeRow, base: bool = False) -> str:
        desc = (t.description or "").strip()
        if base and desc:
            desc = f"{t.go_struct}: {desc}"
        return desc.replace("\\", "\\\\").replace('"', '\\"')

    # -- unions -------------------------------------------------------------

    def emit_union_alias(self, struct: str, out: list[str]) -> None:
        tokens = self.types[struct]
        disc = tokens[0].discriminator
        real = [t for t in tokens if not t.type_token.startswith(VIRTUAL_PREFIX)]
        members = ", ".join(self.class_name(struct, t.type_token) for t in real)
        default_tag = None
        for t in real:
            row = next(
                (f for f in self.fields[(struct, t.type_token)] if f.json_key == disc),
                None,
            )
            if row is not None and row.required is not Required.YES and str(row.default) == t.type_token:
                default_tag = t.type_token
        parts = [f"Union[{members}]", f'Field(discriminator="{disc}")']
        if default_tag is not None:
            parts.append(f'BeforeValidator(_tag_default("{disc}", "{default_tag}"))')
        out.append(f"{struct} = Annotated[{', '.join(parts)}]")

    # -- driver -------------------------------------------------------------

    def emit(self) -> str:
        out: list[str] = [HEADER]
        for struct in sorted(self.types):
            self.emit_struct(struct, out)
        out.append("")
        for struct in sorted(self.types):
            if self.is_union(struct):
                self.emit_union_alias(struct, out)
        out.append("")
        out.append("")
        keys: list[str] = []
        for struct in sorted(self.types):
            for t in self.types[struct]:
                if struct in MERGED:
                    continue
                if self.is_union(struct):
                    cname = self.class_name(struct, t.type_token)
                else:
                    cname = struct
                keys.append(f'    "{cname}": ("{struct}", "{t.type_token}"),')
        out.append("# class -> (go_struct, type_token); the reflect direction's key.")
        out.append("_MATRIX_KEYS = {")
        out.extend(sorted(set(keys)))
        out.append("}")
        out.append("")
        out.append("_MODELS = [v for v in list(globals().values()) if isinstance(v, type) and issubclass(v, BaseModel) and v is not BaseModel]")
        out.append("for _m in _MODELS:")
        out.append("    _m.model_rebuild()")
        out.append("")
        return "\n".join(out)


def literal_token_default(row: FieldRow, token: str) -> str:
    if row.required is not Required.YES:
        return f'default="{token}", '
    return ""


def run_with_matrix(args: argparse.Namespace, matrix: Matrix) -> int:
    text = Emitter(matrix).emit()
    args.out.write_text(text)
    print(f"wrote {args.out}")
    return 0
