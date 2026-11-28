package observe

// Package claimset's summary from records against this package's summary from
// rows. TestClaimSetSummariserMatchesTheStatement needs JETS_TEST_DSN; run
// with -count=1.
//
//	JETS_TEST_DSN=postgres://postgres:pw@localhost:5463/postgres \
//	  go test -count=1 -run 'ClaimSetSummariser|ClaimSetPackage' ./jets/agentic/observe/

import (
	"context"
	"fmt"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/artisoft-io/jetstore/jets/agentic/feedprofile"
	"github.com/artisoft-io/jetstore/jets/claimset"
	"github.com/jackc/pgx/v5"
)

// The same rows, read by the statement and given to the summariser as their
// texts, make the same summary, field by field: a large delivery with exact
// dates, a fingerprint and a sketch; a small one, dated by month and with
// neither; and one with no date of service. The present counts are the same
// for every column given, the table's session_id being the statement's alone.
// So do the same records split over three shards by member, each summarised
// as a partial and the partials added. The row says which path computed it,
// and is otherwise the same.
func TestClaimSetSummariserMatchesTheStatement(t *testing.T) {
	pool := claimSetSummaryScratch(t, "observe_claimset_records_test")
	ctx := context.Background()
	if _, err := pool.Exec(ctx, `CREATE SCHEMA jetsapi;
		CREATE TABLE jetsapi.input_registry (key serial PRIMARY KEY, session_id text, source_type text);
		CREATE TABLE jetsapi.pipeline_execution_status (key int PRIMARY KEY, main_input_registry_key int,
		  input_session_id text, session_id text NOT NULL, status text NOT NULL);
		CREATE TABLE public.claims (session_id text NOT NULL, "claim id" text, line integer, d date, m text,
		  note text);
		INSERT INTO public.claims SELECT 'L', 'c-' || (i / 3), i % 3,
		  CASE WHEN i % 17 = 0 THEN NULL ELSE DATE '2026-03-31' - (i % 800) END,
		  CASE WHEN i % 23 = 0 THEN NULL ELSE 'm' || (i % 211) END,
		  CASE i % 4 WHEN 0 THEN NULL WHEN 1 THEN '' WHEN 2 THEN '  ' ELSE 'n' END
		  FROM generate_series(0, 9999) i;
		INSERT INTO public.claims VALUES ('L', NULL, 1, NULL, 'x', E'\t'), ('L', '', 1, DATE '2024-01-01', NULL, NULL);
		INSERT INTO public.claims SELECT 'S', 'c-' || i, 1, DATE '2026-03-31' - i, 'm' || (i % 2), 'n'
		  FROM generate_series(1, 7) i;
		INSERT INTO public.claims SELECT 'U', 'c-' || i, 1, NULL, 'm' || i, NULL FROM generate_series(1, 30) i`); err != nil {
		t.Fatal(err)
	}
	lag, span := 45, 90
	cs := &feedprofile.ClaimSet{Table: "claims", KeyColumns: []string{"claim id", "line"}, ServiceDateColumn: "d",
		MemberColumn: "m"}
	f := feedprofile.Feed{TableName: "claims", Client: "c", Profile: feedprofile.Profile{ClaimSet: cs,
		MaxServiceDateLagDays: &lag, MaxServiceDateSpanDays: &span}}
	columns := []string{"session_id", "claim id", "line", "d", "m", "note"}
	stmt := ClaimSetSummaryStatement(pgx.Identifier{"public", "claims"}, cs, columns)
	o := ClaimSetOptions{MinMembers: 11, Now: time.Date(2026, 4, 2, 0, 0, 0, 0, time.UTC)}
	arrival := time.Date(2026, 5, 20, 13, 0, 0, 0, time.UTC)

	for _, session := range []string{"L", "S", "U"} {
		want, err := claimSetSummarise(ctx, pool, stmt, columns, f, claimSetDelivery{session: session,
			arrival: &arrival}, o)
		if err != nil {
			t.Fatal(err)
		}
		given := columns[1:]
		s := claimset.NewSummariser(f, given, o.MinMembers)
		var shards [3]*claimset.Summariser
		for i := range shards {
			shards[i] = claimset.NewSummariser(f, given, o.MinMembers)
		}
		rows, err := pool.Query(ctx, `SELECT "claim id", line::text, d, m, note FROM public.claims
			WHERE session_id = $1`, session)
		if err != nil {
			t.Fatal(err)
		}
		for rows.Next() {
			var id, line, m, note *string
			var d *time.Time
			if err := rows.Scan(&id, &line, &d, &m, &note); err != nil {
				t.Fatal(err)
			}
			present := func(v *string) bool { return v != nil && trimSpaces(*v) != "" }
			r := claimset.Record{Key: []*string{id, line}, Member: m, ServiceDate: d,
				Present: []bool{present(id), present(line), d != nil, present(m), present(note)}}
			s.Add(r)
			shard := 0
			if m != nil {
				shard = int(claimset.MemberHash(*m) % 3)
			}
			shards[shard].Add(r)
		}
		if err := rows.Err(); err != nil {
			t.Fatal(err)
		}
		delete(want.Present, "session_id")
		whole := &claimset.Partial{}
		for _, sh := range shards {
			whole.Add(sh.Partial())
		}
		for label, c := range map[string]*claimset.Summary{
			"one summariser": s.Summary(session, session, &arrival, o.Now),
			"three shards":   whole.Summary(f, o.MinMembers, session, session, &arrival, o.Now),
		} {
			got := fromClaimSetPackage(c)
			if got.ComputedBy != ClaimSetComputedByPipeline || want.ComputedBy != ClaimSetComputedByDatabase {
				t.Errorf("session %s, %s: computed by %q, the statement's by %q", session, label, got.ComputedBy,
					want.ComputedBy)
			}
			got.ComputedBy = want.ComputedBy
			if !reflect.DeepEqual(got, want) {
				for _, d := range claimSetSummaryDiff(got, want) {
					t.Errorf("session %s, %s: %s", session, label, d)
				}
			}
		}
		if session == "L" && whole.Overlap() != "" {
			t.Errorf("shards by member read as not disjoint: %s", whole.Overlap())
		}
	}
}

