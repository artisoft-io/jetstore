---
title: "Components and how a file becomes output"
description: "JetStore's components, what each one does and where its code is, and a file traced from its arrival in the bucket to its output files."
kind: explanation
sidebar:
  order: 1
---

JetStore takes files that land in an S3 bucket, runs a workspace's pipelines over them, and writes
the results back to S3 and to PostgreSQL. This page names the components that do it and follows one
file through them. Each component has deeper pages of its own, linked from where it appears.

The stack is deployed by the CDK application in `cdk/jetstore_one`, and that is where the wiring
below is read from. Most of the logic is not in the Lambda handlers but in the packages under `jets/`
they call.

## The components

```text
            browser
               │  HTTPS, through the load balancer
     ┌─────────┴───────────┐
     │  apiserver          │  jets/apiserver: REST API, the web app, /docs/
     │  (UI service, ECS)  │  compiles the workspace at start
     └──┬───────────────┬──┘
        │ SQL           │ StartExecution (a pipeline started by hand)
        │               │
   ┌────┴─────┐   ┌─────┴────────────────────────────────────────┐
   │PostgreSQL│   │ Step Functions: cpipesSM, cpipesNativeSM,    │
   │ jetsapi +│   │ reportsSM                                    │
   │  domain  │   │   start sharding ─▶ sharding map ─▶          │
   │  tables  │   │   start reducing ─▶ reducing map (Lambda,    │
   └────┬─────┘   │   ECS task or Python Lambda, per step) ─▶    │
        │         │   ... ─▶ run reports ─▶ status update        │
        │         └─────┬────────────────────────────────────────┘
        │               │ StartExecution
        │         ┌─────┴────────────┐
        └─────────┤ registerKeyV2    │  cdk/jetstore_one/lambdas/register_keys
                  │ Lambda           │  logic in jets/datatable
                  └─────┬────────────┘
                        │ ObjectCreated
                  ┌─────┴────────────┐
                  │ S3 bucket        │  input, stage and output prefixes
                  └──────────────────┘
```

| Component | Code | What it does |
|---|---|---|
| **apiserver** | `jets/apiserver`, run as `cbooter apiserver` in the UI service | The REST API the web app calls, the web app's bundle (`static_ide.go`) and this documentation (`static_docs.go`). At start it migrates the system tables, compiles the workspace and migrates the domain tables (`listenAndServe` in `server.go`). A pipeline started by hand goes through it. |
| **Web app** | `jetsclient_ide` | The React and TypeScript application, built into the UI image and served by the apiserver at `/`. See [How the web app is served and built](/docs/developer/testing/web-app/serving/). |
| **Register-key Lambda** | `cdk/jetstore_one/lambdas/register_keys/register_keys_v2`, built on `registerkey.Run`; the logic is `RegisterFileKeys` and `StartPipelinesForInputRegistryV2` in `jets/datatable` | Turns an S3 object-created event into registry rows and a state machine execution. |
| **Starter Lambdas** | `cp_sharding_starter` and `cp_reducing_starter` under `cdk/jetstore_one/lambdas/compute_pipes`, calling `StartShardingComputePipes` and `StartReducingComputePipes` in `jets/compute_pipes` | Plan a run: read the pipeline configuration, decide the nodes and partitions of the next step, and hand the state machine its commands. |
| **Node Lambdas** | `cp_node` (in `cpipesSM`) and `cp_node_native` (in `cpipesNativeSM`, a container image), both calling `CoordinateComputePipes` | Run one node of a sharding or reducing step. `cp_node` carries no rule engine. |
| **Compute-pipes ECS task** | `jets/cmds/cpipes_native_server`, run as `cbooter cpipes_native_server` on Fargate (`build_ecs_tasks.go`) | Runs one node of a reducing step that is too large for a Lambda. It carries the C++ rule engine. |
| **Python node Lambda** | a container image, built only when `DEPLOY_CPIPES_PYTHON` is set (`build_cpipes_lambdas.go`) | Runs a reducing step whose operators only the Python node implements. See [The Python compute pipes node](/docs/developer/tools/cpipes-node/). |
| **State machines** | `buildCpipesSMInternal` in `cdk/jetstore_one/stack/build_cpipes_sm.go`; `build_run_reports_sm.go` | `cpipesSM` and `cpipesNativeSM` have the same shape and differ only in their node Lambda; `reportsSM` runs the run-reports task. |
| **Run-reports and status-update Lambdas** | `cdk/jetstore_one/lambdas/run_reports` (`delegate.CoordinateWorkAndUpdateStatus` in `jets/run_reports/delegate`) and `cdk/jetstore_one/lambdas/status_update` (`StatusUpdate.CoordinateWork` in `jets/datatable`) | The last two states of every run: the reports, then the run's final status. |
| **C++ rule engine** | `jets/rete` and `jets/rdf`, built as `libjets`; reached from Go through cgo in `jets/bridge`, which `jets/compute_pipes/jetrules_native_adaptor` wraps | Runs a pipeline's `jetrules` steps over the rules compiled into `workspace.db`. See [The C++ rule engine](/docs/developer/engine/rule-engine/). |
| **PostgreSQL** | the `jetsapi` schema, described by `jets/jets_schema.json`; the domain tables, generated from the workspace's classes | The system tables hold the configuration, the registry and every run's status; the domain tables hold what rule pipelines write. See [Design of the system tables](/docs/developer/workspace/system-tables/). |
| **S3** | one bucket, created by the stack unless `JETS_BUCKET_NAME` names one | Files arrive under the input prefix, a run writes its partitions under the stage prefix, and its results under the output prefix. |

