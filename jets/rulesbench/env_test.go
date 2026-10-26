package rulesbench

import (
	"bytes"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"testing"
)

func TestPreloadMappings(t *testing.T) {
	dir := t.TempDir()
	lib := filepath.Join(dir, "liballoc.so.2")
	if err := os.WriteFile(lib, nil, 0o644); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(dir, "liballoc.so")
	if err := os.Symlink(lib, link); err != nil {
		t.Fatal(err)
	}
	// The temporary directory may itself sit behind a symbolic link.
	real, err := filepath.EvalSymlinks(lib)
	if err != nil {
		t.Fatal(err)
	}
	mapped := []string{"/usr/lib/x86_64-linux-gnu/libc.so.6", real, "/opt/lib/libother.so.1"}
	got := preloadMappings(link+":libother.so.1 "+filepath.Join(dir, "absent.so"), mapped)
	want := [][2]string{
		{link, real},
		{"libother.so.1", "/opt/lib/libother.so.1"},
		{filepath.Join(dir, "absent.so"), ""},
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("preloadMappings: got %v, want %v", got, want)
	}
	if got := preloadMappings("", mapped); len(got) != 0 {
		t.Errorf("an empty LD_PRELOAD gave %v", got)
	}
}

// TestRecordEnvironmentPreload runs this test binary again with a library
// preloaded, and reads what the child records: the library as the loader
// mapped it.
func TestRecordEnvironmentPreload(t *testing.T) {
	if os.Getenv("RULESBENCH_PRELOAD_CHILD") == "1" {
		env := RecordEnvironment()
		json.NewEncoder(os.Stdout).Encode(struct {
			LDPreload string
			Preloaded []PreloadRecord
			NotMapped []string
		}{env.LDPreload, env.Preloaded, env.PreloadNotMapped()})
		return
	}
	lib := ""
	for _, p := range []string{"/usr/lib/x86_64-linux-gnu/libz.so.1", "/lib64/libz.so.1", "/usr/lib/libz.so.1"} {
		if _, err := os.Stat(p); err == nil {
			lib = p
			break
		}
	}
	if lib == "" {
		t.Skip("no libz.so.1 to preload")
	}
	run := func(preload string) (out struct {
		LDPreload string
		Preloaded []PreloadRecord
		NotMapped []string
	}) {
		cmd := exec.Command(os.Args[0], "-test.run=^TestRecordEnvironmentPreload$")
		cmd.Env = append(os.Environ(), "RULESBENCH_PRELOAD_CHILD=1", "LD_PRELOAD="+preload)
		b, err := cmd.Output()
		if err != nil {
			t.Fatalf("child with LD_PRELOAD=%s: %v", preload, err)
		}
		// The test framework prints PASS after the record.
		if err := json.NewDecoder(bytes.NewReader(b)).Decode(&out); err != nil {
			t.Fatalf("child output %q: %v", b, err)
		}
		return out
	}
	real, _ := filepath.EvalSymlinks(lib)
	got := run(lib)
	if got.LDPreload != lib || len(got.Preloaded) != 1 || got.Preloaded[0].Mapped == nil ||
		got.Preloaded[0].Mapped.Path != real || got.Preloaded[0].Mapped.SHA256 == "" || len(got.NotMapped) != 0 {
		t.Errorf("preloading %s: recorded %+v", lib, got)
	}
	// The loader ignores a library it cannot find, and the record says so.
	missing := filepath.Join(t.TempDir(), "absent.so")
	got = run(missing)
	if len(got.Preloaded) != 1 || got.Preloaded[0].Mapped != nil || !reflect.DeepEqual(got.NotMapped, []string{missing}) {
		t.Errorf("preloading a missing library: recorded %+v", got)
	}
}
