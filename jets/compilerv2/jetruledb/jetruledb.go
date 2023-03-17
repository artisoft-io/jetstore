package jetruledb

import (
	"net/http"
	"github.com/jackc/pgx/v4/pgxpool"
)

type CompileJetruleAction struct {
	Action string                   `json:"action"`
	JetruleFile string              `json:"jetruleFile"`
	UserEmail string                `json:"userEmail"`
	Data   []map[string]interface{} `json:"data"`
}

// Jetrule Domain Model
type JetruleModel struct {
	Resources         []ResourceNode    `json:"resources"`
	LookupTables      []LookupTableNode `json:"lookup_tables"`
	Jetrules          []JetruleNode     `json:"jet_rules"`
	Imports           map[string][]string `json:"imports"`
  JetstoreConfig    map[string]string  `json:"jetstore_config"`
	Classes           []ClassNode        `json:"classes"`
	Tables            []TableNode        `json:"tables"`
	Triples           []TripleNode       `json:"triples"`
}

type ResourceNode struct {
	Id                string  `json:"id"`
	Inline            bool    `json:"inline"`        
	IsAntecedent      bool    `json:"is_antecedent"` 
	IsBinded          bool    `json:"is_binded"`     
	Key               int     `json:"key"` 
	SourceFileName    string  `json:"source_file_name"`
	Type              string  `json:"type"`
	Value             string  `json:"value"` 
	VarPos            int     `json:"var_pos"`
	Vertex            int     `json:"vertex"`
}

type LookupTableNode struct {
  Columns          []ColumnNode `json:"columns"`
  CsvFile          string       `json:"csv_file"`
  Key              []string     `json:"key"`
  Name             string       `json:"name"`
  Resources        []string     `json:"resources"`
  SourceFileName   string       `json:"source_file_name"`
  Type             string       `json:"type"`
}

type ColumnNode struct {
	AsArray  string `json:"as_array"`	
	Name     string `json:"name"`
	Type     string `json:"type"`
}

type JetruleNode struct {
	Name          string `json:"name"`	
	Properties    map[string]string `json:"properties"`
	Optimization  bool   `json:"optimization"`
	Salience      int    `json:"salience"`
	Antecedents   []Antecedent `json:"antecedents"`
	Consequents   []Consequent `json:"consequents"`
	AuthoredLabel    string `json:"authoredLabel"`        
	SourceFileName   string `json:"source_file_name"`         
	NormalizedLabel  string `json:"normalizedLabel"`          
	Label	           string `json:"label"` 
}

type Antecedent struct {
	Type                string       `json:"type"`	
	IsNot               bool         `json:"isNot"`
	NormalizedLabel     string       `json:"normalizedLabel"`
	Vertex              int          `json:"vertex"`
	ParentVertex        int          `json:"parent_vertex"`
	BetaRelationVars    []string     `json:"beta_relation_vars"`
	PrunedVars          []string     `json:"pruned_var"`
	BetaVarNodes	      []BetaVarNode `json:"beta_var_nodes"`
	ChildrenVertexes    []int        `json:"children_vertexes"`
	Rules               []string     `json:"rules"`
	Salience            []int        `json:"salience"`
	SubjectKey          int          `json:"subject_key"`
	PredicateKey        int          `json:"predicate_key"`
	ObjectKey           int          `json:"object_key"`
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

type Consequent struct {
	Type               string `json:"type"`   
	NormalizedLabel    string `json:"normalizedLabel"`              
	Vertex             int    `json:"vertex"`     
	ConsequentSeq      int    `json:"consequent_seq"`             
	ConsequentForRule  string `json:"consequent_for_rule"`                  
	ConsequentSalience int    `json:"consequent_salience"`                  
	SubjectKey         int    `json:"subject_key"`          
	PredicateKey       int    `json:"predicate_key"`            
	ObjectKey	         int    `json:"object_key"`
	ObjectExpr         ObjExprNode  `json:"obj_expr"`
}

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
	DataProperties []map[string]string `json:"data_properties"`
	SourceFileName string              `json:"source_file_name"`
	AsTable        bool                `json:"as_table"`
	SubClasses     []string            `json:"sub_classes"`
}

type TableNode struct {
	Type           string              `json:"type"`
	TableName      string              `json:"table_name"`
	ClassName      string              `json:"class_name"`
	Columns        []map[string]string `json:"columns"`
	SourceFileName string              `json:"source_file_name"`
}

type TripleNode struct {
	Type           string  `json:"type"`
	SubjectKey     int     `json:"subject_key"`          
	PredicateKey   int     `json:"predicate_key"`            
	ObjectKey	     int     `json:"object_key"`
}


// Register file_key with file_key_staging table
func CompileJetrule(dbpool *pgxpool.Pool, compileJetruleAction *CompileJetruleAction, token string) (*map[string]interface{}, int, error) {
		// if err != nil {
		// 	return nil, http.StatusInternalServerError, fmt.Errorf("while calling InsertSourcePeriod: %v", err)
		// }

		return &map[string]interface{}{}, http.StatusOK, nil
}
