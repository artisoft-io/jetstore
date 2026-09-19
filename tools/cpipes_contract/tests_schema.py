"""The emitted schema and the model, held to each other.

**They are two readers of one contract and they disagreed for six days.**
Gap 2b (`I-778`) put the `~site` complement branch into `types.csv` and into
the emitted JSON Schema, and left `cpipes_model.py`'s `TransformationSpec`
alias carrying only the nineteen tagged members. Nothing noticed, because until
2026-09-19 no authored `.pc.json` in JetStore's own corpus named a site
operator: the schema said yes, the model said no, and no document asked both.
`workspaces/jets_ws/pipes_config/healthcare_corpus.pc.json` is the document
that asked, and `cpipes_node`'s corpus test is where it was heard.

So the tests here are about *agreement* rather than about either artefact, and
the two that matter are the last two: a document the schema accepts, the model
accepts, and a document the schema refuses, the model refuses. Everything above
them is the mechanism that makes that true.
"""

from __future__ import annotations

import json
from pathlib import Path

import jsonschema
import pytest
from pydantic import ValidationError

from cpipes_contract.reflect import load_model
from cpipes_contract.schema import emit

HERE = Path(__file__).parent
MATRIX = HERE / "matrix"
MODEL_PATH = HERE / "cpipes_model.py"
COMMITTED = (HERE / "cpipes_schema.json").read_text()

model = load_model(MODEL_PATH)

#: The document shape used below: one step, one fan-out pipe, one operator.
#: Deliberately the smallest thing that reaches `apply`, because a bigger
#: fixture would let a refusal about some other field pass for a refusal about
#: this one.
MEMORY_CHANNEL = {"name": "out", "type": "memory", "channel_spec_name": "out"}


def document(step: dict) -> dict:
    return {
        "channels": [{"name": "in", "columns": ["a"]}, {"name": "out", "columns": ["a"]}],
        "conditional_pipes_config": [
            {
                "step_name": "s",
                "pipes_config": [
                    {
                        "type": "fan_out",
                        "input_channel": {"name": "in", "type": "memory"},
                        "apply": [step],
                    }
                ],
            }
        ],
    }


SITE_STEP = {
    "type": "healthcare_corpus",
    "output_channel": MEMORY_CHANNEL,
    "site_config": {"config": {"anything": True}},
}


def emitted() -> str:
    return json.dumps(emit(model, MATRIX / "types.csv"), indent=2, sort_keys=True) + "\n"


def test_the_committed_schema_is_what_the_current_model_emits():
    """The artefact in the tree, against the emitter that produced it.

    No test asserted this before 2026-09-19, so `cpipes_schema.json` could have
    been stale against the model for any length of time and every consumer of
    it — `validate/main.go`, the fragment library, the bundles, the negative
    suite — would have been reading a file nothing checked.
    """
    assert emitted() == COMMITTED


def test_folding_the_complement_branch_changes_no_byte_of_the_schema():
    """The claim the model's widening rests on.

    The alias is `Annotated[Union[<tagged>, <branch>], left_to_right]`, which
    Pydantic writes as a two-member `anyOf`;
    `schema.splice_complement_branches` folds that back into the single
    discriminated `oneOf` the schema has carried since gap 2b. If the fold
    stopped being exact, every consumer of the emitted file would move while
    the model stood still — which is the disagreement this module exists to
    prevent, in the other direction.

    Asserted against the *committed* bytes rather than against a recomputation,
    so the property is "the schema did not move", not "the emitter agrees with
    itself".
    """
    schema = json.loads(COMMITTED)
    union = schema["$defs"]["TransformationSpec"]
    assert set(union) == {"oneOf", "discriminator"}
    assert union["oneOf"][-1] == {"$ref": "#/$defs/TransformationSpecSite"}
    assert len(union["discriminator"]["mapping"]) == len(union["oneOf"]) - 1


def test_the_fold_refuses_a_union_that_is_not_widened():
    """The mutation: un-widen the alias and the fold must say so.

    Before 2026-09-19 this function *added* the branch to a tagged union, so an
    un-widened alias was its normal input and it could not tell the two apart.
    It now refuses, which is what makes `test_..._changes_no_byte_...` evidence
    rather than a coincidence — a fold that silently accepted both shapes would
    pass whether or not the model carried the branch.
    """
    from cpipes_contract.schema import splice_complement_branches

    defs = {
        "TransformationSpec": {
            "oneOf": [{"$ref": "#/$defs/TransformationSpecFilter"}],
            "discriminator": {"propertyName": "type", "mapping": {}},
        }
    }
    with pytest.raises(ValueError, match="two-member anyOf"):
        splice_complement_branches(defs, MATRIX / "types.csv")


