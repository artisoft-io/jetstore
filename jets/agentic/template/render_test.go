package template

import (
	"strings"
	"testing"
	"time"
)

// renderOne compiles one paragraph of markup and renders it, asserting that
// nothing was violated. It is the shape most of the construct assertions take.
func renderOne(t *testing.T, markup string, doc map[string]any) string {
	t.Helper()
	tmpl := mustCompile(t, &Spec{Key: "k", Width: 200, Elements: []*Element{{Text: markup}}})
	out, v := tmpl.Render(doc)
	if len(v) != 0 {
		t.Fatalf("violations: %v", v)
	}
	return out
}

// TestTheTenConstructs walks §10.5's table, each construct on a value it takes
// and on a value it does not.
//
// **The second half is the one worth having.** Every construct is total, and a
// test that only ever feeds it what it wants measures nothing about that: the
// claim criterion 84 rests on is what happens to a chain whose runtime value is
// the wrong kind, and the answer is the empty string rather than a failure.
func TestTheTenConstructs(t *testing.T) {
	cases := []struct {
		markup string
		doc    map[string]any
		want   string
	}{
		{"{{n|words}}", map[string]any{"n": 0}, "zero"},
		{"{{n|words}}", map[string]any{"n": 21}, "twenty-one"},
		{"{{n|words}}", map[string]any{"n": 112}, "one hundred and twelve"},
		{"{{n|words}}", map[string]any{"n": 1000}, "1000"},
		{"{{n|words}}", map[string]any{"n": 2.0}, "two"},
		{"{{n|words}}", map[string]any{"n": "not a number"}, "not a number"},
		{"{{n|words}}", map[string]any{}, ""},

		{"{{s|sentence}}", map[string]any{"s": "hello there"}, "Hello there"},
		{"{{s|sentence}}", map[string]any{"s": ""}, ""},

		{"{{n|plural: 'order'}}", map[string]any{"n": 1}, "order"},
		{"{{n|plural: 'order'}}", map[string]any{"n": 2}, "orders"},
		{"{{n|plural: 'order'}}", map[string]any{"n": "junk"}, "orders"},

		{"{{d|d_MMMM}}", map[string]any{"d": "2025-08-14"}, "14 August"},
		{"{{d|d_MMMM}}", map[string]any{"d": "not a date"}, ""},
		{"{{d|d_MMMM_yyyy}}", map[string]any{"d": "2025-08-14"}, "14 August 2025"},
		{"{{d|d_MMMM_yyyy}}", map[string]any{"d": time.Date(2025, 8, 14, 0, 0, 0, 0, time.UTC)}, "14 August 2025"},

		{"{{with xs[]|latest_by: at}}{{label}}{{end}}", map[string]any{"xs": []any{
			map[string]any{"at": "2025-01-01", "label": "first"},
			map[string]any{"at": "2025-06-01", "label": "last"},
		}}, "last"},
		{"{{with xs[]|latest_by: at}}{{label}}{{end}}", map[string]any{"xs": []any{
			map[string]any{"label": "no date"},
		}}, ""},
		{"{{with xs[]|latest_by: at}}{{label}}{{end}}", map[string]any{"xs": []any{
			map[string]any{"at": "2025-06-01", "label": "first of a tie"},
			map[string]any{"at": "2025-06-01", "label": "second of a tie"},
		}}, "first of a tie"},

		{"{{xs[]|strip_code|join: ', ', ' and '}}", map[string]any{
			"xs": []any{"(A1) Alpha", "Beta", "(C3) Gamma"}}, "Alpha, Beta and Gamma"},
		{"{{xs[]|strip_code|join: ', ', ' and '}}", map[string]any{"xs": []any{"(A1) Alpha"}}, "Alpha"},
		{"{{xs[]|strip_code|join: ', ', ' and '}}", map[string]any{}, ""},
		// I-624: the skip is `join`'s here, so an empty value does not leave a
		// separator with nothing on one side of it.
		{"{{xs[]|join: ', ', ' and '}}", map[string]any{"xs": []any{"a", "", "  ", "b"}}, "a and b"},
		// F1144: a multi-valued property holding one value is a bare scalar.
		{"{{xs[]|join: ', ', ' and '}}", map[string]any{"xs": "alone"}, "alone"},

		{"{{xs[]|count}}", map[string]any{"xs": []any{"a", "b", "c"}}, "3"},
		{"{{xs[]|count}}", map[string]any{}, "0"},
		{"{{xs[]|count}}", map[string]any{"xs": "alone"}, "1"},

		{"{{xs[]|max|d_MMMM}}", map[string]any{
			"xs": []any{"2025-07-02", "2025-08-24", "not a date"}}, "24 August"},
		{"{{xs[]|max|d_MMMM}}", map[string]any{"xs": []any{"not a date"}}, ""},
	}
	for _, tc := range cases {
		got := renderOne(t, tc.markup, tc.doc)
		if got != tc.want {
			t.Errorf("%s over %v = %q, want %q", tc.markup, tc.doc, got, tc.want)
		}
	}
}

