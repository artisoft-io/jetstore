# `rules_bench_diff` — compare two trees of graph dumps

The equivalence check of `projects/rules_benchmark` (§5.7 of its plan, task AB.5). Each engine writes
a **canonical dump** of the RDF session after each rule set of each member
(`jets/rulesbench/graphdump`; the format is in its package documentation). This command compares two
trees of them, file by file, and **classifies each difference** against a list of known differences.
It compares one engine against another, and one engine's run 1 against its run 2, which is the noise
floor to measure first.

It links no engine and opens no workspace.

## Command line

```
rules_bench_diff -a <dir>/<engine> -b <dir>/<engine> [-known list.json] [-json out.json] [-text out.txt]
```

| | |
|---|---|
| `-a`, `-b` | the two trees. Each holds `<member>/<NN>_<ruleset>.tsv`, so a run's tree `<dir>/<engine>/…` is passed as `<dir>/go` or `<dir>/native` (`graphdump.Path` writes that layout) |
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
rules_bench_diff -a out/run1/go -b out/run1/native -json cross.json   # engine against engine
rules_bench_diff -a out/run1/go -b out/run2/go                         # Go against itself
```

## The summary

Per file both trees have: the lines only in A and only in B, **grouped by predicate**, each with the
entry that classified it. Classified lines are counted per entry and predicate; unclassified lines are
always listed in full. Files in one tree only are listed, and so are the totals per entry.

## The known-difference list

A JSON file of entries. **A difference is unclassified unless an entry matches it.** Each entry has an
`id`, `refs` (where the difference is recorded: a section of `jets/jetrules/rete/README.md` or a
register entry of `projects/rules_benchmark`) and a `description`, and then:

| Field | |
|---|---|
| `scope` | `cross` (the default), `self` or `any` |
| `only_on` | the label of the side the line is on, such as `go` |
| `subject`, `predicate`, `object` | regular expressions over the term as written in the dump, anchored at both ends |
| `pair` | a pair kind, below. The entry then matches lines on both sides |
| `rel_tol` | `double_near`'s tolerance |

An entry without `pair` matches one line. An entry with no `pair`, `subject`, `predicate` or `object`
is refused, since it would match every line.

**The pair kinds**, tried in this order whatever their order in the list:

| Kind | Matches |
|---|---|
| `relabelled` | the same line up to the labels of minted nodes, under a one-to-one correspondence of labels. The correspondence comes from the line shapes (a line with its labels blanked) that occur once on each side. The node exists on both sides and was renamed because its neighbourhood differs; that difference is in its own lines, which are classified on their own |
| `merged` | the lines of a node on the `only_on` side that corresponds to several nodes of the other side, each of which corresponds to it alone: one node where the other engine has several |
| `double_quantised` | the same subject and predicate, two doubles equal once both are quantised as the Go engine quantises a literal (`graphdump.Quantise`) |
| `double_near` | the same subject and predicate, two doubles within `rel_tol` of each other relative to the larger |
| `text_double_format` | the same subject and predicate, two texts equal once every number in them is parsed, quantised and written alike |
| `predicate_alias` | the same subject and object, two different predicates that both match `predicate` |

Then the single-line entries, in list order.

### The entries, and what they cover

| Entry | Record | Seen on `cm_loop` |
|---|---|---|
| `minted-node-relabelled` | `graphdump` package documentation | yes |
| `go-double-literal-quantised` | rete README, *The Go and C++ engines … disagree*, item 3 | yes |
| `go-double-quantisation-propagated` | the same; a tolerance of 2^-12, weaker evidence | yes |
| `go-lookup-subject-shared-across-tables` | `projects/rules_benchmark` `I-15` | yes |
| `lookup-cache-predicate` | `graphdump` package documentation | no: `CM` makes no multi-lookup |
| `text-plus-double-format` | `projects/rules_benchmark` `I-16` | yes |
| `literal-regex-claimZip3` | `I-12`, `F53` | no: `cm_loop`'s zip codes all start with three digits |
| `literal-regex-walrus-names` | `I-12`, `F53` | not run: a `walrus_ws` workload |

**The README's other differences have no entry**, because nothing in a dump identifies them by its
shape: `jets:value_property` alone (item 1), `jets:entity_property` not watched in Go (item 4), the
callbacks of C++ `erase`, Go `Retract` and its indexes, and `_0:no_truth_main_on_exist`. Each shows as
a different value or a missing triple, unclassified. When one appears, an entry is written for the
predicates it reaches, and the issue is registered.

## Producing the dumps

`rules_bench` writes them once it calls `graphdump.WriteSession` after each rule set. Until then,
`TestDumpRulesUnitTest` in `jets/rulesbench/graphdump` does it for a rules unit test, on either
engine, through `RunRuleSets`:

```bash
LIB=<a build of libjets.so of this commit>/jets
CGO_CFLAGS="-I$PWD/jets" CGO_LDFLAGS="-L$LIB" go test -c -tags jets_native -o graphdump.test ./jets/rulesbench/graphdump
for e in go native; do
  LD_LIBRARY_PATH=$LIB WORKSPACES_HOME=<home> WORKSPACE=usi_ws \
    GRAPHDUMP_TST=<home>/usi_ws/jet_rules/unit_tests/cm_loop/cm_loop.tst.json \
    GRAPHDUMP_OUT=out/run1 GRAPHDUMP_ENGINE=$e ./graphdump.test -test.run '^TestDumpRulesUnitTest$'
done
```
