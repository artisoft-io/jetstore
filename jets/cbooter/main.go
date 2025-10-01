package main

import (
	"errors"
	"flag"
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
// Commands:
// -ui : run the apiserver
// -reports : run the reports task
// -loader : run the loader task
// -server : run the server task with native rule engine
// -serverv2 : run the server task with go rule engine
// -cpipes : run the cpipes task
//
// The commands are mutually exclusive, only one can be specified at a time.
//
// Arguments specific to the commands:
// -ui: none
// -reports: client, processName, reportName, sessionId, filePath
// -loader: in_file, client, org, objectType, sourcePeriodKey, sessionId, userEmail
// -server: peKey, userEmail, shardId
// -serverv2: id, peKey
// -cpipes: pipeline_execution_key, session_id, file_key
//
// Example usage:
// To run the apiserver:
//
//	docker run --rm -e JETS_TEMP_DATA=/jetsdata -e WORKSPACES_REPO=/go/workspaces -e WORKSPACES_HOME=/jetsdata/workspaces_home myimage -ui
//
// To run the reports task:
//
//	docker run --rm -e JETS_TEMP_DATA=/jetsdata -e WORKSPACES_REPO=/go/workspaces -e WORKSPACES_HOME=/jetsdata/workspaces_home myimage -reports
var runApiServer = flag.Bool("ui", false, "Run apiserver")
var runReports = flag.Bool("reports", false, "Run reports")
var runLoader = flag.Bool("loader", false, "Run loader")
var runServer = flag.Bool("server", false, "Run server with native rule engine")
var runServerv2 = flag.Bool("serverv2", false, "Run server with go rule engine")
var runCpipes = flag.Bool("cpipes", false, "Run cpipes task")

// The target UID and GID to switch to is the jsuser as defined in the Dockerfile
// Ensure this matches the user created in the Dockerfile
var targetUID uint32 = 1000
var targetGID uint32 = 1000

func main() {
	flag.Parse()
	log.Println("cbooter starting...")
	// Validate that exactly one flag is set
	flagCount := 0
	if *runApiServer {
		flagCount++
	}
	if *runReports {
		flagCount++
	}
	if *runLoader {
		flagCount++
	}
	if *runServer {
		flagCount++
	}
	if *runServerv2 {
		flagCount++
	}
	if *runCpipes {
		flagCount++
	}
	if flagCount != 1 {
		log.Fatalf("Exactly one of -ui, -reports, -loader, -server, -serverv2, or -cpipes must be specified.")
	}
	// Validate that JETS_TEMP_DATA, WORKSPACES_REPO, and WORKSPACES_HOME are set when running -ui
	if *runApiServer {
		if os.Getenv("JETS_TEMP_DATA") == "" || os.Getenv("WORKSPACES_REPO") == "" || os.Getenv("WORKSPACES_HOME") == "" {
			log.Fatalf("JETS_TEMP_DATA, WORKSPACES_REPO, and WORKSPACES_HOME environment variables must be set when running -ui")
		}
	}

	// Validate that JETS_TEMP_DATA and WORKSPACES_HOME is set when running any command other than -ui
	if !*runApiServer {
		if os.Getenv("JETS_TEMP_DATA") == "" || os.Getenv("WORKSPACES_HOME") == "" {
			log.Fatalf("JETS_TEMP_DATA and WORKSPACES_HOME environment variables must be set when running -reports, -loader, -server, -serverv2, or -cpipes")
		}
	}

	// Give some time for the mounted volumes to be ready
	time.Sleep(2 * time.Second)

	// Ensure JETS_TEMP_DATA is writable by jsuser (uid 1000)
	// This is important because the mounted volume may have root ownership
	// and jsuser needs write access to it.
	// Determine which command to run based on flags
	switch {
	case *runApiServer:
		// Copy files at location WORKSPACES_REPO  to WORKSPACES_HOME recursively to be writable.
		// Copy files if directory WORKSPACES_HOME does not exists (which means it was already copied)
		if _, err := os.Stat(os.Getenv("WORKSPACES_HOME")); errors.Is(err, fs.ErrNotExist) {
			log.Println("Copying workspace files to WORKSPACES_HOME ...")
			err := runCommand("cp", []string{"-r", os.Getenv("WORKSPACES_REPO"), os.Getenv("WORKSPACES_HOME")}, &syscall.SysProcAttr{
				Credential: &syscall.Credential{
					Uid: 0, // root user
					Gid: 0,
				},
			})
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
		err := runDetachedCommand("apiserver", []string{}, &syscall.SysProcAttr{
			Credential: &syscall.Credential{
				Uid: targetUID,
				Gid: targetGID,
			},
		})
		if err != nil {
			log.Fatalf("Failed to start apiserver: %s", err)
		}

	default:
		// The remaining options all need to make the mounted JETS_TEMP_DATA writable
		err := makeJetsdataWritable()
		if err != nil {
			log.Fatalf("Failed to make JETS_TEMP_DATA writable: %s", err)
		}
		switch {
		case *runReports:
			log.Println("Starting run_reports...")
			err := runDetachedCommand("run_reports", []string{}, &syscall.SysProcAttr{
				Credential: &syscall.Credential{
					Uid: targetUID,
					Gid: targetGID,
				},
			})
			if err != nil {
				log.Fatalf("Failed to start run_reports: %s", err)
			}
		case *runLoader:
			log.Println("Starting loader...")
			err := runDetachedCommand("loader", []string{}, &syscall.SysProcAttr{
				Credential: &syscall.Credential{
					Uid: targetUID,
					Gid: targetGID,
				},
			})
			if err != nil {
				log.Fatalf("Failed to start loader: %s", err)
			}
		case *runServer:
			log.Println("Starting server...")
			err := runDetachedCommand("server", []string{}, &syscall.SysProcAttr{
				Credential: &syscall.Credential{
					Uid: targetUID,
					Gid: targetGID,
				},
			})
			if err != nil {
				log.Fatalf("Failed to start server: %s", err)
			}
		case *runServerv2:
			log.Println("Starting serverv2...")
			err := runDetachedCommand("serverv2", []string{}, &syscall.SysProcAttr{
				Credential: &syscall.Credential{
					Uid: targetUID,
					Gid: targetGID,
				},
			})
			if err != nil {
				log.Fatalf("Failed to start serverv2: %s", err)
			}
		case *runCpipes:
			log.Println("Starting cpipes...")
			err := runDetachedCommand("cpipes_server", []string{}, &syscall.SysProcAttr{
				Credential: &syscall.Credential{
					Uid: targetUID,
					Gid: targetGID,
				},
			})
			if err != nil {
				log.Fatalf("Failed to start cpipes_server: %s", err)
			}
		default:
			log.Println("No valid command specified. Use -ui, -reports, -loader, -server, -serverv2, or -cpipes")
			os.Exit(1)
		}
	}

	log.Println("Parent process exiting now.")
}

func makeJetsdataWritable() error {
	return runCommand("chmod", []string{"-R", "777", os.Getenv("JETS_TEMP_DATA")}, &syscall.SysProcAttr{
		Credential: &syscall.Credential{
			Uid: 0, // root user
			Gid: 0,
		},
	})
}

func runCommand(command string, args []string, sysProcAttr *syscall.SysProcAttr) error {
	cmd := exec.Command(command, args...)
	cmd.SysProcAttr = sysProcAttr
	// Run the command and capture output
	output, err := cmd.Output()
	log.Println(string(output))
	return err
}

// runDetachedCommand runs a command in a detached manner with specified SysProcAttr
// It returns an error if the command fails to start
func runDetachedCommand(command string, args []string, sysProcAttr *syscall.SysProcAttr) error {
	cmd := exec.Command(command, args...)

	// Configure the command for detachment.
	cmd.SysProcAttr = &syscall.SysProcAttr{
		// Set the user and group credentials.
		Credential: &syscall.Credential{
			Uid: targetUID,
			Gid: targetGID,
		},
		// Create a new session and process group for the child.
		Setsid: true,
	}

	// Important: Redirect stdout and stderr to prevent blocking.
	// A detached process should not inherit the parent's terminal.
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	// Start the command. `Start()` returns immediately.
	err := cmd.Start()
	if err != nil {
		return fmt.Errorf("failed to start command: %s", err)
	}

	log.Printf("Child process started with PID: %d\n", cmd.Process.Pid)

	// Release the process, allowing it to continue running independently.
	err = cmd.Process.Release()
	if err != nil {
		return fmt.Errorf("failed to release process: %s", err)
	}
	return nil
}
