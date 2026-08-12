"""One command with an exit code: there is no CI service to host it."""

from __future__ import annotations

import argparse
import sys
from pathlib import Path

from .check import check, check_citations, check_exemplars
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

    args = parser.parse_args(argv)

    try:
        matrix = Matrix.load(args.matrix)
    except (OSError, ValueError) as err:
        print(f"FAIL: {err}", file=sys.stderr)
        return 1

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
    if problems:
        print(f"\n{len(problems)} problem(s) over {counts}", file=sys.stderr)
        return 1
    print(f"ok: {counts}")
    return 0


if __name__ == "__main__":  # pragma: no cover
    raise SystemExit(main())
