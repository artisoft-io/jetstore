package rete

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"slices"
	"sort"
	"strconv"

	"github.com/artisoft-io/jetstore/jets/jetrules/rdf"
)

// Factory class to create and configure ReteMetaStore objects
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

var devMode bool
var workspaceHome string
var wprefix string

func init() {
	_, devMode = os.LookupEnv("JETSTORE_DEV_MODE")
	workspaceHome = os.Getenv("WORKSPACES_HOME")
	wprefix = os.Getenv("WORKSPACE")
}

type VarInfo struct {
	Id       string
	IsBinded bool
	Vertex   int
	VarPos   int
}

// Context for building the components of the ReteMetaStore.
// The meta store is identified by the main rule uri `MainRuleUri`.
// The `ResourcesLookup` and `ResourcesLookup` are lookup to link the json components
// together while `LookupHelper`, `AlphaNodes`, and `NodeVertices` are the actual
// components that are being built here and are making the ReteMetaStore
type ReteBuilderContext struct {
	ResourceMgr     *rdf.ResourceManager
	MetaGraph       *rdf.RdfGraph
	ResourcesLookup map[int]*rdf.Node
	VariablesLookup map[int]*VarInfo
	MainRuleUri     string
	JetruleModel    *JetruleModel
	JetStoreConfig  *map[string]string
	LookupHelper    *LookupSqlHelper
	AlphaNodes      []*AlphaNode
	NodeVertices    []*NodeVertex
}

type ReteMetaStoreFactory struct {
	ResourceMgr     *rdf.ResourceManager
	MetaStoreLookup map[string]*ReteMetaStore
	ReteModelLookup map[string]*JetruleModel
}

// Main function to create the factory and to load ReteMetaStore, one per main rule file.
// Each rule file is a json with extension .jrcc.json produced by the rule compiler
func NewReteMetaStoreFactory(mainRuleFileNames []string) (*ReteMetaStoreFactory, error) {
	if len(mainRuleFileNames) == 0 {
		return nil, fmt.Errorf("error, must provide at least one main rule file name")
	}
	factory := &ReteMetaStoreFactory{
		ResourceMgr:     rdf.NewResourceManager(nil),
		MetaStoreLookup: make(map[string]*ReteMetaStore),
		ReteModelLookup: make(map[string]*JetruleModel),
	}
	for _, ruleFileName := range mainRuleFileNames {
		fpath := fmt.Sprintf("%s/%s/%scc.json", workspaceHome, wprefix, ruleFileName)
		log.Println("Reading JetStore rule config:", ruleFileName, "from:", fpath)
		file, err := os.ReadFile(fpath)
		if err != nil {
			log.Printf("while reading json file:%v\n", err)
			return nil, err
		}
		jetruleModel := JetruleModel{
			Antecedents: make([]*RuleTerm, 0),
			Consequents: make([]*RuleTerm, 0),
		}
		err = json.Unmarshal(file, &jetruleModel)
		if err != nil {
			log.Printf("while unmarshaling json:%v\n", err)
			return nil, err
		}
		factory.ReteModelLookup[ruleFileName] = &jetruleModel
	}
	// All the json rule files are parsed successfully
	// Load the ReteMetaStore from the rule model
	err := factory.Initialize()
	if err != nil {
		log.Printf("while loading the ReteMetaStore from jetrule model:%v\n", err)
		return nil, err
	}
	return factory, nil
}

