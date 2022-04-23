package main

import (
	"fmt"
	"github.com/artisoft-io/jetstore/jets/bridge"
)

type ReteWorkspace struct {
	js          bridge.JetStore
	workspaceDb string
	lookupDb    string
	ruleset     string
	procConfig  *ProcessConfig
}

func LoadReteWorkspace(workspaceDb string, lookupDb string, ruleset string, procConfig *ProcessConfig) (*ReteWorkspace, error) {
	// load the workspace db
	reteWorkspace := ReteWorkspace{workspaceDb: workspaceDb, lookupDb: lookupDb, ruleset: ruleset, procConfig: procConfig}
	js, err := bridge.LoadJetRules(workspaceDb, lookupDb)
	if err != nil {
		return &reteWorkspace, fmt.Errorf("while loading workspace db: %v", err)
	}
	reteWorkspace.js = js

	// assert the rule config triples to meta graph
	err = reteWorkspace.assertRuleConfig()
	return &reteWorkspace, err
}

func (rw *ReteWorkspace) assertRuleConfig() error {
	if rw == nil {
		return fmt.Errorf("ERROR: ReteWorkspace cannot be nil")
	}
	for _, t3 := range rw.procConfig.ruleConfigs {
		subject, err := rw.js.CreateResource(t3.subject)
		if err != nil {
			return fmt.Errorf("while asserting rule config: %v", err)
		}
		predicate, err := rw.js.CreateResource(t3.subject)
		if err != nil {
			return fmt.Errorf("while asserting rule config: %v", err)
		}
		var object bridge.Resource
		switch t3.rdfType {
		case "null":
			object, err = rw.js.CreateNull()
		case "bn":
			object, err = rw.js.CreateBlankNode(0)
		case "resource":
			object, err = rw.js.CreateResource(t3.object)
		case "int":
			var v int
			_, err = fmt.Sscan(t3.object, &v)
			if err != nil {
				return fmt.Errorf("while asserting rule config: %v", err)
			}
			object, err = rw.js.CreateIntLiteral(v)
		case "uint":
			var v uint
			_, err = fmt.Sscan(t3.object, &v)
			if err != nil {
				return fmt.Errorf("while asserting rule config: %v", err)
			}
			object, err = rw.js.CreateUIntLiteral(v)
		case "long":
			var v int
			_, err = fmt.Sscan(t3.object, &v)
			if err != nil {
				return fmt.Errorf("while asserting rule config: %v", err)
			}
			object, err = rw.js.CreateLongLiteral(v)
		case "ulong":
			var v uint
			_, err = fmt.Sscan(t3.object, &v)
			if err != nil {
				return fmt.Errorf("while asserting rule config: %v", err)
			}
			object, err = rw.js.CreateULongLiteral(v)
		case "double":
			var v float64
			_, err = fmt.Sscan(t3.object, &v)
			if err != nil {
				return fmt.Errorf("while asserting rule config: %v", err)
			}
			object, err = rw.js.CreateDoubleLiteral(v)
		case "text":
			object, err = rw.js.CreateTextLiteral(t3.object)
		case "date":
			object, err = rw.js.CreateDateLiteral(t3.object)
		case "datetime":
			object, err = rw.js.CreateDatetimeLiteral(t3.object)
		}
		if err != nil {
			return fmt.Errorf("while asserting rule config: %v", err)
		}
	rw.js.InsertRuleConfig(subject, predicate, object)
	}
	return nil
}
