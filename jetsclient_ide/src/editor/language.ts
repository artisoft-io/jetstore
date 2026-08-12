/**
 * Picks a CodeMirror language from a file name.
 *
 * The extensions here are the ones the workspace tree actually serves — the Go
 * visitor filters to `.jr`, `.jr.sql`, `.csv`, plus the pipeline configs — so this
 * is a closed set rather than a general-purpose registry. `.jr.sql` has to be
 * tested before `.sql`, and both before the bare-extension fallback.
 */

import type { Extension } from "@codemirror/state";
import { json } from "@codemirror/lang-json";
import { sql } from "@codemirror/lang-sql";
import { jetRules } from "./jetrules";

export type LanguageName = "jetrules" | "json" | "sql" | "plain";

export function languageNameFor(fileName: string): LanguageName {
  const name = fileName.toLowerCase();
  // `.jr.sql` is a JetRules-flavoured sql file; sql highlighting is the better fit
  // for its body, and it must not fall through to the `.jr` branch.
  if (name.endsWith(".jr.sql")) return "sql";
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
