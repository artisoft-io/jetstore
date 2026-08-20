"""The bundle layer: abstract types between a root union and its concrete leaves.

**Why the layer exists — corrected 2026-08-20 (Q-15).** As built, the argument
was fit: a schema addressed at `#/$defs/TransformationSpec` is 41,040 tokens
against an infer-server context of 32,768. That premise was measured against the
wrong quantity. The JSON Schema goes to Ollama as `format`, which becomes a
sampling grammar rather than prompt text, and costs 2-3 tokens regardless of its
size; the prompt is what occupies the window, and there the flat union is ~9,090
tokens and fits. **Nothing here was unnecessary, but the stated reason was
wrong**, and the honest one is the one the proposal gave first: a hole should
offer the operators its host can semantically hold, not all fifteen.

The mechanism below is unchanged and still required for that, because the
narrowing is what does not work naively. A union of *one* operator still measures
28,700 as a schema - and roughly its whole union as a prompt - because every leaf
carries `conditional_config` -> `ConditionalTransformationSpec` ->
`TransformationSpecOverride`, which re-admits every operator's config. The
closure of a cycle does not care how the entry points are partitioned.

So the fix is two changes that only work together: an abstract type per bundle,
**and** a range on every place a `TransformationColumnSpec` or a
`TransformationSpec` is reachable. There are four such places, not one:

    TransformationSpec*.columns                     an operator's payload
    CaseExpression.then                             a case leg
    TransformationColumnSpecCase.else_expr          a case fallthrough
    TransformationColumnSpecMapReduce.apply_map     the map phase
    TransformationColumnSpecMapReduce.apply_reduce  the reduce phase

`case` and `map_reduce` are therefore **nesting** operators, and each needs a
variant per bundle rather than a range applied to one shared type: one entry
cannot range differently per host, and the hosts genuinely differ. The corpus
is unambiguous on the last of these - `apply_reduce` holds `count` and nothing
else across twelve instances, while `apply_map` holds distinct_count, sum and
count.

**Why this is a projection over the emitted `$defs` rather than classes in the
model.** Since the B.10 flip, `cpipes_model.py` is the source of truth for the
contract *claims* - which fields exist per token, required, values, defaults.
Bundles claim nothing new about any of that; they restrict what an existing
field admits, for one addressable entry point. Emitting them here keeps
`_MATRIX_KEYS` and the reflect direction untouched (every bundle leaf still maps
to its own `(go_struct, token)` pair, and the abstract nodes have no Go struct
behind them at all), and it keeps the model validating exactly what JetStore
accepts. A config that fills a bundle-shaped hole validates against the ordinary
model afterwards; the bundle constrains *generation*, not acceptance.
"""

from __future__ import annotations

import copy
import csv
from pathlib import Path

REF = "#/$defs/{}"


def _camel(token: str) -> str:
    return "".join(p[:1].upper() + p[1:] for p in token.split("_"))


def _read(path: Path) -> list[dict]:
    with open(path, newline="") as fh:
        return list(csv.DictReader(fh))


def _parse_range(value: str) -> dict[str, str]:
    """`apply_map=X;apply_reduce=Y` -> {'apply_map': 'X', ...}; '-' -> {}."""
    if not value or value == "-":
        return {}
    if "=" not in value:
        return {}
    out = {}
    for part in value.split(";"):
        field, _, target = part.partition("=")
        out[field.strip()] = target.strip()
    return out


def _retarget_list(prop: dict, ref: str) -> None:
    """Point every array branch of `prop` at `ref`, keeping title/description."""
    target = {"$ref": REF.format(ref)}
    if "items" in prop:
        prop["items"] = target
    for branch in prop.get("anyOf", []):
        if isinstance(branch, dict) and "items" in branch:
            branch["items"] = target


def _retarget_ref(prop: dict, ref: str) -> None:
    """Point a single-object property at `ref`."""
    target = REF.format(ref)
    if "$ref" in prop:
        prop["$ref"] = target
    for branch in prop.get("anyOf", []):
        if isinstance(branch, dict) and "$ref" in branch:
            branch["$ref"] = target


