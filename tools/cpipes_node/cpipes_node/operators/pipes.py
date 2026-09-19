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


class MergeFiles(Pipe):
    """Concatenate a stage channel's partition files into one output file.

    What lets a partitioned run be handed to Phase 8's harness, which reads a
    single directory of twelve files.
    """

    token = "merge_files"
    owed_by = "P9-T08"
    summary = "concatenate partition files into a single output file"
