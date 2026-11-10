package datatable

// What RegisterFileKeys does with a request as a whole (JM-096): a bad row is
// refused before any row is written, a spec file_size is recorded, a duplicate
// domain key is registered once, and the result says which rows registered and
// why the others did not. The tests use the fixture of
// register_file_key_action_test.go and skip unless JETS_TEST_DSN is set; see the
// header of that file.

import (
	"context"
	"net/http"
	"testing"
)

func countRows(t *testing.T, ctx *DataTableContext, table string) int {
	t.Helper()
	var n int
	if err := ctx.Dbpool.QueryRow(context.Background(),
		`SELECT count(*) FROM jetsapi.`+table).Scan(&n); err != nil {
		t.Fatalf("counting %s: %v", table, err)
	}
	return n
}

// A request whose second row is bad is a 400 that writes nothing: not the first
// row's file_key_staging, source_period, session or input_registry rows. Each row
// was validated and written in turn, so the 400 followed a partial write.
func TestRegisterFileKeysBadRowWritesNothing(t *testing.T) {
	t.Setenv("JETS_SENTINEL_FILE_NAME", testSentinel)
	ctx := registerFileKeysFixture(t)
	good := schemaEventData("T0601", int64(0), true)
	bad := schemaEventData("T0602", int64(0), true)
	bad["month"] = true
	action := &RegisterFileKeyAction{
		Action:          "register_keys",
		Data:            []map[string]any{good, bad},
		NoAutomatedLoad: true,
		IsSchemaEvent:   true,
	}
	_, status, err := ctx.RegisterFileKeys(action, "")
	if err == nil || status != http.StatusBadRequest {
		t.Fatalf("status %d, err %v; want 400 and an error", status, err)
	}
	for _, table := range []string{"file_key_staging", "source_period", "session_reservation",
		"session_registry", "input_registry"} {
		if n := countRows(t, ctx, table); n != 0 {
			t.Errorf("%s has %d row(s) after a 400, want 0", table, n)
		}
	}
}

// seedSource adds a source_config for testClient and testOrg under objectType,
// with its object type registered.
func seedSource(t *testing.T, ctx *DataTableContext, objectType string, isPartFiles int, domainKeys string) {
	t.Helper()
	for _, stmt := range []string{
		`INSERT INTO jetsapi.object_type_registry (object_type, entity_rdf_type)
			VALUES ('` + objectType + `', 'ex:` + objectType + `')`,
		`INSERT INTO jetsapi.source_config
			(client, org, object_type, table_name, domain_keys, input_format, is_part_files, user_email)
			VALUES ('` + testClient + `', '` + testOrg + `', '` + objectType + `',
				'` + objectType + `_table', '` + domainKeys + `', 'csv', ` + string(rune('0'+isPartFiles)) + `, 'system')`,
	} {
		if _, err := ctx.Dbpool.Exec(context.Background(), stmt); err != nil {
			t.Fatalf("seeding: %v\n%s", err, stmt)
		}
	}
}

// fileData is one row naming a single file of objectType, in the shape the
// register-key lambda builds for an S3 event.
func fileData(objectType, name string) map[string]any {
	return map[string]any{
		"client":      testClient,
		"vendor":      testOrg,
		"object_type": objectType,
		"file_key":    "client=" + testClient + "/org=" + testOrg + "/object_type=" + objectType + "/" + name,
	}
}

// The file_size a schema event carries is what file_key_staging records. It had
// no case in the copying switch and was dropped, so the recorded size was the S3
// event's size, or 0 for a schema event, which has none.
func TestRegisterFileKeysRecordsTheSpecFileSize(t *testing.T) {
	ctx := registerFileKeysFixture(t)
	seedSource(t, ctx, "Claims", 0, "{Claims}")

	// A schema event: file_size and no size, numbers decoded from JSON.
	data := fileData("Claims", "claims_1.csv")
	data["file_size"] = float64(12345)
	if status, err := registerOne(t, ctx, data); err != nil || status != http.StatusOK {
		t.Fatalf("status %d, err %v", status, err)
	}
	if _, size := countFileKeyStaging(t, ctx, data["file_key"].(string)); size != 12345 {
		t.Errorf("file_size = %d, want the spec's 12345", size)
	}

	// Both: the spec's file_size is recorded, not the object's size.
	data = fileData("Claims", "claims_2.csv")
	data["size"] = int64(100)
	data["file_size"] = float64(12345)
	if status, err := registerOne(t, ctx, data); err != nil || status != http.StatusOK {
		t.Fatalf("status %d, err %v", status, err)
	}
	if _, size := countFileKeyStaging(t, ctx, data["file_key"].(string)); size != 12345 {
		t.Errorf("file_size = %d, want the spec's 12345 over the object's 100", size)
	}

	// No file_size: the object's size, as before.
	data = fileData("Claims", "claims_3.csv")
	data["size"] = int64(100)
	if status, err := registerOne(t, ctx, data); err != nil || status != http.StatusOK {
		t.Fatalf("status %d, err %v", status, err)
	}
	if _, size := countFileKeyStaging(t, ctx, data["file_key"].(string)); size != 100 {
		t.Errorf("file_size = %d, want the object's 100", size)
	}
}

// The part-file gate reads the S3 object's size, never the spec's file_size: a
// schema event for a part-file source with a file_size > 1 registers its folder,
// and an S3 part file with a size > 1 is skipped as before.
func TestRegisterFileKeysPartFileGateReadsTheObjectSize(t *testing.T) {
	t.Setenv("JETS_SENTINEL_FILE_NAME", testSentinel)
	ctx := registerFileKeysFixture(t)

	data := schemaEventData("T0701", nil, true)
	data["file_size"] = float64(99999)
	if status, err := registerOne(t, ctx, data); err != nil || status != http.StatusOK {
		t.Fatalf("status %d, err %v", status, err)
	}
	folder := testFolder + "T0701"
	if n, _ := countFileKeyStaging(t, ctx, folder); n != 1 {
		t.Errorf("file_key_staging rows for %q = %d, want 1: a spec file_size took the part-file skip", folder, n)
	}
	if n := countInputRegistry(t, ctx, folder); n != 1 {
		t.Errorf("input_registry rows for %q = %d, want 1", folder, n)
	}

	part := map[string]any{
		"client":      testClient,
		"vendor":      testOrg,
		"object_type": testObjectType,
		"file_key":    testFolder + "T0702/part-00000.parquet",
		"size":        int64(500),
	}
	action := &RegisterFileKeyAction{Action: "register_keys", Data: []map[string]any{part}, NoAutomatedLoad: true}
	if _, status, err := ctx.RegisterFileKeys(action, ""); err != nil || status != http.StatusOK {
		t.Fatalf("status %d, err %v", status, err)
	}
	if n := countRows(t, ctx, "input_registry"); n != 1 {
		t.Errorf("input_registry has %d rows, want 1: the S3 part file was registered", n)
	}
	if n, _ := countFileKeyStaging(t, ctx, part["file_key"].(string)); n != 0 {
		t.Errorf("the S3 part file was staged")
	}
}
