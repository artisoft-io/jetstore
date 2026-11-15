// The workspace generator: a workspace's own guide, from its compiled
// workspace.db, merged with the pages the workspace writes in its docs/ folder.
//
// Run by `npm run gen` (gen.mjs) only when JETS_WORKSPACE_DB names a
// workspace.db; without it the site is JetStore's three guides alone, which is
// the public build. Writes the guide under src/content/docs/workspace/ and its
// manifest to public/workspace/manifest.json, both git-ignored.
//
// **What it reads, and what it never reads.** It reads named columns of the
// compiled workspace's structure tables: source files and the files each main
// rule file includes, classes and their properties, domain tables, lookup tables
// and their columns, rules and the file each is written in, rule properties,
// unit tests, and the rule network's keys and operators, which is how the rules
// that read a lookup are found. Unless the workspace's docs/workspace_docs.json
// says {"rule_text": true}, it never reads the columns that carry the
// workspace's values: resources.value (the rules' literals), triples (the facts
// the rule files assert), jetstore_config, and jet_rules' label, authored_label
// and normalized_label (the rule's text). It never opens lookup.db, so no lookup
// row reaches a page, and it does not read the compiled models (<main>.json)
// written beside the main rule files. The test beside this file proves this on
// a fixture holding a sentinel in each of those places.
import { DatabaseSync } from 'node:sqlite';
import { cpSync, existsSync, mkdirSync, readdirSync, readFileSync, rmSync, statSync, writeFileSync } from 'node:fs';
import { basename, dirname, join, relative, sep } from 'node:path';
import { REPO_ROOT, code, prose, table } from './lib.mjs';
import { readRule } from './rule_reading.mjs';

/** Where the guide goes in the site's content; git-ignored, emptied on every run. */
export const WS_DIR = join(REPO_ROOT, 'docs', 'src', 'content', 'docs', 'workspace');
/** Where the manifest goes; served as /docs/workspace/manifest.json. Git-ignored. */
export const WS_PUBLIC_DIR = join(REPO_ROOT, 'docs', 'public', 'workspace');
/** The guide's address on an installation. */
export const WS_BASE = '/docs/workspace/';
/** The workspace's settings file, in its docs/ folder; read, never published. */
export const SETTINGS_FILE = 'workspace_docs.json';
/**
 * The names a workspace's docs/ may not hold, in lower case: the generated
 * pages' folder, and the pages that would take its address. The IDE's save path
 * refuses them too (wsfile.GeneratedDocsNames, jets/datatable/wsfile/sections.go).
 */
