"""The name-collision pre-check (A1.11) — run before the first install.

F7 makes workspace names one flat namespace, and the compiler's own
enforcement (`data_properties.name` is globally unique in workspace.db) fails
without naming what collided (I-10). This check runs *before* A21.1's install
step, against the target workspace's compiled `workspace.db`, and fails with
the colliding names: every emitted class name against `domain_classes`, every
emitted property name against `data_properties` and `object_properties`.

Because every emitted name carries the reserved `jetsa:` prefix, a collision
means the target is already using the prefix — either a client squatting on
it, or a previous install of this model. Both fail here: the pre-check's
contract is a workspace that does not yet contain the model, and on
*re*-install it is A21.3's file-level guard that governs, not this check.
"""

from __future__ import annotations

import argparse
import sqlite3
from pathlib import Path

from . import model as M
from .compile_check import expected_classes, expected_properties

# The three name tables F7 makes one flat workspace-wide namespace.
TABLES = ("domain_classes", "data_properties", "object_properties")


def run(args: argparse.Namespace) -> int:
    db_path = Path(args.workspace_db)
    if db_path.is_dir():
        db_path = db_path / "workspace.db"
    if not db_path.exists():
        print(f"precheck: no workspace.db at {db_path}")
        return 2
    db = sqlite3.connect(db_path)

    # A workspace compiled by an older schema can be missing one of the three
    # tables entirely - object_properties is the one seen in practice, on a
    # workspace.db that predates it. Skipping it beats crashing: the point of
    # this command is to report names, and a traceback reports nothing while
    # looking like a defect in the tool rather than a fact about the target.
    # A missing table also cannot hold a collision, so skipping loses nothing.
    present = {
        row[0]
        for row in db.execute("select name from sqlite_master where type = 'table'")
    }
    missing = [t for t in TABLES if t not in present]

    ours_classes = expected_classes()
    ours_props = {pname for _, pname, _, _ in expected_properties()}

    collisions: list[str] = []
    for table, names in (
        ("domain_classes", ours_classes),
        ("data_properties", ours_props),
        ("object_properties", ours_props),
    ):
        if table in missing:
            continue
        placeholders = ",".join("?" * len(names))
        for (name,) in db.execute(
            f"select name from {table} where name in ({placeholders})",
            sorted(names),
        ):
            collisions.append(f"{table}: {name}")

    # The prefix is the actual defence, so squatting on it is a collision even
    # when the squatter's name matches nothing we emit today — it would
    # collide with a name we are free to emit tomorrow.
    for table in TABLES:
        if table in missing:
            continue
        for (name,) in db.execute(
            f"select name from {table} where name like ?", (f"{M.PREFIX}:%",)
        ):
            entry = f"{table}: {name}"
            if entry not in collisions:
                collisions.append(f"{entry} (reserved `{M.PREFIX}:` prefix in target)")
    db.close()

    if collisions:
        print(f"precheck: {len(collisions)} colliding name(s) in {db_path}:")
        for c in sorted(collisions):
            print(" ", c)
        return 1
    if missing:
        # Said out loud, because a skipped table makes this a narrower check
        # and a reader should know the answer is "clean over what could be
        # read" rather than "clean".
        print(
            f"precheck: note - {db_path} has no {', '.join(missing)} table, "
            "so those names were not checked"
        )
    print(
        f"precheck: clean - {len(ours_classes)} classes and "
        f"{len(ours_props)} properties collide with nothing in {db_path}"
    )
    return 0
