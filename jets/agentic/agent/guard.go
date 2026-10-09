package agent

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5"
)

// The kill switch (plan §3.5, task D.6, criterion 18).
//
// Two different things wear this name and only one of them is new. Per-run
// cancellation is a context and D.5 built it: the caller cancels, the run ends
// as interrupted. **A durable global stop is the one that matters**, because it
// must survive a process restart and must not be defeatable by the agent — and
// it already exists as the capability model. An agent identity is a user with a
// capability set, so revoking that capability stops it from outside, with an
// existing screen and an existing audit trail.
//
// What Phase 1 adds is only the check. That is the whole of the task and it is
// the right size: two lines of policy over machinery that exists, rather than a
// new switch with a new table and a new way to get it wrong.
//
// **The check must re-read, and that is the part that is easy to get wrong.**
// user.User loads its capabilities once, at construction, from role_capability.
// A loop holding a *user.User would therefore keep whatever it was granted at
// start-up, and revocation would take effect at the next process restart —
// which is precisely the property this control claims to have and would not.
// So Guard is consulted per check and PgGuard queries every time.
//
// **Two honest limits, recorded rather than papered over:**
//
//   - It stops an *identity*, not a run, and takes effect at the next check
//     rather than immediately. A per-run stop an operator can hit from a screen
//     is gap 11's IDE work in Phase 2.
//   - user.HasCapability returns true unconditionally for the admin identity
//     (user.go:70, via IsAdmin). An agent running as admin therefore cannot be
//     stopped this way. Agent identities must not be admin, which is a
//     deployment rule this code cannot enforce and gap 7 should.

// Guard answers whether an identity may still act. It is consulted before the
// first model call and before every tool call that writes, so an implementation
// must be cheap and must not cache.
type Guard interface {
	// Allowed returns nil when the identity may act, and an error naming the
	// missing capability otherwise.
	Allowed(ctx context.Context) error
}

// GuardFunc adapts a function to Guard.
type GuardFunc func(ctx context.Context) error

func (f GuardFunc) Allowed(ctx context.Context) error { return f(ctx) }

// QueryRower is the slice of pgx PgGuard needs.
type QueryRower interface {
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
}

// PgGuard checks an identity's capability against role_capability, freshly,
// every time it is asked.
type PgGuard struct {
	DB QueryRower
	// Email identifies the agent, as a row in jetsapi.users.
	Email string
	// Capability is what the agent must hold to act. Phase 1 does not seed a
	// role or fix a name — agent identities and their tiers are gap 7's — so
	// this is configuration rather than a constant, and a loop with no Guard
	// runs unguarded and says so by having none.
	Capability string
}

func (g *PgGuard) Allowed(ctx context.Context) error {
	if g.Capability == "" {
		return fmt.Errorf("agent: no capability configured to check; a guard that checks nothing is worse than none")
	}
	// The join is against the user's own roles, so this reads exactly what
	// user.User would have read at construction — the difference is only that
	// it reads it now rather than then.
	var ok bool
	err := g.DB.QueryRow(ctx,
		`SELECT EXISTS (
		   SELECT 1
		     FROM jetsapi.users u
		     JOIN jetsapi.role_capability rc ON rc.role = ANY (u.encrypted_roles)
		    WHERE u.user_email = $1
		      AND u.is_active
		      AND rc.capability = $2)`,
		g.Email, g.Capability).Scan(&ok)
	if err != nil {
		// A guard that cannot reach its source must not fail open. Refusing to
		// act because the check could not run is the safe direction, and it is
		// distinguishable from a refusal because the error says so.
		return fmt.Errorf("agent: cannot verify that %s still holds %q, refusing to act: %w",
			g.Email, g.Capability, err)
	}
	if !ok {
		return fmt.Errorf("agent: %s does not hold the %q capability", g.Email, g.Capability)
	}
	return nil
}