def _props(defs: dict, name: str) -> dict:
    return defs[name].setdefault("properties", {})


def _union(members: dict[str, str], description: str) -> dict:
    """A discriminated union entry, shaped as `schema.py` shapes the others."""
    return {
        "description": description,
        "discriminator": {"mapping": dict(sorted(members.items())), "propertyName": "type"},
        "oneOf": [{"$ref": REF.format(members[t])} for t in sorted(members)],
    }


def emit(defs: dict, matrix: Path) -> dict:
    """Return `defs` with the bundle layer added. Mutates and returns it.

    Raises ValueError on anything the authored CSVs get wrong that would
    otherwise emit a silently broken schema - an unknown token, a range naming
    a bundle that does not exist, a range set on a field the operator has not
    got.
    """
    bundles = _read(matrix / "bundles.csv")
    members = _read(matrix / "bundle_members.csv")

    kind = {b["bundle"]: b["applies_to"] for b in bundles}
    describe = {b["bundle"]: b["description"] for b in bundles}
    unknown = {m["bundle"] for m in members} - set(kind)
    if unknown:
        raise ValueError(f"bundle_members.csv names bundles absent from bundles.csv: {sorted(unknown)}")

    col_members: dict[str, list[dict]] = {}
    pipe_member: dict[str, dict] = {}
    for m in members:
        if kind[m["bundle"]] == "TransformationColumnSpec":
            col_members.setdefault(m["bundle"], []).append(m)
        else:
            if m["bundle"] in pipe_member:
                raise ValueError(f"{m['bundle']} names more than one TransformationSpec operator")
            pipe_member[m["bundle"]] = m

    # --- column bundles, and the nesting variants they need ------------------
    for bundle, rows in sorted(col_members.items()):
        mapping: dict[str, str] = {}
        for row in rows:
            token = row["type_token"]
            leaf = f"TransformationColumnSpec{_camel(token)}"
            if leaf not in defs:
                raise ValueError(f"{bundle}: no $defs entry {leaf} for token {token!r}")
            nested = _parse_range(row["columns_range"])
            if not nested:
                mapping[token] = leaf
                continue

            for field, target in nested.items():
                if target not in col_members:
                    raise ValueError(
                        f"{bundle}/{token}: columns_range names {target!r}, "
                        f"which is not a TransformationColumnSpec bundle"
                    )
            variant = f"{leaf}{bundle}"
            defs[variant] = copy.deepcopy(defs[leaf])
            vprops = _props(defs, variant)

            if token == "case":
                # `then` lives on CaseExpression, which needs its own variant.
                legs = f"CaseExpression{bundle}"
                defs[legs] = copy.deepcopy(defs["CaseExpression"])
                if "then" in nested:
                    _retarget_list(_props(defs, legs)["then"], nested["then"])
                _retarget_list(vprops["case_expr"], legs)
                if "else_expr" in nested:
                    _retarget_list(vprops["else_expr"], nested["else_expr"])
            else:
                for field, target in nested.items():
                    if field not in vprops:
                        raise ValueError(f"{bundle}/{token}: no field {field!r} to range")
                    _retarget_list(vprops[field], target)
            mapping[token] = variant

        defs[bundle] = _union(mapping, describe.get(bundle, ""))

    # --- pipe bundles --------------------------------------------------------
    for bundle, row in sorted(pipe_member.items()):
        token = row["type_token"]
        leaf = f"TransformationSpec{_camel(token)}"
        if leaf not in defs:
            raise ValueError(f"{bundle}: no $defs entry {leaf} for token {token!r}")

        cols = row["columns_range"]
        if cols != "-" and cols not in col_members:
            raise ValueError(f"{bundle}: columns_range {cols!r} is not a column bundle")

        defs[bundle] = copy.deepcopy(defs[leaf])
        defs[bundle]["description"] = describe.get(bundle, defs[bundle].get("description", ""))
        bprops = _props(defs, bundle)

        if cols == "-":
            # An operator with no `columns` must not acquire one through its
            # override either; an override overrides fields of its host.
            bprops.pop("columns", None)
        elif "columns" in bprops:
            _retarget_list(bprops["columns"], cols)
        else:
            raise ValueError(
                f"{bundle}: columns_range is {cols!r} but {leaf} has no columns field "
                f"(fields.csv says columns is not applicable to {token})"
            )

        # conditional_config -> a per-bundle conditional -> a per-bundle override
        cond_range = row["conditional_range"]
        if cond_range != "-" and "conditional_config" in bprops:
            if cond_range not in pipe_member:
                raise ValueError(f"{bundle}: conditional_range {cond_range!r} is not a pipe bundle")
            host = pipe_member[cond_range]
            host_leaf = f"TransformationSpec{_camel(host['type_token'])}"

            cond = f"ConditionalTransformationSpec{cond_range}"
            over = f"TransformationSpecOverride{cond_range}"
            if cond not in defs:
                defs[cond] = copy.deepcopy(defs["ConditionalTransformationSpec"])
                _retarget_ref(_props(defs, cond)["then"], over)

                defs[over] = copy.deepcopy(defs["TransformationSpecOverride"])
                oprops = _props(defs, over)
                keep = {"comment", "output_channel", "columns"} | {
                    k for k in defs[host_leaf].get("properties", {})
                    if k.endswith("_config") or k == "high_freq_columns"
                }
                for field in list(oprops):
                    if field not in keep:
                        del oprops[field]
                host_cols = host["columns_range"]
                if host_cols == "-":
                    oprops.pop("columns", None)
                elif "columns" in oprops:
                    _retarget_list(oprops["columns"], host_cols)

            _retarget_list(bprops["conditional_config"], cond)

    return defs


