package jetruledb

import (
	// "context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"os"
	"strings"

	// "github.com/artisoft-io/jetstore/jets/awsi"
	"github.com/artisoft-io/jetstore/jets/datatable"
	"github.com/jackc/pgx/v4/pgxpool"
)

type CompileJetruleAction struct {
	Action       string                   `json:"action"`
	JetruleFile  string                   `json:"jetruleFile"`
	UserEmail    string                   `json:"userEmail"`
	Workspace    string                   `json:"workspace"`
	DropTables   bool                     `json:"drop_tables"`
	UpdateSchema bool                     `json:"update_schema"`
	Data         []map[string]interface{} `json:"data"`
}

// Jetrule Domain Model
type JetruleModel struct {
	MainRuleFileName     string                   `json:"main_rule_file_name"`
	SupportRuleFileNames []string                 `json:"support_rule_file_names"`
	Resources            []ResourceNode           `json:"resources"`
	LookupTables         []LookupTableNode        `json:"lookup_tables"`
	Jetrules             []JetruleNode            `json:"jet_rules"`
	ReteNodes            []RuleTerm               `json:"rete_nodes"`
	Imports              map[string][]string      `json:"imports"`
	JetstoreConfig       map[string]string        `json:"jetstore_config"`
	RuleSequences        []map[string]interface{} `json:"rule_sequences"`
	Classes              []ClassNode              `json:"classes"`
	Tables               []TableNode              `json:"tables"`
	Triples              []TripleNode             `json:"triples"`
}

type ResourceNode struct {
	Id             string `json:"id"`
	Inline         bool   `json:"inline"`
	IsAntecedent   bool   `json:"is_antecedent"`
	IsBinded       bool   `json:"is_binded"`
	Key            int    `json:"key"`
	DbKey          int    `json:"db_key"`
	SourceFileName string `json:"source_file_name"`
	Type           string `json:"type"`
	Value          string `json:"value"`
	VarPos         int    `json:"var_pos"`
	Vertex         int    `json:"vertex"`
}

type LookupTableNode struct {
	Columns        []map[string]interface{} `json:"columns"`
	CsvFile        string                   `json:"csv_file"`
	Key            []string                 `json:"key"`
	Name           string                   `json:"name"`
	Resources      []string                 `json:"resources"`
	SourceFileName string                   `json:"source_file_name"`
	Type           string                   `json:"type"`
	DbKey          int                      `json:"db_key"`
}

// LookupTableNode.Columns
// type LookupColumnNode struct {
// 	Type     string `json:"type"`
// 	Name     string `json:"name"`
// 	AsArray  bool   `json:"as_array"`
// }

type JetruleNode struct {
	Name            string                    `json:"name"`
	Properties      map[string]string         `json:"properties"`
	Optimization    bool                      `json:"optimization"`
	Salience        int                       `json:"salience"`
	Antecedents     []RuleTerm                `json:"antecedents"`
	Consequents     []RuleTerm                `json:"consequents"`
	AuthoredLabel   string                    `json:"authoredLabel"`
	SourceFileName  string                    `json:"source_file_name"`
	NormalizedLabel string                    `json:"normalizedLabel"`
	Label           string                    `json:"label"`
	DbKey              int                    `json:"db_key"`
}

// RulTerm type is either antecedent or consequent
type RuleTerm struct {
	Type               string                 `json:"type"`
	IsNot              bool                   `json:"isNot"`
	NormalizedLabel    string                 `json:"normalizedLabel"`
	Vertex             int                    `json:"vertex"`
	ParentVertex       int                    `json:"parent_vertex"`
	BetaRelationVars   []string               `json:"beta_relation_vars"`
	PrunedVars         []string               `json:"pruned_var"`
	BetaVarNodes       []BetaVarNode          `json:"beta_var_nodes"`
	ChildrenVertexes   []int                  `json:"children_vertexes"`
	Rules              []string               `json:"rules"`
	Salience           []int                  `json:"salience"`
	ConsequentSeq      int                    `json:"consequent_seq"`
	ConsequentForRule  string                 `json:"consequent_for_rule"`
	ConsequentSalience int                    `json:"consequent_salience"`
	SubjectKey         int                    `json:"subject_key"`
	PredicateKey       int                    `json:"predicate_key"`
	ObjectKey          int                    `json:"object_key"`
	ObjectExpr         map[string]interface{} `json:"obj_expr"`
	Filter             map[string]interface{} `json:"filter"`
	DbKey              int                    `json:"db_key"`
}

type BetaVarNode struct {
	Type           string    `json:"type"`
	Id             string    `json:"id"`
	IsBinded       bool      `json:"is_binded"`
	VarPos         int       `json:"var_pos"`
	Vertex         int       `json:"vertex"`
	SourceFileName string    `json:"source_file_name"`
	DbKey          int       `json:"db_key"`
}

