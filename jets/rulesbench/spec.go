// Package rulesbench is the benchmark of the two JetRules engines run by the
// rules_bench command (jets/cmds/rules_bench): it reads a bench spec, builds
// each engine as a pipeline does, runs every input session through the
// pipeline's own rule loop (compute_pipes.RunRuleSets) with its timing sink set,
// extracts the output through the pipeline's own extraction, and returns one
// result per run.
//
// It reuses the rules unit-test harness (jets/rulestest/harness) for the CSV
// reader, the jetrules_config conversion and the engine labels, and changes
// nothing in it. The canonical graph dump is jets/rulesbench/graphdump, a
// separate package that this one does not import: the command wires it in
// through Options.AfterRuleSet.
package rulesbench

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"github.com/artisoft-io/jetstore/jets/rulestest"
)

// Spec is a bench spec, a JSON file. jetrules_config and input_sessions are a
// .tst.json's, in exactly its shape (jets/cmds/rules_unit_test/README.md), so
// that a test definition's input can be benchmarked as written; the test's
// expectations are read and ignored. The other fields are the bench's own.
type Spec struct {
	// BenchName labels the results. A .tst.json's test_name is accepted in its
	// place, so a test definition is a valid spec once the bench fields are added.
	BenchName string `json:"bench_name,omitempty"`
	TestName  string `json:"test_name,omitempty"`
	Comment   string `json:"comment,omitempty"`

	JetrulesConfig rulestest.JetrulesConfig `json:"jetrules_config"`
	// InputSessions are a .tst.json's: each names its CSVs, relative to the
	// spec's directory or absolute. Their expected and excluded triples are ignored.
	InputSessions []rulestest.InputSession `json:"input_sessions,omitempty"`
	// InputDir is a directory holding one CSV per session, for runs of hundreds
	// of members. Its sessions follow InputSessions'.
	InputDir *InputDir `json:"input_dir,omitempty"`

	// Engines are run in this order, interleaved within each repetition:
	// "native" (C++), "go", or both. Empty is both, native first.
	Engines []string `json:"engines,omitempty"`
	// Repetitions is the number of measured runs per engine; 0 is 1.
	Repetitions int `json:"repetitions,omitempty"`
	// Warmup is the number of runs per engine before the measured ones. Their
	// results are written too, marked warmup.
	Warmup int `json:"warmup,omitempty"`
	// PoolSize is the number of workers running sessions concurrently on one
	// engine, as a pipeline's pool_size; 0 is 1.
	PoolSize int `json:"pool_size,omitempty"`
	// MaxVertexVisits is the cap on visits per vertex, set on both engines as
	// (jets:iState jets:max_vertex_visits n) in the meta graph. A cap is
	// required, because Go's default is 10,000 and C++ has none (rules_benchmark
	// F9): this, or -max_vertex_visits, or a jets:max_vertex_visits in
	// rule_config, as cm_loop's has. This replaces rule_config's, so the cap is
	// set once.
	MaxVertexVisits int `json:"max_vertex_visits,omitempty"`
	// MaxLooping, when set, replaces jetrules_config.max_looping, which itself
	// replaces every rule set's $max_looping when not 0.
	MaxLooping *int `json:"max_looping,omitempty"`
	// OutputClasses are extracted after the rules, as a pipeline's jetrules
	// output channels are. None means no extraction.
	OutputClasses []OutputClass `json:"output_classes,omitempty"`

	// A .tst.json's other fields, accepted and ignored. The engines always run
	// in production's retraction mode (rules_benchmark Q-4), so a legacy object
	// that turns a defect off is refused rather than ignored.
	Legacy          *rulestest.LegacyConfig `json:"legacy,omitempty"`
	CompareEngines  bool                    `json:"compare_engines,omitzero"`
	Expect          json.RawMessage         `json:"expect,omitempty"`
	ExpectedTriples json.RawMessage         `json:"expected_triples,omitempty"`
	ExcludedTriples json.RawMessage         `json:"excluded_triples,omitempty"`
}

// InputDir is a directory of session CSVs, one per session.
type InputDir struct {
	// Path is the directory, relative to the spec's directory or absolute.
	Path string `json:"path"`
	// Glob selects the files in it; empty is "*.csv".
	Glob string `json:"glob,omitempty"`
	// NameRegex is the session-name rule. Empty, a session is named after its
	// file without the extension. Otherwise a file whose name the regex does not
	// match is skipped, and the session is named by the regex's first group, or
	// by the whole match when it has none.
	NameRegex string `json:"name_regex,omitempty"`
	// Limit, when not 0, keeps the first Limit files in name order.
	Limit int `json:"limit,omitempty"`
}

// OutputClass is one class to extract, as a jetrules output channel names it.
type OutputClass struct {
	ClassName string `json:"class_name"`
	// Columns are the channel spec's own columns, appended to the class's
	// domain properties as a pipeline appends them.
	Columns []string `json:"columns,omitempty"`
	// DirectPropertiesOnly is the channel spec's direct_properties_only.
	DirectPropertiesOnly bool `json:"direct_properties_only,omitzero"`
}

// Session is one input session, resolved: its name and its CSVs' paths.
type Session struct {
	Name  string
	Files []string // absolute
}

// LoadSpec reads a bench spec. It is decoded strictly: an unknown key at any
// depth is an error, as a .tst.json's is.
func LoadSpec(path string) (*Spec, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("cannot read the bench spec: %w", err)
	}
	return ParseSpec(b)
}

