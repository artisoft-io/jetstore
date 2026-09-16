package datatable

// The first test RegisterFileKeys has ever had, and the first test in package
// datatable. It exists for two branches of that function that no schema-event
// producer in any deployment has ever taken: the part-file gate's int64
// assertion on file_size, and the string assertion on file_key one line above
// the source_config query.
//
// The claims rest on PostgreSQL and pgx rather than on the code, so they are
// worth running rather than reading. What a nil interface value does to an
// unchecked type assertion is Go semantics and provable at a desk; what
// RegisterFileKeys does around it is not — it inserts source_period, reads
// source_config, writes file_key_staging under a unique constraint, reserves a
// session and writes input_registry, and the guards are only interesting if the
// row still lands after them. So the test calls the real function against a real
// PostgreSQL rather than asserting on a refactored fragment of it.
//
// It skips unless JETS_TEST_DSN names a PostgreSQL to write to. It creates its
// own tables in schema jetsapi from jets/jets_schema.json and drops them again,
// so point it at a scratch database and not at a jetsapi you care about. On the
// machine that wrote this:
//
//	docker run --rm -d --name thf-ae-pg -e POSTGRES_PASSWORD=postgres \
//	  -e POSTGRES_DB=jetstest -p 5439:5432 postgres:16-alpine
//	JETS_TEST_DSN='postgres://postgres:postgres@localhost:5439/jetstest' \
//	  go test ./jets/datatable/ -count=1 -run TestRegisterFileKeys -v
//
// It needs no AWS credentials and reaches no S3. RegisterFileKeys lists the
// folder to compute its size and tolerates its own failure there, logging a
// warning and leaving file_size as the caller supplied it, so the listing fails
// and the row is inserted anyway. The one assertion that depends on this is
// guarded by awsi.JetStoreBucket() being empty, which it is in a checkout.
//
// NoAutomatedLoad is true throughout, which stops the function before
// StartPipelinesForInputRegistryV2 and is what keeps the fixture to nine tables
// rather than the whole pipeline-start world.
//
// Measured against PostgreSQL 16.15 and pgx v5.10.0 on 2026-09-16.

import (
	"context"
	"encoding/json"
	"net/http"
	"os"
	"testing"

	"github.com/artisoft-io/jetstore/jets/awsi"
	"github.com/artisoft-io/jetstore/jets/schema"
	"github.com/jackc/pgx/v5/pgxpool"
)

// The tables RegisterFileKeys touches with NoAutomatedLoad set: the three case
// registries updateFileKeyComponentCase reads, source_period, source_config,
// file_key_staging, session_reservation, input_registry, and session_registry
// for the final RegisterSession. The last one is the only one whose absence is
// merely logged; it is created so that a warning in the output means something.
var registerFileKeysTables = []string{
	"client_registry",
	"client_org_registry",
	"object_type_registry",
	"source_period",
	"source_config",
	"file_key_staging",
	"session_reservation",
	"input_registry",
	"session_registry",
}

// The multi-part source the three qc_* pipelines run against, as
// cgt_workspace_init_db.sql registers it: is_part_files = 1, one domain key.
const (
	testClient     = "CGT"
	testOrg        = "Parquet_Parts"
	testObjectType = "Eligibility"
	testTableName  = "cgt_parquet_parts_eligibility"
	testFolder     = "client=CGT/org=Parquet_Parts/object_type=Eligibility/processing_ticket="
	testSentinel   = "_COMPLETE"
)

