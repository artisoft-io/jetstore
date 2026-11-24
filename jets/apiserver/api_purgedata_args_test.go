package main

import (
	"slices"
	"testing"
)

// A reset that runs the init scripts runs the base init too. Without it a new
// installation, whose first start is such a reset, had no roles.
func TestResetDomainTablesRunsTheBaseInit(t *testing.T) {
	args := resetDomainTablesArgs(true, false)
	for _, want := range []string{"-drop", "-migrateDb", "-initBaseWorkspaceDb", "-initWorkspaceDb"} {
		if !slices.Contains(args, want) {
			t.Errorf("args %v lack %s", args, want)
		}
	}
	if args := resetDomainTablesArgs(false, true); slices.Contains(args, "-initBaseWorkspaceDb") ||
		slices.Contains(args, "-initWorkspaceDb") || !slices.Contains(args, "-usingSshTunnel") {
		t.Errorf("a reset without init scripts: %v", args)
	}
}
