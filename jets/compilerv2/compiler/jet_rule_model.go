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
	resourceManager *ResourceManager

	// Internal state
	nextKey                   int
	currentRuleFileName       string
	currentClass              *rete.ClassNode
	currentRuleSequence       *rete.RuleSequence
	currentLookupTableColumns []rete.LookupTableColumn
	currentRuleProperties     map[string]string
	currentRuleAntecedents    []rete.RuleTerm
	currentRuleConsequents    []rete.RuleTerm
	// stack to build expressions in Antecedents and Consequents
	inProgressExpr            *stack.Stack[rete.ExpressionNode]

	// Logs
	parseLog *strings.Builder
	errorLog *strings.Builder
	trace    bool
}

func NewJetRuleListener(basePath string, mainRuleFileName string) *JetRuleListener {
	outJsonFileName := strings.TrimSuffix(mainRuleFileName, ".jetrule") + ".json"
	return &JetRuleListener{
		mainRuleFileName: mainRuleFileName,
		basePath:         basePath,
		outJsonFileName:  outJsonFileName,
		jetRuleModel:     rete.NewJetruleModel(),
		resourceManager:  NewResourceManager(),
		parseLog:         &strings.Builder{},
		errorLog:         &strings.Builder{},
	}
}

// ResourceManager manages the resources in the model
// during the model compilation process performed by the JetRuleListener
// Resources are stored in a map with key as "type|value" to ensure uniqueness
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
