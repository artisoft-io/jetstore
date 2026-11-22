package datatable

import (
	"encoding/json"
	"fmt"

	"github.com/artisoft-io/jetstore/jets/agentic/feedprofile"
)

// normaliseFeedProfile checks row["feed_profile_json"] before a source_config
// insert or update binds it, and leaves in the row what the statement expects
// (sql_stmts.go, the source_config statements):
//
//   - absent or nil stays nil: the insert stores NULL, the update keeps the
//     stored profile;
//   - an empty profile, blank text or the JSON null, becomes the JSON null,
//     which the insert stores as NULL and the update uses to clear the stored
//     profile;
//   - anything else must be a profile that decodes and validates
//     (feedprofile.Parse), given as JSON text or as a JSON object, and is bound
//     as text. Otherwise the request is refused with the findings.
//
// The Source Configuration flow sends the text of its field, so an emptied
// field clears the profile and an untouched record keeps it.
func normaliseFeedProfile(row map[string]any) error {
	raw, ok := row["feed_profile_json"]
	if !ok || raw == nil {
		row["feed_profile_json"] = nil
		return nil
	}
	var text string
	switch v := raw.(type) {
	case string:
		text = v
	case map[string]any:
		b, err := json.Marshal(v)
		if err != nil {
			return fmt.Errorf("source_config: feed_profile_json: %v", err)
		}
		text = string(b)
	default:
		return fmt.Errorf("source_config: feed_profile_json must be JSON text or an object, got %T", raw)
	}
	if feedprofile.IsEmpty(text) {
		row["feed_profile_json"] = "null"
		return nil
	}
	_, findings := feedprofile.Parse([]byte(text))
	if err := feedprofile.Errors(findings); err != nil {
		return fmt.Errorf("source_config: %w", err)
	}
	row["feed_profile_json"] = text
	return nil
}
