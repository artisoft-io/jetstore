"""Checks on the fragment extraction and the corpus it reads. `python -m pytest tests_fragments.py`.

These need no workspace: each one walks a value written here against the committed
schema, or a corpus laid out in a temporary directory.
"""

import json
from collections import Counter
from pathlib import Path
from types import SimpleNamespace

from cpipes_contract import aliases, corpus
from cpipes_contract.fragments import Extractor

HERE = Path(__file__).parent
SCHEMA = json.loads((HERE / "cpipes_schema.json").read_text())
DEFS = SCHEMA["$defs"]


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
