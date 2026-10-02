package workspace

// Validating a workspace's user-flow and table documents before it compiles.
// jetstore_maintenance_02 tasks AG.4 and AG.5 (R12).
//
// The documents under user_flows/ and table_configs/ are going to be written by
// a model, so whatever validates them is the whole of their quality control. The
// per-document half is Go (jets/userflow, on every save); the half that spans
// documents -- a flow's forms and actions resolving against each other, a
// table's actions against the flow's, every escape against the build's registry
// -- is TypeScript, in jetsclient_ide/src/userflow/. It is bundled into one Node
// script, jets_validate_workspace.mjs (jetsclient_ide/vite.validator.config.ts),
// and run here, so every caller of CompileWorkspace -- the apiserver at startup,
// the IDE's compile action, the Docker build's compile_workspace, run_reports --
// passes the same gate.
//
// **It fails when it cannot run. It never skips itself.** A validator that
// quietly does nothing when node or its script is absent passes every compile in
// an image without them, which until this change was every runtime image, and
// would look exactly like a validator that had passed (that project's R-9). The
// only way past it is SkipAssetValidation, which a caller names with a reason.

import (
	"context"
	"errors"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

// ValidatorScriptName is the bundled validator's file name. The images copy it
// beside the binaries that compile -- /usr/local/bin in the apiserver image, /app
// in compile_ws -- which is where locateValidatorScript looks first.
const ValidatorScriptName = "jets_validate_workspace.mjs"

// Environment overrides, for a workstation or a test: where node is, and where
// the script is. Unset, node is found on PATH and the script beside the running
// executable.
const (
	NodeEnvVar            = "JETS_NODE"
	ValidatorScriptEnvVar = "JETS_WORKSPACE_VALIDATOR"
)

// The script reads and parses JSON and nothing else: 0.13 s over cedargate_ws's
// 46 flow documents and 36 tables, measured 2026-10-01. Two minutes is room for a
// cold disk and a slow container, and a hang is a failure rather than a compile
// that never returns.
const assetValidationTimeout = 2 * time.Minute

// CompileOption adjusts one call to CompileWorkspace.
type CompileOption func(*compileOptions)

type compileOptions struct {
	skipAssetValidation bool
	skipReason          string
}

// SkipAssetValidation is the one way past the asset validation step, and the
// reason is required: it is logged, and written into the compile log, so a
// compile that did not validate says so and says why. An empty reason fails the
// compile rather than being taken as a skip -- an opt-out nobody can explain is
// the silent skip this step exists to refuse.
func SkipAssetValidation(reason string) CompileOption {
	return func(o *compileOptions) {
		o.skipAssetValidation = true
		o.skipReason = strings.TrimSpace(reason)
	}
}

// ErrAssetValidation is what a compile refused by the validator wraps, so a
// caller can tell "the documents are invalid" from "the rules did not compile".
var ErrAssetValidation = errors.New("workspace asset validation failed")

// locateNode returns the node interpreter: JETS_NODE when set, else node on PATH.
func locateNode() (string, error) {
	if p := os.Getenv(NodeEnvVar); p != "" {
		if _, err := os.Stat(p); err != nil {
			return "", fmt.Errorf("%s=%s: %w", NodeEnvVar, p, err)
		}
		return p, nil
	}
	p, err := exec.LookPath("node")
	if err != nil {
		return "", fmt.Errorf("node is not on PATH (%w); the apiserver and compile_ws images carry it at /usr/local/node/bin, and %s names it explicitly", err, NodeEnvVar)
	}
	return p, nil
}

// locateValidatorScript returns the bundled script: JETS_WORKSPACE_VALIDATOR
// when set, else ValidatorScriptName beside the running executable.
func locateValidatorScript() (string, error) {
	if p := os.Getenv(ValidatorScriptEnvVar); p != "" {
		if _, err := os.Stat(p); err != nil {
			return "", fmt.Errorf("%s=%s: %w", ValidatorScriptEnvVar, p, err)
		}
		return p, nil
	}
	exe, err := os.Executable()
	if err != nil {
		return "", fmt.Errorf("cannot locate %s beside the executable: %w", ValidatorScriptName, err)
	}
	p := filepath.Join(filepath.Dir(exe), ValidatorScriptName)
	if _, err := os.Stat(p); err != nil {
		return "", fmt.Errorf("%s is not beside %s (%w); build it with `npm run build:validator` in jetsclient_ide and set %s to its path",
			ValidatorScriptName, exe, err, ValidatorScriptEnvVar)
	}
	return p, nil
}

// validateWorkspaceAssets runs the bundled validator over workspaceDir. It
// returns the validator's output, for the compile log, and an error wrapping
// ErrAssetValidation when the documents are invalid or the validator could not
// run.
func validateWorkspaceAssets(workspaceDir string, opts compileOptions) (string, error) {
	if opts.skipAssetValidation {
		if opts.skipReason == "" {
			err := fmt.Errorf("%w: SkipAssetValidation was passed with no reason", ErrAssetValidation)
			return err.Error() + "\n", err
		}
		msg := fmt.Sprintf("Workspace asset validation skipped: %s\n", opts.skipReason)
		log.Print(msg)
		return msg, nil
	}

	node, err := locateNode()
	if err != nil {
		err = fmt.Errorf("%w: cannot run the validator: %v", ErrAssetValidation, err)
		log.Println(err)
		return err.Error() + "\n", err
	}
	script, err := locateValidatorScript()
	if err != nil {
		err = fmt.Errorf("%w: cannot run the validator: %v", ErrAssetValidation, err)
		log.Println(err)
		return err.Error() + "\n", err
	}

	ctx, cancel := context.WithTimeout(context.Background(), assetValidationTimeout)
	defer cancel()
	cmd := exec.CommandContext(ctx, node, script, workspaceDir)
	out, runErr := cmd.CombinedOutput()
	var buf strings.Builder
	fmt.Fprintf(&buf, "Validating workspace documents: %s %s %s\n", node, script, workspaceDir)
	buf.Write(out)
	if runErr == nil {
		log.Print(buf.String())
		return buf.String(), nil
	}

	// Exit 1 is the validator's verdict on the documents; anything else -- 2 for
	// a directory it could not read, a signal, a timeout -- is a validator that
	// did not finish, and is reported as such rather than as invalid documents.
	var exitErr *exec.ExitError
	switch {
	case ctx.Err() != nil:
		err = fmt.Errorf("%w: the validator did not finish within %s", ErrAssetValidation, assetValidationTimeout)
	case errors.As(runErr, &exitErr) && exitErr.ExitCode() == 1:
		err = fmt.Errorf("%w: user_flows/ or table_configs/ has an invalid document, see the findings above", ErrAssetValidation)
	default:
		err = fmt.Errorf("%w: the validator did not run to completion: %v", ErrAssetValidation, runErr)
	}
	buf.WriteString(err.Error())
	buf.WriteString("\n")
	log.Print(buf.String())
	return buf.String(), err
}
