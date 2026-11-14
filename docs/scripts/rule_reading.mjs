// An English reading of a JetRule rule, from the compiled rule network in a
// workspace's workspace.db (jets/workspace_schema.sql), not from the rule's text.
//
// The compiler (jets/compilerv2/compiler/workspace_db_helper.go) writes a rule as
// rows of rule_terms, each naming a rete_nodes row: an antecedent or a consequent,
// whose subject, predicate and object are rows of resources, whose object may
// instead be an expression (obj_expr_key), and whose filter is an expression
// (filter_expr_key). An expression is a tree of expressions rows: a resource leaf,
// a unary operator or a binary operator. Variables are normalised per rule (?x01,
// ?x02, ...), so the reading names them: a variable typed by an rdf:type
// antecedent for its class ("the MedicalClaim"), else for the property that first
// binds it ("the Service_Date_From").
//
// The reading never guesses. A construct or operator it has no wording for is
// written "(not rendered: <name>)" in its clause, marks the reading partial, and is
// listed in unrendered; so is an aggregate whose configuration the engine refuses,
// which fails the session, as "(not rendered: <name>; <why>)". It reads the rule's literals (resources.value) and the
// configuration triples of the aggregate operators, substring_of and replace_char_of,
// so it is to be shown only where a workspace has chosen to show its rules' text.
//
// readRule(db, ruleKey) takes a node:sqlite DatabaseSync and a jet_rules.key, and
// returns { when, then, partial, unrendered }: one plain-text clause per antecedent
// and per consequent, no Markdown (the caller escapes). The same database gives the
// same text: every query is ordered.

const RDF_TYPE = 'rdf:type';

const ORDINALS = ['first', 'second', 'third', 'fourth', 'fifth', 'sixth', 'seventh', 'eighth', 'ninth', 'tenth'];

/** Comparisons and logic, written between their two operands. */
const INFIX = {
	'==': 'is',
	'!=': 'is not',
	'<': 'is less than',
	'<=': 'is at most',
	'>': 'is greater than',
	'>=': 'is at least',
	and: 'and',
	or: 'or',
	'-': 'minus',
	'*': 'times',
	'/': 'divided by',
	contains: 'contains',
	starts_with: 'starts with',
	ends_with: 'ends with',
};

const LOGIC = new Set(['and', 'or']);
/** Operators whose left-nested chain needs no parentheses: (a + b) + c reads as a + b + c. */
const ASSOCIATIVE = new Set(['and', 'or', '+', '*']);

/** Binary operators written as a phrase around their operands, a and b already rendered. */
const BINARY_PHRASE = {
	apply_format: (a, b) => `${a} formatted with ${b}`,
	literal_regex: (a, b) => `the first group of ${a} matching the pattern ${b}`,
	char_at: (a, b) => `the character of ${a} at position ${b}`,
	range: (a, b) => `a range of ${b} values starting at ${a}`,
	age_as_of: (a, b) => `the age in years of ${a} as of ${b}`,
	age_in_months_as_of: (a, b) => `the age in months of ${a} as of ${b}`,
	lookup: (a, b) => `the row of ${a} for ${b}`,
	multi_lookup: (a, b) => `the rows of ${a} for ${b}`,
};

/** Unary operators, the argument already rendered. */
const UNARY_PHRASE = {
	not: (a) => `not ${a}`,
	abs: (a) => `the absolute value of ${a}`,
	to_int: (a) => `${a} as an integer`,
	to_double: (a) => `${a} as a decimal number`,
	to_text: (a) => `${a} as text`,
	to_timestamp: (a) => `${a} as a timestamp`,
	month_period_of: (a) => `the month period of ${a}`,
	week_period_of: (a) => `the week period of ${a}`,
	day_period_of: (a) => `the day period of ${a}`,
	to_upper: (a) => `${a} in upper case`,
	to_lower: (a) => `${a} in lower case`,
	trim: (a) => `${a} with surrounding spaces removed`,
	length_of: (a) => `the length of ${a}`,
	parse_usd_currency: (a) => `${a} read as a US dollar amount`,
	uuid_md5: (a) => `an MD5 UUID made from ${a}`,
	uuid_sha1: (a) => `a SHA-1 UUID made from ${a}`,
	create_literal: (a) => `${a} as a literal`,
	create_resource: (a) => `a resource named ${a}`,
	create_uuid_resource: () => 'a new resource with a random identifier',
	is_literal: (a) => `${a} is a literal`,
	is_resource: (a) => `${a} is a resource`,
	raise_exception: (a) => `raise an error with the message ${a}`,
	lookup_rand: (a) => `a random row of ${a}`,
	multi_lookup_rand: (a) => `random rows of ${a}`,
};