// ParseSpec decodes and checks a bench spec.
func ParseSpec(b []byte) (*Spec, error) {
	dec := json.NewDecoder(bytes.NewReader(b))
	dec.DisallowUnknownFields()
	var spec Spec
	if err := dec.Decode(&spec); err != nil {
		return nil, fmt.Errorf("invalid bench spec: %w", err)
	}
	if dec.More() {
		return nil, errors.New("invalid bench spec: more than one JSON value")
	}
	if err := spec.check(); err != nil {
		return nil, err
	}
	return &spec, nil
}

// Name is the bench's label: bench_name, else test_name.
func (s *Spec) Name() string {
	if s.BenchName != "" {
		return s.BenchName
	}
	return s.TestName
}

func (s *Spec) check() error {
	switch {
	case s.Name() == "":
		return errors.New("invalid bench spec: bench_name (or test_name) is required")
	case s.JetrulesConfig.ProcessName == "":
		return errors.New("invalid bench spec: jetrules_config.process_name is required")
	case len(s.InputSessions) == 0 && s.InputDir == nil:
		return errors.New("invalid bench spec: input_sessions or input_dir is required")
	case s.Repetitions < 0, s.Warmup < 0, s.PoolSize < 0, s.MaxVertexVisits < 0:
		return errors.New("invalid bench spec: repetitions, warmup, pool_size and max_vertex_visits cannot be negative")
	case s.MaxLooping != nil && *s.MaxLooping < 0:
		return errors.New("invalid bench spec: max_looping cannot be negative")
	}
	if s.Legacy.ModeLabel() != rulestest.LegacyModeLegacy {
		return fmt.Errorf("invalid bench spec: legacy asks for %s; the benchmark runs the engines in production's retraction mode, legacy", s.Legacy.ModeLabel())
	}
	for _, e := range s.Engines {
		if e != rulestest.EngineNative && e != rulestest.EngineGo {
			return fmt.Errorf("invalid bench spec: unknown engine %q: %s or %s", e, rulestest.EngineNative, rulestest.EngineGo)
		}
	}
	for _, rc := range s.JetrulesConfig.RuleConfig {
		if _, ok := rc[rulestest.NoTruthMainOnExist]; ok {
			return fmt.Errorf("invalid bench spec: %s exists only in the C++ engine", rulestest.NoTruthMainOnExist)
		}
	}
	if s.InputDir != nil {
		if s.InputDir.Path == "" {
			return errors.New("invalid bench spec: input_dir.path is required")
		}
		if s.InputDir.NameRegex != "" {
			if _, err := regexp.Compile(s.InputDir.NameRegex); err != nil {
				return fmt.Errorf("invalid bench spec: input_dir.name_regex: %v", err)
			}
		}
	}
	for i, oc := range s.OutputClasses {
		if oc.ClassName == "" {
			return fmt.Errorf("invalid bench spec: output_classes[%d].class_name is required", i)
		}
	}
	return nil
}

// EnginesOrDefault is Engines, or both engines, native first.
func (s *Spec) EnginesOrDefault() []string {
	if len(s.Engines) > 0 {
		return s.Engines
	}
	return []string{rulestest.EngineNative, rulestest.EngineGo}
}

// Sessions resolves the input sessions, InputSessions first and then InputDir's
// files in name order, against dir, the spec's directory. Names must be unique:
// they name each session's results and dumps.
func (s *Spec) Sessions(dir string) ([]Session, error) {
	abs := func(p string) string {
		if filepath.IsAbs(p) {
			return p
		}
		return filepath.Join(dir, p)
	}
	var out []Session
	for _, is := range s.InputSessions {
		if len(is.Input) == 0 {
			return nil, fmt.Errorf("input session %q has no input", is.Name)
		}
		sess := Session{Name: is.Name}
		for _, f := range is.Input {
			sess.Files = append(sess.Files, abs(f))
		}
		out = append(out, sess)
	}
	if d := s.InputDir; d != nil {
		glob := d.Glob
		if glob == "" {
			glob = "*.csv"
		}
		root := abs(d.Path)
		files, err := filepath.Glob(filepath.Join(root, glob))
		if err != nil {
			return nil, fmt.Errorf("input_dir: %v", err)
		}
		sort.Strings(files)
		var re *regexp.Regexp
		if d.NameRegex != "" {
			re = regexp.MustCompile(d.NameRegex)
		}
		n := 0
		for _, f := range files {
			if d.Limit > 0 && n >= d.Limit {
				break
			}
			base := filepath.Base(f)
			name := strings.TrimSuffix(base, filepath.Ext(base))
			if re != nil {
				m := re.FindStringSubmatch(base)
				switch {
				case m == nil:
					continue
				case len(m) > 1:
					name = m[1]
				default:
					name = m[0]
				}
			}
			out = append(out, Session{Name: name, Files: []string{f}})
			n++
		}
		if n == 0 {
			return nil, fmt.Errorf("input_dir %s: no file matches %s", root, glob)
		}
	}
	seen := make(map[string]bool, len(out))
	for _, sess := range out {
		switch {
		case sess.Name == "":
			return nil, errors.New("an input session has no name")
		case seen[sess.Name]:
			return nil, fmt.Errorf("two input sessions are named %q", sess.Name)
		}
		seen[sess.Name] = true
	}
	return out, nil
}
