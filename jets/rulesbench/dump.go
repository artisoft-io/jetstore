package rulesbench

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/artisoft-io/jetstore/jets/compute_pipes"
)

// DumpPoint is where in a run Options.AfterRuleSet is called: after rule set
// Index (from 0) of a session, in one repetition of one engine.
type DumpPoint struct {
	Engine     string
	Session    string
	Index      int
	RuleSet    string
	Repetition int
	Warmup     bool
}

// RunDir names the point's repetition, r01 for the first measured run of each
// engine and w01 for the first warmup run: each repetition's dumps are a dump
// tree of their own, so no repetition overwrites another, and two of them
// compare as one engine against itself.
func (p DumpPoint) RunDir() string {
	kind := "r"
	if p.Warmup {
		kind = "w"
	}
	return fmt.Sprintf("%s%02d", kind, p.Repetition)
}

// DumpWriter writes one canonical dump of an rdf session's graph. It is
// graphdump.WriteSession with its options bound (jets/rulesbench/graphdump).
type DumpWriter func(w io.Writer, rdfSession compute_pipes.JetRdfSession) error

// DumpPathFunc is where a dump tree puts the dump of rule set index of member
// on engine: graphdump.Path, the layout jets/cmds/rules_bench_diff reads.
type DumpPathFunc func(dir, engine, member string, index int, ruleSet string) string

// NewDumper is an Options.AfterRuleSet that writes each rule set's dump with
// write, at path(<dir>/<RunDir>, engine, session, index, rule set). So
// <dir>/r01 is the dump tree of the first measured repetition, and each
// <dir>/<RunDir> is one the diff tool reads as it is.
func NewDumper(dir string, path DumpPathFunc, write DumpWriter) func(DumpPoint, compute_pipes.JetRdfSession) error {
	return func(p DumpPoint, rdfSession compute_pipes.JetRdfSession) error {
		file := path(filepath.Join(dir, p.RunDir()), p.Engine, safeName(p.Session), p.Index, p.RuleSet)
		if err := os.MkdirAll(filepath.Dir(file), 0o755); err != nil {
			return err
		}
		f, err := os.Create(file)
		if err != nil {
			return err
		}
		w := bufio.NewWriter(f)
		if err := write(w, rdfSession); err != nil {
			f.Close()
			return fmt.Errorf("%s: %w", file, err)
		}
		if err := w.Flush(); err != nil {
			f.Close()
			return err
		}
		return f.Close()
	}
}

// safeName keeps a session name to one path element.
func safeName(s string) string {
	return strings.Map(func(r rune) rune {
		if r == '/' || r == '\\' || r == 0 {
			return '_'
		}
		return r
	}, s)
}
