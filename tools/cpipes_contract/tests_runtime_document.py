"""The runtime document, and the instrument that would have found its absence.

**A green corpus is what hid this.** `cpipes-contract check --corpus` walks
`workspaces/*/pipes_config/**` - authored documents, every one of which
validates - and a worker node is never handed one of those. A starter marshals
a `ComputePipesConfig` with `common_runtime_args` and `pipes_config` set into
`cpipes_execution_status.cpipes_config_json`, `extra="forbid"` refused it, and
nothing in this package had a subject of that shape to be red about. The gap
survived for as long as it did because every check that could have seen it was
pointed at the other half of the document space.

So these tests are about the *split* rather than about two field names. Three
of them derive their subject from somewhere else - the Go struct's json tags,
the matrix's `applicable` column, the emitted schema - so the sixteenth tag
added to `ComputePipesConfig` in Go goes red here on the day it lands rather
than on the day a node is handed one. A test naming the pair would pass for
ever afterwards while the same gap reopened one field over, which is the whole
reason `AD` is four tasks for a two-line repair.

**And two of them are about what the repair must *not* do.** The authored
model and the emitted schema go on refusing the runtime shape;
`negative_suite.json`'s *root pipes_config (I-14 runtime shape)* case expects
exactly that, and a repair that widened `ComputePipesConfig` instead would have
made that negative start passing. The refusal is the contract, not an
oversight.

Written for `jetstore_maintenance_01` Phase 1, track `AD`, closing
`healthcare_corpus`'s `P9-I28` (read 2026-09-20).
"""

from __future__ import annotations

import csv
import json
import re
from pathlib import Path

import pytest
from pydantic import ValidationError

from cpipes_contract.reflect import load_model

HERE = Path(__file__).parent
#: `tools/cpipes_contract` -> `tools` -> the JetStore tree.
JETSTORE_ROOT = HERE.parents[1]
MODEL_PATH = HERE / "cpipes_model.py"
FIELDS_CSV = HERE / "matrix" / "fields.csv"
SCHEMA_PATH = HERE / "cpipes_schema.json"

model = load_model(MODEL_PATH)

#: The one operator the fixtures reach. Deliberately the smallest thing that
#: gets past `apply`: a larger fixture lets a refusal about some other field
#: pass for a refusal about the one under test.
MAP_RECORD = {
    "type": "map_record",
    "output_channel": {"type": "sql", "output_table_key": "t"},
}

PIPE = {"type": "fan_out", "input_channel": {"name": "i"}, "apply": [MAP_RECORD]}


def authored_document() -> dict:
    """What an author writes, and what the corpus walk validates."""
    return {"conditional_pipes_config": [{"step_name": "s", "pipes_config": [PIPE]}]}


def runtime_document() -> dict:
    """What a starter writes into `cpipes_execution_status.cpipes_config_json`.

    `common_runtime_args` and a **flat** `pipes_config`: the step is already
    chosen by the time a node reads it, which is why the two fields are the
    pair and not either one alone.
    """
    return {
        "common_runtime_args": {"cpipes_mode": "reducing", "session_id": "s1"},
        "pipes_config": [PIPE],
    }


def go_json_tags(struct: str) -> set[str]:
    """The json tags of one struct in `pipes_model.go` - the producing shape.

    Parsed rather than listed, so this file holds no copy of the inventory it
    is checking. `pipes_model.go` is the Go side of the same document and its
    struct is *both* shapes at once, which is the fact the split exists to
    express.
    """
    src = (JETSTORE_ROOT / "jets" / "compute_pipes" / "pipes_model.go").read_text()
    body = re.search(rf"type {struct} struct \{{(.*?)\n\}}", src, re.DOTALL)
    assert body is not None, f"{struct} is no longer declared in pipes_model.go"
    return set(re.findall(r'json:"([A-Za-z0-9_]+)', body.group(1)))


