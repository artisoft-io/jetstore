package main

import (
	"bufio"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/artisoft-io/jetstore/jets/rulesbench/graphdump"
)

// Side is one dump tree, <dir>/<member>/<NN>_<ruleset>.tsv, where dir is
// <tree>/<engine>.
type Side struct {
	Label string `json:"label"`
	Dir   string `json:"dir"`
}

// Summary is a comparison of two dump trees.
type Summary struct {
	A     Side   `json:"a"`
	B     Side   `json:"b"`
	Scope string `json:"scope"`
	// Known is the known-difference list used.
	Known  string `json:"known"`
	Totals Totals `json:"totals"`
	// ByKnown counts the lines each known difference classified.
	ByKnown map[string]int `json:"by_known"`
	// OnlyInA and OnlyInB are dump files one tree has and the other lacks.
	OnlyInA []string     `json:"files_only_in_a"`
	OnlyInB []string     `json:"files_only_in_b"`
	Files   []FileResult `json:"files"`
}

// Totals are over every file both trees have.
type Totals struct {
	Files        int `json:"files"`
	Identical    int `json:"identical"`
	Differing    int `json:"differing"`
	OnlyA        int `json:"lines_only_in_a"`
	OnlyB        int `json:"lines_only_in_b"`
	Classified   int `json:"classified"`
	Unclassified int `json:"unclassified"`
}

// FileResult is one member's dump after one rule set. Only files that differ
// are listed.
type FileResult struct {
	Member       string            `json:"member"`
	File         string            `json:"file"`
	LinesA       int               `json:"lines_a"`
	LinesB       int               `json:"lines_b"`
	OnlyA        int               `json:"only_in_a"`
	OnlyB        int               `json:"only_in_b"`
	Unclassified int               `json:"unclassified"`
	Predicates   []PredicateResult `json:"predicates"`
}

// PredicateResult is a file's differences on one predicate.
type PredicateResult struct {
	Predicate string `json:"predicate"`
	// Known counts the classified lines by entry, both sides together.
	Known map[string]int `json:"known,omitempty"`
	// OnlyA and OnlyB are the unclassified lines, and with -all the classified
	// ones too.
	OnlyA []LineResult `json:"only_in_a,omitempty"`
	OnlyB []LineResult `json:"only_in_b,omitempty"`
}

// LineResult is one line on one side only.
type LineResult struct {
	Line  string `json:"line"`
	Known string `json:"known,omitempty"`
}

type line struct {
	text string
	// rewritten is text in B's labels, for a line of A (classify).
	rewritten string
	s, p, o   string
	object    graphdump.Term
	known     string
}

// mapped is the line as compared: in B's labels for a line of A.
func (l *line) mapped() string {
	if l.rewritten != "" {
		return l.rewritten
	}
	return l.text
}

// setFields makes text the line as compared.
func (l *line) setFields(text string) {
	l.rewritten = text
	if parts := strings.Split(text, "\t"); len(parts) == 3 {
		l.s, l.p, l.o = parts[0], parts[1], parts[2]
		l.object, _ = graphdump.ParseTerm(l.o)
	}
}

// Compare compares two dump trees.
func Compare(a, b Side, scope string, known *KnownList, all bool) (*Summary, error) {
	filesA, err := dumpFiles(a.Dir)
	if err != nil {
		return nil, err
	}
	filesB, err := dumpFiles(b.Dir)
	if err != nil {
		return nil, err
	}
	sum := &Summary{A: a, B: b, Scope: scope, ByKnown: map[string]int{}, OnlyInA: []string{}, OnlyInB: []string{}, Files: []FileResult{}}
	inB := map[string]bool{}
	for _, f := range filesB {
		inB[f] = true
	}
	inA := map[string]bool{}
	for _, f := range filesA {
		inA[f] = true
		if !inB[f] {
			sum.OnlyInA = append(sum.OnlyInA, f)
		}
	}
	for _, f := range filesB {
		if !inA[f] {
			sum.OnlyInB = append(sum.OnlyInB, f)
		}
	}
	for _, f := range filesA {
		if !inB[f] {
			continue
		}
		la, err := readDump(filepath.Join(a.Dir, f))
		if err != nil {
			return nil, err
		}
		lb, err := readDump(filepath.Join(b.Dir, f))
		if err != nil {
			return nil, err
		}
		sum.Totals.Files++
		fr := compareFile(f, la, lb, a.Label, b.Label, scope, known, all, sum.ByKnown)
		if fr == nil {
			sum.Totals.Identical++
			continue
		}
		sum.Totals.Differing++
		sum.Totals.OnlyA += fr.OnlyA
		sum.Totals.OnlyB += fr.OnlyB
		sum.Totals.Unclassified += fr.Unclassified
		sum.Totals.Classified += fr.OnlyA + fr.OnlyB - fr.Unclassified
		sum.Files = append(sum.Files, *fr)
	}
	return sum, nil
}

