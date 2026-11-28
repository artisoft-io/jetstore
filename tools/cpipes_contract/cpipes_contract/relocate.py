"""Carry a citation whose line moved to where the line went.

`check --code` refuses a citation whose line no longer names its field. Most such
failures are not wrong claims but moved lines: an import, a field or a comment added
above the cited line shifts everything below it, and every citation into the file is
off by the same few lines. Re-deriving each one by eye is what produced line numbers
inferred from the shape of a file, which is the failure `--code` exists to catch.

So the repair is read, not guessed. For each citation `--code` rejects:

1. **Find the commit that wrote the citation**, from the history of the matrix file
   that holds the row: the oldest commit of the newest unbroken run in which the row
   carries this citation. The cited file at that commit is the one the citation was
   written against, and its line must name what the row cites; if it does not, the
   citation was wrong when written and nothing is carried.
2. **Follow the line forward through the cited file's history**, version by version
   from that commit to the file as it is now, carrying it through a
   whitespace-normalised diff of each pair.
3. **Where the line itself changed** in one of those steps, look for the cited name's
   declaration inside the declaration that enclosed the line (the struct, type or
   function), as that declaration reads after the change, and take it only when
   exactly one line there declares the name, or failing that exactly one names it.
4. **Keep the result only when the carried line names what the row cites, by
   `--code`'s own rule, inside a declaration of the same name as the one that held
   the line when the citation was written.**

Until 2026-11-27 step 1 took the newest revision of the cited file in which the
cited line number happened to name the field. For 41 of 407 citations that was a
neighbouring line that merely mentioned the name, and the carry landed on it: two on
another struct's field of the same name (`ShufflingSpec.Comment` on
`DistinctSpec`'s), three on a doc comment. All 41 passed `--code`, because the
substring rule cannot tell a field from its namesake. Anchoring at the commit that
wrote the citation, and checking the enclosing declaration at the end, is what
tells them apart.

`all_citations` carries every citation, not only the rejected ones, and reports any
whose line is not where its history says it went: a citation that passes `--code`
on a namesake.

What it never does is touch `review` or `reviewed_hash`. A citation is part of what a
review certifies, so a reviewed row whose `evidence_ref` moves reads "changed since it
was reviewed" until a reviewer re-approves it with `stamp --restamp`. `types.csv`
carries no stamp, so a `doc_ref` moved here is an ordinary edit.
"""

from __future__ import annotations

import csv
import difflib
import io
import re
import subprocess
from dataclasses import dataclass, field
from pathlib import Path

from .check import WORKSPACES_PREFIX, _key, cites
from .matrix_schema import (
    CONSTRAINTS_CSV,
    FIELDS_CSV,
    NONE,
    TYPES_CSV,
    ConstraintRow,
    FieldRow,
    Matrix,
    Review,
    TypeRow,
)

Row = TypeRow | FieldRow | ConstraintRow

# The citation was written in the working tree and is not in any commit yet: the
# file it was written against is the file as it is now.
WORKTREE = "working tree"


@dataclass
class Move:
    where: str  # the row, as `check` names it
    old: str  # file:line
    new: str | None  # file:line, or None when the line could not be carried
    reason: str  # why not, when new is None; the commit that wrote the citation otherwise
    reviewed: bool  # a stamped row this move lapses
    row: Row = field(repr=False)
    rejected: bool = True  # whether `--code` rejects the citation as it stands


def _git(root: Path, *args: str) -> str:
    return subprocess.run(
        ["git", "-C", str(root), *args],
        check=True,
        capture_output=True,
        text=True,
    ).stdout


def _git_or_none(root: Path, *args: str) -> str | None:
    try:
        return _git(root, *args)
    except subprocess.CalledProcessError:
        return None


# ---------------------------------------------------------------------------
# Which commit wrote a citation
# ---------------------------------------------------------------------------


def _row_identity(row: Row) -> tuple[str, tuple[str, ...], str]:
    """(matrix file, the cells naming the row, the citation column)."""
    if isinstance(row, TypeRow):
        return TYPES_CSV, (row.go_struct, row.type_token), "doc_ref"
    if isinstance(row, FieldRow):
        return FIELDS_CSV, (row.go_struct, row.type_token, row.field_name), "evidence_ref"
    return (
        CONSTRAINTS_CSV,
        (row.go_struct, row.type_token, str(row.kind), row.members),
        "evidence_ref",
    )


_IDENTITY_COLUMNS = {
    TYPES_CSV: ("go_struct", "type_token"),
    FIELDS_CSV: ("go_struct", "type_token", "field_name"),
    CONSTRAINTS_CSV: ("go_struct", "type_token", "kind", "members"),
}