// type Antecedent struct {
// 	Type                string        `json:"type"`
// 	IsNot               bool          `json:"isNot"`
// 	NormalizedLabel     string        `json:"normalizedLabel"`
// 	Vertex              int           `json:"vertex"`
// 	ParentVertex        int           `json:"parent_vertex"`
// 	BetaRelationVars    []string      `json:"beta_relation_vars"`
// 	PrunedVars          []string      `json:"pruned_var"`
// 	BetaVarNodes	      []BetaVarNode `json:"beta_var_nodes"`
// 	ChildrenVertexes    []int         `json:"children_vertexes"`
// 	Rules               []string      `json:"rules"`
// 	Salience            []int         `json:"salience"`
// 	SubjectKey          int           `json:"subject_key"`
// 	PredicateKey        int           `json:"predicate_key"`
// 	ObjectKey	          int           `json:"object_key"`
// }

// type Consequent struct {
// 	Type               string `json:"type"`
// 	NormalizedLabel    string `json:"normalizedLabel"`
// 	Vertex             int    `json:"vertex"`
// 	ConsequentSeq      int    `json:"consequent_seq"`
// 	ConsequentForRule  string `json:"consequent_for_rule"`
// 	ConsequentSalience int    `json:"consequent_salience"`
// 	SubjectKey         int    `json:"subject_key"`
// 	PredicateKey       int    `json:"predicate_key"`
// 	ObjectKey	         int    `json:"object_key"`
// 	ObjectExpr         ObjExprNode  `json:"obj_expr"`
// }

// type ObjExprNode struct {
// 	Type           string              `json:"type"`
// 	Lhs            interface{}         `json:"lhs"`
// 	Op             string              `json:"op"`
// 	Rhs            interface{}         `json:"rhs"`
// 	Arg            interface{}         `json:"arg"`
// 	DbKey          int                 `json:"db_key"`
// }

type ClassNode struct {
	Type           string             `json:"type"`
	Name           string             `json:"name"`
	BaseClasses    []string           `json:"base_classes"`
	DataProperties []DataPropertyNode `json:"data_properties"`
	SourceFileName string             `json:"source_file_name"`
	AsTable        bool               `json:"as_table"`
	SubClasses     []string           `json:"sub_classes"`
	DbKey          int                `json:"db_key"`
}

type DataPropertyNode struct {
	Type           string `json:"type"`
	DomainClassKey int    `json:"domain_class_key"`
	Name           string `json:"name"`
	AsArray        bool   `json:"as_array"`
	DbKey          int    `json:"db_key"`
}

type TableNode struct {
	DomainClassKey int               `json:"domain_class_key"`
	TableName      string            `json:"table_name"`
	ClassName      string            `json:"class_name"`
	Columns        []TableColumnNode `json:"columns"`
	SourceFileName string            `json:"source_file_name"`
	DbKey          int               `json:"db_key"`
}

type TableColumnNode struct {
	Type         string `json:"type"`
	AsArray      bool   `json:"as_array"`
	PropertyName string `json:"property_name"`
	ColumnName   string `json:"column_name"`
}

type TripleNode struct {
	Type           string       `json:"type"`
	SubjectKey     int          `json:"subject_key"`
	PredicateKey   int          `json:"predicate_key"`
	ObjectKey      int          `json:"object_key"`
	SourceFileName string       `json:"source_file_name"`
}

func CompileJetrule(dbpool *pgxpool.Pool, compileJetruleAction *CompileJetruleAction, token string) (*map[string]interface{}, int, error) {
	return nil, 0, errors.New("TODO: NOT IMPLEMENTED YET")
}

// local context for writing domain model, used within WriteJetrule
type writeWorkspaceContext struct {
	model           *JetruleModel
	sourceFileKeys  *map[string]int
	domainClassMap  map[string]*ClassNode
	dataPropertyMap map[string]*DataPropertyNode
	// Map[ResourceNode.Key] -> ResourceNode.DbKey
	resourcesMap map[int]int
	// Map of ReteNode using composite key
	// Map[RuleTerm.Vertex+RuleTerm.ConsequentSeq] -> RuleTerm
	reteNodeMap map[string]*RuleTerm
}

