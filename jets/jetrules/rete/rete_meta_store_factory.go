package rete

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"slices"
	"sort"
	"strconv"
	"strings"

	"github.com/artisoft-io/jetstore/jets/jetrules/rdf"
)

// Factory class to create and configure ReteMetaStore components.
// This factory is loading the Rete Network definition from json
// files produced by the rule compiler. The json is loaded into the
// data model `JetruleModel`

// Env variable:
// JETS_DSN_SECRET
// JETS_REGION
// JETS_BUCKET
// WORKSPACES_HOME Home dir of workspaces
// WORKSPACE Workspace currently in use
// JETSTORE_DEV_MODE Indicates running in dev mode, used to determine if sync workspace file from s3

var workspaceHome string
var wprefix string

func init() {
	workspaceHome = os.Getenv("WORKSPACES_HOME")
	wprefix = os.Getenv("WORKSPACE")
}

// Note: single ResourceManager for all reteMetaStores
// Note: single MetaGraph for all reteMetaStores
type ReteMetaStoreFactory struct {
	WorkspaceCtrl     *WorkspaceControl
	MainRuleFileNames []string
	ResourceMgr       *rdf.ResourceManager
	MetaGraph         *rdf.RdfGraph
	MetaStoreLookup   map[string]*ReteMetaStore
	ReteModelLookup   map[string]*JetruleModel
}

// Context for building the components of the ReteMetaStore.
// The meta store is identified by the main rule uri `MainRuleUri`.
// The `ResourcesLookup` and `VariablesLookup` are lookup to link the json components
// together while `LookupTables`, `AlphaNodes`, and `NodeVertices` are the actual
// components that are being built here and are making the ReteMetaStore
type ReteBuilderContext struct {
	ResourceMgr     *rdf.ResourceManager
	WorkspaceCtrl   *WorkspaceControl
	MetaGraph       *rdf.RdfGraph
	ResourcesLookup map[int]*rdf.Node
	VariablesLookup map[int]*VarInfo
	MainRuleUri     string
	JetruleModel    *JetruleModel
	JetStoreConfig  *map[string]string
	LookupTables    *LookupTableManager
	AlphaNodes      []*AlphaNode
	NodeVertices    []*NodeVertex
}

type VarInfo struct {
	Id       string
	IsBinded bool
	Vertex   int
	VarPos   int
}

// Main function to create the factory and to load ReteMetaStore, one per main rule file.
// Each rule file is a json with extension .jrcc.json produced by the rule compiler
// Argument jetRuleName is either a main rule file (ending with .jr) or a RuleSequence name.
// The WorkspaceControl component provides the mapping from jetRuleName to a list of
// main rule files.
func NewReteMetaStoreFactory(jetRuleName string) (*ReteMetaStoreFactory, error) {
	// Load the workspace control file
	wcPath := fmt.Sprintf("%s/%s/workspace_control.json", workspaceHome, wprefix)
	workspaceControl, err := LoadWorkspaceControl(wcPath)
	if err != nil {
		return nil, err
	}

	resourceManager := rdf.NewResourceManager(nil)
	factory := &ReteMetaStoreFactory{
		WorkspaceCtrl:     workspaceControl,
		MainRuleFileNames: workspaceControl.MainRuleFileNames(jetRuleName),
		ResourceMgr:       resourceManager,
		MetaGraph:         rdf.NewMetaRdfGraph(resourceManager),
		MetaStoreLookup:   make(map[string]*ReteMetaStore),
		ReteModelLookup:   make(map[string]*JetruleModel),
	}
	if len(factory.MainRuleFileNames) == 0 {
		return nil, fmt.Errorf("error, %s does not correspond to any rule file names", jetRuleName)
	}
	// Define function to avoid repeating code
	loadJson := func (strFmt, rootName string,  jetruleModel *JetruleModel) error {
		fpath := fmt.Sprintf(strFmt, workspaceHome, wprefix, rootName)
		file, err := os.ReadFile(fpath)
		if err != nil {
			err = fmt.Errorf("while reading .json file (NewReteMetaStoreFactory):%v", err)
			log.Println(err)
			return err
		}
		err = json.Unmarshal(file, jetruleModel)
		if err != nil {
			err = fmt.Errorf("while unmarshaling .json file (NewReteMetaStoreFactory):%v", err)
			log.Println(err)
			return err
		}
		return nil
	}

	for _, ruleFileName := range factory.MainRuleFileNames {
		rootName := strings.TrimSuffix(ruleFileName, ".jr")
		jetruleModel := JetruleModel{
			Antecedents: make([]*RuleTerm, 0),
			Consequents: make([]*RuleTerm, 0),
		}
		log.Println("Reading JetStore rule config:", ruleFileName)
		// .model.json
		err := loadJson("%s/%s/build/%s.model.json", rootName, &jetruleModel)
		if err != nil {
			return nil, err
		}
		// .rete.json
		err = loadJson("%s/%s/build/%s.rete.json", rootName, &jetruleModel)
		if err != nil {
			return nil, err
		}
		// .triples.json
		err = loadJson("%s/%s/build/%s.triples.json", rootName, &jetruleModel)
		if err != nil {
			return nil, err
		}
		factory.ReteModelLookup[ruleFileName] = &jetruleModel
	}
	// All the json rule files are parsed successfully
	// Load the ReteMetaStore from the rule model
	err = factory.initialize()
	if err != nil {
		log.Printf("while loading the ReteMetaStore from jetrule model:%v\n", err)
		return nil, err
	}
	return factory, nil
}

