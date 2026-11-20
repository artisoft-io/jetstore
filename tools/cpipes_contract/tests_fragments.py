"""Checks on the fragment extraction and the corpus it reads. `python -m pytest tests_fragments.py`.

These need no workspace: each one walks a value written here against the committed
schema, reads the committed library, or reads a corpus laid out in a temporary directory.
"""

import json
from collections import Counter
from pathlib import Path
from types import SimpleNamespace

from cpipes_contract import aliases, corpus
from cpipes_contract.fragments import ROOT, Extractor, free_text_properties, tag_defaults

HERE = Path(__file__).parent
SCHEMA = json.loads((HERE / "cpipes_schema.json").read_text())
DEFS = SCHEMA["$defs"]
DEFAULTS = tag_defaults(HERE / "cpipes_model.py")
FILE = "workspaces/x_ws/pipes_config/x.pc.json"


def _walk(value, node):
    ex = Extractor(SCHEMA)
    ex.walk(value, node, "workspaces/x_ws/pipes_config/x.pc.json", "")
    return ex, Counter(p.defs_name for p in ex.parts.values())


def test_a_site_operator_binds_to_the_complement_branch():
    # The type names no built-in operator, so the union's mapping has no entry for
    # it; the schema's `oneOf` carries the branch that admits any other type.
    apply = DEFS["PipeSpecFanOut"]["properties"]["apply"]["anyOf"][0]["items"]
    value = {
        "type": "a_site_operator",
        "output_channel": {"type": "memory", "name": "out"},
        "site_config": {"config": {"k": "v"}},
    }
    ex, bound = _walk(value, apply)
    assert ex.unresolved == []
    assert bound["TransformationSpecSite"] == 1
    assert bound["SiteOperatorSpec"] == 1


def test_a_built_in_type_still_binds_by_its_tag():
    apply = DEFS["PipeSpecFanOut"]["properties"]["apply"]["anyOf"][0]["items"]
    ex, bound = _walk({"type": "sort", "output_channel": {"type": "memory", "name": "out"}}, apply)
    assert ex.unresolved == []
    assert bound["TransformationSpecSort"] == 1
    assert bound["TransformationSpecSite"] == 0


def test_an_untagged_element_binds_by_which_keys_it_has():
    element = {"anyOf": [{"$ref": "#/$defs/ElementParagraph"}, {"$ref": "#/$defs/ElementGroup"}]}
    _, bound = _walk({"text": "a"}, element)
    assert bound == Counter({"ElementParagraph": 1})
    _, bound = _walk({"elements": [{"text": "a"}, {"text": "b"}]}, element)
    assert bound == Counter({"ElementGroup": 1, "ElementParagraph": 2})
    # A value that fits neither member, or both, binds to nothing rather than to a guess.
    _, bound = _walk({"text": "a", "elements": []}, element)
    assert bound == Counter()


def test_installed_jetstore_assets_are_not_corpus(tmp_path):
    assets = corpus.jetstore_assets()
    assert assets, f"no .pc.json under {corpus.JETSTORE_ASSETS}"
    installed = sorted(assets)[0]
    live_dir = tmp_path / "workspaces" / "x_ws" / "pipes_config"
    live_dir.mkdir(parents=True)
    (live_dir / installed).write_text("{}")
    (live_dir / "own.pc.json").write_text("{}")
    # The same name below `pipes_config/` is the workspace's own file, not an install.
    (live_dir / "test").mkdir()
    (live_dir / "test" / installed).write_text("{}")
    live, retired = corpus.config_files(tmp_path)
    assert [p.relative_to(live_dir).as_posix() for p in live] == ["own.pc.json", f"test/{installed}"]
    assert retired == []


def test_corpus_apply_records_exemplars_under_neutral_names(tmp_path, monkeypatch):
    alias_file = tmp_path / "aliases.json"
    alias_file.write_text(json.dumps({"acme_ws": "org1_ws"}))
    monkeypatch.setenv(aliases.ENV, str(alias_file))
    row = SimpleNamespace(
        go_struct="SortSpec", type_token="*", corpus_instances=0, corpus_prod_instances=0,
        exemplar_file="-", exemplar_path="-",
    )
    measurement = corpus.Measurement()
    key = ("SortSpec", "*")
    measurement.instances[key] = 2
    measurement.prod_instances[key] = 1
    measurement.exemplars[key] = ("workspaces/acme_ws/pipes_config/a.pc.json", "x.0.sort_config")
    corpus.apply(SimpleNamespace(types=[row], fields_=[]), measurement)
    assert (row.corpus_instances, row.corpus_prod_instances) == (2, 1)
    assert row.exemplar_file == "workspaces/org1_ws/pipes_config/a.pc.json"
    assert row.exemplar_path == "x.0.sort_config"


# ---------------------------------------------------------------------------
# Free text does not enter the library
# ---------------------------------------------------------------------------


def _keys(value):
    """Every object key at any depth."""
    if isinstance(value, dict):
        for k, v in value.items():
            yield k
            yield from _keys(v)
    elif isinstance(value, list):
        for v in value:
            yield from _keys(v)


