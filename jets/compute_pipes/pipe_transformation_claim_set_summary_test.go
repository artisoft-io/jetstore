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
	"log"
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

// claimSetSummaryTestChannel is a channel spec over columns, with its column
// map set.
func claimSetSummaryTestChannel(name string, columns []string) (*ChannelSpec, *map[string]int) {
	columnsMap := make(map[string]int, len(columns))
	for i, c := range columns {
		columnsMap[c] = i
	}
	spec := &ChannelSpec{Name: name, Columns: columns}
	spec.SetColumnsMap(&columnsMap)
	return spec, &columnsMap
}

// claimSetSummaryTestResolved resolves what pipeline execution peKey's
// summary is for, as the start of a step does, and returns the operator's
// configuration in the given mode.
func claimSetSummaryTestResolved(t *testing.T, pool *pgxpool.Pool, peKey int, mode string) *ClaimSetSummarySpec {
	t.Helper()
	steps := []PipeSpec{{Type: "fan_out", Apply: []TransformationSpec{{Type: ClaimSetSummaryOperatorType,
		ClaimSetSummaryConfig: &ClaimSetSummarySpec{Mode: mode}}}}}
	if err := resolveClaimSetSummaries(context.Background(), pool, peKey, steps); err != nil {
		t.Fatal(err)
	}
	return steps[0].Apply[0].ClaimSetSummaryConfig
}

