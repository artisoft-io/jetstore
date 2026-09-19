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
    authored_document,
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


def test_the_contract_model_accepts_a_site_operator():
    """The second widening's cause, inverted on 2026-09-19 rather than deleted.

    It read `test_the_contract_model_refuses_a_site_operator` and expected a
    `union_tag_invalid`, with a comment naming the three classes to delete when
    it went red. It went red, they were deleted, and what is left is the claim
    the deletion rests on: the *contract's own* `ComputePipesConfig` — no
    subclass of this package's — validates a document naming a site operator
    and gives back a `TransformationSpecSite`. Inverted rather than removed,
    because a widening retired on an untested premise is a widening that comes
    back.
    """
    doc = authored_document([site_step("healthcare_corpus")])
    config = contract.ComputePipesConfig.model_validate(doc)
    step = config.conditional_pipes_config[0].pipes_config[0].apply[0]
    assert isinstance(step, contract.TransformationSpecSite)
    assert step.type == "healthcare_corpus"


def test_a_site_operator_validates_through_the_widened_model():
    config = contract.PipesConfig.model_validate(document([site_step("hc_corpus")]))
    step = config.conditional_pipes_config[0].pipes_config[0].apply[0]
    assert isinstance(step, contract.TransformationSpecSite)
    assert step.type == "hc_corpus"
    assert contract.spec_kind(step) == "transformation"


