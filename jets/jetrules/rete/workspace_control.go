package rete

import (
	"encoding/json"
	"log"
	"os"
)

// Component living in the workspace root directory as a json
// This component list the main rule files of the workspace,
// known as ruleSets, and the ruleSeq which are sequences
// of ruleSets.

// WorkspaceControl defines the main rule files and rule sequences of a workspace
// This is currently user authored and stored in workspace_control.json
// in the root of the workspace directory.
// UseCompilerV2 indicates if the workspace should be compiled with the v2 compiler.
// UseTraceMode indicates if the workspace should be compiled with tracing enabled (applied to v2)
// AutoAddResources indicates if resources in rules should be automatically added when not explicitly defined
// in the resource section of the rule file. (applied to v2)
type WorkspaceControl struct {
	WorkspaceName    string         `json:"workspace_name,omitempty"`
	UseCompilerV2    bool           `json:"use_compiler_v2,omitzero"`
	UseTraceMode     bool           `json:"use_trace_mode,omitzero"`
	AutoAddResources bool           `json:"auto_add_resources,omitzero"`
	RuleSets         []string       `json:"rule_sets,omitempty"`
	RuleSequences    []RuleSequence `json:"rule_sequences,omitempty"`
}

func NewWorkspaceControl(ruleSets []string, ruleSequences []RuleSequence) *WorkspaceControl {
	return &WorkspaceControl{
		RuleSets:      ruleSets,
		RuleSequences: ruleSequences,
	}
}

type RuleSequence struct {
	Name     string   `json:"name"`
	RuleSets []string `json:"rule_sets"`
}

func NewRuleSequence(name string, ruleSets []string) *RuleSequence {
	return &RuleSequence{
		Name:     name,
		RuleSets: ruleSets,
	}
}

func LoadWorkspaceControl(fpath string) (*WorkspaceControl, error) {
	file, err := os.ReadFile(fpath)
	if err != nil {
		log.Printf("while reading workspace_control.json file:%v\n", err)
		return nil, err
	}
	var workspaceControl WorkspaceControl
	err = json.Unmarshal(file, &workspaceControl)
	if err != nil {
		log.Printf("while unmarshaling workspace_control.json:%v\n", err)
		return nil, err
	}
	return &workspaceControl, nil
}

// Provide rule file names (ending in .jr) for a logical ruleFileName
// which may correspong to a RuleSequence or an individual main rule file.
func (wc *WorkspaceControl) MainRuleFileNames(ruleFileName string) []string {
	if wc == nil {
		return nil
	}
	for i := range wc.RuleSequences {
		if wc.RuleSequences[i].Name == ruleFileName {
			return wc.RuleSequences[i].RuleSets
		}
	}
	for i := range wc.RuleSets {
		if wc.RuleSets[i] == ruleFileName {
			return []string{ruleFileName}
		}
	}
	return nil
}