The deployed resources, their roles, security groups and the variables that gate them are in
[Stack components](/docs/install/reference/stack-components/).

## A file, from the bucket to its output files

### 1. The file lands, and a pipeline is registered

S3 sends an object-created event for every key under `JETS_s3_INPUT_PREFIX` to the register-key
Lambda (`AddEventNotification` in `cdk/jetstore_one/stack/build_registerkey_lambdas.go`). There is
no queue in front of it.

`processMessage` (`cdk/jetstore_one/lambdas/register_keys/registerkey/registerkey.go`) parses the key:
its `k=v` segments give the client, the organisation, the object type and the date
(`SplitFileKeyIntoComponents`, `jets/utils/split_file_key_components.go`). `RegisterFileKeys`
(`jets/datatable/register_file_key_action.go`) looks the file up in `jetsapi.source_config`, writes
`file_key_staging`, reserves a session id, and writes one `input_registry` row per domain key.
`StartPipelinesForInputRegistryV2` (`jets/datatable/pipeline_start_with_input_registry.go`) finds
the pipelines whose inputs are now all present and writes a `pipeline_execution_status` row for each.

The places where a file is accepted and no pipeline starts, and the multi-part and schema-event
paths, are in [How a file starts a pipeline](/docs/install/planning/ingest-data-flow/).

### 2. A state machine is chosen and started

Writing the `pipeline_execution_status` row runs the post-processing hook of `InsertRows`
(`jets/datatable/data_table_action.go`). It reads `state_machine_name` from the process's
`jetsapi.process_config` row, takes a global lock (`lockStateMachine`, taken over when older than five
minutes), and checks the throttling
limits (`checkThrottling`); over the limit, the row waits as `pending`. Otherwise
`startStateMachine` (`jets/datatable/pipeline_execution.go`) starts the named state machine,
`cpipesSM`, `cpipesNativeSM` or `reportsSM`, with the execution named by the session id.

**Which state machine runs a pipeline is a column of its process row, not a deployment setting.**
`DEPLOY_CPIPES_NATIVE` only decides whether `cpipesNativeSM` exists. A pipeline started by hand from
the web app takes the same path: the apiserver writes the same row through the same `jets/datatable`
code, so it is throttled and routed the same way.

### 3. The sharding step

The state machine (`buildCpipesSMInternal`, `cdk/jetstore_one/stack/build_cpipes_sm.go`) first invokes
the sharding starter. `StartShardingComputePipes` (`jets/compute_pipes/actions_start_sharding_cp.go`)
fetches the compiled workspace if this process does not already hold its version
(`workspace.SyncComputePipesWorkspace`), reads the pipeline configuration, a `.pc.json` from the
workspace's `pipes_config/`, and writes the run's configuration into
`jetsapi.cpipes_execution_status`. It returns one command per sharding node.

A map state then runs one node Lambda per command. A command carries only the node's id, its
partition label and the pipeline execution key (`ComputePipesNodeArgs`); the node reads everything
else from `cpipes_execution_status` (`CoordinateComputePipes`, `actions_coordinate_cp.go`). The
sharding nodes read the input file and write it as partitions under the stage prefix:
`<JETS_s3_STAGE_PREFIX>/process_name=<p>/session_id=<s>/step_id=<step>/jets_partition=<label>/`.

Sharding always runs on the node Lambda of the state machine: `cp_node` in `cpipesSM`,
`cp_node_native` in `cpipesNativeSM`.

