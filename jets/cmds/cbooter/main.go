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

	"github.com/artisoft-io/jetstore/jets/utils"
)

// Docker image booter to run commands as non-root user inside container
//
// Env Variables
// JETS_TEMP_DATA  - location of the JetStore mount point for temp data
// WORKSPACES_REPO - location of the workspace repo (read-only)
// WORKSPACES_HOME - location where the workspace repo is copied to (read-write)
//
// The first argument must be the command name, one of:
// apiserver, infer_server, run_reports, loader, server, serverv2, cpipes_server, cpipes_native_server.
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
		Uid: 999,
		Gid: 999,
	},
}
var rootSysProcAttr *syscall.SysProcAttr = &syscall.SysProcAttr{
	Credential: &syscall.Credential{
		Uid: 0,
		Gid: 0,
	},
}

// allowedCommands is the set of commands cbooter is permitted to run.
// The user-supplied command must match one of these exactly to prevent
// arbitrary command / argument injection via os.Args.
var allowedCommands = map[string]bool{
	"apiserver":            true,
	"infer_server":         true,
	"run_reports":          true,
	"cpipes_server":        true,
	"cpipes_native_server": true,
}

func main() {
	utils.UseJetStoreLogger()
	log.Printf("cbooter starting with arguments %v...", os.Args[1:])

	// A command name is required as the first argument.
	if len(os.Args) < 2 {
		log.Fatalf("a command name must be provided as the first argument; allowed commands: apiserver, infer_server, run_reports, cpipes_server, cpipes_native_server")
	}

	// Separate cbooter args from command args
	// cbooter args are -ui, -reports, -loader, -server, -serverv2, -cpipes
	// Everything else is considered a cmd arg
	cmd := os.Args[1]
	cmdArgs := os.Args[2:]

	// Validate the command against the allowlist to prevent command injection.
	// Only known, trusted command names may be executed.
	if !allowedCommands[cmd] {
		log.Fatalf("invalid command %q; allowed commands: apiserver, infer_server, run_reports, cpipes_server, cpipes_native_server", cmd)
	}

	// JETS_TEMP_DATA is the mount point every command writes to.
	if os.Getenv("JETS_TEMP_DATA") == "" {
		log.Fatalf("JETS_TEMP_DATA environment variable must be set")
	}
	// The workspace variables are only meaningful to the commands that stage a workspace.
	// infer_server serves models off the mounted volume and never touches one.
	if cmd != "infer_server" {
		if os.Getenv("WORKSPACES_REPO") == "" || os.Getenv("WORKSPACES_HOME") == "" {
			log.Fatalf("WORKSPACES_REPO and WORKSPACES_HOME environment variables must be set for %s", cmd)
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

	// Ensure JETS_TEMP_DATA is writable by jsuser (uid 999)
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

	case "infer_server":
		log.Println("Starting infer_server...")
		// Ollama is installed in the image (see dockerfiles/Dockerfile.infer_service) and runs
		// as a direct child process, not as a nested `docker run`: this container has no access
		// to a Docker socket, and with awsvpc networking a nested container would publish its
		// port on the host namespace rather than on the task ENI the load balancer targets.
		// The GPU is supplied by ECS through the task definition's GpuCount, and the OLLAMA_*
		// tuning variables are inherited from the container environment.
		//
		// OLLAMA_MODELS must live under JETS_TEMP_DATA so the weights land on the persistent
		// volume; jsuser has no writable home for Ollama's default of $HOME/.ollama.
		modelsDir := os.Getenv("OLLAMA_MODELS")
		if modelsDir == "" {
			log.Fatalf("OLLAMA_MODELS environment variable must be set for infer_server")
		}
		if err := os.MkdirAll(modelsDir, 0775); err != nil {
			log.Fatalf("Failed to create the Ollama models directory %s: %s", modelsDir, err)
		}
		// Switching uid does not change HOME, so the child would otherwise inherit root's
		// and fail to write its signing key and caches under /root/.ollama. HOME must point
		// somewhere jsuser owns — see the Dockerfile, which puts it under JETS_TEMP_DATA.
		homeDir := os.Getenv("HOME")
		if homeDir == "" {
			log.Fatalf("HOME environment variable must be set for infer_server")
		}
		if err := os.MkdirAll(homeDir, 0775); err != nil {
			log.Fatalf("Failed to create the home directory %s: %s", homeDir, err)
		}
		// The mounted volume comes up owned by root and the task's root filesystem is
		// read-only, so this has to happen here, while we are still root.
		if err := makeJetsdataWritable(); err != nil {
			log.Fatalf("Failed to make JETS_TEMP_DATA writable: %s", err)
		}
		if err := runCommandAsJsuser("ollama", []string{"serve"}); err != nil {
			log.Fatalf("Failed to start infer_server: %s", err)
		}

	default:
		// Copy the workspace repo to workspace home make the mounted JETS_TEMP_DATA writable
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

		log.Printf("Starting %s...", cmd)
		err = runCommandAsJsuser(cmd, cmdArgs)
		if err != nil {
			log.Fatalf("Failed to start %s: %s", cmd, err)
		}
	}

	log.Println("Parent process exiting now.")
}

func makeJetsdataWritable() error {
	return runCommandAsRoot("chown", []string{"-hR", "999:999", os.Getenv("JETS_TEMP_DATA")})
}

func runCommandAsRoot(command string, args []string) error {
	// Sanitize the command and arguments to prevent injection of options/flags
	args = utils.SanitizeArgs(args)
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
	// Sanitize the command and arguments to prevent injection of options/flags
	args = utils.SanitizeArgs(args)
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
