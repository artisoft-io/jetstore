"""Where the `.pc.json` comes from, how it is validated, and the scope gate.

Everything the node reads beyond `{id, jp, pe}` it reads from
`jetsapi.cpipes_execution_status` and from S3 — Michel's characterisation of
the seam, and `actions_coordinate_cp.go` is exactly that: one `SELECT` for the
config and one object store for the data. This module is the first half.

**The gate walks objects, not field paths.** A hand-written walk down
`conditional_pipes_config[].pipes_config[].apply[]` would be a second
description of where a transformation may appear, and it would go stale the
first time JetStore lets one appear somewhere else — with no symptom, because a
gate that walks nothing reports no findings. So the walk recurses over the
validated model tree and asks `contract.spec_kind` what each object is, which
is read off the contract's own class index. `ScopeReport.accepted` records what
it did examine, and `tests_config.py` asserts a count rather than an absence of
findings.
"""

from __future__ import annotations

import json
from collections.abc import Iterator
from dataclasses import dataclass
from pathlib import Path
from typing import Any, Protocol

from pydantic import BaseModel, ValidationError

from . import contract
from .errors import ConfigInvalid, ConfigNotFound
from .scope import FindingKind, ScopeReport, TokenKind, classify
from .scope import declaration as declaration_for
from .site import EMPTY, SiteOperatorRegistry

#: The statement `CoordinateComputePipes` runs, parameterised rather than
#: formatted. `tests_config.py` parses the Go source and asserts the two name
#: the same column, table and key — a second spelling of a query is a second
#: thing to keep right, and this one crosses a language boundary where no
#: compiler will notice.
CONFIG_QUERY = (
    "SELECT cpipes_config_json FROM jetsapi.cpipes_execution_status "
    "WHERE pipeline_execution_status_key = %s"
)


class ConfigSource(Protocol):
    """Where the pipeline configuration document comes from.

    Two implementations, and the local one is not a test double: X2, X3, X5 and
    X7 all need an executable oracle and there is no docker-compose in this
    repository, so a run against a file on disk and a directory standing in for
    S3 is the path every check takes. The S3 and database implementations are
    the same code with a different source.
    """

    def config_json(self, pipeline_execution_key: int) -> str: ...


@dataclass(frozen=True)
class FileConfigSource:
    """A `.pc.json` on disk, for a run with no database.

    The execution key is accepted and ignored, which is the honest shape: the
    caller passes one because the node's arguments carry one, and a file source
    has nothing to look it up in.
    """

    path: Path

    def config_json(self, pipeline_execution_key: int) -> str:
        try:
            return self.path.read_text()
        except FileNotFoundError as exc:
            raise ConfigNotFound(f"no pipeline configuration at {self.path}") from exc


@dataclass(frozen=True)
class ExecutionStatusConfigSource:
    """`jetsapi.cpipes_execution_status`, the way the Go node reads it.

    Takes anything with DB-API `cursor()`, so this module imports no driver and
    the node has no database dependency at import time. That mirrors the Go
    entry, which is handed a `*pgxpool.Pool` rather than opening one.
    """

    connection: Any

    def config_json(self, pipeline_execution_key: int) -> str:
        with self.connection.cursor() as cur:
            cur.execute(CONFIG_QUERY, (pipeline_execution_key,))
            row = cur.fetchone()
        if row is None:
            raise ConfigNotFound(
                "no row in jetsapi.cpipes_execution_status for "
                f"pipeline_execution_status_key = {pipeline_execution_key}"
            )
        return row[0]


def parse_config(config_json: str) -> Any:
    """Validate the document against the contract model.

    **This carried a post-walk refusal until 2026-09-19 and no longer does.**
    While the site branch lived in this package's own widening, its `type` was
    a bare `str` and a *malformed* built-in could fail its own tagged branch,
    satisfy the site branch and arrive as an unknown site operator; the walk
    caught that on arrival and re-raised the built-in's own error. The contract
    model now refuses a built-in token on the complement branch outright
    (`_unlisted`, `cpipes_model.py`), which is JetStore's `validateSiteOperatorSpec`
    rule moved to where the document is read rather than repeated after it — so
    such a document fails `model_validate` with both branches' errors, the
    built-in's own among them, and the walk could no longer fire. A check that
    cannot fire reports a clean result over a subject it can no longer see, so
    it is deleted rather than kept for reassurance.
    `tests_config.test_a_malformed_builtin_is_reported_against_its_own_branch`
    is what holds the replacement to the same promise.
    """
    try:
        document = json.loads(config_json)
    except json.JSONDecodeError as exc:
        raise ConfigInvalid(f"pipeline configuration is not JSON: {exc}") from exc
    try:
        return contract.PipesConfig.model_validate(document)
    except ValidationError as exc:
        raise ConfigInvalid(f"pipeline configuration is not valid:\n{exc}") from exc


