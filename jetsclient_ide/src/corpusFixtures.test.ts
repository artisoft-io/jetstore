/**
 * The five generated corpora, checked from the end that consumes them.
 *
 * **Why this file exists.** Every corpus is a contract with two ends: the
 * Flutter app produces a JSON document, and this package keeps a copy of it as
 * a fixture. The producing end has been guarded since the first corpus — each
 * `*_corpus_test.dart` computes an FNV-1a checksum of what the app currently
 * emits and asserts it against a constant — and every one of those files says,
 * in its own doc comment, *"update it only together with the React fixture."*
 *
 * **A checksum is a guard on the side that computes it, and nothing more.** The
 * Dart assertion is `checksum(app configuration) == the Dart constant`; the
 * fixture appears in neither operand. So the convention was stated in five
 * places and enforced in none, and on 2026-08-25 C.0 (jetstore#2018) deleted
 * seven registry entries, bumped two Dart constants, left both React fixtures
 * untouched, and the Flutter suite went green: 44 passed, 1 failed, exactly the
 * baseline. The fixtures still described three tables and four forms that no
 * longer existed, and `screen_reachability.json` still carried the `/processConfig`
 * route the same commit had deleted.
 *
 * **This is the missing operand.** The assertion below is
 * `checksum(the fixture on disk) == the constant in the Dart test source`, which
 * fails in exactly the three ways the convention was meant to cover: a fixture
 * left stale behind a regenerated app, a fixture regenerated without the app,
 * and a Dart constant hand-edited to make its own test go green — the last being
 * the one every Dart doc comment forbids in prose and nothing could detect.
 *
 * **The shape is C.1's, with one asymmetry.** `jets/datatable/wsfile/sections_test.go`
 * and `jetsclient/test/workspace_section_contract_test.dart` guard a two-ended
 * contract by carrying the same declaration and the same checksum at both ends,
 * so changing either alone fails. Here the two ends cannot be symmetric: the
 * Dart corpus tests run under `--platform chrome`, where there is no filesystem
 * (I-5), which is why they print the corpus rather than writing it and why they
 * cannot read the fixture back. The comparison has to live on the side that can
 * open both files, and this package is it.
 *
 * **This test is deliberately cheap and deliberately temporary.** It needs no
 * Flutter toolchain and no browser — it reads two files and hashes one — so it
 * runs on every `npm test` rather than only when somebody remembers to
 * regenerate a corpus. And it dies with the Flutter app: track X deletes
 * `jetsclient/`, at which point these fixtures stop being a mirror of a running
 * app and become ordinary source, and this file should be deleted with the thing
 * it mirrors rather than pointed at a path that no longer exists.
 */

import { readFileSync } from "node:fs";
import { fileURLToPath } from "node:url";
import { describe, expect, it } from "vitest";

const dartTestDir = fileURLToPath(new URL("../../jetsclient/test/", import.meta.url));
const fixtureRoot = fileURLToPath(new URL("./", import.meta.url));

/**
 * FNV-1a, 32-bit, over the UTF-8 bytes — the same function as
 * `jetsclient/test/corpus_support.dart`'s `checksum`.
 *
 * `Math.imul` is the 32-bit multiply the Dart side spells out as five shifts,
 * for the same reason: the product overflows a double's 53 bits, and the Dart
 * corpus tests run in a browser.
 */
function fnv1a32(bytes: Uint8Array): string {
  let hash = 0x811c9dc5;
  for (const byte of bytes) {
    hash ^= byte;
    hash = Math.imul(hash, 16777619) >>> 0;
  }
  return `fnv1a32:${hash.toString(16).padStart(8, "0")}`;
}

/**
 * The five corpora, each as (the Dart test that produces it, the fixture that
 * consumes it). Adding a sixth corpus is one row.
 *
 * The fixture paths are the `corpusPath` constants the Dart tests declare, and
 * they are repeated rather than parsed out: a path this test derived from the
 * file it is checking would follow that file wherever it went, including
 * somewhere wrong.
 */
const corpora = [
  { dart: "table_config_corpus_test.dart", fixture: "datatable/fixtures/table_configs.json" },
  { dart: "form_field_corpus_test.dart", fixture: "datatable/fixtures/form_fields.json" },
  { dart: "user_flow_corpus_test.dart", fixture: "userflow/fixtures/user_flows.json" },
  { dart: "screen_config_corpus_test.dart", fixture: "screens/fixtures/screen_configs.json" },
  { dart: "screen_reachability_corpus_test.dart", fixture: "screens/fixtures/screen_reachability.json" },
] as const;

/** `const expectedChecksum = 'fnv1a32:xxxxxxxx';`, anchored to a whole line. */
const checksumDeclaration = /^const expectedChecksum = '(fnv1a32:[0-9a-f]{8})';$/m;

function declaredChecksum(dartFile: string): string {
  const source = readFileSync(`${dartTestDir}${dartFile}`, "utf8");
  const match = checksumDeclaration.exec(source);
  if (match === null) {
    throw new Error(
      `${dartFile} declares no 'const expectedChecksum'. Either the corpus test was ` +
        `renamed or refactored, or its checksum moved — in both cases this guard is ` +
        `no longer watching anything and the row in corpusFixtures.test.ts needs fixing.`,
    );
  }
  return match[1]!;
}

describe("the generated corpora agree with the fixtures this package reads", () => {
  // Separates "the hash is wrong" from "the fixture is stale" when a row below
  // fails. These are the published FNV-1a 32-bit vectors, so they check this
  // implementation against the specification rather than against the Dart one —
  // the five rows below are what check it against Dart.
  it("computes FNV-1a the way corpus_support.dart does", () => {
    expect(fnv1a32(new TextEncoder().encode(""))).toBe("fnv1a32:811c9dc5");
    expect(fnv1a32(new TextEncoder().encode("a"))).toBe("fnv1a32:e40c292c");
    expect(fnv1a32(new TextEncoder().encode("foobar"))).toBe("fnv1a32:bf9cf968");
  });

  it.each(corpora)("$fixture is what $dart last emitted", ({ dart, fixture }) => {
    // Bytes, not parsed JSON. The Dart side hashes the exact text it printed,
    // including its two-space indent and its trailing newline, so a fixture that
    // round-tripped through a formatter is drift even when it parses equal.
    const onDisk = fnv1a32(readFileSync(`${fixtureRoot}${fixture}`));
    expect(onDisk, `${fixture} is stale. Regenerate it from ${dart} — see the README beside it.`).toBe(
      declaredChecksum(dart),
    );
  });
});
