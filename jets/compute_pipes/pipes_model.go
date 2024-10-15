package compute_pipes

import "regexp"

// This file contains the Compute Pipes configuration model
type ComputePipesConfig struct {
	CommonRuntimeArgs   *ComputePipesCommonArgs `json:"common_runtime_args"`
	MetricsConfig       *MetricsSpec            `json:"metrics_config"`
	ClusterConfig       *ClusterSpec            `json:"cluster_config"`
	OutputTables        []*TableSpec            `json:"output_tables"`
	OutputFiles         []OutputFileSpec        `json:"output_files"`
	LookupTables        []*LookupSpec           `json:"lookup_tables"`
	Channels            []ChannelSpec           `json:"channels"`
	Context             *[]ContextSpec          `json:"context"`
	SchemaProviders     *[]*SchemaProviderSpec  `json:"schema_providers"`
	PipesConfig         []PipeSpec              `json:"pipes_config"`
	ReducingPipesConfig [][]PipeSpec            `json:"reducing_pipes_config"`
}

// Cluster configuration
// DefaultMaxConcurrency is to override the env var MAX_CONCURRENCY
type ClusterSpec struct {
	NbrNodes              int                  `json:"nbr_nodes"`
	DefaultMaxConcurrency int                  `json:"default_max_concurrency"`
	S3WorkerPoolSize      int                  `json:"s3_worker_pool_size"`
	NbrNodesLookup        *[]ClusterSizingSpec `json:"nbr_nodes_lookup"`
	IsDebugMode           bool                 `json:"is_debug_mode"`
	KillSwitchMin         int                  `json:"kill_switch_min"`
}

