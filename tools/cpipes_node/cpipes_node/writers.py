"""The device writers: CSV and Parquet, **from one column list**.

`s3_device_writter.go`'s `WriteCsvPartition` and `WriteParquetPartitionV2` are
what is mirrored, and the single most important fact about them is one this
module makes structural rather than promises:

**Both writers take the same tuple of column names and encode every cell with
the same function.** In Go that is visible in two places — `WriteCsvPartition`
writes `ctx.outputCh.Config.Columns` as its header and
`encodeRdfTypeToTxt(inRow[i])` for each cell, while a default parquet schema is
`BuildParquetSchemaInfo(outputCh.Config.Columns)`, which is *all string,
nullable*, and `ConvertToSchemaV2`'s string arm is `encodeRdfTypeToTxt(v)`. So
the two writers differ in container and in nothing else.

**That is what makes X3 provable.** The criterion is *every table written in CSV
and in Parquet from the same run by changing `device_writer_type` alone, and the
two agreeing cell for cell after type normalisation*. Here
:func:`write_csv` and :func:`write_parquet` take `columns` and `rows` and
neither derives a column list of its own; a test asserts the two agree over a
table containing every value shape the encoder distinguishes. If the two writers
could disagree about columns, X3 would be unprovable rather than false.

# Where the column list comes from, and what this package can and cannot say

The charter asks for *the list derived from Phase 8's output data dictionary
rather than restated*. The derivation this package can make is the whole of the
chain it can see: a writer takes `OperatorArgs.output.config.columns`, which is
the `ResolvedChannelSpec` the graph built from the `.pc.json`'s own `channels`
entry. **There is no column list anywhere in this module, and none in
`transformations.py`** — that is checkable by reading, and a test asserts the
writers are a function of their `columns` argument.

**The other half of the chain is not this package's and must not be faked here.**
`healthcare_corpus.generate.dictionary` holds the 511-column declaration, and
this package may not import the corpus (D-216). So the `.pc.json`'s twelve
channel specs have to be *generated from* that dictionary rather than typed by
hand, and nothing in this repository can check that they were: a hand-typed
column list that happens to be right validates, resolves and writes. That is
**P9-I56**, it belongs to P9-T18 which authors the document, and the instrument
it needs is a generator plus a test on the corpus side — stated here rather than
worked around, because a restated list is D-58's defect and it is exactly what
X3 cannot see.

# Two divergences from the Go writer, both deliberate and both named

**CSV quoting is transcribed rather than delegated.** `jets/csv/writer.go` is a
fork of Go's `encoding/csv` with `QuoteAll` and `NoQuotes` added, and its
`fieldNeedsQuotes` quotes a field that contains the delimiter, a quote, CR or
LF, **or begins with a unicode space, or is exactly `\\.`**. `pyarrow.csv`'s
`QuoteStyle.NEEDED` covers the first group and neither of the last two, and
Python's own `csv` module covers the first group and neither of the last two
either. Since X7 compares the two engines **byte for byte**, a writer built on
either library could not be byte-identical to the engine it is measured against,
so the rule is transcribed from that file and asserted against it. That is
**D-222**.

**Parquet bytes are not comparable and cell values are.** Go writes through
`pqarrow` with `created_by: "jetstore"` and its own row-group layout; pyarrow
writes its own. X3 asks for cell-for-cell agreement and X7 for byte identity,
and **X7's subject is the CSV path**: a parquet file's bytes are a property of
the arrow implementation, not of the corpus. Said here so that P9-T22 does not
discover it as a failure.
"""

from __future__ import annotations

import datetime as _dt
import io
import math
import unicodedata
from collections.abc import Iterable, Mapping, Sequence
from decimal import Decimal
from typing import IO, Any

from .errors import NodeError


class WriterError(NodeError):
    """A device writer could not be built or could not write."""


class WriterUnsupported(WriterError):
    """The document names a writer, format or compression outside the subset.

    A distinct type for the reason `errors.py` gives: an unsupported
    `device_writer_type` is a scope question and a mismatched format is an
    authoring mistake, and the reader of each has a different repair.
    """


# --- the text encoding ------------------------------------------------------

#: `time.Time`'s two layouts in `encodeRdfTypeToTxt`, verbatim from
#: `jetrules_rdf.go`: a value whose clock is exactly midnight prints as a date
#: and everything else prints with its time. **The Go test for this function is
#: one of the eight pre-existing failures on `jets_ai`** — `jetrules_rdf_test.go`
#: line 39 expects `2006-01-02T00:00:00` where the function returns
#: `2006-01-02` — so the *test* is stale about the *function*. The function is
#: what the engine runs and is therefore what is mirrored here; an encoder
#: written from that test would disagree with every byte the Go engine writes
#: (P9-I58).
DATE_FORMAT = "%Y-%m-%d"
DATETIME_FORMAT = "%Y-%m-%dT%H:%M:%S"


