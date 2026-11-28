"""Generate `cpipes_model.py` — the Pydantic v2 model — from the reviewed matrix.

Since the source-of-truth flip the model is the source of truth and `reflect`
regenerates the matrix from it. `generate` is the other direction, and since
2026-11-27 the two are held to each other: `generate --check` fails when the
committed model is not what the matrix emits, so a model edit the matrix cannot
carry is caught rather than lost at the next bootstrap. Code the matrix has no
column for lives in the model's hand-written regions, which `generate` carries
over (see "hand-written regions" below).

**It had drifted for three months before that check existed.** Nothing ran
`generate` after the flip, and by 2026-11-27 its output differed from the
committed model in four ways: classes and fields added by hand out of the
emitter's sorted order, the merged `ContextSpec` described by whichever token
sorted first in types.csv, two hand-written docstrings on `Element` that differ
from types.csv, and three blocks of code (a closed list and its validator, a
comment on a union, and the runtime projection) that the emitter deleted. The
emitter's own order was never the cause: everything it emits is sorted, and its
output is identical across hash seeds.

The design: a base class per discriminated struct carrying
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

The CSV remains the review artifact; this file is its projection. The flip moved
the source of truth to the Python model and regenerates the CSV from it.
"""

from __future__ import annotations

import argparse
import difflib
import re
from collections import Counter, defaultdict
from pathlib import Path

from .matrix_schema import (
    ANY_TOKEN,
    NONE,
    FieldRow,
    Matrix,
    Required,
    TypeRow,
    YesNo,
    parse_variant_when,
)

# Structs emitted as one merged class instead of a discriminated union: the
# discriminator is optional in the wire and the variants are structural.
MERGED = {"ExpressionNode", "ContextSpec"}

# Docstrings for the merged classes: the per-token descriptions describe one
# shape each, and the merged class needs the whole story (this carries the
# recursion depth guidance here, since a re-bootstrap must not lose it).
MERGED_DOC = {
    "ExpressionNode": (
        "An expression tree node - one class, several structural shapes: a unary operator node (op + arg), a binary operator node (lhs + op + rhs), or a typed leaf (type: select, value, expr_proxy, function, static_list); unary and binary operator nodes carry no type. Self-referential through arg, lhs, rhs, function_arguments, and default. JSON Schema cannot bound the recursion, so the depth budget lives with the caller: the deepest live expression in the corpus is 8 levels, and prompts should bound generation at 12."
    ),
    # Written into MERGED_DOC on 2026-11-27 as the model states it. Until then
    # the merged class took its first token's description, and when
    # `default_value` was added ahead of `file_key_component` in types.csv the
    # bootstrap's docstring changed under the model without anyone running it.
    "ContextSpec": (
        "ContextSpec: Define env var sourced from the pipeline main file key directory name with pattern expr=value"
    ),
}

# The discriminator of a merged class whose description is not its rows':
# ExpressionNode's `type` is optional because an operator node carries none,
# which no single row says. Every other merged class takes its rows' own.
MERGED_DISC_DOC = {
    "ExpressionNode": "The node shape; unary and binary operator nodes carry no type.",
}

# Virtual tokens (the `~` prefix) carry no literal tag and never join a union.
VIRTUAL_PREFIX = "~"

# The `~override` token is position-bound: it is the shape of
# `conditional_config.N.then` and nothing else (actions_start_common.go's
# MergeTransformationSpec). The edge below re-points that one reference.
REF_OVERRIDES = {
    ("ConditionalTransformationSpec", "then"): "TransformationSpecOverride",
}

