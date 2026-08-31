# jets/workspace — things that are invisible from the code in front of you

Following the habit `5f9b1bc8` started: package-level discoveries go here, not in
doc comments. API documentation belongs on the declarations.

## The workspace's directory permissions, and the lambda cold start they broke

**Symptom.** The native lambda — the image carrying the rule engine — fails on its
first initialisation, three log lines in:

```
Skipping sync of workspace.tgz and sqlite since workspace version … is same as local repo version
Seeding /tmp/workspaces/jets_ws from the image's /workspaces/jets_ws ...
{"errorMessage":"error while synching workspace files from db: while seeding
 /tmp/workspaces/jets_ws from /workspaces/jets_ws: open build: permission denied"}
```

**Cause.** `compileWorkspaceV2` created workspace directories `0770` and wrote the
files inside them `0644`. The files were world-readable; the directories they sat
in were not world-traversable. `modes.go` holds the constants and the invariant
that now guards it.

**`build` is not special.** `os.CopyFS` walks in lexical order and `build` is the
first directory the compiler makes. Every other one would have failed next.

### Why it took eleven months to appear

The mode dates to `e84a2af2` (2025-09-28). It is invisible for as long as the
process that *writes* the workspace is the process that *reads* it, because the
owner's permission bits are consulted first — which was every case there was.

`5f9b1bc8` (2026-08-22) changed that. It added `ensureLocalRepoSeeded`, which
copies the image's baked workspace into `WORKSPACES_HOME` on the native lambda's
cold start. That is the first read of these directories by an identity other than
the root that compiled them inside `compile_ws`, and it failed immediately.

**So the commit that introduced the exposure is not the commit that introduced the
defect**, and neither is where the error is reported
(`compile_workspace_utils.go`, in `ensureLocalRepoSeeded`). Three different
places, and only the middle one shows up in a log.

### Why only this image

| path | who copies the workspace | as whom |
|---|---|---|
| container | `cbooter`, before exec'ing the binary | the same identity, so the mode never mattered |
| zip lambdas | nothing — they carry no workspace | `WORKSPACES_REPO` unset, `ensureLocalRepoSeeded` no-ops |
| **native lambda** | `ensureLocalRepoSeeded`, at cold start | **not the compiler's identity — this is the one that broke** |

`Dockerfile.cpipes_native_lambda_ws` does `COPY --from=builder`, which preserves
the source modes, so the `0770` travels from `compile_ws` into the shipped image.

### Two creators disagreed, and one of them was right

`jets/workspace_assets/install.go` also creates directories under a workspace —
`table_configs/`, `user_flows/` — and has always used `0755` with `0644` files.
So two packages wrote adjacent directories in the same image under different
rules, and nothing compared them. If you add a third writer, match `modes.go`.

### What a test can and cannot do here

**A unit test cannot reproduce the failure.** The test process owns the
directories it creates and the owner's bits are consulted first, so `0770`
behaves exactly like `0755` for it. Reproducing this needs a second uid, which
`go test` does not have.

`seed_local_repo_test.go` shows the trap rather than avoiding it: its fixture
builds `build/` with `0755`, a mode production never used, so the test was green
against a permission the code did not write. **A fixture more permissive than
production is a test that cannot fail the way production does** — when you write
one, use the constant the code uses.

`TestWorkspaceModesAgree` guards the invariant instead of the symptom: it compares
the two modes to each other, which needs no second identity, and the comparison is
the thing that was wrong.
