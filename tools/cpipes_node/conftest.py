"""Shared fixtures, and the two paths every test in this package resolves.

The Go tree is found by walking up from this file rather than by an env
variable, because four tests read Go source as their oracle — the struct tags,
the config query, the argument struct — and a test whose oracle is optional is
a test that passes when the oracle is missing.
"""

from __future__ import annotations

import json
from pathlib import Path
from typing import Any

import pytest

HERE = Path(__file__).resolve().parent
#: tools/cpipes_node -> tools -> the jetstore module root.
JETSTORE_ROOT = HERE.parent.parent


def _superproject() -> Path:
    """The checkout holding `workspaces/`, found by walking up.

    Not `JETSTORE_ROOT.parent`: in a git worktree the submodule's root sits
    four directories below the superproject rather than one, and a relative
    hop that is right in one layout and wrong in the other produces an empty
    corpus — which every test over it would then pass on.
    """
    for candidate in (JETSTORE_ROOT, *JETSTORE_ROOT.parents):
        if any((candidate / "workspaces").glob("*/pipes_config")):
            return candidate
    raise AssertionError(
        f"no workspaces/*/pipes_config above {JETSTORE_ROOT}; the corpus every "
        "contract test reads is not reachable from this checkout"
    )


SUPERPROJECT = _superproject()


def go_source(relative: str) -> str:
    path = JETSTORE_ROOT / relative
    assert path.is_file(), f"the Go oracle is missing: {path}"
    return path.read_text()


def pipes_config_corpus() -> tuple[Path, ...]:
    """Every authored `.pc.json` in the workspaces, sorted.

    The same corpus `cpipes-contract corpus` walks, minus its
    `jets_assets_manifest.json` filtering, which is about what JetStore owns
    rather than about what validates.
    """
    return tuple(sorted((SUPERPROJECT / "workspaces").glob("*/pipes_config/*.pc.json")))


MEMORY_CHANNEL = {"name": "out", "type": "memory", "channel_spec_name": "out"}

#: The generator's record width, and the columns a channel declaring
#: `same_columns_as_input` takes. `LoadMainInput` sends
#: `make([]any, len(mainInput.InputColumns))` per generated row, so a document
#: with no main-input columns generates rows of no width — which is why both
#: fixtures below carry it and why `graph._main_input_columns` refuses its
#: absence rather than defaulting it.
MAIN_INPUT_COLUMNS = ["a"]

#: The channel spec the generator's own input channel takes its shape from. Its
#: columns are `MAIN_INPUT_COLUMNS` deliberately: a generator step whose channel
#: declares more columns than the main input hands every downstream reader a
#: record shorter than its own column map.
GENERATOR_CHANNEL_SPEC = {"name": "in", "columns": MAIN_INPUT_COLUMNS}

_SOURCES_CONFIG = {"main_input": {"input_columns": MAIN_INPUT_COLUMNS}}


def document(apply: list[dict[str, Any]], channel_type: str = "generator") -> dict:
    """A minimal authored document with one step and one pipe.

    Deliberately minimal: every test that wants a token in a particular place
    builds its own `apply`, and a shared fixture carrying six operators would
    make a test that meant to examine one of them pass on another.

    **Minimal and runnable, which it was not until P9-T04.** It grew the
    generator channel's own spec and `common_runtime_args`, because a document
    that stops at the scope gate needs neither and one that runs needs both — and
    a fixture that cannot run is a fixture that can only ever test a refusal.
    """
    return {
        "common_runtime_args": {
            "cpipes_mode": "reducing",
            "session_id": "s1",
            "sources_config": _SOURCES_CONFIG,
        },
        "channels": [GENERATOR_CHANNEL_SPEC, {"name": "out", "columns": ["a"]}],
        "schema_providers": [
            {"type": "default", "key": "main", "source_type": "main_input"}
        ],
        "conditional_pipes_config": [
            {
                "step_name": "s",
                "pipes_config": [
                    {
                        "type": "fan_out",
                        "input_channel": {
                            "name": "in",
                            "type": channel_type,
                            **(
                                {"nbr_nodes": 1, "nbr_rows": 10}
                                if channel_type == "generator"
                                else {}
                            ),
                        },
                        "apply": apply,
                    }
                ],
            }
        ],
    }


def runtime_document(apply: list[dict[str, Any]], mode: str = "reducing") -> dict:
    """What a starter writes into `cpipes_execution_status`.

    `common_runtime_args` and a flat `pipes_config`, which is the pair the
    contract's own model does not carry. `sources_config` is inside the first of
    those and is what the generator's record width comes from.
    """
    return {
        "common_runtime_args": {
            "cpipes_mode": mode,
            "session_id": "s1",
            "sources_config": _SOURCES_CONFIG,
        },
        "channels": [GENERATOR_CHANNEL_SPEC, {"name": "out", "columns": ["a"]}],
        "schema_providers": [
            {"type": "default", "key": "main", "source_type": "main_input"}
        ],
        "pipes_config": [
            {
                "type": "fan_out",
                "input_channel": {
                    "name": "in",
                    "type": "generator",
                    "nbr_nodes": 1,
                    "nbr_rows": 10,
                },
                "apply": apply,
            }
        ],
    }


def site_step(token: str) -> dict:
    return {
        "type": token,
        "output_channel": MEMORY_CHANNEL,
        "site_config": {"config": {"anything": True}},
    }


def map_record_step() -> dict:
    return {"type": "map_record", "output_channel": MEMORY_CHANNEL, "columns": []}


def out_of_scope_step() -> dict:
    """A built-in this node does not declare, in its minimally valid form.

    `aggregate` and not `sort`, because a step must satisfy the contract before
    the scope gate ever sees it — a document that does not validate is refused
    by `parse_config` and would test the wrong refusal. Measured: of JetStore's
    nineteen transformation tokens, three validate without an operator-specific
    config block, and `aggregate` is the only one of those three this node does
    not declare.
    """
    return {
        "type": "aggregate",
        "output_channel": MEMORY_CHANNEL,
        "new_record": True,
    }


@pytest.fixture
def declared_and_unbuilt():
    """Make `aggregate` a declaration with no `build`, for the length of one test.

    **Every test of the declared-and-not-implemented refusal named a real token,
    and P9-T06 is when that stopped working**: `map_record`, `filter` and
    `partition_writer` all carry a `build` now, and the one declaration still
    without one is `merge_files`, which P9-T08 is landing. A test whose subject is
    *which token happens to be unfinished* has a subject that empties as the
    phase progresses — so the subject is a declaration the test makes itself, and
    the refusal stays exercised when the scope is complete.

    `aggregate` is the token because `out_of_scope_step()` already builds a valid
    one: it is the only contract transformation this node does not declare that
    validates with no operator-specific config block, which that function's own
    docstring measured.
    """
    from cpipes_node.scope import _REGISTRY, Operator, TokenKind

    key = (TokenKind.TRANSFORMATION, "aggregate")
    assert key not in _REGISTRY, "aggregate is declared now; pick another token"

    class Unbuilt(Operator):
        kind = TokenKind.TRANSFORMATION
        token = "aggregate"
        owed_by = "a test of the unimplemented refusal"

    try:
        yield Unbuilt
    finally:
        del _REGISTRY[key]


@pytest.fixture
def config_file(tmp_path: Path):
    def write(doc: dict) -> Path:
        path = tmp_path / "pipeline.pc.json"
        path.write_text(json.dumps(doc))
        return path

    return write
