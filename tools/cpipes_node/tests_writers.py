"""The device writers, and X3's own property: one column list, two containers.

**The file worth reading first is the Go one.** Three tests here take
`jets/csv/writer.go`, `jets/compute_pipes/jetrules_rdf.go` and
`jets/compute_pipes/parquet_schema_info.go` as oracles rather than transcribing
what they say into an assertion, so a change on the other side of the seam is
caught here rather than at a deployment.

**The X3 test is `test_the_two_writers_agree_cell_for_cell`.** It is the whole
criterion at the writer level: the same columns and the same rows through both
encoders, read back, compared. It is written over a table containing every value
shape the encoder distinguishes rather than over a plausible one, because a
comparison of two encoders over strings compares nothing.
"""

# ruff: noqa: DTZ001 -- a naive datetime is the subject. `encodeRdfTypeToTxt`
# formats a `time.Time` with a layout carrying no zone, and a corpus row carries
# naive dates; attaching a tzinfo here would test a value this node never meets.
from __future__ import annotations

import datetime as dt
import io
import re
from decimal import Decimal

import pytest

from conftest import go_source
from cpipes_node import writers
from cpipes_node.writers import WriterError, WriterUnsupported

#: Every value shape `encode_rdf_type_to_txt` distinguishes, with the text the Go
#: function produces for it. The list is the switch's arms in order, plus the two
#: Python types that reach its default and are named explicitly.
ENCODINGS: tuple[tuple[object, str], ...] = (
    (None, ""),
    ("hello", "hello"),
    ("", ""),
    (True, "1"),
    (False, "0"),
    (5, "5"),
    (-5, "-5"),
    (12.65, "12.65"),
    (1e-7, "0.0000001"),
    (1e22, "10000000000000000000000"),
    (Decimal("12.50"), "12.50"),
    (dt.date(2006, 1, 2), "2006-01-02"),
    (dt.datetime(2006, 1, 2, 0, 0, 0), "2006-01-02"),
    (dt.datetime(2006, 1, 2, 15, 4, 5), "2006-01-02T15:04:05"),
    (b"bytes", "bytes"),
    (["a1", "a2"], '{"a1","a2"}'),
    ([None], '{""}'),
    ([1, dt.date(2006, 1, 2)], '{"1","2006-01-02"}'),
)


@pytest.mark.parametrize(("value", "text"), ENCODINGS)
def test_the_encoder_answers_what_the_go_function_answers(value: object, text: str):
    assert writers.encode_rdf_type_to_txt(value) == text


def test_the_midnight_date_rule_mirrors_the_function_and_not_its_stale_test():
    """**P9-I58**: the Go *test* for this function is one of the eight red ones.

    `jetrules_rdf_test.go` line 39 asserts `2006-01-02T00:00:00` for a
    `rdf.ParseDate` value, and `encodeRdfTypeToTxt`'s `time.Time` arm returns
    `2006-01-02` for any value whose clock is exactly midnight. The function is
    what the engine runs and is therefore what is mirrored; an encoder written
    from that expectation would disagree with every byte the Go engine writes.

    Asserted against the Go source in both directions, so the day somebody
    repairs the disagreement — in either file — this goes red and says which way
    it was repaired.
    """
    source = go_source("jets/compute_pipes/jetrules_rdf.go")
    arm = source.split("case time.Time:", 1)[1].split("case uint:", 1)[0]
    assert '"2006-01-02"' in arm
    assert '"2006-01-02T15:04:05"' in arm
    test_source = go_source("jets/compute_pipes/jetrules_rdf_test.go")
    assert '!= "2006-01-02T00:00:00"' in test_source

    assert writers.encode_rdf_type_to_txt(dt.datetime(2006, 1, 2)) == "2006-01-02"


def test_a_float_never_reaches_a_file_in_exponent_form():
    """`strconv.FormatFloat(v, 'f', -1, 64)` has no exponent form at all.

    Python's `repr` switches to one outside a range, and a money column written
    as `1e+22` is a column every consumer parses differently.
    """
    for value in (1e16, 1e22, 1.5e-8, 0.1 + 0.2):
        text = writers.encode_rdf_type_to_txt(value)
        assert "e" not in text and "E" not in text, (value, text)
    assert writers.encode_rdf_type_to_txt(float("nan")) == "NaN"
    assert writers.encode_rdf_type_to_txt(float("inf")) == "+Inf"
    assert writers.encode_rdf_type_to_txt(float("-inf")) == "-Inf"


