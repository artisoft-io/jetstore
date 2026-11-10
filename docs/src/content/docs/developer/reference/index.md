---
title: Reference
description: The reference pages generated from JetStore's own source files.
kind: overview
sidebar:
  label: Overview
---

The pages in this section are **generated** when the site is built, from files that JetStore itself
reads. They are therefore as current as the code they were built with, and they are never edited by
hand: to change one, change its source.

| Page | Generated from |
|---|---|
| [Compute pipes configuration](/docs/developer/reference/generated/pipeline-configuration/) | `tools/cpipes_contract/matrix/types.csv` and `fields.csv`, the contract of the `.pc.json` pipeline configuration |
| [System tables](/docs/developer/reference/generated/system-tables/) | `jets/jets_schema.json`, the tables of the `jetsapi` schema |
| [JetRules operators](/docs/developer/reference/generated/jetrules-operators/) | `jets/compilerv2/compiler/operators.txt`, the operators the compiler accepts and the rule engine registers |

The generators are in `docs/scripts/`. `npm run gen` runs them; `npm run dev` and `npm run build`
run them first.
