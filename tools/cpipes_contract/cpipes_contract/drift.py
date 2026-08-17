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
import subprocess
from collections import defaultdict

from .matrix_schema import Matrix

# reflect.Type spellings vs the matrix's Go spellings.
_CANON = {
    "any": "interface {}",
    "map[string]any": "map[string]interface {}",
    "[]map[string]any": "[]map[string]interface {}",
    "rune": "int32",
    "byte": "uint8",
}


def canon(go_type: str) -> str:
    return _CANON.get(go_type, go_type)


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
            if canon(go_type) != canon(got[key]["type"]):
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
