// The run manifest's observed half is SQL, and SQL is the part of it Go's
// compiler has nothing to say about: the NULL handling that distinguishes "no
// rows were counted here" from "zero rows were written" is a property of
// PostgreSQL's aggregates rather than of this package, and a unit test over a
// hand-built []ObservedChannel would assert the fixture rather than the query.
//
// Needs JETS_TEST_DSN (any throwaway database; these tests install the tables
// they name and drop them after); skipped otherwise. Locally:
//
//	docker run -d --rm -e POSTGRES_PASSWORD=pw -p 5466:5432 postgres:16
//	JETS_TEST_DSN=postgres://postgres:pw@localhost:5466/postgres go test ./jets/compute_pipes/ -run RunManifest
package compute_pipes

import (
	"context"
	"encoding/json"
	"os"
	"strings"
	"testing"

	"github.com/artisoft-io/jetstore/jets/schema"
	"github.com/jackc/pgx/v5/pgxpool"
)

func manifestTestPool(t *testing.T, tableNames ...string) *pgxpool.Pool {
	t.Helper()
	dsn := os.Getenv("JETS_TEST_DSN")
	if dsn == "" {
		t.Skip("JETS_TEST_DSN not set; needs a throwaway Postgres")
	}
	pool, err := pgxpool.New(context.Background(), dsn)
	if err != nil {
		t.Fatalf("connecting to %s: %v", dsn, err)
	}
	t.Cleanup(pool.Close)

	b, err := os.ReadFile("../jets_schema.json")
	if err != nil {
		t.Fatalf("reading jets_schema.json: %v", err)
	}
	var defs []schema.TableDefinition
	if err := json.Unmarshal(b, &defs); err != nil {
		t.Fatalf("decoding jets_schema.json: %v", err)
	}
	wanted := make(map[string]bool, len(tableNames))
	for _, n := range tableNames {
		wanted[n] = true
	}
	found := 0
	for i := range defs {
		if !wanted[defs[i].TableName] {
			continue
		}
		found++
		def := defs[i]
		if err := def.UpdateTableSchema(pool, true); err != nil {
			t.Fatalf("installing %s: %v", def.TableName, err)
		}
		t.Cleanup(func() { _ = def.DropTable(pool) })
	}
	if found != len(tableNames) {
		t.Fatalf("found %d of the %d requested tables in jets_schema.json", found, len(tableNames))
	}
	return pool
}

// channelRow is one row of pipeline_execution_channel_details, written the way
// InsertChannelExecutionDetails writes it -- including the NULL row count,
// which is the whole reason this file needs a database.
type channelRow struct {
	parentKey     int
	inputChannel  string
	outputChannel string
	outputType    string
	outputEntity  string
	location      string
	sinks         int
	rowCount      any // nil for RowCountUnknown
	parts         int64
	errMsg        string
}

func insertChannelRows(t *testing.T, pool *pgxpool.Pool, sessionId string, rows []channelRow) {
	t.Helper()
	stmt := `INSERT INTO jetsapi.pipeline_execution_channel_details (
		pipeline_execution_details_key, session_id, input_channel, output_channel,
		output_channel_spec, output_type, output_entity, output_location,
		output_sinks_count, output_records_count, parts_count, error_message)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12)`
	for _, r := range rows {
		_, err := pool.Exec(context.Background(), stmt, r.parentKey, sessionId, r.inputChannel,
			r.outputChannel, r.outputChannel+"_spec", r.outputType, r.outputEntity, r.location,
			r.sinks, r.rowCount, r.parts, r.errMsg)
		if err != nil {
			t.Fatalf("inserting a channel detail row: %v", err)
		}
	}
}