def inapplicable_json_keys(struct: str) -> set[str]:
    """The struct's fields the matrix marks `applicable=no`.

    The matrix is where the split is *recorded* - `common_runtime_args` and
    `pipes_config` carry `applicable=no` with a note saying a schema admitting
    them would permit the runtime shape - so it is the matrix, not this file,
    that decides which fields the runtime model adds.
    """
    with open(FIELDS_CSV, newline="") as fh:
        return {
            row["json_key"]
            for row in csv.DictReader(fh)
            if row["go_struct"] == struct and row["applicable"] == "no"
        }


def test_the_runtime_model_carries_every_json_tag_the_go_struct_declares():
    """The instrument. A sixteenth tag in Go is red here the day it lands.

    Both directions: the runtime model carries nothing Go does not either, so a
    field deleted from the struct is caught by the same assertion rather than
    surviving as a key the model quietly still accepts.
    """
    tags = go_json_tags("ComputePipesConfig")
    runtime = set(model.ComputePipesRuntimeConfig.model_fields)
    assert tags - runtime == set(), "json tags the runtime model does not accept"
    assert runtime - tags == set(), "runtime model fields the Go struct does not declare"


def test_the_split_between_the_two_models_is_the_one_the_matrix_records():
    """What the runtime model adds is what the matrix calls inapplicable.

    Derived rather than typed. A field marked `applicable=no` tomorrow joins
    the runtime model or this goes red, so the two artefacts cannot drift into
    the arrangement that produced `P9-I28` - a contract that knows a field is
    never authored and no model that accepts it anyway.
    """
    authored = set(model.ComputePipesConfig.model_fields)
    runtime = set(model.ComputePipesRuntimeConfig.model_fields)
    assert runtime - authored == inapplicable_json_keys("ComputePipesConfig")
    assert authored - runtime == set()


def test_the_runtime_model_accepts_the_document_a_node_is_handed():
    config = model.ComputePipesRuntimeConfig.model_validate(runtime_document())
    assert config.common_runtime_args is not None
    assert config.common_runtime_args.cpipes_mode == "reducing"
    assert config.pipes_config is not None
    assert config.pipes_config[0].type == "fan_out"


def test_the_runtime_model_still_accepts_an_authored_document():
    """The widening is additive: one model reads both shapes.

    A runtime model that had stopped accepting authored documents would be a
    third reader of the contract rather than a second projection of it, and the
    `cpipes_node` corpus test that validates all of JetStore's own `.pc.json`
    through this class is what would find out.
    """
    config = model.ComputePipesRuntimeConfig.model_validate(authored_document())
    assert config.common_runtime_args is None
    assert config.pipes_config is None
    assert config.conditional_pipes_config is not None


def test_the_authored_model_goes_on_refusing_the_runtime_document():
    """The half a wider `ComputePipesConfig` would have destroyed.

    Both keys are named in the refusal, because the pair is what a starter
    writes and a model that refused only one of them would still accept a
    half-runtime document nothing produces.
    """
    with pytest.raises(ValidationError) as excinfo:
        model.ComputePipesConfig.model_validate(runtime_document())
    refused = {e["loc"][0] for e in excinfo.value.errors()}
    assert refused == {"common_runtime_args", "pipes_config"}


def test_the_emitted_schema_goes_on_refusing_the_runtime_shape():
    """`negative_suite.json`'s *root pipes_config (I-14 runtime shape)* case.

    That case expects `invalid` and is run by `python -m cpipes_contract
    validate` and by `go run ./tools/cpipes_contract/validate`. It is asserted
    here as well because it is the reason the repair is a second class rather
    than two more fields on the first, and a reader of this file should not
    have to find that out from a suite in another format.
    """
    schema = json.loads(SCHEMA_PATH.read_text())
    root = schema["$defs"]["ComputePipesConfig"]
    assert root["additionalProperties"] is False
    assert "common_runtime_args" not in root["properties"]
    assert "pipes_config" not in root["properties"]
    assert "ComputePipesRuntimeConfig" not in schema["$defs"]
