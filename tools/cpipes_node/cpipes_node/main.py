"""The local driver: a node run without AWS, and the scope printed.

**Why a driver at all.** Four of the phase's seven exit criteria — X2, X3, X5
and X7 — compare corpora, and a comparison needs something executable to
produce them. There is no docker-compose in this repository and no local
JetStore; the alternative to this is a deployment per measurement. So the S3
access is behind one seam (`store.py`) with a directory implementation, the
config behind another (`config.py`) with a file implementation, and the local
run is **the same `coordinate` with different arguments** — not a second
engine, which is the distinction that keeps the local result evidence about the
deployed one.

**What a local run does not cover**, said plainly so that nobody reads X1's
*end to end* as met by it: no lambda invocation and so no `{id, jp, pe}` event
crossing a process boundary, no `cpipes_execution_status` read and so no proof
the query is right, no S3 and so no proof of the stage layout or the KMS
settings, no state machine and so no proof that a Map step fans out to the
partitions the pipeline declares, and none of the six side-effect tables
(P9-T09). A local run is evidence about the *pipeline*; X1 asks about the
*integration*, and the reading is Michel's.

Three subcommands:

    cpipes-node scope                 print the declared scope
    cpipes-node check --config FILE   run the scope gate alone
    cpipes-node run --config FILE ... run the node against a local store

`check` is the one to hang X6's evidence on: exit 0 clean, exit 1 out of scope,
exit 2 declared and not implemented. Two exit codes rather than one, for the
reason `errors.py` gives.

**A local merge needs the four S3-area variables in the environment.**
`JETS_s3_STAGE_PREFIX` and its three siblings are read the way `awsi.init()`
reads them, because the keys a merge lists and writes are built from them — so a
local run against a directory has to state the same layout the deployment
states. They are not flags: four flags here would be a second spelling of a
deployment's own configuration, and a local run whose layout differed from the
deployed one would be evidence about nothing.

**`run` registers no site operators** — `site.EMPTY` — and every built-in
transformation is still owed, so as it stands it can refuse a document and cannot
complete one. That is deliberate rather than an oversight: a registry is a
deployment's, and a flag naming an importable factory would make this package
able to load a customer's code, which is the one thing its own docstring says it
must never learn to do. **A driver that runs the corpus pipeline composes
`coordinate` with its own registry**, the way `awslambda.Node` does; that driver
is P9-T19's and this subcommand is what it copies.
"""

from __future__ import annotations

import argparse
import sys
from pathlib import Path

from .args import NodeArgs
from .config import FileConfigSource, check_scope, parse_config
from .errors import NodeError
from .scope import declarations, declared_scope
from .site import EMPTY
from .store import Local

EXIT_OK = 0
EXIT_OUT_OF_SCOPE = 1
EXIT_NOT_IMPLEMENTED = 2
EXIT_REFUSED = 3


def render_declared_scope() -> str:
    """The scope as a person reads it. Sorted; no run-dependent values."""
    lines = ["The operator scope this node declares:", ""]
    for kind, tokens in declared_scope().items():
        lines.append(f"  {kind}:")
        for token in tokens:
            cls = next(c for c in declarations() if c.kind == kind and c.token == token)
            state = "" if cls.implemented() else f"  [owed by {cls.owed_by}]"
            lines.append(f"    {token:<18}{cls.summary}{state}")
        lines.append("")
    lines.append("  transformation, by registration:")
    lines.append(
        "    any token a deployment registers with "
        "cpipes_node.site.Registry.with_operators"
    )
    lines.append("")
    lines.append(
        "Anything else aborts at startup naming the token. Whether the subset "
        "is permanent is P9-I04 / D-206."
    )
    return "\n".join(lines)


def render_run_result(result) -> str:  # type: ignore[no-untyped-def]
    """What one node's run did, **per channel rather than as a total**.

    A total is satisfied by the right number of rows in the wrong channels, which
    is exactly the failure a twelve-output step can have, so the per-channel
    figures are the ones printed and a reader adding them up is doing so on
    purpose.
    """
    lines = [
        f"source rows: {result.source_rows}",
        (
            f"channels: {len(result.channel_rows)}, "
            f"closed: {len(result.closed_channels)}, "
            f"rows written: {result.total_rows()}"
        ),
    ]
    for name, rows in result.channel_rows.items():
        closed = "" if name in result.closed_channels else "  [NOT CLOSED]"
        lines.append(f"  {name}: {rows}{closed}")
    for index, token in result.skipped:
        lines.append(f"  pipe {index}: '{token}' skipped, its `when` was false")
    if result.conditional_overrides:
        lines.append(
            f"conditional_config applied {result.conditional_overrides} override(s): "
            "this document had not been through a starter"
        )
    return "\n".join(lines)


