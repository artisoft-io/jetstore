---
title: "Values and operators"
description: "What the rule engine computes for sums, doubles, text, substrings, regular expressions, arithmetic, comparisons, ties, lookups and text read as a bool."
kind: reference
sidebar:
  order: 2
---

What the rule engine computes for values, operators, lookups and text read as a bool. Most of it was
settled on 2026-10-22, when the C++ and Go engines were compared value by value and each difference
decided (see [The retired Go engine](/docs/developer/engine/rule-engine/go-engine/)); what the Go engine did is kept as history, and a Go file
it names is at the tag `go-engine-final`. The list of operators is the generated
[JetRules operators](/docs/developer/reference/generated/jetrules-operators/) page.

## `substring_of` at the edges — 2026-11-13

`(config substring_of ?text)` takes its cut from the configuration resource `config`: `jets:from`,
the characters skipped from the start, and `jets:length`, the characters kept or, when negative,
the characters dropped from the end. Both must be `int` values (32-bit); anything else inserts
`(jets:iState jets:exception "…")` and the expression is null.

| Case | Result | Example: text `"abcdef"` |
|---|---|---|
| the cut is within the text | the characters cut | from 1, length 3: `"bcd"`; from 1, length -2: `"bcd"` |
| `jets:from + jets:length` runs past the end | the characters that are there | from 4, length 10: `"ef"` |
| `jets:from` at the end of the text | `""` | from 6, length 2: `""` |
| `jets:from` past the end, or negative | `""` | from 9, length 2: `""`; from -1, length 2: `""` |
| a negative `jets:length` dropping as much as is left after `jets:from`, or more | `""` | from 2, length -4: `""`; from 0, length -9: `""` |

**Positions and lengths count bytes, not characters.** Text is UTF-8, so on text that is not
ASCII a position counts each byte of a multi-byte character, and a cut can split one: from 0,
length 1 on `"été"` is the first byte of `é` alone, which is not valid UTF-8.

**Before 2026-11-13 the two edge cases did not give `""`.** A negative `jets:length` on text shorter
than the cut was computed in unsigned arithmetic and wrapped around, so from 0, length -3 on `"ab"`
gave the whole remainder, `"ab"`. A `jets:from` past the end, or negative, made `std::string::substr`
throw `std::out_of_range`, which failed the session's `ExecuteRules`. Both now give `""`, as the
retired Go engine did for the first two (`expr_operator_str_substring_of.go` at `go-engine-final`;
it gave null for a negative `jets:from`). The rows are `ExprOpTest.SubstringOfVisitorEdges` in
`jets/rete/expr_op_test.cc`; the operator is `SubstringOfVisitor` in `jets/rete/expr_op_strings.h`.

## Values and operators — 2026-10-22

Go jetstore#2331, C++ jetstore#2333.

**`sum_values`.** The sum does not depend on the order the graph holds the values in.
- A null is skipped wherever it comes. No value, or only nulls, gives null.
- Numbers alone: a double anywhere makes the sum a double, every value converted.
- Integers alone are summed exactly, in the widest integer type present (int32 < uint32 < int64 <
  uint64), else int64, else a double. A Go int reaches C++ as an int32 and a uint as a uint32 (the
  bridge's `create_int` and `create_uint`), so Go gave a uint while the sum fit 32 unsigned bits.
- Any value that is not a number (text, a date, a resource) makes the sum null, text alone
  included. `join_values` is what concatenates text.

Before: Go started from an int 0 and truncated every double added to it, a null nulled the sum, and
no value summed to 0. C++ nulled the sum when a null came first and skipped it later, and an int met
before a double truncated the double. On a mix of text and numbers the old fold gave `"x25"` or
nothing, depending on which came first.

**Double literals.** A double literal is the double given, as it always was on C++.
- Go's `ResourceManager.NewDoubleLiteral` rounded every double to 15 bits of mantissa (0.891089
  became 0.891082763671875, 1234567.89 became 1234560).
- That reached rule literals, input record values, lookup values and a consequent's computed double
  (`ReifyResource`).
- Two doubles one ulp apart are now two literals, as on C++.
- -Inf is its own literal; Go returned +Inf for it.

**A value written into text.** Every operator that writes a value into text wrote the
same text on both engines. That is text plus a value, `to_text`, `create_entity`, `create_resource`
(Go; C++ has no double case) and `join_values`.
- A double is `std::to_chars(..., std::chars_format::fixed)` with no precision: the fewest digits
  that read back as the same double, in plain notation, never an exponent. 991 is `"991"`,
  1234567.5 is `"1234567.5"`, 0.00001 is `"0.00001"`.
