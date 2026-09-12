package datatable

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/artisoft-io/jetstore/jets/agentic/briefing"
	"github.com/artisoft-io/jetstore/jets/sqlscript"
	"github.com/artisoft-io/jetstore/jets/userflow"
	"github.com/artisoft-io/jetstore/jets/wsvalidate"
)

// The per-suffix validator table the save path dispatches through.
//
// **A table rather than a chain of ifs, agreed with the agentic_ai stream before
// either side wrote it** — recorded as Q-3 in the ui_refresh project's tracking.
// `.uf.json` was the second structured file type and `.pc.json` is a known
// third; two ifs would have been tolerable and three is where it should have
// been a table.
//
// Adding a row is the whole of adding a file type, and `.tc.json` (task I.3,
// 2026-08-23) is the fourth — added by editing this table and nothing else,
// which is the claim the sentence above was making and this is the test of it.
//
// The `.pc.json` row is agentic_ai's; it is deliberately not stubbed here,
// because a row pointing at nothing is worse than no row. **Their gap 6 closed
// on 2026-08-18, so that row is overdue rather than scheduled** — this comment
// said "when their gap 6 activates" for five days after it had (their I-81).
// A note naming a future trigger goes silent when the trigger passes.
//
// `.pv.json` is the fifth (agentic_ai's `AK.2`, 2026-09-05): a briefing
// provenance schema, the document that says what each field of a generated
// briefing is allowed to assert. It is a row and nothing else, again — the
// validator is `briefing.ValidateProvenanceDocument` and it existed before this
// row did, because `ParseSchema` was written to return `wsvalidate.Finding`
// for exactly this reason. **This is the second extension by a party that did
// not write the table**, after ui_refresh's `.tc.json`, and the first by the
// stream the table's comment was addressed to.
//
// `.sql` is the sixth (2026-09-12) and **it is the first row that is not a row
// and nothing else**, which is worth saying because five predecessors had made
// "adding a row is the whole of adding a file type" a measured claim. It is the
// first non-JSON file type, and two things in this file were written around
// JSON in ways the table itself did not record:
//
//   - `checkWorkspaceFile` returned early for any name not ending `.json`, so a
//     `.sql` row would have been dispatched to by `validatorFor` and never
//     reached. The gate is now the JSON *precondition* it always was, applied
//     to the names that need it, instead of a filter on the whole check.
//   - `wsvalidate.Finding` carried a JSON Pointer and no line, and a lexical
//     finding in a SQL file has a line and no pointer. `Finding.Line` was added
//     with this row.
//
// So the claim survives with its scope named: adding a row is the whole of
// adding a *JSON* file type. The first one from outside that set cost two
// changes elsewhere, both of them assumptions nobody had had reason to write
// down.
//
// **Nothing else in the workspace tree ends `.sql`, and that took a deletion to
// become true** — see the `jet_rules` row in `wsfile/sections.go`, which
// filtered on a second `.sql` suffix this row would have matched.
var workspaceFileValidators = []struct {
	suffix   string
	validate wsvalidate.Validator
}{
	{".uf.json", userflow.ValidateFlowDocument},
	{".ua.json", userflow.ValidateActionDocument},
	{".form.json", userflow.ValidateFormDocument},
	{".tc.json", userflow.ValidateTableDocument},
	{briefing.DocumentSuffix, briefing.ValidateProvenanceDocument},
	{".sql", sqlscript.ValidateScript},
}

// validatorFor returns the most specific match, or nil.
//
// **"Most specific" is not decoration.** The JSON well-formedness check is
// `HasSuffix(ToUpper(fileName), ".JSON")`, so `.uf.json` already matches it and
// so will `.pc.json`; a naive dispatch would either double-validate or shadow.
// The JSON suffixes are mutually exclusive, so longest-match still costs
// nothing — it is the rule that keeps a later file type honest. It has still
// never decided anything: the one case that nearly did was settled by deleting
// a file type rather than by adding a row (`wsfile/sections.go`, `jet_rules`).
//
// Matching is case-insensitive, like the JSON check beside it. A workspace file
// named `Foo.UF.JSON` is the same file type as `foo.uf.json`, and the file
// system it lives on may or may not agree.
func validatorFor(fileName string) wsvalidate.Validator {
	upper := strings.ToUpper(fileName)
	var best wsvalidate.Validator
	bestLen := 0
	for _, entry := range workspaceFileValidators {
		if strings.HasSuffix(upper, strings.ToUpper(entry.suffix)) && len(entry.suffix) > bestLen {
			best, bestLen = entry.validate, len(entry.suffix)
		}
	}
	return best
}

// ValidateFlowDocumentForTest exposes the dispatched validator to this package's
// tests without re-importing userflow at every call site.
func ValidateFlowDocumentForTest(content string) []wsvalidate.Finding {
	return userflow.ValidateFlowDocument(content)
}

// checkWorkspaceFile is the whole of what the save path does before writing.
//
// **Extracted so the wiring is testable, not merely the validators.** Mutation
// testing found that bypassing the dispatch inside SaveWorkspaceFileContent
// broke no test: the handler needs a database and a token, so nothing covered
// it, and the validators were being tested in isolation from the thing that
// calls them. This is the seam that fixes that.
//
// Returns nil when the file may be written.
func checkWorkspaceFile(fileName, content string) error {
	// The JSON well-formedness check is a precondition of the JSON validators
	// and applies to every `.json` file, row or no row — a malformed one is
	// refused whatever its suffix. **It used to be the gate on the whole
	// function**, which meant a validator for any other file type would have
	// been dispatched to by `validatorFor` and never called; the `.sql` row
	// found that on 2026-09-12. Non-JSON validators are handed raw text and own
	// their own well-formedness, which `wsvalidate.Validator` now says.
	if strings.HasSuffix(strings.ToUpper(fileName), ".JSON") {
		var m map[string]any
		if err := json.Unmarshal([]byte(content), &m); err != nil {
			return fmt.Errorf("the file is not a valid json file: %v", err)
		}
	}
	validate := validatorFor(fileName)
	if validate == nil {
		return nil
	}
	if errs := wsvalidate.ErrorsOnly(validate(content)); len(errs) > 0 {
		return describeFindings(fileName, errs)
	}
	return nil
}

// describeFindings renders the errors for the http response.
//
// Every finding carries its coordinate where it has one, because the editor
// showing this message is the one that could put a cursor on the offence — and
// because the agentic_ai stream's repair prompts need *where* rather than only
// *what*. A JSON Pointer for a document, a line for a file whose positions are
// lines; a finding carrying both would print `/a/b:12`, which no validator
// produces today and which reads correctly if one ever does.
func describeFindings(fileName string, findings []wsvalidate.Finding) error {
	var b strings.Builder
	fmt.Fprintf(&b, "%s cannot be saved:", fileName)
	for _, f := range findings {
		where := f.Path
		switch {
		case f.Line > 0 && where != "":
			where = fmt.Sprintf("%s:%d", where, f.Line)
		case f.Line > 0:
			where = fmt.Sprintf("line %d", f.Line)
		}
		if where == "" {
			fmt.Fprintf(&b, "\n  %s", f.Message)
		} else {
			fmt.Fprintf(&b, "\n  %s: %s", where, f.Message)
		}
	}
	return fmt.Errorf("%s", b.String())
}
