package template

import (
	"fmt"
	"strings"
)

// Violation is a `require` that a record did not satisfy.
//
// **It is data rather than an error, and that is the design rather than a
// dodge.** The distinction Phase 7 drew is precisely this one: a failure of the
// *renderer* is what disqualifies a control, and a record missing a property
// its template declares required is the *data* failing. The renderer still
// renders; it reports what was not there. What that buys is that the diagnosis
// costs the render nothing and [Template.Render] keeps a signature with no
// error in it, which is what makes criterion 84 the compiler's to enforce.
//
// **The caller decides what a violation costs.** A caller reproducing
// `prose.Render` maps a non-empty list to an error; whether the `render`
// operator fails the row into its error channel or writes the column and
// reports separately is Q-107, settled where the operator's tests are written.
type Violation struct {
	// Message is the template's own words, verbatim. It is the whole of what a
	// reader has to act on, which is why an empty one is a compile failure.
	Message string
	// Chain is the chain that yielded nothing, as it was written.
	Chain string
	// Where is the element and offset the `require` sits at.
	Where string
	// Index is the 0-based position within the innermost enclosing `each`, or
	// -1 outside one. **It is the one piece of context the message cannot
	// carry**, because the message is a constant of the template and the
	// iteration is not: "a pharmacy event carries no Drug_Name" is true of a
	// record and says nothing about which event.
	Index int
}

func (v Violation) String() string {
	if v.Index >= 0 {
		return fmt.Sprintf("%s (%s, item %d, at %s)", v.Message, v.Chain, v.Index, v.Where)
	}
	return fmt.Sprintf("%s (%s, at %s)", v.Message, v.Chain, v.Where)
}

// renderState carries what a render collects. The `each` index is a stack in
// effect - saved and restored around an iteration - so a nested `each` reports
// the innermost position rather than the outermost.
type renderState struct {
	violations []Violation
	index      int
}

// Render writes the document for one decoded record.
//
// **It returns no error, and the absence is the point.** Every construct is
// total, a path that is not there resolves to nothing rather than failing, and
// every question about how the template is spelled was answered by [Compile].
// What is left is a record that does not carry what the template asserts, and
// that is the []Violation.
//
// A nil or empty document renders the document's fallback if it has one, and
// the empty string otherwise. That is not a special case: an absent path
// resolves to nothing whether the document is empty or merely missing the
// property, and a renderer that distinguished them would be answering a
// question about the encoder.
func (t *Template) Render(doc map[string]any) (string, []Violation) {
	r := &renderState{index: -1}
	paragraphs := t.root.render(doc, r)
	wrapped := make([]string, 0, len(paragraphs))
	for _, p := range paragraphs {
		wrapped = append(wrapped, wrap(p, t.width))
	}
	return strings.Join(wrapped, "\n\n"), r.violations
}

// render emits an element's paragraphs (§10.6).
//
// A paragraph emits when its `when` holds and its rendered text is not blank,
// which is `renderBody`'s rule as data. A group emits its children in array
// order, each as its own paragraph; if none of them emitted and it carries a
// fallback, the fallback is that group's one paragraph.
//
// **A group whose `when` does not hold emits nothing at all, its fallback
// included.** The alternative reading - that the fallback stands in for a
// suppressed group - would make `when` mean two things depending on whether an
// `empty` is present.
//
// **Paragraphs are joined with one blank line and the separator is not
// configurable.** It is what the artefact this notation generalises does, a
// configurable one buys nothing, and it is one more thing to get wrong.
func (e *element) render(ctx any, r *renderState) []string {
	if e.when != nil && !e.when.eval(ctx) {
		return nil
	}
	if !e.group {
		s := renderSpans(e.text, ctx, r)
		if strings.TrimSpace(s) == "" {
			return nil
		}
		return []string{s}
	}
	var out []string
	for _, kid := range e.kids {
		out = append(out, kid.render(ctx, r)...)
	}
	if len(out) > 0 || e.empty == nil {
		return out
	}
	s := renderSpans(e.empty, ctx, r)
	if strings.TrimSpace(s) == "" {
		return nil
	}
	return []string{s}
}

func renderSpans(spans []span, ctx any, r *renderState) string {
	var b strings.Builder
	for _, s := range spans {
		b.WriteString(s.render(ctx, r))
	}
	return b.String()
}

func (s literalSpan) render(any, *renderState) string { return s.text }

// render writes the first value the chain yields, and reports a violation when
// the substitution asserts one and there is nothing to write.
//
// **The test is on the rendered text rather than on the resolution**, which is
// the stricter of the two readings of §10.4's "yields nothing" and is the one
// that reproduces what it generalises: both refusals of `prose.Render` compare
// the trimmed text with the empty string, so a property present and blank is
// already a failure there (`Render`, `jets/agentic/briefing/prose/prose.go:148`).
// A property that is there and says nothing is not a property that is there.
func (s substSpan) render(ctx any, r *renderState) string {
	out := s.ch.text(ctx)
	if s.require != "" && strings.TrimSpace(out) == "" {
		r.violations = append(r.violations, Violation{
			Message: s.require,
			Chain:   s.ch.raw,
			Where:   s.pos.String(),
			Index:   r.index,
		})
	}
	return out
}

func (s ifSpan) render(ctx any, r *renderState) string {
	if !s.pred.eval(ctx) {
		return ""
	}
	return renderSpans(s.body, ctx, r)
}

// render scopes a span to the node the chain yields, and emits nothing when it
// yields none. **That guard is the whole argument for the form** (F1148): the
// alternative is writing the chain out once per substitution and guarding each
// occurrence by hand, which is what the notation's source sketch did and is
// what it got wrong.
func (s withSpan) render(ctx any, r *renderState) string {
	v, ok := s.ch.first(ctx)
	if !ok {
		return ""
	}
	node, ok := asNode(v)
	if !ok {
		return ""
	}
	return renderSpans(s.body, node, r)
}

// render emits the span once per node and joins the results.
//
// **It joins with `join`**, so a span that renders blank is skipped rather than
// leaving a separator with nothing on one side of it. That is I-624's
// refinement applied consistently: the same rule in the construct and in the
// block form, rather than two answers to one question.
func (s eachSpan) render(ctx any, r *renderState) string {
	items := make([]string, 0, 4)
	saved := r.index
	for i, v := range s.ch.eval(ctx) {
		node, ok := asNode(v)
		if !ok {
			continue
		}
		r.index = i
		if out := renderSpans(s.body, node, r); strings.TrimSpace(out) != "" {
			items = append(items, out)
		}
	}
	r.index = saved
	return join(items, s.sep, s.last)
}
