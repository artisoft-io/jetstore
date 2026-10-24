# `jets/rete` — the C++ rule engine: its semantics and their history

What you cannot see from the code in front of you. Newest first.

**JetStore has one rule engine, this one.** Until October 2026 it had two: this C++ engine
(`jets/rete` and `jets/rdf`, built as `libjets.so` and reached through `jets/bridge` and
`jets/compute_pipes/jetrules_native_adaptor`), which every deployment ran, and a Go engine
(`jets/jetrules/rete` and `jets/jetrules/rdf`, reached through `jets/compute_pipes/jetrules_go_adaptor`).
The Go engine was retired (see the first entry), and this file, which was its package's notes
(`jets/jetrules/rete/README.md`), moved here with the retirement.

**How to read the entries below.** They were written while both engines ran, and are kept as
written apart from tense: **what they say of the Go engine is history**, and a Go file or test
they name is at the tag `go-engine-final`, not in this tree. What they say of the C++ engine is
current unless the entry says otherwise. Where the Go engine was the reference for a behaviour,
the C++ engine's tests named in the entry are now its specification.

## Leaks, a crash, a default limit and the sanitizer gate — 2026-10-23

`rules_benchmark`'s Phase 7 (track `BK`, with `BO.3`), on the one engine. None of it changes what a
session computes unless the session failed or ran without a limit before.

- **A session with no `jets:max_vertex_visits` stops at 10,000 visits of a vertex** (Michel,
  2026-10-23: without one, a rule that never settles ran until the pipeline timed out, with no
  message). `ReteSession::kDefaultMaxVertexVisits`. The error is the one a set limit gives, with
  the limit and the setting named: `{"error":"Maximum vertex visits reached","limit":10000,
  "reason":"the default limit of 10000 visits of a rule's vertex, since jets:max_vertex_visits is
  not set; ...","data":{...}}`. A set limit gives the message it always gave. A value of 0 or less
  still turns the limit off; an integer of any width is read as the limit, where only an `int32`
  was, and a value that is not an integer gets the default with a warning. Production processes
  set 1,000,000. `VertexVisitLimitTest` in `meta_store_sharing_test.cc`.
- **The ISO date and datetime getters of the C wrapper return text that outlives the call**
  (I-162). They returned `c_str()` of a temporary, so the bridge read freed memory; a datetime's
  text is past the small-string buffer, so it read freed heap. The text is copied into a fixed
  buffer of the calling thread, valid until that thread's next ISO getter call and never
  reallocated. The C signatures are unchanged. `cwrapper_test.cc`.
- **A factory closes what it opened, and so does the lookup helper** (I-144, I-170, I-180).
  `~ReteMetaStoreFactory` calls `reset()` (it was commented out); `LookupSqlHelper` and each
  connection pool close their connections when they go, so a factory deleted any way, not only
  through `delete_jetstore_hdl`, leaks nothing; the helper is owned by the meta stores, so it goes
  after the last session using it. And `TypeOf::type_of` on an unknown data property, and a reset
  or bind error in a lookup, now give their connection back to the pool: each kept one, which
  nothing closed. `LookupDbOpenTest.AFactoryClosesWhatItOpened` counts SQLite's own memory.
- **`lookup.db` is opened only when a lookup table is declared, and read-only** (I-167). It was
  opened on every engine build with `sqlite3_open`'s default flags, which create a missing file.
  Every pooled connection is read-only too: their statements are all `SELECT`s. A missing
  `lookup.db` with a table declared is now an error that creates nothing.
- **The antecedent query keys are set once, when the meta store is built** (I-86).
  `ReteMetaStore::initialize` numbers each vertex's children by the index of the parent relation
  they query (`AlphaNode::antecedent_query_index`); each session's `BetaRelation::initialize` used
  to write the key into the shared node vertex, a data race while the value was the same. A
  session now adds the indexes in the same order and checks the key. `MetaStoreSharingTest`.
- **The sanitizer gate is one command** (I-69), from the repository root:

  ```bash
  cmake -S . -B build-san -DCMAKE_BUILD_TYPE=Release -DJETS_SANITIZE=ON \
    -DJETS_ABSEIL_SOURCE=/path/to/abseil-cpp-20260107.1 \
    && make -C build-san -j8 jets_test && (cd build-san/jets && ctest)
  ```

  `JETS_SANITIZE` compiles and links with AddressSanitizer, leak detection on, and
  UndefinedBehaviorSanitizer, every report fatal, less `null`, `nonnull-attribute` and
  `returns-nonnull-attribute`, under which GCC 13 rejects a constexpr comparison in abseil's
  `hash_policy_traits.h`. abseil must be built with the same flags, since under AddressSanitizer
  its headers change the flat tables' layout: `JETS_ABSEIL_SOURCE` builds it in the tree, or
  `CMAKE_PREFIX_PATH` can name one installed so. The three lookup tests' leaks, which the gate had
  to switch leak detection off for, are gone.
- **The `writes_graph` marks are tested against what the operators do** (I-65).
  `OperatorWritesGraph` (`operator_writes_graph_test.cc`) evaluates every operator of
  `operators.txt` on a set of operands, in a session with a lookup table, and an operator that grows
  the inferred graph must be marked, and a marked one must grow it. Today that is `lookup`,
  `multi_lookup`, `lookup_rand`, `multi_lookup_rand`, `range` and `create_entity`, the six the
  file marks. An operator that writes only for an operand the test does not try would pass
  unmarked.
- **The reason for keeping the two retraction defects is gone**, corrected below in the 2026-10-08
  entry.

## The Go engine is retired — decided 2026-10-22

