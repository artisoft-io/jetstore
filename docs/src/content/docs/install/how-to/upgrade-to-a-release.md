---
title: Upgrade to a release
description: Read a release's upgrade notes, rebuild the images in order, deploy, and check what the apiserver did to the database and the workspace.
kind: how-to
sidebar:
  order: 5
---

This guide is written from the Dockerfiles and code it describes, not from a run: the files in
`dockerfiles/`, and the apiserver's start-up in `jets/apiserver/server.go` (`listenAndServe`,
`checkSystemTablesVersion`, `checkWorkspaceVersion`, `startupCompileVersion` and
`checkDomainTablesVersion`). Your build scripts, which run the `docker build` commands, are outside
the `jetstore` repository.

## 1. Read the upgrade notes

Each release of JetStore is a GitHub release of `artisoft-io/jetstore`, and its notes have a section
**Upgrade notes — read before deploying**. Read it for the release you are going to, and for every
release between it and the one you run. It says which images to rebuild, which settings are new or
changed, and anything the database or the stack needs before or after the deploy.

Then check out the release's tag in your `jetstore` checkout. To see what changed in the parts the
build depends on:

```bash
git diff <deployed-tag> <release-tag> --stat -- dockerfiles/ cdk/
```

## 2. Rebuild the images, in order

The order is the order of the `FROM` lines, described in
[Build and deploy a workspace](/docs/install/how-to/build-and-deploy-a-workspace/):

1. **`cpipes_base_builder`, when the notes say so**, or when `Dockerfile.cpipes_base_builder` changed:
   it pins the Go and Node versions every later image is built with.
2. **`cpipes_builder`, always.** It holds JetStore's source and builds its binaries, so every image
   after it, the web app and this documentation included, is only as new as it is.
3. **JetStore's images**: `ui_service`, `cpipes`, `cpipes_native_lambda` when `DEPLOY_CPIPES_NATIVE`
   is on, and the Python node Lambda and the inference server images when the stack deploys them.
   If a workspace has its own compute pipes binaries, rebuild `cpipes_builder_ws` first and build
   `cpipes` and `cpipes_native_lambda` from it.
4. **Each workspace's images**: `compile_ws`, then `ui_service_ws`, `cpipes_ws` and
   `cpipes_native_lambda_ws`.

Give the whole build one new `JETS_VERSION` that compares above the deployed one as a string (a Unix
time at build does), and push with new tags. The apiserver decides everything below by comparing
`JETS_VERSION` with what the database records, so a version that does not compare above it is a
deploy that migrates nothing.

If you deploy from the image `dockerfiles/Dockerfile.cdk` builds, rebuild it when the release changes
it: an older CDK CLI fails at synth against a newer CDK library.

## 3. Deploy

Set `JETS_IMAGE_TAG` and `CPIPES_IMAGE_TAG` to the new tags, and any setting the upgrade notes name,
then diff and deploy from the release's checkout as in [Deploy the stack](/docs/install/how-to/deploy/).

**When the installation has workspace changes saved in the database, the new UI task compiles the
workspace before it answers its health check** (below). If the compile takes longer than the load
balancer allows, ECS stops the task and starts another. Set `JETS_UI_HEALTH_CHECK_GRACE_PERIOD_SEC`
to give it time (`UiHealthCheckGracePeriodSec`, `cdk/jetstore_one/stack/stack_model.go`); unset, CDK's
default for a service behind a load balancer, 60 seconds, applies.

## After the deploy: the database

The apiserver brings the database to the release at start, before it opens its port, comparing its
`JETS_VERSION` with the highest version in `jetsapi.jetstore_release`:

1. `checkSystemTablesVersion` migrates the system tables (`jets_schema.json`) and logs
   *New JetStore release deployed, migrating the system tables before the workspace compile*.
2. The restricted database logins are created or brought up to date, and logged as
   *Restricted logins ...*.
3. `checkWorkspaceVersion` decides what to do with the workspace (next section).
4. `checkDomainTablesVersion` runs `update_db` with `-initBaseWorkspaceDb -migrateDb`, which applies
   the workspace's database init script and the release's data updates, then records the release in
   `jetstore_release`.

The release is recorded last, so a start that fails part-way repeats all of it at the next start.
Any of these steps failing stops the apiserver; the reason is the last line of its log.

An older image deployed over a newer database finds the database ahead of it and changes nothing.
JetStore does not migrate down.

## After the deploy: the workspace

`checkWorkspaceVersion` compares `JETS_VERSION` with the highest version in
`jetsapi.workspace_version`. On a new release it is higher, and what happens depends on whether the
installation has workspace changes saved in the database (`jetsapi.workspace_changes`: files saved
from the Workspace IDE, or lookups a report refreshed):

| Saved changes | What the start-up does | Version recorded |
|---|---|---|
| none | takes the compiled workspace from the image and uploads it to the database: *Taking workspace from local repository, recompilation not required* | `JETS_VERSION` |
| some | writes them over the image's workspace and compiles it: *Recompiling workspace (... overrides applied: true) as version ...* | the time of the compile |

The second row is why **an installation with saved changes usually needs no compile from the UI**:
the start-up has compiled them under a version of their own, and the pipelines fetch that workspace
rather than use the one in their image (`startupCompileVersion`).

**Compile once from the UI when the log shows this warning instead:**

```
Warning: compile time <t> does not compare above JetStore version <v>; keeping <v>, so the pipelines will use their image's workspace and not the overrides
```

`startupCompileVersion` keeps `JETS_VERSION` when the time of the compile does not compare above it,
which happens when `JETS_VERSION` is not a Unix time or the task's clock is behind the build's. The
compiled workspace then carries the same version as the images', and the pipelines use their image's,
without the saved changes. A compile from **Workspaces**, **Compile Workspace**
([Work on a workspace](/docs/user/how-to/work-on-a-workspace/#compile)) records a new version and
ends it.

**Compile once from the UI, too, when the log shows**
*Workspace version in database ... >= JetStore image version ..., no need to recompile workspace*
on the first start of a new release. The workspace was compiled from the UI after the images were
built, so its version compares above the new `JETS_VERSION`, and the start-up takes that compile from
the database: one made by the previous release.

To check either way, from the [Query Tool](/docs/user/screens/query-tool/):

```sql
SELECT version, workspace_commit FROM jetsapi.workspace_version ORDER BY version DESC LIMIT 3
```

With saved changes, the newest version is later than `JETS_VERSION`; without, it equals it. A
version equal to `JETS_VERSION` while saved changes exist is the case the warning describes. The
background is in
[A workspace override and the version that names it](/docs/developer/compute-pipes/notes/workspace-override-version/).

Saved changes are applied again at every new release, but they reach the next image only once they
are committed to the workspace's repository.

## 4. Verify

Run the checks in [Verify a deployment](/docs/install/how-to/verify-a-deployment/).
