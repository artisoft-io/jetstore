//go:build !linux

package datatable

import (
	"os/exec"
	"time"
)

// prepareHarnessCommand bounds the wait for the log pipe after the deadline.
// Elsewhere than Linux the harness's own -rlimit_data_mb is the only limit.
func prepareHarnessCommand(cmd *exec.Cmd) {
	cmd.WaitDelay = 5 * time.Second
}

func limitHarnessMemory(pid int, memoryMB int) {}
