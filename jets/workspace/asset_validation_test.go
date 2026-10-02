package workspace

import (
	"bytes"
	"errors"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// The validation step's contract, jetstore_maintenance_02 AG.4 (criterion 21):
// a valid workspace passes, an invalid one fails the compile, a missing
// interpreter or script fails it too, and only a named opt-out skips -- logging
// why.
//
// Two layers. The stub tests stand a shell script in for node, so the Go side's
// reading of exit statuses is tested on every machine with /bin/sh. The real
// tests run the bundled validator under the real node over the shipping assets,
// and skip -- saying how to build what they need -- when either is absent.

// stubValidator writes a script that stands in for `node <script> <dir>`: run as
// `sh <script> <dir>`, it exits with the number written in <dir>/exit, or 0.
func stubValidator(t *testing.T) (node, script string) {
	t.Helper()
	dir := t.TempDir()
	script = filepath.Join(dir, ValidatorScriptName)
	body := "#!/bin/sh\necho \"stub validator ran over $1\"\nif [ -f \"$1/exit\" ]; then exit $(cat \"$1/exit\"); fi\nexit 0\n"
	if err := os.WriteFile(script, []byte(body), 0755); err != nil {
		t.Fatal(err)
	}
	return "/bin/sh", script
}

func workspaceWithExit(t *testing.T, code string) string {
	t.Helper()
	dir := t.TempDir()
	if code != "" {
		if err := os.WriteFile(filepath.Join(dir, "exit"), []byte(code), 0644); err != nil {
			t.Fatal(err)
		}
	}
	return dir
}

func useStub(t *testing.T) {
	t.Helper()
	node, script := stubValidator(t)
	t.Setenv(NodeEnvVar, node)
	t.Setenv(ValidatorScriptEnvVar, script)
}

func TestAssetValidationPassesWhenTheValidatorDoes(t *testing.T) {
	useStub(t)
	dir := workspaceWithExit(t, "")
	out, err := validateWorkspaceAssets(dir, compileOptions{})
	if err != nil {
		t.Fatalf("valid workspace failed: %v\n%s", err, out)
	}
	if !strings.Contains(out, "stub validator ran over "+dir) {
		t.Errorf("the validator's output is not in the compile log:\n%s", out)
	}
}

func TestAssetValidationFailsOnAnInvalidDocument(t *testing.T) {
	useStub(t)
	out, err := validateWorkspaceAssets(workspaceWithExit(t, "1"), compileOptions{})
	if !errors.Is(err, ErrAssetValidation) {
		t.Fatalf("want ErrAssetValidation, got %v", err)
	}
	if !strings.Contains(err.Error(), "invalid document") {
		t.Errorf("exit 1 is the validator's verdict on the documents; got %v", err)
	}
	if !strings.Contains(out, "stub validator ran") {
		t.Errorf("the findings must reach the compile log:\n%s", out)
	}
}

func TestAssetValidationFailsWhenTheValidatorCannotSeeTheWorkspace(t *testing.T) {
	// Exit 2 is "could not validate", and must not read as "invalid documents"
	// -- nor, above all, as a pass.
	useStub(t)
	_, err := validateWorkspaceAssets(workspaceWithExit(t, "2"), compileOptions{})
	if !errors.Is(err, ErrAssetValidation) || !strings.Contains(err.Error(), "did not run to completion") {
		t.Fatalf("want a did-not-run failure, got %v", err)
	}
}

func TestAssetValidationFailsWhenNodeIsMissing(t *testing.T) {
	_, script := stubValidator(t)
	t.Setenv(ValidatorScriptEnvVar, script)

	t.Run("named and absent", func(t *testing.T) {
		t.Setenv(NodeEnvVar, filepath.Join(t.TempDir(), "node"))
		_, err := validateWorkspaceAssets(t.TempDir(), compileOptions{})
		if !errors.Is(err, ErrAssetValidation) {
			t.Fatalf("a missing interpreter must fail the compile, got %v", err)
		}
	})
	t.Run("not on PATH", func(t *testing.T) {
		t.Setenv(NodeEnvVar, "")
		t.Setenv("PATH", t.TempDir())
		_, err := validateWorkspaceAssets(t.TempDir(), compileOptions{})
		if !errors.Is(err, ErrAssetValidation) || !strings.Contains(err.Error(), "not on PATH") {
			t.Fatalf("a missing interpreter must fail the compile, got %v", err)
		}
	})
}

func TestAssetValidationFailsWhenTheScriptIsMissing(t *testing.T) {
	t.Setenv(NodeEnvVar, "/bin/sh")
	t.Run("named and absent", func(t *testing.T) {
		t.Setenv(ValidatorScriptEnvVar, filepath.Join(t.TempDir(), ValidatorScriptName))
		if _, err := validateWorkspaceAssets(t.TempDir(), compileOptions{}); !errors.Is(err, ErrAssetValidation) {
			t.Fatalf("want ErrAssetValidation, got %v", err)
		}
	})
	t.Run("not beside the executable", func(t *testing.T) {
		// The test binary lives in a temp directory with no script beside it.
		t.Setenv(ValidatorScriptEnvVar, "")
		_, err := validateWorkspaceAssets(t.TempDir(), compileOptions{})
		if !errors.Is(err, ErrAssetValidation) || !strings.Contains(err.Error(), "is not beside") {
			t.Fatalf("want a not-beside failure, got %v", err)
		}
	})
}

func TestAssetValidationOptOutSkipsAndSaysWhy(t *testing.T) {
	// Node pointed at nothing: if the opt-out ran anything, this would fail.
	t.Setenv(NodeEnvVar, filepath.Join(t.TempDir(), "node"))
	var logged bytes.Buffer
	log.SetOutput(&logged)
	t.Cleanup(func() { log.SetOutput(os.Stderr) })

	var opts compileOptions
	SkipAssetValidation("rebuilding lookup tables only")(&opts)
	out, err := validateWorkspaceAssets(t.TempDir(), opts)
	if err != nil {
		t.Fatalf("a named opt-out must skip, got %v", err)
	}
	for name, text := range map[string]string{"compile log": out, "process log": logged.String()} {
		if !strings.Contains(text, "skipped: rebuilding lookup tables only") {
			t.Errorf("the %s does not say the step was skipped and why:\n%s", name, text)
		}
	}
}

func TestAssetValidationOptOutWithNoReasonFails(t *testing.T) {
	var opts compileOptions
	SkipAssetValidation("  ")(&opts)
	if _, err := validateWorkspaceAssets(t.TempDir(), opts); !errors.Is(err, ErrAssetValidation) {
		t.Fatalf("an opt-out with no reason must not skip, got %v", err)
	}
}

// The step is in CompileWorkspace itself, before any rule is compiled -- which
// is what makes it every caller's gate rather than one caller's.
func TestCompileWorkspaceRunsTheGateFirst(t *testing.T) {
	home := t.TempDir()
	ws := filepath.Join(home, "gate_ws")
	if err := os.MkdirAll(ws, 0755); err != nil {
		t.Fatal(err)
	}
	control := filepath.Join(ws, "workspace_control.json")
	if err := os.WriteFile(control, []byte(`{"workspace_name":"gate_ws"}`), 0644); err != nil {
		t.Fatal(err)
	}
	oldHome, oldPrefix, oldControl := workspaceHome, wprefix, workspaceControlPath
	workspaceHome, wprefix, workspaceControlPath = home, "gate_ws", control
	t.Cleanup(func() { workspaceHome, wprefix, workspaceControlPath = oldHome, oldPrefix, oldControl })

	t.Run("invalid documents", func(t *testing.T) {
		useStub(t)
		if err := os.WriteFile(filepath.Join(ws, "exit"), []byte("1"), 0644); err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { os.Remove(filepath.Join(ws, "exit")) })
		_, err := CompileWorkspace(nil, "gate_ws", "1")
		if !errors.Is(err, ErrAssetValidation) {
			t.Fatalf("want the compile refused by validation, got %v", err)
		}
		if _, statErr := os.Stat(filepath.Join(ws, "build")); statErr == nil {
			t.Errorf("the compile started before the gate refused it: build/ was created")
		}
	})
	t.Run("missing interpreter", func(t *testing.T) {
		_, script := stubValidator(t)
		t.Setenv(ValidatorScriptEnvVar, script)
		t.Setenv(NodeEnvVar, filepath.Join(t.TempDir(), "node"))
		if _, err := CompileWorkspace(nil, "gate_ws", "1"); !errors.Is(err, ErrAssetValidation) {
			t.Fatalf("want the compile refused, got %v", err)
		}
	})
	t.Run("named opt-out reaches the compiler", func(t *testing.T) {
		t.Setenv(NodeEnvVar, filepath.Join(t.TempDir(), "node"))
		compileLog, err := CompileWorkspace(nil, "gate_ws", "1", SkipAssetValidation("test"))
		if errors.Is(err, ErrAssetValidation) {
			t.Fatalf("the opt-out did not skip: %v", err)
		}
		// The compile itself proceeds past the gate; whatever it makes of an
		// empty workspace is not this test's business. What is: it said why.
		if !strings.Contains(compileLog, "skipped: test") {
			t.Errorf("the compile log does not record the skip:\n%s", compileLog)
		}
	})
}

