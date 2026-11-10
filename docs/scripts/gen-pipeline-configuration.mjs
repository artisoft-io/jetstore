// The compute-pipes configuration (.pc.json), from the contract matrix in
// tools/cpipes_contract/matrix/: types.csv has one row per type (a Go struct, or
// one variant of it selected by a discriminator), fields.csv one row per field
// of a type, saying whether the field applies to that type.
//
// Only the reader-facing columns are used: the key, its type, whether it is
// required, its default, its closed set of values and its description. The
// matrix's evidence, corpus and review columns are the contract's audit trail,
// not documentation, and are deliberately left out.
import { join } from 'node:path';
import { parse } from 'csv-parse/sync';
import { OUT_DIR, code, prose, readSource, table, writePage } from './lib.mjs';

const TYPES = 'tools/cpipes_contract/matrix/types.csv';
const FIELDS = 'tools/cpipes_contract/matrix/fields.csv';
const ROOT = 'ComputePipesConfig';

function readCsv(relPath, required) {
	let rows;
	try {
		rows = parse(readSource(relPath), { columns: true, skip_empty_lines: true });
	} catch (err) {
		throw new Error(err.code?.startsWith('CSV_') ? `${relPath}: ${err.message}` : err.message);
	}
	const missing = required.filter((c) => rows.length === 0 || !(c in rows[0]));
	if (missing.length) throw new Error(`${relPath}: missing column(s) ${missing.join(', ')}`);
	return rows;
}

/** A matrix cell: `-` means none, and a blank is an unreviewed claim; both render empty. */
const cell = (v) => (v === '-' || v === undefined ? '' : v);

/** The anchor Starlight gives a heading: lower case, punctuation dropped, spaces as hyphens. */
const anchor = (heading) => heading.toLowerCase().replace(/[^\p{L}\p{N}\s_-]/gu, '').replace(/\s/g, '-');

