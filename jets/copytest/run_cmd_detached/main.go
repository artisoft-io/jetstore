package main

import (
	"fmt"
	"log"
	"os"
	"os/exec"
	"os/user"
	"strconv"
	"syscall"
	"time"
)

func main() {
	// Your program must be run as root for this to work.
	// Use `sudo go run yourprogram.go` to execute.

	targetUser := "michel"
	commandToRun := "sleep"
	commandArgs := []string{"60"} // Run sleep for 60 seconds

	// Look up the user by name.
	u, err := user.Lookup(targetUser)
	if err != nil {
		log.Fatalf("failed to find user: %s", err)
	}

	// Convert the UID and GID to uint32.
	uid, err := strconv.Atoi(u.Uid)
	if err != nil {
		log.Fatalf("failed to parse UID: %s", err)
	}
	gid, err := strconv.Atoi(u.Gid)
	if err != nil {
		log.Fatalf("failed to parse GID: %s", err)
	}

	// Create the command.
	cmd := exec.Command(commandToRun, commandArgs...)

	// Configure the command for detachment.
	cmd.SysProcAttr = &syscall.SysProcAttr{
		// Set the user and group credentials.
		Credential: &syscall.Credential{
			Uid: uint32(uid),
			Gid: uint32(gid),
		},
		// Create a new session and process group for the child.
		Setsid: true,
	}

	// Important: Redirect stdout and stderr to prevent blocking.
	// A detached process should not inherit the parent's terminal.
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	// Start the command. `Start()` returns immediately.
	err = cmd.Start()
	if err != nil {
		log.Fatalf("failed to start command: %s", err)
	}

	fmt.Printf("Child process started with PID: %d\n", cmd.Process.Pid)

	// Release the process, allowing it to continue running independently.
	err = cmd.Process.Release()
	if err != nil {
		log.Fatalf("failed to release process: %s", err)
	}

	fmt.Println("Parent process exiting now.")

	// For demonstration, allow the parent to sleep briefly before exiting.
	time.Sleep(2 * time.Second)
	// How to verify detachment
	// Run the Go program using sudo.
	// Immediately after the "Parent process exiting now" message, run ps -u michel | grep sleep in your terminal.
	// You will see that the sleep process is still running, even though the Go program has exited.
}
