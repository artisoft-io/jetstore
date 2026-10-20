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

// DumpWriter writes one canonical dump of an rdf session's graph. It is
// graphdump.WriteSession with its options bound (jets/rulesbench/graphdump).
type DumpWriter func(w io.Writer, rdfSession compute_pipes.JetRdfSession) error

// DumpPath is where the dump after a rule set goes:
// <dir>/<engine>/<session>/<NN>_<rule set>.tsv, with NN the rule set's
// position from 00 and the rule set's file name without its directory or .jr.
func DumpPath(dir, engine, session string, index int, ruleSet string) string {
	name := strings.TrimSuffix(filepath.Base(ruleSet), ".jr")
	return filepath.Join(dir, engine, safeName(session), fmt.Sprintf("%02d_%s.tsv", index, safeName(name)))
}

// safeName keeps a name to one path element.
func safeName(s string) string {
	return strings.Map(func(r rune) rune {
		if r == '/' || r == '\\' || r == 0 {
			return '_'
		}
		return r
	}, s)
}

// NewDumper is an Options.AfterRuleSet that writes each rule set's dump to
// DumpPath under dir.
func NewDumper(dir string, write DumpWriter) func(engine, session string, index int, ruleSet string, rdfSession compute_pipes.JetRdfSession) error {
	return func(engine, session string, index int, ruleSet string, rdfSession compute_pipes.JetRdfSession) error {
		path := DumpPath(dir, engine, session, index, ruleSet)
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			return err
		}
		f, err := os.Create(path)
		if err != nil {
			return err
		}
		w := bufio.NewWriter(f)
		if err := write(w, rdfSession); err != nil {
			f.Close()
			return fmt.Errorf("%s: %w", path, err)
		}
		if err := w.Flush(); err != nil {
			f.Close()
			return err
		}
		return f.Close()
	}
}
