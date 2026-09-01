"""T.3, criterion 38: generate-and-compile, per operator, one arm against another.

For each hole of each authored template: select candidate fragments by one method,
generate, expand the template, and put the expanded configuration through the **real**
startup validator - `ValidatePipeSpecConfig`, via the Go runner in `harness/`, which is
the same gate `validate_cpipes_config` is. Report per operator with denominators.

**Why this had to wait for T.2.** I-68 measured that a correctly selected tool call can
arrive with its payload mangled, deterministically and invisibly to schema validation.
Run generate-and-compile before that is fixed and a config that does not compile is
ambiguous between bad retrieval and a corrupted payload, which is the single-number
problem decision 13 refuses one level up (plan section 5.1). The path arm of T.2 removes
the payload from the wire; this run is what the removal was for.

**Two gates, and they answer different questions.**

- *fragment* - the generated value validates against the hole's own bundle subschema.
  This is criterion 22's gate, and `from_model` already applies it, because Ollama's
  `format` does not enforce the schema and a parseable response is no evidence at all.
- *compile-pass* - the expanded config is accepted by the engine's own validator. This
  is criterion 38's gate, and it is strictly harder: every hole has to be filled with
  something that validates *and* the assembled document has to hold together.

A template whose fragments all fail cannot reach the second gate, and the report says
`not reached` rather than `0 of 1` - the distinction P.1 had to draw the hard way, where
121 of 121 failures never reached the verifier and a rate whose failures are all upstream
of the gate is not a compile-pass rate.

**No aggregate, and per operator rather than per hole.** Decision 13's rule, and
`Report.per_operator` carries the argument.
"""

from __future__ import annotations

import json
from dataclasses import dataclass, field
from pathlib import Path

from . import retrieval as R
from .authoring import Report, from_model, reachable
from .expand import expand as expand_template
from .harness import run_cases
from .template import Template
from .template import check as check_template
from .template import load as load_template

# `eval.RateThreshold`, restated here rather than imported across the Go/Python
# boundary: decision 13 says four or fewer attempts report cases, inclusive.
RATE_THRESHOLD = 5


@dataclass
class ArmResult:
    """One (template, method) run."""

    template: str
    method: str
    report: Report
    # unfilled holes reported by `expand`; a template with any of these did not
    # assemble and its config is not put to the validator.
    findings: list[str] = field(default_factory=list)
    config: dict | None = None
    # compile is one of: pass | fail | not-reached
    compile: str = "not-reached"
    compile_error: str = ""
    compile_steps: int = 0


def run_arm(
    tpl: Template,
    context: dict,
    schema: dict,
    matrix: Path,
    library: list[dict],
    model: str,
    method: str,
    host: str,
    shots: int,
    embedder: R.Embedder | None,
) -> ArmResult:
    report = Report(model=model, method=method)
    config, findings = expand_template(
        tpl,
        context,
        from_model(
            schema,
            matrix,
            model,
            host,
            report=report,
            library=library,
            shots=shots,
            method=method,
            embedder=embedder,
        ),
        schema,
    )
    return ArmResult(
        template=tpl.name,
        method=method,
        report=report,
        findings=list(findings),
        config=config,
    )


def compile_pass(code_root: Path, arms: list[ArmResult]) -> None:
    """Put every assembled config through the engine's own validator.

    Batched into one `go run`, because starting the Go toolchain once per template is
    most of the wall clock. An arm whose expansion left a hole unfilled is not sent:
    the document is not the one the template describes, and scoring it would report the
    generator's failure as the engine's verdict.
    """
    cases: list[tuple[str, dict]] = []
    index: dict[str, ArmResult] = {}
    for arm in arms:
        if arm.findings or arm.config is None:
            continue
        cid = f"{arm.template}::{arm.method}"
        cases.append((cid, arm.config))
        index[cid] = arm
    if not cases:
        return
    for cid, res in run_cases(code_root, cases).items():
        arm = index[cid]
        arm.compile = "pass" if res.ok else "fail"
        # **A pass over zero model-authored fragments is not evidence about the
        # model, and the first run of this tool produced two of them.** `qc_report`
        # asks a filler for nothing - every one of its holes carries a `$body`, so
        # it is a loop rather than a blank and its configuration lives in its
        # bindings (I-76, sharpened by I-78). Expanding it therefore assembles a
        # valid config from the bindings alone and the validator accepts it, which
        # renders as `pass` beside two genuine `not-reached`s and reads as the
        # arm's best result. It is the arm's *empty* result.
        if not arm.report.attempts:
            arm.compile = arm.compile + " (vacuous: 0 fragments authored)"
        arm.compile_error = res.error
        arm.compile_steps = res.steps


