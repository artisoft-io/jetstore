package observe

// The claim-set summaries: computed inside the database to keyset's bits,
// written without a member-level value, and judged from once the claim rows
// are gone. Needs JETS_TEST_DSN; run with -count=1.
//
//	JETS_TEST_DSN=postgres://postgres:pw@localhost:5531/postgres \
//	  go test -count=1 -run ClaimSetSummar ./jets/agentic/observe/

import (
	"context"
	"fmt"
	"net/url"
	"os"
	"testing"
	"time"

	"github.com/artisoft-io/jetstore/jets/agentic/feedprofile"
	"github.com/artisoft-io/jetstore/jets/keyset"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// claimSetSummaryScratch is a fresh, empty database of its own.
func claimSetSummaryScratch(t *testing.T, database string) *pgxpool.Pool {
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
	u, _ := url.Parse(dsn)
	u.Path = "/" + database
	pool, err := pgxpool.New(ctx, u.String())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(pool.Close)
	return pool
}

// The statement's fingerprint and sketch are keyset's, bit for bit, over keys
// with nulls, empty values, a non-ASCII value and a number; and a delivery
// with fewer distinct keys than the minimum keeps neither.
func TestClaimSetSummaryHashesAsKeysetDoes(t *testing.T) {
	pool := claimSetSummaryScratch(t, "observe_claimset_hash_test")
	ctx := context.Background()
	if _, err := pool.Exec(ctx, `CREATE SCHEMA jetsapi;
		CREATE TABLE jetsapi.input_registry (key serial PRIMARY KEY, session_id text, source_type text);
		CREATE TABLE jetsapi.pipeline_execution_status (key int PRIMARY KEY, main_input_registry_key int,
		  input_session_id text, session_id text NOT NULL, status text NOT NULL);
		CREATE TABLE public.claims (session_id text NOT NULL, "claim id" text, line integer, d date, m text);
		INSERT INTO public.claims SELECT 'S', 'c-' || (i / 2), i % 2, DATE '2026-03-31' - (i % 40), 'm' || (i % 97)
		  FROM generate_series(0, 39999) i;
		INSERT INTO public.claims VALUES ('S', NULL, 1, NULL, 'x'), ('S', '', 1, NULL, 'x'), ('S', NULL, NULL, NULL, 'x'),
		  ('S', 'é-ü', 7, NULL, 'x'), ('S', 'c-0', 0, NULL, 'x');
		INSERT INTO public.claims SELECT 'T', 'c-' || i, 1, DATE '2026-03-31', 'm' FROM generate_series(1, 5) i`); err != nil {
		t.Fatal(err)
	}
	// The same keys, as text, in Go.
	b := keyset.NewBuilder()
	str := func(s string) *string { return &s }
	for i := range 40000 {
		b.AddText(keyset.KeyText([]*string{str(fmt.Sprintf("c-%d", i/2)), str(fmt.Sprint(i % 2))}))
	}
	for _, k := range [][]*string{{nil, str("1")}, {str(""), str("1")}, {nil, nil}, {str("é-ü"), str("7")},
		{str("c-0"), str("0")}} {
		b.AddText(keyset.KeyText(k))
	}
	want := b.Summary()

	cs := &feedprofile.ClaimSet{Table: "claims", KeyColumns: []string{"claim id", "line"}, ServiceDateColumn: "d",
		MemberColumn: "m"}
	columns := []string{"session_id", "claim id", "line", "d", "m"}
	stmt := ClaimSetSummaryStatement(pgx.Identifier{"public", "claims"}, cs, columns)
	f := feedprofile.Feed{TableName: "claims", Profile: feedprofile.Profile{ClaimSet: cs}}
	o := ClaimSetOptions{MinMembers: 11, Now: time.Date(2026, 4, 2, 0, 0, 0, 0, time.UTC)}
	at := o.Now
	s, err := claimSetSummarise(ctx, pool, stmt, columns, f, claimSetDelivery{session: "S", arrival: &at}, o)
	if err != nil {
		t.Fatal(err)
	}
	if s.DistinctKeys != want.Count || s.Fingerprint == nil || uint64(*s.Fingerprint) != want.Fingerprint {
		t.Fatalf("the database's %d keys and fingerprint %v, keyset's %d and %d", s.DistinctKeys, s.Fingerprint,
			want.Count, want.Fingerprint)
	}
	sk, err := s.sketch()
	if err != nil || sk == nil || *sk != want.Sketch {
		t.Fatalf("the database's sketch is not keyset's (%v)", err)
	}
	if len(s.Sketch) != keyset.SketchBytes || s.Rows != 40005 || s.Present["claim id"] != 40002 ||
		s.Present["d"] != 40000 || s.Members != 98 || s.DatesInMonths {
		t.Errorf("counts: %d rows, present %v, %d members", s.Rows, s.Present, s.Members)
	}
	small, err := claimSetSummarise(ctx, pool, stmt, columns, f, claimSetDelivery{session: "T", arrival: &at}, o)
	if err != nil {
		t.Fatal(err)
	}
	if small.DistinctKeys != 5 || small.Fingerprint != nil || small.Sketch != nil || !small.DatesInMonths ||
		small.Newest == nil || small.Newest.Day() != 1 {
		t.Errorf("a delivery of five keys and one member: %+v", small)
	}
}
