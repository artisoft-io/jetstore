/**
 * The compiled-view documents, against the corpus they are transcribed from.
 * Task C.3.
 *
 * **Half of one of these documents cannot be checked against anything, and this
 * file's job is to be precise about which half.** `allFields`
 * (`jetsclient/test/corpus_support.dart`, `allFields`) maps `formTabsConfig` to
 * `tab.inputField`, so `screen_configs.json` reports `workspace.data_model.form`
 * as four flat `FormDataTableFieldConfig` and `FormTabConfig.label` is in nothing
 * generated. The **table keys and their order** are therefore measured here; the
 * **labels** are transcribed from the Dart and cited in `sectionContract.ts`.
 *
 * That is a third shape of corpus loss and it is worth naming, because C.0b's fix
 * could not have caught it. C.0b found a container that was **not walked**
 * (`FormConfig.actions`) and a branch that **stripped** what it walked
 * (`fieldToJson` having no `FormActionConfig` case). This is a container that
 * **is** walked, for its contents, with its own structure discarded — and the
 * fixture is stable and complete about the thing it decided to carry, so no
 * checksum moves and nothing looks wrong. **I-205.**
 */

import { describe, expect, it } from "vitest";

import screenCorpus from "./fixtures/screen_configs.json";
import { makeQuery } from "../datatable/query";
import { fromDocument } from "../datatable/tableTranslate";
import { CompiledViewDocumentSchema } from "./compiledView";
import { COMPILED_VIEW_TABLES } from "./CompiledView";
import { compiledViews, TABS_DEFERRED_TO_C3B } from "./sectionContract";

interface CorpusField {
  type: string;
  dataTableConfig?: string;
}
const forms = (screenCorpus as { forms: Record<string, { fields: CorpusField[] }> }).forms;

/** The table keys one of the Dart's tabbed forms holds, in declaration order. */
function corpusTables(formKey: string): string[] {
  const form = forms[formKey];
  if (form === undefined) throw new Error(`${formKey} is not in the screen corpus`);
  return form.fields
    .filter((f) => f.type === "FormDataTableFieldConfig")
    .map((f) => f.dataTableConfig!);
}

/**
 * The Dart form key a compiled view corresponds to.
 *
 * **`lookups` has none, and that absence is the whole of C.3a.** The Flutter app
 * declared `FormKeys.wsLookupsForm` and never registered it; C.1 deleted the
 * constant. So there is no corpus entry to compare that view's tabs against, and
 * the cases below skip it by construction rather than by an exclusion list — a
 * view whose key is missing here has nothing to be measured against, which is a
 * different claim from a view whose tabs disagree.
 */
const DART_FORM_KEY: Record<string, string> = {
  data_model: "workspace.data_model.form",
  jet_rules: "workspace.jet_rules.form",
};

