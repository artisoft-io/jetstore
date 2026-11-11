---
title: "Rule engine and compiler"
description: "JetStore's rule engine, the JetRules compiler, how SQL files are read, the runbook format, and logging."
kind: overview
sidebar:
  label: Overview
---

This section covers the parts of JetStore that run a workspace's rules and the code around them:

| Page | What it covers |
|---|---|
| [The C++ rule engine](/docs/developer/engine/rule-engine/) | `jets/rete` and `jets/rdf`: retraction, values and operators, the network at run time, limits, and the retired Go engine |
| [The JetRules compiler](/docs/developer/engine/compiler/) | `jets/compilerv2`: how rules are compiled, how antecedents are reordered, and JetRules written as JSON |
| [How JetStore reads a SQL file](/docs/developer/engine/sql-scripts/) | `jets/sqlscript`: statement boundaries, the save-time check, and why most SQL files run whole |
| [The workspace runbook format](/docs/developer/engine/runbook-format/) | `jets/runbook`: the files, steps, types and result of a runbook |
| [Logging and log injection](/docs/developer/engine/logging/) | structured JSON logging, and the call every Go binary makes |

How a workspace is laid out, compiled and installed is in [Workspaces](/docs/developer/workspace/).
The operators a rule may call are listed in the generated
[JetRules operators](/docs/developer/reference/generated/jetrules-operators/) page.