`jetstore_agentic_ai`'s `rules_benchmark` project benchmarked both engines on a client's claim
processes, fixed and settled their differences (the next entry), and Michel decided on 2026-10-22
to keep one engine, the C++ one, computing what it computed (that project's Phase 6). The last
`jets_ai` commit with the Go engine is tagged **`go-engine-final`**; `git checkout go-engine-final`
brings it back.

- **What stays of the Go side is the rule *model***, not an engine: `rete_meta_store_model.go`,
  `workspace_control.go` and `retraction_mode.go` in `jets/jetrules/rete`, and `ast.go` and
  `datetime_parser.go` in `jets/jetrules/rdf`. The compiler writes a workspace with them and the
  pipelines read their configuration through them.
- **A configuration that asks for the Go engine is refused**, with a message naming the
  retirement: `use_jet_rules_go: true` in a pipeline's or a bench spec's `jetrules_config`, and
  `use_jet_rules_go`, `compare_engines` or an engine of `go` in a rules unit test.
  `use_jet_rules_go: false` is accepted, so existing configurations need no change.
- **The shared tables are the C++ engine's specification.** Each of the tables the Phase 4 entry
  names lived in a Go test with a C++ copy; the C++ copies stay, in `jets/rete/expr_op_test.cc`:
  `DoubleToText.ShortestDigitsNeverAnExponent`, `ArithmeticTypes.TheSharedTypingTable`,
  `Comparisons.TheSharedTable` and `ArithmeticTypes.DivisionByZero`.
- **The compiler checks operator names against the C++ engine** (the last entry, superseded):
  `jets/compilerv2/compiler/operators.txt` lists every operator `create_binary_expr` and
  `create_unary_expr` register, the compiler refuses a rule calling any other, and
  `jets/rete/operator_list_test.cc` keeps the list equal to `expr_operator_factory.h`. It also
  marks the operators a filter may not call because they write to the graph, which the Go
  engine's operator types used to tell the compiler.
- **What was still different at the retirement** (the Phase 4 entry's last list) is now simply
  what the engine does.

## The engines' differences settled in rules_benchmark Phase 4 — 2026-10-22

`jetstore_agentic_ai`'s `rules_benchmark` project compared the two engines value by value, and Michel
decided each difference on 2026-10-22 (its `Q-60` to `Q-69`). This entry said what **both engines
did** from then until the Go engine's retirement, and since which pull request. Go: jetstore#2330,
#2331, #2334, #2336. C++: #2332, #2333, #2335, #2336. The end-to-end rule test is `jets_ws` `jet_rules/unit_tests/reentry_error_path/`
(jets_ws#55). The older entries below that these settle say so in place. **Shared tables** held both
engines to the same values: each table lived in a Go test, and a C++ test checks the same rows. The
C++ tests are now the specification (see the entry above).

### Values and operators (Go #2331, C++ #2333)

**`sum_values` (I-30, I-88).** The sum does not depend on the order the graph holds the values in.
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

**Double literals (I-40).** A double literal is the double given, as it always was on C++.
- Go's `ResourceManager.NewDoubleLiteral` rounded every double to 15 bits of mantissa (0.891089
  became 0.891082763671875, 1234567.89 became 1234560).
- That reached rule literals, input record values, lookup values and a consequent's computed double
  (`ReifyResource`).
- Two doubles one ulp apart are now two literals, as on C++.
- -Inf is its own literal; Go returned +Inf for it.

**A value written into text (I-16, I-32).** Every operator that writes a value into text wrote the
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

**`literal_regex` (I-12, `Q-11`).** Both engines returned capture group 1, and null when nothing
matches. A pattern with no group 1, or a group 1 that took no part in the match, gives `""`, as
C++'s unmatched `sub_match` does. Go returned the whole match, or `""` on no match, before #2331. `apply_regex`
was Go's alias, with the same meaning; C++ never registered it.

**Arithmetic (Q-67).** In `+`, `-`, `*` and `/`, any double operand makes the result a double, both
operands converted, whichever side it is on.
- Before: `+` and `-` truncated *int op double* to an int on both engines (`2 + 0.5` was 2, `0.5 + 2`
  was 2.5), and Go's `*` too.
- Two integers keep C++'s rule: the left operand's type, the right converted to it, and integer
  division. Go gained its missing uint cases.
- The shared table was `TestArithmeticTyping` (`jets/jetrules/rdf/ast_math_operators_test.go` at `go-engine-final`); its
  C++ copy is `ArithmeticTypes.TheSharedTypingTable`.

**Comparisons (Q-68).** This covers `==`, `!=`, `<`, `<=`, `>` and `>=`.
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

**Division by zero (Q-69).** An integer divided by an integer zero is null, as it was on both engines. A double
operand divides as IEEE does: `+inf`, `-inf`, and NaN for 0/0. Before, C++ raised SIGFPE and ended
the process, and Go gave NaN for all of them. The shared table was `DivisionByZeroCases`; its C++
copy is `ArithmeticTypes.DivisionByZero`.

**Four operations that failed the session give null — decided 2026-10-23 (`rules_benchmark`
`Q-85`).** Each threw or trapped and ended the session; each now gives null, as integer division by
zero does, so the session goes on and the rule's consequent sees no value. Only a session that
failed before computes anything different. Each has its rows in a table of `expr_op_test.cc`:

| Operation | Was | Rows |
|---|---|---|
| the minimum `int32` or `int64` divided by -1, directly or through a mixed integer type (I-126) | SIGFPE, which ended the process | `ArithmeticTypes.DivisionByZero` |
| `+ - * /` on two integer types, the right operand out of the left operand's range: an `int32` plus a large `int64`, a `uint32` plus -1 (I-134) | `boost::numeric_cast` threw | `ArithmeticTypes.TheSharedTypingTable` |
| text plus a named resource, either way round (I-109) | threw in a session's rule | `ArithmeticTypes.TheSharedTypingTable` |
| `lookup` or `multi_lookup` with a null key (I-104) | threw in a session's rule | `Lookups.ANullKeyIsNull` |

`+ - * /` are their `Impl` visitors wrapped in `NullWhenOutOfRange`, which turns a
`bad_numeric_cast` into null (`expr_op_arithmetics.h`); integer division goes through
`integer_quotient`. An operand that fits converts as before. **"In a session's rule" is not a
detail**: with no beta row, as the shared tables call a visitor, an operator's catch-all gives null
for operands it does not take; with a row, it throws. The rows of the last two cases call the
visitor with a row.

**Ties in `min_of`, `max_of` and `sorted_head`, and Go's `min_head_of` and `max_head_of` (I-33, Q-61).**
Among equal values the first met used to win, and the graph's iteration order varies from run to
run (C++'s hash sets by address and seed; Go's maps on every range). So a tie gave a different head
on each run, and a rule set's duplicate removal kept a different record.

A tie is now two values the engine's own `==` calls equal (C++ `EqVisitor`, Go's `EQ`); from Q-68 the
two agreed. On a tie, the object first in this total order wins:
1. by kind: named resource, blank node, literal, null;
2. a resource by its name and a literal by its text (the rendering above), compared as bytes;
3. on equal text, by type, signed before unsigned: int32, int64, uint32, uint64, double, text, date,
   datetime (Go: int, uint, double, text, date, datetime);
4. blank nodes by key, last.

So equal weights give the smallest name, in any insertion order. C++ has `tie_break_less`; Go had
`compareObjects` and `tieEqual` (`expr_operator_math_minmax.go` at `go-engine-final`).
- **Known limits.** A NaN, an infinity, or two values that do not compare are not a tie, and keep the
  first met, which is order-dependent (I-125). An entity named by a UUID still orders differently
  from run to run (I-124).
- **`max_of` and `min_of`** return the winner's value, so they move only between equal values of
  different types: in the direct form, 5 and 5.0 give the int every time.

### Lookups name their table (I-15, I-31, I-104; Go #2330)

Go's lookup subjects were `jets:lookup:<key>`, with no table name. So two tables looked up by one
key shared a subject, and the second lookup read the first one's cached row (I-15). In `jets_ws` the
diagnosis and procedure description lookups crossed this way. They are now C++'s names:
`jets:<T>:<key>` for `lookup`, `multi_lookup` and `lookup_rand`, and `jets:rand:<T>:<n>` for
`multi_lookup_rand`, each cached as `(<T> jets:lookup_row subject)` (Go's `LookupTableSqlite3.lookup`,
`lookup_table_sqlite3.go` at `go-engine-final`).
- **A `multi_lookup` miss** returns null and writes nothing, as it did on both, so a rule testing a
  miss with `(?x p null)` works after it (I-31).
- **An integer key**, signed or unsigned, was converted to text instead of panicking on Go (I-104). A
  null key gave no result on Go; C++ throws. *(Since 2026-10-23 a null key gives null on C++,
  `Q-85`: see the Phase 4 values above.)*
- **`graphdump`** maps the current cache form to the old Go name, so dumps made before and after
  compare equal.

### A filter's re-entry is reconciled before its rows are kept (I-91; C++ #2332, #2335; Go #2330, #2334)

A filter can write to the graph. In a compiled rule set the only write left is the `jets:exception` a
failed truth test raises (the compiler refuses graph-writing operators in filters). The write's
callbacks re-enter the session at once, while a scan is still building candidates. Candidates were
then kept from parent rows that no longer existed, or with filters no longer true, and never
withdrawn, so the result depended on the order the triples arrived in.

The first fix (#2332, #2330) dropped a candidate whose *own* filter's write removed its support. The
review found that check too narrow in three ways, all closed by #2334 and #2335:
1. **A later candidate of a re-entered scan.** An earlier candidate's write removes a later
   candidate's parent row, or the absence a negation tests, just as surely, and the later candidate's
   own filter may write nothing new. In C++ a frozen pending scan could also see the removed row
   twice.
2. **A relation's first scan.** A candidate kept before the write is already inserted into a relation
   no callback reaches yet. The removal's nested visit walks the parent's remaining rows, not the
   removed one, and the filter replay does not run.
3. **A filter whose answer flips.** For example, `exist_not` of the `jets:exception` that it, or a
   later candidate, raises: its answer is out of date as soon as it is given.

C++ also did not count a callback it ignored for a relation not yet activated. Go counted it before
its activation check, and C++ now does too (`count_ignored_reentry`).

Both engines did the same three things from then on, and C++ does them now:
- **A relation's first scan, at a vertex with a filter, holds its candidates.** If the session was
  re-entered during the scan, it drops them and scans again from the parent's rows as they are now,
  then inserts them. The hold is limited to *filtered* first scans: an unfiltered scan writes
  nothing, so it cannot be re-entered, and keeps its single insert.
- **An activated relation inserts each candidate at once.** It first re-evaluates a filter whose
  evaluation re-entered, then re-checks the parent row and triple once the re-entry count has moved
  (Go `stillHolds`, C++ `candidate_still_supported`). A later re-entry still reaches the row: the
  parent row's removal withdraws it, and a graph-reading filter has `TripleUpdatedForFilter` replay
  the relation. Holding here would let the replay rebuild the relation without the held rows.
- **Both retries stop at 16** and raise an error.

Without a re-entry the cost is one comparison per scan and per kept candidate. A filtered first scan
also holds its rows. C++'s `BetaRelation::holds` asks for the same row instance, where Go compared
rows by equality. The engines still converged, because a re-inserted equal row's own pending visit
makes the same candidate.

Tests: `CandidateRowReentryTest` (C++, `jets/rete/rete_session_test.cc`); on Go, `TestRegression_I91_*` in `defects_rbm_i91_test.go` at `go-engine-final`.
End to end, the `jets_ws` rule `[(jets:iState exist_not jets:exception) and ((not ?v) != 1)]` runs
through the rules unit-test harness (`reentry_error_path`). It failed on both engines before these
fixes and passed on both after them (`compare_engines`), and it runs on C++ alone since the
retirement.

### Removing a row from an index removes only that row (I-85; C++ #2332)

When a row left a C++ beta relation, `AQVRemoveIndexBetaRowsVisitor` erased its index entries with
`erase(key)`, which removed **every** row under the key. The rows sharing the key stayed in the
relation but could no longer be joined, so a triple arriving later found nothing to join them to.
The Go engine removed the row alone.

`BetaRowIndex::erase(key, row)` (`jets/rete/beta_row_iterator.h`) now removes that row. The others keep their order (newest first),
and the key goes with its last row. A removal searches the rows under its key; on the benchmark's
workloads that is about one row, and three at most.

### `Retract` deletes, and an erase tells the rete (I-6, I-25, I-103; C++ #2333, both adaptors #2336)

**The adaptor's `Retract` deletes an asserted fact**, however many times it was inserted, and the
rete withdraws what depended on it at the next `ExecuteRules` (`Q-63`).
- **C++** calls the session's erase. That erase removed the triple and told nothing: the notice was
  dropped in 2022 (`2507bf7c`).
- **Go** called `RdfSession.Erase` from #2336. Before, it called `RdfSession.Retract`, the
  inferred-only, one-support retraction the rete uses for consequents, which never removed an
  asserted triple.
- **`RDFGraph::erase` now sends `triple_deleted`** for each triple it removes, as Go's `Erase` did.
  That reaches `RDFSession::erase`, the bridge's `erase` and the native adaptor's `Retract`.
- The engine's own retraction, the session's `retract` (Go's `RdfSession.Retract`), is unchanged.

**One erase stays silent, deliberately (I-103):** `RDFSession::insert`'s move of an inferred copy to
the asserted graph (`erase_without_notice`). The triple never leaves the session, so a notice would
only retract and rebuild rows. Production's one other erase, the pool worker clearing `jets:loop`
and `jets:completed` at a rule set's start (`jetrules_pool_worker.go`), runs before the new rete
session has rows, so its notice changes nothing.

### A live iterator sees the graph as it was when frozen (I-74; C++ #2322)

Since the graph's tables became open-addressing tables (#2322), an insert can invalidate their
iterators, so the graph freezes its live iterators before it changes structure: each copies the
triples it has yet to return and continues over that copy (`freeze`,
`jets/rdf/base_graph_iterator.h`). **A live C++ iterator returns the graph as it was when it was
frozen**, a triple erased afterwards included and nothing inserted afterwards, which is how the Go
engine's cursor behaved: one difference between the engines fewer, recorded here on 2026-10-23.
Before, with node-based tables, such an iteration was undefined. The meta graph, shared across
threads and not changed once rules run, registers no iterator and must not change under one.

### Still different, recorded

Recorded, not changed. Each is an issue in `rules_benchmark`'s registers. Since the Go engine's
retirement each is simply what the C++ engine does; what Go did is kept for the record.

- **A date against a datetime.** C++'s `EqVisitor` can call a date equal to a datetime (the date as
  its midnight). Go never did.
- **`numeric_cast` throws.** C++ mixed-integer arithmetic converts the right operand to the left's
  type with `boost::numeric_cast`, which throws out of range (int32 + a large int64, uint32 + -1).
  Go returned no result. *(Null since 2026-10-23, `Q-85`, I-134.)*
- **`INT_MIN / -1`** traps in C++'s `DivVisitor`, for int32 and int64, and through `numeric_cast`
  in the mixed integer cases (I-126). Go gave the minimum int. *(Null since 2026-10-23, `Q-85`.)*
- **28 near-power-of-ten double pairs.** Go reproduced glibc's `log10` and `pow`, and agreed with the
  compiled C++ comparison on 600,017 of 600,045 tested pairs. The 28 others lie within a few ulps of
  a power of ten, where glibc's `log` and Go's `math.Log` round differently. The Go port mirrored
  x86-64 glibc 2.39; another glibc, or an aarch64 build that fuses into an FMA, may differ again.
- **A negation's free variable (I-115).** Deleting one of several triples that match a negated
  antecedent with a free variable re-satisfies the negation (it did in both engines), although another
  still matches. With `not(?s blocked ?b)` and both `(s blocked b1)` and `(s blocked b2)`,
  retracting `(s blocked b1)` infers the consequent; asserting `(s blocked b2)` alone does not. It is
  an order dependence. Since I-91, a row inserted this way after a re-entry is rejected. The fix
  would change what the engine infers, so it waits for its own change.
- **`ClearPendingRows` during the filter replay (I-106).** A nested visit clears a child's pending
  rows during `TripleUpdatedForFilter`'s replay, so rows the replay inserted and has not yet passed
  on cannot reach that child's children. It predates Phase 4 and was read, not run, on Go; C++'s
  `rete_session.cc` reads as having the same shape, also not run.
- **Text plus a named resource.** C++ has no such case and throws (or gives null in a filter); Go
  concatenated the name. *(Null since 2026-10-23, `Q-85`, I-109; the name is not concatenated.)*
- **`join_values` over a blank node** writes `bn(<k>)` on C++; Go wrote `BN<k>`.
- **Regex dialects.** C++'s `literal_regex` compiles ECMAScript, so a lookahead or a backreference
  compiles; Go's compiled RE2 and panicked on them.
- **`lookup` and `multi_lookup` on one table and key share a subject and a cache triple (I-105)**, as
  they did on both engines. Whichever runs first in a session decides what the other returns. None of the
  four workspaces calls both on one table (counted 2026-10-22).

## Every rule fires at the salience it declares — 2026-10-11

**A vertex has one salience, and rules with the same antecedents used to share their last vertex.**
So a vertex could carry rules with different saliences, and the two engines settled it differently:
- **Go** took the lowest (`slices.Min`, in its `NewReteMetaStoreFactory`, over the list in `.rete.json`).
- **C++** took, and would still take, the first rule's: it reads one number, `workspace.db`'s `rete_nodes.salience`, and
  the compiler wrote `Salience[0]` there.

Either way, **a declared salience could be overruled with no error.** Raising a rule's salience did
nothing if a rule with the same antecedents sat lower. Lowering an aggregate below its producer
dragged the producer down with it when the two shared antecedents. The compiler's aggregate salience
check compared declared saliences, so it missed both.

Measured on a three-rule case in the rules unit test harness: a producer declared at s=100 sharing
its antecedents with an s=5 rule, feeding an aggregate at s=50. It compiled. C++ ran the vertex at
100 and the aggregate counted the producer's value; Go ran it at 5 and counted nothing
(`jetstore_agentic_ai` `rules_unit_test_harness` `I-25`).

**Fixed in the compiler's network builder** (`terminalReteNode`,
`jets/compilerv2/compiler/jet_rule_listener_rete.go`): rules share their last vertex only when their
saliences are equal, and a rule with another salience gets a vertex of its own; the antecedents
before it are still shared. So every vertex fires at one salience, the declared one, on both
engines, and the aggregate check's declared saliences are the ones that run. The same three-rule
case then counted the value on both engines.

**One way to defeat a declared salience remains, and the compiler warns about it**
(`WarnSameConsequentAtTwoSaliences`, same file). Two rules with the same antecedents and different
saliences that infer the same triple: the triple exists from when the higher one fires, and the
lower one only adds a reference, so its salience changes nothing about that triple. That is what is
left when a rule was copied to change its salience and the original kept. It is a warning, not an
error, because the pair can differ in other consequents. ~~A warning reaches the process log and
not the compile output a workspace compile returns on success.~~ **Since 2026-10-12 a successful
compile's output carries its warnings** under *Compilation warnings:* (`compileWarnings`,
`jets/workspace/compile_workspace_v2.go`), so View Last Log in the Workspace IDE shows them; until
then only the warnings the compiler also writes to the parse log reached it.

