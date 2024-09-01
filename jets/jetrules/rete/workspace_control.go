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
type WorkspaceControl struct {
	RuleSets      []string       `json:"rule_sets"`
	RuleSequences []RuleSequence `json:"rule_sequences"`
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
