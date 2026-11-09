package datatable

// The guards and default cases of RegisterFileKeys and RegisterSchemaEvent found
// by the JM-035 review, JM-037's checked schema_provider_json, and JM-084's
// session registration order, and JM-089's domain keys of a load. The tests
// that write use the fixtures of register_file_key_action_test.go and
// register_file_input_source_test.go, and skip unless JETS_TEST_DSN is set; see
// the header of the first.

import (
	"context"
	"database/sql"
	"encoding/json"
	"net/http"
	"testing"
)

// registerOne calls RegisterFileKeys on one row, as the register-key lambda
// does, with no pipeline start.
func registerOne(t *testing.T, ctx *DataTableContext, data map[string]any) (int, error) {
	t.Helper()
	action := &RegisterFileKeyAction{
		Action:          "register_keys",
		Data:            []map[string]any{data},
		NoAutomatedLoad: true,
		IsSchemaEvent:   true,
	}
	_, status, err := ctx.RegisterFileKeys(action, "")
	return status, err
}

// JM-037: a schema_provider_json that is not a string is a 400 and registers
// nothing. It panicked with "interface conversion: interface {} is float64, not
// string" before. A null one is an absent one and registers.
func TestRegisterFileKeysSchemaProviderJsonNotAString(t *testing.T) {
	t.Setenv("JETS_SENTINEL_FILE_NAME", testSentinel)
	ctx := registerFileKeysFixture(t)

	data := schemaEventData("T0101", int64(0), true)
	data["schema_provider_json"] = float64(42)
	status, err := registerOne(t, ctx, data)
	if err == nil || status != http.StatusBadRequest {
		t.Fatalf("a numeric schema_provider_json: status %d, err %v; want 400 and an error", status, err)
	}
	if n := countInputRegistry(t, ctx, testFolder+"T0101"); n != 0 {
		t.Errorf("input_registry rows = %d, want 0", n)
	}

	data = schemaEventData("T0102", int64(0), true)
	data["schema_provider_json"] = nil
	status, err = registerOne(t, ctx, data)
	if err != nil || status != http.StatusOK {
		t.Fatalf("a null schema_provider_json: status %d, err %v; want 200", status, err)
	}
	if n := countInputRegistry(t, ctx, testFolder+"T0102"); n != 1 {
		t.Errorf("input_registry rows = %d, want 1", n)
	}
}

// JM-035 review: client, vendor, org and object_type are read as strings without
// a check in updateFileKeyComponentCase, so a non-string panicked there. It is a
// 400 now.
func TestRegisterFileKeysComponentNotAString(t *testing.T) {
	t.Setenv("JETS_SENTINEL_FILE_NAME", testSentinel)
	ctx := registerFileKeysFixture(t)
	for i, k := range []string{"client", "vendor", "org", "object_type"} {
		data := schemaEventData("T020"+string(rune('0'+i)), int64(0), true)
		data[k] = float64(7)
		status, err := registerOne(t, ctx, data)
		if err == nil || status != http.StatusBadRequest {
			t.Errorf("a numeric %s: status %d, err %v; want 400 and an error", k, status, err)
		}
	}
}

