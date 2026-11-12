---
title: Reference
description: The stack's components, the variables a synth and the running services read, the CloudFormation outputs, and the images.
kind: overview
sidebar:
  label: Overview
---

| Page | Contents |
|---|---|
| [Stack components](/docs/install/reference/stack-components/) | Every service, task, Lambda and state machine the stack builds: what gates it, its role and security groups, and what it talks to |
| [Required variables](/docs/install/reference/required-variables/) | The environment variables a synth refuses to run without, those that fail later, and the components that are skipped silently |
| [Stack outputs](/docs/install/reference/stack-outputs/) | The CloudFormation outputs, and which are conditional |
| [Environment variables](/docs/install/reference/environment-variables/) | The variables the running services read, with each one's default and effect |
| [Images](/docs/install/reference/images/) | Each Dockerfile: what it builds, from what base, and what runs it |

The complete list of environment variables a synth reads is the comment block headed
*Expected Env Variables* in `cdk/jetstore_one/jetstore_one.go`, below the function
`NewJetstoreOneStack`. A synth logs every value it read. What the running services read is in
[Environment variables](/docs/install/reference/environment-variables/).
