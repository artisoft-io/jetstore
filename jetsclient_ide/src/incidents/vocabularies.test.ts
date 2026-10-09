/**
 * The vocabularies the incident screens spell, against the vocabularies the
 * database enforces.
 *
 * **A file of its own because it needs the node environment.** Under jsdom
 * `import.meta.url` is an http url and `fileURLToPath` refuses it, so a test that
 * reads a file cannot sit beside the component tests — which is why
 * `incidents.test.tsx` is jsdom and this is not.
 */

import { readFileSync } from "node:fs";
import { fileURLToPath } from "node:url";

import { describe, expect, it } from "vitest";

import { AGENT_PHI_ACCESS, ADJUDICATED_STATUSES, LOCUS_GLOSS, OPEN_STATUSES } from "./api";

// The one test here that is not about a browser.
//
// **The gloss table and the two filter lists spell vocabularies that are defined
// in Postgres**, generated from the domain model by `jets-agentic generate`. The
// Go side asserts its own copies against those CHECK constraints
// (`TestIncidentLociMatchTheGeneratedCheck`); nothing asserted the browser's, and
// a locus added by a regeneration would reach this app as a value with no gloss
// and no filter — which degrades quietly by design and is still worth knowing
// about. So this reads the generated SQL directly, the way
// `sharedTableDocuments.test.ts` reads the installed table documents.
describe("the vocabularies this app spells", () => {
  const sql = readFileSync(
    fileURLToPath(new URL("../../../jets/agentic/audit/agent_audit.sql", import.meta.url)),
    "utf8",
  );

  function checkVocabulary(pattern: RegExp): string[] {
    const m = sql.match(pattern);
    if (!m?.[1]) throw new Error(`no CHECK matching ${pattern}; this test is stale`);
    return m[1].split(",").map((s) => s.trim().replace(/^'|'$/g, ""));
  }

  it("glosses every locus the CHECK admits, and no others", () => {
    const inCheck = checkVocabulary(/incident_locus_ck CHECK \(incident_locus IN \(([^)]*)\)\)/);
    expect([...Object.keys(LOCUS_GLOSS)].sort()).toEqual([...inCheck].sort());
  });

  it("filters only on statuses the CHECK admits", () => {
    const inCheck = new Set(checkVocabulary(/incident_status_ck CHECK \(status IN \(([^)]*)\)\)/));
    for (const s of [...OPEN_STATUSES, ...ADJUDICATED_STATUSES]) {
      expect(inCheck.has(s), `${s} is not an incident status`).toBe(true);
    }
    // The two lists are disjoint: a status cannot be both open work and a
    // verdict on finished work.
    expect(OPEN_STATUSES.filter((s) => ADJUDICATED_STATUSES.includes(s))).toEqual([]);
  });
});

/**
 * The PHI marker, from the browser's end (task AE.2, I-311).
 *
 * **The screen renders a withholding notice for one field, and it has no way of
 * its own to know that field is the marked one.** The generated manifest is that
 * statement, so this reads it — the same move as the CHECK above, one file over.
 * If a regeneration marks a second property, this fails and names it, which is
 * what stops the browser from quietly withholding one of two.
 */
describe("the PHI marker this app renders a notice for", () => {
  const generated = readFileSync(
    fileURLToPath(new URL("../../../jets/agentic/audit/data_classification.go", import.meta.url)),
    "utf8",
  );

  const entries = [...generated.matchAll(
    /\{Entity: "([^"]+)", Property: "([^"]+)", Classification: "([^"]+)"\}/g,
  )].map(([, entity, property, classification]) => ({ entity, property, classification }));

  it("is Evidence.statement, and it is the only one", () => {
    expect(entries).toEqual([
      { entity: "Evidence", property: "statement", classification: "PHI" },
    ]);
  });

  // The capability name is the server's and this constant is only a fallback for
  // a response that predates `phiCapability` — but a fallback nobody checks is a
  // fallback that is wrong when it is finally used.
  it("names the capability the server documents", () => {
    const seeding = readFileSync(
      fileURLToPath(new URL("../../../jets/jets_init_db.sql", import.meta.url)),
      "utf8",
    );
    expect(seeding.includes(`- ${AGENT_PHI_ACCESS}:`)).toBe(true);
  });
});
