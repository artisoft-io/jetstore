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

from . import jr, sidecar

# The emitter registry: (output file name, emitter). Item 2a's JSON Schema
# emitter and item 3's glossary and tool-signature emitters append here.
EMITTERS: list[tuple[str, object]] = [
    ("jets_agentic.jr", jr.emit),
    ("jets_agentic.meta.json", sidecar.emit),
]

# tools/jets_agentic/jets_agentic/main.py -> the JetStore repo root.
REPO_ROOT = Path(__file__).resolve().parents[3]
DEFAULT_OUT = REPO_ROOT / "jets" / "workspace_assets" / "data_model"


def main(argv: list[str] | None = None) -> int:
    parser = argparse.ArgumentParser(prog="jets-agentic")
    sub = parser.add_subparsers(dest="command", required=True)
    generate = sub.add_parser("generate", help="emit every registered artifact")
    generate.add_argument(
        "--out", type=Path, default=DEFAULT_OUT, help="destination directory"
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

    args = parser.parse_args(argv)

    if args.command == "compile":
        from . import compile_check

        return compile_check.run(args)

    if args.command == "precheck":
        from . import precheck

        return precheck.run(args)

    args.out.mkdir(parents=True, exist_ok=True)
    stale: list[str] = []
    for name, emitter in EMITTERS:
        text = emitter()  # type: ignore[operator]
        target = args.out / name
        if args.check:
            if not target.exists() or target.read_text() != text:
                stale.append(name)
        else:
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
