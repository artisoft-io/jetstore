package compute_pipes

import (
	"fmt"
	"log"
	"strings"
)

// The backends an infer operator can resolve to, and the operator type each one
// becomes. A new backend is a row here and a case in resolveInferSpec, not a new
// transformation type.
const (
	inferBackendOllama = "ollama"
	inferBackendVllm   = "vllm"
)

// ResolveInferBackend rewrites every `type: infer` transformation into the concrete
// operator its backend names, so that nothing downstream has to know the abstract
// type exists.
//
// Call it after ApplyAllConditionalTransformationSpec and before
// SynthesizeDefaultErrorChannels. The order is not a preference: the conditional
// pass may introduce or replace an infer step, and the error-channel synthesis
// dispatches on the operator type and would not recognise "infer". Every later
// stage -- the validator, the executors, SelectActiveOutputTable -- sees an ollama
// or vllm step and needs no change at all.
//
// It mutates pipeConfig in place and adds no PipeSpec, so the caller's slice header
// stays valid.
func ResolveInferBackend(pipeConfig []PipeSpec, env map[string]any) error {
	for i := range pipeConfig {
		for j := range pipeConfig[i].Apply {
			spec := &pipeConfig[i].Apply[j]
			if spec.Type != "infer" {
				continue
			}
			if spec.InferConfig == nil {
				return fmt.Errorf("error: transformation of type 'infer' must have infer_config")
			}
			if err := resolveInferSpec(spec, env); err != nil {
				return fmt.Errorf("while resolving infer operator at pipe %d, apply %d: %v", i, j, err)
			}
		}
	}
	return nil
}

// resolveInferBackendName reads the backend, following one env var reference. The
// lookup is an exact key match rather than a substitution scan, so "$INFER_BACKEND"
// cannot be partially matched by a longer key that shares its prefix.
//
// An unset or empty value is ollama: that is what every document in the corpus was
// before this type existed, so a deployment that says nothing keeps the behaviour it
// already had.
func resolveInferBackendName(backend string, env map[string]any) string {
	if strings.HasPrefix(backend, "$") {
		v, ok := env[backend]
		if !ok {
			return inferBackendOllama
		}
		s, ok := v.(string)
		if !ok {
			return inferBackendOllama
		}
		backend = s
	}
	backend = strings.TrimSpace(strings.ToLower(backend))
	if backend == "" {
		return inferBackendOllama
	}
	return backend
}

func resolveInferSpec(spec *TransformationSpec, env map[string]any) error {
	c := spec.InferConfig
	backend := resolveInferBackendName(c.Backend, env)

	// Report the keys this backend will not read. Logged rather than refused: a
	// document that serves both backends is expected to carry both operators'
	// specialized keys, and refusing one would make the abstraction useless. Silence
	// would be worse than either -- an author who misspells a backend name and loses
	// their structured output should be able to find out from the log.
	var inert []string
	switch backend {
	case inferBackendOllama:
		if c.StructuredOutput != "" {
			inert = append(inert, "structured_output")
		}
	case inferBackendVllm:
		if c.Think != nil {
			inert = append(inert, "think")
		}
		if c.KeepAlive != "" {
			inert = append(inert, "keep_alive")
		}
	}
	if len(inert) > 0 {
		log.Printf(
			"infer_config: backend %s does not read %s; the key is inert on this backend",
			backend, strings.Join(inert, ", "))
	}

	switch backend {
	case inferBackendOllama:
		spec.Type = inferBackendOllama
		spec.OllamaConfig = &OllamaSpec{
			Comment:         c.Comment,
			Model:           c.Model,
			Api:             c.Api,
			Options:         c.Options,
			KeepAlive:       c.KeepAlive,
			Think:           c.Think,
			Server:          c.Server,
			InferCommonSpec: c.InferCommonSpec,
		}
	case inferBackendVllm:
		spec.Type = inferBackendVllm
		spec.VllmConfig = &VllmSpec{
			Comment:          c.Comment,
			Model:            c.Model,
			Api:              c.Api,
			StructuredOutput: c.StructuredOutput,
			Options:          c.Options,
			Server:           c.Server,
			InferCommonSpec:  c.InferCommonSpec,
		}
	default:
		return fmt.Errorf(
			"error: unknown infer backend '%s', must be one of '%s' or '%s'",
			backend, inferBackendOllama, inferBackendVllm)
	}

	// The abstract config is consumed, not kept alongside its resolution: leaving it
	// set would give the document two states that can disagree.
	spec.InferConfig = nil
	return nil
}