describe("the compiled view documents", () => {
  it("all parse", () => {
    for (const [key, doc] of Object.entries(compiledViews)) {
      expect(CompiledViewDocumentSchema.safeParse(doc).success, key).toBe(true);
    }
  });

  it("name only tables this app bundles", () => {
    for (const doc of Object.values(compiledViews)) {
      for (const tab of doc.tabs) {
        expect(Object.keys(COMPILED_VIEW_TABLES)).toContain(tab.table);
      }
    }
  });

  it("bundles no table no view names, so the import list cannot outlive a tab", () => {
    const named = new Set(
      Object.values(compiledViews).flatMap((doc) => doc.tabs.map((t) => t.table)),
    );
    expect(Object.keys(COMPILED_VIEW_TABLES).filter((k) => !named.has(k))).toEqual([]);
  });

  /**
   * **The assertion the deferral rests on.** A view that silently shows three of
   * four tabs is a screen that looks complete, so the tabs C.3b will add are
   * named rather than omitted, and drawn + deferred has to equal the Dart's list
   * *in order*. When C.3b lands, `TABS_DEFERRED_TO_C3B` empties and this case
   * becomes an equality — it does not have to be rewritten.
   */
  it("account for every table the Dart form holds, drawn or deferred, in order", () => {
    for (const [view, doc] of Object.entries(compiledViews)) {
      const formKey = DART_FORM_KEY[view];
      if (formKey === undefined) continue;
      const drawn = doc.tabs.map((t) => t.table);
      const deferred = TABS_DEFERRED_TO_C3B[view] ?? [];
      expect([...drawn, ...deferred], view).toEqual(corpusTables(formKey));
    }
    // The skip is asserted rather than silent: exactly one view has no Dart form,
    // and if a second ever does, this fails and somebody says why.
    expect(Object.keys(compiledViews).filter((v) => DART_FORM_KEY[v] === undefined)).toEqual([
      "lookups",
    ]);
  });

  it("defers exactly the two tables that carry an action bar, and no others", () => {
    // The reason for the deferral, measured rather than asserted: these are the
    // only two of the eight with any action at all, and both carry a `showDialog`
    // naming a form this app has no host for (I-68).
    const tables = (screenCorpus as { tables: Record<string, { actions: unknown[] }> }).tables;
    const eight = Object.values(DART_FORM_KEY).flatMap((f) => corpusTables(f));
    expect(eight.length).toBe(8);
    const withActions = eight.filter((k) => tables[k]!.actions.length > 0);
    expect(withActions).toEqual(["wsDataModelFilesTable", "wsJetRulesFilesTable"]);
    expect(Object.values(TABS_DEFERRED_TO_C3B).flat().sort()).toEqual(withActions.slice().sort());
  });

  /**
   * **The measurement behind rendering these tabs with `useDataTable` rather
   * than `useTableBinding`.**
   *
   * In the Dart every one of these eight tables *is* a form field — a
   * `FormDataTableFieldConfig` under `wsDataModelForm` or `wsJetRulesForm` — and
   * C.6 found the hard way that a screen table having a `formField` is not a
   * property of screens in general: the home screen's three are form fields, and
   * `makeQuery` compares that field key against four names before splicing the
   * home filters. So "a screen table has no form field" is not an assumption
   * this task is entitled to inherit.
   *
   * It is entitled to *measure* it, and the measurement is stronger than the
   * claim: for these six the payload is **identical** with and without a form
   * field. Three things in `makeQuery` read `formField` and none of them fires —
   * no where clause names a `formStateKey` (0 of 10 clauses), no key is one of
   * the four `KEYS` the filter blocks compare against, and the `workspaceName`
   * lookup falls back to `routeParams` when there is no form state to read.
   */
  it("query the same with a form field as without one, so a form buys these tables nothing", () => {
    for (const [tableKey, document] of Object.entries(COMPILED_VIEW_TABLES)) {
      const config = fromDocument(tableKey, document);
      const base = {
        config,
        routeParams: { workspace_name: "ws" },
        indexOffset: 0,
        rowsPerPage: 20,
        sortColumnName: config.sortColumnName,
        sortColumnTableName: config.sortColumnTableName,
        sortAscending: config.sortAscending,
      };
      const withoutField = makeQuery(base);
      const withField = makeQuery({
        ...base,
        formField: { group: 0, key: tableKey },
        formState: { getValue: () => undefined, isDialog: false },
        homeFilters: [{ column: "status", defaultValue: ["x"], lookupColumnInFormState: false }],
        dataRegistryFilters: [
          { column: "client", defaultValue: ["y"], lookupColumnInFormState: false },
        ],
      });
      expect(JSON.stringify(withField), tableKey).toBe(JSON.stringify(withoutField));
    }
  });

  it("are query tables with a real FROM, so none of them wants C.9's third source arm", () => {
    const tables = (
      screenCorpus as {
        tables: Record<
          string,
          {
            fromClauses: unknown[];
            modelStateFormKey?: string;
            hasModelStateHandler: boolean;
            columns: { name: string }[];
          }
        >;
      }
    ).tables;
    for (const key of Object.values(DART_FORM_KEY).flatMap((f) => corpusTables(f))) {
      const t = tables[key]!;
      expect(t.fromClauses.length, key).toBeGreaterThan(0);
      expect(t.modelStateFormKey, key).toBeUndefined();
      expect(t.hasModelStateHandler, key).toBe(false);
      // No `client` column, so the implicit client filter never applies and this
      // screen has no reason to read C.6's `selectedClient` store.
      expect(t.columns.some((c) => c.name === "client"), key).toBe(false);
    }
  });

  it("refuses a one-tab document, which is the rule about screens made checkable", () => {
    // "A screen with one table renders it; a screen with several declares them"
    // (I-177). `workspaceHome` and `workspaceRegistry` are the two the Dart
    // registers as forms and this app will not give a document to.
    const oneTab = {
      schemaVersion: 1,
      view: "workspace_home",
      label: "Workspace Home",
      tabs: [{ label: "Workspace Changes", table: "workspaceChangesTable" }],
    };
    expect(CompiledViewDocumentSchema.safeParse(oneTab).success).toBe(false);
  });

  it("refuses an unknown property, a missing label and a wrong schemaVersion", () => {
    const base = () => structuredClone(compiledViews["data_model"]!) as Record<string, unknown>;
    const rejects = (doc: unknown) =>
      expect(CompiledViewDocumentSchema.safeParse(doc).success).toBe(false);
    rejects({ ...base(), title: "Data Model" });
    rejects({ ...base(), schemaVersion: 2 });
    rejects({ ...base(), label: "" });
    rejects({ ...base(), tabs: [{ table: "wsDomainClassTable" }, { table: "wsDataPropertyTable" }] });
  });
});
