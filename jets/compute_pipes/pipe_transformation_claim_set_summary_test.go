package compute_pipes

// The claim_set_summary operator. The tests that need a database need
// JETS_TEST_DSN, a throwaway Postgres; run them with -count=1.
//
//	JETS_TEST_DSN=postgres://postgres:pw@localhost:5463/postgres \
//	  go test -count=1 -run ClaimSetSummary ./jets/compute_pipes/

import (
	"context"
	"encoding/json"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"math"
	"net/url"
	"os"
	"reflect"
	"slices"
	"sort"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/artisoft-io/jetstore/jets/agentic/audit"
	"github.com/artisoft-io/jetstore/jets/agentic/feedprofile"
	"github.com/artisoft-io/jetstore/jets/agentic/observe"
	"github.com/artisoft-io/jetstore/jets/agentic/observe/feedfixture"
	"github.com/artisoft-io/jetstore/jets/claimset"
	"github.com/artisoft-io/jetstore/jets/schema"
	"github.com/artisoft-io/jetstore/jets/sessionpurge"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// claimSetSummaryTestDB is a fresh database of its own.
func claimSetSummaryTestDB(t *testing.T, database string) *pgxpool.Pool {
	t.Helper()
	dsn := os.Getenv("JETS_TEST_DSN")
	if dsn == "" {
		t.Skip("JETS_TEST_DSN not set; needs a throwaway Postgres")
	}
	ctx := context.Background()
	admin, err := pgxpool.New(ctx, dsn)
	if err != nil {
		t.Fatal(err)
	}
	name := pgx.Identifier{database}.Sanitize()
	if _, err = admin.Exec(ctx, "DROP DATABASE IF EXISTS "+name+" WITH (FORCE)"); err == nil {
		_, err = admin.Exec(ctx, "CREATE DATABASE "+name)
	}
	admin.Close()
	if err != nil {
		t.Fatal(err)
	}
	u, err := url.Parse(dsn)
	if err != nil {
		t.Fatal(err)
	}
	u.Path = "/" + database
	pool, err := pgxpool.New(ctx, u.String())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(pool.Close)
	return pool
}

// A value's text is the text PostgreSQL gives the column it would be loaded
// into, for every kind of value a record carries.
func TestClaimSetSummaryTextIsPostgres(t *testing.T) {
	pool := claimSetSummaryTestDB(t, "cp_claimset_text_test")
	ctx := context.Background()
	cases := []struct {
		value any
		cast  string
	}{
		{"abc", "text"}, {"", "text"}, {"  x ", "text"}, {"é-ü", "text"},
		{int(-42), "bigint"}, {int32(7), "integer"}, {int64(9007199254740993), "bigint"}, {uint64(18), "bigint"},
		{1.5, "float8"}, {0.1 + 0.2, "float8"}, {100.0, "float8"}, {1e14, "float8"}, {123456789012345.0, "float8"},
		{1e15, "float8"}, {1e300, "float8"}, {0.0001, "float8"}, {0.00001, "float8"}, {-2.5e-7, "float8"},
		{0.0, "float8"}, {math.Copysign(0, -1), "float8"}, {math.Inf(1), "float8"}, {math.NaN(), "float8"},
		{true, "boolean"}, {false, "boolean"},
		{time.Date(2026, 3, 9, 0, 0, 0, 0, time.UTC), "date"},
		{time.Date(2026, 3, 9, 14, 5, 6, 0, time.UTC), "timestamp"},
		{time.Date(2026, 3, 9, 14, 5, 6, 500000000, time.UTC), "timestamp"},
		{[]any{"a", "b c", "", nil, `q"\`, "NULL", "{x}"}, "text[]"},
		{[]any{int64(1), int64(2)}, "bigint[]"},
	}
	for _, c := range cases {
		got, err := claimSetSummaryText(c.value)
		if err != nil {
			t.Errorf("%#v: %v", c.value, err)
			continue
		}
		var want string
		arg := c.value
		if strings.HasSuffix(c.cast, "[]") {
			// Bind the array as PostgreSQL reads it, from its own text.
			arg = *got
		}
		if err := pool.QueryRow(ctx, fmt.Sprintf("SELECT ($1::%s)::text", c.cast), arg).Scan(&want); err != nil {
			t.Fatalf("%#v as %s: %v", c.value, c.cast, err)
		}
		if *got != want {
			t.Errorf("%#v: %q, the database's %q", c.value, *got, want)
		}
	}
	// An array is bound from the text above, so read one back from a value
	// the database built, to check the quoting against array_out itself.
	var want string
	if err := pool.QueryRow(ctx, `SELECT ARRAY['a', 'b c', '', NULL, E'q"\\', 'NULL', '{x}']::text`).Scan(&want); err != nil {
		t.Fatal(err)
	}
	got, _ := claimSetSummaryText([]any{"a", "b c", "", nil, `q"\`, "NULL", "{x}"})
	if *got != want {
		t.Errorf("an array: %q, the database's %q", *got, want)
	}
	if v, err := claimSetSummaryText(nil); v != nil || err != nil {
		t.Error("a null has a text")
	}
	if _, err := claimSetSummaryText([]byte("x")); err == nil {
		t.Error("bytes were given a text")
	}
	for _, v := range []any{"2026-03-09", " 2026-03-09 ", "2026-03-09 10:00:00", "2026-03-09T10:00:00Z",
		time.Date(2026, 3, 9, 23, 0, 0, 0, time.UTC)} {
		d, err := claimSetSummaryDate(v)
		if err != nil || d == nil || !d.Equal(time.Date(2026, 3, 9, 0, 0, 0, 0, time.UTC)) {
			t.Errorf("the date of %#v is %v (%v)", v, d, err)
		}
	}
	for _, v := range []any{"", "03/09/2026", "2026-3-9", 20260309} {
		if _, err := claimSetSummaryDate(v); err == nil {
			t.Errorf("%#v was read as a date", v)
		}
	}
}

// claimSetSummaryTestChannels are a source and a pass-through output channel
// over columns.
func claimSetSummaryTestChannels(columns []string) (*InputChannel, *OutputChannel, *ChannelRegistry) {
	columnsMap := make(map[string]int, len(columns))
	for i, c := range columns {
		columnsMap[c] = i
	}
	spec := &ChannelSpec{Name: "claims", Columns: columns}
	spec.SetColumnsMap(&columnsMap)
	registry := &ChannelRegistry{
		ComputeChannels: map[string]*Channel{
			"claims.out": {Name: "claims.out", Channel: make(chan []any), Columns: &columnsMap, Config: spec},
		},
		ClosedChannels: make(map[string]bool),
	}
	outputCh, _ := registry.GetOutputChannel("claims.out")
	return &InputChannel{Name: "claims.in", Columns: &columnsMap, Config: spec}, outputCh, registry
}

// runClaimSetSummary runs the operator over records as a fan_out would, and
// returns what reached the output channel.
func runClaimSetSummary(t *testing.T, bc *BuilderContext, columns []string, records [][]any) [][]any {
	t.Helper()
	source, _, registry := claimSetSummaryTestChannels(columns)
	bc.channelRegistry = registry
	pipe, err := bc.BuildPipeTransformationEvaluator(source, nil, nil, &TransformationSpec{
		Type: ClaimSetSummaryOperatorType, OutputChannel: OutputChannelConfig{Name: "claims.out"}})
	if err != nil {
		t.Fatal(err)
	}
	var out [][]any
	var wg sync.WaitGroup
	wg.Go(func() {
		for r := range registry.ComputeChannels["claims.out"].Channel {
			out = append(out, r)
		}
	})
	for i := range records {
		if err := pipe.Apply(&records[i]); err != nil {
			t.Fatal(err)
		}
	}
	if err := pipe.Done(); err != nil {
		t.Fatal(err)
	}
	pipe.Finally()
	registry.CloseChannel("claims.out")
	wg.Wait()
	return out
}

// Without a database the operator writes nothing and forwards every record,
// unchanged and in order; so it does when a value has no text.
func TestClaimSetSummaryForwardsEveryRecord(t *testing.T) {
	columns := []string{"k", "m", "d"}
	records := [][]any{{"a", "m1", time.Now()}, {"b", nil, nil}, {[]byte("x"), "m2", "not a date"}}
	bc := &BuilderContext{done: make(chan struct{}), errCh: make(chan error, 1), env: map[string]any{}}
	out := runClaimSetSummary(t, bc, columns, slices.Clone(records))
	if !reflect.DeepEqual(out, records) {
		t.Errorf("forwarded %v, want %v", out, records)
	}
}

// A step carrying the operator runs on one node: the starters refuse more,
// and the startup validation refuses it under a splitter.
func TestClaimSetSummaryRunsOnOneNode(t *testing.T) {
	steps := []PipeSpec{{Type: "fan_out", Apply: []TransformationSpec{{Type: "map_record"},
		{Type: ClaimSetSummaryOperatorType}}}}
	if err := validateClaimSetSummaryNodes(steps, 1, "s"); err != nil {
		t.Errorf("one node refused: %v", err)
	}
	if err := validateClaimSetSummaryNodes(steps, 3, "s"); err == nil || !strings.Contains(err.Error(), "one node") {
		t.Errorf("three nodes: %v", err)
	}
	if err := validateClaimSetSummaryNodes([]PipeSpec{{Type: "fan_out", Apply: []TransformationSpec{
		{Type: "map_record"}}}}, 3, "s"); err != nil {
		t.Errorf("a step without the operator refused: %v", err)
	}

	for _, pipeType := range []string{"fan_out", "splitter"} {
		args := &CpipesStartup{}
		pipeConfig := []PipeSpec{{Type: pipeType, InputChannel: InputChannelConfig{Name: "claims.in"},
			Apply: []TransformationSpec{{Type: ClaimSetSummaryOperatorType,
				OutputChannel: OutputChannelConfig{Name: "claims.out", SpecName: "claims"}}}}}
		err := args.ValidatePipeSpecConfig(&ComputePipesConfig{}, pipeConfig)
		if (pipeType == "splitter") != (err != nil && strings.Contains(err.Error(), "splitter")) {
			t.Errorf("under a %s: %v", pipeType, err)
		}
	}

	// A save warns of the splitter, and of nothing else.
	doc := `{"cluster_config": {}, "conditional_pipes_config": [{"pipes_config": [
		{"type": "fan_out", "input_channel": {"name": "a"}, "apply": [{"type": "claim_set_summary",
		  "output_channel": {"name": "b"}}]},
		{"type": "splitter", "input_channel": {"name": "b"}, "apply": [{"type": "claim_set_summary",
		  "output_channel": {"name": "c"}}]}]}]}`
	findings := ValidateConfigDocument(doc)
	if len(findings) != 1 || findings[0].Code != "cpipes_config_claim_set_summary_splitter" ||
		findings[0].Path != "/conditional_pipes_config/0/pipes_config/1" || findings[0].Severity != "warning" {
		t.Errorf("the save's findings: %+v", findings)
	}

	// Both starters call the node check, with the count they start.
	for file, count := range map[string]string{"actions_start_sharding_cp.go": "shardResult.nbrShardingNodes",
		"actions_start_reducing_cp.go": "len(partitions)"} {
		f, err := parser.ParseFile(token.NewFileSet(), file, nil, 0)
		if err != nil {
			t.Fatal(err)
		}
		found := false
		ast.Inspect(f, func(n ast.Node) bool {
			if call, ok := n.(*ast.CallExpr); ok {
				if id, ok := call.Fun.(*ast.Ident); ok && id.Name == "validateClaimSetSummaryNodes" && len(call.Args) == 3 {
					var b strings.Builder
					_ = astExprString(&b, call.Args[1])
					found = found || b.String() == count
				}
			}
			return true
		})
		if !found {
			t.Errorf("%s does not call validateClaimSetSummaryNodes with %s", file, count)
		}
	}
}

func astExprString(b *strings.Builder, e ast.Expr) error {
	switch x := e.(type) {
	case *ast.Ident:
		b.WriteString(x.Name)
	case *ast.SelectorExpr:
		_ = astExprString(b, x.X)
		b.WriteString("." + x.Sel.Name)
	case *ast.CallExpr:
		_ = astExprString(b, x.Fun)
		b.WriteString("(")
		for i, a := range x.Args {
			if i > 0 {
				b.WriteString(", ")
			}
			_ = astExprString(b, a)
		}
		b.WriteString(")")
	default:
		b.WriteString("?")
	}
	return nil
}

// claimSetSummaryTestFixture installs the feed fixture with its claims under
// completed pipeline runs, every jetsapi table of jets_schema.json, and the
// agentic tables.
func claimSetSummaryTestFixture(t *testing.T) *pgxpool.Pool {
	t.Helper()
	pool := claimSetSummaryTestDB(t, "cp_claimset_summary_test")
	ctx := context.Background()
	buf, err := os.ReadFile(feedfixture.SchemaPath())
	if err != nil {
		t.Fatal(err)
	}
	var defs []schema.TableDefinition
	if err := json.Unmarshal(buf, &defs); err != nil {
		t.Fatal(err)
	}
	for i := range defs {
		if defs[i].SchemaName == "jetsapi" {
			if err := defs[i].UpdateTableSchema(pool, false); err != nil {
				t.Fatalf("jetsapi.%s: %v", defs[i].TableName, err)
			}
		}
	}
	if _, err := feedfixture.InstallUnderPipelineRuns(ctx, pool); err != nil {
		t.Fatalf("installing the feed fixture: %v", err)
	}
	if err := audit.InstallSchema(ctx, pool); err != nil {
		t.Fatalf("installing the agentic DDL: %v", err)
	}
	return pool
}

// claimSetSummaryTestFeeds are every fixture feed as source_config holds it.
func claimSetSummaryTestFeeds(t *testing.T, pool *pgxpool.Pool) []feedprofile.Feed {
	t.Helper()
	var all []feedprofile.Feed
	seen := map[string]bool{}
	for _, f := range feedfixture.Feeds() {
		if seen[f.Client] {
			continue
		}
		seen[f.Client] = true
		p, u, err := feedprofile.Load(context.Background(), pool, f.Client)
		if err != nil {
			t.Fatal(err)
		}
		all = append(append(all, p...), u...)
	}
	return all
}

// claimSetSummaryTestRows are the stored summaries, by feed and delivery, as
// json objects without summarised_at; withoutSession drops the table's own
// session_id from the present counts, which records do not carry.
func claimSetSummaryTestRows(t *testing.T, pool *pgxpool.Pool, withoutSession bool) map[string]map[string]any {
	t.Helper()
	rows, err := pool.Query(context.Background(), `SELECT table_name || ' ' || delivery_session_id,
		(to_jsonb(s) - 'summarised_at')::text FROM jetsapi.claim_set_summary s`)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	out := map[string]map[string]any{}
	for rows.Next() {
		var k, v string
		if err := rows.Scan(&k, &v); err != nil {
			t.Fatal(err)
		}
		var m map[string]any
		if err := json.Unmarshal([]byte(v), &m); err != nil {
			t.Fatal(err)
		}
		if p, ok := m["present_counts"].(map[string]any); ok && withoutSession {
			delete(p, "session_id")
		}
		out[k] = m
	}
	if err := rows.Err(); err != nil {
		t.Fatal(err)
	}
	return out
}

// claimSetSummaryHeld is the sessions a dry run of the table-data purge holds.
func claimSetSummaryHeld(t *testing.T, pool *pgxpool.Pool) []string {
	t.Helper()
	now := time.Now()
	report, err := sessionpurge.PurgeTableData(context.Background(), pool, now.AddDate(0, 0, -7),
		now.AddDate(0, 0, -30), sessionpurge.Options{DryRun: true})
	if err != nil {
		t.Fatal(err)
	}
	sort.Strings(report.HeldSessions)
	return report.HeldSessions
}

// The operator over each delivery's records, as a pipeline run that does not
// load them would see them, writes the row the feed detection computes from
// the table, field by field, for every delivery of every profiled feed of the
// fixture. Then the feed detection reads those rows and computes none, with the
// rows present and with them gone, and reaches the verdicts it reaches from
// the rows; and the table-data purge, which held every run's claim rows until
// a summary named it, holds none.
func TestClaimSetSummaryOperatorWritesTheFeedDetectionsRow(t *testing.T) {
	pool := claimSetSummaryTestFixture(t)
	ctx := context.Background()
	feeds := claimSetSummaryTestFeeds(t, pool)
	o := observe.DefaultClaimSetOptions()
	o.Now = feedfixture.Clock

	// The verdicts and summaries from the rows, with no summary stored.
	fromRows, sums, err := observe.SummariseClaimSets(ctx, pool, feeds, o)
	if err != nil {
		t.Fatal(err)
	}
	if len(sums) == 0 {
		t.Fatal("the feed detection computed no summary")
	}

	// Every run's claim rows are held: no summary names them yet.
	type run struct {
		key            int
		session, table string
	}
	var runs []run
	rows, err := pool.Query(ctx, `SELECT pe.key, pe.session_id, ir.table_name FROM jetsapi.pipeline_execution_status pe
		JOIN jetsapi.input_registry ir ON ir.session_id = pe.input_session_id AND ir.source_type = 'file'
		ORDER BY pe.key`)
	if err != nil {
		t.Fatal(err)
	}
	for rows.Next() {
		var r run
		if err := rows.Scan(&r.key, &r.session, &r.table); err != nil {
			t.Fatal(err)
		}
		runs = append(runs, r)
	}
	if rows.Err() != nil || len(runs) == 0 {
		t.Fatalf("no pipeline run in the fixture (%v)", rows.Err())
	}
	for _, r := range runs {
		if _, err := pool.Exec(ctx, `INSERT INTO jetsapi.session_registry (session_id, source_type, client,
			source_period_key, month_period, week_period, day_period, last_update)
			VALUES ($1, 'file', 'c', 0, 0, 0, 0, now() - interval '10 days')`, r.session); err != nil {
			t.Fatal(err)
		}
	}
	heldBefore := claimSetSummaryHeld(t, pool)
	if len(heldBefore) == 0 {
		t.Fatal("the purge holds no session before any summary is written; the test asserts nothing")
	}

	// The rows the feed detection writes.
	if n, err := observe.WriteClaimSetSummaries(ctx, pool, sums); err != nil || n != len(sums) {
		t.Fatalf("wrote %d of %d: %v", n, len(sums), err)
	}
	want := claimSetSummaryTestRows(t, pool, true)
	if _, err := pool.Exec(ctx, `TRUNCATE jetsapi.claim_set_summary`); err != nil {
		t.Fatal(err)
	}

	// The operator, once per run, over the run's rows read as records.
	claimTables := map[string]pgx.Identifier{}
	for _, f := range feeds {
		if f.Profiled() && f.Profile.ClaimSet != nil {
			id, err := observe.ClaimSetTable(f.Profile.ClaimSet.Table)
			if err != nil {
				t.Fatal(err)
			}
			claimTables[f.TableName] = id
		}
	}
	for _, r := range runs {
		table, ok := claimTables[r.table]
		if !ok {
			continue
		}
		var exists bool
		if err := pool.QueryRow(ctx, `SELECT to_regclass($1) IS NOT NULL`, table.Sanitize()).Scan(&exists); err != nil {
			t.Fatal(err)
		}
		if !exists {
			continue
		}
		var columns []string
		if err := pool.QueryRow(ctx, `SELECT array_agg(column_name::text ORDER BY ordinal_position)
			FROM information_schema.columns WHERE table_schema = $1 AND table_name = $2 AND column_name <> 'session_id'`,
			table[0], table[1]).Scan(&columns); err != nil {
			t.Fatal(err)
		}
		quoted := make([]string, len(columns))
		for i, c := range columns {
			quoted[i] = pgx.Identifier{c}.Sanitize()
		}
		recs, err := pool.Query(ctx, fmt.Sprintf(`SELECT %s FROM %s WHERE session_id = $1 ORDER BY 1`,
			strings.Join(quoted, ", "), table.Sanitize()), r.session)
		if err != nil {
			t.Fatal(err)
		}
		records, err := pgx.CollectRows(recs, func(row pgx.CollectableRow) ([]any, error) { return row.Values() })
		if err != nil {
			t.Fatal(err)
		}
		bc := &BuilderContext{dbpool: pool, peKey: r.key, sessionId: r.session, done: make(chan struct{}),
			errCh: make(chan error, 1), env: map[string]any{}}
		if out := runClaimSetSummary(t, bc, columns, records); len(out) != len(records) {
			t.Errorf("run %d forwarded %d of %d records", r.key, len(out), len(records))
		}
	}
	got := claimSetSummaryTestRows(t, pool, false)

	keys := func(m map[string]map[string]any) []string {
		k := make([]string, 0, len(m))
		for x := range m {
			k = append(k, x)
		}
		sort.Strings(k)
		return k
	}
	if !slices.Equal(keys(got), keys(want)) {
		t.Fatalf("the operator wrote the summaries of\n  %v\nthe feed detection of\n  %v", keys(got), keys(want))
	}
	for _, k := range keys(want) {
		for col, w := range want[k] {
			if g := got[k][col]; !reflect.DeepEqual(g, w) {
				t.Errorf("%s %s: the operator's %v, the feed detection's %v", k, col, g, w)
			}
		}
		if len(got[k]) != len(want[k]) {
			t.Errorf("%s: %d columns, want %d", k, len(got[k]), len(want[k]))
		}
	}
	t.Logf("%d summaries, %d pipeline runs", len(want), len(runs))

	// The purge holds nothing a summary names.
	if held := claimSetSummaryHeld(t, pool); len(held) != 0 {
		t.Errorf("the purge still holds %v", held)
	}

	// The feed detection reads the operator's rows and computes none, with the
	// claim rows present and once they are gone, and reaches the same verdicts.
	same := func(label string, after []observe.ClaimSetVerdict) {
		if len(after) != len(fromRows) {
			t.Fatalf("%s: %d verdicts, %d from rows", label, len(after), len(fromRows))
		}
		for i := range fromRows {
			b, a := fromRows[i], after[i]
			if b.Detector != a.Detector || b.SessionID != a.SessionID || b.Verdict != a.Verdict ||
				b.Reason != a.Reason || b.Detail != a.Detail || !reflect.DeepEqual(b.Anomalies, a.Anomalies) {
				t.Errorf("%s: %s %s %s: from rows %s %q, from the operator's summaries %s %q\n  %s\n  %s", label,
					b.Detector, b.TableName, b.SessionID, b.Verdict, b.Reason, a.Verdict, a.Reason, b.Detail, a.Detail)
			}
		}
	}
	present, again, err := observe.SummariseClaimSets(ctx, pool, feeds, o)
	if err != nil {
		t.Fatal(err)
	}
	if len(again) != 0 {
		t.Errorf("with the operator's summaries stored, the feed detection computed %d", len(again))
	}
	same("rows present", present)
	for _, table := range claimTables {
		if _, err := pool.Exec(ctx, "DELETE FROM "+table.Sanitize()); err != nil && !strings.Contains(err.Error(), "42P01") {
			t.Fatal(err)
		}
	}
	gone, again, err := observe.SummariseClaimSets(ctx, pool, feeds, o)
	if err != nil {
		t.Fatal(err)
	}
	if len(again) != 0 {
		t.Errorf("with the rows gone, the feed detection computed %d", len(again))
	}
	same("rows gone", gone)
}

// What a run's summary is for, read through main_input_registry_key as a
// deployment records it: a run of a delivery is resolved to it; a run that
// re-processes a delivered file, a run of an unprofiled feed and a run whose
// main input is not a file are not, and say why.
func TestClaimSetSummaryResolvesOnlyADelivery(t *testing.T) {
	pool := claimSetSummaryTestFixture(t)
	ctx := context.Background()
	var irKey int
	var table, session string
	if err := pool.QueryRow(ctx, `SELECT ir.key, ir.table_name, ir.session_id
		FROM jetsapi.input_registry ir JOIN jetsapi.source_config sc ON sc.table_name = ir.table_name
		WHERE ir.source_type = 'file' AND sc.feed_profile_json IS NOT NULL AND ir.file_key IS NOT NULL
		ORDER BY ir.key LIMIT 1`).Scan(&irKey, &table, &session); err != nil {
		t.Fatal(err)
	}
	var unprofiledKey int
	if err := pool.QueryRow(ctx, `SELECT ir.key FROM jetsapi.input_registry ir
		JOIN jetsapi.source_config sc ON sc.table_name = ir.table_name
		WHERE ir.source_type = 'file' AND sc.feed_profile_json IS NULL ORDER BY ir.key LIMIT 1`).
		Scan(&unprofiledKey); err != nil {
		t.Fatal(err)
	}
	again := fmt.Sprint(time.Now().UnixMilli())
	var againKey int
	if err := pool.QueryRow(ctx, `INSERT INTO jetsapi.input_registry (client, org, object_type, file_key,
		  source_period_key, table_name, source_type, session_id, user_email)
		SELECT client, org, object_type, file_key, source_period_key, table_name, source_type, $2, user_email
		  FROM jetsapi.input_registry WHERE key = $1 RETURNING key`, irKey, again).Scan(&againKey); err != nil {
		t.Fatal(err)
	}
	var domainKey int
	if err := pool.QueryRow(ctx, `INSERT INTO jetsapi.input_registry (client, org, object_type, source_period_key,
		  table_name, source_type, session_id, user_email)
		SELECT client, org, object_type, source_period_key, table_name, 'domain_table', $2, user_email
		  FROM jetsapi.input_registry WHERE key = $1 RETURNING key`, irKey, again+"1").Scan(&domainKey); err != nil {
		t.Fatal(err)
	}
	pe := func(registryKey int) int {
		var key int
		if err := pool.QueryRow(ctx, `INSERT INTO jetsapi.pipeline_execution_status (pipeline_config_key,
			  main_input_registry_key, client, process_name, main_object_type, session_id, source_period_key, status,
			  user_email)
			VALUES (0, $1, 'c', 'p', 'o', $2, 0, 'in progress', 'u') RETURNING key`, registryKey,
			fmt.Sprintf("run-%d", registryKey)).
			Scan(&key); err != nil {
			t.Fatal(err)
		}
		return key
	}
	target, why, err := claimset.Resolve(ctx, pool, pe(irKey))
	if err != nil || why != "" || target.DeliverySession != session || target.Session != fmt.Sprintf("run-%d", irKey) ||
		target.Feed.TableName != table || target.Arrival.UnixMilli() == 0 {
		t.Errorf("a delivery's run: %+v %q %v", target, why, err)
	}
	for registryKey, says := range map[int]string{againKey: "re-processes delivery " + session,
		unprofiledKey: "no usable profile", domainKey: "no main input registered as a file"} {
		target, why, err := claimset.Resolve(ctx, pool, pe(registryKey))
		if err != nil || target != nil || !strings.Contains(why, says) {
			t.Errorf("registration %d: %+v %q %v; want it to say %q", registryKey, target, why, err, says)
		}
	}
}
