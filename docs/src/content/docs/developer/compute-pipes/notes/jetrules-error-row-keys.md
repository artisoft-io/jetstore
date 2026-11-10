---
title: "The keys on a jetrules error row"
description: "What grouping_key and row_jets_key carry on a jetrules process_errors row, and the loop-counter quirk that reporting surfaced."
kind: explanation
sidebar:
  order: 11
---

**Recorded 2026-10-13.**

**Until this date no jetrules site set `grouping_key` or `row_jets_key`**, and the four
sites that write a `process_errors` row (the `ExecuteRules` error, `MAX LOOP REACHED`, a
`jets:exception`, and a property with several values at extraction) all left both null.
Two things about them were not visible from the code.

**The subject in *"property X is not multi-value but has multiple values for subject S"*
is the extracted entity, not the group.** `extractLiteralValue` is called per output
entity, so S is that entity's resource name: for an entity made by `create_entity 0`, a
fresh UUID that is also its `jets:key` (`CreateEntityVisitor`, `jets/rete/expr_op_resources.h`; on the
retired Go engine `CreateEntityKey`, `jets/jetrules/rdf/ast_r_operators.go` at the tag `go-engine-final`).
It names the output row, and says nothing about which input group produced it. That row now
carries S's `jets:key` in `row_jets_key` (the resource name when it has none) and the
property in `input_column`.

**A worker task carries only the group's rows, so the group's key is recovered from the
configuration** (`jetrulesGroupingDomainKey`, `jetrules_grouping_key.go`): the domain key of
the `group_by` or `merge` writing the operator's input channel in the same step, else the
class's only domain key, else none. It is computed from the first row as `compute_domain_key`
computes it, so it is the value of the output tables' `<ObjectType>:domain_key` column, hashed
or not as that is. A `group_by` on names, positions or a count gives no grouping key, by
design: the group then has no domain key. The rule-set sites set `row_jets_key` only when the
session has one input row.

**Reporting a text property with several values surfaced a quirk of the loop.**
`RunRuleSets` inserts `(jets:iState jets:loop n)` for each loop without erasing the previous
value, so by loop 3 a rule matching `(?s jets:loop ?i)` sees 1, 2 and 3. In
`jets/jetrules/test_ws/jet_rules/test_looping_main.jr` that gives each `lp:Person` one
`lp:name` per loop value it joins, which was always written as an array literal and was
silently so; the characterisation test now extracts persons by `jets:key` for that reason.
A workspace with a text property set from the loop counter will see new error rows for the
same reason, and they are correct.
