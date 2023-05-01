package jetruledb

import (
	// "context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"os"

	// "github.com/artisoft-io/jetstore/jets/awsi"
	"github.com/artisoft-io/jetstore/jets/datatable"
	"github.com/jackc/pgx/v4/pgxpool"
)

type CompileJetruleAction struct {
	Action        string                   `json:"action"`
	JetruleFile   string                   `json:"jetruleFile"`
	UserEmail     string                   `json:"userEmail"`
	Workspace     string                   `json:"workspace"`
	DropTables    bool                     `json:"drop_tables"`
	UpdateSchema  bool                     `json:"update_schema"`
	Data          []map[string]interface{} `json:"data"`
}

// Jetrule Domain Model
type JetruleModel struct {
	MainRuleFileName     string              `json:"main_rule_file_name"`
	SupportRuleFileNames [] string           `json:"support_rule_file_names"`
	Resources            []ResourceNode      `json:"resources"`
	LookupTables         []LookupTableNode   `json:"lookup_tables"`
	Jetrules             []JetruleNode       `json:"jet_rules"`
	ReteNodes            []RuleTerm          `json:"rete_nodes"`
	Imports              map[string][]string `json:"imports"`
  JetstoreConfig       map[string]string   `json:"jetstore_config"`
	Classes              []ClassNode         `json:"classes"`
	Tables               []TableNode         `json:"tables"`
	Triples              []TripleNode        `json:"triples"`
} 

type ResourceNode struct {
	Id                string  `json:"id"`
	Inline            bool    `json:"inline"`        
	IsAntecedent      bool    `json:"is_antecedent"` 
	IsBinded          bool    `json:"is_binded"`     
	Key               int     `json:"key"` 
	DbKey             int     `json:"db_key"` 
	SourceFileName    string  `json:"source_file_name"`
	Type              string  `json:"type"`
	Value             string  `json:"value"` 
	VarPos            int     `json:"var_pos"`
	Vertex            int     `json:"vertex"`
}

type LookupTableNode struct {
  Columns          []LookupColumnNode `json:"columns"`
  CsvFile          string             `json:"csv_file"`
  Key              []string           `json:"key"`
  Name             string             `json:"name"`
  Resources        []string           `json:"resources"`
  SourceFileName   string             `json:"source_file_name"`
  Type             string             `json:"type"`
}

type LookupColumnNode struct {
	Type     string `json:"type"`
	Name     string `json:"name"`
	AsArray  bool   `json:"as_array"`	
}

type JetruleNode struct {
	Name          string            `json:"name"`	
	Properties    map[string]string `json:"properties"`
	Optimization  bool              `json:"optimization"`
	Salience      int               `json:"salience"`
	Antecedents   []RuleTerm        `json:"antecedents"`
	Consequents   []RuleTerm        `json:"consequents"`
	AuthoredLabel    string         `json:"authoredLabel"`        
	SourceFileName   string         `json:"source_file_name"`         
	NormalizedLabel  string         `json:"normalizedLabel"`          
	Label	           string         `json:"label"` 
}

// RulTerm type is either antecedent or consequent
type RuleTerm struct {
	Type                string        `json:"type"`	
	IsNot               bool          `json:"isNot"`
	NormalizedLabel     string        `json:"normalizedLabel"`
	Vertex              int           `json:"vertex"`
	ParentVertex        int           `json:"parent_vertex"`
	BetaRelationVars    []string      `json:"beta_relation_vars"`
	PrunedVars          []string      `json:"pruned_var"`
	BetaVarNodes	      []BetaVarNode `json:"beta_var_nodes"`
	ChildrenVertexes    []int         `json:"children_vertexes"`
	Rules               []string      `json:"rules"`
	Salience            []int         `json:"salience"`
	ConsequentSeq       int           `json:"consequent_seq"`             
	ConsequentForRule   string        `json:"consequent_for_rule"`                  
	ConsequentSalience  int           `json:"consequent_salience"`                  
	SubjectKey          int           `json:"subject_key"`          
	PredicateKey        int           `json:"predicate_key"`            
	ObjectKey	          int           `json:"object_key"`
	ObjectExpr          ObjExprNode   `json:"obj_expr"`
}

