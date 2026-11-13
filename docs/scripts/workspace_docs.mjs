// The workspace generator: a workspace's own guide, from its compiled
// workspace.db, merged with the pages the workspace writes in its docs/ folder.
//
// Run by `npm run gen` (gen.mjs) only when JETS_WORKSPACE_DB names a
// workspace.db; without it the site is JetStore's three guides alone, which is
// the public build. Writes the guide under src/content/docs/workspace/ and its
// manifest to public/workspace/manifest.json, both git-ignored.
//
// **What it reads, and what it never reads.** It reads named columns of the
// compiled workspace's structure tables: source files, classes and their
// properties, domain tables, lookup tables and their columns, rules, rule
// properties, unit tests, and the rule network's keys and operators, which is
// how the rules that read a lookup are found. It reads the names of the rules,
// classes and lookups in each main rule file's compiled model (<main>.json),
// which is where the compiler records the file each rule came from. Unless the
// workspace's docs/workspace_docs.json says {"rule_text": true}, it never reads
// the columns that carry the workspace's values: resources.value (the rules'
// literals), triples (the facts the rule files assert), jetstore_config, and
// jet_rules' label, authored_label and normalized_label (the rule's text). It
// never opens lookup.db, so no lookup row reaches a page. The test beside this
// file proves both on a fixture holding a sentinel in each of those places.
import { DatabaseSync } from 'node:sqlite';
import { cpSync, existsSync, mkdirSync, readdirSync, readFileSync, rmSync, statSync, writeFileSync } from 'node:fs';
import { basename, dirname, join, relative, sep } from 'node:path';
import { REPO_ROOT, code, prose, table } from './lib.mjs';

/** Where the guide goes in the site's content; git-ignored, emptied on every run. */
export const WS_DIR = join(REPO_ROOT, 'docs', 'src', 'content', 'docs', 'workspace');
/** Where the manifest goes; served as /docs/workspace/manifest.json. Git-ignored. */
export const WS_PUBLIC_DIR = join(REPO_ROOT, 'docs', 'public', 'workspace');
/** The guide's address on an installation. */
export const WS_BASE = '/docs/workspace/';
/** The workspace's settings file, in its docs/ folder; read, never published. */
export const SETTINGS_FILE = 'workspace_docs.json';

const GENERATED_NOTE =
	"*Generated from the workspace's compiled `workspace.db` when this site was built. " +
	"Do not edit this page: change the workspace's sources.*";

// The lookup operators: the binary ones take the table as their left argument,
// the unary ones as their only argument (jets/compilerv2/compiler/operators.txt).
const LOOKUP_OPS = ['lookup', 'multi_lookup', 'lookup_rand', 'multi_lookup_rand'];

// Every query the generator makes. Named columns only: a value column is read by
// the one query that is run only when the workspace asks for its rules' text.
export const QUERIES = {
	tables: "SELECT name FROM sqlite_master WHERE type = 'table'",
	files: 'SELECT key, source_file_name, is_main FROM workspace_control ORDER BY key',
	supportFiles: 'SELECT main_file_key, support_file_key FROM main_support_files ORDER BY rowid',
	classes: 'SELECT key, name, as_table, source_file_key FROM domain_classes ORDER BY name',
	baseClasses: 'SELECT domain_class_key, base_class_key FROM base_classes ORDER BY rowid',
	dataProperties: 'SELECT domain_class_key, name, type, as_array, is_grouping FROM data_properties ORDER BY name',
	objectProperties: 'SELECT domain_class_key, name, type, as_array FROM object_properties ORDER BY name',
	domainTables: 'SELECT key, domain_class_key, name FROM domain_tables ORDER BY name',
	domainColumns:
		'SELECT domain_table_key, name, type, as_array, is_object, is_grouping FROM domain_columns ORDER BY name',
	lookupTables: 'SELECT key, name, table_name, csv_file, lookup_key, source_file_key FROM lookup_tables ORDER BY name',
	lookupColumns: 'SELECT lookup_table_key, name, type, as_array FROM lookup_columns ORDER BY rowid',
	ruleSequences: 'SELECT key, name, source_file_key FROM rule_sequences ORDER BY name',
	mainRuleSets:
		'SELECT rule_sequence_key, main_ruleset_name, ruleset_file_key, seq FROM main_rule_sets ORDER BY rule_sequence_key, seq',
	rules: 'SELECT key, name, salience, source_file_key FROM jet_rules ORDER BY key',
	ruleProperties: 'SELECT rule_key, name, value FROM rule_properties ORDER BY rowid',
	unitTests:
		'SELECT path, test_name, process_name, legacy_mode, findings IS NULL AS runnable FROM unit_tests ORDER BY path',
	expressions: 'SELECT key, type, op, arg0_key, arg1_key FROM expressions',
	reteNodes: 'SELECT key, obj_expr_key, filter_expr_key FROM rete_nodes',
	ruleTerms: 'SELECT rule_key, rete_node_key FROM rule_terms',
	// resources.id is the resource's name, structure rather than value; it is read
	// only for the resource a lookup operator names as its table.
	resourceId: 'SELECT resources.type, resources.id FROM resources WHERE resources.key = ?',
	// Run only when the workspace's workspace_docs.json has "rule_text": true.
	ruleText: 'SELECT key, label FROM jet_rules ORDER BY key',
};

