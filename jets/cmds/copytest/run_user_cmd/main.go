package main

import (
	"fmt"
	"log"
	"os/exec"
	"syscall"
)

func main() {
	// Replace with the desired UID and GID of the user
	// You can get these from /etc/passwd and /etc/group or using os/user package
	targetUID := uint32(999) // Example UID
	targetGID := uint32(999) // Example GID

	// Command to run (e.g., id or whoami to verify user)
	cmd := exec.Command("id")

	// Set SysProcAttr to specify user credentials
	cmd.SysProcAttr = &syscall.SysProcAttr{
		Credential: &syscall.Credential{
			Uid: targetUID,
			Gid: targetGID,
		},
	}

	// Run the command and capture output
	output, err := cmd.Output()
	if err != nil {
		log.Fatalf("Command failed: %v", err)
	}

	fmt.Printf("Command output: %s\n", string(output))
}
