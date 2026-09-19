"""The entry and the local driver: the order of the refusals, and X6's place.

The order matters more than any individual refusal. `coordinate` judges the
document's tokens **before** it resolves the schema provider, builds the
environment or asks how this node gets its rows — which is what "aborts at
startup" means, and is a guarantee the Go node cannot give because its operator
registry and its document are seen in different processes.
"""

from __future__ import annotations

import json
from pathlib import Path

import pytest

from conftest import (
    document,
    map_record_step,
    out_of_scope_step,
    runtime_document,
    site_step,
)
from cpipes_node import contract, main
from cpipes_node.args import NodeArgs
from cpipes_node.config import FileConfigSource, parse_config
from cpipes_node.errors import (
    OperatorNotImplemented,
    OperatorOutOfScope,
    StartupError,
)
from cpipes_node.graph import GraphNotBuilt
from cpipes_node.node import GENERATOR_FILE_PROXY, coordinate, environment
from cpipes_node.settings import MINIMUM_DB_POOL_SIZE, Settings
from cpipes_node.site import Registry
from cpipes_node.store import Local

ENV = {
    "JETS_BUCKET": "b",
    "JETS_REGION": "us-east-1",
    "JETS_DSN_SECRET": "s",
}


# --- settings ---------------------------------------------------------------


def test_every_missing_variable_is_reported_at_once():
    # The Go main collects them all and panics once; a deployment missing three
    # that is told about one is a deployment redeployed three times.
    with pytest.raises(StartupError) as exc:
        Settings.from_env({})
    message = str(exc.value)
    for name in ("JETS_DSN_SECRET", "JETS_REGION", "JETS_BUCKET"):
        assert name in message


def test_the_pool_size_floor_is_the_go_main_s():
    assert Settings.from_env({**ENV, "CPIPES_DB_POOL_SIZE": "1"}).db_pool_size == 3
    assert Settings.from_env({**ENV, "CPIPES_DB_POOL_SIZE": "8"}).db_pool_size == 8
    assert Settings.from_env({**ENV, "CPIPES_DB_POOL_SIZE": "x"}).db_pool_size == (
        MINIMUM_DB_POOL_SIZE
    )
    assert Settings.from_env(ENV).kms_key_arn == ""


# --- the environment --------------------------------------------------------


def test_the_environment_carries_the_two_the_node_adds():
    config = contract.PipesConfig.model_validate(document([map_record_step()]))
    env = environment(config, NodeArgs(id=7, pe=1))
    assert env["$SHARD_ID"] == 7
    assert env["$JETS_PARTITION_LABEL"] == "0007P"


def test_a_missing_main_input_provider_is_the_go_node_s_refusal():
    doc = document([map_record_step()])
    doc["schema_providers"] = []
    config = contract.PipesConfig.model_validate(doc)
    with pytest.raises(StartupError, match="main_input schema provider"):
        environment(config, NodeArgs(id=0, pe=1))


def test_a_key_that_is_a_prefix_of_another_key_is_refused():
    # The Go source states the hazard in a comment — `$FILE_KEY` and
    # `$FILE_KEY_PATH` — and checks nothing. Substitution is textual, so the
    # longer key silently takes the shorter one's value and a dangling suffix.
    doc = document([map_record_step()])
    doc["schema_providers"][0]["env"] = {"$FILE_KEY": "a", "$FILE_KEY_PATH": "b"}
    config = contract.PipesConfig.model_validate(doc)
    with pytest.raises(StartupError, match="prefix"):
        environment(config, NodeArgs(id=0, pe=1))


# --- coordinate -------------------------------------------------------------


def _source(tmp_path: Path, doc: dict) -> FileConfigSource:
    path = tmp_path / "p.pc.json"
    path.write_text(json.dumps(doc))
    return FileConfigSource(path)


def test_an_out_of_scope_token_aborts_before_anything_else(tmp_path: Path):
    # The document has no schema provider either, so if the scope gate ran
    # second this would raise the provider's refusal instead. That is the
    # assertion: X6 is first.
    doc = runtime_document([out_of_scope_step()])
    doc["schema_providers"] = []
    with pytest.raises(OperatorOutOfScope) as exc:
        coordinate(NodeArgs(id=0, pe=1), _source(tmp_path, doc), store=Local(tmp_path))
    assert "aggregate" in str(exc.value)
    assert "the scope searched" in str(exc.value)


