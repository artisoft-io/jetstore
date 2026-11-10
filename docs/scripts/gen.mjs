// npm run gen: writes the generated reference pages into
// src/content/docs/developer/reference/generated/ (git-ignored) from JetStore's
// own source files. Exits non-zero when a source is missing or cannot be read.
import { mkdirSync, rmSync } from 'node:fs';
import { relative } from 'node:path';
import { OUT_DIR, REPO_ROOT } from './lib.mjs';
import { generatePipelineConfiguration } from './gen-pipeline-configuration.mjs';
import { generateSystemTables } from './gen-system-tables.mjs';
import { generateJetrulesOperators } from './gen-jetrules-operators.mjs';

const generators = [generatePipelineConfiguration, generateSystemTables, generateJetrulesOperators];

try {
	rmSync(OUT_DIR, { recursive: true, force: true });
	mkdirSync(OUT_DIR, { recursive: true });
	for (const generate of generators) {
		const written = generate();
		console.log(`gen: ${relative(REPO_ROOT, written)}`);
	}
} catch (err) {
	console.error(`gen: ${err.message}`);
	process.exit(1);
}
