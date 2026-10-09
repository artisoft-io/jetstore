"""The B.18 field-inventory drift check (I-9: one command, exit code).

Reflects `pipes_model.go` through the Go runner in `inventory/` and compares
the field inventory - Go names, json keys, Go types, nesting (the type
strings carry it) - against the matrix's Go-binding columns, which the
Python model's claims are synced to. Fails on anything present in one and
absent from the other: the common case it exists for is a field added to Go
and forgotten in the contract. It does not check applicability - only the
matrix knows that (§5.2.2, criterion 8).
"""

from __future__ import annotations

import argparse
import json
import re
import subprocess
from collections import defaultdict
from pathlib import Path

from .matrix_schema import Matrix

# reflect.Type spellings vs the matrix's Go spellings.
_CANON = {
    "any": "interface {}",
    "map[string]any": "map[string]interface {}",
    "[]map[string]any": "[]map[string]interface {}",
    "rune": "int32",
    "byte": "uint8",
}

# The package whose alias declarations decide the matrix's unqualified spellings:
# `inventory/main.go`'s typeName strips `compute_pipes.` and nothing else, so an
# unqualified name in `go_type` is a name that resolves inside this package.
ALIAS_PACKAGE = Path("jets") / "compute_pipes"

# `type Local = pkg.Name`, standalone or inside a `type ( ... )` block. Only the
# alias form (`=`) is read: `type Local pkg.Name` declares a *distinct* type,
# which reflection reports as `compute_pipes.Local` anyway and which nothing here
# needs to rewrite.
_ALIAS_DECL = re.compile(r"^([A-Z]\w*)\s*=\s*([a-z]\w*)\.([A-Z]\w*)\s*(?://.*)?$")


def alias_map(code_root: Path) -> dict[str, str]:
    """Qualified spelling -> the local alias, read from the Go source.

    **Reflection cannot see an alias, and that is the whole reason this exists.**
    A Go type alias is not a distinct type at runtime, so `reflect.Type` reports
    the *declaring* package: after `BC.2` split the operator contract into
    `jets/compute_pipes/pipesmodel`, the runner says `pipesmodel.ExpressionNode`
    while `pipes_model.go` still declares the field as `*ExpressionNode`, through
    `type ExpressionNode = pipesmodel.ExpressionNode`. Both spellings are correct
    about different things, and 34 of the 35 differences the check reported on
    2026-09-12 were exactly this and nothing else - stripping the qualifier left
    zero residue on all 34.

    **It is derived rather than listed, and the difference is what happens when
    an alias goes away.** A hard-coded set would keep normalising a qualifier
    after the alias that justified it had been deleted, leaving the check silent
    while the matrix's spelling no longer matched any name the package has.
    Reading the declarations means the normalisation lasts exactly as long as the
    aliases do, and a genuinely different package - one nothing aliases in - stays
    qualified on the reflected side and still drifts. That is the property worth
    protecting: this must not become "strip any package qualifier".
    """
    aliases: dict[str, str] = {}
    directory = code_root / ALIAS_PACKAGE
    in_block = False
    for path in sorted(directory.glob("*.go")):
        if path.name.endswith("_test.go"):
            continue
        for line in path.read_text().splitlines():
            stripped = line.strip()
            if in_block:
                if stripped == ")":
                    in_block = False
                    continue
            elif stripped == "type (":
                in_block = True
                continue
            elif stripped.startswith("type "):
                stripped = stripped[len("type "):].strip()
            else:
                continue
            match = _ALIAS_DECL.match(stripped)
            if match is not None:
                local, package, name = match.groups()
                aliases[f"{package}.{name}"] = local
    return aliases


def canon(go_type: str, aliases: dict[str, str]) -> str:
    """One spelling for one type, so the two sides can be compared.

    `_CANON` settles the spellings of the same builtin (`any` / `interface {}`);
    `aliases` settles a qualified name against the local alias it is reachable by.
    Applied to *both* sides, so the matrix may record either spelling of an
    aliased type - the same latitude `_CANON` already gives `any`.
    """
    go_type = _CANON.get(go_type, go_type)
    for qualified, local in aliases.items():
        # Bounded by a word boundary on the right so `pipesmodel.Map` cannot
        # rewrite the head of `pipesmodel.MapExpression`.
        go_type = re.sub(rf"\b{re.escape(qualified)}\b", local, go_type)
    return go_type


def run(args: argparse.Namespace, matrix: Matrix) -> int:
    proc = subprocess.run(
        ["go", "run", "./tools/cpipes_contract/inventory"],
        capture_output=True,
        text=True,
        cwd=args.code,
    )
    if proc.returncode != 0:
        print(f"go inventory runner failed: {proc.stderr.strip()}")
        return 2
    reflected: dict[str, dict[str, dict]] = {
        struct: {f["json"]: f for f in fields}
        for struct, fields in json.loads(proc.stdout).items()
    }
    aliases = alias_map(Path(args.code))

    recorded: dict[str, dict[str, tuple[str, str]]] = defaultdict(dict)
    for f in matrix.fields_:
        recorded[f.go_struct][f.json_key] = (f.field_name, f.go_type)

    drifts: list[str] = []
    for struct in sorted(recorded):
        got = reflected.get(struct)
        if got is None:
            drifts.append(f"{struct}: in the matrix, not reachable in pipes_model.go")
            continue
        for key in sorted(set(recorded[struct]) - set(got)):
            drifts.append(f"{struct}.{key}: in the matrix, not in pipes_model.go")
        for key in sorted(set(got) - set(recorded[struct])):
            drifts.append(f"{struct}.{key}: in pipes_model.go, not in the matrix")
        for key in sorted(set(got) & set(recorded[struct])):
            name, go_type = recorded[struct][key]
            if got[key]["name"] != name:
                drifts.append(
                    f"{struct}.{key}: Go name {got[key]['name']!r} vs matrix {name!r}"
                )
            if canon(go_type, aliases) != canon(got[key]["type"], aliases):
                drifts.append(
                    f"{struct}.{key}: Go type {got[key]['type']!r} vs matrix {go_type!r}"
                )

    if drifts:
        print(f"drift: {len(drifts)} difference(s) between pipes_model.go and the matrix:")
        for d in drifts:
            print(" ", d)
        return 1
    n_structs = len(recorded)
    n_fields = sum(len(v) for v in recorded.values())
    print(f"drift: clean - {n_structs} structs, {n_fields} field bindings agree")
    return 0
