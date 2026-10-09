package template

import (
	"encoding/json"
	"fmt"
	"strings"
)

// Template is a compiled document. It is immutable after [Compile] and safe for
// concurrent use: nothing in it is written during a render, which is what lets
// one compiled template serve every worker node of a pipeline.
type Template struct {
	key   string
	width int
	root  *element
}

// Key is the name the document was registered under.
func (t *Template) Key() string { return t.key }

// Width is the column each paragraph is wrapped to.
func (t *Template) Width() int { return t.width }

// element is a compiled paragraph or group.
type element struct {
	when  predicate
	group bool
	text  []span
	kids  []*element
	empty []span
}

// Compile reads a template document and returns a template that cannot fail.
//
// **It is the only function in this package that returns an error**, and that
// is R-108's mitigation rather than a house rule: [Template.Render] has no
// error to return, so a later maintainer cannot introduce a render-time failure
// without changing a signature and every call site. Criterion 84 is enforced by
// the compiler.
//
// The twelve failures it admits are §10.8's C1 to C12. Each names the construct
// and its position, on `compileInferPromptTemplate`'s precedent
// (`compileInferPromptTemplate`, `jets/compute_pipes/pipe_transformation_infer.go:529`):
// the reader of the error is the author of the template.
func Compile(spec *Spec) (*Template, error) {
	if spec == nil {
		return nil, fmt.Errorf("a text template document is required")
	}
	if strings.TrimSpace(spec.Key) == "" {
		return nil, fmt.Errorf("a text template carries no key; it is what a step names it by")
	}
	if spec.Width <= 0 {
		return nil, fmt.Errorf("text template %q: width is absent or is not a positive integer; "+
			"it is the column each paragraph is wrapped to and is required rather than defaulted", spec.Key)
	}
	root, err := compileGroup(spec.Elements, "", spec.Empty, position{path: "(document)", off: -1})
	if err != nil {
		return nil, fmt.Errorf("text template %q: %v", spec.Key, err)
	}
	return &Template{key: spec.Key, width: spec.Width, root: root}, nil
}

// CompileJSON reads a template document from json and compiles it.
//
// It exists for a caller that holds the document as bytes - a test, a workspace
// file, a configuration element decoded on its own. A caller that decodes the
// document into a [Spec] as part of a larger structure gets C1 anyway, because
// the strictness is on the type.
func CompileJSON(data []byte) (*Template, error) {
	var spec Spec
	if err := json.Unmarshal(data, &spec); err != nil {
		return nil, err
	}
	return Compile(&spec)
}

// compileGroup compiles an element list and its fallback. **A document is a
// group**, so the fallback is one recursive concept rather than a
// document-level special case - which is what lets a template put a notice and
// a rule above a body that may have nothing to say, and still have the
// fallback fire (§10.6).
func compileGroup(elements []*Element, when, empty string, p position) (*element, error) {
	if len(elements) == 0 {
		return nil, compileErr(p, "carries no elements; a group with nothing in it emits nothing "+
			"and is a document that was not finished")
	}
	out := &element{group: true}
	if strings.TrimSpace(when) != "" {
		pred, err := parsePredicate(when, p)
		if err != nil {
			return nil, err
		}
		out.when = pred
	}
	for i, el := range elements {
		child, err := compileElement(el, position{path: childPath(p.path, i), off: -1})
		if err != nil {
			return nil, err
		}
		out.kids = append(out.kids, child)
	}
	if strings.TrimSpace(empty) != "" {
		spans, err := compileMarkup(empty, position{path: p.path + ".empty", off: -1})
		if err != nil {
			return nil, err
		}
		out.empty = spans
	}
	return out, nil
}

func childPath(parent string, i int) string {
	if parent == "(document)" {
		return fmt.Sprintf("elements[%d]", i)
	}
	return fmt.Sprintf("%s.elements[%d]", parent, i)
}

// compileElement is C2: an element is a paragraph or a group and the fields
// present say which.
func compileElement(el *Element, p position) (*element, error) {
	if el == nil {
		return nil, compileErr(p, "is null")
	}
	hasText := strings.TrimSpace(el.Text) != ""
	hasKids := el.Elements != nil
	switch {
	case hasText && hasKids:
		return nil, compileErr(p, "carries both text and elements; an element is a paragraph or a group")
	case !hasText && !hasKids:
		return nil, compileErr(p, "carries neither text nor elements; an element is a paragraph or a group")
	case hasKids:
		return compileGroup(el.Elements, el.When, el.Empty, p)
	}
	if strings.TrimSpace(el.Empty) != "" {
		return nil, compileErr(p, "is a paragraph and carries an empty; a fallback is a group's, "+
			"because it is what a group says when none of its children said anything")
	}
	out := &element{}
	if strings.TrimSpace(el.When) != "" {
		pred, err := parsePredicate(el.When, p)
		if err != nil {
			return nil, err
		}
		out.when = pred
	}
	spans, err := compileMarkup(el.Text, position{path: p.path + ".text", off: -1})
	if err != nil {
		return nil, err
	}
	out.text = spans
	return out, nil
}
