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
from cpipes_node import contract, main, scope
from cpipes_node.args import NodeArgs
from cpipes_node.config import FileConfigSource, parse_config
from cpipes_node.errors import (
    OperatorNotImplemented,
    OperatorOutOfScope,
    StartupError,
)
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


def test_a_declared_but_unbuilt_token_aborts_distinguishably(
    tmp_path: Path, declared_and_unbuilt
):
    """It named `map_record` until P9-T06 built it; see the fixture's docstring."""
    doc = runtime_document([out_of_scope_step()])
    with pytest.raises(OperatorNotImplemented) as exc:
        coordinate(NodeArgs(id=0, pe=1), _source(tmp_path, doc), store=Local(tmp_path))
    assert declared_and_unbuilt.owed_by in str(exc.value)


def test_a_clean_document_reaches_the_graph_and_runs(tmp_path: Path):
    """Every token in scope and built: the node runs the document end to end.

    **This test asserted the opposite until P9-T04**: it required
    `GraphNotBuilt` naming the task, because reaching the seam was P9-T03's whole
    deliverable. Inverted rather than deleted, so the property it stood for — the
    scope gate passes a clean document *through* — is still checked by something,
    and by something that now goes red if the graph stops running.

    **It also monkeypatched `build` onto `generator` and `fan_out`** to get past
    the scope gate, both being declared and owed by this task at the time. Both
    now carry one, so the patching is gone and the gate is satisfied by the tree
    rather than by the test.

    The site factory is the smallest recognisable operator: it counts what it is
    given and writes nothing, so what is asserted is the graph's arithmetic
    rather than an operator's.
    """
    applied: list[list] = []

    class Counting:
        def apply(self, record):
            applied.append(record)

        def done(self):
            pass

        def finally_(self):
            pass

    doc = runtime_document([site_step("hc_corpus")])
    result = coordinate(
        NodeArgs(id=3, pe=1),
        _source(tmp_path, doc),
        store=Local(tmp_path),
        site_operators=Registry().with_operators(
            {"hc_corpus": lambda e, s: Counting()}
        ),
    )
    # `nbr_rows` is 10 in the fixture, and the record's width is the main
    # input's one column — both asserted, because a source that generated ten
    # records of no width would satisfy a count alone.
    assert result.source_rows == 10
    assert len(applied) == 10
    assert applied[0] == [None]
    # And the channel the step declared is closed, which is what a reader of it
    # would otherwise wait on forever.
    assert "out" in result.closed_channels


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
    """Every declared token is printed, and an owed marker appears for exactly
    those the registry reports unimplemented — **derived from the registry, never
    listed**.

    This test pinned a list twice and was wrong twice, each time correctly about
    its own branch: it asserted `partition_writer` owed by P9-T07 until P9-T07
    built it, and `merge_files` owed by P9-T08 until P9-T08 built it. **The two
    repairs were on branches that could not see each other**, so each passed alone
    and the merged state failed (P7-I87). A list of who owes what is P3-I20's
    shape — omission and completion print the same thing — so the subject is now
    the registry's own answer and this test needs no edit when the next token
    lands.

    **As of the merge of P9-T06/T07 and P9-T08/T09 nothing is owed**: all seven
    declared tokens are built. That is asserted as a consequence of the derivation
    rather than written down, so it stops being true the moment somebody declares
    an eighth.
    """
    assert main.main(["scope"]) == main.EXIT_OK
    out = capsys.readouterr().out
    # **`scope.declarations()` is the registry itself** — one class per declared
    # token — so the subject is the producer and not a list beside it.
    for operator in scope.declarations():
        token = operator.token
        assert token in out, f"{token} is declared and unprinted"
        line = _line_for(out, token)
        if operator.implemented():
            assert "owed by" not in line, (
                f"{token} is built and still prints an owed marker"
            )
        else:
            assert "owed by" in line and operator.owed_by in line, (
                f"{token} is owed and prints no owner"
            )


def _line_for(out: str, token: str) -> str:
    """The one line of the rendered scope that names `token`."""
    for line in out.splitlines():
        if line.strip().startswith(token):
            return line
    raise AssertionError(f"no line of the rendered scope names {token}")


def test_the_check_command_separates_the_two_failures(
    tmp_path: Path, capsys, declared_and_unbuilt
):
    # The fixture makes `aggregate` *declared*, so it is the unimplemented
    # exemplar below and cannot also be the out-of-scope one. A token nothing
    # declares at all is a site token with no registry behind it, which is the
    # honest out-of-scope case anyway.
    out_of_scope = tmp_path / "bad.pc.json"
    out_of_scope.write_text(json.dumps(document([site_step("transmogrify")])))
    assert main.main(["check", "--config", str(out_of_scope)]) == main.EXIT_OUT_OF_SCOPE

    declared = tmp_path / "ok.pc.json"
    declared.write_text(json.dumps(document([out_of_scope_step()])))
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


def test_the_run_command_refuses_and_says_why(
    tmp_path: Path, capsys, declared_and_unbuilt
):
    """It ran `map_record` until P9-T06 built it, and then exited 0.

    Inverted rather than deleted: the refusal path is still the subject, so the
    exemplar is the fixture's declared-and-unbuilt token — and the *other*
    direction is asserted by `test_the_run_command_runs_a_built_document` below,
    which is new and is the evidence that this operator reaches a row.
    """
    path = tmp_path / "p.pc.json"
    path.write_text(json.dumps(runtime_document([out_of_scope_step()])))
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


# --- the lambda entry -------------------------------------------------------


def test_the_lambda_handler_takes_the_go_entry_s_event(tmp_path: Path):
    # `handler(ctx, arg ComputePipesNodeArgs)` in Python. The event is the
    # same three fields, and the scope gate is reached through it — which is
    # the assertion: the deployed path and the local path are one `coordinate`.
    from cpipes_node.awslambda import Node

    doc = runtime_document([out_of_scope_step()])

    class _Cur:
        def __enter__(self):
            return self

        def __exit__(self, *exc):
            return False

        def execute(self, sql, params):
            pass

        def fetchone(self):
            return (json.dumps(doc),)

    class _Conn:
        def cursor(self):
            return _Cur()

    node = Node(settings=Settings.from_env(ENV), connect=lambda s: _Conn())
    with pytest.raises(OperatorOutOfScope, match="aggregate"):
        node.handler({"id": 0, "jp": "0000P", "pe": 9})


def test_a_node_with_no_connection_refuses_at_invocation():
    # Not at import: a cold start that succeeded and an invocation that cannot
    # read its configuration are different things to see in a log.
    from cpipes_node.awslambda import Node

    node = Node(settings=Settings.from_env(ENV))
    with pytest.raises(StartupError, match="no database connection"):
        node.handler({"id": 0, "pe": 1})
