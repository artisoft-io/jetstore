package observe

// The claims of a delivery under the session of the pipeline run that read
// it, which is where a mapping pipeline writes them, and the rule that picks
// that run. Needs a fresh database on the server JETS_TEST_DSN names; skipped
// when it is unset.

import (
	"context"
	"net/url"
	"os"
	"strings"
	"testing"

	"github.com/artisoft-io/jetstore/jets/agentic/observe/feedfixture"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// The checks reach the same verdicts, reasons and anomalies whether a
// delivery's claims are under its registration's session or under the run's
// that read it, and a judged verdict names the run it read.
func TestClaimSetReadsTheClaimsOfThePipelineRun(t *testing.T) {
	own := claimSetTestRun(t, claimSetTestPool(t), claimSetTestOptions())
	runs := claimSetTestRun(t, claimSetTestPoolWith(t, claimSetTestDatabase+"_runs",
		feedfixture.InstallUnderPipelineRuns), claimSetTestOptions())
	if len(own) != len(runs) {
		t.Fatalf("%d verdicts under the registrations' sessions, %d under the runs'", len(own), len(runs))
	}
	type key struct{ det, table, session string }
	byKey := map[key]ClaimSetVerdict{}
	for _, v := range own {
		byKey[key{v.Detector, v.TableName, v.SessionID}] = v
	}
	read := 0
	for _, r := range runs {
		o, ok := byKey[key{r.Detector, r.TableName, r.SessionID}]
		if !ok {
			t.Errorf("%s %s %s: no verdict under the registrations' sessions", r.Detector, r.TableName, r.SessionID)
			continue
		}
		if o.Verdict != r.Verdict || o.Reason != r.Reason {
			t.Errorf("%s %s %s: %s %s under the registration, %s %s under the run",
				r.Detector, r.TableName, r.SessionID, o.Verdict, o.Reason, r.Verdict, r.Reason)
		}
		if len(o.Anomalies) != len(r.Anomalies) {
			t.Errorf("%s %s %s: %d anomalies under the registration, %d under the run",
				r.Detector, r.TableName, r.SessionID, len(o.Anomalies), len(r.Anomalies))
		} else {
			for i := range o.Anomalies {
				oa, ra := o.Anomalies[i], r.Anomalies[i]
				if oa.AnomalyId != ra.AnomalyId || oa.SessionId != ra.SessionId || oa.ObservedValue != ra.ObservedValue {
					t.Errorf("%s %s %s: anomaly %q (%s, %q) became %q (%s, %q)", r.Detector, r.TableName,
						r.SessionID, oa.AnomalyId, oa.SessionId, oa.ObservedValue, ra.AnomalyId, ra.SessionId,
						ra.ObservedValue)
				}
			}
		}
		if r.Aggregates != nil && r.Aggregates.Rows > 0 {
			want := "read from session " + feedfixture.RunSessionOf(r.SessionID)
			if !strings.Contains(r.Detail, want) {
				t.Errorf("%s %s %s: detail %q does not say %q", r.Detector, r.TableName, r.SessionID, r.Detail, want)
			}
			if r.Aggregates.ClaimSession == nil || *r.Aggregates.ClaimSession != feedfixture.RunSessionOf(r.SessionID) {
				t.Errorf("%s %s %s: claim session %v", r.Detector, r.TableName, r.SessionID, r.Aggregates.ClaimSession)
			}
			read++
		}
		if r.Aggregates != nil && o.Aggregates != nil && r.Aggregates.Rows != o.Aggregates.Rows {
			t.Errorf("%s %s %s: %d rows under the registration, %d under the run",
				r.Detector, r.TableName, r.SessionID, o.Aggregates.Rows, r.Aggregates.Rows)
		}
	}
	if read == 0 {
		t.Fatal("no verdict read a run's claims")
	}
}

// The session read for a registration: its own when the table has rows of it;
// otherwise a run that read it and wrote rows, completed runs first and the
// earliest first, found by input_session_id or by its main input's
// registration; none when no session has a row.
func TestClaimSetSessionOfPicksTheRun(t *testing.T) {
	dsn := os.Getenv("JETS_TEST_DSN")
	if dsn == "" {
		t.Skip("JETS_TEST_DSN not set; needs a throwaway Postgres")
	}
	ctx := context.Background()
	const database = "observe_claimset_session_of_test"
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
	defer pool.Close()
	if _, err = pool.Exec(ctx, `CREATE SCHEMA jetsapi;
		CREATE TABLE jetsapi.input_registry (key serial PRIMARY KEY, session_id text, source_type text);
		CREATE TABLE jetsapi.pipeline_execution_status (key int PRIMARY KEY, main_input_registry_key int,
		  input_session_id text, session_id text NOT NULL, status text NOT NULL);
		CREATE TABLE "public"."hc:Claim" (session_id text NOT NULL);
		INSERT INTO jetsapi.input_registry (key, session_id, source_type) VALUES (1, 'R4', 'file');
		INSERT INTO "public"."hc:Claim" VALUES ('R1'), ('P1'), ('P20'), ('P22'), ('P23'), ('P30'), ('P40');
		INSERT INTO jetsapi.pipeline_execution_status VALUES
		  (10, NULL, 'R1', 'P1', 'completed'),
		  (20, NULL, 'R2', 'P20', 'failed'),
		  (21, NULL, 'R2', 'P21', 'completed'),
		  (22, NULL, 'R2', 'P22', 'completed'),
		  (23, NULL, 'R2', 'P23', 'completed'),
		  (30, NULL, 'R3', 'P30', 'failed'),
		  (40, 1, NULL, 'P40', 'completed'),
		  (50, NULL, 'R5', 'P50', 'completed')`); err != nil {
		t.Fatal(err)
	}
	stmt := "SELECT (" + claimSetSessionOf(pgx.Identifier{"public", "hc:Claim"}.Sanitize(), "$1") + ")"
	for _, c := range []struct {
		registration *string
		want         *string
		why          string
	}{
		{strp("R1"), strp("R1"), "the registration's own rows come first"},
		{strp("R2"), strp("P22"), "the first completed run with rows, past a failed one with rows and a completed one without"},
		{strp("R3"), strp("P30"), "a failed run with rows when no completed one has any"},
		{strp("R4"), strp("P40"), "a run found by its main input's registration"},
		{strp("R5"), nil, "a run that wrote no row"},
		{nil, nil, "no predecessor"},
	} {
		var got *string
		if err := pool.QueryRow(ctx, stmt, c.registration).Scan(&got); err != nil {
			t.Fatalf("%s: %v", c.why, err)
		}
		if (got == nil) != (c.want == nil) || (got != nil && *got != *c.want) {
			t.Errorf("%s: got %v, want %v", c.why, deref(got), deref(c.want))
		}
	}
}

func strp(s string) *string { return &s }

func deref(s *string) string {
	if s == nil {
		return "<none>"
	}
	return *s
}
