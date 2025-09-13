package rete

// Data model for ReteMetaStore / ReteMetaStoreFactory

type JetruleModel struct {
	MainRuleFileName string `json:"main_rule_file_name"`
	// SupportRuleFileNames []string                 `json:"support_rule_file_names"`
	CompilerDirectives map[string]string `json:"compiler_directives,omitempty"`
	Resources          []ResourceNode    `json:"resources,omitempty"`
	LookupTables       []LookupTableNode `json:"lookup_tables,omitempty"`
	Jetrules           []JetruleNode     `json:"jet_rules,omitempty"`
	ReteNodes          []RuleTerm        `json:"rete_nodes,omitempty"`
	// Imports              map[string][]string      `json:"imports"`
	JetstoreConfig map[string]string `json:"jetstore_config,omitempty"`
	RuleSequences  []RuleSequence    `json:"rule_sequences,omitempty"`
	Classes        []ClassNode       `json:"classes,omitempty"`
	Tables         []TableNode       `json:"tables,omitempty"`
	Triples        []TripleNode      `json:"triples,omitempty"`
	HeadRuleTerm   *RuleTerm         `json:"head_rule_term,omitzero"`
	Antecedents    []*RuleTerm       `json:"antecedents,omitempty"`
	Consequents    []*RuleTerm       `json:"consequents,omitempty"`
}

func NewJetruleModel() *JetruleModel {
	return &JetruleModel{
		CompilerDirectives: make(map[string]string),
		Resources:          []ResourceNode{},
		LookupTables:       []LookupTableNode{},
		Jetrules:           []JetruleNode{},
		ReteNodes:          []RuleTerm{},
		JetstoreConfig:     make(map[string]string),
		RuleSequences:      []RuleSequence{},
		Classes:            []ClassNode{},
		Tables:             []TableNode{},
		Triples:            []TripleNode{},
		Antecedents:        []*RuleTerm{},
		Consequents:        []*RuleTerm{},
	}
}

type ResourceNode struct {
	Id             string `json:"id,omitempty"`
	Inline         bool   `json:"inline,omitzero"`
	IsAntecedent   bool   `json:"is_antecedent,omitzero"`
	IsBinded       bool   `json:"is_binded,omitzero"`
	Key            int    `json:"key,omitzero"`
	SourceFileName string `json:"source_file_name,omitempty"`
	Type           string `json:"type,omitempty"`
	Value          string `json:"value,omitempty"`
	VarPos         int    `json:"var_pos,omitzero"`
	Vertex         int    `json:"vertex,omitzero"`
}

type LookupTableNode struct {
	Columns        []LookupTableColumn  `json:"columns,omitempty"`
	DataInfo       *LookupTableDataInfo `json:"data_file_info,omitzero"`
	CsvFile        string               `json:"csv_file,omitempty"`
	Key            []string             `json:"key,omitempty"`
	Name           string               `json:"name,omitempty"`
	Resources      []string             `json:"resources,omitempty"`
	SourceFileName string               `json:"source_file_name,omitempty"`
	Type           string               `json:"type,omitempty"`
}

type LookupTableColumn struct {
	Name    string `json:"name,omitempty"`
	Type    string `json:"type,omitempty"`
	IsArray bool   `json:"as_array,omitzero"`
}

// LookupTableDataInfo contain the information of the lookup table data.
// Originally this was the `lookup.db` and is a sqlite3 file.
// Which is the default when data_file_info is not specified.
// Other option will be in a jetstore binary format (TODO).
type LookupTableDataInfo struct {
	DbFileName    string `json:"db_file_name,omitempty"`
	Format        string `json:"format,omitempty"`
	IndexFileName string `json:"index_file_name,omitempty"`
}

// JetruleNode provides a rule view of the rete network
type JetruleNode struct {
	Name            string            `json:"name,omitempty"`
	Properties      map[string]string `json:"properties,omitempty"`
	Optimization    bool              `json:"optimization,omitzero"`
	Salience        int               `json:"salience,omitzero"`
	Antecedents     []RuleTerm        `json:"antecedents,omitempty"`
	Consequents     []RuleTerm        `json:"consequents,omitempty"`
	AuthoredLabel   string            `json:"authoredLabel,omitempty"`
	SourceFileName  string            `json:"source_file_name,omitempty"`
	NormalizedLabel string            `json:"normalizedLabel,omitempty"`
	Label           string            `json:"label,omitempty"`
}

