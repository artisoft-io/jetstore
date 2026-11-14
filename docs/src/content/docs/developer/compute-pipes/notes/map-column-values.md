---
title: "A map column: empty, rejected and absent values"
description: "What a map column writes when its input is absent, empty or rejected, per record in map_record or accumulating in aggregate and map_reduce, and how the substring and overpunch_number cleansing functions treat positions out of range."
kind: reference
sidebar:
  order: 14
---

**Recorded 2026-10-18.**

A `map` column (`mapColumnEval.Update`, `column_evaluators_mapping.go`) takes one input value, passes
it through the cleansing function if any, casts it to `rdf_type`, and maps it through the code value
mapping if any. What it writes when that does not produce a value depends on two things: why there is
no value, and whether the column writes one record per input record or accumulates across records.

**Three situations.**
- **Absent**: the input value is null.
- **Rejected**: the input has a value and the cleansing function reports an error or the cast fails.
- **Empty**: the input has a value but none comes out of the map. The value is blank (empty or only
  white space) before or after cleansing (a `find_and_replace` of a placeholder with `""`, say), the
  cast yields null, or the code value mapping yields a blank string.

**A map never outputs a blank string.** A blank result is treated as empty.

**Two modes.** `map_record` marks every column it builds as per record (`markPerRecord`,
`pipe_transformation_map_record.go`). Everywhere else, in an `aggregate` or in a `map_reduce`'s
`apply_map` or `apply_reduce`, a map column accumulates: its output position is kept across records.

| Situation | Per record (`map_record`) | Accumulating (`aggregate`, `map_reduce`) |
|---|---|---|
| Valid value | the cast, mapped value | the cast, mapped value |
| Absent, with a default | the default | the default |
| Absent, no default | the output is left as it is; the `err_msg` error is returned if `err_msg` is set | the same |
| Empty, with a default | the default | **skipped**: nothing is written, the output keeps its value |
| Empty, no default | **null is written**; the `err_msg` error is returned if `err_msg` is set | **skipped** |
| Rejected, with a default | the default, no error | **skipped**, no error |
| Rejected, no default | **null is written**, and the cleansing or cast error is returned | **skipped**, no error |

The default itself goes through the code value mapping; if that makes it blank, the result is null.

**Why the cases differ.**
- **An absent input leaves the output alone** so that several map columns can fall back onto one
  output: the first with an input value wins, and a later one whose input is missing does not erase
  it.
- **In `map_record` an empty or rejected value is overwritten** because the input had a value. Without
  `new_record`, `map_record` updates its input record in place (`MapRecordTransformationPipe.Apply`),
  and a column often maps onto itself, so the output position still holds the raw input. Leaving it
  alone would let the value the map just emptied or rejected flow downstream. A non-numeric string
  left in a numeric column then fails the first comparison that reads it, such as a `count`'s `where`
  (`opLT string and double`).
- **In an accumulating column an empty or rejected value is skipped** because the output holds the
  value accumulated over earlier records. One record whose value fails the cast must not reset it to
  null or to the default part way through. An error is not returned either: `aggregate` and
  `map_reduce` have no `on_error` policy, so it would stop the step.

**Where an error goes.** In `map_record` a returned error is handled by its `on_error` setting: the
default `pass_through` keeps the record, logs the error and reports it to the error channel, capped at
`max_error_count`.

**Measured** by `column_evaluators_mapping_test.go`, which builds both modes directly, including a map
column updated in place followed by a `count` with a `where` on it. No pipeline in the workspace corpus
has a `map` column in an `aggregate` or a `map_reduce` as of 2026-10-18.

## Cleansing functions that take a position

**Recorded 2026-11-13.** Two cleansing functions read positions from their argument. Out of range,
neither stops the step: before this date a negative `substring` start, or an `overpunch_number`
value shorter than its decimal places, panicked, and the panic is recovered by the pipe, not per
record, so it failed the step.

**Positions are bytes, not characters.** A value is sliced as UTF-8 bytes, so a position that falls
inside a multi-byte character (an accented letter, say) splits it, and the result is not valid UTF-8.
This is deliberate and unchanged; the rule engine's `substring_of` counts bytes too. Use these
functions on ASCII codes and identifiers.

### `substring`

The argument is `start,end`: two integers, white space around each allowed. `start` is inclusive,
`end` exclusive, both counted from 0 at the first byte. A negative `end` counts back from the end of
the value: `0,-2` drops the last two bytes.

**Refused when parsed**, which makes the value *rejected* in the table above, on every record (a
refused argument is not cached):
- not exactly two integers (`3`, `1,2,3`, `a,3`), or an empty argument;
- a negative `start` (`-1,3`);
- a positive `end` not greater than `start` (`5,3`, `3,3`).

The argument is parsed when the first record reaches the column, not when the pipeline is built, so a
refused argument is found by a run, as one error per record under `map_record`'s `on_error`.

**Empty** (no value, the *empty* row of the table) when `end`, once a negative one is resolved
against the value's length, is past the end of the value or not after `start`:

| Argument | Value | Result |
|---|---|---|
| `0,3` | `abcdef` | `abc` |
| `1,-1` | `abcdef` | `bcde` |
| `0,10` | `abcdef` | empty: `end` past the value |
| `8,10` | `abcdef` | empty: `start` past the value |
| `0,3` | (empty) | empty |
| `0,-10` | `abcdef` | empty: a negative `end` reaching back past `start` |
| `4,-3` | `abcdef` | empty: `end` resolves to 3, before `start` |
| `0,0` | `abcdef` | empty, on every value: an `end` of 0 is never after `start` |
| `0,2` | `héllo` | `h` and the first byte of `é`: a split character |

A value too short for its `end` gives nothing rather than what it has: `0,10` on `abcdef` is empty,
not `abcdef`.

### `overpunch_number`

The argument is the number of decimal places, an integer of 0 or more: the right-most characters,
the overpunch character included, that are the decimal part. A negative or non-integer argument is
*rejected* on every record; an empty one stops the step, as for the other functions whose argument is
required.

**A value with fewer characters than decimal places is padded with zeros on the left**, as a
fixed-width field whose leading zeros were dropped: `5` with 2 decimal places is `.05`, as `05` is,
and `E` is `+.05`. A value at least as long as the decimal places is decoded as before.

**Measured** by `out_of_range_test.go` in `jets/cleansing_functions`, which runs every case above
through `ApplyCleasingFunction`.
