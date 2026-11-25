"""Checks over the applicability matrix.

The matrix is meant to be executable rather than documentary, and these are the checks
that hold before any of the extraction is done: internal coherence, referential
integrity between the three tables, and — with `--corpus` — that every exemplar the
fragment library points at actually resolves.

Two modes. The default runs the checks that hold while the matrix is still partial,
which is the whole of the extraction work. `--strict` adds the checks that only make sense
once extraction is complete, and is what the exit criteria run.
"""

from __future__ import annotations

import json
from collections import Counter, defaultdict
from pathlib import Path

from . import aliases
from .corpus import LIVE_PARENT
from .matrix_schema import (
    ANY_TOKEN,
    NONE,
    VIRTUAL_PREFIX,
    ConstraintKind,
    ConstraintRow,
    FieldRow,
    Matrix,
    Review,
    TypeRow,
    YesNo,
    defs_name_for,
    row_hash,
    split_csv_cell,
    split_list,
    unfilled,
    variant_matches,
)


def unfilled_report(matrix: Matrix) -> str | None:
    """One line naming how much of the matrix still awaits judgment, or None.

    Not a failure outside --strict: a blank cell is the extraction working as
    designed. But it is the number the review risk watches, so `check` always prints it.
    """
    rows = cells = 0
    for row in [*matrix.fields_, *matrix.constraints]:
        blanks = unfilled(row)
        if blanks:
            rows += 1
            cells += len(blanks)
    if not rows:
        return None
    return f"unfilled: {cells} cells over {rows} rows await review"


def _key(row: TypeRow | FieldRow | ConstraintRow) -> str:
    if isinstance(row, TypeRow):
        return f"{row.go_struct}/{row.type_token}"
    if isinstance(row, FieldRow):
        return f"{row.go_struct}/{row.type_token}.{row.field_name}"
    return f"{row.go_struct}/{row.type_token} {row.kind}({row.members})"


