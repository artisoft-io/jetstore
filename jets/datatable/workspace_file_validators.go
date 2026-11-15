package datatable

import (
	"encoding/json"
	"fmt"
	"io/fs"
	"log"
	"net/http"
	"path"
	"strings"

	"github.com/artisoft-io/jetstore/jets/agentic/briefing"
	"github.com/artisoft-io/jetstore/jets/compute_pipes"
	"github.com/artisoft-io/jetstore/jets/datatable/wsfile"
	"github.com/artisoft-io/jetstore/jets/optstats"
	"github.com/artisoft-io/jetstore/jets/run_reports/directives"
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
// `.sql` files were saved through the `.sql` row above, lexically, and have
// their own row since 2026-11-14 (below), which also refuses a statement no
// step may run; their `:name` bindings are checked against the runbook at run
// start, because a SQL file does not know which runbook reads it.
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
//
// `reports/config.json` is the thirteenth (2026-11-14), the second row scoped
// to a path and the first scoped to one file: the report directives
// run_reports follows, checked as run_reports checks them before any report
// runs (workspace_reports_config.go). Its suffix is `config.json`, which
// names no file type, so the row applies to that one file and no other
// `config.json` (isReportsConfig).
//
// **A row may read the rest of the workspace since 2026-11-14**, through its
// `inWorkspace` check, which is given the file and the workspace's tree as the
// save finds it. A validator is given one file, and two references were
// checked only when the workspace compiled or a report ran: the table a form
// draws (`.form.json`, checkFormTables) and the reports files the report
// directives run (`reports/config.json`, reportsFilesFindings). The check runs
// once the file's own validator has refused nothing, and only where the save
// has a workspace to read (checkWorkspaceFileIn). It is given the file's
// workspace-relative name too since later the same day, when a reports file
// needed it to find the directives that run it.
//
// **Two `.sql` rows scoped to a directory, the fourteenth and fifteenth
// (2026-11-14), beside the `.sql` row that checks every SQL file lexically.**
// A runbook's SQL, under `runbooks/`, is refused when it holds a statement no
// runbook step may run (validateRunbookSQL), which a run refused when it
// started. A reports file, under `reports/`, is checked against the directives
// in `reports/config.json` that run it (reportsSQLFindings), which the
// directives' own save did and the file's save did not, so a reports file
// saved after its directives was found wrong when a report ran. Both validate
// lexically first, as the `.sql` row does. The three rows share a suffix, so
// the dispatch prefers a scoped row to an unscoped one on a suffix of the same
// length (validatorsFor).
var workspaceFileValidators = []struct {
	suffix string
	// applies, when set, narrows the row to the files it returns true for,
	// given the workspace-relative path; unset, the row applies to every file
	// with its suffix.
	applies  func(fileName string) bool
	validate wsvalidate.Validator
	// inWorkspace, when set, checks the file against the rest of the
	// workspace, whose root is workspace.
	inWorkspace inWorkspaceCheck
}{
	{".uf.json", nil, userflow.ValidateFlowDocument, nil},
	{".ua.json", nil, userflow.ValidateActionDocument, nil},
	{".form.json", nil, userflow.ValidateFormDocument, checkFormTables},
	{".tc.json", nil, userflow.ValidateTableDocument, nil},
	{briefing.DocumentSuffix, nil, briefing.ValidateProvenanceDocument, nil},
	{".sql", nil, sqlscript.ValidateScript, nil},
	{path.Base(optstats.FileName), nil, validateOptimizerStatistics, nil},
	{runbook.FileSuffix, nil, runbook.ValidateFile, nil},
	{workspaceDocsSettingsFile, nil, validateWorkspaceDocsSettings, nil},
	{".md", isDocsPage, validateDocsPage, nil},
	{".mdx", isDocsPage, validateDocsPage, nil},
	{".pc.json", nil, compute_pipes.ValidateConfigDocument, nil},
	{path.Base(directives.ConfigFile), isReportsConfig, validateReportsConfig, reportsFilesFindings},
	{".sql", isRunbookSQL, validateRunbookSQL, nil},
	{".sql", isReportsSQL, sqlscript.ValidateScript, reportsSQLFindings},
}

// inWorkspaceCheck is a row's check of a file against the rest of the
// workspace: fileName is the file's workspace-relative name, content what the
// save would write, and workspace the workspace's tree as the save finds it,
// the file's previous content included.
type inWorkspaceCheck func(fileName, content string, workspace fs.FS) []wsvalidate.Finding

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
	validate, _ := validatorsFor(fileName)
	return validate
}

// validatorsFor is validatorFor's row: its validator and its inWorkspace
// check, either of which may be nil.
//
// On two suffixes of the same length, a row scoped by `applies` is more
// specific than one that is not: `runbooks/a/check.sql` is a runbook's SQL
// before it is SQL. Two scoped rows of one suffix apply to disjoint files.
func validatorsFor(fileName string) (wsvalidate.Validator, inWorkspaceCheck) {
	upper := strings.ToUpper(fileName)
	var best wsvalidate.Validator
	var bestInWorkspace inWorkspaceCheck
	bestLen, bestScoped := 0, false
	for _, entry := range workspaceFileValidators {
		scoped := entry.applies != nil
		if scoped && !entry.applies(fileName) {
			continue
		}
		if !strings.HasSuffix(upper, strings.ToUpper(entry.suffix)) {
			continue
		}
		if n := len(entry.suffix); n > bestLen || (n == bestLen && scoped && !bestScoped) {
			best, bestInWorkspace, bestLen, bestScoped = entry.validate, entry.inWorkspace, n, scoped
		}
	}
	return best, bestInWorkspace
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
//
// It checks the file alone: checkWorkspaceFileIn with no workspace.
func checkWorkspaceFile(fileName, content string) (warnings []wsvalidate.Finding, err error) {
	return checkWorkspaceFileIn(fileName, content, nil)
}

// checkWorkspaceFileIn is checkWorkspaceFile with the workspace the file is
// saved into, whose root is workspace, for the rows that check a file against
// the rest of it (a row's inWorkspace). A nil workspace skips those checks.
func checkWorkspaceFileIn(fileName, content string, workspace fs.FS) (warnings []wsvalidate.Finding, err error) {
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
	validate, inWorkspace := validatorsFor(fileName)
	if validate == nil {
		return nil, nil
	}
	findings := validate(content)
	if errs := wsvalidate.ErrorsOnly(findings); len(errs) > 0 {
		return nil, describeFindings(fileName, errs)
	}
	if inWorkspace != nil && workspace != nil {
		findings = append(findings, inWorkspace(fileName, content, workspace)...)
		if errs := wsvalidate.ErrorsOnly(findings); len(errs) > 0 {
			return nil, describeFindings(fileName, errs)
		}
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
//
// workspace is the root of the workspace the file is saved into, for the rows
// that check a file against the rest of it, or nil (checkWorkspaceFileIn).
func saveCheckedWorkspaceFile(wsFileName any, fileName, content string, workspace fs.FS,
	write func() error) (results *map[string]any, httpStatus int, err error) {
	warnings, err := checkWorkspaceFileIn(fileName, content, workspace)
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