# --- the quoting rule --------------------------------------------------------


@pytest.mark.parametrize(
    ("field", "quoted"),
    [
        ("", False),
        ("plain", False),
        ("with,comma", True),
        ('with"quote', True),
        ("with\nnewline", True),
        ("with\rcarriage", True),
        (" leading space", True),
        ("trailing space ", False),
        (" nbsp", True),
        (" line separator", True),
        ("\x1frecord separator", False),
        ("\\.", True),
        ("\\.x", False),
    ],
)
def test_the_quoting_rule_is_the_forked_writer_s(field: str, quoted: bool):
    assert writers.field_needs_quotes(field, ",") is quoted


def test_a_record_separator_is_whitespace_to_python_and_not_to_go():
    """The one case `str.isspace()` would have got wrong, named out loud.

    Go's `unicode.IsSpace` is the Unicode White_Space property; Python's
    `str.isspace()` also answers true for `\\x1c`-`\\x1f`. A field beginning with
    one would be quoted by this writer and not by the engine's.
    """
    assert "\x1f".isspace() is True
    assert writers.field_needs_quotes("\x1fx", ",") is False


def test_the_four_clauses_are_the_ones_the_go_source_states():
    """The rule read off `jets/csv/writer.go` rather than believed.

    Its own comment enumerates them — a comma, a quote, a newline, a leading
    space — and the body adds the Postgres terminator. The assertion is that all
    five still appear in that function, so a fork that dropped one is caught
    here.
    """
    source = go_source("jets/csv/writer.go")
    body = source.split("func (w *Writer) fieldNeedsQuotes", 1)[1]
    assert "field == `\\.`" in body
    assert "unicode.IsSpace(r1)" in body
    assert "c == '\\n' || c == '\\r' || c == '\"' || c == byte(w.Comma)" in body
    assert writers.POSTGRES_TERMINATOR == "\\."


def test_the_delimiter_is_honoured_by_the_rule_and_by_the_record():
    assert writers.field_needs_quotes("a|b", "|") is True
    assert writers.field_needs_quotes("a|b", ",") is False
    assert (
        writers.write_csv(["x"], [["a|b"]], header=False, delimiter="|") == b'"a|b"\n'
    )


# --- the csv writer ----------------------------------------------------------


def test_the_header_is_the_column_list_and_the_rows_are_encoded():
    data = writers.write_csv(["a", "b"], [[1, None], ["x,y", dt.date(2020, 3, 4)]])
    assert data == b'a,b\n1,\n"x,y",2020-03-04\n'


def test_a_headerless_file_carries_no_column_names():
    assert writers.write_csv(["a"], [["v"]], header=False) == b"v\n"


def test_quote_all_quotes_every_field_including_the_header():
    assert writers.write_csv(["a"], [["v"]], quote_all=True) == b'"a"\n"v"\n'


def test_no_quotes_writes_every_field_verbatim_and_is_lossy():
    """The flag exists for consumers that cannot parse quotes; it is not safe.

    A value containing the delimiter produces a line with more fields than
    columns, which is asserted rather than warned about.
    """
    data = writers.write_csv(["a", "b"], [["x,y", "z"]], header=False, no_quotes=True)
    assert data == b"x,y,z\n"
    assert data.count(b",") == 2


def test_crlf_ends_every_line_and_rewrites_a_newline_inside_a_field():
    data = writers.write_csv(["a"], [["p\nq"]], use_crlf=True)
    assert data == b'a\r\n"p\r\nq"\r\n'


def test_a_multi_character_delimiter_is_refused():
    with pytest.raises(WriterError, match="one character"):
        writers.write_csv(["a"], [], delimiter=";;")


def test_an_empty_table_writes_its_header_and_nothing_else():
    assert writers.write_csv(["a", "b"], []) == b"a,b\n"


