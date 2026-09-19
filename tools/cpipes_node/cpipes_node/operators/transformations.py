"""Transformation tokens this node declares.

Three of JetStore's nineteen. The other sixteen are out of scope and abort by
name, which is X6, and **whether that subset is ever permanent is not decided
here**: the charter's §1.3 says in terms that parity with the built-ins is not
this phase's to answer, the question is P9-I04, and the number reserved for the
ruling is D-206. What this module does is make the subset a fact a reader can
check rather than a sentence in a plan.

The site operator is not one of these. See `operators/__init__.py`.
"""

from __future__ import annotations

from ..scope import Operator, TokenKind


class Transformation(Operator):
    kind = TokenKind.TRANSFORMATION


class MapRecord(Transformation):
    """Map the input record onto the output channel's columns."""

    token = "map_record"
    owed_by = "P9-T06"
    summary = "map the input record onto the output channel's columns"


class Filter(Transformation):
    """Pass on the records satisfying the authored condition."""

    token = "filter"
    owed_by = "P9-T06"
    summary = "pass on the records satisfying the authored condition"


class PartitionWriter(Transformation):
    """Write the input channel's records to partition files.

    The `device_writer_type` enum is where §7's *Parquet and CSV writers* is
    answered by something that already exists; this node has to honour the
    value, not invent an encoder.
    """

    token = "partition_writer"
    owed_by = "P9-T07"
    summary = "write the input channel's records to partition files"
