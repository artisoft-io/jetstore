package optstats

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// statsWith is a file holding the rule sets named, each with one session of
// acme's shape, members as given.
func statsWith(t *testing.T, ruleSets map[string]int) []byte {
	t.Helper()
	a := NewAccumulator()
	for name, members := range ruleSets {
		if err := a.Add(name, count(acmeWanted(), acmeSession(members, 2))); err != nil {
			t.Fatal(err)
		}
	}
	b, err := Marshal(a.File())
	if err != nil {
		t.Fatal(err)
	}
	return b
}

func writeWs(t *testing.T, root, name string, content []byte) {
	t.Helper()
	p := filepath.Join(root, name)
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, content, 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestProcessFileName(t *testing.T) {
	for name, want := range map[string]string{
		"Optimizer_Statistics": "jet_rules/Optimizer_Statistics.optimizer_statistics.json",
		"a-1":                  "jet_rules/a-1.optimizer_statistics.json",
	} {
		if got, err := ProcessFileName(name); err != nil || got != want {
			t.Errorf("%q: got (%q, %v), want %q", name, got, err, want)
		}
		got, _ := ProcessFileName(name)
		if !IsFileName(got) {
			t.Errorf("%s is not recognised as a statistics file", got)
		}
	}
	for _, bad := range []string{"", "a b", "../x", "a/b", "a.b"} {
		if _, err := ProcessFileName(bad); err == nil {
			t.Errorf("%q: accepted", bad)
		}
	}
	for name, want := range map[string]bool{
		FileName:                                    true,
		"jet_rules/x.optimizer_statistics.json":     true,
		"jet_rules/sub/x.optimizer_statistics.json": false,
		"jet_rules/.optimizer_statistics.json":      false,
		"jet_rules/x_optimizer_statistics.json":     false,
		"other/optimizer_statistics.json":           false,
	} {
		if got := IsFileName(name); got != want {
			t.Errorf("IsFileName(%q) = %v, want %v", name, got, want)
		}
	}
}

func TestReadWorkspace(t *testing.T) {
	a, _ := ProcessFileName("A")
	b, _ := ProcessFileName("B")

	if w, err := ReadWorkspace(t.TempDir()); err != nil || w != nil {
		t.Fatalf("no file: got (%v, %v), want (nil, nil)", w, err)
	}

	// Each process's file holds its own rule sets; the shared file fills in the
	// others, and a process's copy of a rule set replaces the shared one
	root := t.TempDir()
	writeWs(t, root, FileName, statsWith(t, map[string]int{"jet_rules/x.jr": 1, "jet_rules/y.jr": 1}))
	writeWs(t, root, a, statsWith(t, map[string]int{"jet_rules/x.jr": 5}))
	writeWs(t, root, b, statsWith(t, map[string]int{"jet_rules/z.jr": 3}))
	writeWs(t, root, "jet_rules/notes.json", []byte("not statistics"))
	w, err := ReadWorkspace(root)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Join(w.Files, " ") != strings.Join([]string{a, b, FileName}, " ") {
		t.Errorf("files %v", w.Files)
	}
	for rs, want := range map[string]string{"jet_rules/x.jr": a, "jet_rules/y.jr": FileName, "jet_rules/z.jr": b} {
		if got := w.RuleSets[rs].File; got != want {
			t.Errorf("%s from %s, want %s", rs, got, want)
		}
	}
	if n, _ := w.RuleSets["jet_rules/x.jr"].RuleSet.N("acme:hasClaim"); n == 0 {
		t.Errorf("x.jr: no count for acme:hasClaim")
	}
	if len(w.Shadowed) != 1 || w.Shadowed[0] != "jet_rules/x.jr" {
		t.Errorf("shadowed %v, want [jet_rules/x.jr]", w.Shadowed)
	}

	// The same rule set in two processes' files: a conflict, with no statistics
	// from either nor from the shared file, and not an error
	writeWs(t, root, b, statsWith(t, map[string]int{"jet_rules/x.jr": 2}))
	w, err = ReadWorkspace(root)
	if err != nil {
		t.Fatalf("two processes' files: %v", err)
	}
	if got := w.Conflicts["jet_rules/x.jr"]; strings.Join(got, " ") != a+" "+b {
		t.Errorf("conflict %v, want [%s %s]", got, a, b)
	}
	if _, ok := w.RuleSets["jet_rules/x.jr"]; ok {
		t.Errorf("a conflicting rule set has statistics")
	}
	if _, ok := w.RuleSets["jet_rules/y.jr"]; !ok {
		t.Errorf("the rule sets around the conflict lost theirs")
	}

	// A process's file reached through a link is read, as the shared file is
	linked := t.TempDir()
	target := filepath.Join(linked, "real.json")
	if err := os.WriteFile(target, statsWith(t, map[string]int{"jet_rules/l.jr": 1}), 0o644); err != nil {
		t.Fatal(err)
	}
	ln, _ := ProcessFileName("L")
	if err := os.Symlink(target, filepath.Join(root, ln)); err != nil {
		t.Fatal(err)
	}
	if w, err = ReadWorkspace(root); err != nil || w.RuleSets["jet_rules/l.jr"].File != ln {
		t.Errorf("a linked process file: %v", err)
	}
	if err := os.Remove(filepath.Join(root, ln)); err != nil {
		t.Fatal(err)
	}

	// A file that does not read: refused, naming it
	writeWs(t, root, b, []byte(`{}`))
	if _, err := ReadWorkspace(root); err == nil || !strings.Contains(err.Error(), b) {
		t.Errorf("a broken file: %v", err)
	}
}
