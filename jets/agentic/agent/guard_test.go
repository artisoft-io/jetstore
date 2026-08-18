package agent

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/artisoft-io/jetstore/jets/agentic/audit"
	"github.com/artisoft-io/jetstore/jets/agentic/tools"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Criterion 18: the kill switch is real. Revoking the agent identity's
// capability stops the next run before its first model call, and the refusal is
// an error event naming the capability.

func TestCriterion18_RevokedIdentityIsStoppedBeforeTheFirstCall(t *testing.T) {
	in := &stubInfer{answers: []string{`{"a":1}`}}
	rec := &recorder{}
	l := &Loop{
		Infer: in, Registry: &stubRegistry{}, Audit: rec,
		Guard: GuardFunc(func(context.Context) error {
			return errors.New(`agent: agent@x does not hold the "run_pipelines" capability`)
		}),
		Budget: Budget{MaxIterations: 3}, RunId: "run-revoked", Actor: "agent@x",
	}

	res, err := l.Run(context.Background(), task())
	if err == nil {
		t.Fatal("a revoked identity must not run")
	}
	if res.Outcome != OutcomeFailed {
		t.Errorf("outcome = %q, want failed", res.Outcome)
	}
	// Before the first model call, which is the clause that makes this a kill
	// switch rather than a report.
	if len(in.seen) != 0 {
		t.Errorf("made %d model calls after refusal; none is permitted", len(in.seen))
	}
	// And the refusal is on the transcript, naming the capability.
	if len(rec.events) != 1 || rec.events[0].EventType != audit.EventError {
		t.Fatalf("events = %v, want a single error event", rec.types())
	}
	if !strings.Contains(string(rec.events[0].Payload), "run_pipelines") {
		t.Errorf("the error event does not name the capability: %s", rec.events[0].Payload)
	}
}

// The refusal happens before the run is recorded. A run that was never allowed
// to start should not leave a record suggesting it did.
func TestCriterion18_RefusalPrecedesTheRunRecord(t *testing.T) {
	started := false
	l := &Loop{
		Infer: &stubInfer{}, Registry: &stubRegistry{}, Audit: &recorder{},
		Recorder: recorderFunc{start: func() error { started = true; return nil }},
		Guard:    GuardFunc(func(context.Context) error { return errors.New("revoked") }),
		Budget:   Budget{MaxIterations: 2}, RunId: "run-x",
	}
	if _, err := l.Run(context.Background(), task()); err == nil {
		t.Fatal("expected refusal")
	}
	if started {
		t.Error("the run was recorded despite being refused; a refused run is not a run")
	}
}

// Revocation part-way through takes effect at the next check. This is what
// "durable" buys over a flag read once at start-up, and it is the reason the
// guard is consulted again before each tool call.
func TestGuard_RevocationMidRunStopsTheNextToolCall(t *testing.T) {
	calls := 0
	l := &Loop{
		Infer:    &stubInfer{answers: []string{`{"a":1}`, `{"b":2}`}},
		Registry: &stubRegistry{reports: []any{&tools.ValidationReport{Valid: false}}},
		Audit:    &recorder{},
		Guard: GuardFunc(func(context.Context) error {
			calls++
			if calls > 1 { // allowed to start, revoked before the tool call
				return errors.New("revoked mid-run")
			}
			return nil
		}),
		Budget: Budget{MaxIterations: 3}, RunId: "run-mid",
	}
	res, err := l.Run(context.Background(), task())
	if err == nil {
		t.Fatal("expected the mid-run revocation to stop the run")
	}
	if res.Outcome != OutcomeFailed {
		t.Errorf("outcome = %q, want failed", res.Outcome)
	}
}

// A guard that cannot reach its source refuses rather than failing open.
// Refusing because the check could not run is the safe direction, and the error
// says which it was.
func TestPgGuard_UnreachableSourceRefuses(t *testing.T) {
	g := &PgGuard{DB: brokenDB{}, Email: "agent@x", Capability: "run_pipelines"}
	err := g.Allowed(context.Background())
	if err == nil {
		t.Fatal("expected a refusal when the check cannot run")
	}
	if !strings.Contains(err.Error(), "cannot verify") {
		t.Errorf("the error does not distinguish 'could not check' from 'not permitted': %v", err)
	}
}

