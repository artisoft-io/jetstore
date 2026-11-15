// node --test: the workspace generator on a fixture workspace built from
// jets/workspace_schema.sql. Every column that carries a workspace's values
// holds a sentinel, and lookup.db beside it holds a sentinel row: with
// rule_text absent none may reach a page or the manifest, and none of those
// columns may even be queried; with rule_text true the rule's text is shown,
// with its reading generated from the compiled rule.
//
// The fixture's rules are laid out as the compiler writes them: a rule written
// in a file two main rule files include is a row under each, with the file it
// is written in as authored_file_key; the compiler's ci_ rule is its class's
// file's; and one row has no authored file.
import { test } from 'node:test';
import assert from 'node:assert/strict';
import { DatabaseSync } from 'node:sqlite';
import { mkdirSync, mkdtempSync, readdirSync, readFileSync, rmSync, writeFileSync } from 'node:fs';
import { tmpdir } from 'node:os';
import { join, relative } from 'node:path';
import { REPO_ROOT } from './lib.mjs';
import { QUERIES, collapseRules, generateWorkspaceGuide, oneLine, pageSlugger, pageUrl, unquote } from './workspace_docs.mjs';

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
		[5, 'jet_rules/fixture_rules.jr', 0],
		[6, 'jet_rules/fixture_resources.jr', 0],
	]);
	// What each main rule file includes, in the order the compiler read them.
	run('INSERT INTO main_support_files (main_file_key, support_file_key) VALUES (?, ?)', [
		[1, 2],
		[1, 3],
		[1, 5],
		[1, 6],
		[4, 2],
		[4, 5],
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
		'INSERT INTO jet_rules (key, name, optimization, salience, authored_label, normalized_label, label, source_file_key, authored_file_key) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)',
		[
			// Written in fixture_rules.jr, which both main rule files include: a row under each.
			[1, 'FX_Described01', 1, 10, SENTINELS.authoredLabel, SENTINELS.normalizedLabel, `[FX_Described01]: ${SENTINELS.label};`, 1, 5],
			[2, 'FX_Undescribed02', 1, 100, SENTINELS.authoredLabel, SENTINELS.normalizedLabel, SENTINELS.label, 1, 1],
			[3, 'FX_Second01', 1, 100, SENTINELS.authoredLabel, SENTINELS.normalizedLabel, SENTINELS.label, 4, 4],
			[4, 'FX_Described01', 1, 10, SENTINELS.authoredLabel, SENTINELS.normalizedLabel, `[FX_Described01]: ${SENTINELS.label};`, 4, 5],
			// The compiler's rule for the class fx:Claim, in its class's file, under each main rule file.
			[5, 'ci_fx:Claim', 1, 100, SENTINELS.authoredLabel, SENTINELS.normalizedLabel, SENTINELS.label, 1, 2],
			[6, 'ci_fx:Claim', 1, 100, SENTINELS.authoredLabel, SENTINELS.normalizedLabel, SENTINELS.label, 4, 2],
			// The compiler had no file name for it.
			[7, 'FX_Unrecorded01', 1, 100, SENTINELS.authoredLabel, SENTINELS.normalizedLabel, SENTINELS.label, 4, null],
		],
	);
	run('INSERT INTO rule_properties (rule_key, name, value) VALUES (?, ?, ?)', [
		[1, 's', '10'],
		// As the compiler stores a string property: the token, quotes and escapes included.
		[1, 'description', '"The claim\'s \\"primary\\" code is one of its codes"'],
		[2, 'i', 'true'],
		[4, 's', '10'],
		[4, 'description', '"The claim\'s \\"primary\\" code is one of its codes"'],
		[5, 'i', 'true'],
		[6, 'i', 'true'],
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
			// The same rule in second_main.jr's network.
			[3, 1, 'antecedent', 2, 4, 2, null, 5, null, 0, 4, 0, null, 0],
			[4, 1, 'consequent', 2, 4, null, 3, null, null, 0, 4, 0, 10, 0],
		],
	);
	run('INSERT INTO rule_terms (rule_key, rete_node_key, is_antecedent) VALUES (?, ?, ?)', [
		[1, 1, 1],
		[1, 2, 0],
		[2, 1, 1],
		[4, 3, 1],
		[4, 4, 0],
	]);
	run(
		'INSERT INTO unit_tests (path, test_name, process_name, legacy_mode, comment, findings) VALUES (?, ?, ?, ?, ?, ?)',
		[
			['jet_rules/unit_tests/fx/fx.tst.json', 'the fixture test', 'jet_rules/fixture_main.jr', 'legacy', null, null],
			['jet_rules/unit_tests/fx/bad.tst.json', 'a refused test', 'jet_rules/fixture_main.jr', null, null, '[{"error":"x"}]'],
		],
	);
	db.close();

	// The main rule file's compiled model, beside it, which the generator does not read.
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
		const rules = pages[INCLUDED];
		assert.ok(rules.includes(`[FX_Described01]: ${SENTINELS.label};`), 'the label shows');
		assert.ok(rules.includes('*How the engine reads it*'), "the rule's generated reading shows beside its text");
		const everything = Object.values(pages).join('\n') + manifestText;
		for (const [where, sentinel] of Object.entries(SENTINELS)) {
			if (where !== 'label') assert.ok(!everything.includes(sentinel), `the ${where} sentinel reached the output`);
		}
	} finally {
		rmSync(fixture.root, { recursive: true, force: true });
	}
});