export function generatePipelineConfiguration() {
	const types = readCsv(TYPES, ['go_struct', 'type_token', 'defs_name', 'discriminator', 'variant_when', 'deprecated', 'description']);
	const fields = readCsv(FIELDS, [
		'go_struct', 'type_token', 'json_key', 'go_type', 'container', 'ref_struct', 'values',
		'applicable', 'required', 'required_when', 'default', 'deprecated', 'description',
	]);

	// Types grouped by struct, in the matrix's order.
	const structs = new Map();
	for (const t of types) {
		if (!structs.has(t.go_struct)) structs.set(t.go_struct, []);
		structs.get(t.go_struct).push(t);
	}
	if (!structs.has(ROOT)) throw new Error(`${TYPES}: no row for the root type ${ROOT}`);

	const fieldsOf = new Map();
	for (const f of fields) {
		if (f.applicable !== 'yes') continue;
		const key = `${f.go_struct}/${f.type_token}`;
		if (!fieldsOf.has(key)) fieldsOf.set(key, []);
		fieldsOf.get(key).push(f);
	}

	// Only the structs an author can reach from the root through applicable
	// fields: the internal ones JetStore writes for itself are not documented.
	const reachable = new Set([ROOT]);
	const queue = [ROOT];
	while (queue.length) {
		for (const t of structs.get(queue.shift()) ?? []) {
			for (const f of fieldsOf.get(`${t.go_struct}/${t.type_token}`) ?? []) {
				const ref = cell(f.ref_struct);
				if (ref && structs.has(ref) && !reachable.has(ref)) {
					reachable.add(ref);
					queue.push(ref);
				}
			}
		}
	}

	// Every heading becomes an anchor; two that collide would make links ambiguous.
	const headings = new Set();
	const claim = (name) => {
		if (headings.has(anchor(name))) throw new Error(`${TYPES}: two sections would share the anchor #${anchor(name)}`);
		headings.add(anchor(name));
	};

	const typeOf = (f) => {
		const ref = cell(f.ref_struct);
		const inner = ref
			? reachable.has(ref)
				? `[${ref}](#${anchor(ref)})`
				: code(ref)
			: code(f.go_type.replace(/^[*[\]]+/, '').replace(/^map\[string\]\*?/, ''));
		switch (f.container) {
			case 'array':
				return `array of ${inner}`;
			case 'array2':
				return `array of arrays of ${inner}`;
			case 'map':
				return `map of ${inner}`;
			case 'any':
			case 'raw_json':
				return 'any JSON value';
			default:
				return inner;
		}
	};

	const requiredOf = (f) => {
		if (f.required === 'yes') return 'yes';
		if (f.required === 'conditional') {
			const when = cell(f.required_when)
				.replace(/^absent\((.+)\)$/, 'when `$1` is absent')
				.replace(/^present\((.+)\)$/, 'when `$1` is present');
			return when ? `${prose(when)}` : 'conditional';
		}
		return f.required === 'no' ? 'no' : '';
	};

	const descriptionOf = (f) => {
		const values = cell(f.values);
		return [
			f.deprecated === 'yes' ? '**Deprecated.**' : '',
			prose(cell(f.description)),
			values ? `One of ${values.split('|').map(code).join(', ')}.` : '',
		]
			.filter(Boolean)
			.join(' ');
	};

	const fieldTable = (t) => {
		const rows = (fieldsOf.get(`${t.go_struct}/${t.type_token}`) ?? []).map((f) => [
			code(f.json_key),
			typeOf(f),
			requiredOf(f),
			cell(f.default) ? code(f.default) : '',
			descriptionOf(f),
		]);
		return rows.length
			? table(['Key', 'Type', 'Required', 'Default', 'Description'], rows)
			: 'No keys are written on this type.';
	};

	const selection = (t) => {
		const when = cell(t.variant_when);
		const key = cell(t.discriminator);
		if (when.startsWith('present(')) return `Selected when ${code(when.slice(8, -1))} is present.`;
		if (when.startsWith('absent(')) return `Selected when ${code(when.slice(7, -1))} is absent.`;
		if (when.startsWith('unlisted(')) return `Selected when ${code(when.slice(9, -1))} names none of this type's other variants.`;
		return key ? `Selected when ${code(key)} is ${code(t.type_token)}.` : '';
	};

	const sections = [];
	for (const [struct, rows] of structs) {
		if (!reachable.has(struct)) continue;
		claim(struct);
		const deprecated = (t) => (t.deprecated === 'yes' ? '**Deprecated.** ' : '');
		if (rows.length === 1 && rows[0].type_token === '*') {
			const t = rows[0];
			sections.push([`## ${struct}`, deprecated(t) + prose(cell(t.description)), fieldTable(t)].filter(Boolean).join('\n\n'));
			continue;
		}
		const key = cell(rows[0].discriminator);
		// A variant's heading is its own type name, or, where the matrix gives
		// several variants one name, the struct and the variant's token.
		const named = (t) => rows.filter((r) => r.defs_name === t.defs_name).length === 1 && t.defs_name !== struct;
		const heading = (t) => (named(t) ? t.defs_name : `${struct} ${t.type_token.replace(/^~/, '')}`);
		const variants = rows.map((t) => `[${heading(t)}](#${anchor(heading(t))})`).join(', ');
		const parts = [
			`## ${struct}`,
			`${struct} has ${rows.length} variants${key ? `, selected by ${code(key)}` : ''}: ${variants}.`,
		];
		for (const t of rows) {
			claim(heading(t));
			parts.push(
				[`### ${heading(t)}`, selection(t), deprecated(t) + prose(cell(t.description)), fieldTable(t)].filter(Boolean).join('\n\n'),
			);
		}
		sections.push(parts.join('\n\n'));
	}

	const body = [
		`A compute-pipes pipeline is configured by a \`.pc.json\` document in a workspace's \`pipes_config/\` folder. ` +
			`Its root is [${ROOT}](#${anchor(ROOT)}); every other type below is reached from it. ` +
			`${sections.length} types are listed, each with the keys that apply to it.`,
		'A type with variants is one Go struct whose keys depend on the value of a discriminating key, usually `type`: ' +
			'each variant lists only the keys that mean something for it. An empty cell means none was recorded.',
		...sections,
	].join('\n\n');

	const file = 'pipeline-configuration.md';
	writePage(file, {
		title: 'Compute pipes configuration',
		description: 'Every type and key of a compute-pipes pipeline configuration (.pc.json), with its type, requirement, default and values.',
		sources: [TYPES, FIELDS],
		body,
	});
	return join(OUT_DIR, file);
}
