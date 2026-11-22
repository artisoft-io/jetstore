package observe

import (
	"strings"
	"testing"
	"time"

	"github.com/artisoft-io/jetstore/jets/agentic/feedprofile"
)

// The one function that turns arguments into text: literals in place of the
// parameters, quoted names and strings left alone, one line, and no semicolon
// from a value.
func TestFeedProcessRenderWritesLiterals(t *testing.T) {
	n := 45
	var none *int
	sql := `SELECT "a$1;b", 'it''s $2'
  FROM t
 WHERE x = $1 AND y = ANY($2) AND z = $3::int AND w = $4 AND v = $5`
	got, err := FeedProcessRender(sql, []any{"o'k;go\nnow", []string{"p", "q;r"}, &n, none, int64(7)})
	if err != nil {
		t.Fatal(err)
	}
	want := `SELECT "a$1;b", 'it''s $2' FROM t WHERE x = ('o''k' || chr(59) || 'go' || chr(10) || 'now') ` +
		`AND y = ANY(ARRAY['p', ('q' || chr(59) || 'r')]::text[]) AND z = 45::int AND w = NULL AND v = 7`
	if got != want {
		t.Errorf("rendered\n%s\nwant\n%s", got, want)
	}
	for _, bad := range []struct {
		sql  string
		args []any
		want string
	}{
		{"SELECT $1", []any{1, 2}, "$2 is not used"},
		{"SELECT $3", []any{1}, "names $3"},
		{"SELECT 1 -- note", nil, "line comment"},
		{"SELECT 'open", nil, "unterminated"},
		{"SELECT $1", []any{struct{}{}}, "cannot be written"},
	} {
		if _, err := FeedProcessRender(bad.sql, bad.args); err == nil || !strings.Contains(err.Error(), bad.want) {
			t.Errorf("%q: %v, want an error saying %q", bad.sql, err, bad.want)
		}
	}
}

// A result line is read by position, so two columns of one name keep both
// values; lines that are not results are skipped, and so is a second result
// for one statement.
func TestFeedProcessResultsAreReadByPosition(t *testing.T) {
	text := `-- a comment
"{"id" : "s1", "rows" : [{"n":1,"n":2,"p":[3,4],"b":null,"d":"2026-01-01"}]}"
{"id" : "s2", "rows" : []}
{"id" : "s1", "rows" : [{"n":9}]}
`
	r, err := ReadFeedProcessResults(strings.NewReader(text))
	if err != nil {
		t.Fatal(err)
	}
	if len(r) != 2 || len(r["s1"]) != 1 || len(r["s1"][0]) != 5 || len(r["s2"]) != 0 {
		t.Fatalf("read %v", r)
	}
	var a, b int64
	var p []int64
	var lag *int64
	var d string
	if err := feedProcessRow(r["s1"][0]).Scan(&a, &b, &p, &lag, &d); err != nil {
		t.Fatal(err)
	}
	if a != 1 || b != 2 || len(p) != 2 || p[1] != 4 || lag != nil || d != "2026-01-01" {
		t.Errorf("scanned %d %d %v %v %q", a, b, p, lag, d)
	}
	var x int64
	if err := feedProcessRow(r["s1"][0]).Scan(&x); err == nil {
		t.Error("a row of five columns scanned into one")
	}
	if _, err := ReadFeedProcessResults(strings.NewReader(`{"rows": []}`)); err == nil {
		t.Error("a result naming no statement was read")
	}
}

