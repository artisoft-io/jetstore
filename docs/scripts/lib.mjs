// Shared helpers for the reference page generators.
import { existsSync, readFileSync, writeFileSync } from 'node:fs';
import { dirname, join, resolve } from 'node:path';
import { fileURLToPath } from 'node:url';

/** The root of the jetstore repository: docs/scripts/ is two levels below it. */
export const REPO_ROOT = resolve(dirname(fileURLToPath(import.meta.url)), '..', '..');

/** Where the generated pages go; git-ignored, and emptied on every run. */
export const OUT_DIR = join(REPO_ROOT, 'docs', 'src', 'content', 'docs', 'developer', 'reference', 'generated');

/** Read a source file named relative to the repository root, or fail the run. */
export function readSource(relPath) {
	const abs = join(REPO_ROOT, relPath);
	if (!existsSync(abs)) {
		throw new Error(`missing input: ${relPath} (looked for ${abs})`);
	}
	return readFileSync(abs, 'utf8');
}

/**
 * Escape Markdown outside code spans, and the table delimiter inside them, so
 * the text reads as written: on one line, and, where it starts with what would
 * begin a block (`# `, `- `, `+ `, `1. ` or `1) `, a thematic break such as
 * `---`), with that marker escaped. Text that is a paragraph, a list item or a
 * heading's content would otherwise become a heading, a list or a rule (`* `,
 * `_` and `> ` are escaped wherever they are). Leading spaces are dropped, as a
 * paragraph drops them, so that four of them cannot start a code block.
 */
export function prose(text) {
	if (!text) return '';
	return String(text)
		.split(/(`[^`]*`)/)
		.map((part, i) =>
			i % 2 === 1
				? part.replace(/\|/g, '\\|')
				: part
						.replace(/\\/g, '\\\\')
						.replace(/([*_|[\]])/g, '\\$1')
						.replace(/</g, '&lt;')
						.replace(/>/g, '&gt;'),
		)
		.join('')
		.replace(/\s*\n\s*/g, ' ')
		.replace(/^\s+/, '')
		.replace(/^([#+-])/, '\\$1')
		.replace(/^(\d{1,9})([.)])/, '$1\\$2');
}

/** A value as inline code, safe inside a table cell. */
export function code(text) {
	const s = String(text);
	const fence = s.includes('`') ? '`` ' : '`';
	const close = s.includes('`') ? ' ``' : '`';
	return fence + s.replace(/\|/g, '\\|') + close;
}

/** A Markdown table from a header row and body rows of already-formatted cells. */
export function table(header, rows) {
	const line = (cells) => `| ${cells.join(' | ')} |`;
	return [line(header), line(header.map(() => '---')), ...rows.map(line)].join('\n');
}

/** Write one generated page with its front matter and the generated-from line. */
export function writePage(fileName, { title, description, sources, body }) {
	const fm = [
		'---',
		`title: ${JSON.stringify(title)}`,
		`description: ${JSON.stringify(description)}`,
		'kind: reference',
		'editUrl: false',
		'---',
		'',
	].join('\n');
	const from = sources.map((s) => code(s)).join(' and ');
	const first = `*Generated from ${from} when this site was built. Do not edit this page: change its source.*`;
	writeFileSync(join(OUT_DIR, fileName), `${fm}${first}\n\n${body.trim()}\n`);
}