def check(matrix: Matrix, strict: bool = False) -> list[str]:
    """Return the problems found. An empty list is a pass."""
    problems: list[str] = []

    def bad(where: str, message: str) -> None:
        problems.append(f"{where}: {message}")

    types_by_key = {(t.go_struct, t.type_token): t for t in matrix.types}
    known_structs = {t.go_struct for t in matrix.types}

    # --- uniqueness -------------------------------------------------------
    for label, rows in (
        ("types", matrix.types),
        ("fields", matrix.fields_),
        ("constraints", matrix.constraints),
    ):
        counts = Counter(_key(r) for r in rows)
        for key, n in counts.items():
            if n > 1:
                bad(label, f"{key} appears {n} times")

    # --- types ------------------------------------------------------------
    discriminators: dict[str, set[str]] = {}
    # Two rows may share a defs_name only by sharing a struct the model merges -
    # ExpressionNode's seven tokens are one class and one $defs entry. Two different
    # structs colliding on one key is the collision the naming rule exists to prevent.
    structs_by_defs_name: dict[str, set[str]] = defaultdict(set)
    for t in matrix.types:
        structs_by_defs_name[t.defs_name].add(t.go_struct)
    for name, structs in structs_by_defs_name.items():
        if len(structs) > 1:
            bad("types", f"defs_name {name} is claimed by {sorted(structs)}; $defs keys are unique")
    for t in matrix.types:
        where = f"types {_key(t)}"
        expected = defs_name_for(t.go_struct, t.type_token)
        # A struct the model expresses as one class rather than one per token is
        # addressable only under the struct's own name - ExpressionNode and
        # ContextSpec by merge, ReportCmdSpec by having a single variant. For those
        # the column records what the schema calls it, which is the column's job.
        if t.defs_name not in (expected, t.go_struct):
            bad(where, f"defs_name should be {expected} or {t.go_struct}, found {t.defs_name}")
        # A token implies a discriminator, **except where the discrimination is
        # by shape all the way down.** The rule read "'-' exactly when the token
        # is '*'" until 2026-09-11, which conflated *this struct has variants*
        # with *this struct has a discriminating key*. The two virtual tokens
        # that existed then - ExpressionNode's `~unary` and `~binary`,
        # TransformationSpec's `~override` - both sit on structs that also carry
        # a value discriminator, so the conflation cost nothing and was never
        # tested. `Element` is the first struct whose only variants are virtual:
        # a paragraph and a group are told apart by which of `text` and
        # `elements` is present and by nothing else, and `variant_when` is the
        # column that says so. Naming a discriminator there would be inventing a
        # key the wire format has not got.
        virtual = t.type_token.startswith(VIRTUAL_PREFIX)
        shape_only = virtual and all(
            r.type_token.startswith(VIRTUAL_PREFIX)
            for r in matrix.types
            if r.go_struct == t.go_struct
        )
        if (t.discriminator == NONE) != (t.type_token == ANY_TOKEN or shape_only):
            bad(
                where,
                "discriminator must be '-' exactly when type_token is '*' or the "
                "struct's variants are all `~virtual`",
            )
        if strict and (t.corpus_instances > 0) != (t.exemplar_file != NONE):
            # Strict-only: while extraction is partial the walk cannot reach every
            # type, so a hand-proposed exemplar legitimately sits on a row whose
            # instances have not been measured yet. `check --corpus` verifies the
            # proposal either way; once extraction is complete, `corpus --apply`
            # makes the two agree and this becomes an invariant.
            bad(
                where,
                "exemplar_file must be set exactly when corpus_instances > 0",
            )
        if t.exemplar_file == NONE and t.exemplar_path != NONE:
            # `exemplar_path = -` is legitimate on its own - it means the exemplar is
            # the whole file - but a path with no file to resolve it against is not.
            bad(where, "exemplar_path must be '-' when there is no exemplar_file")
        if t.fragment is YesNo.NO and t.notes == NONE:
            bad(where, "fragment=no must say why in notes")
        discriminators.setdefault(t.go_struct, set()).add(t.discriminator)

    for struct, seen in discriminators.items():
        if len(seen) > 1:
            bad(
                f"types {struct}",
                f"rows disagree on the discriminator: {sorted(seen)}",
            )

    # --- fields -----------------------------------------------------------
    fields_by_type: dict[tuple[str, str], list[FieldRow]] = {}
    for f in matrix.fields_:
        where = f"fields {_key(f)}"
        parent = types_by_key.get((f.go_struct, f.type_token))
        if parent is None:
            bad(where, f"no types row for {f.go_struct}/{f.type_token}")
            continue
        fields_by_type.setdefault((f.go_struct, f.type_token), []).append(f)

        if f.declared_in != f.go_struct:
            embedded = list(split_csv_cell(parent.embeds))
            if f.declared_in not in embedded:
                bad(
                    where,
                    f"declared_in={f.declared_in} but the types row does not embed it",
                )
        if f.json_key == parent.discriminator and f.values not in (NONE, None):
            bad(
                where,
                "the discriminator's value set is the types table, leave values as '-'",
            )
        if strict:
            if unfilled(f):
                bad(where, f"unfilled cells: {', '.join(unfilled(f))}")
            if f.ref_struct != NONE and f.ref_struct not in known_structs:
                bad(where, f"ref_struct {f.ref_struct} has no types row")
            if isinstance(f.corpus_count, int) and f.corpus_count > parent.corpus_instances:
                bad(
                    where,
                    f"corpus_count {f.corpus_count} exceeds the type's "
                    f"{parent.corpus_instances} instances",
                )

    if strict:
        for key, t in types_by_key.items():
            rows = fields_by_type.get(key, [])
            if not rows:
                bad(f"types {_key(t)}", "no field rows")
                continue
            if t.discriminator != NONE and not any(
                r.json_key == t.discriminator for r in rows
            ):
                bad(
                    f"types {_key(t)}",
                    f"no field row for the discriminator '{t.discriminator}'",
                )

    # --- constraints ------------------------------------------------------
    for c in matrix.constraints:
        where = f"constraints {_key(c)}"
        if (c.go_struct, c.type_token) not in types_by_key:
            bad(where, f"no types row for {c.go_struct}/{c.type_token}")
            continue
        if strict:
            if unfilled(c):
                bad(where, f"unfilled cells: {', '.join(unfilled(c))}")
            keys = {r.json_key for r in fields_by_type.get((c.go_struct, c.type_token), [])}
            members = split_list(c.members)
            # A constraint is recorded on the innermost type from which every member is
            # reachable, so members are fields of that type or dotted paths below it.
            # `external` is the exception: only its first member is in the type, the
            # rest name something outside it - another section of the document, or the
            # position of the pipe.
            if c.kind is ConstraintKind.EXTERNAL:
                members = members[:1]
            for member in members:
                if "." in member:
                    continue  # a path below this type; resolved by the harness
                if member not in keys:
                    bad(where, f"member '{member}' is not a field of this type")

    # --- the review stamp -------------------------------------------------
    # A `reviewed` mark certifies the row as it stood; the stamp is the proof.
    # An unstamped reviewed row has not finished being reviewed, a mismatched
    # stamp means the row changed underneath its tick (a re-extraction or a
    # `corpus --apply`), and a stamp on an unreviewed row is a leftover. The
    # `stamp` command fills and clears; only a human sets `review`.
    for label, rows in (("fields", matrix.fields_), ("constraints", matrix.constraints)):
        for row in rows:
            where = f"{label} {_key(row)}"
            reviewed = row.review is Review.REVIEWED
            if reviewed and row.reviewed_hash == NONE:
                bad(where, "reviewed but not stamped: run `cpipes-contract stamp`")
            elif reviewed and row.reviewed_hash != row_hash(row):
                bad(
                    where,
                    "changed since it was reviewed: re-review it, then "
                    "`cpipes-contract stamp --restamp`",
                )
            elif not reviewed and row.reviewed_hash != NONE:
                bad(where, "carries a stamp but no reviewed mark: run `cpipes-contract stamp`")

    return problems


