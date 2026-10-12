package tools

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/artisoft-io/jetstore/jets/compilerv2/compiler"
	"github.com/artisoft-io/jetstore/jets/compilerv2/symbolic"
)

// ---------------------------------------------------------------------------
// Tool 4 — compile_rule_file: the real compiler, on a throwaway workspace.
// ---------------------------------------------------------------------------
//
// The second of gap 6's two verifiers. validate_cpipes_config covers .pc.json;
// this covers .jr, and it is the one that could not be written until the
// compiler reported diagnostics as data — before that, a failing compile
// returned the constant string "compilation failed with errors" and a repair
// prompt built from it could say only that something, somewhere, was wrong.
//
// Three properties, and each is a constraint rather than a preference:
//
//   - **The live workspace is never touched.** The submitted text is compiled
//     against a throwaway copy. Nothing is saved: the compiler runs with
//     saveJson false, so SaveModel is never called and workspace.db is neither
//     rewritten nor deleted. That last part matters — a full compile deletes
//     workspace.db first (Phase 0's F4), which is exactly what a validation
//     tool must not do to a workspace someone is using.
//   - **Imports resolve.** A rule file is textually inlined with its data model
//     before parsing and its class declarations generate rete nodes (F1, F2),
//     so a rule compiled without its workspace's vocabulary is not the same
//     rule. The throwaway carries the workspace's rule sources for that reason,
//     and copying only .jr files keeps it cheap.
//   - **Diagnostics name the authored file.** Which is the whole point, and is
//     what E.1-E.5 built.

// CompileReport is what the tool returns. It mirrors ValidationReport's shape
// deliberately — the loop treats verifiers alike, and two report shapes for the
// same job would be two things to special-case.
type CompileReport struct {
	Valid bool `json:"valid"`
	// Diagnostics carries warnings as well as errors, in emission order. A
	// report can be valid and non-empty; that is a compile that warned.
	Diagnostics []RuleDiagnostic `json:"diagnostics,omitempty"`
	// Source is the JetRules text the 'rules' arm rendered and compiled, so a
	// caller holding a valid report holds the .jr to save. Empty for the other
	// two arms, whose source the caller already has.
	Source string `json:"source,omitempty"`
}

// RuleDiagnostic is a compiler diagnostic, plus — for the 'rules' arm — the
// JSON Pointer of the element of the submitted document it was raised
// against. The compiler reports a line of the rendering, which the author
// never wrote; the pointer is the location the author can act on.
type RuleDiagnostic struct {
	compiler.Diagnostic
	Pointer string `json:"pointer,omitempty"`
}

type compileArgs struct {
	RuleText string `json:"rule_text"`
	// RulePath is I-94's remedy: name a workspace file instead of carrying its
	// text (T.2). J.2 measured the model reproducing a rule file correctly 9
	// times in 18, deterministically, with a verbatim-copy instruction making
	// it worse — and `rule_text` is `{"type":"string"}`, so no schema anywhere
	// can see a dropped import line. A path removes the failure rather than
	// detecting it: the model never holds the text.
	//
	// **Exactly one of the two, not a precedence rule.** A caller sending both
	// has two different intentions and the tool cannot know which, so it says
	// so instead of quietly preferring one. Silently ignoring `rule_text`
	// beside a path would reproduce I-68's own defect — a call that looks
	// answered and compiled something else.
	RulePath string `json:"rule_path"`
	// Rules is I-250's arm: the rules as a symbolic document
	// (jets/compilerv2/symbolic), rendered to JetRules source by the tool.
	// F176 measured the model carrying a cpipes config through this tool
	// family intact 17 and 16 times in 18 and a rule file 6 and 4 times, with
	// every dropped fragment a JetRules construct — a label, an import line.
	// In this arm the model writes none of them: the renderer does. It serves
	// the case the path arm cannot, a rule composed and not saved.
	//
	// **The document's own fields are the arguments**: 'rules' is the array
	// of rules and 'imports' sits beside it. The first shape nested a whole
	// document under 'rules', and the document has a 'rules' of its own: on
	// granite4.1:3b, told the shape in words, the model sent the inner array
	// as the argument 10 times of 10 — the right rules in the wrong place —
	// and, shown an inline example, sent the document as a string, whose
	// quotes around ?variables it rewrote as curly quotes 11 times of 14
	// (I-250, 2026-10-12). A whole document, and a string holding either
	// form, are still accepted, since each has one reading.
	Rules    json.RawMessage `json:"rules"`
	Imports  []string        `json:"imports"`
	FileName string          `json:"file_name"`
}

