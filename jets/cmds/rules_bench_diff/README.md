# `rules_bench_diff` — compare two trees of graph dumps

The equivalence check of `projects/rules_benchmark` (§5.7 of its plan, task AB.5). The engine writes
a **canonical dump** of the RDF session after each rule set of each member
(`jets/rulesbench/graphdump`; the format is in its package documentation). This command compares two
trees of them, file by file, and **classifies each difference** against a list of known differences.
It compares two runs: the engine before and after a change, or its run 1 against its run 2, which is
the noise floor to measure first. Until the Go engine was retired (October 2026) it also compared one
engine against the other, and trees and lists written then name `go` and `native`.

It links no engine and opens no workspace.

## Command line

```
rules_bench_diff -a <dir>/<engine> -b <dir>/<engine> [-known list.json] [-json out.json] [-text out.txt]
```

| | |
|---|---|
| `-a`, `-b` | the two trees. Each holds `<member>/<NN>_<ruleset>.tsv`, so a run's tree `<dir>/<engine>/…` is passed as `<dir>/native` (`<dir>/go` in a tree written before the retirement; `graphdump.Path` writes that layout) |
| `-a-label`, `-b-label` | the engine labels, which entries name in `only_on`. Default: the base name of `-a` and `-b` |
| `-scope` | `cross` or `self`. Default: `self` when the two labels are equal, `cross` otherwise |
| `-known` | the known-difference list. Default: `known_differences.json`, built into the binary |
| `-json` | the summary as JSON |
| `-text` | the summary as text. Default: standard output |
| `-all` | list classified lines too, not only their counts |
| `-max-lines` | unclassified lines shown per file in the text summary (20); 0 for all |

**Exit status**: `0` when every difference is classified and both trees have the same files; `1`
when a difference is unclassified or a file is in one tree only; `2` on a usage or read error.

```bash
rules_bench_diff -a base/r01/native -b head/r01/native -json change.json   # before and after a change
rules_bench_diff -a out/r01/native -b out/r02/native -scope self           # run against run
```

## The summary

Per file both trees have: the lines only in A and only in B, **grouped by predicate**, each with the
entry that classified it. Classified lines are counted per entry and predicate; unclassified lines are
always listed in full. Files in one tree only are listed, and so are the totals per entry.

## The known-difference list

A JSON file of entries. **A difference is unclassified unless an entry matches it.** Each entry has an
`id`, `refs` (where the difference is recorded: a section of `jets/rete/README.md` or a
register entry of `projects/rules_benchmark`) and a `description`, and then:

| Field | |
|---|---|
| `scope` | `cross` (the default), `self` or `any` |
| `only_on` | the label of the side the line is on, such as `native` (`go` in a list written before the retirement) |
| `subject`, `predicate`, `object` | regular expressions over the term as written in the dump, anchored at both ends |
| `pair` | a pair kind, below. The entry then matches lines on both sides |
| `quanta` | `double_near`'s tolerance, in quanta of the 15-bit grid the Go engine quantised literals to (`graphdump.Quantum`) |
| `member`, `file` | regular expressions over the dump's member and file name (`<member>/<file>` in the tree), anchored at both ends: the entry applies only there. For a difference known in one member of one workload. They narrow an entry and do not count as the field that keeps it from matching every line |

An entry without `pair` matches one line. An entry with no `pair`, `subject`, `predicate` or `object`
is refused, since it would match every line. **A field a pair kind does not read is refused too**, as
is an unknown field, so an entry cannot look narrower than it is. The last column below says which
fields each kind reads.

**The pair kinds**, tried in this order whatever their order in the list:

