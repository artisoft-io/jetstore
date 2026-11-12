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
  and security, [sizing](/docs/install/planning/sizing/), [customising an
  installation](/docs/install/planning/customise-an-installation/), running several stacks in one
  account, arranging DEV, UAT and PROD, how a file becomes a pipeline run, and the inference server's
  memory.
- **[How-to guides](/docs/install/how-to/)**: [bootstrap an
  account](/docs/install/how-to/bootstrap-an-account/), [build and deploy a
  workspace](/docs/install/how-to/build-and-deploy-a-workspace/), [deploy the
  stack](/docs/install/how-to/deploy/), [verify a deployment](/docs/install/how-to/verify-a-deployment/),
  [upgrade to a release](/docs/install/how-to/upgrade-to-a-release/), [rotate
  secrets](/docs/install/how-to/rotate-secrets/), [peer a VPC](/docs/install/how-to/peer-vpcs/), deploy a
  second stack in a shared VPC, tear a stack down, find why a file did not start a pipeline, build the
  inference server's AMI, start and stop the inference server, and run against a local checkout.
- **[Reference](/docs/install/reference/)**: the stack's components, the variables a deployment
  requires, [the variables the running services read](/docs/install/reference/environment-variables/),
  [the container images](/docs/install/reference/images/), and the stack's outputs.

Start with [Architecture](/docs/install/planning/architecture/), then follow the how-to guides in
their order: bootstrap the account, build the images, deploy, verify.

The how-to guides that act in AWS are written from the scripts and code they call, each named in its
first paragraph.