export const RESERVED_NAMES = ['reference', 'reference.md', 'reference.mdx'];

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
	// source_file_key is the main rule file whose network holds the row; a rule
	// in a file several main rule files include is one row under each.
	// authored_file_key is the file the rule is written in.
	ruleColumns: "SELECT name FROM pragma_table_info('jet_rules')",
	rules: 'SELECT key, name, salience, source_file_key, authored_file_key FROM jet_rules ORDER BY key',
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
		// Matched without case: the site's addresses are lower case, so a
		// Reference/ folder would take the generated pages' address too.
		for (const entry of readdirSync(docsDir)) {
			if (RESERVED_NAMES.includes(entry.toLowerCase())) {
				throw new Error(
					`${join(docsDir, entry)}: a workspace's docs/ may not hold reference/, which is the generated pages' place`,
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
		model = readWorkspace(db, dbPath, settings.ruleText);
	} finally {
		db.close();
	}

	const ctx = { guideName, model, settings };
	planPages(model);
	const domainPages = writeDomainPages(ctx, write);
	const lookupsUrl = writeLookupsPage(ctx, write);
	const rulePages = writeRulePages(ctx, write, lookupsUrl);
	const counts = writeReferenceIndex(ctx, write, domainPages, rulePages);
	if (!written.some((w) => w.path === 'index.md' || w.path === 'index.mdx')) {
		writeGuideIndex(ctx, write, written, counts);
	}

	// The manifest: each workspace file with a page, and the page. A rule file's
	// own page is preferred to the domain page of the classes it declares.
	for (const file of model.files) {
		const path = file.source_file_name;
		if (!path || model.sequenceFileKeys.has(file.key)) continue;
		const url = model.filePageUrl(file.key, lookupsUrl);
		if (url) manifestPages[path] = url;
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

/** The character each escape stands for, after its backslash: UnquoteString's table. */
const UNESCAPED = { '"': '"', '\\': '\\', t: '\t', n: '\n', r: '\r' };

/**
 * A string rule property's value, read as the compiler reads a string literal.
 * The compiler stores a property as its token, quotes and escapes included, and
 * reads it with UnquoteString (jets/compilerv2/compiler/string_literal.go); this
 * is the same reading. A double-quoted token loses its quotes, and \" \\ \t \n
 * \r stand for a double quote, a backslash, a tab, a line feed and a carriage
 * return. Any other backslash is kept, with the character after it, and so is
 * a backslash at the end. A token that is not double-quoted is returned as it is.
 */
export function unquote(value) {
	if (value == null) return '';
	const s = String(value);
	if (s.length >= 2 && s.startsWith('"') && s.endsWith('"')) {
		return s.slice(1, -1).replace(/\\(["\\tnr])/g, (_, c) => UNESCAPED[c]);
	}
	return s;
}

/**
 * Text on one line: each run of whitespace holding a line feed or a carriage
 * return becomes one space. A description or a property may hold a line break,
 * written \n or \r, or across lines in the rule file; in the page's Markdown it
 * would end the paragraph or the list item, and the line after it could read as
 * a heading, a table row or a thematic break. A tab is kept.
 */
export function oneLine(text) {
	return String(text ?? '').replace(/\s*[\r\n]\s*/g, ' ');
}

function readWorkspace(db, dbPath, ruleText) {
	const present = new Set(db.prepare(QUERIES.tables).all().map((r) => r.name));
	// A table an older compiler did not write reads as empty.
	const all = (queryName, table) => (present.has(table) ? db.prepare(QUERIES[queryName]).all() : []);

	// The file each rule is written in is jet_rules.authored_file_key. A compiler
	// older than this generator did not write it, and without it every rule would
	// be shown once per main rule file again, so such a workspace.db is refused.
	if (present.has('jet_rules')) {
		const columns = new Set(db.prepare(QUERIES.ruleColumns).all().map((c) => c.name));
		if (!columns.has('authored_file_key')) {
			throw new Error(
				`${dbPath}: jet_rules has no authored_file_key, the file each rule is written in: ` +
					'the workspace was compiled by an older JetStore. Compile it with the release this site is built from.',
			);
		}
	}

	const files = all('files', 'workspace_control');
	const fileName = new Map(files.map((f) => [f.key, f.source_file_name]));
	const classes = all('classes', 'domain_classes');
	const lookupTables = all('lookupTables', 'lookup_tables');
	const ruleRows = all('rules', 'jet_rules');
	const ruleSequences = all('ruleSequences', 'rule_sequences');

	const props = new Map();
	for (const p of all('ruleProperties', 'rule_properties')) {
		if (!props.has(p.rule_key)) props.set(p.rule_key, []);
		props.get(p.rule_key).push({ name: p.name, value: p.value });
	}
	const rules = collapseRules(ruleRows, fileName, classes);
	for (const r of rules) {
		const own = props.get(r.rows[0].key) ?? [];
		const description = own.find((p) => p.name === 'description');
		r.description = description ? unquote(description.value).trim() : '';
		r.properties = own.filter((p) => p.name !== 'description').map((p) => ({ name: p.name, value: unquote(p.value) }));
	}
	if (ruleText) {
		const labels = new Map(all('ruleText', 'jet_rules').map((r) => [r.key, r.label]));
		for (const r of rules) r.variants = readVariants(db, r, labels);
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
		ruleRows,
		rules,
		ruleOfRow: new Map(rules.flatMap((r) => r.rows.map((row) => [row.key, r]))),
		unitTests: all('unitTests', 'unit_tests'),
		readers: lookupReaders(db, present, ruleRows, lookupTables),
	};
}

/**
 * The workspace's rules, each once: jet_rules has a row per main rule file that
 * runs the rule, so the rows of one rule are those with its name and the file it
 * is written in, one under each main rule file. A name written twice in one file
 * (the compiler accepts it) is two rules: the nth row under each main rule file
 * is the nth rule. A row whose file is not recorded stays its main rule file's
 * own, not collapsed with anything. Each rule: { name, salience, fileKey, recorded,
 * generated, rows, mains }, mains being the main rule files it runs under.
 */
export function collapseRules(rows, fileName, classes = []) {
	const generatedNames = new Set(classes.map((c) => `ci_${c.name}\u0000${c.source_file_key}`));
	const groups = new Map();
	const rules = [];
	for (const row of rows) {
		const recorded = row.authored_file_key != null && fileName.has(row.authored_file_key);
		const fileKey = recorded ? row.authored_file_key : row.source_file_key;
		const id = `${row.name}\u0000${recorded ? fileKey : `main ${row.source_file_key}`}`;
		if (!groups.has(id)) groups.set(id, { rules: [], seen: new Map() });
		const group = groups.get(id);
		const nth = group.seen.get(row.source_file_key) ?? 0;
		group.seen.set(row.source_file_key, nth + 1);
		if (!group.rules[nth]) {
			const rule = {
				name: row.name,
				salience: row.salience,
				fileKey,
				recorded,
				// The compiler's ci_<class> rule, in its class's file: it gives an entity
				// of the class the types of its base classes.
				generated: recorded && generatedNames.has(`${row.name}\u0000${fileKey}`),
				rows: [],
				mains: [],
			};
			group.rules[nth] = rule;
			rules.push(rule);
		}
		group.rules[nth].rows.push(row);
		group.rules[nth].mains.push(row.source_file_key);
	}
	return rules;
}

/**
 * A rule's text and reading under each main rule file that runs it. The reading
 * matches the rule's configuration on its main rule file (rule_reading.mjs), so
 * it is read from every row; rows that read alike are one variant, and a rule
 * normally has one. Each variant: { mains, label, reading }.
 */
function readVariants(db, rule, labels) {
	const variants = new Map();
	for (const row of rule.rows) {
		let reading;
		try {
			reading = readRule(db, row.key);
		} catch (err) {
			reading = { error: err.message };
		}
		const label = labels.get(row.key) ?? '';
		const id = JSON.stringify([label, reading]);
		if (!variants.has(id)) variants.set(id, { mains: [], label, reading });
		variants.get(id).mains.push(row.source_file_key);
	}
	return [...variants.values()];
}

/**
 * For each lookup table, the rules that read it: each lookup operator in a
 * rule's antecedent filters or consequent expressions, through rule_terms and
 * rete_nodes, whose table argument names the lookup. `unresolved` counts the
 * lookup operators whose table could not be read as a lookup's name.
 */
function lookupReaders(db, present, rules, lookupTables) {
	// rules are jet_rules rows here: the network is per main rule file.
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
				// A page whose name starts with _ is copied, for a page that imports
				// it, but is not a page: Starlight's loader skips it.
				if (/\.(md|mdx)$/.test(rel) && !entry.name.startsWith('_')) {
					pages.push({ path: rel, url: pageUrl(rel, readFileSync(abs, 'utf8')) });
				}
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
/** A count and its noun: 1 rule, 2 rules. */
const count = (n, noun) => `${n} ${noun}${n === 1 ? '' : 's'}`;
const link = (text, url) => `[${text}](${url})`;
/** A link to a rule, at its own heading on its page. */
const ruleLink = (r) => link(code(r.name), `${r.pageUrl}#${r.anchor}`);

/**
 * The ids of one page's headings, as Astro gives them (github-slugger): each
 * heading's slug, and a heading whose slug an earlier one on the page has
 * already taken gets -1, -2 and so on. Seeded with the headings above the ones
 * being numbered; returns the function that gives the next heading its id.
 */
export function pageSlugger(earlier = []) {
	const seen = new Map();
	const slug = (text) => {
		const base = slugSegment(text);
		let id = base;
		while (seen.has(id)) {
			seen.set(base, seen.get(base) + 1);
			id = `${base}-${seen.get(base)}`;
		}
		seen.set(id, 0);
		return id;
	};
	for (const text of earlier) slug(text);
	return slug;
}

/** The section headings of the generated pages that come above their rules. */
const HEADINGS = {
	includes: 'Files it includes',
	writtenInMain: 'Rules written in it',
	rules: 'Rules',
	writtenInFile: 'Rules written in this file',
};

function fencedText(text) {
	const s = String(text ?? '');
	const longest = Math.max(2, ...[...s.matchAll(/`+/g)].map((m) => m[0].length));
	const fence = '`'.repeat(longest + 1);
	return `${fence}text\n${s}\n${fence}`;
}

// ---------------------------------------------------------------------------
// Planning: every page's address, set on the model before any page is written

/**
 * Where each file and each rule is shown. A file declaring a class has a domain
 * page. A main rule file has a rules page, and so does every other file a rule
 * is written in or a main rule file includes, except a data-model file, whose
 * rules (the compiler's ci_ rules, and any written there) are on its domain page,
 * and a file declaring only lookup tables, which is the lookups page's. A rule is
 * shown once, on its file's page.
 */
function planPages(model) {
	const classFile = new Map();
	for (const c of model.classes) {
		if (!model.fileName.has(c.source_file_key)) continue;
		if (!classFile.has(c.source_file_key)) classFile.set(c.source_file_key, []);
		classFile.get(c.source_file_key).push(c);
	}
	const byName = (a, b) => model.fileName.get(a).localeCompare(model.fileName.get(b));
	model.classFile = classFile;
	model.domainFileKeys = [...classFile.keys()].sort(byName);
	const domainNames = pageNames(
		model.domainFileKeys.map((k) => model.fileName.get(k)),
		['index', 'tables'],
	);
	model.domainPageUrl = new Map(
		model.domainFileKeys.map((k) => [k, `${WS_BASE}reference/domain/${domainNames.get(model.fileName.get(k))}/`]),
	);
	model.domainPagePath = new Map(
		model.domainFileKeys.map((k) => [k, `reference/domain/${domainNames.get(model.fileName.get(k))}.md`]),
	);
	model.classUrl = new Map();
	for (const [k, list] of classFile) for (const c of list) model.classUrl.set(c.key, model.domainPageUrl.get(k));

	const authoring = new Set(model.rules.map((r) => r.fileKey));
	const included = new Set(model.supportFiles.map((s) => s.support_file_key));
	const declaresLookups = new Set(model.lookupTables.map((l) => l.source_file_key));
	const named = model.files.filter((f) => f.source_file_name && !model.sequenceFileKeys.has(f.key));
	model.mains = named.filter((f) => f.is_main);
	model.ruleFiles = named
		.filter(
			(f) =>
				!f.is_main &&
				!model.domainPageUrl.has(f.key) &&
				(authoring.has(f.key) || (included.has(f.key) && !declaresLookups.has(f.key))),
		)
		.sort((a, b) => a.source_file_name.localeCompare(b.source_file_name));
	const mainNames = pageNames(
		model.mains.map((m) => m.source_file_name),
		['index', 'included'],
	);
	const fileNames = pageNames(
		model.ruleFiles.map((f) => f.source_file_name),
		['index'],
	);
	model.mainPageNames = mainNames;
	model.rulePagePath = new Map([
		...model.mains.map((m) => [m.key, `reference/rules/${mainNames.get(m.source_file_name)}.md`]),
		...model.ruleFiles.map((f) => [f.key, `reference/rules/included/${fileNames.get(f.source_file_name)}.md`]),
	]);
	model.rulePageUrl = new Map(
		[...model.rulePagePath].map(([k, path]) => [k, `${WS_BASE}${path.replace(/\.md$/, '')}/`]),
	);

	model.rulesByFile = new Map();
	for (const r of model.rules) {
		r.pageUrl = model.rulePageUrl.get(r.fileKey) ?? model.domainPageUrl.get(r.fileKey);
		if (!model.rulesByFile.has(r.fileKey)) model.rulesByFile.set(r.fileKey, []);
		model.rulesByFile.get(r.fileKey).push(r);
	}
	// Each rule's anchor: its heading's id on its page, which the headings above
	// it decide. A name written twice in one file is two rules and two headings,
	// and the second heading's id is the name's with -1, so each is linked to
	// its own. The headings above a file's rules are those its page writes
	// first: writeRulePages and writeDomainPages, with HEADINGS.
	const mainKeys = new Set(model.mains.map((m) => m.key));
	for (const [fileKey, rules] of model.rulesByFile) {
		let earlier;
		if (mainKeys.has(fileKey)) earlier = [HEADINGS.includes, HEADINGS.writtenInMain];
		else if (model.rulePageUrl.has(fileKey)) earlier = [HEADINGS.rules];
		else if (model.domainPageUrl.has(fileKey))
			earlier = [...classFile.get(fileKey).map((c) => c.name), HEADINGS.writtenInFile];
		else continue;
		const slug = pageSlugger(earlier);
		for (const r of rules) r.anchor = slug(r.name);
	}
	// The main rule files that include each file, in main rule file order.
	model.includedBy = new Map();
	const mainOrder = new Map(model.mains.map((m, i) => [m.key, i]));
	for (const s of model.supportFiles) {
		if (!model.includedBy.has(s.support_file_key)) model.includedBy.set(s.support_file_key, []);
		const list = model.includedBy.get(s.support_file_key);
		if (!list.includes(s.main_file_key)) list.push(s.main_file_key);
	}
	for (const list of model.includedBy.values()) list.sort((a, b) => (mainOrder.get(a) ?? 0) - (mainOrder.get(b) ?? 0));
	model.filePageUrl = (key, lookupsUrl) =>
		model.rulePageUrl.get(key) ?? model.domainPageUrl.get(key) ?? (declaresLookups.has(key) ? lookupsUrl : undefined);
}

// ---------------------------------------------------------------------------
// Domain

function writeDomainPages(ctx, write) {
	const { model } = ctx;
	const classFile = model.classFile;
	const fileKeys = model.domainFileKeys;
	const className = new Map(model.classes.map((c) => [c.key, c.name]));
	const classRef = (key) => {
		const n = className.get(key);
		if (n === undefined) return 'not recorded';
		return model.classUrl.has(key) ? link(code(n), model.classUrl.get(key)) : code(n);
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
		write(model.domainPagePath.get(k), {
			title: path.split('/').pop(),
			description:
				`The classes of the workspace's data model declared in ${path}, with their base classes and properties` +
				(model.rulesByFile.has(k) ? ', and the rules written in the file.' : '.'),
			kind: 'reference',
			body: [
				`The classes declared in ${code(path)}: ${sections.length}. A property's type is a data type for a data ` +
					'property and a class for an object property; an array property holds several values; a grouping ' +
					'property groups the rows of an entity.',
				...sections,
				...domainRulesSection(ctx, k),
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
		return [link(code(p), model.domainPageUrl.get(k)), String(classFile.get(k).length)];
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
	return { byFileKey: model.domainPageUrl, classUrl: model.classUrl, indexUrl: `${WS_BASE}reference/domain/` };
}

/** The rules written in a data-model file, on its domain page; a pointer when the file has a rules page of its own. */
function domainRulesSection(ctx, fileKey) {
	const { model } = ctx;
	const rules = model.rulesByFile.get(fileKey) ?? [];
	if (!rules.length) return [];
	const own = model.rulePageUrl.get(fileKey);
	if (own) {
		return [`## ${HEADINGS.writtenInFile}`, `The ${count(rules.length, 'rule')} written in this file are on ${link('its rules page', own)}.`];
	}
	const generated = rules.filter((r) => r.generated).length;
	const runsUnder = model.includedBy.get(fileKey) ?? [];
	return [
		`## ${HEADINGS.writtenInFile}`,
		`${count(rules.length, 'rule')} ${rules.length === 1 ? 'is' : 'are'} written in this file` +
			(generated
				? `, ${generated} of them generated by the compiler: ${code('ci_<class>')}, for a class declared here, gives ` +
					'an entity of the class the types of its base classes. '
				: '. ') +
			(runsUnder.length
				? `They run under the main rule files that include this file: ${mainLinks(model, runsUnder)}.`
				: 'No main rule file includes this file.'),
		...rules.map((r) => renderRule(r, ctx, runsUnder)),
		...missingDescriptions(rules, '###'),
	];
}

// ---------------------------------------------------------------------------
// Lookups

function writeLookupsPage({ model }, write) {
	const url = `${WS_BASE}reference/lookups/`;
	const sections = model.lookupTables.map((l) => {
		const cols = model.lookupColumns
			.filter((c) => c.lookup_table_key === l.key)
			.map((c) => [code(c.name), code(c.type), yes(c.as_array)]);
		let readers = 'not recorded';
		if (model.readers.recorded) {
			// The readers are rows, one per main rule file; each rule is named once.
			const rules = [
				...new Set([...(model.readers.byLookup.get(l.name) ?? [])].map((k) => model.ruleOfRow.get(k)).filter(Boolean)),
			];
			readers = rules.length
				? rules.map((r) => (r.pageUrl && r.anchor ? ruleLink(r) : code(r.name))).join(', ')
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

/** Links to main rule files' pages, by workspace_control key. */
function mainLinks(model, keys) {
	return keys
		.map((k) => {
			const path = model.fileName.get(k) ?? 'not recorded';
			const url = model.rulePageUrl.get(k);
			return url ? link(code(path), url) : code(path);
		})
		.join(', ');
}

/** A link to a file's page, if it has one. */
function fileLink(model, key, lookupsUrl) {
	const path = model.fileName.get(key) ?? 'not recorded';
	const url = model.filePageUrl(key, lookupsUrl);
	return url ? link(code(path), url) : code(path);
}

/** One reading of a rule and its text: how the engine reads it, then the rule as written. */
function variantParts(variant) {
	const parts = [];
	const r = variant?.reading;
	if (r && !r.error) {
		const clauses = (label, list) =>
			list.length ? [`${label}:`, list.map((c) => `- ${prose(oneLine(c))}`).join('\n')].join('\n\n') : '';
		parts.push(
			r.partial
				? '*How the engine reads it*, generated from the compiled rule; partly, where it says *not rendered*:'
				: '*How the engine reads it*, generated from the compiled rule:',
		);
		const when = clauses('When', r.when);
		const then = clauses('Then', r.then);
		if (when) parts.push(when);
		if (then) parts.push(then);
	} else if (r?.error) {
		parts.push(`*The reading could not be generated*: ${prose(oneLine(r.error))}`);
	}
	parts.push('*The rule as written:*', fencedText(variant?.label));
	return parts;
}

/** Where the compiler's per-main-rule-file ordering of a rule's conditions is explained. */
const COST_ORDER_URL = '/docs/developer/engine/compiler/antecedent-order/#the-dynamic-optimization-the-cost-order-from-statistics';

/**
 * The rule's text and reading; shown only when the workspace asks for it
 * (rule_text). Read under each main rule file that runs it: when they differ,
 * each is shown with the main rule files it holds for. The compiler orders a
 * rule's conditions for each main rule file from that file's optimizer
 * statistics, and a configuration resource may be set differently in two main
 * rule files, so the compiled rule is not always the same.
 */
function ruleTextParts(rule, ctx) {
	const variants = rule.variants ?? [];
	if (variants.length <= 1) return variantParts(variants[0]);
	const parts = [
		'*It is compiled differently under the main rule files that run it*: the compiler orders a rule\'s conditions ' +
			`for each main rule file, from its ${link('optimizer statistics', COST_ORDER_URL)}, and a configuration ` +
			'resource may be set differently in each. Each version, with the main rule files it is compiled for:',
	];
	for (const v of variants) parts.push(`**Under ${mainLinks(ctx.model, v.mains)}:**`, ...variantParts(v));
	return parts;
}

/**
 * A rule: its description, where it runs, its salience and properties, and,
 * with rule_text, its text. `pageMains` are the main rule files the page says
 * its rules run under; a rule that runs under others says so.
 */
function renderRule(rule, ctx, pageMains) {
	const parts = [`### ${prose(rule.name)}`];
	parts.push(rule.description ? prose(oneLine(rule.description)) : rule.generated ? '*Generated by the compiler.*' : '*No description.*');
	const same =
		rule.mains.length === pageMains.length && [...rule.mains].sort().every((k, i) => k === [...pageMains].sort()[i]);
	const facts = [
		['Source file', rule.recorded ? '' : 'not recorded; shown on the page of the main rule file it runs under'],
		['Runs under', same ? '' : mainLinks(ctx.model, rule.mains)],
		['Salience', rule.salience != null ? String(rule.salience) : ''],
		['Properties', rule.properties.map((p) => code(`${p.name}=${oneLine(p.value)}`)).join(', ')],
	].filter(([, v]) => v);
	if (facts.length) parts.push(facts.map(([k, v]) => `- ${k}: ${v}`).join('\n'));
	if (ctx.settings.ruleText) parts.push(...ruleTextParts(rule, ctx));
	return parts.join('\n\n');
}

/** The rules a page holds that have no description, linked; the compiler's generated rules are not counted. */
function missingDescriptions(rules, heading) {
	const missing = rules.filter((r) => !r.description && !r.generated);
	if (!missing.length) return [];
	return [
		`${heading} Rules with no description`,
		missing.map((r) => `- ${ruleLink(r)}`).join('\n'),
	];
}

function writeRulePages(ctx, write, lookupsUrl) {
	const { model } = ctx;
	const indexUrl = `${WS_BASE}reference/rules/`;
	const written = (key) => model.rulesByFile.get(key) ?? [];
	const described = (rules) => rules.filter((r) => r.description).length;
	const runsRows = new Map();
	for (const row of model.ruleRows) runsRows.set(row.source_file_key, (runsRows.get(row.source_file_key) ?? 0) + 1);

	for (const main of model.mains) {
		const path = main.source_file_name;
		const rules = written(main.key);
		const includes = model.supportFiles.filter((s) => s.main_file_key === main.key).map((s) => s.support_file_key);
		const includedBy = model.includedBy.get(main.key) ?? [];
		const pageMains = [main.key, ...includedBy];
		const tests = model.unitTests.filter((t) => t.process_name === path);
		const runs = runsRows.get(main.key) ?? 0;
		const body = [
			`The main rule file ${code(path)} runs ${count(runs, 'rule')}: ${rules.length} written in it, ${described(rules)} of ` +
				'them with a description, and the rest written in the files it includes, on their pages.' +
				(includedBy.length ? ` It is itself included by ${mainLinks(model, includedBy)}.` : ''),
			`## ${HEADINGS.includes}`,
			includes.length
				? 'In the order the compiler read them, with the number of rules written in each:\n\n' +
					includes
						.map((k, i) => {
							const n = written(k).length;
							return `${i + 1}. ${fileLink(model, k, lookupsUrl)}${n ? ` (${count(n, 'rule')})` : ''}`;
						})
						.join('\n')
				: 'None.',
			`## ${HEADINGS.writtenInMain}`,
			rules.length
				? 'Each rule with its description, its salience and its other properties. Each runs under this main rule file' +
					(includedBy.length ? ' and those that include it' : '') +
					', unless it says otherwise.'
				: 'No rule is written in this main rule file.',
			...rules.map((r) => renderRule(r, ctx, pageMains)),
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
			...missingDescriptions(rules, '##'),
		];
		write(model.rulePagePath.get(main.key), {
			title: path.split('/').pop(),
			description: `The main rule file ${path}: the files it includes, the rules written in it with their descriptions and properties, and its unit tests.`,
			kind: 'reference',
			body: body.join('\n\n'),
		});
	}

	for (const file of model.ruleFiles) {
		const path = file.source_file_name;
		const rules = written(file.key);
		const includedBy = model.includedBy.get(file.key) ?? [];
		const body = [
			`The rule file ${code(path)}: ${count(rules.length, 'rule')} written in it, ${described(rules)} with a ` +
				'description. ' +
				(includedBy.length
					? `It is included by ${count(includedBy.length, 'main rule file')}, and its rules run under each, unless a rule says otherwise: ${mainLinks(model, includedBy)}. Their pages list their unit tests.`
					: 'No main rule file includes it.'),
			`## ${HEADINGS.rules}`,
			rules.length ? 'Each rule with its description, its salience and its other properties.' : 'No rule is written in this file.',
			...rules.map((r) => renderRule(r, ctx, includedBy)),
			...missingDescriptions(rules, '##'),
		];
		write(model.rulePagePath.get(file.key), {
			title: path.split('/').pop(),
			description: `The rule file ${path}: the main rule files that include it, and the rules written in it with their descriptions and properties.`,
			kind: 'reference',
			body: body.join('\n\n'),
		});
	}

	// One page per rule sequence: its main rule files, in the order they run.
	const seqNames = pageNames(
		model.ruleSequences.map((s) => `sequence-${s.name}`),
		[...model.mainPageNames.values(), 'index', 'included'],
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
								const url = model.rulePageUrl.get(m.ruleset_file_key);
								return `${i + 1}. ${url ? link(code(m.main_ruleset_name), url) : code(m.main_ruleset_name)}`;
							})
							.join('\n')
					: 'None recorded.',
			].join('\n\n'),
		});
		sequenceRows.push([link(code(s.name), `${indexUrl}${page}/`), String(steps.length)]);
	}

	const totals = ruleTotals(model.rules);
	const domainWithRules = model.domainFileKeys.filter((k) => written(k).length && !model.rulePageUrl.has(k));
	write('reference/rules/index.md', {
		title: 'Rules',
		description: "The workspace's rules, each on the page of the file it is written in, with their descriptions and unit tests.",
		kind: 'overview',
		sidebar: { label: 'Overview', order: 0 },
		body: [
			`The workspace's rules: ${totals.rules}, ${totals.described} of them with a description` +
				(totals.generated ? `, and ${totals.generated} generated by the compiler from the classes, which have none` : '') +
				'. Each rule is shown once, on the page of the file it is written in, which says which main rule files run ' +
				'it. A rule\'s description is its `description` property, written by its author in the rule\'s header; ' +
				'each page lists the rules that have none.' +
				(totals.differing
					? ` ${count(totals.differing, 'rule')} ${totals.differing === 1 ? 'is' : 'are'} compiled differently under different main rule files; their pages show each version.`
					: ''),
			'## Main rule files',
			model.mains.length
				? table(
						['Main rule file', 'Files it includes', 'Rules it runs', 'Written in it', 'With a description'],
						model.mains.map((m) => [
							link(code(m.source_file_name), model.rulePageUrl.get(m.key)),
							String(model.supportFiles.filter((s) => s.main_file_key === m.key).length),
							String(runsRows.get(m.key) ?? 0),
							String(written(m.key).length),
							String(described(written(m.key))),
						]),
					)
				: 'The workspace has no main rule file.',
			...(model.ruleFiles.length
				? [
						'## Included rule files',
						'The files the main rule files include, other than data-model and lookup files, with the rules written in each.',
						table(
							['Rule file', 'Included by', 'Rules', 'With a description'],
							model.ruleFiles.map((f) => [
								link(code(f.source_file_name), model.rulePageUrl.get(f.key)),
								String((model.includedBy.get(f.key) ?? []).length),
								String(written(f.key).length),
								String(described(written(f.key))),
							]),
						),
					]
				: []),
			...(domainWithRules.length
				? [
						'## Rules in data-model files',
						"A data-model file's rules are on its domain page: the compiler's generated rules, one per class, and " +
							'any rule written there.',
						table(
							['Data-model file', 'Rules', 'Generated'],
							domainWithRules.map((k) => [
								link(code(model.fileName.get(k)), model.domainPageUrl.get(k)),
								String(written(k).length),
								String(written(k).filter((r) => r.generated).length),
							]),
						),
					]
				: []),
			...(sequenceRows.length ? ['## Rule sequences', table(['Sequence', 'Main rule files'], sequenceRows)] : []),
		].join('\n\n'),
	});
	return { indexUrl, ruleFiles: model.ruleFiles.length, ...totals };
}

/** The workspace's rule counts, each rule once however many main rule files run it. */
function ruleTotals(rules) {
	return {
		rules: rules.length,
		described: rules.filter((r) => r.description).length,
		generated: rules.filter((r) => r.generated).length,
		missing: rules.filter((r) => !r.description && !r.generated).length,
		partial: rules.filter((r) => (r.variants ?? []).some((v) => v.reading?.partial || v.reading?.error)).length,
		differing: rules.filter((r) => (r.variants ?? []).length > 1).length,
	};
}

// ---------------------------------------------------------------------------
// Indexes

function writeReferenceIndex({ guideName, model, settings }, write, domainPages, rulePages) {
	const classes = model.classes.filter((c) => model.fileName.has(c.source_file_key)).length;
	const counts = {
		classes,
		domainTables: model.domainTables.length,
		lookups: model.lookupTables.length,
		mainRuleFiles: model.mains.length,
		ruleFiles: rulePages.ruleFiles,
		rules: rulePages.rules,
		described: rulePages.described,
		generated: rulePages.generated,
		missing: rulePages.missing,
		partial: rulePages.partial,
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
						`${counts.rules} rules, ${counts.described} of them with a description, each on the page of its file: ` +
							`${counts.mainRuleFiles} main rule files and ${counts.ruleFiles} files they include`,
					],
				],
			),
			`**${counts.described} of the workspace's ${counts.rules} rules carry a description**` +
				(counts.generated ? `; ${counts.generated} are generated by the compiler from the classes and have none. ` : '. ') +
				(settings.ruleText
					? "The rules' text is shown, as the workspace's `docs/workspace_docs.json` asks, with how the engine reads each rule, generated from the compiled rule" +
						(counts.partial ? `; ${counts.partial} of the readings are partial.` : '.')
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
