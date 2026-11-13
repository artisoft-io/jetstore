// node --test: the workspace generator on a fixture workspace built from
// jets/workspace_schema.sql. Every column that carries a workspace's values
// holds a sentinel, and lookup.db beside it holds a sentinel row: with
// rule_text absent none may reach a page or the manifest, and none of those
// columns may even be queried; with rule_text true the rule's text is shown.
import { test } from 'node:test';
import assert from 'node:assert/strict';
import { DatabaseSync } from 'node:sqlite';
import { mkdirSync, mkdtempSync, readdirSync, readFileSync, rmSync, writeFileSync } from 'node:fs';
import { tmpdir } from 'node:os';
import { join, relative } from 'node:path';
import { REPO_ROOT } from './lib.mjs';
import { QUERIES, generateWorkspaceGuide, pageUrl, unquote } from './workspace_docs.mjs';

const SENTINELS = {
	resourceValue: 'SENTINEL_RESOURCE_VALUE_7f3a',
	tripleValue: 'SENTINEL_TRIPLE_VALUE_7f3a',
	config: 'SENTINEL_CONFIG_VALUE_7f3a',
	label: 'SENTINEL_RULE_LABEL_7f3a',
	authoredLabel: 'SENTINEL_AUTHORED_LABEL_7f3a',
	normalizedLabel: 'SENTINEL_NORMALIZED_LABEL_7f3a',
	lookupRow: 'SENTINEL_LOOKUP_ROW_7f3a',
	modelLabel: 'SENTINEL_MODEL_LABEL_7f3a',
	modelResource: 'SENTINEL_MODEL_RESOURCE_7f3a',
};

// The queries that may read a value column; only ruleText is allowed to, and
// only when the workspace asks.
const VALUE_COLUMNS = [/\btriples\b/i, /\bjetstore_config\b/i, /\blabel\b/i, /_label\b/i, /resources\.value/i, /select\s+\*/i];

