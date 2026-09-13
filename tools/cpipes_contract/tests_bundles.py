"""Checks on the bundle layer. `python -m pytest tests_bundles.py` or run directly.

**The `bundles` command is a falsifier and not a check on the authoring.** It walks
the live corpus and validates each `apply` fragment against its bundle, so it says
nothing at all about an operator no `.pc.json` uses - which is exactly how `vllm`
and `embed` sat in no bundle from the day each was built (I-512). `embed` only
became visible on 2026-09-08, when Phase 6's asset install put the JetStore-owned
`embed_input_parts.pc.json` into all four workspaces and the command started
reporting `operator 'embed' is in no bundle` four times.

So these tests hold the two halves the corpus cannot:

- **every `TransformationSpec` token the emitted schema admits has a bundle**, which
  is the rule that would have caught `vllm` and `embed` on the day their leaf rows
  landed rather than months later;
- **a bundle still refuses what it should refuse.** A bundle is a `deepcopy` of its
  leaf with two properties retargeted, so a bundle that accepted everything would
  pass the corpus check and the authorability check alike, and both would read as
  green. The negative cases below are the five classes `negative_suite.json` uses
  at the root, asked at the bundle position instead.
"""

import copy
import csv
import json
from pathlib import Path

import jsonschema

HERE = Path(__file__).parent
SCHEMA = json.loads((HERE / "cpipes_schema.json").read_text())
DEFS = SCHEMA["$defs"]


def _rows(name):
    with open(HERE / "matrix" / name, newline="") as fh:
        return list(csv.DictReader(fh))


def _validator(bundle):
    return jsonschema.Draft202012Validator(
        {"$schema": SCHEMA["$schema"], "$ref": f"#/$defs/{bundle}", "$defs": DEFS}
    )


def errors(bundle, doc):
    return sorted(_validator(bundle).iter_errors(doc), key=lambda e: len(e.path))


# A minimal valid fragment per infer-family bundle. `output_mapping` is required on
# the two generative specs and absent from `EmbedSpec`, whose vector reaches the
# record through `vector_column`.
VALID = {
    "OllamaPipe": {
        "type": "ollama",
        "ollama_config": {
            "model": "m",
            "prompt_template": "{{input}}",
            "output_mapping": [{"column": "answer"}],
        },
        "output_channel": {"name": "o", "channel_spec_name": "c"},
    },
    "InferPipe": {
        "type": "infer",
        "infer_config": {
            "model": "m",
            "prompt_template": "{{input}}",
            "output_mapping": [{"column": "answer"}],
        },
        "output_channel": {"name": "o", "channel_spec_name": "c"},
    },
    "VllmPipe": {
        "type": "vllm",
        "vllm_config": {
            "model": "m",
            "prompt_template": "{{input}}",
            "output_mapping": [{"column": "answer"}],
        },
        "output_channel": {"name": "o", "channel_spec_name": "c"},
    },
    "EmbedPipe": {
        "type": "embed",
        "embed_config": {
            "model": "m",
            "prompt_template": "{{part_text}}",
            "vector_column": "embedding",
        },
        "output_channel": {"name": "o", "channel_spec_name": "c"},
    },
}

CONFIG_KEY = {
    "OllamaPipe": "ollama_config",
    "InferPipe": "infer_config",
    "VllmPipe": "vllm_config",
    "EmbedPipe": "embed_config",
}


def test_every_admitted_operator_has_a_bundle():
    """The rule I-512 was the absence of.

    `TransformationSpec`'s discriminator is what the emitted schema admits as an
    `apply` step; `bundle_members.csv` is what the *bundle* tier admits. Nothing
    compared the two, so adding a leaf row and adding a bundle member stayed
    separate acts and only the first was prompted by `drift`.
    """
    kind = {r["bundle"]: r["applies_to"] for r in _rows("bundles.csv")}
    bundled = {
        r["type_token"]
        for r in _rows("bundle_members.csv")
        if kind.get(r["bundle"]) == "TransformationSpec"
    }
    admitted = set(DEFS["TransformationSpec"]["discriminator"]["mapping"])
    assert admitted - bundled == set(), f"operators in no bundle: {sorted(admitted - bundled)}"


