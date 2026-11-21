package wsfile

import (
	"context"
	"os"
	"strings"
	"testing"

	"github.com/artisoft-io/jetstore/jets/schema"
	"github.com/jackc/pgx/v5/pgxpool"
)

// The two lists getColumns returns must differ in exactly one way: an array
// column carries a `::text` cast in the select list and must not carry one in
// the INSERT column list. A single list served both until 2026-09-12, and every
// export of a table with an array column was an init script PostgreSQL refuses
// to parse.
func TestGetColumnsSeparatesSelectFromInsert(t *testing.T) {
	jetsSchema := map[string]schema.TableDefinition{
		"source_config": {
			TableName: "source_config",
			Columns: []schema.ColumnDefinition{
				{ColumnName: "key", DataType: "int"},
				{ColumnName: "object_type", DataType: "text"},
				{ColumnName: "domain_keys", DataType: "text", IsArray: true},
				{ColumnName: "user_email", DataType: "text"},
				{ColumnName: "last_update", DataType: "datetime"},
			},
		},
	}
	selectList, columnNames, err := getColumns(&jetsSchema, true, "source_config")
	if err != nil {
		t.Fatalf("getColumns: %v", err)
	}
	wantSelect := "object_type,domain_keys::text,user_email"
	wantColumns := "object_type,domain_keys,user_email"
	if got := strings.Join(selectList, ","); got != wantSelect {
		t.Errorf("select list = %q, want %q", got, wantSelect)
	}
	if got := strings.Join(columnNames, ","); got != wantColumns {
		t.Errorf("column names = %q, want %q", got, wantColumns)
	}
	// skipKeyColumn false keeps `key`; `last_update` is never exported.
	selectList, columnNames, err = getColumns(&jetsSchema, false, "source_config")
	if err != nil {
		t.Fatalf("getColumns: %v", err)
	}
	if got := strings.Join(columnNames, ","); got != "key,object_type,domain_keys,user_email" {
		t.Errorf("column names = %q", got)
	}
	if strings.Contains(strings.Join(columnNames, ","), "::") {
		t.Errorf("an INSERT column list must carry no cast: %q", columnNames)
	}
	if !strings.Contains(strings.Join(selectList, ","), "domain_keys::text") {
		t.Errorf("the select list lost its cast: %q", selectList)
	}
}

func TestGetColumnsUnknownTable(t *testing.T) {
	jetsSchema := map[string]schema.TableDefinition{}
	if _, _, err := getColumns(&jetsSchema, true, "nope"); err == nil {
		t.Error("want an error for a table the schema does not define")
	}
}

// A source configuration's feed profile travels in the client's workspace
// initialisation script, and the script loads it back. The profile carries a
// quote, so the literal's escaping is exercised too. Needs JETS_TEST_DSN, a
// PostgreSQL it may write to (see jets/datatable/scratch_postgres_test.go).
func TestExportedFeedProfileLoadsBack(t *testing.T) {
	dsn := os.Getenv("JETS_TEST_DSN")
	if dsn == "" {
		t.Skip("JETS_TEST_DSN is not set")
	}
	ctx := context.Background()
	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()
	t.Setenv("JETS_SCHEMA_FILE", "../../jets_schema.json")
	jetsSchema, err := getJetsSchema()
	if err != nil {
		t.Fatal(err)
	}
	def := (*jetsSchema)["source_config"]
	if _, err = pool.Exec(ctx, "CREATE SCHEMA IF NOT EXISTS jetsapi"); err != nil {
		t.Fatal(err)
	}
	if err = def.UpdateTableSchema(pool, true); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { pool.Exec(context.Background(), "DROP TABLE IF EXISTS jetsapi.source_config") })

	const profile = `{"declared_at": "2026-11-21", "declared_by": "o'brien@example.com", "period_type": "week_period"}`
	if _, err = pool.Exec(ctx, `INSERT INTO jetsapi.source_config
		(object_type, client, org, table_name, user_email, feed_profile_json)
		VALUES ('hc:Claim', 'c', 'o', 'c_o_claim', 'u@example.com', $1::jsonb),
		       ('hc:Eligibility', 'c', 'o', 'c_o_elig', 'u@example.com', NULL)`, profile); err != nil {
		t.Fatal(err)
	}
	var buf strings.Builder
	if err = writeTable(pool, jetsSchema, "source_config", "client", "c", "key", true, false, &buf); err != nil {
		t.Fatal(err)
	}
	script := buf.String()
	if !strings.Contains(script, `'{"declared_at": "2026-11-21", "declared_by": "o''brien@example.com"`) {
		t.Fatalf("the export does not carry the profile as a literal:\n%s", script)
	}
	// The script deletes the client's rows and inserts them again.
	if _, err = pool.Exec(ctx, script); err != nil {
		t.Fatalf("the exported script does not load: %v\n%s", err, script)
	}
	var got, none *string
	if err = pool.QueryRow(ctx, `SELECT
		(SELECT feed_profile_json::text FROM jetsapi.source_config WHERE table_name = 'c_o_claim'),
		(SELECT feed_profile_json::text FROM jetsapi.source_config WHERE table_name = 'c_o_elig')`).Scan(&got, &none); err != nil {
		t.Fatal(err)
	}
	if got == nil || *got != profile || none != nil {
		t.Errorf("loaded back %v and %v, want %s and NULL", got, none, profile)
	}
}
