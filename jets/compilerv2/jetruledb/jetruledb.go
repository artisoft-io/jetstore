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


// Register file_key with file_key_staging table
func CompileJetrule(dbpool *pgxpool.Pool, compileJetruleAction *CompileJetruleAction, token string) (*map[string]interface{}, int, error) {
		// if err != nil {
		// 	return nil, http.StatusInternalServerError, fmt.Errorf("while calling InsertSourcePeriod: %v", err)
		// }

		return &map[string]interface{}{}, http.StatusOK, nil
}