def test_every_class_carrying_a_transformation_list_reaches_the_site_branch():
    """Derived from the model, so a new pipe kind is not silently missed.

    This asserted that every `apply` carrier had a subclass here widening it.
    Since the branch joined the alias upstream there is nothing per-carrier to
    widen, and the property worth keeping is the one the widening existed to
    buy: each carrier *accepts* a site operator. Measured through the carrier
    itself rather than through a whole document, so a carrier that gained an
    `apply` list and was missed fails here rather than in whichever document
    happens to exercise it.
    """
    carriers = {
        cls.__name__: cls
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
    assert set(carriers) == {"PipeSpecFanOut", "PipeSpecSplitter"}
    extras = {
        "PipeSpecFanOut": {"type": "fan_out"},
        "PipeSpecSplitter": {
            "type": "splitter",
            "splitter_config": {"column": "a"},
        },
    }
    for name, cls in carriers.items():
        pipe = cls.model_validate(
            {
                "input_channel": {"name": "in", "type": "memory"},
                "apply": [site_step("hc_corpus")],
                **extras[name],
            }
        )
        assert isinstance(pipe.apply[0], contract.TransformationSpecSite), name


def test_a_malformed_builtin_is_reported_against_its_own_branch():
    """A built-in that fails its own branch must not surface as an unknown
    operator — the residual hazard of any ordered union whose last branch keys
    on a bare `str`.

    Closed in the model rather than after it: `TransformationSpecSite.type`
    refuses a built-in token outright, which is `validateSiteOperatorSpec`'s
    rule. So the document fails validation naming the *built-in's own* missing
    field, and `parse_config`'s post-walk refusal — which re-derived that error
    by re-validating the offending node — is deleted.
    """
    doc = document(
        [{"type": "partition_writer", "output_channel": MEMORY_CHANNEL}]
    )
    with pytest.raises(ConfigInvalid) as exc:
        parse_config(json.dumps(doc))
    message = str(exc.value)
    # The built-in's own branch, naming the field it is missing.
    assert "partition_writer.partition_writer_config" in message
    assert "Field required" in message
    # And the site branch refusing it by name rather than accepting it.
    assert "'partition_writer' is a built-in operator" in message


def test_a_builtin_carrying_a_site_config_is_refused():
    # JetStore's `validateSiteOperatorSpec` refuses a `site_config` on a token
    # the dispatch handles itself; the model refuses it twice over — the
    # built-in's branch forbids the extra key and the site branch forbids the
    # built-in token.
    doc = document(
        [{"type": "map_record", "output_channel": MEMORY_CHANNEL, "site_config": {}}]
    )
    with pytest.raises(ConfigInvalid) as exc:
        parse_config(json.dumps(doc))
    message = str(exc.value)
    assert "map_record.site_config" in message
    assert "Extra inputs are not permitted" in message
    assert "'map_record' is a built-in operator" in message


def test_a_site_operator_may_not_take_a_builtin_name():
    # The complement branch's membership, asserted on the branch itself: a
    # `json_schema_extra` `not: enum` is read by the emitted JSON Schema and by
    # nothing in Pydantic, so without `_unlisted` this passes.
    for token in contract.contract_tokens("transformation"):
        with pytest.raises(ValidationError, match="is a built-in operator"):
            contract.TransformationSpecSite.model_validate(
                {"type": token, "output_channel": MEMORY_CHANNEL}
            )


def test_a_site_operator_may_not_be_nameless():
    # `not: enum` admits the empty string and `WithOperators` refuses to
    # register an empty name, so the branch carries `minLength: 1` beside it;
    # this is that half at validation time.
    with pytest.raises(ValidationError, match="cannot be empty"):
        contract.TransformationSpecSite.model_validate(
            {"type": "", "output_channel": MEMORY_CHANNEL}
        )


# --- the corpus -------------------------------------------------------------


def test_the_authored_corpus_validates_through_both_models():
    """Every authored `.pc.json` in `workspaces/` validates through both models.

    **The count is pinned and the pin is the point**: a corpus that quietly
    shrank would make this pass over fewer documents, which is the standing class
    this repository names oftenest. So the number is re-derived when it moves and
    the move is explained, never widened to a `>=`.

    **48 -> 50 on 2026-09-19**, and the cause is worth recording because it is not
    this repository's: `healthcare_corpus`'s Phase 9 authored `qc_denial_share.pc.json`
    and `qc_succession.pc.json` into `jets_ws` (its P9-T12, D-203's two population
    checks becoming authored QC pipelines). **So a merge in one repository turned a
    test red in another**, on a branch that did not cause it — which is P9-I10's
    shape at the level of a count, recorded there as **P9-I71**.

    **50 -> 51 the same day, and the second move is the interesting one: P9-I71
    predicted its own recurrence and nothing prevented it.** `healthcare_corpus`'s
    P9-T18 authored `healthcare_corpus.pc.json` into `jets_ws`, and merging it
    turned this red again -- found by an agent whose branch did not cause it, on
    a baseline it measured before editing anything. **Two merges, two repositories,
    one direction, and no instrument connects them**: nothing in `jets_ws` runs
    this suite and nothing here watches that workspace. Recording the recurrence
    rather than widening the pin, because the pin is what made both visible at
    all -- a `>=` would have made the corpus able to shrink in silence, which is
    the class this docstring opens by naming.
    """
    corpus = pipes_config_corpus()
    assert len(corpus) == 51, "the authored corpus moved; re-derive the count"
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


def test_the_gate_reports_all_three_kinds(declared_and_unbuilt):
    doc = document([out_of_scope_step(), site_step("hc_corpus")])
    report = check_scope(
        contract.PipesConfig.model_validate(doc),
        site_operators=Registry().with_operators({"hc_corpus": lambda e, s: None}),
    )
    kinds = {kind for kind, _, _ in report.accepted} | {
        f.kind for f in report.out_of_scope + report.unimplemented
    }
    assert kinds == set(TokenKind)
    # The site operator is accepted, and so are `fan_out` and `generator` since
    # P9-T04 gave both a `build`; the fixture's token is declared and not built.
    # The order is the model's field order and not the JSON's — `PipeSpecFanOut`
    # declares `apply` before `input_channel` — which is stable and is what
    # "deterministic" has to mean here; it is asserted rather than described so
    # that a walk that started yielding in set order would be caught.
    #
    # **The unimplemented list read `["fan_out", "map_record", "generator"]`
    # until P9-T04 and `["map_record"]` until P9-T06.** Tokens keep moving from
    # that list to the accepted one, and the ordering claim is what survives
    # every move: the transformation still precedes `generator`, wherever each
    # one lands. The unimplemented exemplar is now a declaration the fixture
    # makes, because a test whose subject is the unfinished token has a subject
    # that empties.
    assert [f.token for f in report.unimplemented] == ["aggregate"]
    assert report.out_of_scope == []
    assert [t for _, t, _ in report.accepted] == ["fan_out", "hc_corpus", "generator"]


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