// dumpFiles lists the .tsv files under dir as member/file, sorted.
func dumpFiles(dir string) ([]string, error) {
	var files []string
	err := filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() || !strings.HasSuffix(path, ".tsv") {
			return nil
		}
		rel, err := filepath.Rel(dir, path)
		if err != nil {
			return err
		}
		files = append(files, filepath.ToSlash(rel))
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("reading the dump tree %s: %w", dir, err)
	}
	sort.Strings(files)
	return files, nil
}

// readDump reads a dump's lines, without blank lines and # comments.
func readDump(path string) ([]string, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	var lines []string
	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 1<<20), 1<<26)
	for sc.Scan() {
		if l := sc.Text(); l != "" && !strings.HasPrefix(l, "#") {
			lines = append(lines, l)
		}
	}
	return lines, sc.Err()
}

// compareFile compares one file of each tree; nil when they are identical.
func compareFile(f string, a, b []string, labelA, labelB, scope string, known *KnownList, all bool, byKnown map[string]int) *FileResult {
	onlyA, onlyB := minus(a, b), minus(b, a)
	if len(onlyA) == 0 && len(onlyB) == 0 {
		return nil
	}
	member, file := filepath.Split(f)
	fr := &FileResult{Member: strings.TrimSuffix(member, "/"), File: file, LinesA: len(a), LinesB: len(b), OnlyA: len(onlyA), OnlyB: len(onlyB)}
	la, lb := parseLines(onlyA), parseLines(onlyB)
	classify(la, lb, a, b, fr.Member, fr.File, labelA, labelB, scope, known)

	byPredicate := map[string]*PredicateResult{}
	result := func(p string) *PredicateResult {
		pr := byPredicate[p]
		if pr == nil {
			pr = &PredicateResult{Predicate: p}
			byPredicate[p] = pr
		}
		return pr
	}
	add := func(ls []*line, onA bool) {
		for _, l := range ls {
			pr := result(l.p)
			if l.known != "" {
				if pr.Known == nil {
					pr.Known = map[string]int{}
				}
				pr.Known[l.known]++
				byKnown[l.known]++
				if !all {
					continue
				}
			} else {
				fr.Unclassified++
			}
			lr := LineResult{Line: l.text, Known: l.known}
			if onA {
				pr.OnlyA = append(pr.OnlyA, lr)
			} else {
				pr.OnlyB = append(pr.OnlyB, lr)
			}
		}
	}
	add(la, true)
	add(lb, false)
	keys := make([]string, 0, len(byPredicate))
	for p := range byPredicate {
		keys = append(keys, p)
	}
	sort.Strings(keys)
	for _, p := range keys {
		fr.Predicates = append(fr.Predicates, *byPredicate[p])
	}
	return fr
}

func minus(a, b []string) []string {
	in := make(map[string]bool, len(b))
	for _, l := range b {
		in[l] = true
	}
	var out []string
	for _, l := range a {
		if !in[l] {
			out = append(out, l)
		}
	}
	return out
}

func parseLines(texts []string) []*line {
	out := make([]*line, len(texts))
	for i, t := range texts {
		l := &line{text: t}
		l.setFields(t)
		l.rewritten = ""
		if l.s == "" {
			l.s = t
		}
		out[i] = l
	}
	return out
}