def encode_rdf_type_to_txt(value: Any) -> str:
    """`encodeRdfTypeToTxt`: one cell as text, for either writer.

    The array case is the `{"a","b"}` encoding the engine uses for a
    multi-valued cell, and it recurses, so an array of nulls is `{"",""}`
    rather than empty.

    `Decimal` and `datetime.date` have no Go counterpart in that switch and
    reach it through the default arm, which is `fmt.Sprintf("%v")`. They are
    named here instead because **this node's site operator is a Python one and
    a corpus row carries both** — `Decimal("12.50")` through `%v` would print
    the Go struct rather than the number. Naming them is the conservative
    reading of a default arm rather than an extension of it.
    """
    if value is None:
        return ""
    if isinstance(value, str):
        return value
    if isinstance(value, bool):
        # Go prints 1 / 0 for a bool, not true / false. `_as_text` in
        # `expressions.py` prints true / false, and the two are different
        # functions in Go as well: one is env substitution and this is a cell.
        return "1" if value else "0"
    if isinstance(value, int):
        return str(value)
    if isinstance(value, float):
        # `strconv.FormatFloat(v, 'f', -1, 64)`: the shortest decimal that
        # round-trips, never an exponent. Python's repr uses an exponent outside
        # a range, so the formatting is explicit.
        return _format_float(value)
    if isinstance(value, Decimal):
        return format(value, "f")
    if isinstance(value, _dt.datetime):
        if (value.hour, value.minute, value.second, value.microsecond) == (0, 0, 0, 0):
            return value.strftime(DATE_FORMAT)
        return value.strftime(DATETIME_FORMAT)
    if isinstance(value, _dt.date):
        return value.strftime(DATE_FORMAT)
    if isinstance(value, bytes):
        return value.decode("utf-8", errors="replace")
    if isinstance(value, (list, tuple)):
        inner = '","'.join(encode_rdf_type_to_txt(item) for item in value)
        return '{"' + inner + '"}'
    return str(value)


def _format_float(value: float) -> str:
    """`strconv.FormatFloat(v, 'f', -1, 64)`: no exponent, shortest round-trip."""
    if math.isnan(value) or math.isinf(value):
        # Go prints NaN / +Inf / -Inf; Python prints nan / inf / -inf.
        return {"nan": "NaN", "inf": "+Inf", "-inf": "-Inf"}[str(value)]
    text = repr(value)
    if "e" not in text and "E" not in text:
        return text
    return format(Decimal(text), "f")


# --- the csv writer ---------------------------------------------------------

#: The literal `jets/csv/writer.go` quotes for Postgres' sake. Kept as a
#: constant so the test that asserts the rule against the Go source has
#: something to name.
POSTGRES_TERMINATOR = "\\."


def field_needs_quotes(field: str, delimiter: str) -> bool:
    """`Writer.fieldNeedsQuotes`, transcribed. See D-222 and the module docstring.

    Four clauses and the last two are the ones no library reproduces: a field
    beginning with a unicode space, and the Postgres data terminator.
    """
    if field == "":
        return False
    if field == POSTGRES_TERMINATOR:
        return True
    if any(c in ("\n", "\r", '"', delimiter) for c in field):
        return True
    return _is_go_space(field[0])


def _is_go_space(char: str) -> bool:
    """`unicode.IsSpace`, which is not `str.isspace()`.

    Go's set is the Unicode `White_Space` property: the ASCII run plus NEL,
    NBSP and the `Zs`/`Zl`/`Zp` categories. Python's `str.isspace()` also
    answers true for the file/group/record separators `\\x1c`-`\\x1f`, which
    Go's does not, so a field beginning with a record separator would be quoted
    by one engine and not the other.
    """
    if char in " \t\n\v\f\r\x85\xa0":
        return True
    return unicodedata.category(char) in ("Zs", "Zl", "Zp")


