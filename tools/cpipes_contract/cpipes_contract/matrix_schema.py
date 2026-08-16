"""Schema of the cpipes applicability matrix.

The matrix is the primary deliverable of gap 2b: the contract that says which fields
apply to which operator, which of those are required, and what the defaults are.
Reflection over `pipes_model.go` recovers the field inventory and nothing about
applicability, so the matrix is extracted from the code and the corpus and reviewed a
row at a time.

Three tables, not one. `types.csv` holds one row per addressable type — a Go struct
paired with one value of its discriminator — and is the unit the JSON Schema `$defs`
entries, the Pydantic subclasses and the fragment library are all keyed by.
`fields.csv` holds one row per field of one of those types, and is the matrix proper.
`constraints.csv` holds the requirements that span more than one field, which
(applicable, required) cannot express.

Every *judged* cell is filled: `-` means "none / not applicable", and a blank cell is
**unfilled** — mechanically extracted but not yet judged. The three states are
distinct on purpose: `-` is a claim, a blank is the absence of one, and the review
turns blanks into claims. A blank identity cell (the struct, the field, the wire key)
is still an error, since those come from the source and are never a judgment; and a
`reviewed` row may not contain a blank at all. `check --strict` treats any blank as a
failure, because at end state the worklist must be empty.
"""

from __future__ import annotations

import csv
import re
from enum import StrEnum
from pathlib import Path
from typing import Annotated, Iterable, Sequence, TypeVar

from pydantic import BaseModel, BeforeValidator, ConfigDict, model_validator

# The sentinel for an absent value. Chosen over the empty string so that an empty cell
# never reads as a claim: `-` is "none, and reviewed to be none", a blank is "not yet
# judged" (loaded as Python None), and the two must never collapse into each other.
NONE = "-"


def _blank_to_none(value):
    """A blank CSV cell is an unfilled one. Applied to the judgment columns only."""
    if isinstance(value, str) and value.strip() == "":
        return None
    return value


def _opt(kind):
    """An optional (fillable-later) column of the given type."""
    return Annotated[kind | None, BeforeValidator(_blank_to_none)]


def unfilled(row: "Row") -> list[str]:
    """The columns of this row still awaiting judgment. Empty for a finished row."""
    return [name for name, value in row if value is None]

# The token standing for "this struct has no discriminator, the row covers every
# instance of it".
ANY_TOKEN = "*"

# The prefix of a *virtual* token: a variant not identified by a value of the
# discriminator but by the shape of the node, as `variant_when` states. The engine
# contains two such discriminations and the seed's grammar could express neither:
# an ExpressionNode is unary when `arg` is set and binary when `lhs` is
# (eval_expression.go:208, tested in that order, before `type` is consulted), and a
# TransformationSpec inside conditional_config.then with no `type` at all is a field
# override rather than an operator (actions_start_common.go:707). The `~` keeps the
# virtual vocabulary out of the wire vocabulary while letting the mechanical naming
# rule apply unchanged: CamelCase(unary) + ExpressionNode = UnaryExpressionNode.
VIRTUAL_PREFIX = "~"

_VIRTUAL_TOKEN = re.compile(r"^~[a-z][a-z0-9_]*$")

# The membership predicate of a virtual token. `present(k)` means the json key k is
# set and not null; `absent(k)` means it is missing, null or the empty string - the
# reading Go gives an omitted `string` field.
_VARIANT_WHEN = re.compile(r"^(present|absent)\(([a-z_][a-z0-9_]*)\)$")


def parse_variant_when(cell: str) -> tuple[str, str]:
    """Split `present(lhs)` into ('present', 'lhs'). The cell must already be valid."""
    match = _VARIANT_WHEN.match(cell)
    if match is None:
        raise ValueError(f"not a variant_when predicate: {cell!r}")
    return match.group(1), match.group(2)