const MAIN = join('reference', 'rules', 'fixture_main.md');
const SECOND = join('reference', 'rules', 'second_main.md');
const INCLUDED = join('reference', 'rules', 'included', 'fixture_rules.md');
const RESOURCES = join('reference', 'rules', 'included', 'fixture_resources.md');
const DOMAIN = join('reference', 'domain', 'fixture_model.md');

test('the pages: descriptions, files, lookups readers, unit tests and the rules without a description', () => {
	const fixture = buildFixture();
	try {
		const { pages, result } = generate(fixture, { ruleText: undefined });
		const main = pages[MAIN];
		assert.match(main, /^title: "fixture_main\.jr"$/m);
		assert.match(main, /^kind: reference$/m);
		assert.ok(main.includes('runs 3 rules: 1 written in it, 0 of them with a description'), 'the rows it runs, the rules written in it');
		assert.ok(
			main.includes(
				'1. [`data_model/fixture_model.jr`](/docs/workspace/reference/domain/fixture_model/) (1 rule)\n' +
					'2. [`lookups/fixture_lookups.jr`](/docs/workspace/reference/lookups/)\n' +
					'3. [`jet_rules/fixture_rules.jr`](/docs/workspace/reference/rules/included/fixture_rules/) (1 rule)\n' +
					'4. [`jet_rules/fixture_resources.jr`](/docs/workspace/reference/rules/included/fixture_resources/)',
			),
			'the files it includes, in the order read, each linked to its own page',
		);
		assert.ok(main.includes('### FX\\_Undescribed02'));
		assert.ok(!main.includes('### FX\\_Described01'), "an included file's rule is on that file's page, not the main's");
		assert.ok(main.includes('the fixture test') && main.includes('refused by the compile'));
		assert.match(main, /## Rules with no description\n\n- \[`FX_Undescribed02`\]\(\/docs\/workspace\/reference\/rules\/fixture_main\/#fx_undescribed02\)/);

		const included = pages[INCLUDED];
		assert.match(included, /^title: "fixture_rules\.jr"$/m);
		assert.ok(
			included.includes(
				'It is included by 2 main rule files, and its rules run under each, unless a rule says otherwise: ' +
					'[`jet_rules/fixture_main.jr`](/docs/workspace/reference/rules/fixture_main/), ' +
					'[`jet_rules/second_main.jr`](/docs/workspace/reference/rules/second_main/).',
			),
		);
		assert.equal(included.split('### FX\\_Described01').length, 2, 'a rule under two main rule files is shown once');
		assert.ok(included.includes('The claim\'s "primary" code is one of its codes'), 'the description, unquoted');
		assert.ok(!included.includes('Runs under:'), 'it runs under every main rule file that includes its file');
		assert.ok(!included.includes('Rules with no description'));
		assert.ok(pages[RESOURCES].includes('No rule is written in this file.'), 'an included file with no rule has a page');

		const second = pages[SECOND];
		assert.ok(second.includes('### FX\\_Second01') && second.includes('### FX\\_Unrecorded01'));
		assert.ok(
			second.includes('- Source file: not recorded; shown on the page of the main rule file it runs under'),
			'a rule with no authored file is its main rule file\'s, not guessed',
		);
		assert.ok(!second.includes('ci_fx:Claim') && !main.includes('ci_fx:Claim'));

		const domain = pages[DOMAIN];
		assert.ok(domain.includes('## fx:Claim') && domain.includes('`fx:hasMember` | object | `fx:Member`'));
		assert.ok(domain.includes('## Rules written in this file') && domain.includes('### ci\\_fx:Claim'), "the compiler's rule is on its class's page");
		assert.ok(domain.includes('*Generated by the compiler.*'));
		assert.ok(!domain.includes('Rules with no description'), 'a generated rule is not listed as missing a description');
		assert.ok(pages[join('reference', 'domain', 'tables.md')].includes('Maps the class [`fx:Claim`]'));

		const lookups = pages[join('reference', 'lookups.md')];
		assert.ok(
			lookups.includes('Read by: [`FX_Described01`](/docs/workspace/reference/rules/included/fixture_rules/#fx_described01)\n'),
			'a reader is named once, on its own file\'s page',
		);
		assert.ok(lookups.includes('Read by: none found'), 'the unread lookup');

		const rulesIndex = pages[join('reference', 'rules', 'index.md')];
		assert.ok(rulesIndex.includes("The workspace's rules: 5, 1 of them with a description, and 1 generated by the compiler"));
		assert.ok(rulesIndex.includes('| [`jet_rules/fixture_main.jr`](/docs/workspace/reference/rules/fixture_main/) | 4 | 3 | 1 | 0 |'));
		assert.ok(rulesIndex.includes('| [`jet_rules/fixture_rules.jr`](/docs/workspace/reference/rules/included/fixture_rules/) | 2 | 1 | 1 |'));
		assert.ok(rulesIndex.includes('| [`data_model/fixture_model.jr`](/docs/workspace/reference/domain/fixture_model/) | 1 | 1 |'));

		const index = pages[join('reference', 'index.md')];
		assert.ok(index.includes("**1 of the workspace's 5 rules carry a description**; 1 are generated"), 'each rule counted once');
		assert.match(pages['index.md'], /^kind: overview$/m);
		assert.ok(pages[join('customisation', 'A Site Operator.md')].includes('Written.'));
		for (const [path, text] of Object.entries(pages)) {
			if (!path.endsWith('.md')) continue;
			for (const field of ['title', 'description', 'kind']) {
				assert.match(text, new RegExp(`^${field}: `, 'm'), `${path} has no ${field}`);
			}
		}
		assert.deepEqual(
			[result.counts.rules, result.counts.described, result.counts.generated, result.counts.missing],
			[5, 1, 1, 3],
		);
	} finally {
		rmSync(fixture.root, { recursive: true, force: true });
	}
});

test('the manifest maps each workspace file to its own page', () => {
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
				'jet_rules/fixture_resources.jr': '/docs/workspace/reference/rules/included/fixture_resources/',
				'jet_rules/fixture_rules.jr': '/docs/workspace/reference/rules/included/fixture_rules/',
				'jet_rules/second_main.jr': '/docs/workspace/reference/rules/second_main/',
				'lookups/fixture_lookups.jr': '/docs/workspace/reference/lookups/',
			},
		});
	} finally {
		rmSync(fixture.root, { recursive: true, force: true });
	}
});

