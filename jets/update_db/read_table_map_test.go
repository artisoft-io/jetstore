package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// A workspace that is not compiled has no build/tables.json. In dev mode the
// domain-table stage is skipped; out of it, which is every deployment, the
// missing file is the error it always was.
func TestReadTableMapWithNoCompiledWorkspace(t *testing.T) {
	missing := filepath.Join(t.TempDir(), "build", "tables.json")

	m, compiled, err := readTableMap(missing, true)
	if err != nil || compiled || m != nil {
		t.Fatalf("dev mode: %v, compiled %v, %v; want a skip", m, compiled, err)
	}

	_, compiled, err = readTableMap(missing, false)
	if err == nil || compiled || !strings.Contains(err.Error(), "while reading table.json json file") {
		t.Fatalf("deployment: compiled %v, %v; want the read error", compiled, err)
	}
}

// A compiled workspace's file is read the same in and out of dev mode, and a
// malformed one is an error in both.
func TestReadTableMapCompiled(t *testing.T) {
	dir := t.TempDir()
	good := filepath.Join(dir, "tables.json")
	if err := os.WriteFile(good, []byte(`{}`), 0o644); err != nil {
		t.Fatal(err)
	}
	bad := filepath.Join(dir, "bad.json")
	if err := os.WriteFile(bad, []byte(`{`), 0o644); err != nil {
		t.Fatal(err)
	}
	for _, dev := range []bool{true, false} {
		m, compiled, err := readTableMap(good, dev)
		if err != nil || !compiled || m == nil {
			t.Fatalf("dev %v: %v, compiled %v, %v", dev, m, compiled, err)
		}
		if _, _, err = readTableMap(bad, dev); err == nil || !strings.Contains(err.Error(), "while unmarshaling tables.json") {
			t.Fatalf("dev %v: malformed file: %v", dev, err)
		}
	}
}
