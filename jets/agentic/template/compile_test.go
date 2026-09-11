package template

import (
	"strings"
	"testing"
)

// The twelve compile failures of plan §10.8, one subtest each.
//
// **Every one of them is demonstrated by a failing *compile* rather than by a
// render**, which is the whole of criterion 84: if any of these could be made
// to fail at render time instead, the criterion would be false and this file is
// where that would show.
//
// The fixtures are deliberately neutral - a catalogue of orders and items -
// because criterion 81 is that nothing under this package names a domain, and a
// test fixture is under this package.
func TestCompileRefusals(t *testing.T) {
	cases := []struct {
		name string
		code string // the C-number of §10.8
		spec *Spec
		json string // used instead of spec when set, for the failures that are decoding ones
		want string
	}{
		{
			name: "C1 an unknown key on a paragraph",
			code: "C1",
			json: `{"key":"k","width":40,"elements":[{"wen":"a == 'b'","text":"x"}]}`,
			want: "unknown field",
		},
		{
			name: "C1 an unknown key on the document",
			code: "C1",
			json: `{"key":"k","width":40,"elements":[{"text":"x"}],"wrap":78}`,
			want: "unknown field",
		},
		{
			name: "C2 no key",
			code: "C2",
			spec: &Spec{Width: 40, Elements: []*Element{{Text: "x"}}},
			want: "carries no key",
		},
		{
			name: "C2 no elements",
			code: "C2",
			spec: &Spec{Key: "k", Width: 40},
			want: "carries no elements",
		},
		{
			name: "C2 an empty element list",
			code: "C2",
			spec: &Spec{Key: "k", Width: 40, Elements: []*Element{}},
			want: "carries no elements",
		},
		{
			name: "C2 both text and elements",
			code: "C2",
			spec: &Spec{Key: "k", Width: 40, Elements: []*Element{
				{Text: "x", Elements: []*Element{{Text: "y"}}}}},
			want: "carries both text and elements",
		},
		{
			name: "C2 neither text nor elements",
			code: "C2",
			spec: &Spec{Key: "k", Width: 40, Elements: []*Element{{When: "a == 'b'"}}},
			want: "carries neither text nor elements",
		},
		{
			name: "C2 a paragraph carrying an empty",
			code: "C2",
			spec: &Spec{Key: "k", Width: 40, Elements: []*Element{{Text: "x", Empty: "nothing"}}},
			want: "is a paragraph and carries an empty",
		},
		{
			name: "C3 width absent",
			code: "C3",
			spec: &Spec{Key: "k", Elements: []*Element{{Text: "x"}}},
			want: "width is absent or is not a positive integer",
		},
		{
			name: "C3 width not positive",
			code: "C3",
			spec: &Spec{Key: "k", Width: -1, Elements: []*Element{{Text: "x"}}},
			want: "width is absent or is not a positive integer",
		},
		{
			name: "C3 width not an integer",
			code: "C3",
			json: `{"key":"k","width":"78","elements":[{"text":"x"}]}`,
			want: "cannot unmarshal",
		},
		{
			name: "C4 an opening with no close",
			code: "C4",
			spec: paragraph("Orders: {{Order_Count"),
			want: "is never closed",
		},
		{
			name: "C4 a close with no opening",
			code: "C4",
			spec: paragraph("Orders: Order_Count}}"),
			want: "closes nothing",
		},
		{
			name: "C5 an if with no end",
			code: "C5",
			spec: paragraph("{{if Order_Count > 0}}some"),
			want: "{{if}} is never closed",
		},
		{
			name: "C5 an each with no end",
			code: "C5",
			spec: paragraph("{{each has_Items[] sep: ', ' last: ' and '}}{{Item_Name}}"),
			want: "{{each}} is never closed",
		},
		{
			name: "C5 an end closing nothing",
			code: "C5",
			spec: paragraph("some{{end}}"),
			want: "{{end}} closes nothing",
		},
		{
			name: "C6 an unknown construct",
			code: "C6",
			spec: paragraph("{{Order_Count|wordz}}"),
			want: `"wordz" is not a construct`,
		},
		{
			name: "C6 an unknown construct after a path",
			code: "C6",
			spec: paragraph("{{Tag_Summary[]|titlecase|count}}"),
			want: "the constructs are count, d_MMMM, d_MMMM_yyyy, join, latest_by, max, plural, sentence, strip_code, words",
		},
		{
			name: "C7 too few arguments",
			code: "C7",
			spec: paragraph("{{Tag_Summary[]|join: ', '}}"),
			want: `"join" takes 2 argument(s) and is given 1`,
		},
		{
			name: "C7 an argument given to a construct that takes none",
			code: "C7",
			spec: paragraph("{{Tag_Summary[]|count: 'x'}}"),
			want: `"count" takes 0 argument(s) and is given 1`,
		},
		{
			name: "C8 a path where a literal belongs",
			code: "C8",
			spec: paragraph("{{Order_Count|plural: order}}"),
			want: "takes a quoted literal as argument 1",
		},
		{
			name: "C8 a literal where a path belongs",
			code: "C8",
			spec: paragraph("{{has_Items[]|latest_by: 'Shipment_Date'}}"),
			want: "takes a path as argument 1",
		},
		{
			name: "C9 a fold on a single value",
			code: "C9",
			spec: paragraph("{{Shipment_Date|max|d_MMMM}}"),
			want: `"max" is a fold and is applied to a single value`,
		},
		{
			name: "C9 a map on a single value",
			code: "C9",
			spec: paragraph("{{Tag_Summary[]|join: ', ', ' and '|strip_code}}"),
			want: `"strip_code" is a map and is applied to a single value`,
		},
		{
			name: "C9 a date consumer after a text producer",
			code: "C9",
			spec: paragraph("{{Tag_Summary[]|join: ', ', ' and '|d_MMMM}}"),
			want: `"d_MMMM" takes a date and is applied to text`,
		},
		{
			name: "C9 a number consumer after a text producer",
			code: "C9",
			spec: paragraph("{{Tag_Summary[]|join: ', ', ' and '|words}}"),
			want: `"words" takes a number and is applied to text`,
		},
		{
			name: "C9 with over a chain that is not nodes",
			code: "C9",
			spec: paragraph("{{with Tag_Summary[]|count}}x{{end}}"),
			want: "is scoped to a number, which is not a node",
		},
		{
			name: "C9 each over a chain that is not nodes",
			code: "C9",
			spec: paragraph("{{each Tag_Summary[]|join: ', ', ' and ' sep: '; ' last: '; and '}}x{{end}}"),
			want: "is scoped to text, which is not a node",
		},
		{
			name: "C9 a path addressed into a construct's text",
			code: "C9",
			spec: paragraph("{{Order_Count|words|Item_Name}}"),
			want: "addresses into text, which is not a node",
		},
		{
			name: "C10 a numeric segment",
			code: "C10",
			spec: paragraph("{{has_Items.0.Item_Name}}"),
			want: "is an array index",
		},
		{
			name: "C10 an empty segment",
			code: "C10",
			spec: paragraph("{{has_Items..Item_Name}}"),
			want: "has an empty segment",
		},
		{
			name: "C10 a stray bracket",
			code: "C10",
			spec: paragraph("{{has_Items[0].Item_Name}}"),
			want: "is malformed",
		},
		{
			name: "C11 no operator",
			code: "C11",
			spec: gated("Order_Count", "x"),
			want: "carries no operator",
		},
		{
			name: "C11 an unknown operator",
			code: "C11",
			spec: gated("Order_Count = 3", "x"),
			want: `"=" is not an operator`,
		},
		{
			name: "C11 an unquoted literal",
			code: "C11",
			spec: gated("Status == open", "x"),
			want: "needs a quoted literal on its right",
		},
		{
			name: "C11 a quoted literal on a numeric operator",
			code: "C11",
			spec: gated("Order_Count > '3'", "x"),
			want: "needs a bare number on its right",
		},
		{
			name: "C11 an unbalanced parenthesis",
			code: "C11",
			spec: gated("(Order_Count > 0 or Item_Count > 0", "x"),
			want: "unbalanced '('",
		},
		{
			name: "C11 a connective with nothing after it",
			code: "C11",
			spec: gated("Order_Count > 0 and", "x"),
			want: "ends after 'and' or 'or'",
		},
		{
			name: "C12 require with no message",
			code: "C12",
			spec: paragraph("{{Item_Name require}}"),
			want: "require carries no message",
		},
		{
			name: "C12 require with an unquoted message",
			code: "C12",
			spec: paragraph("{{Item_Name require missing}}"),
			want: "require carries no message",
		},
		{
			name: "C12 require on a block form",
			code: "C12",
			spec: paragraph("{{if Item_Name != 'x' require 'no'}}y{{end}}"),
			want: "require is a modifier on a substitution",
		},
	}
	for _, tc := range cases {
		t.Run(tc.code+" "+tc.name, func(t *testing.T) {
			var err error
			if tc.json != "" {
				_, err = CompileJSON([]byte(tc.json))
			} else {
				_, err = Compile(tc.spec)
			}
			if err == nil {
				t.Fatalf("%s: compiled and should not have", tc.code)
			}
			if !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("%s: error is %q, which does not carry %q", tc.code, err, tc.want)
			}
		})
	}
}