test('a rule run by two main rule files is read once when its compiled rows agree, and each version is shown when not', () => {
	const fixture = buildFixture();
	try {
		const agree = generate(fixture, { ruleText: true });
		const included = agree.pages[INCLUDED];
		assert.equal(included.split('*How the engine reads it*').length, 2, 'one reading for the two rows');
		assert.ok(!included.includes('compiled differently'));

		// Under second_main.jr the rule loses its consequent: its two rows now read differently.
		const db = new DatabaseSync(join(fixture.ws, 'workspace.db'));
		db.exec('DELETE FROM rule_terms WHERE rule_key = 4 AND is_antecedent = 0');
		db.close();
		const differ = generate(fixture, { ruleText: true });
		const page = differ.pages[INCLUDED];
		assert.ok(page.includes('*It is compiled differently under the main rule files that run it*'));
		assert.ok(page.includes('**Under [`jet_rules/fixture_main.jr`](/docs/workspace/reference/rules/fixture_main/):**'));
		assert.ok(page.includes('**Under [`jet_rules/second_main.jr`](/docs/workspace/reference/rules/second_main/):**'));
		assert.ok(differ.pages[join('reference', 'rules', 'index.md')].includes('1 rule is compiled differently under different main rule files'));
	} finally {
		rmSync(fixture.root, { recursive: true, force: true });
	}
});

