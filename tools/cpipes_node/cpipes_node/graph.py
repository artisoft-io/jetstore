"""The channel graph: the seam P9-T04 fills.

Everything before this module is startup — the arguments, the document, the
scope gate, the environment. Everything after it is the run. The split is where
it is because X6 is a *startup* criterion: by the time `run` is called, every
token the document names has been judged, so the graph runner never meets an
unknown operator and never has to decide what to do about one.

**What P9-T04 implements is this function and the four things underneath it**,
and the order is fixed by `operator.go` rather than by preference:

1. The channel registry — `ChannelSpec` in, the named input and output channels
   out, with `same_columns_as_input` and `class_name` resolved.
2. The `generator` source. In reducing mode `CoordinateComputePipes` puts a
   single `generator_file_proxy` marker in the file-key list rather than
   querying S3; the source then pushes `nbr_rows` empty records of the right
   width. `nbr_nodes` and `nbr_rows` are `int | str` in the contract because
   either may be an env-var reference, so both go through `Substitute` first.
3. `Apply` / `Done` / `Finally` — `pipesmodel.PipeTransformationEvaluator`'s
   three methods — and the channel-close semantics around them.
4. **`when` and `conditional_config`, evaluated before the factory is
   reached.** `BuildPipeTransformationEvaluator` resolves the output channel,
   then evaluates `when`, and **returns a nil evaluator when it is false** —
   which is how the builder says *do not apply this step*, and is why a site
   factory returning nil is an error rather than a skip. `conditional_config`
   is resolved earlier still. A node that evaluated either after building
   would build operators an author asked it not to.

One thing this seam already knows and P9-T04 should not have to re-derive:
`ctx.env` carries `$SHARD_ID` and `$JETS_PARTITION_LABEL`, set by
`node.environment()` exactly where `CoordinateComputePipes` sets them and with
the same warning the Go source carries — **no key may be a prefix of another
key**, or `$FILE_KEY_PATH` takes `$FILE_KEY`'s value with a dangling `_PATH`.
"""

from __future__ import annotations

from typing import TYPE_CHECKING, Any

from .errors import NodeError

if TYPE_CHECKING:  # pragma: no cover
    from .node import NodeContext


class GraphNotBuilt(NodeError):
    """The channel graph has not been built yet.

    A distinct type rather than `NotImplementedError`, so a caller can tell the
    seam from a Python-level programming error, and so that the message names
    the task rather than the function.
    """


def run(ctx: NodeContext) -> Any:
    """Build the channel graph for one node and run it to completion."""
    raise GraphNotBuilt(
        "the channel graph is not built: P9-T04 owns cpipes_node/graph.py. "
        f"The node reached it with {len(ctx.scope_report.accepted)} token(s) "
        "accepted by the scope gate, "
        f"partition {ctx.jets_partition_label}, "
        f"mode {ctx.cpipes_mode!r}."
    )