# --- the parquet writer ------------------------------------------------------


def read_parquet(data: bytes):
    import pyarrow.parquet as pq

    return pq.read_table(io.BytesIO(data))


def test_the_parquet_schema_is_the_column_list_all_string_and_nullable():
    """`BuildParquetSchemaInfo` makes every field a nullable string.

    Read off the Go source as well as asserted here, because it is the fact X3
    rests on: the two writers cannot disagree about a *type* if there is only one
    type.
    """
    source = go_source("jets/compute_pipes/parquet_schema_info.go")
    builder = source.split("func BuildParquetSchemaInfo", 1)[1].split("\n}\n", 1)[0]
    assert "arrow.BinaryTypes.String.Name()" in builder
    assert "Nullable: true" in builder

    table = read_parquet(writers.write_parquet(["a", "b"], [[1, "x"]]))
    assert [field.name for field in table.schema] == ["a", "b"]
    assert all(str(field.type) == "string" for field in table.schema)
    assert all(field.nullable for field in table.schema)


def test_a_row_of_the_wrong_width_is_refused_naming_both_widths():
    with pytest.raises(WriterError, match="does not match"):
        writers.write_parquet(["a", "b"], [["only one"]])


def test_the_batch_size_defaults_to_the_go_writer_s():
    assert writers.PARQUET_DEFAULT_BATCH_SIZE == 1024
    source = go_source("jets/compute_pipes/parquet_write_file.go")
    assert "nrowsInRec = 1024" in source


# --- X3 ---------------------------------------------------------------------

#: A table containing every shape the encoder distinguishes, so that comparing
#: the two writers compares something. The columns are the same tuple for both,
#: which is the property under test.
X3_COLUMNS = ("text", "number", "money", "when", "flag", "empty", "absent")
X3_ROWS = (
    ["plain", 5, Decimal("12.50"), dt.date(2020, 3, 4), True, "", None],
    [
        'with"quote, and comma',
        -1,
        Decimal("0.00"),
        dt.datetime(2020, 3, 4, 5, 6, 7),
        False,
        " ",
        None,
    ],
    ["\nnewline", 0, Decimal("-3.75"), None, None, "\\.", None],
)


def test_the_two_writers_agree_cell_for_cell():
    """**X3 at the writer level.** One column list, two containers, same cells.

    The CSV side is parsed back with Python's own reader rather than compared as
    text, because what the criterion asks about is cells. The one documented
    difference is the null: CSV has no way to write one, so a null and an empty
    string are both an empty field — asserted in the next test rather than
    smoothed over here.
    """
    import csv

    csv_bytes = writers.write_partition(
        "csv_writer", X3_COLUMNS, X3_ROWS, output_format="csv"
    )
    parquet_bytes = writers.write_partition(
        "parquet_writer", X3_COLUMNS, X3_ROWS, output_format="parquet"
    )

    parsed = list(csv.reader(io.StringIO(csv_bytes.decode("utf-8"))))
    assert parsed[0] == list(X3_COLUMNS)
    table = read_parquet(parquet_bytes)
    assert table.column_names == list(X3_COLUMNS)

    from_parquet = [
        ["" if cell is None else cell for cell in row]
        for row in zip(*[table.column(name).to_pylist() for name in X3_COLUMNS])
    ]
    assert [list(row) for row in parsed[1:]] == from_parquet
    # And it examined something: the table is not empty and every column was
    # compared, which a zero-row comparison would also satisfy.
    assert len(from_parquet) == len(X3_ROWS) == 3
    assert len(X3_COLUMNS) == 7


def test_the_one_difference_is_the_null_and_it_is_the_normalisation_x3_allows():
    table = read_parquet(writers.write_parquet(["a", "b"], [[None, ""]]))
    assert table.column("a").to_pylist() == [None]
    assert table.column("b").to_pylist() == [""]
    assert writers.write_csv(["a", "b"], [[None, ""]], header=False) == b",\n"


