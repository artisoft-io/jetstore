---
title: Planning
description: What a JetStore installation contains, how its pieces are wired, and the choices to make before deploying one.
kind: overview
sidebar:
  label: Overview
---

Read these pages before a first deployment, or before changing how an installation is arranged.

| Page | What it answers |
|---|---|
| [Architecture](/docs/install/planning/architecture/) | What one JetStore stack contains, and how the pieces connect |
| [Network and security](/docs/install/planning/network-and-security/) | The IAM roles, subnets, VPC endpoints and security groups the stack creates |
| [Several stacks in one account](/docs/install/planning/multiple-stacks/) | Running a second stack beside the first, sharing a VPC, and leaving out what an installation does not use |
| [DEV, UAT and PROD](/docs/install/planning/environments/) | Three ways to arrange three environments, what each shares, and what each costs |
| [How a file starts a pipeline](/docs/install/planning/ingest-data-flow/) | The path from a file landing in S3 to a running Step Functions execution |
| [Inference server memory](/docs/install/planning/infer-server-memory/) | How the optional GPU inference server uses memory, and how to choose its Ollama settings |
| [Sizing an installation](/docs/install/planning/sizing/) | What has a size (database, ECS tasks, Lambdas, the compute-pipes cluster, the inference server), its default, and the setting that changes it |
| [Customising an installation](/docs/install/planning/customise-an-installation/) | What makes one installation different from another, and where each difference lives |

Every component of the stack, and what decides whether it is built, is listed in the
[stack components](/docs/install/reference/stack-components/) reference.