type BetaVarNode struct {
	Type           string `json:"type"`       
	Id             string `json:"id"`       
	IsBinded       bool   `json:"is_binded"`     
	VarPos         int    `json:"var_pos"`    
	Vertex         int    `json:"vertex"`    
	Key            int    `json:"key"`    
	SourceFileName string `json:"source_file_name"`       
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

type ObjExprNode struct {
	Type string      `json:"type"`
	Lhs  interface{} `json:"lhs"`
	Op   string      `json:"op"`
	Rhs  interface{} `json:"rhs"`
	Arg  interface{} `json:"arg"`
}

type ClassNode struct {
	Type           string              `json:"type"`
	Name           string              `json:"name"`
	BaseClasses    []string            `json:"base_classes"`
	DataProperties []DataPropertyNode  `json:"data_properties"`
	SourceFileName string              `json:"source_file_name"`
	AsTable        bool                `json:"as_table"`
	SubClasses     []string            `json:"sub_classes"`
	DbKey          int                 `json:"db_key"` 
}

type DataPropertyNode struct {
	Type           string              `json:"type"`
	DomainClassKey int                 `json:"domain_class_key"`
	Name           string              `json:"name"`
	AsArray        bool                `json:"as_array"`
	DbKey          int                 `json:"db_key"` 
}

type TableNode struct {
	DomainClassKey int                 `json:"domain_class_key"`
	TableName      string              `json:"table_name"`
	ClassName      string              `json:"class_name"`
	Columns        []TableColumnNode   `json:"columns"`
	SourceFileName string              `json:"source_file_name"`
	DbKey          int                 `json:"db_key"` 
}

type TableColumnNode struct {
	Type           string    `json:"type"`
	AsArray        bool      `json:"as_array"`          
	PropertyName   string    `json:"property_name"`
	ColumnName     string    `json:"column_name"`
}

type TripleNode struct {
	Type           string  `json:"type"`
	SubjectKey     int     `json:"subject_key"`          
	PredicateKey   int     `json:"predicate_key"`            
	ObjectKey	     int     `json:"object_key"`
}


func CompileJetrule(dbpool *pgxpool.Pool, compileJetruleAction *CompileJetruleAction, token string) (*map[string]interface{}, int, error) {
	return nil, 0, errors.New("TODO: NOT IMPLEMENTED YET")
}

// local context for writing domain model, used within WriteJetrule
type writeWorkspaceContext struct {
	model *JetruleModel
	sourceFileKeys *map[string]int
	domainClassMap map[string]*ClassNode
	dataPropertyMap map[string]*DataPropertyNode
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
		model: &JetruleModel{},
		sourceFileKeys: &map[string]int{},
		domainClassMap: map[string]*ClassNode{},
	}

	log.Println("ReadFile:",compileJetruleAction.JetruleFile)
	file, err := os.ReadFile(compileJetruleAction.JetruleFile)
	if err != nil {
		log.Printf("while reading json file:%v\n",err)
		return &map[string]interface{}{}, http.StatusBadRequest,err		
	}
 
	err = json.Unmarshal(file, writeWorkspaceCtx.model)
	if err != nil {
		log.Printf("while unmarshaling json:%v\n",err)
		return &map[string]interface{}{}, http.StatusBadRequest,err		
	}
	// //*
	// fmt.Println("GOT",writeWorkspaceCtx.model)

	// Persist the Resources
	if len(writeWorkspaceCtx.model.Resources) > 0 {
		log.Println("Writing Resources")
		writeWorkspaceCtx.WriteResources(datatableCtx, compileJetruleAction.Workspace, &token)	
	}

	// Persist the Classes & Tables
	if len(writeWorkspaceCtx.model.Classes) > 0 {
		log.Println("Writing Domain Classes")
		writeWorkspaceCtx.WriteDomainClasses(datatableCtx, compileJetruleAction.Workspace, &token)
		// init the map of domain class
		for i := range writeWorkspaceCtx.model.Classes {
			cls := &writeWorkspaceCtx.model.Classes[i]
			writeWorkspaceCtx.domainClassMap[cls.Name] = cls
		}
	}
	if len(writeWorkspaceCtx.model.Tables) > 0 {
		log.Println("Writing Domain Tables")
		writeWorkspaceCtx.WriteDomainTables(datatableCtx, compileJetruleAction.Workspace, &token)
	}

	//* DEV
	// Write back as json
	b, err := json.MarshalIndent(writeWorkspaceCtx.model, "", "  ")
	if err != nil {
		log.Printf("while writing json:%v\n",err)
		return &map[string]interface{}{}, http.StatusBadRequest,err		
	}
	os.WriteFile("out.jrcc.json", b, 0666)

	return &map[string]interface{}{}, http.StatusOK, nil
}

