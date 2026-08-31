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
in were not world-traversable. `utils.WorkspaceDirMode` holds the constants and
`TestWorkspaceModesAgree` the invariant that now guards them.

### The fix was wrong once, and the second cause is the more useful one

**The first fix passed `WorkspaceDirMode` to `os.MkdirAll` and the symptom did not
change.** `os.MkdirAll`'s mode applies only to directories it *creates*; given one
that already exists it returns nil and leaves the mode alone. Measured, because
this is the whole of the second failure:

```
existing  after MkdirAll(0755): -rwx------
fresh     after MkdirAll(0755): -rwxr-xr-x
```

**And `build/` already existed.** It arrives through `COPY . $WORKSPACES_REPO/$WORKSPACE/`
in `Dockerfile.compile_ws` — it is in the client's repository — so the compiler
never created it and never re-moded it. The shipped image says so plainly: `build`
dated **2026-08-27** and `build/jet_rules` dated **2025-07-05**, both `drwxrwx---`,
holding `classes.json` written `-rw-r--r--` at **2026-08-31 14:59** by that day's
compile. `table_configs/`, which the asset installer *did* create that morning, is
`drwxr-xr-x`. Same image, same run: **the mode a directory ends up with depends
only on whether it had to be created.**

Three directories in the whole tree, and zero files:

```
rwxrwx---  /workspaces/jets_ws/build
rwxrwx---  /workspaces/jets_ws/build/jet_rules
rwxrwx---  /workspaces/jets_ws/build/jet_rules/clinical_intel
```

`utils.EnsureWorkspaceDir` asserts the mode instead of requesting it, and
**unlike the original defect this one is unit-testable** — whether `MkdirAll`
re-modes an existing directory is a question the owning process can answer, so
`TestEnsureWorkspaceDirFixesAnExistingDirectory` is a real regression test rather
than an invariant standing in for one. It was confirmed to fail against the first
fix before being kept.

### Reproducing it without AWS

The failure is one `docker run` away from any machine holding the image, which is
worth knowing before the next three-hour rebuild-and-deploy cycle:

```bash
docker run --rm --entrypoint /bin/ls cpipes_lambda_jets_ws:latest /workspaces/jets_ws/build
docker run --rm --user 1000:1000 --entrypoint /bin/ls cpipes_lambda_jets_ws:latest /workspaces/jets_ws/build
```

The first lists four entries; the second is `Permission denied`. That pair also
settles what no documentation states outright — **the Lambda handler is not
effectively root**, since nothing else makes `EACCES` reachable on a
`drwxrwx---` directory.

### The workspace is a client artifact, so Go cannot be the whole answer

The compiler can assert modes on directories it creates. It has no say over
`lookups/`, `jet_rules/` or anything else a client's checkout carried, and a
client shipping one directory at 0700 reproduces this exactly. So
`Dockerfile.compile_ws` runs `chmod -R a+rX` over the workspace **twice, and
neither is redundant**: once immediately after the `COPY`, which normalises what
the client shipped *before* `workspace.tgz` is built from it — a tarball preserves
modes, so normalising only at the end would ship a correct tree beside an archive
carrying the bad one — and once after the compile, covering whatever the two build
steps created. All three `*_ws` images (`cpipes_ws`, `ui_service_ws`,
`cpipes_native_lambda_ws`) `COPY --from` `compile_ws`, so that one place covers
every baked workspace.

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

| path | who copies the workspace | why the mode does not bite |
|---|---|---|
| ECS tasks | `cbooter`: `cp -r` **as root**, then `chown -hR 999:999 $JETS_TEMP_DATA`, then exec **as jsuser** | jsuser ends up **owning** the copy, and the owner bits of 0770 are `rwx` |
| zip lambdas | nothing — they carry no workspace | `WORKSPACES_REPO` is unset, so `ensureLocalRepoSeeded` no-ops on its first guard |
| **native lambda** | `ensureLocalRepoSeeded`, in-process, at cold start | **nothing rewrites ownership — this is the one that broke** |

**~~The container is safe because the same identity copies and reads.~~ It is not,
and the difference matters.** Three identities are involved: root copies, root
chowns to 999:999, and jsuser reads. The mode never mattered because **ownership
was rewritten**, not because the writer and the reader are the same.

That distinction is the whole reason this section exists rather than a one-line
"containers are fine". `makeJetsdataWritable` is named and commented for a
different purpose — *"the mounted volume may have root ownership and jsuser needs
write access"* — so **ECS's immunity to this defect is a side effect of a call
made for writability.** Anyone who removes or narrows that chown, on the entirely
reasonable ground that the volume is already writable, breaks every ECS task the
way the lambda broke, and nothing in cbooter says so.

