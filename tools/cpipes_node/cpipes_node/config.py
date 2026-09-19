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

    Refuses on arrival the one thing the site-operator widening makes possible
    and JetStore refuses too: a built-in token carrying a `site_config`. In Go
    that is `validateSiteOperatorSpec`, and the message it gives is better than
    the one this node would otherwise give, because the widened union lets a
    *malformed* built-in fall through to the site branch and be reported as an
    unknown operator. Re-validating the offending node against the contract's
    own union is what recovers the real error.
    """
    try:
        document = json.loads(config_json)
    except json.JSONDecodeError as exc:
        raise ConfigInvalid(f"pipeline configuration is not JSON: {exc}") from exc
    try:
        config = contract.PipesConfig.model_validate(document)
    except ValidationError as exc:
        raise ConfigInvalid(f"pipeline configuration is not valid:\n{exc}") from exc

    for obj, where in _walk(config):
        if not isinstance(obj, contract.TransformationSpecSite):
            continue
        if obj.type not in contract.contract_tokens("transformation"):
            continue
        # A built-in reached the site branch, which happens exactly when its
        # own configuration failed. Say which, and say it in the built-in's
        # own words.
        detail = ""
        try:
            _validate_as_builtin(obj)
        except ValidationError as exc:
            detail = f"\n{exc}"
        raise ConfigInvalid(
            f"{where}: '{obj.type}' is a built-in operator and cannot be "
            "configured as a site operator; either its own configuration is "
            f"invalid or it carries a site_config.{detail}"
        )
    return config


def _validate_as_builtin(obj: Any) -> None:
    from pydantic import TypeAdapter

    TypeAdapter(contract.TransformationSpec).validate_python(
        obj.model_dump(exclude_none=True)
    )


def _walk(obj: Any, where: str = "$") -> Iterator[tuple[Any, str]]:
    """Every model instance in the tree, with a JSON-ish path.

    Depth-first in field-declaration order, which is Pydantic's own and is
    stable across runs — the findings a gate reports are in document order and
    not in whatever order a set happened to yield.
    """
    if isinstance(obj, BaseModel):
        yield obj, where
        for name in type(obj).model_fields:
            yield from _walk(getattr(obj, name, None), f"{where}.{name}")
    elif isinstance(obj, (list, tuple)):
        for i, item in enumerate(obj):
            yield from _walk(item, f"{where}[{i}]")
    elif isinstance(obj, dict):
        for key in obj:
            yield from _walk(obj[key], f"{where}.{key}")


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
    for obj, where in _walk(config):
        kind_name = contract.spec_kind(obj)
        if kind_name is None:
            continue
        token = getattr(obj, "type", None)
        if not isinstance(token, str):
            continue
        kind = TokenKind(kind_name)
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