// A guard configured with no capability is a guard that checks nothing, which
// is more dangerous than none because it looks like protection.
func TestPgGuard_NoCapabilityConfiguredIsAnError(t *testing.T) {
	g := &PgGuard{DB: brokenDB{}, Email: "agent@x"}
	if err := g.Allowed(context.Background()); err == nil {
		t.Error("a guard with no capability must refuse rather than allow")
	}
}

// The real query, against a real database: granted, then revoked.
func TestCriterion18_PgGuardAgainstPostgres(t *testing.T) {
	pool := testPool(t)
	ctx := context.Background()
	seedIdentity(t, pool)

	g := &PgGuard{DB: pool, Email: "agent@example.com", Capability: "run_pipelines"}
	if err := g.Allowed(ctx); err != nil {
		t.Fatalf("the identity holds the capability and was refused: %v", err)
	}

	// Revoke it the way an operator would — remove the capability from the role.
	if _, err := pool.Exec(ctx,
		`DELETE FROM jetsapi.role_capability WHERE role = 'agent_role' AND capability = 'run_pipelines'`); err != nil {
		t.Fatal(err)
	}

	// No restart, no new object: the same guard must now refuse. A guard that
	// cached its answer would still allow, which is the defect this shape
	// exists to avoid.
	err := g.Allowed(ctx)
	if err == nil {
		t.Fatal("the capability was revoked and the guard still allows; revocation is not durable")
	}
	if !strings.Contains(err.Error(), "run_pipelines") {
		t.Errorf("the refusal does not name the capability: %v", err)
	}
}

// seedIdentity creates the minimum an agent identity needs: a user row with a
// role, and that role holding a capability.
func seedIdentity(t *testing.T, pool *pgxpool.Pool) {
	t.Helper()
	ctx := context.Background()
	for _, stmt := range []string{
		`CREATE SCHEMA IF NOT EXISTS jetsapi`,
		`CREATE TABLE IF NOT EXISTS jetsapi.users (
		   user_email text PRIMARY KEY, name text, password text,
		   encrypted_roles text[], is_active boolean DEFAULT true,
		   git_name text, git_email text, git_handle text)`,
		`CREATE TABLE IF NOT EXISTS jetsapi.role_capability (
		   role text, capability text, UNIQUE (role, capability))`,
		`INSERT INTO jetsapi.users (user_email, name, encrypted_roles, is_active)
		 VALUES ('agent@example.com', 'agent', ARRAY['agent_role'], true)
		 ON CONFLICT (user_email) DO UPDATE SET encrypted_roles = EXCLUDED.encrypted_roles,
		                                        is_active = EXCLUDED.is_active`,
		`INSERT INTO jetsapi.role_capability (role, capability)
		 VALUES ('agent_role', 'run_pipelines') ON CONFLICT DO NOTHING`,
	} {
		if _, err := pool.Exec(ctx, stmt); err != nil {
			t.Fatalf("seeding: %v\n%s", err, stmt)
		}
	}
	t.Cleanup(func() {
		_, _ = pool.Exec(context.Background(),
			`DELETE FROM jetsapi.users WHERE user_email = 'agent@example.com'`)
		_, _ = pool.Exec(context.Background(),
			`DELETE FROM jetsapi.role_capability WHERE role = 'agent_role'`)
	})
}

type brokenDB struct{}

func (brokenDB) QueryRow(context.Context, string, ...any) pgx.Row { return errRow{} }

type errRow struct{}

func (errRow) Scan(...any) error { return errors.New("no database") }

type recorderFunc struct{ start func() error }

func (r recorderFunc) Start(context.Context, []byte) error      { return r.start() }
func (recorderFunc) Finish(context.Context, Outcome, int) error { return nil }