// Persist Jetrule json structure to database
func WriteJetrule(dbpool *pgxpool.Pool, compileJetruleAction *CompileJetruleAction, token string) (*map[string]interface{}, int, error) {
	// if err != nil {
	// 	return nil, http.StatusInternalServerError, fmt.Errorf("while calling InsertSourcePeriod: %v", err)
	// }
	datatableCtx := &datatable.Context{
		Dbpool: dbpool,
	}
	writeWorkspaceCtx := &writeWorkspaceContext{
		model:           &JetruleModel{},
		sourceFileKeys:  &map[string]int{},
		domainClassMap:  map[string]*ClassNode{},
		dataPropertyMap: map[string]*DataPropertyNode{},
		resourcesMap:    map[int]int{},
		reteNodeMap:     map[string]*RuleTerm{},
	}

	log.Println("ReadFile:", compileJetruleAction.JetruleFile)
	file, err := os.ReadFile(compileJetruleAction.JetruleFile)
	if err != nil {
		log.Printf("while reading json file:%v\n", err)
		return &map[string]interface{}{}, http.StatusBadRequest, err
	}

	err = json.Unmarshal(file, writeWorkspaceCtx.model)
	if err != nil {
		log.Printf("while unmarshaling json:%v\n", err)
		return &map[string]interface{}{}, http.StatusBadRequest, err
	}
	// //*
	// fmt.Println("GOT",writeWorkspaceCtx.model)

	// insert the main rule file in workspace_control table
	if writeWorkspaceCtx.model.MainRuleFileName != "" {
		writeWorkspaceCtx.insertRuleFileName(datatableCtx,
			writeWorkspaceCtx.model.MainRuleFileName, true, compileJetruleAction.Workspace, &token)
	}

	// Persist the Resources
	if len(writeWorkspaceCtx.model.Resources) > 0 {
		log.Println("Writing Resources")
		err = writeWorkspaceCtx.WriteResources(datatableCtx, compileJetruleAction.Workspace, &token)
		if err != nil {
			log.Printf("while WriteResources:%v\n", err)
			return &map[string]interface{}{}, http.StatusBadRequest, err
		}
	}

	// Persist the Classes & Tables
	if len(writeWorkspaceCtx.model.Classes) > 0 {
		// init the map of domain class
		for i := range writeWorkspaceCtx.model.Classes {
			cls := &writeWorkspaceCtx.model.Classes[i]
			writeWorkspaceCtx.domainClassMap[cls.Name] = cls
		}
		log.Println("Writing Domain Classes")
		err = writeWorkspaceCtx.WriteDomainClasses(datatableCtx, compileJetruleAction.Workspace, &token)
		if err != nil {
			log.Printf("while WriteDomainClasses:%v\n", err)
			return &map[string]interface{}{}, http.StatusBadRequest, err
		}
	}
	if len(writeWorkspaceCtx.model.Tables) > 0 {
		log.Println("Writing Domain Tables")
		err = writeWorkspaceCtx.WriteDomainTables(datatableCtx, compileJetruleAction.Workspace, &token)
		if err != nil {
			log.Printf("while WriteDomainTables:%v\n", err)
			return &map[string]interface{}{}, http.StatusBadRequest, err
		}
	}

	// Persist the JetStore Config
	if writeWorkspaceCtx.model.JetstoreConfig != nil {
		log.Println("Writing JetStore Config")
		err = writeWorkspaceCtx.WriteJetStoreConfig(datatableCtx, compileJetruleAction.Workspace, &token)
		if err != nil {
			log.Printf("while WriteJetStoreConfig:%v\n", err)
			return &map[string]interface{}{}, http.StatusBadRequest, err
		}
	}

	// Persist Rule Sequences
	if writeWorkspaceCtx.model.RuleSequences != nil {
		log.Println("Writing Rule Sequences")
		err = writeWorkspaceCtx.WriteRuleSequences(datatableCtx, compileJetruleAction.Workspace, &token)
		if err != nil {
			log.Printf("while WriteRuleSequences:%v\n", err)
			return &map[string]interface{}{}, http.StatusBadRequest, err
		}
	}

	// Persist Lookup Tables
	if writeWorkspaceCtx.model.LookupTables != nil {
		log.Println("Writing Lookup Tables")
		err = writeWorkspaceCtx.WriteLookupTables(datatableCtx, compileJetruleAction.Workspace, &token)
		if err != nil {
			log.Printf("while WriteLookupTables:%v\n", err)
			return &map[string]interface{}{}, http.StatusBadRequest, err
		}
	}

	// Persist Expressions
	if writeWorkspaceCtx.model.ReteNodes != nil {
		log.Println("Writing Expressions")
		err = writeWorkspaceCtx.WriteExpr(datatableCtx, compileJetruleAction.Workspace, &token)
		if err != nil {
			log.Printf("while WriteExpr:%v\n", err)
			return &map[string]interface{}{}, http.StatusBadRequest, err
		}
	}

	// Persist Rete Nodes
	if writeWorkspaceCtx.model.ReteNodes != nil {
		log.Println("Writing Rete Nodes")
		err = writeWorkspaceCtx.WriteReteNodes(datatableCtx, compileJetruleAction.Workspace, &token)
		if err != nil {
			log.Printf("while WriteReteNodes:%v\n", err)
			return &map[string]interface{}{}, http.StatusBadRequest, err
		}
	}

	// Persist Jet Rules
	if writeWorkspaceCtx.model.Jetrules != nil {
		log.Println("Writing Jet Rules")
		err = writeWorkspaceCtx.WriteJetRules(datatableCtx, compileJetruleAction.Workspace, &token)
		if err != nil {
			log.Printf("while WriteJetRules:%v\n", err)
			return &map[string]interface{}{}, http.StatusBadRequest, err
		}
	}

	// Persist Triples
	if writeWorkspaceCtx.model.Triples != nil {
		log.Println("Writing Triples")
		err = writeWorkspaceCtx.WriteTriples(datatableCtx, compileJetruleAction.Workspace, &token)
		if err != nil {
			log.Printf("while WriteTriples:%v\n", err)
			return &map[string]interface{}{}, http.StatusBadRequest, err
		}
	}

	//* DEV
	// Write back as json
	b, err := json.MarshalIndent(writeWorkspaceCtx.model, "", "  ")
	if err != nil {
		log.Printf("while writing json:%v\n", err)
		return &map[string]interface{}{}, http.StatusBadRequest, err
	}
	os.WriteFile("out.jrcc.json", b, 0666)

	return &map[string]interface{}{}, http.StatusOK, nil
}

