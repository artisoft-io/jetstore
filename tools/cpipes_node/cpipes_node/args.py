"""The node's arguments: `{id, jp, pe}`, and nothing else.

`ComputePipesNodeArgs` is three fields, at `actions_common_model.go` line 46,
and its own comment says why — *minimal set of arguments to reduce the size of
the json to call the lambda functions*. Everything else the node reads from
`jetsapi.cpipes_execution_status` and from S3. **This class mirrors that
contract and does not extend it**: a fourth field here would be a fourth field
the state machine would have to send, and the Map step that fans out to forty
nodes sends this object forty times.

The JSON keys are the Go tags verbatim — `id`, `jp`, `pe` — because the same
event payload must be accepted by either node. `extra="forbid"` is the
assertion that they are: a payload carrying a field this node does not know is
a payload the Go node was sent and this one was not written for, and silently
ignoring it is how the two would diverge.
"""

from __future__ import annotations

from pydantic import BaseModel, ConfigDict, Field


class NodeArgs(BaseModel):
    """The lambda event `cp_node` is invoked with."""

    model_config = ConfigDict(extra="forbid")

    node_id: int = Field(alias="id")
    #: `omitempty` in Go, so an absent label is the ordinary case rather than
    #: an error; `jets_partition_label()` derives it.
    jets_partition_label: str = Field(default="", alias="jp")
    pipeline_execution_key: int = Field(alias="pe")

    def jets_partition_label_or_default(self) -> str:
        """The label, defaulted from the node id the way the Go node does.

        `CoordinateComputePipes` fills an empty label with `%04dP` of the node
        id before anything reads it, and the format is not cosmetic: it is the
        `jets_partition=NNNNP` path segment a partitioned output is written
        under, so a node that formatted it differently would write a corpus
        into a directory no reader looks in.
        """
        if self.jets_partition_label:
            return self.jets_partition_label
        return f"{self.node_id:04d}P"

    def to_event(self) -> dict[str, object]:
        """The payload the Go node would be invoked with.

        Round-tripping through this is how `tests_args.py` asserts the two
        entries accept one another's events.
        """
        event: dict[str, object] = {
            "id": self.node_id,
            "pe": self.pipeline_execution_key,
        }
        if self.jets_partition_label:
            event["jp"] = self.jets_partition_label
        return event
