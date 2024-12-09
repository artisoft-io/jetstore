package rete

// Data model for ReteMetaStore / ReteMetaStoreFactory

type JetruleModel struct {
	MainRuleFileName string `json:"main_rule_file_name"`
	// SupportRuleFileNames []string                 `json:"support_rule_file_names"`
	Resources    []ResourceNode    `json:"resources"`
	LookupTables []LookupTableNode `json:"lookup_tables"`
	// Jetrules             []JetruleNode            `json:"jet_rules"`
	ReteNodes []RuleTerm `json:"rete_nodes"`
	// Imports              map[string][]string      `json:"imports"`
	JetstoreConfig map[string]string        `json:"jetstore_config"`
	RuleSequences  []map[string]interface{} `json:"rule_sequences"`
	Classes        []ClassNode              `json:"classes"`
	Tables         []TableNode              `json:"tables"`
	Triples        []TripleNode             `json:"triples"`
	HeadRuleTerm   *RuleTerm
	Antecedents    []*RuleTerm
	Consequents    []*RuleTerm
}

type ResourceNode struct {
	Id             string `json:"id"`
	Inline         bool   `json:"inline"`
	IsAntecedent   bool   `json:"is_antecedent"`
	IsBinded       bool   `json:"is_binded"`
	Key            int    `json:"key"`
	SourceFileName string `json:"source_file_name"`
	Type           string `json:"type"`
	Value          string `json:"value"`
	VarPos         int    `json:"var_pos"`
	Vertex         int    `json:"vertex"`
}

type LookupTableNode struct {
	Columns        []LookupTableColumn  `json:"columns"`
	DataInfo       *LookupTableDataInfo `json:"data_file_info"`
	CsvFile        string               `json:"csv_file"`
	Key            []string             `json:"key"`
	Name           string               `json:"name"`
	Resources      []string             `json:"resources"`
	SourceFileName string               `json:"source_file_name"`
	Type           string               `json:"type"`
}

type LookupTableColumn struct {
	Name    string `json:"name"`
	Type    string `json:"type"`
	IsArray bool   `json:"as_array"`
}

// LookupTableDataInfo contain the information of the lookup table data.
// Originally this was the `lookup.db` and is a sqlite3 file.
// Which is the default when data_file_info is not specified.
// Other option will be in a jetstore binary format (TODO).
type LookupTableDataInfo struct {
	DbFileName    string `json:"db_file_name"`
	Format        string `json:"format"`
	IndexFileName string `json:"index_file_name"`
}

// JetruleNode provides a rule view of the rete network
type JetruleNode struct {
	Name            string            `json:"name"`
	Properties      map[string]string `json:"properties"`
	Optimization    bool              `json:"optimization"`
	Salience        int               `json:"salience"`
	Antecedents     []RuleTerm        `json:"antecedents"`
	Consequents     []RuleTerm        `json:"consequents"`
	AuthoredLabel   string            `json:"authoredLabel"`
	SourceFileName  string            `json:"source_file_name"`
	NormalizedLabel string            `json:"normalizedLabel"`
	Label           string            `json:"label"`
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
}

type BetaVarNode struct {
	Type           string `json:"type"`
	Id             string `json:"id"`
	IsBinded       bool   `json:"is_binded"`
	VarPos         int    `json:"var_pos"`
	Vertex         int    `json:"vertex"`
	SourceFileName string `json:"source_file_name"`
}

type ClassNode struct {
	Type           string             `json:"type"`
	Name           string             `json:"name"`
	BaseClasses    []string           `json:"base_classes"`
	DataProperties []DataPropertyNode `json:"data_properties"`
	SourceFileName string             `json:"source_file_name"`
	AsTable        bool               `json:"as_table"`
	SubClasses     []string           `json:"sub_classes"`
}

type DataPropertyNode struct {
	Type      string `json:"type"`
	Name      string `json:"name"`
	ClassName string `json:"class_name"`
	AsArray   bool   `json:"as_array"`
}

type TableNode struct {
	DomainClassKey int               `json:"domain_class_key"`
	TableName      string            `json:"table_name"`
	ClassName      string            `json:"class_name"`
	Columns        []TableColumnNode `json:"columns"`
	SourceFileName string            `json:"source_file_name"`
}

type TableColumnNode struct {
	Type         string `json:"type"`
	AsArray      bool   `json:"as_array"`
	PropertyName string `json:"property_name"`
	ColumnName   string `json:"column_name"`
}

type TripleNode struct {
	Type           string `json:"type"`
	SubjectKey     int    `json:"subject_key"`
	PredicateKey   int    `json:"predicate_key"`
	ObjectKey      int    `json:"object_key"`
	SourceFileName string `json:"source_file_name"`
}