// JM-035 review: a request decoded from JSON, as the apiserver decodes
// register_keys, carries year, month and day as float64. They had no case and
// the row was registered under 1970-01-01.
func TestRegisterFileKeysDateFromJson(t *testing.T) {
	t.Setenv("JETS_SENTINEL_FILE_NAME", testSentinel)
	ctx := registerFileKeysFixture(t)
	body := `{"action":"register_keys","noAutomatedLoad":true,"isSchemaEvent":true,"data":[{
		"client":"` + testClient + `","vendor":"` + testOrg + `","object_type":"` + testObjectType + `",
		"file_key":"` + testFolder + `T0301/` + testSentinel + `","size":0,
		"year":2026,"month":3,"day":4}]}`
	var action RegisterFileKeyAction
	if err := json.Unmarshal([]byte(body), &action); err != nil {
		t.Fatal(err)
	}
	if _, ok := action.Data[0]["year"].(float64); !ok {
		t.Fatalf("fixture error: year decoded as %T, want float64", action.Data[0]["year"])
	}
	if _, status, err := ctx.RegisterFileKeys(&action, ""); err != nil || status != http.StatusOK {
		t.Fatalf("RegisterFileKeys: status %d, err %v", status, err)
	}
	var year, month, day int
	if err := ctx.Dbpool.QueryRow(context.Background(), `SELECT sp.year, sp.month, sp.day
		FROM jetsapi.input_registry ir JOIN jetsapi.source_period sp ON sp.key = ir.source_period_key
		WHERE ir.file_key = $1`, testFolder+"T0301").Scan(&year, &month, &day); err != nil {
		t.Fatalf("reading the registered source period: %v", err)
	}
	if year != 2026 || month != 3 || day != 4 {
		t.Errorf("registered under %04d-%02d-%02d, want 2026-03-04", year, month, day)
	}

	// A type ToInt cannot read is a 400, as a non-numeric string already was.
	data := schemaEventData("T0302", int64(0), true)
	data["month"] = true
	if status, err := registerOne(t, ctx, data); err == nil || status != http.StatusBadRequest {
		t.Errorf("a boolean month: status %d, err %v; want 400 and an error", status, err)
	}
}

// JM-035 review: request_id was declared above the row loop, so a row with none
// was registered with the request_id of the row before it.
func TestRegisterFileKeysRequestIdIsPerRow(t *testing.T) {
	t.Setenv("JETS_SENTINEL_FILE_NAME", testSentinel)
	ctx := registerFileKeysFixture(t)
	first := schemaEventData("T0401", int64(0), true)
	first["request_id"] = "req-0401"
	second := schemaEventData("T0402", int64(0), true)
	action := &RegisterFileKeyAction{
		Action:          "register_keys",
		Data:            []map[string]any{first, second},
		NoAutomatedLoad: true,
		IsSchemaEvent:   true,
	}
	if _, status, err := ctx.RegisterFileKeys(action, ""); err != nil || status != http.StatusOK {
		t.Fatalf("RegisterFileKeys: status %d, err %v", status, err)
	}
	requestIdOf := func(fileKey string) sql.NullString {
		var r sql.NullString
		if err := ctx.Dbpool.QueryRow(context.Background(),
			`SELECT request_id FROM jetsapi.input_registry WHERE file_key = $1`, fileKey).Scan(&r); err != nil {
			t.Fatalf("reading request_id of %s: %v", fileKey, err)
		}
		return r
	}
	if r := requestIdOf(testFolder + "T0401"); r.String != "req-0401" {
		t.Errorf("first row request_id = %+v, want req-0401", r)
	}
	if r := requestIdOf(testFolder + "T0402"); r.Valid {
		t.Errorf("second row request_id = %q, want null: it inherited the first row's", r.String)
	}
}

// JM-084: the file's session is in session_registry before any pipeline start
// begins, so a register-key lambda timing out while a start waits on the state
// machine lock leaves no input_registry row under an unregistered session.
func TestRegisterFileKeysRegistersTheSessionBeforeStartingPipelines(t *testing.T) {
	t.Setenv("JETS_SENTINEL_FILE_NAME", testSentinel)
	ctx := registerFileKeysFixture(t)
	type start struct {
		sessionId  string
		registered int
	}
	var starts []start
	startPipelinesOverride = func(c *DataTableContext, _, _ int, inputSessionId, _, _, _, _ string) error {
		var n int
		if err := c.Dbpool.QueryRow(context.Background(),
			`SELECT count(*) FROM jetsapi.session_registry WHERE session_id = $1`, inputSessionId).Scan(&n); err != nil {
			t.Errorf("reading session_registry: %v", err)
		}
		starts = append(starts, start{inputSessionId, n})
		return nil
	}
	t.Cleanup(func() { startPipelinesOverride = nil })

	action := &RegisterFileKeyAction{
		Action:        "register_keys",
		Data:          []map[string]any{schemaEventData("T0501", int64(0), true)},
		IsSchemaEvent: true,
	}
	if _, status, err := ctx.RegisterFileKeys(action, ""); err != nil || status != http.StatusOK {
		t.Fatalf("RegisterFileKeys: status %d, err %v", status, err)
	}
	if len(starts) != 1 {
		t.Fatalf("pipeline starts = %d, want 1 (one domain key)", len(starts))
	}
	if starts[0].registered != 1 {
		t.Errorf("session %s had %d session_registry rows when the pipeline start began, want 1",
			starts[0].sessionId, starts[0].registered)
	}
}