const LITERAL_TYPES = new Set(['int', 'uint', 'long', 'ulong', 'double']);

/** The configuration predicates an operator's resource may carry, and their field names. */
const CONFIG_FIELDS = {
	'jets:entity_property': 'entity',
	'jets:value_property': 'value',
	'jets:operator': 'operator',
	'jets:separator': 'separator',
	'jets:from': 'from',
	'jets:length': 'length',
	'jets:replace_chars': 'replace_chars',
	'jets:replace_with': 'replace_with',
};

/** Characters a quoted literal would hide or garble, named in words. */
const CHAR_NAMES = {
	' ': 'space',
	'\t': 'tab',
	'\n': 'line break',
	'\r': 'carriage return',
	'"': 'double quote',
	'\\': 'backslash',
};

/** a; a and b; a, b and c. */
function listOf(items) {
	if (items.length <= 1) return items.join('');
	return `${items.slice(0, -1).join(', ')} and ${items[items.length - 1]}`;
}

/** Per database: the names of the model's classes and properties, read once. */
const MODEL_CACHE = new WeakMap();

function modelOf(db) {
	let m = MODEL_CACHE.get(db);
	if (!m) {
		const names = (sql) => new Set(db.prepare(sql).all().map((r) => r.name));
		m = {
			classes: names('SELECT name FROM domain_classes'),
			properties: new Set([
				...names('SELECT name FROM data_properties'),
				...names('SELECT name FROM object_properties'),
			]),
		};
		MODEL_CACHE.set(db, m);
	}
	return m;
}

/** The name after the prefix: hc:Service_Date_From is Service_Date_From. */
function localName(id) {
	const s = String(id ?? '');
	const i = s.lastIndexOf(':');
	return i >= 0 ? s.slice(i + 1) : s;
}

function article(word) {
	return /^[aeio]/i.test(word) || /^u(?!ni|s[eu])/i.test(word) ? 'an' : 'a';
}

function possessive(phrase) {
	return `${phrase}'s`;
}

function quote(text) {
	return `"${text ?? ''}"`;
}

/** The one reading of one rule; holds the rule's rows and the variables' names. */
class RuleReader {
	constructor(db, ruleKey) {
		this.db = db;
		this.model = modelOf(db);
		this.unrendered = new Set();
		const rule = db.prepare('SELECT key, source_file_key FROM jet_rules WHERE key = ?').get(ruleKey);
		if (!rule) throw new Error(`rule_reading: no rule with key ${ruleKey}`);
		this.sourceFileKey = rule.source_file_key;
		this.resourceStmt = db.prepare('SELECT key, type, id, value FROM resources WHERE key = ?');
		this.exprStmt = db.prepare(
			'SELECT key, type, arg0_key, arg1_key, op FROM expressions WHERE key = ?',
		);
		this.configStmt = db.prepare(
			`SELECT p.id AS predicate, o.type AS type, o.id AS id, o.value AS value
			 FROM triples t
			 JOIN resources s ON s.key = t.subject_key
			 JOIN resources p ON p.key = t.predicate_key
			 JOIN resources o ON o.key = t.object_key
			 WHERE s.id = ? AND t.source_file_key = ?
			   AND p.id IN ('jets:entity_property', 'jets:value_property', 'jets:operator', 'jets:separator',
			                'jets:from', 'jets:length', 'jets:replace_chars', 'jets:replace_with')
			 ORDER BY p.id, o.key`,
		);
		this.configWriteStmt = db.prepare(
			`SELECT 1 AS found
			 FROM rete_nodes n
			 JOIN resources s ON s.key = n.subject_key
			 JOIN resources p ON p.key = n.predicate_key
			 WHERE n.type = 'consequent' AND n.source_file_key = ? AND s.id = ? AND p.id = ?
			 LIMIT 1`,
		);
		const terms = db
			.prepare(
				`SELECT n.key, n.vertex, n.type, n.subject_key, n.predicate_key, n.object_key,
				        n.obj_expr_key, n.filter_expr_key, n.parent_vertex, n.is_negation,
				        n.consequent_seq, rt.is_antecedent
				 FROM rule_terms rt JOIN rete_nodes n ON n.key = rt.rete_node_key
				 WHERE rt.rule_key = ?
				 ORDER BY n.vertex, n.consequent_seq, n.key`,
			)
			.all(ruleKey);
		this.antecedents = chainOrder(terms.filter((t) => t.is_antecedent));
		this.consequents = terms
			.filter((t) => !t.is_antecedent)
			.sort((a, b) => a.consequent_seq - b.consequent_seq || a.key - b.key);
		this.resources = new Map();
		this.exprs = new Map();
		this.nameVariables();
	}