# A citation into a workspace rather than into JetStore: generator evidence, which
# lives in a deployment's own lambdas. It resolves against the folder holding
# `workspaces/`, and is checked only where that workspace is present.
WORKSPACES_PREFIX = "workspaces/"


def cites(row: TypeRow | FieldRow | ConstraintRow, line: str) -> bool:
    """Whether `line` names what `row` cites it for: `--code`'s rule, in one place.

    A type row's line must name the struct, exactly. A field row's line must name the
    field by its Go name or its json key, and a constraint's one of its members; for
    those, `spec.Path` is a citation for `path` and `OllamaConfig` for
    `ollama_config`: the Go name and the json key are the same word in two spellings.
    """
    if isinstance(row, TypeRow):
        return row.go_struct in line
    if isinstance(row, FieldRow):
        wanted: tuple[str, ...] = (row.field_name, row.json_key)
    else:
        wanted = tuple(m.rsplit(".", 1)[-1] for m in split_list(row.members))
    flat = line.lower().replace("_", "")
    return any(w.lower().replace("_", "") in flat for w in wanted)


def _cited_names(row: TypeRow | FieldRow | ConstraintRow) -> list[str]:
    if isinstance(row, TypeRow):
        return [row.go_struct]
    if isinstance(row, FieldRow):
        return [row.field_name, row.json_key]
    return [m.rsplit(".", 1)[-1] for m in split_list(row.members)]


def check_citations(
    matrix: Matrix,
    code_root: Path,
    workspaces_root: Path | None = None,
    unchecked: list[str] | None = None,
) -> list[str]:
    """Resolve every `doc_ref` and `evidence_ref`, and check the line is about what it
    is cited for.

    A citation is a claim, and a claim nobody can follow is worth nothing. The cited line
    itself must name the field - by its Go name, its json key, or (for a constraint) one
    of the members. Citing the `if` that decides a default rather than the assignment
    below it still passes, because the test is the line that mentions the field; citing a
    line number that was inferred rather than read does not, which is the failure this
    exists for.

    A citation into `workspaces/` resolves against `workspaces_root` (default: the
    folder holding `code_root`, where a checkout of the workspaces beside JetStore puts
    them). When that workspace is not there at all, the citation cannot be checked from
    this checkout: it is appended to `unchecked` rather than reported as a problem, so
    the count a JetStore-only checkout reports is the count a full one reports.
    """
    problems: list[str] = []
    cache: dict[Path, list[str]] = {}
    names = aliases.load()
    ws_root = (workspaces_root or code_root).resolve()
    if workspaces_root is None:
        # resolve() first: the root is normally given as a relative `../..`, whose
        # textual .parent would be `..` rather than one level further up.
        ws_root = ws_root.parent

    def lines_of(path: Path) -> list[str] | None:
        if path not in cache:
            if not path.is_file():
                return None
            cache[path] = path.read_text(encoding="utf-8").splitlines()
        return cache[path]

    refs: list[tuple[str, TypeRow | FieldRow | ConstraintRow, str, str]] = [
        *((f"types {_key(t)}", t, "doc_ref", t.doc_ref) for t in matrix.types),
        *(
            (_key(r), r, "evidence_ref", r.evidence_ref)
            for r in [*matrix.fields_, *matrix.constraints]
            if r.evidence_ref not in (NONE, None)
        ),
    ]
    for where, row, column, ref in refs:
        file_part, _, line_part = ref.rpartition(":")
        if not file_part or not line_part.isdigit():
            problems.append(f"{where}: {column} is not file:line: {ref}")
            continue
        if file_part.startswith(WORKSPACES_PREFIX):
            # Recorded under the workspace's neutral name; `aliases` says which
            # directory that is, and without it the workspace is simply not here.
            on_disk = Path(names.resolve(file_part))
            workspace = ws_root / on_disk.parts[0] / on_disk.parts[1]
            if not workspace.is_dir():
                if unchecked is not None:
                    unchecked.append(f"{where}: {ref}")
                continue
            path = ws_root / on_disk
        else:
            path = code_root / file_part
        lines = lines_of(path)
        if lines is None:
            problems.append(f"{where}: {column} file not found: {file_part}")
            continue
        n = int(line_part)
        if not 1 <= n <= len(lines):
            problems.append(f"{where}: {ref} is past the end of the file")
            continue
        if not cites(row, lines[n - 1]):
            cited = _cited_names(row)
            named = cited[0] if isinstance(row, TypeRow) else f"any of {cited}"
            problems.append(
                f"{where}: {ref} does not name {named}: {lines[n - 1].strip()!r}"
            )
    return problems


