---
title: "A map column: empty, rejected and absent values"
description: "What a map column writes when its input is absent, empty or rejected, per record in map_record or accumulating in aggregate and map_reduce."
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
