/**
 * Picks a CodeMirror language from a file name.
 *
 * The extensions here are the ones the workspace tree actually serves — the Go
 * visitor filters to `.jr`, `.csv`, the pipeline configs, and the `.sql` under
 * `process_config/` and `reports/` — so this is a closed set rather than a
 * general-purpose registry.
 *
 * A `.jr.sql` arm sat at the top of this until 2026-09-12, deleted with the
 * file type it served — and it was redundant as well as unused. See the
 * `jet_rules` row in the Go visitor's `wsfile/sections.go` for why.
 */

import type { Extension } from "@codemirror/state";
import { json } from "@codemirror/lang-json";
import { sql } from "@codemirror/lang-sql";
import { jetRules } from "./jetrules";

export type LanguageName = "jetrules" | "json" | "sql" | "plain";

export function languageNameFor(fileName: string): LanguageName {
  const name = fileName.toLowerCase();
  if (name.endsWith(".jr")) return "jetrules";
  if (name.endsWith(".json")) return "json";
  if (name.endsWith(".sql")) return "sql";
  return "plain";
}

export function languageExtension(fileName: string): Extension[] {
  switch (languageNameFor(fileName)) {
    case "jetrules":
      return [jetRules()];
    case "json":
      return [json()];
    case "sql":
      return [sql()];
    case "plain":
      return [];
  }
}

/**
 * Whether the server will parse this file before agreeing to save it.
 * `SaveWorkspaceFileContent` rejects a `.json` file that does not parse, so the
 * editor can warn about that locally instead of relying on a 400.
 */
export function isServerValidatedJson(fileName: string): boolean {
  return fileName.toLowerCase().endsWith(".json");
}
