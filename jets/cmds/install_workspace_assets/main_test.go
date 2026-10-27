package main

import (
	"fmt"
	"strings"
	"testing"

	workspace_assets "github.com/artisoft-io/jetstore/jets/workspace_assets"
)

// The summary counts a removed retired file apart from the assets the workspace
// holds (I-22): before, three removals in a fresh-enough workspace read as three
// more assets.
func TestSummaryCountsRemovedFilesApart(t *testing.T) {
	results := []workspace_assets.Result{
		{Dir: "data_model", Name: "a.json", Action: workspace_assets.Installed},
		{Dir: "user_flows", Name: "b.uf.json", Action: workspace_assets.Unchanged},
		{Dir: "user_flows", Name: "old.uf.json", Action: workspace_assets.Removed},
		{Dir: "table_configs", Name: "c.tc.json", Action: workspace_assets.Updated},
		{Dir: "table_configs", Name: "old1.tc.json", Action: workspace_assets.Removed},
		{Dir: "table_configs", Name: "old2.tc.json", Action: workspace_assets.Removed},
	}
	got := summary("/ws", results)
	for _, want := range []string{
		"3 asset(s) in /ws (",
		"1 in data_model", "0 in pipes_config", "1 in user_flows", "1 in table_configs",
		"; 3 retired file(s) removed (1 in user_flows, 2 in table_configs)",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("summary %q lacks %q", got, want)
		}
	}
}

// A real install into an empty workspace removes nothing, and every result is
// an asset.
func TestSummaryOfAFreshInstall(t *testing.T) {
	dir := t.TempDir()
	results, err := workspace_assets.Install(dir, workspace_assets.Options{})
	if err != nil {
		t.Fatalf("Install: %v", err)
	}
	got := summary(dir, results)
	want := fmt.Sprintf("install_workspace_assets: %d asset(s) in %s (", len(results), dir)
	if !strings.HasPrefix(got, want) || !strings.HasSuffix(got, "; 0 retired file(s) removed") {
		t.Errorf("summary %q, want it to start %q and remove nothing", got, want)
	}
}