**Nothing in production moved**, recompiled 2026-10-11 against the previous compiler:
- **The three client workspaces** at the pins `jetstore_agentic_ai` records compile to identical
  networks, node for node (11,363 rete nodes in the largest).
- **The clients' own copies of two of them** have no vertex carrying two saliences.
- **`jets_ws` gains one vertex**: the generated class-inheritance rule
  `ci_cintel:Briefing_Pharmacy_Event` stops sharing `BP_RxFillDates10`'s vertex and fires at its own
  s=100 instead of 10. It infers only `rdf:type owl:Thing`, which no rule reads.

## Text read as a bool: one specification on both engines — 2026-10-11

**Until 2026-10-11 the two engines read a string as a bool the opposite way round.** C++
(`rdf::to_bool`) took any text but a few spellings of false as true; Go (`rdf.ParseBool` for
literals, `Node.Bool` for a filter) took any text but a few spellings of true as false, and
`Node.Bool` called `"false"` true. Rule configuration read the first letter, so `tomato` was true.
None of the four workspaces reached the difference on that day: no rule file wrote `bool("…")` or
`bool x = "…";`, and no filter tested a bare variable. This is `rules_unit_test_harness` `I-45`.

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

## A minting consequent is retracted by replay; two retraction defects are kept — 2026-10-08, revised 2026-10-09

