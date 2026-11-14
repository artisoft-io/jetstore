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
 *
 * Markdown since 2026-11-13: the `docs` section's pages (`.md` and `.mdx`) are
 * edited here, and the READMEs elsewhere are shown read-only.
 *
 * **Front matter since 2026-11-14.** A page opens with a `---` block of YAML,
 * its title, description and kind, and Markdown alone reads that block's
 * closing `---` as the underline of a setext heading, so the fields showed as a
 * heading. `yamlFrontmatter` (lang-yaml) parses a leading `---` … `---` block
 * as YAML and the rest as Markdown, as the documentation build reads a page.
 */

import type { Extension } from "@codemirror/state";
import { json } from "@codemirror/lang-json";
import { sql } from "@codemirror/lang-sql";
import { markdownLanguage } from "@codemirror/lang-markdown";
import { yamlFrontmatter } from "@codemirror/lang-yaml";
import { LanguageSupport } from "@codemirror/language";
import { jetRules } from "./jetrules";

export type LanguageName = "jetrules" | "json" | "sql" | "markdown" | "plain";

export function languageNameFor(fileName: string): LanguageName {
  const name = fileName.toLowerCase();
  if (name.endsWith(".jr")) return "jetrules";
  if (name.endsWith(".json")) return "json";
  if (name.endsWith(".sql")) return "sql";
  if (name.endsWith(".md") || name.endsWith(".mdx")) return "markdown";
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
    case "markdown":
      // The language alone, not `markdown()`: that one also embeds an HTML
      // parser for inline tags (lang-html, with CSS and JavaScript). Measured
      // on 2026-11-13, `markdown()` added 203 kB to the bundle (77 kB gzipped)
      // and this adds 37 kB (12 kB). GFM, as the documentation site reads it.
      // The front matter's YAML parser adds 20 kB more (8 kB gzipped),
      // measured on 2026-11-14.
      return [yamlFrontmatter({ content: new LanguageSupport(markdownLanguage) })];
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