def variant_matches(cell: str, node: dict) -> bool:
    """Does this node satisfy a virtual token's membership predicate?"""
    kind, key = parse_variant_when(cell)
    value = node.get(key)
    if kind == "present":
        return value is not None
    return value is None or value == ""


# ---------------------------------------------------------------------------
# Controlled vocabularies
# ---------------------------------------------------------------------------


class Container(StrEnum):
    """How the field's value is shaped on the wire."""

    SCALAR = "scalar"
    OBJECT = "object"
    ARRAY = "array"
    ARRAY2 = "array2"  # `[][]T`: reducing_pipes_config, date_formats, other_date_formats
    MAP = "map"
    RAW_JSON = "raw_json"  # json.RawMessage - carried through unparsed
    ANY = "any"  # Go `any`: accepts more than one shape, see notes


class YesNo(StrEnum):
    YES = "yes"
    NO = "no"


# `deprecated` is a third axis, not a state of `applicable`. A deprecated field is
# applicable, is present in the corpus, and must still validate - three things
# `applicable=no` denies. What it must not do is appear in anything the model is
# prompted to produce, or in the fragment library.


class Required(StrEnum):
    """Three states, not two.

    `conditional` is the state that (applicable, required) alone cannot carry and the
    code contains: device_writer_type is required unless the output channel names a
    schema provider.
    """

    YES = "yes"
    NO = "no"
    CONDITIONAL = "conditional"
    NA = "na"  # the field does not apply to this type at all


class DefaultBy(StrEnum):
    """Who applies the default, which decides who can observe it.

    A default applied by the config validator is already in the config by the time the
    harness looks at it (the validator mutates its input); one applied by the operator
    builder is invisible at config level and invisible to the schema.
    """

    VALIDATOR = "validator"
    BUILDER = "builder"
    NONE = "none"


class Evidence(StrEnum):
    """Where the row's claim comes from, in the authority order of the plan's §5.2.1."""

    VALIDATOR = "validator"  # CpipesStartup.ValidatePipeSpecConfig - hard
    BUILDER = "builder"  # the operator builders and their defaults - hard
    CORPUS = "corpus"  # the live .pc.json: strong for presence, weak for absence,
    # and worth nothing against the validator - see corpus.py
    GENERATOR = "generator"  # the org1 lambda generators
    COMMENT = "comment"  # the Go doc comments


class Harness(StrEnum):
    """The result of the B.7 harness for this row, so the review reads a test result."""

    PASS = "pass"
    FAIL = "fail"
    UNTESTABLE = "untestable"  # builder-evidence rows the config validator never sees
    PENDING = "pending"


class Review(StrEnum):
    """The review axis, kept separate from the evidence axis.

    A row can be corpus-derived and reviewed, or validator-derived and unreviewed. The
    number to watch is rows still `unreviewed` when the model generation wants to start.
    """

    UNREVIEWED = "unreviewed"
    REVIEWED = "reviewed"
    DISPUTED = "disputed"  # the evidence sources disagree; see notes


class ConstraintKind(StrEnum):
    ONE_OF = "one_of"  # exactly one of the members
    AT_LEAST_ONE = "at_least_one"
    MUTUALLY_EXCLUSIVE = "mutually_exclusive"  # at most one
    PRECEDENCE = "precedence"  # all may be set, the first member wins
    REQUIRES = "requires"  # the first member requires the rest
    FORBIDS = "forbids"  # the first member forbids the rest
    EXTERNAL = "external"  # depends on something outside this type


class Enforce(StrEnum):
    """Where the constraint can actually be enforced."""

    SCHEMA = "schema"  # expressible as an if/then overlay
    VALIDATOR = "validator"  # needs the Go validator or the harness
    PROMPT = "prompt"  # expressible only as an instruction to the model


# ---------------------------------------------------------------------------
# Rows
# ---------------------------------------------------------------------------

_IDENT = re.compile(r"^[A-Za-z_][A-Za-z0-9_]*$")


