package compute_pipes

// Every .pc.json committed under jets/ must parse as a ComputePipesConfig.
//
// The corpus tests beside this one (error_channel_default_corpus_test.go and
// its readers) walk the rule workspaces, which live in other repositories, and
// skip unless JETS_PC_CORPUS_DIR names them. That left the configurations this
// repository carries itself -- the test workspace under jets/jetrules/test_ws
// and the shipped assets under jets/workspace_assets -- read by nothing.
//
// **Two of the three under test_ws had never parsed** (jetstore_maintenance_02
// I-316, 2026-10-27; first reported as cgt_test_harness_filters I-17). Both
// wrote "delimiter" as a one-character string, and every Delimiter field in
// pipes_model.go is a rune, which encoding/json reads only from a number:
// `json: cannot unmarshal string into Go struct field ... of type int32`. The
// rule workspaces spell it as a code point (124 for '|'), so the two files now
// do too. This test needs no environment and no database, so it runs in every
// `go test ./jets/compute_pipes/`.

import (
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestEveryInRepoPcJsonParses(t *testing.T) {
	root := ".." // jets/
	var files []string
	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() && d.Name() == "node_modules" {
			return filepath.SkipDir
		}
		if !d.IsDir() && strings.HasSuffix(path, ".pc.json") {
			files = append(files, path)
		}
		return nil
	})
	if err != nil {
		t.Fatalf("walking %s: %v", root, err)
	}
	// A floor rather than a count: the walk finding nothing would pass vacuously.
	if len(files) == 0 {
		t.Fatalf("no .pc.json found under %s; the walk is looking in the wrong place", root)
	}
	for _, path := range files {
		raw, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("%s: %v", path, err)
		}
		doc := string(raw)
		if _, err := UnmarshalComputePipesConfig(&doc); err != nil {
			t.Errorf("%s: %v", path, err)
		}
	}
	t.Logf("%d .pc.json under jets/ checked", len(files))
}
