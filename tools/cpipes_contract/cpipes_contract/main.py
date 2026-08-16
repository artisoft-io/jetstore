"""One command with an exit code: there is no CI service to host it."""

from __future__ import annotations

import argparse
import sys
from pathlib import Path

from . import corpus as corpus_mod
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

    args = parser.parse_args(argv)

    try:
        matrix = Matrix.load(args.matrix)
    except (OSError, ValueError) as err:
        print(f"FAIL: {err}", file=sys.stderr)
        return 1

    if args.command == "corpus":
        return _corpus(args, matrix)

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


if __name__ == "__main__":  # pragma: no cover
    raise SystemExit(main())
