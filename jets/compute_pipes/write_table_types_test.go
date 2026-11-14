package compute_pipes

// The output table column types against the closed list of schema.ToPgType. The
// DB half is skipped unless JETS_TEST_DSN names a throwaway server, for example:
//
//	docker run -d --rm --name cp-pg -e POSTGRES_PASSWORD=pw -p 5497:5432 postgres:16-alpine
//	JETS_TEST_DSN=postgres://postgres:pw@localhost:5497/postgres go test -count=1 -run OutputTable ./jets/compute_pipes/

import (
	"context"
	"net/url"
	"os"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

const injectedRdfType = "text); UPDATE public.victim SET v = 'injected'; CREATE TABLE public.z (y text"

func TestOutputTableColumnTypesAreValidated(t *testing.T) {
	cfg := &ComputePipesConfig{OutputTables: []*TableSpec{{
		Key: "out", Name: "public.out",
		Columns: []TableColumnSpec{
			{Name: "session_id", RdfType: "text"},
			{Name: "amount", RdfType: "double precision"},
			{Name: "codes", RdfType: "text", IsArray: true},
			{Name: "n", RdfType: "int"},
		},
	}}}
	startup := &CpipesStartup{}
	if err := startup.ValidatePipeSpecConfig(cfg, nil); err != nil {
		t.Fatalf("a configuration naming known types: %v", err)
	}
	for _, rdfType := range []string{injectedRdfType, "text[]", "numeric", "TEXT"} {
		cfg.OutputTables[0].Columns[1].RdfType = rdfType
		err := startup.ValidatePipeSpecConfig(cfg, nil)
		if err == nil || !strings.Contains(err.Error(), `output table "out", column "amount"`) {
			t.Errorf("rdf_type %q: %v; want it refused, naming the table and the column", rdfType, err)
		}
	}
	// A column with no rdf_type is left to the table creation, which needs one.
	cfg.OutputTables[0].Columns[1].RdfType = ""
	if err := startup.ValidatePipeSpecConfig(cfg, nil); err != nil {
		t.Errorf("a column with no rdf_type: %v", err)
	}
}

func outputTableDB(t *testing.T) *pgxpool.Pool {
	t.Helper()
	dsn := os.Getenv("JETS_TEST_DSN")
	if dsn == "" {
		t.Skip("JETS_TEST_DSN is not set; see the header of this file")
	}
	ctx := context.Background()
	admin, err := pgxpool.New(ctx, dsn)
	if err != nil {
		t.Fatal(err)
	}
	defer admin.Close()
	const name = "cp_output_table_types"
	if _, err = admin.Exec(ctx, "DROP DATABASE IF EXISTS "+name+" WITH (FORCE)"); err != nil {
		t.Fatal(err)
	}
	if _, err = admin.Exec(ctx, "CREATE DATABASE "+name); err != nil {
		t.Fatal(err)
	}
	u, err := url.Parse(dsn)
	if err != nil {
		t.Fatal(err)
	}
	u.Path = "/" + name
	pool, err := pgxpool.New(ctx, u.String())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(pool.Close)
	return pool
}

func TestOutputTableInjectedTypeRunsNothing(t *testing.T) {
	pool := outputTableDB(t)
	ctx := context.Background()
	for _, s := range []string{
		"CREATE TABLE public.victim (v text)",
		"INSERT INTO public.victim VALUES ('original')",
		"CREATE TABLE public.out (a text)",
	} {
		if _, err := pool.Exec(ctx, s); err != nil {
			t.Fatal(err)
		}
	}
	spec := &TableSpec{Key: "out", Name: "public.out", Columns: []TableColumnSpec{{Name: "a", RdfType: injectedRdfType}}}
	if err := CreateOutputTable(pool, pgx.Identifier{"public", "out"}, spec); err == nil {
		t.Fatal("CreateOutputTable accepted the type")
	}
	if err := PrepareOutoutTable(pool, pgx.Identifier{"public", "new_out"}, spec); err == nil {
		t.Fatal("PrepareOutoutTable accepted the type")
	}
	var v string
	var tables int
	if err := pool.QueryRow(ctx, "SELECT v FROM public.victim").Scan(&v); err != nil {
		t.Fatal(err)
	}
	if err := pool.QueryRow(ctx, "SELECT count(*) FROM pg_tables WHERE schemaname = 'public' AND tablename IN ('out', 'new_out', 'z')").Scan(&tables); err != nil {
		t.Fatal(err)
	}
	if v != "original" || tables != 1 {
		t.Errorf("victim.v = %q, tables out, new_out and z = %d; want original, and out alone (not dropped)", v, tables)
	}
}

// check_schema_changed adds the columns a table lacks, on a table with an index
// (the session_id one), and as_array makes an array column.
func TestOutputTableSchemaChangeAddsColumns(t *testing.T) {
	pool := outputTableDB(t)
	ctx := context.Background()
	id := pgx.Identifier{"public", "out"}
	spec := &TableSpec{Key: "out", Name: "public.out", Columns: []TableColumnSpec{
		{Name: "session_id", RdfType: "text"},
		{Name: "a", RdfType: "text"},
		{Name: "codes", RdfType: "text", IsArray: true},
	}}
	if err := PrepareOutoutTable(pool, id, spec); err != nil {
		t.Fatal(err)
	}
	spec.CheckSchemaChanged = true
	spec.Columns = append(spec.Columns,
		TableColumnSpec{Name: "n", RdfType: "int"},
		TableColumnSpec{Name: "amounts", RdfType: "double", IsArray: true})
	if err := PrepareOutoutTable(pool, id, spec); err != nil {
		t.Fatal(err)
	}
	rows, err := pool.Query(ctx, `SELECT column_name, data_type, udt_name FROM information_schema.columns
		WHERE table_schema = 'public' AND table_name = 'out' ORDER BY ordinal_position`)
	if err != nil {
		t.Fatal(err)
	}
	var got []string
	for rows.Next() {
		var c, dt, udt string
		if err = rows.Scan(&c, &dt, &udt); err != nil {
			t.Fatal(err)
		}
		got = append(got, c+" "+udt)
	}
	want := "session_id text, a text, codes _text, n int4, amounts _float8"
	if strings.Join(got, ", ") != want {
		t.Errorf("columns: %s; want %s", strings.Join(got, ", "), want)
	}
	// A second pass with nothing new is a no-op.
	if err := PrepareOutoutTable(pool, id, spec); err != nil {
		t.Fatal(err)
	}
}