// fromClaimSetPackage is claimset's summary as this package's, field by field
// of the same name: TestClaimSetPackageWritesTheSameRow holds the two types to
// the same fields.
func fromClaimSetPackage(c *claimset.Summary) *ClaimSetSummary {
	if c == nil {
		return nil
	}
	out := &ClaimSetSummary{sketchRead: true, settled: true}
	src, dst := reflect.ValueOf(*c), reflect.ValueOf(out).Elem()
	for i := range src.NumField() {
		name := src.Type().Field(i).Name
		if name == "Histogram" {
			if c.Histogram != nil {
				out.Histogram = &ClaimSetHistogram{BoundsDays: c.Histogram.BoundsDays, Rows: c.Histogram.Rows}
			}
			continue
		}
		dst.FieldByName(name).Set(src.Field(i))
	}
	return out
}

// Package claimset writes the row this package writes: the same format
// version, histogram bounds and minimum, the same exported fields of the same
// types but the judged mark, which no summary's writer sets, and the same
// statement, character for character. A change to the row
// here fails this test until claimset makes it too.
func TestClaimSetPackageWritesTheSameRow(t *testing.T) {
	if claimset.Version != ClaimSetSummaryVersion {
		t.Errorf("claimset writes version %d, this package %d", claimset.Version, ClaimSetSummaryVersion)
	}
	if !reflect.DeepEqual(claimset.HistogramBounds, ClaimSetHistogramBounds) {
		t.Errorf("the histogram bounds differ: %v and %v", claimset.HistogramBounds, ClaimSetHistogramBounds)
	}
	if claimset.DefaultMinMembers != DefaultClaimSetOptions().MinMembers {
		t.Errorf("the minimum differs: %d and %d", claimset.DefaultMinMembers, DefaultClaimSetOptions().MinMembers)
	}
	if claimset.UpsertSQL != claimSetUpsertSQL {
		t.Error("claimset.UpsertSQL is not this package's claimSetUpsertSQL")
	}
	// A table the migration has not reached is written without computed_by,
	// by both, and both ask the same question to tell.
	if claimset.UpsertWithoutPathSQL != claimSetUpsertWithoutPathSQL {
		t.Error("claimset.UpsertWithoutPathSQL is not this package's claimSetUpsertWithoutPathSQL")
	}
	if claimset.PathColumnSQL != claimSetPathColumnSQL {
		t.Error("claimset.PathColumnSQL is not this package's claimSetPathColumnSQL")
	}
	if strings.Contains(claimSetUpsertWithoutPathSQL, "computed_by") || !strings.Contains(claimSetUpsertSQL, "computed_by") ||
		strings.Replace(strings.Replace(strings.Replace(claimSetUpsertSQL, ", computed_by)", ")", 1), ", $26)", ")", 1),
			", computed_by = EXCLUDED.computed_by", "", 1) != claimSetUpsertWithoutPathSQL {
		t.Error("the two upserts differ in more than computed_by")
	}
	// Fields this package reads that the summary's writer does not write, and
	// why a pipeline's summary has none.
	notWritten := map[string]string{
		"JudgedAt": "claim_set_judged_at is set by WriteClaimSetJudged when a run judges the delivery, not by the upsert",
	}
	ours, theirs := reflect.TypeOf(ClaimSetSummary{}), reflect.TypeOf(claimset.Summary{})
	for i := range ours.NumField() {
		f := ours.Field(i)
		if !f.IsExported() || notWritten[f.Name] != "" {
			continue
		}
		g, ok := theirs.FieldByName(f.Name)
		switch {
		case !ok:
			t.Errorf("claimset.Summary has no field %s", f.Name)
		case f.Name == "Histogram":
			if !reflect.DeepEqual(reflect.VisibleFields(g.Type.Elem()), reflect.VisibleFields(f.Type.Elem())) {
				t.Error("the histograms differ")
			}
		case g.Type != f.Type:
			t.Errorf("%s is a %s here and a %s in claimset", f.Name, f.Type, g.Type)
		}
	}
	for i := range theirs.NumField() {
		if _, ok := ours.FieldByName(theirs.Field(i).Name); !ok {
			t.Errorf("claimset.Summary has a field %s this package's has not", theirs.Field(i).Name)
		}
	}
}