- From 2^53 up it is the double's exact digits (1e23 is `"99999999999999991611392"`).
- NaN is `"nan"` (sign dropped), and the infinities are `"inf"` and `"-inf"`, as `std::to_string`
  wrote them.
- A date is `YYYY-MM-DD`, a datetime `YYYY-MM-DDTHH:MM:SS`, with `.ffffff` only when the
  microseconds are not zero (boost's `to_iso_extended_string`), and a null is `"null"`.
- C++'s side is `rdf::double_to_text` and `rdf::to_string`; Go's was `rdf.TextOf` and
  `rdf.FormatDouble` (`jets/jetrules/rdf/text.go` at `go-engine-final`). The shared table was `DoubleTextCases` (28 rows,
  `jets/jetrules/rdf/text_test.go` at `go-engine-final`); its C++ copy, `DoubleToText.ShortestDigitsNeverAnExponent`, stays.
- Before: C++ wrote `std::to_string`'s six decimals (`"rate: 991.000000"`). Go wrote `%v`, with an
  exponent from 1e6 and below 1e-4 (`1.2345675e+06`), or `%G` with 15 digits (`1E+06`) for a minted
  name. A date came out as Go's struct (`{2025-06-05 00:00:00 +0000 UTC}`). `join_values` wrote `%g`'s
  six significant digits on both.
- `apply_format` follows the rule's own format string and is unchanged.
- **`Node.String()` was not this rendering.** It was the Go engine's debugging and export form, and
  wrote a date as the struct.

**`literal_regex`.** Both engines returned capture group 1, and null when nothing
matches. A pattern with no group 1, or a group 1 that took no part in the match, gives `""`, as
C++'s unmatched `sub_match` does. Go returned the whole match, or `""` on no match, before #2331. `apply_regex`
was Go's alias, with the same meaning; C++ never registered it.

**Arithmetic.** In `+`, `-`, `*` and `/`, any double operand makes the result a double, both
operands converted, whichever side it is on.
- Before: `+` and `-` truncated *int op double* to an int on both engines (`2 + 0.5` was 2, `0.5 + 2`
  was 2.5), and Go's `*` too.
- Two integers keep C++'s rule: the left operand's type, the right converted to it, and integer
  division. Go gained its missing uint cases.
- The shared table was `TestArithmeticTyping` (`jets/jetrules/rdf/ast_math_operators_test.go` at `go-engine-final`); its
  C++ copy is `ArithmeticTypes.TheSharedTypingTable`.

**Comparisons.** This covers `==`, `!=`, `<`, `<=`, `>` and `>=`.
- Integers with integers, int against uint included, compare by value (`std::cmp_*`): int 5 equals
  uint 5.
- Any double operand compares as two doubles, by C++'s rule: each value rounded to 15 significant
  digits (`round_to_digits`), then Knuth's `fcmp` at machine epsilon (`rdf::is_eq` and its siblings,
  `jets/rdf/rdf_ast.h`).
- A NaN or an infinity is never equal and never ordered, and `!=` is the negation of `==`.
- Before:
  - Go used a relative 1e-10 tolerance, and called a NaN literal equal to itself (one shared node).
  - Go had no uint case at all.
  - C++ cast the double to the integer's type for `==` (`5 == 5.5` was true, a `uint64` was cut to
    `uint32`, and a double out of the integer's range threw).
  - C++'s int-against-double `<` and `>` skipped the rounding, so an infinity ordered above every
    number.
- Go reproduced C++'s rounding with a port of glibc's `__ieee754_log10` (`glibcLog10`) and glibc's
  powers of ten (`pow10Table`), both mirroring x86-64 glibc 2.39 (`jets/jetrules/rdf/compare.go` at `go-engine-final`).
  The shared table was `ComparisonCases` (`jets/jetrules/rdf/compare_test.go` at `go-engine-final`); its C++ copy is
  `Comparisons.TheSharedTable`.
- **Fast path, the same in both engines and result-preserving.** Equal doubles are equal. Doubles
  further apart than 2e-14 of the larger keep their plain order: rounding moves each by at most
  5e-15 of itself and is monotonic. Only closer pairs are rounded. Each engine checked this against
  the slow rule on 10^6 pairs. Roughly measured on Go, a double against an int went from 49 to 15 ns.
- **Tiny doubles.** Below about 1e-294 the rounding's scaling factor overflowed, the rounded value
  was NaN, and `fcmp` takes NaN for equal, so `1e-300 == 5` and `1e-300 <= -7` were true. Such a
  value is now compared unrounded.

