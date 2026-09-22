# The run manifest

**A published contract.** This document is read by parties outside this repository, who cannot
check what produced it. Anything here that changes is a breaking change for them, which is why
the document carries a version and why this file states what is guaranteed and what is not.

Added 2026-09-21.

---

## 1. What it is

One JSON document per pipeline run, describing **what the run's configuration declared as
deliverables, and what the run wrote for each of them**.

Its subject is the *declaration* — the compute pipes document's own `output_tables` and
`output_files` — and never the compute graph. An edge of the graph is how the engine got there;
a deliverable is what the author asked for. A `jets_partition` sink is a shard, and a shard is
not a deliverable, so it is not in the manifest.

**It is whole or it is absent.** A manifest describes every declared deliverable of the whole
run, across every step and every worker. There is no partial manifest and no per-step manifest.

**It exists only for a run that completed.** `StatusUpdate.CoordinateWork` computes a five-way
terminal status — `completed`, `recovered`, `errors`, `interrupted`, `failed` — and writes the
manifest on the first of those alone. `recovered` in particular leaves none: a recovered run is
one where a worker failed and the state machine took the success path anyway, which is exactly
the half-written prefix the manifest exists to make detectable.

**So an absent manifest means an incomplete run** — with one caveat stated plainly in §6.

## 2. Where it is

Two stores, both written at the same moment, carrying byte-identical documents.

| Store | Where |
|---|---|
| S3 | `<JETS_s3_STAGE_PREFIX>/process_name=<process>/session_id=<session>/run_manifest.json` |
| Database | `jetsapi.cpipes_execution_status.run_manifest_json`, keyed by `session_id` |

The object key is run-scoped: it carries no `step_id=` and no `jets_partition=` component, which
is what makes the document one per run rather than one per slice. The bucket is the deployment's
own (`JETS_BUCKET`).

## 3. The document

```json
{
  "schema": "jetstore.cpipes.run_manifest/v1",
  "session_id": "20260921T120000",
  "process_name": "healthcare_corpus",
  "pipeline_execution_key": 412,
  "status": "completed",
  "written_at": "2026-09-21T12:04:11.238Z",
  "entries": [
    {
      "channel": "claims_out",
      "declared_in": "output_tables",
      "declared_name": "jetsapi.claims",
      "written": true,
      "location": "sql://jetsapi.claims",
      "output_type": "db_table",
      "records_count": 1300,
      "parts_count": null,
      "sinks_count": 12
    },
    {
      "channel": "export",
      "declared_in": "output_files",
      "declared_name": "export.csv",
      "written": true,
      "location": "s3://acme-out/exports/export.csv",
      "output_type": "output_file",
      "records_count": null,
      "parts_count": 1,
      "sinks_count": 1
    }
  ]
}
```

### 3.1 Top level

| Field | Meaning |
|---|---|
| `schema` | The document's kind and version. **Pin it.** A second document named `run_manifest.json`, produced by a different tool for a different purpose, can sit in the same bucket; this field is how they are told apart, and a shape change bumps the version rather than moving a field silently |
| `session_id` | The run's session id, which is the key both stores agree on |
| `process_name` | `pipeline_execution_status.process_name` |
| `pipeline_execution_key` | `pipeline_execution_status.key` |
| `status` | The run's terminal status. It is `completed` in every manifest that exists; it is written down so the document says what it is rather than leaving a consumer to infer it from the document's own existence |
| `written_at` | When the manifest was produced, UTC, RFC 3339 |
| `entries` | One per declared deliverable. Order follows the document: `output_tables` then `output_files` |

### 3.2 An entry

**One shape, and `location` is the discriminator.** A `sql://` entry is a database deliverable
and carries a row count and no parts; an `s3://` entry is a file one and carries both. The
alternative — two arrays mirroring `output_tables` and `output_files` — would make every consumer
handle two shapes to learn one thing, and the authored split is recoverable from `declared_in`.

