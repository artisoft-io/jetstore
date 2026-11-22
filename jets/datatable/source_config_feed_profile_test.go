package datatable

import (
	"context"
	"strconv"
	"strings"
	"testing"

	"github.com/artisoft-io/jetstore/jets/agentic/feedprofile"
	"github.com/jackc/pgx/v5/pgxpool"
)

const testFeedProfile = `{"period_type":"month_period","overdue_after_days":10,` +
	`"claim_set":{"table":"hc:Claim","key_columns":["hc:claim_id"],"service_date_column":"hc:service_date","member_column":"hc:member_id"},` +
	`"declared_by":"o'brien@example.com","declared_at":"2026-11-21"}`

func TestNormaliseFeedProfile(t *testing.T) {
	cases := []struct {
		name string
		in   any
		set  bool
		want any
		err  string
	}{
		{"absent", nil, false, nil, ""},
		{"nil", nil, true, nil, ""},
		{"blank clears", "  ", true, "null", ""},
		{"null clears", "null", true, "null", ""},
		{"text", testFeedProfile, true, testFeedProfile, ""},
		{"object", map[string]any{"declared_by": "a", "declared_at": "2026-01-01"}, true,
			`{"declared_at":"2026-01-01","declared_by":"a"}`, ""},
		{"not json", "{", true, nil, "not a valid document"},
		{"invalid", `{"declared_by":"a","declared_at":"2026-01-01","period_type":"year"}`, true, nil, "/period_type"},
		{"selected row array", []any{testFeedProfile}, true, nil, "must be JSON text or an object"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			row := map[string]any{}
			if c.set {
				row["feed_profile_json"] = c.in
			}
			err := normaliseFeedProfile(row)
			if c.err != "" {
				if err == nil || !strings.Contains(err.Error(), c.err) {
					t.Fatalf("err = %v, want one naming %q", err, c.err)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if got := row["feed_profile_json"]; got != c.want {
				t.Errorf("bound %v, want %v", got, c.want)
			}
		})
	}
}

// saveSourceConfig binds a row the way InsertRows does for the source_config
// statements, hooks included, and runs the statement. It returns the hooks'
// refusal, or the database's error.
func saveSourceConfig(t *testing.T, pool *pgxpool.Pool, table string, row map[string]any) (int, error) {
	t.Helper()
	stmt := sqlInsertStmts[table]
	if err := populateDomainKeys(row); err != nil {
		return 0, err
	}
	if err := normaliseFeedProfile(row); err != nil {
		return 0, err
	}
	args := make([]any, len(stmt.ColumnKeys))
	for i, k := range stmt.ColumnKeys {
		args[i] = row[k]
	}
	var key int
	var err error
	if strings.Contains(stmt.Stmt, "RETURNING key") {
		err = pool.QueryRow(context.Background(), stmt.Stmt, args...).Scan(&key)
	} else {
		_, err = pool.Exec(context.Background(), stmt.Stmt, args...)
	}
	return key, err
}

func sourceConfigRow(objectType string) map[string]any {
	return map[string]any{
		"object_type": objectType, "client": "c", "org": "o", "table_name": "c_o_" + objectType,
		"input_format": "csv", "is_part_files": 0, "input_format_data_json": "",
		"user_email": "u@example.com", "is_cumulative": 1,
	}
}

// storedProfile is the stored profile, as text, or "NULL".
func storedProfile(t *testing.T, pool *pgxpool.Pool, key int) string {
	t.Helper()
	var text *string
	if err := pool.QueryRow(context.Background(),
		`SELECT feed_profile_json::text FROM jetsapi.source_config WHERE key = $1`, key).Scan(&text); err != nil {
		t.Fatal(err)
	}
	if text == nil {
		return "NULL"
	}
	return *text
}

// The column is added to a source_config that already exists and holds rows,
// by the migration's update path, which is what every deployment takes; and
// the migration is run again, as the next deployment will. The rows survive,
// unprofiled, and the profile is then written and read through the statements
// the Source Configuration flow posts to.
func TestFeedProfileColumnMigratesAndSaves(t *testing.T) {
	pool := scratchDatabase(t, "feed_profile_test")
	ctx := context.Background()
	if _, err := pool.Exec(ctx, "CREATE SCHEMA IF NOT EXISTS jetsapi"); err != nil {
		t.Fatal(err)
	}
	shipped := *jetsSchemaTables(t)["source_config"]

	// source_config as it stood before the column.
	before := shipped
	before.Columns = nil
	for _, c := range shipped.Columns {
		if c.ColumnName != "feed_profile_json" {
			before.Columns = append(before.Columns, c)
		}
	}
	if len(before.Columns) != len(shipped.Columns)-1 {
		t.Fatal("jets_schema.json's source_config has no feed_profile_json column")
	}
	if err := before.UpdateTableSchema(pool, true); err != nil {
		t.Fatalf("creating the earlier source_config: %v", err)
	}
	t.Cleanup(func() { pool.Exec(ctx, "DROP TABLE IF EXISTS jetsapi.source_config") })
	if _, err := pool.Exec(ctx, `INSERT INTO jetsapi.source_config
		(object_type, client, org, table_name, user_email, is_cumulative)
		VALUES ('hc:Eligibility', 'c', 'o', 'c_o_hc:Eligibility', 'u@example.com', 1)`); err != nil {
		t.Fatal(err)
	}

	// Load runs against a database that has no such column, and reads every
	// feed as unprofiled rather than failing.
	profiled, unprofiled, err := feedprofile.Load(ctx, pool, "c")
	if err != nil || len(profiled) != 0 || len(unprofiled) != 1 || !unprofiled[0].Cumulative {
		t.Fatalf("Load before the column: %v %v %v", profiled, unprofiled, err)
	}

	for pass := 1; pass <= 2; pass++ {
		if err := shipped.UpdateTableSchema(pool, false); err != nil {
			t.Fatalf("migration pass %d: %v", pass, err)
		}
	}
	var dataType string
	if err := pool.QueryRow(ctx, `SELECT data_type FROM information_schema.columns
		WHERE table_schema = 'jetsapi' AND table_name = 'source_config' AND column_name = 'feed_profile_json'`).
		Scan(&dataType); err != nil || dataType != "jsonb" {
		t.Fatalf("feed_profile_json is %q (%v), want jsonb", dataType, err)
	}
	var n, cumulative int
	var fp *string
	if err := pool.QueryRow(ctx, `SELECT count(*), max(is_cumulative), max(feed_profile_json::text)
		FROM jetsapi.source_config`).Scan(&n, &cumulative, &fp); err != nil ||
		n != 1 || cumulative != 1 || fp != nil {
		t.Fatalf("after migrating: %d rows, is_cumulative %d, profile %v (%v)", n, cumulative, fp, err)
	}

	// Insert with a profile.
	row := sourceConfigRow("hc:Claim")
	row["feed_profile_json"] = testFeedProfile
	key, err := saveSourceConfig(t, pool, "source_config", row)
	if err != nil {
		t.Fatal(err)
	}
	if got := storedProfile(t, pool, key); !strings.Contains(got, `"declared_by": "o'brien@example.com"`) {
		t.Fatalf("stored %s", got)
	}

	// Update without the key keeps it, as is_cumulative is kept.
	row = sourceConfigRow("hc:Claim")
	row["key"], row["is_cumulative"] = key, nil
	if _, err := saveSourceConfig(t, pool, "update/source_config", row); err != nil {
		t.Fatal(err)
	}
	if got := storedProfile(t, pool, key); got == "NULL" {
		t.Fatal("an update that does not send the profile cleared it")
	}

	// Load reads it back with *pgxpool.Pool as the Querier.
	profiled, unprofiled, err = feedprofile.Load(ctx, pool, "c")
	if err != nil || len(profiled) != 1 || len(unprofiled) != 1 {
		t.Fatalf("Load: %v %v %v", profiled, unprofiled, err)
	}
	if f := profiled[0]; f.TableName != "c_o_hc:Claim" || !f.Cumulative || f.Profile.ClaimSet.MemberColumn != "hc:member_id" {
		t.Errorf("profiled feed %+v", f)
	}

	// An invalid profile is refused before the statement, and the stored one stays.
	row = sourceConfigRow("hc:Claim")
	row["key"], row["feed_profile_json"] = key, `{"declared_by":"a","declared_at":"2026-01-01","overdue_after_days":-2}`
	if _, err := saveSourceConfig(t, pool, "update/source_config", row); err == nil ||
		!strings.Contains(err.Error(), "/overdue_after_days") {
		t.Fatalf("an invalid profile: err %v", err)
	}
	if got := storedProfile(t, pool, key); got == "NULL" {
		t.Fatal("a refused update changed the stored profile")
	}

	// An emptied field clears it.
	row = sourceConfigRow("hc:Claim")
	row["key"], row["feed_profile_json"] = key, ""
	if _, err := saveSourceConfig(t, pool, "update/source_config", row); err != nil {
		t.Fatal(err)
	}
	if got := storedProfile(t, pool, key); got != "NULL" {
		t.Fatalf("an emptied profile stored %s, want NULL", got)
	}

	// An insert with an empty profile stores NULL, not the JSON null.
	row = sourceConfigRow("hc:Pharmacy")
	row["feed_profile_json"] = ""
	key, err = saveSourceConfig(t, pool, "source_config", row)
	if err != nil {
		t.Fatal(err)
	}
	if got := storedProfile(t, pool, key); got != "NULL" {
		t.Fatalf("an empty profile on insert stored %s, want NULL", got)
	}
}

// The statement's ColumnKeys and its placeholders agree, and the profile is
// the parameter the NULLIF reads, in both statements.
func TestSourceConfigStatementsBindTheProfile(t *testing.T) {
	for _, table := range []string{"source_config", "update/source_config"} {
		stmt := sqlInsertStmts[table]
		idx := -1
		for i, k := range stmt.ColumnKeys {
			if k == "feed_profile_json" {
				idx = i
			}
		}
		if idx < 0 {
			t.Fatalf("%s: feed_profile_json is not bound", table)
		}
		if p := "$" + strconv.Itoa(idx+1) + "::jsonb"; !strings.Contains(stmt.Stmt, p) {
			t.Errorf("%s: feed_profile_json is parameter %d and the statement has no %s", table, idx+1, p)
		}
		if last := "$" + strconv.Itoa(len(stmt.ColumnKeys)); !strings.Contains(stmt.Stmt, last) ||
			strings.Contains(stmt.Stmt, "$"+strconv.Itoa(len(stmt.ColumnKeys)+1)) {
			t.Errorf("%s: %d column keys and the placeholders disagree", table, len(stmt.ColumnKeys))
		}
	}
}