// registerFileKeysFixture connects, recreates the nine tables from the
// repository's own schema definition rather than from hand-written DDL — so the
// unique constraint file_key_staging_unique_cstraintv3 the insert names is the
// one the deployment has — and seeds the registry rows.
func registerFileKeysFixture(t *testing.T) *DataTableContext {
	t.Helper()
	dsn := os.Getenv("JETS_TEST_DSN")
	if dsn == "" {
		t.Skip("JETS_TEST_DSN is not set; see the header of this file")
	}
	// The folder listing is expected to fail and its failure is tolerated, but
	// without this the AWS SDK spends five seconds asking EC2 IMDS for a role it
	// will not find. The listing then fails on the empty bucket name instead,
	// which is a client-side validation error and reaches no network at all.
	t.Setenv("AWS_EC2_METADATA_DISABLED", "true")
	ctx := context.Background()
	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		t.Fatalf("connecting: %v", err)
	}
	t.Cleanup(pool.Close)

	defs := make(map[string]*schema.TableDefinition)
	buf, err := os.ReadFile("../jets_schema.json")
	if err != nil {
		t.Fatalf("reading ../jets_schema.json: %v", err)
	}
	var schemaDef []schema.TableDefinition
	if err = json.Unmarshal(buf, &schemaDef); err != nil {
		t.Fatalf("decoding ../jets_schema.json: %v", err)
	}
	for i := range schemaDef {
		defs[schemaDef[i].TableName] = &schemaDef[i]
	}
	drop := func() {
		for _, name := range registerFileKeysTables {
			if _, err := pool.Exec(ctx, "DROP TABLE IF EXISTS jetsapi."+name+" CASCADE"); err != nil {
				t.Logf("warning, while dropping jetsapi.%s: %v", name, err)
			}
		}
	}
	for _, name := range registerFileKeysTables {
		def, ok := defs[name]
		if !ok {
			t.Fatalf("jets_schema.json carries no definition for jetsapi.%s", name)
		}
		// dropExisting, so the fixture owns the table whatever was there before
		if err = def.UpdateTableSchema(pool, true); err != nil {
			t.Fatalf("creating jetsapi.%s: %v", name, err)
		}
	}
	t.Cleanup(drop)

	seed := []string{
		`INSERT INTO jetsapi.client_registry (client) VALUES ('` + testClient + `')`,
		`INSERT INTO jetsapi.client_org_registry (client, org) VALUES ('` + testClient + `', '` + testOrg + `')`,
		`INSERT INTO jetsapi.object_type_registry (object_type, entity_rdf_type)
			VALUES ('` + testObjectType + `', 'cgt:Eligibility')`,
		`INSERT INTO jetsapi.source_config
			(client, org, object_type, table_name, domain_keys, input_format, is_part_files, user_email)
			VALUES ('` + testClient + `', '` + testOrg + `', '` + testObjectType + `',
				'` + testTableName + `', '{` + testObjectType + `}', 'parquet_select', 1, 'system')`,
	}
	for _, stmt := range seed {
		if _, err = pool.Exec(ctx, stmt); err != nil {
			t.Fatalf("seeding: %v\n%s", err, stmt)
		}
	}
	return &DataTableContext{Dbpool: pool}
}

// schemaEventData builds the Data map for one file key in the shape the
// filtered-path lambda hands over: the sentinel key untouched, the components
// the key carries, and schema_provider_json. Whether a size entry is present is
// the whole subject of this test, so the caller decides.
func schemaEventData(ticket string, size any, withFileKey bool) map[string]any {
	data := map[string]any{
		"client":               testClient,
		"vendor":               testOrg,
		"object_type":          testObjectType,
		"year":                 1970,
		"month":                1,
		"day":                  1,
		"schema_provider_json": `{"key":"_main_input_","type":"default","client":"CGT"}`,
	}
	if withFileKey {
		data["file_key"] = testFolder + ticket + "/" + testSentinel
	}
	if size != nil {
		data["size"] = size
	}
	return data
}

func countFileKeyStaging(t *testing.T, ctx *DataTableContext, fileKey string) (int, int64) {
	t.Helper()
	var n int
	var size int64
	err := ctx.Dbpool.QueryRow(context.Background(),
		`SELECT count(*), coalesce(max(file_size), 0) FROM jetsapi.file_key_staging WHERE file_key = $1`,
		fileKey).Scan(&n, &size)
	if err != nil {
		t.Fatalf("counting file_key_staging: %v", err)
	}
	return n, size
}

func countInputRegistry(t *testing.T, ctx *DataTableContext, fileKey string) int {
	t.Helper()
	var n int
	err := ctx.Dbpool.QueryRow(context.Background(),
		`SELECT count(*) FROM jetsapi.input_registry WHERE file_key = $1`, fileKey).Scan(&n)
	if err != nil {
		t.Fatalf("counting input_registry: %v", err)
	}
	return n
}

// A schema event against a multi-part source registers, with size set to the
// sentinel's own 0 as the lambda passes it through. This is the path the change
// request puts into production, and it establishes that the two cases below
// differ from it in one map entry and nothing else.
func TestRegisterFileKeysSchemaEventWithSizeZero(t *testing.T) {
	t.Setenv("JETS_SENTINEL_FILE_NAME", testSentinel)
	ctx := registerFileKeysFixture(t)
	action := &RegisterFileKeyAction{
		Action:          "register_file_key",
		Data:            []map[string]any{schemaEventData("CGT0001", int64(0), true)},
		NoAutomatedLoad: true,
		IsSchemaEvent:   true,
	}
	_, status, err := ctx.RegisterFileKeys(action, "")
	if err != nil || status != http.StatusOK {
		t.Fatalf("RegisterFileKeys: status %d, err %v", status, err)
	}
	folder := testFolder + "CGT0001"
	// The sentinel branch strips the last path segment, so the registered key is
	// the folder and not the key handed in.
	if n, _ := countFileKeyStaging(t, ctx, folder); n != 1 {
		t.Errorf("file_key_staging rows for %q = %d, want 1", folder, n)
	}
	if n, _ := countFileKeyStaging(t, ctx, folder+"/"+testSentinel); n != 0 {
		t.Errorf("the sentinel key itself was registered; the strip at :228 did not run")
	}
	if n := countInputRegistry(t, ctx, folder); n != 1 {
		t.Errorf("input_registry rows for %q = %d, want 1", folder, n)
	}
}

