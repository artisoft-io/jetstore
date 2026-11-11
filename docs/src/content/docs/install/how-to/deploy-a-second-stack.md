---
title: Deploy a second stack in a shared VPC
description: Add a JetStore stack that reuses an existing stack's VPC and endpoints, and what the two stacks do and do not share.
kind: how-to
sidebar:
  order: 7
---

Why this works, and what a shared VPC costs and couples, is explained in
[Several stacks in one account](/docs/install/planning/multiple-stacks/) and
[DEV, UAT and PROD](/docs/install/planning/environments/).

Set `JETS_STACK_ID` and `JETS_STACK_SUFFIX` **together** — the first names the CloudFormation stack,
the second is appended by `props.MkId` to every resource whose name must be unique in the account.

Then take three values from the first stack's outputs (see [Stack outputs](/docs/install/reference/stack-outputs/)):

| Set on the second stack | From the first stack's output |
|---|---|
| `JETS_VPC_ID` | `JetStoreVpcID` |
| `JETS_VPC_ENDPOINTS_SG_ID` | `VpcEndpointsSGID` |
| `JETS_API_GATEWAY_VPC_ENDPOINT_ID` | `ApiGatewayVpcEndpointId` — only if deploying the private API |

With `JETS_VPC_ID` set, five variables become inert: `JETS_NBR_NAT_GATEWAY`,
`JETS_VPC_INTERNET_GATEWAY`, `JETS_VPC_CIDR`, `AWS_PREFIX_LIST_ROUTE53_HEALTH_CHECK` and
`AWS_PREFIX_LIST_S3`. The last two are still required by the preflight even though nothing consumes
them on this path.

Three things worth knowing about what is and is not shared:

- **Only the endpoint security group is shared.** `RdsAccessSg` and `InternetAccessSg` are created
  per stack, so each stack keeps its own database boundary and its own Aurora cluster.
- **The CIDR is read back from the imported VPC** (`LookupJetStoreVPC`, `cdk/jetstore_one/stack/jetstore_vpc.go`), so you do not pass it.
- **There is no CloudFormation dependency between the stacks.** No output carries an `ExportName`;
  the coupling is values copied at synth. The two stacks can be updated and deleted independently,
  and equally, deleting the first stack's VPC out from under the second is not prevented.

Give the second stack its own `JETS_BUCKET_NAME` or its own S3 prefixes. Two stacks sharing a bucket
*and* an input prefix will both receive the same S3 events and both register the same file keys.
