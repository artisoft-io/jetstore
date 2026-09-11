package briefingtmpl_test

// **The drift guard.** `BA.2` inlines `patient_profile_briefing.json` into
// `patient_profile_template.pc.json`'s `text_templates` array, because Q-104
// keeps the template body inline in the pipeline configuration by the user's
// instruction. That is a second copy of these bytes, and this file is what
// keeps the two equal.
//
// # Why it skips by default, and why that is not a cop-out
//
// The workspaces are submodules. A test that fails when `workspaces/jets_ws` is
// not checked out is a test that fails on a fresh clone, and a suite that is red
// for a known reason is a suite people stop reading - which is the argument
// `generate-doc.sh`'s missing-glyph baseline is built on. So it follows
// `JETS_PC_CORPUS_DIR`'s established pattern: skip unless pointed at a corpus
// (`error_channel_default_corpus_test.go:37`).
//
//	JETS_PC_CORPUS_DIR=<...>/workspaces go test -count=1 ./jets/agentic/briefing/briefingtmpl/
//
// **`-count=1` is not optional and the reason is in the repository's own
// `CLAUDE.md`**: the corpus lives outside the Go module, so nothing in the test
// cache key changes when a submodule moves and a stale `ok` is
// indistinguishable from a real one.
//
// # The polarity is deliberate
//
// A missing `patient_profile_template.pc.json` **skips**, because `BA.2` has not
// run yet and a guard that fails before its subject exists teaches nothing. A
// file that exists and does not carry the document, or carries a different one,
// **fails**. So this test goes from inert to load-bearing on the day `BA.2`
// lands, with no edit.

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/artisoft-io/jetstore/jets/agentic/briefing/briefingtmpl"
	"github.com/artisoft-io/jetstore/jets/agentic/template"
)

// pipelineFile is what `BA.2` writes. It is named rather than searched for by
// key, so that a run against a corpus that does not carry it says *this file is
// not here* rather than *no template anywhere matches*.
const pipelineFile = "patient_profile_template.pc.json"

func TestTheShippedPipelineCarriesThisDocument(t *testing.T) {
	dir := os.Getenv("JETS_PC_CORPUS_DIR")
	if dir == "" {
		t.Skip("JETS_PC_CORPUS_DIR is not set; see the header of this file")
	}
	path := findPipeline(t, dir)
	if path == "" {
		t.Skipf("no %s under %s; BA.2 is the task that writes it", pipelineFile, dir)
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("reading %s: %v", path, err)
	}
	// Only `text_templates` is decoded. The rest of a `.pc.json` is `AZ`'s and
	// `BA`'s business, and decoding it here would make this test fail for
	// reasons that are not about the template.
	var config struct {
		TextTemplates []*template.Spec `json:"text_templates"`
	}
	if err := json.Unmarshal(raw, &config); err != nil {
		t.Fatalf("decoding %s: %v", path, err)
	}
	var shipped *template.Spec
	for _, s := range config.TextTemplates {
		if s != nil && s.Key == briefingtmpl.Key {
			shipped = s
			break
		}
	}
	if shipped == nil {
		t.Fatalf("%s carries no text_templates entry keyed %q; the pipeline and this package have "+
			"diverged, and this package is the one copy", path, briefingtmpl.Key)
	}
	var canonical template.Spec
	if err := json.Unmarshal(briefingtmpl.Document, &canonical); err != nil {
		t.Fatalf("decoding the canonical document: %v", err)
	}
	// **Compared as decoded documents rather than as bytes**, because the two
	// files are indented differently and a whitespace difference is not a
	// drift. Every field that changes what is rendered - and the comments,
	// which change what a reader understands - is in the decoded form.
	if !reflect.DeepEqual(&canonical, shipped) {
		t.Errorf("%s carries a different document from patient_profile_briefing.json.\n"+
			"The canonical copy is this package's; re-inline it rather than editing the pipeline.\n"+
			"--- canonical ---\n%s\n--- shipped ---\n%s",
			path, mustIndent(t, &canonical), mustIndent(t, shipped))
	}
}

func findPipeline(t *testing.T, dir string) string {
	t.Helper()
	var found string
	err := filepath.WalkDir(dir, func(p string, e os.DirEntry, err error) error {
		if err != nil || e.IsDir() || e.Name() != pipelineFile {
			return nil //nolint:nilerr // an unreadable subtree is not this test's subject
		}
		found = p
		return filepath.SkipAll
	})
	if err != nil {
		t.Fatalf("walking %s: %v", dir, err)
	}
	return found
}

func mustIndent(t *testing.T, v any) string {
	t.Helper()
	b, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		t.Fatalf("re-encoding: %v", err)
	}
	return string(b)
}