def _walk(
    obj: Any, where: str = "$", parent: Any = None
) -> Iterator[tuple[Any, str, Any]]:
    """Every model instance in the tree, with a JSON-ish path and its parent.

    Depth-first in field-declaration order, which is Pydantic's own and is
    stable across runs — the findings a gate reports are in document order and
    not in whatever order a set happened to yield.

    **The parent is the nearest enclosing model instance**, skipping the lists
    and dicts in between, which is what a caller asking "whose input channel is
    this?" wants. It is carried because one judgement genuinely depends on it:
    a `merge_files` pipe fixes its input channel's type rather than choosing it
    (D-224), and the only way to know which pipe an `InputChannelConfig` belongs
    to is to have been told on the way down. Deriving it afterwards from the
    `where` string would be parsing a path this function built, which is two
    spellings of one structure.
    """
    if isinstance(obj, BaseModel):
        yield obj, where, parent
        for name in type(obj).model_fields:
            yield from _walk(getattr(obj, name, None), f"{where}.{name}", obj)
    elif isinstance(obj, (list, tuple)):
        for i, item in enumerate(obj):
            yield from _walk(item, f"{where}[{i}]", parent)
    elif isinstance(obj, dict):
        for key in obj:
            yield from _walk(obj[key], f"{where}.{key}", parent)


def _fixed_input_channel_type(kind: TokenKind, parent: Any) -> str | None:
    """The input-channel type the enclosing pipe fixes, or None if it fixes none.

    Read off the *pipe's own declaration* — `operators.pipes.Pipe
    .fixed_input_channel_type` — so there is no list of pipe kinds here for an
    edit to widen, and a pipe kind added with a fixed channel type is covered by
    its class being written (P3-I20). The pipe's token is the parent's `type`,
    which is the same string the gate would have classified one iteration
    earlier.
    """
    if kind is not TokenKind.INPUT_CHANNEL or parent is None:
        return None
    parent_kind = contract.spec_kind(parent)
    if parent_kind != TokenKind.PIPE.value:
        return None
    declaration = declaration_for(TokenKind.PIPE, getattr(parent, "type", "") or "")
    if declaration is None:
        return None
    return getattr(declaration, "fixed_input_channel_type", "") or None


def check_scope(
    config: Any, site_operators: SiteOperatorRegistry = EMPTY
) -> ScopeReport:
    """Judge every token the document names against the declared scope.

    This is X6's instrument. It runs before any channel is opened, which is
    what "aborts at startup" means and is a guarantee the Go node cannot give:
    there the registry is an argument to the node while the document is seen by
    the starters, so a mistyped token is reported by the dispatch inside a
    running worker (`site_operators.go`, I-779).
    """
    report = ScopeReport()
    site_tokens = site_operators.tokens()
    for obj, where, parent in _walk(config):
        kind_name = contract.spec_kind(obj)
        if kind_name is None:
            continue
        token = getattr(obj, "type", None)
        if not isinstance(token, str):
            continue
        kind = TokenKind(kind_name)
        fixed = _fixed_input_channel_type(kind, parent)
        if fixed is not None:
            # D-224: this channel's type is fixed by JetStore's own validator
            # rather than chosen, so it is asserted and no token is classified.
            # Strictly narrower than classifying it — the fixed value is
            # accepted here and nowhere else in the document.
            if token != fixed:
                raise ConfigInvalid(
                    f"{where}: configuration error: {parent.type} must read from "
                    f"input_channel of type {fixed!r} (this one reads {token!r}). "
                    "The type is fixed by ValidatePipeSpecConfig rather than "
                    "chosen, so it is not a scope question."
                )
            report.accepted.append((kind, token, where))
            continue
        finding = classify(
            kind,
            token,
            where,
            site_tokens=site_tokens,
            contract_tokens=contract.contract_tokens(kind_name),
        )
        if finding is None:
            report.accepted.append((kind, token, where))
        elif finding.category is FindingKind.UNIMPLEMENTED:
            report.unimplemented.append(finding)
        else:
            report.out_of_scope.append(finding)
    return report
