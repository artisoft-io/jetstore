---
title: Add a workspace file type
description: Give a kind of workspace file a validator that the Workspace IDE's save runs, list it in an IDE section, and test both.
kind: how-to
sidebar:
  order: 6
---

When the Workspace IDE saves a file, the apiserver checks it first, in `checkWorkspaceFile`
(`jets/datatable/workspace_file_validators.go`). What it checks is chosen by the file name's suffix
from one table, `workspaceFileValidators`. A file type with a row there is refused when its content is
wrong; a file type without one is only checked to be well-formed JSON when its name ends `.json`, and
not at all otherwise.

This was carried out on 2026-11-11 with a scratch `.note.json` type: a validator row and a test were
added, the package's tests run before and after the file type was listed in an IDE section, and the
changes discarded.

## How the save check works

`checkWorkspaceFile` does three things, in order:

1. **Refuses a read-only file** (`wsfile.IsReadOnly`; the IDE shows `.md` files and does not save
   them).
2. **Refuses a `.json` file that is not a JSON object**, whatever its suffix. That is a precondition
   of every JSON validator, so a validator for a `.json` type may assume well-formed content. A
   validator for any other suffix receives the raw text and checks its own form, as
   `sqlscript.ValidateScript` does for `.sql`.
3. **Runs the validator `validatorFor` picks**: the row whose suffix ends the file name, compared
   without regard to case, the longest when several match. Any finding of severity `Error` refuses
   the save; warnings do not.

A validator is a `wsvalidate.Validator`, a function from the file's content to a list of
`wsvalidate.Finding` (`jets/wsvalidate/wsvalidate.go`). A finding carries a `Severity`, a `Code`, a
`Message`, and where it is: `Path`, a JSON Pointer into the document, or `Line`, for a file whose
coordinates are lines. `describeFindings` prints each with its coordinate in the error the IDE shows.
`jets/wsvalidate` imports nothing, so a validator can live in the package its file type belongs to
(`jets/userflow`, `jets/runbook`, `jets/optstats`) without importing the HTTP layer.

## 1. Write the validator

Write it in the package that owns the file type, returning findings rather than an error. The scratch
one required a `title`:

```go
func validateNote(content string) []wsvalidate.Finding {
	var doc struct{ Title string `json:"title"` }
	if err := json.Unmarshal([]byte(content), &doc); err != nil || doc.Title == "" {
		return []wsvalidate.Finding{{Severity: wsvalidate.Error, Code: "note_title",
			Message: "a note needs a title", Path: "/title"}}
	}
	return nil
}
```

Give the finding a `Path` or a `Line` whenever there is one: the IDE, and anything repairing the file
from the message, need to know where.

## 2. Add the row

Append it to `workspaceFileValidators`:

```go
{".note.json", validateNote},
```

**Choose a suffix no other row ends with**, and no other row ends with it. `TestSuffixesAreDistinct`
fails when one suffix ends another, because longest match would then decide between them silently.
Prefer a constant from the owning package (`runbook.FileSuffix`, `briefing.DocumentSuffix`) to a
literal.

## 3. List it in an IDE section

The IDE shows a workspace's files by section, `WorkspaceSections` in `jets/datatable/wsfile/sections.go`,
and a section shows only the suffixes in its `Filters`. **A validated file type that no section lists
fails `TestEveryValidatedFileTypeIsShownInTheIde`**, because the IDE could not open it. With the row
added and nothing else, the run here failed:

```text
--- FAIL: TestEveryValidatedFileTypeIsShownInTheIde (0.00s)
    ".note.json" is a workspace file type (workspace_file_validators.go, or workspace.UnitTestSuffix)
    and no section of wsfile.WorkspaceSections lists it, so the IDE cannot open it. Add it to a
    section's Filters, or to fileTypesNotShownInTheIde with the reason.
```

Add the suffix to the `Filters` of the section for its folder. The filter must be at least as specific
as the row's suffix: a section's bare `.json` does not count as showing `.note.json`. A type that
deliberately has no section goes in `fileTypesNotShownInTheIde`
(`jets/datatable/workspace_file_sections_test.go`) with the reason. A new folder is a new `Section`,
which changes `wsfile.SectionDeclaration` and so the two tests that pin it, Go's and the web app's
(`jetsclient_ide/src/screens/sectionContract.ts`); a filter added to an existing section changes
neither.

## 4. Test it

Add a test that goes through `checkWorkspaceFile`, not only the validator, so that the dispatch is
tested too, as `TestRunbookGoesThroughTheSavePath` does:

```go
func TestNoteGoesThroughTheSavePath(t *testing.T) {
	if err := checkWorkspaceFile("reports/a.note.json", `{"title": "x"}`); err != nil {
		t.Fatalf("a note with a title must save: %v", err)
	}
	if err := checkWorkspaceFile("reports/a.note.json", `{"body": "x"}`); err == nil {
		t.Fatal("a note without a title must not save")
	}
}
```

and the file names it should and should not match to the table in
`TestValidatorForPicksTheMostSpecificSuffix`. Then:

```bash
go test -count=1 -run 'Validator|Suffixes|SaveCheck|CheckWorkspaceFile|Shown|ReadOnly|Note' ./jets/datatable/
go test -count=1 ./jets/datatable/wsfile/ ./jets/wsvalidate/
```

With the section filter added, every test passed here, and the refused save read:

```text
reports/a.note.json cannot be saved:
  /title: a note needs a title
```

**A file type outside JSON may cost more than a row.** The `.sql` row was the first, and needed the
JSON check to stop gating the whole function and `Finding` to gain `Line`. The comment above
`workspaceFileValidators` records what each row cost; read it before adding one of a new kind.