def write_csv(
    columns: Sequence[str],
    rows: Iterable[Sequence[Any]],
    *,
    header: bool = True,
    delimiter: str = ",",
    quote_all: bool = False,
    no_quotes: bool = False,
    use_crlf: bool = False,
) -> bytes:
    """One partition file, as the Go csv writer would write it.

    `no_quotes` writes every field verbatim, which is what that flag means in
    the fork and is *lossy on purpose*: a value containing the delimiter
    produces a file with more fields than columns, and the flag exists for
    downstream consumers that cannot parse quotes at all.
    """
    sink = io.BytesIO()
    write_csv_to(
        sink,
        columns,
        rows,
        header=header,
        delimiter=delimiter,
        quote_all=quote_all,
        no_quotes=no_quotes,
        use_crlf=use_crlf,
    )
    return sink.getvalue()


def write_csv_to(
    sink: IO[bytes],
    columns: Sequence[str],
    rows: Iterable[Sequence[Any]],
    *,
    header: bool = True,
    delimiter: str = ",",
    quote_all: bool = False,
    no_quotes: bool = False,
    use_crlf: bool = False,
) -> None:
    """The same file, written into `sink` one record at a time.

    `write_csv` is this over a `BytesIO`, so there is **one** csv encoder and
    `stream_data_out` selects where the bytes land rather than how they are
    made. Two encoders would be two chances to disagree about the quoting rule
    X7 compares bytes against (D-222).
    """
    if len(delimiter) != 1:
        raise WriterError(f"the csv delimiter must be one character, not {delimiter!r}")
    newline = "\r\n" if use_crlf else "\n"
    if header:
        line = _csv_record(columns, delimiter, quote_all, no_quotes, use_crlf)
        sink.write((line + newline).encode("utf-8"))
    for row in rows:
        line = _csv_record(
            [encode_rdf_type_to_txt(cell) for cell in row],
            delimiter,
            quote_all,
            no_quotes,
            use_crlf,
        )
        sink.write((line + newline).encode("utf-8"))


def _csv_record(
    fields: Sequence[str],
    delimiter: str,
    quote_all: bool,
    no_quotes: bool,
    use_crlf: bool,
) -> str:
    parts: list[str] = []
    for field in fields:
        if no_quotes or (not quote_all and not field_needs_quotes(field, delimiter)):
            parts.append(field)
            continue
        body = field.replace('"', '""')
        if use_crlf:
            # The fork writes \r\n for a bare \n inside a quoted field and
            # drops a bare \r, which is Go's own asymmetry.
            body = body.replace("\r", "").replace("\n", "\r\n")
        parts.append(f'"{body}"')
    return delimiter.join(parts)


# --- the parquet writer -----------------------------------------------------


def write_parquet(
    columns: Sequence[str],
    rows: Iterable[Sequence[Any]],
    *,
    batch_size: int = 0,
) -> bytes:
    """One partition file, over the default all-string schema.

    `BuildParquetSchemaInfo` makes every field `string`, nullable, in the order
    of the same column list the csv writer takes — so **the schema is derived
    from `columns` here exactly as it is there**, and a cell is
    `encode_rdf_type_to_txt` of the value with `None` staying null.

    `None` is the one place the two writers do not agree as bytes and cannot:
    CSV has no null, so a null and an empty string are both an empty field,
    where parquet keeps them apart. That is the *type normalisation* X3's
    wording allows for, and it is asserted in that direction rather than left
    to be discovered.

    `batch_size` is `output_channel.nbr_rows_in_record`, defaulting to the Go
    writer's 1024, which fixes the row-group length.
    """
    sink = io.BytesIO()
    write_parquet_to(sink, columns, rows, batch_size=batch_size)
    return sink.getvalue()


def write_parquet_to(
    sink: IO[bytes],
    columns: Sequence[str],
    rows: Iterable[Sequence[Any]],
    *,
    batch_size: int = 0,
) -> None:
    """The same file, written into `sink`.

    **Parquet streams less than csv does and the difference is the format's, not
    this node's**: the schema's footer is written last and the whole table is
    built before `write_table` is called, so `stream_data_out` over parquet
    bounds what the *store* holds and not what the encoder holds. Go has the same
    property — `WriteParquetPartitionV3` accumulates a row group at a time and
    the arrow writer's footer is still last — and saying so here is cheaper than
    a later measurement wondering why the memory did not fall.
    """
    pa, pq = _pyarrow()
    if batch_size <= 0:
        batch_size = PARQUET_DEFAULT_BATCH_SIZE
    schema = pa.schema([pa.field(name, pa.string(), nullable=True) for name in columns])
    width = len(columns)
    cells: list[list[str | None]] = [[] for _ in range(width)]
    for row in rows:
        if len(row) != width:
            raise WriterError(
                f"error: len(row) {len(row)} does not match len(builders) {width} "
                "in WriteParquetPartition"
            )
        for i, cell in enumerate(row):
            cells[i].append(None if cell is None else encode_rdf_type_to_txt(cell))
    table = pa.Table.from_arrays(
        [pa.array(column, type=pa.string()) for column in cells], schema=schema
    )
    pq.write_table(
        table,
        sink,
        compression="snappy",
        row_group_size=batch_size,
    )


