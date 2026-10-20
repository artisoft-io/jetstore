package rulesbench

import (
	"bufio"
	"crypto/sha256"
	"encoding/hex"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"runtime/debug"
	"sort"
	"strconv"
	"strings"
	"syscall"
	"time"
)

// Environment is what a result must carry for two results to be compared: the
// machine's state, the build of the library and of the command, and the
// workspace (rules_benchmark §5.5, R-10).
type Environment struct {
	Host       string    `json:"host"`
	Kernel     string    `json:"kernel"`
	RecordedAt time.Time `json:"recorded_at"`
	Args       []string  `json:"args"`

	GoVersion  string `json:"go_version"`
	GOMAXPROCS int    `json:"gomaxprocs"`
	NumCPU     int    `json:"num_cpu"`
	// CPUAffinity is the process's Cpus_allowed_list, which taskset sets.
	CPUAffinity string `json:"cpu_affinity"`
	CPUModel    string `json:"cpu_model"`
	// Governors is each distinct scaling_governor among the CPUs the process may
	// run on, with the CPUs that have it.
	Governors         map[string]string `json:"governors"`
	PerfEventParanoid string            `json:"perf_event_paranoid"`

	// LibJets is the libjets.so the loader mapped into this process, read from
	// /proc/self/maps: the library that ran, whatever the cgo flags named.
	LibJets *FileRecord `json:"libjets,omitempty"`
	// JetstoreRevision is the jetstore commit the command was built from, from
	// the binary's build information; Modified says the tree had changes.
	JetstoreRevision string `json:"jetstore_revision"`
	JetstoreModified bool   `json:"jetstore_modified"`
	BuildSettings    string `json:"build_settings,omitempty"`

	WorkspacesHome string `json:"workspaces_home"`
	Workspace      string `json:"workspace"`
	// WorkspaceRevision is git's HEAD in the workspace, when it is a checkout;
	// WorkspaceDirty says it has uncommitted changes.
	WorkspaceRevision string      `json:"workspace_revision,omitempty"`
	WorkspaceDirty    bool        `json:"workspace_dirty,omitempty"`
	WorkspaceDB       *FileRecord `json:"workspace_db,omitempty"`
}

// FileRecord identifies a file.
type FileRecord struct {
	Path   string    `json:"path"`
	SHA256 string    `json:"sha256"`
	Size   int64     `json:"size"`
	MTime  time.Time `json:"mtime"`
}

// RecordEnvironment reads the environment. It never fails: what cannot be read
// is left empty.
func RecordEnvironment() *Environment {
	env := &Environment{
		RecordedAt:     time.Now().UTC(),
		Args:           os.Args,
		GoVersion:      runtime.Version(),
		GOMAXPROCS:     runtime.GOMAXPROCS(0),
		NumCPU:         runtime.NumCPU(),
		WorkspacesHome: os.Getenv("WORKSPACES_HOME"),
		Workspace:      os.Getenv("WORKSPACE"),
	}
	env.Host, _ = os.Hostname()
	var u syscall.Utsname
	if syscall.Uname(&u) == nil {
		env.Kernel = utsString(u.Release[:])
	}
	env.CPUAffinity = procStatusField("Cpus_allowed_list")
	env.CPUModel = cpuModel()
	env.Governors = governors(env.CPUAffinity)
	env.PerfEventParanoid = readTrimmed("/proc/sys/kernel/perf_event_paranoid")
	if p := mappedLibrary("libjets.so"); p != "" {
		env.LibJets = recordFile(p)
	}
	if bi, ok := debug.ReadBuildInfo(); ok {
		var settings []string
		for _, s := range bi.Settings {
			switch s.Key {
			case "vcs.revision":
				env.JetstoreRevision = s.Value
			case "vcs.modified":
				env.JetstoreModified = s.Value == "true"
			case "CGO_CFLAGS", "CGO_LDFLAGS", "-tags", "-race", "GOAMD64":
				settings = append(settings, s.Key+"="+s.Value)
			}
		}
		env.BuildSettings = strings.Join(settings, " ")
	}
	if env.WorkspacesHome != "" && env.Workspace != "" {
		root := filepath.Join(env.WorkspacesHome, env.Workspace)
		if out, err := exec.Command("git", "-C", root, "rev-parse", "HEAD").Output(); err == nil {
			env.WorkspaceRevision = strings.TrimSpace(string(out))
			if out, err := exec.Command("git", "-C", root, "status", "--porcelain", "--untracked-files=no").Output(); err == nil {
				env.WorkspaceDirty = len(strings.TrimSpace(string(out))) > 0
			}
		}
		env.WorkspaceDB = recordFile(filepath.Join(root, "workspace.db"))
	}
	return env
}

