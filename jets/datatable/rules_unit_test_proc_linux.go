//go:build linux

package datatable

import (
	"log"
	"os/exec"
	"syscall"
	"time"

	"golang.org/x/sys/unix"
)

// prepareHarnessCommand puts the harness in a process group of its own and makes
// the deadline kill the group, so a child the harness started cannot hold the
// log pipe open after the harness is gone; WaitDelay bounds the wait for the
// pipe in any case.
func prepareHarnessCommand(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	cmd.Cancel = func() error {
		return syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL)
	}
	cmd.WaitDelay = 5 * time.Second
}

// limitHarnessMemory sets RLIMIT_DATA on the started harness, the backstop to
// the -rlimit_data_mb the harness applies to itself first thing. It is a
// backstop and not the mechanism: whatever the child allocates before this call
// is not limited, and Go's os/exec has no way to set a limit before exec.
func limitHarnessMemory(pid int, memoryMB int) {
	limit := uint64(memoryMB) << 20
	if err := unix.Prlimit(pid, unix.RLIMIT_DATA, &unix.Rlimit{Cur: limit, Max: limit}, nil); err != nil {
		log.Printf("Warning: cannot set RLIMIT_DATA on harness process %d: %v", pid, err)
	}
}