test('a rule under some of the main rule files that include its file says which', () => {
	const fixture = buildFixture();
	try {
		const db = new DatabaseSync(join(fixture.ws, 'workspace.db'));
		db.exec('DELETE FROM jet_rules WHERE key = 4');
		db.close();
		const { pages } = generate(fixture, { ruleText: undefined });
		assert.ok(pages[INCLUDED].includes('- Runs under: [`jet_rules/fixture_main.jr`](/docs/workspace/reference/rules/fixture_main/)\n'));
	} finally {
		rmSync(fixture.root, { recursive: true, force: true });
	}
});

test('a workspace.db compiled before rules recorded their file fails, naming the column', () => {
	const fixture = buildFixture();
	try {
		const db = new DatabaseSync(join(fixture.ws, 'workspace.db'));
		db.exec('ALTER TABLE jet_rules DROP COLUMN authored_file_key');
		db.close();
		assert.throws(() => generate(fixture, { ruleText: undefined }), /jet_rules has no authored_file_key/);
	} finally {
		rmSync(fixture.root, { recursive: true, force: true });
	}
});

test('collapseRules: one rule per name and file, a name written twice is two rules, an unrecorded file is not collapsed', () => {
	const fileName = new Map([
		[1, 'a.jr'],
		[2, 'b.jr'],
		[3, 'shared.jr'],
		[4, 'model.jr'],
	]);
	const rows = [
		{ key: 1, name: 'R', source_file_key: 1, authored_file_key: 3 },
		{ key: 2, name: 'R', source_file_key: 1, authored_file_key: 3 },
		{ key: 3, name: 'R', source_file_key: 2, authored_file_key: 3 },
		{ key: 4, name: 'R', source_file_key: 2, authored_file_key: 3 },
		{ key: 5, name: 'X', source_file_key: 1, authored_file_key: null },
		{ key: 6, name: 'X', source_file_key: 2, authored_file_key: null },
		{ key: 7, name: 'ci_m:C', source_file_key: 1, authored_file_key: 4 },
		{ key: 8, name: 'Y', source_file_key: 1, authored_file_key: 99 },
	];
	const rules = collapseRules(rows, fileName, [{ name: 'm:C', source_file_key: 4 }]);
	assert.deepEqual(
		rules.map((r) => [r.name, r.fileKey, r.recorded, r.generated, r.rows.map((x) => x.key), r.mains]),
		[
			['R', 3, true, false, [1, 3], [1, 2]],
			['R', 3, true, false, [2, 4], [1, 2]],
			['X', 1, false, false, [5], [1]],
			['X', 2, false, false, [6], [2]],
			['ci_m:C', 4, true, true, [7], [1]],
			['Y', 1, false, false, [8], [1]],
		],
	);
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

// Matched without case: the site's addresses are lower case, so Reference/ or
// REFERENCE.md would take the generated pages' address too.
test('a workspace docs/ folder holding Reference/ or REFERENCE.md fails too', () => {
	for (const make of [(d) => mkdirSync(join(d, 'Reference')), (d) => writeFileSync(join(d, 'REFERENCE.md'), '')]) {
		const fixture = buildFixture();
		try {
			make(fixture.docs);
			assert.throws(() => generate(fixture, { ruleText: undefined }), /may not hold reference/);
		} finally {
			rmSync(fixture.root, { recursive: true, force: true });
		}
	}
});

// Starlight's loader skips a file whose name starts with _, so it is no page: it is
// copied, for a page that imports it, and is neither in the manifest nor listed.
test('a written file whose name starts with _ is copied, and is not a page', () => {
	const fixture = buildFixture();
	try {
		writeFileSync(join(fixture.docs, 'customisation', '_partial.md'), 'A partial.\n');
		writeFileSync(join(fixture.docs, '_shared.mdx'), 'Shared.\n');
		const { pages, manifestText, result } = generate(fixture, { ruleText: undefined });
		assert.equal(pages[join('customisation', '_partial.md')], 'A partial.\n', 'copied');
		assert.equal(pages['_shared.mdx'], 'Shared.\n', 'copied');
		const manifest = JSON.parse(manifestText);
		assert.deepEqual(
			Object.keys(manifest.pages).filter((k) => k.startsWith('docs/')),
			['docs/customisation/A Site Operator.md'],
		);
		assert.ok(!result.pages.some((p) => p.includes('/_') || p.startsWith('_')), 'not counted as a page');
		assert.ok(!pages['index.md'].includes('_partial') && !pages['index.md'].includes('_shared'), 'not listed');
	} finally {
		rmSync(fixture.root, { recursive: true, force: true });
	}
});

// Astro gives a heading whose id an earlier heading on the page has taken -1, -2.
// Two rules of one name in one file are two headings, and each is linked to its own;
// a rule named like a section heading above it takes the next id too.
test('two rules of one name in one file each have their own anchor, and links reach each', () => {
	const fixture = buildFixture();
	try {
		const db = new DatabaseSync(join(fixture.ws, 'workspace.db'));
		const add = db.prepare(
			'INSERT INTO jet_rules (key, name, optimization, salience, source_file_key, authored_file_key) VALUES (?, ?, 1, 100, ?, ?)',
		);
		// Written twice in fixture_rules.jr, which both main rule files include, and
		// a rule named Rules, as the included page's section heading is.
		for (const [key, name, main] of [
			[20, 'FX_Twice', 1],
			[21, 'FX_Twice', 1],
			[22, 'Rules', 1],
			[23, 'FX_Twice', 4],
			[24, 'FX_Twice', 4],
			[25, 'Rules', 4],
		]) {
			add.run(key, name, main, 5);
		}
		db.prepare('INSERT INTO rule_properties (rule_key, name, value) VALUES (?, ?, ?)').run(20, 'description', '"The first."');
		db.close();
		const { pages } = generate(fixture, { ruleText: undefined });
		const included = pages[INCLUDED];
		assert.equal(included.split('### FX\\_Twice').length, 3, 'two headings');
		const base = '/docs/workspace/reference/rules/included/fixture_rules/';
		assert.ok(
			included.includes(`## Rules with no description\n\n- [\`FX_Twice\`](${base}#fx_twice-1)\n- [\`Rules\`](${base}#rules-1)`),
			'the second FX_Twice, the first having a description, and Rules, whose id the section heading took first',
		);
	} finally {
		rmSync(fixture.root, { recursive: true, force: true });
	}
});

test('pageSlugger numbers a repeated heading as github-slugger does', () => {
	const slug = pageSlugger(['Rules']);
	assert.deepEqual(
		['FX_Twice', 'FX_Twice', 'Rules', 'fx_twice-1', 'FX_Twice', 'ci_fx:Claim'].map(slug),
		['fx_twice', 'fx_twice-1', 'rules-1', 'fx_twice-1-1', 'fx_twice-2', 'ci_fxclaim'],
	);
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

// unquote reads a property's token as the compiler's UnquoteString reads a
// string literal (jets/compilerv2/compiler/string_literal.go), one row per rule.
// String.raw keeps each backslash as it is written in the rule file.
test('unquote reads the escapes the compiler reads, and keeps any other backslash', () => {
	for (const [token, want, why] of [
		[String.raw`"a \"b\""`, 'a "b"', 'a double quote'],
		[String.raw`"a\\b"`, 'a\\b', 'a backslash'],
		[String.raw`"a\tb"`, 'a\tb', 'a tab'],
		[String.raw`"a\nb"`, 'a\nb', 'a line feed'],
		[String.raw`"a\rb"`, 'a\rb', 'a carriage return'],
		[String.raw`"^(\d\d\d)"`, String.raw`^(\d\d\d)`, 'an unknown escape is kept, backslash and all'],
		[String.raw`"ends with a backslash \"`, 'ends with a backslash \\', 'a backslash at the end is kept'],
		[String.raw`"ends with a backslash \\"`, 'ends with a backslash \\', 'an escaped backslash at the end'],
		[String.raw`"\\\"quoted\\\""`, '\\"quoted\\"', 'an escaped backslash, then an escaped quote'],
		[String.raw`"\\n"`, String.raw`\n`, 'an escaped backslash before n is a backslash and an n'],
		[String.raw`a\nb`, String.raw`a\nb`, 'no quotes: returned as it is'],
		['10', '10', 'no quotes: a number'],
		['"', '"', 'a lone double quote'],
		['""', '', 'empty'],
	]) {
		assert.equal(unquote(token), want, why);
	}
	assert.equal(unquote(null), '');
	assert.equal(oneLine('a\n# b\r\n---\r| c |\td'), 'a # b --- | c |\td');
});

// A description or a property holding a line break, written \n or \r, reads as
// one line: the line after the break must not become a heading, a thematic
// break (a setext heading under the line before it) or a table row.
test('a line break in a description or a property is a space on the page', () => {
	const fixture = buildFixture();
	try {
		const db = new DatabaseSync(join(fixture.ws, 'workspace.db'));
		const add = db.prepare('INSERT INTO rule_properties (rule_key, name, value) VALUES (?, ?, ?)');
		add.run(7, 'description', String.raw`"First line\n# Not a heading\r\n---\r| a | b |\tend"`);
		add.run(7, 'note', String.raw`"one\n# two"`);
		db.close();
		const { pages } = generate(fixture, { ruleText: undefined });
		const second = pages[SECOND];
		assert.ok(
			second.includes('### FX\\_Unrecorded01\n\nFirst line # Not a heading --- \\| a \\| b \\|\tend\n\n'),
			'the description, on one line',
		);
		assert.ok(second.includes('`note=one # two`'), 'the property, on one line');
		const lines = second.split('\n');
		assert.ok(!lines.some((l) => /^(# |\|)/.test(l)), 'no stray heading or table row');
		assert.equal(lines.filter((l) => l === '---').length, 2, 'the front matter only');
	} finally {
		rmSync(fixture.root, { recursive: true, force: true });
	}
});

test('pageUrl', () => {
	assert.equal(pageUrl('index.md'), '/docs/workspace/');
	assert.equal(pageUrl('customisation/index.md'), '/docs/workspace/customisation/');
	assert.equal(pageUrl('customisation/Site Ops.md'), '/docs/workspace/customisation/site-ops/');
	assert.equal(pageUrl('x.md', '---\nslug: workspace/elsewhere\n---\n'), '/docs/workspace/elsewhere/');
});