### 4. The reducing steps, and how each chooses where it runs

The reducing starter, `StartReducingComputePipes` (`actions_start_reducing_cp.go`), prepares the next
step: it lists the step's partitions and returns one command per partition. It also sets the flags
the state machine's choice reads, from the pipeline configuration, **for this step alone**:

| Flag on `ComputePipesRun` | Computed by | Sends the step to |
|---|---|---|
| `usePythonReducingTask` | `EvalUsePythonNode`, from the step's `use_python_node` and `use_python_node_when` | the Python node Lambda, when it is deployed |
| `useECSReducingTask` | `EvalUseEcsTask`, from the step's `use_ecs_tasks` and `use_ecs_tasks_when` | the compute-pipes ECS task, one task per partition |
| `noMoreTask` | no step left to run | the run-reports state |
| none of the above | | the state machine's node Lambda |

The choice is first-match-wins in that order (`EcsOrLambdaChoice` in `build_cpipes_sm.go`). For an
ECS step the commands are argument vectors for `cpipes_native_server` rather than Lambda events, and
the container runs them. After each step the state machine loops back to the reducing starter until
the starter reports `isLastReducing`.

A `use_ecs_tasks_when` expression is evaluated against the run's environment, which includes the
main input's row count, so a pipeline can run a step on Lambdas for small inputs and on ECS for large
ones.

**Rules run on the C++ engine, so not on `cp_node`.** A `jetrules` step builds its engine through
the node's `JetRulesProxy`. `cp_node_native` and `cpipes_native_server` hand it the native factory
(`jetrules_native_adaptor.NewJetRulesFactory`); `cp_node` hands it nothing, and a `jetrules` step
there fails with `NoRuleEngineOnNode` (`jets/compute_pipes/jetrules_interface.go`). The starter
refuses such a configuration before the run (`validateNoRulesOnCpipesSM`,
`actions_start_common.go`), except for a step that may run on the ECS task, whose container carries
the engine. So a rule pipeline's process names `cpipesNativeSM`.

The engine reads the rules compiled into `workspace.db` and the lookup tables in `lookup.db`, both
part of the compiled workspace the node fetched or carried in its image. How a compiled workspace
reaches a node is in
[How a compiled workspace reaches a running process](/docs/developer/compute-pipes/notes/workspace-version-chain/)
and [The workspace on a compute node](/docs/developer/workspace/compute-node-workspace/).

### 5. The outputs

A step writes what the configuration declares:

- **Files**, through the `output_files` of the configuration. A `merge_files` step writes the final
  file, by default under the output prefix, `<JETS_s3_OUTPUT_PREFIX>/<the input file key's folder>/`;
  an output file may instead name the input, stage or schema-events location, or a key of its own
  (`StartMergeFiles`, `pipe_executor_merge_files.go`). Intermediate partitions stay under the stage
  prefix.
- **Rows**, through the `output_tables` of the configuration, copied into PostgreSQL tables
  (`NewWriteTableSource`, `write_table.go`).

### 6. Reports, and the final status

When no step is left, the state machine runs the run-reports Lambda, then the status-update Lambda.
`StatusUpdate.CoordinateWork` (`jets/datatable/status_update.go`) records the run's outcome in
`pipeline_execution_status`, writes the [run manifest](/docs/developer/compute-pipes/run-manifest/)
for a run that completed, and starts the pipelines that were waiting as `pending`
(`StartPendingTasks`). When the run's outputs are usable, it registers the process's output tables
(`output_tables` of `process_config`) in `input_registry` (`RegisterDomainTables`,
`jets/datatable/register_domain_table.go`), so the pipelines that read those tables can start: one
pipeline's output is how the next one is triggered.

Every state catches any failure (`MkCatchProps`) and routes it to the same status-update Lambda with
the error, so a failed run is recorded as failed rather than ending silently.

## Where the configuration comes from

Everything above that is particular to an installation is data, not code:

- the **workspace**: the rules, the data model, the pipeline configurations and the reports, compiled
  by the apiserver at start or at image build time (see [Workspaces](/docs/developer/workspace/)), and
  overridden file by file through the Workspace IDE (see
  [Workspace overrides](/docs/developer/workspace/overrides/));
- the **system tables**: which object types a source carries (`source_config`), which pipelines
  read them (`process_input`, `pipeline_config`), and which state machine runs each process
  (`process_config`);
- the **stack's variables**, read at synth time: which Lambdas, tasks and state machines exist, and
  their sizes (see [Required variables](/docs/install/reference/required-variables/)).
