package main

import (
	"errors"
	"fmt"
	"io/fs"
	"log"
	"os"
	"os/exec"
	"syscall"
	"time"
)

// Docker image booter to run commands as non-root user inside container
//
// Env Variables
// JETS_TEMP_DATA  - location of the JetStore mount point for temp data
// WORKSPACES_REPO - location of the workspace repo (read-only)
// WORKSPACES_HOME - location where the workspace repo is copied to (read-write)
//
// The first argument must be the command name, one of:
// apiserver, run_reports, loader, server, serverv2, cpipes_server.
// The other arguments are passed to the command being run.
//
// The commands are mutually exclusive, only one can be specified at a time.
//
// Example usage:
// To run the apiserver:
//
//	docker run --rm -e JETS_TEMP_DATA=/jetsdata -e WORKSPACES_REPO=/go/workspaces -e WORKSPACES_HOME=/jetsdata/workspaces_home myimage apiserver
//
// To run the reports task:
//
//	docker run --rm -e JETS_TEMP_DATA=/jetsdata -e WORKSPACES_REPO=/go/workspaces -e WORKSPACES_HOME=/jetsdata/workspaces_home myimage run_reports -client Acme -processName MyProcess -reportName MyReport -sessionId 123 -filePath /jetsdata/input/myinput.csv

// The target UID and GID to switch to is the jsuser as defined in the Dockerfile
// Ensure this matches the user created in the Dockerfile
var jsuserSysProcAttr *syscall.SysProcAttr = &syscall.SysProcAttr{
	Credential: &syscall.Credential{
		Uid: 1000,
		Gid: 1000,
	},
}
var rootSysProcAttr *syscall.SysProcAttr = &syscall.SysProcAttr{
	Credential: &syscall.Credential{
		Uid: 0,
		Gid: 0,
	},
}

func main() {
	log.Printf("cbooter starting with arguments %v...", os.Args[1:])

	// Separate cbooter args from command args
	// cbooter args are -ui, -reports, -loader, -server, -serverv2, -cpipes
	// Everything else is considered a cmd arg
	cmd := os.Args[1]
	cmdArgs := os.Args[2:]

	// Validate that JETS_TEMP_DATA, WORKSPACES_REPO, and WORKSPACES_HOME are set when running apiserver
	if cmd == "apiserver" {
		if os.Getenv("JETS_TEMP_DATA") == "" || os.Getenv("WORKSPACES_REPO") == "" || os.Getenv("WORKSPACES_HOME") == "" {
			log.Fatalf("JETS_TEMP_DATA, WORKSPACES_REPO, and WORKSPACES_HOME environment variables must be set when running apiserver")
		}
	}

	// Validate that JETS_TEMP_DATA and WORKSPACES_HOME is set when running any command other than apiserver
	if cmd != "apiserver" /* for testing && cmd != "ls" */ {
		if os.Getenv("JETS_TEMP_DATA") == "" || os.Getenv("WORKSPACES_HOME") == "" {
			log.Fatalf("JETS_TEMP_DATA and WORKSPACES_HOME environment variables must be set when running run_reports, loader, server, serverv2, or cpipes_server")
		}
	}

	// Give some time for the mounted volumes to be ready
	time.Sleep(2 * time.Second)

	// Create the tmp directory inside JETS_TEMP_DATA if it does not exist
	tmpDir := os.Getenv("TMPDIR")
	if tmpDir == "" {
		log.Fatalf("TMPDIR environment variable must be set in Dockerfile as a subdirectory of JETS_TEMP_DATA")
	}
	_, err := os.Stat(tmpDir)
	if errors.Is(err, fs.ErrNotExist) {
		err := os.MkdirAll(tmpDir, 0775)
		if err != nil {
			log.Fatalf("Failed to create tmp directory %s: %s", tmpDir, err)
		}
	}

	// Ensure JETS_TEMP_DATA is writable by jsuser (uid 1000)
	// This is important because the mounted volume may have root ownership
	// and jsuser needs write access to it.
	// Determine which command to run based on flags
	switch cmd {
	case "apiserver":
		// Copy files at location WORKSPACES_REPO  to WORKSPACES_HOME recursively to be writable.
		// Copy files if directory WORKSPACES_HOME does not exists (which means it was already copied)
		if _, err := os.Stat(os.Getenv("WORKSPACES_HOME")); errors.Is(err, fs.ErrNotExist) {
			log.Println("Copying workspace files to WORKSPACES_HOME ...")
			err := runCommandAsRoot("cp", []string{"-r", os.Getenv("WORKSPACES_REPO"), os.Getenv("WORKSPACES_HOME")})
			if err != nil {
				log.Fatalf("Failed to copy workspace files: %s", err)
			}
			// Make sure the copied files are writable by jsuser
			err = makeJetsdataWritable()
			if err != nil {
				log.Fatalf("Failed to make JETS_TEMP_DATA writable: %s", err)
			}
		} else {
			log.Println("Workspace files already exist in WORKSPACES_HOME, skipping workspace setup.")
		}
		log.Println("Starting apiserver...")
		err := runCommandAsJsuser("apiserver", cmdArgs)
		if err != nil {
			log.Fatalf("Failed to start apiserver: %s", err)
		}

	default:
		// All the remaining commands need to make the mounted JETS_TEMP_DATA writable
		err := makeJetsdataWritable()
		if err != nil {
			log.Fatalf("Failed to make JETS_TEMP_DATA writable: %s", err)
		}
		log.Printf("Starting %s...", cmd)
		err = runCommandAsJsuser(cmd, cmdArgs)
		if err != nil {
			log.Fatalf("Failed to start %s: %s", cmd, err)
		}
	}

	log.Println("Parent process exiting now.")
}

func makeJetsdataWritable() error {
	return runCommandAsRoot("chown", []string{"-hR", "1000:1000", os.Getenv("JETS_TEMP_DATA")})
}

func runCommandAsRoot(command string, args []string) error {
	cmd := exec.Command(command, args...)
	cmd.SysProcAttr = rootSysProcAttr
	// Run the command and capture output
	output, err := cmd.Output()
	log.Println(string(output))
	return err
}

// runCommandAsJsuser runs a command with specified user
// It returns an error if the command fails to start
func runCommandAsJsuser(command string, args []string) error {
	cmd := exec.Command(command, args...)
	cmd.SysProcAttr = jsuserSysProcAttr

	// Important: Redirect stdout and stderr
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	err := cmd.Run()
	if err != nil {
		return fmt.Errorf("failed to start command: %s", err)
	}
	// This point means the command has exited
	log.Printf("Command %s exited", command)
	return nil
}
