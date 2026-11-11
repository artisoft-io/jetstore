---
title: How-to guides
description: Step-by-step guides for bootstrapping an account, building, deploying, verifying, upgrading and operating a JetStore installation, and tearing it down.
kind: overview
sidebar:
  label: Overview
---

Each guide is one task. The first five are in the order a new installation needs them.

| Guide | Task |
|---|---|
| [Bootstrap an account](/docs/install/how-to/bootstrap-an-account/) | Prepare an empty account: the CDK bootstrap, the image repositories, and optionally the role a GitHub workflow deploys with |
| [Build and deploy a workspace](/docs/install/how-to/build-and-deploy-a-workspace/) | Build a workspace's images from JetStore's, push them, and deploy them |
| [Deploy the stack](/docs/install/how-to/deploy/) | Prerequisites, the deploy itself, and the first deploy of a new stack |
| [Verify a deployment](/docs/install/how-to/verify-a-deployment/) | The checks after a deploy: outputs, the UI service, the start-up log, sign-in, the documentation, a compile, a pipeline run |
| [Upgrade to a release](/docs/install/how-to/upgrade-to-a-release/) | Read the upgrade notes, rebuild the images in order, deploy, and check what the start-up did |
| [Rotate secrets](/docs/install/how-to/rotate-secrets/) | The stack's secrets, their rotation schedule, rotating one now, and how a rotation reaches the running processes |
| [Deploy a second stack in a shared VPC](/docs/install/how-to/deploy-a-second-stack/) | Add a stack that reuses an existing stack's network |
| [Peer a VPC with the stack](/docs/install/how-to/peer-vpcs/) | Reach the stack's VPC from another VPC, or from a new one with a bastion host |
| [Tear down a stack](/docs/install/how-to/tear-down/) | Lift the deletion guards and remove a stack, and what survives it |
| [Find why a file did not start a pipeline](/docs/install/how-to/troubleshoot-ingest/) | Walk the ingest path in order when a file lands and nothing runs |
| [Build the inference server AMI](/docs/install/how-to/build-the-infer-ami/) | Build the GPU image the inference server's instances boot from |
| [Start and stop the inference server](/docs/install/how-to/start-and-stop-the-infer-server/) | Scale the GPU service and its instance up and down, from the command line, from Go or from the UI |
| [Run against a local checkout](/docs/install/how-to/run-against-a-local-checkout/) | Settings a workstation needs so the UI cannot write into the developer's own repository |
