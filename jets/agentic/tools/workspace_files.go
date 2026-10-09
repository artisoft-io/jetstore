package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

// ---------------------------------------------------------------------------
// Tools 5 and 6 — list_workspace_files, read_workspace_file (T.1).
// ---------------------------------------------------------------------------
//
// J.1 decided these two in Phase 2 and built neither (F34). A§5.2 names
// *Read — workspace* as a tool class and no tool read a workspace file, so an
// MCP client authoring against the live workspace could neither fetch a source
// nor discover a path — while compile_rule_file and validate_cpipes_config both
// assume the caller already holds one.
//
// **A workspace-relative path is not the local-disk shape Workspace's comment
// refuses, and the two are worth separating before reading any of this.** That
// comment says tools never take a path; what it is protecting is the *workspace
// root* — the adapter resolves a checkout once and hands over a handle, so the
// registry interface does not acquire the stdio adapter's shape and a future
// exposure over some other transport is not born holding it. A path *inside*
// that workspace says nothing about this machine: "jet_rules/main.jr" is the
// same string in every checkout, and it is meaningless without the handle that
// resolves it. So these signatures take one, and the root still arrives the way
// it always did.
//
// That distinction is also what T.2 negotiates one file over: it is why
// compile_rule_file can take a rule_path instead of a rule_text.
//
// **What confinement means here, and what it does not.** resolveWorkspacePath
// refuses an absolute path, a path with a `..` segment, and — after resolving
// symlinks — anything landing outside the workspace root. It is a guard against
// a *mistaken* path, not a security boundary: J.3 already priced that, and the
// answer is I-69's deferral. Over stdio the client is the party that chose the
// workspace, so a path bounded by that workspace grants nothing it did not
// already have. The moment the catalogue is exposed over HTTP it becomes
// A§5.2's open-proxy hazard, and I-69's trigger is unchanged.

// listableSuffixes is what a workspace's *authored* content is written in.
//
// An allow-list rather than a deny-list, and the reason is the same one
// copyRuleSources gives for copying only .jr: a workspace holds compiled
// artefacts, a SQLite database and, in a checkout, a .git directory, none of
// which a model can read as text and one of which is much the largest thing
// there. Listing by what is authored rather than by what is not compiled means
// a new build output does not have to be remembered here.
var listableSuffixes = map[string]bool{
	".jr":   true, // rule and data-model sources
	".json": true, // pipeline configs, process configs, workspace_control
	".sql":  true, // schema and report SQL
	".csv":  true, // lookup tables
	".md":   true, // workspace documentation
	".txt":  true,
	".yaml": true,
	".yml":  true,
	".sh":   true,
}

// skipDirs are the directories a listing never descends into. `build` holds
// compiled output (F4: a full compile deletes and rewrites it), `.git` is
// version-control metadata, and neither is workspace content an author edits.
var skipDirs = map[string]bool{
	".git":  true,
	"build": true,
}

const (
	defaultListLimit = 500
	maxListLimit     = 5000
	defaultReadBytes = 256 * 1024
	maxReadBytes     = 4 * 1024 * 1024
)

type listFilesArgs struct {
	Pattern    string `json:"pattern"`
	MaxResults int    `json:"max_results"`
}

// WorkspaceFile is one listed file. Size travels with the path because the
// caller's next move is usually read_workspace_file, and a 3 MB lookup table is
// a different decision from a 40-line rule.
type WorkspaceFile struct {
	Path  string `json:"path"`
	Bytes int64  `json:"bytes"`
}

// FileListing is what list_workspace_files returns.
type FileListing struct {
	Files []WorkspaceFile `json:"files"`
	Count int             `json:"count"`
	// Truncated says the limit was reached, so a caller can tell a workspace
	// with no more matches from one whose listing was cut. A count alone
	// cannot: both look like `count == max_results`.
	Truncated bool `json:"truncated,omitempty"`
	// Pattern echoes what was filtered on, empty for an unfiltered listing.
	Pattern string `json:"pattern,omitempty"`
}

