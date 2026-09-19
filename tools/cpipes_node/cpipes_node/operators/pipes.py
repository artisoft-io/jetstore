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

from ..scope import Operator, TokenKind


class Pipe(Operator):
    kind = TokenKind.PIPE


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
    single directory of twelve files.
    """

    token = "merge_files"
    owed_by = "P9-T08"
    summary = "concatenate partition files into a single output file"