// utility functions
func (ctx *writeWorkspaceContext) insertRuleFileName(datatableCtx *datatable.Context, ruleFileName string, isMain bool, workspace string, token *string) (int, error) {
	keys, err := ctx.insertRows(datatableCtx, &[]map[string]interface{}{
		{"source_file_name": ruleFileName, "is_main": isMain},
	}, "workspace_control", workspace, token)
	if keys == nil || err != nil {
		err = fmt.Errorf("error: no keys or err returned from InsertRows in insertRows (for workspace_control table), err is '%v'", err)
		log.Println(err)
		return 0, err
	}
	key := (*keys)[0]
	// save the key in the context
	(*ctx.sourceFileKeys)[ruleFileName] = key
	return key, nil
}

func (ctx *writeWorkspaceContext) insertRows(datatableCtx *datatable.Context, data *[]map[string]interface{}, table string, workspace string, token *string) (returnedKeys *[]int, err error) {
	// check if data has key 'source_file_name' and convert it into 'source_file_key'
	if table != "workspace_control" {
		for i := range *data {
			s := (*data)[i]["source_file_name"]
			if s != nil {
				sourceFileName := s.(string)
				skey := (*ctx.sourceFileKeys)[sourceFileName]
				if skey == 0 {
					skey, err = ctx.insertRuleFileName(datatableCtx, sourceFileName, false, workspace, token)
					if err != nil {
						err = fmt.Errorf("while calling insertRuleFileName: %v", err)
						log.Println(err)
						return
					}
				}
				(*data)[i]["source_file_key"] = skey
			}
		}
	}
	dataTableAction := &datatable.DataTableAction{
		Action:      "insert_rows",
		FromClauses: []datatable.FromClause{{Schema: workspace, Table: fmt.Sprintf("WORKSPACE/%s", table)}},
		Data:        *data,
	}

	results, _, err := datatableCtx.InsertRows(dataTableAction, *token)
	if err != nil {
		log.Printf("while calling InsertRows:%v\n", err)
		return
	}
	returnedKeys = (*results)["returned_keys"].(*[]int)
	return
}

// Transform a struct into json and then back into a row (array of interface{})
// to insert into database using api
func appendDataRow(v any, data *[]map[string]interface{}) error {
	b, err := json.Marshal(v)
	if err != nil {
		log.Printf("while writing json:%v\n", err)
		return err
	}
	row := map[string]interface{}{}
	err = json.Unmarshal(b, &row)
	if err != nil {
		log.Printf("while reading json:%v\n", err)
		return err
	}
	*data = append(*data, row)
	return nil
}

// JetruleModel Methods
// --------------------
// WriteResources
func (ctx *writeWorkspaceContext) WriteResources(datatableCtx *datatable.Context, workspace string, token *string) error {

	data := []map[string]interface{}{}
	for i := range ctx.model.Resources {
		r := &ctx.model.Resources[i]
		err := appendDataRow(r, &data)
		if err != nil {
			return err
		}
	}
	returnedKeys, err := ctx.insertRows(datatableCtx, &data, "resources", workspace, token)
	if returnedKeys == nil || err != nil {
		err = fmt.Errorf("error: no keys or err returned from InsertRows in WriteResources, err is '%v'", err)
		log.Println(err)
		return err
	}
	for i := range ctx.model.Resources {
		r := &ctx.model.Resources[i]
		r.DbKey = (*returnedKeys)[i]
		ctx.resourcesMap[r.Key] = r.DbKey
	}

	return nil
}

func (ctx *writeWorkspaceContext) getClass(name *string) *ClassNode {
	switch {
	case name == nil:
		return nil
	case *name == "owl:Thing":
		return &ClassNode{
			Name:           "owl:Thing",
			BaseClasses:    []string{},
			DataProperties: []DataPropertyNode{},
			AsTable:        false,
			SubClasses:     []string{},
			DbKey:          1,
		}
	default:
		return ctx.domainClassMap[*name]
	}
}

func (ctx *writeWorkspaceContext) getDataPropertyKey(name *string) int {
	switch {
	case name == nil:
		return 0
	default:
		p := ctx.dataPropertyMap[*name]
		if p == nil {
			return 0
		}
		return p.DbKey
	}
}