Both engines. **Retraction recomputes a beta row's consequents** — it evaluates them again
against the withdrawn row and retracts whatever comes out (`ReteSession::compute_consequent_triples`,
`ReteSession::compute_consequent_triples`, `jets/rete/rete_session.cc:409`; Go's `ComputeConsequentTriples`, `rete_session_exec_rules.go:212` at `go-engine-final`). Found
by a client workspace's assessment in `jetstore_agentic_ai` (its Q-37 and R-13), where one rule's
`create_entity` was keyed as a workaround, in that workspace's own repository.

### What is fixed: a consequent that mints a resource

- **`create_entity 0`** minted a second UUID on retraction, inserted a stray `jets:key` triple for
  it, and retracted a link to it that did not exist, so the real link stayed.
- **a keyed `create_entity`** re-inserted `jets:key` on every retraction, so its reference count
  never reached zero.

At rule load a node vertex is flagged when one of its consequent terms mints a resource:
`create_entity`, `create_uuid_resource`, `range`, `lookup_rand` or `multi_lookup_rand`. C++: the
`MintsResource` marker on those visitors (`jets/rete/expr.h:147`) and `ExprBase::mints_resource`,
read by `ReteMetaStore::initialize` (`jets/rete/rete_meta_store.h:201`). Go had `expr_mints_resource.go` at `go-engine-final`
and `AddConsequentTerm` (`node_vertex.go:77` at `go-engine-final`). A row of a flagged vertex records every triple its
consequents raised the reference count of — the consequent triple and `create_entity`'s `jets:key`,
both inserted through `insert_inferred_for` / `InsertInferredFor` (`jets/rete/rete_session.h:359`,
`rete_session.go:34` at `go-engine-final`) — and **retraction retracts that record and evaluates nothing**
(`beta_row->is_recording()` in `compute_consequent_triples`, `rete_session.cc:500`; `rete_session_exec_rules.go:270` at `go-engine-final`). The record is emptied rather than
released, because a withdrawn row stays in its relation (below) and is retracted again on later
withdrawals; released, the second retraction fell back to recomputing `create_entity` and
re-inserted `jets:key`. **A keyed entity's `jets:key` goes with its last support** — decided by
Michel on 2026-10-08 ("retract the keyed entity"). Lookup cache rows and `range` values are not
recorded and still outlive the row.

### What is kept: two defects, and what depends on them

**The 2026-10-08 version of this entry fixed two more things, and both were reverted on 2026-10-09
by Michel's decision** ("go with (a), the narrow fix"), because with them a client workspace's rules never
terminate with `_0:no_truth_main_on_exist` off. *(That reason is gone since 2026-10-11: see the
correction of 2026-10-23 at the end of this entry. The defects are still what production runs.)* They are kept as **known defects**, pinned by the
`LEGACY` cases in `jets/rete/retract_recorded_consequents_test.cc` (and, until the retirement, in Go's
`retract_recorded_consequents_test.go` at `go-engine-final`):

1. **A consequent that reads the graph is recomputed on retraction** — the aggregates, `exist`,
   `size_of`, `lookup`. Once its input has moved, the triple retracted is not the triple inferred,
   and the inferred one stays.
2. **A withdrawn row never fires again.** After retracting a row's consequents the engine calls
   `remove_beta_row` (`rete_session.cc:533`; Go's `RemoveBetaRow`, `rete_session_exec_rules.go:305` at `go-engine-final`), which
   returns at once for a row marked `kDeleted`, which that row is. It stays in its relation as
   `kProcessed`, and an equal row arriving later is taken for one already inferred.

**Measured on the assessment's local harness, flag off, against the engine at `45dd7851`:**

- **The loop rules depend on both together.** The workspace's three claim processes each have a
  set of loop rules of one shape: an unmerged match is on the priority list, a loop rule guarded
  by `[exist priorityList]` infers `topMatch` by `sorted_head`, and `topMatch` puts the match on
  `mergeList`. That is an **odd loop through
  negation, with no fixed point under correct truth maintenance**. Replaying the `sorted_head`
  consequent retracts `topMatch` when the list empties, which un-merges, which restores the list,
  which re-admits the loop rule — and with defect 2 fixed, it fires again. Consequent visits went
  from tens to millions; one process's test run never finished, and another's members ended at the
  vertex visit limit.
  Recomputing gave null over the empty list and retracted nothing, and the stale row never
  re-fired, so the loop ended.
  `LoopThroughNegationTest` (`jets/rete/loop_through_negation_test.cc`), and on Go
  `TestLoopControlRulesTerminate` (`loop_through_negation_test.go` at `go-engine-final`), reproduce it in five rule terms
  and guard that it still terminates.
- **A self-guarded rule depends on defect 2 alone**, with pure consequents. Its guard is two
  `exist_not` tests on properties that its own chain of five rules goes on to infer, so the guard is
  falsified by that chain, the row is withdrawn and its consequents retracted, and the guard is true
  again. With defect 2 fixed it repeats until the vertex visit limit, a million times each on two
  vertexes of its rule set; on the pin it stops after one swing.
- **Fixing defect 1 alone** terminates but undoes every merge: all four members of each of two
  processes' test claims changed.
- **With the flag on**, as the client runs in production, no `exist` callback is registered, none
  of this happens, and the full fix gave the same member outcomes as the pin.

**The rule language cannot end these loops by itself**: a rule has the properties `o`, `s` and
`flag`, and nothing that makes a consequent non-retractable. *(Wrong, corrected 2026-10-11 below:
the workspace ended them in the rule language, by changing what the rules are guarded on.)* So if a
workspace ever needs correct truth maintenance on these shapes, the paths are:

- **(b)** put aggregate replay and row erasure behind an opt-in rule-config switch, default off —
  two code paths, no workspace changing behaviour unless it asks; or
- **(c)** fix both outright and add an engine primitive making a loop rule's consequents
  non-retractable, then rewrite the workspace's loop rules and its `exist_not` self-guards before running it
  with the flag off.

**2026-10-11: option (b) is built, for rules unit tests only, and no pipeline can reach it.**
`jetstore_agentic_ai`'s `rules_unit_test_harness` project (Phase 1, track `AF`) restored
`1d993844`'s two fixes behind a **retraction mode fixed when the network is built**, one switch
per defect, because each was measured above to do something different alone. Go had
`RetractionMode` (`retraction_mode.go`), passed as `WithRetractionMode` to
`NewReteMetaStoreFactory`; `AddConsequentTerm` recorded a graph-reading consequent
(`expr_reads_graph.go` at `go-engine-final`, the wide half of `1d993844`'s row purity) only with
`ReplayGraphReadingConsequents`, and `ComputeConsequentTriples` called `EraseRetractedBetaRow`
instead of `RemoveBetaRow` only with `EraseWithdrawnRows`. C++: `RetractionMode`
(`jets/rete/rete_meta_store.h`), passed to `ReteMetaStore::initialize` through
`ReteMetaStoreFactory::load_database` from the C entry point `create_jetstore_hdl_with_mode`;
`create_jetstore_hdl` is unchanged and builds the legacy mode. **The zero value is legacy for
both, in both engines** (the Go struct `rete.RetractionMode` stays, without the factory option:
it carries the mode to the bridge), and nothing a pipeline configures sets anything else: there is no
`rule_config` key and no `JetrulesSpec` field for it. The mode is set by a unit test definition
(its `legacy` object), which the harness reads before it builds the test's engine, so each test
runs in its own process in one mode. Production runs exactly what the decision of 2026-10-09
left it running. The `LEGACY` tests in both engines ran every case in the four modes, and C++'s still do:
`legacy` pins today's behaviour, and `fixed-1`, `fixed-2` and `fixed` assert what `1d993844`
asserted. On the five-term loop shape they reproduced the measurement above on both engines:
`fixed` ends at the vertex visit limit, `fixed-1` terminates with both merges undone, and
`fixed-2` terminates as `legacy` does.

**2026-10-11, later: the workspace depends on neither defect any more, nor on the switch.** Two
corrections to what is above, from `jetstore_agentic_ai`'s `rules_unit_test_harness` (its `I-38` to
`I-40` and `I-44`), which reduced these shapes to unit tests:

- **What held the shapes up in production was the switch, not the defects.** The measurements
  above were taken with `_0:no_truth_main_on_exist` off. With it on, as the client ran until 2026-10-11, an `exist` filter
  is evaluated once, when its row is made, and the compiler puts the loop rule's filter on the
  loop-0 `jets:State` row. So the guard never moved. With the switch on, the workspace at the time passed
  with both defects off as well. The defects held the shape up only with the switch off, which is
  what this entry measured. They do so in `jets_ws`'s `looping_test1`, which has no switch.
- **The rule language could end them; what was missing was the right guard, not an engine
  primitive.** A change in the workspace's own repository rewrote them without (c)'s
  non-retractable consequent:
  - Each process's loop rule is guarded by `size_of` on the match list, which no merge changes,
    instead of `exist` on the priority list, which every merge shrinks.
  - The loop completes on an empty priority list. `size_of` is maintained in every mode.
  - Two processes' adjustment loops run in their own rule sets, each with its own loop counter.
  - The self-guarded rules' chain now ends in an `_0:` property, which a rule of the next tier
    copies to the property the guard tested, where nothing tests for its absence.
  - Two exclusion rules test their own `_0:` flag instead of the property their own result
    reached.

  A second change then stopped setting the switch. The workspace's three loop unit tests pass in the
  legacy mode and with both defects off (`fixed`). With the switch off, two processes give the
  switch-on export on the assessment's 28 test claims.

**So the case for keeping the defects now rests on the client's own copy of the workspace**, which
still sets the switch and has neither change; it must take both together, because the switch removed
without the first change's rules brings the hangs back. Nothing in the workspaces `jetstore_agentic_ai` pins depends
on them except `jets_ws`'s `looping_test1`, which pins them on purpose, and the `LEGACY` tests.
Whether pipelines should run with them fixed is a decision this entry does not make.

**Corrected 2026-10-23: the client workspace that relied on them no longer does.** The paragraph
above was right on 2026-10-11 and stopped being right the same day, and it stood until Michel
pointed at the release notes. The client workspace that relied on the defects took both changes
together on 2026-10-11, its rewritten loops and the switch no longer set, and they were released
in Release 4.0.3. Its strict unit tests, which run with both defects off, pass with the legacy
tests' expectations. So no workspace depends on the defects or on the switch. **Production still
runs the legacy mode**: no pipeline can select another, and `create_jetstore_hdl` builds legacy.
Whether production moves to the fixed mode is measured first, legacy against fixed on the
benchmark's inputs and the workspaces' unit tests (`rules_benchmark` `Q-92`), and is Michel's
decision after that.

**Two divergences found on the way, neither fixed here.** C++ `RDFGraph::erase` notified no graph
callback (`erase_internal`, `jets/rdf/rdf_graph.h:302` today), where Go's `RdfGraph.erase_internal` did —
so erasing an asserted triple drove truth maintenance in Go and not in C++, which is why the cycle
tests drive retraction with `insert_inferred` and `retract` instead. **Settled 2026-10-22
(rules_benchmark I-6, I-103; jetstore#2333):** C++'s erase now tells the rete, as Go's did; only
`RDFSession::insert`'s move of an inferred copy stays silent. See the Phase 4 entry above. ~~And Go `RdfGraph.Retract`
erases the triple from its `pos` and `osp` indexes even when the `spo` reference count is still
above zero, so a triple with two supports that loses one should stay `Contains`-true and stop
being found by predicate or object, read from the code, not measured.~~ **Fixed 2026-10-21
(rules_benchmark I-7)**: Go's `RdfGraph.Retract` retracted all three indexes from then on, as C++
does, and `TestRetractKeepsATripleWithASupportLeftInEveryIndex` (`rdf_graph_test.go` at `go-engine-final`) held it.

## `_0:no_truth_main_on_exist` — a production flag, and where its test is — 2026-09-09

C++ only; the Go engine had no counterpart, so a rule set that needed it could not run on the Go engine at all.
A client workspace set it in its pipeline configuration (until 2026-10-11; see the entry above; the
client's own copy still does) because its rules predate
rule filters participating in truth maintenance, and it makes `ExistVisitor::register_callback` and
`ExistNotVisitor::register_callback` (`jets/rete/expr_op_resources.h`) register nothing.

**The two directions fail differently, which is worth knowing before reading a wrong result.** With
the flag on, an `exist` filter that becomes true is never noticed, and an `exist_not` filter that
becomes false keeps whatever it inferred — the second is a triple that should have been retracted and
was not. `NoTruthMainOnExistTest` (`jets/rete/no_truth_main_on_exist_test.cc`) pins both, and the
default: **absent means truth maintenance ON**, and so does a value that is present and is not an
int32, which is how a workspace writing `"1"` as text gets the opposite of what it asked for with
nothing logged.

## The Go and C++ engines are independently maintained and they disagree — 2026-09-09

*History: written while both engines ran. The C++ facts in it stand; the Go side was settled by the
Phase 4 entry above or ended with the retirement.*

Found while adding `join_values` (`JoinValuesOp`, `expr_operator_math_join_values.go` at `go-engine-final`; the C++ half
is `JoinValuesVisitor`, `jets/rete/expr_op_arithmetics.h`). Every operator here has a C++ twin and
nothing checks that the two behave alike, so **a rule can compile against one engine and not the
other, or — worse — run on both and produce different values**. Three divergences were measured on
`sum_values` while writing its sibling. ~~None is fixed here; they~~ **They** are recorded so the
next operator author does not rediscover them.

**A fourth entry was added later the same day**, by the session that fixed 2, and it is a different
animal: **item 4 was not a divergence when it was found** — both engines had it — and it became one
because only C++ was fixed. It is filed here anyway because this is where a reader looking for
"why is this aggregate wrong" will be standing.

**Items 2 and 4 were fixed in the C++ engine on 2026-09-09**, and each entry says so in place
rather than being struck out, because what a fix leaves behind matters: 4 is fixed on one side
only, and both are reachable by far less of the rule corpus than they look — see the paragraph
after 4, which is the one to read first if you arrived here from a wrong aggregate in production.

**1. A config carrying `jets:value_property` alone works in C++ and silently yielded nothing in Go.**
`SumValuesVisitor` resolves `datap = (rhs, jets:value_property, ?)` and falls back to `rhs` itself
(`jets/rete/expr_op_arithmetics.h`); `SumValuesOp.InitializeOperator`
(`expr_operator_math_sum_values.go:20` at `go-engine-final`) only looked at `jets:value_property` when
`jets:entity_property` was present, and otherwise used `rhs` as the data property — so the config
resource was walked as if it were a property of the subject, matching nothing. The C++
`expr_op_specialty_test.cc` `SumValuesVisitor1` case is exactly this form, and it had no Go
counterpart. `join_values` accepted the form on **both** sides, deliberately.

**2. ~~C++ registers no truth-maintenance callback for the direct-property form.~~ FIXED in C++
2026-09-09.** `SumValuesVisitor::register_callback` returned 0 when the rhs carried no
`jets:value_property`, while `SumValuesOp.RegisterCallback` fell back to watching `objProperty`. So
`(?s sum_values someMultiValuedProperty)` was recomputed on change under the Go engine and was not
under the C++ one. `join_values` registered in both cases on both sides. The C++ side now goes
through `register_callbacks_for_aggregate` (`jets/rete/expr_op_arithmetics.h`), which carries that
fallback for every operator that accepts the form. `SumValuesRecomputesOnDirectMultiValuedProperty`
in `jets/rete/expr_op_specialty_test.cc` fails without it.

**3. ~~A double *literal* is quantised to 15 bits of mantissa by the Go engine and not by the C++ one.~~
SETTLED 2026-10-22 (rules_benchmark I-40, jetstore#2331): `NewDoubleLiteral` stores the double given,
on both engines; see the Phase 4 entry above.** As it was:
`ResourceManager.NewDoubleLiteral` (`jets/jetrules/rdf/resource_manager.go:287` at `go-engine-final`) does
`big.NewFloat(x).SetPrec(15).Float64()` — 15 **bits**, not digits — so `0.891089` is stored as
`0.891082763671875`. Computed doubles do not take that path (`rdf.F`, `jets/jetrules/rdf/ast.go:372`,
stores the value as given), so the same number reaches the graph differently depending on whether a
rule wrote it down or worked it out. Measured 2026-09-09 by a rendering test that expected the two
engines to agree on `0.891089` and got `0.891083` from Go.

**4. No aggregate operator watched `jets:entity_property`, on either side — FIXED in C++
2026-09-09, and open in Go until its retirement.** An aggregate configured with `jets:entity_property` **and** `jets:value_property` walks
`(s, entityP, ?o).(?o, valueP, ?v)` and therefore depends on both properties, and every one of the
five operators registered its callback on the value property alone — `sum_values`, `min_of`,
`max_of`, `sorted_head` and `join_values`, in both engines. **Only a change to an already-linked
child was ever seen.** Linking a further child does not touch the value property, so an entity
materialised with its value and then attached — which is the ordinary order for an entity built by
rules — moves the aggregate without anything noticing. The C++ side now registers on both
(`register_callbacks_for_aggregate`, `jets/rete/expr_op_arithmetics.h`); Go's `MinMaxOp`, `SumValuesOp`
and `JoinValuesOp` never did.

**And a fifth thing, which is not a divergence and is the one that actually bites.** All of the
above is about `register_callback`, and **`register_callback` is never called for a consequent
expression.** `ReteSession::set_graph_callbacks` (`jets/rete/rete_session.cc`) registers a node
vertex's *antecedent* alpha node and its *filter* expression, and nothing else; Go's `ReteSession`
initialisation in `rete_session.go` at `go-engine-final` did the same and called `InitializeExpression` alone on a
consequent. So **an aggregate in a consequent has no truth maintenance** (it had none in either engine), whatever
its config, and the four fixes above cannot reach one.

That is not a hypothetical shape. **All 89 uses of these five operators across the four workspaces
under `workspaces/` are in consequents; none is in a filter** (counted 2026-09-09). The visible case
is `CE_RxDateRange10` (`workspaces/jets_ws/jet_rules/clinical_intel/common_events.jr`), whose
antecedent `(?entity tag hasPharmacyClaim)` is a *constant* triple that `AM_PCreateEvent40`
(`analysis_pharmacy_rules.jr`) asserts alongside the first claim it links. The rule term is created
once, at that first claim, and its `min_of`/`max_of`/`sum_values` consequents are computed over
whatever is linked when the row is drained from the consequent queue — which for equal salience is
heap order. A three-fill event aggregating exactly one claim is that, not any of the four
divergences above. `AggregateInConsequentIsNotMaintained` in
`jets/rete/expr_op_specialty_test.cc` pins the behaviour so that a reader who fixes
`register_callback` and expects that rule to change is told otherwise by a test rather than by a
production number.

**The consequence for anyone adding an operator, while there were two engines: write the tests in
pairs.** `expr_operator_math_join_values_test.go` at `go-engine-final` and the `JoinValues*` cases in
`jets/rete/expr_op_specialty_test.cc` were deliberately the same cases in the same order, and the
rendering test in the Go file existed only to pin agreement with the C++ `JoinTextVisitor`. The C++
cases are now `join_values`' specification. Go's `Node.String()` was **not** that rendering: it formatted an `LDate` with `%v` on the struct
(`{2025-06-15 00:00:00 +0000 UTC}`), where C++ emits ISO-8601. **Since 2026-10-22 (rules_benchmark
I-16, I-32; jetstore#2331, #2333)** every operator that writes a value into text, `join_values`
included, shared one rendering on both engines (`rdf.TextOf` in Go), and a double in `join_values`
is no longer `%g` with six digits but the shortest round-trip digits. See the Phase 4 entry above.

**And write a rule as well as a pair of unit tests, because a visitor test cannot see the name.**
`join_values` shipped with matched unit tests on both engines and no `.jr` file anywhere used it, so
nothing exercised compile → metastore → execution — the leg on which an operator name is an opaque
string. `jets/jetrules/test_ws/jet_rules/test_join_values_main.jr` is that rule; it was run on Go by
`TestJoinValuesCompilesAndRuns` (`join_values_rule_test.go` at `go-engine-final`), which compiled a scratch copy of
`test_ws` with `workspace.CompileWorkspace`, asset validation skipped, and asserted the joined strings. **Two of the three config forms
are in it deliberately**: the `jets:entity_property` + `jets:value_property` form with an explicit
`jets:separator`, and the `jets:value_property`-alone form with the default — which is item 1 above,
the form `sum_values` got wrong in Go, so the rule is what said `join_values` did not.

## Operator names are not registered anywhere but the two factories — 2026-09-09

**Superseded 2026-10-22, with the Go engine's retirement.** The compiler now refuses an operator
the C++ engine does not register: the names are in `jets/compilerv2/compiler/operators.txt`, and
`jets/rete/operator_list_test.cc` checks them against `expr_operator_factory.h` in both directions.
The Go tests this entry names were deleted with the Go engine; `corpus_operators.txt` and
`OperatorFactoryTest.CorpusOperatorNames` stay, the manifest as last collected. The six names the
table below lists as one-sided are now a compile error, or, for `raise_exception`, an operator
like any other. The entry is kept as written.

Searched while adding `join_values`. A binary operator name reaches the engine as an opaque string:
the ANTLR lexer has no token for `sum_values` or any of its siblings
(`jets/compilerv2/parser/JetRuleLexer.tokens` carries only punctuation and structural keywords), the
compiler carries the name through as `ExpressionNode.Op` and writes it to `workspace.db` unexamined
(`workspace_db_helper.go`), and no validator whitelists it. So **adding an operator is exactly two
registrations** — a `case` in `CreateBinaryOperator` (`expr_operator_factory.go` at `go-engine-final`) and a line in
`create_binary_expr` (`jets/rete/expr_operator_factory.h`) — plus the implementation.

The cost of that design is that **an unknown operator is not a compile error**. It compiles, and
fails at rule-execution time: `create_binary_expr` throws, `CreateBinaryOperator` returns nil.

**That is now checked at test time instead, and the check is in two halves because no one process can
do both.** `operator_registration_test.go` at `go-engine-final` in this package compiles every rule set of every
workspace — 50 of them, from each `workspace_control.json`'s `rule_sets` rather than from a
`*_main.jr` glob, which finds 2 of the largest workspace's 34 — collects every `ExpressionNode.Op` and asserts each
one resolves in `CreateBinaryOperator` / `CreateUnaryOperator`. It writes the names it found to
`jets/rete/test_data/corpus_operators.txt`, and `OperatorFactoryTest.CorpusOperatorNames`
(`jets/rete/expr_operator_factory_test.cc`) asserts the same list against `create_binary_expr` /
`create_unary_expr` — **the C++ engine cannot compile a `.jr` file, so it cannot collect the names
for itself**, and the manifest is the only thing keeping the two halves looking at the same list. It
is checked in and the Go test fails when it drifts.

**And the *two registrations* claim is itself now a test, which is how the next entry got measured.**
`TestBothFactoriesRegisterTheSameOperators` reads both factories as text — `case "name":` and
`if(op == "name")`, C++ comments stripped so the two commented-out `to_type_of` / `cast_to` lines do
not count — and fails when a name is in one and not the other.

**Six names are, and none of them is in the corpus, which is the only reason nobody has paid for
it.** Measured 2026-09-09 and listed in `knownFactoryDivergences` so a seventh fails the test rather
than joining them:

| Name | | |
|---|---|---|
| `apply_regex` | binary | Go only, an alias for `literal_regex`; C++ has `literal_regex` alone. Since 2026-10-22 (rules_benchmark I-12, jetstore#2331) both names mean what C++'s does: capture group 1, or null |
| `min_head_of`, `max_head_of` | binary | Go only, `NewMinMaxOp(_, true)`; C++ has no head form |
| `to_date`, `to_datetime` | unary | Go only; C++ has `to_timestamp` and neither of these |
| `raise_exception` | unary | C++ only, `RaiseExceptionVisitor`; the Go engine has no counterpart |

**Read that as five ways to write a rule that runs on the Go engine and throws on the deployed one**,
and one the other way round. `cpipes_native_server` is what ships (`Dockerfile.cpipes:46` on 2026-09-09; the `COPY` of `cpipes_native_server` is at `:73` today) and its
default factory is `jetrules_native_adaptor`, so the Go-only column is the dangerous one — a rule
using `to_date` passes every test a developer runs with `use_jet_rules_go` and fails in production.
