package main

import (
	"os"
	"strings"
	"testing"
)

// TestRawQueryToolIsGatedMoreTightlyThanRawQuery pins the split I-2 introduced.
//
// The two shared a case and required nothing. `raw_query` carries a query the
// application composed; `raw_query_tool` carries whatever the user typed into
// the IDE's SQL box. `jets_init_db.sql` already says workspace_ide covers "the
// query tool" — this is that sentence, enforced.
func TestRawQueryToolIsGatedMoreTightlyThanRawQuery(t *testing.T) {
	src, err := os.ReadFile("api_tables.go")
	if err != nil {
		t.Fatalf("reading api_tables.go: %v", err)
	}
	text := string(src)

	if strings.Contains(text, `case "raw_query", "raw_query_tool":`) {
		t.Fatal("raw_query and raw_query_tool share a case again; they need different capabilities")
	}
	for _, want := range []string{
		`case "raw_query":`,
		"datatable.CapabilityReadData",
		`case "raw_query_tool":`,
		"datatable.CapabilityQueryTool",
	} {
		if !strings.Contains(text, want) {
			t.Errorf("api_tables.go no longer contains %q", want)
		}
	}
}
