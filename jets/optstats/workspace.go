package optstats

import (
	"fmt"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

// A workspace's statistics are in one or more files under jet_rules/:
//
//   - FileName, jet_rules/optimizer_statistics.json, written by rules_bench -stats
//     or by hand, and committed;
//   - one file per statistics process, jet_rules/<process>.optimizer_statistics.json,
//     each written only by that process's merge_optimizer_statistics report.
//
// **One file per process, so that no run reads another's file back.** Two rule
// processes, each with its own statistics process, then each own their file:
// a run replaces its own file and nothing else, two runs cannot lose each
// other's rule sets, and committing or reverting a file is the workspace's
// ordinary edit of one file. Added 2026-10-26, after a single shared file
// written by read, merge and write went stale on a commit or revert made
// without a compile.
//
// The compiler reads them all (ReadWorkspace). A rule set in a process's file
// takes precedence over the same rule set in FileName; a rule set in two
// processes' files is an error, because nothing says which is right.

// ProcessFileSuffix ends the name of a statistics process's file.
const ProcessFileSuffix = ".optimizer_statistics.json"

// processNameRe is what a process name may be to name a file.
var processNameRe = regexp.MustCompile(`^[A-Za-z0-9_-]+$`)

// ProcessFileName is the workspace file of the statistics process processName,
// jet_rules/<process>.optimizer_statistics.json. A process name that is not
// letters, digits, _ and - is refused rather than turned into a path.
func ProcessFileName(processName string) (string, error) {
	if !processNameRe.MatchString(processName) {
		return "", fmt.Errorf("process name %q cannot name a statistics file: use letters, digits, _ and -", processName)
	}
	return path.Join(path.Dir(FileName), processName+ProcessFileSuffix), nil
}

// IsFileName reports whether name, relative to the workspace root, is one of the
// workspace's statistics files: FileName, or a process's file beside it.
func IsFileName(name string) bool {
	name = filepath.ToSlash(name)
	if name == FileName {
		return true
	}
	dir, base := path.Split(name)
	return dir == path.Dir(FileName)+"/" && strings.HasSuffix(base, ProcessFileSuffix) &&
		processNameRe.MatchString(strings.TrimSuffix(base, ProcessFileSuffix))
}

// Located is one rule set's statistics and the file they were read from.
type Located struct {
	RuleSet *RuleSet
	File    string
}

// Workspace is what a workspace's statistics files hold, by rule set.
type Workspace struct {
	// Files are the statistics files read, relative to the workspace root, sorted.
	Files []string
	// RuleSets are the statistics of each rule set and the file they came from.
	RuleSets map[string]Located
	// Shadowed are the rule sets of FileName that a process's file replaces.
	Shadowed []string
}

// ReadWorkspace reads every statistics file of the workspace at root and
// combines them. It returns nil when the workspace has none. A file that does
// not read refuses the whole, naming the file, as does a rule set found in two
// processes' files.
func ReadWorkspace(root string) (*Workspace, error) {
	var files []string
	if _, err := os.Stat(filepath.Join(root, FileName)); err == nil {
		files = append(files, FileName)
	} else if !os.IsNotExist(err) {
		return nil, err
	}
	entries, err := os.ReadDir(filepath.Join(root, path.Dir(FileName)))
	if err != nil && !os.IsNotExist(err) {
		return nil, err
	}
	for _, e := range entries {
		name := path.Join(path.Dir(FileName), e.Name())
		if e.Type().IsRegular() && name != FileName && IsFileName(name) {
			files = append(files, name)
		}
	}
	if len(files) == 0 {
		return nil, nil
	}
	sort.Strings(files)
	w := &Workspace{Files: files, RuleSets: map[string]Located{}}
	var shared *File
	for _, name := range files {
		f, err := Read(filepath.Join(root, name))
		if err != nil {
			return nil, err
		}
		if name == FileName {
			shared = f
			continue
		}
		for rs, st := range f.RuleSets {
			if prev, ok := w.RuleSets[rs]; ok {
				return nil, fmt.Errorf("rule set %s has statistics in two files, %s and %s: remove it from one",
					rs, prev.File, name)
			}
			w.RuleSets[rs] = Located{RuleSet: st, File: name}
		}
	}
	if shared != nil {
		for rs, st := range shared.RuleSets {
			if _, ok := w.RuleSets[rs]; ok {
				w.Shadowed = append(w.Shadowed, rs)
				continue
			}
			w.RuleSets[rs] = Located{RuleSet: st, File: FileName}
		}
		sort.Strings(w.Shadowed)
	}
	return w, nil
}