#: `WriteParquetPartitionV3`'s own default when `nbr_rows_in_record` is absent.
PARQUET_DEFAULT_BATCH_SIZE = 1024


def _pyarrow() -> tuple[Any, Any]:
    """pyarrow, imported at the call rather than at module scope.

    **It is a dependency and not an extra**, deliberately: X3 is an exit
    criterion about writing every table in both formats, and a format that works
    only where somebody remembered an extra is a criterion that depends on an
    install. What the lazy import buys is cold-start time in a lambda whose
    pipeline writes csv — the scope gate, the graph, `map_record` and `filter`
    never touch arrow — and the refusal below stays because an import that
    cannot fail is an import nobody checks.
    """
    try:
        import pyarrow as pa  # type: ignore[import-not-found]
        import pyarrow.parquet as pq  # type: ignore[import-not-found]
    except ImportError as exc:  # pragma: no cover - depends on the install
        raise WriterError(
            "the parquet device writer needs pyarrow, which is a dependency of "
            "this package rather than an extra; an install that does not carry "
            "it is broken rather than minimal. The csv device writer needs "
            "nothing."
        ) from exc
    return pa, pq


# --- the dispatch -----------------------------------------------------------

#: `device_writer_type` -> the file extension the Go writer gives a partition
#: whose output channel names no `file_name`. Read off
#: `PartitionWriterTransformationPipe.applyInternal`'s own switch.
FILE_EXTENSIONS: Mapping[str, str] = {
    "csv_writer": "csv",
    "parquet_writer": "parquet",
    "fixed_width_writer": "fixed_width",
}

#: `device_writer_type` -> the `output_channel.format` values it accepts, from
#: `NewPartitionWriterTransformationPipe`'s verification switch. The two xlsx
#: values are in the Go switch and in **no** `format` enum the contract
#: declares, so they are unreachable through a validated document; they are kept
#: because the set is transcribed rather than chosen.
SUPPORTED_FORMATS: Mapping[str, tuple[str, ...]] = {
    "csv_writer": ("csv", "headerless_csv", "xlsx", "headerless_xlsx"),
    "parquet_writer": ("parquet", "parquet_select"),
    "fixed_width_writer": ("fixed_width",),
}

#: The device writers this node implements. `fixed_width_writer` is declared by
#: the contract, is **out of this task's scope by instruction**, and is refused
#: by name: it needs `fixed_width_columns_csv`'s offsets, and a fixed-width file
#: written at the wrong offsets is a file every consumer reads successfully and
#: wrongly.
IMPLEMENTED_DEVICE_WRITERS: tuple[str, ...] = ("csv_writer", "parquet_writer")

REFUSED_DEVICE_WRITERS: Mapping[str, str] = {
    "fixed_width_writer": (
        "out of this phase's declared scope; it needs the column offsets from "
        "output_channel.fixed_width_columns_csv, and a fixed-width file written "
        "at the wrong offsets is read successfully and wrongly by every consumer"
    )
}

#: The compressions the csv writer implements. The Go writer wraps its output in
#: `snappy.NewBufferedWriter`, which is the snappy **framing** format; nothing in
#: this package can produce that stream, and raw snappy would be a file the Go
#: reader cannot open — worse than a refusal. Recorded as **P9-I59**, with the
#: consequence for the authored document: a **stage** channel's compression
#: defaults to `snappy`, so the corpus `.pc.json` must set `"compression":
#: "none"` explicitly or be refused while the graph is built.
IMPLEMENTED_COMPRESSIONS: tuple[str, ...] = ("none",)


def check_device_writer(device_writer_type: str, output_format: str) -> None:
    """Refuse an unsupported writer, and a writer-format pair the Go engine refuses.

    Both refusals are the Go constructor's and both happen before a record
    moves. The format check is the one worth keeping: `csv_writer` with
    `format: parquet` produces a csv file under a `.parquet` name, and every
    consumer of that file fails somewhere else.
    """
    if device_writer_type in REFUSED_DEVICE_WRITERS:
        raise WriterUnsupported(
            f"device_writer_type {device_writer_type!r} is declared by JetStore's "
            f"contract and not implemented here: "
            f"{REFUSED_DEVICE_WRITERS[device_writer_type]}. This node implements "
            f"{list(IMPLEMENTED_DEVICE_WRITERS)}."
        )
    if device_writer_type not in IMPLEMENTED_DEVICE_WRITERS:
        raise WriterUnsupported(
            f"unknown device_writer_type {device_writer_type!r}; this node "
            f"implements {list(IMPLEMENTED_DEVICE_WRITERS)} and refuses "
            f"{list(REFUSED_DEVICE_WRITERS)} by name."
        )
    accepted = SUPPORTED_FORMATS[device_writer_type]
    if output_format not in accepted:
        raise WriterUnsupported(
            f"error: {device_writer_type} does not support file format "
            f"{output_format!r} (it supports {list(accepted)})"
        )