// WriteDomainClasses
func (ctx *writeWorkspaceContext) WriteDomainClasses(datatableCtx *datatable.Context, workspace string, token *string) error {
	// write to domain_classes
	data := []map[string]interface{}{}
	for i := range ctx.model.Classes {
		cls := &ctx.model.Classes[i]
		err := appendDataRow(cls, &data)
		if err != nil {
			return err
		}
	}
	returnedKeys, err := ctx.insertRows(datatableCtx, &data, "domain_classes", workspace, token)
	if returnedKeys == nil || err != nil {
		err = fmt.Errorf("error: no keys or err returned from InsertRows in WriteDomainClasses writing to domain_classes, err is '%v'", err)
		log.Println(err)
		return err
	}
	for i := range ctx.model.Classes {
		r := &ctx.model.Classes[i]
		r.DbKey = (*returnedKeys)[i]
	}

	// write to base_classes
	data = data[:0]
	var row map[string]interface{}
	for i := range ctx.model.Classes {
		cls := &ctx.model.Classes[i]
		for j := range cls.BaseClasses {
			baseCls := ctx.getClass(&cls.BaseClasses[j])
			if baseCls == nil {
				err = fmt.Errorf("error: cannot find domain class with name: %s", cls.BaseClasses[j])
				log.Println(err)
				return err
			}
			row = map[string]interface{}{}
			row["domain_class_key"] = cls.DbKey
			row["base_class_key"] = baseCls.DbKey
			data = append(data, row)
		}
	}
	_, err = ctx.insertRows(datatableCtx, &data, "base_classes", workspace, token)
	if err != nil {
		err = fmt.Errorf("error: err returned from InsertRows in WriteDomainClasses writing base_classes: %v", err)
		log.Println(err)
		return err
	}

	// write to data_properties
	data = data[:0]
	for i := range ctx.model.Classes {
		cls := &ctx.model.Classes[i]
		for j := range cls.DataProperties {
			cls.DataProperties[j].DomainClassKey = cls.DbKey
			err := appendDataRow(&cls.DataProperties[j], &data)
			if err != nil {
				return err
			}
			ctx.dataPropertyMap[cls.DataProperties[j].Name] = &cls.DataProperties[j]
		}
	}
	returnedKeys, err = ctx.insertRows(datatableCtx, &data, "data_properties", workspace, token)
	if returnedKeys == nil || err != nil {
		err = fmt.Errorf("error: no keys or err returned from InsertRows in WriteDomainClasses writing data_properties, err is '%v'", err)
		log.Println(err)
		return err
	}
	ipos := 0
	for i := range ctx.model.Classes {
		cls := &ctx.model.Classes[i]
		for j := range cls.DataProperties {
			r := &cls.DataProperties[j]
			r.DbKey = (*returnedKeys)[ipos]
			ipos += 1
		}
	}
	return nil
}

// WriteDomainTables
func (ctx *writeWorkspaceContext) WriteDomainTables(datatableCtx *datatable.Context, workspace string, token *string) error {
	// write to domain_tables
	data := []map[string]interface{}{}
	for i := range ctx.model.Tables {
		v := &ctx.model.Tables[i]
		v.DomainClassKey = ctx.getClass(&v.ClassName).DbKey
		row := map[string]interface{}{}
		row["domain_class_key"] = v.DomainClassKey
		row["name"] = v.TableName
		data = append(data, row)
	}
	returnedKeys, err := ctx.insertRows(datatableCtx, &data, "domain_tables", workspace, token)
	if returnedKeys == nil || err != nil {
		err = fmt.Errorf("error: no keys or err returned from InsertRows in WriteDomainTables writing to domain_tables, err is '%v'", err)
		log.Println(err)
		return err
	}
	for i := range ctx.model.Tables {
		r := &ctx.model.Tables[i]
		r.DbKey = (*returnedKeys)[i]
	}

	// write to domain_columns
	data = data[:0]
	for i := range ctx.model.Tables {
		v := &ctx.model.Tables[i]
		for j := range v.Columns {
			domainColumn := &v.Columns[j]
			row := map[string]interface{}{}
			row["domain_table_key"] = v.DbKey
			row["data_property_key"] = ctx.getDataPropertyKey(&domainColumn.PropertyName)
			row["as_array"] = domainColumn.AsArray
			row["name"] = domainColumn.ColumnName
			data = append(data, row)
		}
	}
	_, err = ctx.insertRows(datatableCtx, &data, "domain_columns", workspace, token)
	if err != nil {
		err = fmt.Errorf("error: err returned from InsertRows in WriteDomainTables writing domain_columns: %v", err)
		log.Println(err)
		return err
	}
	return nil
}

// WriteJetStoreConfig
func (ctx *writeWorkspaceContext) WriteJetStoreConfig(datatableCtx *datatable.Context, workspace string, token *string) error {
	// write to jetstore_config
	data := []map[string]interface{}{}
	sourceFileName := ctx.model.JetstoreConfig["source_file_name"]
	for k, v := range ctx.model.JetstoreConfig {
		if strings.HasPrefix(k, "$") {
			row := map[string]interface{}{}
			row["config_key"] = k
			row["config_value"] = v
			row["source_file_name"] = sourceFileName
			data = append(data, row)
		}
	}
	_, err := ctx.insertRows(datatableCtx, &data, "jetstore_config", workspace, token)
	if err != nil {
		err = fmt.Errorf("error: err returned from InsertRows in WriteJetStoreConfig writing jetstore_config: %v", err)
		log.Println(err)
		return err
	}

	return nil
}