function buildFixture() {
	const root = mkdtempSync(join(tmpdir(), 'workspace-docs-test-'));
	const ws = join(root, 'ws');
	mkdirSync(join(ws, 'jet_rules'), { recursive: true });
	const db = new DatabaseSync(join(ws, 'workspace.db'));
	db.exec(readFileSync(join(REPO_ROOT, 'jets', 'workspace_schema.sql'), 'utf8'));
	const run = (sql, rows) => {
		const stmt = db.prepare(sql);
		for (const r of rows) stmt.run(...r);
	};
	run('INSERT INTO workspace_control (key, source_file_name, is_main) VALUES (?, ?, ?)', [
		[1, 'jet_rules/fixture_main.jr', 1],
		[2, 'data_model/fixture_model.jr', 0],
		[3, 'lookups/fixture_lookups.jr', 0],
		[4, 'jet_rules/second_main.jr', 1],
	]);
	run('INSERT INTO domain_classes (key, name, as_table, source_file_key) VALUES (?, ?, ?, ?)', [
		[1, 'owl:Thing', 0, -1],
		[2, 'fx:Claim', 1, 2],
		[3, 'fx:Member', 0, 2],
	]);
	run('INSERT INTO base_classes (domain_class_key, base_class_key) VALUES (?, ?)', [
		[2, 1],
		[3, 1],
	]);
	run('INSERT INTO data_properties (key, domain_class_key, name, type, as_array, is_grouping) VALUES (?, ?, ?, ?, ?, ?)', [
		[1, 2, 'fx:Claim_Number', 'text', 0, 1],
		[2, 2, 'fx:Codes', 'text', 1, 0],
	]);
	run('INSERT INTO object_properties (key, domain_class_key, name, type, as_array) VALUES (?, ?, ?, ?, ?)', [
		[1, 2, 'fx:hasMember', 'fx:Member', 0],
	]);
	run('INSERT INTO domain_tables (key, domain_class_key, name) VALUES (?, ?, ?)', [[1, 2, 'fx:Claim']]);
	run(
		'INSERT INTO domain_columns (domain_table_key, data_property_key, name, type, as_array, is_object, is_grouping) VALUES (?, ?, ?, ?, ?, ?, ?)',
		[[1, 1, 'fx:Claim_Number', 'text', 0, 0, 1]],
	);
	run(
		'INSERT INTO lookup_tables (key, name, table_name, csv_file, lookup_key, lookup_resources, source_file_key) VALUES (?, ?, ?, ?, ?, ?, ?)',
		[
			[1, 'FixtureCodeLookup', null, 'lookups/fixture_codes.csv', 'CODE', 'CODE,LABEL', 3],
			[2, 'UnreadLookup', null, 'lookups/unread.csv', 'KEY', 'KEY', 3],
		],
	);
	run('INSERT INTO lookup_columns (lookup_table_key, name, type, as_array) VALUES (?, ?, ?, ?)', [
		[1, 'CODE', 'text', 0],
		[1, 'LABEL', 'text', 0],
		[2, 'KEY', 'text', 0],
	]);
	run(
		'INSERT INTO jet_rules (key, name, optimization, salience, authored_label, normalized_label, label, source_file_key) VALUES (?, ?, ?, ?, ?, ?, ?, ?)',
		[
			[1, 'FX_Described01', 1, 10, SENTINELS.authoredLabel, SENTINELS.normalizedLabel, `[FX_Described01]: ${SENTINELS.label};`, 1],
			[2, 'FX_Undescribed02', 1, 100, SENTINELS.authoredLabel, SENTINELS.normalizedLabel, SENTINELS.label, 1],
			[3, 'FX_Second01', 1, 100, SENTINELS.authoredLabel, SENTINELS.normalizedLabel, SENTINELS.label, 4],
		],
	);
	run('INSERT INTO rule_properties (rule_key, name, value) VALUES (?, ?, ?)', [
		[1, 's', '10'],
		// As the compiler stores a string property: the token, quotes and escapes included.
		[1, 'description', '"The claim\'s \\"primary\\" code is one of its codes"'],
		[2, 'i', 'true'],
	]);
	run(
		'INSERT INTO resources (key, type, id, value, symbol, is_binded, inline, source_file_key, vertex, row_pos) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)',
		[
			[1, 'resource', 'FixtureCodeLookup', SENTINELS.resourceValue, null, null, null, 1, null, null],
			[2, 'var', '?x01', SENTINELS.resourceValue, null, 1, null, 1, 1, 0],
			[3, 'text', 'fx:literal', SENTINELS.resourceValue, null, null, 1, 1, null, null],
			[4, 'resource', 'fx:Claim_Number', SENTINELS.tripleValue, null, null, null, 1, null, null],
		],
	);
	run('INSERT INTO triples (subject_key, predicate_key, object_key, source_file_key) VALUES (?, ?, ?, ?)', [[4, 4, 3, 1]]);
	run('INSERT INTO jetstore_config (config_key, config_value, source_file_key) VALUES (?, ?, ?)', [
		['$max_looping', SENTINELS.config, 1],
	]);
	// lookup(FixtureCodeLookup, ?x01) in rule 1's consequent; a filter in rule 2 that does not look up.
	run(
		'INSERT INTO expressions (key, type, arg0_key, arg1_key, arg2_key, arg3_key, arg4_key, arg5_key, op, source_file_key) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)',
		[
			[1, 'resource', 1, null, null, null, null, null, null, 1],
			[2, 'resource', 2, null, null, null, null, null, null, 1],
			[3, 'binary', 1, 2, null, null, null, null, 'lookup', 1],
			[4, 'resource', 3, null, null, null, null, null, null, 1],
			[5, 'binary', 2, 4, null, null, null, null, '==', 1],
		],
	);
	run(
		'INSERT INTO rete_nodes (key, vertex, type, subject_key, predicate_key, object_key, obj_expr_key, filter_expr_key, normalizedLabel, parent_vertex, source_file_key, is_negation, salience, consequent_seq) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)',
		[
			[1, 1, 'antecedent', 2, 4, 2, null, 5, null, 0, 1, 0, null, 0],
			[2, 1, 'consequent', 2, 4, null, 3, null, null, 0, 1, 0, 10, 0],
		],
	);
	run('INSERT INTO rule_terms (rule_key, rete_node_key, is_antecedent) VALUES (?, ?, ?)', [
		[1, 1, 1],
		[1, 2, 0],
		[2, 1, 1],
	]);
	run(
		'INSERT INTO unit_tests (path, test_name, process_name, legacy_mode, comment, findings) VALUES (?, ?, ?, ?, ?, ?)',
		[
			['jet_rules/unit_tests/fx/fx.tst.json', 'the fixture test', 'jet_rules/fixture_main.jr', 'legacy', null, null],
			['jet_rules/unit_tests/fx/bad.tst.json', 'a refused test', 'jet_rules/fixture_main.jr', null, null, '[{"error":"x"}]'],
		],
	);
	db.close();

	// The main rule file's compiled model, beside it: names, and values the generator must not use.
	writeFileSync(
		join(ws, 'jet_rules', 'fixture_main.jr.json'),
		JSON.stringify({
			main_rule_file_name: 'jet_rules/fixture_main.jr',
			resources: [{ id: 'fx:literal', value: SENTINELS.modelResource }],
			classes: [{ name: 'fx:Claim', source_file_name: 'data_model/fixture_model.jr' }],
			lookup_tables: [{ name: 'FixtureCodeLookup', source_file_name: 'lookups/fixture_lookups.jr' }],
			jet_rules: [
				{ name: 'FX_Described01', source_file_name: 'jet_rules/fixture_rules.jr', label: SENTINELS.modelLabel },
				{ name: 'FX_Undescribed02', source_file_name: 'jet_rules/fixture_main.jr', label: SENTINELS.modelLabel },
			],
			triples: [[SENTINELS.modelResource]],
		}),
	);

	const lookupDb = new DatabaseSync(join(ws, 'lookup.db'));
	lookupDb.exec('CREATE TABLE FixtureCodeLookup (CODE TEXT, LABEL TEXT)');
	lookupDb.prepare('INSERT INTO FixtureCodeLookup VALUES (?, ?)').run('C1', SENTINELS.lookupRow);
	lookupDb.close();

	const docs = join(ws, 'docs');
	mkdirSync(join(docs, 'customisation'), { recursive: true });
	writeFileSync(
		join(docs, 'customisation', 'A Site Operator.md'),
		'---\ntitle: A site operator\ndescription: d\nkind: reference\n---\n\nWritten.\n',
	);
	return { root, ws, docs };
}

