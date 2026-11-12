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
| Database rule | with `JETS_DB_SECURITY_GROUP_ID` set, an ingress rule on that security group: TCP 5432 from the host VPC's CIDR | the same |

Each route table gets one route, even when several subnets share it.

Three consequences to plan for:

- **The two CIDRs must not overlap.** The new host VPC is `10.100.0.0/16` and the JetStore VPC's
  default is `10.10.0.0/16` (`JETS_VPC_CIDR`). An existing host VPC must not overlap whatever the
  JetStore VPC uses.
- **Only the host VPC's public subnets are routed.** A host VPC whose clients sit in private subnets
  needs its own routes added by hand.
- **The peering connection names neither a peer account nor a peer region**, so both VPCs must be in
  the stack's account and in `AWS_REGION`. `main` refuses a `HOST_VPC_REGION` other than
  `AWS_REGION` before synth.

**The database is opened only with `JETS_DB_SECURITY_GROUP_ID`.** Set it to the security group on
the JetStore database cluster, and the app adds a rule to that group admitting TCP 5432 from the host
VPC's CIDR, so the bastion and anything else in the host VPC can reach the database through the
peering. The rule names the CIDR rather than the bastion's security group, which is in the other VPC.
The cluster's security group is not a stack output; read it from the cluster named by the stack's
`JetStore_RDS_Cluster_ID` output:

```bash
aws rds describe-db-clusters --db-cluster-identifier <JetStore_RDS_Cluster_ID> \
  --query 'DBClusters[0].VpcSecurityGroups[].VpcSecurityGroupId'
```

The rule belongs to this app's stack, so `cdk destroy` here removes it. `JETS_DB_CLUSTER_ID`, which
this app read before, never added a rule: the cluster was imported without its security groups. It
is still accepted, and when it is set without `JETS_DB_SECURITY_GROUP_ID`, `main` logs a warning naming
the variable to set instead. The
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
| `HOST_VPC_REGION` | Optional. Where to look up `HOST_VPC_ID`; default `AWS_REGION`, and any other value stops the synth. |
| `JETS_DB_SECURITY_GROUP_ID` | Optional. The JetStore database cluster's security group, opened on 5432 to the host VPC's CIDR as described above. |
| `JETS_DB_CLUSTER_ID` | Optional, superseded by `JETS_DB_SECURITY_GROUP_ID`. Adds no rule; set alone, it logs a warning. |
| `JETS_TAG_NAME_OWNER`, `JETS_TAG_VALUE_OWNER` | Optional. A tag on every resource, when both are set. |
| `JETS_TAG_NAME_PROD`, `JETS_TAG_VALUE_PROD` | Optional. The same, for a production marker. |
| `JETS_TAG_NAME_PHI`, `JETS_TAG_NAME_PII`, `JETS_TAG_NAME_DESCRIPTION` | Optional. Resource tags on the new VPC and the resources in it (`true`, with the VPC's description) and on the bastion host (`false`, with the bastion's description). |

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
