package utils

import (
	"os"
	"path/filepath"
)

// JetsBinDirEnvVar names the directory holding the JetStore binaries and
// support files that a running process executes or reads by fixed name:
// update_db, run_reports, local_test_driver and workspace_schema.sql.
const JetsBinDirEnvVar = "JETS_BIN_DIR"

// DefaultJetsBinDir is where the images install them, and where they are
// looked for when JETS_BIN_DIR is unset or empty.
const DefaultJetsBinDir = "/usr/local/bin"

// JetsBinDir returns JETS_BIN_DIR, or /usr/local/bin when it is unset or empty.
func JetsBinDir() string {
	if dir := os.Getenv(JetsBinDirEnvVar); dir != "" {
		return dir
	}
	return DefaultJetsBinDir
}

// JetsBinPath returns the path of the named JetStore binary or support file:
// name in JETS_BIN_DIR, or in /usr/local/bin when it is unset or empty. A
// workstation sets JETS_BIN_DIR to the directory of the binaries it built, so
// that the apiserver does not run whatever was installed in /usr/local/bin.
// Unset, the path is exactly what the images have always used
// ("/usr/local/bin/update_db").
func JetsBinPath(name string) string {
	return filepath.Join(JetsBinDir(), name)
}