/** Every file under a directory, as {relative path: text}. */
function readTree(dir) {
	const out = {};
	const walk = (d) => {
		for (const e of readdirSync(d, { withFileTypes: true })) {
			const abs = join(d, e.name);
			if (e.isDirectory()) walk(abs);
			else out[relative(dir, abs)] = readFileSync(abs, 'utf8');
		}
	};
	walk(dir);
	return out;
}

/** Runs the generator, recording every query it prepares and every database it opens. */
function generate(fixture, { ruleText }) {
	writeFileSync(join(fixture.docs, 'workspace_docs.json'), JSON.stringify(ruleText === undefined ? {} : { rule_text: ruleText }));
	const outDir = join(fixture.root, `out-${ruleText}`);
	const publicDir = join(fixture.root, `public-${ruleText}`);
	const queries = [];
	const opened = [];
	const result = generateWorkspaceGuide({
		dbPath: join(fixture.ws, 'workspace.db'),
		name: 'fixture_ws',
		docsDir: fixture.docs,
		outDir,
		publicDir,
		openDatabase: (p) => {
			opened.push(p);
			const db = new DatabaseSync(p, { readOnly: true });
			const prepare = db.prepare.bind(db);
			db.prepare = (sql) => {
				queries.push(sql);
				return prepare(sql);
			};
			return db;
		},
	});
	return { result, pages: readTree(outDir), manifestText: readFileSync(join(publicDir, 'manifest.json'), 'utf8'), queries, opened };
}

