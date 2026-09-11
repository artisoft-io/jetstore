package template

import (
	"bytes"
	"encoding/json"
	"fmt"
)

// Spec is a template document (§10.2).
//
// **The body is a structured document rather than one string**, and that is the
// notation's one consequential choice. Order is a quarter of what this phase
// configures, and in a single string order is position in text: moving an
// element means moving a block of characters and hoping the whitespace comes
// with it. Conditional emission is a property of an element rather than of a
// span, and a paragraph that emits nothing has to take its blank line with it -
// which in a single string is a rule about what a blank line means. So the
// split is **structure above the paragraph, text inside it**: order, grouping
// and the fallback are data, and the sentence is markup.
//
// `Key` is `PromptTemplateSpec`'s move repeated (F1129): a named document at
// the configuration root, referenced by a step, so that a template can be
// shared and a name that is not there fails at build time.
type Spec struct {
	// Key names the document within the configuration's `text_templates`.
	Key string `json:"key"`
	// Width is the column each paragraph is wrapped to. **Required rather than
	// defaulted**: the shape of the output depends on it and a silent default
	// is a byte difference nobody is looking for.
	Width int `json:"width"`
	// Elements are emitted in array order, each as its own paragraph.
	Elements []*Element `json:"elements"`
	// Empty is the markup emitted when no element emitted anything. Optional;
	// absent means a document with nothing to say renders as nothing.
	Empty string `json:"empty,omitempty"`
}

// Element is a paragraph or a group, and **nothing distinguishes them but which
// field is present** - `text` for a paragraph, `elements` for a group. An
// object carrying both or neither is a compile failure. Whether the cpipes
// contract wants an explicit discriminator instead is Q-106, settled where the
// matrix rows are written rather than here.
type Element struct {
	// When gates the element. Absent means always.
	When string `json:"when,omitempty"`
	// Text is a paragraph's markup.
	Text string `json:"text,omitempty"`
	// Elements is a group's children.
	Elements []*Element `json:"elements,omitempty"`
	// Empty is a group's fallback, emitted as one paragraph when no child
	// emitted. A paragraph carrying one is a compile failure.
	Empty string `json:"empty,omitempty"`
}

// UnmarshalJSON decodes strictly, which is C1.
//
// **The strictness rides on the type rather than on the caller**, so that a
// document decoded as part of a pipeline configuration gets it too: `wen`
// written for `when` is a template that silently never fires, and it is the
// failure this repository's editorial stance would call the worst kind - one
// that looks like a decision.
func (s *Spec) UnmarshalJSON(data []byte) error {
	type alias Spec
	var a alias
	if err := decodeStrict(data, &a); err != nil {
		return fmt.Errorf("while reading a text template: %v", err)
	}
	*s = Spec(a)
	return nil
}

// UnmarshalJSON decodes strictly, which is C1 one level down.
func (e *Element) UnmarshalJSON(data []byte) error {
	type alias Element
	var a alias
	if err := decodeStrict(data, &a); err != nil {
		return fmt.Errorf("while reading a template element: %v", err)
	}
	*e = Element(a)
	return nil
}

func decodeStrict(data []byte, into any) error {
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.DisallowUnknownFields()
	return dec.Decode(into)
}