	resource(key) {
		if (key === null || key === undefined) return null;
		if (!this.resources.has(key)) this.resources.set(key, this.resourceStmt.get(key) ?? null);
		return this.resources.get(key);
	}

	expr(key) {
		if (key === null || key === undefined) return null;
		if (!this.exprs.has(key)) this.exprs.set(key, this.exprStmt.get(key) ?? null);
		return this.exprs.get(key);
	}

	/** A construct the reading has no wording for, or, with a reason, one it will not word. */
	notRendered(name, reason) {
		this.unrendered.add(name);
		return reason ? `(not rendered: ${name}; ${reason})` : `(not rendered: ${name})`;
	}

	// ---- naming the variables --------------------------------------------------------

	/** Give every variable a base name, in the order the network binds them. */
	nameVariables() {
		const classOf = new Map();
		for (const t of this.antecedents) {
			if (t.is_negation) continue;
			const s = this.resource(t.subject_key);
			const p = this.resource(t.predicate_key);
			const o = this.resource(t.object_key);
			if (s?.type === 'var' && p?.id === RDF_TYPE && o?.type === 'resource' && !classOf.has(s.id)) {
				classOf.set(s.id, localName(o.id));
			}
		}
		const base = new Map();
		const see = (res, fallback) => {
			if (res?.type !== 'var' || base.has(res.id)) return;
			base.set(res.id, classOf.get(res.id) ?? fallback);
		};
		const seeExpr = (key, depth = 0) => {
			const e = this.expr(key);
			if (!e || depth > 64) return;
			if (e.type === 'resource') see(this.resource(e.arg0_key), 'value');
			else {
				seeExpr(e.arg0_key, depth + 1);
				seeExpr(e.arg1_key, depth + 1);
			}
		};
		for (const t of [...this.antecedents, ...this.consequents]) {
			const p = this.resource(t.predicate_key);
			see(this.resource(t.subject_key), 'entity');
			see(p, 'property');
			const objectBase = p?.type === 'var' ? 'value' : p?.id === RDF_TYPE ? 'type' : localName(p?.id) || 'value';
			see(this.resource(t.object_key), objectBase);
			seeExpr(t.obj_expr_key);
			seeExpr(t.filter_expr_key);
		}
		const count = new Map();
		for (const b of base.values()) count.set(b, (count.get(b) ?? 0) + 1);
		const seen = new Map();
		this.varNames = new Map();
		for (const [id, b] of base) {
			const n = (seen.get(b) ?? 0) + 1;
			seen.set(b, n);
			const ord = count.get(b) > 1 ? (ORDINALS[n - 1] ?? `number ${n}`) : null;
			this.varNames.set(id, {
				definite: ord ? `the ${ord} ${b}` : `the ${b}`,
				indefinite: ord ? `${article(ord)} ${ord} ${b}` : `${article(b)} ${b}`,
				base: b,
			});
		}
		this.introduced = new Set();
	}