def test_the_complement_branch_reaches_every_occurrence_of_the_union():
    """Not only the addressable `$defs` entry.

    `TransformationSpec` is an alias rather than a model, so Pydantic inlines
    the whole union at each use site; folding the named entry alone leaves a
    schema whose addressable entry admits a site operator and whose documents
    do not. Counted rather than spot-checked, because "some occurrence was
    missed" and "every occurrence was folded" look identical at one call site.
    """
    schema = json.loads(COMMITTED)
    branch = {"$ref": "#/$defs/TransformationSpecSite"}
    found = []

    def walk(node):
        if isinstance(node, dict):
            one_of = node.get("oneOf")
            if isinstance(one_of, list) and branch in one_of:
                found.append(node)
            for value in node.values():
                walk(value)
        elif isinstance(node, list):
            for value in node:
                walk(value)

    walk(schema)
    # The named entry plus the `apply` arrays of the two pipe kinds that carry
    # one. A bundle's narrowed union is deliberately not among them.
    assert len(found) == 3
    assert not any("anyOf" in node for node in found)


# --- the two readers, on the same documents ---------------------------------


def _schema_errors(doc: dict) -> list[str]:
    validator = jsonschema.Draft202012Validator(json.loads(COMMITTED))
    return [e.message for e in validator.iter_errors(doc)]


def test_both_readers_accept_a_site_operator():
    doc = document(SITE_STEP)
    assert _schema_errors(doc) == []
    config = model.ComputePipesConfig.model_validate(doc)
    step = config.conditional_pipes_config[0].pipes_config[0].apply[0]
    assert isinstance(step, model.TransformationSpecSite)


@pytest.mark.parametrize(
    "step, why",
    [
        (
            {"type": "map_record", "output_channel": MEMORY_CHANNEL, "site_config": {}},
            "a built-in carrying a site_config -- validateSiteOperatorSpec's own rule",
        ),
        (
            {"type": "", "output_channel": MEMORY_CHANNEL},
            "an empty type, which is the ~override shape and reaches no registry",
        ),
        (
            {"type": "healthcare_corpus"},
            "a site operator with no output_channel, which validateOutputChConfig "
            "requires of every transformation",
        ),
    ],
)
def test_both_readers_refuse_the_same_documents(step, why):
    """The half that fails silently if only one reader is checked.

    Each case is a thing the *engine* refuses, and the point is that the two
    contract artefacts refuse it too and refuse it together. A case the schema
    caught and the model did not is exactly the state gap 2b left behind, one
    direction over.
    """
    doc = document(step)
    assert _schema_errors(doc), why
    with pytest.raises(ValidationError):
        model.ComputePipesConfig.model_validate(doc)


def test_the_models_refusal_of_a_builtin_name_is_the_schemas_not_enum():
    """One list, two readers, asserted to be one list.

    `_unlisted` and the branch's `json_schema_extra` both take
    `_TRANSFORMATION_SPEC_TOKENS`, and this is the assertion that they are the
    same tokens as the union's own members — the drift `builtinOperatorTypes`
    and `reportsRowLevelFailures` each have a test against on the Go side.
    """
    schema = json.loads(COMMITTED)
    site = schema["$defs"]["TransformationSpecSite"]["properties"]["type"]
    excluded = site["not"]["enum"]
    mapping = schema["$defs"]["TransformationSpec"]["discriminator"]["mapping"]
    assert sorted(excluded) == sorted(mapping)
    for token in excluded:
        with pytest.raises(ValidationError, match="is a built-in operator"):
            model.TransformationSpecSite.model_validate(
                {"type": token, "output_channel": MEMORY_CHANNEL}
            )


def test_a_site_operator_carrying_a_builtins_config_block_is_refused():
    # `siteOperatorArgs` reads none of the eighteen `*_config` pointers, which
    # the matrix records as inapplicable on `~site`; `extra="forbid"` is what
    # turns that record into a refusal.
    doc = document({**SITE_STEP, "map_record_config": {}})
    assert _schema_errors(doc)
    with pytest.raises(ValidationError):
        model.ComputePipesConfig.model_validate(doc)