// TestPredicates covers §10.4's two operator families, the connectives and the
// parentheses - including the grouping that is the reason parentheses are in
// the grammar at all.
func TestPredicates(t *testing.T) {
	doc := map[string]any{"a": 0, "b": 3, "s": "open", "blank": "", "ratio": 0.75}
	cases := []struct {
		pred string
		want bool
	}{
		{"b > 0", true},
		{"a > 0", false},
		{"missing > 0", false},
		{"s > 0", false}, // not a number, and so false rather than an error
		{"b >= 3", true},
		{"b < 3", false},
		{"b <= 3", true},
		{"ratio >= 0.75", true},
		{"s == 'open'", true},
		{"s == 'Open'", false}, // case-sensitive, which is I-622's subject
		{"s != ''", true},
		{"blank != ''", false},
		{"missing != ''", false},
		{"a > 0 or b > 0", true},
		{"a > 0 and b > 0", false},
		{"a > 0 or b > 0 and s == 'shut'", false},
		{"(a > 0 or b > 0) and s == 'open'", true},
		{"(a > 0 or b > 0) and s == 'shut'", false},
	}
	for _, tc := range cases {
		tmpl := mustCompile(t, gated(tc.pred, "yes"))
		got, _ := tmpl.Render(doc)
		if (got == "yes") != tc.want {
			t.Errorf("%s over the fixture emitted %q, want emitted=%v", tc.pred, got, tc.want)
		}
	}
}

// TestElementOrderAndConditionalEmission is §10.6: order is array order, a
// paragraph emits when its `when` holds and its text is not blank, and
// paragraphs are joined with one blank line.
func TestElementOrderAndConditionalEmission(t *testing.T) {
	spec := &Spec{Key: "k", Width: 200, Elements: []*Element{
		{Text: "first"},
		{When: "n > 0", Text: "second"},
		{Text: "{{missing}}"}, // renders blank, so emits nothing
		{Text: "third"},
	}}
	tmpl := mustCompile(t, spec)
	got, _ := tmpl.Render(map[string]any{"n": 1})
	if got != "first\n\nsecond\n\nthird" {
		t.Fatalf("got %q", got)
	}
	got, _ = tmpl.Render(map[string]any{"n": 0})
	if got != "first\n\nthird" {
		t.Fatalf("with the gate closed, got %q", got)
	}
}

// TestTheEmptyCaseIsAGroupsFallback is the structural half of §10.6, and the
// reason a document is a group rather than a flat list: a preamble that emits
// regardless must not suppress a fallback for the body.
func TestTheEmptyCaseIsAGroupsFallback(t *testing.T) {
	spec := &Spec{Key: "k", Width: 200, Elements: []*Element{
		{Text: "notice"},
		{Text: "--------"},
		{Empty: "Nothing on record.", Elements: []*Element{
			{When: "n > 0", Text: "{{n|words}} on record."},
			{When: "tags[]|count > 0", Text: "Tags: {{tags[]|join: ', ', ' and '}}."},
		}},
	}}
	tmpl := mustCompile(t, spec)

	got, _ := tmpl.Render(map[string]any{"n": 2})
	if got != "notice\n\n--------\n\ntwo on record." {
		t.Fatalf("populated: got %q", got)
	}
	got, _ = tmpl.Render(map[string]any{})
	if got != "notice\n\n--------\n\nNothing on record." {
		t.Fatalf("empty: got %q", got)
	}

	// A flat list could not express that: a document-level fallback under an
	// unconditional notice would never fire.
	flat := mustCompile(t, &Spec{Key: "k", Width: 200, Empty: "Nothing on record.", Elements: []*Element{
		{Text: "notice"},
		{When: "n > 0", Text: "{{n|words}} on record."},
	}})
	got, _ = flat.Render(map[string]any{})
	if got != "notice" {
		t.Fatalf("flat: got %q, and the fallback should not have fired", got)
	}
}

