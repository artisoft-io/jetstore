"""Input-channel types this node declares.

An input channel is not a transformation, and the distinction is load-bearing
rather than tidy: `CoordinateComputePipes` branches on
`inputChannelConfig.Type` *before* any pipe is built — the `generator` case is
what puts the `generator_file_proxy` marker in the file-key list instead of
querying S3 — so a channel type this node cannot serve is a failure that
happens earlier than an operator it cannot build, and it would otherwise be
diagnosed as "no files found".
"""

from __future__ import annotations

from ..scope import Operator, TokenKind


class InputChannel(Operator):
    kind = TokenKind.INPUT_CHANNEL


class Generator(InputChannel):
    """The source that reads no file: `nbr_nodes` partitions of `nbr_rows`.

    It is why this phase exists in the shape it does — a corpus generator is
    exactly a source with no input, and without this channel type the
    integration would have had to add one to JetStore. A pipeline built on it is
    a **reducing** pipeline: `CoordinateComputePipes` special-cases `generator`
    under reducing alone, and in sharding mode it resolves its file keys from the
    shard registry, finds none, and never reaches the generator — so
    `node._file_keys` refuses that combination by name rather than writing
    nothing and exiting 0.
    """

    token = "generator"
    owed_by = "P9-T04"
    summary = "source with no input: nbr_nodes partitions of nbr_rows empty records"

    @classmethod
    def build(cls, env: object, spec: object) -> object:
        """The source `graph._drive` pushes records from.

        Returning the handler rather than an object is what `build` means for a
        channel type: there is no per-step runtime object for a source, and the
        graph owns the width and the termination signal. **The point of routing
        through here is that `graph.py` contains no `if type == "generator"`** —
        the dispatch is the declaration, so a channel type this node grows is
        reached by the class being written and not by an `if` being remembered
        (P3-I20).
        """
        from .. import graph

        return graph.generator_records


class Memory(InputChannel):
    """A channel another step in this process wrote.

    Declared although the charter's list of six does not name it, because the
    twelve partition writers read the site operator's twelve output channels
    and there is no other type they could read them as. See the package
    docstring and P9-I26.
    """

    token = "memory"
    owed_by = "P9-T04"
    summary = "an in-process channel a previous step of this node wrote"

    @classmethod
    def build(cls, env: object, spec: object) -> object:
        """No source: a memory channel is fed by another pipe of this node.

        `None` is the answer and it is a real one rather than a stub. The graph
        asks for a source only for the pipe it feeds itself, so a `memory`
        channel there is a document whose first pipe waits on records nothing in
        this node produces — and the graph refuses it naming the channel, which
        is the same refusal `execution_order` makes for any unwritten source.
        """
        return None