// Transform the jetrule models into a set of ReteMetaStore
func (factory *ReteMetaStoreFactory) initialize() error {
	// Note: single ResourceManager for all reteMetaStores
	// Note: single MetaGraph for all reteMetaStores
	for ruleUri, jrModel := range factory.ReteModelLookup {
		log.Println("Building ReteMetaStore for ruleset", ruleUri)
		builderContext := &ReteBuilderContext{
			ResourceMgr:     factory.ResourceMgr,
			WorkspaceCtrl:   factory.WorkspaceCtrl,
			MetaGraph:       factory.MetaGraph,
			ResourcesLookup: make(map[int]*rdf.Node),
			VariablesLookup: make(map[int]*VarInfo),
			MainRuleUri:     ruleUri,
			JetruleModel:    jrModel,
			JetStoreConfig:  &jrModel.JetstoreConfig,
			AlphaNodes:      make([]*AlphaNode, 0),
			NodeVertices:    make([]*NodeVertex, 0),
		}
		metaStore, err := builderContext.BuildReteMetaStore()
		if err != nil {
			return err
		}
		factory.MetaStoreLookup[ruleUri] = metaStore
	}
	return nil
}

func (ctx *ReteBuilderContext) BuildReteMetaStore() (*ReteMetaStore, error) {
	// Load all resources
	err := ctx.loadResources()
	if err != nil {
		return nil, err
	}

	// Load the meta triples from the rule files
	for i := range ctx.JetruleModel.Triples {
		t3 := &ctx.JetruleModel.Triples[i]
		s := ctx.ResourcesLookup[t3.SubjectKey]
		p := ctx.ResourcesLookup[t3.PredicateKey]
		o := ctx.ResourcesLookup[t3.ObjectKey]
		if s == nil || p == nil || o == nil {
			err := fmt.Errorf("error: invalid triples in metastore config, resource not found: (%v, %v, %v)", s, p, o)
			log.Println(err)
			return nil, err
		}
		_, err = ctx.MetaGraph.Insert(s, p, o)
		if err != nil {
			return nil, err
		}
	}

	// Load LookupTableManager
	ctx.LookupTables, err = NewLookupTableManager(ctx.ResourceMgr, ctx.MetaGraph, ctx.JetruleModel)
	if err != nil {
		return nil, fmt.Errorf("while calling LoadLookupTables for ruleUri %s: %v", ctx.MainRuleUri, err)
	}

	// Sort the antecedent terms from the consequent terms
	for i := range ctx.JetruleModel.ReteNodes {
		reteNode := &ctx.JetruleModel.ReteNodes[i]
		switch reteNode.Type {
		case "antecedent":
			ctx.JetruleModel.Antecedents = append(ctx.JetruleModel.Antecedents, reteNode)
		case "consequent":
			ctx.JetruleModel.Consequents = append(ctx.JetruleModel.Consequents, reteNode)
		case "head_node":
			ctx.JetruleModel.HeadRuleTerm = reteNode
		}
	}
	// Load all NodeVertex
	err = ctx.loadNodeVertices()
	if err != nil {
		return nil, err
	}

	// Load the alpha nodes
	// Initialize the network with the root node
	ctx.AlphaNodes = append(ctx.AlphaNodes, NewRootAlphaNode(ctx.NodeVertices[0]))
	for _, ruleTerm := range ctx.JetruleModel.Antecedents {
		var u, v, w AlphaFunctor
		var expr Expression
		if u, err = ctx.NewAlphaFunctor(ruleTerm.SubjectKey); err != nil {
			return nil, err
		}
		if v, err = ctx.NewAlphaFunctor(ruleTerm.PredicateKey); err != nil {
			return nil, err
		}
		if ruleTerm.ObjectExpr != nil {
			if expr, err = ctx.makeExpression(ruleTerm.ObjectExpr); err != nil {
				return nil, err
			}
			w = &FExpression{expression: expr}
		} else {
			if w, err = ctx.NewAlphaFunctor(ruleTerm.ObjectKey); err != nil {
				return nil, err
			}
		}
		if w == nil {
			return nil, fmt.Errorf("error: invalid AlphaNode configuration for vertex %d", ruleTerm.Vertex)
		}
		ctx.AlphaNodes = append(ctx.AlphaNodes,
			NewAlphaNode(u, v, w, ctx.NodeVertices[ruleTerm.Vertex], true, ruleTerm.NormalizedLabel))
	}
	// Initializing consequent terms
	for _, ruleTerm := range ctx.JetruleModel.Consequents {
		var u, v, w AlphaFunctor
		var expr Expression
		if u, err = ctx.NewAlphaFunctor(ruleTerm.SubjectKey); err != nil {
			return nil, err
		}
		if v, err = ctx.NewAlphaFunctor(ruleTerm.PredicateKey); err != nil {
			return nil, err
		}
		if ruleTerm.ObjectExpr != nil {
			if expr, err = ctx.makeExpression(ruleTerm.ObjectExpr); err != nil {
				return nil, err
			}
			w = &FExpression{expression: expr}
		} else {
			if w, err = ctx.NewAlphaFunctor(ruleTerm.ObjectKey); err != nil {
				return nil, err
			}
		}
		if w == nil {
			return nil, fmt.Errorf("error: invalid AlphaNode configuration for vertex %d", ruleTerm.Vertex)
		}
		ctx.AlphaNodes = append(ctx.AlphaNodes,
			NewAlphaNode(u, v, w, ctx.NodeVertices[ruleTerm.Vertex], false, ruleTerm.NormalizedLabel))
	}

	// Initialize routine perform important connection between the
	// metadata entities, such as reverse lookup of the consequent terms
	// and children lookup for each NodeVertex.

	// Perform reverse lookup of NodeVertex to their child AlphaNode
	// (which are neccessarily antecedent terms)
	for _, node := range ctx.AlphaNodes {
		if node.IsAntecedent {
			node.NdVertex.ParentNodeVertex.AddChildAlphaNode(node)
		} else {
			if node.NdVertex.Vertex > 0 {
				// Done with antecedent terms
				break
			}
		}
	}

	// Assign consequent terms vertex (AlphaNode) to NodeVertex
	// and validate that alpha nodes
	nbrVertices := len(ctx.NodeVertices)
	for ipos, alphaNode := range ctx.AlphaNodes {
		switch {
		case ipos == 0 && alphaNode.IsHeadNode:
			// pass
		case ipos > 0 && ipos < nbrVertices && alphaNode.IsAntecedent:
			// pass
		case ipos >= nbrVertices && alphaNode.IsConsequent:
			alphaNode.NdVertex.AddConsequentTerm(alphaNode)
		default:
			// something is wrong
			err = fmt.Errorf("BuildReteMetaStore: AlphaNode at position %d, with vertex %d fails validation",
				ipos, alphaNode.NdVertex.Vertex)
			return nil, err
		}
	}

	// Prepare a lookup of Data Properties (from classes) by name
	dataPropertyMap := make(map[string]*DataPropertyNode)
	for i := range ctx.JetruleModel.Classes {
		for j := range ctx.JetruleModel.Classes[i].DataProperties {
			p := &ctx.JetruleModel.Classes[i].DataProperties[j]
			dataPropertyMap[p.Name] = p
		}
	}

	// Prepare a lookup of Domain Tables by name
	domainTableMap := make(map[string]*TableNode)
	for i := range ctx.JetruleModel.Tables {
		t := &ctx.JetruleModel.Tables[i]
		domainTableMap[t.TableName] = t
	}

	// Create & initialize the ReteMetaStore
	return NewReteMetaStore(ctx.ResourceMgr, ctx.MetaGraph, ctx.LookupTables,
		ctx.AlphaNodes, ctx.NodeVertices, ctx.JetStoreConfig, dataPropertyMap, domainTableMap)
}

