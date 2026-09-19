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
    integration would have had to add one to JetStore. It is **refused in
    sharding mode by name**, so a pipeline built on it is a reducing pipeline,
    which is a constraint on the authored `.pc.json` rather than on this node.
    """

    token = "generator"
    owed_by = "P9-T04"
    summary = "source with no input: nbr_nodes partitions of nbr_rows empty records"


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