// The bundled validator under the real node, over the shipping assets and over
// a copy with one reference broken. This is the end-to-end form of the stub
// tests above, and the one that proves the Go step and the script agree on the
// contract.
func TestRealValidatorOverTheShippingAssets(t *testing.T) {
	node, err := exec.LookPath("node")
	if err != nil {
		t.Skip("node is not on PATH")
	}
	script, err := filepath.Abs("../../jetsclient_ide/dist-validator/" + ValidatorScriptName)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(script); err != nil {
		t.Skipf("%s is not built; run `npm ci && npm run build:validator` in jetsclient_ide", script)
	}
	t.Setenv(NodeEnvVar, node)
	t.Setenv(ValidatorScriptEnvVar, script)

	ws := t.TempDir()
	for _, dir := range []string{"user_flows", "table_configs"} {
		if err := os.CopyFS(filepath.Join(ws, dir), os.DirFS(filepath.Join("../workspace_assets", dir))); err != nil {
			t.Fatal(err)
		}
	}
	if out, err := validateWorkspaceAssets(ws, compileOptions{}); err != nil {
		t.Fatalf("the shipping assets failed validation: %v\n%s", err, out)
	}

	// Break one reference no schema can see: a table action naming an action
	// its flow does not define.
	path := filepath.Join(ws, "user_flows", "clientRegistryUF.ua.json")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	broken := strings.Replace(string(data), `"deleteClientAction"`, `"deleteClientActionRenamed"`, 1)
	if broken == string(data) {
		t.Fatal("the mutation did not apply; the fixture has moved")
	}
	if err := os.WriteFile(path, []byte(broken), 0644); err != nil {
		t.Fatal(err)
	}
	out, err := validateWorkspaceAssets(ws, compileOptions{})
	if !errors.Is(err, ErrAssetValidation) {
		t.Fatalf("a broken reference passed validation:\n%s", out)
	}
	// The file and the pointer's shape, not the button's index: D06 put *+ Add*
	// ahead of *Delete* on 2026-10-01 and moved it from /actions/0 to /actions/1,
	// which this test -- skipped wherever the bundle is unbuilt -- did not see.
	if !regexp.MustCompile(`table_configs/client\.tc\.json#/actions/\d+/actionName`).MatchString(out) {
		t.Errorf("the finding does not name the file and pointer:\n%s", out)
	}
}