/**
 * Build the workspace's guide.
 *
 * @param {object} opts
 * @param {string} opts.dbPath the compiled workspace.db
 * @param {string} [opts.name] the guide's name, the workspace's; defaults to the database's folder
 * @param {string} [opts.docsDir] the workspace's docs/ folder, copied in first when present
 * @param {string} [opts.outDir] where the guide's pages go
 * @param {string} [opts.publicDir] where the manifest goes
 * @param {(path: string) => DatabaseSync} [opts.openDatabase] opens workspace.db read-only
 * @returns {{ pages: string[], manifest: object, counts: object, ruleText: boolean }}
 */
export function generateWorkspaceGuide({
	dbPath,
	name,
	docsDir,
	outDir = WS_DIR,
	publicDir = WS_PUBLIC_DIR,
	openDatabase = (p) => new DatabaseSync(p, { readOnly: true }),
}) {
	if (!dbPath || !existsSync(dbPath) || !statSync(dbPath).isFile()) {
		throw new Error(`JETS_WORKSPACE_DB: no workspace.db at ${dbPath}`);
	}
	const workspaceRoot = dirname(dbPath);
	const guideName = (name ?? '').trim() || basename(workspaceRoot);
	const hasDocs = Boolean(docsDir) && existsSync(docsDir) && statSync(docsDir).isDirectory();
	const settings = readSettings(hasDocs ? docsDir : undefined);
	if (hasDocs) {
		for (const reserved of ['reference', 'reference.md', 'reference.mdx']) {
			if (existsSync(join(docsDir, reserved))) {
				throw new Error(
					`${join(docsDir, reserved)}: a workspace's docs/ may not hold reference/, which is the generated pages' place`,
				);
			}
		}
	}

	rmSync(outDir, { recursive: true, force: true });
	rmSync(publicDir, { recursive: true, force: true });
	mkdirSync(outDir, { recursive: true });
	mkdirSync(publicDir, { recursive: true });

	const pages = [];
	const manifestPages = {};
	const write = (relPath, page) => {
		writePageFile(outDir, relPath, page);
		pages.push(relPath);
	};

	// The workspace's own pages first, so a generated page can never replace one.
	const written = hasDocs ? copyWrittenPages(docsDir, outDir) : [];
	for (const w of written) {
		manifestPages[`docs/${w.path}`] = w.url;
		pages.push(w.path);
	}

	const db = openDatabase(dbPath);
	let model;
	try {
		model = readWorkspace(db, workspaceRoot, settings.ruleText);
	} finally {
		db.close();
	}

	const ctx = { guideName, model, settings };
	planRulePages(model);
	const domainPages = writeDomainPages(ctx, write);
	const lookupsUrl = writeLookupsPage(ctx, write);
	const rulePages = writeRulePages(ctx, write, domainPages, lookupsUrl);
	const counts = writeReferenceIndex(ctx, write, domainPages, rulePages);
	if (!written.some((w) => w.path === 'index.md' || w.path === 'index.mdx')) {
		writeGuideIndex(ctx, write, written, counts);
	}

	// The manifest: each workspace file with a page, and the page. A main rule
	// file's rules page is preferred to the domain page of the classes it declares.
	for (const file of model.files) {
		const path = file.source_file_name;
		if (!path || model.sequenceFileKeys.has(file.key)) continue;
		const url =
			rulePages.byMainKey.get(file.key) ??
			domainPages.byFileKey.get(file.key) ??
			(model.lookupTables.some((l) => l.source_file_key === file.key) ? lookupsUrl : undefined);
		if (url) manifestPages[path] = url;
	}
	for (const path of rulePages.supportFiles) {
		if (!(path in manifestPages)) manifestPages[path] = rulePages.indexUrl;
	}
	const manifest = {
		workspace: guideName,
		index: WS_BASE,
		pages: Object.fromEntries(Object.keys(manifestPages).sort().map((k) => [k, manifestPages[k]])),
	};
	writeFileSync(join(publicDir, 'manifest.json'), JSON.stringify(manifest, null, 2) + '\n');

	return { pages, manifest, counts, ruleText: settings.ruleText };
}

// ---------------------------------------------------------------------------
// Reading

