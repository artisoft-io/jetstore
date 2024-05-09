package compute_pipes

// This file contains the Compute Pipes configuration model
type ComputePipesConfig struct {
	MetricsConfig       *MetricsSpec   `json:"metrics_config"`
	ClusterConfig       *ClusterSpec   `json:"cluster_config"`
	OutputTables        []TableSpec    `json:"output_tables"`
	Channels            []ChannelSpec  `json:"channels"`
	Context             *[]ContextSpec `json:"context"`
	PipesConfig         []PipeSpec     `json:"pipes_config"`
	ShardingPipesConfig []PipeSpec     `json:"sharding_pipes_config"`
	ReducingPipesConfig [][]PipeSpec   `json:"reducing_pipes_config"`
}

// Config for peer2peer communication
type ClusterSpec struct {
	CpipesMode              string `json:"cpipes_mode"`
	ReadTimeout             int    `json:"read_timeout"`
	WriteTimeout            int    `json:"write_timeout"`
	PeerRegistrationTimeout int    `json:"peer_registration_timeout"`
	NbrNodes                int    `json:"nbr_nodes"`
	ShardingNbrNodes        int    `json:"sharding_nbr_nodes"`
	ReducingNbrNodes        int    `json:"reducing_nbr_nodes"`
	NbrSubClusters          int    `json:"nbr_sub_clusters"`
	NbrJetsPartitions       uint64 `json:"nbr_jets_partitions"`
	PeerBatchSize           int    `json:"peer_batch_size"`
	NodeId                  int    // calculated field
	SubClusterId            int    // calculated field
	NbrSubClusterNodes      int    // calculated field
	SubClusterNodeId        int    // calculated field
}

type MetricsSpec struct {
	ReportInterval int      `json:"report_interval_sec"`
	RuntimeMetrics []Metric `json:"runtime_metrics"`
}

type Metric struct {
	// Type range: runtime
	// Name values: alloc_mb, total_alloc_mb, sys_mb, nbr_gc
	// note: suffix _mb for units in MiB
	Type string `json:"type"`
	Name string `json:"name"`
}
type ChannelSpec struct {
	Name    string   `json:"name"`
	Columns []string `json:"columns"`
}

type ContextSpec struct {
	// Type range: file_key_component, partfile_key_component
	Type string `json:"type"`
	Key  string `json:"key"`
	Expr string `json:"expr"`
}

type TableSpec struct {
	Key     string            `json:"key"`
	Name    string            `json:"name"`
	Columns []TableColumnSpec `json:"columns"`
}

type TableColumnSpec struct {
	Name    string `json:"name"`
	RdfType string `json:"rdf_type"`
}

type PipeSpec struct {
	// Type range: fan_out, splitter, distribute_data
	Type   string               `json:"type"`
	Input  string               `json:"input"`
	Column *string              `json:"column"` // splitter column
	Apply  []TransformationSpec `json:"apply"`
}

type TransformationSpec struct {
	// Type range: map_record, aggregate, partition_writer
	Type                  string                     `json:"type"`
	PartitionSize         *int                       `json:"partition_size"`
	FilePathSubstitutions *[]PathSubstitution        `json:"file_path_substitutions"`
	StepId                *string                    `json:"step_id"`
	Columns               []TransformationColumnSpec `json:"columns"`
	DataSchema            *[]DataSchemaSpec          `json:"data_schema"`
	Output                string                     `json:"output"`
}

type PathSubstitution struct {
	Replace string `json:"replace"`
	With    string `json:"with"`
}

type DataSchemaSpec struct {
	Columns string `json:"column"`
	RdfType string `json:"rdf_type"`
}

type TransformationColumnSpec struct {
	// Type range: select, value, eval, map, hash
	// (applicable to aggregate) count, distinct_count, sum, min,
	// case, map_reduce
	Name        string                      `json:"name"`
	Type        string                      `json:"type"`
	Expr        *string                     `json:"expr"`
	MapExpr     *MapExpression              `json:"map_expr"`
	EvalExpr    *ExpressionNode             `json:"eval_expr"`
	HashExpr    *HashExpression             `json:"hash_expr"`
	Where       *ExpressionNode             `json:"where"`
	CaseExpr    []CaseExpression            `json:"case_expr"`
	ElseExpr    *ExpressionNode             `json:"else_expr"`
	MapOn       *string                     `json:"map_on"`
	ApplyMap    *[]TransformationColumnSpec `json:"apply_map"`
	ApplyReduce *[]TransformationColumnSpec `json:"apply_reduce"`
}

type HashExpression struct {
	Expr              string          `json:"expr"`
	Format            *string         `json:"format"`
	NbrJetsPartitions *uint64         `json:"nbr_jets_partitions"`
	DefaultExpr       *ExpressionNode `json:"default_expr"`
}

type MapExpression struct {
	CleansingFunction *string `json:"cleansing_function"`
	Argument          *string `json:"argument"`
	Default           *string `json:"default"`
	ErrMsg            *string `json:"err_msg"`
	RdfType           string  `json:"rdf_type"`
}

type ExpressionNode struct {
	// Type for leaf node: select, value, eval
	Type     *string         `json:"type"`
	Expr     *string         `json:"expr"`
	EvalExpr *ExpressionNode `json:"eval_expr"`
	Arg      *ExpressionNode `json:"arg"`
	Lhs      *ExpressionNode `json:"lhs"`
	Op       *string         `json:"op"`
	Rhs      *ExpressionNode `json:"rhs"`
}

type CaseExpression struct {
	When ExpressionNode `json:"when"`
	Then ExpressionNode `json:"then"`
}