def _complement_rows():
    return [r for r in _rows("types.csv") if r["variant_when"].startswith("unlisted(")]


def test_the_complement_branch_excludes_exactly_its_union_s_own_tokens():
    """The site branch is the complement of the union, not a second list of it.

    `TransformationSpecSite` says `type` is a string that is `not` one of the
    nineteen built-in tokens. That enum and the union's own branches are the same
    fact written twice, which is the shape `builtinOperatorTypes` and
    `reportsRowLevelFailures` each already have a Go test against - and the cost
    of their drifting here is worse than a wrong warning: a token in the union
    and missing from the enum matches *two* `oneOf` branches, so the union stops
    being exclusive and a document that should be one operator is neither.

    The emitter derives the enum from `types.csv`, so this holds by construction
    today. It is asserted anyway because "by construction" is a property of one
    revision of one function.
    """
    rows = _complement_rows()
    assert rows, "no complement token in types.csv; this test asserted nothing"
    for row in rows:
        struct, defs_name = row["go_struct"], row["defs_name"]
        excluded = DEFS[defs_name]["properties"][row["discriminator"]]["not"]["enum"]
        admitted = set(DEFS[struct]["discriminator"]["mapping"])
        assert set(excluded) == admitted, (
            f"{defs_name}: excludes {sorted(set(excluded) ^ admitted)} "
            f"more or less than {struct} admits"
        )
        assert len(excluded) == len(set(excluded)), f"{defs_name}: duplicate tokens"


def test_the_complement_branch_joins_every_occurrence_of_its_union():
    """Pydantic inlines an `Annotated[Union[...]]` at each use site.

    So `$defs/TransformationSpec` is one occurrence among several - the three
    `PipeSpec*.apply` arrays carry their own copy - and splicing only the named
    entry produces a schema whose addressable entry admits a site operator while
    no document containing one validates. Counting the occurrences is the cheap
    way to say that the walk in `schema.py` still reaches all of them.
    """
    for row in _complement_rows():
        branch = {"$ref": f"#/$defs/{row['defs_name']}"}
        tokens = set(DEFS[row["go_struct"]]["discriminator"]["mapping"])
        with_branch = without = 0

        def walk(node):
            nonlocal with_branch, without
            if isinstance(node, dict):
                one_of = node.get("oneOf")
                mapping = (node.get("discriminator") or {}).get("mapping")
                if isinstance(one_of, list) and mapping and set(mapping) == tokens:
                    if branch in one_of:
                        with_branch += 1
                    else:
                        without += 1
                for value in node.values():
                    walk(value)
            elif isinstance(node, list):
                for value in node:
                    walk(value)

        walk(DEFS)
        assert with_branch > 1, (
            f"{row['defs_name']}: spliced into {with_branch} occurrence(s); the named "
            f"$defs entry alone is the failure this test exists for"
        )
        assert without == 0, (
            f"{row['defs_name']}: {without} occurrence(s) of the {row['go_struct']} "
            f"union do not carry the complement branch"
        )


def test_a_complement_token_is_in_no_bundle_and_that_is_deliberate():
    """A bundle groups operators by what they are *for*, and nobody here knows.

    `test_every_admitted_operator_has_a_bundle` reads the discriminator mapping,
    which a complement token is absent from by construction, so that rule stays
    silent here rather than being satisfied. Saying so is the point: the silence
    is a decision - a site operator's meaning belongs to the deployment that
    wrote it, so no bundle in this repository can describe one - and not an
    omission for someone to tidy up.
    """
    bundled = {r["type_token"] for r in _rows("bundle_members.csv")}
    for row in _complement_rows():
        assert row["type_token"] not in bundled
        assert row["type_token"] not in DEFS[row["go_struct"]]["discriminator"]["mapping"]


