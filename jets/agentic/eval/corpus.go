package eval

import (
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// The corpus, its split, and the cases drawn from it (decision 13).
//
// **Live means workspaces/*/pipes_config/**.** The .pc.json under */data/ are
// developer reference material JetStore never loads, and counting them
// manufactures contradictions with the validator — that is I-13, and it moved
// the corpus from 71 files to 45. This walk follows the same rule as the track-B
// validator so the two cannot disagree about what the corpus is.

// Instance is one transformation instance in one file.
type Instance struct {
	File     string
	Operator string
	// Path locates the `apply` array holding it: alternating map keys and
	// array indices from the document root. Index is its position within that
	// array.
	//
	// **A path rather than fixed indices, because transformations nest.** The
	// first version of this walk read only the top-level pipes of the two
	// authored shapes and found 257 instances across 8 operators. The corpus
	// holds 458 across 14 — the figure every count in this project rests on —
	// because `apply` arrays also occur inside conditional overrides and nested
	// pipes. A flat walk undercounts by 44% and, worse, reports ten operators
	// as untested when they have instances, which is precisely the failure
	// criterion 20's denominators exist to prevent.
	Path  []Step
	Index int
}

// Step is one hop of a path: either a map key or an array index.
type Step struct {
	Key   string
	Index int
}

func key(k string) Step { return Step{Key: k, Index: -1} }
func at(i int) Step     { return Step{Index: i} }

// Corpus is the live configs, indexed by operator.
type Corpus struct {
	Files     []string
	Instances []Instance
}

// LoadCorpus walks root/workspaces/*/pipes_config/** and enumerates every
// transformation instance.
func LoadCorpus(root string) (*Corpus, error) {
	dirs, err := filepath.Glob(filepath.Join(root, "workspaces", "*", "pipes_config"))
	if err != nil {
		return nil, err
	}
	if len(dirs) == 0 {
		return nil, fmt.Errorf("eval: no workspaces/*/pipes_config under %s", root)
	}
	c := &Corpus{}
	for _, dir := range dirs {
		err := filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
			if err != nil || d.IsDir() || !strings.HasSuffix(path, ".pc.json") {
				return err
			}
			rel, _ := filepath.Rel(root, path)
			raw, err := os.ReadFile(path)
			if err != nil {
				return fmt.Errorf("reading %s: %w", rel, err)
			}
			inst, err := instancesIn(raw, rel)
			if err != nil {
				return fmt.Errorf("parsing %s: %w", rel, err)
			}
			c.Files = append(c.Files, rel)
			c.Instances = append(c.Instances, inst...)
			return nil
		})
		if err != nil {
			return nil, err
		}
	}
	sort.Strings(c.Files)
	return c, nil
}

// instancesIn enumerates the transformation instances of one authored config.
//
// Both authored shapes are walked. I-14 established that the root type carries
// an authored document and a runtime one, and that the corpus is unanimous: 29
// files use conditional_pipes_config, 20 use reducing_pipes_config, none uses
// both and none uses the runtime root. Reading only one would silently measure
// part of the corpus.
func instancesIn(raw []byte, file string) ([]Instance, error) {
	var doc any
	if err := json.Unmarshal(raw, &doc); err != nil {
		return nil, err
	}
	var out []Instance
	var walk func(node any, path []Step)
	walk = func(node any, path []Step) {
		switch t := node.(type) {
		case map[string]any:
			for k, v := range t {
				child := append(append([]Step{}, path...), key(k))
				if k == "apply" {
					if arr, ok := v.([]any); ok {
						for i, e := range arr {
							if m, ok := e.(map[string]any); ok {
								if op, ok := m["type"].(string); ok && op != "" {
									out = append(out, Instance{
										File: file, Operator: op,
										Path: child, Index: i,
									})
								}
							}
							walk(e, append(append([]Step{}, child...), at(i)))
						}
						continue
					}
				}
				walk(v, child)
			}
		case []any:
			for i, e := range t {
				walk(e, append(append([]Step{}, path...), at(i)))
			}
		}
	}
	walk(doc, nil)
	// Deterministic order: a map walk in Go is randomised, and a harness whose
	// case list reshuffles between runs cannot be compared with itself.
	sort.Slice(out, func(i, j int) bool {
		if out[i].Operator != out[j].Operator {
			return out[i].Operator < out[j].Operator
		}
		return pathString(out[i]) < pathString(out[j])
	})
	return out, nil
}