def render_merge_result(result) -> str:  # type: ignore[no-untyped-def]
    """What one merge did. **No row count, and that is the point.**

    A merge parses no record, so there is no number to print and `0` would read
    as a collapse. What it prints instead is the part count, the destination and
    which arm of the header switch ran — the last being what a merged file with a
    header line in the middle is diagnosed by, and the one thing no downstream
    reader can tell you.
    """
    lines = [
        f"merged {len(result.input_keys)} part file(s) -> {result.output_location}",
        f"bytes written: {result.bytes_written}",
        "row count: not a measurement (the merge moves bytes and parses no record)",
    ]
    if result.header_plan is not None:
        lines.append(
            f"headers: write={result.header_plan.write_headers} "
            f"skip_input={result.header_plan.skip_input_headers} "
            f"({result.header_plan.reason})"
        )
    for key in result.input_keys:
        lines.append(f"  {key}")
    return "\n".join(lines)


def render(result) -> str:  # type: ignore[no-untyped-def]
    """Whichever of the two results `coordinate` returned.

    Dispatched on the type and not on a field, because the two records differ in
    what they can say rather than in one value: a graph run has per-channel row
    counts and a merge has none, and a renderer that fell back to zeros would
    print an edge nothing crossed where there is no edge at all.
    """
    from .merge import MergeResult

    if isinstance(result, MergeResult):
        return render_merge_result(result)
    return render_run_result(result)


def _report(report) -> int:  # type: ignore[no-untyped-def]
    for finding in report.out_of_scope:
        print(f"out of scope: {finding}", file=sys.stderr)
    for finding in report.unimplemented:
        print(f"not implemented: {finding}", file=sys.stderr)
    print(
        f"{len(report.accepted)} token(s) examined and accepted, "
        f"{len(report.out_of_scope)} out of scope, "
        f"{len(report.unimplemented)} declared and not built."
    )
    if report.out_of_scope:
        return EXIT_OUT_OF_SCOPE
    if report.unimplemented:
        return EXIT_NOT_IMPLEMENTED
    return EXIT_OK


def main(argv: list[str] | None = None) -> int:
    parser = argparse.ArgumentParser(prog="cpipes-node", description=__doc__)
    sub = parser.add_subparsers(dest="command", required=True)

    sub.add_parser("scope", help="print the declared operator scope")

    check = sub.add_parser("check", help="run the scope gate over a .pc.json")
    check.add_argument("--config", type=Path, required=True)

    run = sub.add_parser("run", help="run the node against a local store")
    run.add_argument("--config", type=Path, required=True)
    run.add_argument("--id", type=int, default=0, help="node id ($SHARD_ID)")
    run.add_argument("--jp", default="", help="jets partition label; derived if unset")
    run.add_argument("--pe", type=int, default=0, help="pipeline execution key")
    run.add_argument(
        "--store", type=Path, required=True, help="directory standing in for the bucket"
    )

    args = parser.parse_args(argv)

    if args.command == "scope":
        print(render_declared_scope())
        return EXIT_OK

    try:
        if args.command == "check":
            # Through the source rather than `read_text`, so a missing file is
            # the node's own refusal and not a traceback.
            config = parse_config(FileConfigSource(args.config).config_json(0))
            return _report(check_scope(config, site_operators=EMPTY))

        from .node import coordinate

        result = coordinate(
            NodeArgs(id=args.id, jp=args.jp, pe=args.pe),
            FileConfigSource(args.config),
            store=Local(args.store),
            site_operators=EMPTY,
        )
        print(render(result))
    except NodeError as exc:
        print(f"{type(exc).__name__}: {exc}", file=sys.stderr)
        return EXIT_REFUSED
    return EXIT_OK


if __name__ == "__main__":  # pragma: no cover
    raise SystemExit(main())