class Row(BaseModel):
    """Base row: no extra columns, no blank identity cells.

    Columns typed plainly are identity: they come from the source or the measurement
    and a blank one is an error. Columns typed through `_opt` are judgment: a blank
    is the unfilled state, loaded as None and written back as a blank.
    """

    model_config = ConfigDict(extra="forbid", str_strip_whitespace=True)

    @model_validator(mode="after")
    def _no_blank_identity(self) -> "Row":
        for name, value in self:
            if isinstance(value, str) and not value:
                raise ValueError(
                    f"column '{name}' is blank; it is not a judgment column, so use "
                    f"'{NONE}' for none"
                )
        return self


class TypeRow(Row):
    """One addressable type: a Go struct paired with one discriminator token.

    This is the unit of the JSON Schema `$defs`, of the Pydantic subclass, and of the
    fragment library. `corpus_instances` and the exemplar are what make it a parts
    library rather than a schema index.
    """

    go_struct: str
    type_token: str  # a value of the discriminator, `*`, or a `~virtual` token
    defs_name: str  # the $defs key; mechanical, see check_defs_name
    discriminator: str  # json key of the discriminating field, or `-`
    variant_when: str  # membership predicate of a `~virtual` token, or `-`
    embeds: str  # comma-separated embedded structs whose fields are promoted, or `-`
    fragment: YesNo  # can it be authored and validated standing alone?
    deprecated: YesNo  # superseded but still valid: excluded from the fragment library
    corpus_instances: int
    corpus_prod_instances: int  # of those, outside a `test/` directory
    exemplar_file: str  # a real occurrence, or `-` when the corpus has none
    exemplar_path: str  # dot path into that file, the OllamaMappingSpec.Path convention
    doc_ref: str  # file:line of the type definition
    harness: Harness  # written by the B.7 harness: was the minimal config accepted?
    description: str
    notes: str

    @model_validator(mode="after")
    def _coherent(self) -> "TypeRow":
        virtual = self.type_token.startswith(VIRTUAL_PREFIX)
        if virtual and not _VIRTUAL_TOKEN.match(self.type_token):
            raise ValueError(f"malformed virtual token: {self.type_token!r}")
        if virtual != (self.variant_when != NONE):
            raise ValueError(
                "variant_when must be set exactly when the token is `~virtual`: a value "
                "token's membership is the discriminator, a virtual token's is its predicate"
            )
        if virtual:
            parse_variant_when(self.variant_when)  # raises on a malformed predicate
            if self.discriminator == NONE:
                raise ValueError(
                    "a virtual token still names the struct's discriminator, so the "
                    "rows of one struct can be checked for agreement on it"
                )
        return self


