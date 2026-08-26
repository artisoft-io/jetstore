/**
 * What this app renders for each compiled view the apiserver declares. Task C.3.
 *
 * **This is the third reader of a declaration no corpus can reach**, and the
 * first one in this repository. The Workspace IDE's section list is *server*
 * data — one row of `wsfile.WorkspaceSections` per heading — so the Flutter
 * suite's five generated corpora, all built by walking client registries, close
 * over none of it. C.1 guarded it with two tests that check each other:
 * `jets/datatable/wsfile/sections_test.go` and
 * `jetsclient/test/workspace_section_contract_test.dart` carry the same
 * declaration string and the same checksum, so changing the Go table fails the
 * Go test and updating the Go constant alone fails the Dart one.
 *
 * **C.1's note said a third reader owes the same treatment, and this is it.**
 * `sectionContract.test.ts` is the mirror of the Dart file, one repository over.
 *
 * ## The two honest answers, and why an empty set would be the wrong shape
 *
 * For every non-empty compiled view the server declares, this client either
 * renders it — an entry in `compiledViews` — or names it in
 * `viewsNotBuiltInReact`. Nothing may be in both, and nothing may be in neither.
 * That is C.1's `viewsNotBuiltInFlutter` on this side of the port, and the
 * asymmetry between the two lists is the interesting part: `lookups` is in the
 * Dart's list permanently, because track X deletes that app rather than teaching
 * it the view, and it is in this one only until C.3a lands.
 */

import dataModelView from "./views/data_model.view.json";
import jetRulesView from "./views/jet_rules.view.json";
import lookupsView from "./views/lookups.view.json";
import { parseCompiledView, type CompiledViewDocument } from "./compiledView";

/**
 * The section contract as the apiserver declares it, one `dir=compiledView` per
 * section in display order.
 *
 * **Copied from `wsfile.SectionDeclaration()`, not derived from anything here.**
 * Deriving it from this app's own registry is the mistake the file exists to
 * avoid — it would agree with the client whatever the server sends. Recorded
 * 2026-08-25 against `jetstore_ai` at `worktree-ui-c3-workspacehome`, and
 * identical to the copy in `jetsclient/test/workspace_section_contract_test.dart`.
 */
export const SERVER_SECTION_DECLARATION = [
  "data_model=data_model",
  "jet_rules=jet_rules",
  "lookups=lookups",
  "pipes_config=",
  "user_flows=",
  "table_configs=",
  "process_config=",
  "reports=",
  "",
].join("\n");

/**
 * Duplicated in `jets/datatable/wsfile/sections_test.go` and in
 * `jetsclient/test/workspace_section_contract_test.dart`, both as
 * `expectedDeclarationChecksum`. Update all three together or none.
 */
export const EXPECTED_DECLARATION_CHECKSUM = "fnv1a32:b3fc79eb";

/**
 * FNV-1a, 32 bit, over the UTF-8 bytes — the same function the Go and Dart sides
 * hand-roll, for the same reason: this is drift detection between three
 * codebases, not a security boundary.
 */
export function fnv1a32(s: string): string {
  let hash = 0x811c9dc5;
  for (const byte of new TextEncoder().encode(s)) {
    hash ^= byte;
    // The FNV prime, 16777619, as shifts: the product overflows a double before
    // it wraps, so `hash * 16777619` gives the wrong answer in JavaScript.
    hash =
      (hash +
        ((hash << 1) >>> 0) +
        ((hash << 4) >>> 0) +
        ((hash << 7) >>> 0) +
        ((hash << 8) >>> 0) +
        ((hash << 24) >>> 0)) >>>
      0;
  }
  return `fnv1a32:${hash.toString(16).padStart(8, "0")}`;
}

/** `(directory, compiled view)` pairs; an empty view means the section has none. */
export function parseDeclaration(): { dir: string; view: string }[] {
  return SERVER_SECTION_DECLARATION.trim()
    .split("\n")
    .map((line) => {
      const at = line.indexOf("=");
      return { dir: line.slice(0, at), view: line.slice(at + 1) };
    });
}

/**
 * The compiled views this app renders, keyed by the value the server sends.
 *
 * The tab labels are **transcribed from the Dart and are in no corpus** — see
 * `compiledView.ts` for the mechanism and **I-205** for why it is a third shape
 * of corpus loss rather than an instance of C.0b's:
 *
 * - `data_model` — `FormKeys.wsDataModelForm`, `formTabsConfig`
 *   (`jetsclient/lib/modules/workspace_ide/form_config.dart`, `wsDataModelForm`):
 *   *Domain Classes*, *Data Properties*, *Domain Tables*, *Data Model Files*.
 * - `jet_rules` — `FormKeys.wsJetRulesForm`, `formTabsConfig` in the same file:
 *   *Jet Rules*, *Rule Terms*, *Files Relationship*, *Jet Rules Files*.
 * - `lookups` — **transcribed from nothing.** The Flutter app never built this
 *   view; `FormKeys.wsLookupsForm` and `DTKeys.wsLookupsTable` were declared,
 *   empty, and deleted by C.1. So its two labels are authored beside its two
 *   table documents (C.3a), and the pair to check them against is
 *   `jets/workspace_schema.sql` rather than a form registry.
 *
 * The section labels are `wsfile.WorkspaceSections`' `Label`, which is what the
 * heading already reads in the tree — "Jets Rules", not "Jet Rules".
 */
export const compiledViews: Record<string, CompiledViewDocument> = {
  data_model: parseCompiledView(dataModelView),
  jet_rules: parseCompiledView(jetRulesView),
  lookups: parseCompiledView(lookupsView),
};

/**
 * Compiled views the server declares that this app does not render.
 *
 * **Empty as of C.3a, and kept.** It held `lookups` for one task: that view
 * compiles into `lookup_tables` and `lookup_columns` and was routed here rather
 * than to the Flutter app because track X deletes that one (**I-45**, decided
 * 2026-08-23 by the user). C.3a built it, so the set emptied — which is the
 * outcome a debt with a task against it is supposed to have, and the reason it
 * was a set rather than a comment.
 *
 * It is not deleted, because what it asserts is *every declared view has been
 * answered*, and the ninth section is the reader that needs it.
 */
export const viewsNotBuiltInReact: ReadonlySet<string> = new Set<string>();

/**
 * Tables of a declared view that this app does not draw yet, and the task that
 * will.
 *
 * **Empty as of C.3b, and kept**, on `viewsNotBuiltInReact`'s terms one level
 * down. It held `wsDataModelFilesTable` and `wsJetRulesFilesTable` for one task:
 * both carry an action bar whose *Add File* opens a dialog and whose *Delete* is
 * gated on a selected row, and the app had no dialog host (**I-68**) when C.3
 * drew the other six tabs.
 *
 * **What it bought is the reason it was a set rather than a comment.** A view
 * that silently shows three of four tabs is a screen that looks complete — the
 * tab strip has no gap in it, and nothing in the rendered page says a table is
 * missing. `compiledView.test.ts` asserted drawn + deferred against the corpus
 * *in order*, so the omission was checked rather than remembered, and emptying
 * this constant turned that assertion into a plain equality without a line of it
 * being rewritten.
 */
export const TABS_DEFERRED_TO_C3B: Readonly<Record<string, readonly string[]>> = {};

/** The document for a section's declared view, or null when this app has none. */
export function compiledViewFor(compiledView: string | undefined): CompiledViewDocument | null {
  if (compiledView === undefined || compiledView === "") return null;
  return compiledViews[compiledView] ?? null;
}