// Transform the jetrule model into a set of ReteMetaStore
func (factory *ReteMetaStoreFactory) Initialize() error {
	// Note: single ResourceManager for all reteMetaStore
	// Note: each reteMetaStore have it's own MetaGraph
	for ruleUri, jrModel := range factory.ReteModelLookup {
		log.Println("Building ReteMetaStore for ruleset", ruleUri)
		builderContext := &ReteBuilderContext{
			ResourceMgr:     factory.ResourceMgr,
			MetaGraph:       rdf.NewRdfGraph("META"),
			ResourcesLookup: make(map[int]*rdf.Node),
			VariablesLookup: make(map[int]*VarInfo),
			MainRuleUri:     ruleUri,
			JetruleModel:    jrModel,
			JetStoreConfig:  &jrModel.JetstoreConfig,
			AlphaNodes:      make([]*AlphaNode, 0),
			NodeVertices:    []*NodeVertex{NewNodeVertex(0, nil, false, 0, nil, "(* * *)", nil)},
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

	//*##*TODO Load LookupSqlHelper
	ctx.LookupHelper = &LookupSqlHelper{}

	// Sort the antecedent terms from the consequent terms
	for i := range ctx.JetruleModel.ReteNodes {
		reteNode := ctx.JetruleModel.ReteNodes[i]
		switch reteNode.Type {
		case "antecedent":
			ctx.JetruleModel.Antecedents = append(ctx.JetruleModel.Antecedents, &reteNode)
		case "consequent":
			ctx.JetruleModel.Consequents = append(ctx.JetruleModel.Consequents, &reteNode)
		}
	}
	// Load all NodeVertex
	err = ctx.loadNodeVertices()
	if err != nil {
		return nil, err
	}

	// Load the alpha nodes
	//*##* CONTINUE HERE
	return nil, nil

	// Create & initialize the ReteMetaStore
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
			ctx.ResourcesLookup[resourceNode.Key] = ctx.ResourceMgr.NewLiteral(resourceNode.Value)
		case "int":
			v, err := strconv.Atoi(resourceNode.Value)
			if err != nil {
				return err
			}
			ctx.ResourcesLookup[resourceNode.Key] = ctx.ResourceMgr.NewLiteral(int32(v))
		case "date":
			v, err := rdf.NewLDate(resourceNode.Value)
			if err != nil {
				return err
			}
			ctx.ResourcesLookup[resourceNode.Key] = ctx.ResourceMgr.NewLiteral(v)
		case "double":
			v, err := strconv.ParseFloat(resourceNode.Value, 64)
			if err != nil {
				return err
			}
			ctx.ResourcesLookup[resourceNode.Key] = ctx.ResourceMgr.NewLiteral(v)
		case "bool":
			ctx.ResourcesLookup[resourceNode.Key] = ctx.ResourceMgr.NewLiteral(rdf.ParseBool(resourceNode.Value))
		case "datetime":
			v, err := rdf.NewLDatetime(resourceNode.Value)
			if err != nil {
				return err
			}
			ctx.ResourcesLookup[resourceNode.Key] = ctx.ResourceMgr.NewLiteral(v)
		case "long":
			v, err := strconv.Atoi(resourceNode.Value)
			if err != nil {
				return err
			}
			ctx.ResourcesLookup[resourceNode.Key] = ctx.ResourceMgr.NewLiteral(int64(v))
		case "uint":
			v, err := strconv.ParseUint(resourceNode.Value, 10, 32)
			if err != nil {
				return err
			}
			ctx.ResourcesLookup[resourceNode.Key] = ctx.ResourceMgr.NewLiteral(uint32(v))
		case "ulong":
			v, err := strconv.ParseUint(resourceNode.Value, 10, 64)
			if err != nil {
				return err
			}
			ctx.ResourcesLookup[resourceNode.Key] = ctx.ResourceMgr.NewLiteral(uint64(v))
		case "keyword":
			switch resourceNode.Value {
			case "true":
				ctx.ResourcesLookup[resourceNode.Key] = ctx.ResourceMgr.NewLiteral(int32(1))
			case "false":
				ctx.ResourcesLookup[resourceNode.Key] = ctx.ResourceMgr.NewLiteral(int32(0))
			case "null":
				ctx.ResourcesLookup[resourceNode.Key] = rdf.Null()
			}
		}
	}
	return nil
}

func (ctx *ReteBuilderContext) loadNodeVertices() error {
	// Load all NodeVertex
	// Ensure the Antecedents are sorted by vertex so to create NodeVertex recursively
	l := &ctx.JetruleModel.Antecedents
	sort.Slice(*l, func(i, j int) bool { return (*l)[i].Vertex < (*l)[j].Vertex })
	for i := range ctx.JetruleModel.Antecedents {
		reteNode := ctx.JetruleModel.ReteNodes[i]

		// Make the BetaRowInitializer
		sz := len(reteNode.BetaVarNodes)
		if sz == 0 {
			return fmt.Errorf("bug: antecedent with no beta variable")
		}
		brData := make([]int, sz)
		brLabels := make([]string, sz)
		for j := range reteNode.BetaVarNodes {
			betaVarNode := &reteNode.BetaVarNodes[j]
			if betaVarNode.IsBinded {
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
		if reteNode.ParentVertex >= len(ctx.NodeVertices) {
			return fmt.Errorf("bug: something is wrong, parent vertex >= vertex at vertex %d", reteNode.Vertex)
		}
		parent := ctx.NodeVertices[reteNode.ParentVertex]
		salience := 100
		if len(reteNode.Salience) > 0 {
			salience = slices.Min(reteNode.Salience)
		}
		ctx.NodeVertices = append(ctx.NodeVertices,
			NewNodeVertex(reteNode.Vertex, parent, reteNode.IsNot, salience, filterExpr,
				reteNode.NormalizedLabel, brInitializer))
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
		operator := CreateBinaryOperator(opStr)
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
		operator := CreateUnaryOperator(opStr)
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
	case int:
		// argv is a resource or a binded var
		r, ok := ctx.ResourcesLookup[vv]
		if ok {
			lhs = NewExprCst(r)
		} else {
			v, ok := ctx.VariablesLookup[vv]
			if !ok {
				return nil, fmt.Errorf("error: makeExpression called with %s as key %d but it's not a resource or a binded variable", argv, vv)
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
		return nil, fmt.Errorf("error: makeExpression called with unexpected type for %s", argv)
	}
	return lhs, nil
}