test('with rule_text absent, no value reaches a page or the manifest, and no value column is read', () => {
	const fixture = buildFixture();
	try {
		const { pages, manifestText, queries, opened } = generate(fixture, { ruleText: undefined });
		const everything = Object.values(pages).join('\n') + manifestText;
		for (const [where, sentinel] of Object.entries(SENTINELS)) {
			assert.ok(!everything.includes(sentinel), `the ${where} sentinel reached the output`);
		}
		for (const sql of queries) {
			for (const column of VALUE_COLUMNS) assert.ok(!column.test(sql), `a value column was queried: ${sql}`);
		}
		assert.ok(!queries.includes(QUERIES.ruleText));
		assert.deepEqual(opened, [join(fixture.ws, 'workspace.db')], 'only workspace.db is opened');
		assert.ok(!('workspace_docs.json' in pages), 'the settings file is not a page');
	} finally {
		rmSync(fixture.root, { recursive: true, force: true });
	}
});

test('rule_text false is the same as absent', () => {
	const fixture = buildFixture();
	try {
		const { pages, queries } = generate(fixture, { ruleText: false });
		const everything = Object.values(pages).join('\n');
		assert.ok(!everything.includes(SENTINELS.label));
		assert.ok(!queries.includes(QUERIES.ruleText));
	} finally {
		rmSync(fixture.root, { recursive: true, force: true });
	}
});

test('with rule_text true the rule text is shown, and nothing else of the values is', () => {
	const fixture = buildFixture();
	try {
		const { pages, manifestText } = generate(fixture, { ruleText: true });
		const rules = pages[join('reference', 'rules', 'fixture_main.md')];
		assert.ok(rules.includes(`[FX_Described01]: ${SENTINELS.label};`), 'the label shows');
		const everything = Object.values(pages).join('\n') + manifestText;
		for (const [where, sentinel] of Object.entries(SENTINELS)) {
			if (where !== 'label') assert.ok(!everything.includes(sentinel), `the ${where} sentinel reached the output`);
		}
	} finally {
		rmSync(fixture.root, { recursive: true, force: true });
	}
});