// defaultCompileFileName is used when the caller names nothing. It is
// deliberately recognisable in a diagnostic: a file name appearing in a repair
// prompt should say where it came from.
const defaultCompileFileName = "agentic_compile_check.jr"

func CompileRuleFile(_ context.Context, ws *Workspace, args json.RawMessage) (any, error) {
	var in compileArgs
	if err := json.Unmarshal(args, &in); err != nil {
		return nil, fmt.Errorf("while parsing arguments: %w", err)
	}
	hasText := strings.TrimSpace(in.RuleText) != ""
	hasPath := strings.TrimSpace(in.RulePath) != ""
	hasRules := len(in.Rules) > 0 && string(in.Rules) != "null"
	switch given := btoi(hasText) + btoi(hasPath) + btoi(hasRules); {
	case given > 1:
		return nil, fmt.Errorf(
			"give exactly one of 'rule_path', 'rules' and 'rule_text': they name different " +
				"rules and this tool will not guess which one you meant")
	case len(in.Imports) > 0 && !hasRules:
		return nil, fmt.Errorf(
			"argument 'imports' belongs with 'rules'; a .jr names its imports in its own text")
	case given == 0:
		return nil, fmt.Errorf(
			"one of 'rule_path' (a workspace-relative .jr, preferred), 'rules' (the rules as a " +
				"JSON document) or 'rule_text' (JetRules source) is required")
	}

	// The rules arm renders first, so a document that cannot be rendered is
	// answered with its location before a workspace is copied.
	var rendered *symbolic.Rendered
	if hasRules {
		var report *CompileReport
		rendered, report = renderRules(in.Rules, in.Imports)
		if report != nil {
			return report, nil
		}
		in.RuleText = rendered.Source
		hasText = true
	}

	wsDir, err := ws.LocalDir()
	if err != nil {
		return nil, err
	}

	// fileName is what the compile is attributed to, and it is also the path
	// inside the throwaway. For the path arm that is the file's own relative
	// path, so a rule sitting in jet_rules/ compiles from jet_rules/ and its
	// imports resolve exactly as they do in the workspace — which is the point
	// of the arm rather than a convenience.
	fileName := ""
	if hasPath {
		rel, _, err := resolveWorkspacePath(ws, in.RulePath)
		if err != nil {
			return nil, err
		}
		if filepath.Ext(rel) != ".jr" {
			return nil, fmt.Errorf("argument 'rule_path' must name a .jr file, got %q", rel)
		}
		if _, err := os.Stat(filepath.Join(wsDir, filepath.FromSlash(rel))); err != nil {
			return nil, fmt.Errorf(
				"no rule file at %q in this workspace (list_workspace_files reports the paths that exist): %w",
				rel, err)
		}
		if in.FileName != "" {
			return nil, fmt.Errorf(
				"argument 'file_name' renames submitted text and does not apply to 'rule_path'; " +
					"a file compiled by path is attributed to its own name")
		}
		fileName = filepath.FromSlash(rel)
	} else {
		fileName, err = compileFileName(in.FileName)
		if err != nil {
			return nil, err
		}
	}

	scratch, err := os.MkdirTemp("", "jets_compile_rule_")
	if err != nil {
		return nil, fmt.Errorf("while creating the throwaway workspace: %w", err)
	}
	defer os.RemoveAll(scratch)

	if err := copyRuleSources(wsDir, scratch); err != nil {
		return nil, err
	}
	// Write the submitted text last, so a caller cannot overwrite one of the
	// workspace's own files in the copy and have the compile silently read
	// something other than what it was given. The path arm writes nothing:
	// copyRuleSources has already staged that file at its own relative path,
	// which is what makes the arm cheap.
	if hasText {
		if err := os.WriteFile(filepath.Join(scratch, fileName), []byte(in.RuleText), 0o600); err != nil {
			return nil, fmt.Errorf("while writing the submitted rule text: %w", err)
		}
	}

	// saveJson false: compile and report, write nothing. autoAddResources
	// true matches how the workspace compile itself runs, so an undeclared
	// resource is the warning it is there rather than an error only here.
	c := compiler.NewCompiler(scratch, fileName, false, false, true)
	err = c.Compile()

	report := &CompileReport{Valid: err == nil}
	for _, d := range c.Diagnostics() {
		rd := RuleDiagnostic{Diagnostic: d}
		if rendered != nil && filepath.Clean(d.File) == filepath.Clean(fileName) {
			rd.Pointer = rendered.PointerAt(d.Line)
		}
		report.Diagnostics = append(report.Diagnostics, rd)
	}
	if rendered != nil {
		report.Source = rendered.Source
	}
	if err != nil {
		var cerr *compiler.CompilationError
		if errors.As(err, &cerr) {
			// The diagnostics are already on the report; the error itself
			// carries no information the caller does not have.
			return report, nil
		}
		// Anything else — an unreadable import, a path escaping the base — is
		// a failure of the tool rather than a verdict about the rule text, and
		// must not be reported as "your rule is invalid".
		return nil, fmt.Errorf("while compiling %s: %w", fileName, err)
	}
	return report, nil
}

