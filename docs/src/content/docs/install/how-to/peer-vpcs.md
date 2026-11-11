---
title: Peer a VPC with the stack
description: Reach a JetStore stack's VPC from another VPC, an existing one or a new one with a bastion host, with the vpc_peering app.
kind: how-to
sidebar:
  order: 8
---

This guide is written from the CDK app it describes, not from a run in an account: `cdk/vpc_peering`
(`NewVpcPeeringStack` and `main` in `cdk/vpc_peering/vpc_peering.go`, with its `cdk.json`). Check
each step against your account as you go.

A JetStore stack runs in its own VPC, and its load balancer is internal unless `JETS_ELB_MODE` is
`public` ([Network and security](/docs/install/planning/network-and-security/)). `vpc_peering` is a
separate CDK app that peers that VPC with another one, so that people or tools in the other VPC can
reach the stack. It is not the way two JetStore stacks share a network: for that, see
[Several stacks in one account](/docs/install/planning/multiple-stacks/) and
[Deploy a second stack in a shared VPC](/docs/install/how-to/deploy-a-second-stack/).

## What it creates

`NewVpcPeeringStack` creates the stack `VpcPeeringStack`. The *host* VPC is the one being connected;
the *peer* VPC is the JetStore stack's.

| | With `HOST_VPC_ID` set | With `HOST_VPC_ID` unset |
|---|---|---|
| Host VPC | the existing VPC, looked up by id | a new VPC, `10.100.0.0/16`, two availability zones, public subnets only, no NAT gateway, flow logs on |
| Bastion host | none | `PublicJumpServer`, a Linux bastion in a public subnet with the key pair `BASTION_HOST_KEYPAIR_NAME`, SSH open from any IPv4 address |
| Peering connection | host VPC to the JetStore VPC | the same |
| Routes | from each of the host VPC's **public** subnets to the JetStore VPC's CIDR, and from each of the JetStore VPC's isolated and private subnets to the host VPC's CIDR | the same |

Each route table gets one route, even when several subnets share it.

Three consequences to plan for:

- **The two CIDRs must not overlap.** The new host VPC is `10.100.0.0/16` and the JetStore VPC's
  default is `10.10.0.0/16` (`JETS_VPC_CIDR`). An existing host VPC must not overlap whatever the
  JetStore VPC uses.
- **Only the host VPC's public subnets are routed.** A host VPC whose clients sit in private subnets
  needs its own routes added by hand.
- **The peering connection names neither a peer account nor a peer region**, so both VPCs must be in
  the stack's account and in `AWS_REGION`. `HOST_VPC_REGION` changes only where the host VPC is
  looked up.

**The database is not opened by this app.** With `JETS_DB_CLUSTER_ID` set, the code adds an
allowance from the bastion to the cluster on port 5432, but the cluster is imported by identifier
alone, without its security groups, so no ingress rule reaches the database's side. To reach the
database from the host VPC, add an ingress rule to the cluster's security group yourself. The
JetStore stack's own bastion (`BASTION_HOST_KEYPAIR_NAME` on `cdk/jetstore_one`, in
`NewJetstoreOneStack`) is created inside the stack's VPC with that rule, and needs no peering.

## Variables

From the *Expected Env Variables* comment and the checks in `main`, which panics before synth,
naming what is missing:

| Variable | |
|---|---|
| `AWS_ACCOUNT`, `AWS_REGION` | Required. The stack's environment. |
| `JETSTORE_VPC_ID` | Required. The JetStore stack's VPC: its `JetStoreVpcID` output ([Stack outputs](/docs/install/reference/stack-outputs/)). |
| `HOST_VPC_ID` | Optional. An existing VPC to peer. Unset, a VPC and a bastion host are created. |
| `BASTION_HOST_KEYPAIR_NAME` | Required when `HOST_VPC_ID` is unset: an EC2 key pair that already exists in the region. |
| `HOST_VPC_REGION` | Optional. Where to look up `HOST_VPC_ID`; default `AWS_REGION`. |
| `JETS_DB_CLUSTER_ID` | Optional. The JetStore database cluster's identifier, for the allowance described above. |
| `JETS_TAG_NAME_OWNER`, `JETS_TAG_VALUE_OWNER` | Optional. A tag on every resource, when both are set. |
| `JETS_TAG_NAME_PROD`, `JETS_TAG_VALUE_PROD` | Optional. The same, for a production marker. |
| `JETS_TAG_NAME_PHI`, `JETS_TAG_NAME_PII`, `JETS_TAG_NAME_DESCRIPTION` | Optional. Resource tags on the new VPC. The code sets them on the VPC twice, the second time with the values meant for the bastion host, so the VPC ends with `false` and the bastion's description, and the bastion host itself is not tagged. |

## Deploy it

1. Bootstrap the account first ([Bootstrap an account](/docs/install/how-to/bootstrap-an-account/)):
   the VPC lookups run at synth through the CDK bootstrap's lookup role.
2. Set the variables, then diff and deploy from the app's folder; `cdk.json` runs
   `go run vpc_peering.go`, and `main` logs the variables it read:

   ```bash
   cd cdk/vpc_peering
   cdk diff
   cdk deploy
   ```

   The lookups are cached in `cdk.context.json` beside `cdk.json`. Delete an entry there if a VPC
   it describes has changed.

3. From the bastion host, or a client in a host VPC public subnet, open the stack's `UiListenerUrl`
   output. Then run the checks in [Verify a deployment](/docs/install/how-to/verify-a-deployment/).

To remove the peering, `cdk destroy` from the same folder.
