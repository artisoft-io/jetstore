---
title: How-to guides
description: Step-by-step guides for deploying, verifying, operating and tearing down a JetStore installation.
kind: overview
sidebar:
  label: Overview
---

Each guide is one task.

| Guide | Task |
|---|---|
| [Deploy the stack](/docs/install/how-to/deploy/) | Prerequisites, the deploy itself, the first deploy of a new stack, and the checks afterwards |
| [Deploy a second stack in a shared VPC](/docs/install/how-to/deploy-a-second-stack/) | Add a stack that reuses an existing stack's network |
| [Tear down a stack](/docs/install/how-to/tear-down/) | Lift the deletion guards and remove a stack, and what survives it |
| [Find why a file did not start a pipeline](/docs/install/how-to/troubleshoot-ingest/) | Walk the ingest path in order when a file lands and nothing runs |
| [Build the inference server AMI](/docs/install/how-to/build-the-infer-ami/) | Build the GPU image the inference server's instances boot from |
| [Start and stop the inference server](/docs/install/how-to/start-and-stop-the-infer-server/) | Scale the GPU service and its instance up and down, from the command line, from Go or from the UI |
| [Run against a local checkout](/docs/install/how-to/run-against-a-local-checkout/) | Settings a workstation needs so the UI cannot write into the developer's own repository |