func ListWorkspaceFiles(_ context.Context, ws *Workspace, args json.RawMessage) (any, error) {
	var in listFilesArgs
	if len(args) > 0 {
		if err := json.Unmarshal(args, &in); err != nil {
			return nil, fmt.Errorf("while parsing arguments: %w", err)
		}
	}
	limit := defaultListLimit
	if in.MaxResults > 0 {
		limit = in.MaxResults
	}
	if limit > maxListLimit {
		limit = maxListLimit
	}
	var matcher *regexp.Regexp
	if in.Pattern != "" {
		var err error
		matcher, err = compileGlob(in.Pattern)
		if err != nil {
			return nil, err
		}
	}
	root, err := ws.LocalDir()
	if err != nil {
		return nil, err
	}

	out := &FileListing{Files: []WorkspaceFile{}, Pattern: in.Pattern}
	err = filepath.WalkDir(root, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			if skipDirs[d.Name()] {
				return filepath.SkipDir
			}
			return nil
		}
		if !listableSuffixes[strings.ToLower(filepath.Ext(p))] {
			return nil
		}
		rel, err := filepath.Rel(root, p)
		if err != nil {
			return err
		}
		rel = filepath.ToSlash(rel)
		if matcher != nil && !matcher.MatchString(rel) {
			return nil
		}
		info, err := d.Info()
		if err != nil {
			return err
		}
		out.Files = append(out.Files, WorkspaceFile{Path: rel, Bytes: info.Size()})
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("while walking the workspace: %w", err)
	}
	sort.Slice(out.Files, func(i, j int) bool { return out.Files[i].Path < out.Files[j].Path })
	if len(out.Files) > limit {
		out.Files = out.Files[:limit]
		out.Truncated = true
	}
	out.Count = len(out.Files)
	return out, nil
}

type readFileArgs struct {
	Path     string `json:"path"`
	MaxBytes int    `json:"max_bytes"`
}

// FileContent is what read_workspace_file returns.
type FileContent struct {
	Path    string `json:"path"`
	Content string `json:"content"`
	Bytes   int    `json:"bytes"`
	// Truncated says the file is longer than what came back. Reported rather
	// than silently cut, for the reason from_model refuses an over-budget
	// prompt: a caller reasoning about a truncated artefact as though it were
	// whole is the failure, not the truncation.
	Truncated bool `json:"truncated,omitempty"`
	// TotalBytes is the file's own size, so a truncated reply says by how much.
	TotalBytes int64 `json:"total_bytes"`
}

func ReadWorkspaceFile(_ context.Context, ws *Workspace, args json.RawMessage) (any, error) {
	var in readFileArgs
	if err := json.Unmarshal(args, &in); err != nil {
		return nil, fmt.Errorf("while parsing arguments: %w", err)
	}
	if strings.TrimSpace(in.Path) == "" {
		return nil, fmt.Errorf("argument 'path' is required")
	}
	limit := defaultReadBytes
	if in.MaxBytes > 0 {
		limit = in.MaxBytes
	}
	if limit > maxReadBytes {
		limit = maxReadBytes
	}
	rel, abs, err := resolveWorkspacePath(ws, in.Path)
	if err != nil {
		return nil, err
	}
	if !listableSuffixes[strings.ToLower(filepath.Ext(rel))] {
		return nil, fmt.Errorf(
			"%q is not a readable workspace source; this tool reads authored text "+
				"(%s), not compiled output or binary artefacts",
			rel, strings.Join(sortedSuffixes(), " "))
	}
	info, err := os.Stat(abs)
	if err != nil {
		return nil, fmt.Errorf("no file at %q in this workspace (list_workspace_files reports the paths that exist): %w", rel, err)
	}
	if info.IsDir() {
		return nil, fmt.Errorf("%q is a directory; list_workspace_files lists what is in it", rel)
	}
	data, err := os.ReadFile(abs)
	if err != nil {
		return nil, fmt.Errorf("while reading %q: %w", rel, err)
	}
	out := &FileContent{Path: rel, TotalBytes: info.Size()}
	if len(data) > limit {
		data = data[:limit]
		out.Truncated = true
	}
	out.Content = string(data)
	out.Bytes = len(data)
	return out, nil
}

