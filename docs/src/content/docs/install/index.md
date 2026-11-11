---
title: Installation Guide
description: What the Installation Guide covers and how it is organised.
kind: overview
sidebar:
  label: Overview
---

The Installation Guide is for whoever deploys JetStore into an AWS account and keeps it running.

JetStore is deployed with the AWS CDK, from the `cdk/` folder of the `jetstore` repository: one
CloudFormation stack, `cdk/jetstore_one`, whose composition is decided by environment variables read
at synth time. The guide has three parts.

- **[Planning](/docs/install/planning/)**: what the stack contains and how it is wired, its network
  and security, running several stacks in one account, arranging DEV, UAT and PROD, how a file
  becomes a pipeline run, and the inference server's memory.
- **[How-to guides](/docs/install/how-to/)**: deploy the stack and check it, deploy a second stack
  in a shared VPC, tear a stack down, find why a file did not start a pipeline, build the inference
  server's AMI, start and stop the inference server, and run against a local checkout.
- **[Reference](/docs/install/reference/)**: the stack's components, the variables a deployment
  requires, and the stack's outputs.

Start with [Architecture](/docs/install/planning/architecture/), then
[Deploy the stack](/docs/install/how-to/deploy/).

Not yet covered here: bootstrapping an account (`cdk/bootstrap_aws`), VPC peering
(`cdk/vpc_peering`), sizing, building and deploying a workspace, upgrading to a new release,
rotating secrets, and the container images.
