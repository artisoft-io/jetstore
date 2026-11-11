package user

// A process started without secrets on purpose -- the rules unit test harness,
// which the apiserver starts with every secret-bearing variable removed -- does
// not open its log with an error about the missing encryption key. init runs
// before any test, so each case runs this test binary as a child.

import (
	"os"
	"os/exec"
	"strings"
	"testing"
)

const initChildEnv = "JETS_USER_INIT_CHILD"

func TestInitChild(t *testing.T) {
	if os.Getenv(initChildEnv) == "" {
		t.Skip("run as a child by TestInitIsQuietWhenSecretsAreWithheld")
	}
}

func TestInitIsQuietWhenSecretsAreWithheld(t *testing.T) {
	run := func(extra ...string) string {
		t.Helper()
		cmd := exec.Command(os.Args[0], "-test.run=^TestInitChild$", "-test.count=1")
		env := []string{initChildEnv + "=1"}
		for _, kv := range os.Environ() {
			name, _, _ := strings.Cut(kv, "=")
			switch name {
			case "JETS_ENCRYPTION_KEY", "JETS_ENCRYPTION_KEY_SECRET", "API_SECRET", "AWS_API_SECRET",
				SecretsWithheldEnvVar:
				continue
			}
			env = append(env, kv)
		}
		cmd.Env = append(env, extra...)
		out, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("child: %v\n%s", err, out)
		}
		return string(out)
	}
	if out := run(); !strings.Contains(out, "user.init(): error: could not load value for JETS_ENCRYPTION_KEY") {
		t.Errorf("without the marker the missing key is reported, got:\n%s", out)
	}
	if out := run(SecretsWithheldEnvVar + "=1"); strings.Contains(out, "user.init()") {
		t.Errorf("with %s=1 init logged:\n%s", SecretsWithheldEnvVar, out)
	}
}