def test_neither_writer_derives_a_column_list_of_its_own():
    """Asserted structurally, because a correct docstring stopped nobody (P7-I88).

    The module holds no tuple of column names, so a writer cannot disagree with
    its caller about the columns: both take the argument. The check is over the
    source text of the two writers, and it is a *negative* claim, which is why it
    is worth making mechanically.
    """
    import inspect

    for function in (writers.write_csv, writers.write_parquet):
        source = inspect.getsource(function)
        # Every reference to columns is the parameter; no literal name appears.
        assert not re.search(r"\[\s*['\"][a-z_]+['\"]\s*,", source), function.__name__
        assert "columns" in source


# --- the dispatch ------------------------------------------------------------


@pytest.mark.parametrize(
    ("writer", "fmt", "ok"),
    [
        ("csv_writer", "csv", True),
        ("csv_writer", "headerless_csv", True),
        ("csv_writer", "parquet", False),
        ("parquet_writer", "parquet", True),
        ("parquet_writer", "parquet_select", True),
        ("parquet_writer", "csv", False),
    ],
)
def test_a_writer_accepts_only_the_formats_the_go_constructor_lists(
    writer: str, fmt: str, ok: bool
):
    if ok:
        writers.check_device_writer(writer, fmt)
        return
    with pytest.raises(WriterUnsupported, match="does not support file format"):
        writers.check_device_writer(writer, fmt)


def test_the_format_sets_are_the_go_constructor_s():
    source = go_source("jets/compute_pipes/pipe_transformation_partition_writer.go")
    body = source.split("// Verify that the device writer supports the file format", 1)[
        1
    ]
    for writer, formats in writers.SUPPORTED_FORMATS.items():
        assert f'case "{writer}":' in body
        for fmt in formats:
            assert f'"{fmt}"' in body


def test_fixed_width_is_declared_by_the_contract_and_refused_by_name():
    assert "fixed_width_writer" in writers.FILE_EXTENSIONS
    with pytest.raises(WriterUnsupported) as exc:
        writers.check_device_writer("fixed_width_writer", "fixed_width")
    assert "offsets" in str(exc.value)
    assert "csv_writer" in str(exc.value)


def test_an_unknown_device_writer_names_the_two_that_exist():
    with pytest.raises(WriterUnsupported, match="unknown device_writer_type"):
        writers.check_device_writer("hand_writer", "csv")


def test_snappy_is_refused_and_the_refusal_names_the_authored_consequence():
    """**P9-I59**: the framing format, and what a stage channel defaults to."""
    writers.check_compression("none")
    with pytest.raises(WriterUnsupported) as exc:
        writers.check_compression("snappy")
    assert "framing" in str(exc.value)
    assert '"compression": "none"' in str(exc.value)
    # The Go writer's own wrapper, so the claim is checked against the source.
    source = go_source("jets/compute_pipes/s3_device_writter.go")
    assert "snappy.NewBufferedWriter" in source


def test_a_parquet_partition_ignores_the_compression_setting():
    """Parquet is always snappy in the Go writer, so the csv check must not run.

    Asserted by writing parquet with a compression the csv writer refuses: if
    the check were unconditional, this would raise.
    """
    data = writers.write_partition(
        "parquet_writer",
        ["a"],
        [["v"]],
        output_format="parquet",
        compression="snappy",
    )
    assert read_parquet(data).column("a").to_pylist() == ["v"]


def test_the_partition_file_name_is_the_one_both_engines_form():
    assert writers.partition_file_name(0, 1, "csv_writer") == "part0000-0000001.csv"
    assert (
        writers.partition_file_name(39, 12, "parquet_writer")
        == "part0039-0000012.parquet"
    )
    source = go_source("jets/compute_pipes/pipe_transformation_partition_writer.go")
    assert '"part%04d-%07d.%s"' in source


def test_only_the_csv_format_carries_a_header():
    assert (
        writers.write_partition("csv_writer", ["a"], [["v"]], output_format="csv")
        == b"a\nv\n"
    )
    assert (
        writers.write_partition(
            "csv_writer", ["a"], [["v"]], output_format="headerless_csv"
        )
        == b"v\n"
    )
    source = go_source("jets/compute_pipes/s3_device_writter.go")
    assert 'ctx.spec.OutputChannel.Format == "csv" &&' in source