	/** A variable: indefinite where this clause binds it, definite after. */
	variable(res, { introduce = false } = {}) {
		const name = this.varNames.get(res.id) ?? { definite: 'the value', indefinite: 'a value', base: 'value' };
		if (introduce && !this.introduced.has(res.id)) {
			this.introduced.add(res.id);
			return name.indefinite;
		}
		return name.definite;
	}

	// ---- terms -----------------------------------------------------------------------

	/** A resource as a value: a variable, a literal, or a named resource. */
	value(res, opts) {
		if (!res) return this.notRendered('missing resource');
		switch (res.type) {
			case 'var':
				return this.variable(res, opts);
			case 'text':
				return quote(res.value);
			case 'date':
				return `the date ${res.value}`;
			case 'datetime':
				return `the date and time ${res.value}`;
			case 'bool':
				return `the boolean ${quote(res.value)}`;
			case 'keyword':
				return String(res.value);
			case 'volatile_resource':
				return String(res.value ?? res.id);
			case 'resource': {
				const id = res.id ?? res.value;
				if (this.model.classes.has(id) || this.model.properties.has(id)) return localName(id);
				return String(id);
			}
			default:
				if (LITERAL_TYPES.has(res.type)) return String(res.value);
				return this.notRendered(res.type);
		}
	}

	/** A resource in a predicate's place: its name, without the prefix. */
	predicate(res) {
		if (!res) return this.notRendered('missing resource');
		if (res.type === 'resource') return localName(res.id ?? res.value);
		if (res.type === 'volatile_resource') return String(res.value ?? res.id);
		return this.notRendered(res.type);
	}

	isNewVar(res) {
		return res?.type === 'var' && !this.introduced.has(res.id);
	}

	antecedentClause(t) {
		const s = this.resource(t.subject_key);
		const p = this.resource(t.predicate_key);
		const o = this.resource(t.object_key);
		const neg = !!t.is_negation;
		let clause;
		if (p?.id === RDF_TYPE && o?.type === 'resource') {
			const cls = localName(o.id);
			if (this.isNewVar(s) && !neg) {
				clause = `there is ${this.variable(s, { introduce: true })}`;
			} else if (this.isNewVar(s)) {
				clause = `there is no ${cls}`;
			} else {
				clause = `${this.value(s)} is ${neg ? 'not ' : ''}${article(cls)} ${cls}`;
			}
		} else if (p?.type === 'var') {
			const subj = this.value(s, { introduce: !neg });
			const pred = this.value(p, { introduce: !neg });
			if (neg) clause = `${subj} does not relate ${pred} to ${this.isNewVar(o) ? 'any value' : this.value(o)}`;
			else clause = `${subj} relates ${pred} to ${this.value(o, { introduce: true })}`;
		} else {
			const subj = this.value(s, { introduce: !neg });
			const pred = this.predicate(p);
			if (neg) {
				clause = this.isNewVar(o) ? `${subj} has no ${pred}` : `${subj} has no ${pred} equal to ${this.value(o)}`;
			} else if (this.isNewVar(o)) {
				const name = this.varNames.get(o.id);
				const intro = this.variable(o, { introduce: true });
				clause = name?.base === pred ? `${subj} has ${intro}` : `${subj} has ${intro} as its ${pred}`;
			} else {
				clause = `${subj} has ${this.value(o)} as its ${pred}`;
			}
		}
		if (t.filter_expr_key !== null && t.filter_expr_key !== undefined) {
			clause += `, where ${this.expression(t.filter_expr_key).text}`;
		}
		return clause;
	}