// utility function
func (ctx *writeWorkspaceContext)insertRows(datatableCtx *datatable.Context, data *[]map[string]interface{}, table string, workspace string, token *string) (returnedKeys *[]int, err error) {
	// check if data has key 'source_file_name' and convert it into 'source_file_key'
	if table != "workspace_control" {
		for i := range *data {
			s := (*data)[i]["source_file_name"]
			if s != nil {
				sourceFileName := s.(string)
				skey := (*ctx.sourceFileKeys)[sourceFileName]
				if skey == 0 {
					keys, err2 := ctx.insertRows(datatableCtx, &[]map[string]interface{}{
						{"source_file_name": sourceFileName, "is_main": false},
					}, "workspace_control", workspace, token)
					if keys == nil || err2 != nil {
						err = fmt.Errorf("error: no keys or err returned from InsertRows in insertRows (for workspace_control table), err is '%v'", err2)
						log.Println(err)
						return
					}
					skey = (*keys)[0]
					// save the key in the context
					(*ctx.sourceFileKeys)[sourceFileName] = skey
				}
				(*data)[i]["source_file_key"] = skey			
			}
		}	
	}
	dataTableAction := &datatable.DataTableAction{
		Action:      "insert_rows",
		FromClauses: []datatable.FromClause{{Schema: workspace, Table: fmt.Sprintf("WORKSPACE/%s", table)}},
		Data: *data,
	}

	results, _, err := datatableCtx.InsertRows(dataTableAction, *token)
	if err != nil {
		log.Printf("while calling InsertRows:%v\n",err)
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
		log.Printf("while writing json:%v\n",err)
		return err
	}
	row := map[string]interface{}{}
	err = json.Unmarshal(b, &row)
	if err != nil {
		log.Printf("while reading json:%v\n",err)
		return err
	}
	*data = append(*data, row)
	return nil
}

// JetruleModel Methods
// --------------------
// WriteResources
func (ctx *writeWorkspaceContext)WriteResources(datatableCtx *datatable.Context, workspace string, token *string) error {

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
	}

	return nil
}

func (ctx *writeWorkspaceContext)getClass(name *string) *ClassNode {
	switch {
	case name == nil:
		return nil
	case *name == "owl:Thing":
		return &ClassNode{
			Name: "owl:Thing",
			BaseClasses: []string{},
			DataProperties: []DataPropertyNode{},
			AsTable: false,
			SubClasses: []string{},
			DbKey: 1,
		}
	default:
		return ctx.domainClassMap[*name]
	}
}

func (ctx *writeWorkspaceContext)getDataPropertyKey(name *string) int {
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
func (ctx *writeWorkspaceContext)WriteDomainClasses(datatableCtx *datatable.Context, workspace string, token *string) error {
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
func (ctx *writeWorkspaceContext)WriteDomainTables(datatableCtx *datatable.Context, workspace string, token *string) error {
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
