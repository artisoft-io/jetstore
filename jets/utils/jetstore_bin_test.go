package utils

import "testing"

// Unset or empty, JETS_BIN_DIR leaves every path exactly as the images have
// always used it.
func TestJetsBinPath(t *testing.T) {
	t.Setenv(JetsBinDirEnvVar, "")
	for name, want := range map[string]string{
		"update_db":            "/usr/local/bin/update_db",
		"run_reports":          "/usr/local/bin/run_reports",
		"local_test_driver":    "/usr/local/bin/local_test_driver",
		"workspace_schema.sql": "/usr/local/bin/workspace_schema.sql",
	} {
		if got := JetsBinPath(name); got != want {
			t.Errorf("unset: JetsBinPath(%q) = %q, want %q", name, got, want)
		}
	}
	t.Setenv(JetsBinDirEnvVar, "/home/dev/jetstore-bin/")
	if got := JetsBinPath("update_db"); got != "/home/dev/jetstore-bin/update_db" {
		t.Errorf("set: JetsBinPath(update_db) = %q", got)
	}
}