def test_a_declared_but_unbuilt_token_aborts_distinguishably(tmp_path: Path):
    doc = runtime_document([map_record_step()])
    with pytest.raises(OperatorNotImplemented) as exc:
        coordinate(NodeArgs(id=0, pe=1), _source(tmp_path, doc), store=Local(tmp_path))
    assert "P9-T06" in str(exc.value)


def test_a_clean_document_reaches_the_graph_seam(tmp_path: Path, monkeypatch):
    # Every token in scope and built: the node gets as far as the channel
    # graph, which is P9-T04's. Reaching it is the deliverable of this task.
    from cpipes_node import scope
    from cpipes_node.scope import TokenKind

    built = {}
    for kind, token in (
        (TokenKind.INPUT_CHANNEL, "generator"),
        (TokenKind.PIPE, "fan_out"),
    ):
        cls = scope.declaration(kind, token)
        built[(kind, token)] = cls
        monkeypatch.setattr(cls, "build", classmethod(lambda c, e, s: None))

    doc = runtime_document([site_step("hc_corpus")])
    with pytest.raises(GraphNotBuilt) as exc:
        coordinate(
            NodeArgs(id=3, pe=1),
            _source(tmp_path, doc),
            store=Local(tmp_path),
            site_operators=Registry().with_operators({"hc_corpus": lambda e, s: None}),
        )
    assert "P9-T04" in str(exc.value)
    assert "0003P" in str(exc.value)
    assert "reducing" in str(exc.value)


def test_a_generator_in_reducing_mode_gets_the_proxy_marker(
    tmp_path: Path, monkeypatch
):
    from cpipes_node import node as node_module

    doc = runtime_document([site_step("hc_corpus")])
    config = parse_config(json.dumps(doc))
    assert node_module._file_keys(config, NodeArgs(id=0, pe=1)) == (
        GENERATOR_FILE_PROXY,
    )


def test_an_invalid_mode_is_refused_with_the_go_node_s_message(tmp_path: Path):
    from cpipes_node import node as node_module

    doc = runtime_document([site_step("hc")], mode="cruising")
    config = parse_config(json.dumps(doc))
    with pytest.raises(StartupError, match="invalid cpipesMode"):
        node_module._file_keys(config, NodeArgs(id=0, pe=1))


# --- the driver -------------------------------------------------------------


def test_the_scope_command_prints_the_declaration(capsys):
    assert main.main(["scope"]) == main.EXIT_OK
    out = capsys.readouterr().out
    assert "partition_writer" in out and "P9-T07" in out


def test_the_check_command_separates_the_two_failures(tmp_path: Path, capsys):
    out_of_scope = tmp_path / "bad.pc.json"
    out_of_scope.write_text(json.dumps(document([out_of_scope_step()])))
    assert main.main(["check", "--config", str(out_of_scope)]) == main.EXIT_OUT_OF_SCOPE

    declared = tmp_path / "ok.pc.json"
    declared.write_text(json.dumps(document([map_record_step()])))
    assert main.main(["check", "--config", str(declared)]) == main.EXIT_NOT_IMPLEMENTED
    assert "examined and accepted" in capsys.readouterr().out


def test_the_check_command_is_deterministic(tmp_path: Path, capsys):
    path = tmp_path / "p.pc.json"
    path.write_text(json.dumps(document([map_record_step(), site_step("x")])))
    runs = []
    for _ in range(3):
        main.main(["check", "--config", str(path)])
        runs.append(capsys.readouterr())
    assert len({r.out for r in runs}) == 1
    assert len({r.err for r in runs}) == 1


def test_the_run_command_refuses_and_says_why(tmp_path: Path, capsys):
    path = tmp_path / "p.pc.json"
    path.write_text(json.dumps(runtime_document([map_record_step()])))
    code = main.main(
        [
            "run",
            "--config",
            str(path),
            "--store",
            str(tmp_path),
            "--id",
            "0",
            "--pe",
            "1",
        ]
    )
    assert code == main.EXIT_REFUSED
    assert "OperatorNotImplemented" in capsys.readouterr().err