// RulTerm type is either antecedent or consequent
type RuleTerm struct {
	Type               string          `json:"type,omitempty"`
	IsNot              bool            `json:"isNot,omitzero"`
	NormalizedLabel    string          `json:"normalizedLabel,omitempty"`
	Vertex             int             `json:"vertex,omitzero"`
	ParentVertex       int             `json:"parent_vertex,omitzero"`
	BetaRelationVars   []string        `json:"beta_relation_vars,omitempty"`
	PrunedVars         []string        `json:"pruned_var,omitempty"`
	BetaVarNodes       []BetaVarNode   `json:"beta_var_nodes,omitempty"`
	ChildrenVertexes   []int           `json:"children_vertexes,omitempty"`
	Rules              []string        `json:"rules,omitempty"`
	Salience           []int           `json:"salience,omitempty"`
	ConsequentSeq      int             `json:"consequent_seq,omitzero"`
	ConsequentForRule  string          `json:"consequent_for_rule,omitempty"`
	ConsequentSalience int             `json:"consequent_salience,omitzero"`
	SubjectKey         int             `json:"subject_key,omitzero"`
	PredicateKey       int             `json:"predicate_key,omitzero"`
	ObjectKey          int             `json:"object_key,omitzero"`
	ObjectExpr         *ExpressionNode `json:"obj_expr,omitempty"`
	Filter             *ExpressionNode `json:"filter,omitempty"`
}

type ExpressionNode struct {
	Type  string          `json:"type,omitempty"`
	Op    string          `json:"op,omitempty"`
	Arg   *ExpressionNode `json:"arg,omitzero"`
	Lhs   *ExpressionNode `json:"lhs,omitempty"`
	Rhs   *ExpressionNode `json:"rhs,omitempty"`
	Value *ResourceNode   `json:"value,omitempty"`
}

type BetaVarNode struct {
	Type           string `json:"type,omitempty"`
	Id             string `json:"id,omitzero,omitempty"`
	IsBinded       bool   `json:"is_binded,omitzero"`
	VarPos         int    `json:"var_pos,omitzero"`
	Vertex         int    `json:"vertex,omitzero"`
	SourceFileName string `json:"source_file_name,omitempty"`
}

type ClassNode struct {
	Type           string             `json:"type,omitempty"`
	Name           string             `json:"name,omitempty"`
	BaseClasses    []string           `json:"base_classes,omitempty"`
	DataProperties []DataPropertyNode `json:"data_properties,omitempty"`
	SourceFileName string             `json:"source_file_name,omitempty"`
	AsTable        bool               `json:"as_table,omitzero"`
	SubClasses     []string           `json:"sub_classes,omitempty"`
}

type DataPropertyNode struct {
	Type      string `json:"type,omitempty"`
	Name      string `json:"name,omitempty"`
	ClassName string `json:"class_name,omitempty"`
	AsArray   bool   `json:"as_array,omitzero"`
}

type TableNode struct {
	DomainClassKey int               `json:"domain_class_key,omitzero"`
	TableName      string            `json:"table_name,omitempty"`
	ClassName      string            `json:"class_name,omitempty"`
	Columns        []TableColumnNode `json:"columns,omitempty"`
	SourceFileName string            `json:"source_file_name,omitempty"`
}

type TableColumnNode struct {
	Type         string `json:"type,omitempty"`
	AsArray      bool   `json:"as_array,omitzero"`
	PropertyName string `json:"property_name,omitempty"`
	ColumnName   string `json:"column_name,omitempty"`
}

type TripleNode struct {
	Type           string `json:"type,omitempty"`
	SubjectKey     int    `json:"subject_key,omitzero"`
	PredicateKey   int    `json:"predicate_key,omitzero"`
	ObjectKey      int    `json:"object_key,omitzero"`
	SourceFileName string `json:"source_file_name,omitempty"`
}
