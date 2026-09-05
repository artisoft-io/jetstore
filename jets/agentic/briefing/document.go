package briefing

import "github.com/artisoft-io/jetstore/jets/wsvalidate"

// DocumentSuffix is the workspace file suffix of a provenance schema, and the
// row it takes in the save path's per-suffix validator table
// (`workspaceFileValidators`, `jets/datatable/workspace_file_validators.go:29`).
//
// **`AK.1` left this open on purpose and `AK.2` closes it** (I-417): `ParseSchema`
// already returned `wsvalidate.Finding`, which is the signature a row takes, so
// what was missing was not a mechanism but a home. The suffix follows the family
// already in that table - `.uf.json`, `.ua.json`, `.form.json`, `.tc.json` - and
// the longest-match dispatch in `validatorFor` keeps it from being shadowed by
// the generic `.JSON` well-formedness check it sits behind.
//
// # Where the file lives, which the table does not decide
//
// By convention `provenance/<key>.pv.json` in the workspace, beside the
// `pipes_config/` whose briefing it describes. The table dispatches on suffix
// alone, so the directory is a convention rather than a rule - which is the same
// arrangement `.tc.json` has.
//
// **It is workspace content rather than a JetStore asset, and that is a choice.**
// A provenance schema names the fields of one briefing and the properties of one
// domain model; `cintel:` is `jets_ws`'s vocabulary and means nothing in the
// other three workspaces. That puts it on the opposite side of the line from
// P3 U.2's projected user flows, which are byte-identical everywhere and are
// installed rather than committed.
const DocumentSuffix = ".pv.json"

// ValidateProvenanceDocument is the save path's check on a provenance schema.
//
// It is `ParseSchema` with the parsed schema dropped, which is the whole of what
// a validator row is: the save path wants to know whether the file may be
// written, and the reader that will act on it parses it again when it runs.
//
// **What a save-time check buys here is more than well-formedness.** Through
// Cover, saving a `.pv.json` that declares its `response_format` asserts that
// every field the briefing can carry has a rule and that no rule names a field
// it cannot carry. So the totality that `Check` establishes per record is
// established for the contract at the moment somebody edits it, by the editor
// they edited it in.
func ValidateProvenanceDocument(content string) []wsvalidate.Finding {
	_, findings := ParseSchema(content)
	return findings
}
