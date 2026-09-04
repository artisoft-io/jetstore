package main

import (
	"errors"
	"fmt"
	"io/fs"
	"log"
	"os"
	"os/exec"
	"strings"
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
// infer_server reads one more, JETS_INFER_BACKEND, which selects the model server to
// start: "ollama" (the default) or "vllm". See inferServerCommand for the variables each
// backend then requires, and dockerfiles/Dockerfile.infer_service{,_vllm} for the images
// that carry them.
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
		if err := startInferServer(); err != nil {
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

// The inference backends cbooter can start, and the values of JETS_INFER_BACKEND that
// select them. Item 15b added the second; the first is the default, so an image built
// before the toggle existed keeps working when the variable is absent.
//
// The value is normally baked into the image rather than set on the container definition:
// each of the two infer Dockerfiles sets it, because the image is what carries the server
// binary and a container told to run the other one cannot exec it.
const (
	inferBackendOllama = "ollama"
	inferBackendVllm   = "vllm"
)

// inferServerPort is the port both backends serve on. Fixed rather than configurable, and
// the same 11434 appears in three other places: the EXPOSE of both infer Dockerfiles, the
// container definition's port mapping (cdk/jetstore_one/stack/build_infer_service.go) and
// the load balancer target (cdk/jetstore_one/stack/build_elb.go). vLLM's own default is
// 8000; serving on 11434 instead is what leaves JETS_INFER_URL, the port mapping and the
// target group untouched when the backend changes.
const inferServerPort = "11434"

// inferBackendFromEnv reads the toggle, defaulting to Ollama when it is absent.
//
// The default is deliberate and is not a preference between the two: whether vLLM is
// promoted to the default image is item 17's decision and not this code's, so an
// unset variable has to keep meaning what it meant before the toggle existed.
func inferBackendFromEnv() string {
	if backend := os.Getenv("JETS_INFER_BACKEND"); backend != "" {
		return backend
	}
	return inferBackendOllama
}

// startInferServer starts the model server named by JETS_INFER_BACKEND.
//
// The server runs as a direct child process rather than as a nested `docker run`: this
// container has no access to a Docker socket, and with awsvpc networking a nested
// container would publish its port on the host namespace rather than on the task ENI the
// load balancer targets. The GPU is supplied by ECS through the task definition's
// GpuCount.
//
// Everything needing root happens here, before runCommandAsJsuser drops to uid 999: the
// cache directories under the mounted volume, and the chown of the volume itself.
func startInferServer() error {
	backend := inferBackendFromEnv()
	command, args, dirs, err := inferServerCommand(backend)
	if err != nil {
		return err
	}
	log.Printf("Starting infer_server with the %s backend: %s %v", backend, command, args)

	// Switching uid does not change HOME, so the child would otherwise inherit root's and
	// write its caches under /root, which the read-only root filesystem refuses. HOME must
	// point somewhere jsuser owns; both Dockerfiles put it under JETS_TEMP_DATA.
	homeDir := os.Getenv("HOME")
	if homeDir == "" {
		return fmt.Errorf("HOME environment variable must be set for infer_server")
	}
	for _, dir := range append([]string{homeDir}, dirs...) {
		if err := os.MkdirAll(dir, 0775); err != nil {
			return fmt.Errorf("failed to create the directory %s: %w", dir, err)
		}
	}
	// The mounted volume comes up owned by root and the task's root filesystem is
	// read-only, so this has to happen here, while we are still root.
	if err := makeJetsdataWritable(); err != nil {
		return fmt.Errorf("failed to make JETS_TEMP_DATA writable: %w", err)
	}
	return runCommandAsJsuser(command, args)
}

// inferServerCommand resolves a backend to the process that serves it, returning the
// command, its arguments, and the directories that must exist and be writable by jsuser
// before privileges are dropped.
//
// It reads the environment and touches nothing else, so both arms can be exercised
// without a server, a GPU or a container.
func inferServerCommand(backend string) (command string, args []string, dirs []string, err error) {
	switch backend {
	case inferBackendOllama:
		// OLLAMA_MODELS must live under JETS_TEMP_DATA so the weights land on the
		// persistent volume; jsuser has no writable home for Ollama's default of
		// $HOME/.ollama. OLLAMA_HOST carries the bind address, so `serve` takes no
		// arguments at all, and the OLLAMA_* tuning variables are read by the server
		// itself from the container environment.
		modelsDir := os.Getenv("OLLAMA_MODELS")
		if modelsDir == "" {
			return "", nil, nil, fmt.Errorf(
				"OLLAMA_MODELS environment variable must be set for the %s backend", inferBackendOllama)
		}
		return "ollama", []string{"serve"}, []string{modelsDir}, nil

	case inferBackendVllm:
		return vllmServerCommand()

	default:
		return "", nil, nil, fmt.Errorf("unknown JETS_INFER_BACKEND %q; expected %q or %q",
			backend, inferBackendOllama, inferBackendVllm)
	}
}

// vllmServerCommand builds the `vllm serve` invocation from the environment.
//
// One asymmetry with Ollama governs the whole of it: vLLM binds a single model at startup
// and Ollama chooses one per request. So JETS_INFER_MODEL is required here and has no
// Ollama counterpart; changing model is a task-definition revision rather than a pull; and
// the model an operator names in its .pc.json has to be the one this process served, which
// is what JETS_INFER_SERVED_MODEL_NAME is for -- a configuration can then name a short
// alias instead of a Hugging Face repository id.
//
// The bind address is passed as arguments rather than read from the environment, which is
// the other shape difference: there is no OLLAMA_HOST equivalent.
func vllmServerCommand() (string, []string, []string, error) {
	model := os.Getenv("JETS_INFER_MODEL")
	if model == "" {
		return "", nil, nil, fmt.Errorf(
			"JETS_INFER_MODEL environment variable must be set for the %s backend: vLLM serves the single model it is started with",
			inferBackendVllm)
	}
	// HF_HOME is where the weights land, and is the analogue of OLLAMA_MODELS. It has to be
	// on the mounted volume for the same two reasons: the root filesystem is read-only, and
	// a download repeated at every task start is paid in the cold-start figure item 17
	// measures.
	hfHome := os.Getenv("HF_HOME")
	if hfHome == "" {
		return "", nil, nil, fmt.Errorf(
			"HF_HOME environment variable must be set for the %s backend; see dockerfiles/Dockerfile.infer_service_vllm",
			inferBackendVllm)
	}
	args := []string{"serve", model, "--host", "0.0.0.0", "--port", inferServerPort}
	// Optional tuning. Each is omitted entirely when unset, so vLLM's own default applies
	// rather than a value invented here; the middle two are the counterparts of the
	// OLLAMA_CONTEXT_LENGTH and OLLAMA_NUM_PARALLEL pair the container definition documents
	// at length, and they divide VRAM against each other in the same way.
	for _, opt := range []struct{ env, flag string }{
		{"JETS_INFER_SERVED_MODEL_NAME", "--served-model-name"},
		{"JETS_VLLM_MAX_MODEL_LEN", "--max-model-len"},
		{"JETS_VLLM_MAX_NUM_SEQS", "--max-num-seqs"},
		{"JETS_VLLM_GPU_MEMORY_UTILIZATION", "--gpu-memory-utilization"},
	} {
		if v := os.Getenv(opt.env); v != "" {
			args = append(args, opt.flag, v)
		}
	}
	// The escape hatch for everything not named above, split on whitespace -- so a value
	// containing a quoted argument with a space in it will not survive, and nothing here
	// pretends otherwise. Appended last so that on argparse's usual last-one-wins handling
	// of a repeated option it can override a flag set above; that has not been exercised
	// against a running server, and the ordering is what this side controls.
	args = append(args, strings.Fields(os.Getenv("JETS_VLLM_EXTRA_ARGS"))...)
	return "vllm", args, []string{hfHome}, nil
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
