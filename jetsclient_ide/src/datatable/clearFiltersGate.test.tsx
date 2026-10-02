/**
 * @vitest-environment jsdom
 *
 * **The *Clear Filters* gate.** `jetstore_maintenance_02`, defect `D02` and its
 * neighbour, tasks `AA.1` to `AA.3`, 2026-10-01.
 *
 * Two defects in one gate, in opposite directions. Pipeline Status's button was
 * **never** gated, because the action bar looked predicates up by action key in a
 * one-entry map that had no entry for `clearHomeFilters`. Data Registry's and
 * both *Start Pipeline* pickers' buttons could **never** enable, because the
 * predicate they name was a hard-coded `false`. No test asserted either: until
 * this file and the `Home.test.tsx` cases beside it, nothing anywhere asserted
 * *Clear Filters*' enabled state.
 *
 * `Home.test.tsx` covers the two Home tables in the screen that draws them. This
 * file covers the pickers — **the picker is where two of the three
 * data-registry buttons live**, and a fix tested only on Home would leave them
 * as unexamined as they were — and holds the invariant that keeps the runtime's
 * mapping and the documents' `isEnabled` from drifting apart.
 */

import { cleanup, render, screen } from "@testing-library/react";
import { afterEach, describe, expect, it, vi } from "vitest";

import { productionRegistry } from "../actions/registry";
import { resetHomeFilters, updateHomeFilters } from "../actions/homeFilters";
import { ApiProvider } from "../shell/capabilities";
import mainInputRegistryKey from "../../../jets/workspace_assets/table_configs/main_input_registry_key.tc.json";
import mergedInputRegistryKeys from "../../../jets/workspace_assets/table_configs/merged_input_registry_keys.tc.json";
import { FormState } from "./formState";
import type { TableConfigDocument } from "./table";
import { TableView } from "./TableView";
import { fromDocument, isEnabledEscapeFor } from "./tableTranslate";

afterEach(() => {
  cleanup();
  resetHomeFilters();
});

/**
 * Applies the home-filters dialog's answers, as `homeFiltersUF` does.
 *
 * `status` alone narrows Pipeline Status and nothing else; a start offset
 * narrows both it and the three data-registry tables (`homeFilters.ts`,
 * `updateHomeFilters`). So the two seeds below are what tell the two predicates
 * apart — a gate wired to the wrong store passes one of them and fails the other.
 */
function applyHomeFilters(answers: Record<string, unknown>): void {
  const formState = new FormState();
  for (const [key, value] of Object.entries(answers)) formState.setValue(0, key, value as never);
  void updateHomeFilters({ formState, group: 0, flowKey: "homeFiltersUF" });
}

const HOME_ONLY = { status: ["failed"] };
const BOTH = { status: ["failed"], hfStartOffset: "3 days" };

describe("the Start Pipeline pickers' Clear Filters", () => {
  const api = { can: () => true, isAuthenticated: () => true } as never;
  const fetcher = vi.fn(async () => ({ rows: [], totalRowCount: 0 }));

  const pickers = [
    ["main_input_registry_key", mainInputRegistryKey],
    ["merged_input_registry_keys", mergedInputRegistryKeys],
  ] as const;

  /** Mounts the picker as `startPipelineUF`'s form does, under the real registry. */
  function clearFiltersOn(key: string, doc: unknown): HTMLButtonElement {
    render(
      <ApiProvider api={api}>
        <TableView
          config={fromDocument(key, doc as TableConfigDocument)}
          field={{ group: 0, key }}
          formState={new FormState()}
          fetcher={fetcher}
          predicates={productionRegistry.predicates}
          cellFilters={productionRegistry.cellFilters}
          onAction={vi.fn()}
        />
      </ApiProvider>,
    );
    return screen.getByRole("button", { name: "Clear Filters" }) as HTMLButtonElement;
  }

  for (const [key, doc] of pickers) {
    it(`${key}: disabled when no data-registry filter is set`, () => {
      expect(clearFiltersOn(key, doc).disabled).toBe(true);
    });

    it(`${key}: still disabled when only a home filter is set`, () => {
      // `status` narrows Pipeline Status only. A picker gated on the home filters
      // would enable here and offer to clear a filter it is not applying.
      applyHomeFilters(HOME_ONLY);
      expect(clearFiltersOn(key, doc).disabled).toBe(true);
    });

    it(`${key}: enabled once a data-registry filter is set`, () => {
      // **Reverting AA.2 turns this red**: `hasDataRegistryFilters` as `false`.
      applyHomeFilters(BOTH);
      expect(clearFiltersOn(key, doc).disabled).toBe(false);
    });
  }
});

/**
 * **The runtime derives the predicate's name; the document states it.** They
 * have to agree, and nothing made them until this test.
 *
 * `fromDocument` keeps only the boolean `hasIsEnabledFnc` (`ui_refresh`'s I-66:
 * the translation must lose nothing, so `ActionConfig` carries no field the
 * corpus cannot express), so the action bar asks `isEnabledEscapeFor` at runtime
 * — the function the emitter used to *write* `isEnabled`. For a document the
 * fixture emits, that is agreement by construction. **For a hand-authored one it
 * is not**, and `jetstore_maintenance_02` moves documents out of the fixture
 * (`Q-5`) and expects them to be written by a model: a document naming a
 * predicate the derivation would not choose would render with the derivation's
 * gate and no error. So the agreement is asserted over every committed document,
 * whoever wrote it.
 */
describe("every committed table document's isEnabled", () => {
  // Both directories a committed table document lives in — the bundled screens'
  // and the workspace assets — by glob rather than by list, so a document added
  // later is covered without anyone remembering to add it. `import.meta.glob`
  // rather than `readdirSync`, because this file runs under jsdom, where
  // `import.meta.url` is not a `file:` URL (`routes.test.ts` does the same).
  type ActionDoc = { key: string; isEnabled?: string };
  type Doc = {
    default: { actions?: ActionDoc[]; secondRowActions?: ActionDoc[]; thirdRowActions?: ActionDoc[] };
  };
  const documents = {
    ...(import.meta.glob("./tables/*.tc.json", { eager: true }) as Record<string, Doc>),
    ...(import.meta.glob("../../../jets/workspace_assets/table_configs/*.tc.json", {
      eager: true,
    }) as Record<string, Doc>),
  };

  const sites = Object.entries(documents).flatMap(([path, { default: doc }]) => {
    const key = path.slice(path.lastIndexOf("/") + 1, -".tc.json".length);
    // Every row, the third (`D04`, `AE.1`) included: a hand-authored button there
    // naming a predicate the derivation would not choose is exactly the drift
    // this test exists for.
    return [...(doc.actions ?? []), ...(doc.secondRowActions ?? []), ...(doc.thirdRowActions ?? [])]
      .filter((a) => a.isEnabled !== undefined)
      .map((a) => ({ table: key, action: a.key, authored: a.isEnabled! }));
  });

  it("is what the action bar resolves at runtime", () => {
    // Six sites on 2026-10-01: Data Registry's, the two pickers', and Pipeline
    // Status's three. Asserted as non-empty rather than as six, because the
    // number is a fact about the corpus and the agreement is the invariant.
    expect(sites.length).toBeGreaterThan(0);
    for (const site of sites) {
      expect({ ...site, runtime: isEnabledEscapeFor(site.table, site.action) }).toEqual({
        ...site,
        runtime: site.authored,
      });
    }
  });

  it("names a predicate the production registry holds", () => {
    for (const site of sites) {
      expect(productionRegistry.predicates[site.authored], site.authored).toBeTypeOf("function");
    }
  });
});
