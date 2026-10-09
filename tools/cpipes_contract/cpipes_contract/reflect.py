"""Regenerate the matrix's claim columns from `cpipes_model.py` — the B.10 flip.

After B.9 the Python model is the source of truth for the *contract claims*:
which fields exist per token (`applicable`), which are required, their value
ranges, defaults, and descriptions. This module reflects the imported model and
rewrites exactly those columns of `fields.csv`, carrying everything else — the
audit trail (evidence, citations, corpus counts, harness verdicts, review
stamps, notes) and the Go binding (go_type, container, ref_struct, declared_in)
— from the existing rows, keyed by (go_struct, type_token, json_key).

`--check` compares instead of writing and exits nonzero on any difference: the
divergence guard the plan's §5.2.2 asks for. The regeneration is a no-op when
model and matrix agree, so `reflect --check` belongs wherever the matrix is
consumed.

What the model cannot express stays CSV-authoritative and is carried over
unchanged: the merged classes (`ExpressionNode`, `ContextSpec` — their
per-token claims are structural, weakened in the merge), rows for fields
inapplicable on every token of their struct (a Go-binding fact; B.18's drift
check owns it), and `constraints.csv` entirely.
"""

from __future__ import annotations

import argparse
import csv
import importlib.util
import re
import typing
from collections import defaultdict
from pathlib import Path

from pydantic import BaseModel

# Structs whose per-token claims the merged classes cannot carry; their rows
# stay CSV-authoritative. Mirrors generate.MERGED.
MERGED = {"ExpressionNode", "ContextSpec"}

_DEFAULT_RE = re.compile(r" Engine default: (.+) \((builder|validator|none)\)\.$")
_REQUIRED_WHEN_RE = re.compile(r" Required when: (.+)\.$")

CLAIM_COLUMNS = ("applicable", "required", "required_when", "values", "default", "default_by", "description", "deprecated")


def load_model(path: Path):
    spec = importlib.util.spec_from_file_location("cpipes_model", path)
    module = importlib.util.module_from_spec(spec)
    spec.loader.exec_module(module)
    return module


def literal_args(annotation) -> list[str] | None:
    if typing.get_origin(annotation) is typing.Literal:
        return [str(a) for a in typing.get_args(annotation)]
    return None


def unwrap_optional(annotation):
    """str | None -> str; leaves everything else alone."""
    if typing.get_origin(annotation) in (typing.Union, __import__("types").UnionType):
        args = [a for a in typing.get_args(annotation) if a is not type(None)]
        if len(args) == 1:
            return args[0]
    return annotation


class Claims:
    """Per (struct, token, json_key): the claim cells the model expresses."""

    def __init__(self, module, discriminators: dict[str, str]) -> None:
        self.discriminators = discriminators
        self.rows: dict[tuple[str, str, str], dict[str, str]] = {}
        self.universe: dict[str, set[str]] = defaultdict(set)
        by_key: dict[tuple[str, str], type[BaseModel]] = {}
        for cname, key in module._MATRIX_KEYS.items():
            by_key[tuple(key)] = getattr(module, cname)
        for (struct, token), cls in by_key.items():
            if struct in MERGED:
                continue
            for name in cls.model_fields:
                self.universe[struct].add(name)
        for (struct, token), cls in by_key.items():
            if struct in MERGED:
                continue
            present = set(cls.model_fields)
            for name, fi in cls.model_fields.items():
                is_disc = name == self.discriminators.get(struct)
                self.rows[(struct, token, name)] = self.field_claims(fi, is_disc)
            for name in self.universe[struct] - present:
                self.rows[(struct, token, name)] = {
                    "applicable": "no",
                    "required": "na",
                    "required_when": "-",
                }

    def field_claims(self, fi, is_disc: bool = False) -> dict[str, str]:
        desc = fi.description or ""
        deprecated = "no"
        if desc.startswith("DEPRECATED. "):
            deprecated = "yes"
            desc = desc[len("DEPRECATED. "):]
        default, default_by = "-", "none"
        m = _DEFAULT_RE.search(desc)
        if m:
            default, default_by = m.group(1), m.group(2)
            desc = desc[: m.start()]
        required_when = "-"
        m = _REQUIRED_WHEN_RE.search(desc)
        if m:
            required_when = m.group(1)
            desc = desc[: m.start()]
        ann = unwrap_optional(fi.annotation)
        lits = literal_args(ann)
        if is_disc:
            # the discriminator: its value set is the types table
            values = "-"
        elif lits is not None:
            values = "|".join(lits)
        else:
            values = "-"
        if fi.is_required():
            required = "yes"
        elif required_when != "-":
            required = "conditional"
        else:
            required = "no"
        return {
            "applicable": "yes",
            "required": required,
            "required_when": required_when,
            "values": values,
            "default": default,
            "default_by": default_by,
            "description": desc.strip(),
            "deprecated": deprecated,
        }


def run(args: argparse.Namespace) -> int:
    module = load_model(args.model)
    discriminators: dict[str, str] = {}
    with open(args.matrix / "types.csv", newline="") as fh:
        for trow in csv.DictReader(fh):
            if trow["discriminator"] != "-":
                discriminators[trow["go_struct"]] = trow["discriminator"]
    claims = Claims(module, discriminators)
    path = args.matrix / "fields.csv"
    with open(path, newline="") as fh:
        reader = csv.reader(fh)
        header = next(reader)
        rows = list(reader)
    idx = {c: header.index(c) for c in header}

    diffs: list[str] = []
    seen: set[tuple[str, str, str]] = set()
    for row in rows:
        key = (row[idx["go_struct"]], row[idx["type_token"]], row[idx["json_key"]])
        seen.add(key)
        claimed = claims.rows.get(key)
        if claimed is None:
            continue  # merged struct, or a field the model does not carry
        for col, want in claimed.items():
            have = row[idx[col]]
            if have != want:
                diffs.append(f"{key[0]}/{key[1]}.{key[2]} {col}: {have!r} -> {want!r}")
                row[idx[col]] = want

    missing = sorted(set(claims.rows) - seen)
    for key in missing:
        diffs.append(f"{key[0]}/{key[1]}.{key[2]}: in the model, not in the matrix")

    if args.check:
        if diffs:
            print(f"reflect --check: {len(diffs)} divergence(s) between model and matrix:")
            for d in diffs[:40]:
                print(" ", d)
            return 1
        print("reflect --check: model and matrix agree")
        return 0

    if missing:
        print(f"reflect: {len(missing)} model fields have no matrix row; add rows for:")
        for key in missing[:20]:
            print(f"  {key[0]}/{key[1]}.{key[2]}")
        return 1
    with open(path, "w", newline="") as fh:
        writer = csv.writer(fh, lineterminator="\n")
        writer.writerow(header)
        writer.writerows(rows)
    print(f"reflect: {len(diffs)} cell(s) updated from the model" if diffs else "reflect: no changes")
    return 0