func (ctx *ReteBuilderContext) NewAlphaFunctor(key int) (AlphaFunctor, error) {
	varInfo := ctx.VariablesLookup[key]
	if varInfo != nil {
		if varInfo.IsBinded {
			return &FBinded{pos: varInfo.VarPos}, nil
		} else {
			return &FVariable{variable: varInfo.Id}, nil
		}
	}
	resource := ctx.ResourcesLookup[key]
	if resource != nil {
		return &FConstant{node: resource}, nil
	}
	return nil, fmt.Errorf("error: key %d not found in NewAlphaFunctor", key)
}

func (ctx *ReteBuilderContext) loadResources() error {
	// Load all resources
	for i := range ctx.JetruleModel.Resources {
		resourceNode := &ctx.JetruleModel.Resources[i]
		switch resourceNode.Type {

		case "var":
			varInfo := &VarInfo{
				Id:       resourceNode.Id,
				IsBinded: resourceNode.IsBinded,
				Vertex:   resourceNode.Vertex,
				VarPos:   resourceNode.VarPos,
			}
			ctx.VariablesLookup[resourceNode.Key] = varInfo

		case "resource":
			if len(resourceNode.Value) > 0 {
				// main case, use the value to create the resource
				ctx.ResourcesLookup[resourceNode.Key] = ctx.ResourceMgr.NewResource(resourceNode.Value)
			} else {
				return fmt.Errorf("error: resource require 'value' to be provided as resource name")
			}
		case "volatile_resource":
			if len(resourceNode.Value) > 0 {
				ctx.ResourcesLookup[resourceNode.Key] = ctx.ResourceMgr.NewResource("_0:" + resourceNode.Value)
			} else {
				return fmt.Errorf("error: resource require 'value' to be provided as resource name")
			}
		case "text", "string":
			ctx.ResourcesLookup[resourceNode.Key] = ctx.ResourceMgr.NewTextLiteral(resourceNode.Value)
		case "int":
			v, err := strconv.Atoi(resourceNode.Value)
			if err != nil {
				return err
			}
			ctx.ResourcesLookup[resourceNode.Key] = ctx.ResourceMgr.NewIntLiteral(v)
		case "date":
			v, err := rdf.NewLDate(resourceNode.Value)
			if err != nil {
				return err
			}
			ctx.ResourcesLookup[resourceNode.Key] = ctx.ResourceMgr.NewDateLiteral(v)
		case "double":
			v, err := strconv.ParseFloat(resourceNode.Value, 64)
			if err != nil {
				return err
			}
			ctx.ResourcesLookup[resourceNode.Key] = ctx.ResourceMgr.NewDoubleLiteral(v)
		case "bool":
			ctx.ResourcesLookup[resourceNode.Key] = ctx.ResourceMgr.NewIntLiteral(rdf.ParseBool(resourceNode.Value))
		case "datetime":
			v, err := rdf.NewLDatetime(resourceNode.Value)
			if err != nil {
				return err
			}
			ctx.ResourcesLookup[resourceNode.Key] = ctx.ResourceMgr.NewDatetimeLiteral(v)
		case "long":
			v, err := strconv.Atoi(resourceNode.Value)
			if err != nil {
				return err
			}
			ctx.ResourcesLookup[resourceNode.Key] = ctx.ResourceMgr.NewIntLiteral(v)
		case "uint":
			v, err := strconv.Atoi(resourceNode.Value)
			if err != nil {
				return err
			}
			ctx.ResourcesLookup[resourceNode.Key] = ctx.ResourceMgr.NewIntLiteral(v)
		case "ulong":
			v, err := strconv.Atoi(resourceNode.Value)
			if err != nil {
				return err
			}
			ctx.ResourcesLookup[resourceNode.Key] = ctx.ResourceMgr.NewIntLiteral(v)
		case "keyword":
			switch resourceNode.Value {
			case "true":
				ctx.ResourcesLookup[resourceNode.Key] = ctx.ResourceMgr.NewIntLiteral(1)
			case "false":
				ctx.ResourcesLookup[resourceNode.Key] = ctx.ResourceMgr.NewIntLiteral(0)
			case "null":
				ctx.ResourcesLookup[resourceNode.Key] = rdf.Null()
			}
		}
	}
	return nil
}

