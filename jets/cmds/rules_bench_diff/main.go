// Command rules_bench_diff compares two trees of canonical graph dumps
// (jets/rulesbench/graphdump), per member and rule set, and classifies each
// difference against a known-difference list. See README.md.
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

func main() {
	os.Exit(run(os.Args[1:], os.Stdout, os.Stderr))
}

func run(args []string, stdout, stderr io.Writer) int {
	fl := flag.NewFlagSet("rules_bench_diff", flag.ContinueOnError)
	fl.SetOutput(stderr)
	a := fl.String("a", "", "dump tree A: <dir>/<engine>, holding <member>/<NN>_<ruleset>.tsv (required)")
	b := fl.String("b", "", "dump tree B, as -a (required)")
	labelA := fl.String("a-label", "", "A's engine label; default: the base name of -a")
	labelB := fl.String("b-label", "", "B's engine label; default: the base name of -b")
	scope := fl.String("scope", "", "cross or self; default: self when the labels are equal, cross otherwise")
	knownPath := fl.String("known", "", "known-difference list; default: the one built in (known_differences.json)")
	jsonOut := fl.String("json", "", "write the summary as JSON to this file")
	textOut := fl.String("text", "", "write the summary as text to this file; default: standard output")
	all := fl.Bool("all", false, "list classified lines too, not only their counts")
	maxLines := fl.Int("max-lines", 20, "unclassified lines shown per file in the text summary; 0 for all")
	if err := fl.Parse(args); err != nil {
		return 2
	}
	if *a == "" || *b == "" || fl.NArg() > 0 {
		fmt.Fprintln(stderr, "usage: rules_bench_diff -a <dir>/<engine> -b <dir>/<engine> [-known list.json] [-json out.json] [-text out.txt]")
		return 2
	}
	sa := Side{Label: *labelA, Dir: *a}
	sb := Side{Label: *labelB, Dir: *b}
	if sa.Label == "" {
		sa.Label = filepath.Base(filepath.Clean(*a))
	}
	if sb.Label == "" {
		sb.Label = filepath.Base(filepath.Clean(*b))
	}
	sc := *scope
	switch {
	case sc == "" && sa.Label == sb.Label:
		sc = ScopeSelf
	case sc == "":
		sc = ScopeCross
	case sc != ScopeSelf && sc != ScopeCross:
		fmt.Fprintf(stderr, "rules_bench_diff: -scope is %s or %s\n", ScopeCross, ScopeSelf)
		return 2
	}
	known, err := LoadKnown(*knownPath)
	if err != nil {
		fmt.Fprintln(stderr, "rules_bench_diff:", err)
		return 2
	}
	sum, err := Compare(sa, sb, sc, known, *all)
	if err != nil {
		fmt.Fprintln(stderr, "rules_bench_diff:", err)
		return 2
	}
	sum.Known = *knownPath
	if sum.Known == "" {
		sum.Known = "(built in)"
	}
	if *jsonOut != "" {
		raw, err := json.MarshalIndent(sum, "", "  ")
		if err == nil {
			err = os.WriteFile(*jsonOut, append(raw, '\n'), 0o644)
		}
		if err != nil {
			fmt.Fprintln(stderr, "rules_bench_diff:", err)
			return 2
		}
	}
	w := stdout
	if *textOut != "" {
		f, err := os.Create(*textOut)
		if err != nil {
			fmt.Fprintln(stderr, "rules_bench_diff:", err)
			return 2
		}
		defer f.Close()
		w = f
	}
	writeText(w, sum, known, *maxLines)
	if sum.Totals.Unclassified > 0 || len(sum.OnlyInA) > 0 || len(sum.OnlyInB) > 0 {
		return 1
	}
	return 0
}

// writeText writes the summary for a reader: the totals, the known differences
// that occurred, then each file's unclassified lines by predicate.
func writeText(w io.Writer, s *Summary, known *KnownList, maxLines int) {
	fmt.Fprintf(w, "A: %s (%s)\nB: %s (%s)\nscope: %s; known differences: %s\n\n", s.A.Label, s.A.Dir, s.B.Label, s.B.Dir, s.Scope, s.Known)
	t := s.Totals
	fmt.Fprintf(w, "files compared %d: %d identical, %d differing\n", t.Files, t.Identical, t.Differing)
	fmt.Fprintf(w, "lines only in A %d, only in B %d: %d classified, %d unclassified\n", t.OnlyA, t.OnlyB, t.Classified, t.Unclassified)
	for _, f := range s.OnlyInA {
		fmt.Fprintf(w, "file only in A: %s\n", f)
	}
	for _, f := range s.OnlyInB {
		fmt.Fprintf(w, "file only in B: %s\n", f)
	}
	if len(s.ByKnown) > 0 {
		fmt.Fprintln(w, "\nknown differences:")
		ids := make([]string, 0, len(s.ByKnown))
		for id := range s.ByKnown {
			ids = append(ids, id)
		}
		sort.Strings(ids)
		for _, id := range ids {
			desc := ""
			for _, e := range known.Entries {
				if e.ID == id {
					desc = " — " + strings.Join(e.Refs, "; ")
				}
			}
			fmt.Fprintf(w, "  %6d  %s%s\n", s.ByKnown[id], id, desc)
		}
	}
	for _, f := range s.Files {
		if f.Unclassified == 0 {
			continue
		}
		fmt.Fprintf(w, "\n%s/%s: %d unclassified (only in A %d, only in B %d)\n", f.Member, f.File, f.Unclassified, f.OnlyA, f.OnlyB)
		shown := 0
		for _, p := range f.Predicates {
			var lines []string
			for _, l := range p.OnlyA {
				if l.Known == "" {
					lines = append(lines, "    A  "+l.Line)
				}
			}
			for _, l := range p.OnlyB {
				if l.Known == "" {
					lines = append(lines, "    B  "+l.Line)
				}
			}
			if len(lines) == 0 {
				continue
			}
			fmt.Fprintf(w, "  %s: %d\n", p.Predicate, len(lines))
			for _, l := range lines {
				if maxLines > 0 && shown >= maxLines {
					break
				}
				fmt.Fprintln(w, l)
				shown++
			}
		}
		if maxLines > 0 && shown >= maxLines && f.Unclassified > shown {
			fmt.Fprintf(w, "    … %d more\n", f.Unclassified-shown)
		}
	}
}