// Cluster sizing configuration
// Allows to dynamically determine the NbrNodes based on total size of input files.
// UseEcsTasks and MaxConcurrency is used for step id 'reducing0'
// When UseEcsTasks == true, MaxConcurrency applies to ECS cluster (reducing0 step id).
// Note that S3WorkerPoolSize is used for reducing01, all other reducing steps use the
// S3WorkerPoolSize set at the ClusterSpec level.
type ClusterSizingSpec struct {
	WhenTotalSizeGe  int  `json:"when_total_size_ge_mb"`
	NbrNodes         int  `json:"nbr_nodes"`
	S3WorkerPoolSize int  `json:"s3_worker_pool_size"`
	UseEcsTasks      bool `json:"use_ecs_tasks"`
	MaxConcurrency   int  `json:"max_concurrency"`
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

type LookupSpec struct {
	// type range: sql_lookup, s2_csv_lookup
	Key          string            `json:"key"`
	Type         string            `json:"type"`
	Query        string            `json:"query"`      // for sql_lookup
	CsvSource    *CsvSourceSpec    `json:"csv_source"` //for s2_csv_lookup
	Columns      []TableColumnSpec `json:"columns"`
	LookupKey    []string          `json:"lookup_key"`
	LookupValues []string          `json:"lookup_values"`
}

type CsvSourceSpec struct {
	// Type range: cpipes, csv_file (future)
	// Default values are taken from current pipeline
	// InputFormat: csv, headerless_csv, compressed_csv, compressed_headerless_csv
	Type               string `json:"type"`
	InputFormat        string `json:"input_format"`
	ProcessName        string `json:"process_name"`   // for cpipes
	ReadStepId         string `json:"read_step_id"`   // for cpipes
	JetsPartitionLabel string `json:"jets_partition"` // for cpipes
	SessionId          string `json:"session_id"`     // for cpipes
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

type SchemaProviderSpec struct {
	// Type range: default
	// InputFormat: csv, headerless_csv, fixed-width, parquet
	Type        string             `json:"type"`
	Key         string             `json:"key"`
	Client      string             `json:"client"`
	Vendor      string             `json:"vendor"`
	ObjectType  string             `json:"object_type"`
	SchemaName  string             `json:"schema_name"`
	InputFormat string             `json:"input_format"`
	Delimiter   string             `json:"delimiter"`
	Columns     []SchemaColumnSpec `json:"columns"`
}

type SchemaColumnSpec struct {
	Name      string `json:"name"`
	Length    int    `json:"length"`
	Precision *int   `json:"precision"`
}

type TableSpec struct {
	Key                string            `json:"key"`
	Name               string            `json:"name"`
	CheckSchemaChanged bool              `json:"check_schema_changed"`
	Columns            []TableColumnSpec `json:"columns"`
}

type OutputFileSpec struct {
	Key     string   `json:"key"`
	Name    string   `json:"name"`
	Headers []string `json:"headers"`
}

type TableColumnSpec struct {
	Name    string `json:"name"`
	RdfType string `json:"rdf_type"`
	IsArray bool   `json:"as_array"`
}

type PipeSpec struct {
	// Type range: fan_out, splitter, merge_files
	Type           string               `json:"type"`
	InputChannel   InputChannelConfig   `json:"input_channel"`
	SplitterConfig *SplitterSpec        `json:"splitter_config"`
	Apply          []TransformationSpec `json:"apply"`
	OutputFile     *string              `json:"output_file"` // for merge_files
}

type SplitterSpec struct {
	// Type range: standard (default), ext_count
	// standard: split on Column / DefaultSplitterValue, create partition for each value
	// ext_count: split on Column / DefaultSplitterValue + N, N = 0..ExtPartitionsCount-1
	//            where each partition has up to RowCount rows
	Type                 string `json:"type"`
	Column               string `json:"column"`                 // splitter column
	DefaultSplitterValue string `json:"default_splitter_value"` // splitter default value
	PartitionRowCount    int    `json:"partition_row_count"`    // nbr of row for each ext partition
}

type TransformationSpec struct {
	// Type range: map_record, aggregate, analyze, high_freq, partition_writer, anonymize, distinct
	Type                  string                     `json:"type"`
	NewRecord             bool                       `json:"new_record"`
	PartitionSize         *int                       `json:"partition_size"`
	JetsPartitionKey      *string                    `json:"jets_partition_key"`
	FilePathSubstitutions *[]PathSubstitution        `json:"file_path_substitutions"`
	Columns               []TransformationColumnSpec `json:"columns"`
	DataSchema            *[]DataSchemaSpec          `json:"data_schema"`
	DeviceWriterType      *string                    `json:"device_writer_type"`
	WriteHeaders          bool                       `json:"write_headers"`
	RegexTokens           *[]RegexNode               `json:"regex_tokens"`      // Type analyze
	LookupTokens          *[]LookupTokenNode         `json:"lookup_tokens"`     // Type analyze
	KeywordTokens         *[]KeywordTokenNode        `json:"keyword_tokens"`    // Type analyze
	HighFreqColumns       *[]*HighFreqSpec           `json:"high_freq_columns"` // Type high_freq
	AnonymizeConfig       *AnonymizeSpec             `json:"anonymize_config"`
	DistinctConfig        *DistinctSpec              `json:"distinct_config"`
	OutputChannel         OutputChannelConfig        `json:"output_channel"`
}

type InputChannelConfig struct {
	// Type range: input, stage (default)
	// Format: csv, headerless_csv, compressed_csv, compressed_headerless_csv, etc
	Type         string `json:"type"`
	Name         string `json:"name"`
	Format       string `json:"format"` // Type input
	ReadStepId   string `json:"read_step_id"`
	SamplingRate int    `json:"sampling_rate"`
}

type OutputChannelConfig struct {
	// Type range: stage (default), output, sql
	// Format: csv, headerless_csv, compressed_csv, compressed_headerless_csv, etc
	Type           string `json:"type"`
	Name           string `json:"name"`
	Format         string `json:"format"`           // Type output
	WriteStepId    string `json:"write_step_id"`    // Type stage
	OutputTableKey string `json:"output_table_key"` // Type sql
	KeyPrefix      string `json:"key_prefix"`       // Type output
	FileName       string `json:"file_name"`        // Type output
	SpecName       string `json:"channel_spec_name"`
}

type PathSubstitution struct {
	Replace string `json:"replace"`
	With    string `json:"with"`
}

type DataSchemaSpec struct {
	Columns string `json:"column"`
	RdfType string `json:"rdf_type"`
}

type RegexNode struct {
	Name  string `json:"name"`
	Rexpr string `json:"re"`
}

type LookupTokenNode struct {
	Name   string   `json:"lookup_name"`
	KeyRe  string   `json:"key_re"`
	Tokens []string `json:"tokens"`
}

type KeywordTokenNode struct {
	Name     string   `json:"name"`
	Keywords []string `json:"keywords"`
}

type HighFreqSpec struct {
	Name          string `json:"name"`
	KeyRe         string `json:"key_re"`
	TopPercentile int    `json:"top_pct"`
	TopRank       int    `json:"top_rank"`
	re            *regexp.Regexp
}

type AnonymizeSpec struct {
	LookupName        string              `json:"lookup_name"`
	AnonymizeType     string              `json:"anonymize_type"`
	KeyPrefix         string              `json:"key_prefix"`
	KeysOutputChannel OutputChannelConfig `json:"keys_output_channel"`
}

type DistinctSpec struct {
	DistinctOn []string `json:"distinct_on"`
}

type TransformationColumnSpec struct {
	// Type range: select, value, eval, map, hash
	// count, distinct_count, sum, min, case,
	// map_reduce, lookup
	Name           string                      `json:"name"`
	Type           string                      `json:"type"`
	Expr           *string                     `json:"expr"`
	MapExpr        *MapExpression              `json:"map_expr"`
	EvalExpr       *ExpressionNode             `json:"eval_expr"`
	HashExpr       *HashExpression             `json:"hash_expr"`
	Where          *ExpressionNode             `json:"where"`
	CaseExpr       []CaseExpression            `json:"case_expr"` // case operator
	ElseExpr       []*ExpressionNode           `json:"else_expr"` // case operator
	MapOn          *string                     `json:"map_on"`
	AlternateMapOn *[]string                   `json:"alternate_map_on"`
	ApplyMap       *[]TransformationColumnSpec `json:"apply_map"`
	ApplyReduce    *[]TransformationColumnSpec `json:"apply_reduce"`
	LookupName     *string                     `json:"lookup_name"`
	LookupKey      *[]LookupColumnSpec         `json:"key"`
	LookupValues   *[]LookupColumnSpec         `json:"values"`
}

type LookupColumnSpec struct {
	// Type range: select, value
	Name string  `json:"name"`
	Type string  `json:"type"`
	Expr *string `json:"expr"`
}

type HashExpression struct {
	Expr                   string    `json:"expr"`
	NbrJetsPartitions      *uint64   `json:"nbr_jets_partitions"`
	AlternateCompositeExpr *[]string `json:"alternate_composite_expr"`
}

type MapExpression struct {
	CleansingFunction *string `json:"cleansing_function"`
	Argument          *string `json:"argument"`
	Default           *string `json:"default"`
	ErrMsg            *string `json:"err_msg"`
	RdfType           string  `json:"rdf_type"`
}

type ExpressionNode struct {
	// Type is for leaf nodes: select, value
	// Name is for CaseExpression.Then and TransformationColumnSpec.ElseExpr
	// to indicate which column to set the calculated value
	Name      *string         `json:"name"` // TransformationColumnSpec case operator
	Type      *string         `json:"type"`
	Expr      *string         `json:"expr"`
	AsRdfType *string         `json:"as_rdf_type"`
	Arg       *ExpressionNode `json:"arg"`
	Lhs       *ExpressionNode `json:"lhs"`
	Op        *string         `json:"op"`
	Rhs       *ExpressionNode `json:"rhs"`
}

type CaseExpression struct {
	When ExpressionNode    `json:"when"`
	Then []*ExpressionNode `json:"then"`
}