func (ctx *ReteBuilderContext) loadNodeVertices() error {
	// Load all NodeVertex from RuleTerms
	// Ensure the Antecedents are sorted by vertex so to create NodeVertex recursively
	l := &ctx.JetruleModel.Antecedents
	sort.Slice(*l, func(i, j int) bool { return (*l)[i].Vertex < (*l)[j].Vertex })

	// Initialize the slice of *NodeVertex with the root node
	ctx.NodeVertices = append(ctx.NodeVertices, NewNodeVertex(0, nil, false, 0, nil, "(* * *)", nil, nil))

	for _, reteNode := range ctx.JetruleModel.Antecedents {
		if reteNode.ParentVertex >= len(ctx.NodeVertices) {
			return fmt.Errorf("bug: something is wrong, parent vertex >= vertex at vertex %d", reteNode.Vertex)
		}
		parent := ctx.NodeVertices[reteNode.ParentVertex]
		// Make the BetaRowInitializer
		sz := len(reteNode.BetaVarNodes)
		brData := make([]int, sz)
		brLabels := make([]string, sz)
		for j := range reteNode.BetaVarNodes {
			betaVarNode := &reteNode.BetaVarNodes[j]
			if betaVarNode.IsBinded {
				if reteNode.ParentVertex == 0 {
					return fmt.Errorf(
						"bug: something is wrong, cannot have binded var %s at node %d since it's parent node is root node",
						betaVarNode.Id, reteNode.Vertex)
				}
				brData[j] = betaVarNode.VarPos | brcParentNode
			} else {
				brData[j] = betaVarNode.VarPos | brcTriple
			}
			brLabels[j] = betaVarNode.Id
		}
		brInitializer := NewBetaRowInitializer(brData, brLabels)

		// Make the Expression
		filterExpr, err := ctx.makeExpression(reteNode.Filter)
		if err != nil {
			err = fmt.Errorf("while making FilterExpr for NodeVertex at %d: %v", reteNode.Vertex, err)
			return err
		}
		salience := 100
		if len(reteNode.Salience) > 0 {
			salience = slices.Min(reteNode.Salience)
		}
		ctx.NodeVertices = append(ctx.NodeVertices,
			NewNodeVertex(reteNode.Vertex, parent, reteNode.IsNot, salience, filterExpr,
				reteNode.NormalizedLabel, reteNode.Rules, brInitializer))
	}
	return nil
}