// renderRules parses and renders the 'rules' argument. A document that does
// not parse or does not render is a verdict about the rules, not a failure of
// the tool, so it comes back as an invalid report carrying the location — the
// same shape a compile error has, which is what lets a repair loop treat the
// two alike.
func renderRules(raw json.RawMessage, imports []string) (*symbolic.Rendered, *CompileReport) {
	invalid := func(ptr, msg string) *CompileReport {
		return &CompileReport{Diagnostics: []RuleDiagnostic{{
			Diagnostic: compiler.Diagnostic{Severity: compiler.SeverityError, Message: "error: " + msg},
			Pointer:    ptr,
		}}}
	}
	f, err := ReadRulesArgument(raw, imports)
	if err != nil {
		var re *symbolic.RenderError
		if errors.As(err, &re) {
			return nil, invalid(re.Pointer, re.Message)
		}
		return nil, invalid("", err.Error())
	}
	rendered, err := symbolic.Render(f)
	if err != nil {
		var re *symbolic.RenderError
		if errors.As(err, &re) {
			return nil, invalid(re.Pointer, re.Message)
		}
		return nil, invalid("", err.Error())
	}
	return rendered, nil
}

// ReadRulesArgument is how the 'rules' arm reads its arguments into a
// symbolic rule file. It is exported so that tools/tool_conformance judges a
// call by what this tool would compile, not by a reimplementation of it.
//
// The canonical form is 'rules' an array of rules with 'imports' beside it.
// Three other forms are accepted because each has one reading, and the
// measurement found a model sending all three (I-250, 2026-10-12):
//
//   - a whole document, {"imports": [...], "rules": [...]}, under 'rules';
//   - a string holding the array or the document;
//   - the array with a last element {"imports": [...]} and nothing else —
//     an object with no name and no terms cannot be a rule. Only the last
//     element, so that a pointer into /rules/N still names the Nth rule.
//
// Imports given twice — beside a document, or both beside the array and in
// it — are refused rather than merged.
func ReadRulesArgument(raw json.RawMessage, imports []string) (*symbolic.RuleFile, error) {
	doc := []byte(raw)
	var asString string
	if json.Unmarshal(raw, &asString) == nil {
		doc = []byte(asString)
	}
	trimmed := bytes.TrimSpace(doc)
	if len(trimmed) == 0 || trimmed[0] != '[' {
		if len(imports) > 0 {
			return nil, &symbolic.RenderError{Pointer: "/imports",
				Message: "'imports' was given beside a whole rules document; put them in the document or pass 'rules' as an array"}
		}
		f, err := symbolic.Parse(doc)
		if err != nil {
			return nil, fmt.Errorf("the rules do not parse: %w", err)
		}
		return f, nil
	}
	var elems []json.RawMessage
	if err := json.Unmarshal(trimmed, &elems); err != nil {
		return nil, fmt.Errorf("the rules do not parse: %w", err)
	}
	if n := len(elems); n > 0 {
		var last map[string]json.RawMessage
		if json.Unmarshal(elems[n-1], &last) == nil && len(last) == 1 && last["imports"] != nil {
			if len(imports) > 0 {
				return nil, &symbolic.RenderError{Pointer: fmt.Sprintf("/rules/%d", n-1),
					Message: "imports were given both in 'rules' and in 'imports'; give them once, in 'imports'"}
			}
			if err := json.Unmarshal(last["imports"], &imports); err != nil {
				return nil, &symbolic.RenderError{Pointer: fmt.Sprintf("/rules/%d/imports", n-1),
					Message: "imports are a list of .jr paths: " + err.Error()}
			}
			elems = elems[:n-1]
		}
	}
	wrapped, err := json.Marshal(struct {
		Imports []string          `json:"imports,omitempty"`
		Rules   []json.RawMessage `json:"rules"`
	}{imports, elems})
	if err != nil {
		return nil, err
	}
	f, err := symbolic.Parse(wrapped)
	if err != nil {
		return nil, fmt.Errorf("the rules do not parse: %w", err)
	}
	return f, nil
}

