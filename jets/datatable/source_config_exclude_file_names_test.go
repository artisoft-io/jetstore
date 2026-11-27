package datatable

// source_config.exclude_file_names_json: what a save stores, what a file's
// name is matched against, and what RegisterFileKeys does with an excluded
// file. The database tests use the fixture of register_file_key_action_test.go
// and skip unless JETS_TEST_DSN is set; see the header of that file.

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"testing"

	"github.com/artisoft-io/jetstore/jets/compute_pipes"
)

func TestExcludedFileName(t *testing.T) {
	const key = "client=c/org=o/object_type=Claims/"
	cases := []struct {
		name, fileKey, list string
		want                bool
		err                 bool
	}{
		{"null list", key + "claims_trailer.csv", "", false, false},
		{"json null", key + "claims_trailer.csv", "null", false, false},
		{"empty list", key + "claims_trailer.csv", "[]", false, false},
		{"name contains it", key + "claims_trailer.csv", `["_trailer"]`, true, false},
		{"name does not", key + "claims.csv", `["_trailer"]`, false, false},
		{"second of two", key + "CONTROL.txt", `["_trailer", "CONTROL"]`, true, false},
		{"case-sensitive", key + "claims_TRAILER.csv", `["_trailer"]`, false, false},
		{"a folder does not exclude", key + "batch_trailer/claims.csv", `["_trailer"]`, false, false},
		{"an object type does not exclude", "client=c/org=o/object_type=x_trailer/claims.csv", `["_trailer"]`, false, false},
		{"a key with no folder", "claims_trailer.csv", `["_trailer"]`, true, false},
		{"a folder key", key + "processing_ticket=T1/", `["T1"]`, false, false},
		{"an empty entry is ignored", key + "claims.csv", `[""]`, false, false},
		{"not an array", key + "claims_trailer.csv", `"_trailer"`, false, true},
		{"not json", key + "claims_trailer.csv", `_trailer`, false, true},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got, err := excludedFileName(c.fileKey, c.list)
			if (err != nil) != c.err {
				t.Fatalf("err = %v, want an error: %v", err, c.err)
			}
			if err != nil && !errors.Is(err, errExcludeFileNames) {
				t.Errorf("err = %v, want one wrapping errExcludeFileNames", err)
			}
			if got != c.want {
				t.Errorf("excluded = %v, want %v", got, c.want)
			}
		})
	}
}

