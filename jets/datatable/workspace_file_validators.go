package datatable

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"path"
	"strings"

	"github.com/artisoft-io/jetstore/jets/agentic/briefing"
	"github.com/artisoft-io/jetstore/jets/compute_pipes"
	"github.com/artisoft-io/jetstore/jets/datatable/wsfile"
	"github.com/artisoft-io/jetstore/jets/optstats"
	"github.com/artisoft-io/jetstore/jets/runbook"
	"github.com/artisoft-io/jetstore/jets/sqlscript"
	"github.com/artisoft-io/jetstore/jets/userflow"
	"github.com/artisoft-io/jetstore/jets/wsvalidate"
)

// The per-suffix validator table the save path dispatches through.
//
// **A table rather than a chain of ifs, agreed with the agentic_ai stream before
// either side wrote it** — recorded as Q-3 in the ui_refresh project's tracking.
// `.uf.json` was the second structured file type and `.pc.json` was a known
// third; two ifs would have been tolerable and three is where it should have
// been a table.
//
// Adding a row is the whole of adding a file type, and `.tc.json` (task I.3,
// 2026-08-23) is the fourth — added by editing this table and nothing else,
// which is the claim the sentence above was making and this is the test of it.
//
// The `.pc.json` row was reserved here for the step-by-step startup validator
// in jets/agentic/tools (validate_cpipes_config), and this paragraph said for
// months that the row was overdue. **It is the twelfth row (2026-11-14), and it
// is not that validator**: `compute_pipes.ValidateConfigDocument` refuses only
// what certainly stops the pipeline at its start, whatever the run's
// environment, and warns of the rest. The agentic validator resolves each step
// in a fixed environment and flags configurations that run (4 of 51 in four
// workspaces), so it could only warn, and it would warn falsely on those four;
// ValidateConfigDocument's comment says exactly why. A note naming a future
// trigger goes silent when the trigger passes, which is what this one did.
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
//
// `.rb.json` is the eighth (runbooks Phase 1, `AA.4`, 2026-11-06): a workspace
// runbook, `runbooks/<name>.rb.json`. The save check has the file alone, so it
// checks everything but what needs the file's name or its SQL files, which
// the executor checks with both when a run starts (runbook.Load). A runbook's
// `.sql` files are saved through the `.sql` row above, lexically; their
// `:name` bindings are checked against the runbook at run start, because a
// SQL file does not know which runbook reads it.
//
// `workspace_docs.json` is the ninth (2026-11-13): the workspace guide's
// settings, listed with the guide's pages by the IDE's `docs` section. This
// said the pages themselves had no row, their front matter and links being
// checked by the documentation build, which needs the whole site to check a
// link.
//
// **The pages have two rows since later the same day, `.md` and `.mdx`, and
// they are the first rows scoped to a path.** A page whose front matter the
// build refuses fails the next UI image build, and front matter, unlike a link,
// can be checked alone (validateDocsPage, workspace_docs_page.go). A `.md`
// anywhere but `docs/` is read-only (wsfile.ReadOnlySuffixes) and refused
// before validation, so a row matching every `.md` would have been harmless
// there; it is scoped anyway, by `applies`, so that the table says which files
// a row checks and TestNoReadOnlyFileTypeHasAValidator can keep asserting that
// no read-only file has a validator.
var workspaceFileValidators = []struct {
	suffix string
	// applies, when set, narrows the row to the files it returns true for,
	// given the workspace-relative path; unset, the row applies to every file
	// with its suffix.
	applies  func(fileName string) bool
	validate wsvalidate.Validator
}{
	{".uf.json", nil, userflow.ValidateFlowDocument},
	{".ua.json", nil, userflow.ValidateActionDocument},
	{".form.json", nil, userflow.ValidateFormDocument},
	{".tc.json", nil, userflow.ValidateTableDocument},
	{briefing.DocumentSuffix, nil, briefing.ValidateProvenanceDocument},
	{".sql", nil, sqlscript.ValidateScript},
	{path.Base(optstats.FileName), nil, validateOptimizerStatistics},
	{runbook.FileSuffix, nil, runbook.ValidateFile},
	{workspaceDocsSettingsFile, nil, validateWorkspaceDocsSettings},
	{".md", isDocsPage, validateDocsPage},
	{".mdx", isDocsPage, validateDocsPage},
	{".pc.json", nil, compute_pipes.ValidateConfigDocument},
}

// workspaceDocsSettingsFile is the workspace guide's settings file,
// `docs/workspace_docs.json`, as the documentation build names it
// (SETTINGS_FILE, docs/scripts/workspace_docs.mjs).
const workspaceDocsSettingsFile = "workspace_docs.json"