func btoi(b bool) int {
	if b {
		return 1
	}
	return 0
}

// compileFileName validates the caller's chosen name. A bare name is required:
// the compiler confines paths to its base directory anyway, but rejecting a
// separator here gives a comprehensible error instead of a confinement failure
// from three layers down.
func compileFileName(name string) (string, error) {
	if name == "" {
		return defaultCompileFileName, nil
	}
	if name != filepath.Base(name) || strings.ContainsAny(name, `/\`) {
		return "", fmt.Errorf(
			"argument 'file_name' must be a bare file name with no path separators, got %q", name)
	}
	if !strings.HasSuffix(name, ".jr") {
		return "", fmt.Errorf("argument 'file_name' must end in .jr, got %q", name)
	}
	return name, nil
}

// copyRuleSources copies the workspace's .jr files into the throwaway, keeping
// their relative paths so import statements resolve unchanged. Only .jr files
// are copied: they are what imports name, they are small, and copying a whole
// client workspace to compile one file would be the expensive way to get the
// same answer.
func copyRuleSources(from, to string) error {
	return filepath.WalkDir(from, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			// Skip the compiled outputs and the version-control metadata;
			// neither is importable and .git is much the largest thing here.
			switch d.Name() {
			case ".git", "build", "process_config":
				return filepath.SkipDir
			}
			return nil
		}
		if filepath.Ext(path) != ".jr" {
			return nil
		}
		rel, err := filepath.Rel(from, path)
		if err != nil {
			return err
		}
		dest := filepath.Join(to, rel)
		if err := os.MkdirAll(filepath.Dir(dest), 0o700); err != nil {
			return err
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return fmt.Errorf("while reading %s: %w", rel, err)
		}
		if err := os.WriteFile(dest, data, 0o600); err != nil {
			return fmt.Errorf("while staging %s: %w", rel, err)
		}
		return nil
	})
}
