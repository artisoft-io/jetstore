/**
 * A compiled view: the tabbed strip of tables a Workspace IDE section heading
 * shows. Task C.3.
 *
 * ## What a compiled view is
 *
 * **The Workspace IDE shows a workspace from both sides of the compiler.** The
 * nodes below a section heading are the *source* files that go into `compilerv2`;
 * the heading itself is a view of `workspace.db`, the *compiled* artifact, which
 * is why Data Model opens on Domain Classes and Data Properties rather than on a
 * file list. C.1 made that duality explicit on the wire: a section declares its
 * `CompiledView` (`jets/datatable/wsfile/sections.go`, `WorkspaceSections`) and
 * `WorkspaceNode.compiled_view` carries it (`api/workspace.ts`, `WorkspaceNode`).
 *
 * This file is the third reader of that declaration, and `sectionContract.test.ts`
 * gives it the same treatment the other two have.
 *
 * ## Why this is not a `.form.json`
 *
 * The Dart registers each of these as a `FormConfig` with a `formTabsConfig` and
 * **no actions** (`jetsclient/lib/modules/workspace_ide/form_config.dart`,
 * `FormKeys.wsDataModelForm` and `FormKeys.wsJetRulesForm`), so the obvious port
 * is a form document. It is the wrong port, and the reason is that
 * `FormSchema.actions` is `.min(1)` **on purpose**: a flow form with no button is
 * a dead end, because nothing advances the state machine — and `jets/userflow`
 * enforces that schema on every `.form.json` saved into a workspace. Relaxing it
 * would make that document valid at save time and strand a user at run time, and
 * it would do so in a schema the agentic_ai project's generator writes to
 * (repository `CLAUDE.md`, *Where the two projects actually collide*).
 *
 * **The Dart conflates a form and a screen layout because `FormConfig` was the
 * only container it had.** Reproducing that in a schema written from scratch
 * would be inheriting a limitation rather than porting a design. So: a form is a
 * thing with fields and buttons that submits, and stays `FormDocumentSchema`;
 * arranging tables is this. Ruled 2026-08-25 (**I-177**).
 *
 * ## Why there is no entry for a screen with one table
 *
 * `workspaceHome` and `workspaceRegistry` are registered as forms too, each with
 * a single `FormDataTableFieldConfig` and no actions — and that registration
 * carries no information at all once a screen is not a form. **A screen with one
 * table renders it; a screen with several declares them.** That is why `tabs` is
 * `.min(2)`: a one-table document could only exist to be uniform, and this schema
 * refuses to be the place a table key is written down twice.
 */

import { z } from "zod";

import { Identifier } from "../userflow/schema";

/**
 * One tab: a label and the table configuration key it shows.
 *
 * **The label is transcribed from the Dart and is in no corpus**, which is worth
 * stating where the value is used rather than only in a register entry.
 * `allFields` (`jetsclient/test/corpus_support.dart`, `allFields`) maps
 * `formTabsConfig` to `tab.inputField`, so `screen_configs.json` reports these
 * forms as flat lists of `FormDataTableFieldConfig` and *Domain Classes*, *Data
 * Properties*, *Domain Tables*, *Jet Rules*, *Rule Terms* and *Files
 * Relationship* appear in nothing generated. `compiledView.test.ts` therefore
 * checks the table keys and their order against the corpus and cannot check the
 * labels; each document cites its Dart line instead (**I-205**).
 */
export const CompiledViewTabSchema = z
  .strictObject({
    label: z.string().min(1),
    /** A table configuration key; the document is imported by `compiledViews`. */
    table: Identifier,
  })
  .meta({ id: "CompiledViewTab", description: "One tab of a compiled view" });

export const CompiledViewDocumentSchema = z
  .strictObject({
    schemaVersion: z.literal(1),
    /**
     * The `CompiledView` value this document renders, which is what the server
     * sends on a section node. It is in the document rather than only in the
     * file name so that the contract test compares two declarations rather than
     * a declaration and a directory listing.
     */
    view: Identifier,
    /** The heading the opened tab carries. */
    label: z.string().min(1),
    tabs: z.array(CompiledViewTabSchema).min(2),
  })
  .meta({
    id: "CompiledViewDocument",
    title: "JetStore Workspace IDE compiled view",
    description: "The tables a Workspace IDE section heading shows",
  });

export type CompiledViewTab = z.infer<typeof CompiledViewTabSchema>;
export type CompiledViewDocument = z.infer<typeof CompiledViewDocumentSchema>;

/**
 * Parse a document, throwing with the schema's own message when it does not fit.
 *
 * These documents are **bundled, never workspace files** — there is one Data
 * Model view per deployment, not one per workspace — so nothing on the Go side
 * validates them and there is no `.schema.json` artifact beside this file. That
 * is the same argument I-170 makes for a screen's table documents, and it is why
 * this construct costs one schema rather than a schema plus a validator row.
 */
export function parseCompiledView(doc: unknown): CompiledViewDocument {
  return CompiledViewDocumentSchema.parse(doc);
}
