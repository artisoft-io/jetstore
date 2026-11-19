"""Checks on the corpus aliases. `python -m pytest tests_aliases.py`.

The aliases themselves are not in this repository, so these tests use a made-up
mapping, and the last one checks the committed data rather than the mapping: every
workspace a citation names is either ArtiSoft's own or a neutral `org<N>_ws`.
"""

import csv
import json
import re
from pathlib import Path

from cpipes_contract import aliases

HERE = Path(__file__).parent

SAMPLE = {
    "acme_ws": "org1_ws",
    "acme": "org1",
    "ACME": "ORG1",
    "acmeclaim": "org1claim",
    "ab": "org2",
}


def test_without_a_file_both_directions_are_the_identity(monkeypatch):
    monkeypatch.delenv(aliases.ENV, raising=False)
    a = aliases.load()
    assert not a
    assert a.anonymise("workspaces/acme_ws/x") == "workspaces/acme_ws/x"
    assert a.resolve("workspaces/org1_ws/x") == "workspaces/org1_ws/x"
    assert a.anonymise_json({"acme:k": ["acme"]}) == {"acme:k": ["acme"]}


def test_tokens_match_whole_words_only():
    a = aliases.Aliases(SAMPLE)
    assert a.anonymise("acme:Claim $ACME_OUT acme-prod acme_x.go") == (
        "org1:Claim $ORG1_OUT org1-prod org1_x.go"
    )
    # A token run into other letters or digits is not that token.
    assert a.anonymise("acmes about ab1 tab") == "acmes about ab1 tab"
    assert a.anonymise("ab-x-ab") == "org2-x-org2"


def test_the_longer_token_wins():
    a = aliases.Aliases(SAMPLE)
    assert a.anonymise("workspaces/acme_ws/pipes_config/acmeclaim_x.pc.json") == (
        "workspaces/org1_ws/pipes_config/org1claim_x.pc.json"
    )


def test_a_recorded_path_resolves_to_the_directory_on_disk(tmp_path):
    a = aliases.Aliases(SAMPLE)
    recorded = "workspaces/org1_ws/go/org1_handler.go"
    assert a.resolve(recorded) == "workspaces/acme_ws/go/acme_handler.go"
    assert a.path(tmp_path, recorded) == tmp_path / "workspaces/acme_ws/go/acme_handler.go"
    # Round trip: what is recorded from a real path resolves back to it.
    real = "workspaces/acme_ws/pipes_config/acmeclaim_x.pc.json"
    assert a.resolve(a.anonymise(real)) == real


def test_keys_are_anonymised_as_well_as_values():
    a = aliases.Aliases(SAMPLE)
    doc = {"$ACME_BUCKET": "acme-prod", "n": 1, "list": ["acme:A", True, None]}
    assert a.anonymise_json(doc) == {
        "$ORG1_BUCKET": "org1-prod", "n": 1, "list": ["org1:A", True, None],
    }


def test_the_file_is_read_from_the_environment(tmp_path, monkeypatch):
    path = tmp_path / "aliases.json"
    path.write_text(json.dumps(SAMPLE))
    monkeypatch.setenv(aliases.ENV, str(path))
    assert aliases.load().anonymise("acme_ws") == "org1_ws"


def test_a_malformed_file_is_refused(tmp_path):
    path = tmp_path / "aliases.json"
    path.write_text(json.dumps({"acme": ""}))
    try:
        aliases.from_file(path)
    except ValueError:
        return
    raise AssertionError("an empty replacement must be refused")


def test_the_committed_data_names_only_neutral_workspaces():
    """A library regenerated without the alias file would cite the directories by
    their names on disk. This is the check that it was not committed that way."""
    allowed = re.compile(r"^(jets_ws|org[0-9]+_ws)$")
    cited = re.compile(r"workspaces/([A-Za-z0-9_]+)")
    found: set[str] = set()
    for name in ("fragments/library.jsonl", "fragments/library.curated.jsonl"):
        for line in (HERE / name).read_text().splitlines():
            for c in json.loads(line)["citations"]:
                found.update(cited.findall(c["file"]))
    for line in (HERE / "repairs" / "library.jsonl").read_text().splitlines():
        found.add(json.loads(line)["workspace"])
    for name, column in (
        ("fields.csv", "evidence_ref"),
        ("types.csv", "exemplar_file"),
        ("recovered_sources.csv", "workspace"),
    ):
        with open(HERE / "matrix" / name, newline="") as fh:
            for row in csv.DictReader(fh):
                value = row[column]
                found.update(cited.findall(value) if "/" in value else [value])
    found.discard("-")
    assert found, "no workspace citation was found: the check reads nothing"
    assert sorted(w for w in found if not allowed.match(w)) == []