`WORKSPACES_HOME` is `JETS_TEMP_DATA + "/workspaces"` in the CDK
(`build_ecs_tasks.go`), derived rather than configured independently, so the
chown provably covers it. If that ever becomes two separate settings, the chown
can miss the workspace and this returns.

`Dockerfile.cpipes_native_lambda_ws` does `COPY --from=builder`, which preserves
the source modes, so the `0770` travels from `compile_ws` into the shipped image.

### Three writers disagreed, and the constants moved because of it

| package | what it writes | dirs (before) | files |
|---|---|---|---|
| `jets/workspace` | compile output | **0770** | 0644 |
| `jets/awsi` | files fetched from S3 | **0750** | 0644 |
| `jets/workspace_assets` | installed assets (`table_configs/`, `user_flows/`) | 0755 | 0644 |

Three packages wrote adjacent directories in one tree under three different
rules, and nothing compared them. **Only the first cost an incident, and fixing
only the first would have left the identical trap one package over** — latent for
exactly the reason this one was latent for eleven months, which is that the
process fetching is the process reading.

So the constants live in `jets/utils`, which `jets/workspace` and `jets/awsi` both
already import and which neither imports from. `workspace_assets` is left alone:
it was already correct, and rewriting a correct call site to name a constant is
churn. If it grows a second mode, move it too.

**If you add a fourth writer, use `utils.WorkspaceDirMode`.**

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

## What the seed copies, and why it is less than the workspace

**The seed exists to stand in for a fetch, so it should deliver what that fetch
would have delivered.** It did not: `os.CopyFS` copies a directory, and the
directory is the client's whole repository.

`SyncWorkspaceFiles` already declares what each runtime needs, through its
`contentType` argument:

| caller | fetches | which contains |
|---|---|---|
| `SyncComputePipesWorkspace` | `workspace.tgz`, `sqlite` | `workspace_control.json`, `build/**`, `pipes_config/**`, `lookup.db`, `workspace.db` |
| `SyncRunReportsWorkspace` | `reports.tgz` | the report definitions |

**Neither carries `.git`, and neither carries `lookups/`** — those CSVs are the
*source* that the compile turns into the `lookup.db` that `sqlite` delivers. So a
compute-pipes run that took the fetch never had them, and a seeded one having
them was the anomaly rather than the saving.

Measured on the native lambda's cold start of 2026-08-31: **113.5 MB over 222
files in 24.4s**, of which

| | MB | files |
|---|---:|---:|
| `lookup.db` and the other root files | 59.0 | 7 |
| `lookups/` — the CSV source | 46.2 | 9 |
| `.git` | 6.9 | 83 |
| everything else | 1.4 | 123 |

so the two exclusions are **53 MB, 47%**.

**The list belongs to the caller, and `run_reports` is why** — for a stronger
reason than "it reads them". `run_reports` syncs the lookups to S3 (`:287`) and
then **recompiles the workspace** unless `SkipCompileWorkspace` is set (`:293`),
and the compile is what consumes the CSVs: `PackageLookupTablesToSqlite` opens
each one (`lookup_tables.go:87`) to rebuild `lookup.db`. **A report that updates
a lookup table needs the source present**, not merely copied onward. So a list
baked into the seed would be right for one caller and wrong for the other — the
same reason `contentType` is a parameter.

**That also settles whether the compute-pipes exclusion is safe.** The compile is
the only thing that reads the CSVs, and its three callers are the apiserver
(`datatable/workspace_helper_functions.go:92`, `apiserver/server.go`),
`run_reports` (`:295`), and the build-time CLI (`cmds/compile_workspace`).
**`jets/compute_pipes` contains no call to `CompileWorkspace` at all**, so nothing
on the path that passes the exclusions can ever need them. That is a demonstrated
property rather than an assumption about what gets read.

**The default is to copy.** A new top-level entry is included unless somebody
names it. An unnecessary copy costs seconds; a missing one breaks a pipeline, and
the failure would look exactly like the two that preceded it.

**The images are untouched.** `.git` and `lookups/` are still in every `*_ws`
image, because the UI service needs both — it compiles the workspace locally and
syncs it with git. What changed is only what the lambda copies into `/tmp`.

Byte counts for both sides are logged, so a skip that stops paying says so:

```
Seeded /tmp/workspaces/jets_ws in 12.1s (60.4 MB copied, 53.1 MB skipped: [.git lookups])
```