def _library(name):
    text = (HERE / "fragments" / name).read_text()
    return [json.loads(line) for line in text.splitlines() if line.strip()]


def _sort_step(comment=None):
    op = {"type": "sort", "output_channel": {"name": "out"}, "sort_config": {"domain_keys": ["k"]}}
    if comment is not None:
        op["comment"] = comment
        op["output_channel"]["comment"] = comment
    return op


def _config(*ops, comment=None):
    cfg = {
        "cluster_config": {},
        "reducing_pipes_config": [],
        "conditional_pipes_config": [
            {"pipes_config": [{"type": "fan_out", "input": "input_row", "apply": list(ops)}]}
        ],
    }
    if comment is not None:
        cfg["comment"] = comment
    return cfg


def test_the_schema_names_comment_and_only_comment_as_free_text():
    # The strip is keyed on the schema's own words for a free-text property. If they
    # change, this finds nothing, and the strip would silently strip nothing.
    found = free_text_properties(SCHEMA)
    assert "ComputePipesConfig" in found
    assert len(found) > 100
    assert {p for props in found.values() for p in props} == {"comment"}
    assert not any("comment" in DEFS[name].get("required", []) for name in found)


def test_a_comment_in_the_corpus_reaches_no_part():
    config = _config(_sort_step("a note for a colleague"), comment="what this pipeline is for")
    before = json.dumps(config, sort_keys=True)
    ex = Extractor(SCHEMA, DEFAULTS)
    ex.add(config, FILE)
    assert ex.unresolved == []
    names = Counter(p.defs_name for p in ex.parts.values())
    # The comment sat on the root, on an operator and on an untagged output channel,
    # which binds only through the union's default; all three are gone, at every depth.
    assert {"ComputePipesConfig", "TransformationSpecSort", "OutputChannelConfigMemory"} <= set(names)
    assert all("comment" not in set(_keys(p.value)) for p in ex.parts.values())
    assert ex.stripped == {"comment": 3}
    assert json.dumps(config, sort_keys=True) == before, "the strip must not change its input"


def test_parts_that_differ_only_by_a_comment_are_one_part():
    ex = Extractor(SCHEMA, DEFAULTS)
    ex.add(_config(_sort_step("first author's note")), FILE)
    ex.add(_config(_sort_step("second author's note")), "workspaces/y_ws/pipes_config/y.pc.json")
    ex.add(_config(_sort_step()), "workspaces/z_ws/pipes_config/z.pc.json")
    sorts = [p for p in ex.parts.values() if p.defs_name == "TransformationSpecSort"]
    assert len(sorts) == 1
    assert sorts[0].instances == 3


def test_a_comment_key_in_data_the_schema_does_not_describe_is_kept():
    # A site operator's own config is carried verbatim and has no schema, so a key
    # named `comment` in it is the site's data, not prose the schema declares.
    op = {
        "type": "a_site_operator",
        "output_channel": {"type": "memory", "name": "out"},
        "site_config": {"comment": "dropped", "config": {"comment": "kept"}},
    }
    ex = Extractor(SCHEMA, DEFAULTS)
    ex.add(_config(op), FILE)
    (site,) = [p for p in ex.parts.values() if p.defs_name == "SiteOperatorSpec"]
    assert site.value == {"config": {"comment": "kept"}}


def test_the_committed_libraries_carry_no_free_text():
    # The guard on the published files: stripping a part that is already stripped
    # changes nothing, for every part of both libraries, at every depth.
    ex = Extractor(SCHEMA, DEFAULTS)
    for name in ("library.jsonl", "library.curated.jsonl"):
        for part in _library(name):
            ref = {"$ref": f"#/$defs/{part['defs_name']}"}
            assert ex.strip(part["value"], ref) == part["value"], f"{name}: {part['defs_name']}"
    assert dict(ex.stripped) == {}


# Bundles whose curated parts carried a workspace author's comment before the strip,
# so the two consumer tests below fail on the library as it was.
WITNESSES = ["FilterPipe", "InferPipe", "RenderPipe"]


def test_the_authoring_prompt_shows_examples_without_free_text():
    from cpipes_contract.authoring import instruction_for
    from cpipes_contract.template import Hole

    curated = _library("library.curated.jsonl")
    for ref in WITNESSES:
        hole = Hole(name="op", schema_ref=ref, prompt="Configure this operator.")
        prompt = instruction_for(hole, SCHEMA, HERE / "matrix", curated)
        # Examples are still shown, each under its type's schema-derived line, and
        # none of them carries a comment for the model to copy.
        assert "real examples from existing JetStore configurations" in prompt, ref
        block = prompt.split("```json\n", 1)[1].split("```", 1)[0]
        assert block.startswith("// "), ref
        assert '"comment"' not in block, ref


def test_an_expansion_from_the_library_carries_no_free_text():
    from cpipes_contract.expand import from_library
    from cpipes_contract.template import Hole

    fill = from_library(_library("library.curated.jsonl"), HERE / "matrix")
    for ref in WITNESSES:
        value = fill(Hole(name="h", schema_ref=ref, prompt="-"), {})
        assert isinstance(value, dict), ref
        assert "comment" not in set(_keys(value)), ref

