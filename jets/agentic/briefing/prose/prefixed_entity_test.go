package prose_test

import (
	"strings"
	"testing"

	"github.com/artisoft-io/jetstore/jets/agentic/briefing/briefingtmpl"
	"github.com/artisoft-io/jetstore/jets/agentic/template"
)

// **I-623 and I-693 settled, in the package that holds the refusal.**
//
// `prose.Render` refuses an entity whose root keys still carry a model prefix
// (`prefixedKey`, `jets/agentic/briefing/prose/entity.go:135`): that is
// `remove_model_prefixes: false` on the `column_encodings` entry, and without
// the refusal the briefing renders as *"No claims activity on record for this
// member"* for a member who has claims. `I-623` asked where that check goes when
// the configured engine replaces this renderer and proposed the operator.
// `I-693` answered that the operator must not have it: `ParseSelector` accepts a
// colon (`ParseSelector`, `jets/agentic/briefing/selector.go:57`), so a template
// written against an unstripped entity is legal and an unconditional refusal
// would refuse a working configuration.
//
// **`AY.5`'s answer is that the refusal belongs to the caller and already has
// two homes, and that what replaces it in the operator is a property of the
// document rather than of the engine.** Both halves are asserted below rather
// than argued, because `I-693`'s replacement - a `require` on the first
// substitution - had never been run against a prefixed entity, and running it
// found that the answer is two decisions rather than one: the `require` fires,
// **and the engine renders the empty briefing anyway**, so what keeps it from a
// reader is the operator failing the row (`Q-107`, answered at `AZ.3`) rather
// than the violation existing.
//
// # What the two tests divide
//
// [TestTheShippedDocumentReportsAPrefixedEntityRatherThanRefusingIt] is the
// claim `I-693` makes, for the document the operator will actually run. Its
// name is *reports* rather than *refuses* for the reason above. It goes
// through [template.Template.Render] rather than through `briefingtmpl.Render`
// on purpose: the caller's refusal is what is *not* there in the operator, and a
// test that kept it would be testing the wrong path.
//
// [TestADocumentWithNoRequireRendersThePrefixedEntityAsEmpty] is the residual,
// and it is the reason this pair is a demonstration rather than a fix. A
// template that asserts nothing about its own input renders a prefixed entity as
// an affirmative false statement, with no violation and no error, and **nothing
// in the engine or the operator can catch that without refusing a legal
// configuration**. What is available is the author's own `require`, which is why
// the shipped document carries one and why `I-732` records the general case as
// guidance owed rather than as a check owed.

// prefixedEntity is the section-1106 fixture as a column with
// `remove_model_prefixes: false` delivers it: the same map with `cintel:` still
// on every root key. Only the root is prefixed because `extractAsEntity` passes
// the flag down its own recursion, so a prefixed root means a prefixed map.
func prefixedEntity() map[string]any {
	return map[string]any{
		"cintel:Briefing_Disclaimer":  "Informational only. This is not medical advice.",
		"cintel:Medical_Event_Count":  2,
		"cintel:Pharmacy_Event_Count": 0,
		"cintel:has_Briefing_Medical_Events": []any{
			map[string]any{"cintel:Care_Setting": "Independent Laboratory"},
			map[string]any{"cintel:Care_Setting": "Office"},
		},
	}
}

func TestTheShippedDocumentReportsAPrefixedEntityRatherThanRefusingIt(t *testing.T) {
	tmpl, err := briefingtmpl.Compiled()
	if err != nil {
		t.Fatalf("compile: %v", err)
	}
	out, violations := tmpl.Render(prefixedEntity())
	if len(violations) == 0 {
		t.Fatalf("a prefixed entity produced no violation, so the operator would write this column "+
			"and a member with two visits would be told there are none:\n%s", out)
	}
	// The diagnosis is weaker than `prose.Render`'s - it names the missing
	// property rather than the column setting that lost it - and it is not
	// silent, which is the property that mattered. Said here so that the
	// difference is a recorded trade rather than a surprise in an error log.
	joined := violations[0].String()
	if !strings.Contains(joined, "Briefing_Disclaimer") {
		t.Errorf("the first violation does not name the property that is missing, which is the whole of "+
			"what a reader gets in place of the refusal: %q", joined)
	}
	// **And the engine renders the false artefact beside the violation**, which
	// is the half of `I-693`'s answer the entry does not say. `prose.Render`
	// returns `("", err)`: the sentence never exists. The engine returns the
	// sentence *and* a violation, and the only thing between that string and a
	// member is `AZ.3`'s answer to `Q-107` - the operator fails the row rather
	// than writing the column and reporting beside it. Asserted rather than
	// noted, because it means the replacement for the refusal is two decisions
	// in two packages and not one `require`.
	if !strings.Contains(out, "No claims activity") {
		t.Errorf("the rendered text is no longer the empty case, so this test is no longer watching what "+
			"a prefixed entity would say if the operator wrote the column:\n%s", out)
	}
}

// TestADocumentWithNoRequireRendersThePrefixedEntityAsEmpty is the failure the
// refusal exists to prevent, reproduced under the configured engine.
//
// The document is the shape of the shipped one with its assertions removed: a
// group with an `empty` fallback and a gate on a count. Nothing about it is
// malformed - it compiles, it renders, it returns no violation - and what it
// says about a member with two visits is that there are none.
func TestADocumentWithNoRequireRendersThePrefixedEntityAsEmpty(t *testing.T) {
	const doc = `{
	  "key": "no_assertions",
	  "width": 78,
	  "elements": [
	    {
	      "empty": "No claims activity on record for this member.",
	      "elements": [
	        {"when": "Medical_Event_Count > 0",
	         "text": "{{Medical_Event_Count|words|sentence}} medical {{Medical_Event_Count|plural: 'visit'}} on record."}
	      ]
	    }
	  ]
	}`
	tmpl, err := template.CompileJSON([]byte(doc))
	if err != nil {
		t.Fatalf("compile: %v", err)
	}

	if out, violations := tmpl.Render(map[string]any{"Medical_Event_Count": 2}); len(violations) > 0 ||
		!strings.Contains(out, "Two medical visits") {
		t.Fatalf("the control case does not render, so the case below proves nothing: %q %v", out, violations)
	}

	out, violations := tmpl.Render(prefixedEntity())
	if len(violations) > 0 {
		t.Fatalf("this document asserts nothing, so a violation here means the engine has gained a check "+
			"and I-732 should be reopened rather than this test relaxed: %v", violations)
	}
	if !strings.Contains(out, "No claims activity") {
		t.Fatalf("expected the empty fallback, got %q", out)
	}
	// Asserted rather than commented: what this document says about a member
	// with two visits is that there are none, with no error anywhere, and a
	// template author's own `require` is the only thing that would have said
	// otherwise.
	if strings.Contains(out, "Two") {
		t.Error("the prefixed entity resolved after all, which would mean ParseSelector no longer accepts a colon")
	}
}
