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

from . import ddl, jr, schema, sidecar

# The emitter registry: (repo-root-relative output path, emitter). Item 3's
# glossary and tool-signature emitters append here. Workspace-installed
# assets live under jets/workspace_assets/ (A21.1); the audit DDL lands in
# the Go package that go:embeds it, which cannot cross package directories.
EMITTERS: list[tuple[str, object]] = [
    ("jets/workspace_assets/data_model/jets_agentic.jr", jr.emit),
    ("jets/workspace_assets/data_model/jets_agentic.meta.json", sidecar.emit),
    ("jets/workspace_assets/data_model/jets_agentic.schema.json", schema.emit),
    ("jets/agentic/audit/agent_audit.sql", ddl.emit),
]

# tools/jets_agentic/jets_agentic/main.py -> the JetStore repo root.
REPO_ROOT = Path(__file__).resolve().parents[3]
DEFAULT_OUT = REPO_ROOT


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
        if stale:
            print(f"stale generated output (regenerate with `jets-agentic generate`): {', '.join(stale)}")
            return 1
        print("generated outputs match their source")
    return 0


if __name__ == "__main__":
    sys.exit(main())
