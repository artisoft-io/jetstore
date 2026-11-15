package datatable

import (
	"fmt"
	"path"
	"regexp"
	"strings"

	"github.com/artisoft-io/jetstore/jets/datatable/wsfile"
	"github.com/artisoft-io/jetstore/jets/wsvalidate"
	"go.yaml.in/yaml/v3"
)

// The save check of a page of the workspace guide: a `.md` or `.mdx` file under
// a workspace's `docs/` (the IDE's `docs` section, wsfile/sections.go). Added
// 2026-11-13.
//
// **It refuses what the documentation build refuses, and nothing else.** The
// pages are merged into the installation's documentation site at the UI image's
// build (docs/scripts/workspace_docs.mjs copies them as they are, adding
// nothing to their front matter), where the site's content schema
// (docs/src/content.config.ts, on Starlight's docsSchema) fails the build on a
// page with no `title`, an empty or missing `description`, or a `kind` outside
// the five. A page saved in that state fails the next UI image build, far from
// the edit that broke it. So this checks:
//
//   - that the page has front matter, found as Astro finds it (frontmatterRE,
//     @astrojs/internal-helpers' frontmatter.js): a `---` block at the top,
//     after any blank lines;
//   - that the front matter is a YAML mapping that parses;
//   - that `title` is a string, `description` a non-empty string, and `kind`
//     one of docsPageKinds.
//
// **What it leaves to the build**: links, which need the whole site; the
// types of Starlight's optional fields (`sidebar`, `template` and the others);
// an `.mdx` page's body, which only the MDX compiler can judge; front matter
// written in TOML (`+++`), which Astro also reads and this does not parse, so it
// is let through rather than refused; and a page whose name starts with `_`,
// which Starlight's loader skips, so the build does not check it either.
//
// The YAML parser is not the build's (the `yaml` package, core schema), so the
// two could disagree on an unusual document. On 2026-11-13 every case in
// workspace_docs_page_test.go was also run through Astro's own parseFrontmatter
// and the schema's three fields, and the verdicts agreed, a date for a title
// and a duplicated field among them.

// docsPageKinds is the `kind` a page may have: PAGE_KINDS in
// docs/src/content.config.ts. TestDocsPageKindsAreTheSitesKinds reads that file
// and fails when the two differ.
var docsPageKinds = []string{"tutorial", "how-to", "reference", "explanation", "overview"}

// docsFrontMatterRE is Astro's frontmatterRE: an opening fence at the very
// start (after a byte order mark) or after leading whitespace ending in a
// newline, then the front matter, then a closing fence. The fence is `---`
// (YAML) or `+++` (TOML); the opening one decides which.
var docsFrontMatterRE = regexp.MustCompile(`^(?:\x{FEFF}?|\s*\n)(---|\+\+\+)((?s:.*?)\n)(?:---|\+\+\+)`)

// isDocsPage reports whether the file at fileName, a workspace-relative path,
// is one the documentation build reads as a page when its suffix is `.md` or
// `.mdx` (the validator rows decide the suffix): it is under `docs/`
// (wsfile.WritableMarkdownDir, cleaned as IsReadOnly cleans it) and its name
// does not start with `_`. A Markdown file elsewhere is read-only and refused
// before any validator, so this keeps the row from claiming files it never sees.
func isDocsPage(fileName string) bool {
	if !wsfile.InDir(fileName, wsfile.WritableMarkdownDir) {
		return false
	}
	base := path.Base(strings.ReplaceAll(fileName, "\\", "/"))
	return !strings.HasPrefix(base, "_")
}

// validateDocsPage refuses a page whose front matter the documentation build
// would refuse. See the comment at the top of this file.
func validateDocsPage(content string) []wsvalidate.Finding {
	refuse := func(where, format string, args ...any) []wsvalidate.Finding {
		return []wsvalidate.Finding{{
			Severity: wsvalidate.Error,
			Code:     "docs_front_matter",
			Message:  fmt.Sprintf(format, args...),
			Path:     where,
		}}
	}
	m := docsFrontMatterRE.FindStringSubmatch(content)
	if m == nil {
		return refuse("", "the page has no front matter: it must open with a --- block "+
			"giving its title, description and kind (%s)", strings.Join(docsPageKinds, ", "))
	}
	if m[1] == "+++" {
		// TOML front matter: read by the build, not checked here.
		return nil
	}

	var parsed any
	if err := yaml.Unmarshal([]byte(m[2]), &parsed); err != nil {
		return refuse("", "the front matter is not valid YAML: %v", err)
	}
	fields, ok := frontMatterFields(parsed)
	if !ok {
		return refuse("", "the front matter must be a set of fields (title: ..., description: ..., kind: ...)")
	}

	var findings []wsvalidate.Finding
	if v, present := fields["title"]; !present || v == nil {
		findings = append(findings, refuse("/title", "title is required")...)
	} else if _, isString := v.(string); !isString {
		findings = append(findings, refuse("/title", "title must be text")...)
	}
	if v, present := fields["description"]; !present || v == nil {
		findings = append(findings, refuse("/description", "description is required")...)
	} else if s, isString := v.(string); !isString || s == "" {
		findings = append(findings, refuse("/description", "description must be text, not empty")...)
	}
	kinds := strings.Join(docsPageKinds, ", ")
	if v, present := fields["kind"]; !present || v == nil {
		findings = append(findings, refuse("/kind", "kind is required: one of %s", kinds)...)
	} else if s, isString := v.(string); !isString || !isDocsPageKind(s) {
		findings = append(findings, refuse("/kind", "kind must be one of %s, not %v", kinds, v)...)
	}
	return findings
}

// frontMatterFields returns the front matter's top-level fields by name, and
// false when it is not a mapping. An empty front matter is a mapping with no
// fields, as Astro reads it. A key that is not text (`1: x`) is named as YAML
// writes it, which is how the build's parser names it too.
func frontMatterFields(parsed any) (map[string]any, bool) {
	switch m := parsed.(type) {
	case nil:
		return map[string]any{}, true
	case map[string]any:
		return m, true
	case map[any]any:
		fields := make(map[string]any, len(m))
		for k, v := range m {
			fields[fmt.Sprint(k)] = v
		}
		return fields, true
	}
	return nil, false
}

func isDocsPageKind(kind string) bool {
	for _, k := range docsPageKinds {
		if k == kind {
			return true
		}
	}
	return false
}
