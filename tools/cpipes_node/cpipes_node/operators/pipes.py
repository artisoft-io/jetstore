"""Pipe types this node declares.

A pipe is one input channel and the transformations applied to its records;
`merge_files` is one of these and not a transformation, which the assessment's
six-token list obscures by naming it beside `map_record`. The contract is
unambiguous — `_MATRIX_KEYS` keys it `("PipeSpec", "merge_files")` — and the
consequence of getting it wrong is a document naming `{"type": "merge_files"}`
inside an `apply` list, which would be refused by the model but with a message
about a transformation union rather than about a pipe.
"""

from __future__ import annotations

from typing import ClassVar

from ..scope import Operator, TokenKind


class Pipe(Operator):
    kind = TokenKind.PIPE

    #: Whether this pipe kind is a pipe *of the channel graph*.
    #:
    #: **Go has two paths and this says which one a pipe kind takes** (D-224).
    #: `ProcessFilesAndReportStatus` branches on `ComputePipesArgs.MergeFiles`
    #: before `StartComputePipes` is reached, so a merge step never registers a
    #: channel, never builds an evaluator and never sees a record
    #: (`actions_process_file.go:33-39`). `graph.run` reads this off the first
    #: pipe's declaration, which is what keeps that module free of an
    #: `if spec.type == "merge_files"` — the dispatch is the declaration, one
    #: level up from `build`.
    #:
    #: Declared on the base rather than on each subclass so that a pipe kind
    #: added later is a graph pipe unless it says otherwise. That is the
    #: majority case and it is the safe default: a node-mode pipe wrongly run
    #: through the graph is refused by a channel registry that cannot feed its
    #: source, where a graph pipe wrongly run as a node mode would be handed to
    #: a merge that has no records to move.
    drives_channel_graph: ClassVar[bool] = True

    #: The input-channel type this pipe kind fixes, or empty when the type is a
    #: scope token to be judged.
    #:
    #: The second limb of D-224. A `merge_files` pipe's input channel is typed
    #: `stage` by `ValidatePipeSpecConfig` rather than chosen by an author
    #: (`actions_start_common.go:973-980`), and `stage` is not a channel type
    #: this node declares — a `fan_out` reading one needs an S3 reader and a
    #: record parser this node does not have. So for this pipe kind the scope
    #: gate asserts the fixed type instead of classifying a token, which is
    #: strictly narrower than classifying it: `stage` is accepted nowhere else,
    #: and a merge pipe reading `memory` is refused at startup here where Go
    #: refuses it only in the starter.
    fixed_input_channel_type: ClassVar[str] = ""


class FanOut(Pipe):
    """One input channel, its transformations applied to every record.

    Declared although the charter's six do not name it: it is the only pipe
    kind the phase's pipeline uses and every transformation in the scope must
    sit inside one. See `operators/__init__.py` and P9-I26.
    """

    token = "fan_out"
    owed_by = "P9-T04"
    summary = "apply each transformation to every record of one input channel"

    @classmethod
    def build(cls, env: object, spec: object) -> object:
        """The pipe executor: `StartFanOutPipe`'s counterpart.

        Returns the handler rather than an object, which is what `build` means
        for a *pipe* type — the graph owns the registry, the termination signal
        and the pipe's index, and a pipe is not a per-record object the way a
        transformation is. Dispatching through here is what keeps
        `graph.py` free of an `if spec.type == "fan_out"` (P3-I20): a pipe kind
        this node grows is reached by its class being written.

        **This method is the only thing P9-T04 added to this file**, and it is
        added rather than handed over because without it no document runs: the
        scope gate refuses `fan_out` as declared-and-not-implemented, so the
        whole graph would be reachable from tests alone — a component whose own
        tests pass and which reaches no working path (P4-I43). `MergeFiles`
        below is untouched and is P9-T08's.
        """
        from .. import graph

        return graph.fan_out_pipe


class MergeFiles(Pipe):
    """Concatenate a stage channel's partition files into one output file.

    What lets a partitioned run be handed to Phase 8's harness, which reads a
    single directory of twelve files. **Whether the harness should read the tree
    instead is P9-I09 and is not answered by this existing**: an assembled
    directory is what today's harness accepts, and `merge_files` is the bridge
    to it.

    **It is not a pipe of the channel graph** — see `Pipe.drives_channel_graph`
    and the module docstring of `cpipes_node.merge`, which is where the work is.
    """

    token = "merge_files"
    owed_by = "P9-T08"
    summary = "concatenate partition files into a single output file"
    drives_channel_graph = False
    fixed_input_channel_type = "stage"

    @classmethod
    def build(cls, env: object, spec: object) -> object:
        """The merge itself: `StartMergeFiles`' counterpart.

        Returns the handler rather than an object, the way `FanOut.build` does
        and for the same reason — but the handler's *shape* differs, because the
        two Go paths differ: a graph pipe's executor is handed the registry, the
        termination signal and its own index and returns a `BuiltPipe`, and a
        merge is handed the context and the spec and does the whole node's work.
        `Pipe.drives_channel_graph` is what tells `graph.run` which it has, so
        the shape is read off the declaration rather than off the token.
        """
        from .. import merge

        return merge.run_merge
