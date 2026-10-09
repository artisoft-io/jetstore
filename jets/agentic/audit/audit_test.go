// The item-8 exit criteria, tested against a real Postgres (plan section
// 7.2): criterion 9 — UPDATE and DELETE against agent_audit both fail;
// criterion 10 — an interrupt between the intent write and the act leaves
// the intent row present. Plus the hash chain (A8.5) recomputed
// independently in Go, and the Go event-type constants asserted against the
// generated CHECK constraint.
//
// Needs JETS_TEST_DSN (any throwaway database; the suite installs the
// schema); skipped otherwise. Locally:
//
//	docker run -d --rm -e POSTGRES_PASSWORD=pw -p 5455:5432 postgres:16-alpine
//	JETS_TEST_DSN=postgres://postgres:pw@localhost:5455/postgres go test ./jets/agentic/audit/
package audit

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

func testPool(t *testing.T) *pgxpool.Pool {
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
	if err := InstallSchema(context.Background(), pool); err != nil {
		t.Fatalf("installing schema: %v", err)
	}
	return pool
}

func mustAppend(t *testing.T, pool *pgxpool.Pool, ev *Event) int {
	t.Helper()
	seq, err := Append(context.Background(), pool, ev)
	if err != nil {
		t.Fatal(err)
	}
	return seq
}

// Criterion 9: UPDATE and DELETE both fail — and TRUNCATE, one better.
func TestAppendOnly(t *testing.T) {
	pool := testPool(t)
	ctx := context.Background()
	runId := fmt.Sprintf("run_ao_%d", time.Now().UnixNano())
	mustAppend(t, pool, &Event{RunId: runId, EventType: EventIntent, Actor: "test", Payload: []byte(`{}`)})

	for name, sql := range map[string]string{
		"UPDATE":   "UPDATE jetsapi.agent_audit SET actor = 'tampered' WHERE run_id = $1",
		"DELETE":   "DELETE FROM jetsapi.agent_audit WHERE run_id = $1",
		"TRUNCATE": "TRUNCATE jetsapi.agent_audit",
	} {
		var err error
		if name == "TRUNCATE" {
			_, err = pool.Exec(ctx, sql)
		} else {
			_, err = pool.Exec(ctx, sql, runId)
		}
		if err == nil {
			t.Fatalf("%s against agent_audit succeeded; the table must be append-only", name)
		}
		if !strings.Contains(err.Error(), "append-only") {
			t.Fatalf("%s failed for the wrong reason: %v", name, err)
		}
	}

	var actor string
	if err := pool.QueryRow(ctx,
		"SELECT actor FROM jetsapi.agent_audit WHERE run_id = $1 AND seq = 1", runId).Scan(&actor); err != nil {
		t.Fatal(err)
	}
	if actor != "test" {
		t.Fatalf("row was altered to actor=%q", actor)
	}
}

// A8.4 + A8.5: outcomes are second rows correlated by run_id/seq, and the
// hash chain links them — recomputed here independently of the trigger.
func TestOutcomeRowsAndHashChain(t *testing.T) {
	pool := testPool(t)
	ctx := context.Background()
	runId := fmt.Sprintf("run_hc_%d", time.Now().UnixNano())

	events := []*Event{
		{RunId: runId, EventType: EventIntent, Actor: "agent:rca", Tier: "T2", ToolName: "propose_remediation", Payload: []byte(`{"action":"requeue"}`)},
		{RunId: runId, EventType: EventApproval, Actor: "michel@artisoft.io", Tier: "T2", Payload: []byte(`{"to_state":"approved"}`)},
		{RunId: runId, EventType: EventOutcome, Actor: "agent:rca", Tier: "T2", ToolName: "propose_remediation", Payload: []byte(`{"status":"succeeded"}`)},
	}
	for i, ev := range events {
		if seq := mustAppend(t, pool, ev); seq != i+1 {
			t.Fatalf("event %d got seq %d; the trigger must assign monotonically", i, seq)
		}
	}

	rows, err := pool.Query(ctx,
		`SELECT seq, event_type, actor, coalesce(tier,''), coalesce(tool_name,''), payload::text,
		        to_char(created_at AT TIME ZONE 'UTC', 'YYYY-MM-DD"T"HH24:MI:SS.US"Z"'),
		        coalesce(prev_hash, ''::bytea), row_hash
		 FROM jetsapi.agent_audit WHERE run_id = $1 ORDER BY seq`, runId)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()

	var lastHash []byte
	n := 0
	for rows.Next() {
		var seq int
		var eventType, actor, tier, toolName, payload, createdAt string
		var prevHash, rowHash []byte
		if err := rows.Scan(&seq, &eventType, &actor, &tier, &toolName, &payload, &createdAt, &prevHash, &rowHash); err != nil {
			t.Fatal(err)
		}
		if n > 0 && !bytes.Equal(prevHash, lastHash) {
			t.Fatalf("seq %d: prev_hash does not link to the previous row_hash", seq)
		}
		// Recompute row_hash exactly as the generated trigger defines it:
		// SHA-256 over the fields joined by the 0x1F unit separator.
		prevHex := ""
		if len(prevHash) > 0 {
			prevHex = hex.EncodeToString(prevHash)
		}
		material := strings.Join([]string{
			runId, fmt.Sprint(seq), eventType, actor, tier, toolName, payload, createdAt, prevHex,
		}, "\x1f")
		want := sha256.Sum256([]byte(material))
		if !bytes.Equal(rowHash, want[:]) {
			t.Fatalf("seq %d: row_hash does not recompute; the chain is not independently checkable", seq)
		}
		lastHash = rowHash
		n++
	}
	if n != 3 {
		t.Fatalf("expected 3 rows for the run, found %d", n)
	}
}

