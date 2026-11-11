---
title: The rule engine
description: What a rule is, how the rule engine sees one member's records, what it writes, and where its settings and errors show up, for a user rather than a developer.
kind: explanation
sidebar:
  order: 7
---

The rule engine is the part of JetStore that draws conclusions. A pipeline hands it the records of
one entity at a time, a member's eligibility and claims in `jets_ws`; the rules add what follows from
them; and the pipeline writes the result out. This page is what a user of JetStore needs to know
about it. How it works inside is in the Developer's Guide, under
[The C++ rule engine](/docs/developer/engine/rule-engine/).

## Facts

Inside the engine, data is a set of **facts**, each one a statement of three parts: a subject, a
property and a value.

```
  member 1001    rdf:type          hc:Eligibility
  member 1001    hc:Member_ID      "1001"
  member 1001    hc:Member_DOB     1957-04-02
  claim C-17     rdf:type          hc:PharmacyClaim
  claim C-17     hc:Member_ID      "1001"
  claim C-17     hc:NDC            "00001005823"
```

Each mapped row becomes an entity, and each of its columns a fact about it, named by the property
the source mapping gave it. That is why the mapping matters: **the rules know only the data model's
properties**, never a vendor's column names.

The data model's classes build on each other. In `jets_ws`, `hc:Eligibility` is both an `hc:Member`
and an `hc:Subscriber`, so a rule written about any `hc:Member` applies to an eligibility row too.

## Rules

A **rule** says: *when facts like these hold, add these facts.* Rules are written in JetRules, in the
workspace's `jet_rules/` folder. This one, from `jets_ws`, copies a list of properties from the
member onto the member's patient profile:

```
[CI_CopyMemberProperties]:
  (?member rdf:type hc:Member).
  (?profile rdf:type cintel:Patient_Profile).
  (iProperties memberProperties2Copy ?property).
  (?member ?property ?v)
->
  (?profile ?property ?v)
;
```

The lines before the arrow are the conditions, and `?member`, `?profile`, `?property` and `?v` are
variables the engine fills with every combination that matches. The line after it is what is added.

**Rules chain.** A fact one rule adds can satisfy another rule's conditions, so the engine keeps
applying rules until none can add anything new. The engine, not the order of the file, decides when
each rule applies, which is what lets a rule file be read one rule at a time: each rule states one
consequence.

## One member at a time

The engine does not see the whole file. The pipeline groups the rows by their
[domain key](/docs/user/concepts/source-configurations/#domain-keys), and gives the engine one group
at a time: in `jets_ws`, one member's eligibility row and all of that member's medical and pharmacy
claims. That unit is a **rule session**. Its facts are discarded when the session ends.

So a rule can compare a member's claims with each other and with the member's eligibility, but never
with another member's. That limit is what lets a run spread over many workers, and it is why the
domain key has to be right: a member's claims under a different key would never meet the member.

## What the rules write

The data model marks some classes as tables, and `cintel:Patient_Profile` is one. At the end of each
rule session, the pipeline takes the entities of the classes its definition names and writes each as a
row of its table, with its properties as columns and the run's session id beside them. The rules
create the profile and fill it in; the pipeline writes it.

## Lookups

Rules often need reference data: a drug's name and class from its NDC code, the description of a
diagnosis. A **lookup** is a table of such data, a CSV file in the workspace's `lookups/` folder with a
declaration of its key and columns. Rules query it by key. Lookups are compiled into the workspace
with the rules, so they change with a new workspace, not with the client's data.

## Rule settings at run time

Some values are better set when a pipeline runs than written into the rules. A pipeline
configuration can carry **rule settings**, given to every rule session of its runs. `CI`'s
`Patient_Profile` configuration sets `jets:max_vertex_visits` to 1,000,000, and has a setting,
`cintel:breakOnMemberID`, that makes the rules raise an error on one member, or on every member, to
help find a fault. The *Rules Configuration* screen holds further settings per client and process.
Settings given at run time add to those in the rule files and do not replace them.

## When something goes wrong

- **A rule can raise an error** on purpose, when the data breaks an expectation. It is recorded as a
  **process error** against the member's session, with the member's key, and the run ends with the
  status `errors`. The *Process Errors* screen lists them.
- **A runaway session is stopped.** Rules that keep adding facts forever would never end, so the
  engine counts the work done in each session and stops it past a limit, `jets:max_vertex_visits`.
  The stop is recorded as an error like any other.
- **What happens to a session with an error** is set in the pipeline definition: by default its
  output is still written; it can be dropped instead, or the whole run failed.

## Changing the rules

A rule change takes effect when the workspace is [compiled](/docs/user/concepts/workspaces/#why-it-is-compiled).
The compiler checks the rules and reports errors with the file and line they are on. Before a change
reaches a pipeline, **rules unit tests** can check it: each test, a `.tst.json` file under
`jet_rules/unit_tests/`, gives the rules some input entities and says which facts must and must not
hold afterwards. They are run from the *Workspaces* screen. How to write them is in the Developer's
Guide, under [Rules unit tests](/docs/developer/testing/rules-unit-tests/).
