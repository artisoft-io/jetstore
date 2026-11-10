---
title: Installation Guide
description: What the Installation Guide covers and how it is organised.
kind: overview
sidebar:
  label: Overview
---

The Installation Guide is for whoever deploys JetStore into an AWS account and keeps it running.

It covers:

- **Planning**: the components and which of them to deploy (the core stack, the inference server,
  VPC peering), sizing, network and security, and customising an installation for a client.
- **How-to guides**: bootstrap an account, deploy, build and deploy a workspace, upgrade to a new
  release, rotate secrets, and verify a deployment.
- **Reference**: the stack's parameters and outputs, the environment variables each component reads,
  and the container images and what each contains.

JetStore is deployed with the AWS CDK, from the `cdk/` folder of the `jetstore` repository. The
deployment material that lives beside that code is being moved into this guide.