func utsString(b []int8) string {
	var sb strings.Builder
	for _, c := range b {
		if c == 0 {
			break
		}
		sb.WriteByte(byte(c))
	}
	return sb.String()
}

func readTrimmed(path string) string {
	b, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(b))
}

func procStatusField(name string) string {
	f, err := os.Open("/proc/self/status")
	if err != nil {
		return ""
	}
	defer f.Close()
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		if k, v, ok := strings.Cut(sc.Text(), ":"); ok && k == name {
			return strings.TrimSpace(v)
		}
	}
	return ""
}

func cpuModel() string {
	f, err := os.Open("/proc/cpuinfo")
	if err != nil {
		return ""
	}
	defer f.Close()
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		if k, v, ok := strings.Cut(sc.Text(), ":"); ok && strings.TrimSpace(k) == "model name" {
			return strings.TrimSpace(v)
		}
	}
	return ""
}

// governors maps each scaling governor to the CPUs of list ("0-5,8") that have it.
func governors(list string) map[string]string {
	byGov := map[string][]int{}
	for _, cpu := range parseCPUList(list) {
		g := readTrimmed(filepath.Join("/sys/devices/system/cpu", "cpu"+strconv.Itoa(cpu), "cpufreq/scaling_governor"))
		if g == "" {
			g = "unknown"
		}
		byGov[g] = append(byGov[g], cpu)
	}
	out := make(map[string]string, len(byGov))
	for g, cpus := range byGov {
		sort.Ints(cpus)
		out[g] = formatCPUList(cpus)
	}
	return out
}

// GovernorsOK reports whether every CPU the process may run on uses the
// performance governor.
func (e *Environment) GovernorsOK() bool {
	return len(e.Governors) == 1 && e.Governors["performance"] != ""
}

func parseCPUList(list string) []int {
	var cpus []int
	for _, part := range strings.Split(list, ",") {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		lo, hi, isRange := strings.Cut(part, "-")
		a, err := strconv.Atoi(lo)
		if err != nil {
			continue
		}
		b := a
		if isRange {
			if b, err = strconv.Atoi(hi); err != nil {
				continue
			}
		}
		for c := a; c <= b; c++ {
			cpus = append(cpus, c)
		}
	}
	return cpus
}

func formatCPUList(cpus []int) string {
	var parts []string
	for i := 0; i < len(cpus); {
		j := i
		for j+1 < len(cpus) && cpus[j+1] == cpus[j]+1 {
			j++
		}
		if j == i {
			parts = append(parts, strconv.Itoa(cpus[i]))
		} else {
			parts = append(parts, strconv.Itoa(cpus[i])+"-"+strconv.Itoa(cpus[j]))
		}
		i = j + 1
	}
	return strings.Join(parts, ",")
}

// mappedLibrary is the path of the first mapping in /proc/self/maps whose file
// name is name.
func mappedLibrary(name string) string {
	f, err := os.Open("/proc/self/maps")
	if err != nil {
		return ""
	}
	defer f.Close()
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		fields := strings.Fields(sc.Text())
		if len(fields) >= 6 && filepath.Base(fields[5]) == name {
			return fields[5]
		}
	}
	return ""
}

func recordFile(path string) *FileRecord {
	f, err := os.Open(path)
	if err != nil {
		return nil
	}
	defer f.Close()
	st, err := f.Stat()
	if err != nil {
		return nil
	}
	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return nil
	}
	return &FileRecord{Path: path, SHA256: hex.EncodeToString(h.Sum(nil)), Size: st.Size(), MTime: st.ModTime().UTC()}
}