def test_every_bundle_admits_exactly_its_own_operator():
    """A pipe bundle names one operator, so its discriminator is that token alone."""
    kind = {r["bundle"]: r["applies_to"] for r in _rows("bundles.csv")}
    for row in _rows("bundle_members.csv"):
        bundle = row["bundle"]
        if kind.get(bundle) != "TransformationSpec":
            continue
        assert bundle in DEFS, f"{bundle} is in no $defs"
        const = DEFS[bundle]["properties"]["type"].get("const")
        assert const == row["type_token"], f"{bundle}: type const is {const!r}"


def test_the_infer_family_fragments_validate_against_their_bundles():
    for bundle, doc in VALID.items():
        assert errors(bundle, doc) == [], f"{bundle}: {[e.message for e in errors(bundle, doc)]}"


def test_a_bundle_refuses_an_invented_field_name():
    for bundle, doc in VALID.items():
        bad = copy.deepcopy(doc)
        bad["not_a_field"] = 1
        assert errors(bundle, bad), f"{bundle} accepted an invented field"


def test_a_bundle_refuses_a_misspelt_key_inside_the_operators_own_config():
    """The case that makes the positive one mean something.

    A bundle is a `deepcopy` of its leaf, so the way it goes wrong is by admitting
    more than the leaf did rather than by failing to exist. A misspelt
    `prompt_template` is the cheapest probe of `additionalProperties: false`
    surviving the copy.
    """
    for bundle, doc in VALID.items():
        key = CONFIG_KEY[bundle]
        bad = copy.deepcopy(doc)
        bad[key]["promt_template"] = bad[key].pop("prompt_template")
        assert errors(bundle, bad), f"{bundle} accepted a misspelt {key} key"


def test_a_bundle_refuses_a_missing_required_config():
    for bundle, doc in VALID.items():
        bad = copy.deepcopy(doc)
        bad.pop(CONFIG_KEY[bundle])
        assert errors(bundle, bad), f"{bundle} accepted a fragment with no {CONFIG_KEY[bundle]}"


def test_a_bundle_refuses_a_sibling_backends_operator():
    """The four infer-family bundles are near-identical, which is the risk.

    Their leaves differ in one property name and one `const`, so a bundle that took
    its neighbour's fragment would be indistinguishable from a correct one on every
    other axis - and the corpus, which carries a live fragment for `embed` and none
    for `vllm`, could not tell them apart either.
    """
    for bundle in VALID:
        for other, doc in VALID.items():
            if other == bundle:
                continue
            assert errors(bundle, doc), f"{bundle} accepted a {other} fragment"


def test_a_bundle_narrows_its_conditional_override_to_itself():
    """The narrowing that stops the union re-inflating, asked as a negative.

    `conditional_config` -> `ConditionalTransformationSpec<Bundle>` ->
    `TransformationSpecOverride<Bundle>` keeps only the host's own `_config` key, so
    an override carrying a sibling backend's config is refused. That the narrowing
    also forbids a `then` that *replaces* the host outright is I-511, and is neither
    tested nor decided here.
    """
    for bundle, doc in VALID.items():
        foreign = next(CONFIG_KEY[b] for b in VALID if b != bundle)
        bad = copy.deepcopy(doc)
        bad["conditional_config"] = [
            {"when": [{"expr": "1"}], "then": {foreign: {"model": "m"}}}
        ]
        assert errors(bundle, bad), f"{bundle} accepted an override carrying {foreign}"


if __name__ == "__main__":
    import traceback

    failed = 0
    for _name, _fn in sorted(globals().items()):
        if not _name.startswith("test_") or not callable(_fn):
            continue
        try:
            _fn()
            print(f"ok   {_name}")
        except Exception:
            failed += 1
            print(f"FAIL {_name}")
            traceback.print_exc()
    raise SystemExit(1 if failed else 0)