// Criterion 13. Without the comma-ok assertion this panics with
// "interface conversion: interface {} is nil, not int64" — an absent file_size
// is what every schema event RegisterSchemaEvent builds carries, because
// SchemaProviderSpec.FileSize serialises as file_size and the copying switch
// has a case for size only.
func TestRegisterFileKeysSchemaEventWithSizeAbsent(t *testing.T) {
	t.Setenv("JETS_SENTINEL_FILE_NAME", testSentinel)
	ctx := registerFileKeysFixture(t)
	data := schemaEventData("CGT0002", nil, true)
	if _, present := data["size"]; present {
		t.Fatal("fixture error: this case is about size being absent")
	}
	action := &RegisterFileKeyAction{
		Action:          "register_file_key",
		Data:            []map[string]any{data},
		NoAutomatedLoad: true,
		IsSchemaEvent:   true,
	}
	_, status, err := ctx.RegisterFileKeys(action, "")
	if err != nil || status != http.StatusOK {
		t.Fatalf("RegisterFileKeys: status %d, err %v", status, err)
	}
	folder := testFolder + "CGT0002"
	n, size := countFileKeyStaging(t, ctx, folder)
	if n != 1 {
		t.Errorf("file_key_staging rows for %q = %d, want 1", folder, n)
	}
	// The guard yields 0, the size > 1 gate is not taken, and the folder listing
	// leaves the value alone when it fails. With a bucket configured the listing
	// may succeed and overwrite it, so this assertion is only made without one.
	if awsi.JetStoreBucket() == "" && size != 0 {
		t.Errorf("file_size = %d, want 0 with no JETS_BUCKET set", size)
	}
	if n := countInputRegistry(t, ctx, folder); n != 1 {
		t.Errorf("input_registry rows for %q = %d, want 1", folder, n)
	}
}

// Criterion 14. Without the comma-ok assertion this panics with
// "interface conversion: interface {} is nil, not string" at :206, before the
// source_config query. Not reachable from this change request — the lambda
// always sets file_key — and it is the same defect class in the same function.
func TestRegisterFileKeysFileKeyAbsentIsSkipped(t *testing.T) {
	t.Setenv("JETS_SENTINEL_FILE_NAME", testSentinel)
	ctx := registerFileKeysFixture(t)
	data := schemaEventData("CGT0003", int64(0), false)
	if _, present := data["file_key"]; present {
		t.Fatal("fixture error: this case is about file_key being absent")
	}
	action := &RegisterFileKeyAction{
		Action:          "register_file_key",
		Data:            []map[string]any{data},
		NoAutomatedLoad: true,
		IsSchemaEvent:   true,
	}
	_, status, err := ctx.RegisterFileKeys(action, "")
	if err != nil || status != http.StatusOK {
		t.Fatalf("RegisterFileKeys: status %d, err %v", status, err)
	}
	var n int
	if err := ctx.Dbpool.QueryRow(context.Background(),
		`SELECT count(*) FROM jetsapi.file_key_staging`).Scan(&n); err != nil {
		t.Fatalf("counting file_key_staging: %v", err)
	}
	if n != 0 {
		t.Errorf("file_key_staging has %d rows, want 0: the row was registered rather than skipped", n)
	}
	if err := ctx.Dbpool.QueryRow(context.Background(),
		`SELECT count(*) FROM jetsapi.input_registry`).Scan(&n); err != nil {
		t.Fatalf("counting input_registry: %v", err)
	}
	if n != 0 {
		t.Errorf("input_registry has %d rows, want 0", n)
	}
	// An empty file_key is the same defect by the other door: FileConfig.FileKey
	// is omitempty, so a spec that sets it to "" serialises without it, and one
	// that reaches the map as "" asserts cleanly and registers a row naming the
	// bucket root.
	data = schemaEventData("CGT0004", int64(0), true)
	data["file_key"] = ""
	action.Data = []map[string]any{data}
	_, status, err = ctx.RegisterFileKeys(action, "")
	if err != nil || status != http.StatusOK {
		t.Fatalf("RegisterFileKeys with an empty file_key: status %d, err %v", status, err)
	}
	if err := ctx.Dbpool.QueryRow(context.Background(),
		`SELECT count(*) FROM jetsapi.file_key_staging`).Scan(&n); err != nil {
		t.Fatalf("counting file_key_staging: %v", err)
	}
	if n != 0 {
		t.Errorf("file_key_staging has %d rows, want 0: an empty file_key was registered", n)
	}
}
