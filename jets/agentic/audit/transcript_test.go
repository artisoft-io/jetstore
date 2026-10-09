// Criterion 29 — one agent run's transcript is viewable from its audit record
// alone — tested against a real Postgres. Needs JETS_TEST_DSN; see the note at
// the head of audit_test.go.
package audit

import (
	"context"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// oneTableOnly enforces the "alone" of criterion 29 structurally rather than by
// reading the source. It fails the read if the SELECT names any table but
// jetsapi.agent_audit, so a future join to agent_run for a convenience column
// breaks this test rather than quietly weakening the claim.
type oneTableOnly struct {
	t  *testing.T
	db Querier
}

func (q oneTableOnly) Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error) {
	q.t.Helper()
	for _, table := range []string{"agent_run", "change_proposal", "approval_event"} {
		if strings.Contains(sql, table) {
			q.t.Fatalf("the transcript query reads jetsapi.%s; criterion 29 is that the audit record alone suffices", table)
		}
	}
	if !strings.Contains(sql, "agent_audit") {
		q.t.Fatalf("the transcript query does not read jetsapi.agent_audit at all:\n%s", sql)
	}
	return q.db.Query(ctx, sql, args...)
}

// seedRun writes one run's whole story: the intent that opened it, a tool call,
// a decision, a recovered error, and the outcome that closed it. Unique per
// call — I-72's lesson is that a test sharing a run id with another passes in
// isolation and fails in a suite.
func seedRun(t *testing.T, pool *pgxpool.Pool, suffix string) string {
	t.Helper()
	ctx := context.Background()
	runId := fmt.Sprintf("run_k2_%s_%d", suffix, time.Now().UnixNano())
	if err := StartRun(ctx, pool, &Run{
		RunId: runId, AgentId: "agent:authoring", AgentVersion: "0.1.0",
		ModelId: "granite4.1:3b", PromptVersion: "p1", Tier: "T1",
		StartedAt: time.Now().UTC(), DomainModelVersion: "0.1.0",
		IterationCap: 3, WallClockCapSeconds: 60,
	}, []byte(`{"instruction":"author a map_record transformation","max_iterations":3}`)); err != nil {
		t.Fatalf("seeding run: %v", err)
	}
	for _, ev := range []*Event{
		{RunId: runId, EventType: EventError, Actor: "agent:authoring", Tier: "T1",
			Payload: []byte(`{"kind":"schema","message":"missing required field"}`)},
		{RunId: runId, EventType: EventDecision, Actor: "agent:authoring", Tier: "T1",
			Payload: []byte(`{"iteration":2,"artifact":{"type":"map_record"}}`)},
		{RunId: runId, EventType: EventToolCall, Actor: "agent:authoring", Tier: "T1",
			ToolName: "compile_rule_file", Payload: []byte(`{"request":{"file":"x.jr"},"response":{"ok":true}}`)},
		{RunId: runId, EventType: EventOutcome, Actor: "agent:authoring", Tier: "T1",
			Payload: []byte(`{"outcome":"succeeded","iterations":2,"token_spend":1400}`)},
	} {
		mustAppend(t, pool, ev)
	}
	if err := FinishRun(ctx, pool, runId, "succeeded", 1400); err != nil {
		t.Fatalf("finishing run: %v", err)
	}
	return runId
}

// The criterion itself: the run's story is legible from agent_audit and nothing
// else, in order, with the chain checked.
func TestTranscriptIsReadableFromTheAuditRecordAlone(t *testing.T) {
	pool := testPool(t)
	runId := seedRun(t, pool, "alone")

	tr, err := ReadTranscript(context.Background(), oneTableOnly{t: t, db: pool}, runId)
	if err != nil {
		t.Fatalf("reading the transcript: %v", err)
	}
	if !tr.Verified() {
		t.Fatalf("a chain written by this package does not verify: %v", tr.Defects)
	}
	if len(tr.Events) != 5 {
		t.Fatalf("got %d events, want the 5 the run wrote", len(tr.Events))
	}
	// Ordered by seq, contiguous from 1 — the reader's sense of what happened
	// after what depends entirely on this.
	want := []string{EventIntent, EventError, EventDecision, EventToolCall, EventOutcome}
	for i, ev := range tr.Events {
		if ev.Seq != i+1 {
			t.Errorf("event %d has seq %d, want %d", i, ev.Seq, i+1)
		}
		if ev.EventType != want[i] {
			t.Errorf("event at seq %d is %q, want %q", ev.Seq, ev.EventType, want[i])
		}
	}

	// "Viewable" means a reader can answer what it was asked, what it did, and
	// how it ended, without another source.
	rendered := tr.String()
	for _, fragment := range []string{
		runId,
		"chain verified",
		"author a map_record transformation", // what it was asked
		"tool=compile_rule_file",             // what it did
		"missing required field",             // what went wrong on the way
		`"outcome": "succeeded"`,             // how it ended
		"agent:authoring", "T1",              // who, with what authority
	} {
		if !strings.Contains(rendered, fragment) {
			t.Errorf("the rendered transcript does not show %q:\n%s", fragment, rendered)
		}
	}
}