func pathString(i Instance) string {
	var b strings.Builder
	for _, s := range i.Path {
		if s.Key != "" {
			b.WriteString("/" + s.Key)
		} else {
			fmt.Fprintf(&b, "/%d", s.Index)
		}
	}
	fmt.Fprintf(&b, "/%d", i.Index)
	return b.String()
}

// ByOperator counts instances per operator.
func (c *Corpus) ByOperator() map[string]int {
	out := map[string]int{}
	for _, i := range c.Instances {
		out[i.Operator]++
	}
	return out
}

// Split holds out whole files, never instances.
//
// **Instance-level holdout leaks and file-level does not.** Transformation
// instances within one .pc.json share channel names, column vocabularies and
// house idiom, so holding out an instance whose siblings sit in the few-shot
// pool hands over most of the answer. With 45 live files the split is coarse,
// and coarse is the honest option (decision 13).
type Split struct {
	HeldOut []string
	Train   []string
}

// SplitFiles holds out every nth file, which is deterministic and needs no
// seed. Stratification by operator is the caller's to check with Coverage:
// a split that holds out no analyze instance cannot measure analyze, and that
// is a fact about the split worth seeing before a run rather than after.
func (c *Corpus) SplitFiles(everyNth int) (*Split, error) {
	if everyNth < 2 {
		return nil, fmt.Errorf("eval: holding out every %d files leaves nothing to learn from", everyNth)
	}
	s := &Split{}
	for i, f := range c.Files {
		if i%everyNth == 0 {
			s.HeldOut = append(s.HeldOut, f)
		} else {
			s.Train = append(s.Train, f)
		}
	}
	if len(s.HeldOut) == 0 || len(s.Train) == 0 {
		return nil, fmt.Errorf("eval: the split leaves one side empty")
	}
	return s, nil
}

// Coverage reports how many instances of each operator the held-out side
// carries. An operator absent here cannot be measured by this split, which the
// report renders as "not run" rather than as a failure.
func (c *Corpus) Coverage(s *Split) map[string]int {
	held := map[string]bool{}
	for _, f := range s.HeldOut {
		held[f] = true
	}
	out := map[string]int{}
	for _, i := range c.Instances {
		if held[i.File] {
			out[i.Operator]++
		}
	}
	return out
}

// Case is one evaluation case: a config with one instance removed, and the
// instance that was removed.
//
// **Mutation is what gives a case ground truth.** Decision 13 chose it for that
// reason: take a held-out file, delete one transformation, ask for it back from
// a description, and the original is a known-correct answer. That makes a diff
// against the original available — and it is diagnostic colour only. **Only
// compile-pass is the gate**, because a different-but-equally-good answer diffs
// badly and scoring the diff would punish exactly the model behaviour worth
// having.
type Case struct {
	File     string
	Operator string
	// Context is the config with the instance removed — what the model is
	// given.
	Context json.RawMessage
	// Expected is the instance that was removed — ground truth, for the diff
	// and never for the gate.
	Expected json.RawMessage
	// Hole locates what was cut, so an answer can be put back where the
	// removed instance was.
	//
	// **Added at P.1, 2026-08-27, because the first caller could not score a
	// case without it.** A compile-pass gate runs the *whole config* through
	// the startup validator, and a proposed transformation is not a config —
	// so the caller has to splice the answer into the hole before it has
	// anything a verifier will judge. Without this field the path is knowable
	// only to whoever still holds the Instance that made the case, which is an
	// accident of iteration order rather than a property of the case. Four
	// functions could cut a hole and none could fill one, for as long as
	// nothing called them (F32).
	Hole Instance
}

