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
	"reflect"
	"strings"
	"testing"

	"github.com/artisoft-io/jetstore/jets/compute_pipes"
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

// A source_config naming a domain key twice registers it once. The second
// insert met input_registry's unique constraint, ON CONFLICT DO NOTHING returned
// no row, and the request failed with a 500 after the first domain key was
// registered and its pipelines started.
func TestRegisterFileKeysDuplicateDomainKeyRegistersOnce(t *testing.T) {
	ctx := registerFileKeysFixture(t)
	seedSource(t, ctx, "Claims", 0, "{Claims,ClaimLine,Claims}")
	data := fileData("Claims", "claims.csv")
	if status, err := registerOne(t, ctx, data); err != nil || status != http.StatusOK {
		t.Fatalf("status %d, err %v; want 200", status, err)
	}
	rows, err := ctx.Dbpool.Query(context.Background(),
		`SELECT object_type FROM jetsapi.input_registry WHERE file_key = $1 ORDER BY key`, data["file_key"])
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	var got []string
	for rows.Next() {
		var ot string
		if err := rows.Scan(&ot); err != nil {
			t.Fatal(err)
		}
		got = append(got, ot)
	}
	if len(got) != 2 || got[0] != "Claims" || got[1] != "ClaimLine" {
		t.Errorf("input_registry object types %v, want [Claims ClaimLine]", got)
	}
}

// The result says how many rows registered input and why each other row did
// not. It was an empty map with a 200 whatever happened, so a request that
// registered nothing could not be told from one that registered everything.
func TestRegisterFileKeysReportsWhatItRegistered(t *testing.T) {
	t.Setenv("JETS_SENTINEL_FILE_NAME", testSentinel)
	ctx := registerFileKeysFixture(t)
	seedSource(t, ctx, "Claims", 0, "{Claims}")

	noFileKey := fileData("Claims", "x.csv")
	delete(noFileKey, "file_key")
	partFile := map[string]any{"client": testClient, "vendor": testOrg, "object_type": testObjectType,
		"file_key": testFolder + "T0801/part-00000.parquet", "size": int64(500)}
	notTheSentinel := map[string]any{"client": testClient, "vendor": testOrg, "object_type": testObjectType,
		"file_key": testFolder + "T0802/_DONE", "size": int64(0)}
	unknown := fileData("Unknown", "u.csv")
	action := &RegisterFileKeyAction{
		Action: "register_keys",
		Data: []map[string]any{fileData("Claims", "claims.csv"), noFileKey, partFile,
			notTheSentinel, unknown},
		NoAutomatedLoad: true,
	}
	res, status, err := ctx.RegisterFileKeys(action, "")
	if err != nil || status != http.StatusOK {
		t.Fatalf("status %d, err %v; want 200", status, err)
	}
	if n, _ := (*res)["registered"].(int); n != 1 {
		t.Errorf("registered = %v, want 1", (*res)["registered"])
	}
	skipped, _ := (*res)["skipped"].([]fileKeySkip)
	want := map[int]string{1: skipNoFileKey, 2: skipPartFile, 3: skipSentinelName, 4: skipNoSourceConfig}
	if len(skipped) != len(want) {
		t.Fatalf("skipped = %+v, want rows %v", (*res)["skipped"], want)
	}
	for _, s := range skipped {
		if want[s.Row] != s.Reason {
			t.Errorf("row %d skipped as %q, want %q", s.Row, s.Reason, want[s.Row])
		}
	}
	if n, _ := (*res)["skipped_count"].(int); n != len(want) {
		t.Errorf("skipped_count = %v, want %d", (*res)["skipped_count"], len(want))
	}
	// A file with no source_config is staged, so it shows among the files that
	// arrived, and nothing else: no session is reserved for a file that
	// registers no input.
	if n, _ := countFileKeyStaging(t, ctx, unknown["file_key"].(string)); n != 1 {
		t.Errorf("the file with no source_config was not staged")
	}
	if n := countRows(t, ctx, "session_reservation"); n != 1 {
		t.Errorf("session_reservation has %d rows, want 1: a session was reserved for a file with no input", n)
	}
	if n := countRows(t, ctx, "session_registry"); n != 1 {
		t.Errorf("session_registry has %d rows, want 1", n)
	}
}

// A schema event that registers no input is an error. RegisterSchemaEvent
// returned nil, so a schema event for an unknown source config, or with no
// file_key, was reported as handled.
func TestRegisterSchemaEventThatRegistersNothingIsAnError(t *testing.T) {
	ctx := registerFileKeysFixture(t)
	seedSource(t, ctx, "Claims", 0, "{Claims}")
	var starts int
	startPipelinesOverride = func(*DataTableContext, int, int, string, string, string, string, string, bool) error {
		starts++
		return nil
	}
	t.Cleanup(func() { startPipelinesOverride = nil })

	event := func(objectType, fileKey string) *compute_pipes.SchemaProviderSpec {
		e := &compute_pipes.SchemaProviderSpec{Type: "default", Client: testClient, Vendor: testOrg,
			ObjectType: objectType}
		e.FileKey = fileKey
		return e
	}
	if err := ctx.RegisterSchemaEvent(ctx.Dbpool, event("Claims", "in/claims.csv"), ""); err != nil {
		t.Fatalf("a schema event for a known source config: %v", err)
	}
	if starts != 1 {
		t.Errorf("pipeline starts = %d, want 1", starts)
	}
	err := ctx.RegisterSchemaEvent(ctx.Dbpool, event("Unknown", "in/unknown.csv"), "")
	if err == nil || !strings.Contains(err.Error(), skipNoSourceConfig) {
		t.Errorf("a schema event with no source config: err %v, want one naming %s", err, skipNoSourceConfig)
	}
	err = ctx.RegisterSchemaEvent(ctx.Dbpool, event("Claims", ""), "")
	if err == nil || !strings.Contains(err.Error(), skipNoFileKey) {
		t.Errorf("a schema event with no file_key: err %v, want one naming %s", err, skipNoFileKey)
	}
}

// Every key RegisterFileKeysReads names changes what a row reads as, and the
// keys probed here that it does not name change nothing. The first half is the
// one that matters: a key the set names and the reader drops would hide the
// very misspelling the register-key lambda warns of.
func TestRegisterFileKeysReadsWhatItSays(t *testing.T) {
	value := map[string]any{
		"year": 2000, "month": 2, "day": 3, "size": int64(5), "file_size": int64(5),
	}
	empty, err := readRegisterFileKeyRow(0, map[string]any{})
	if err != nil {
		t.Fatal(err)
	}
	for k := range registerFileKeysReads {
		v, ok := value[k]
		if !ok {
			v = "x"
		}
		r, err := readRegisterFileKeyRow(0, map[string]any{k: v})
		if err != nil {
			t.Fatalf("%s: %v", k, err)
		}
		if reflect.DeepEqual(r, empty) {
			t.Errorf("RegisterFileKeysReads(%q) is true and a row reads the same without it", k)
		}
	}
	for _, k := range []string{"processing_ticket", "schema_provider_jsn", "domain_keys", "env"} {
		if RegisterFileKeysReads(k) {
			t.Errorf("RegisterFileKeysReads(%q) is true", k)
		}
		r, err := readRegisterFileKeyRow(0, map[string]any{k: "x"})
		if err != nil {
			t.Fatalf("%s: %v", k, err)
		}
		if !reflect.DeepEqual(r, empty) {
			t.Errorf("%q is not in the set and a row reads differently with it", k)
		}
	}
}