def check_corpus(schema: dict, matrix: Path, root: Path) -> tuple[int, list[str]]:
    """Validate every live TransformationSpec fragment against its bundle.

    This is what keeps the authored bundles honest. The corpus cannot *propose*
    a bundle - with 45 live configs, most operators appear once or twice, and at
    those counts "semantically excluded" and "nobody has written one yet" are
    indistinguishable. What it can do is **falsify** one: a fragment JetStore
    runs that its own bundle rejects means the authoring is wrong, and the
    `corpus_prod_files` column says how much to believe the config.

    Returns (fragments checked, findings).
    """
    import jsonschema

    from .corpus import config_files

    defs = schema["$defs"]
    kind = {b["bundle"]: b["applies_to"] for b in _read(matrix / "bundles.csv")}
    bundle_of = {
        m["type_token"]: m["bundle"]
        for m in _read(matrix / "bundle_members.csv")
        if kind.get(m["bundle"]) == "TransformationSpec"
    }
    validators = {
        name: jsonschema.Draft202012Validator(
            {"$schema": schema["$schema"], "$ref": REF.format(name), "$defs": defs}
        )
        for name in set(bundle_of.values())
    }

    checked = 0
    findings: list[str] = []

    def visit(node, path: str) -> None:
        nonlocal checked
        if isinstance(node, dict):
            for key, value in node.items():
                if key == "apply" and isinstance(value, list):
                    for i, item in enumerate(value):
                        if isinstance(item, dict) and "type" in item:
                            bundle = bundle_of.get(item["type"])
                            if bundle is None:
                                findings.append(f"{path}.apply.{i}: operator {item['type']!r} is in no bundle")
                            else:
                                checked += 1
                                errors = sorted(
                                    validators[bundle].iter_errors(item), key=lambda e: len(e.path)
                                )
                                if errors:
                                    where = ".".join(str(p) for p in errors[0].path)
                                    findings.append(
                                        f"{path}.apply.{i} ({bundle}): {where or '<root>'}: {errors[0].message}"
                                    )
                        visit(item, f"{path}.apply.{i}")
                else:
                    visit(value, f"{path}.{key}")
        elif isinstance(node, list):
            for i, item in enumerate(node):
                visit(item, f"{path}.{i}")

    import json as _json

    live, _ = config_files(root)
    for path in live:
        visit(_json.loads(path.read_text()), str(path))
    return checked, findings