| Kind | Matches | Reads |
|---|---|---|
| `relabelled` | the same line up to the labels of minted nodes, under a one-to-one correspondence of labels. The correspondence comes from the line shapes (a line with its labels blanked) that occur once on each side. The node exists on both sides and was renamed because its neighbourhood differs; that difference is in its own lines, which are classified on their own | nothing |
| `merged` | the lines of a node on the `only_on` side that corresponds to several nodes of the other side, each of which corresponds to it alone: one node where the other engine has several. **The node must be the object of a line of its side whose subject and predicate match the entry's** (the anchor) | `subject`, `predicate` (both required), `only_on` |
| `double_quantised` | the same subject and predicate, two doubles equal once both are quantised as the Go engine quantised a literal (`graphdump.Quantise`) | `subject`, `predicate` |
| `double_near` | the same subject and predicate, two doubles at most `quanta` quanta apart at the larger magnitude | `subject`, `predicate` |
| `text_double_format` | the same subject and predicate, two texts equal between their numbers, each differing pair of numbers being one in `std::to_string`'s six-decimal form and one not, equal once quantised. Integers that differ never match | `subject`, `predicate` |
| `literal_regex_group` | the same subject and predicate, two texts; the `only_on` side's (the whole match) matches `object`, is longer, and contains the other's (the capture group) | `subject`, `predicate` (required), `object`, `only_on` (required) |
| `predicate_alias` | the same subject and object, two different predicates that both match `predicate` | `subject`, `predicate` (required) |

Then the single-line entries, in list order.

### The entries, and what they cover

**The built-in list holds only differences of the engine itself**, which any workload can show; until the
retirement they also covered the two engines against each other:

| Entry | Record |
|---|---|
| `minted-node-relabelled` | `graphdump` package documentation |
| `double-summation-last-bits` | `projects/rules_benchmark` `I-33`, `F212`, `F217`: an aggregate summed in another order, across engines or between runs; about 3e-12 relative |

**Three entries left the list in `projects/rules_benchmark` Phase 4** (2026-10-22), when the differences they
classified were fixed: `go-double-literal-quantised` (`I-40`, Go no longer quantises a double literal),
`go-lookup-subject-shared-across-tables` (`I-15`, Go names a lookup subject per table, as C++ does) and
`lookup-cache-predicate` (`I-15`, both engines cache a multi-lookup with `jets:lookup_row`). Measured on the
benchmark's nine inputs, Go and C++ then differ only in last-bit sums. The tests keep the three as fixtures
of their pair kinds. **Comparing a dump made before Phase 4 with one made after** needs them back: pass the
list as it was at `b3b348f1` with `-known`.

**An entry that names a workload's predicates, rules, members or files belongs in a list of its own,
passed with `-known`**, not in the built-in one: the built-in list is part of this repository, and a
workload's vocabulary is not. Such a list is the built-in one plus the entries for that workload, and
is kept with that workload's data. Several engine differences are classified that way, because only
the predicates a workload computes them on narrow them enough to be safe: a double computed from
quantised literals within a tolerance, text + double written with six decimals on C++, and
`literal_regex`'s two meanings (`projects/rules_benchmark` `I-12`, `I-16`).

**Money computed from quantised literals has no entry, deliberately**, in either kind of list (and from Phase 4
Go computed from exact literals, so there was none to make). Amounts
a workload computes from quantised literals differ from C++ by 0.5 to 1.04 quanta, a cent or two. A
tolerance would hide a wrong amount of the same size, so these stay unclassified.

**The README's other differences have no entry**, because nothing in a dump identifies them by its
shape: `jets:value_property` alone (item 1), `jets:entity_property` not watched in Go (item 4), the
callbacks of C++ `erase`, Go `Retract` and its indexes (until `I-7` was fixed), and
`_0:no_truth_main_on_exist`. Each shows as
a different value or a missing triple, unclassified. When one appears, an entry is written for the
predicates it reaches, and the issue is registered.

## Producing the dumps

`rules_bench -dump_dir` writes them after each rule set of each repetition (`jets/cmds/rules_bench/README.md`).
For a rules unit test, `TestDumpRulesUnitTest` in `jets/rulesbench/graphdump` does it, on the C++
engine, through `RunRuleSets`:

```bash
LIB=<a build of libjets.so of this commit>/jets
CGO_CFLAGS="-I$PWD/jets" CGO_LDFLAGS="-L$LIB" go test -c -o graphdump.test ./jets/rulesbench/graphdump
LD_LIBRARY_PATH=$LIB WORKSPACES_HOME=<home> WORKSPACE=my_ws \
  GRAPHDUMP_TST=<home>/my_ws/jet_rules/unit_tests/my_test/my_test.tst.json \
  GRAPHDUMP_OUT=out/run1 ./graphdump.test -test.run '^TestDumpRulesUnitTest$'
```