func TestNormaliseExcludeFileNames(t *testing.T) {
	cases := []struct {
		name string
		in   any
		set  bool
		want any
		err  string
	}{
		{"absent", nil, false, nil, ""},
		{"nil", nil, true, nil, ""},
		{"blank clears", "  ", true, "", ""},
		{"null clears", "null", true, "", ""},
		{"empty array clears", "[]", true, "", ""},
		{"text is compacted", ` [ "_trailer" ,  "CONTROL" ] `, true, `["_trailer","CONTROL"]`, ""},
		{"decoded array", []any{"_trailer"}, true, `["_trailer"]`, ""},
		{"not json", "_trailer", true, nil, "must be a JSON array of strings"},
		{"an object", `{"a": "b"}`, true, nil, "must be a JSON array of strings"},
		{"a number in the array", []any{"_trailer", 3.0}, true, nil, "entry 1 must be a string"},
		{"an empty entry", `["_trailer", ""]`, true, nil, "entry 1 is empty"},
		{"a slash", `["in/_trailer"]`, true, nil, `holds a "/"`},
		{"a number", 3.0, true, nil, "must be a JSON array of strings"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			row := map[string]any{}
			if c.set {
				row["exclude_file_names_json"] = c.in
			}
			err := normaliseExcludeFileNames(row)
			if c.err != "" {
				if err == nil || !strings.Contains(err.Error(), c.err) {
					t.Fatalf("err = %v, want one naming %q", err, c.err)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if got := row["exclude_file_names_json"]; got != c.want {
				t.Errorf("bound %#v, want %#v", got, c.want)
			}
		})
	}
}

// setExcludeFileNames stores list, NULL when nil, as objectType's exclusion.
func setExcludeFileNames(t *testing.T, ctx *DataTableContext, objectType string, list *string) {
	t.Helper()
	if _, err := ctx.Dbpool.Exec(context.Background(),
		`UPDATE jetsapi.source_config SET exclude_file_names_json = $1 WHERE object_type = $2`,
		list, objectType); err != nil {
		t.Fatalf("setting exclude_file_names_json: %v", err)
	}
}

func strPtr(s string) *string { return &s }

// A file whose name contains an excluded substring is staged, registers no
// input and is reported as excluded_file_key; a file of the same source whose
// name does not, or whose folder does, registers. The other reasons are
// unchanged in the same request.
func TestRegisterFileKeysExcludesAFileByName(t *testing.T) {
	t.Setenv("JETS_SENTINEL_FILE_NAME", testSentinel)
	ctx := registerFileKeysFixture(t)
	seedSource(t, ctx, "Claims", 0, "{Claims}")
	setExcludeFileNames(t, ctx, "Claims", strPtr(`["_trailer","CONTROL"]`))

	data := fileData("Claims", "claims.csv")
	trailer := fileData("Claims", "claims_trailer.csv")
	control := fileData("Claims", "CONTROL.txt")
	inFolder := fileData("Claims", "batch_trailer/claims.csv")
	partFile := map[string]any{"client": testClient, "vendor": testOrg, "object_type": testObjectType,
		"file_key": testFolder + "T0901/part-00000.parquet", "size": int64(500)}
	unknown := fileData("Unknown", "u_trailer.csv")
	action := &RegisterFileKeyAction{
		Action:          "register_keys",
		Data:            []map[string]any{data, trailer, control, inFolder, partFile, unknown},
		NoAutomatedLoad: true,
	}
	res, status, err := ctx.RegisterFileKeys(action, "")
	if err != nil || status != http.StatusOK {
		t.Fatalf("status %d, err %v; want 200", status, err)
	}
	if n, _ := (*res)["registered"].(int); n != 2 {
		t.Errorf("registered = %v, want 2", (*res)["registered"])
	}
	skipped, _ := (*res)["skipped"].([]fileKeySkip)
	want := map[int]string{1: skipExcludedFileKey, 2: skipExcludedFileKey, 4: skipPartFile, 5: skipNoSourceConfig}
	if len(skipped) != len(want) {
		t.Fatalf("skipped = %+v, want rows %v", skipped, want)
	}
	for _, s := range skipped {
		if want[s.Row] != s.Reason {
			t.Errorf("row %d skipped as %q, want %q", s.Row, s.Reason, want[s.Row])
		}
	}
	for _, d := range []map[string]any{trailer, control} {
		fk := d["file_key"].(string)
		if n, _ := countFileKeyStaging(t, ctx, fk); n != 1 {
			t.Errorf("%s: staged %d times, want 1", fk, n)
		}
		if n := countInputRegistry(t, ctx, fk); n != 0 {
			t.Errorf("%s: %d input_registry rows, want 0", fk, n)
		}
	}
	for _, d := range []map[string]any{data, inFolder} {
		fk := d["file_key"].(string)
		if n := countInputRegistry(t, ctx, fk); n != 1 {
			t.Errorf("%s: %d input_registry rows, want 1", fk, n)
		}
	}
	// Two files registered, so two sessions: none for an excluded file.
	if n := countRows(t, ctx, "session_reservation"); n != 2 {
		t.Errorf("session_reservation has %d rows, want 2", n)
	}
}

// NULL, [] and blank exclude nothing, and so does a stored list register keys
// cannot read, which it logs rather than holding back every file of the source.
func TestRegisterFileKeysExcludesNothingByDefault(t *testing.T) {
	ctx := registerFileKeysFixture(t)
	seedSource(t, ctx, "Claims", 0, "{Claims}")
	for i, list := range []*string{nil, strPtr("[]"), strPtr(""), strPtr("_trailer")} {
		setExcludeFileNames(t, ctx, "Claims", list)
		data := fileData("Claims", "claims_trailer_"+string(rune('a'+i))+".csv")
		res, status, err := ctx.RegisterFileKeys(&RegisterFileKeyAction{Action: "register_keys",
			Data: []map[string]any{data}, NoAutomatedLoad: true}, "")
		if err != nil || status != http.StatusOK {
			t.Fatalf("list %v: status %d, err %v; want 200", list, status, err)
		}
		if n, _ := (*res)["registered"].(int); n != 1 {
			t.Errorf("list %v: registered = %v, skipped %v; want 1", list, (*res)["registered"], (*res)["skipped"])
		}
	}
}

// Registration reads the column through to_jsonb of the row, so it runs
// against a database the migration has not yet given the column, and
// excludes nothing there.
func TestRegisterFileKeysBeforeTheColumnExists(t *testing.T) {
	ctx := registerFileKeysFixture(t)
	seedSource(t, ctx, "Claims", 0, "{Claims}")
	if _, err := ctx.Dbpool.Exec(context.Background(),
		`ALTER TABLE jetsapi.source_config DROP COLUMN exclude_file_names_json`); err != nil {
		t.Fatal(err)
	}
	data := fileData("Claims", "claims_trailer.csv")
	res, status, err := ctx.RegisterFileKeys(&RegisterFileKeyAction{Action: "register_keys",
		Data: []map[string]any{data}, NoAutomatedLoad: true}, "")
	if err != nil || status != http.StatusOK {
		t.Fatalf("status %d, err %v; want 200", status, err)
	}
	if n, _ := (*res)["registered"].(int); n != 1 {
		t.Errorf("registered = %v, want 1", (*res)["registered"])
	}
}

// For a part-file source, a part file stays a part file, excluded or not, and
// is not staged; an excluded file the size of a sentinel is not taken for one:
// it is staged under its own key, not its folder's, and registers nothing.
func TestRegisterFileKeysExcludedFileOfAPartFileSource(t *testing.T) {
	t.Setenv("JETS_SENTINEL_FILE_NAME", "")
	ctx := registerFileKeysFixture(t)
	setExcludeFileNames(t, ctx, testObjectType, strPtr(`["_trailer"]`))
	part := map[string]any{"client": testClient, "vendor": testOrg, "object_type": testObjectType,
		"file_key": testFolder + "T1001/part_trailer.parquet", "size": int64(500)}
	small := map[string]any{"client": testClient, "vendor": testOrg, "object_type": testObjectType,
		"file_key": testFolder + "T1002/x_trailer", "size": int64(0)}
	res, status, err := ctx.RegisterFileKeys(&RegisterFileKeyAction{Action: "register_keys",
		Data: []map[string]any{part, small}, NoAutomatedLoad: true}, "")
	if err != nil || status != http.StatusOK {
		t.Fatalf("status %d, err %v; want 200", status, err)
	}
	skipped, _ := (*res)["skipped"].([]fileKeySkip)
	if len(skipped) != 2 || skipped[0].Reason != skipPartFile || skipped[1].Reason != skipExcludedFileKey {
		t.Fatalf("skipped = %+v, want part_file then excluded_file_key", skipped)
	}
	if n, _ := countFileKeyStaging(t, ctx, part["file_key"].(string)); n != 0 {
		t.Errorf("the part file was staged")
	}
	if n, _ := countFileKeyStaging(t, ctx, small["file_key"].(string)); n != 1 {
		t.Errorf("the excluded file was not staged under its own key")
	}
	if n, _ := countFileKeyStaging(t, ctx, testFolder+"T1002"); n != 0 {
		t.Errorf("the excluded file was staged as its folder")
	}
	if n := countRows(t, ctx, "input_registry"); n != 0 {
		t.Errorf("input_registry has %d rows, want 0", n)
	}
}

// A schema event naming an excluded file registers nothing, and so is an error
// naming the reason, as every schema event that registers nothing is.
func TestRegisterSchemaEventForAnExcludedFile(t *testing.T) {
	ctx := registerFileKeysFixture(t)
	seedSource(t, ctx, "Claims", 0, "{Claims}")
	setExcludeFileNames(t, ctx, "Claims", strPtr(`["_trailer"]`))
	startPipelinesOverride = func(*DataTableContext, int, int, string, string, string, string, string, bool) error {
		return nil
	}
	t.Cleanup(func() { startPipelinesOverride = nil })
	e := &compute_pipes.SchemaProviderSpec{Type: "default", Client: testClient, Vendor: testOrg, ObjectType: "Claims"}
	e.FileKey = "in/claims_trailer.csv"
	err := ctx.RegisterSchemaEvent(ctx.Dbpool, e, "")
	if err == nil || !strings.Contains(err.Error(), skipExcludedFileKey) {
		t.Errorf("err %v, want one naming %s", err, skipExcludedFileKey)
	}
	e.FileKey = "in/claims.csv"
	if err := ctx.RegisterSchemaEvent(ctx.Dbpool, e, ""); err != nil {
		t.Errorf("a schema event for a file not excluded: %v", err)
	}
}

// The column is added by the migration, and the source_config statements the
// Source Configuration flow posts to store, keep and clear the list.
func TestExcludeFileNamesColumnSaves(t *testing.T) {
	pool := createTablesFromSchema(t, "source_config")
	stored := func(key int) string {
		t.Helper()
		var text *string
		if err := pool.QueryRow(context.Background(),
			`SELECT exclude_file_names_json FROM jetsapi.source_config WHERE key = $1`, key).Scan(&text); err != nil {
			t.Fatal(err)
		}
		if text == nil {
			return "NULL"
		}
		return *text
	}
	row := sourceConfigRow("hc:Claim")
	row["exclude_file_names_json"] = `[ "_trailer" ]`
	key, err := saveSourceConfig(t, pool, "source_config", row)
	if err != nil {
		t.Fatal(err)
	}
	if got := stored(key); got != `["_trailer"]` {
		t.Fatalf("stored %s", got)
	}
	// An update that does not send it keeps it.
	row = sourceConfigRow("hc:Claim")
	row["key"] = key
	if _, err := saveSourceConfig(t, pool, "update/source_config", row); err != nil {
		t.Fatal(err)
	}
	if got := stored(key); got != `["_trailer"]` {
		t.Fatalf("an update without the list stored %s", got)
	}
	// A list that would exclude every file is refused, and the stored one stays.
	row = sourceConfigRow("hc:Claim")
	row["key"], row["exclude_file_names_json"] = key, `[""]`
	if _, err := saveSourceConfig(t, pool, "update/source_config", row); err == nil {
		t.Fatal("an empty entry was saved")
	}
	if got := stored(key); got != `["_trailer"]` {
		t.Fatalf("a refused update stored %s", got)
	}
	// An emptied field clears it.
	row = sourceConfigRow("hc:Claim")
	row["key"], row["exclude_file_names_json"] = key, ""
	if _, err := saveSourceConfig(t, pool, "update/source_config", row); err != nil {
		t.Fatal(err)
	}
	if got := stored(key); got != "NULL" {
		t.Fatalf("an emptied list stored %s, want NULL", got)
	}
	// An insert with an empty list, or without one, stores NULL.
	for i, v := range []any{"[]", nil} {
		row = sourceConfigRow("hc:Pharmacy" + string(rune('a'+i)))
		if v != nil {
			row["exclude_file_names_json"] = v
		}
		key, err = saveSourceConfig(t, pool, "source_config", row)
		if err != nil {
			t.Fatal(err)
		}
		if got := stored(key); got != "NULL" {
			t.Fatalf("insert with %v stored %s, want NULL", v, got)
		}
	}
	// A base script's insert that names none of the newer columns still runs.
	if _, err := pool.Exec(context.Background(), `INSERT INTO jetsapi.source_config
		(object_type, client, org, table_name, user_email) VALUES ('hc:Lab', 'c', 'o', 'c_o_hc:Lab', 'u@example.com')`); err != nil {
		t.Fatalf("an insert without the column: %v", err)
	}
}