class FieldRow(Row):
    """One field of one addressable type. The matrix proper.

    The identity columns (through `ref_struct`) are extracted from the source and are
    never blank. The judgment columns load a blank as None — the unfilled state B.2's
    mechanical extraction leaves behind for the review to settle — and every coherence
    rule below runs only once the cells it relates are filled, so filling one column
    does not demand the whole row at once. The exception is `reviewed`, which demands
    the whole row: see the last rule.
    """

    go_struct: str
    type_token: str
    field_name: str  # the Go field name
    json_key: str
    declared_in: str  # the struct declaring it; differs when promoted through an embed
    go_type: str
    container: Container
    ref_struct: str  # the struct this field's value is, or `-` for scalars
    values: _opt(str)  # pipe-separated closed value set, or `-`
    applicable: _opt(YesNo)
    required: _opt(Required)
    required_when: _opt(str)  # the condition, when required is `conditional`
    default: _opt(str)  # the literal applied when absent, or `-`
    default_by: _opt(DefaultBy)
    evidence: _opt(Evidence)
    evidence_ref: _opt(str)  # file:line; `-` is allowed only for corpus evidence
    deprecated: _opt(YesNo)  # superseded but still valid: must validate, not generate
    corpus_count: _opt(int)  # measured; blank until the walk reaches the type
    corpus_prod_count: _opt(int)  # of those, outside a `test/` directory
    harness: Harness
    review: Review
    description: _opt(str)
    notes: _opt(str)

    @model_validator(mode="after")
    def _coherent(self) -> "FieldRow":
        def have(*cells) -> bool:
            return all(cell is not None for cell in cells)

        if have(self.required, self.applicable):
            if (self.required is Required.NA) != (self.applicable is YesNo.NO):
                raise ValueError("required must be 'na' exactly when applicable is 'no'")
        if have(self.required) and (
            (self.required_when is not None and self.required_when != NONE)
            != (self.required is Required.CONDITIONAL)
        ):
            raise ValueError(
                "required_when must be set exactly when required is 'conditional'"
            )
        if have(self.default_by, self.default):
            if (self.default_by is DefaultBy.NONE) != (self.default == NONE):
                raise ValueError("default_by must be 'none' exactly when default is '-'")
        if have(self.applicable, self.default):
            if self.applicable is YesNo.NO and self.default != NONE:
                raise ValueError(
                    "an inapplicable field has no default; put the forcing in notes"
                )
        if self.container is Container.OBJECT and self.ref_struct == NONE:
            raise ValueError("an object field must name its ref_struct")
        if self.ref_struct != NONE and self.container in (
            Container.SCALAR,
            Container.RAW_JSON,
        ):
            raise ValueError(f"a {self.container} field cannot have a ref_struct")
        if have(self.evidence):
            if self.evidence is not Evidence.CORPUS and (
                self.evidence_ref is None or self.evidence_ref == NONE
            ):
                raise ValueError(f"{self.evidence} evidence must carry a file:line ref")
        if have(self.deprecated, self.applicable):
            if self.deprecated is YesNo.YES and self.applicable is YesNo.NO:
                raise ValueError(
                    "deprecated and applicable=no are different claims; a deprecated "
                    "field is still applicable and must still validate"
                )
        if have(self.corpus_count, self.corpus_prod_count):
            if self.corpus_prod_count > self.corpus_count:
                raise ValueError("corpus_prod_count cannot exceed corpus_count")
        if have(self.applicable, self.corpus_count):
            if self.applicable is YesNo.NO and self.corpus_count > 0:
                raise ValueError(
                    f"claimed inapplicable but occurs {self.corpus_count} times "
                    f"in the corpus"
                )
        if self.review is Review.REVIEWED:
            if unfilled(self):
                raise ValueError(
                    f"a reviewed row may not have unfilled cells: {unfilled(self)}"
                )
            if self.evidence_ref == NONE and self.corpus_count == 0:
                raise ValueError("a reviewed row must cite something")
        return self


class ConstraintRow(Row):
    """A requirement spanning more than one field of one type."""

    go_struct: str
    type_token: str
    kind: ConstraintKind
    members: str  # pipe-separated json keys, in significant order for `precedence`
    enforce: _opt(Enforce)
    evidence: _opt(Evidence)
    evidence_ref: _opt(str)
    review: Review
    notes: _opt(str)

    @model_validator(mode="after")
    def _members(self) -> "ConstraintRow":
        parts = [p for p in self.members.split("|") if p]
        if len(parts) < 2:
            raise ValueError("a constraint needs at least two members")
        if self.evidence is not None:
            if self.evidence is not Evidence.CORPUS and (
                self.evidence_ref is None or self.evidence_ref == NONE
            ):
                raise ValueError(f"{self.evidence} evidence must carry a file:line ref")
        if self.review is Review.REVIEWED and unfilled(self):
            raise ValueError(
                f"a reviewed row may not have unfilled cells: {unfilled(self)}"
            )
        return self


# ---------------------------------------------------------------------------
# CSV round-trip
# ---------------------------------------------------------------------------

R = TypeVar("R", bound=Row)

TYPES_CSV = "types.csv"
FIELDS_CSV = "fields.csv"
CONSTRAINTS_CSV = "constraints.csv"