// K.1 carried this forward: run_ref is the originating run, not the approver's
// session, so a decision taken after the run ended still belongs to its story.
// A transcript that stopped at ended_at would omit the approval entirely.
func TestTranscriptCarriesApprovalsTakenAfterTheRunEnded(t *testing.T) {
	pool := testPool(t)
	ctx := context.Background()
	runId, proposalId := seedProposal(t, pool, "k2_transcript")

	if _, err := RecordApproval(ctx, pool, &Approval{
		ApprovalEventId: "apv_k2_" + proposalId, RunRef: runId, SubjectRef: proposalId,
		FromState: "draft", ToState: "approved",
		Actor: "michel@artisoft.io", TierAtEvent: "T2",
		DecisionRationale: "reviewed the artifact",
	}); err != nil {
		t.Fatalf("recording approval: %v", err)
	}

	tr, err := ReadTranscript(ctx, pool, runId)
	if err != nil {
		t.Fatalf("reading the transcript: %v", err)
	}
	if !tr.Verified() {
		t.Fatalf("the chain does not verify after a post-run approval: %v", tr.Defects)
	}
	last := tr.Events[len(tr.Events)-1]
	if last.EventType != EventApproval {
		t.Fatalf("the last event is %q, want the approval that closed the proposal", last.EventType)
	}
	if last.Actor != "michel@artisoft.io" {
		t.Errorf("the approval is attributed to %q; an unattributable approval is what the record exists to prevent", last.Actor)
	}
	rendered := tr.String()
	for _, fragment := range []string{proposalId, "approved", "michel@artisoft.io"} {
		if !strings.Contains(rendered, fragment) {
			t.Errorf("the transcript does not show %q:\n%s", fragment, rendered)
		}
	}
}

// The Go recomputation of the trigger's timestamp rendering is the one part of
// the material this package derives rather than reads back, so it is asserted
// against Postgres directly. A drift here would surface as every row failing
// its hash, which is a confusing way to learn that a format string moved.
func TestChainTimeMatchesPostgres(t *testing.T) {
	pool := testPool(t)
	ctx := context.Background()
	runId := seedRun(t, pool, "chaintime")

	tr, err := ReadTranscript(ctx, pool, runId)
	if err != nil {
		t.Fatalf("reading the transcript: %v", err)
	}
	for _, ev := range tr.Events {
		var fromPg string
		if err := pool.QueryRow(ctx,
			`SELECT to_char(created_at AT TIME ZONE 'UTC', 'YYYY-MM-DD"T"HH24:MI:SS.US"Z"')
			   FROM jetsapi.agent_audit WHERE run_id = $1 AND seq = $2`,
			runId, ev.Seq).Scan(&fromPg); err != nil {
			t.Fatal(err)
		}
		if got := chainTime(ev.CreatedAt); got != fromPg {
			t.Fatalf("seq %d: Go renders the chain timestamp as %q, Postgres as %q", ev.Seq, got, fromPg)
		}
	}
}

