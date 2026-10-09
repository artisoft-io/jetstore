"""The `jets-agentic generate` command (A1.7).

One command, emitters registered so items 2a and 3 extend the registry rather
than edit the command (§4, deliverable 2). Outputs land in
`jets/workspace_assets/data_model/` (§3.9) and are committed — the generator
stays off the deployment path and out of the client's toolchain.
"""

from __future__ import annotations

import argparse
import sys
from pathlib import Path

from . import ddl, header, jr, phi, schema, sidecar, toolsig

# The emitter registry: (repo-root-relative output path, emitter). Item 3's
# glossary and tool-signature emitters append here. Workspace-installed
# assets live under jets/workspace_assets/ (A21.1); the audit DDL lands in
# the Go package that go:embeds it, which cannot cross package directories.
EMITTERS: list[tuple[str, object]] = [
    ("jets/workspace_assets/data_model/jets_agentic.jr", jr.emit),
    ("jets/workspace_assets/data_model/jets_agentic.meta.json", sidecar.emit),
    ("jets/workspace_assets/data_model/jets_agentic.schema.json", schema.emit),
    ("jets/agentic/audit/agent_audit.sql", ddl.emit),
    # AE.2: the data_classification markers as data, beside the DDL and for
    # the same reason -- the package that consumes it is the package it has
    # to live in.
    ("jets/agentic/audit/data_classification.go", phi.emit),
    ("jets/agentic/tools/jets_agentic_tools.json", toolsig.emit),
]

# tools/jets_agentic/jets_agentic/main.py -> the JetStore repo root.
REPO_ROOT = Path(__file__).resolve().parents[3]
DEFAULT_OUT = REPO_ROOT

# A21.7. `jets/workspace_assets/data_model/` is installed into client
# workspaces wholesale by `jets/workspace_assets/install.go`, whose embed glob
# takes every `.jr` and `.json` in it — so a file arriving there by any route
# reaches every client. --check therefore accounts for the whole directory, not
# only for what the registry writes: anything not emitted above has to be named
# here, with the reason it is exempt.
# The sibling group, jets/workspace_assets/pipes_config/, is installed by the
# same step and reaches every client the same way, but nothing here emits into
# it and --check does not account for it: its assets are hand-authored .pc.json
# and the guard against a stray file there is review, not this check. If a
# generator ever writes a pipeline configuration, give it its own ASSET_DIR
# entry rather than widening this one.
ASSET_DIR = "jets/workspace_assets/data_model"
HAND_AUTHORED = {
    # The platform base model (A21.6). Hand-authored in JetStore rather than
    # generated, JetStore-owned all the same, and installed by the same step —
    # which is what makes the six divergent copies non-authoritative.
    "jets_model.jr",
}
NOT_INSTALLED = {
    # JetStore's own documentation of the convention (A1.10, A21.4). The embed
    # glob excludes it; this list is what makes that deliberate rather than
    # incidental.
    "README.md",
}


def main(argv: list[str] | None = None) -> int:
    parser = argparse.ArgumentParser(prog="jets-agentic")
    sub = parser.add_subparsers(dest="command", required=True)
    generate = sub.add_parser("generate", help="emit every registered artifact")
    generate.add_argument(
        "--out",
        type=Path,
        default=DEFAULT_OUT,
        help="root directory the registry's relative output paths are joined "
        "under (default: the repo root)",
    )
    generate.add_argument(
        "--check",
        action="store_true",
        help="compare against the committed outputs instead of writing; "
        "exit nonzero on any difference (the CI shape of I-9)",
    )
    compile_cmd = sub.add_parser(
        "compile",
        help="compile the emitted .jr in a throwaway workspace and assert the "
        "class, vocabulary and round-trip properties (A1.8, A1.9)",
    )
    compile_cmd.add_argument("--keep", action="store_true", help="keep the throwaway workspace")
    precheck_cmd = sub.add_parser(
        "precheck",
        help="fail with the colliding names if any emitted class or property "
        "name is already taken in a target workspace's workspace.db (A1.11); "
        "run before the first install",
    )
    precheck_cmd.add_argument(
        "workspace_db", help="path to the target workspace's workspace.db (or its directory)"
    )
    schema_cmd = sub.add_parser(
        "schema",
        help="print the JSON Schema projection of an entity over an "
        "allowed-field subset — what an agent's decode is constrained by "
        "(A2a.1); pass it as-is to Ollama `format` or vLLM `guided_json`",
    )
    schema_cmd.add_argument("entity", help="entity class name, e.g. Incident")
    schema_cmd.add_argument(
        "--fields",
        help="comma-separated allowed fields; omit for the whole entity",
    )
    decode_cmd = sub.add_parser(
        "decode-check",
        help="verify the emitted schema drives constrained decoding with no "
        "translation step (A2a.3) and that vocabularies survive as $defs "
        "enums the projection cannot widen (A2a.4)",
    )
    decode_cmd.add_argument(
        "--ollama-model",
        help="run a live Ollama `format` decode against this model "
        "(needs a reachable server); without it the Ollama half only "
        "reports how to run it",
    )

    args = parser.parse_args(argv)

    if args.command == "compile":
        from . import compile_check

        return compile_check.run(args)

    if args.command == "precheck":
        from . import precheck

        return precheck.run(args)

    if args.command == "schema":
        import json

        entity = schema.entity_by_name(args.entity)
        fields = (
            [f.strip() for f in args.fields.split(",")]
            if args.fields
            else list(entity.model_fields)
        )
        print(json.dumps(schema.schema_for(entity, fields), indent=2))
        return 0

    if args.command == "decode-check":
        from . import decode_check

        return decode_check.run(args)

    stale: list[str] = []
    for name, emitter in EMITTERS:
        text = emitter()  # type: ignore[operator]
        target = args.out / name
        if args.check:
            if not target.exists() or target.read_text() != text:
                stale.append(name)
        else:
            target.parent.mkdir(parents=True, exist_ok=True)
            target.write_text(text)
            print(f"wrote {target}")
    if args.check:
        problems = [
            f"stale generated output (regenerate with `jets-agentic generate`): {name}"
            for name in stale
        ]
        problems.extend(check_asset_dir(args.out))
        if problems:
            for p in problems:
                print(p)
            return 1
        print(
            f"generated outputs match their source; {ASSET_DIR} holds nothing unaccounted for"
        )
    return 0


def check_asset_dir(out: Path) -> list[str]:
    """A21.7's second half: the installed set is the directory's contents, so
    the directory is what has to be accounted for."""
    directory = out / ASSET_DIR
    if not directory.is_dir():
        return [f"{ASSET_DIR} does not exist"]
    generated = {Path(name).name for name, _ in EMITTERS if name.startswith(ASSET_DIR)}
    problems: list[str] = []
    for path in sorted(directory.iterdir()):
        if path.name in generated or path.name in NOT_INSTALLED:
            continue
        if path.name in HAND_AUTHORED:
            if header.TOKEN not in path.read_text():
                problems.append(
                    f"{ASSET_DIR}/{path.name} has lost its {header.TOKEN} header, which "
                    f"is what the install guard reads to tell a JetStore file from a "
                    f"client's (A21.4)"
                )
            continue
        problems.append(
            f"{ASSET_DIR}/{path.name} is neither generated nor declared hand-authored, "
            f"and every .jr and .json in that directory is installed into every client "
            f"workspace. Add it to EMITTERS, HAND_AUTHORED or NOT_INSTALLED."
        )
    return problems


if __name__ == "__main__":
    sys.exit(main())
