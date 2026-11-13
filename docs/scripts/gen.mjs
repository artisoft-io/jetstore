// npm run gen: writes the generated reference pages into
// src/content/docs/developer/reference/generated/ (git-ignored) from JetStore's
// own source files. Exits non-zero when a source is missing or cannot be read.
//
// With JETS_WORKSPACE_DB naming a compiled workspace.db, it also writes that
// workspace's guide into src/content/docs/workspace/ and its manifest into
// public/workspace/ (both git-ignored; see workspace_docs.mjs):
//   JETS_WORKSPACE_NAME  the guide's name, the workspace's (default: the database's folder)
//   JETS_WORKSPACE_DOCS  the workspace's docs/ folder, copied into the guide when present
// Without JETS_WORKSPACE_DB both folders are removed, so the site is JetStore's
// three guides alone: the public build.
import { mkdirSync, rmSync } from 'node:fs';
import { relative } from 'node:path';
import { OUT_DIR, REPO_ROOT } from './lib.mjs';
import { generatePipelineConfiguration } from './gen-pipeline-configuration.mjs';
import { generateSystemTables } from './gen-system-tables.mjs';
import { generateJetrulesOperators } from './gen-jetrules-operators.mjs';
import { WS_DIR, WS_PUBLIC_DIR, generateWorkspaceGuide } from './workspace_docs.mjs';

const generators = [generatePipelineConfiguration, generateSystemTables, generateJetrulesOperators];

try {
	rmSync(OUT_DIR, { recursive: true, force: true });
	mkdirSync(OUT_DIR, { recursive: true });
	for (const generate of generators) {
		const written = generate();
		console.log(`gen: ${relative(REPO_ROOT, written)}`);
	}

	// A guide left by an earlier run must not reach a build without a workspace.
	rmSync(WS_DIR, { recursive: true, force: true });
	rmSync(WS_PUBLIC_DIR, { recursive: true, force: true });
	const dbPath = process.env.JETS_WORKSPACE_DB?.trim();
	if (dbPath) {
		const docsDir = process.env.JETS_WORKSPACE_DOCS?.trim() || undefined;
		const { pages, manifest, counts, ruleText } = generateWorkspaceGuide({
			dbPath,
			name: process.env.JETS_WORKSPACE_NAME,
			docsDir,
		});
		console.log(
			`gen: ${relative(REPO_ROOT, WS_DIR)}: the ${manifest.workspace} guide, ${pages.length} pages ` +
				`(${counts.classes} classes, ${counts.lookups} lookups, ${counts.rules} rules, ` +
				`${counts.described} described; rule text ${ruleText ? 'shown' : 'not shown'})` +
				(docsDir ? '' : '; no JETS_WORKSPACE_DOCS'),
		);
		console.log(`gen: ${relative(REPO_ROOT, WS_PUBLIC_DIR)}/manifest.json`);
	} else if (process.env.JETS_WORKSPACE_DOCS?.trim()) {
		console.log('gen: JETS_WORKSPACE_DOCS is ignored: JETS_WORKSPACE_DB is not set');
	}
} catch (err) {
	console.error(`gen: ${err.message}`);
	process.exit(1);
}