def check_compression(compression: str) -> None:
    if compression in IMPLEMENTED_COMPRESSIONS:
        return
    raise WriterUnsupported(
        f"compression {compression!r} is not implemented by this node's csv "
        "device writer. The Go writer uses the snappy framing format "
        "(snappy.NewBufferedWriter) and a raw-snappy stand-in would be a file "
        "the Go reader cannot open, which is worse than a refusal (P9-I59). "
        "Note that a 'stage' output channel's compression defaults to 'snappy', "
        'so an authored document must set "compression": "none".'
    )


def write_partition(
    device_writer_type: str,
    columns: Sequence[str],
    rows: Sequence[Sequence[Any]],
    *,
    output_format: str,
    compression: str = "none",
    delimiter: str = ",",
    quote_all: bool = False,
    no_quotes: bool = False,
    batch_size: int = 0,
    headers_on_this_node: bool = True,
) -> bytes:
    """One partition's bytes, by writer type. **The one branch on the type.**

    The header rule is the Go writer's: a header is written when the format is
    exactly `csv`, so `headerless_csv` has none and `parquet` carries its schema
    instead — **and** when `headers_on_this_node`, which is
    `put_headers_on_first_partition` resolved against the node id by the caller,
    because only the caller has it. See `write_partition_to` and P9-I151.
    """
    sink = io.BytesIO()
    write_partition_to(
        sink,
        device_writer_type,
        columns,
        rows,
        output_format=output_format,
        compression=compression,
        delimiter=delimiter,
        quote_all=quote_all,
        no_quotes=no_quotes,
        batch_size=batch_size,
        headers_on_this_node=headers_on_this_node,
    )
    return sink.getvalue()


def write_partition_to(
    sink: IO[bytes],
    device_writer_type: str,
    columns: Sequence[str],
    rows: Sequence[Sequence[Any]],
    *,
    output_format: str,
    compression: str = "none",
    delimiter: str = ",",
    quote_all: bool = False,
    no_quotes: bool = False,
    batch_size: int = 0,
    headers_on_this_node: bool = True,
) -> None:
    """One partition into `sink`, by writer type. **The one branch on the type.**

    `write_partition` is this over a `BytesIO`, so the dispatch is not duplicated
    and `stream_data_out` cannot select a different encoder by accident — which
    is the failure mode a second copy of this switch would have, silently, in
    the one place X7 compares bytes.

    `headers_on_this_node` is the second half of Go's header condition —
    `Format == "csv" && (!PutHeadersOnFirstPartition || nodeId == 0)`
    (`s3_device_writter.go:148`). It is a parameter because the format is a
    property of the channel and the node id is a property of the run, so only
    the caller can answer it. `write_partition`'s docstring said exactly that
    before any caller did it, and **nothing in the package read
    `put_headers_on_first_partition` at all** (P9-I151): measured over the
    corpus document at four nodes, every node wrote a header on its own first
    part and the merged `member` carried **four** header lines over 200 rows
    where Go writes one.
    """
    check_device_writer(device_writer_type, output_format)
    if device_writer_type == "parquet_writer":
        write_parquet_to(sink, columns, rows, batch_size=batch_size)
        return
    check_compression(compression)
    write_csv_to(
        sink,
        columns,
        rows,
        header=output_format == "csv" and headers_on_this_node,
        delimiter=delimiter,
        quote_all=quote_all,
        no_quotes=no_quotes,
    )


def partition_file_name(
    node_id: int, partition_number: int, device_writer_type: str
) -> str:
    """`part%04d-%07d.%s`, the name a partition takes when none is authored.

    Transcribed because it is a value two engines must agree on: a merge step
    (P9-T08) and Phase 8's harness both find these files by listing a prefix,
    and a node that named them differently would write a corpus nothing reads.
    """
    extension = FILE_EXTENSIONS.get(device_writer_type, "")
    return f"part{node_id:04d}-{partition_number:07d}.{extension}"
