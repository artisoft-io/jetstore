---
title: "Object properties are graph structure, not columns"
description: "Why a flat record carries a class's data properties only, and the defect that discarded the values of multi-valued object properties."
kind: explanation
sidebar:
  order: 4
---

**Symptom.** A pipeline reports, at warning level, that a property it can plainly
see is multi-valued is not:

```
warning: property cintel:has_Medical_Events is not multi-value but has multiple
values for subject 4c336cda-…, setting value to null
```

and the values are discarded.

**The rule, which the code now states in both places it matters.** A flat record
— the main flow, an entity exported to CSV or a domain table — carries the
class's **data properties**. Object properties are how the graph is *traversed*
when a channel asks for a serialised entity, and the serialisation lands in a
**data** property: in `patient_profile.pc.json` the `toon` encoding names
`cintel:Claim_Summary`, which is `type: text`, while the two object properties
walked to build it are `type: resource`.

That is why the filter needs no knowledge of `column_encodings`, and why it is
unconditional: **an encoding names the data property that receives the
serialisation, never the object property that was traversed.**

**What was wrong.** `GetDomainProperties` derived a flat record's columns from
every entry in the domain table, object properties included. Those columns then
reached `extractLiteralValue`, where `GetMultiValueDataProperties` excludes
object properties from the multi-value set — so a **multi-valued** object
property looked single-valued and its values were dropped. Both halves arrived in
`b2461545` (2026-07-05), the commit that added object properties: it split them
out of the multi-value set on the assumption they would all take the
special-encoding path, collected them into `objectProperties` — **which was
stored on the worker and never read** — and left the derived column list
untouched. The split was started and not finished.

`jets/workspace.DomainColumnsOf` applies the same rule to the database schema,
where the effect was a column that could only ever be null.

**The natural experiment that identified it** is inside one class.
`cintel:Patient_Profile` has four array columns; two carry `is_object` and two do
not, and only the two object ones were nulled:

| column | `as_array` | `is_object` | before |
|---|---|---|---|
| `cintel:has_Medical_Events` | ✔ | ✔ | nulled |
| `cintel:has_Pharmacy_Events` | ✔ | ✔ | nulled |
| `jets:ruleTag` | ✔ | — | fine |
| `rdf:type` | ✔ | — | fine |

**The escape hatch is deliberate.** Only the *derived* list is filtered;
`chSpec.Columns` is appended afterwards and is not, so an explicitly listed
column is still honoured if one is ever wanted materialised.

**And this changes the output schema**: those columns are gone rather than
present-and-null. That is the intended behaviour and it is worth saying out loud,
because anything reading the CSV or the domain table by position will notice.
