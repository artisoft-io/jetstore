package workspace

import (
	"sort"

	"github.com/artisoft-io/jetstore/jets/jetrules/rete"
)

// The workspace-wide class view written to build/classes.json.
//
// Each main rule file is compiled on its own, so a class imported by several of
// them is compiled several times, and the resulting nodes are not
// interchangeable: SubClasses is derived from the classes declared in one
// compilation unit, so each node lists only the subclasses that unit could see.
// Keeping one node and discarding the rest gives the workspace-wide view one
// main file's answer to a question that is about the whole workspace — and,
// since the main files are visited in map order, a different answer between
// runs of the same compile.
//
// These two functions fold the nodes together instead. Everything unioned here
// is a set with no meaning in its order; declaration order is preserved where
// it exists (base classes, properties), and SubClasses is sorted because it is
// assembled from several files and has no declaration order to preserve.

// cloneClassNode copies a compiled node, so merging into it does not mutate the
// per-main-file model the caller still holds and has already written to
// <main_rule_file>.model.json.
func cloneClassNode(src *rete.ClassNode) *rete.ClassNode {
	dst := *src
	dst.BaseClasses = append([]string(nil), src.BaseClasses...)
	dst.SubClasses = append([]string(nil), src.SubClasses...)
	dst.DataProperties = append([]rete.PropertyNode(nil), src.DataProperties...)
	dst.ObjectProperties = append([]rete.PropertyNode(nil), src.ObjectProperties...)
	sort.Strings(dst.SubClasses)
	return &dst
}

// mergeClassNode folds src into dst — two compilations of the same class, from
// two main rule files.
func mergeClassNode(dst, src *rete.ClassNode) {
	dst.SubClasses = sortedUnion(dst.SubClasses, src.SubClasses)
	dst.BaseClasses = appendMissing(dst.BaseClasses, src.BaseClasses)
	dst.DataProperties = appendMissingProperties(dst.DataProperties, src.DataProperties)
	dst.ObjectProperties = appendMissingProperties(dst.ObjectProperties, src.ObjectProperties)
	// The remaining fields come from the class declaration and are the same in
	// every unit that saw it; take whichever node has an answer.
	if dst.SourceFileName == "" {
		dst.SourceFileName = src.SourceFileName
	}
	if dst.Type == "" {
		dst.Type = src.Type
	}
	dst.AsTable = dst.AsTable || src.AsTable
}

func sortedUnion(a, b []string) []string {
	seen := make(map[string]bool, len(a)+len(b))
	out := make([]string, 0, len(a)+len(b))
	for _, list := range [][]string{a, b} {
		for _, v := range list {
			if !seen[v] {
				seen[v] = true
				out = append(out, v)
			}
		}
	}
	sort.Strings(out)
	return out
}

func appendMissing(dst, src []string) []string {
	seen := make(map[string]bool, len(dst))
	for _, v := range dst {
		seen[v] = true
	}
	for _, v := range src {
		if !seen[v] {
			seen[v] = true
			dst = append(dst, v)
		}
	}
	return dst
}

func appendMissingProperties(dst, src []rete.PropertyNode) []rete.PropertyNode {
	seen := make(map[string]bool, len(dst))
	for _, p := range dst {
		seen[p.Name] = true
	}
	for _, p := range src {
		if !seen[p.Name] {
			seen[p.Name] = true
			dst = append(dst, p)
		}
	}
	return dst
}
