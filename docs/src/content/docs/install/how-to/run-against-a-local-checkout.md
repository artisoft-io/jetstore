---
title: Run against a local checkout
description: Why a workstation running JetStore against its own repository should set JETS_NO_GIT_ACCESS, the validator setting a local compile needs, and the settings that point the apiserver at locally built binaries and a port of its own.
kind: how-to
sidebar:
  order: 7
---

This page is about a workstation rather than a stack. It sits in the Installation Guide because the
setting below is an environment variable like the stack's, and one a developer wants **before** the
first run rather than after the first surprise.

**Locally, `WORKSPACES_HOME` is a tree of submodules of the developer's own repository.** JetStore's
checkout on the `jets_ai` branch puts the code at `jetstore_ai/` and the workspaces at `workspaces/`,
so `${WORKSPACES_HOME}/${WORKSPACE}` is a working tree of the repository the developer is editing in,
not a copy the server owns. Two consequences follow, and the second is the expensive one.

**The Workspace Registry screen does not render.** An uninitialised submodule is an existing
directory that is not a repository, so the directory-absent branch of `GetStatus`
(`jets/datatable/git/workspace_git.go`) does not catch it: `git rev-parse` exits 128 with
`fatal: not a git repository`, and the read action turns that into a 400 for the whole request. One
workspace nobody initialised blanks the screen for the others as well.

**A button in a local UI can write into the developer's repository.** The git operations in that
package go through one helper, `runGit`, which sets `cmd.Dir` to `${WORKSPACES_HOME}/${WORKSPACE}`.
`UpdateLocalWorkspace` runs `switch`, `pull` and `push`; `CommitLocalWorkspace` runs `add -A`,
`commit` and `push`. So pressing *Update Local Workspace* in a UI started for an unrelated reason can
move the developer's HEAD, stage everything below it, and push it.

**And that failure hides itself, which is the argument for setting the variable in advance.** A
submodule moved under you leaves a detached HEAD with `git status` reporting **clean** about a tree
you did not ask for. The signals an ordinary check produces all say nothing happened, so the cost is
not a visible error but an afternoon spent working out where the work went.

**So set `JETS_NO_GIT_ACCESS=1` in the local environment file**, alongside `WORKSPACES_HOME`,
`WORKSPACE` and `WORKSPACE_BRANCH`. Truthy is `1`, `true`, `yes` or `on`, case-insensitive and
trimmed; anything else, including unset and including the empty string, leaves git integration on
(`jets/datatable/git/no_git_access.go`, which is the authority on the semantics). With it set, the
registry screen reports its status from the file system without shelling out to git, and the write
actions return a notice naming the variable instead of acting. The apiserver states at startup which
way the switch went, so a misread setting is visible in the first few lines of the log rather than in
its consequences.

**It is not a local-only workaround.** The same variable serves a site with no route to a
source-control host, which is the deployment it was added for: no `JETS_GIT_ACCESS`, no
`WORKSPACE_URI`, and a workspace baked into the image with no `.git` in it. Local development is the
second consumer and the one that reaches a developer's own files.

**And set `JETS_WORKSPACE_VALIDATOR` if the apiserver will compile a workspace** (added
2026-10-01). Compiling now validates the workspace's flow and table documents with a Node script
and fails rather than skips when the script or `node` cannot be found; the images put the script
beside the binaries, which a local build does not. Build it with `npm ci && npm run build:validator`
in `jetsclient_ide/` and point the variable at `jetsclient_ide/dist-validator/jets_validate_workspace.mjs`
([Workspaces](/docs/developer/workspace/) has the detail).

## Run the binaries you built, on a port of your own

**Set `JETS_BIN_DIR` to the directory of the binaries you built.** The apiserver runs three JetStore
programs by name and reads one support file: `update_db` (at start-up and when a workspace is
compiled or its client configuration loaded), `run_reports` and `local_test_driver` (the dev-mode
pipeline runs), and `workspace_schema.sql` (when a workspace is compiled). Each is looked for in
`JETS_BIN_DIR`, and in `/usr/local/bin` when the variable is unset or empty, which is where the images
install them (`utils.JetsBinPath`, `jets/utils/jetstore_bin.go`). Without it, a workstation runs
whatever happens to be installed in `/usr/local/bin`, built from some other checkout. The child
processes inherit the variable. `JETS_WORKSPACE_DB_SCHEMA_SCRIPT`, when set, still names the schema
script directly and wins over `JETS_BIN_DIR`.

```bash
mkdir -p $HOME/jetstore-bin
go build -o $HOME/jetstore-bin/ ./jets/apiserver ./jets/update_db ./jets/run_reports \
  ./jets/compute_pipes/local_test_driver
cp jets/workspace_schema.sql $HOME/jetstore-bin/
export JETS_BIN_DIR=$HOME/jetstore-bin
```

**Set `-listenAddr`, or `JETS_LISTEN_ADDR`, to run a second apiserver beside the first.** The address
is `host:port` or `:port`. Unset, it is `:8080` with `-usingSshTunnel` (plain HTTP) and `:8443`
without it (TLS), as before; the setting changes only the address, never whether TLS is used. The flag
wins over the variable.

```bash
$HOME/jetstore-bin/apiserver -usingSshTunnel -listenAddr 127.0.0.1:18080
```
