"""The three-field entry, held to the Go struct rather than to a memory.

`ComputePipesNodeArgs` is the payload the state machine sends a node, and the
two engines must accept the same one. So the json tags are read out of
`actions_common_model.go` and compared, rather than transcribed here where a
rename on the Go side would leave this file green and the deployment broken.
"""

from __future__ import annotations

import re

import pytest
from pydantic import ValidationError

from conftest import go_source
from cpipes_node.args import NodeArgs


def test_the_aliases_are_the_go_struct_s_json_tags():
    src = go_source("jets/compute_pipes/actions_common_model.go")
    body = re.search(r"type ComputePipesNodeArgs struct \{(.*?)\n\}", src, re.DOTALL)
    assert body is not None, "ComputePipesNodeArgs is no longer where it was"
    tags = {t for t in re.findall(r'json:"([A-Za-z0-9_]+)', body.group(1))}
    aliases = {f.alias for f in NodeArgs.model_fields.values()}
    assert aliases == tags == {"id", "jp", "pe"}


def test_an_event_round_trips():
    args = NodeArgs(id=7, jp="0007P", pe=42)
    assert args.to_event() == {"id": 7, "jp": "0007P", "pe": 42}
    assert NodeArgs(**args.to_event()) == args


def test_an_absent_label_is_omitted_from_the_event():
    # `jp` is `omitempty` in Go, so an event carrying an empty string is one
    # the Go node would never have sent.
    assert NodeArgs(id=3, pe=1).to_event() == {"id": 3, "pe": 1}


@pytest.mark.parametrize(
    "node_id,label", [(0, "0000P"), (7, "0007P"), (40, "0040P"), (1234, "1234P")]
)
def test_the_label_is_derived_the_way_the_go_node_derives_it(node_id, label):
    # `fmt.Sprintf("%04dP", args.NodeId)`. Not cosmetic: it is the
    # `jets_partition=NNNNP` path segment partitioned output is written under.
    assert NodeArgs(id=node_id, pe=1).jets_partition_label_or_default() == label


def test_a_label_that_was_sent_is_kept():
    assert (
        NodeArgs(id=3, jp="reducing00", pe=1).jets_partition_label_or_default()
        == "reducing00"
    )


def test_a_missing_execution_key_is_refused():
    with pytest.raises(ValidationError):
        NodeArgs(id=1)


def test_an_unknown_field_is_refused():
    # A payload carrying a field this node does not know is a payload the Go
    # node was sent and this one was not written for; ignoring it is how two
    # entries diverge without anything going red.
    with pytest.raises(ValidationError):
        NodeArgs(id=1, pe=2, unexpected="x")