// JM-035 review: a nil schema event is an error. The register-key lambda decodes
// a list of events into []*SchemaProviderSpec, where a JSON null is a nil
// pointer, and RegisterSchemaEvent read its Type first.
func TestRegisterSchemaEventNilIsAnError(t *testing.T) {
	ctx := &DataTableContext{}
	if err := ctx.RegisterSchemaEvent(nil, nil, ""); err == nil {
		t.Errorf("a nil schema event registered; want an error")
	}
}

// JM-089: through the state machine the status-update lambda receives the cpipes
// env decoded from JSON, so $ORIGIN_DOMAIN_KEYS is a []any. Only a []string was
// read, so a load on the cluster was registered under its object type alone and
// not under each of the source's domain keys.
func TestALoadFromJsonRegistersEachDomainKey(t *testing.T) {
	pool := widenedTestPool(t, originSessionTables...)
	withNoManifestUpload(t)
	const origin = "1762500000101"
	ca, _ := newLoadStatusUpdate(t, pool, "domainkeys", "completed", []string{"completed"}, origin)
	// As startLoader puts it in the env: source_config.domain_keys, with the
	// special key RegisterFileInputSource skips.
	ca.CpipesEnv["$ORIGIN_DOMAIN_KEYS"] = []string{"claim", "jets:hashing_override", "claim_line"}
	buf, err := json.Marshal(ca.CpipesEnv)
	if err != nil {
		t.Fatal(err)
	}
	ca.CpipesEnv = nil
	if err := json.Unmarshal(buf, &ca.CpipesEnv); err != nil {
		t.Fatal(err)
	}
	if _, ok := ca.CpipesEnv["$ORIGIN_DOMAIN_KEYS"].([]any); !ok {
		t.Fatalf("fixture error: $ORIGIN_DOMAIN_KEYS decoded as %T, want []any", ca.CpipesEnv["$ORIGIN_DOMAIN_KEYS"])
	}
	if err := ca.CoordinateWork(); err != nil {
		t.Fatalf("CoordinateWork: %v", err)
	}
	files := readFileRegistrations(t, pool, origin)
	var got []string
	for _, f := range files {
		got = append(got, f.objectType)
	}
	if len(got) != 2 || got[0] != "claim" || got[1] != "claim_line" {
		t.Errorf("the load registered object types %v, want [claim claim_line]", got)
	}
}

func TestOriginDomainKeysOf(t *testing.T) {
	for _, c := range []struct {
		name    string
		keys    any
		want    []string
		wantErr bool
	}{
		{"in process", []string{"a", "b"}, []string{"a", "b"}, false},
		{"from json", []any{"a", "b"}, []string{"a", "b"}, false},
		{"absent", nil, []string{"ot"}, false},
		{"empty from json", []any{}, []string{"ot"}, false},
		{"empty in process", []string{}, []string{"ot"}, false},
		{"another type", "a", []string{"ot"}, false},
		{"a non-string element", []any{"a", 1.0}, nil, true},
	} {
		t.Run(c.name, func(t *testing.T) {
			got, err := originDomainKeysOf(c.keys, "ot")
			if (err != nil) != c.wantErr {
				t.Fatalf("err = %v, wantErr %v", err, c.wantErr)
			}
			if len(got) != len(c.want) {
				t.Fatalf("got %v, want %v", got, c.want)
			}
			for i := range got {
				if got[i] != c.want[i] {
					t.Fatalf("got %v, want %v", got, c.want)
				}
			}
		})
	}
	if _, err := originDomainKeysOf(nil, nil); err == nil {
		t.Errorf("no domain keys and no object type: want an error")
	}
}