// TestADocumentIsAGroup asserts the other half: a document with no preamble
// falls back exactly as a group does.
func TestADocumentIsAGroup(t *testing.T) {
	tmpl := mustCompile(t, &Spec{Key: "k", Width: 200, Empty: "Nothing on record.",
		Elements: []*Element{{When: "n > 0", Text: "something"}}})
	got, _ := tmpl.Render(map[string]any{})
	if got != "Nothing on record." {
		t.Fatalf("got %q", got)
	}
	got, _ = tmpl.Render(map[string]any{"n": 1})
	if got != "something" {
		t.Fatalf("got %q", got)
	}
}

// TestAGroupWhoseGateIsShutEmitsNothing pins the reading §10.6 leaves open: a
// closed `when` suppresses the fallback too, so that `when` does not mean two
// things depending on whether an `empty` is present.
func TestAGroupWhoseGateIsShutEmitsNothing(t *testing.T) {
	tmpl := mustCompile(t, &Spec{Key: "k", Width: 200, Elements: []*Element{
		{Text: "notice"},
		{When: "show == 'yes'", Empty: "Nothing on record.", Elements: []*Element{
			{When: "n > 0", Text: "something"},
		}},
	}})
	got, _ := tmpl.Render(map[string]any{})
	if got != "notice" {
		t.Fatalf("got %q", got)
	}
	got, _ = tmpl.Render(map[string]any{"show": "yes"})
	if got != "notice\n\nNothing on record." {
		t.Fatalf("got %q", got)
	}
}

// TestBlockForms covers `if`, `with` and `each`, including the two things that
// make `with` worth its place (F1148): the chain is written once, and a chain
// yielding nothing takes its whole span with it.
func TestBlockForms(t *testing.T) {
	doc := map[string]any{
		"xs": []any{
			map[string]any{"name": "widget", "at": "2025-01-02"},
			map[string]any{"name": "gadget", "at": "2025-03-04"},
			map[string]any{"name": "sprocket"},
		},
		"flag": "Y",
	}
	cases := []struct {
		markup string
		want   string
	}{
		{"{{if flag == 'Y'}}on{{end}}", "on"},
		{"{{if flag == 'N'}}on{{end}}", ""},
		{"a{{if missing == 'Y'}} and b{{end}}.", "a."},
		{"{{with xs[]|latest_by: at}}latest is {{name}}{{end}}", "latest is gadget"},
		{"{{with missing[]|latest_by: at}}latest is {{name}}{{end}}", ""},
		{"{{each xs[] sep: ', ' last: ' and '}}{{name}}{{end}}", "widget, gadget and sprocket"},
		{"{{each xs[] sep: ', ' last: ' and '}}{{name}}{{if at|d_MMMM != ''}} ({{at|d_MMMM}}){{end}}{{end}}",
			"widget (2 January), gadget (4 March) and sprocket"},
		{"{{each missing[] sep: ', ' last: ' and '}}{{name}}{{end}}", ""},
		// A span that renders blank is skipped rather than leaving a separator
		// with nothing on one side of it.
		{"{{each xs[] sep: ', ' last: ' and '}}{{at|d_MMMM}}{{end}}", "2 January and 4 March"},
	}
	for _, tc := range cases {
		if got := renderOne(t, tc.markup, doc); got != tc.want {
			t.Errorf("%s = %q, want %q", tc.markup, got, tc.want)
		}
	}
}

// TestNestedBlocksScopeTheContext asserts that `with` and `each` rebind the
// context and that a group does not - which is what makes a chain inside an
// element independent of how deeply the element is nested.
func TestNestedBlocksScopeTheContext(t *testing.T) {
	doc := map[string]any{
		"name": "root",
		"xs":   []any{map[string]any{"name": "child", "ys": []any{map[string]any{"name": "grandchild"}}}},
	}
	got := renderOne(t,
		"{{name}}{{each xs[] sep: '' last: ''}}/{{name}}{{each ys[] sep: '' last: ''}}/{{name}}{{end}}{{end}}", doc)
	if got != "root/child/grandchild" {
		t.Fatalf("got %q", got)
	}
	nested := mustCompile(t, &Spec{Key: "k", Width: 200, Elements: []*Element{
		{Elements: []*Element{{Elements: []*Element{{Text: "{{name}}"}}}}},
	}})
	out, _ := nested.Render(doc)
	if out != "root" {
		t.Fatalf("a group rebound the context: got %q", out)
	}
}