// Tampering, exercised as a pure function over the events. The append-only
// trigger makes a real UPDATE impossible — criterion 9 — which is exactly why
// the detector would otherwise never run against a damaged chain.
func TestVerifyDetectsTampering(t *testing.T) {
	pool := testPool(t)
	runId := seedRun(t, pool, "tamper")
	sound, err := ReadTranscript(context.Background(), pool, runId)
	if err != nil {
		t.Fatalf("reading the transcript: %v", err)
	}
	if !sound.Verified() {
		t.Fatalf("the control chain does not verify: %v", sound.Defects)
	}

	for _, tc := range []struct {
		name    string
		damage  func(ev []TranscriptEvent) []TranscriptEvent
		wantAt  int
		wantKey DefectKind
	}{
		{
			// The attack the record exists to stop: attributing a decision to
			// someone who did not take it.
			name:    "actor rewritten",
			damage:  func(ev []TranscriptEvent) []TranscriptEvent { ev[2].Actor = "someone:else"; return ev },
			wantAt:  3,
			wantKey: DefectHash,
		},
		{
			name: "payload rewritten",
			damage: func(ev []TranscriptEvent) []TranscriptEvent {
				ev[4].Payload = []byte(`{"outcome": "failed"}`)
				return ev
			},
			wantAt:  5,
			wantKey: DefectHash,
		},
		{
			name:    "tier escalated",
			damage:  func(ev []TranscriptEvent) []TranscriptEvent { ev[1].Tier = "T4"; return ev },
			wantAt:  2,
			wantKey: DefectHash,
		},
		{
			// A row excised from the middle: the link across the hole breaks
			// and the sequence is no longer contiguous. Both are reported,
			// because they are two different things a reader wants to know.
			name:    "event excised",
			damage:  func(ev []TranscriptEvent) []TranscriptEvent { return append(ev[:2:2], ev[3:]...) },
			wantAt:  4,
			wantKey: DefectLink,
		},
		{
			name: "chain relinked to hide the hole",
			damage: func(ev []TranscriptEvent) []TranscriptEvent {
				out := append(ev[:2:2], ev[3:]...)
				out[2].PrevHash = out[1].RowHash // repair the link, leave the hash
				return out
			},
			wantAt:  4,
			wantKey: DefectHash,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			damaged := tc.damage(copyEvents(sound.Events))
			defects := Verify(runId, damaged)
			if len(defects) == 0 {
				t.Fatalf("tampering went undetected; the chain is decorative")
			}
			found := false
			for _, d := range defects {
				if d.Seq == tc.wantAt && d.Kind == tc.wantKey {
					found = true
				}
			}
			if !found {
				t.Errorf("want a %s defect at seq %d, got %v", tc.wantKey, tc.wantAt, defects)
			}
			// A damaged transcript still renders — destroying the evidence
			// would be the wrong response to finding it.
			rendered := (&Transcript{RunId: runId, Events: damaged, Defects: defects}).String()
			if !strings.Contains(rendered, "CHAIN NOT VERIFIED") {
				t.Errorf("the rendering does not lead with the verdict:\n%s", rendered)
			}
		})
	}
}

// The first event of a run has no predecessor, and a prev_hash on it means the
// rows are not the run they claim to be.
func TestVerifyRejectsALinkedFirstEvent(t *testing.T) {
	pool := testPool(t)
	runId := seedRun(t, pool, "firstlink")
	tr, err := ReadTranscript(context.Background(), pool, runId)
	if err != nil {
		t.Fatal(err)
	}
	events := copyEvents(tr.Events)
	events[0].PrevHash, _ = hex.DecodeString(strings.Repeat("ab", 32))
	defects := Verify(runId, events)
	var linked bool
	for _, d := range defects {
		if d.Seq == 1 && d.Kind == DefectLink {
			linked = true
		}
	}
	if !linked {
		t.Errorf("a first event linking to a predecessor was accepted; got %v", defects)
	}
}

// An empty result is not an empty transcript. StartRun writes the run row and
// its intent event in one transaction, so no rows means no such run.
func TestUnknownRunHasNoTranscript(t *testing.T) {
	pool := testPool(t)
	_, err := ReadTranscript(context.Background(), pool, fmt.Sprintf("run_absent_%d", time.Now().UnixNano()))
	var missing *ErrNoTranscript
	if !errors.As(err, &missing) {
		t.Fatalf("want ErrNoTranscript for a run with no rows, got %v", err)
	}
	if _, err := ReadTranscript(context.Background(), pool, ""); err == nil {
		t.Error("an empty run id must be refused rather than scanning the table")
	}
}

func copyEvents(in []TranscriptEvent) []TranscriptEvent {
	out := make([]TranscriptEvent, len(in))
	copy(out, in)
	return out
}