	consequentClause(t) {
		const s = this.resource(t.subject_key);
		const p = this.resource(t.predicate_key);
		const object =
			t.obj_expr_key !== null && t.obj_expr_key !== undefined
				? this.expression(t.obj_expr_key)
				: { text: this.value(this.resource(t.object_key)) };
		const objRes = t.object_key !== null && t.object_key !== undefined ? this.resource(t.object_key) : this.exprResource(t.obj_expr_key);
		if (p?.id === RDF_TYPE && objRes?.type === 'resource') {
			const cls = localName(objRes.id);
			return `${this.value(s)} is ${article(cls)} ${cls}`;
		}
		if (p?.type === 'var') return `${this.value(s)} relates ${this.value(p)} to ${object.text}`;
		return `${this.value(s)} has ${object.text} as its ${this.predicate(p)}`;
	}

	/** The resource an expression is, when it is a single resource. */
	exprResource(key) {
		const e = this.expr(key);
		return e?.type === 'resource' ? this.resource(e.arg0_key) : null;
	}

	// ---- expressions -----------------------------------------------------------------

	/**
	 * An expression as { text, op, prefix, isText }: op is set for an infix operator,
	 * so a parent knows when to put it in parentheses; isText marks a text literal or a
	 * concatenation, which decides how + reads.
	 */
	expression(key, depth = 0) {
		const e = this.expr(key);
		if (!e) return { text: this.notRendered('missing expression') };
		if (depth > 64) return { text: this.notRendered('expression too deep') };
		if (e.type === 'resource') {
			const res = this.resource(e.arg0_key);
			return { text: this.value(res), isText: res?.type === 'text' };
		}
		if (e.type === 'unary') return this.unary(e, depth);
		if (e.type === 'binary') return this.binary(e, depth);
		return { text: this.notRendered(`expression ${e.type}`) };
	}

	unary(e, depth) {
		const op = e.op;
		if (op === 'create_entity') {
			const arg = this.exprResource(e.arg0_key);
			if (arg && LITERAL_TYPES.has(arg.type) && Number(arg.value) === 0) return { text: 'a new entity' };
			return { text: `a new entity keyed by ${this.operand(this.expression(e.arg0_key, depth + 1))}` };
		}
		const phrase = UNARY_PHRASE[op];
		if (!phrase) return { text: this.notRendered(op) };
		const arg = this.operand(this.expression(e.arg0_key, depth + 1));
		return { text: phrase(arg), prefix: op === 'not' };
	}

	binary(e, depth) {
		const op = e.op;
		const aggregate = this.aggregate(e, depth);
		if (aggregate) return aggregate;
		if (op === 'substring_of') return this.substring(e, depth);
		if (op === 'replace_char_of') return this.replaceChars(e, depth);
		const lhs = this.expression(e.arg0_key, depth + 1);
		const rhs = this.expression(e.arg1_key, depth + 1);
		if (op === '+') {
			const isText = !!(lhs.isText || rhs.isText);
			const word = isText ? 'followed by' : 'plus';
			const l = this.child(lhs, op, true, isText);
			const r = this.child(rhs, op, false, isText);
			return { text: `${l} ${word} ${r}`, op, isText };
		}
		if (INFIX[op]) {
			return { text: `${this.child(lhs, op, true)} ${INFIX[op]} ${this.child(rhs, op, false)}`, op };
		}
		if (BINARY_PHRASE[op]) {
			return { text: BINARY_PHRASE[op](this.operand(lhs), this.operand(rhs)) };
		}
		return { text: this.notRendered(op) };
	}

	/**
	 * An operand of an infix operator, in parentheses unless it reads unambiguously.
	 * Joining text is associative, so a concatenation inside a concatenation needs none.
	 */
	child(c, parentOp, isLeft, parentIsText = false) {
		if (c.prefix) return `(${c.text})`;
		if (!c.op) return c.text;
		if (LOGIC.has(parentOp) && !LOGIC.has(c.op)) return c.text;
		if (parentOp === '+' && c.op === '+' && parentIsText && c.isText) return c.text;
		if (c.op === parentOp && isLeft && ASSOCIATIVE.has(parentOp)) return c.text;
		return `(${c.text})`;
	}

	/** An argument of a phrase: an infix or prefix expression goes in parentheses. */
	operand(c) {
		return c.op || c.prefix ? `(${c.text})` : c.text;
	}

