"""Emit the cpipes JSON Schema: one document, every type in `$defs` (B.11).

Every addressable type of the matrix — each (go_struct, token) class, the
merged classes, and the discriminated-union aliases — lands as an independently
addressable `$defs` entry. That is a hard requirement, not a convenience: the
plan's §5.3 typed holes bind to exactly one `$defs` entry
(`{"$ref": "#/$defs/TransformationSpec"}` and the like) and have no other way
to constrain what fills them. The document root is the full config schema via
`$ref` to `#/$defs/ComputePipesConfig`, so the same file validates whole
documents and fragments alike.

The discriminated unions emit as `oneOf` + `discriminator`; the engine-default
variants (`memory`, `standard`, `anonymization`) are the only ones whose tag is
optional, which keeps an untagged instance matching exactly one branch — JSON
Schema has no equivalent of the model's tag-injecting BeforeValidator.
"""

from __future__ import annotations

import argparse
import csv
import json
from pathlib import Path

from pydantic import TypeAdapter
from pydantic.json_schema import models_json_schema

from .reflect import MERGED, load_model

REF_TEMPLATE = "#/$defs/{model}"


def emit(module, types_csv: Path) -> dict:
    models = [(m, "validation") for m in module._MODELS]
    _, definitions = models_json_schema(models, ref_template=REF_TEMPLATE)
    defs: dict[str, dict] = dict(definitions.get("$defs", {}))

    # The union aliases, each as its own named entry referencing its members.
    union_structs = sorted(
        {struct for _, (struct, _) in module._MATRIX_KEYS.items()}
        - {cname for cname in module._MATRIX_KEYS}
    )
    for struct in union_structs:
        adapter = TypeAdapter(getattr(module, struct))
        schema = adapter.json_schema(ref_template=REF_TEMPLATE)
        for name, entry in schema.pop("$defs", {}).items():
            if name in defs and defs[name] != entry:
                raise ValueError(f"$defs collision on {name}")
            defs.setdefault(name, entry)
        defs[struct] = schema

    # Every matrix type must be independently addressable.
    missing = []
    with open(types_csv, newline="") as fh:
        for row in csv.DictReader(fh):
            struct, token = row["go_struct"], row["type_token"]
            if struct in MERGED or token == "*":
                expected = struct
            else:
                cname = next(
                    (c for c, k in module._MATRIX_KEYS.items() if tuple(k) == (struct, token)),
                    None,
                )
                expected = cname or struct
            if expected not in defs:
                missing.append(f"{struct}/{token} -> {expected}")
    if missing:
        raise ValueError(f"types not addressable in $defs: {missing}")

    return {
        "$schema": "https://json-schema.org/draft/2020-12/schema",
        "$ref": "#/$defs/ComputePipesConfig",
        "$defs": {k: defs[k] for k in sorted(defs)},
    }


def run(args: argparse.Namespace) -> int:
    module = load_model(args.model)
    document = emit(module, args.matrix / "types.csv")
    args.out.write_text(json.dumps(document, indent=2, sort_keys=True) + "\n")
    print(f"wrote {args.out}: {len(document['$defs'])} $defs entries")
    return 0
