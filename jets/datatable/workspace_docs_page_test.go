package datatable

import (
	"os"
	"regexp"
	"strings"
	"testing"

	"github.com/artisoft-io/jetstore/jets/wsvalidate"
)

const goodDocsPage = "---\ntitle: The observe operator\n" +
	"description: demo_observe, a pass-through operator that counts the records of a step.\n" +
	"kind: reference\n---\n\nThe body.\n"

func TestADocsPageWithTheFrontMatterTheBuildRequiresIsAccepted(t *testing.T) {
	for name, content := range map[string]string{
		"the usual page":            goodDocsPage,
		"quoted values":             "---\ntitle: \"A: page\"\ndescription: 'Why: this'\nkind: \"how-to\"\n---\n",
		"Starlight's other fields":  "---\ntitle: A\ndescription: B\nkind: overview\nsidebar:\n  order: 1\n---\n",
		"leading blank lines":       "\n\n---\ntitle: A\ndescription: B\nkind: tutorial\n---\n",
		"a byte order mark":         "\xef\xbb\xbf---\ntitle: A\ndescription: B\nkind: explanation\n---\n",
		"CRLF line ends":            "---\r\ntitle: A\r\ndescription: B\r\nkind: reference\r\n---\r\n",
		"an empty title":            "---\ntitle: ''\ndescription: B\nkind: reference\n---\n",
		"TOML, read but unchecked":  "+++\ntitle = \"A\"\n+++\n",
		"a key that is not text":    "---\n1: one\ntitle: A\ndescription: B\nkind: reference\n---\n",
		"a closing fence mid-line":  "---\ntitle: A\ndescription: B\nkind: reference\n---more\n",
		"an MDX page's import":      "---\ntitle: A\ndescription: B\nkind: reference\n---\nimport X from './x.astro';\n",
		"a multi-line description":  "---\ntitle: A\ndescription: >\n  One line\n  and another.\nkind: reference\n---\n",
		"front matter after spaces": "  \n---\ntitle: A\ndescription: B\nkind: reference\n---\n",
	} {
		if findings := validateDocsPage(content); len(findings) > 0 {
			t.Errorf("%s: refused, want accepted: %v", name, findings)
		}
	}
}

func TestADocsPageTheBuildWouldRefuseIsRefused(t *testing.T) {
	for name, tc := range map[string]struct {
		content string
		path    string // the finding's path, or "" for the whole front matter
		message string // a fragment of the message
	}{
		"no front matter":          {"# A page\n\nText.\n", "", "no front matter"},
		"front matter not at top":  {"Intro\n---\ntitle: A\n---\n", "", "no front matter"},
		"unclosed front matter":    {"---\ntitle: A\ndescription: B\nkind: reference\n", "", "no front matter"},
		"invalid YAML":             {"---\ntitle: [A\ndescription: B\nkind: reference\n---\n", "", "not valid YAML"},
		"a duplicated field":       {"---\ntitle: A\ntitle: B\ndescription: C\nkind: reference\n---\n", "", "not valid YAML"},
		"a list, not fields":       {"---\n- title\n- kind\n---\n", "", "set of fields"},
		"empty front matter":       {"---\n---\n", "/title", "required"},
		"no title":                 {"---\ndescription: B\nkind: reference\n---\n", "/title", "required"},
		"a null title":             {"---\ntitle:\ndescription: B\nkind: reference\n---\n", "/title", "required"},
		"a number for a title":     {"---\ntitle: 2026\ndescription: B\nkind: reference\n---\n", "/title", "text"},
		"a date for a title":       {"---\ntitle: 2026-01-02\ndescription: B\nkind: reference\n---\n", "/title", "text"},
		"no description":           {"---\ntitle: A\nkind: reference\n---\n", "/description", "required"},
		"an empty description":     {"---\ntitle: A\ndescription: ''\nkind: reference\n---\n", "/description", "not empty"},
		"no kind":                  {"---\ntitle: A\ndescription: B\n---\n", "/kind", "required"},
		"a kind outside the five":  {"---\ntitle: A\ndescription: B\nkind: guide\n---\n", "/kind", "one of"},
		"a kind in the wrong case": {"---\ntitle: A\ndescription: B\nkind: Reference\n---\n", "/kind", "one of"},
		"a list for a kind":        {"---\ntitle: A\ndescription: B\nkind: [reference]\n---\n", "/kind", "one of"},
	} {
		findings := validateDocsPage(tc.content)
		errs := wsvalidate.ErrorsOnly(findings)
		if len(errs) == 0 {
			t.Errorf("%s: accepted, want refused", name)
			continue
		}
		found := false
		for _, f := range errs {
			if f.Path == tc.path && strings.Contains(f.Message, tc.message) {
				found = true
			}
		}
		if !found {
			t.Errorf("%s: findings %v, want one at %q saying %q", name, errs, tc.path, tc.message)
		}
	}
}

// Every missing field is named at once, so one save says everything to fix.
func TestADocsPageMissingEveryFieldNamesEachOne(t *testing.T) {
	findings := validateDocsPage("---\nsidebar:\n  order: 1\n---\n")
	var paths []string
	for _, f := range findings {
		paths = append(paths, f.Path)
	}
	if strings.Join(paths, ",") != "/title,/description,/kind" {
		t.Errorf("finding paths %v, want /title, /description and /kind", paths)
	}
}

// Through the save path: a page under docs/ is checked, and the message names
// the file and the field.
func TestTheSavePathChecksADocsPagesFrontMatter(t *testing.T) {
	err := checkWorkspaceFile("docs/customisation/observe.md", "---\ntitle: A\n---\n")
	if err == nil {
		t.Fatal("a page with no description and no kind was saved")
	}
	for _, want := range []string{"docs/customisation/observe.md cannot be saved", "/description", "/kind"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("error %q does not contain %q", err, want)
		}
	}
	if err := checkWorkspaceFile("docs/index.mdx", goodDocsPage); err != nil {
		t.Errorf("docs/index.mdx: %v, want saved", err)
	}
	// A file Starlight's loader skips is not a page, so it is not checked.
	if err := checkWorkspaceFile("docs/_snippet.md", "Some text.\n"); err != nil {
		t.Errorf("docs/_snippet.md: %v, want saved", err)
	}
}

// docsPageKinds is a copy of the documentation site's PAGE_KINDS. Read the
// site's file and compare, so that a kind added there is not refused here.
func TestDocsPageKindsAreTheSitesKinds(t *testing.T) {
	const config = "../../docs/src/content.config.ts"
	src, err := os.ReadFile(config)
	if err != nil {
		t.Fatalf("reading %s: %v", config, err)
	}
	m := regexp.MustCompile(`PAGE_KINDS\s*=\s*\[([^\]]*)\]`).FindSubmatch(src)
	if m == nil {
		t.Fatalf("%s: no PAGE_KINDS array", config)
	}
	var kinds []string
	for _, q := range regexp.MustCompile(`'([^']*)'|"([^"]*)"`).FindAllSubmatch(m[1], -1) {
		kinds = append(kinds, string(q[1])+string(q[2]))
	}
	if strings.Join(kinds, ",") != strings.Join(docsPageKinds, ",") {
		t.Errorf("the site's PAGE_KINDS are %v and docsPageKinds %v; update docsPageKinds", kinds, docsPageKinds)
	}
}
