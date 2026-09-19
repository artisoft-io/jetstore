"""The config load, and the three places the contract model had to be widened.

Every widening is asserted against the thing that caused it, so that none of
them can outlive its cause: the runtime fields against the Go struct's tags,
the site union against the contract's own refusal, and the set of classes
carrying a transformation list against the model by reflection. When JetStore
closes one of these, the test that pins it goes red and says what to delete.
"""

from __future__ import annotations

import json
import re
from pathlib import Path

import pytest
from pydantic import BaseModel, ValidationError

from conftest import (
    MEMORY_CHANNEL,
    document,
    go_source,
    map_record_step,
    out_of_scope_step,
    pipes_config_corpus,
    runtime_document,
    site_step,
)
from cpipes_node import contract
from cpipes_node.config import (
    CONFIG_QUERY,
    ExecutionStatusConfigSource,
    FileConfigSource,
    check_scope,
    parse_config,
)
from cpipes_node.errors import ConfigInvalid, ConfigNotFound
from cpipes_node.scope import TokenKind
from cpipes_node.site import Registry

# --- the contract model, and what it does not cover -------------------------


def test_the_contract_model_is_still_not_importable():
    """`cpipes_model.py` is a sibling file of the package, not a module in it.

    When this goes red, `cpipes_model` became importable: delete the path
    fallback in `contract._load_contract_model` and delete this test. Pinned
    rather than worked around silently, because the consequence reaches the
    container image (P9-T14) — a wheel of `cpipes_contract` does not carry the
    file at all.
    """
    with pytest.raises(ImportError):
        __import__("cpipes_contract.cpipes_model")


def test_the_runtime_only_fields_are_exactly_the_two_widened():
    src = go_source("jets/compute_pipes/pipes_model.go")
    body = re.search(r"type ComputePipesConfig struct \{(.*?)\n\}", src, re.DOTALL)
    assert body is not None
    go_tags = set(re.findall(r'json:"([A-Za-z0-9_]+)', body.group(1)))
    authored = set(contract.ComputePipesConfig.model_fields)
    runtime_only = go_tags - authored
    added = set(contract.PipesConfig.model_fields) - authored
    assert runtime_only == added == {"common_runtime_args", "pipes_config"}
    # And the model carries nothing the Go struct does not, so the widening is
    # the whole of the difference in both directions.
    assert authored - go_tags == set()


def test_the_contract_model_refuses_the_document_a_node_is_handed():
    # The finding this widening exists for: `cpipes-contract check --corpus`
    # walks authored documents, all of which validate, and a node never sees
    # one of those.
    doc = runtime_document([map_record_step()])
    with pytest.raises(ValidationError):
        contract.ComputePipesConfig.model_validate(doc)
    assert contract.PipesConfig.model_validate(doc).common_runtime_args is not None


def test_the_contract_model_refuses_a_site_operator():
    # The second widening's cause. When this goes red the contract's union
    # gained `~site`: delete `TransformationSpecOrSite` and the two pipe
    # subclasses in contract.py.
    doc = document([site_step("healthcare_corpus")])
    with pytest.raises(ValidationError, match="union_tag_invalid"):
        contract.ComputePipesConfig.model_validate(doc)


def test_a_site_operator_validates_through_the_widened_model():
    config = contract.PipesConfig.model_validate(document([site_step("hc_corpus")]))
    step = config.conditional_pipes_config[0].pipes_config[0].apply[0]
    assert isinstance(step, contract.TransformationSpecSite)
    assert step.type == "hc_corpus"
    assert contract.spec_kind(step) == "transformation"


def test_every_class_carrying_a_transformation_list_is_widened():
    """Derived from the model, so a new pipe kind is not silently missed."""
    carriers = {
        cls.__name__
        for cls in vars(contract.model).values()
        if isinstance(cls, type)
        and issubclass(cls, BaseModel)
        and any(
            "TransformationSpec" in str(f.annotation)
            and "TransformationColumnSpec" not in str(f.annotation)
            and name == "apply"
            for name, f in cls.model_fields.items()
        )
    }
    widened_bases = {
        base.__name__
        for cls in contract.WIDENED_CLASSES
        for base in cls.__mro__[1:]
        if base.__module__ == "cpipes_model"
    }
    assert carriers == {"PipeSpecFanOut", "PipeSpecSplitter"}
    assert carriers <= widened_bases


def test_a_malformed_builtin_does_not_arrive_as_a_site_operator():
    # The price of a left-to-right union: a built-in that fails its own branch
    # can satisfy the site branch, whose `type` is a bare `str`. Refused on
    # arrival, with the built-in's own error attached, which is also JetStore's
    # rule — `validateSiteOperatorSpec` refuses a `site_config` on a built-in.
    doc = document(
        [{"type": "map_record", "output_channel": MEMORY_CHANNEL, "site_config": {}}]
    )
    with pytest.raises(ConfigInvalid) as exc:
        parse_config(json.dumps(doc))
    assert "map_record" in str(exc.value)
    assert "built-in" in str(exc.value)


