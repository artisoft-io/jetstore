---
title: "Workspace validation fails when it cannot run"
description: "Why a workspace compile fails, rather than skips the check, when the validator for its flow and table documents cannot run."
kind: explanation
---

*Decided 2026-10-01.*

## Context

A workspace's user-flow and table documents (`user_flows/`, `table_configs/`) are checked in two
halves. Each document is checked on its own in Go (`jets/userflow`, on every save). The checks that
span documents, a flow's forms and actions resolving against each other and every escape against the
build's registry, exist only in TypeScript, in `jetsclient_ide/src/userflow/`. These documents are
expected to be written by a model, so the validator is the whole of their quality control.

## Decision

Bundle the TypeScript checks into one Node script, `jets_validate_workspace.mjs`, and run it at the
start of every workspace compile (`validateWorkspaceAssets`, `jets/workspace/asset_validation.go`).
**When it cannot run, because Node or the script is missing, the compile fails.** The only way past it
is `SkipAssetValidation`, which a caller must give with a reason; an empty reason fails the compile.

## Consequences

- Every caller of `CompileWorkspace` passes the same gate: the apiserver at start, the IDE's compile
  action and the image build's `compile_workspace`.
- The images carry Node and the script beside the binaries that compile. A workstation sets
  `JETS_NODE` and `JETS_WORKSPACE_VALIDATOR` (see
  [Compiling a workspace](/docs/developer/workspace/compiling/#compiling-validates-the-flow-documents-first-and-needs-node-to-do-it)).
- `run_reports` is the one caller that skips, with its reason logged: it recompiles to rebuild lookup
  tables, changes no flow document, and runs in an image without Node.
- The script's exit status is the contract: 0 passes, 1 is an invalid document, anything else is a
  validator that did not finish.

## Where the reason is recorded

- The file comment of `jets/workspace/asset_validation.go`: *It fails when it cannot run. It never
  skips itself.*
- Commit `2b9b47cb`, *workspace: CompileWorkspace validates flow documents first; run_reports opts out*.
- [Compiling a workspace](/docs/developer/workspace/compiling/).