	/** The operators over a subject's values: size_of, exist, exist_not and the aggregates. */
	aggregate(e, depth) {
		const op = e.op;
		if (!['size_of', 'exist', 'exist_not', 'min_of', 'max_of', 'sum_values', 'join_values', 'sorted_head'].includes(op)) {
			return null;
		}
		const subject = this.operand(this.expression(e.arg0_key, depth + 1));
		const rhsRes = this.exprResource(e.arg1_key);
		const whose = possessive(subject);
		if (op === 'size_of' || op === 'exist' || op === 'exist_not') {
			if (!rhsRes) return { text: this.notRendered(op) };
			const prop = rhsRes.type === 'var' ? null : this.predicate(rhsRes);
			if (op === 'size_of') return { text: `the number of ${prop ? `${whose} ${prop}` : `values ${subject} has for ${this.value(rhsRes)}`}` };
			if (op === 'exist') return { text: prop ? `${subject} has ${article(prop)} ${prop}` : `${subject} has a value for ${this.value(rhsRes)}` };
			return { text: prop ? `${subject} has no ${prop}` : `${subject} has no value for ${this.value(rhsRes)}` };
		}
		// The aggregates take a property or a configuration resource carrying
		// jets:entity_property and jets:value_property (jets/rete/expr_op_arithmetics.h).
		if (!rhsRes || (rhsRes.type !== 'resource' && rhsRes.type !== 'volatile_resource')) {
			return { text: this.notRendered(op) };
		}
		const cfg = this.config(rhsRes);
		const rhsName = this.predicate(rhsRes);
		// A configuration the engine refuses fails the session (the configuration
		// error, check_aggregate_config_properties and each operator's own check,
		// jets/rete/expr_op_arithmetics.h, since 2026-11-14), so it is not read as
		// if it computed. A right operand carrying neither property is no
		// configuration: it is the multi-valued property itself.
		const refused = this.refusedConfig(op, cfg, rhsName);
		if (refused) return { text: this.notRendered(op, refused) };
		const entity = cfg.entity ? this.predicate(cfg.entity) : null;
		const val = cfg.value ? this.predicate(cfg.value) : null;
		switch (op) {
			case 'min_of':
			case 'max_of': {
				const which = op === 'min_of' ? 'smallest' : 'largest';
				if (!entity) return { text: `the ${which} ${rhsName} of ${subject}` };
				if (!val) return { text: `the ${which} of ${whose} ${entity}` };
				return { text: `the ${which} ${val} across ${whose} ${entity}` };
			}
			case 'sum_values': {
				const data = val ?? rhsName;
				if (!entity) return { text: `the sum of ${whose} ${data}` };
				return { text: `the sum of ${data} across ${whose} ${entity}` };
			}
			case 'join_values': {
				const sep = cfg.separator === undefined ? quote(', ') : this.value(cfg.separator);
				if (entity) return { text: `the ${val} values across ${whose} ${entity} joined with ${sep}` };
				return { text: `${whose} ${val ?? rhsName} values joined with ${sep}` };
			}
			case 'sorted_head': {
				const opText = cfg.operator.value;
				return { text: `the one of ${whose} ${entity} with the ${opText === '<' ? 'smallest' : 'largest'} ${val}` };
			}
		}
		return null;
	}

