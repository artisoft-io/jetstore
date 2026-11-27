package datatable

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"
)

// source_config.exclude_file_names_json names the files of a data source that
// register keys does not register: a JSON array of substrings, for example
// ["_trailer"]. A file whose name, the last segment of its file key, contains
// one of them, case-sensitive, is staged in file_key_staging, so it is listed
// among the files that arrived, and registers no input: RegisterFileKeys skips
// it as excluded_file_key. NULL, blank text and [] exclude nothing.
//
// It is for a file a client sends beside its data that is not data, such as a
// trailer or a control file, which would otherwise be registered as a delivery
// and fail the pipeline it starts.

// parseExcludeFileNames decodes the stored list. Blank text and the JSON null
// are an empty list. Anything other than an array of strings is an error.
func parseExcludeFileNames(text string) ([]string, error) {
	text = strings.TrimSpace(text)
	if text == "" || text == "null" {
		return nil, nil
	}
	var names []string
	if err := json.Unmarshal([]byte(text), &names); err != nil {
		return nil, fmt.Errorf("exclude_file_names_json must be a JSON array of strings, for example [\"_trailer\"]: %v", err)
	}
	return names, nil
}

// checkExcludeFileNames refuses what a save must not store: an empty
// substring, which every name contains, so the source would register nothing,
// and one holding a "/", which no name can contain, so it would exclude nothing.
func checkExcludeFileNames(names []string) error {
	for i, n := range names {
		switch {
		case n == "":
			return fmt.Errorf("exclude_file_names_json: entry %d is empty, which would exclude every file", i)
		case strings.Contains(n, "/"):
			return fmt.Errorf("exclude_file_names_json: entry %d (%q) holds a \"/\": it is matched against the file's name, the last segment of its file key, so it would exclude nothing", i, n)
		}
	}
	return nil
}

// normaliseExcludeFileNames checks row["exclude_file_names_json"] before a
// source_config insert or update binds it, and leaves in the row what the
// statement expects (sql_stmts.go, the source_config statements):
//
//   - absent or nil stays nil: the insert stores NULL, the update keeps the
//     stored list;
//   - blank text, the JSON null or an empty array becomes "", which the insert
//     stores as NULL and the update uses to clear the stored list;
//   - anything else must be a JSON array of non-empty strings with no "/",
//     given as JSON text or as a decoded array, and is bound as compact JSON
//     text. Otherwise the request is refused.
func normaliseExcludeFileNames(row map[string]any) error {
	raw, ok := row["exclude_file_names_json"]
	if !ok || raw == nil {
		row["exclude_file_names_json"] = nil
		return nil
	}
	var names []string
	switch v := raw.(type) {
	case string:
		var err error
		if names, err = parseExcludeFileNames(v); err != nil {
			return fmt.Errorf("source_config: %w", err)
		}
	case []any:
		for i, e := range v {
			s, ok := e.(string)
			if !ok {
				return fmt.Errorf("source_config: exclude_file_names_json: entry %d must be a string, got %T", i, e)
			}
			names = append(names, s)
		}
	case []string:
		names = v
	default:
		return fmt.Errorf("source_config: exclude_file_names_json must be a JSON array of strings, got %T", raw)
	}
	if err := checkExcludeFileNames(names); err != nil {
		return fmt.Errorf("source_config: %w", err)
	}
	if len(names) == 0 {
		row["exclude_file_names_json"] = ""
		return nil
	}
	b, err := json.Marshal(names)
	if err != nil {
		return fmt.Errorf("source_config: exclude_file_names_json: %v", err)
	}
	row["exclude_file_names_json"] = string(b)
	return nil
}

// errExcludeFileNames marks a stored list RegisterFileKeys cannot read.
var errExcludeFileNames = errors.New("unreadable exclude_file_names_json")

// excludedFileName reports whether fileKey's name, its last segment, contains
// one of the substrings stored in text. A folder of the key never excludes it.
// An empty substring, which a base script can store, is ignored rather than
// excluding every file. A stored list that is not a JSON array of strings
// excludes nothing and is returned as an error wrapping errExcludeFileNames,
// for the caller to log: the file then registers, as it did before the list
// existed, rather than holding back every file of the source.
func excludedFileName(fileKey string, text string) (bool, error) {
	names, err := parseExcludeFileNames(text)
	if err != nil {
		return false, fmt.Errorf("%w: %v", errExcludeFileNames, err)
	}
	if len(names) == 0 {
		return false, nil
	}
	name := fileKey[strings.LastIndex(fileKey, "/")+1:]
	if name == "" {
		return false, nil
	}
	for _, n := range names {
		if n != "" && strings.Contains(name, n) {
			return true, nil
		}
	}
	return false, nil
}