// TestRunManifestObservedHalfIsOneQuery: two workers of step 1 and one of step
// 2, all under one session_id, fold into one row per edge with no join and no
// accumulation across iterations. That is what moving the producer to the end
// of the run bought.
func TestRunManifestObservedHalfIsOneQuery(t *testing.T) {
	pool := manifestTestPool(t, "pipeline_execution_channel_details")
	insertChannelRows(t, pool, "sess-A", []channelRow{
		// step 1, worker 0 and worker 1, same edge
		{parentKey: 10, inputChannel: "in", outputChannel: "claims_out", outputType: SinkDbTable,
			outputEntity: "jetsapi.claims", location: "sql://jetsapi.claims", sinks: 1, rowCount: int64(600)},
		{parentKey: 11, inputChannel: "in", outputChannel: "claims_out", outputType: SinkDbTable,
			outputEntity: "jetsapi.claims", location: "sql://jetsapi.claims", sinks: 1, rowCount: int64(700)},
		// step 2, one worker, a different edge
		{parentKey: 20, inputChannel: "merged", outputChannel: "export", outputType: SinkOutputFile,
			location: "s3://acme/out/export.csv", sinks: 1, rowCount: nil, parts: 1},
		// another session's row, which must not be counted
		{parentKey: 99, inputChannel: "in", outputChannel: "claims_out", outputType: SinkDbTable,
			outputEntity: "jetsapi.claims", location: "sql://jetsapi.claims", sinks: 1, rowCount: int64(999999)},
	})
	// the last row belongs to a different run
	if _, err := pool.Exec(context.Background(),
		`UPDATE jetsapi.pipeline_execution_channel_details SET session_id = 'sess-B'
		 WHERE pipeline_execution_details_key = 99`); err != nil {
		t.Fatalf("%v", err)
	}

	observed, err := ReadObservedChannels(context.Background(), pool, "sess-A")
	if err != nil {
		t.Fatalf("ReadObservedChannels: %v", err)
	}
	if len(observed) != 2 {
		t.Fatalf("got %d edges, want 2: %+v", len(observed), observed)
	}
	byChannel := map[string]ObservedChannel{}
	for _, o := range observed {
		byChannel[o.OutputChannel] = o
	}
	claims := byChannel["claims_out"]
	if claims.RecordsCount != 1300 || claims.RecordsUnknown {
		t.Errorf("claims_out = %d rows (unknown=%v), want 1300 summed over two workers",
			claims.RecordsCount, claims.RecordsUnknown)
	}
	if claims.SinksCount != 2 {
		t.Errorf("claims_out folded %d sinks, want 2", claims.SinksCount)
	}
	export := byChannel["export"]
	if !export.RecordsUnknown {
		t.Errorf("a merge sink's NULL row count must survive the GROUP BY as not-measured, got %d",
			export.RecordsCount)
	}
	if export.PartsCount != 1 {
		t.Errorf("export parts = %d, want 1", export.PartsCount)
	}
}

// TestRunManifestMixedNullRowCounts is the trap the query is written around:
// PostgreSQL's sum() ignores NULLs, so an edge mixing a measured worker with an
// unmeasurable one would report the measured part as if it were the total. One
// sink that cannot count makes the edge total not a measurement.
func TestRunManifestMixedNullRowCounts(t *testing.T) {
	pool := manifestTestPool(t, "pipeline_execution_channel_details")
	insertChannelRows(t, pool, "sess-mix", []channelRow{
		{parentKey: 1, inputChannel: "in", outputChannel: "mixed", outputType: SinkDbTable,
			location: "sql://jetsapi.mixed", sinks: 1, rowCount: int64(12)},
		{parentKey: 2, inputChannel: "in", outputChannel: "mixed", outputType: SinkDbTable,
			location: "sql://jetsapi.mixed", sinks: 1, rowCount: nil},
	})
	observed, err := ReadObservedChannels(context.Background(), pool, "sess-mix")
	if err != nil {
		t.Fatalf("ReadObservedChannels: %v", err)
	}
	if len(observed) != 1 {
		t.Fatalf("got %d edges, want 1", len(observed))
	}
	if !observed[0].RecordsUnknown {
		t.Fatalf("the edge summed a number with a non-number and reported %d as a measurement",
			observed[0].RecordsCount)
	}
	cfg := &ComputePipesConfig{OutputTables: []*TableSpec{{Key: "mixed", Name: "jetsapi.mixed"}}}
	m, err := BuildRunManifest(cfg, observed, RunManifestMeta{SessionId: "sess-mix", Status: "completed"})
	if err != nil {
		t.Fatalf("BuildRunManifest: %v", err)
	}
	if m.Entries[0].RecordsCount != nil {
		t.Fatalf("the manifest reported %d rows for an edge that was not fully measured",
			*m.Entries[0].RecordsCount)
	}
}