func (ctx *ReteBuilderContext) makeExpression(expr map[string]interface{}) (Expression, error) {
	if len(expr) == 0 {
		return nil, nil
	}
	var err error
	var lhs, rhs Expression
	exprType, ok := expr["type"].(string)
	if !ok {
		return nil, fmt.Errorf("error: makeExpression called with expr without a proper type")
	}
	switch exprType {
	case "binary":
		lhs, err = ctx.makeExpressionArgument(expr, "lhs")
		if err != nil {
			return nil, err
		}
		rhs, err = ctx.makeExpressionArgument(expr, "rhs")
		if err != nil {
			return nil, err
		}
		opStr, ok := expr["op"].(string)
		if !ok {
			return nil, fmt.Errorf("error: makeExpression called for binary expression without an op")
		}
		operator := ctx.CreateBinaryOperator(opStr)
		if operator == nil {
			return nil, fmt.Errorf("error: makeExpression called for binary expression with unknown op %s", opStr)
		}
		return NewExprBinaryOp(lhs, operator, rhs), nil
	case "unary":
		rhs, err = ctx.makeExpressionArgument(expr, "arg")
		if err != nil {
			return nil, err
		}
		opStr, ok := expr["op"].(string)
		if !ok {
			return nil, fmt.Errorf("error: makeExpression called for unary expression without an op")
		}
		operator := ctx.CreateUnaryOperator(opStr)
		if operator == nil {
			return nil, fmt.Errorf("error: makeExpression called for unary expression with unknown op %s", opStr)
		}
		return NewExprUnaryOp(operator, rhs), nil
	}
	return nil, fmt.Errorf("error: makeExpression called with unknown type %s", exprType)
}

func (ctx *ReteBuilderContext) makeExpressionArgument(expr map[string]interface{}, argv string) (Expression, error) {
	var lhs Expression
	var err error
	switch vv := expr[argv].(type) {
	case float64:
		// argv is a resource or a binded var
		key := int(vv)
		r, ok := ctx.ResourcesLookup[key]
		if ok {
			lhs = NewExprCst(r)
		} else {
			v, ok := ctx.VariablesLookup[key]
			if !ok {
				return nil, fmt.Errorf("error: makeExpression called with %s as key %d but it's not a resource or a binded variable", argv, key)
			}
			if !v.IsBinded {
				return nil, fmt.Errorf("error: makeExpression called with %s as variable %s, but it's not a binded var", argv, v.Id)
			}
			lhs = NewExprBindedVar(v.VarPos, v.Id)
		}
	case map[string]interface{}:
		// argv is an expression
		lhs, err = ctx.makeExpression(vv)
		if err != nil {
			return nil, err
		}
	default:
		return nil, fmt.Errorf("error: makeExpression called with unexpected type for %s, it's %T from expr: %v", argv, expr[argv], expr)
	}
	return lhs, nil
}
