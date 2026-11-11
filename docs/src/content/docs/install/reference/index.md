---
title: Reference
description: The stack's components, the environment variables a deployment must set, and the CloudFormation outputs it publishes.
kind: overview
sidebar:
  label: Overview
---

| Page | Contents |
|---|---|
| [Stack components](/docs/install/reference/stack-components/) | Every service, task, Lambda and state machine the stack builds: what gates it, its role and security groups, and what it talks to |
| [Required variables](/docs/install/reference/required-variables/) | The environment variables a synth refuses to run without, those that fail later, and the components that are skipped silently |
| [Stack outputs](/docs/install/reference/stack-outputs/) | The CloudFormation outputs, and which are conditional |

The complete list of environment variables the stack reads is the comment block headed
*Expected Env Variables* in `cdk/jetstore_one/jetstore_one.go`, below the function
`NewJetstoreOneStack`. A synth logs every value it read.
