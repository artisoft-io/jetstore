"""The compile and round-trip check (A1.8, A1.9) — one command, exit code.

Builds a throwaway workspace (the platform base model from the test workspace
plus the freshly emitted jets_agentic.jr), compiles it under compilerv2, and
asserts against the produced workspace.db:

- all ten entity classes are in `domain_classes` (A1.8);
- every vocabulary value is in `resources` as a *named* text literal —
  `type='text'`, `id` set, `inline` null (the §4 verification shape);
- the round trip holds: the classes and properties read back from the
  compiled model equal the projection of the Python source, with the one
  documented asymmetry — Evidence appears as a class plus object properties
  where the source declares a nested value object (§4).

Note on F5: `build/classes.json` — the workspace-wide view — is written by
the *full* workspace compile (`jets/workspace/compile_workspace_v2.go`).
An earlier version of this note said that compile needs Postgres; item 3
disproved that — `compile_workspace` runs with a nil dbpool (Postgres is
only for the upload), and the item-3 test fixture compiles a workspace
offline. This check still reads `workspace.db` directly, which carries the
same class model and needs no per-run workspace copy of the platform model;
the classes.json comparison lives in the item-3 tool tests, which resolve a
fully compiled fixture.

Beware F4: a full compile deletes workspace.db first, and re-saving a main
source file already present is an error — hence the throwaway directory per
run.
"""

from __future__ import annotations

import argparse
import shutil
import sqlite3
import subprocess
import tempfile
from enum import StrEnum
from pathlib import Path

from pydantic import BaseModel

from . import jr
from . import model as M
from .main import REPO_ROOT

MAIN_JR = """# Throwaway main file of the jets_agentic compile check (A1.8).
import "data_model/jets_model.jr"
import "data_model/jets_agentic.jr"
"""


def expected_classes() -> set[str]:
    return {jr.qname(e.__name__) for e in M.ENTITIES}


def expected_literals() -> dict[str, str]:
    out: dict[str, str] = {}
    for cls, _ in jr.collect_vocabularies():
        for member in cls:
            out[jr.qname(jr.snake_upper(cls.__name__) + "_" + member.name.upper())] = member.value
    return out


def expected_properties() -> set[tuple[str, str, str, int]]:
    """(class, property, type, as_array) as the compiled model will carry them:
    data properties with their .jr type, object properties as `resource`."""
    out: set[tuple[str, str, str, int]] = set()
    for entity in M.ENTITIES:
        cname = jr.qname(entity.__name__)
        for fname, field in entity.model_fields.items():
            jr_type, is_array, cls = jr.analyze(field.annotation)
            if cls is not None and issubclass(cls, BaseModel):
                out.add((cname, jr.qname(fname), "resource", int(is_array)))
            else:
                out.add((cname, jr.qname(fname), jr_type, int(is_array)))
    return out


def run(args: argparse.Namespace) -> int:
    tmp = Path(tempfile.mkdtemp(prefix="jets_agentic_ws_"))
    problems: list[str] = []
    try:
        (tmp / "data_model").mkdir()
        (tmp / "jet_rules").mkdir()
        shutil.copy(
            REPO_ROOT / "jets" / "jetrules" / "test_ws" / "data_model" / "jets_model.jr",
            tmp / "data_model" / "jets_model.jr",
        )
        (tmp / "data_model" / "jets_agentic.jr").write_text(jr.emit())
        (tmp / "jet_rules" / "jets_agentic_main.jr").write_text(MAIN_JR)

        proc = subprocess.run(
            ["go", "run", "./jets/compilerv2",
             "-base_path", str(tmp), "-in_file", "jet_rules/jets_agentic_main.jr",
             "-s"],
            capture_output=True, text=True, cwd=REPO_ROOT,
        )
        if proc.returncode != 0:
            print(proc.stdout[-3000:])
            print(proc.stderr[-3000:])
            print("compile: FAILED")
            return 1
        db_path = tmp / "workspace.db"
        if not db_path.exists():
            print("compile succeeded but no workspace.db was written")
            return 1
        db = sqlite3.connect(db_path)

        # A1.8: the ten classes
        got_classes = {r[0] for r in db.execute("select name from domain_classes")}
        missing = expected_classes() - got_classes
        if missing:
            problems.append(f"classes missing from domain_classes: {sorted(missing)}")

        # A1.8: every vocabulary value as a named text literal
        got_literals = {
            r[0]: r[1]
            for r in db.execute(
                "select id, value from resources "
                "where type='text' and id is not null and inline is null "
                f"and id like '{M.PREFIX}:%'"
            )
        }
        for name, value in expected_literals().items():
            if got_literals.get(name) != value:
                problems.append(
                    f"vocabulary literal {name} = {value!r}: "
                    f"got {got_literals.get(name)!r}"
                )

        # A1.9: the round trip - properties read back vs the source projection
        got_props: set[tuple[str, str, str, int]] = set()
        for cname, pname, ptype, as_array in db.execute(
            """select dc.name, dp.name, dp.type, dp.as_array
               from data_properties dp join domain_classes dc on dp.domain_class_key = dc.key
               where dp.name like ?""",
            (f"{M.PREFIX}:%",),
        ):
            got_props.add((cname, pname, ptype, int(as_array)))
        for cname, pname, ptype, as_array in db.execute(
            """select dc.name, op.name, 'resource', op.as_array
               from object_properties op join domain_classes dc on op.domain_class_key = dc.key
               where op.name like ?""",
            (f"{M.PREFIX}:%",),
        ):
            got_props.add((cname, pname, ptype, int(as_array)))
        want = expected_properties()
        for item in sorted(want - got_props):
            problems.append(f"in the source, not in the compiled model: {item}")
        for item in sorted(got_props - want):
            problems.append(f"in the compiled model, not in the source: {item}")

        db.close()
    finally:
        if getattr(args, "keep", False):
            print(f"workspace kept at {tmp}")
        else:
            shutil.rmtree(tmp, ignore_errors=True)

    if problems:
        print(f"compile check: {len(problems)} problem(s):")
        for p in problems:
            print(" ", p)
        return 1
    print(
        f"compile check: clean - {len(expected_classes())} classes, "
        f"{len(expected_literals())} vocabulary literals, "
        f"{len(expected_properties())} properties round-trip"
    )
    return 0