// The Go constants against the generated CHECK constraint: every constant
// inserts, an out-of-taxonomy type does not.
func TestEventTypes(t *testing.T) {
	pool := testPool(t)
	runId := fmt.Sprintf("run_et_%d", time.Now().UnixNano())
	for _, et := range []string{EventIntent, EventToolCall, EventDecision, EventOutcome, EventApproval, EventError} {
		mustAppend(t, pool, &Event{RunId: runId, EventType: et, Actor: "test", Payload: []byte(`{}`)})
	}
	_, err := Append(context.Background(), pool, &Event{RunId: runId, EventType: "compacted", Actor: "test", Payload: []byte(`{}`)})
	if err == nil || !strings.Contains(err.Error(), "agent_audit_event_type_ck") {
		t.Fatalf("out-of-taxonomy event_type must fail the CHECK constraint; got %v", err)
	}
}

// Criterion 10: the child process commits the intent and dies before the
// act; the parent finds the intent row and no outcome. The negative control
// dies before the commit and must leave nothing — the transaction boundary,
// not the log call, is what carries the guarantee (section 7.1's point
// against zap).
func TestWriteBeforeActSurvivesInterrupt(t *testing.T) {
	pool := testPool(t)
	ctx := context.Background()
	for _, tc := range []struct {
		mode     string
		wantRows int
	}{
		{"after-commit", 1},
		{"before-commit", 0},
	} {
		runId := fmt.Sprintf("run_wba_%s_%d", tc.mode, time.Now().UnixNano())
		cmd := exec.Command(os.Args[0], "-test.run", "TestInterruptedChild")
		cmd.Env = append(os.Environ(),
			"AUDIT_CHILD_MODE="+tc.mode, "AUDIT_CHILD_RUN_ID="+runId)
		out, err := cmd.CombinedOutput()
		if err == nil {
			t.Fatalf("%s: the child is meant to die mid-flight, it exited clean: %s", tc.mode, out)
		}
		var count int
		if err := pool.QueryRow(ctx,
			"SELECT count(*) FROM jetsapi.agent_audit WHERE run_id = $1", runId).Scan(&count); err != nil {
			t.Fatal(err)
		}
		if count != tc.wantRows {
			t.Fatalf("%s: found %d rows for the interrupted run, want %d\nchild output: %s",
				tc.mode, count, tc.wantRows, out)
		}
		if tc.wantRows == 1 {
			var eventType string
			if err := pool.QueryRow(ctx,
				"SELECT event_type FROM jetsapi.agent_audit WHERE run_id = $1 AND seq = 1",
				runId).Scan(&eventType); err != nil {
				t.Fatal(err)
			}
			if eventType != EventIntent {
				t.Fatalf("surviving row is %q, want the intent", eventType)
			}
		}
	}
}

// TestInterruptedChild is the re-exec target of the interrupt test, not a
// test in its own right: it writes an intent row in a transaction and kills
// its own process — after the commit or before it, per AUDIT_CHILD_MODE.
func TestInterruptedChild(t *testing.T) {
	mode := os.Getenv("AUDIT_CHILD_MODE")
	if mode == "" {
		t.Skip("re-exec target of TestWriteBeforeActSurvivesInterrupt")
	}
	ctx := context.Background()
	pool, err := pgxpool.New(ctx, os.Getenv("JETS_TEST_DSN"))
	if err != nil {
		t.Fatal(err)
	}
	tx, err := pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = Append(ctx, tx, &Event{
		RunId:     os.Getenv("AUDIT_CHILD_RUN_ID"),
		EventType: EventIntent,
		Actor:     "agent:test",
		ToolName:  "dangerous_act",
		Payload:   []byte(`{"about_to":"act"}`),
	}); err != nil {
		t.Fatal(err)
	}
	if mode == "after-commit" {
		if err = tx.Commit(ctx); err != nil {
			t.Fatal(err)
		}
	}
	os.Exit(3) // the interrupt: the process dies before the act happens
}