/** The workspace's settings: {"rule_text": true} shows the rules' text. */
export function readSettings(docsDir) {
	const settings = { ruleText: false };
	if (!docsDir) return settings;
	const path = join(docsDir, SETTINGS_FILE);
	if (!existsSync(path)) return settings;
	let parsed;
	try {
		parsed = JSON.parse(readFileSync(path, 'utf8'));
	} catch (err) {
		throw new Error(`${path}: ${err.message}`);
	}
	if (parsed === null || typeof parsed !== 'object' || Array.isArray(parsed)) {
		throw new Error(`${path}: expected a JSON object, such as {"rule_text": true}`);
	}
	if ('rule_text' in parsed && typeof parsed.rule_text !== 'boolean') {
		throw new Error(`${path}: rule_text must be true or false`);
	}
	settings.ruleText = parsed.rule_text === true;
	return settings;
}

/** A string rule property as the author wrote it: the compiler keeps the token's quotes and escapes. */
export function unquote(value) {
	if (value == null) return '';
	const s = String(value);
	if (s.length >= 2 && s.startsWith('"') && s.endsWith('"')) {
		return s.slice(1, -1).replace(/\\(["\\])/g, '$1');
	}
	return s;
}

function readWorkspace(db, workspaceRoot, ruleText) {
	const present = new Set(db.prepare(QUERIES.tables).all().map((r) => r.name));
	// A table an older compiler did not write reads as empty.
	const all = (queryName, table) => (present.has(table) ? db.prepare(QUERIES[queryName]).all() : []);

	const files = all('files', 'workspace_control');
	const fileName = new Map(files.map((f) => [f.key, f.source_file_name]));
	const classes = all('classes', 'domain_classes');
	const lookupTables = all('lookupTables', 'lookup_tables');
	const rules = all('rules', 'jet_rules');
	const ruleSequences = all('ruleSequences', 'rule_sequences');

	const props = new Map();
	for (const p of all('ruleProperties', 'rule_properties')) {
		if (!props.has(p.rule_key)) props.set(p.rule_key, []);
		props.get(p.rule_key).push({ name: p.name, value: p.value });
	}
	for (const r of rules) {
		const own = props.get(r.key) ?? [];
		const description = own.find((p) => p.name === 'description');
		r.description = description ? unquote(description.value).trim() : '';
		r.properties = own.filter((p) => p.name !== 'description').map((p) => ({ name: p.name, value: unquote(p.value) }));
	}
	if (ruleText) {
		const labels = new Map(all('ruleText', 'jet_rules').map((r) => [r.key, r.label]));
		for (const r of rules) r.label = labels.get(r.key) ?? '';
	}

	// The compiled model of each main rule file records the file each rule, class
	// and lookup came from; workspace.db records every rule under its main file.
	const compiled = new Map();
	for (const f of files.filter((f) => f.is_main)) {
		compiled.set(f.key, readCompiledModel(workspaceRoot, f.source_file_name));
	}

	return {
		files,
		fileName,
		supportFiles: all('supportFiles', 'main_support_files'),
		classes,
		baseClasses: all('baseClasses', 'base_classes'),
		dataProperties: all('dataProperties', 'data_properties'),
		objectProperties: all('objectProperties', 'object_properties'),
		domainTables: all('domainTables', 'domain_tables'),
		domainColumns: all('domainColumns', 'domain_columns'),
		lookupTables,
		lookupColumns: all('lookupColumns', 'lookup_columns'),
		ruleSequences,
		sequenceFileKeys: new Set(ruleSequences.map((s) => s.source_file_key)),
		mainRuleSets: all('mainRuleSets', 'main_rule_sets'),
		rules,
		unitTests: all('unitTests', 'unit_tests'),
		compiled,
		readers: lookupReaders(db, present, rules, lookupTables),
	};
}

/**
 * The names in a main rule file's compiled model, <main>.json beside it: for
 * each rule its file, and the files in the order the compiler read them. Only
 * names are kept; null when the file is not there.
 */
function readCompiledModel(workspaceRoot, mainFile) {
	if (!mainFile) return null;
	const path = join(workspaceRoot, `${mainFile.replace(/\.jetrule$/, '')}.json`);
	if (!existsSync(path)) return null;
	let parsed;
	try {
		parsed = JSON.parse(readFileSync(path, 'utf8'));
	} catch (err) {
		throw new Error(`${path}: ${err.message}`);
	}
	const fileOf = (x) => (x && typeof x.source_file_name === 'string' ? x.source_file_name : undefined);
	const order = [];
	const see = (f) => {
		if (f && f !== mainFile && !order.includes(f)) order.push(f);
	};
	for (const c of parsed.classes ?? []) see(fileOf(c));
	for (const l of parsed.lookup_tables ?? []) see(fileOf(l));
	const ruleFile = new Map();
	const duplicate = new Set();
	for (const r of parsed.jet_rules ?? []) {
		if (!r || typeof r.name !== 'string') continue;
		const f = fileOf(r);
		see(f);
		if (ruleFile.has(r.name)) duplicate.add(r.name);
		ruleFile.set(r.name, f);
	}
	for (const n of duplicate) ruleFile.delete(n);
	return { includes: order, ruleFile };
}

/**
 * For each lookup table, the rules that read it: each lookup operator in a
 * rule's antecedent filters or consequent expressions, through rule_terms and
 * rete_nodes, whose table argument names the lookup. `unresolved` counts the
 * lookup operators whose table could not be read as a lookup's name.
 */
function lookupReaders(db, present, rules, lookupTables) {
	const readers = new Map(lookupTables.map((l) => [l.name, new Set()]));
	const result = { byLookup: readers, unresolved: 0, recorded: false };
	if (!['expressions', 'rete_nodes', 'rule_terms', 'resources'].every((t) => present.has(t))) return result;
	result.recorded = true;

	const exprs = new Map(db.prepare(QUERIES.expressions).all().map((e) => [e.key, e]));
	const resourceId = db.prepare(QUERIES.resourceId);
	const lookupNames = new Set(lookupTables.map((l) => l.name));
	const tableOf = new Map();
	for (const e of exprs.values()) {
		if (!LOOKUP_OPS.includes(e.op)) continue;
		const arg = exprs.get(e.arg0_key);
		let table;
		if (arg && arg.type === 'resource' && arg.arg0_key != null) {
			const r = resourceId.get(arg.arg0_key);
			if (r && r.type === 'resource' && lookupNames.has(r.id)) table = r.id;
		}
		tableOf.set(e.key, table);
	}
	if (tableOf.size === 0) return result;

	const nodes = new Map(db.prepare(QUERIES.reteNodes).all().map((n) => [n.key, n]));
	const termsOf = new Map();
	for (const t of db.prepare(QUERIES.ruleTerms).all()) {
		if (!termsOf.has(t.rule_key)) termsOf.set(t.rule_key, []);
		termsOf.get(t.rule_key).push(t.rete_node_key);
	}
	// The lookup operators an expression tree holds. A `resource` expression's
	// argument is a resource, not an expression, so the walk stops there.
	const found = (key, out, seen) => {
		if (key == null || seen.has(key)) return;
		seen.add(key);
		const e = exprs.get(key);
		if (!e) return;
		if (tableOf.has(key)) out.add(key);
		if (e.type === 'unary') found(e.arg0_key, out, seen);
		if (e.type === 'binary') {
			found(e.arg0_key, out, seen);
			found(e.arg1_key, out, seen);
		}
	};
	const used = new Set();
	for (const rule of rules) {
		const ops = new Set();
		for (const nodeKey of termsOf.get(rule.key) ?? []) {
			const n = nodes.get(nodeKey);
			if (!n) continue;
			found(n.obj_expr_key, ops, new Set());
			found(n.filter_expr_key, ops, new Set());
		}
		for (const op of ops) {
			used.add(op);
			const table = tableOf.get(op);
			if (table) readers.get(table).add(rule.key);
		}
	}
	for (const op of used) if (!tableOf.get(op)) result.unresolved++;
	return result;
}

// ---------------------------------------------------------------------------
// The workspace's written pages

/** github-slugger's form of one path segment, which is how the site addresses a page. */
export function slugSegment(segment) {
	return segment
		.toLowerCase()
		.replace(/[^\p{L}\p{M}\p{N}\s_-]/gu, '')
		.replace(/\s/g, '-');
}

/** The address of a page at a path under the guide, from its file name or its front matter's slug. */
export function pageUrl(relPath, text) {
	const slug = text ? frontMatterSlug(text) : undefined;
	if (slug !== undefined) return `/docs/${slug.replace(/^\/+|\/+$/g, '')}/`;
	const noExt = relPath.replace(/\.(md|mdx)$/, '');
	const path = noExt
		.split('/')
		.map(slugSegment)
		.join('/')
		.replace(/(^|\/)index$/, '');
	return path ? `${WS_BASE}${path}/` : WS_BASE;
}

function frontMatterSlug(text) {
	const fm = /^---\r?\n([\s\S]*?)\r?\n---/.exec(text);
	if (!fm) return undefined;
	const line = /^slug:\s*(.+?)\s*$/m.exec(fm[1]);
	return line ? line[1].replace(/^["']|["']$/g, '') : undefined;
}

function copyWrittenPages(docsDir, outDir) {
	const pages = [];
	const walk = (dir) => {
		for (const entry of readdirSync(dir, { withFileTypes: true }).sort((a, b) => a.name.localeCompare(b.name))) {
			const abs = join(dir, entry.name);
			const rel = relative(docsDir, abs).split(sep).join('/');
			if (entry.isDirectory()) {
				walk(abs);
			} else if (entry.isFile() && rel !== SETTINGS_FILE) {
				mkdirSync(dirname(join(outDir, rel)), { recursive: true });
				cpSync(abs, join(outDir, rel));
				if (/\.(md|mdx)$/.test(rel)) pages.push({ path: rel, url: pageUrl(rel, readFileSync(abs, 'utf8')) });
			}
		}
	};
	walk(docsDir);
	return pages;
}

// ---------------------------------------------------------------------------
// Writing

function writePageFile(outDir, relPath, { title, description, kind, sidebar, body }) {
	const fm = ['---', `title: ${JSON.stringify(title)}`, `description: ${JSON.stringify(description)}`, `kind: ${kind}`];
	if (sidebar) {
		fm.push('sidebar:');
		if (sidebar.label) fm.push(`  label: ${JSON.stringify(sidebar.label)}`);
		if (sidebar.order !== undefined) fm.push(`  order: ${sidebar.order}`);
	}
	fm.push('editUrl: false', '---', '');
	const abs = join(outDir, relPath);
	mkdirSync(dirname(abs), { recursive: true });
	writeFileSync(abs, `${fm.join('\n')}${GENERATED_NOTE}\n\n${body.trim()}\n`);
}

/** A page file name from a source file's name: lower case, no extension, safe in an address. */
function fileSlug(path) {
	return (
		path
			.replace(/\.[^./]*$/, '')
			.toLowerCase()
			.replace(/[^a-z0-9_-]+/g, '-')
			.replace(/^[-_]+|-+$/g, '') || 'file'
	);
}

/** Page names for a set of source files: the base name, or the whole path where two base names collide. */
function pageNames(paths, reserved) {
	const base = (p) => fileSlug(p.split('/').pop());
	const counts = new Map();
	for (const p of paths) counts.set(base(p), (counts.get(base(p)) ?? 0) + 1);
	const names = new Map();
	const taken = new Set(reserved);
	for (const p of paths) {
		let n = counts.get(base(p)) > 1 ? fileSlug(p) : base(p);
		while (taken.has(n)) n = `${n}-file`;
		taken.add(n);
		names.set(p, n);
	}
	return names;
}

const yes = (flag) => (flag ? 'yes' : '');
const link = (text, url) => `[${text}](${url})`;
/** A rule's anchor on its page: Astro's heading id, for an identifier. */
const ruleAnchor = (name) => slugSegment(name);

function fencedText(text) {
	const s = String(text ?? '');
	const longest = Math.max(2, ...[...s.matchAll(/`+/g)].map((m) => m[0].length));
	const fence = '`'.repeat(longest + 1);
	return `${fence}text\n${s}\n${fence}`;
}

// ---------------------------------------------------------------------------
// Domain

function writeDomainPages({ model }, write) {
	const classFile = new Map();
	for (const c of model.classes) {
		if (!model.fileName.has(c.source_file_key)) continue;
		if (!classFile.has(c.source_file_key)) classFile.set(c.source_file_key, []);
		classFile.get(c.source_file_key).push(c);
	}
	const fileKeys = [...classFile.keys()].sort((a, b) => model.fileName.get(a).localeCompare(model.fileName.get(b)));
	const names = pageNames(
		fileKeys.map((k) => model.fileName.get(k)),
		['index', 'tables'],
	);
	const className = new Map(model.classes.map((c) => [c.key, c.name]));
	const byFileKey = new Map();
	const classUrl = new Map();
	for (const k of fileKeys) {
		const url = `${WS_BASE}reference/domain/${names.get(model.fileName.get(k))}/`;
		byFileKey.set(k, url);
		for (const c of classFile.get(k)) classUrl.set(c.key, url);
	}
	const classRef = (key) => {
		const n = className.get(key);
		if (n === undefined) return 'not recorded';
		return classUrl.has(key) ? link(code(n), classUrl.get(key)) : code(n);
	};

	for (const k of fileKeys) {
		const path = model.fileName.get(k);
		const sections = classFile.get(k).map((c) => {
			const bases = model.baseClasses.filter((b) => b.domain_class_key === c.key).map((b) => classRef(b.base_class_key));
			const rows = [
				...model.dataProperties
					.filter((p) => p.domain_class_key === c.key)
					.map((p) => [code(p.name), 'data', code(p.type), yes(p.as_array), yes(p.is_grouping)]),
				...model.objectProperties
					.filter((p) => p.domain_class_key === c.key)
					.map((p) => [code(p.name), 'object', code(p.type), yes(p.as_array), '']),
			];
			const parts = [`## ${prose(c.name)}`];
			const facts = [`Base classes: ${bases.length ? bases.join(', ') : 'none'}.`];
			if (c.as_table) facts.push('Stored as a domain table.');
			parts.push(facts.join(' '));
			parts.push(
				rows.length ? table(['Property', 'Kind', 'Type', 'Array', 'Grouping'], rows) : 'No properties of its own.',
			);
			return parts.join('\n\n');
		});
		write(`reference/domain/${names.get(path)}.md`, {
			title: path.split('/').pop(),
			description: `The classes of the workspace's data model declared in ${path}, with their base classes and properties.`,
			kind: 'reference',
			body: [
				`The classes declared in ${code(path)}: ${sections.length}. A property's type is a data type for a data ` +
					'property and a class for an object property; an array property holds several values; a grouping ' +
					'property groups the rows of an entity.',
				...sections,
			].join('\n\n'),
		});
	}

	const tableSections = model.domainTables.map((t) => {
		const rows = model.domainColumns
			.filter((c) => c.domain_table_key === t.key)
			.map((c) => [code(c.name), code(c.type), yes(c.as_array), yes(c.is_object), yes(c.is_grouping)]);
		return [
			`## ${prose(t.name)}`,
			`Maps the class ${classRef(t.domain_class_key)}.`,
			rows.length ? table(['Column', 'Type', 'Array', 'Object', 'Grouping'], rows) : 'No columns.',
		].join('\n\n');
	});
	write('reference/domain/tables.md', {
		title: 'Domain tables',
		description: "The workspace's domain tables, their columns, and the class each maps.",
		kind: 'reference',
		body: [
			`The domain tables: ${model.domainTables.length}. Each holds the entities of one class, a column per property.`,
			...(tableSections.length ? tableSections : ['The workspace declares no domain table.']),
		].join('\n\n'),
	});

	const rows = fileKeys.map((k) => {
		const p = model.fileName.get(k);
		return [link(code(p), byFileKey.get(k)), String(classFile.get(k).length)];
	});
	write('reference/domain/index.md', {
		title: 'Domain',
		description: "The workspace's data model: its classes, one page per source file, and its domain tables.",
		kind: 'overview',
		sidebar: { label: 'Overview', order: 0 },
		body: [
			`The workspace's data model: ${model.classes.filter((c) => model.fileName.has(c.source_file_key)).length} ` +
				`classes in ${fileKeys.length} source files, and ${link(`${model.domainTables.length} domain tables`, `${WS_BASE}reference/domain/tables/`)}.`,
			rows.length ? table(['Source file', 'Classes'], rows) : 'The workspace declares no class.',
		].join('\n\n'),
	});
	return { byFileKey, classUrl, indexUrl: `${WS_BASE}reference/domain/` };
}

// ---------------------------------------------------------------------------
// Lookups

function writeLookupsPage({ model }, write) {
	const url = `${WS_BASE}reference/lookups/`;
	const ruleByKey = new Map(model.rules.map((r) => [r.key, r]));
	const sections = model.lookupTables.map((l) => {
		const cols = model.lookupColumns
			.filter((c) => c.lookup_table_key === l.key)
			.map((c) => [code(c.name), code(c.type), yes(c.as_array)]);
		let readers = 'not recorded';
		if (model.readers.recorded) {
			const keys = [...(model.readers.byLookup.get(l.name) ?? [])];
			readers = keys.length
				? keys
						.map((k) => ruleByKey.get(k))
						.filter(Boolean)
						.map((r) => {
							const page = model.rulePageUrl.get(r.source_file_key);
							return page ? link(code(r.name), `${page}#${ruleAnchor(r.name)}`) : code(r.name);
						})
						.join(', ')
				: 'none found';
		}
		const file = model.fileName.get(l.source_file_key);
		const facts = [
			['Key', l.lookup_key ? code(l.lookup_key) : ''],
			['Source file', file ? code(file) : 'not recorded'],
			['CSV file', l.csv_file ? code(l.csv_file) : ''],
			['Table', l.table_name ? code(l.table_name) : ''],
			['Read by', readers],
		].filter(([, v]) => v);
		return [
			`## ${prose(l.name)}`,
			facts.map(([k, v]) => `- ${k}: ${v}`).join('\n'),
			cols.length ? table(['Column', 'Type', 'Array'], cols) : 'No columns recorded.',
		].join('\n\n');
	});
	const notes = [];
	if (model.readers.unresolved > 0) {
		notes.push(
			`${model.readers.unresolved} lookup call(s) in the rules name their table in a way the compiled workspace ` +
				'does not resolve to a lookup; the rules making them are not listed above.',
		);
	}
	write('reference/lookups.md', {
		title: 'Lookups',
		description: "The workspace's lookup tables: their keys, columns, source files, and the rules that read them.",
		kind: 'reference',
		body: [
			`The lookup tables: ${model.lookupTables.length}. Each is loaded from its CSV file when the workspace is ` +
				'compiled and read by the rules through its key. Their rows are not shown here.',
			...notes,
			...(sections.length ? sections : ['The workspace declares no lookup table.']),
		].join('\n\n'),
	});
	return url;
}

// ---------------------------------------------------------------------------
// Rules

/**
 * The rule's text, shown only when the workspace asks for it (rule_text). The
 * generated reading of the rule goes here too, beside the text.
 */
// eslint-disable-next-line no-unused-vars
function ruleTextParts(rule, ctx) {
	return [fencedText(rule.label)];
}

function renderRule(rule, ruleFile, ctx) {
	const parts = [`### ${prose(rule.name)}`];
	parts.push(rule.description ? prose(rule.description) : '*No description.*');
	const facts = [
		['Source file', ruleFile ? code(ruleFile) : 'not recorded'],
		['Salience', rule.salience != null ? String(rule.salience) : ''],
		['Properties', rule.properties.map((p) => code(`${p.name}=${p.value}`)).join(', ')],
	].filter(([, v]) => v);
	parts.push(facts.map(([k, v]) => `- ${k}: ${v}`).join('\n'));
	if (ctx.settings.ruleText) parts.push(...ruleTextParts(rule, ctx));
	return parts.join('\n\n');
}

/** The rule pages' names and addresses, one per main rule file, set on the model before any page is written. */
function planRulePages(model) {
	model.mains = model.files.filter((f) => f.is_main && f.source_file_name && !model.sequenceFileKeys.has(f.key));
	model.rulePageName = pageNames(
		model.mains.map((m) => m.source_file_name),
		['index'],
	);
	model.rulePageUrl = new Map(
		model.mains.map((m) => [m.key, `${WS_BASE}reference/rules/${model.rulePageName.get(m.source_file_name)}/`]),
	);
}

function writeRulePages(ctx, write, domainPages, lookupsUrl) {
	const { model } = ctx;
	const mains = model.mains;
	const names = model.rulePageName;
	const indexUrl = `${WS_BASE}reference/rules/`;
	const byMainKey = model.rulePageUrl;
	const fileKeyByName = new Map(model.files.map((f) => [f.source_file_name, f.key]));
	const fileLink = (path) => {
		const key = fileKeyByName.get(path);
		const url =
			key !== undefined
				? (byMainKey.get(key) ??
					domainPages.byFileKey.get(key) ??
					(model.lookupTables.some((l) => l.source_file_key === key) ? lookupsUrl : undefined))
				: undefined;
		return url ? link(code(path), url) : code(path);
	};

	const supportFiles = new Set();
	const summary = [];
	for (const main of mains) {
		const path = main.source_file_name;
		const compiled = model.compiled.get(main.key);
		const rules = model.rules.filter((r) => r.source_file_key === main.key);
		const fromDb = model.supportFiles
			.filter((s) => s.main_file_key === main.key)
			.map((s) => model.fileName.get(s.support_file_key))
			.filter(Boolean);
		const includes = fromDb.length ? fromDb : (compiled?.includes ?? null);
		for (const f of includes ?? []) supportFiles.add(f);
		const ruleFile = (r) => compiled?.ruleFile.get(r.name);
		for (const r of rules) {
			const f = ruleFile(r);
			if (f && f !== path) supportFiles.add(f);
		}
		const described = rules.filter((r) => r.description);
		const missing = rules.filter((r) => !r.description);
		const tests = model.unitTests.filter((t) => t.process_name === path);

		const body = [
			`The main rule file ${code(path)}: ${rules.length} rules, ${described.length} with a description.`,
			'## Files it includes',
			includes === null
				? 'Not recorded: the main rule file\'s compiled model was not found beside it when this site was built.'
				: includes.length
					? `In the order the compiler read them:\n\n${includes.map((f, i) => `${i + 1}. ${fileLink(f)}`).join('\n')}`
					: 'None.',
			'## Rules',
			rules.length
				? 'Each rule with its description, the file it is written in, its salience and its other properties.'
				: 'This main rule file compiles to no rule.',
			...rules.map((r) => renderRule(r, ruleFile(r), ctx)),
			'## Unit tests',
			tests.length
				? table(
						['Test', 'File', 'Mode', 'Status'],
						tests.map((t) => [
							t.test_name ? prose(t.test_name) : '',
							code(t.path),
							t.legacy_mode ? code(t.legacy_mode) : '',
							t.runnable ? 'runnable' : 'refused by the compile',
						]),
					)
				: 'No unit test names this main rule file as its process.',
			'## Rules with no description',
			missing.length
				? missing.map((r) => `- ${link(code(r.name), `${byMainKey.get(main.key)}#${ruleAnchor(r.name)}`)}`).join('\n')
				: 'None: every rule has a description.',
		];
		write(`reference/rules/${names.get(path)}.md`, {
			title: path.split('/').pop(),
			description: `The rules of the main rule file ${path}: the files it includes, each rule with its description and properties, and its unit tests.`,
			kind: 'reference',
			body: body.join('\n\n'),
		});
		summary.push({ main, rules: rules.length, described: described.length });
	}

	// One page per rule sequence: its main rule files, in the order they run.
	const seqNames = pageNames(
		model.ruleSequences.map((s) => `sequence-${s.name}`),
		[...names.values(), 'index'],
	);
	const sequenceRows = [];
	for (const s of model.ruleSequences) {
		const page = seqNames.get(`sequence-${s.name}`);
		const steps = model.mainRuleSets.filter((m) => m.rule_sequence_key === s.key);
		write(`reference/rules/${page}.md`, {
			title: `Sequence ${s.name}`,
			description: `The rule sequence ${s.name}: the main rule files it runs, in order.`,
			kind: 'reference',
			body: [
				`The rule sequence ${code(s.name)} runs ${steps.length} main rule files, in this order:`,
				steps.length
					? steps
							.map((m, i) => {
								const url = byMainKey.get(m.ruleset_file_key);
								return `${i + 1}. ${url ? link(code(m.main_ruleset_name), url) : code(m.main_ruleset_name)}`;
							})
							.join('\n')
					: 'None recorded.',
			].join('\n\n'),
		});
		sequenceRows.push([link(code(s.name), `${indexUrl}${page}/`), String(steps.length)]);
	}

	const totalRules = summary.reduce((n, s) => n + s.rules, 0);
	const totalDescribed = summary.reduce((n, s) => n + s.described, 0);
	const support = [...supportFiles].filter((f) => !mains.some((m) => m.source_file_name === f)).sort();
	write('reference/rules/index.md', {
		title: 'Rules',
		description: "The workspace's rules, one page per main rule file, with their descriptions and unit tests.",
		kind: 'overview',
		sidebar: { label: 'Overview', order: 0 },
		body: [
			`The workspace's main rule files: ${mains.length}, compiling to ${totalRules} rules, ${totalDescribed} of them ` +
				'with a description. A rule\'s description is its `description` property, written by its author in the ' +
				'rule\'s header; each page lists the rules that have none.',
			summary.length
				? table(
						['Main rule file', 'Rules', 'With a description'],
						summary.map((s) => [
							link(code(s.main.source_file_name), byMainKey.get(s.main.key)),
							String(s.rules),
							String(s.described),
						]),
					)
				: 'The workspace has no main rule file.',
			...(sequenceRows.length
				? ['## Rule sequences', table(['Sequence', 'Main rule files'], sequenceRows)]
				: []),
			...(support.length
				? [
						'## Files the main rule files include',
						'Each is listed, in order, on the page of each main rule file that includes it.',
						support.map((f) => `- ${fileLink(f)}`).join('\n'),
					]
				: []),
		].join('\n\n'),
	});
	return { byMainKey, indexUrl, supportFiles: support, totalRules, totalDescribed };
}

// ---------------------------------------------------------------------------
// Indexes

function writeReferenceIndex({ guideName, model, settings }, write, domainPages, rulePages) {
	const classes = model.classes.filter((c) => model.fileName.has(c.source_file_key)).length;
	const counts = {
		classes,
		domainTables: model.domainTables.length,
		lookups: model.lookupTables.length,
		mainRuleFiles: rulePages.byMainKey.size,
		rules: rulePages.totalRules,
		described: rulePages.totalDescribed,
	};
	write('reference/index.md', {
		title: 'Reference',
		description: `The pages generated from the compiled ${guideName} workspace: its domain, lookups and rules.`,
		kind: 'overview',
		sidebar: { label: 'Overview', order: 0 },
		body: [
			`The pages in this section are **generated** from the compiled ${code(guideName)} workspace each time the ` +
				"image is built, so they are as current as the workspace it was built from. They say what the workspace's " +
				"rules and data model are, not why: the reasons belong in the workspace's own written pages.",
			table(
				['Page', 'What it holds'],
				[
					[link('Domain', domainPages.indexUrl), `${classes} classes, one page per source file, and ${counts.domainTables} domain tables`],
					[link('Lookups', `${WS_BASE}reference/lookups/`), `${counts.lookups} lookup tables, their columns and the rules that read them; not their rows`],
					[
						link('Rules', rulePages.indexUrl),
						`${counts.mainRuleFiles} main rule files and ${counts.rules} rules, ${counts.described} of them with a description`,
					],
				],
			),
			`**${counts.described} of the workspace's ${counts.rules} rules carry a description.** ` +
				(settings.ruleText
					? "The rules' text is shown, as the workspace's `docs/workspace_docs.json` asks."
					: "The rules' text is not shown: the workspace has not asked for it in `docs/workspace_docs.json`."),
		].join('\n\n'),
	});
	return counts;
}

function writeGuideIndex({ guideName }, write, written, counts) {
	const own = written.filter((w) => w.path !== 'index.md');
	write('index.md', {
		title: guideName,
		description: `The guide of the ${guideName} workspace: its written pages and its generated reference.`,
		kind: 'overview',
		sidebar: { label: 'Overview', order: 0 },
		body: [
			`The guide of the ${code(guideName)} workspace, built with this installation's documentation.`,
			`- ${link('Reference', `${WS_BASE}reference/`)}: generated from the compiled workspace, its domain ` +
				`(${counts.classes} classes), its lookups (${counts.lookups}) and its rules (${counts.rules}, ` +
				`${counts.described} with a description).`,
			own.length
				? `The workspace's own pages:\n\n${own.map((w) => `- ${link(code(w.path), w.url)}`).join('\n')}`
				: "The workspace has written no pages of its own: they go in its repository's `docs/` folder.",
		].join('\n\n'),
	});
}
