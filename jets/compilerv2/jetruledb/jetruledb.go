package jetruledb

import (
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"os"
	"strings"

	"github.com/artisoft-io/jetstore/jets/datatable"
	"github.com/artisoft-io/jetstore/jets/schema"
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
	Name           string              `json:"name"`
	AsArray        bool                `json:"as_array"`
	DbKey          int                 `json:"db_key"` 
}

type TableNode struct {
	Type           string              `json:"type"`
	TableName      string              `json:"table_name"`
	ClassName      string              `json:"class_name"`
	Columns        []TableColumnNode   `json:"columns"`
	SourceFileName string              `json:"source_file_name"`
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


// Persist Jetrule json structure to database
func WriteJetrule(dbpool *pgxpool.Pool, compileJetruleAction *CompileJetruleAction, token string) (*map[string]interface{}, int, error) {
		// if err != nil {
		// 	return nil, http.StatusInternalServerError, fmt.Errorf("while calling InsertSourcePeriod: %v", err)
		// }
	ctx := &datatable.Context{
		Dbpool: dbpool,
	}
	if compileJetruleAction.UpdateSchema {
		// Update / Create the jetrule schema, table schema name is workspace name
		err := UpdateSchema(ctx, compileJetruleAction.DropTables, compileJetruleAction.Workspace)
		if err != nil {
			log.Printf("while updating jetrule schema for workspace %s: %v\n", compileJetruleAction.Workspace, err)
			return &map[string]interface{}{}, http.StatusBadRequest,err		
		}
	}

	fmt.Println("*** ReadFile:",compileJetruleAction.JetruleFile)
	file, err := os.ReadFile(compileJetruleAction.JetruleFile)
	if err != nil {
		log.Printf("while reading json file:%v\n",err)
		return &map[string]interface{}{}, http.StatusBadRequest,err		
	}
 
	data := JetruleModel{}
	err = json.Unmarshal(file, &data)
	if err != nil {
		log.Printf("while unmarshaling json:%v\n",err)
		return &map[string]interface{}{}, http.StatusBadRequest,err		
	}

	// Persist the Resources
	data.WriteResources(ctx, compileJetruleAction.Workspace, &token)

	//* DEV
	// Write back as json
	b, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		log.Printf("while writing json:%v\n",err)
		return &map[string]interface{}{}, http.StatusBadRequest,err		
	}
	os.WriteFile("out.jrcc.json", b, 0666)

	return &map[string]interface{}{}, http.StatusOK, nil
}

func UpdateSchema(ctx *datatable.Context, dropTables bool, workspace string) error {
	// read jetrule schema definition using schema in json from location specified by env var
	schemaFname := os.Getenv("JETS_RULES_SCHEMA_FILE")
	if len(schemaFname) == 0 {
		schemaFname = "workspace_schema.json"
	}
	// read json file
	fmt.Println("*** Read Schema File:",schemaFname)
	file, _ := os.ReadFile(schemaFname)

	// Inject the workspace name as the table schema name
	jetrule := strings.ReplaceAll(string(file), "$SCHEMA", workspace)
 
	// Un-marshal the schema
	schemaDef := &[]schema.TableDefinition{}
	err := json.Unmarshal([]byte(jetrule), schemaDef)
	if err != nil {
		log.Printf("while reading json:%v\n",err)
		return err		
	}

	for i := range *schemaDef {
		fmt.Println("-- Got schema for",(*schemaDef)[i].SchemaName,".",(*schemaDef)[i].TableName)
		err = (*schemaDef)[i].UpdateTableSchema(ctx.Dbpool, dropTables)
		if err != nil {
			return fmt.Errorf("error while jetrule schema: %v", err)
		}
	}
	return nil
}

// JetruleModel Methods
// --------------------
// WriteResources
func (model *JetruleModel)WriteResources(ctx *datatable.Context, workspace string, token *string) error {

	data := []map[string]interface{}{}
	for i := range model.Resources {
		r := &model.Resources[i]
		b, err := json.Marshal(r)
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
		data = append(data, row)
	}
	dataTableAction := &datatable.DataTableAction{
		Action:      "insert_rows",
		FromClauses: []datatable.FromClause{{Schema: workspace, Table: "WORKSPACE/resources"}},
		Data: data,
	}		
	results, _, err := ctx.InsertRows(dataTableAction, *token)
	if err != nil {
		log.Printf("while calling InsertRows:%v\n",err)
		return err
	}
	returnedKeys := (*results)["returned_keys"].(*[]int)
	if returnedKeys == nil {
		err = fmt.Errorf("error: no keys returned from InsertRows in WriteResources")
		log.Println(err)
		return err
	}
	for i := range model.Resources {
		r := &model.Resources[i]
		r.DbKey = (*returnedKeys)[i]
	}

	return nil
}

// WriteDomainClasses
func (model *JetruleModel)WriteDomainClasses(ctx *datatable.Context, workspace string, token *string) error {

	//*TODO
	data := []map[string]interface{}{}
	for i := range model.Resources {
		r := &model.Resources[i]
		b, err := json.Marshal(r)
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
		data = append(data, row)
	}
	dataTableAction := &datatable.DataTableAction{
		Action:      "insert_rows",
		FromClauses: []datatable.FromClause{{Schema: workspace, Table: "WORKSPACE/resources"}},
		Data: data,
	}		
	results, _, err := ctx.InsertRows(dataTableAction, *token)
	if err != nil {
		log.Printf("while calling InsertRows:%v\n",err)
		return err
	}
	returnedKeys := (*results)["returned_keys"].(*[]int)
	if returnedKeys == nil {
		err = fmt.Errorf("error: no keys returned from InsertRows in WriteResources")
		log.Println(err)
		return err
	}
	for i := range model.Resources {
		r := &model.Resources[i]
		r.DbKey = (*returnedKeys)[i]
	}

	return nil
}
