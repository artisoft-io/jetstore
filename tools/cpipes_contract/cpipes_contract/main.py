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

    templates_cmd = sub.add_parser(
        "templates",
        help="check authored templates: holes declared, placed, typed and covered (F.3)",
    )
    templates_cmd.add_argument("--matrix", type=Path, default=DEFAULT_MATRIX)
    templates_cmd.add_argument(
        "--schema", type=Path, default=DEFAULT_MATRIX.parent / "cpipes_schema.json"
    )
    templates_cmd.add_argument(
        "--library",
        type=Path,
        default=DEFAULT_MATRIX.parent / "fragments" / "library.curated.jsonl",
    )
    templates_cmd.add_argument(
        "--dir", type=Path, default=DEFAULT_MATRIX.parent / "templates"
    )
    templates_cmd.add_argument(
        "--expand",
        type=Path,
        help="a JSON file binding each repeat_over name to a list; expands every "
        "template against it and validates the result as a whole config (F.4)",
    )
    templates_cmd.add_argument(
        "--out", type=Path, help="where to write the expanded config (with --expand)"
    )
    templates_cmd.add_argument(
        "--author",
        metavar="MODEL",
        help="fill every hole by asking the infer server to author it from the hole's "
        "prompt, constrained by its schema_ref - criterion 22 (F.6)",
    )
    templates_cmd.add_argument("--host", default="http://localhost:11434")

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

    fixtures_cmd = sub.add_parser(
        "fixtures",
        help="assess the harvested compute_pipes test fixtures as library material (F.2b)",
    )
    fixtures_cmd.add_argument(
        "--fixtures", type=Path, required=True,
        help="JSONL written by `go run ./tools/cpipes_contract/fixtures -out ...`",
    )
    fixtures_cmd.add_argument("--matrix", type=Path, default=DEFAULT_MATRIX)
    fixtures_cmd.add_argument(
        "--schema", type=Path, default=DEFAULT_MATRIX.parent / "cpipes_schema.json"
    )
    fixtures_cmd.add_argument(
        "--library", type=Path,
        default=DEFAULT_MATRIX.parent / "fragments" / "library.jsonl",
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

    if args.command == "templates":
        import json

        from .template import check as check_template
        from .template import load as load_template

        schema = json.loads(args.schema.read_text())
        library = (
            [json.loads(line) for line in args.library.read_text().splitlines() if line.strip()]
            if args.library.exists()
            else None
        )
        paths = sorted(args.dir.glob("*.template.json"))
        if not paths:
            print(f"no templates in {args.dir}")
            return 0
        total = 0
        for path in paths:
            tpl = load_template(path)
            findings, notes = check_template(tpl, schema, library, args.matrix)
            holes = len(tpl.holes)
            repeating = sum(1 for h in tpl.holes if h.repeat_over)
            print(
                f"{path.name}: {holes} hole(s), {repeating} repeating, "
                f"{len(findings)} finding(s)"
            )
            for note in notes:
                print(f"  note {note}")
            for finding in findings:
                print(f"FINDING {path.name}: {finding}", file=sys.stderr)
            total += len(findings)

            if args.author and not findings:
                from .authoring import Report, from_model, reachable

                ready, detail = reachable(args.host)
                if not ready:
                    print(f"FINDING infer server not ready: {detail}", file=sys.stderr)
                    return 1
                # Bindings are per template: `<name>.bindings.json` beside it. One
                # template's context applied to another's holes is not a partial
                # answer, it is a different question.
                beside = path.with_name(path.name.replace(".template.json", ".bindings.json"))
                source = beside if beside.exists() else args.expand
                if source is None:
                    print(f"  {path.name}: no bindings; skipped")
                    continue
                context = json.loads(source.read_text())
                report = Report(model=args.author)
                from .expand import expand as expand_template

                config, bad = expand_template(
                    tpl,
                    context,
                    from_model(
                        schema,
                        args.matrix,
                        args.author,
                        args.host,
                        report=report,
                        library=library,
                    ),
                    schema,
                )
                print()
                print(report.render())
                for finding in bad:
                    print(f"  invalid: {finding}")
                if args.out:
                    args.out.write_text(json.dumps(config, indent=2) + "\n")
                    print(f"  wrote {args.out}")
                continue

            if args.expand and not findings:
                import jsonschema

                from .expand import expand as expand_template
                from .expand import from_library

                beside = path.with_name(path.name.replace(".template.json", ".bindings.json"))
                context = json.loads((beside if beside.exists() else args.expand).read_text())
                if library is None:
                    print(f"FINDING {path.name}: --expand needs the library", file=sys.stderr)
                    total += 1
                    continue
                config, bad = expand_template(
                    tpl, context, from_library(library, args.matrix), schema
                )
                for finding in bad:
                    print(f"FINDING {path.name}: {finding}", file=sys.stderr)
                validator = jsonschema.Draft202012Validator(
                    {
                        "$schema": schema["$schema"],
                        "$ref": "#/$defs/ComputePipesConfig",
                        "$defs": schema["$defs"],
                    }
                )
                errors = sorted(validator.iter_errors(config), key=lambda e: len(e.path))
                for error in errors[:5]:
                    at = ".".join(str(p) for p in error.path) or "<root>"
                    print(f"FINDING {path.name}: expanded config invalid at {at}: "
                          f"{error.message[:120]}", file=sys.stderr)
                total += len(bad) + len(errors)
                if not bad and not errors:
                    print(f"  expanded and validates as an ordinary .pc.json")
                if args.out:
                    args.out.write_text(json.dumps(config, indent=2) + "\n")
                    print(f"  wrote {args.out}")
        return 1 if total else 0

    if args.command == "fragments":
        import json

        from .fragments import (
            check_against_matrix,
            criterion_24,
            curate,
            extract,
            recovered_parts,
            to_jsonl,
        )

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

        rec = recovered_parts(schema, args.matrix, args.corpus, args.model)
        recovered = rec.library()
        for note in rec.stale:
            print(f"  note stale in a recovered source, not admitted: {note}")
        for finding in rec.unresolved:
            print(f"FINDING recovered source unreadable - {finding}", file=sys.stderr)

        kept, stats = curate(parts)
        # Criterion 24 is about what the curation *dropped* from the corpus, so it is
        # checked before recovered parts are added: an addition is not a loss, and
        # comparing after would report a gain as a violation.
        thin_lost = criterion_24(parts, kept, args.matrix / "types.csv")

        # Recovered parts join the *curated* library, not the extraction: library.jsonl
        # stays a faithful record of the live corpus, while the consumable library gets
        # the operator coverage the corpus can no longer provide.
        have = {p.defs_name for p in kept}
        added = [p for p in recovered if p.defs_name not in have]
        kept.extend(added)
        kept.sort(key=lambda p: (p.defs_name, -p.instances))
        stats["recovered_added"] = len(added)
        stats["output"] = len(kept)
        args.curated_out.write_text(to_jsonl(kept))
        print(
            f"wrote {args.curated_out}: {stats['output']} curated "
            f"({stats['excluded_deprecated']} deprecated and "
            f"{stats['excluded_runtime_shape']} runtime-shape excluded, "
            f"{stats['kept_thin']} kept whole from thin types, "
            f"{stats['dropped_repetition']} repetitions dropped, "
            f"{stats['recovered_added']} recovered from history)"
        )
        for finding in thin_lost:
            print(f"FINDING criterion 24 - {finding}", file=sys.stderr)
        return 1 if (ex.unresolved or drift or thin_lost or rec.unresolved) else 0

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

    if args.command == "fixtures":
        import json

        from .fixtures import coverage, load, render, validate as validate_fixtures

        schema = json.loads(args.schema.read_text())
        library = [
            json.loads(line)
            for line in args.library.read_text().splitlines()
            if line.strip()
        ]
        good, bad = validate_fixtures(load(args.fixtures), schema)
        print(render(good, bad, coverage(good, library, args.matrix)))
        # **Not a gate.** The report is the deliverable; there is nothing here for CI to
        # fail on, because nothing is merged into the library.
        return 0

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
