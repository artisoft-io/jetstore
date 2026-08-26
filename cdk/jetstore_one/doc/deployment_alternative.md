# Deployment alternatives — DEV, UAT and PROD

Three stacks, `JETS_STACK_SUFFIX` of `DEV`, `UAT` and `PROD`, and three ways to arrange them. This
compares what each costs, what each couples, and what each requires you to get right.

Resource counts below are **measured from a synthesized template**
(`cdk.out/JetstoreOneStack.template.json`), not estimated. Prices are not quoted — AWS rates change
and vary by region, so this describes what scales with what and leaves the arithmetic to current
pricing.

## The three scenarios

| | VPC | JetStore bucket | RDS / ALB / ECS |
|---|---|---|---|
| **1 — Full isolation** | 3 | 3 | 3 each |
| **2 — DEV apart, UAT+PROD together** | 2 | 2 (DEV's own; UAT+PROD share, different prefixes) | 3 each |
| **3 — One VPC** | 1 | 2 (DEV's own; UAT+PROD share) | 3 each |

The right-hand column is the point: **nothing in the third column is ever shared.** Every stack
builds its own Aurora cluster, its own load balancer, its own ECS cluster, its own secrets, its own
state machines and its own Lambdas. Sharing changes the network and the bucket, and nothing else.

## The per-stack floor

Present in all three scenarios, three times over, from one synthesized stack:

| Resource | Count | Cost shape |
|---|---|---|
| Aurora Serverless v2 cluster | 1 | Hourly at `JETS_DB_MIN_CAPACITY` ACU (default 0.5) even when idle, plus storage and I/O |
| Application Load Balancer | 1 | Hourly plus LCU |
| UI Fargate service | 1 task, `DesiredCount: 1` | Continuous — 1 vCPU / 4 GiB, always running |
| Secrets Manager secrets | 4 | Per secret per month, plus rotation invocations |
| Lambda functions | 15 | Per invocation; idle costs nothing |
| State machines | 3 | Per state transition |
| ECS cluster | 1 | Free; the tasks are the cost |
| CloudWatch log groups | many | Ingestion and 3-month retention |

**Two of these are the floor that no sharing strategy touches**: the Aurora cluster's minimum
capacity and the always-on UI task. Three environments means three of each, in every scenario. If the
goal is to cut the cost of a DEV environment, lowering `JETS_DB_MIN_CAPACITY` and stopping the UI
service out of hours will do more than any of the choices below.

## What sharing a VPC actually saves

Per created VPC, measured:

| | Count | Cost shape |
|---|---|---|
| Interface VPC endpoints | **18** | **Hourly per endpoint per AZ**, plus per-GB processed |
| Availability zones | 2 (`MaxAzs: 2`) | — |
| **Billable endpoint-AZ units** | **36** | 18 × 2 |
| S3 gateway endpoints | 2 | Free |
| NAT gateways | `JETS_NBR_NAT_GATEWAY`, else 0 | Hourly **and** per-GB processed |
| Internet gateway | 0 or 1 | **Free** |

So the VPC-sharing decision is essentially a decision about **36 endpoint-AZ units and any NAT
gateways**, multiplied by the number of VPCs:

| Scenario | VPCs | Endpoint-AZ units | NAT gateways |
|---|---|---|---|
| 1 | 3 | **108** | 3 × `JETS_NBR_NAT_GATEWAY` |
| 2 | 2 | **72** | 2 × `JETS_NBR_NAT_GATEWAY` |
| 3 | 1 | **36** | 1 × `JETS_NBR_NAT_GATEWAY` |

Interface endpoints are billed whether or not traffic flows, so this is a fixed monthly difference,
not a usage-driven one. It is the single largest lever in the comparison.

Two mechanics worth knowing:

- **Importing a VPC skips endpoint creation entirely.** `AddVpcEndpoints` is only called on the
  create path (`jetstore_one.go:148`); with `JETS_VPC_ID` set, the stack looks up the endpoint
  security group instead and creates no endpoints. The saving is structural, not a tuning knob.
- **The app synthesizes two S3 gateway endpoints per created VPC**, one in `CreateJetStoreVPC`
  (`jetstore_vpc.go:163`) and one in `AddVpcEndpoints` (`jetstore_vpc.go:258`). Gateway endpoints are
  free, so this costs nothing; it is noted because a reader counting endpoints in the console will
  find 20, not 19.

### On "three internet gateways"

**Internet gateways are free.** AWS charges nothing for the gateway itself, and nothing for data
through it beyond the normal data-transfer-out rates that apply regardless. Three IGWs and one IGW
cost the same.

The cost that sits behind the question is the **NAT gateway**, which is billed hourly *and* per GB
processed, and in this stack the two are linked by one variable:

```go
if igEV == "true" || igEV == "TRUE" { internetGateway = true } else { nbrNatGateway = 0 }
```

`JETS_VPC_INTERNET_GATEWAY` gates both — with it unset or false, `JETS_NBR_NAT_GATEWAY` is **forced
to 0** regardless of what you set it to (`nbrNatGateway`, `jetstore_vpc.go:120`). So a stack either has an IGW and
however many NAT gateways you asked for, or neither.

The practical consequence: **most JetStore stacks need neither.** The 18 interface endpoints and the
S3 gateway endpoint exist precisely so that the isolated and private tiers reach AWS services without
egress. `JETS_NBR_NAT_GATEWAY` is documented as needed to reach GitHub for workspace pulls — so if
git integration is off, or the workspace is baked into the image, a NAT gateway per environment is
three recurring charges for a path nothing uses. Check whether DEV and UAT need git access before
giving them one.

## Scenario 1 — full isolation

Three VPCs, three buckets, nothing shared. Separate accounts optional and orthogonal.

**For it.** No shared failure domain and no shared quota. A runaway DEV pipeline cannot exhaust a
NAT gateway, an ENI limit or a bucket request rate that PROD depends on. Blast radius on a
misconfiguration is one environment. If the environments are in separate accounts this is also the
only arrangement that gives a hard IAM boundary — a role in DEV cannot name a PROD resource at all.
Teardown is clean: destroying DEV touches nothing else.

**Against it.** 108 endpoint-AZ units, and up to three NAT gateways. Three VPC CIDRs to allocate and,
if anything must reach across, to keep non-overlapping for future peering.

**When it is right.** Separate accounts, or a compliance boundary that has to be demonstrable rather
than argued. Also the correct default when the environments have different owners.

## Scenario 2 — DEV standalone, UAT and PROD together

DEV gets its own VPC and bucket. UAT and PROD share a VPC and a bucket, separated by
`JETS_s3_INPUT_PREFIX`, `JETS_s3_OUTPUT_PREFIX` and `JETS_s3_STAGE_PREFIX`.

**For it.** 72 endpoint-AZ units instead of 108 — a third off the endpoint bill. DEV, the
environment most likely to be broken, redeployed or destroyed, stays isolated where a mistake costs
nothing. UAT and PROD are the pair most likely to need the same network reachability (the same
on-premises routes, the same peering, the same allowlists), so configuring that once is a real
operational saving beyond the money.

**Against it.** It puts UAT and PROD in the same VPC and the same bucket, which is the *one* pairing
where a mistake is most expensive. UAT exists to run realistic load, and realistic load in a shared
VPC is contention with PROD for NAT bandwidth and ENI capacity.

**What it requires you to get right.** All three prefixes must differ, not just the input prefix.
`JETS_s3_STAGE_PREFIX` and `JETS_s3_SCHEMA_TRIGGERS` **default by string substitution** on the input
prefix (`GetS3StagePrefix`, `stack_model.go`), so setting a distinct `JETS_s3_INPUT_PREFIX` per stack
gives distinct derived prefixes for free — but only if you let them derive. Setting the input prefix
per environment and pinning the stage prefix to a shared literal is the failure this scenario invites.

## Scenario 3 — one VPC for all three

One VPC. DEV keeps its own bucket; UAT and PROD share one.

**For it.** 36 endpoint-AZ units — a third of scenario 1, and one NAT gateway at most. One CIDR, one
set of routes, one peering relationship, one place to change a security group. On pure infrastructure
cost this is the cheapest of the three by a wide margin, and the difference is fixed monthly spend
rather than usage.

**Against it.** Every environment shares one network failure domain and one set of VPC quotas. A DEV
change to the shared endpoint security group affects PROD, and `JETS_VPC_ENDPOINTS_SG_ID` is imported
`Mutable: true` (`Mutable`, `jetstore_vpc.go:75`), so a stack *can* modify the shared group's rules. The
isolation that remains is real but narrower than it looks: each stack still has its own
`RdsAccessSg`, its own database and its own secrets, so DEV cannot reach PROD's data. What it shares
is reachability and capacity, not credentials.

**The subtle one.** Whoever owns the VPC owns it for everybody. Nothing in CloudFormation records
that the DEV stack depends on the VPC the PROD stack created — no output carries an `ExportName`, so
the coupling is values copied at synth (see [`stack_outputs.md`](stack_outputs.md)). Deleting the
owning stack does not warn you that two others are using its network. Create the shared VPC outside
all three stacks if you take this path.

## Sharing a bucket between UAT and PROD

Common to scenarios 2 and 3, and the part with the most sharp edges.

**Notification filters must not overlap.** Each stack registers **two** `OBJECT_CREATED`
notifications on the bucket — one on the input prefix (carrying `JETS_SENTINEL_FILE_NAME` as a suffix
filter when set) and one on the schema-triggers prefix (`build_registerkey_lambdas.go:90`). S3
rejects a notification configuration whose rules overlap for the same event type. `uat/input` and
`prod/input` are fine; `jetstore/input` and `jetstore/input/prod` are not. Prefixes must be siblings,
not nested.

**Two stacks writing one bucket's notification config is a read-modify-write.** For an imported
bucket, CDK marks the custom resource `Managed: false` (confirmed in the synthesized template) and
merges with the existing configuration rather than replacing it, which is what makes this viable at
all. It is still a read-modify-write against shared state: **deploy UAT and PROD one at a time**, not
concurrently, and check the bucket's notification configuration afterwards.

**The blast radius is the bucket.** One bucket policy, one KMS key, one versioning and lifecycle
configuration, one set of access logs, and one request-rate budget covering both environments. A
lifecycle rule written for UAT retention applies to PROD objects under a different prefix unless it is
scoped.

**Keep the removal policy in mind.** A bucket named by `JETS_BUCKET_NAME` is imported and never
destroyed by any stack — which is the right behaviour here, and the reason the shared bucket should
always be pre-created and named rather than created by whichever stack deploys first.

## Choosing

| If this matters most | Take |
|---|---|
| A demonstrable boundary, separate owners, or separate accounts | **1** |
| Cutting fixed cost while keeping the riskiest environment isolated | **2** |
| Lowest infrastructure cost, one team, one network to reason about | **3** |

Two closing points that apply whichever you pick.

**The savings are all fixed, and the floor is untouched.** Sharing changes endpoint and NAT charges
— real, recurring, and independent of usage. It does not change the three Aurora clusters, three
ALBs and three always-on UI tasks, which for a lightly used DEV environment are likely the larger
number. Price the floor before optimising the network.

**Scenario 2 and 3 differ less than they look.** Both share a bucket between UAT and PROD, which is
where the operational sharp edges are; they differ only in whether DEV's *network* is separate. If
the reason for isolating DEV is that it gets broken often, note that the bucket — not the VPC — is
where breaking it would hurt, and DEV has its own bucket in both.
