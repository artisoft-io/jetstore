"""One command with an exit code: there is no CI service to host it."""

from __future__ import annotations

import argparse
import sys
from pathlib import Path

from . import corpus as corpus_mod
from . import harness as harness_mod
from .check import check, check_citations, check_exemplars, unfilled_report
from .matrix_schema import Matrix

DEFAULT_MATRIX = Path(__file__).resolve().parent.parent / "matrix"


def main(argv: list[str] | None = None) -> int:
    parser = argparse.ArgumentParser(
        prog="cpipes-contract",
        description="Checks over the cpipes applicability matrix.",
    )
    sub = parser.add_subparsers(dest="command", required=True)

    check_cmd = sub.add_parser("check", help="check the matrix against its schema")
    check_cmd.add_argument(
        "--matrix", type=Path, default=DEFAULT_MATRIX, help="the matrix directory"
    )
    check_cmd.add_argument(
        "--strict",
        action="store_true",
        help="add the checks that hold only once extraction is complete",
    )
    check_cmd.add_argument(
        "--corpus",
        type=Path,
        default=None,
        help="root the exemplar paths resolve against, eg the repo holding workspaces/",
    )
    check_cmd.add_argument(
        "--code",
        type=Path,
        default=None,
        help="root the evidence_ref citations resolve against, ie the JetStore repo",
    )

    corpus_cmd = sub.add_parser(
        "corpus", help="measure the matrix against the live corpus"
    )
    corpus_cmd.add_argument("--matrix", type=Path, default=DEFAULT_MATRIX)
    corpus_cmd.add_argument(
        "--corpus",
        type=Path,
        required=True,
        help="root holding workspaces/; only workspaces/*/pipes_config/** is counted",
    )
    corpus_cmd.add_argument(
        "--apply",
        action="store_true",
        help="write the measured counts and live exemplars back onto the rows",
    )
    corpus_cmd.add_argument(
        "--unknown",
        action="store_true",
        help="also list keys seen in the corpus that no field row accounts for",
    )

    harness_cmd = sub.add_parser(
        "harness",
        help="synthesize a minimal config per type, run it through "
        "ValidatePipeSpecConfig, and turn every row into a test result",
    )
    harness_cmd.add_argument("--matrix", type=Path, default=DEFAULT_MATRIX)
    harness_cmd.add_argument(
        "--code",
        type=Path,
        required=True,
        help="the JetStore repo root; the Go runner is `go run` from there",
    )
    harness_cmd.add_argument(
        "--apply",
        action="store_true",
        help="write the results back onto the harness column of the rows",
    )
    harness_cmd.add_argument(
        "--dump",
        type=Path,
        default=None,
        help="also write each synthesized config to this directory, for reading",
    )

    drift_cmd = sub.add_parser(
        "drift",
        help="field-inventory drift check between pipes_model.go and the "
        "matrix (B.18); exit code carries the verdict",
    )
    drift_cmd.add_argument("--matrix", type=Path, default=DEFAULT_MATRIX)
    drift_cmd.add_argument(
        "--code", type=Path, required=True, help="the JetStore repo root"
    )

    gofile_cmd = sub.add_parser(
        "gofile",
        help="emit the matrix as one generated Go data file (B.17)",
    )
    gofile_cmd.add_argument("--matrix", type=Path, default=DEFAULT_MATRIX)
    gofile_cmd.add_argument(
        "--out",
        type=Path,
        default=DEFAULT_MATRIX.parent.parent.parent
        / "jets"
        / "compute_pipes"
        / "cpipes_contract_data.go",
        help="destination of the generated Go file",
    )

    stamp_cmd = sub.add_parser(
        "stamp",
        help="certify reviewed rows: write the fingerprint a reviewed mark "
        "certifies, and clear leftovers on unreviewed rows",
    )
    generate_cmd = sub.add_parser(
        "generate",
        help="generate cpipes_model.py (Pydantic v2) from the reviewed matrix",
    )
    generate_cmd.add_argument("--matrix", type=Path, default=DEFAULT_MATRIX)
    generate_cmd.add_argument(
        "--out",
        type=Path,
        default=DEFAULT_MATRIX.parent / "cpipes_model.py",
        help="destination of the generated model",
    )

    stamp_cmd.add_argument("--matrix", type=Path, default=DEFAULT_MATRIX)
    stamp_cmd.add_argument(
        "--restamp",
        action="store_true",
        help="also re-certify reviewed rows whose stamp no longer matches - "
        "an explicit re-approval of the changed row, never the default",
    )

    reflect_cmd = sub.add_parser(
        "reflect",
        help="regenerate the matrix's claim columns from cpipes_model.py (B.10)",
    )
    reflect_cmd.add_argument("--matrix", type=Path, default=DEFAULT_MATRIX)
    reflect_cmd.add_argument(
        "--model",
        type=Path,
        default=DEFAULT_MATRIX.parent / "cpipes_model.py",
        help="the model to reflect",
    )
    reflect_cmd.add_argument(
        "--check",
        action="store_true",
        help="compare instead of writing; exit nonzero on divergence",
    )

    schema_cmd = sub.add_parser(
        "schema",
        help="emit the JSON Schema with every type addressable in $defs (B.11)",
    )
    schema_cmd.add_argument("--matrix", type=Path, default=DEFAULT_MATRIX)
    schema_cmd.add_argument(
        "--model", type=Path, default=DEFAULT_MATRIX.parent / "cpipes_model.py"
    )
    schema_cmd.add_argument(
        "--out", type=Path, default=DEFAULT_MATRIX.parent / "cpipes_schema.json"
    )

    fragments_cmd = sub.add_parser(
        "fragments",
        help="extract the fragment library from the live corpus, keyed by defs_name (F.1)",
    )
    fragments_cmd.add_argument("--matrix", type=Path, default=DEFAULT_MATRIX)
    fragments_cmd.add_argument(
        "--schema", type=Path, default=DEFAULT_MATRIX.parent / "cpipes_schema.json"
    )
    fragments_cmd.add_argument(
        "--model", type=Path, default=DEFAULT_MATRIX.parent / "cpipes_model.py"
    )
    fragments_cmd.add_argument(
        "--corpus", type=Path, default=DEFAULT_MATRIX.parent.parent.parent.parent
    )
    fragments_cmd.add_argument(
        "--out", type=Path, default=DEFAULT_MATRIX.parent / "fragments" / "library.jsonl"
    )
    fragments_cmd.add_argument(
        "--curated-out",
        type=Path,
        default=DEFAULT_MATRIX.parent / "fragments" / "library.curated.jsonl",
        help="the F.2 curation: decision 14's rules applied to the extraction",
    )

    bundles_cmd = sub.add_parser(
        "bundles",
        help="validate every live TransformationSpec fragment against its bundle",
    )
    bundles_cmd.add_argument("--matrix", type=Path, default=DEFAULT_MATRIX)
    bundles_cmd.add_argument(
        "--schema", type=Path, default=DEFAULT_MATRIX.parent / "cpipes_schema.json"
    )
    bundles_cmd.add_argument(
        "--corpus",
        type=Path,
        default=DEFAULT_MATRIX.parent.parent.parent.parent,
        help="root holding workspaces/",
    )

    validate_cmd = sub.add_parser(
        "validate",
        help="validate the live corpus against the emitted schema with the Go "
        "validator (santhosh-tekuri/jsonschema/v6) - the B.15 gate",
    )
    validate_cmd.add_argument(
        "--code", type=Path, required=True, help="the JetStore repo root"
    )
    validate_cmd.add_argument(
        "--schema",
        type=Path,
        default=Path("tools/cpipes_contract/cpipes_schema.json"),
        help="schema path, relative to --code",
    )
    validate_cmd.add_argument(
        "--corpus", type=Path, default=Path(".."), help="root holding workspaces/, relative to --code"
    )

    args = parser.parse_args(argv)

    if args.command == "fragments":
        import json

        from .fragments import check_against_matrix, criterion_24, curate, extract, to_jsonl

        schema = json.loads(args.schema.read_text())
        ex = extract(schema, args.corpus, args.model)
        parts = ex.library()
        for finding in ex.unresolved:
            print(f"FINDING unresolved union at {finding}", file=sys.stderr)
        args.out.parent.mkdir(parents=True, exist_ok=True)
        args.out.write_text(to_jsonl(parts))
        drift = check_against_matrix(parts, args.matrix / "types.csv")
        for finding in drift:
            print(f"FINDING corpus count disagrees - {finding}", file=sys.stderr)
        names = {p.defs_name for p in parts}
        print(
            f"wrote {args.out}: {len(parts)} distinct parts over {len(names)} $defs "
            f"entries, from {sum(p.instances for p in parts)} corpus instances"
        )

        kept, stats = curate(parts)
        args.curated_out.write_text(to_jsonl(kept))
        print(
            f"wrote {args.curated_out}: {stats['output']} curated "
            f"({stats['excluded_deprecated']} deprecated and "
            f"{stats['excluded_runtime_shape']} runtime-shape excluded, "
            f"{stats['kept_thin']} kept whole from thin types, "
            f"{stats['dropped_repetition']} repetitions dropped)"
        )
        thin_lost = criterion_24(parts, kept, args.matrix / "types.csv")
        for finding in thin_lost:
            print(f"FINDING criterion 24 - {finding}", file=sys.stderr)
        return 1 if (ex.unresolved or drift or thin_lost) else 0

    if args.command == "bundles":
        import json

        from .bundles import check_corpus

        schema = json.loads(args.schema.read_text())
        checked, findings = check_corpus(schema, args.matrix, args.corpus)
        for finding in findings:
            print(f"FINDING {finding}", file=sys.stderr)
        print(
            f"{checked} live fragments checked against their bundle; "
            f"{len(findings)} finding(s)"
        )
        return 1 if findings else 0

    if args.command == "validate":
        import subprocess

        proc = subprocess.run(
            [
                "go", "run", "./tools/cpipes_contract/validate",
                "-schema", str(args.schema), "-corpus", str(args.corpus),
            ],
            cwd=args.code,
        )
        return proc.returncode

    if args.command == "schema":
        from . import schema as schema_mod

        return schema_mod.run(args)

    if args.command == "reflect":
        # reflect reads the CSV raw (it rewrites cells the schema validates)
        from . import reflect as reflect_mod

        return reflect_mod.run(args)

    try:
        matrix = Matrix.load(args.matrix)
    except (OSError, ValueError) as err:
        print(f"FAIL: {err}", file=sys.stderr)
        return 1

    if args.command == "corpus":
        return _corpus(args, matrix)
    if args.command == "harness":
        return _harness(args, matrix)
    if args.command == "stamp":
        return _stamp(args, matrix)
    if args.command == "drift":
        from . import drift as drift_mod

        return drift_mod.run(args, matrix)
    if args.command == "gofile":
        from . import emit_go

        return emit_go.run_with_matrix(args, matrix)
    if args.command == "generate":
        from . import generate as generate_mod

        return generate_mod.run_with_matrix(args, matrix)

    problems = check(matrix, strict=args.strict)
    if args.corpus is not None:
        problems += check_exemplars(matrix, args.corpus)
    if args.code is not None:
        problems += check_citations(matrix, args.code)

    for problem in problems:
        print(f"FAIL {problem}", file=sys.stderr)

    counts = (
        f"{len(matrix.types)} types, {len(matrix.fields_)} fields, "
        f"{len(matrix.constraints)} constraints"
    )
    report = unfilled_report(matrix)
    if report is not None:
        print(report)
    if problems:
        print(f"\n{len(problems)} problem(s) over {counts}", file=sys.stderr)
        return 1
    print(f"ok: {counts}")
    return 0