def check_exemplars(matrix: Matrix, corpus_root: Path) -> list[str]:
    """Resolve every exemplar. A fragment library whose exemplars do not resolve is a
    catalogue of things that may not exist."""
    problems: list[str] = []
    names = aliases.load()
    for t in matrix.types:
        if t.exemplar_file == NONE:
            continue
        path = names.path(corpus_root, t.exemplar_file)
        if not path.is_file():
            hint = "" if names else f" (is {aliases.ENV} set?)"
            problems.append(f"types {_key(t)}: exemplar_file not found: {path}{hint}")
            continue
        if LIVE_PARENT not in Path(t.exemplar_file).parts:
            # A retired config under `workspaces/*/data/` is not evidence of anything
            # current, so it cannot stand as the exemplar of a fragment.
            problems.append(
                f"types {_key(t)}: exemplar is not a live config: {t.exemplar_file}"
            )
            continue
        node = json.loads(path.read_text(encoding="utf-8"))
        if t.exemplar_path == NONE:
            # The exemplar is the document itself, which is the root type's case and
            # only its case. `-` reads as "no path into the file", and no path into a
            # file is the file - the same reading `OllamaMappingSpec.Path` gives an
            # empty path (`pipe_transformation_ollama.go:439`). The every-cell-is-filled
            # rule forbids the empty string the Go side uses, so the sentinel stands in.
            continue
        for step in t.exemplar_path.split("."):
            if isinstance(node, list):
                index = int(step) if step.isdigit() else -1
                if index < 0 or index >= len(node):
                    node = None
                    break
                node = node[index]
            elif isinstance(node, dict):
                if step not in node:
                    node = None
                    break
                node = node[step]
            else:
                node = None
                break
        if node is None:
            problems.append(
                f"types {_key(t)}: exemplar_path does not resolve: "
                f"{t.exemplar_file}#{t.exemplar_path}"
            )
            continue
        # The exemplar must be an instance of *this* row, not merely a node that
        # exists. A path landing on the containing array (`lookup_tables` for a
        # LookupSpec) resolves and is still wrong; so is a `standard` splitter held
        # up as the exemplar of `ext_count`.
        if not isinstance(node, dict):
            problems.append(
                f"types {_key(t)}: exemplar_path lands on a {type(node).__name__}, "
                f"not an object; point at one instance: "
                f"{t.exemplar_file}#{t.exemplar_path}"
            )
            continue
        if t.type_token.startswith(VIRTUAL_PREFIX):
            value_tokens = {
                r.type_token
                for r in matrix.types
                if r.go_struct == t.go_struct
                and not r.type_token.startswith(VIRTUAL_PREFIX)
            }
            if not variant_matches(t.variant_when, node, value_tokens):
                problems.append(
                    f"types {_key(t)}: exemplar does not satisfy {t.variant_when}: "
                    f"{t.exemplar_file}#{t.exemplar_path}"
                )
        elif t.type_token != ANY_TOKEN:
            found = node.get(t.discriminator)
            # An absent discriminator is the defaulted case (standard splitter,
            # memory input channel) and is accepted; a *different* value is a node
            # of another row standing in for this one.
            if found is not None and found != "" and found != t.type_token:
                problems.append(
                    f"types {_key(t)}: exemplar is a {t.discriminator}={found!r} "
                    f"node, not {t.type_token}: {t.exemplar_file}#{t.exemplar_path}"
                )
    return problems