// WriteRuleSequences
func (ctx *writeWorkspaceContext) WriteRuleSequences(datatableCtx *datatable.Context, workspace string, token *string) error {
	// write to rule_sequences
	returnedKeys, err := ctx.insertRows(datatableCtx, &ctx.model.RuleSequences, "rule_sequences", workspace, token)
	if returnedKeys == nil || err != nil {
		err = fmt.Errorf("error: no keys or err returned from InsertRows in WriteRuleSequences writing to rule_sequences, err is '%v'", err)
		log.Println(err)
		return err
	}
	for i := range ctx.model.RuleSequences {
		ruleSequence := &ctx.model.RuleSequences[i]
		(*ruleSequence)["db_key"] = (*returnedKeys)[i]
	}

	// write to main_rule_sets
	data := []map[string]interface{}{}
	for i := range ctx.model.RuleSequences {
		ruleSequence := &ctx.model.RuleSequences[i]
		for j, mainRuleSet := range (*ruleSequence)["main_rule_sets"].([]interface{}) {
			row := map[string]interface{}{}
			row["rule_sequence_key"] = (*ruleSequence)["db_key"]
			row["main_ruleset_name"] = mainRuleSet
			row["ruleset_file_key"] = (*ctx.sourceFileKeys)[mainRuleSet.(string)]
			row["seq"] = j
			data = append(data, row)

		}
	}
	_, err = ctx.insertRows(datatableCtx, &data, "main_rule_sets", workspace, token)
	if err != nil {
		err = fmt.Errorf("error: err returned from InsertRows in WriteRuleSequences writing main_rule_sets: %v", err)
		log.Println(err)
		return err
	}

	return nil
}

// WriteLookupTables
func (ctx *writeWorkspaceContext) WriteLookupTables(datatableCtx *datatable.Context, workspace string, token *string) error {
	// write to lookup_tables
	data := []map[string]interface{}{}
	for i := range ctx.model.LookupTables {
		v := &ctx.model.LookupTables[i]
		err := appendDataRow(v, &data)
		if err != nil {
			return err
		}
	}
	returnedKeys, err := ctx.insertRows(datatableCtx, &data, "lookup_tables", workspace, token)
	if returnedKeys == nil || err != nil {
		err = fmt.Errorf("error: no keys or err returned from InsertRows in WriteLookupTables writing to lookup_tables, err is '%v'", err)
		log.Println(err)
		return err
	}
	for i := range ctx.model.LookupTables {
		r := &ctx.model.LookupTables[i]
		r.DbKey = (*returnedKeys)[i]
	}

	// write to lookup_columns
	data = data[:0]
	for i := range ctx.model.LookupTables {
		lookupTable := &ctx.model.LookupTables[i]
		for j := range lookupTable.Columns {
			row := &lookupTable.Columns[j]
			(*row)["lookup_table_key"] = lookupTable.DbKey
			data = append(data, *row)
		}
	}
	_, err = ctx.insertRows(datatableCtx, &data, "lookup_columns", workspace, token)
	if err != nil {
		err = fmt.Errorf("error: err returned from InsertRows in WriteLookupTables writing lookup_columns: %v", err)
		log.Println(err)
		return err
	}

	return nil
}

func (ctx *writeWorkspaceContext) expr2Key(datatableCtx *datatable.Context, expr *interface{}, workspace string, token *string) (int, error) {
	if expr == nil {
		return 0, fmt.Errorf("error, nil expr argument to expr2Key")
	}
	switch vv := (*expr).(type) {
	case int, float64:
	case map[string]interface{}:
		// expr as map[string]interface{}
		switch vv["type"] {
		case "binary":
			var lhs, rhs interface{}
			var err error
			lhs = vv["lhs"]
			vv["arg0_key"], err = ctx.expr2Key(datatableCtx, &lhs, workspace, token)
			if err != nil {
				return 0, err
			}
			rhs = vv["rhs"]
			vv["arg1_key"], err = ctx.expr2Key(datatableCtx, &rhs, workspace, token)
			if err != nil {
				return 0, err
			}
		case "unary":
			var arg interface{}
			var err error
			arg = vv["arg"]
			vv["arg0_key"], err = ctx.expr2Key(datatableCtx, &arg, workspace, token)
			if err != nil {
				return 0, err
			}
		}
	default:
		return 0, fmt.Errorf("error, unknown type for expr in expr2Key:%v, type %T", vv, vv)
	}
	return ctx.persistExpr(datatableCtx, expr, workspace, token)
}

func (ctx *writeWorkspaceContext) applyDefaults2Expressions(data *[]map[string]interface{}) (*[]map[string]interface{}, error) {
	if data == nil {
		return nil, fmt.Errorf("error: data is nil in applyDefaults2Expressions")
	}
	for i := range *data {
		_, ok := (*data)[i]["arg0_key"]
		if !ok {
			(*data)[i]["arg0_key"] = -1
		}
		_, ok = (*data)[i]["arg1_key"]
		if !ok {
			(*data)[i]["arg1_key"] = -1
		}
		_, ok = (*data)[i]["arg2_key"]
		if !ok {
			(*data)[i]["arg2_key"] = -1
		}
		_, ok = (*data)[i]["arg3_key"]
		if !ok {
			(*data)[i]["arg3_key"] = -1
		}
		_, ok = (*data)[i]["arg4_key"]
		if !ok {
			(*data)[i]["arg4_key"] = -1
		}
		_, ok = (*data)[i]["arg5_key"]
		if !ok {
			(*data)[i]["arg5_key"] = -1
		}
		_, ok = (*data)[i]["op"]
		if !ok {
			(*data)[i]["op"] = ""
		}
		(*data)[i]["source_file_key"] = (*ctx.sourceFileKeys)[ctx.model.MainRuleFileName]
	}
	return data, nil
}