func sortedSuffixes() []string {
	out := make([]string, 0, len(listableSuffixes))
	for s := range listableSuffixes {
		out = append(out, s)
	}
	sort.Strings(out)
	return out
}

// resolveWorkspacePath turns a caller's workspace-relative path into a cleaned
// relative path and an absolute one, refusing anything that leaves the
// workspace. It returns the *cleaned* relative form so diagnostics quote back
// what was actually resolved rather than what was typed.
//
// **Symlinks are resolved before the containment check**, because a link inside
// the workspace pointing out of it would otherwise pass a purely textual test.
// A path that does not exist yet cannot be resolved, so the check falls back to
// the lexical form — which is sound here: both tools read, so a path that does
// not exist fails at the read with a comprehensible error.
func resolveWorkspacePath(ws *Workspace, given string) (rel, abs string, err error) {
	root, err := ws.LocalDir()
	if err != nil {
		return "", "", err
	}
	slashed := filepath.ToSlash(given)
	if path.IsAbs(slashed) || filepath.IsAbs(given) || strings.HasPrefix(slashed, "~") {
		return "", "", fmt.Errorf(
			"argument path must be relative to the workspace root, not an absolute path, got %q", given)
	}
	rel = path.Clean(slashed)
	if rel == "." || rel == "" {
		return "", "", fmt.Errorf("argument path names the workspace root rather than a file")
	}
	if rel == ".." || strings.HasPrefix(rel, "../") {
		return "", "", fmt.Errorf(
			"argument path may not leave the workspace, got %q", given)
	}
	abs = filepath.Join(root, filepath.FromSlash(rel))

	realRoot, err := filepath.EvalSymlinks(root)
	if err != nil {
		realRoot = root
	}
	if real, e := filepath.EvalSymlinks(abs); e == nil {
		within, e2 := filepath.Rel(realRoot, real)
		if e2 != nil || within == ".." || strings.HasPrefix(within, ".."+string(filepath.Separator)) {
			return "", "", fmt.Errorf(
				"argument path resolves outside the workspace, got %q", given)
		}
	}
	return rel, abs, nil
}

// compileGlob turns a shell-style glob over slash-separated paths into a
// regexp. `*` matches within one path segment; `**` matches across segments;
// `?` matches one character that is not a separator.
//
// **Written rather than taken from path.Match, which has no `**`.** A model
// asked for "every rule file" writes `**/*.jr`, and path.Match's `*` stops at a
// separator — so the natural pattern would silently match only the top level
// and report an empty workspace as an answer rather than as a mistake.
func compileGlob(pattern string) (*regexp.Regexp, error) {
	if strings.Contains(pattern, "\\") {
		return nil, fmt.Errorf(
			"pattern %q uses a backslash; workspace paths are slash-separated", pattern)
	}
	var b strings.Builder
	b.WriteString("^")
	for i := 0; i < len(pattern); i++ {
		switch c := pattern[i]; c {
		case '*':
			if i+1 < len(pattern) && pattern[i+1] == '*' {
				i++
				// `**/` also matches zero segments, so `**/*.jr` finds a .jr at
				// the root as well as one three directories down. Without this
				// the pattern a caller means by "anywhere" excludes the top.
				if i+1 < len(pattern) && pattern[i+1] == '/' {
					i++
					b.WriteString("(?:[^/]*/)*")
				} else {
					b.WriteString(".*")
				}
			} else {
				b.WriteString("[^/]*")
			}
		case '?':
			b.WriteString("[^/]")
		default:
			b.WriteString(regexp.QuoteMeta(string(c)))
		}
	}
	b.WriteString("$")
	re, err := regexp.Compile(b.String())
	if err != nil {
		return nil, fmt.Errorf("pattern %q is not a usable glob: %w", pattern, err)
	}
	return re, nil
}
