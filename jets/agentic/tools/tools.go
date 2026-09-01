package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"strings"

	cp "github.com/artisoft-io/jetstore/jets/compute_pipes"
	"github.com/artisoft-io/jetstore/jets/jetrules/rete"
)

// ---------------------------------------------------------------------------
// Tool 1 — list_domain_classes: the workspace-wide class view (F5).
// ---------------------------------------------------------------------------

type ClassSummary struct {
	Name        string   `json:"name"`
	BaseClasses []string `json:"base_classes,omitempty"`
	AsTable     bool     `json:"as_table"`
	SourceFile  string   `json:"source_file,omitempty"`
}

func ListDomainClasses(_ context.Context, ws *Workspace, _ json.RawMessage) (any, error) {
	out := make([]ClassSummary, 0, len(ws.Classes))
	for name, cls := range ws.Classes {
		out = append(out, ClassSummary{
			Name:        name,
			BaseClasses: cls.BaseClasses,
			AsTable:     cls.AsTable,
			SourceFile:  cls.SourceFileName,
		})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return map[string]any{"classes": out, "count": len(out)}, nil
}

// ---------------------------------------------------------------------------
// Tool 2 — describe_domain_class: compiled model + sidecar + triples.
// ---------------------------------------------------------------------------

type describeArgs struct {
	Name string `json:"name"`
}

type PropertyDescription struct {
	Name               string `json:"name"`
	Type               string `json:"type"`
	AsArray            bool   `json:"as_array,omitempty"`
	IsObject           bool   `json:"is_object,omitempty"`
	Description        string `json:"description,omitempty"`
	Required           bool   `json:"required,omitempty"`
	Key                bool   `json:"key,omitempty"`
	Vocabulary         string `json:"vocabulary,omitempty"`
	DataClassification string `json:"data_classification,omitempty"`
}

type ClassDescription struct {
	ClassSummary
	Documentation string                `json:"documentation"`
	Properties    []PropertyDescription `json:"properties"`
	Vocabularies  map[string][]string   `json:"vocabularies,omitempty"`
	Triples       []Triple              `json:"triples,omitempty"`
}

func DescribeDomainClass(_ context.Context, ws *Workspace, args json.RawMessage) (any, error) {
	var in describeArgs
	if err := json.Unmarshal(args, &in); err != nil {
		return nil, fmt.Errorf("while parsing arguments: %w", err)
	}
	if in.Name == "" {
		return nil, fmt.Errorf("argument 'name' is required")
	}
	cls, ok := ws.Classes[in.Name]
	if !ok {
		return nil, fmt.Errorf("no class named %q in this workspace (list_domain_classes has the %d that exist)",
			in.Name, len(ws.Classes))
	}
	desc := &ClassDescription{
		ClassSummary: ClassSummary{
			Name:        in.Name,
			BaseClasses: cls.BaseClasses,
			AsTable:     cls.AsTable,
			SourceFile:  cls.SourceFileName,
		},
	}
	for _, p := range cls.DataProperties {
		desc.Properties = append(desc.Properties, PropertyDescription{
			Name: p.Name, Type: p.Type, AsArray: p.AsArray, IsObject: p.IsObject,
		})
	}
	for _, p := range cls.ObjectProperties {
		desc.Properties = append(desc.Properties, PropertyDescription{
			Name: p.Name, Type: "resource", AsArray: p.AsArray, IsObject: true,
		})
	}
	sort.Slice(desc.Properties, func(i, j int) bool { return desc.Properties[i].Name < desc.Properties[j].Name })

	// The sidecar overlay: descriptions, requiredness, vocabularies inlined
	// in full — for schema-first classes only.
	var ent *SidecarEntity
	if ws.Meta != nil {
		if e, ok := ws.Meta.Entities[in.Name]; ok {
			ent = &e
		}
	}
	if ent != nil {
		desc.Documentation = ent.Description
		desc.Vocabularies = map[string][]string{}
		for i := range desc.Properties {
			p := &desc.Properties[i]
			sp, ok := ent.Properties[p.Name]
			if !ok {
				continue
			}
			p.Description = sp.Description
			p.Required = sp.Required
			p.Key = sp.Key
			p.DataClassification = sp.DataClassification
			if sp.Vocabulary != "" {
				p.Vocabulary = sp.Vocabulary
				desc.Vocabularies[sp.Vocabulary] = ws.Meta.Vocabularies[sp.Vocabulary].Values
			}
		}
	}

	// The triples channel — the description source for .jr-authored classes,
	// and rule-visible metadata for schema-first ones.
	triples, err := ws.TriplesAbout(classSubjects(in.Name, cls))
	if err != nil {
		return nil, err
	}
	desc.Triples = triples

	// A class declared with no authored metadata is the defined answer, not
	// a missing-key error: client classes with no sidecar entry arrive on
	// the first real run (section 3.5).
	if ent == nil {
		if len(triples) == 0 {
			desc.Documentation = "declared, no metadata authored"
		} else {
			desc.Documentation = "documented by workspace triples"
		}
	}
	return desc, nil
}

// ---------------------------------------------------------------------------
// Tool 3 — validate_cpipes_config: the real startup validator, on a copy.
// ---------------------------------------------------------------------------

type validateArgs struct {
	Config json.RawMessage `json:"config"`
	// ConfigPath is I-94's remedy on this tool (T.2). `config` relaxes to
	// {"type":"object"} at the wire — relaxExternalRefs must, since an MCP
	// client cannot resolve a cross-file $ref — so a config arriving with half
	// its pipes missing satisfies the schema. A path is not carried and cannot
	// be mangled. Exactly one of the two, for the reason compileArgs gives.
	ConfigPath string `json:"config_path"`
}

type StepDiagnostic struct {
	Step  int    `json:"step"`
	Error string `json:"error"`
}

type ValidationReport struct {
	Valid          bool             `json:"valid"`
	StepsValidated int              `json:"steps_validated"`
	Diagnostics    []StepDiagnostic `json:"diagnostics,omitempty"`
}

// ValidateCpipesConfig wraps CpipesStartup.ValidatePipeSpecConfig
// (actions_start_common.go), step by step, exactly the way the startup
// actions and the track-B harness run it. The submitted JSON is unmarshalled
// into a private CpipesStartup, so the validator's default-applying
// mutations (I-4) touch a copy and nothing the caller holds. The env is the
// quiet single-shard case the harness uses.
func ValidateCpipesConfig(_ context.Context, ws *Workspace, args json.RawMessage) (any, error) {
	var in validateArgs
	if err := json.Unmarshal(args, &in); err != nil {
		return nil, fmt.Errorf("while parsing arguments: %w", err)
	}
	hasConfig := len(in.Config) > 0 && string(in.Config) != "null"
	hasPath := strings.TrimSpace(in.ConfigPath) != ""
	switch {
	case hasConfig && hasPath:
		return nil, fmt.Errorf(
			"give either 'config' or 'config_path', not both: they name two different " +
				"configurations and this tool will not guess which one you meant")
	case !hasConfig && !hasPath:
		return nil, fmt.Errorf(
			"one of 'config_path' (a workspace-relative .pc.json, preferred) or 'config' " +
				"(the configuration object itself) is required")
	}
	if hasPath {
		// The path arm needs the handle this tool did not previously use, and
		// that is the one visible cost of the remedy here: validation itself
		// touches no workspace — the config is validated in a private
		// CpipesStartup — so a config given by value can be checked against a
		// workspace that was never materialised on disk, and one given by path
		// cannot.
		rel, abs, err := resolveWorkspacePath(ws, in.ConfigPath)
		if err != nil {
			return nil, err
		}
		if !strings.HasSuffix(rel, ".json") {
			return nil, fmt.Errorf(
				"argument 'config_path' must name a .json file (a cpipes config is a .pc.json), got %q", rel)
		}
		raw, err := os.ReadFile(abs)
		if err != nil {
			return nil, fmt.Errorf(
				"no configuration at %q in this workspace (list_workspace_files reports the paths that exist): %w",
				rel, err)
		}
		if !json.Valid(raw) {
			// A file that is not JSON is a fact about the file, and reporting
			// it as a step diagnostic keeps the caller's report shape the same
			// whichever arm produced it.
			return &ValidationReport{
				Valid:       false,
				Diagnostics: []StepDiagnostic{{Step: -1, Error: fmt.Sprintf("%s does not hold JSON", rel)}},
			}, nil
		}
		in.Config = json.RawMessage(raw)
	}
	var probe cp.ComputePipesConfig
	if err := json.Unmarshal(in.Config, &probe); err != nil {
		return &ValidationReport{
			Valid:       false,
			Diagnostics: []StepDiagnostic{{Step: -1, Error: fmt.Sprintf("config does not decode: %v", err)}},
		}, nil
	}
	report := &ValidationReport{Valid: true}
	for stepId := 0; stepId < probe.NbrComputePipes(); stepId++ {
		if diag := validateStep(in.Config, stepId); diag != "" {
			report.Valid = false
			report.Diagnostics = append(report.Diagnostics, StepDiagnostic{Step: stepId, Error: diag})
		}
		report.StepsValidated++
	}
	return report, nil
}

func validateStep(raw json.RawMessage, stepId int) (diag string) {
	defer func() {
		if r := recover(); r != nil {
			diag = fmt.Sprintf("validator panic: %v", r)
		}
	}()
	startup := &cp.CpipesStartup{EnvSettings: validationEnv()}
	if err := json.Unmarshal(raw, &startup.CpConfig); err != nil {
		return err.Error()
	}
	pipeConfig, _, err := startup.CpConfig.GetComputePipes(stepId, startup.EnvSettings)
	if err != nil {
		return err.Error()
	}
	if pipeConfig == nil {
		return ""
	}
	if err := cp.ApplyAllConditionalTransformationSpec(pipeConfig, startup.EnvSettings); err != nil {
		return err.Error()
	}
	if err := startup.ValidatePipeSpecConfig(&startup.CpConfig, pipeConfig); err != nil {
		return err.Error()
	}
	return ""
}

// validationEnv mirrors what actions_start_sharding_cp.go puts in the env:
// the quiet single-shard case, as the track-B harness settled.
func validationEnv() map[string]any {
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

// ---------------------------------------------------------------------------
// Shared helpers over the compiled class model.
// ---------------------------------------------------------------------------

func classSubjects(name string, cls *rete.ClassNode) []string {
	subjects := []string{name}
	subjects = append(subjects, classPropertyNames(cls)...)
	return subjects
}

func classPropertyNames(cls *rete.ClassNode) []string {
	var names []string
	for _, p := range cls.DataProperties {
		names = append(names, p.Name)
	}
	for _, p := range cls.ObjectProperties {
		names = append(names, p.Name)
	}
	sort.Strings(names)
	return names
}
