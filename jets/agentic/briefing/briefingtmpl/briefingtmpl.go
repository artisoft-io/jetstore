// Package briefingtmpl holds the `patient_profile` briefing as a template
// document, and holds it once.
//
// It is agentic_ai's Phase 8 `AY.3`. What it carries is **configuration, not
// code**: `patient_profile_briefing.json` is a [template.Spec] in the notation
// of plan §10 of `projects/agentic_ai/plan/phase8_plan.md`, and everything that
// makes it a *briefing* rather than a language lives in that file.
// `jets/agentic/template` is the engine and names none of it, which is
// criterion 81; this package names all of it and contains no logic, which is
// the other half of the same line.
//
// # Why the document is a file in a package rather than a fixture
//
// It has two consumers and they are in different repositories. `AY.4` compiles
// it and renders it against `jets/agentic/briefing/prose`'s seventeen tests;
// `BA.2` inlines it into `patient_profile_template.pc.json`'s `text_templates`
// array, because **Q-104 keeps the body inline in the pipeline configuration**
// by the user's instruction. So there will be two copies of these bytes and the
// question is not how to avoid the second one - it is what keeps them equal.
//
// **Three things keep them equal, and only the third is a mechanism.**
//
// The file is the one hand-edited copy, and it says so at the top of its own
// `comment`. A `testdata/` directory would have said the opposite: Go's own
// convention is that `testdata` is test input, and this is a shipped
// configuration document that a test happens to read.
//
// It is embedded rather than read from disk, so a Go consumer that wants the
// document gets it from here rather than from a path it has to spell. `AY.5`,
// `AZ` and a future validator are all such consumers.
//
// **And [TestTheShippedPipelineCarriesThisDocument] compares this file with the
// workspace's**, when it is pointed at one. It is skipped by default, on
// `JETS_PC_CORPUS_DIR`'s established pattern
// (`error_channel_default_corpus_test.go:37`), because the workspaces are
// submodules and a test that fails when a submodule is absent is a test people
// delete. **A skipped guard is weaker than a running one and it is stronger
// than a sentence**, and the sentence is what this section would otherwise be:
// the failure it catches - an edit to one copy and not the other - is silent,
// produces a working pipeline, and changes what a member is told.
//
// # What this package is not
//
// It is **not** where the `render` operator's caller behaviour is decided.
// [Render] is the reference caller of §10.7 - the four lines that stay outside
// the engine - and it exists so that criterion 82 has something to compare with
// `prose.Render`. Whether the operator fails the row into its error channel or
// writes the column and reports separately is **Q-107**, settled at `AZ.3`.
package briefingtmpl

import (
	_ "embed"
	"fmt"
	"strings"
	"sync"

	"github.com/artisoft-io/jetstore/jets/agentic/template"
)

// Document is the template document, verbatim.
//
// **`BA.2` inlines these bytes** into the `text_templates` array of
// `patient_profile_template.pc.json`. Read them from here rather than retyping
// them; see the package comment for what compares the two afterwards.
//
//go:embed patient_profile_briefing.json
var Document []byte

// Key is the name the document registers under, and it is what a step's
// `text_template_name` has to say. It is duplicated from the document rather
// than parsed out of it at package initialisation, because a package variable
// that can be wrong at compile time is better than one that can only be wrong
// at run time - and `TestTheDocumentCompiles` is what makes that true.
const Key = "patient_profile_briefing"

var (
	once     sync.Once
	compiled *template.Template
	compErr  error
)

// Compiled compiles the document once and returns the same template thereafter.
//
// **A compiled template is immutable and safe for concurrent use**, which is
// what lets one of them serve every worker node of a pipeline
// (`Template`, `jets/agentic/template/compile.go:11`). The error can only be a
// defect in the file beside this one: it is a constant of the build, so a
// caller that ignores it is wrong on every record rather than on some.
func Compiled() (*template.Template, error) {
	once.Do(func() { compiled, compErr = template.CompileJSON(Document) })
	return compiled, compErr
}

// Render writes the briefing for one projected `cintel:Briefing` entity, with
// the signature `prose.Render` has.
//
// **It is the reference caller rather than the engine**, and the three things
// it does are exactly the three §10.7 says do not belong inside one.
//
// It refuses a prefixed entity. That is the one of `prose.Render`'s three
// refusals that stays with the caller (**I-623**): it is a claim about the
// column's `remove_model_prefixes` setting rather than about the data, it is
// the same for every template, and no arrangement of a template makes it
// decidable at build time. Without it a misconfigured column renders an empty
// briefing that looks like a member with no claims.
//
// It maps a non-empty violation list to an error. **A [template.Violation] is
// data because the distinction Phase 7 drew is precisely this one** - a failure
// of the *renderer* disqualifies a control, and a record missing a property its
// template declares required is the data failing - so the engine reports and
// the caller decides what it costs. This caller decides it costs the artefact,
// which is what `prose.Render` does and is what lets `AY.4` compare the two.
//
// And it compiles. A caller rendering more than one record should hold the
// [Compiled] template instead; this one is written for a test and for a reader.
func Render(entity map[string]any) (string, error) {
	if entity == nil {
		return "", fmt.Errorf("the briefing entity is nil")
	}
	if prefixed := prefixedKey(entity); prefixed != "" {
		return "", fmt.Errorf(
			"the briefing entity carries the prefixed property %q; the prose encoding renders the "+
				"property names the projection asserts and needs remove_model_prefixes on its column_encodings entry",
			prefixed)
	}
	tmpl, err := Compiled()
	if err != nil {
		return "", err
	}
	out, violations := tmpl.Render(entity)
	if len(violations) > 0 {
		msgs := make([]string, 0, len(violations))
		for _, v := range violations {
			msgs = append(msgs, v.String())
		}
		return "", fmt.Errorf("the briefing entity does not carry what its template asserts: %s",
			strings.Join(msgs, "; "))
	}
	return out, nil
}

// prefixedKey reports a root property that still carries a model prefix.
//
// It looks at the root only, for `prefixedKey`'s own reason: `extractAsEntity`
// passes `remove_model_prefixes` down its own recursion, so a prefixed root
// means the whole map is prefixed and one look is the whole check
// (`prefixedKey`, `jets/agentic/briefing/prose/entity.go:136`). **This is a
// fourth copy of four lines and it is not I-643's kind of duplication**: the
// three-way copy I-643 records is of `scalar`, `asInt` and `dateLayouts`, which
// decide what a *value* means and diverge silently. This one is a `strings.Contains`
// over a map key.
func prefixedKey(entity map[string]any) string {
	for k := range entity {
		if strings.Contains(k, ":") {
			return k
		}
	}
	return ""
}