class _MatrixHistory:
    """The citations each committed version of a matrix file carried."""

    def __init__(self, matrix_dir: Path) -> None:
        self.root: Path | None = None
        top = _git_or_none(matrix_dir, "rev-parse", "--show-toplevel")
        if top is None:
            return
        self.root = Path(top.strip())
        self.rel = matrix_dir.resolve().relative_to(self.root.resolve())
        self.revisions: dict[str, list[str]] = {}
        self.versions: dict[tuple[str, str], dict[tuple[str, ...], str] | None] = {}

    def _revisions(self, name: str) -> list[str]:
        if name not in self.revisions:
            out = _git(self.root, "log", "--format=%H", "--", str(self.rel / name))
            self.revisions[name] = out.split()
        return self.revisions[name]

    def _version(self, commit: str, name: str) -> dict[tuple[str, ...], str] | None:
        key = (commit, name)
        if key not in self.versions:
            text = _git_or_none(self.root, "show", f"{commit}:{self.rel / name}")
            cited: dict[tuple[str, ...], str] | None = None
            if text is not None:
                cited = {}
                column = "doc_ref" if name == TYPES_CSV else "evidence_ref"
                for record in csv.DictReader(io.StringIO(text)):
                    try:
                        ident = tuple(record[c] for c in _IDENTITY_COLUMNS[name])
                        cited[ident] = record[column]
                    except KeyError:
                        cited = None
                        break
            self.versions[key] = cited
        return self.versions[key]

    def citations(self, row: Row, ref: str) -> list[tuple[str, str]] | None:
        """The row's citations as its history wrote them, newest first: the current
        one with the commit that wrote it (WORKTREE when no commit carries it yet),
        then each one it replaced with the commit that wrote that, up to
        MAX_PRIOR_CITATIONS. None when the matrix is not in a git repository."""
        if self.root is None:
            return None
        name, ident, _ = _row_identity(row)
        found: list[tuple[str, str]] = []
        cited, since = ref, WORKTREE
        for commit in self._revisions(name):
            version = self._version(commit, name)
            then = None if version is None else version.get(ident)
            if then == cited:
                since = commit
                continue
            found.append((cited, since))
            if then in (None, NONE) or len(found) > MAX_PRIOR_CITATIONS:
                return found
            cited, since = then, commit
        found.append((cited, since))
        return found


# How many of the citations a row's current one replaced are tried, when the
# current one does not name what it cites at the commit that wrote it.
MAX_PRIOR_CITATIONS = 4


# ---------------------------------------------------------------------------
# The cited file's history
# ---------------------------------------------------------------------------

# A declaration at the top level of a source file, and the name it declares: Go's
# `type X`, `func X(` and `func (r *T) X(`, Python's `class X` and `def x`, and
# TypeScript's declarations.
_DECLARATION = re.compile(
    r"^(?:"
    r"type\s+(?P<gotype>\w+)"
    r"|func\s+(?:\([^)]*?\*?\s*(?P<recv>\w+)(?:\[[^\]]*\])?\)\s*)?(?P<gofunc>\w+)"
    r"|(?:async\s+)?(?:def|class)\s+(?P<py>\w+)"
    r"|(?:export\s+)?(?:default\s+)?(?:async\s+)?"
    r"(?:function|class|interface|type|enum|const|let)\s+(?P<ts>\w+)"
    r")"
)


def _declaration_name(line: str) -> str | None:
    m = _DECLARATION.match(line)
    if m is None:
        return None
    if m.group("gofunc"):
        return f"{m.group('recv')}.{m.group('gofunc')}" if m.group("recv") else m.group("gofunc")
    return m.group("gotype") or m.group("py") or m.group("ts")


def _enclosing(lines: list[str], index: int) -> tuple[str | None, int]:
    """The name of the top-level declaration holding lines[index] (0-based), and
    the index of its first line; (None, 0) for a line before any declaration."""
    for i in range(index, -1, -1):
        name = _declaration_name(lines[i])
        if name is not None:
            return name, i
    return None, 0


def _scope(lines: list[str], name: str | None) -> range | None:
    """The lines of the one top-level declaration called `name`, from its first
    line to the next declaration; None when there is not exactly one."""
    starts = [i for i, line in enumerate(lines) if _declaration_name(line) == name]
    if name is None or len(starts) != 1:
        return None
    start = starts[0]
    end = next(
        (i for i in range(start + 1, len(lines)) if _declaration_name(lines[i]) is not None),
        len(lines),
    )
    return range(start, end)