// TestACompileFailureNamesItsPosition is the second half of the task's "naming
// the construct and its position": the message has to place the failure in a
// document a reader is looking at, which for a nested element is not a line
// number in a file.
func TestACompileFailureNamesItsPosition(t *testing.T) {
	spec := &Spec{Key: "catalogue", Width: 40, Elements: []*Element{
		{Text: "fine"},
		{Elements: []*Element{
			{Text: "also fine"},
			{Text: "and then {{Tag_Summary[]|wordz|count}}"},
		}},
	}}
	_, err := Compile(spec)
	if err == nil {
		t.Fatal("compiled and should not have")
	}
	for _, want := range []string{`"catalogue"`, "elements[1].elements[1].text", "offset 9"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("error %q does not carry %q", err, want)
		}
	}
}

// TestPathsAreNeverRefusedForTheirKind is C9's boundary, and it is the half
// that decides whether the check over-refuses (R-114). A chain of paths alone
// says nothing about what it addressed, so every construct must accept it.
func TestPathsAreNeverRefusedForTheirKind(t *testing.T) {
	for _, markup := range []string{
		"{{Order_Count|words}}",
		"{{Order_Count|sentence}}",
		"{{Shipment_Date|d_MMMM}}",
		"{{Shipment_Date|d_MMMM_yyyy}}",
		"{{Tag_Summary[]|count}}",
		"{{Tag_Summary[]|max|d_MMMM}}",
		"{{Tag_Summary[]|strip_code|join: ', ', ' and '}}",
		"{{with has_Items[]|latest_by: Shipment_Date}}{{Item_Name}}{{end}}",
		"{{each has_Items[] sep: '; ' last: '; and '}}{{Item_Name}}{{end}}",
		"{{with has_Items[]|latest_by: Shipment_Date}}{{Shipment_Date|d_MMMM}}{{end}}",
	} {
		if _, err := Compile(paragraph(markup)); err != nil {
			t.Errorf("%s refused: %v", markup, err)
		}
	}
}

// TestAConstructNameIsReachableAsAProperty is I-642's escape hatch, asserted
// rather than described: a document whose property is spelled like a construct
// is addressed by writing it as a path that no construct could be.
func TestAConstructNameIsReachableAsAProperty(t *testing.T) {
	tmpl := mustCompile(t, paragraph("{{count[]|join: ', ', ' and '}}"))
	got, _ := tmpl.Render(map[string]any{"count": []any{"a", "b"}})
	if got != "a and b" {
		t.Fatalf("got %q, want %q", got, "a and b")
	}
}

func paragraph(markup string) *Spec {
	return &Spec{Key: "k", Width: 78, Elements: []*Element{{Text: markup}}}
}

func gated(when, markup string) *Spec {
	return &Spec{Key: "k", Width: 78, Elements: []*Element{{When: when, Text: markup}}}
}

func mustCompile(t *testing.T, spec *Spec) *Template {
	t.Helper()
	tmpl, err := Compile(spec)
	if err != nil {
		t.Fatalf("compile: %v", err)
	}
	return tmpl
}