# The decisions on the untyped fields, as full-type replacements (the
# container is part of the decision, so no wrapping applies on top). Each is
# checked against the forms the corpus actually contains and the consuming
# code: the *_any quartet reads through utils.ToIntWithEnv (int or env-var
# string); domain_keys values are ParseDomainKeyInfo's two forms ("key" or
# ["k1","k2"]); env values are the scalar settings the engine substitutes;
# response_format is the inference API's "json"-or-schema union. ollama's
# options stays dict[str, Any] deliberately - it is a pass-through to the API.
FULL_TYPE_OVERRIDES = {
    "domain_keys": "dict[str, str | list[str]]",
    "env": "dict[str, str | int | float | bool]",
    "response_format": "str | dict[str, Any]",
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

HEADER = '''"""The cpipes contract - THE SOURCE OF TRUTH for the contract claims.

Since the source-of-truth flip, this Pydantic v2 model is where the contract is edited:
which fields exist per operator token, which are required, their value ranges,
and their descriptions. The matrix CSVs remain the review artifact and the
audit/Go-binding record, regenerated FROM this file:

    python -m cpipes_contract reflect          # sync the matrix claim columns
    python -m cpipes_contract reflect --check  # the divergence guard (CI)

`generate` emits this file from the matrix, and carries over from it the
regions marked `# >>> hand-written` (code the matrix has no column for).
`generate --check` (and tests_generate.py) fail when this file is not what it
emits, so an edit here is reflected into the matrix before it is committed:

    python -m cpipes_contract reflect           # the edit, into the matrix
    python -m cpipes_contract generate --check  # the matrix still emits this file

Engine-applied defaults are noted in the descriptions and deliberately
NOT materialised as Pydantic defaults, so a dumped document stays minimal and
the wire format unchanged.
"""

from __future__ import annotations

from typing import Annotated, Any, Literal, Union

from pydantic import AfterValidator, BaseModel, BeforeValidator, ConfigDict, Field


def _tag_default(key: str, default: str):
    """Inject the engine's default discriminator value when the document
    omits it, so the discriminated union accepts untyped instances the way
    the engine does."""

    def inject(value):
        if isinstance(value, dict) and key not in value:
            return {**value, key: default}
        return value

    return inject


def _unlisted(tokens: tuple[str, ...]):
    """Refuse a discriminator value that is one of the union's own tokens, or
    empty - the two halves of a complement branch's membership.

    The branch states both in `json_schema_extra` (`"not": {"enum": [...]}`
    and `"minLength": 1`) and states them to the *schema*: Pydantic never
    reads that dict while validating. Without this the emitted JSON Schema and
    this model disagree about one rule, and they disagree in the direction
    that hurts - a malformed built-in fails its own tagged branch, satisfies
    the complement branch whose `type` is a bare `str`, and is reported as an
    unknown site operator rather than as the malformed built-in it is. The
    engine's counterpart is `validateSiteOperatorSpec`
    (`jets/compute_pipes/site_operators.go`), which refuses a `site_config` on
    a token the dispatch handles itself.

    The empty string is the second half and is not padding: `not: enum`
    admits it, an empty `type` is the `~override` shape, and in an `apply`
    position it reaches `buildSiteOperator`, which cannot find it -
    `WithOperators` refuses to register an empty name at all.
    """

    def check(value: str) -> str:
        if value in tokens:
            raise ValueError(
                f"'{value}' is a built-in operator, dispatched by name, and cannot "
                "be a site operator. If it is meant to be the built-in, its own "
                "configuration is what failed to validate."
            )
        if not value:
            raise ValueError(
                "a site operator's type is the name it is registered under and "
                "cannot be empty"
            )
        return value

    return check


class _Base(BaseModel):
    model_config = ConfigDict(extra="forbid")

'''


def camel(token: str) -> str:
    parts = token.lstrip(VIRTUAL_PREFIX).replace("-", "_").split("_")
    return "".join(p[:1].upper() + p[1:] for p in parts if p)


def complement_tokens_name(struct: str) -> str:
    """`TransformationSpec` -> `_TRANSFORMATION_SPEC_TOKENS`."""
    snake = re.sub(r"(?<!^)(?=[A-Z])", "_", struct).upper()
    return f"_{snake}_TOKENS"


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
        override = FULL_TYPE_OVERRIDES.get(row.json_key)
        if override is not None:
            return override
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
            has_default = (
                row.required is not Required.YES and str(row.default) == literal_token
            )
            default = f' = "{literal_token}"' if has_default else ""
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

    def complement_field_line(self, row: FieldRow, struct: str) -> str:
        """The discriminator of an `unlisted(...)` token: a string that is none of them.

        A Pydantic `Literal` cannot say *any token but these*, so the constraint
        reaches the emitted schema as `json_schema_extra` — `"not": {"enum":
        [...]}`— and reaches *validation* through `AfterValidator(_unlisted(...))`,
        because `json_schema_extra` is schema-only and a model that merely
        described the rule would disagree with the schema the same emitter
        writes. `_unlisted`'s docstring carries which direction that disagreement
        runs in.

        **Both readers take the tuple emitted by `complement_tokens_line`**, so
        the enum and the refusal are one list rather than two copies of one —
        which is the mistake `builtinOperatorTypes` and `reportsRowLevelFailures`
        each have a test against on the Go side. The tuple itself is the struct's
        own value tokens read off `types.csv`, so the branch is the exact
        complement of the union's other branches.

        **`minLength` is half the constraint and is not padding.** `not: enum`
        admits the empty string, which is not an unlisted token: an empty `type`
        is the `~override` shape, and in an `apply` position it reaches
        `buildSiteOperator`, which cannot find it - `WithOperators` refuses to
        register an empty name at all (`site_operators.go:64`). Without this the
        negative suite's own case validated.
        """
        desc = (row.description or "").strip()
        desc = desc.replace("\\", "\\\\").replace('"', '\\"')
        const = complement_tokens_name(struct)
        extra = (
            f'json_schema_extra={{"minLength": 1, "not": {{"enum": list({const})}}}}'
        )
        parts = [f'description="{desc}"'] if desc else []
        parts.append(extra)
        return (
            f"    {row.json_key}: Annotated[str, AfterValidator(_unlisted({const}))]"
            f" = Field({', '.join(parts)})"
        )

    def complement_tokens_line(self, struct: str, value_tokens: list[str]) -> str:
        """The union's own value tokens, named once and read twice.

        Emitted immediately above the complement class, because a class body
        evaluates its `json_schema_extra` at definition time.
        """
        tokens = ", ".join(f'"{token}"' for token in value_tokens)
        if len(value_tokens) == 1:
            tokens += ","
        return f"{complement_tokens_name(struct)} = ({tokens})"

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
        # **A union may have no value token at all, and this raised IndexError on
        # `real[0]` until 2026-09-13.** `Element` is the case: a paragraph and a
        # group are told apart by which of `text` and `elements` is present, so
        # both of its rows are `~virtual` and there is no real token to take the
        # common fields from. The bootstrap has been unrunnable since that row
        # landed on 2026-09-11 and nothing said so, because `generate` is the one
        # command the working loop never runs - the model is the source of truth
        # and `reflect` is what checks it. Found here only because the contract work needed
        # the bootstrap to verify a class it was about to write by hand.
        first: dict[str, FieldRow] = {}
        if real:
            first = per_token[real[0].type_token]
            for key, row in first.items():
                sig = self.signature(row)
                if all(
                    key in per_token[t.type_token]
                    and self.signature(per_token[t.type_token][key]) == sig
                    for t in real[1:]
                ):
                    common.append(key)

        # A base class with no value token under it would be a class nothing
        # inherits: the virtual subclasses deliberately descend from `_Base`.
        if real:
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
            # **A complement token keeps its discriminator, and it is the only
            # kind of virtual token that does.** `~override` is selected by the
            # discriminator being *absent*, so carrying it would contradict the
            # membership; `~site` is selected by its value being one no row of
            # this struct claims, so the key is not merely present but required,
            # and the branch is nothing without it. `unlisted(...)` is what
            # tells the two apart.
            when_kind, when_key = (
                parse_variant_when(t.variant_when)
                if t.variant_when not in (NONE, None)
                else ("", "")
            )
            complement = when_kind == "unlisted" and when_key == disc
            value_tokens = [r.type_token for r in real]
            if complement:
                out.append(self.complement_tokens_line(struct, value_tokens))
                out.append("")
                out.append("")
            out.append(f"class {cname}(_Base):")
            doc = self.type_doc(t)
            if doc:
                out.append(f'    """{doc}"""')
            rows = [
                f
                for f in self.fields[(struct, t.type_token)]
                if f.applicable is YesNo.YES
                and (f.json_key != disc or complement)
            ]
            if not rows and not doc:
                out.append("    pass")
            for row in sorted(rows, key=lambda r: r.json_key):
                if complement and row.json_key == disc:
                    out.append(self.complement_field_line(row, struct))
                else:
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
        doc = MERGED_DOC.get(struct) or self.type_doc(tokens[0], base=len(tokens) > 1)
        if doc:
            out.append(f'    """{doc}"""')
        if not merged and not doc:
            out.append("    pass")
        for key in sorted(merged):
            row = self.merged_row(struct, tokens, key)
            if disc and key == disc and len(tokens) > 1:
                opts = ", ".join(f'"{v}"' for v in literal_values)
                desc = MERGED_DISC_DOC.get(struct) or (row.description or "").strip()
                desc = desc.replace("\\", "\\\\").replace('"', '\\"')
                out.append(
                    f'    {key}: Literal[{opts}] | None = Field(default=None, description="{desc}")'
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

    def merged_row(self, struct: str, tokens: list[TypeRow], key: str) -> FieldRow:
        """The row a merged class's field is emitted from: the first one carrying
        the description most of the struct's tokens give it.

        Taking the first token's row made the emitted description depend on the
        order of types.csv: ContextSpec's `expr` reads "The extraction pattern
        (expr=value) or the literal value." on three tokens and something else
        on `default_value`, which sorts first, so the bootstrap described the
        field by its one exception.
        """
        rows = [
            f
            for t in tokens
            for f in self.fields[(struct, t.type_token)]
            if f.applicable is YesNo.YES and f.json_key == key
        ]
        counts = Counter((r.description or "").strip() for r in rows)
        best = max(counts.values())
        return next(r for r in rows if counts[(r.description or "").strip()] == best)

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
        if not real:
            # Shape-selected all the way down: there is no discriminator field to
            # tag a branch with, so the union is ordered rather than tagged and
            # `extra="forbid"` on both branches is what makes it exact. This is
            # the shape `Element` already has in the model, by hand.
            members = ", ".join(
                self.class_name(struct, t.type_token)
                for t in tokens
                if t.type_token.startswith(VIRTUAL_PREFIX)
            )
            out.append(
                f'{struct} = Annotated[Union[{members}], Field(union_mode="left_to_right")]'
            )
            return
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
        tagged = f"Annotated[{', '.join(parts)}]"

        complement = self.complement_token(struct)
        if complement is None:
            out.append(f"{struct} = {tagged}")
            return

        # **The complement branch joins the alias, and it cannot join the tagged
        # union.** `Field(discriminator=...)` keys its members by literal, and a
        # branch whose token is *any string but* those has no literal to be keyed
        # by - which is why the emitted JSON Schema gets this member by a splice
        # (`schema.py`) rather than from Pydantic. What Pydantic *can* do is hold
        # the tagged union and the complement branch in one ordered union, and
        # that is what the engine's dispatch is: `BuildPipeTransformationEvaluator`
        # tries the built-in tokens and falls through to the site registry.
        #
        # **Left-to-right and not `smart`**, because the order is the rule rather
        # than a preference: the complement branch's discriminator is a bare `str`
        # and would otherwise be a candidate for every token, built-ins included.
        # The second half of that guarantee is `_unlisted`, which refuses a
        # built-in token on the complement branch outright - so a *malformed*
        # built-in fails both branches and is reported against its own.
        #
        # **Emitting this changed no byte of `cpipes_schema.json`.** Pydantic
        # writes the widened alias as `anyOf: [<tagged oneOf>, <branch>]`, which
        # `schema.splice_complement_branches` flattens back into the single
        # discriminated `oneOf` the schema has always had; that function's
        # docstring carries the argument.
        branch = self.class_name(struct, complement.type_token)
        out.append(f"{struct} = Annotated[")
        out.append("    Union[")
        out.append(f"        {tagged},")
        out.append(f"        {branch},")
        out.append("    ],")
        out.append('    Field(union_mode="left_to_right"),')
        out.append("]")

    def complement_token(self, struct: str) -> TypeRow | None:
        """The struct's `unlisted(<discriminator>)` token, or None.

        One row at most: two complement branches on one union would be two
        branches claiming the same membership, which `check` refuses.
        """
        disc = self.types[struct][0].discriminator
        for t in self.types[struct]:
            if t.variant_when in (NONE, None):
                continue
            when_kind, when_key = parse_variant_when(t.variant_when)
            if when_kind == "unlisted" and when_key == disc:
                return t
        return None

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
    # Only the engine-default variant of a union may leave its tag optional:
    # in JSON Schema (no tag injection there) an untagged document must match
    # exactly one oneOf branch, so every other variant requires its tag.
    if row.required is not Required.YES and str(row.default) == token:
        return f'default="{token}", '
    return ""


# -- hand-written regions -----------------------------------------------------
#
# **The model is edited by hand, and most of what is edited flows back through
# the matrix.** A field's claims and description are reflected into fields.csv
# (`reflect`), so emitting from the matrix reproduces them. What the matrix has
# no column for is code: a validator, a closed list a validator reads, the
# runtime projection defined after `_MODELS`, a comment explaining a union.
# Until 2026-11-27 that code lived only in the model, so running `generate`
# deleted it, and nothing ran `generate` to notice. It is now marked in the
# model and carried through:
#
#     # >>> hand-written: before TableSpec
#     ...
#     # <<< hand-written
#
# The anchor says where the region goes in the emitted text:
#
#     before <Name>          directly above `class <Name>(` or `<Name> = `
#     field <Class>.<key>    in place of that field's emitted line
#     end                    after the emitted text
#
# A region whose anchor names nothing the emitter writes is refused rather than
# dropped: a region that silently stops being emitted is the deletion this
# exists to prevent.

REGION_OPEN = "# >>> hand-written:"
REGION_CLOSE = "# <<< hand-written"


class RegionError(ValueError):
    pass


def hand_regions(text: str) -> list[tuple[str, list[str]]]:
    """The hand-written regions of a model, each as (anchor, lines), markers included."""
    regions: list[tuple[str, list[str]]] = []
    current: list[str] | None = None
    anchor = ""
    for n, line in enumerate(text.split("\n"), 1):
        bare = line.strip()
        if bare.startswith(REGION_OPEN):
            if current is not None:
                raise RegionError(f"line {n}: a hand-written region opens inside another")
            anchor = bare[len(REGION_OPEN):].strip()
            current = [line]
        elif bare == REGION_CLOSE:
            if current is None:
                raise RegionError(f"line {n}: a hand-written region closes without opening")
            current.append(line)
            regions.append((anchor, current))
            current = None
        elif current is not None:
            current.append(line)
    if current is not None:
        raise RegionError(f"the hand-written region `{anchor}` never closes")
    return regions


def splice(text: str, regions: list[tuple[str, list[str]]]) -> str:
    """The emitted text with each hand-written region placed at its anchor."""
    lines = text.split("\n")
    if lines and lines[-1] == "":
        lines.pop()
    tail: list[str] = []
    for anchor, region in regions:
        kind, _, name = anchor.partition(" ")
        if kind == "end" and not name:
            tail += ["", ""] + region
            continue
        if kind == "before" and name:
            at = [
                i for i, l in enumerate(lines)
                if l.startswith(f"class {name}(") or l.startswith(f"{name} = ")
            ]
            if len(at) != 1:
                raise RegionError(f"hand-written region `{anchor}`: generate emits no `{name}` to place it above")
            i = at[0]
            blank = 0
            while blank < i and lines[i - 1 - blank] == "":
                blank += 1
            lines[i:i] = region + [""] * blank
            continue
        if kind == "field" and "." in name:
            cls, key = name.split(".", 1)
            start = next(
                (i for i, l in enumerate(lines) if l.startswith(f"class {cls}(")), None
            )
            hit = None
            if start is not None:
                for i in range(start + 1, len(lines)):
                    if lines[i] and not lines[i].startswith(" "):
                        break
                    if lines[i].startswith(f"    {key}:"):
                        hit = i
                        break
            if hit is None:
                raise RegionError(f"hand-written region `{anchor}`: generate emits no field `{key}` on `{cls}`")
            lines[hit:hit + 1] = region
            continue
        raise RegionError(
            f"hand-written region `{anchor}`: the anchor is `before <Name>`, `field <Class>.<key>` or `end`"
        )
    return "\n".join(lines + tail) + "\n"


def render(matrix: Matrix, model: Path | None) -> str:
    """The model as the matrix emits it, with `model`'s hand-written regions carried."""
    text = Emitter(matrix).emit()
    if model is None or not model.is_file():
        return text
    return splice(text, hand_regions(model.read_text()))


def run_with_matrix(args: argparse.Namespace, matrix: Matrix) -> int:
    model = args.model
    try:
        text = render(matrix, model)
    except RegionError as err:
        print(f"generate: {model}: {err}")
        return 1
    if args.check:
        have = model.read_text() if model.is_file() else ""
        if have == text:
            print(f"generate --check: {model} is what the matrix generates")
            return 0
        diff = list(
            difflib.unified_diff(
                have.splitlines(), text.splitlines(),
                f"{model.name} (committed)", f"{model.name} (generated)", lineterm="", n=1,
            )
        )
        print(f"generate --check: {model} differs from what the matrix generates:")
        for line in diff[:80]:
            print(line)
        if len(diff) > 80:
            print(f"... {len(diff) - 80} more diff line(s)")
        print(
            "Reflect a model edit into the matrix (`reflect`), mark code the matrix "
            "cannot carry as a hand-written region, then `generate`."
        )
        return 1
    out = args.out or model
    out.write_text(text)
    print(f"wrote {out}")
    return 0
