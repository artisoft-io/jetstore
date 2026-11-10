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