**Division by zero.** An integer divided by an integer zero is null, as it was on both engines. A double
operand divides as IEEE does: `+inf`, `-inf`, and NaN for 0/0. Before, C++ raised SIGFPE and ended
the process, and Go gave NaN for all of them. The shared table was `DivisionByZeroCases`; its C++
copy is `ArithmeticTypes.DivisionByZero`.

**Four operations that failed the session give null — decided 2026-10-23.** Each threw or trapped and ended the session; each now gives null, as integer division by
zero does, so the session goes on and the rule's consequent sees no value. Only a session that
failed before computes anything different. Each has its rows in a table of `expr_op_test.cc`:

| Operation | Was | Rows |
|---|---|---|
| the minimum `int32` or `int64` divided by -1, directly or through a mixed integer type | SIGFPE, which ended the process | `ArithmeticTypes.DivisionByZero` |
| `+ - * /` on two integer types, the right operand out of the left operand's range: an `int32` plus a large `int64`, a `uint32` plus -1 | `boost::numeric_cast` threw | `ArithmeticTypes.TheSharedTypingTable` |
| text plus a named resource, either way round | threw in a session's rule | `ArithmeticTypes.TheSharedTypingTable` |
| `lookup` or `multi_lookup` with a null key | threw in a session's rule | `Lookups.ANullKeyIsNull` |
| *Added 2026-10-23, from the review of the first four:* `to_int` of a number out of `int32`'s range | `boost::numeric_cast` threw | `ArithmeticTypes.ToIntOutOfRangeIsNull` |
| text with a blank node, null with text and a double with text, either way round, in each of `+ - * /`; text plus null and text plus a double still concatenate | threw in a session's rule | `ArithmeticTypes.TheSharedTypingTable` |
| `lookup` or `multi_lookup` with a blank-node key | threw in a session's rule | `Lookups.ANullKeyIsNull` |
| *Added 2026-10-24:* **every type mismatch of `+ - * /` and `abs`**: a number with text, a named resource, a blank node or null, either way round, and two operands neither of which the operator takes; text first in `+` still concatenates, by design | threw in a session's rule | `ArithmeticTypes.EveryTypeMismatchIsNull` |
| `to_int` and `to_double` of what they cannot read: text that does not read as their type, empty or blank text, null, a named resource, a blank node or a date | threw in a session's rule; text it could not read threw with no row too | `ArithmeticTypes.ToIntAndToDoubleOfWhatTheyCannotReadIsNull` |
| `to_int` of a NaN | converted to `int32` by undefined behaviour, which `numeric_cast` does not check (the minimum `int32` on x86-64) | `ArithmeticTypes.ToIntAndToDoubleOfWhatTheyCannotReadIsNull` |
| `lookup` or `multi_lookup` with a key or a table of a type it does not take: a double, a date or a datetime key, a table named by anything but a resource or text | threw in a session's rule | `Lookups.AKeyOrTableOfAnotherTypeIsNull` |

`+ - * /` are their `Impl` visitors wrapped in `NullWhenOutOfRange`, which turns a
`bad_numeric_cast` into null (`expr_op_arithmetics.h`); integer division goes through
`integer_quotient`. An operand that fits converts as before. **"In a session's rule" was not a
detail**: with no beta row, as the shared tables call a visitor, an operator's catch-all gave null
for operands it does not take; with a row, it threw. So a table that calls a visitor with no row can
pass for a case that fails a session, and every row of the cases above calls it with a row. Until
2026-10-24 the null mixes were added one overload at a time; **since then the catch-alls of `+ - * /`,
`abs`, `to_int`, `to_double`, `lookup` and `multi_lookup` are null, with a row or without**, and the
overloads that only gave null are gone. A grid of those operators over 33 operand values, each pair
with a row, counts **3,641 cells that threw from a type mismatch and none now** (2026-10-24; 1,650
in an earlier grid of 44 values over `+ - * /`, `lookup` and `multi_lookup`, 2026-10-23).
**Other operators' catch-alls still throw in a session's rule**: `age_as_of` and
`age_in_months_as_of` on anything but their dates, and `to_text` of a blank node, among those the
grid counts. They are not arithmetic, and the change of 2026-10-24 did not name them.

**Ties in `min_of`, `max_of` and `sorted_head`, and Go's `min_head_of` and `max_head_of`.**
Among equal values the first met used to win, and the graph's iteration order varies from run to
run (C++'s hash sets by address and seed; Go's maps on every range). So a tie gave a different head
on each run, and a rule set's duplicate removal kept a different record.

