// Package template renders a decoded document as text from a configured
// template, and is the engine behind the `render` operator.
//
// It is agentic_ai's Phase 8 `AY.2`, and plan §10 of
// `projects/agentic_ai/plan/phase8_plan.md` is its specification rather than
// its documentation: the notation was written as a plan section, argued over,
// and accepted before this package existed. Read §10 before changing anything
// here. Where this file argues a point §10 does not settle, it says so and
// names the choice.
//
// # Nothing here knows what it is rendering
//
// The package names no property of any entity, no domain and no pipeline. That
// is criterion 81 and it is the whole of what this package is for: everything
// `jets/agentic/briefing/prose/` knows about a briefing becomes a document, and
// what is left is a language. A grep for a property name in this directory
// finding nothing is the criterion.
//
// # Every failure this package admits is a compile failure
//
// Phase 7's `prose` package refuses a template engine in terms, and its
// argument is sound: "a mistyped filter name, a filter applied to the wrong
// arity, an unclosed `each`" are failures of the renderer rather than of the
// data, and an arm that can fail for reasons of its own is not a control
// (`prose.go:17`). Phase 8's subject is a product operator with no experiment
// to keep clean, so the premise is gone - but the property it protected is not,
// and it is restated as criterion 84.
//
// The mechanism is `compileInferPromptTemplate`'s
// (`compileInferPromptTemplate`, `jets/compute_pipes/pipe_transformation_infer.go:529`):
// a template is a constant of the pipeline configuration, so every question
// about how it is spelled is answerable before a record arrives, and the answer
// fails the *build*. Twelve such failures are enumerated in §10.8 and every one
// is implemented here.
//
// **The design that enforces it is the signature, not a convention.** Compile
// is the only function in this package that returns an error and [Template.Render]
// returns none at all, so a later maintainer cannot add a render-time failure
// without changing a signature and every call site. That is R-108's mitigation,
// and it is why a violated `require` is a [Violation] rather than an error: the
// distinction is the one Phase 7 drew, between the renderer failing and the data
// failing, and a record missing a property its template declares required is
// the data.
//
// # The path syntax is imported rather than re-implemented
//
// Paths are [briefing.Selector], unchanged. The rule a second traversal would
// get wrong is that a `[]` segment over a scalar yields the scalar
// (`Resolve`, `jets/agentic/briefing/selector.go:104`), because
// `addToEntityObj` promotes a property to a slice only when a second value
// arrives - so a multi-valued property holding one value is a bare scalar, and
// a traversal insisting on an array is wrong about exactly the record a
// guardrail is least entitled to be wrong about. `ParseSelector`'s three
// refusals then become compile failures for free, because a path in a template
// document is a string constant.
package template