| Field | Meaning |
|---|---|
| `channel` | The declared key: `output_tables[].key` or the `output_files[].key`. **This is the identity.** It is what the rest of the configuration refers to the output by, and it is what the two halves of the manifest are joined on |
| `declared_in` | `output_tables` or `output_files` — which array declared it |
| `declared_name` | The name the document gave it. **Not the identity**: most `output_tables` entries in the rule corpus have a `key` differing from the table `name`, and this field is the document's text, so it may still carry an unexpanded `${…}` reference. `location` is the resolved one |
| `written` | Whether the run wrote anything for this declaration |
| `location` | Where it went, as a URI with environment variables already substituted. `sql://<schema>.<table>` or `s3://<bucket>/<key>`. Absent when `written` is false |
| `output_type` | The engine's sink kind: `db_table` or `output_file`. Redundant with `location`'s scheme by construction, and carried so that a consumer reading one need not parse the other |
| `records_count` | Rows written, summed over every worker of every step — **or `null`, which is not `0`**. See §4 |
| `parts_count` | File parts written. `null` for a `sql://` entry, which has no parts |
| `sinks_count` | How many sink instances the run folded into this deliverable. A splitter writing one output channel into many partitions reports one each |
| `error_message` | What the writers reported for this deliverable, if anything. A completed run can still have carried an error on a channel |
| `note` | The manifest's own reservation about the entry, in prose. Empty is the ordinary case |

## 4. `records_count: null` is not zero

`null` means **no row count exists for this deliverable**, not that no rows were written. The
`merge_files` multipart-copy path moves bytes and never parses a record, so the engine records a
NULL row count for it deliberately rather than inventing a `0`.

A consumer that reads `null` as `0` is reading a measurement nobody made. A consumer that wants a
total must either skip those entries or say that the total is a lower bound.

**The rule is applied per deliverable and it is one-directional**: one sink that cannot count
makes the whole entry `null`, because summing a number with a non-number gives a number that
means nothing.

## 5. The bucket in a `location` is a resolved one

A configured bucket name that still carries a `${…}` reference is not a bucket name — it is a
variable the environment did not supply. **A manifest that recorded one would name a location no
consumer can open, in the one artefact whose job is to be believed by somebody who cannot
check.** So the producer refuses: the manifest is not written at all, and the refusal is logged.

The one weaker case is noted rather than refused. A location naming the deployment's own bucket
by the `jetstore_bucket` sentinel is a legal configuration value — every writer in the engine
resolves it before recording, so a manifest carrying it is a surprise rather than a failure — but
it is not resolvable outside the deployment, and the entry's `note` says so.

## 6. What an absent manifest means, exactly

**A run that completed and left no manifest is possible, and it is the one thing a consumer
cannot distinguish.**

The manifest write is *additive*: it is observability, and a pipeline that ran correctly is not
reported failed because an observability write did not land. So a failure to build or store the
manifest is logged and the run still completes.

That is the same posture the per-channel detail rows take — and with one difference worth naming.
A missing detail row is detectable downstream by arithmetic (`sum(child) != parent`). **A missing
manifest is indistinguishable from a run that never completed.** There is no second signal.

So the log line is the whole of the signal, and it says so in those words. Grep the Status Update
Lambda's log group for `NO RUN MANIFEST` when a run reports `completed` in
`jetsapi.pipeline_execution_status` and no manifest is found.

## 7. Where it is produced

`StatusUpdate.CoordinateWork` (`jets/datatable/status_update.go`), immediately after the switch
that computes the run's terminal status, and gated on it.

**That placement is the whole of the guard.** The Status Update Lambda is invoked on the error
path as well as the success path — `runErrorStatusLambdaTask` and `runSuccessStatusLambdaTask`
are the same Lambda object — so a producer that wrote before branching would write a manifest for
a run that failed, which is the one outcome the manifest exists to make impossible.

The declared half is read from `cpipes_execution_status.cpipes_startup_json`, which carries the
whole document. It is deliberately **not** read from `cpipes_config_json`, which holds at most one
step's configuration — the sharding step's, overwritten by each reducing step — and whose
`output_tables` is therefore the last step to have started rather than the run's.

The observed half is one `GROUP BY` over `jetsapi.pipeline_execution_channel_details` for the
run's `session_id`, grouped by `(output_type, output_channel)`. That table carries `session_id`
on the row and is written by every worker of every step, in both the Go and the Python engine, so
one query covers the run and a mixed-engine pipeline is described truthfully.