// TestRunManifestNamesTheChannelNotTheEntity is AC.3, and it is a question
// rather than a defect: AggregateChannelResults blanks OutputEntity when an
// edge folds several sinks, deliberately, so that an aggregate is never
// mistaken for an instance. The field whose name most sounds like "the
// deliverable" is therefore empty on exactly the configurations that fan out --
// and for a jets_partition sink it holds a shard label, which is not a
// deliverable at all.
//
// The manifest joins on output_channel, which is the declared key. This test is
// what stops a later reader reaching for the other field.
func TestRunManifestNamesTheChannelNotTheEntity(t *testing.T) {
	pool := manifestTestPool(t, "pipeline_execution_channel_details")
	insertChannelRows(t, pool, "sess-split", []channelRow{
		// A splitter-fed edge: one output channel, many jets_partition sinks,
		// so AggregateChannelResults blanked the entity before the insert.
		{parentKey: 1, inputChannel: "in", outputChannel: "claims_out", outputType: SinkDbTable,
			outputEntity: "", location: "sql://jetsapi.claims", sinks: 12, rowCount: int64(4800)},
		// The splitter's own partition rows, whose entity is a shard label.
		{parentKey: 1, inputChannel: "in", outputChannel: "by_year", outputType: SinkJetsPartition,
			outputEntity: "jets_partition=2026-01", location: "s3://acme/parts", sinks: 12,
			rowCount: int64(4800), parts: 12},
	})
	observed, err := ReadObservedChannels(context.Background(), pool, "sess-split")
	if err != nil {
		t.Fatalf("ReadObservedChannels: %v", err)
	}
	cfg := &ComputePipesConfig{OutputTables: []*TableSpec{{Key: "claims_out", Name: "jetsapi.claims"}}}
	m, err := BuildRunManifest(cfg, observed, RunManifestMeta{SessionId: "sess-split", Status: "completed"})
	if err != nil {
		t.Fatalf("BuildRunManifest: %v", err)
	}
	if len(m.Entries) != 1 {
		t.Fatalf("got %d entries, want the one declared table", len(m.Entries))
	}
	e := m.Entries[0]
	if e.Channel != "claims_out" {
		t.Errorf("the entry is named %q; the deliverable's name is the declared channel", e.Channel)
	}
	if !e.Written || e.Location != "sql://jetsapi.claims" {
		t.Errorf("the identity of a db_table sink survives in output_location: %+v", e)
	}
	if e.RecordsCount == nil || *e.RecordsCount != 4800 {
		t.Errorf("records_count = %v, want 4800", e.RecordsCount)
	}
	buf, _ := json.Marshal(m)
	for _, forbidden := range []string{"jets_partition", "by_year", "2026-01"} {
		if strings.Contains(string(buf), forbidden) {
			t.Errorf("the manifest carries %q, which is a shard rather than a deliverable: %s",
				forbidden, buf)
		}
	}
}

