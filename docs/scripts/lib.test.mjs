// Tests for lib.mjs: run with `npm test` in docs/, or `node --test scripts/`.
import assert from 'node:assert/strict';
import { test } from 'node:test';
import { prose } from './lib.mjs';

test('prose escapes a block marker at the start of the text', () => {
	// Each of these, as a paragraph, a list item or a heading's content, would
	// start a heading, a list, a quote or a rule. Escaped, it reads as written.
	for (const [text, escaped] of [
		['# Not a heading', '\\# Not a heading'],
		['## Nor this', '\\## Nor this'],
		['- not a list', '\\- not a list'],
		['* not a list', '\\* not a list'],
		['+ not a list', '\\+ not a list'],
		['1. not a list', '1\\. not a list'],
		['12) not a list', '12\\) not a list'],
		['> not a quote', '&gt; not a quote'],
		['---', '\\---'],
		['- - -', '\\- - -'],
		['***', '\\*\\*\\*'],
		['___', '\\_\\_\\_'],
	]) {
		assert.equal(prose(text), escaped, text);
	}
});

test('prose drops leading spaces, so that four cannot start a code block', () => {
	assert.equal(prose('    # indented'), '\\# indented');
	assert.equal(prose(' \n- after a line break'), '\\- after a line break');
});

test('prose leaves a marker that is not at the start, and text that starts otherwise', () => {
	assert.equal(prose('A - b # c 1. d'), 'A - b # c 1. d');
	assert.equal(prose('2026 was'), '2026 was');
	assert.equal(prose('`# code` first'), '`# code` first');
	assert.equal(prose('a line\n# then a heading'), 'a line # then a heading');
});
