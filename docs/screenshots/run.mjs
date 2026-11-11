// `npm run shots [-- <spec folder or file>...]`: the whole screenshot run.
//
// Starts the stack (stack/up.sh), runs the specs with Playwright, and takes the
// stack down (stack/down.sh) whether the specs passed or not, and on Ctrl-C.
// The admin's password is generated here, passed to the stack and the specs in
// the environment, and written nowhere.
import { spawn } from "node:child_process";
import { randomBytes } from "node:crypto";
import { existsSync, readFileSync, rmSync } from "node:fs";
import { dirname, join } from "node:path";
import { fileURLToPath } from "node:url";

const here = dirname(fileURLToPath(import.meta.url));
const resultsFile = join(here, "test-results", "shots.jsonl");
const started = Date.now();

if (!process.env.JETS_BIN_DIR) {
  console.error("Set JETS_BIN_DIR to the directory of the JetStore binaries; stack/build-bin.sh builds them there.");
  process.exit(2);
}

const env = {
  ...process.env,
  JETS_ADMIN_PWD: randomBytes(18).toString("base64url"),
};

/** Runs a command, its output passed through; resolves to its exit code and its stdout. */
function run(command, args, { captureStdout = false } = {}) {
  return new Promise((resolveRun) => {
    const child = spawn(command, args, {
      cwd: here,
      env,
      stdio: ["ignore", captureStdout ? "pipe" : "inherit", "inherit"],
    });
    let stdout = "";
    if (captureStdout) {
      child.stdout.on("data", (chunk) => {
        stdout += chunk;
        process.stdout.write(chunk);
      });
    }
    child.on("close", (code, signal) => resolveRun({ code: code ?? (signal ? 1 : 0), stdout }));
  });
}

let stopping = null;
function stopStack() {
  stopping ??= run("bash", [join(here, "stack/down.sh")]);
  return stopping;
}

for (const signal of ["SIGINT", "SIGTERM"]) {
  process.on(signal, async () => {
    console.error(`\n[shots] ${signal}: taking the stack down`);
    await stopStack();
    process.exit(130);
  });
}

function summary() {
  if (!existsSync(resultsFile)) return "no image taken";
  const counts = { new: 0, written: 0, unchanged: 0 };
  const changed = [];
  for (const line of readFileSync(resultsFile, "utf8").split("\n").filter(Boolean)) {
    const { file, outcome } = JSON.parse(line);
    counts[outcome] += 1;
    if (outcome !== "unchanged") changed.push(`  ${outcome.padEnd(7)} ${file}`);
  }
  return [
    `${counts.new} new, ${counts.written} rewritten, ${counts.unchanged} unchanged`,
    ...changed,
  ].join("\n");
}

let exitCode = 1;
try {
  rmSync(resultsFile, { force: true });
  const up = await run("bash", [join(here, "stack/up.sh")], { captureStdout: true });
  const url = /^STACK_URL=(\S+)$/m.exec(up.stdout)?.[1];
  if (up.code !== 0 || !url) {
    console.error("[shots] the stack did not start");
  } else {
    env.STACK_URL = url;
    const playwright = join(here, "node_modules", ".bin", "playwright");
    const tests = await run(playwright, ["test", ...process.argv.slice(2)]);
    exitCode = tests.code;
    if (exitCode !== 0) {
      console.error("[shots] a spec failed: the page it illustrates is owed an update (see above)");
    }
  }
} finally {
  await stopStack();
  console.log(`[shots] ${summary()}`);
  console.log(`[shots] ${Math.round((Date.now() - started) / 1000)} s`);
}
process.exit(exitCode);
