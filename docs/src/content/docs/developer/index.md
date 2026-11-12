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

- **[Architecture](/docs/developer/architecture/)**: [the components and how a file becomes
  output](/docs/developer/architecture/components/): the apiserver, the register-key and compute-pipes
  Lambdas, the state machines and ECS tasks, the native rule engine and its bridge, PostgreSQL and S3.
  Also [the decisions behind the design](/docs/developer/architecture/decisions/), each with where its
  reason is recorded.
- **[How-to guides](/docs/developer/how-to/)**: [build JetStore from
  source](/docs/developer/how-to/build-from-source/), write a workspace Lambda or a compute-pipes
  operator, add a system table, a screen or user flow, or a workspace file type.
- **[Compute pipes](/docs/developer/compute-pipes/)**: the pipeline engine,
  [its operators](/docs/developer/compute-pipes/operators/) and their configuration.
- **The rule engine and the workspace**: JetRules, the compiler, and how a workspace is laid out and
  compiled.
- **[Tools](/docs/developer/tools/) and [testing](/docs/developer/testing/)**: the command-line
  tools, and how to test JetStore's [Go and C++](/docs/developer/testing/go-and-cpp/) and UI code, the
  rules unit-test harness and the rules benchmark.
- **[Reference](/docs/developer/reference/)**: pages generated from the code at build time, for the
  compute-pipes configuration, the system tables and the JetRules operators.
- **[Contributing](/docs/developer/contributing/documentation/)**: how to write and preview these
  pages.

Much of this guide was moved here from the readme files that sit beside the code; a readme that has
moved is a pointer to its page. The pages under *Engineering notes* are dated records of how a part of
the design was reached, and say so.