	/**
	 * Why the engine refuses an aggregate's configuration, or null when it accepts
	 * it: jets:entity_property or jets:value_property that is not a resource (a
	 * literal, which until 2026-11-14 gave null silently); min_of and max_of with
	 * jets:value_property alone; sum_values and join_values with
	 * jets:entity_property alone; join_values with a jets:separator that is not
	 * text; and sorted_head without both properties and a jets:operator of "<" or
	 * ">". The accepted shapes are listed in the developer's guide, values and
	 * operators, aggregate configurations.
	 */
	refusedConfig(op, cfg, cfgName) {
		const isConfig = cfg.entity !== undefined || cfg.value !== undefined;
		const named = (v) => v.type === 'resource' || v.type === 'volatile_resource';
		const refused = (what) => `${cfgName} ${what}, a configuration the engine refuses`;
		if (op === 'sorted_head') {
			const opText = cfg.operator?.type === 'text' ? cfg.operator.value : null;
			if (!cfg.entity || !cfg.value) return refused('lacks jets:entity_property or jets:value_property');
			if (!named(cfg.entity) || !named(cfg.value)) return refused('gives a literal for a property');
			if (opText !== '<' && opText !== '>') return refused('has no jets:operator of "<" or ">"');
			return null;
		}
		if (!isConfig) return null;
		if ((cfg.entity && !named(cfg.entity)) || (cfg.value && !named(cfg.value))) {
			return refused('gives a literal for a property');
		}
		if ((op === 'min_of' || op === 'max_of') && !cfg.entity) {
			return refused('has jets:value_property and no jets:entity_property');
		}
		if ((op === 'sum_values' || op === 'join_values') && !cfg.value) {
			return refused('has jets:entity_property and no jets:value_property');
		}
		if (op === 'join_values' && cfg.separator !== undefined && cfg.separator.type !== 'text') {
			return refused('has a jets:separator that is not text');
		}
		return null;
	}

	/**
	 * substring_of: (config substring_of text). The configuration resource carries
	 * jets:from, the number of characters skipped, and jets:length, the number kept or,
	 * when negative, the number dropped from the end (SubstringOfVisitor,
	 * jets/rete/expr_op_strings.h). The two numbers are read from the configuration
	 * triples of the rule's main file, and only when each is a single int literal that
	 * no rule of that file writes; otherwise the reading names the configuration
	 * resource rather than guess its values.
	 */
	substring(e, depth) {
		const text = this.operand(this.expression(e.arg1_key, depth + 1));
		const cfgRes = this.exprResource(e.arg0_key);
		const configured = () => ({
			text: `the substring of ${text} configured by ${this.operand(this.expression(e.arg0_key, depth + 1))}`,
			isText: true,
		});
		if (!cfgRes || (cfgRes.type !== 'resource' && cfgRes.type !== 'volatile_resource')) return configured();
		const cfgId = cfgRes.id ?? cfgRes.value;
		const cfg = this.config(cfgRes);
		const number = (field) => {
			const v = cfg[field];
			if (!v || v.type !== 'int' || cfg.repeated.has(field)) return null;
			if (this.configWriteStmt.get(this.sourceFileKey, cfgId, `jets:${field}`)) return null;
			const n = Number(v.value);
			return Number.isInteger(n) ? n : null;
		};
		const from = number('from');
		const length = number('length');
		if (from === null || length === null || from < 0) return configured();
		const chars = (n) => (n === 1 ? 'character' : `${n} characters`);
		if (length === 0) return { text: 'an empty text', isText: true };
		if (length > 0) {
			if (from === 0) return { text: `the first ${chars(length)} of ${text}`, isText: true };
			return { text: `the ${chars(length)} of ${text} after its first ${chars(from)}`, isText: true };
		}
		if (from === 0) return { text: `${text} without its last ${chars(-length)}`, isText: true };
		return { text: `${text} without its first ${chars(from)} and its last ${chars(-length)}`, isText: true };
	}

