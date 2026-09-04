package main

// Tests for the infer_server backend toggle (item 15b, task AG.3).
//
// What is testable here and what is not is worth stating, because the gap is the whole of
// risk R-29. inferServerCommand is a pure reading of the environment, so every assertion
// below is real: the command, the arguments, the directories that must exist before
// privileges are dropped, and the errors. What no test in this package can reach is
// whether the resulting process starts, whether the image carries the binary it names, or
// whether the model loads on a GPU. There is no vLLM in this deployment or on the machine
// these tests run on.

import (
	"slices"
	"strings"
	"testing"

	"github.com/artisoft-io/jetstore/jets/utils"
)

// The toggle defaults to Ollama when unset, which is what keeps an image built before the
// toggle existed working, and what keeps the promotion decision (item 17) out of the code.
func TestInferBackendFromEnvDefaultsToOllama(t *testing.T) {
	t.Setenv("JETS_INFER_BACKEND", "")
	if got := inferBackendFromEnv(); got != inferBackendOllama {
		t.Errorf("unset JETS_INFER_BACKEND: got %q, want %q", got, inferBackendOllama)
	}
	t.Setenv("JETS_INFER_BACKEND", inferBackendVllm)
	if got := inferBackendFromEnv(); got != inferBackendVllm {
		t.Errorf("JETS_INFER_BACKEND=vllm: got %q, want %q", got, inferBackendVllm)
	}
}

// The Ollama arm is unchanged by the toggle: the same `ollama serve` with no arguments,
// the same OLLAMA_MODELS requirement, the same directory created before the uid drop.
func TestInferServerCommandOllama(t *testing.T) {
	t.Setenv("OLLAMA_MODELS", "/jetsdata/ollama")
	command, args, dirs, err := inferServerCommand(inferBackendOllama)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if command != "ollama" {
		t.Errorf("command: got %q, want %q", command, "ollama")
	}
	if !slices.Equal(args, []string{"serve"}) {
		t.Errorf("args: got %v, want [serve]", args)
	}
	if !slices.Equal(dirs, []string{"/jetsdata/ollama"}) {
		t.Errorf("dirs: got %v, want [/jetsdata/ollama]", dirs)
	}
}

func TestInferServerCommandOllamaRequiresModelsDir(t *testing.T) {
	t.Setenv("OLLAMA_MODELS", "")
	_, _, _, err := inferServerCommand(inferBackendOllama)
	if err == nil {
		t.Fatal("expected an error when OLLAMA_MODELS is unset")
	}
	if !strings.Contains(err.Error(), "OLLAMA_MODELS") {
		t.Errorf("the error must name the variable, got: %v", err)
	}
}

// The minimal vLLM invocation. The port is asserted literally rather than through the
// constant, because 11434 being vLLM's port is the decision that leaves JETS_INFER_URL,
// the container port mapping and the load balancer target group untouched -- vLLM's own
// default is 8000, and a test that reads the constant would follow it if it changed.
func TestInferServerCommandVllmMinimal(t *testing.T) {
	setVllmEnv(t)
	command, args, dirs, err := inferServerCommand(inferBackendVllm)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if command != "vllm" {
		t.Errorf("command: got %q, want %q", command, "vllm")
	}
	want := []string{"serve", "ibm-granite/granite-4.0-h-tiny", "--host", "0.0.0.0", "--port", "11434"}
	if !slices.Equal(args, want) {
		t.Errorf("args:\n got %v\nwant %v", args, want)
	}
	if !slices.Equal(dirs, []string{"/jetsdata/huggingface"}) {
		t.Errorf("dirs: got %v, want [/jetsdata/huggingface]", dirs)
	}
}

// vLLM binds one model at startup, so the model is required rather than optional. This is
// the asymmetry with Ollama that the CDK's env block and the deploy runbook both turn on.
func TestInferServerCommandVllmRequiresModel(t *testing.T) {
	setVllmEnv(t)
	t.Setenv("JETS_INFER_MODEL", "")
	_, _, _, err := inferServerCommand(inferBackendVllm)
	if err == nil {
		t.Fatal("expected an error when JETS_INFER_MODEL is unset")
	}
	if !strings.Contains(err.Error(), "JETS_INFER_MODEL") {
		t.Errorf("the error must name the variable, got: %v", err)
	}
}

// HF_HOME is the analogue of OLLAMA_MODELS: unset, the weights land somewhere the
// read-only root filesystem refuses, and the failure would be a task that starts and dies.
func TestInferServerCommandVllmRequiresHfHome(t *testing.T) {
	setVllmEnv(t)
	t.Setenv("HF_HOME", "")
	_, _, _, err := inferServerCommand(inferBackendVllm)
	if err == nil {
		t.Fatal("expected an error when HF_HOME is unset")
	}
	if !strings.Contains(err.Error(), "HF_HOME") {
		t.Errorf("the error must name the variable, got: %v", err)
	}
}

