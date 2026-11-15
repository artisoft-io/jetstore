---
title: Verify a deployment
description: The checks to run after a deploy, in order - the stack's outputs, the UI service, the apiserver's start-up log, signing in, the documentation, a compile and a pipeline run.
kind: how-to
sidebar:
  order: 4
---

This guide is written from the code it describes, not from a run in an account: the stack's outputs
and the UI's load balancer (`cdk/jetstore_one`, `stack/build_elb.go`), the UI container's start
(`jets/cmds/cbooter/main.go`), the apiserver's start-up (`main` in `jets/apiserver/main.go`,
`listenAndServe` and the `check...` functions in `jets/apiserver/server.go`, `Ensure` in
`jets/dblogin/dblogin.go`), and the documentation's serving (`jets/apiserver/static_docs.go`).

Run the checks in this order: each one depends on the one before.

## 1. The stack's outputs

```bash
aws cloudformation describe-stacks --stack-name "${JETS_STACK_ID:-JetstoreOneStack}" --query 'Stacks[0].Outputs'
```

- `UiListenerUrl` is present. It reads `https://` when `JETS_ELB_MODE` is `public`, where the
  listener is HTTPS, and `http://` otherwise.
- If the API was expected, `JetsApiUrl` is present.
- If the inference server was expected, `InferListenerUrl` is present and `PersistentVolumeId` names a
  volume.

What each output holds is in [Stack outputs](/docs/install/reference/stack-outputs/).

## 2. The UI service

The UI service should have one running task, healthy in the load balancer's target group. The health
check is `GET /healthcheck/status` on the container's port 8443, which answers 200 once the apiserver
is listening.

**The apiserver listens only after its start-up checks** (next step), which on a new release migrate
the database and may compile the workspace. A task that ECS stops for failing its health check while
the log shows those checks still running needs more time: set
`JETS_UI_HEALTH_CHECK_GRACE_PERIOD_SEC` and deploy again.

A task that starts and stops at once is usually the container's start failing before the apiserver
runs: `cbooter` copies the workspace from the image to `WORKSPACES_HOME`, and logs
*Failed to copy workspace files* when that fails. Read the UI container's log group,
`UiContainerLogGroup`.

## 3. The apiserver's start-up log

In the same log group, the apiserver logs its arguments and environment (*apiserver argument:*, then
*Got argument: ...* and *ENV ...* lines), then runs its checks in this order. Each line below is one
of the lines it can print; a check that fails stops the apiserver, and the reason is the last line.

| Check | Lines | What to look for |
|---|---|---|
| The release | *JetStore image version JETS_VERSION is ...* | the `JETS_VERSION` you built |
| System tables (`checkSystemTablesVersion`) | *New JetStore release deployed, migrating the system tables before the workspace compile*, or *JetStore system tables are at the deployed release ... - no migration needed* | the first on a new release, the second on a restart |
| Restricted logins (`Ensure`) | *Restricted logins jets_reader_... (reads) and jets_ide_... (Query Tool, Submit DDL, runbook ddl, workspace scripts) on schemas [...]; IDE code guard on* | present on every start; the database user needs `CREATEROLE` for it. *IDE code guard off* means the database user may not create event triggers, or *(disabled by JETS_IDE_CODE_GUARD)* that the variable turned it off, and a *Warning:* line follows; see [the code jets_ide may not create](/docs/developer/workspace/system-tables/#the-code-jets_ide-may-not-create) |
| Interrupted work | *Swept interrupted work: ...* | only when a stopped apiserver left work running |
| Workspace (`checkWorkspaceVersion`) | *Taking workspace from local repository, recompilation not required*, *Recompiling workspace (...) as version ...*, or *Workspace version in database ... >= JetStore image version ..., no need to recompile workspace* | which one, and why, is in [Upgrade to a release](/docs/install/how-to/upgrade-to-a-release/#after-the-deploy-the-workspace) |
| Database scripts (`checkDomainTablesVersion`) | *New JetStore Release deployed, update domain and system tables and run workspace db init script*, or *JetStore version in database ... >= JetStore image version ...* | the first on a new release |
| Administrator (`initUsers`) | *Admin User is not defined in users table, creating it* | on the first deploy only |
| Listening | *Listening to address ...* | the apiserver is up |

**Read every *Warning:* line between them.** One in particular asks for a compile from the UI:
*Warning: compile time ... does not compare above JetStore version ...*; see
[Upgrade to a release](/docs/install/how-to/upgrade-to-a-release/#after-the-deploy-the-workspace).

## 4. Sign in

Open the UI and sign in ([Sign in](/docs/user/screens/sign-in/)). On a first deploy the only account
is the administrator's: the login is `JETS_ADMIN_EMAIL` (`admin` when it is not set), and the password is the value of the
stack's administrator password secret (see [Rotate secrets](/docs/install/how-to/rotate-secrets/#the-secrets)).

## 5. The documentation

Press **Help** in the top bar. It opens this documentation, at the page for the screen you are on, in a
tab of its own. The site is served by the apiserver at `/docs/`, from the files the UI image carries
(`DOCS_DEPLOYMENT_DIR`, logged at start; default `/usr/local/lib/docs`).

Help first asks the apiserver for a documentation cookie; `/docs/` refuses a browser without one. So
an address under `/docs/` opened directly in a new browser answers
*Open the documentation from JetStore's Help button*: that is the check working, not a fault. A
missing page is the site's own *404* page.

## 6. A compile

Open **Workspaces**, tick the active workspace, and press **Compile Workspace**; **Refresh** until its
status shows the compile has finished; if it failed, **View Last Log** says why
([Work on a workspace](/docs/user/how-to/work-on-a-workspace/#compile)).

## 7. A pipeline run

Load a file and run a pipeline on it, and check the run ends with its outputs written
([Run a pipeline](/docs/user/how-to/run-a-pipeline/)). On a new installation,
[Your first pipeline](/docs/user/tutorials/first-pipeline/) walks the whole path. If the file lands and
nothing runs, follow
[Find why a file did not start a pipeline](/docs/install/how-to/troubleshoot-ingest/).

## See also

- [Environment variables](/docs/install/reference/environment-variables/): what each service reads.
