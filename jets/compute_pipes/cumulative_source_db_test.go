package compute_pipes

import (
	"context"
	"strings"
	"testing"

	"github.com/artisoft-io/jetstore/jets/sessionpurge/purgetest"
)

// loadDeletedSessions against a scratch PostgreSQL, named by JETS_TEST_DSN (see
// the header of jets/sessionpurge/purgetest); skipped when it is unset. Run with
// -count=1.
func TestLoadDeletedSessionsListsTheClientsUserDeletions(t *testing.T) {
	pool := purgetest.DB(t, "cpipes_deleted_sessions")
	ctx := context.Background()
	env := map[string]any{}
	if err := loadDeletedSessions(ctx, pool, "jets_ws", env); err != nil {
		t.Fatal(err)
	}
	if got, ok := env[deletedSessionsEnv]; !ok || got != "" {
		t.Errorf("no deletion: got %v (set %v), want \"\"", got, ok)
	}

	for _, r := range []struct{ id, client, reason, status string }{
		{"1792000000003", "jets_ws", "user", "failed"},
		{"1792000000001", "jets_ws", "user", "pending"},
		{"1792000000002", "jets_ws", "user", "done"},
		{"1792000000004", "jets_ws", "retention", "done"},
		{"1792000000005", "jets_ws", "orphan", "pending"},
		{"1792000000006", "other_ws", "user", "done"},
	} {
		purgetest.Exec(t, pool, `INSERT INTO jetsapi.session_deletion
			(session_id, client, reason, requested_by, status) VALUES ($1, $2, $3, 'u@example.com', $4)`,
			r.id, r.client, r.reason, r.status)
	}
	env = map[string]any{}
	if err := loadDeletedSessions(ctx, pool, "jets_ws", env); err != nil {
		t.Fatal(err)
	}
	if want := "1792000000001,1792000000002,1792000000003"; env[deletedSessionsEnv] != want {
		t.Errorf("got %v, want %s", env[deletedSessionsEnv], want)
	}

	// No pool or no client: set, and empty.
	for _, c := range []struct {
		name   string
		client string
		nopool bool
	}{{"no pool", "jets_ws", true}, {"no client", "", false}} {
		env = map[string]any{deletedSessionsEnv: "stale"}
		p := pool
		if c.nopool {
			p = nil
		}
		if err := loadDeletedSessions(ctx, p, c.client, env); err != nil || env[deletedSessionsEnv] != "" {
			t.Errorf("%s: got %v, %v; want \"\", nil", c.name, env[deletedSessionsEnv], err)
		}
	}

	// A database that predates the table: an error naming the migration.
	purgetest.Exec(t, pool, `DROP TABLE jetsapi.session_deletion`)
	env = map[string]any{}
	err := loadDeletedSessions(ctx, pool, "jets_ws", env)
	if err == nil || !strings.Contains(err.Error(), "update_db -migrateDb") {
		t.Errorf("missing table: got %v, want an error naming update_db -migrateDb", err)
	}
	if got, ok := env[deletedSessionsEnv]; !ok || got != "" {
		t.Errorf("missing table: got %v (set %v), want \"\"", got, ok)
	}
}