def _declares(row: Row, line: str) -> bool:
    """Whether `line` is the declaration of what the row cites: a struct field
    declared by its Go name or tagged with its json key, or the struct itself."""
    if isinstance(row, TypeRow):
        return _declaration_name(line) == row.go_struct
    if isinstance(row, FieldRow):
        return bool(
            re.match(rf"^\s*{re.escape(row.field_name)}\b(?!\s*[:(=.])", line)
            or re.search(rf'json:"{re.escape(row.json_key)}[",]', line)
        )
    return False


def _normalised(lines: list[str]) -> list[str]:
    return [" ".join(line.split()) for line in lines]


class _FileHistory:
    """The versions of one cited file, and the line mapping between them."""

    def __init__(self, code_root: Path, file_part: str) -> None:
        self.code_root = code_root
        self.file_part = file_part
        self.texts: dict[str, list[str] | None] = {}
        self.chains: dict[str, list[list[str]]] = {}
        self.opcodes: dict[tuple[int, int], list] = {}

    def at(self, commit: str) -> list[str] | None:
        if commit not in self.texts:
            text = _git_or_none(self.code_root, "show", f"{commit}:{self.file_part}")
            self.texts[commit] = None if text is None else text.splitlines()
        return self.texts[commit]

    def chain(self, start: str, current: list[str]) -> list[list[str]]:
        """Every distinct version of the file from `start` to the working tree, in
        order, starting with the version at `start`."""
        if start not in self.chains:
            out = _git(
                self.code_root, "log", "--reverse", "--topo-order", "--ancestry-path",
                "--format=%H", f"{start}..HEAD", "--", self.file_part,
            )
            versions: list[list[str]] = []
            for commit in [start, *out.split()]:
                text = self.at(commit)
                if text is not None and (not versions or text != versions[-1]):
                    versions.append(text)
            if not versions or current != versions[-1]:
                versions.append(current)
            self.chains[start] = versions
        return self.chains[start]

    def carry(self, old: list[str], new: list[str], index: int) -> int | None:
        """lines old[index] as an index into new, or None when the line is inside a
        changed block."""
        key = (id(old), id(new))
        if key not in self.opcodes:
            self.opcodes[key] = difflib.SequenceMatcher(
                None, _normalised(old), _normalised(new), autojunk=False
            ).get_opcodes()
        for tag, i1, i2, j1, _ in self.opcodes[key]:
            if i1 <= index < i2:
                return j1 + (index - i1) if tag == "equal" else None
        return None


def _search_declaration(row: Row, old: list[str], new: list[str], index: int) -> int | None:
    """Where the line old[index] went when it changed: the one line of the same
    enclosing declaration, as it reads in `new`, that declares the cited name, or
    failing that the one that names it."""
    name, _ = _enclosing(old, index)
    scope = _scope(new, name)
    if scope is None:
        return None
    for test in (_declares, cites):
        found = [i for i in scope if test(row, new[i])]
        if len(found) == 1:
            return found[0]
        if len(found) > 1:
            return None
    return None


def _follow(row: Row, history: _FileHistory, start: str, current: list[str], n: int) -> tuple[int | None, str]:
    """Line n (1-based) of the file at `start`, followed to the working tree.
    Returns (line, "") or (None, why not)."""
    chain = history.chain(start, current)
    origin = chain[0]
    if not (1 <= n <= len(origin)) or not cites(row, origin[n - 1]):
        return None, f"its line does not name it at {start[:8]}, which wrote it"
    anchor_name, anchor_start = _enclosing(origin, n - 1)
    if (
        isinstance(row, FieldRow)
        and _declares(row, origin[n - 1])
        and origin[anchor_start].startswith("type ")
        and anchor_name != row.declared_in
    ):
        # The substring rule accepted a namesake: the line declares a field of
        # another struct.
        return None, (
            f"its line declares a field of {anchor_name}, not of {row.declared_in}, "
            f"at {start[:8]}, which wrote it"
        )
    index = n - 1
    for old, new in zip(chain, chain[1:]):
        carried = history.carry(old, new, index)
        if carried is None:
            carried = _search_declaration(row, old, new, index)
            if carried is None:
                return None, (
                    f"the line changed after {start[:8]}, and no single line of "
                    f"{anchor_name or 'the file'} declares it after the change"
                )
        index = carried
    if not cites(row, current[index]):
        return None, f"carried from {start[:8]} to a line that does not name it"
    if _enclosing(current, index)[0] != anchor_name:
        return None, (
            f"carried from {start[:8]} out of {anchor_name or 'the file top'} into "
            f"{_enclosing(current, index)[0]}"
        )
    return index + 1, ""