func trimSpaces(s string) string {
	for len(s) > 0 && s[0] == ' ' {
		s = s[1:]
	}
	for len(s) > 0 && s[len(s)-1] == ' ' {
		s = s[:len(s)-1]
	}
	return s
}

// claimSetSummaryDiff names the fields in which two summaries differ.
func claimSetSummaryDiff(got, want *ClaimSetSummary) []string {
	if got == nil || want == nil {
		return []string{fmt.Sprintf("records %v, rows %v", got != nil, want != nil)}
	}
	var out []string
	g, w := reflect.ValueOf(*got), reflect.ValueOf(*want)
	for i := range g.NumField() {
		name := g.Type().Field(i).Name
		if !g.Type().Field(i).IsExported() {
			continue
		}
		if !reflect.DeepEqual(g.Field(i).Interface(), w.Field(i).Interface()) {
			out = append(out, fmt.Sprintf("%s: from records %s, from rows %s", name, show(g.Field(i)), show(w.Field(i))))
		}
	}
	return out
}

func show(v reflect.Value) string {
	if v.Kind() == reflect.Pointer && !v.IsNil() {
		return fmt.Sprintf("%v", v.Elem().Interface())
	}
	if v.Kind() == reflect.Slice && v.Type().Elem().Kind() == reflect.Uint8 {
		return fmt.Sprintf("%d bytes", v.Len())
	}
	return fmt.Sprintf("%v", v.Interface())
}
