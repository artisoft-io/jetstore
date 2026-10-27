package datatable

import (
	"strings"
	"testing"

	"github.com/artisoft-io/jetstore/jets/datatable/wsfile"
	"github.com/artisoft-io/jetstore/jets/workspace"
)

// The validator's file types and the IDE's sections, tied.
//
// **They are two lists in two packages, and a file type added to one and not
// the other has happened at least three times** (jetstore_maintenance_02
// `I-48`, 2026-10-27, which names ui_refresh `I-27`, cgt_test_harness_filters
// `R-15` and rules_unit_test_harness `I-46`). A validator row with no section is
// a file type the save path checks and the IDE cannot open: `.pv.json` had been
// one since 2026-09-05, and this test found it on its first run. Michel's answer
// to that project's `Q-21`, the same day, kept the sections a Go table and asked
// for this test rather than a data-driven registry.

// fileTypesNotShownInTheIde are the validated file types that no section lists,
// each on purpose, with the reason. **Empty, and kept**: a type goes here only
// when somebody decides the IDE should not show it, and the reason is written
// beside it. A type missing from both this list and every section turns
// TestEveryValidatedFileTypeIsShownInTheIde red.
var fileTypesNotShownInTheIde = map[string]string{}

// shownBy names a section whose Filters list a file of this type, or "".
//
// **A filter shows a type only when it is at least as specific as the type's
// suffix**: `.uf.json` and `workspace_init_db.sql` qualify for `.uf.json` and
// `.sql`, and the `reports` section's bare `.json` does not qualify for
// `.pv.json`. A filter that is a suffix of the type would let every JSON type
// pass through `reports`, which is not where any of them lives.
func shownBy(suffix string) string {
	for _, s := range wsfile.WorkspaceSections {
		for _, filter := range s.Filters {
			if strings.HasSuffix(strings.ToLower(filter), strings.ToLower(suffix)) {
				return s.Dir
			}
		}
	}
	return ""
}

// validatedFileTypes is every suffix the save path dispatches on, and the rules
// unit tests' suffix, which has no row (a test case is checked when it runs,
// not when it is saved) and has been missed by the sections before.
func validatedFileTypes() []string {
	types := make([]string, 0, len(workspaceFileValidators)+1)
	for _, entry := range workspaceFileValidators {
		types = append(types, entry.suffix)
	}
	return append(types, workspace.UnitTestSuffix)
}

func TestEveryValidatedFileTypeIsShownInTheIde(t *testing.T) {
	for _, suffix := range validatedFileTypes() {
		dir := shownBy(suffix)
		_, notShown := fileTypesNotShownInTheIde[suffix]
		switch {
		case dir == "" && !notShown:
			t.Errorf("%q is a workspace file type (workspace_file_validators.go, or "+
				"workspace.UnitTestSuffix) and no section of wsfile.WorkspaceSections lists it, "+
				"so the IDE cannot open it. Add it to a section's Filters, or to "+
				"fileTypesNotShownInTheIde with the reason.", suffix)
		case dir != "" && notShown:
			t.Errorf("%q is listed by the %q section and is also in fileTypesNotShownInTheIde; "+
				"one of the two is stale", suffix, dir)
		}
	}
}

// Pins where the two types the plan asked about are shown, so that a filter
// edit that moves one is seen rather than passed by another section.
func TestTheSqlAndStatisticsTypesAreShownWhereTheyLive(t *testing.T) {
	for suffix, want := range map[string]string{
		".pv.json":                  "provenance",
		"optimizer_statistics.json": "jet_rules",
		workspace.UnitTestSuffix:    "jet_rules",
	} {
		if got := shownBy(suffix); got != want {
			t.Errorf("%q: shown by %q, want %q", suffix, got, want)
		}
	}
	// `.sql` is listed whole by `reports` and as `workspace_init_db.sql` by
	// `process_config`, whose every SQL file has that name in the four
	// workspaces (2026-10-27).
	var sqlSections []string
	for _, s := range wsfile.WorkspaceSections {
		for _, f := range s.Filters {
			if strings.HasSuffix(f, ".sql") {
				sqlSections = append(sqlSections, s.Dir)
			}
		}
	}
	if strings.Join(sqlSections, ",") != "process_config,reports" {
		t.Errorf("sections listing SQL: got %v, want [process_config reports]", sqlSections)
	}
}

// A read-only type with a validator row would be checked on a save that cannot
// happen; the plan made a README read-only *unless* a row exists for it, so the
// two may not both hold.
func TestNoReadOnlyFileTypeHasAValidator(t *testing.T) {
	for _, suffix := range wsfile.ReadOnlySuffixes {
		if validatorFor("x"+suffix) != nil {
			t.Errorf("%q is read-only in the IDE and has a validator row; drop one of the two", suffix)
		}
	}
}

func TestTheSavePathRefusesAReadOnlyFile(t *testing.T) {
	for _, name := range []string{"README.md", "pipes_config/README.md"} {
		if err := checkWorkspaceFile(name, "# notes\n"); err == nil {
			t.Errorf("%s: saved, want refused as read-only", name)
		}
	}
	// And only those: a file type with no row is still saved unchecked.
	if err := checkWorkspaceFile("jet_rules/a.jr", "# rules\n"); err != nil {
		t.Errorf("jet_rules/a.jr: %v, want saved", err)
	}
}