	/**
	 * replace_char_of: (config replace_char_of text). The configuration resource carries
	 * jets:replace_chars, the characters to replace, and jets:replace_with, the text that
	 * replaces each of them, empty to remove them (ReplaceCharOfVisitor,
	 * jets/rete/expr_op_strings.h). The engine replaces every occurrence of each
	 * character in turn. The two texts are read from the configuration triples of the
	 * rule's main file, and only when each is a single text literal that no rule of that
	 * file writes; otherwise the reading names the configuration resource rather than
	 * guess its values. A character is a UTF-8 character, as the engine reads it since
	 * 2026-11-13 (split_utf8_chars), so one outside ASCII is quoted like any other. The
	 * reading names the configuration resource too where a plain reading would not say
	 * what the engine does: no characters to replace, a control character (C0, DEL or
	 * C1) not in CHAR_NAMES, or a replacement of more than one character holding a
	 * character replaced after the first, which a later turn would replace again.
	 */
	replaceChars(e, depth) {
		const text = this.operand(this.expression(e.arg1_key, depth + 1));
		const configured = () => ({
			text: `${text} with characters replaced as configured by ${this.operand(this.expression(e.arg0_key, depth + 1))}`,
			isText: true,
		});
		const cfgRes = this.exprResource(e.arg0_key);
		if (!cfgRes || (cfgRes.type !== 'resource' && cfgRes.type !== 'volatile_resource')) return configured();
		const cfgId = cfgRes.id ?? cfgRes.value;
		const cfg = this.config(cfgRes);
		const literal = (field) => {
			const v = cfg[field];
			if (!v || v.type !== 'text' || cfg.repeated.has(field)) return null;
			if (this.configWriteStmt.get(this.sourceFileKey, cfgId, `jets:${field}`)) return null;
			return String(v.value ?? '');
		};
		const chars = literal('replace_chars');
		const by = literal('replace_with');
		if (chars === null || by === null || chars === '') return configured();
		// C0 controls, DEL and C1 controls: a quote would show nothing, or garble them.
		const control = (c) => c < ' ' || (c >= '\x7f' && c <= '\x9f');
		if ([...chars].some((c) => control(c) && !CHAR_NAMES[c])) return configured();
		if ([...by].length > 1 && [...chars].slice(1).some((c) => by.includes(c))) return configured();
		if ([...by].some(control) && !CHAR_NAMES[by]) return configured();
		const names = [...new Set(chars)].map((c) => CHAR_NAMES[c] ?? quote(c));
		if (by === '') return { text: `${text} with every ${listOf(names)} removed`, isText: true };
		const replacement = CHAR_NAMES[by] ? `${article(CHAR_NAMES[by])} ${CHAR_NAMES[by]}` : quote(by);
		return { text: `${text} with every ${listOf(names)} replaced by ${replacement}`, isText: true };
	}

	/**
	 * The configuration triples of an operator's resource, in the rule's main file: the
	 * first value of each field, and in repeated the fields that have more than one.
	 */
	config(res) {
		const out = { repeated: new Set() };
		for (const r of this.configStmt.all(res.id ?? res.value, this.sourceFileKey)) {
			const field = CONFIG_FIELDS[r.predicate];
			if (out[field] === undefined) out[field] = { type: r.type, id: r.id, value: r.value };
			else out.repeated.add(field);
		}
		return out;
	}

	read() {
		const when = this.antecedents.map((t) => this.antecedentClause(t));
		const then = this.consequents.map((t) => this.consequentClause(t));
		const unrendered = [...this.unrendered].sort();
		return { when, then, partial: unrendered.length > 0, unrendered };
	}
}

/**
 * The antecedents in the order the network chains them: each one's parent_vertex is
 * the vertex of the one before it. Any not on the chain follow, by vertex.
 */
function chainOrder(antecedents) {
	const byParent = new Map();
	const vertices = new Set(antecedents.map((a) => a.vertex));
	for (const a of antecedents) {
		if (!byParent.has(a.parent_vertex)) byParent.set(a.parent_vertex, []);
		byParent.get(a.parent_vertex).push(a);
	}
	const ordered = [];
	const used = new Set();
	let current = antecedents.filter((a) => !vertices.has(a.parent_vertex)).sort((a, b) => a.vertex - b.vertex)[0];
	while (current && !used.has(current.key)) {
		ordered.push(current);
		used.add(current.key);
		current = (byParent.get(current.vertex) ?? []).filter((a) => !used.has(a.key))[0];
	}
	for (const a of antecedents) if (!used.has(a.key)) ordered.push(a);
	return ordered;
}

/**
 * The English reading of one rule: { when, then, partial, unrendered }.
 * @param {import('node:sqlite').DatabaseSync} db a workspace.db, opened read-only
 * @param {number} ruleKey a jet_rules.key
 */
export function readRule(db, ruleKey) {
	return new RuleReader(db, ruleKey).read();
}