A tie is now two values the engine's own `==` calls equal (C++ `EqVisitor`, Go's `EQ`); from 2026-10-22 the
two agreed. On a tie, the object first in this total order wins:
1. by kind: named resource, blank node, literal, null;
2. a resource by its name and a literal by its text (the rendering above), compared as bytes;
3. on equal text, by type, signed before unsigned: int32, int64, uint32, uint64, double, text, date,
   datetime (Go: int, uint, double, text, date, datetime);
4. blank nodes by key, last.

So equal weights give the smallest name, in any insertion order. C++ has `tie_break_less`; Go had
`compareObjects` and `tieEqual` (`expr_operator_math_minmax.go` at `go-engine-final`).
- **Known limits.** A NaN, an infinity, or two values that do not compare are not a tie, and keep the
  first met, which is order-dependent. An entity named by a UUID still orders differently
  from run to run.
- **`max_of` and `min_of`** return the winner's value, so they move only between equal values of
  different types: in the direct form, 5 and 5.0 give the int every time.

## Lookups name their table — 2026-10-22

Go jetstore#2330.

Go's lookup subjects were `jets:lookup:<key>`, with no table name. So two tables looked up by one
key shared a subject, and the second lookup read the first one's cached row. In `jets_ws` the
diagnosis and procedure description lookups crossed this way. They are now C++'s names:
`jets:<T>:<key>` for `lookup`, `multi_lookup` and `lookup_rand`, and `jets:rand:<T>:<n>` for
`multi_lookup_rand`, each cached as `(<T> jets:lookup_row subject)` (Go's `LookupTableSqlite3.lookup`,
`lookup_table_sqlite3.go` at `go-engine-final`).
- **A `multi_lookup` miss** returns null and writes nothing, as it did on both, so a rule testing a
  miss with `(?x p null)` works after it.
- **An integer key**, signed or unsigned, was converted to text instead of panicking on Go. A
  null key gave no result on Go; C++ throws. *(Since 2026-10-23 a null key gives null on C++:
  see *Four operations that failed the session give null*, above.)*
- **`graphdump`** maps the current cache form to the old Go name, so dumps made before and after
  compare equal.

## Text read as a bool: one specification on both engines — 2026-10-11

**Until 2026-10-11 the two engines read a string as a bool the opposite way round.** C++
(`rdf::to_bool`) took any text but a few spellings of false as true; Go (`rdf.ParseBool` for
literals, `Node.Bool` for a filter) took any text but a few spellings of true as false, and
`Node.Bool` called `"false"` true. Rule configuration read the first letter, so `tomato` was true.
None of the four workspaces reached the difference on that day: no rule file wrote `bool("…")` or
`bool x = "…";`, and no filter tested a bare variable.

**Now one parser, `rdf::parse_bool` in C++, and `rdf.ParseBool` in Go on the same sets**: trimmed,
in any case, `true, t, yes, y, 1` and `false, f, no, n, 0`; empty is **null**; anything else is an
error. The parser decides nothing about what an error means. Each caller does, and that is the
specification:

| Where | Empty | Not empty and not in the sets |
|---|---|---|
| Rule-file bool literal | null | compile error naming file and line (`checkBoolLiteral`, compilerv2); refused at load by the engine |
| Lookup bool column (packager, `jets/workspace/lookup_tables.go`) | SQL `NULL`, served as rdf null | compile error naming lookup, CSV row, column and value |
| Rule configuration (`ParseRdfNodeValue`) | null | pipeline fails at start |
| Text a rule tests for truth: filter, `and`, `or`, `not` | null; a filter does not pass; no exception | `(jets:iState jets:exception "…")` and the expression is null; the rule set carries on and the pipeline reports it under `on_error` |
| `map_record` bool cell | null (an empty cell is never cast) | record-level error under the operator's `on_error` |
| Typed file read (`castToRdfTypeFromTxt`) | null | the read fails, as for a bad date or int: JetStore wrote the file |
| Rules unit test expected bool | null | the check errors |

**Three things that were wrong on the way, and are fixed with it.** `bool x = "false";` kept its
quotes, so both engines received `"false"` with the quote characters: true on C++, false on Go.
C++ `not` on any text raised `Invalid arguments for not`, failing the session, even for `"false"`.
And C++ read a SQL `NULL` in an int or double lookup column as `0` where Go served null
(`lookup_sql_helper.cc`), which mattered the moment an empty bool cell became `NULL`.

**The run-time exception is the C++ `substring_of` pattern** (`expr_op_strings.h`), which Go took up
as `ReteSession.RaiseException`: a rule-time error is data in the session, not a failed
`ExecuteRules`, so one bad value in one row does not stop the others.