def render(arms: list[ArmResult], methods: list[str], unavailable: dict[str, str]) -> str:
    lines: list[str] = []

    # --- the per-operator table, which is criterion 38's unit ------------------
    per: dict[tuple[str, str], list[int]] = {}
    for arm in arms:
        for ref, (ok, n) in arm.report.per_operator().items():
            row = per.setdefault((ref, arm.method), [0, 0])
            row[0] += ok
            row[1] += n
    refs = sorted({ref for ref, _ in per})
    header = f"{'operator (schema_ref)':30s}" + "".join(f"{m:>18s}" for m in methods)
    lines += ["fragment gate - the generated value validates against the operator's own bundle", "", header]
    for ref in refs:
        cells = ""
        for m in methods:
            row = per.get((ref, m))
            cells += f"{'-':>18s}" if row is None else f"{f'{row[0]} of {row[1]}':>18s}"
        # Decision 13's threshold, applied here as it is in `eval`: below five
        # attempts an operator reports cases and not a rate, and marking the row
        # is what stops a reader reading `1 of 3` against `2 of 3` as 33% against
        # 67%.
        thin = max((per[(ref, m)][1] for m in methods if (ref, m) in per), default=0)
        mark = "   (cases, not a rate)" if thin < RATE_THRESHOLD else ""
        lines.append(f"{ref:30s}{cells}{mark}")

    # --- the compile-pass table ------------------------------------------------
    lines += ["", "compile-pass gate - the expanded config accepted by ValidatePipeSpecConfig", ""]
    lines.append(f"{'template':30s}" + "".join(f"{m:>34s}" for m in methods))
    for name in sorted({a.template for a in arms}):
        cells = ""
        for m in methods:
            hit = [a for a in arms if a.template == name and a.method == m]
            cells += f"{'-':>18s}" if not hit else f"{hit[0].compile:>34s}"
        lines.append(f"{name:30s}{cells}")

    for arm in arms:
        if arm.findings:
            lines.append(
                f"  {arm.template} / {arm.method}: {len(arm.findings)} hole(s) left unfilled, "
                f"so the config was not put to the validator"
            )
        elif arm.compile == "fail":
            lines.append(f"  {arm.template} / {arm.method}: {arm.compile_error[:120]}")

    for method, why in sorted(unavailable.items()):
        lines += ["", f"ARM NOT RUN - {method}: {why}"]

    lines += [
        "",
        "Denominators are the repetitions each hole was expanded to; there is no "
        "aggregate (decision 13). A `not-reached` compile-pass is a template whose "
        "fragments did not assemble, and is not a compile failure: a rate whose "
        "failures are all upstream of the gate is not a compile-pass rate.",
    ]
    return "\n".join(lines)


def run(args, matrix: Path) -> int:
    schema = json.loads(args.schema.read_text())
    library = [
        json.loads(line)
        for line in args.library.read_text().splitlines()
        if line.strip()
    ]
    ready, detail = reachable(args.host)
    if not ready:
        print(f"infer server not ready: {detail}")
        return 1

    methods = [m.strip() for m in args.select.split(",") if m.strip()]
    for m in methods:
        if m not in R.METHODS:
            print(f"{m!r} is not one of {R.METHODS}")
            return 2

    # The semantic arm is probed once, before any generation, so an unavailable
    # embedding model is reported as an arm that was not run rather than
    # discovered mid-run and papered over.
    unavailable: dict[str, str] = {}
    embedder: R.Embedder | None = None
    if R.SEMANTIC in methods:
        embedder = R.Embedder(args.embed_model, args.host, args.timeout)
        try:
            embedder.vectors(["probe"])
        except Exception as err:  # noqa: BLE001
            unavailable[R.SEMANTIC] = str(err)[:300]
            methods = [m for m in methods if m != R.SEMANTIC]
            embedder = None

    paths = sorted(args.dir.glob("*.template.json"))
    arms: list[ArmResult] = []
    for path in paths:
        tpl = load_template(path)
        findings, _ = check_template(tpl, schema, library, matrix)
        if findings:
            for finding in findings:
                print(f"FINDING {path.name}: {finding}")
            return 1
        beside = path.with_name(path.name.replace(".template.json", ".bindings.json"))
        if not beside.exists():
            print(f"  {path.name}: no bindings; skipped")
            continue
        context = json.loads(beside.read_text())
        for method in methods:
            print(f"  {tpl.name} / {method} ...", flush=True)
            arms.append(
                run_arm(
                    tpl, context, schema, matrix, library,
                    args.model, method, args.host, args.shots, embedder,
                )
            )

    if args.code is not None:
        compile_pass(args.code, arms)

    print()
    print(f"model: {args.model}")
    if embedder is not None:
        print(f"embedding model: {embedder.model}")
    print()
    print(render(arms, methods + sorted(unavailable), unavailable))
    if args.out:
        args.out.write_text(
            json.dumps(
                [
                    {
                        "template": a.template,
                        "method": a.method,
                        "per_operator": a.report.per_operator(),
                        "per_hole": a.report.per_hole(),
                        "unfilled": a.findings,
                        "compile": a.compile,
                        "compile_error": a.compile_error,
                        "compile_steps": a.compile_steps,
                    }
                    for a in arms
                ],
                indent=2,
            )
            + "\n"
        )
        print(f"wrote {args.out}")
    # A measurement's exit code says the measurement ran, not that the model did
    # well: a zero pass rate is a result and an unrunnable arm is not.
    return 1 if unavailable else 0
