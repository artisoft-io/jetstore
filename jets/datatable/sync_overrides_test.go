package datatable

import (
	"errors"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
)

// syncOverridesToLocal is what the UI's compile and commit run first, so that a lookup
// a refresh wrote to the database since the apiserver's last start is the one they use.
func TestSyncOverridesToLocal(t *testing.T) {
	saved := syncWorkspaceOverrides
	t.Cleanup(func() { syncWorkspaceOverrides = saved })

	var gotName, gotType string
	var gotSkipDb, gotSkipTgz bool
	syncWorkspaceOverrides = func(_ *pgxpool.Pool, name, contentType string, skipDb, skipTgz bool) (bool, error) {
		gotName, gotType, gotSkipDb, gotSkipTgz = name, contentType, skipDb, skipTgz
		return true, nil
	}
	if msg := syncOverridesToLocal(nil, "sample_ws"); msg != "" {
		t.Errorf("a sync that succeeds reports nothing, got %q", msg)
	}
	if gotName != "sample_ws" || gotType != "" {
		t.Errorf("synced workspace %q content type %q, want every override of sample_ws", gotName, gotType)
	}
	// The compiled files are what the compile rewrites, and what the commit keeps.
	if !gotSkipDb || !gotSkipTgz {
		t.Errorf("compiled files must be skipped: skipSqlite %v skipTgz %v", gotSkipDb, gotSkipTgz)
	}

	syncWorkspaceOverrides = func(*pgxpool.Pool, string, string, bool, bool) (bool, error) {
		return false, errors.New("connection refused")
	}
	msg := syncOverridesToLocal(nil, "sample_ws")
	if !strings.Contains(msg, "connection refused") {
		t.Errorf("a failed sync must be reported to the action's log, got %q", msg)
	}
}