test('the pages: descriptions, files, lookups readers, unit tests and the rules without a description', () => {
	const fixture = buildFixture();
	try {
		const { pages, result } = generate(fixture, { ruleText: undefined });
		const rules = pages[join('reference', 'rules', 'fixture_main.md')];
		assert.match(rules, /^title: "fixture_main\.jr"$/m);
		assert.match(rules, /^kind: reference$/m);
		assert.ok(rules.includes('The claim\'s "primary" code is one of its codes'), 'the description, unquoted');
		assert.ok(rules.includes('- Source file: `jet_rules/fixture_rules.jr`'), 'the rule\'s own file, from the compiled model');
		assert.ok(rules.includes('1. [`data_model/fixture_model.jr`](/docs/workspace/reference/domain/fixture_model/)'));
		assert.ok(rules.includes('the fixture test') && rules.includes('refused by the compile'));
		assert.match(rules, /## Rules with no description\n\n- \[`FX_Undescribed02`\]\(\/docs\/workspace\/reference\/rules\/fixture_main\/#fx_undescribed02\)/);
		const second = pages[join('reference', 'rules', 'second_main.md')];
		assert.ok(second.includes('- Source file: not recorded'), 'no compiled model: not recorded, not guessed');

		const lookups = pages[join('reference', 'lookups.md')];
		assert.ok(lookups.includes('Read by: [`FX_Described01`](/docs/workspace/reference/rules/fixture_main/#fx_described01)'));
		assert.ok(lookups.includes('Read by: none found'), 'the unread lookup');

		const domain = pages[join('reference', 'domain', 'fixture_model.md')];
		assert.ok(domain.includes('## fx:Claim') && domain.includes('`fx:hasMember` | object | `fx:Member`'));
		assert.ok(pages[join('reference', 'domain', 'tables.md')].includes('Maps the class [`fx:Claim`]'));

		const index = pages[join('reference', 'index.md')];
		assert.ok(index.includes('**1 of the workspace\'s 3 rules carry a description.**'));
		assert.match(pages['index.md'], /^kind: overview$/m);
		assert.ok(pages[join('customisation', 'A Site Operator.md')].includes('Written.'));
		for (const [path, text] of Object.entries(pages)) {
			if (!path.endsWith('.md')) continue;
			for (const field of ['title', 'description', 'kind']) {
				assert.match(text, new RegExp(`^${field}: `, 'm'), `${path} has no ${field}`);
			}
		}
		assert.equal(result.counts.described, 1);
	} finally {
		rmSync(fixture.root, { recursive: true, force: true });
	}
});

test('the manifest maps each workspace file to its page', () => {
	const fixture = buildFixture();
	try {
		const { manifestText } = generate(fixture, { ruleText: undefined });
		assert.deepEqual(JSON.parse(manifestText), {
			workspace: 'fixture_ws',
			index: '/docs/workspace/',
			pages: {
				'data_model/fixture_model.jr': '/docs/workspace/reference/domain/fixture_model/',
				'docs/customisation/A Site Operator.md': '/docs/workspace/customisation/a-site-operator/',
				'jet_rules/fixture_main.jr': '/docs/workspace/reference/rules/fixture_main/',
				'jet_rules/fixture_rules.jr': '/docs/workspace/reference/rules/',
				'jet_rules/second_main.jr': '/docs/workspace/reference/rules/second_main/',
				'lookups/fixture_lookups.jr': '/docs/workspace/reference/lookups/',
			},
		});
	} finally {
		rmSync(fixture.root, { recursive: true, force: true });
	}
});

test('a workspace docs/ folder holding reference/ fails', () => {
	const fixture = buildFixture();
	try {
		mkdirSync(join(fixture.docs, 'reference'));
		assert.throws(() => generate(fixture, { ruleText: undefined }), /may not hold reference/);
	} finally {
		rmSync(fixture.root, { recursive: true, force: true });
	}
});

test('a workspace with no docs/ builds, with a generated landing page', () => {
	const fixture = buildFixture();
	try {
		rmSync(fixture.docs, { recursive: true, force: true });
		const outDir = join(fixture.root, 'out');
		const { manifest } = generateWorkspaceGuide({
			dbPath: join(fixture.ws, 'workspace.db'),
			name: 'fixture_ws',
			docsDir: fixture.docs,
			outDir,
			publicDir: join(fixture.root, 'public'),
		});
		assert.match(readFileSync(join(outDir, 'index.md'), 'utf8'), /^title: "fixture_ws"$/m);
		assert.ok(!Object.keys(manifest.pages).some((k) => k.startsWith('docs/')));
	} finally {
		rmSync(fixture.root, { recursive: true, force: true });
	}
});

test('unquote and pageUrl', () => {
	assert.equal(unquote('"a \\"b\\" \\\\ c"'), 'a "b" \\ c');
	assert.equal(unquote('10'), '10');
	assert.equal(pageUrl('index.md'), '/docs/workspace/');
	assert.equal(pageUrl('customisation/index.md'), '/docs/workspace/customisation/');
	assert.equal(pageUrl('customisation/Site Ops.md'), '/docs/workspace/customisation/site-ops/');
	assert.equal(pageUrl('x.md', '---\nslug: workspace/elsewhere\n---\n'), '/docs/workspace/elsewhere/');
});
