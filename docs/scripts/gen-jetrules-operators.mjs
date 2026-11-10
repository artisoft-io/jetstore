// The JetRules operators, from the list the compiler embeds and the rule
// engine's test checks against its operator factory.
import { join } from 'node:path';
import { OUT_DIR, code, readSource, table, writePage } from './lib.mjs';

const SOURCE = 'jets/compilerv2/compiler/operators.txt';

export function generateJetrulesOperators() {
	const operators = { binary: [], unary: [] };
	let category = '';
	for (const [n, raw] of readSource(SOURCE).split('\n').entries()) {
		const line = raw.trim();
		if (line === '') continue;
		if (line.startsWith('#')) {
			// The last comment line above a run of operators names their category.
			category = line.replace(/^#\s*/, '');
			continue;
		}
		const [arity, name, ...flags] = line.split(/\s+/);
		if (!(arity in operators) || !name || flags.some((f) => f !== 'writes_graph')) {
			throw new Error(`${SOURCE}:${n + 1}: cannot read operator line "${line}"`);
		}
		operators[arity].push({ category, name, writes: flags.includes('writes_graph') });
	}
	if (operators.binary.length + operators.unary.length === 0) {
		throw new Error(`${SOURCE}: no operators found`);
	}

	const section = (arity, intro) => {
		const rows = operators[arity].map((op) => [op.category, code(op.name), op.writes ? 'yes' : '']);
		return `## ${arity === 'binary' ? 'Binary' : 'Unary'} operators\n\n${intro}\n\n${table(['Category', 'Operator', 'Writes to the graph'], rows)}`;
	};

	const body = [
		`The rule compiler accepts these operators, and no others, in a rule's expressions; the rule engine registers each of them under the same arity. ` +
			`${operators.binary.length} are binary and ${operators.unary.length} unary.`,
		'*Writes to the graph* marks an operator that inserts triples into the session when it is evaluated. ' +
			'The compiler refuses such an operator in a rule\'s filter, where evaluating an expression must not change the graph.',
		section('binary', 'A binary operator is written between its two arguments: `(?a + ?b)`, `(ex:Table lookup ?key)`.'),
		section('unary', 'A unary operator is written before its one argument: `(to_text ?amount)`, `(create_entity ?key)`.'),
	].join('\n\n');

	const file = 'jetrules-operators.md';
	writePage(file, {
		title: 'JetRules operators',
		description: 'Every operator the JetRules compiler accepts, by arity and category.',
		sources: [SOURCE],
		body,
	});
	return join(OUT_DIR, file);
}
