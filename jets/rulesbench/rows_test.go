package rulesbench

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestRenderCellKeepsTypeAndOrder(t *testing.T) {
	day := time.Date(2025, 8, 14, 0, 0, 0, 0, time.UTC)
	for _, c := range []struct {
		in   any
		want string
	}{
		{nil, "null"},
		{"a\tb", `string:"a\tb"`},
		{1, `"int:1"`},
		{1.0, "float64:1"},
		{0.1, "float64:0.1"},
		{day, "time:2025-08-14T00:00:00Z UTC"},
		{[]any{"b", 2, nil}, `[string:"b", "int:2", null]`},
	} {
		if got := renderCell(c.in); got != c.want {
			t.Errorf("renderCell(%#v) = %s, want %s", c.in, got, c.want)
		}
	}
}

// A session's write error is its own, and its files are closed when it is done.
func TestRowWriterErrorIsTheSessions(t *testing.T) {
	dir := t.TempDir()
	w := NewRowWriter(dir)
	good := DumpPoint{Engine: "go", Session: "good", Repetition: 1}
	bad := DumpPoint{Engine: "go", Session: "bad", Repetition: 1}
	// a file where bad's session directory would be
	if err := os.MkdirAll(filepath.Join(dir, "r01", "go"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "r01", "go", "bad"), nil, 0o644); err != nil {
		t.Fatal(err)
	}
	w.Write(good, "c", []any{1})
	w.Write(bad, "c", []any{1})
	if err := w.SessionDone(good); err != nil {
		t.Errorf("good session: %v", err)
	}
	if err := w.SessionDone(bad); err == nil {
		t.Errorf("bad session: no error")
	}
	if len(w.open) != 0 {
		t.Errorf("%d sessions' files open after both are done", len(w.open))
	}
	if raw, err := os.ReadFile(filepath.Join(dir, "r01", "go", "good", "c.rows")); err != nil || string(raw) != "\"int:1\"\n" {
		t.Errorf("good session's rows %q, %v", raw, err)
	}
	if err := w.Close(); err != nil {
		t.Errorf("Close after both sessions are done: %v", err)
	}
}