_WS = re.compile(r"\s+")


def header(model: type[Row]) -> list[str]:
    """The column order, taken from the model. The model is the source of the header."""
    return list(model.model_fields)


def read_table(path: Path, model: type[R]) -> list[R]:
    """Read a matrix CSV, failing on a header that is not exactly the model's."""
    with path.open(newline="", encoding="utf-8") as fh:
        reader = csv.DictReader(fh)
        expected = header(model)
        if reader.fieldnames != expected:
            raise ValueError(
                f"{path.name}: header mismatch\n  expected: {','.join(expected)}\n"
                f"  found:    {','.join(reader.fieldnames or [])}"
            )
        rows: list[R] = []
        for n, raw in enumerate(reader, start=2):
            try:
                rows.append(model(**raw))
            except Exception as err:  # noqa: BLE001 - the line number is the point
                raise ValueError(f"{path.name}:{n}: {err}") from err
        return rows


def write_table(path: Path, rows: Sequence[Row], model: type[Row]) -> None:
    """Write a matrix CSV. Descriptions are flattened to one line so the file stays
    reviewable in a spreadsheet and diffable in git."""
    with path.open("w", newline="", encoding="utf-8") as fh:
        writer = csv.DictWriter(fh, fieldnames=header(model), lineterminator="\n")
        writer.writeheader()
        for row in rows:
            record = {
                k: _WS.sub(" ", str(v)).strip() if isinstance(v, str) else v
                for k, v in row.model_dump().items()
            }
            writer.writerow(record)


class Matrix(BaseModel):
    """The three tables, loaded together."""

    model_config = ConfigDict(arbitrary_types_allowed=True)

    types: list[TypeRow]
    fields_: list[FieldRow]
    constraints: list[ConstraintRow]

    @classmethod
    def load(cls, directory: Path) -> "Matrix":
        return cls(
            types=read_table(directory / TYPES_CSV, TypeRow),
            fields_=read_table(directory / FIELDS_CSV, FieldRow),
            constraints=read_table(directory / CONSTRAINTS_CSV, ConstraintRow),
        )

    def save(self, directory: Path) -> None:
        directory.mkdir(parents=True, exist_ok=True)
        write_table(directory / TYPES_CSV, self.types, TypeRow)
        write_table(directory / FIELDS_CSV, self.fields_, FieldRow)
        write_table(directory / CONSTRAINTS_CSV, self.constraints, ConstraintRow)


# ---------------------------------------------------------------------------
# The $defs naming rule
# ---------------------------------------------------------------------------


def defs_name_for(go_struct: str, type_token: str) -> str:
    """`OllamaTransformationSpec` from (TransformationSpec, ollama).

    Mechanical so that the Pydantic subclass name, the `$defs` key and the fragment
    library entry are one name rather than three conventions. Where a token repeats a
    word of the struct the name stutters (`OutputOutputChannelConfig`) - the price of
    a rule under which no hand-picked pair of names can collide.

    A virtual token drops its `~` and is otherwise treated the same, which is what
    makes `~override` + TransformationSpec = OverrideTransformationSpec. Hyphens split
    like underscores, for `de-identification` -> DeIdentificationAnonymizeSpec.
    """
    if type_token == ANY_TOKEN:
        return go_struct
    token = type_token.removeprefix(VIRTUAL_PREFIX)
    camel = "".join(part.capitalize() for part in re.split(r"[_-]", token))
    return f"{camel}{go_struct}"


def split_list(cell: str) -> list[str]:
    """Read a `a|b|c` cell. `-` is the empty list."""
    return [] if cell == NONE else [p for p in cell.split("|") if p]


def split_csv_cell(cell: str) -> Iterable[str]:
    """Read an `a,b` cell (used by `embeds`). `-` is the empty list."""
    return [] if cell == NONE else [p.strip() for p in cell.split(",") if p.strip()]