// TestRequireReportsAndStillRenders is §10.7's design: a violated assertion is
// data, the render completes, and the caller decides what it costs.
func TestRequireReportsAndStillRenders(t *testing.T) {
	tmpl := mustCompile(t, &Spec{Key: "k", Width: 200, Elements: []*Element{
		{Text: "{{notice require 'the record carries no notice'}}"},
		{Text: "Items: {{each xs[] sep: '; ' last: '; and '}}" +
			"{{name require 'an item carries no name'}}{{if flag == 'Y'}}, flagged{{end}}{{end}}."},
	}})
	out, violations := tmpl.Render(map[string]any{
		"notice": "  ", // present and blank, which is not present
		"xs": []any{
			map[string]any{"name": "widget"},
			map[string]any{"flag": "Y"},
		},
	})
	if out != "Items: widget; and , flagged." {
		t.Fatalf("the render did not complete: got %q", out)
	}
	if len(violations) != 2 {
		t.Fatalf("got %d violations, want 2: %v", len(violations), violations)
	}
	if violations[0].Message != "the record carries no notice" || violations[0].Index != -1 {
		t.Errorf("first violation is %+v", violations[0])
	}
	if violations[1].Message != "an item carries no name" || violations[1].Index != 1 {
		t.Errorf("second violation is %+v", violations[1])
	}
	if !strings.Contains(violations[1].String(), "item 1") {
		t.Errorf("the violation does not say which item: %s", violations[1])
	}
	// And a record that carries everything reports nothing.
	_, violations = tmpl.Render(map[string]any{
		"notice": "a notice",
		"xs":     []any{map[string]any{"name": "widget"}},
	})
	if len(violations) != 0 {
		t.Fatalf("got %v, want none", violations)
	}
}

// TestRenderIsTotal is criterion 84 from the other side: whatever the record
// is, the render returns. There is no error to check because there is no error
// in the signature, so what this asserts is that nothing panics and that the
// output is the specified empty form.
func TestRenderIsTotal(t *testing.T) {
	tmpl := mustCompile(t, &Spec{Key: "k", Width: 40, Empty: "Nothing on record.", Elements: []*Element{
		{When: "n > 0", Text: "{{n|words}} {{n|plural: 'thing'}}, {{d|d_MMMM}}"},
		{Text: "{{each xs[] sep: ', ' last: ' and '}}{{name}}{{end}}"},
		{Text: "{{with xs[]|latest_by: at}}{{name}}{{end}}"},
	}})
	for _, doc := range []map[string]any{
		nil,
		{},
		{"n": "junk", "d": 17, "xs": "a scalar where a list was meant"},
		{"n": 3, "d": "2025-02-30", "xs": []any{"a scalar where a node was meant"}},
		{"n": -1, "xs": []any{map[string]any{"name": nil}}},
		{"xs": map[string]any{"name": "a node where a list was meant"}},
	} {
		out, violations := tmpl.Render(doc)
		if len(violations) != 0 {
			t.Errorf("over %v: unexpected violations %v", doc, violations)
		}
		_ = out
	}
	out, _ := tmpl.Render(nil)
	if out != "Nothing on record." {
		t.Fatalf("a nil record rendered %q", out)
	}
}

// TestTheWrapIsPerParagraph is §10.7's third piece: `width` applies to each
// paragraph after it is rendered, and whitespace inside a paragraph collapses -
// so a template author decides whether there is a space at a markup boundary
// and never how much.
func TestTheWrapIsPerParagraph(t *testing.T) {
	tmpl := mustCompile(t, &Spec{Key: "k", Width: 20, Elements: []*Element{
		{Text: "one two three four five six seven eight nine ten"},
		{Text: "short"},
	}})
	got, _ := tmpl.Render(map[string]any{})
	want := "one two three four\nfive six seven eight\nnine ten\n\nshort"
	if got != want {
		t.Fatalf("got %q, want %q", got, want)
	}
	for _, line := range strings.Split(got, "\n") {
		if len([]rune(line)) > 20 {
			t.Errorf("line %q is longer than the width", line)
		}
	}

	// The collapse is unconditional - it is `strings.Fields`, not a width
	// decision - so a line break inside a template's text is a space and two
	// spaces are one. **The one thing a boundary still decides is whether
	// there is a space there at all**, which is what a template author has to
	// get right and what a document laid out for reading gets wrong.
	tmpl = mustCompile(t, &Spec{Key: "k", Width: 78, Elements: []*Element{{Text: "a  b\tc\nd   {{x}}  e"}}})
	got, _ = tmpl.Render(map[string]any{"x": "f"})
	if got != "a b c d f e" {
		t.Fatalf("the wrap did not collapse whitespace: %q", got)
	}
}
