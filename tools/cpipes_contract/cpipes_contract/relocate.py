"""Carry a citation whose line moved to where the line went.

`check --code` refuses a citation whose line no longer names its field. Most such
failures are not wrong claims but moved lines: an import, a field or a comment added
above the cited line shifts everything below it, and every citation into the file is
off by the same few lines. Re-deriving each one by eye is what produced line numbers
inferred from the shape of a file, which is the failure `--code` exists to catch.

So the repair is read, not guessed. For each citation `--code` rejects:

1. Walk the cited file's git history from the newest commit back, and take the newest
   version in which the cited line **did** name the field: that is the version the
   citation was written against, or the last one it was still true of.
2. Diff that version against the file as it is now, with whitespace normalised, and
   carry the line through the diff. A line inside a changed block is not carried:
   the line itself changed, and where it went is a judgment, not a mapping.
3. Keep the result only when the carried line names the field by `--code`'s own rule.

What it never does is touch `review` or `reviewed_hash`. A citation is part of what a
review certifies, so a reviewed row whose `evidence_ref` moves reads "changed since it
was reviewed" until a reviewer re-approves it with `stamp --restamp`. `types.csv`
carries no stamp, so a `doc_ref` moved here is an ordinary edit.
"""

from __future__ import annotations

import difflib
import subprocess
from dataclasses import dataclass, field
from pathlib import Path

from .check import WORKSPACES_PREFIX, _key, cites
from .matrix_schema import NONE, ConstraintRow, FieldRow, Matrix, Review, TypeRow

# How far back the walk goes before it gives up on a citation. A citation that has
# not been true in this many revisions of its file is not a moved line.
MAX_REVISIONS = 200


@dataclass
class Move:
    where: str  # the row, as `check` names it
    old: str  # file:line
    new: str | None  # file:line, or None when the line could not be carried
    reason: str  # why not, when new is None; the anchor commit otherwise
    reviewed: bool  # a stamped row this move lapses
    row: TypeRow | FieldRow | ConstraintRow = field(repr=False)


def _git(code_root: Path, *args: str) -> str:
    return subprocess.run(
        ["git", "-C", str(code_root), *args],
        check=True,
        capture_output=True,
        text=True,
    ).stdout


def _revisions(code_root: Path, file_part: str) -> list[str]:
    out = _git(
        code_root, "log", f"--max-count={MAX_REVISIONS}", "--format=%H", "--", file_part
    )
    return out.split()


def _blob(code_root: Path, commit: str, file_part: str) -> list[str] | None:
    try:
        return _git(code_root, "show", f"{commit}:{file_part}").splitlines()
    except subprocess.CalledProcessError:
        return None


def _carry(old: list[str], new: list[str], n: int) -> int | None:
    """Line n (1-based) of `old`, as a line number of `new`, or None."""
    a = [" ".join(line.split()) for line in old]
    b = [" ".join(line.split()) for line in new]
    for tag, i1, i2, j1, _ in difflib.SequenceMatcher(None, a, b, autojunk=False).get_opcodes():
        if i1 <= n - 1 < i2:
            return j1 + (n - 1 - i1) + 1 if tag == "equal" else None
    return None


def relocate(matrix: Matrix, code_root: Path) -> list[Move]:
    """Find a new line for every citation into the JetStore repository that `--code`
    rejects because its line no longer names the field. Citations into `workspaces/`
    are outside the repository's history and are left alone."""
    moves: list[Move] = []
    current: dict[str, list[str] | None] = {}
    history: dict[str, list[str]] = {}
    blobs: dict[tuple[str, str], list[str] | None] = {}

    rows: list[tuple[str, TypeRow | FieldRow | ConstraintRow, str]] = [
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
        if 1 <= n <= len(lines) and cites(row, lines[n - 1]):
            continue
        reviewed = (
            not isinstance(row, TypeRow) and row.review is Review.REVIEWED
        )

        if file_part not in history:
            history[file_part] = _revisions(code_root, file_part)
        anchor = None
        for commit in history[file_part]:
            key = (commit, file_part)
            if key not in blobs:
                blobs[key] = _blob(code_root, commit, file_part)
            old = blobs[key]
            if old is not None and 1 <= n <= len(old) and cites(row, old[n - 1]):
                anchor = commit
                break
        if anchor is None:
            moves.append(Move(where, ref, None, "no revision where this line names it", reviewed, row))
            continue
        carried = _carry(blobs[(anchor, file_part)], lines, n)
        if carried is None:
            moves.append(
                Move(where, ref, None, f"the line changed after {anchor[:8]}", reviewed, row)
            )
            continue
        if not cites(row, lines[carried - 1]):
            moves.append(
                Move(
                    where, ref, None,
                    f"carried from {anchor[:8]} to a line that does not name it",
                    reviewed, row,
                )
            )
            continue
        moves.append(Move(where, ref, f"{file_part}:{carried}", anchor[:8], reviewed, row))
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
