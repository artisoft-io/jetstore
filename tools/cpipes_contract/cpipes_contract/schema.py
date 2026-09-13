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

from .bundles import emit as emit_bundles
from .reflect import MERGED, load_model

REF_TEMPLATE = "#/$defs/{model}"


def splice_complement_branches(defs: dict, types_csv: Path) -> None:
    """Add each `unlisted(...)` token to its union's `oneOf`, everywhere it occurs.

    **Pydantic cannot emit this branch and cannot be made to.**
    `Field(discriminator="type")` builds a `oneOf` of literal-tagged members with
    a `discriminator.mapping` keyed by those literals; a variant whose token is
    *any string but* those has no literal to be keyed by. So the branch is
    spliced here, from `types.csv`, which is where the token is recorded.

    **It is every occurrence rather than the named entry, and that distinction
    cost an hour.** `TransformationSpec` is an `Annotated[Union[...]]` alias
    rather than a model, so Pydantic *inlines* it at each use site - the three
    `PipeSpec*.apply` arrays carry their own copy of the whole `oneOf`, and
    `$defs/TransformationSpec` is a separate entry this module builds for
    addressability. Splicing the named entry alone leaves a schema where
    `#/$defs/TransformationSpec` admits a site operator and no document
    containing one validates, which is the worst of both: the addressable entry
    a typed hole binds says yes and the corpus gate says no.

    **Matching is on the exact member list, so a narrowed union is left alone.**
    A bundle is a deliberate restriction of what a hole may offer
    (`bundles.py`), and a subset never equals the full list. Whether a site
    operator should be offered at a *template hole* is a separate decision with
    a separate owner, and this must not take it by accident.
    """
    with open(types_csv, newline="") as fh:
        complements = [
            row
            for row in csv.DictReader(fh)
            if row["variant_when"].startswith("unlisted(")
        ]
    for row in complements:
        union = defs.get(row["go_struct"])
        if union is None or "oneOf" not in union:
            raise ValueError(
                f"{row['go_struct']}/{row['type_token']} is a complement token of a "
                f"$defs entry that is not a oneOf union"
            )
        branch = {"$ref": REF_TEMPLATE.format(model=row["defs_name"])}
        signature = json.dumps(union["oneOf"], sort_keys=True)
        targets: list[dict] = []

        def find(node) -> None:
            if isinstance(node, dict):
                one_of = node.get("oneOf")
                if isinstance(one_of, list) and json.dumps(one_of, sort_keys=True) == signature:
                    targets.append(node)
                for value in node.values():
                    find(value)
            elif isinstance(node, list):
                for value in node:
                    find(value)

        find(defs)
        if not targets:
            raise ValueError(
                f"{row['go_struct']}/{row['type_token']}: no occurrence of the "
                f"{row['go_struct']} union to splice into"
            )
        for node in targets:
            node["oneOf"] = [*node["oneOf"], branch]


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

    # Every matrix type must be independently addressable, **under the name the matrix
    # records for it**. Checking the model's own class name here would only prove the
    # emitter self-consistent; it is `defs_name` that the fragment library, the bundle
    # layer and every typed hole key off, so that is the column to hold to the schema.
    # Comparing the two is the check whose absence let `defs_name` diverge from the
    # emitted keys for 68 of 127 types until 2026-08-20.
    missing = []
    with open(types_csv, newline="") as fh:
        for row in csv.DictReader(fh):
            struct, token, name = row["go_struct"], row["type_token"], row["defs_name"]
            if name not in defs:
                missing.append(f"{struct}/{token} -> defs_name {name!r}")
    if missing:
        raise ValueError(f"defs_name values with no $defs entry: {missing}")

    # The bundle layer: abstract types between the root unions and their leaves,
    # with every reachable column list ranged. See bundles.py for why this is a
    # projection over $defs rather than classes in the model.
    emit_bundles(defs, types_csv.parent)

    splice_complement_branches(defs, types_csv)

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
