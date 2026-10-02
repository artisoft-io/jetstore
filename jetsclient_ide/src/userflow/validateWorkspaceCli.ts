/**
 * `node jets_validate_workspace.mjs <workspace-dir> [--json]` —
 * `jetstore_maintenance_02` tasks AG.2 and AG.3 (`R12`).
 *
 * The Node entry to `validateWorkspace`, bundled into one script by
 * `vite.validator.config.ts` so that it runs with nothing beside it: no
 * `node_modules`, no TypeScript. It is what `CompileWorkspace` runs over a
 * workspace before compiling it (`jets/workspace/asset_validation.go`), and what
 * a generator runs over a document set it has just written.
 *
 * ## Exit status is the contract
 *
 * - **0** — no error finding. Warnings are printed and do not fail.
 * - **1** — at least one error finding.
 * - **2** — it could not validate at all: no argument, or a directory that does
 *   not exist. **Never 0**, because the Go side reads anything but 0 as a failed
 *   compile, and a validator that cannot see the workspace has not passed it.
 *
 * `--json` prints the findings as one JSON array on stdout instead of lines, for
 * a caller that wants the pointers as data — the shape `validate_cpipes_config`
 * gives an agent for `.pc.json`.
 *
 * The reachability policy is read from `JETS_USERFLOW_STRICT_REACHABILITY`, the
 * variable Go's save-time check reads (`jets/userflow/policy.go`), so the two
 * gates agree on whether an unreachable state is an error.
 */

import { existsSync, readFileSync, readdirSync, statSync } from "node:fs";
import { join } from "node:path";

import { productionRegistry } from "../actions/registry";
import { policyFromEnv } from "./validate";
import { formatFindings, validateWorkspace, type WorkspaceReader } from "./validateWorkspace";

export function directoryReader(root: string): WorkspaceReader {
  return {
    list(dir) {
      const full = join(root, dir);
      if (!existsSync(full) || !statSync(full).isDirectory()) return null;
      return readdirSync(full).filter((name) => statSync(join(full, name)).isFile());
    },
    read(path) {
      const full = join(root, path);
      return existsSync(full) ? readFileSync(full, "utf8") : null;
    },
  };
}

/** The whole command, with its I/O injected so a test can drive it. */
export function main(
  argv: readonly string[],
  env: Record<string, string | undefined>,
  out: (line: string) => void,
  err: (line: string) => void,
): number {
  const json = argv.includes("--json");
  const positional = argv.filter((a) => !a.startsWith("--"));
  if (positional.length !== 1) {
    err("usage: jets_validate_workspace <workspace-dir> [--json]");
    return 2;
  }
  const root = positional[0]!;
  if (!existsSync(root) || !statSync(root).isDirectory()) {
    err(`jets_validate_workspace: ${root} is not a directory`);
    return 2;
  }

  const findings = validateWorkspace(directoryReader(root), {
    registry: productionRegistry,
    policy: policyFromEnv(env),
  });
  const errors = findings.filter((f) => f.severity === "error").length;
  const warnings = findings.length - errors;

  if (json) {
    out(JSON.stringify(findings, null, 2));
  } else {
    for (const line of formatFindings(findings)) out(line);
    out(
      `jets_validate_workspace: ${errors} error(s), ${warnings} warning(s) in user_flows/ and table_configs/ of ${root}`,
    );
  }
  return errors > 0 ? 1 : 0;
}

// Run only when executed, not when imported by a test. The bundle is the
// executed file, and `process.argv[1]` is its path.
const invokedAs = process.argv[1] ?? "";
if (/jets_validate_workspace\.m?js$/.test(invokedAs) || /validateWorkspaceCli\.ts$/.test(invokedAs)) {
  process.exitCode = main(
    process.argv.slice(2),
    process.env,
    (line) => process.stdout.write(`${line}\n`),
    (line) => process.stderr.write(`${line}\n`),
  );
}