func (ctx *writeWorkspaceContext) persistExpr(datatableCtx *datatable.Context, expr *interface{}, workspace string, token *string) (int, error) {
	if expr == nil {
		return 0, fmt.Errorf("error, nil expr argument to persistExpr")
	}
	data := []map[string]interface{}{}
	switch vv := (*expr).(type) {
	case int:
		// convert the resource key into the db_key
		data = append(data, map[string]interface{}{
			"type":     "resource",
			"arg0_key": ctx.resourcesMap[vv], // the resource db_key
		})
	case float64:
		// convert the resource key into the db_key
		data = append(data, map[string]interface{}{
			"type":     "resource",
			"arg0_key": ctx.resourcesMap[int(vv)], // the resource db_key
		})
	case map[string]interface{}:
		data = append(data, vv)
	}
	dataWithDefaults, err := ctx.applyDefaults2Expressions(&data)
	if err != nil {
		return 0, fmt.Errorf("bug, unexpected error: %v", err)
	}
	returnedKeys, err := ctx.insertRows(datatableCtx, dataWithDefaults, "expressions", workspace, token)
	if returnedKeys == nil || err != nil {
		err = fmt.Errorf("error: no keys or err returned from InsertRows in persistExpr writing to expressions, err is '%v'", err)
		log.Println(err)
		return 0, err
	}
	return (*returnedKeys)[0], nil
}

// WriteExpr
func (ctx *writeWorkspaceContext) WriteExpr(datatableCtx *datatable.Context, workspace string, token *string) error {
	// write to expressions
	var err error
	var expr interface{}
	for i := range ctx.model.ReteNodes {
		v := &ctx.model.ReteNodes[i]
		if v.Filter != nil {
			expr = v.Filter
			v.Filter["db_key"], err = ctx.expr2Key(datatableCtx, &expr, workspace, token)
			if err != nil {
				return err
			}
		}
		if v.ObjectExpr != nil {
			expr = v.ObjectExpr
			v.ObjectExpr["db_key"], err = ctx.expr2Key(datatableCtx, &expr, workspace, token)
			if err != nil {
				return err
			}
		}
	}
	return nil
}

