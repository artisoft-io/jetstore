// The Go side of the B.7 harness: feed synthesized configs through the real
// ValidatePipeSpecConfig, exactly the way the startup actions do.
//
// The Python side (cpipes_contract/harness.py) synthesizes a minimal config per
// (go_struct, type_token) from the matrix and the required-field-removal mutants;
// this program is deliberately dumb — it holds no opinion about the matrix and
// only answers "does JetStore accept this document", so the contract under test
// is the one the engine actually enforces.
//
// Protocol: a JSON array of {"id", "config"} on stdin, a JSON array of results on
// stdout. Two failure stages are distinguished because they mean different things
// to the caller: "decode" is a document json.Unmarshal refuses (a wrong go_type in
// the matrix, or a synthesis bug), "validate" is a rejection by the config
// validator (the thing the harness exists to observe).
//
// Each step is validated on a fresh unmarshal of the document: the validator
// mutates its input as it applies defaults (issue I-4), and the real callers each
// see a fresh copy per lambda invocation, so validating step N on a document
// step N-1 already mutated would test a state that never occurs.
package main

import (
	"encoding/json"
	"fmt"
	"io"
	"os"

	cp "github.com/artisoft-io/jetstore/jets/compute_pipes"
)

type caseIn struct {
	ID     string          `json:"id"`
	Config json.RawMessage `json:"config"`
}

type caseOut struct {
	ID    string `json:"id"`
	OK    bool   `json:"ok"`
	Stage string `json:"stage"` // ok | decode | validate
	Steps int    `json:"steps"` // steps that were validated; 0 is a vacuous accept
	Error string `json:"error,omitempty"`
}

// The env the When expressions of conditional steps are evaluated against,
// mirroring what actions_start_sharding_cp.go:100 puts there. Values are the
// quiet single-shard case; a synthesized config that depends on them is testing
// the wrong thing.
func evalEnv() map[string]any {
	env := map[string]any{
		"multi_step_sharding": 0,
		"total_file_size":     1024,
		"total_file_size_gb":  float64(1024) / 1024 / 1024 / 1024,
		"nbr_partitions":      1,
	}
	env["$MULTI_STEP_SHARDING"] = env["multi_step_sharding"]
	env["$TOTAL_FILE_SIZE"] = env["total_file_size"]
	env["$TOTAL_FILE_SIZE_GB"] = env["total_file_size_gb"]
	env["$NBR_PARTITIONS"] = env["nbr_partitions"]
	return env
}

// validateStep runs one step the way the startup actions do: select the step's
// pipes, apply the conditional transformation specs, then validate. The panic
// guard is there because the harness feeds the validator shapes no authored
// config ever has, and a panic must come back as a rejection, not kill the run.
func validateStep(raw json.RawMessage, stepId int) (validated bool, err error) {
	defer func() {
		if r := recover(); r != nil {
			validated, err = true, fmt.Errorf("panic: %v", r)
		}
	}()
	startup := &cp.CpipesStartup{EnvSettings: evalEnv()}
	if err := json.Unmarshal(raw, &startup.CpConfig); err != nil {
		return true, err
	}
	pipeConfig, _, err := startup.CpConfig.GetComputePipes(stepId, startup.EnvSettings)
	if err != nil {
		return true, err
	}
	if pipeConfig == nil {
		return false, nil
	}
	if err := cp.ApplyAllConditionalTransformationSpec(pipeConfig, startup.EnvSettings); err != nil {
		return true, err
	}
	return true, startup.ValidatePipeSpecConfig(&startup.CpConfig, pipeConfig)
}

func runCase(c caseIn) caseOut {
	var probe cp.ComputePipesConfig
	if err := json.Unmarshal(c.Config, &probe); err != nil {
		return caseOut{ID: c.ID, Stage: "decode", Error: err.Error()}
	}
	steps := 0
	for stepId := 0; stepId < probe.NbrComputePipes(); stepId++ {
		validated, err := validateStep(c.Config, stepId)
		if validated {
			steps++
		}
		if err != nil {
			return caseOut{ID: c.ID, Stage: "validate", Steps: steps, Error: err.Error()}
		}
	}
	return caseOut{ID: c.ID, OK: true, Stage: "ok", Steps: steps}
}

func main() {
	input, err := io.ReadAll(os.Stdin)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(2)
	}
	var cases []caseIn
	if err := json.Unmarshal(input, &cases); err != nil {
		fmt.Fprintf(os.Stderr, "harness: bad input: %v\n", err)
		os.Exit(2)
	}
	results := make([]caseOut, len(cases))
	for i, c := range cases {
		results[i] = runCase(c)
	}
	out, err := json.Marshal(results)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(2)
	}
	os.Stdout.Write(out)
}