def _corpus(args, matrix: Matrix) -> int:
    live, retired = corpus_mod.config_files(args.corpus)
    measurement = corpus_mod.measure(matrix, args.corpus)
    print(
        f"corpus: {len(live)} live .pc.json under */pipes_config/, "
        f"{len(retired)} retired under */data/ and not counted"
    )

    if args.unknown:
        for key, counter in sorted(measurement.unknown_keys.items()):
            for name, n in counter.most_common():
                print(f"  unaccounted {key[0]}/{key[1]}.{name}: {n}")

    for (struct, token), n in measurement.unreachable.most_common():
        print(f"  unreachable {struct}/{token}: {n} node(s) - no matrix row")

    if args.apply:
        corpus_mod.apply(matrix, measurement)
        matrix.save(args.matrix)
        print(f"applied to {args.matrix}")
        return 0

    problems = corpus_mod.drift(matrix, measurement)
    for problem in problems:
        print(f"DRIFT {problem}", file=sys.stderr)
    if problems:
        print(f"\n{len(problems)} drift(s); rerun with --apply", file=sys.stderr)
        return 1
    print("ok: the matrix matches the live corpus")
    return 0


def _stamp(args, matrix: Matrix) -> int:
    """Certify what the reviewer marked. The command never touches `review`
    itself - it only writes the fingerprint of rows already marked reviewed
    (so `check` can tell when one later changes underneath its tick), clears
    leftover stamps on rows no longer marked, and, only under --restamp,
    re-certifies mismatches as an explicit re-approval."""
    from .matrix_schema import NONE, Review, row_hash

    stamped = cleared = restamped = mismatched = 0
    for row in [*matrix.fields_, *matrix.constraints]:
        if row.review is Review.REVIEWED:
            current = row_hash(row)
            if row.reviewed_hash == NONE:
                row.reviewed_hash = current
                stamped += 1
            elif row.reviewed_hash != current:
                if args.restamp:
                    row.reviewed_hash = current
                    restamped += 1
                else:
                    mismatched += 1
        elif row.reviewed_hash != NONE:
            row.reviewed_hash = NONE
            cleared += 1
    matrix.save(args.matrix)
    print(
        f"stamped {stamped}, cleared {cleared}, restamped {restamped}; "
        f"{mismatched} changed-since-review left for re-review"
    )
    return 1 if mismatched else 0


def _harness(args, matrix: Matrix) -> int:
    try:
        report = harness_mod.evaluate(matrix, args.code, dump_dir=args.dump)
    except (RuntimeError, LookupError) as err:
        print(f"FAIL: {err}", file=sys.stderr)
        return 1

    for note in report.notes:
        print(f"  note {note}")
    for finding in report.findings:
        print(f"FINDING {finding}", file=sys.stderr)
    print(harness_mod.summary(report))

    if args.apply:
        harness_mod.apply(matrix, report)
        matrix.save(args.matrix)
        print(f"applied to {args.matrix}")
        return 0
    return 1 if report.findings else 0


if __name__ == "__main__":  # pragma: no cover
    raise SystemExit(main())