func minSalience(v []int) int {

	m := 9999
	for _,i := range v {
		if i < m {
			m = i
		}
	}
	return m
}
// WriteReteNodes
func (ctx *writeWorkspaceContext) WriteReteNodes(datatableCtx *datatable.Context, workspace string, token *string) error {
	// write to rete_nodes from RuleTerm struct
	sourceFileName := ctx.model.JetstoreConfig["source_file_name"]
	data := []map[string]interface{}{}
	for i := range ctx.model.ReteNodes {
		v := &ctx.model.ReteNodes[i]
		row := map[string]interface{}{
			"vertex":               v.Vertex,
			"type":                 v.Type,
			"subject_key":          v.SubjectKey,
			"predicate_key":        v.PredicateKey,
			"object_key":           v.ObjectKey,
			"parent_vertex":        v.ParentVertex,
			"normalized_label":     v.NormalizedLabel,
			"consequent_seq":       v.ConsequentSeq,
			"source_file_name":     sourceFileName,
		}
		if v.ObjectExpr != nil {
			row["obj_expr_key"] = v.ObjectExpr["db_key"]
		}
		if v.Filter != nil {
			row["filter_expr_key"] = v.Filter["db_key"]
		}
		if v.Salience != nil {
			row["salience"] = minSalience(v.Salience)
		}
		if v.IsNot {
			row["is_negation"] = 1
		} else {
			if v.Type == "antecedent" {
				row["is_negation"] = 0
			}
		}
		data = append(data, row)

		// keep a map of the ReteNode for JetRules to reference them
		ctx.reteNodeMap[fmt.Sprintf("%d|%d", v.Vertex, v.ConsequentSeq)] = v
	}
	returnedKeys, err := ctx.insertRows(datatableCtx, &data, "rete_nodes", workspace, token)
	if returnedKeys == nil || err != nil {
		err = fmt.Errorf("error: no keys or err returned from InsertRows in WriteReteNodes writing to rete_nodes, err is '%v'", err)
		log.Println(err)
		return err
	}
	for i := range ctx.model.ReteNodes {
		r := &ctx.model.ReteNodes[i]
		r.DbKey = (*returnedKeys)[i]
	}

	// write to beta_row_config
	data = []map[string]interface{}{}
	for i := range ctx.model.ReteNodes {
		v := &ctx.model.ReteNodes[i]
		for j := range v.BetaVarNodes {
			bn := &v.BetaVarNodes[j]
			row := map[string]interface{}{
				"vertex":               bn.Vertex,
				"seq":                  j,
				"row_pos":              bn.VarPos,
				"id":                   bn.Id,
				"source_file_name":     sourceFileName,
			}
			if bn.IsBinded {
				row["is_binded"] = 1
			} else {
				row["is_binded"] = 0
			}
			data = append(data, row)
		}
	}
	_, err = ctx.insertRows(datatableCtx, &data, "beta_row_config", workspace, token)
	if err != nil {
		err = fmt.Errorf("error: err returned from InsertRows in WriteReteNodes writing to beta_row_config, err is '%v'", err)
		log.Println(err)
		return err
	}

	return nil
}
// WriteJetRules
func (ctx *writeWorkspaceContext) WriteJetRules(datatableCtx *datatable.Context, workspace string, token *string) error {
	// write to jet_rules
	data := []map[string]interface{}{}
	for i := range ctx.model.Jetrules {
		v := &ctx.model.Jetrules[i]
		row := map[string]interface{}{
			"name":                 v.Name,
			"salience":             v.Salience,
			"optimization":         v.Optimization,
			"authored_label":       v.AuthoredLabel,
			"normalized_label":     v.NormalizedLabel,
			"label":                v.Label,
			"source_file_name":     v.SourceFileName,
		}
		data = append(data, row)
	}
	returnedKeys, err := ctx.insertRows(datatableCtx, &data, "jet_rules", workspace, token)
	if returnedKeys == nil || err != nil {
		err = fmt.Errorf("error: no keys or err returned from InsertRows in WriteJetRules writing to jet_rules, err is '%v'", err)
		log.Println(err)
		return err
	}
	for i := range ctx.model.Jetrules {
		r := &ctx.model.Jetrules[i]
		r.DbKey = (*returnedKeys)[i]
	}

	// write to rule_properties
	data = []map[string]interface{}{}
	for i := range ctx.model.Jetrules {
		v := &ctx.model.Jetrules[i]
		for pname,pvalue := range v.Properties {
			row := map[string]interface{}{
				"rule_key":    v.DbKey,
				"name":        pname,
				"value":       pvalue,
			}
			data = append(data, row)
		}
	}
	_, err = ctx.insertRows(datatableCtx, &data, "rule_properties", workspace, token)
	if err != nil {
		err = fmt.Errorf("error: err returned from InsertRows in WriteJetRules writing to rule_properties, err is '%v'", err)
		log.Println(err)
		return err
	}

	// write to rule_terms
	data = []map[string]interface{}{}
	for i := range ctx.model.Jetrules {
		v := &ctx.model.Jetrules[i]
		for j := range v.Antecedents {
			rt := &v.Antecedents[j]
			ruleTerm, ok := ctx.reteNodeMap[fmt.Sprintf("%d|%d", rt.Vertex, rt.ConsequentSeq)]
			if !ok {
				err = fmt.Errorf("bug: RuleTerm with 'vertex|consequent_seq' of '%d|%d' not found", rt.Vertex, rt.ConsequentSeq)
				log.Println(err)
				return err
			}
			row := map[string]interface{}{
				"rule_key":        v.DbKey,
				"rete_node_key":   ruleTerm.DbKey,
				"is_antecedent":   true,
			}
			data = append(data, row)
		}
		for j := range v.Consequents {
			rt := &v.Consequents[j]
			ruleTerm, ok := ctx.reteNodeMap[fmt.Sprintf("%d|%d", rt.Vertex, rt.ConsequentSeq)]
			if !ok {
				err = fmt.Errorf("bug: RuleTerm with 'vertex|consequent_seq' of '%d|%d' not found", rt.Vertex, rt.ConsequentSeq)
				log.Println(err)
				return err
			}
			row := map[string]interface{}{
				"rule_key":        v.DbKey,
				"rete_node_key":   ruleTerm.DbKey,
				"is_antecedent":   false,
			}
			data = append(data, row)
		}
	}
	_, err = ctx.insertRows(datatableCtx, &data, "rule_terms", workspace, token)
	if err != nil {
		err = fmt.Errorf("error: err returned from InsertRows in WriteJetRules writing to rule_terms, err is '%v'", err)
		log.Println(err)
		return err
	}

	return nil
}
// WriteTriples
func (ctx *writeWorkspaceContext) WriteTriples(datatableCtx *datatable.Context, workspace string, token *string) error {
	// write to triples
	data := []map[string]interface{}{}
	for i := range ctx.model.Triples {
		v := &ctx.model.Triples[i]
		s, ok := ctx.resourcesMap[v.SubjectKey]
		if !ok {
			err := fmt.Errorf("error: subject_key not found in resourceMap")
			log.Println(err)
			return err	
		}
		p, ok := ctx.resourcesMap[v.PredicateKey]
		if !ok {
			err := fmt.Errorf("error: predicate_key not found in resourceMap")
			log.Println(err)
			return err	
		}
		o, ok := ctx.resourcesMap[v.ObjectKey]
		if !ok {
			err := fmt.Errorf("error: object_key not found in resourceMap")
			log.Println(err)
			return err	
		}
		row := map[string]interface{}{
			"subject_key":          s,
			"predicate_key":        p,
			"object_key":           o,
			"source_file_name":     v.SourceFileName,
		}
		data = append(data, row)
	}
	_, err := ctx.insertRows(datatableCtx, &data, "triples", workspace, token)
	if err != nil {
		err = fmt.Errorf("error: err returned from InsertRows in WriteTriples writing to triples, err is '%v'", err)
		log.Println(err)
		return err
	}

	return nil
}