// A replay with no results records what it is asked, once each, and answers
// with nothing, so a round of the export can see everything it needs.
func TestFeedProcessReplayRecordsWhatItCannotAnswer(t *testing.T) {
	r := NewFeedProcessReplay(nil)
	rows, err := r.Query(t.Context(), "SELECT a FROM t WHERE b = $1", "x")
	if err != nil || rows.Next() {
		t.Fatalf("an unanswered query: %v", err)
	}
	r.Query(t.Context(), "SELECT a FROM t WHERE b = $1", "x")
	var n int64
	var ok *bool
	if err := r.QueryRow(t.Context(), "SELECT count(*), true WHERE $1", true).Scan(&n, &ok); err != nil || n != 0 || ok != nil {
		t.Fatalf("an unanswered row: %v %d %v", err, n, ok)
	}
	if p := r.Pending(); len(p) != 2 || p[0].Text != "SELECT a FROM t WHERE b = 'x'" {
		t.Errorf("pending %+v", p)
	}
	s := r.Pending()[1]
	if !strings.Contains(s.Wrapped(), "'id', '"+s.ID+"'") || strings.Contains(s.Wrapped(), ";") {
		t.Errorf("wrapped %s", s.Wrapped())
	}
	answered := NewFeedProcessReplay(FeedProcessResults{s.ID: {}})
	if err := answered.QueryRow(t.Context(), "SELECT count(*), true WHERE $1", true).Scan(&n, &ok); err == nil {
		t.Error("a statement answered with no rows scanned a row")
	}
}

func TestFeedProcessPartsKeepToTheirSize(t *testing.T) {
	var stmts []FeedProcessStatement
	for i := range 10 {
		stmts = append(stmts, FeedProcessStatement{ID: strings.Repeat("a", 16), Text: "SELECT " + strings.Repeat("1", 100+i)})
	}
	parts := FeedProcessParts(stmts, 900)
	if len(parts) < 3 {
		t.Fatalf("%d parts", len(parts))
	}
	total := 0
	for _, p := range parts {
		total += len(p.Statements)
		if len(p.Text) > 900 && len(p.Statements) > 1 {
			t.Errorf("a part of %d statements is %d bytes", len(p.Statements), len(p.Text))
		}
		if !strings.Contains(p.Text, "VALUES (1, (") {
			t.Errorf("a part does not number from 1: %.60s", p.Text)
		}
	}
	if total != 10 {
		t.Errorf("%d statements in the parts", total)
	}
	if one := FeedProcessParts(stmts, 10); len(one) != 10 {
		t.Errorf("statements longer than the bound: %d parts, want one each", len(one))
	}
}

func TestFeedProcessStepsReplayEachDay(t *testing.T) {
	asOf := time.Date(2026, 6, 1, 5, 0, 0, 0, time.UTC)
	o := DefaultFeedProcessOptions(asOf)
	if s := o.Steps(); len(s) != 1 || !s[0].Equal(asOf) {
		t.Errorf("a window: %v", s)
	}
	o.Since, o.Step = asOf.Add(-72*time.Hour), 24*time.Hour
	s := o.Steps()
	if len(s) != 4 || !s[0].Equal(o.Since) || !s[3].Equal(asOf) {
		t.Errorf("three days back: %v", s)
	}
	if at := o.detectedAt(s, asOf.Add(-49*time.Hour)); !at.Equal(asOf.Add(-48 * time.Hour)) {
		t.Errorf("a delivery 49 hours before is first seen at %s", at)
	}
}

func TestFeedProcessExportRefusalsNameTheFeedAndField(t *testing.T) {
	share := 0.9
	feeds := []feedprofile.Feed{
		{TableName: "fine", Profile: feedprofile.Profile{ClaimSet: &feedprofile.ClaimSet{Table: "s.t",
			KeyColumns: []string{"k"}, ServiceDateColumn: "d", MemberColumn: "m"}}},
		{TableName: "bad", Profile: feedprofile.Profile{ClaimSet: &feedprofile.ClaimSet{Table: "t;x",
			KeyColumns: []string{"k;"}, ServiceDateColumn: "d", MemberColumn: "m"},
			RequiredFields: []feedprofile.RequiredField{{Column: "c;", MinPresentShare: &share}}}},
		{TableName: "none"},
	}
	got := FeedProcessExportRefusals(feeds)
	if len(got) != 3 || !strings.HasPrefix(got[0], "feed bad: claim_set.table") ||
		!strings.Contains(got[2], "required_fields.column") {
		t.Errorf("refusals %q", got)
	}
}
