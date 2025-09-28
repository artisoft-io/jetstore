package compiler

import (
	"strings"

	"github.com/artisoft-io/jetstore/jets/compilerv2/parser"
	"github.com/artisoft-io/jetstore/jets/jetrules/rete"
	"github.com/artisoft-io/jetstore/jets/stack"
)

// This file contains the object model for the JetRule compiler
// to support the resource parsing, transforming, and validating
// The domain object model is in rete package (see rete_meta_store_model.go)

// JetRuleListener to build the tree
type JetRuleListener struct {
	*parser.BaseJetRuleListener
	// Input
	mainRuleFileName string
	basePath         string

	// Parse Tree Output
	outJsonFileName string
	jetRuleModel    *rete.JetruleModel

	// ResourceManager
	autoAddResources bool
	resourceManager  *ResourceManager
	classesByName    map[string]*rete.ClassNode

	// Internal state
	currentRuleFileName       string
	currentClass              *rete.ClassNode
	currentRuleSequence       *rete.RuleSequence
	currentLookupTableColumns []rete.LookupTableColumn
	currentRuleProperties     map[string]string
	currentRuleAntecedents    []*rete.RuleTerm
	currentRuleConsequents    []*rete.RuleTerm
	currentJetruleNode        *rete.JetruleNode
	currentRuleVarByValue     map[string]*rete.ResourceNode // map of variable Value (original name) to ResourceNode, rule level
	// stack to build expressions in Antecedents and Consequents
	inProgressExpr *stack.Stack[rete.ExpressionNode]

	// Logs
	parseLog *strings.Builder
	errorLog *strings.Builder
	trace    bool
}

func NewJetRuleListener(basePath string, mainRuleFileName string) *JetRuleListener {
	outJsonFileName := strings.TrimSuffix(mainRuleFileName, ".jetrule") + ".json"
	l := &JetRuleListener{
		mainRuleFileName:      mainRuleFileName,
		basePath:              basePath,
		outJsonFileName:       outJsonFileName,
		jetRuleModel:          rete.NewJetruleModel(),
		resourceManager:       NewResourceManager(),
		classesByName:         make(map[string]*rete.ClassNode),
		currentRuleVarByValue: make(map[string]*rete.ResourceNode),
		parseLog:              &strings.Builder{},
		errorLog:              &strings.Builder{},
	}
	l.AddR("jets:client")
	l.AddR("jets:completed")
	l.AddR("jets:currentSourcePeriod")
	l.AddR("jets:currentSourcePeriodDate")
	l.AddR("jets:entity_property")
	l.AddR("jets:exception")
	l.AddR("jets:from")
	l.AddR("jets:InputRecord")
	l.AddR("jets:iState")
	l.AddR("jets:key")
	l.AddR("jets:length")
	l.AddR("jets:lookup_multi_rows")
	l.AddR("jets:lookup_row")
	l.AddR("jets:loop")
	l.AddR("jets:max_vertex_visits")
	l.AddR("jets:operator")
	l.AddR("jets:org")
	l.AddR("jets:range_value")
	l.AddR("jets:replace_chars")
	l.AddR("jets:replace_with")
	l.AddR("jets:source_period_sequence")
	l.AddR("jets:sourcePeriodType")
	l.AddR("jets:State")
	l.AddR("jets:value_property")
	l.AddR("rdf:type")
	return l
}

// ResourceManager manages the resources in the model
// during the model compilation process performed by the JetRuleListener
// Resources are stored in a map with key as "type|value" to ensure uniqueness (does not applies to ?var)
// ResourceById is a map of resources by their Id
// ResourceByKey is a map of resources by their Key
// see jet_rule_listener_utility.go for usage
type ResourceManager struct {
	NextKey       int
	Resources     map[string]*rete.ResourceNode
	ResourceById  map[string]*rete.ResourceNode
	ResourceByKey map[int]*rete.ResourceNode
}

func NewResourceManager() *ResourceManager {
	return &ResourceManager{
		NextKey:       1,
		Resources:     make(map[string]*rete.ResourceNode),
		ResourceById:  make(map[string]*rete.ResourceNode),
		ResourceByKey: make(map[int]*rete.ResourceNode),
	}
}