// runClaimSetSummaryPartial runs the partial mode over records as a node of a
// fan_out would, with no database, and returns what reached its output
// channel.
func runClaimSetSummaryPartial(t *testing.T, config *ClaimSetSummarySpec, columns []string, records [][]any) [][]any {
	t.Helper()
	spec, columnsMap := claimSetSummaryTestChannel("claims", columns)
	pspec, pcolumns := claimSetSummaryTestChannel("partial", claimset.PartialColumns)
	registry := &ChannelRegistry{
		ComputeChannels: map[string]*Channel{
			"partial.out": {Name: "partial.out", Channel: make(chan []any), Columns: pcolumns, Config: pspec},
		},
		ClosedChannels: make(map[string]bool),
	}
	bc := &BuilderContext{done: make(chan struct{}), errCh: make(chan error, 1), env: map[string]any{},
		channelRegistry: registry}
	pipe, err := bc.BuildPipeTransformationEvaluator(&InputChannel{Name: "claims.in", Columns: columnsMap,
		Config: spec}, nil, nil, &TransformationSpec{Type: ClaimSetSummaryOperatorType,
		ClaimSetSummaryConfig: config, OutputChannel: OutputChannelConfig{Name: "partial.out"}})
	if err != nil {
		t.Fatal(err)
	}
	var out [][]any
	var wg sync.WaitGroup
	wg.Go(func() {
		for r := range registry.ComputeChannels["partial.out"].Channel {
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
	registry.CloseChannel("partial.out")
	wg.Wait()
	return out
}

// runClaimSetSummaryMerge runs the merge mode over partials as the one node of
// its step would. textual gives it every value as the text a staged csv file
// holds.
func runClaimSetSummaryMerge(t *testing.T, pool *pgxpool.Pool, config *ClaimSetSummarySpec, partials [][]any,
	textual bool) {
	t.Helper()
	spec, columnsMap := claimSetSummaryTestChannel("partial", claimset.PartialColumns)
	bc := &BuilderContext{dbpool: pool, done: make(chan struct{}), errCh: make(chan error, 1), env: map[string]any{},
		channelRegistry: &ChannelRegistry{ComputeChannels: map[string]*Channel{}, ClosedChannels: map[string]bool{}}}
	pipe, err := bc.BuildPipeTransformationEvaluator(&InputChannel{Name: "partial.in", Columns: columnsMap,
		Config: spec}, nil, nil, &TransformationSpec{Type: ClaimSetSummaryOperatorType, ClaimSetSummaryConfig: config})
	if err != nil {
		t.Fatal(err)
	}
	for _, p := range partials {
		r := slices.Clone(p)
		if textual {
			for i, v := range r {
				if v != nil {
					r[i] = fmt.Sprint(v)
				}
			}
		}
		if err := pipe.Apply(&r); err != nil {
			t.Fatal(err)
		}
	}
	if err := pipe.Done(); err != nil {
		t.Fatal(err)
	}
	pipe.Finally()
}

// claimSetSummaryShardOfMember shards a record by its member's text, as a
// splitter sharding on the member column does; a record with no member goes to
// shard 0.
func claimSetSummaryShardOfMember(member any, shards int) int {
	text, _ := claimSetSummaryText(member)
	if text == nil {
		return 0
	}
	return int(claimset.MemberHash(*text) % uint64(shards))
}

// runClaimSetSummarySharded summarises a run's records as a pipeline does:
// the start resolves the target, each of shards nodes summarises its records
// in partial mode, with no database, and one node merges the partials and
// writes the row. shardOf names each record's shard.
func runClaimSetSummarySharded(t *testing.T, pool *pgxpool.Pool, peKey int, columns []string, records [][]any,
	shards int, shardOf func(record []any) int, textual bool) {
	t.Helper()
	parts := make([][][]any, shards)
	for _, r := range records {
		k := shardOf(r)
		parts[k] = append(parts[k], r)
	}
	partial := claimSetSummaryTestResolved(t, pool, peKey, ClaimSetSummaryPartial)
	var partials [][]any
	for k := range parts {
		out := runClaimSetSummaryPartial(t, partial, columns, parts[k])
		if len(out) != 1 {
			t.Fatalf("shard %d emitted %d partials", k, len(out))
		}
		partials = append(partials, out...)
	}
	runClaimSetSummaryMerge(t, pool, claimSetSummaryTestResolved(t, pool, peKey, ClaimSetSummaryMerge), partials,
		textual)
}

// The partial mode emits exactly one record, its partial, whatever it read,
// and forwards none; a partial node is given no database. Without a resolved
// target, as when the start of the step resolved none, the partial carries why
// and counts nothing, and a merge of it writes nothing.
func TestClaimSetSummaryPartialEmitsOneRecord(t *testing.T) {
	columns := []string{"k", "m", "d"}
	records := [][]any{{"a", "m1", time.Now()}, {"b", nil, nil}, {[]byte("x"), "m2", "not a date"}}
	out := runClaimSetSummaryPartial(t, &ClaimSetSummarySpec{Mode: ClaimSetSummaryPartial}, columns, records)
	if len(out) != 1 {
		t.Fatalf("emitted %d records, want one partial", len(out))
	}
	pos := slices.Index(claimset.PartialColumns, claimset.PartialSkip)
	if why, _ := out[0][pos].(string); !strings.Contains(why, "resolved no feed") {
		t.Errorf("the partial's skip: %q", why)
	}
	p, err := claimset.PartialOf(func(c string) any { return out[0][slices.Index(claimset.PartialColumns, c)] })
	if err != nil || p.Rows != 0 {
		t.Errorf("the partial %+v (%v)", p, err)
	}

	// A resolved target with a data error on one record: the node gives up
	// and says why, and still emits its one partial.
	f := feedprofile.Feed{TableName: "t", Client: "c", Profile: feedprofile.Profile{DeclaredBy: "x",
		ClaimSet: &feedprofile.ClaimSet{Table: "claims", KeyColumns: []string{"k"}, ServiceDateColumn: "d",
			MemberColumn: "m"}}}
	resolved, _ := json.Marshal(claimset.Resolution{Target: &claimset.Target{Feed: f, DeliverySession: "1",
		Session: "s", Arrival: time.Now()}})
	config := &ClaimSetSummarySpec{Mode: ClaimSetSummaryPartial, Resolved: resolved}
	out = runClaimSetSummaryPartial(t, config, columns, records)
	if len(out) != 1 {
		t.Fatalf("emitted %d records, want one partial", len(out))
	}
	if why, _ := out[0][pos].(string); !strings.Contains(why, "the key column 'k' carries a []uint8") {
		t.Errorf("the partial's skip: %q", why)
	}
	out = runClaimSetSummaryPartial(t, config, columns, records[:2])
	p, err = claimset.PartialOf(func(c string) any { return out[0][slices.Index(claimset.PartialColumns, c)] })
	if err != nil || p.Skip != "" || p.Rows != 2 || p.Keys.Count != 2 || p.Members != 1 || p.DatedRows != 1 {
		t.Errorf("the partial %+v (%v)", p, err)
	}
}

// The partial is valid on any number of nodes and under a splitter; the merge
// only on one node, outside a splitter and with no output channel. The
// starters refuse a merge on more than one node, ValidatePipeSpecConfig
// refuses the rest, and a save warns of each.
func TestClaimSetSummaryNodes(t *testing.T) {
	step := func(mode string) []PipeSpec {
		return []PipeSpec{{Type: "fan_out", Apply: []TransformationSpec{{Type: "map_record"},
			{Type: ClaimSetSummaryOperatorType, ClaimSetSummaryConfig: &ClaimSetSummarySpec{Mode: mode}}}}}
	}
	for _, n := range []int{1, 3, 40} {
		if err := validateClaimSetSummaryNodes(step(ClaimSetSummaryPartial), n, "s"); err != nil {
			t.Errorf("a partial on %d nodes refused: %v", n, err)
		}
	}
	if err := validateClaimSetSummaryNodes(step(ClaimSetSummaryMerge), 1, "s"); err != nil {
		t.Errorf("a merge on one node refused: %v", err)
	}
	if err := validateClaimSetSummaryNodes(step(ClaimSetSummaryMerge), 3, "s"); err == nil ||
		!strings.Contains(err.Error(), "one node") {
		t.Errorf("a merge on three nodes: %v", err)
	}

	cpConfig := &ComputePipesConfig{Channels: []ChannelSpec{
		{Name: "claims", Columns: []string{"k", "m", "d"}},
		{Name: "partials", Columns: slices.Clone(claimset.PartialColumns)},
		{Name: "short", Columns: claimset.PartialColumns[1:]}}}
	cases := []struct {
		pipeType, mode, output, outputSpec, inputType, inputName string
		refused                                                  string
	}{
		{"fan_out", ClaimSetSummaryPartial, "p.out", "partials", "", "claims.in", ""},
		{"splitter", ClaimSetSummaryPartial, "p.out", "partials", "", "claims.in", ""},
		{"fan_out", ClaimSetSummaryPartial, "", "", "", "claims.in", "lacks"},
		{"fan_out", ClaimSetSummaryPartial, "p.out", "short", "", "claims.in", "claim_set_skip"},
		{"fan_out", ClaimSetSummaryMerge, "", "", "stage", "partials", ""},
		{"fan_out", ClaimSetSummaryMerge, "", "", "stage", "short", "claim_set_skip"},
		{"fan_out", ClaimSetSummaryMerge, "p.out", "partials", "stage", "partials", "no output_channel"},
		{"splitter", ClaimSetSummaryMerge, "", "", "stage", "partials", "splitter"},
		{"fan_out", "", "p.out", "partials", "", "claims.in", "not partial or merge"},
		{"fan_out", "whole", "p.out", "partials", "", "claims.in", "not partial or merge"},
	}
	for _, c := range cases {
		pipe := PipeSpec{Type: c.pipeType, InputChannel: InputChannelConfig{Name: c.inputName, Type: c.inputType},
			Apply: []TransformationSpec{{Type: ClaimSetSummaryOperatorType,
				ClaimSetSummaryConfig: &ClaimSetSummarySpec{Mode: c.mode},
				OutputChannel:         OutputChannelConfig{Name: c.output, SpecName: c.outputSpec}}}}
		err := validateClaimSetSummarySpec(cpConfig, &pipe, &pipe.Apply[0])
		if (c.refused == "") != (err == nil) || (err != nil && !strings.Contains(err.Error(), c.refused)) {
			t.Errorf("%+v: %v", c, err)
		}
	}
	missing := PipeSpec{Type: "fan_out", Apply: []TransformationSpec{{Type: ClaimSetSummaryOperatorType}}}
	if err := validateClaimSetSummarySpec(cpConfig, &missing, &missing.Apply[0]); err == nil ||
		!strings.Contains(err.Error(), "missing claim_set_summary_config") {
		t.Errorf("no configuration: %v", err)
	}
	// ValidatePipeSpecConfig calls it.
	args := &CpipesStartup{}
	pipeConfig := []PipeSpec{{Type: "splitter", InputChannel: InputChannelConfig{Name: "partials.in"},
		Apply: []TransformationSpec{{Type: ClaimSetSummaryOperatorType,
			ClaimSetSummaryConfig: &ClaimSetSummarySpec{Mode: ClaimSetSummaryMerge}}}}}
	if err := args.ValidatePipeSpecConfig(cpConfig, pipeConfig); err == nil || !strings.Contains(err.Error(), "splitter") {
		t.Errorf("a merge under a splitter: %v", err)
	}

	// A save warns of what the start refuses, and of nothing else.
	doc := `{"cluster_config": {}, "channels": [{"name": "partials", "columns": ` + claimSetSummaryJSON(claimset.PartialColumns) + `}],
	  "conditional_pipes_config": [{"pipes_config": [
		{"type": "splitter", "input_channel": {"name": "a"}, "apply": [{"type": "claim_set_summary",
		  "claim_set_summary_config": {"mode": "partial"}, "output_channel": {"name": "b", "channel_spec_name": "partials"}}]},
		{"type": "splitter", "input_channel": {"name": "b"}, "apply": [{"type": "claim_set_summary",
		  "claim_set_summary_config": {"mode": "merge"}}]}]}]}`
	findings := ValidateConfigDocument(doc)
	if len(findings) != 1 || findings[0].Code != "cpipes_config_claim_set_summary" ||
		findings[0].Path != "/conditional_pipes_config/0/pipes_config/1/apply/0" || findings[0].Severity != "warning" {
		t.Errorf("the save's findings: %+v", findings)
	}

	// Both starters call the node check with the count they start, and resolve
	// the target for the nodes.
	for file, count := range map[string]string{"actions_start_sharding_cp.go": "shardResult.nbrShardingNodes",
		"actions_start_reducing_cp.go": "len(partitions)"} {
		f, err := parser.ParseFile(token.NewFileSet(), file, nil, 0)
		if err != nil {
			t.Fatal(err)
		}
		found, resolves := false, false
		ast.Inspect(f, func(n ast.Node) bool {
			if call, ok := n.(*ast.CallExpr); ok {
				if id, ok := call.Fun.(*ast.Ident); ok && id.Name == "validateClaimSetSummaryNodes" && len(call.Args) == 3 {
					var b strings.Builder
					_ = astExprString(&b, call.Args[1])
					found = found || b.String() == count
				}
				if id, ok := call.Fun.(*ast.Ident); ok && id.Name == "resolveClaimSetSummaries" && len(call.Args) == 4 {
					var b strings.Builder
					_ = astExprString(&b, call.Args[2])
					resolves = resolves || b.String() == "args.PipelineExecKey"
				}
			}
			return true
		})
		if !found {
			t.Errorf("%s does not call validateClaimSetSummaryNodes with %s", file, count)
		}
		if !resolves {
			t.Errorf("%s does not call resolveClaimSetSummaries for the run", file)
		}
	}
}

func claimSetSummaryJSON(v any) string {
	b, _ := json.Marshal(v)
	return string(b)
}

// The layout the documentation shows, a partial beside the sort of a step
// sharded by member and a merge in the last step reading the partials one
// partition_writer wrote to one partition, is a configuration every step's
// start accepts, and its save raises no finding.
func TestClaimSetSummaryLayoutValidates(t *testing.T) {
	doc, err := os.ReadFile("testdata/claim_set_summary_sharded.pc.json")
	if err != nil {
		t.Fatal(err)
	}
	if findings := ValidateConfigDocument(string(doc)); len(findings) != 0 {
		t.Errorf("the save's findings: %+v", findings)
	}
	var cfg ComputePipesConfig
	if err := json.Unmarshal(doc, &cfg); err != nil {
		t.Fatal(err)
	}
	modes := 0
	for i, step := range cfg.ReducingPipesConfig {
		args := &CpipesStartup{CpConfig: cfg}
		if err := args.ValidatePipeSpecConfig(&args.CpConfig, step); err != nil {
			t.Errorf("step %d: %v", i, err)
		}
		for _, p := range step {
			for _, a := range p.Apply {
				if a.Type == ClaimSetSummaryOperatorType {
					modes++
				}
			}
		}
	}
	if modes != 2 {
		t.Errorf("%d claim_set_summary operators, want a partial and a merge", modes)
	}
	// The partition_writer that carries the partials writes one partition.
	pw := cfg.ReducingPipesConfig[1][2].Apply[0]
	if pw.PartitionWriterConfig == nil || pw.PartitionWriterConfig.JetsPartitionKey == nil {
		t.Error("the partials' partition_writer has no fixed jets_partition_key")
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

// claimSetSummaryTestRecords are a session's rows of a claim table as the
// records a pipeline would carry, every column but session_id; nil columns
// when the table does not exist.
func claimSetSummaryTestRecords(t *testing.T, pool *pgxpool.Pool, table pgx.Identifier,
	session string) ([]string, [][]any) {
	t.Helper()
	ctx := context.Background()
	var exists bool
	if err := pool.QueryRow(ctx, `SELECT to_regclass($1) IS NOT NULL`, table.Sanitize()).Scan(&exists); err != nil {
		t.Fatal(err)
	}
	if !exists {
		return nil, nil
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
		strings.Join(quoted, ", "), table.Sanitize()), session)
	if err != nil {
		t.Fatal(err)
	}
	records, err := pgx.CollectRows(recs, func(row pgx.CollectableRow) ([]any, error) { return row.Values() })
	if err != nil {
		t.Fatal(err)
	}
	return columns, records
}

// The operator over each delivery's records, as a pipeline run that does not
// load them would see them, writes the row the feed detection computes from
// the table, field by field, for every delivery of every profiled feed of the
// fixture. Then the feed detection reads those rows and computes none, with the
// rows present and with them gone, and reaches the verdicts it reaches from
// the rows; and the table-data purge, which held every run's claim rows until
// a summary named it, holds none.
func TestClaimSetSummaryOperatorWritesTheFeedDetectionsRow(t *testing.T) {
	claimSetSummaryAgainstFeedDetection(t, true)
}

// The same on a summary table the migration adding computed_by has not
// reached, as when a deployment's code runs before its migration: the feed
// detection and the operator's merge both write without the column, and the
// feed detection reads every row as its own and judges as it does from rows,
// comparisons included.
func TestClaimSetSummaryWritesWithoutThePathColumn(t *testing.T) {
	claimSetSummaryAgainstFeedDetection(t, false)
}

func claimSetSummaryAgainstFeedDetection(t *testing.T, migrated bool) {
	pool := claimSetSummaryTestFixture(t)
	ctx := context.Background()
	if !migrated {
		if _, err := pool.Exec(ctx, `ALTER TABLE jetsapi.claim_set_summary DROP COLUMN computed_by`); err != nil {
			t.Fatal(err)
		}
	}
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

	// The operator, once per run, over the run's rows read as records: in
	// partial mode on three nodes, the records sharded by member, then in
	// merge mode on one, every other run reading its partials as the text of
	// a staged file.
	claimTables := map[string]pgx.Identifier{}
	memberColumns := map[string]string{}
	for _, f := range feeds {
		if f.Profiled() && f.Profile.ClaimSet != nil {
			id, err := observe.ClaimSetTable(f.Profile.ClaimSet.Table)
			if err != nil {
				t.Fatal(err)
			}
			claimTables[f.TableName] = id
			memberColumns[f.TableName] = f.Profile.ClaimSet.MemberColumn
		}
	}
	const shards = 3
	sharded := 0
	for i, r := range runs {
		table, ok := claimTables[r.table]
		if !ok {
			continue
		}
		columns, records := claimSetSummaryTestRecords(t, pool, table, r.session)
		if columns == nil {
			continue
		}
		member := slices.Index(columns, memberColumns[r.table])
		used := map[int]bool{}
		shardOf := func(record []any) int {
			k := 0
			if member >= 0 {
				k = claimSetSummaryShardOfMember(record[member], shards)
			}
			used[k] = true
			return k
		}
		runClaimSetSummarySharded(t, pool, r.key, columns, records, shards, shardOf, i%2 == 1)
		if len(used) > 1 {
			sharded++
		}
	}
	if sharded == 0 {
		t.Fatal("no run's records were spread over more than one shard; the test asserts nothing of the merge")
	}
	got := claimSetSummaryTestRows(t, pool, false)
	// Every column but the path that computed the row is the same; without
	// the column, neither writer wrote one.
	for label, rows := range map[string]map[string]map[string]any{"database": want, "pipeline": got} {
		for k := range rows {
			by, has := rows[k]["computed_by"]
			if has != migrated || (migrated && by != label) {
				t.Errorf("%s: the %s path's row has computed_by %v (present %v)", k, label, by, has)
			}
			delete(rows[k], "computed_by")
		}
	}
	t.Logf("%d of the runs spread over more than one shard", sharded)

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

// A registration is a delivery for the operator exactly when it is one for the
// feed detection, purged first registrations included: over every file
// registration of every profiled feed, with one file whose first registration
// a session deletion removed before its surviving registrations arrived, and
// one whose record is later than its first surviving registration and so
// changes nothing. A run re-processing the purged file writes no row.
func TestClaimSetSummaryReadsDeliveriesAsTheFeedDetectionDoes(t *testing.T) {
	pool := claimSetSummaryTestFixture(t)
	ctx := context.Background()
	type reg struct {
		key                     int
		table, fileKey, session string
	}
	var regs []reg
	rows, err := pool.Query(ctx, `SELECT ir.key, ir.table_name, coalesce(ir.file_key, ''), ir.session_id
		FROM jetsapi.input_registry ir JOIN jetsapi.source_config sc ON sc.table_name = ir.table_name
		WHERE ir.source_type = 'file' AND sc.feed_profile_json IS NOT NULL ORDER BY ir.key`)
	if err != nil {
		t.Fatal(err)
	}
	for rows.Next() {
		var r reg
		if err := rows.Scan(&r.key, &r.table, &r.fileKey, &r.session); err != nil {
			t.Fatal(err)
		}
		regs = append(regs, r)
	}
	if rows.Err() != nil || len(regs) < 2 {
		t.Fatalf("%d registrations (%v)", len(regs), rows.Err())
	}
	tables := map[string]bool{}
	for _, r := range regs {
		tables[r.table] = true
	}
	var names []string
	for n := range tables {
		names = append(names, n)
	}
	deliveries := func() map[string]bool {
		feedRegs, err := observe.ReadFeedRegistrations(ctx, pool, names)
		if err != nil {
			t.Fatal(err)
		}
		out := map[string]bool{}
		for _, n := range names {
			for _, d := range feedRegs.Deliveries(n, time.Time{}) {
				out[n+" "+d.SessionId] = true
			}
		}
		return out
	}
	// Two deliveries of files with a key: the first and the last.
	var candidates []reg
	before := deliveries()
	for _, r := range regs {
		if r.fileKey != "" && before[r.table+" "+r.session] {
			candidates = append(candidates, r)
		}
	}
	if len(candidates) < 2 {
		t.Fatalf("%d deliveries of a file with a key", len(candidates))
	}
	gone, later := candidates[0], candidates[len(candidates)-1]
	ms := func(s string) int64 {
		var v int64
		fmt.Sscan(s, &v)
		return v
	}
	for _, p := range []struct {
		r  reg
		at int64
	}{{gone, ms(gone.session) - 86400000}, {later, ms(later.session) + 1}} {
		if _, err := pool.Exec(ctx, `INSERT INTO jetsapi.purged_delivery (table_name, file_key, first_session_id,
			first_arrival, deletion_reason) VALUES ($1, $2, $3::text, 'epoch'::timestamptz + $3::bigint * interval '1 millisecond',
			'retention')`, p.r.table, p.r.fileKey, fmt.Sprint(p.at)); err != nil {
			t.Fatal(err)
		}
	}

	delivery := deliveries()
	if delivery[gone.table+" "+gone.session] || !delivery[later.table+" "+later.session] {
		t.Fatalf("the feed detection reads the purged file's registration as a delivery %v, the later record's %v",
			delivery[gone.table+" "+gone.session], delivery[later.table+" "+later.session])
	}
	resolved := 0
	for _, r := range regs {
		var pe int
		if err := pool.QueryRow(ctx, `INSERT INTO jetsapi.pipeline_execution_status (pipeline_config_key,
			  main_input_registry_key, client, process_name, main_object_type, session_id, source_period_key, status,
			  user_email)
			VALUES (0, $1, 'c', 'p', 'o', $2, 0, 'completed', 'u') RETURNING key`, r.key,
			fmt.Sprintf("run-%d", r.key)).Scan(&pe); err != nil {
			t.Fatal(err)
		}
		target, why, err := claimset.Resolve(ctx, pool, pe)
		if err != nil {
			t.Fatal(err)
		}
		if (target != nil) != delivery[r.table+" "+r.session] {
			t.Errorf("registration %s of %s: the operator reads it as a delivery %v (%s), the feed detection %v",
				r.session, r.table, target != nil, why, delivery[r.table+" "+r.session])
		}
		if target != nil {
			resolved++
		}
		if r == gone && !strings.Contains(why, "was deleted") {
			t.Errorf("the purged file's registration: %q", why)
		}
	}
	if resolved == 0 {
		t.Fatal("no registration resolved; the comparison asserts nothing")
	}

	// The fixture's own run of the purged file's registration writes no row.
	var runKey int
	var runSession string
	if err := pool.QueryRow(ctx, `SELECT key, session_id FROM jetsapi.pipeline_execution_status
		WHERE input_session_id = $1`, gone.session).Scan(&runKey, &runSession); err != nil {
		t.Fatal(err)
	}
	var claimTable string
	if err := pool.QueryRow(ctx, `SELECT feed_profile_json -> 'claim_set' ->> 'table' FROM jetsapi.source_config
		WHERE table_name = $1`, gone.table).Scan(&claimTable); err != nil {
		t.Fatal(err)
	}
	id, err := observe.ClaimSetTable(claimTable)
	if err != nil {
		t.Fatal(err)
	}
	columns, records := claimSetSummaryTestRecords(t, pool, id, runSession)
	if len(records) == 0 {
		t.Fatalf("the purged file's run %s has no claim rows to summarise", runSession)
	}
	runClaimSetSummarySharded(t, pool, runKey, columns, records, 2, func(r []any) int { return len(fmt.Sprint(r...)) % 2 },
		false)
	var n int
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM jetsapi.claim_set_summary`).Scan(&n); err != nil {
		t.Fatal(err)
	}
	if n != 0 {
		t.Errorf("a run re-processing a purged file wrote %d summaries", n)
	}
}

// The merge writes nothing where the shards were not disjoint, and logs which
// of the keys and the members were split: a key that also arrives under a
// member of another shard, and records sharded by claim rather than by member.
// Sharded by member, the same records are written.
func TestClaimSetSummaryMergeRefusesSplitShards(t *testing.T) {
	pool := claimSetSummaryTestFixture(t)
	ctx := context.Background()
	if _, err := pool.Exec(ctx, `TRUNCATE jetsapi.claim_set_summary`); err != nil {
		t.Fatal(err)
	}
	f := feedprofile.Feed{TableName: "split_test", Client: "c", Profile: feedprofile.Profile{DeclaredBy: "x",
		ClaimSet: &feedprofile.ClaimSet{Table: "claims", KeyColumns: []string{"claim_id", "line"},
			ServiceDateColumn: "dos", MemberColumn: "member"}}}
	resolved, err := json.Marshal(claimset.Resolution{Target: &claimset.Target{Feed: f, DeliverySession: "1700000000000",
		Session: "run-split", Arrival: time.UnixMilli(1700000000000).UTC()}})
	if err != nil {
		t.Fatal(err)
	}
	columns := []string{"claim_id", "line", "member", "dos"}
	var records [][]any
	for m := range 200 {
		for c := range 3 {
			records = append(records, []any{fmt.Sprintf("c%d-%d", m, c), int64(1), fmt.Sprintf("m%d", m),
				time.Date(2026, 3, 1+c, 0, 0, 0, 0, time.UTC)})
		}
	}
	byMember := func(r []any) int { return claimSetSummaryShardOfMember(r[2], 3) }
	byClaim := func(r []any) int { return int(claimset.MemberHash(r[0].(string)) % 3) }
	// 40 keys that also arrive under a member of another shard.
	split := slices.Clone(records)
	for i := 0; len(split) < len(records)+40; i++ {
		r, other := records[3*i], records[len(records)-1-3*i]
		if byMember(r) != byMember(other) {
			split = append(split, []any{r[0], r[1], other[2], r[3]})
		}
	}
	run := func(records [][]any, shardOf func([]any) int) (rows int, logged string) {
		parts := make([][][]any, 3)
		for _, r := range records {
			parts[shardOf(r)] = append(parts[shardOf(r)], r)
		}
		var partials [][]any
		for _, p := range parts {
			partials = append(partials, runClaimSetSummaryPartial(t, &ClaimSetSummarySpec{
				Mode: ClaimSetSummaryPartial, Resolved: resolved}, columns, p)...)
		}
		var buf strings.Builder
		log.SetOutput(&buf)
		runClaimSetSummaryMerge(t, pool, &ClaimSetSummarySpec{Mode: ClaimSetSummaryMerge, Resolved: resolved},
			partials, true)
		log.SetOutput(os.Stderr)
		if err := pool.QueryRow(ctx, `SELECT count(*) FROM jetsapi.claim_set_summary`).Scan(&rows); err != nil {
			t.Fatal(err)
		}
		return rows, buf.String()
	}
	if rows, logged := run(split, byMember); rows != 0 || !strings.Contains(logged, "distinct keys sum to") ||
		strings.Contains(logged, "members sum to") {
		t.Errorf("a key on two shards: %d rows written, logged %s", rows, logged)
	}
	if rows, logged := run(records, byClaim); rows != 0 || strings.Contains(logged, "distinct keys sum to") ||
		!strings.Contains(logged, "members sum to") {
		t.Errorf("a member on two shards: %d rows written, logged %s", rows, logged)
	}
	if rows, logged := run(records, byMember); rows != 1 {
		t.Errorf("sharded by member: %d rows written, logged %s", rows, logged)
	}
	var got struct {
		rows, keys, members int64
		by                  string
	}
	if err := pool.QueryRow(ctx, `SELECT row_count, distinct_key_count, member_count, computed_by
		FROM jetsapi.claim_set_summary`).Scan(&got.rows, &got.keys, &got.members, &got.by); err != nil {
		t.Fatal(err)
	}
	if got.rows != 600 || got.keys != 600 || got.members != 200 || got.by != claimset.ComputedByPipeline {
		t.Errorf("the row written: %+v", got)
	}
}
