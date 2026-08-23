# JetStore-owned workspace assets — pipeline configurations

This directory is the single home for Compute Pipes configurations that JetStore
owns and installs into client workspaces at image-build time
(`Dockerfile.compile_ws` runs `install_workspace_assets` into
`$WORKSPACES_REPO/$WORKSPACE/pipes_config/` before `compile_workspace`). It is
the sibling of `../data_model/`, installed by the same step under the same
guard; that directory's README carries the parts of the mechanism common to
both, and this one carries only what differs.

## What is installed, and how

Every `.json` in this directory — so every `.pc.json` — is installed into every
client workspace: the set is the directory's contents, embedded into the
installer binary by `jets/workspace_assets/install.go` (this README is
excluded).

| Asset | What it is | Registered as |
|---|---|---|
| `jets_loader.pc.json` | The platform loader: reads a client's input file and copies it into the staging table. | `Jets_Loader`, in `jets/jets_init_db.sql` |
| `embed_input_parts.pc.json` | The embeddings pipeline: reads a csv of parts, calls the infer server once per row, writes the vector back onto the row. | `Embed_Input_Parts`, same file |

**A `.pc.json` here is inert until a `process_config` row names it.** That row is
JetStore's too and lives in `jets/jets_init_db.sql`, which the apiserver loads at
deployment — so a configuration added here needs a row added there in the same
change, and neither half does anything alone.

`jets_loader.pc.json` — the platform loader — used to exist as one committed
copy per workspace, and **two of the four had diverged**. This README first said
they had not; that was measured against working trees which had already been
normalised by hand, rather than against what the workspaces commit, and it was
wrong. Corrected 2026-08-22:

| Workspace | Committed parquet sharding tier | Adopting it changed |
|---|---|---|
| `cedargate_ws` | 100 / 100 / 110 | nothing |
| `usi_ws` | 100 / 100 / 110 | nothing |
| `jets_ws` | 100 / 100 / **120** | `shard_max_size_mb`, set by "adjusted sharding" |
| `walrus_ws` | **50 / 50 / 55** | the whole tier, set by "Adjust parquet shard sizing" |

(`when_total_size_ge_mb` / `shard_size_mb` / `shard_max_size_mb`.)

**Both divergences were deliberate commits**, so adopting the JetStore version
reverses two pieces of tuning. That is the cost of consolidating a file that is
already in use, and it is what the guard below exists to make visible: had the
install run before those two workspaces were normalised by hand, it would have
refused them by name rather than overwriting them.

**The lesson is not "consolidate early while it is free".** A file that four
teams can edit will have been edited; the question is only whether you find out
from a build failure or from a diff. Check what the workspaces *commit* — `git
show HEAD:<path>` — because a working tree can already have been changed by the
very consolidation you are trying to assess.

## The guard, and the one way it differs from `data_model`

The three states an existing file can be in — identical, stale, or edited — are
`../data_model/README.md`'s, unchanged. What differs is the **evidence** the
install has to tell them apart.

A `.jr` file carries a `jetstore-owned-asset` header, so a file at a reserved
path with no header is known never to have been installed by JetStore. **A
`.pc.json` carries no such header, deliberately**: it is machine-read
configuration, read by the pipeline builder and by the Workspace IDE and written
by config authors and by generators, and a token planted in it would be one more
field to reproduce or lose. So in this directory `pipes_config/jets_assets_manifest.json`
is the *whole* of the evidence, and the install says so rather than claiming a
file is not JetStore's on the strength of a header JetStore never writes.

The practical consequence is one lost distinction: a workspace whose
`jets_loader.pc.json` differs and has no manifest entry gets

> a JetStore-owned file differs from the one being installed, and no
> `jets_assets_manifest.json` entry says what the last install left here

which is true whether the file was edited or predates the mechanism. Adopt with
`install_workspace_assets -d <workspace> -force` once, and commit the result.

## Running a variant

A JetStore-owned configuration is replaced on every build, so editing one in
place is lost work. To run a variant, copy it under a name of your own and
register *that* file in your workspace's own `process_config` row:

```bash
cp pipes_config/jets_loader.pc.json pipes_config/<your_workspace>_loader.pc.json
```

Unlike a `.jr` extension, which imports the JetStore file and therefore tracks
it, a copied `.pc.json` is a fork: it does not follow the original. That is the
trade, and it is why the platform configurations are kept small.