# --- the corpus -------------------------------------------------------------


def test_the_authored_corpus_validates_through_both_models():
    corpus = pipes_config_corpus()
    assert len(corpus) == 48, "the authored corpus moved; re-derive the count"
    for path in corpus:
        doc = json.loads(path.read_text())
        contract.ComputePipesConfig.model_validate(doc)
        contract.PipesConfig.model_validate(doc)


def test_the_gate_examines_the_corpus_rather_than_passing_over_it():
    # A gate that walked nothing reports no findings and looks exactly like a
    # clean pass. So the assertion is a count of what it examined.
    examined = 0
    for path in pipes_config_corpus():
        report = check_scope(
            contract.PipesConfig.model_validate(json.loads(path.read_text()))
        )
        examined += (
            len(report.accepted) + len(report.out_of_scope) + len(report.unimplemented)
        )
    assert examined > 400, examined


def test_the_gate_reports_all_three_kinds():
    doc = document([map_record_step(), site_step("hc_corpus")])
    report = check_scope(
        contract.PipesConfig.model_validate(doc),
        site_operators=Registry().with_operators({"hc_corpus": lambda e, s: None}),
    )
    kinds = {kind for kind, _, _ in report.accepted} | {
        f.kind for f in report.out_of_scope + report.unimplemented
    }
    assert kinds == set(TokenKind)
    # The site operator is accepted; the generator, the fan_out and the
    # map_record are declared and not built. The order is the model's field
    # order and not the JSON's — `PipeSpecFanOut` declares `apply` before
    # `input_channel` — which is stable and is what "deterministic" has to mean
    # here; it is asserted rather than described so that a walk that started
    # yielding in set order would be caught.
    assert [f.token for f in report.unimplemented] == [
        "fan_out",
        "map_record",
        "generator",
    ]
    assert report.out_of_scope == []
    assert [t for _, t, _ in report.accepted] == ["hc_corpus"]


def test_an_out_of_scope_token_is_located_in_the_document():
    doc = document([out_of_scope_step()])
    report = check_scope(contract.PipesConfig.model_validate(doc))
    assert len(report.out_of_scope) == 1
    finding = report.out_of_scope[0]
    assert finding.token == "aggregate"
    assert finding.where.endswith("apply[0]")


def test_findings_are_in_document_order():
    doc = document([out_of_scope_step(), map_record_step(), out_of_scope_step()])
    report = check_scope(contract.PipesConfig.model_validate(doc))
    assert [f.where for f in report.out_of_scope] == [
        f.where for f in sorted(report.out_of_scope, key=lambda f: f.where)
    ]
    assert report.out_of_scope[0].where.endswith("apply[0]")
    assert report.out_of_scope[1].where.endswith("apply[2]")


# --- the sources ------------------------------------------------------------


def test_the_config_query_names_what_the_go_node_names():
    src = go_source("jets/compute_pipes/actions_coordinate_cp.go")
    go_stmt = re.search(r'stmt := "(SELECT cpipes_config_json[^"]+)"', src)
    assert go_stmt is not None, "the Go node's config query moved"
    normalise = lambda s: re.sub(r"(%d|%s)", "?", " ".join(s.split()))
    assert normalise(CONFIG_QUERY) == normalise(go_stmt.group(1))


def test_a_file_source_reads_the_document(config_file):
    path = config_file(document([map_record_step()]))
    assert '"map_record"' in FileConfigSource(path).config_json(0)


def test_a_missing_file_is_a_startup_refusal(tmp_path: Path):
    with pytest.raises(ConfigNotFound):
        FileConfigSource(tmp_path / "absent.pc.json").config_json(1)


class _Cursor:
    def __init__(self, row):
        self.row = row
        self.seen = None

    def __enter__(self):
        return self

    def __exit__(self, *exc):
        return False

    def execute(self, sql, params):
        self.seen = (sql, params)

    def fetchone(self):
        return self.row


class _Connection:
    def __init__(self, row):
        self._cursor = _Cursor(row)

    def cursor(self):
        return self._cursor


def test_the_execution_status_source_parameterises_the_key():
    # Parameterised and not formatted. The Go node interpolates an int with
    # %d, which is safe there because it is an int; this one is handed to a
    # driver, and a formatted key would be a string concatenation on a path
    # that reaches a database.
    conn = _Connection(('{"channels": []}',))
    assert ExecutionStatusConfigSource(conn).config_json(17) == '{"channels": []}'
    assert conn._cursor.seen == (CONFIG_QUERY, (17,))


def test_no_row_for_the_execution_key_is_a_startup_refusal():
    with pytest.raises(ConfigNotFound, match="17"):
        ExecutionStatusConfigSource(_Connection(None)).config_json(17)


def test_a_document_that_is_not_json_is_refused():
    with pytest.raises(ConfigInvalid, match="not JSON"):
        parse_config("{not json")


def test_a_document_that_is_not_a_pipeline_is_refused():
    with pytest.raises(ConfigInvalid, match="not valid"):
        parse_config(json.dumps({"channels": [{"no_name": 1}]}))