// validateWorkspaceDocsSettings refuses a guide settings file the
// documentation build would refuse (readSettings, docs/scripts/workspace_docs.mjs):
// one that is not a JSON object, or whose `rule_text` is not a boolean. The
// ninth row, added on 2026-11-13 with the IDE's `docs` section
// (wsfile/sections.go), for the same reason as the seventh: listing the file
// made it editable, and a file the build refuses fails the next UI image build.
// The JSON check before it lets `null` through, which is not an object.
func validateWorkspaceDocsSettings(content string) []wsvalidate.Finding {
	var settings map[string]any
	if err := json.Unmarshal([]byte(content), &settings); err != nil || settings == nil {
		return []wsvalidate.Finding{{
			Severity: wsvalidate.Error,
			Code:     "workspace_docs",
			Message:  `expected a JSON object, such as {"rule_text": true}`,
		}}
	}
	if v, ok := settings["rule_text"]; ok {
		if _, isBool := v.(bool); !isBool {
			return []wsvalidate.Finding{{
				Severity: wsvalidate.Error,
				Code:     "workspace_docs",
				Message:  "rule_text must be true or false",
				Path:     "/rule_text",
			}}
		}
	}
	return nil
}

// validateOptimizerStatistics refuses a statistics file the compiler would
// refuse. The compiler reads it strictly (optstats.Decode: format, version, exact
// keys, valid counts) and a file it cannot read fails every compile of the
// workspace, the next deployment's start-up compile included. The seventh row,
// added on 2026-10-26 when the IDE began listing the file (wsfile/sections.go,
// `jet_rules`): listing it made it editable, and a save checked only that it
// was a JSON object. As a suffix it covers each statistics process's own file,
// `<process>.optimizer_statistics.json`, too.
func validateOptimizerStatistics(content string) []wsvalidate.Finding {
	if _, err := optstats.Decode(strings.NewReader(content)); err != nil {
		return []wsvalidate.Finding{{
			Severity: wsvalidate.Error,
			Code:     "optimizer_statistics",
			Message:  err.Error(),
		}}
	}
	return nil
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
		if entry.applies != nil && !entry.applies(fileName) {
			continue
		}
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
// Returns a nil error when the file may be written, with the validator's
// warnings, which do not refuse the save and which the response carries to the
// editor (saveCheckedWorkspaceFile). **Until 2026-11-14 the warnings were
// dropped here**, so a validator's warning tier reached nobody: the IDE showed
// a refusal and nothing else.
func checkWorkspaceFile(fileName, content string) (warnings []wsvalidate.Finding, err error) {
	// A file the IDE shows read-only is refused here too, so that the editor's
	// read-only state is not the only thing between a direct request and the
	// write (wsfile.ReadOnlySuffixes, 2026-10-27).
	if wsfile.IsReadOnly(fileName) {
		return nil, fmt.Errorf("%s is read-only in the workspace IDE; edit it in the workspace repository", fileName)
	}
	// docs/reference/ is where the documentation build writes the workspace
	// guide's generated pages, and it fails on a workspace whose docs/ holds
	// one (wsfile.IsGeneratedDocsPath, 2026-11-14), so a page saved there would
	// fail the next UI image build.
	if wsfile.IsGeneratedDocsPath(fileName) {
		return nil, fmt.Errorf("%s cannot be saved: docs/reference/ is generated, the workspace guide's pages "+
			"made from the compiled workspace, and a written page may not take its place; "+
			"save the page under another name", fileName)
	}
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
			return nil, fmt.Errorf("the file is not a valid json file: %v", err)
		}
	}
	validate := validatorFor(fileName)
	if validate == nil {
		return nil, nil
	}
	findings := validate(content)
	if errs := wsvalidate.ErrorsOnly(findings); len(errs) > 0 {
		return nil, describeFindings(fileName, errs)
	}
	return wsvalidate.WarningsOnly(findings), nil
}

// saveCheckedWorkspaceFile is SaveWorkspaceFileContent once the user's
// capability is established: check the file, write it with write when nothing
// refuses it, and answer with the warnings.
//
// The response gained `warnings` on 2026-11-14, a list of wsvalidate.Finding
// (severity, code, message, and path or line where the finding has one),
// empty rather than absent when there are none. **An added field, so a client
// that reads only `file_name` is unaffected**; a refused save is the 400 with
// the message describeFindings writes, as before.
func saveCheckedWorkspaceFile(wsFileName any, fileName, content string,
	write func() error) (results *map[string]any, httpStatus int, err error) {
	warnings, err := checkWorkspaceFile(fileName, content)
	if err != nil {
		log.Println(err)
		return nil, http.StatusBadRequest, err
	}
	// **A failed write is a 500.** It was the error with a 200 until 2026-11-14,
	// and DoDataTableAction writes an error body with the status it is given
	// (ERROR, jets/apiserver/utils.go), so the IDE, which reads the status,
	// reported a file whose write had failed as saved.
	if err = write(); err != nil {
		log.Println(err)
		return nil, http.StatusInternalServerError, err
	}
	if warnings == nil {
		warnings = []wsvalidate.Finding{}
	}
	return &map[string]any{
		"file_name": wsFileName,
		"warnings":  warnings,
	}, http.StatusOK, nil
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
