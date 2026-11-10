---
title: Developer's Guide
description: What the Developer's Guide covers and how it is organised.
kind: overview
sidebar:
  label: Overview
---

The Developer's Guide is for engineers who extend or maintain JetStore. It assumes you can read Go,
and that you have a checkout of the `jetstore` repository.

It covers:

- **Architecture**: the components and how a file becomes output. The apiserver, the register-key
  and compute-pipes Lambdas, the state machines and ECS tasks, the native rule engine and its bridge,
  PostgreSQL and S3. Also the workspace compiler, and the decisions behind the design with their
  reasons.
- **Compute pipes**: the pipeline engine, its operators and their configuration.
- **The rule engine and the workspace**: JetRules, the compiler, and how a workspace is laid out and
  compiled.
- **Tools and testing**: the command-line tools, and how to test JetStore's Go, C++ and UI code, the
  rules unit-test harness and the rules benchmark.
- **[Reference](/docs/developer/reference/)**: pages generated from the code at build time, for the
  compute-pipes configuration, the system tables and the JetRules operators.
- **[Contributing](/docs/developer/contributing/documentation/)**: how to write and preview these
  pages.

Material is being moved here from the readme files that sit beside the code. A readme that has moved
is replaced by a pointer to its page.