# ---------------------------------------------------------------------------
# The command
# ---------------------------------------------------------------------------


def relocate(
    matrix: Matrix,
    code_root: Path,
    matrix_dir: Path,
    all_citations: bool = False,
) -> list[Move]:
    """Carry every citation into the JetStore repository that `--code` rejects, or
    with `all_citations` every one, from the commit that wrote it to the file as it
    is now. `matrix_dir` is where the matrix's files are, whose git history says
    which commit wrote each citation. Citations into `workspaces/` are outside the
    repository's history and are left alone.

    Without `all_citations` the result lists the rejected citations; with it, it
    lists those whose carried line differs from the cited one, and those that could
    not be carried."""
    moves: list[Move] = []
    current: dict[str, list[str] | None] = {}
    files: dict[str, _FileHistory] = {}
    matrix_history = _MatrixHistory(matrix_dir)
    code_top = _git_or_none(code_root, "rev-parse", "--show-toplevel")
    same_repo = (
        code_top is not None
        and matrix_history.root is not None
        and Path(code_top.strip()).resolve() == matrix_history.root.resolve()
    )

    rows: list[tuple[str, Row, str]] = [
        *((f"types {_key(t)}", t, t.doc_ref) for t in matrix.types),
        *((_key(r), r, r.evidence_ref) for r in [*matrix.fields_, *matrix.constraints]),
    ]

    for where, row, ref in rows:
        if ref in (NONE, None):
            continue
        file_part, _, line_part = ref.rpartition(":")
        if not file_part or not line_part.isdigit() or file_part.startswith(WORKSPACES_PREFIX):
            continue
        if file_part not in current:
            path = code_root / file_part
            current[file_part] = (
                path.read_text(encoding="utf-8").splitlines() if path.is_file() else None
            )
        lines = current[file_part]
        if lines is None:
            continue
        n = int(line_part)
        rejected = not (1 <= n <= len(lines) and cites(row, lines[n - 1]))
        if not rejected and not all_citations:
            continue
        reviewed = not isinstance(row, TypeRow) and row.review is Review.REVIEWED

        def stuck(reason: str) -> None:
            moves.append(Move(where, ref, None, reason, reviewed, row, rejected))

        lineage = matrix_history.citations(row, ref)
        if lineage is None:
            stuck("the matrix is not in a git repository, so nothing says which commit wrote the citation")
            continue
        if lineage[0][1] == WORKTREE:
            if rejected:
                stuck("written in the working tree, where its line does not name it")
            continue
        # The citation as written, then each one it replaced in the same file:
        # a citation carried onto a namesake is wrong at the commit that wrote
        # it, and the one it replaced may still be followed.
        first_reason = None
        result: tuple[int, str] | None = None
        for k, (cited, written) in enumerate(lineage):
            cited_file, _, cited_line = cited.rpartition(":")
            if written == WORKTREE or cited_file != file_part or not cited_line.isdigit():
                continue
            start = written
            if not same_repo:
                # The matrix and the code are separate repositories: the code as
                # it stood when the matrix commit was made.
                when = _git(matrix_history.root, "log", "-1", "--format=%cI", written).strip()
                start = _git(code_root, "log", "-1", f"--before={when}", "--format=%H", "HEAD").strip()
                if not start:
                    first_reason = first_reason or "no commit of the code predates the matrix commit that wrote it"
                    continue
            if file_part not in files:
                files[file_part] = _FileHistory(code_root, file_part)
            carried, why = _follow(row, files[file_part], start, lines, int(cited_line))
            if carried is None:
                first_reason = first_reason or why
                continue
            how = written[:8] if k == 0 else f"{written[:8]}, as {cited}, the citation it replaced"
            result = (carried, how)
            break
        if result is None:
            stuck(first_reason or "no citation in its history is into this file")
            continue
        carried, how = result
        if carried == n and not rejected:
            continue  # where its history says it went
        moves.append(Move(where, ref, f"{file_part}:{carried}", how, reviewed, row, rejected))
    return moves


def apply(moves: list[Move]) -> int:
    """Write every carried citation back onto its row; returns how many moved.
    `review` and `reviewed_hash` are left as they are."""
    moved = 0
    for m in moves:
        if m.new is None:
            continue
        if isinstance(m.row, TypeRow):
            m.row.doc_ref = m.new
        else:
            m.row.evidence_ref = m.new
        moved += 1
    return moved