// TestWriteRunManifestWritesBothStores: the object and the column, and the
// declared half taken from the whole document rather than from the one step
// cpipes_config_json holds.
func TestWriteRunManifestWritesBothStores(t *testing.T) {
	pool := manifestTestPool(t, "pipeline_execution_channel_details", "cpipes_execution_status",
		"pipeline_execution_status")
	ctx := context.Background()

	var peKey int
	err := pool.QueryRow(ctx, `INSERT INTO jetsapi.pipeline_execution_status
		(pipeline_config_key, merged_input_registry_keys, client, process_name, main_object_type,
		 session_id, source_period_key, status, user_email)
		VALUES (1, '{}', 'acme', 'myprocess', 'claim', 'sess-W', 1, 'submitted', 'x@y.z')
		RETURNING key`).Scan(&peKey)
	if err != nil {
		t.Fatalf("inserting pipeline_execution_status: %v", err)
	}

	startup := CpipesStartup{
		ProcessName: "myprocess",
		CpConfig: ComputePipesConfig{
			OutputTables: []*TableSpec{
				{Key: "claims_out", Name: "jetsapi.claims"},
				{Key: "members_out", Name: "jetsapi.members"},
			},
			OutputFiles: []OutputFileSpec{{Key: "export", FileName2: "export.csv"}},
		},
	}
	startupJson, err := json.Marshal(startup)
	if err != nil {
		t.Fatalf("%v", err)
	}
	if _, err := pool.Exec(ctx, `INSERT INTO jetsapi.cpipes_execution_status
		(pipeline_execution_status_key, session_id, cpipes_startup_json) VALUES ($1, $2, $3)`,
		peKey, "sess-W", string(startupJson)); err != nil {
		t.Fatalf("inserting cpipes_execution_status: %v", err)
	}
	insertChannelRows(t, pool, "sess-W", []channelRow{
		{parentKey: 1, inputChannel: "in", outputChannel: "claims_out", outputType: SinkDbTable,
			location: "sql://jetsapi.claims", sinks: 1, rowCount: int64(600)},
		{parentKey: 2, inputChannel: "in", outputChannel: "members_out", outputType: SinkDbTable,
			location: "sql://jetsapi.members", sinks: 1, rowCount: int64(40)},
		{parentKey: 3, inputChannel: "merged", outputChannel: "export", outputType: SinkOutputFile,
			location: "s3://acme-out/out/export.csv", sinks: 1, rowCount: nil, parts: 2},
	})

	var uploadedKey string
	var uploaded []byte
	restore := RunManifestUploader
	RunManifestUploader = func(bucket, objKey string, buf []byte) error {
		uploadedKey, uploaded = objKey, buf
		return nil
	}
	t.Cleanup(func() { RunManifestUploader = restore })

	if err := WriteRunManifest(ctx, pool, "sess-W", "completed"); err != nil {
		t.Fatalf("WriteRunManifest: %v", err)
	}

	if !strings.HasSuffix(uploadedKey, "/process_name=myprocess/session_id=sess-W/run_manifest.json") {
		t.Errorf("the object key is %q", uploadedKey)
	}
	var fromS3 CpipesRunManifest
	if err := json.Unmarshal(uploaded, &fromS3); err != nil {
		t.Fatalf("the uploaded document does not parse: %v", err)
	}
	var stored string
	if err := pool.QueryRow(ctx,
		`SELECT run_manifest_json FROM jetsapi.cpipes_execution_status WHERE session_id = 'sess-W'`).
		Scan(&stored); err != nil {
		t.Fatalf("reading run_manifest_json: %v", err)
	}
	if stored != string(uploaded) {
		t.Errorf("the two stores hold different documents")
	}
	if len(fromS3.Entries) != 3 {
		t.Fatalf("the manifest has %d entries, want the 3 the document declared", len(fromS3.Entries))
	}
	if fromS3.PipelineExecutionKey != peKey || fromS3.ProcessName != "myprocess" ||
		fromS3.Status != "completed" {
		t.Errorf("the manifest does not identify its run: %+v", fromS3)
	}
	for _, e := range fromS3.Entries {
		if !e.Written {
			t.Errorf("entry %q reports nothing written", e.Channel)
		}
	}
}

// TestWriteRunManifestRefusesWithoutAConfigRow: a run that failed at sharding
// validation leaves no cpipes_execution_status row, so there is no declared
// half. Saying so is better than writing a manifest with an empty entry list,
// which would read as "this run declared nothing".
func TestWriteRunManifestRefusesWithoutAConfigRow(t *testing.T) {
	pool := manifestTestPool(t, "pipeline_execution_channel_details", "cpipes_execution_status",
		"pipeline_execution_status")
	called := false
	restore := RunManifestUploader
	RunManifestUploader = func(bucket, objKey string, buf []byte) error {
		called = true
		return nil
	}
	t.Cleanup(func() { RunManifestUploader = restore })
	err := WriteRunManifest(context.Background(), pool, "sess-missing", "completed")
	if err == nil {
		t.Fatal("a session with no configuration row produced a manifest")
	}
	if called {
		t.Error("nothing should have been uploaded")
	}
}