// Fill puts a proposed instance back where the removed one was and returns the
// whole config, which is the artefact a compile-pass gate actually judges.
//
// **It works from Context rather than from the original file**, so what is
// verified is the document the model was given plus the model's answer, and
// nothing else. Verifying against the pristine file would let a context that
// was mangled on the way out still produce a passing case — the failure I-147
// found one layer along, where a stub shaped like the caller agreed with the
// caller by construction.
func (c *Case) Fill(artifact json.RawMessage) ([]byte, error) {
	var proposed any
	if err := json.Unmarshal(artifact, &proposed); err != nil {
		return nil, fmt.Errorf("eval: the proposed instance is not JSON: %w", err)
	}
	var doc any
	if err := json.Unmarshal(c.Context, &doc); err != nil {
		return nil, fmt.Errorf("eval: %s: the case context does not parse: %w", c.File, err)
	}
	holder, err := navigate(doc, c.Hole.Path)
	if err != nil {
		return nil, fmt.Errorf("eval: %s: %w", c.File, err)
	}
	apply, ok := holder.([]any)
	if !ok {
		return nil, fmt.Errorf("eval: %s: %s is not an apply array", c.File, pathString(c.Hole))
	}
	// The index is a position in the *original* array, and the context is that
	// array one shorter, so the hole's index may legitimately equal its length
	// — the instance that was cut was the last one.
	if c.Hole.Index > len(apply) {
		return nil, fmt.Errorf("eval: %s: cannot fill position %d of a %d-element apply array",
			c.File, c.Hole.Index, len(apply))
	}
	filled := make([]any, 0, len(apply)+1)
	filled = append(filled, apply[:c.Hole.Index]...)
	filled = append(filled, proposed)
	filled = append(filled, apply[c.Hole.Index:]...)
	if err := replace(doc, c.Hole.Path, filled); err != nil {
		return nil, fmt.Errorf("eval: %s: %w", c.File, err)
	}
	return json.Marshal(doc)
}

// MakeCase removes one instance from a config and returns both halves.
func MakeCase(raw []byte, inst Instance) (*Case, error) {
	var doc any
	if err := json.Unmarshal(raw, &doc); err != nil {
		return nil, fmt.Errorf("eval: %s does not parse: %w", inst.File, err)
	}
	holder, err := navigate(doc, inst.Path)
	if err != nil {
		return nil, fmt.Errorf("eval: %s: %w", inst.File, err)
	}
	apply, ok := holder.([]any)
	if !ok {
		return nil, fmt.Errorf("eval: %s: %s is not an apply array", inst.File, pathString(inst))
	}
	if inst.Index >= len(apply) {
		return nil, fmt.Errorf("eval: %s: %s has no element %d", inst.File, pathString(inst), inst.Index)
	}
	expected, err := json.Marshal(apply[inst.Index])
	if err != nil {
		return nil, err
	}
	// Remove it in place, leaving the rest of the document exactly as it was:
	// the context is what an author would see with one step missing.
	pruned := append(append([]any{}, apply[:inst.Index]...), apply[inst.Index+1:]...)
	if err := replace(doc, inst.Path, pruned); err != nil {
		return nil, fmt.Errorf("eval: %s: %w", inst.File, err)
	}
	context, err := json.Marshal(doc)
	if err != nil {
		return nil, err
	}
	return &Case{
		File: inst.File, Operator: inst.Operator,
		Context: context, Expected: expected,
		Hole: inst,
	}, nil
}

func navigate(node any, path []Step) (any, error) {
	cur := node
	for _, s := range path {
		if s.Key != "" {
			m, ok := cur.(map[string]any)
			if !ok {
				return nil, fmt.Errorf("expected an object at %q", s.Key)
			}
			cur, ok = m[s.Key]
			if !ok {
				return nil, fmt.Errorf("no key %q", s.Key)
			}
			continue
		}
		arr, ok := cur.([]any)
		if !ok || s.Index >= len(arr) {
			return nil, fmt.Errorf("no element %d", s.Index)
		}
		cur = arr[s.Index]
	}
	return cur, nil
}

// replace sets the value at path. The last step is always a map key — an
// `apply` array is reached by name — so this walks to its parent and assigns.
func replace(node any, path []Step, value any) error {
	if len(path) == 0 {
		return fmt.Errorf("cannot replace the document root")
	}
	parent, err := navigate(node, path[:len(path)-1])
	if err != nil {
		return err
	}
	last := path[len(path)-1]
	if last.Key == "" {
		arr, ok := parent.([]any)
		if !ok || last.Index >= len(arr) {
			return fmt.Errorf("no element %d to replace", last.Index)
		}
		arr[last.Index] = value
		return nil
	}
	m, ok := parent.(map[string]any)
	if !ok {
		return fmt.Errorf("expected an object holding %q", last.Key)
	}
	m[last.Key] = value
	return nil
}
