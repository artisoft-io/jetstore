package main

// feed_detect against the synthetic feed fixture, in a database of its own on
// the server JETS_TEST_DSN names; the database tests are skipped when it is
// unset. Run with -count=1: the fixture reads jets_schema.json, which the test
// cache does not see.
//
//	docker run -d --rm --name feed-detect-pg -e POSTGRES_PASSWORD=pw -p 55913:5432 postgres:16-alpine
//	JETS_TEST_DSN=postgres://postgres:pw@localhost:55913/postgres go test -count=1 ./jets/cmds/feed_detect/

import (
	"bytes"
	"context"
	"encoding/json"
	"log"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/artisoft-io/jetstore/jets/agentic/audit"
	"github.com/artisoft-io/jetstore/jets/agentic/detectrun"
	"github.com/artisoft-io/jetstore/jets/agentic/feedprofile"
	"github.com/artisoft-io/jetstore/jets/agentic/observe/feedfixture"
	"github.com/artisoft-io/jetstore/jets/schema"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

const testDatabase = "feed_detect_cmd_test"

func testDB(t *testing.T) *pgxpool.Pool {
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
	name := pgx.Identifier{testDatabase}.Sanitize()
	_, err = admin.Exec(ctx, "DROP DATABASE IF EXISTS "+name+" WITH (FORCE)")
	if err == nil {
		_, err = admin.Exec(ctx, "CREATE DATABASE "+name)
	}
	admin.Close()
	if err != nil {
		t.Fatal(err)
	}
	u, _ := url.Parse(dsn)
	u.Path = "/" + testDatabase
	pool, err := pgxpool.New(ctx, u.String())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(pool.Close)
	if _, err := feedfixture.Install(ctx, pool); err != nil {
		t.Fatal(err)
	}
	if err := audit.InstallSchema(ctx, pool); err != nil {
		t.Fatal(err)
	}
	f, err := os.Open("../../jets_schema.json")
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	var defs []schema.TableDefinition
	if err := json.NewDecoder(f).Decode(&defs); err != nil {
		t.Fatal(err)
	}
	for i := range defs {
		if defs[i].TableName == "agentic_detection_run" {
			if err := defs[i].UpdateTableSchema(pool, false); err != nil {
				t.Fatal(err)
			}
		}
	}
	return pool
}

// noInference clears every variable that would point the agentic layer at a
// model, so the run proves it needs none.
func noInference(t *testing.T) {
	for _, v := range []string{"JETS_INFER_URL", "JETS_INFER_BACKEND", "JETS_INFER_MODEL",
		"JETS_INFER_SERVED_MODEL_NAME"} {
		t.Setenv(v, "")
		os.Unsetenv(v)
	}
}

// captureLog sends the standard logger to a buffer for the rest of the test.
func captureLog(t *testing.T) *bytes.Buffer {
	var buf bytes.Buffer
	prev, flags := log.Writer(), log.Flags()
	log.SetOutput(&buf)
	t.Cleanup(func() { log.SetOutput(prev); log.SetFlags(flags) })
	return &buf
}

func runOf(t *testing.T, pool *pgxpool.Pool) detectrun.Run {
	t.Helper()
	runs, err := detectrun.List(context.Background(), pool, 0)
	if err != nil {
		t.Fatal(err)
	}
	for _, r := range runs {
		if r.Command == detectrun.CommandFeedDetect {
			return r
		}
	}
	t.Fatal("no feed_detect run recorded")
	return detectrun.Run{}
}

func canaries(t *testing.T, where, text string) {
	t.Helper()
	for _, c := range feedfixture.Canaries() {
		if strings.Contains(text, c) {
			t.Errorf("the %s carries the canary %s", where, c)
		}
	}
}

// The command's dependencies include nothing under
// jets/agentic/infer.
func TestFeedDetectDependsOnNoInference(t *testing.T) {
	out, err := exec.Command("go", "list", "-deps", ".").Output()
	if err != nil {
		t.Fatalf("go list: %v", err)
	}
	deps := strings.Fields(string(out))
	if len(deps) < 50 {
		t.Fatalf("go list gave %d dependencies", len(deps))
	}
	for _, d := range deps {
		if strings.Contains(d, "/jets/agentic/infer") || strings.Contains(d, "/jets/agentic/agent") {
			t.Errorf("feed_detect depends on %s", d)
		}
	}
}

// With no inference endpoint configured, a backfill judges the fixture, writes
// its findings, records its run with the coverage, and no canary reaches the
// log, the run row or a written finding. A dry run after it writes nothing.
func TestFeedDetectRunsWithNoModelAndRecordsItsCoverage(t *testing.T) {
	pool := testDB(t)
	noInference(t)
	logs := captureLog(t)
	ctx := context.Background()
	c, err := parseFlags([]string{"-mode", "backfill", "-from", "2025-12-01",
		"-as-of", feedfixture.Clock.Format(time.RFC3339), "-profiles", feedfixture.ProfilesPath()})
	if err != nil {
		t.Fatal(err)
	}
	o, err := c.options(time.Now())
	if err != nil {
		t.Fatal(err)
	}
	if err := recorded(ctx, pool, c, o); err != nil {
		t.Fatalf("the run failed: %v", err)
	}
	run := runOf(t, pool)
	if run.Status != detectrun.StatusOk || run.Since != "from 2025-12-01T00:00:00Z" {
		t.Errorf("the run row: status %s since %q", run.Status, run.Since)
	}
	var sum struct {
		Written  int `json:"anomaliesWritten"`
		Coverage []struct {
			Detector         string         `json:"detector"`
			NotJudged        map[string]int `json:"notJudged"`
			HistoryTruncated map[string]int `json:"historyTruncated"`
		} `json:"coverage"`
	}
	if err := json.Unmarshal(run.Summary, &sum); err != nil {
		t.Fatal(err)
	}
	if sum.Written != 54 || len(sum.Coverage) != len(feedprofile.Detectors) {
		t.Errorf("the summary records %d written and %d coverage lines", sum.Written, len(sum.Coverage))
	}
	reasons, truncated := 0, 0
	for _, cv := range sum.Coverage {
		reasons += len(cv.NotJudged)
		truncated += len(cv.HistoryTruncated)
	}
	if reasons == 0 || truncated == 0 {
		t.Errorf("the run row records %d not-judged reasons and %d history_truncated", reasons, truncated)
	}
	for _, want := range []string{"coverage delivery_overlap:", "history_truncated", "no_predecessor", "no_profile"} {
		if !strings.Contains(logs.String(), want) {
			t.Errorf("the log does not say %q", want)
		}
	}
	canaries(t, "log", logs.String())
	canaries(t, "run row", string(run.Summary)+run.ErrorMessage)
	var written string
	if err := pool.QueryRow(ctx, `SELECT coalesce(string_agg(to_jsonb(a)::text, E'\n'), '') FROM jetsapi.anomaly a`).
		Scan(&written); err != nil {
		t.Fatal(err)
	}
	canaries(t, "anomaly table", written)

	// A dry window over the same deliveries writes nothing and records itself.
	c, _ = parseFlags([]string{"-dry-run", "-since", "2400h", "-as-of", feedfixture.Clock.Format(time.RFC3339),
		"-profiles", feedfixture.ProfilesPath()})
	o, _ = c.options(time.Now())
	var before, after int
	pool.QueryRow(ctx, "SELECT count(*) FROM jetsapi.anomaly").Scan(&before)
	if err := recorded(ctx, pool, c, o); err != nil {
		t.Fatal(err)
	}
	pool.QueryRow(ctx, "SELECT count(*) FROM jetsapi.anomaly").Scan(&after)
	if after != before {
		t.Errorf("a dry run wrote %d anomalies", after-before)
	}
}

// A feed whose content checks fail does not stop the others: their findings
// are written, and the run is recorded failed, naming the feed, without the
// database's message, which quotes the value it could not read.
func TestFeedDetectRecordsAFailingFeedAndJudgesTheRest(t *testing.T) {
	pool := testDB(t)
	logs := captureLog(t)
	ctx := context.Background()
	raw, err := feedfixture.Profiles()
	if err != nil {
		t.Fatal(err)
	}
	var p map[string]any
	json.Unmarshal(raw[feedfixture.StaleCumulative], &p)
	p["claim_set"].(map[string]any)["service_date_column"] = feedfixture.ColMember
	raw[feedfixture.StaleCumulative], _ = json.Marshal(p)
	doc, _ := json.Marshal(raw)
	path := filepath.Join(t.TempDir(), "profiles.json")
	os.WriteFile(path, doc, 0o644)

	c, err := parseFlags([]string{"-since", "2400h", "-as-of", feedfixture.Clock.Format(time.RFC3339), "-profiles", path})
	if err != nil {
		t.Fatal(err)
	}
	o, _ := c.options(time.Now())
	err = recorded(ctx, pool, c, o)
	if err == nil || !strings.Contains(err.Error(), feedfixture.StaleCumulative) {
		t.Fatalf("the run: %v; want a failure naming the feed", err)
	}
	run := runOf(t, pool)
	if run.Status != detectrun.StatusFailed || !strings.Contains(run.ErrorMessage, feedfixture.StaleCumulative) {
		t.Errorf("the run row: %s %q", run.Status, run.ErrorMessage)
	}
	if !strings.Contains(string(run.Summary), "SQLSTATE") {
		t.Errorf("the summary does not give the failing feed's SQLSTATE: %s", run.Summary)
	}
	var written int
	pool.QueryRow(ctx, "SELECT count(*) FROM jetsapi.anomaly").Scan(&written)
	if written < 40 {
		t.Errorf("%d anomalies written; the other feeds should have been judged", written)
	}
	canaries(t, "log", logs.String())
	canaries(t, "run row", string(run.Summary)+run.ErrorMessage)
}

// The export, run at the client and judged here, prints the verdicts the
// window run reaches, and the export's statements never carry a semicolon.
func TestFeedDetectExportsAndJudges(t *testing.T) {
	pool := testDB(t)
	ctx := context.Background()
	dir := t.TempDir()
	results := filepath.Join(dir, "results.txt")
	args := []string{"-from", "2025-12-01", "-as-of", feedfixture.Clock.Format(time.RFC3339),
		"-profiles", feedfixture.ProfilesPath(), "-results", results}
	for round := 1; ; round++ {
		if round > 6 {
			t.Fatal("the export did not complete")
		}
		c, err := parseFlags(append([]string{"-mode", "export"}, args...))
		if err != nil {
			t.Fatal(err)
		}
		var out bytes.Buffer
		if err := runExport(ctx, c, time.Now(), &out); err != nil {
			t.Fatal(err)
		}
		if strings.Contains(out.String(), "The export is complete") {
			if !strings.Contains(out.String(), "-mode judge") {
				t.Errorf("the last round does not say how to judge: %s", out.String())
			}
			break
		}
		f, _ := os.OpenFile(results, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
		for _, line := range strings.Split(out.String(), "\n") {
			if line == "" || strings.HasPrefix(line, "--") {
				continue
			}
			if strings.Contains(line, ";") {
				t.Fatal("an exported statement carries a semicolon")
			}
			var text string
			if err := pool.QueryRow(ctx, line, pgx.QueryExecModeSimpleProtocol).Scan(&text); err != nil {
				t.Fatalf("round %d: %v", round, err)
			}
			f.WriteString(text + "\n")
		}
		f.Close()
	}
	c, _ := parseFlags(append([]string{"-mode", "judge", "-json", filepath.Join(dir, "judged.json")}, args...))
	var out bytes.Buffer
	if err := runJudge(ctx, c, time.Now(), &out); err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"feed_overdue@1 arrival subject feed stopped_incremental",
		"delivery_unchanged\tstale_cumulative\t3\t", "coverage service_date_span:"} {
		if !strings.Contains(out.String(), want) {
			t.Errorf("the judge's output does not say %q", want)
		}
	}
	canaries(t, "judge's output", out.String())
	doc, _ := os.ReadFile(filepath.Join(dir, "judged.json"))
	var j judged
	if err := json.Unmarshal(doc, &j); err != nil || len(j.Anomalies) != 54 {
		t.Errorf("the judge's JSON: %d anomalies, %v", len(j.Anomalies), err)
	}
}
