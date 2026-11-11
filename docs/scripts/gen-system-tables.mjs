// The system tables of the jetsapi schema, from the schema file the database
// migration (update_db) applies.
import { join } from 'node:path';
import { OUT_DIR, code, prose, readSource, table, writePage } from './lib.mjs';

const SOURCE = 'jets/jets_schema.json';

export function generateSystemTables() {
	let tables;
	try {
		tables = JSON.parse(readSource(SOURCE));
	} catch (err) {
		throw new Error(err instanceof SyntaxError ? `${SOURCE}: ${err.message}` : err.message);
	}
	if (!Array.isArray(tables) || tables.length === 0) throw new Error(`${SOURCE}: no tables found`);

	const live = (x) => !x.deleted;
	const sections = tables
		.filter(live)
		.sort((a, b) => a.tableName.localeCompare(b.tableName))
		.map((t) => {
			const rows = t.columns.filter(live).map((c) => [
				code(c.columnName),
				code(c.dataType + (c.isArray ? '[]' : '')),
				c.isPK ? 'primary key' : c.isNotNull ? 'not null' : '',
				c.default !== undefined ? code(c.default) : '',
				[c.deprecated ? '**Deprecated.**' : '', prose(c.description)].filter(Boolean).join(' '),
			]);
			const parts = [`## ${t.tableName}`];
			if (t.description) parts.push(prose(t.description));
			parts.push(table(['Column', 'Type', 'Constraint', 'Default', 'Description'], rows));
			const extras = [
				...(t.tableConstraints ?? []).filter(live).map((k) => k.definition),
				...(t.indexes ?? []).filter(live).map((i) => i.indexDef),
			];
			if (extras.length) parts.push(`Constraints and indexes:\n\n${extras.map((d) => `- ${code(d)}`).join('\n')}`);
			return parts.join('\n\n');
		});

	const schemas = [...new Set(tables.map((t) => t.schemaName))].map(code).join(', ');
	const body = [
		`JetStore's own tables, in the ${schemas} schema of its PostgreSQL database: ${sections.length} tables. ` +
			'The database migration creates and updates them from the same file when an installation is deployed or upgraded. ' +
			'A primary-key column is also not null. Columns that have been removed from the schema are not listed.',
		...sections,
	].join('\n\n');

	const file = 'system-tables.md';
	writePage(file, {
		title: 'System tables',
		description: "The tables of JetStore's jetsapi schema, their columns, constraints and indexes.",
		sources: [SOURCE],
		body,
	});
	return join(OUT_DIR, file);
}
