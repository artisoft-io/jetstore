---
title: Architecture
description: What one JetStore stack contains (VPC, Aurora, ECS, Lambdas, Step Functions, load balancer and WAF) and how the pieces are wired.
kind: explanation
sidebar:
  order: 1
---

JetStore is deployed by an AWS CDK (Go) application, `cdk/jetstore_one`: one CloudFormation stack
containing the VPC, an Aurora Serverless v2 cluster, ECS Fargate services and tasks, Go Lambdas, Step
Functions state machines, an Application Load Balancer with WAFv2, and an optional GPU inference
service.

Stack composition is driven **entirely by environment variables read at synth time**. The
authoritative variable list is the comment block headed *Expected Env Variables* in
`cdk/jetstore_one/jetstore_one.go`, and `main` logs every value at synth. This page describes what
gets built and how the pieces are wired; it does not repeat that list.

The rest of the Installation Guide covers:

- [Deploy the stack](/docs/install/how-to/deploy/): preflight, deploy, first deploy of a new stack;
  and [a second stack sharing a VPC](/docs/install/how-to/deploy-a-second-stack/).
- [Stack outputs](/docs/install/reference/stack-outputs/): the CloudFormation outputs, and which are
  conditional.
- [How a file starts a pipeline](/docs/install/planning/ingest-data-flow/): how a file key travels
  from an S3 event to a running Step Functions execution.
- [DEV, UAT and PROD](/docs/install/planning/environments/): three ways to arrange three
  environments, what each shares, and what each costs.

## The stack

```
                    Internet / corporate network
                              │
                    ┌─────────┴──────────┐
                    │  ALB  + WAFv2      │   public or private (JETS_ELB_MODE)
                    └─────────┬──────────┘
                              │ listener JETS_UI_PORT (8080) -> container :8443
                    ┌─────────┴──────────┐
                    │  UI service        │   Fargate, apiserver, always deployed
                    │  (jetstore-ui)     │
                    └─────────┬──────────┘
                              │ StartExecution
        ┌─────────────────────┼──────────────────────┐
        │                     │                      │
  ┌─────┴──────┐      ┌───────┴───────┐      ┌───────┴───────┐
  │ cpipesSM   │      │ cpipesNativeSM│      │  reportsSM    │   Step Functions
  └─────┬──────┘      └───────┬───────┘      └───────┬───────┘
        │                     │                      │
  sharding / reducing   same, with the native   run-reports
  Lambdas, node Lambda  node Lambda             Fargate task
  or Fargate task, and  (DEPLOY_CPIPES_NATIVE)
  Python node Lambda
  (DEPLOY_CPIPES_PYTHON)
        │
        └──────────────┬───────────────┐
                       │               │
                 ┌─────┴─────┐   ┌─────┴──────┐
                 │  Aurora   │   │ S3 bucket  │
                 │ Postgres  │   │            │
                 └───────────┘   └─────┬──────┘
                                       │ ObjectCreated
                                 ┌─────┴──────┐
                                 │ registerKey│  starts a pipeline
                                 │ V2 Lambda  │
                                 └────────────┘

  Optional: cpipesNativeSM · Python node Lambda · infer service (EC2 GPU, Ollama or vLLM) ·
            private REST API · SQS ingest · purge data
```

| Layer | Resources |
|---|---|
| **Edge** | ALB (`UIELB`), WAFv2 web ACL, optional private REST API Gateway |
| **Compute — services** | UI service (Fargate), infer service (EC2/GPU, optional) |
| **Compute — tasks** | Compute Pipes task, Run Reports task — both Fargate, started by state machines |
| **Compute — Lambdas** | Up to 15 functions; 7 built by default, 8 conditional (see [stack components](/docs/install/reference/stack-components/)) |
| **Orchestration** | `cpipesSM`, `cpipesNativeSM` (optional), `reportsSM` (unless `JETS_SKIP_REPORTS_SM`) |
| **Data** | Aurora Serverless v2 Postgres, S3 bucket, 4 Secrets Manager secrets |
| **Network** | VPC with public/private/isolated subnets, 19 VPC endpoints (fewer with `JETS_VPC_ENDPOINTS_EXCLUDE`), 4 core security groups (2 more conditional) |
| **Ops** | CloudWatch alarms, optional SNS alarm action, optional bastion host |

**Two container images**, both from ECR and both required: `JETS_ECR_REPO_ARN` + `JETS_IMAGE_TAG`
(UI and run-reports) and `CPIPES_ECR_REPO_ARN` + `CPIPES_IMAGE_TAG` (compute pipes). The infer
service adds a third, `INFER_ECR_REPO_ARN` + `INFER_IMAGE_TAG`. The tags are not interchangeable —
each image is built from a different source.

## Stack-level protections

`TerminationProtection` on the stack, `DeletionProtection` on the Aurora cluster, and
`RemovalPolicy_RETAIN` on the infer persistent volume are all deliberate. A `cdk destroy` will not
remove the database or the model volume without those being lifted first; see
[Tear down a stack](/docs/install/how-to/tear-down/).
