package graphdump

import (
	"bufio"
	"fmt"
	"io"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"github.com/artisoft-io/jetstore/jets/compute_pipes"
	"github.com/artisoft-io/jetstore/jets/rulestest/harness"
)

// Options says what WriteSession writes.
type Options struct {
	// IncludeMeta writes the meta graph's triples as well. They are left out by
	// default; see the package documentation for how.
	IncludeMeta bool
	// ExcludePredicates are predicate names, such as "jets:loop", whose triples are
	// left out. They are left out before minted nodes are named, so they do not
	// affect the labels.
	ExcludePredicates []string
}

// WriteSession writes sess as a canonical dump: one line per triple, sorted, each
// ending in a newline. See the package documentation for the format.
func WriteSession(w io.Writer, sess compute_pipes.JetRdfSession, opts Options) error {
	lines, err := Lines(sess, opts)
	if err != nil {
		return err
	}
	bw := bufio.NewWriter(w)
	for _, l := range lines {
		bw.WriteString(l)
		bw.WriteByte('\n')
	}
	return bw.Flush()
}

// Lines is the dump WriteSession writes, as sorted lines without their newline.
func Lines(sess compute_pipes.JetRdfSession, opts Options) ([]string, error) {
	if sess == nil {
		return nil, fmt.Errorf("graphdump: nil session")
	}
	triples, err := readTriples(sess)
	if err != nil {
		return nil, err
	}
	if !opts.IncludeMeta {
		meta, err := metaOf(sess)
		if err != nil {
			return nil, err
		}
		triples = without(triples, meta)
	}
	if len(opts.ExcludePredicates) > 0 {
		excluded := make(map[string]bool, len(opts.ExcludePredicates))
		for _, p := range opts.ExcludePredicates {
			excluded[p] = true
		}
		kept := triples[:0]
		for _, t := range triples {
			if !(t.P.Kind == harness.TermResource && excluded[t.P.Text]) {
				kept = append(kept, t)
			}
		}
		triples = kept
	}
	return canonicalLines(triples), nil
}

// readTriples reads every triple Find returns, decoded, without duplicates.
func readTriples(sess compute_pipes.JetRdfSession) ([]harness.SnapshotTriple, error) {
	itor := sess.Find()
	if itor == nil {
		return nil, fmt.Errorf("graphdump: the session's Find returned no iterator")
	}
	defer itor.Release()
	seen := make(map[tripleKey]bool)
	var triples []harness.SnapshotTriple
	for !itor.IsEnd() {
		s, err := harness.DecodeNode(itor.GetSubject())
		if err != nil {
			return nil, fmt.Errorf("graphdump: subject: %w", err)
		}
		p, err := harness.DecodeNode(itor.GetPredicate())
		if err != nil {
			return nil, fmt.Errorf("graphdump: predicate of %s: %w", s, err)
		}
		o, err := harness.DecodeNode(itor.GetObject())
		if err != nil {
			return nil, fmt.Errorf("graphdump: object of (%s %s): %w", s, p, err)
		}
		t := harness.SnapshotTriple{S: s, P: p, O: o}
		if k := keyOf(t); !seen[k] {
			seen[k] = true
			triples = append(triples, t)
		}
		itor.Next()
	}
	return triples, nil
}

// tripleKey identifies a decoded triple by value.
type tripleKey struct{ s, p, o string }

func termKey(t harness.Term) string { return t.Kind + "\x00" + t.Text }

func keyOf(t harness.SnapshotTriple) tripleKey {
	return tripleKey{termKey(t.S), termKey(t.P), termKey(t.O)}
}

func without(triples []harness.SnapshotTriple, drop map[tripleKey]bool) []harness.SnapshotTriple {
	if len(drop) == 0 {
		return triples
	}
	kept := triples[:0]
	for _, t := range triples {
		if !drop[keyOf(t)] {
			kept = append(kept, t)
		}
	}
	return kept
}

var unsafeName = regexp.MustCompile(`[^A-Za-z0-9_]+`)

// FileName is the dump's file name for the rule set at index (0-based) of a
// process's sequence, <NN>_<ruleset>.tsv: NN is index+1 on two digits, and the
// rule set is its file's base name without .jr, with any run of characters other
// than letters, digits and _ made one _. So jet_rules/main/CM/2_CM02_Matching_SM_Main1.jr
// at index 4 is 05_2_CM02_Matching_SM_Main1.tsv.
func FileName(index int, ruleset string) string {
	base := strings.TrimSuffix(filepath.Base(ruleset), ".jr")
	return fmt.Sprintf("%02d_%s.tsv", index+1, unsafeName.ReplaceAllString(base, "_"))
}

// Path is where a dump goes in a dump tree, <dir>/<engine>/<member>/<FileName>:
// the layout jets/cmds/rules_bench_diff reads.
func Path(dir, engine, member string, index int, ruleset string) string {
	return filepath.Join(dir, engine, member, FileName(index, ruleset))
}

func sortedUnique(lines []string) []string {
	sort.Strings(lines)
	out := lines[:0]
	for i, l := range lines {
		if i == 0 || l != lines[i-1] {
			out = append(out, l)
		}
	}
	return out
}