// Each optional flag is omitted when its variable is unset, so vLLM's own default applies
// rather than a number invented in this repository.
func TestInferServerCommandVllmOptionalFlagsAreOmitted(t *testing.T) {
	setVllmEnv(t)
	_, args, _, err := inferServerCommand(inferBackendVllm)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	for _, flag := range []string{"--served-model-name", "--max-model-len", "--max-num-seqs", "--gpu-memory-utilization"} {
		if slices.Contains(args, flag) {
			t.Errorf("%s present with its variable unset: %v", flag, args)
		}
	}
}

func TestInferServerCommandVllmOptionalFlags(t *testing.T) {
	setVllmEnv(t)
	t.Setenv("JETS_INFER_SERVED_MODEL_NAME", "granite")
	t.Setenv("JETS_VLLM_MAX_MODEL_LEN", "98304")
	t.Setenv("JETS_VLLM_MAX_NUM_SEQS", "2")
	t.Setenv("JETS_VLLM_GPU_MEMORY_UTILIZATION", "0.92")
	_, args, _, err := inferServerCommand(inferBackendVllm)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := []string{
		"serve", "ibm-granite/granite-4.0-h-tiny", "--host", "0.0.0.0", "--port", "11434",
		"--served-model-name", "granite",
		"--max-model-len", "98304",
		"--max-num-seqs", "2",
		"--gpu-memory-utilization", "0.92",
	}
	if !slices.Equal(args, want) {
		t.Errorf("args:\n got %v\nwant %v", args, want)
	}
}

// The escape hatch is appended last and split on whitespace. Last is the deliberate
// position: on the usual argparse behaviour a repeated option takes its final occurrence,
// so a value here would override one of the named flags above without anyone editing this
// file. **That override has not been exercised against a vLLM server** -- what this test
// asserts is the ordering, which is the part this repository controls.
func TestInferServerCommandVllmExtraArgsComeLast(t *testing.T) {
	setVllmEnv(t)
	t.Setenv("JETS_VLLM_MAX_MODEL_LEN", "98304")
	t.Setenv("JETS_VLLM_EXTRA_ARGS", "  --enforce-eager   --max-model-len 32768 ")
	_, args, _, err := inferServerCommand(inferBackendVllm)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := []string{
		"serve", "ibm-granite/granite-4.0-h-tiny", "--host", "0.0.0.0", "--port", "11434",
		"--max-model-len", "98304",
		"--enforce-eager", "--max-model-len", "32768",
	}
	if !slices.Equal(args, want) {
		t.Errorf("args:\n got %v\nwant %v", args, want)
	}
}

// An unrecognised value fails at start with both accepted values named, rather than
// silently starting Ollama -- which is what a default-on-unknown would do, and would read
// as a vLLM arm producing Ollama's numbers.
func TestInferServerCommandUnknownBackend(t *testing.T) {
	_, _, _, err := inferServerCommand("llamacpp")
	if err == nil {
		t.Fatal("expected an error for an unknown backend")
	}
	for _, want := range []string{"llamacpp", inferBackendOllama, inferBackendVllm} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("the error must name %q, got: %v", want, err)
		}
	}
}

// The arguments are handed to runCommandAsJsuser, which passes them through
// utils.SanitizeArgs. Nothing a well-formed invocation produces may be altered by it --
// a mangled --gpu-memory-utilization would be a server that refuses to start with a
// message about a flag nobody wrote.
func TestVllmArgsSurviveSanitizeArgs(t *testing.T) {
	setVllmEnv(t)
	t.Setenv("JETS_INFER_SERVED_MODEL_NAME", "granite")
	t.Setenv("JETS_VLLM_MAX_MODEL_LEN", "98304")
	t.Setenv("JETS_VLLM_GPU_MEMORY_UTILIZATION", "0.92")
	_, args, _, err := inferServerCommand(inferBackendVllm)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if sanitized := utils.SanitizeArgs(args); !slices.Equal(args, sanitized) {
		t.Errorf("SanitizeArgs altered the invocation:\n got %v\nwant %v", sanitized, args)
	}
}

// setVllmEnv sets the two required variables and clears every optional one, so a test
// that does not set an option is asserting its absence rather than inheriting it from the
// machine the suite runs on.
func setVllmEnv(t *testing.T) {
	t.Helper()
	t.Setenv("JETS_INFER_MODEL", "ibm-granite/granite-4.0-h-tiny")
	t.Setenv("HF_HOME", "/jetsdata/huggingface")
	for _, name := range []string{
		"JETS_INFER_SERVED_MODEL_NAME", "JETS_VLLM_MAX_MODEL_LEN",
		"JETS_VLLM_MAX_NUM_SEQS", "JETS_VLLM_GPU_MEMORY_UTILIZATION", "JETS_VLLM_EXTRA_ARGS",
	} {
		t.Setenv(name, "")
	}
}
