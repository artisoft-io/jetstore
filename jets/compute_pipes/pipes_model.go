package compute_pipes

import (
	"regexp"
)

// This file contains the Compute Pipes configuration model
type ComputePipesConfig struct {
	CommonRuntimeArgs      *ComputePipesCommonArgs `json:"common_runtime_args,omitzero"`
	MetricsConfig          *MetricsSpec            `json:"metrics_config,omitzero"`
	ClusterConfig          *ClusterSpec            `json:"cluster_config,omitzero"`
	OutputTables           []*TableSpec            `json:"output_tables,omitempty"`
	OutputFiles            []OutputFileSpec        `json:"output_files,omitempty"`
	LookupTables           []*LookupSpec           `json:"lookup_tables,omitempty"`
	Channels               []ChannelSpec           `json:"channels,omitempty"`
	Context                []ContextSpec           `json:"context,omitempty"`
	SchemaProviders        []*SchemaProviderSpec   `json:"schema_providers,omitempty"`
	PipesConfig            []PipeSpec              `json:"pipes_config,omitempty"`
	ReducingPipesConfig    [][]PipeSpec            `json:"reducing_pipes_config,omitempty"`
	ConditionalPipesConfig []ConditionalPipeSpec   `json:"conditional_pipes_config,omitempty"`
}

func (cp *ComputePipesConfig) MainInputChannel() *InputChannelConfig {
	switch {
	case len(cp.ReducingPipesConfig) > 0 && len(cp.ReducingPipesConfig[0]) > 0:
		return &cp.ReducingPipesConfig[0][0].InputChannel
	case len(cp.ConditionalPipesConfig) > 0 && len(cp.ConditionalPipesConfig[0].PipesConfig) > 0:
		return &cp.ConditionalPipesConfig[0].PipesConfig[0].InputChannel
	}
	return nil
}

func (cp *ComputePipesConfig) NbrComputePipes() int {
	l := len(cp.ReducingPipesConfig)
	if l > 0 {
		return l
	}
	return len(cp.ConditionalPipesConfig)
}

// This function is called once per compute pipes step (sharding or redicung)
// so we construct the ExprNodeEvaluator as needed.
func (cp *ComputePipesConfig) GetComputePipes(stepId int, env map[string]any) ([]PipeSpec, int, error) {
	switch {
	case len(cp.ReducingPipesConfig) > stepId:
		return cp.ReducingPipesConfig[stepId], stepId, nil
	case len(cp.ConditionalPipesConfig) > stepId:
		if cp.ConditionalPipesConfig[stepId].When != nil {
			// Check if condition is met
			// Available expr variables:
			// multi_step_sharding as int, when > 0, nbr of shards is nbr_partition**2
			// total_file_size in bytes
			// total_file_size_gb in GiB
			// nbr_partitions as int (assuming each sharding step has the same nbr of partitions?)
			builderContext := ExprBuilderContext(env)
			for {
				if cp.ConditionalPipesConfig[stepId].When != nil {
					evaluator, err := builderContext.BuildExprNodeEvaluator("conditional_steps", nil,
						cp.ConditionalPipesConfig[stepId].When)
					if err != nil {
						return nil, 0, err
					}
					v, err := evaluator.eval(env)
					if err != nil {
						return nil, 0, err
					}
					if ToBool(v) {
						return cp.ConditionalPipesConfig[stepId].PipesConfig, stepId, nil
					}
					stepId += 1
					if len(cp.ConditionalPipesConfig) == stepId {
						// Got no more steps
						return nil, stepId, nil
					}
				} else {
					return cp.ConditionalPipesConfig[stepId].PipesConfig, stepId, nil
				}
			}
		}
		return cp.ConditionalPipesConfig[stepId].PipesConfig, stepId, nil
	}
	return nil, stepId, nil
}

// Cluster configuration
// [DefaultMaxConcurrency] is to override the env var MAX_CONCURRENCY
// [nbrPartitions] is specified at ClusterShardingSpec level otherwise at the
// ClusterSpec level. [nbrPartitions] is determined by the nbr of sharding nodes,
// capped by MaxNbrPartitions.
// [DefaultShardSizeMb] is the default value (in MB) when not specified at ClusterShardingSpec level.
// [DefaultShardMaxSizeMb] is the default value (in MB) when not specified at ClusterShardingSpec level.
// [DefaultShardSizeBy] is the default value (in bytes) when not specified at ClusterShardingSpec level.
// [DefaultShardMaxSizeBy] is the default value (in bytes) when not specified at ClusterShardingSpec level.
// NOTE: [ShardSizeMb] / [ShardMaxSizeMb] must be spefified for the sharding to take place.
// [MultiStepShardingThresholds] is the number of partitions to trigger the use of multi step sharding.
// When [MultiStepShardingThresholds] > 0 then [nbrPartitions] is sqrt(nbr of sharding nodes).
// [ShardingInfo] is calculated based on input files.
// [ShardingInfo] is used by the hash operator.
// Do not set [ShardingInfo] at configuration time, it will be ignored and replaced with the calculated values.
// Note: Make sure that ClusterShardingSpec is in decreasing order of WhenTotalSizeGe.
type ClusterSpec struct {
	MaxNbrPartitions            int                   `json:"max_nbr_partitons,omitzero"`
	MultiStepShardingThresholds int                   `json:"multi_step_sharding_thresholds,omitzero"`
	DefaultShardSizeMb          float64               `json:"default_shard_size_mb,omitzero"`
	DefaultShardMaxSizeMb       float64               `json:"default_shard_max_size_mb,omitzero"`
	DefaultShardSizeBy          float64               `json:"default_shard_size_by,omitzero"`     // for testing only
	DefaultShardMaxSizeBy       float64               `json:"default_shard_max_size_by,omitzero"` // for testing only
	ShardOffset                 int                   `json:"shard_offset,omitzero"`
	DefaultMaxConcurrency       int                   `json:"default_max_concurrency,omitzero"`
	S3WorkerPoolSize            int                   `json:"s3_worker_pool_size,omitzero"`
	ClusterShardingTiers        []ClusterShardingSpec `json:"cluster_sharding_tiers,omitempty"`
	IsDebugMode                 bool                  `json:"is_debug_mode,omitzero"`
	KillSwitchMin               int                   `json:"kill_switch_min,omitzero"`
	ShardingInfo                *ClusterShardingInfo  `json:"sharding_info,omitzero"`
}

func (cs *ClusterSpec) NbrPartitions(mode string) int {
	switch mode {
	case "limited_range":
		return cs.ShardingInfo.NbrPartitions
	case "full_range":
		if cs.ShardingInfo.MultiStepSharding > 0 {
			return cs.ShardingInfo.NbrPartitions * cs.ShardingInfo.NbrPartitions
		}
		return cs.ShardingInfo.NbrPartitions
	default:
		if cs.ShardingInfo.MultiStepSharding > 0 {
			n := cs.ShardingInfo.NbrPartitions * cs.ShardingInfo.NbrPartitions
			if n > cs.ShardingInfo.MaxNbrPartitions {
				return cs.ShardingInfo.MaxNbrPartitions
			}
			return n
		}
		return cs.ShardingInfo.NbrPartitions
	}
}

// Cluster sizing configuration
// Allows to dynamically determine the NbrNodes based on total size of input files.
// When using ecs tasks, MaxConcurrency applies to ECS cluster,
// otherwise MaxConcurrency is the number of concurrent lambda functions executing.
// Note that S3WorkerPoolSize is used for reducing01, all other reducing steps use the
// S3WorkerPoolSize set at the ClusterSpec level.
// ShardSizeMb/ShardMaxSizeMb must be spcified to determine the nbr of nodes and to allocate files
// to shards.
// When [MaxNbrPartitions] is not specified, the value at the ClusterSpec level is taken.
type ClusterShardingSpec struct {
	WhenTotalSizeGe             int     `json:"when_total_size_ge_mb,omitzero"`
	MaxNbrPartitions            int     `json:"max_nbr_partitions,omitzero"`
	MultiStepShardingThresholds int     `json:"multi_step_sharding_thresholds,omitzero"`
	ShardSizeMb                 float64 `json:"shard_size_mb,omitzero"`
	ShardMaxSizeMb              float64 `json:"shard_max_size_mb,omitzero"`
	ShardSizeBy                 float64 `json:"shard_size_by,omitzero"`     // for testing only
	ShardMaxSizeBy              float64 `json:"shard_max_size_by,omitzero"` // for testing only
	S3WorkerPoolSize            int     `json:"s3_worker_pool_size,omitzero"`
	MaxConcurrency              int     `json:"max_concurrency,omitzero"`
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
	// type range: sql_lookup, s3_csv_lookup
	Key          string            `json:"key"`
	Type         string            `json:"type"`
	Query        string            `json:"query,omitempty"`     // for sql_lookup
	CsvSource    *CsvSourceSpec    `json:"csv_source,omitzero"` //for s3_csv_lookup
	Columns      []TableColumnSpec `json:"columns,omitempty"`
	LookupKey    []string          `json:"lookup_key,omitempty"`
	LookupValues []string          `json:"lookup_values,omitempty"`
}

type CsvSourceSpec struct {
	// This is used for lookup tables and loading metadata in jetrules.
	// This is a single file source, the first file found is taken.
	// Type range: cpipes, csv_file (future)
	// Default values are taken from current pipeline
	// Format: csv, headerless_csv
	// Compression: none, snappy
	// MakeEmptyWhenNoFile: Do not make an error when no files
	// are found, make empty source. Default: generate an error when no files
	// are found in s3.
	Type                string `json:"type"`
	Format              string `json:"format,omitempty"`
	Compression         string `json:"compression,omitempty"`
	Delimiter           rune   `json:"delimiter,omitzero"`       // default ','
	ProcessName         string `json:"process_name,omitempty"`   // for cpipes
	ReadStepId          string `json:"read_step_id,omitempty"`   // for cpipes
	JetsPartitionLabel  string `json:"jets_partition,omitempty"` // for cpipes
	SessionId           string `json:"session_id,omitempty"`     // for cpipes
	ClassName           string `json:"class_name,omitempty"`     // used by jetrules_config
	MakeEmptyWhenNoFile bool   `json:"make_empty_source_when_no_files_found,omitzero"`
}

// ChannelSpec specifies the columns of a channel
// The columns can be obtained from a domain class from the
// local workspace using class_name.
// In that case, the columns
// that are specified in the slice, are added to the columns of
// the domain class.
// When direct_properties_only is true, only take the data properties
// of the class, not including the properties of the parent classes.
// DomainKeys provide the ability to configure the domain keys in the cpipes config document.
// DomainKeysSpec is parsed version of DomainKeys or the spec from the domain_keys_registry table.
// DomainKeysSpec is derived from DomainKeys when provided.
// columnsMap is added in StartComputePipes
type ChannelSpec struct {
	Name                 string          `json:"name"`
	Columns              []string        `json:"columns"`
	ClassName            string          `json:"class_name,omitempty"`
	DirectPropertiesOnly bool            `json:"direct_properties_only,omitzero"`
	HasDynamicColumns    bool            `json:"has_dynamic_columns,omitzero"`
	DomainKeys           map[string]any  `json:"domain_keys,omitempty"`
	DomainKeysInfo       *DomainKeysSpec `json:"domain_keys_spec,omitzero"`
	columnsMap           *map[string]int
}

type ContextSpec struct {
	// Type range: file_key_component, partfile_key_component
	Type string `json:"type,omitempty"`
	Key  string `json:"key,omitempty"`
	Expr string `json:"expr,omitempty"`
}

// Configuration type for factoring out all file settings.
// This is used by more specific types such as:
// SchemaProviderSpec, InputChannelConfig, OutputChannelConfig, OutputFileSpec
type FileConfig struct {
	BadRowsConfig           *BadRowsSpec   `json:"bad_rows_config,omitzero"`
	Bucket                  string         `json:"bucket,omitempty"`
	Compression             string         `json:"compression,omitempty"`
	Delimiter               rune           `json:"delimiter,omitzero"`
	DetectEncoding          bool           `json:"detect_encoding,omitzero"`
	DomainClass             string         `json:"domain_class,omitempty"`
	DomainKeys              map[string]any `json:"domain_keys,omitempty"`
	Encoding                string         `json:"encoding,omitempty"`
	EnforceRowMaxLength     bool           `json:"enforce_row_max_length,omitzero"`
	EnforceRowMinLength     bool           `json:"enforce_row_min_length,omitzero"`
	FileKey                 string         `json:"file_key,omitempty"`
	FileName                string         `json:"file_name,omitempty"` // Type output
	FixedWidthColumnsCsv    string         `json:"fixed_width_columns_csv,omitempty"`
	Format                  string         `json:"format,omitempty"`
	InputFormatDataJson     string         `json:"input_format_data_json,omitempty"`
	IsPartFiles             bool           `json:"is_part_files,omitzero"`
	KeyPrefix               string         `json:"key_prefix,omitempty"`
	NbrRowsInRecord         int64          `json:"nbr_rows_in_record,omitzero"` // Format: parquet
	NoQuotes                bool           `json:"no_quotes,omitzero"`
	QuoteAllRecords         bool           `json:"quote_all_records,omitzero"`
	ReadBatchSize           int64          `json:"read_batch_size,omitzero"` // Format: parquet
	ReadDateLayout          string         `json:"read_date_layout,omitempty"`
	TrimColumns             bool           `json:"trim_columns,omitzero"`
	UseLazyQuotes           bool           `json:"use_lazy_quotes,omitzero"`
	VariableFieldsPerRecord bool           `json:"variable_fields_per_record,omitzero"`
	WriteDateLayout         string         `json:"write_date_layout,omitempty"`
}

type SchemaProviderSpec struct {
	// Type range: default
	// Key is schema provider key for reference by compute pipes steps
	// Format: csv, headerless_csv, fixed_width, parquet, parquet_select,
	//              xlsx, headerless_xlsx
	// Compression: none, snappy (parquet is always snappy)
	// ReadBatchSize: nbr of rows to read per record (format: parquet)
	// NbrRowsInRecord: nbr of rows in record (format: parquet)
	// InputFormatDataJson: json config based on Format (typically used for xlsx)
	// example: {"currentSheet": "Daily entry for Approvals"} (for xlsx).
	// EnforceRowMinLength: when true, all columns must be in input record, otherwise missing columns are null
	// EnforceRowMaxLength: when true, no extra characters must exist past last field (applies to text format)
	// BadRowsConfig: Specify how to handle bad rows when bot specified on InputChannelConfig.
	// SourceType range: main_input, merged_input, historical_input (from input_source table)
	// Columns may be ommitted if fixed_width_columns_csv is provided or is a csv format
	// UseLazyQuotes, VariableFieldsPerRecord see csv.NewReader
	// QuoteAllRecords will quote all records for csv writer
	// NoQuotes will no quote any records for csv writer (even if the record contains '"')
	// Bucket and FileKey are location and source object (fileKey may be directory if IsPartFiles is true)
	// KmsKey is kms key to use when writing output data. May be empty.
	// Contains properties to register FileKey with input_registry table:
	// Client, Vendor, ObjectType, FileDate
	// NotificationTemplatesOverrides have the following keys to override the templates defined
	// in the deployment environment var: CPIPES_START_NOTIFICATION_JSON,
	// CPIPES_COMPLETED_NOTIFICATION_JSON, and CPIPES_FAILED_NOTIFICATION_JSON.
	//*TODO domain_keys_json
	//*TODO code_values_mapping_json
	FileConfig
	Key                              string             `json:"key"`
	Type                             string             `json:"type"`
	FileSize                         int64              `json:"file_size,omitzero"`
	KmsKey                           string             `json:"kms_key_arn,omitempty"`
	Client                           string             `json:"client,omitempty"`
	Vendor                           string             `json:"vendor,omitempty"`
	ObjectType                       string             `json:"object_type,omitempty"`
	FileDate                         string             `json:"file_date,omitempty"`
	SourceType                       string             `json:"source_type,omitempty"`
	SchemaName                       string             `json:"schema_name,omitempty"`
	Columns                          []SchemaColumnSpec `json:"columns,omitempty"`
	Env                              map[string]any     `json:"env,omitempty"`
	ReportCmds                       []ReportCmdSpec    `json:"report_cmds,omitempty"`
	NotificationTemplatesOverrides   map[string]string  `json:"notification_templates_overrides,omitempty"`
	NotificationRoutingOverridesJson string             `json:"notification_routing_overrides_json,omitempty"`
}

// Commands for the run_report step
// Type range: s3_copy_file
type ReportCmdSpec struct {
	Type             string          `json:"type"`
	S3CopyFileConfig *S3CopyFileSpec `json:"s3_copy_file_config,omitzero"`
}

// ReportCommand to copy file from s3 to s3
// Default WorkerPoolSize is calculated based on number of tasks
type S3CopyFileSpec struct {
	SourceBucket      string `json:"src_bucket,omitempty"`
	SourceKey         string `json:"src_key,omitempty"`
	DestinationBucket string `json:"dest_bucket,omitempty"`
	DestinationKey    string `json:"dest_key,omitempty"`
	WorkerPoolSize    int    `json:"worker_pool_size,omitzero"`
}

type SchemaColumnSpec struct {
	Name      string `json:"name,omitempty"`
	Length    int    `json:"length,omitzero"`    // for fixed_width
	Precision *int   `json:"precision,omitzero"` // for fixed_width
}

// ChannelSpecName specify the channel spec.
// Column provides metadata info
type TableSpec struct {
	Key                string            `json:"key"`
	Name               string            `json:"name"`
	CheckSchemaChanged bool              `json:"check_schema_changed,omitzero"`
	Columns            []TableColumnSpec `json:"columns,omitempty"`
	ChannelSpecName    string            `json:"channel_spec_name,omitempty"`
}

type OutputFileSpec struct {
	// OutputLocation: jetstore_s3_input, jetstore_s3_output (default), or custom file key.
	// When OutputLocation has a custom file key, it replace Name and KeyPrefix.
	// Note: refactoring using FileConfig.FileKey is synonym to OutputLocation
	// Note: refactoring using FileConfig.FileName is synonym to Name
	// KeyPrefix is optional, default to input file key path in OutputLocation.
	// Name is file name (required or via OutputLocation).
	// Headers overrides the headers from the input_channel's spec or
	// from the schema_provider.
	// Schema provider indicates if put the header line or not.
	// The input channel's schema provider indicates what delimiter
	// to use on the header line.
	FileConfig
	Key            string   `json:"key"`
	FileName2      string   `json:"name,omitempty"`
	FileKey2       string   `json:"output_location,omitempty"`
	SchemaProvider string   `json:"schema_provider,omitempty"`
	Headers        []string `json:"headers,omitempty"`
}

// Note: refactoring using FileConfig.FileKey is synonym to OutputLocation
func (r OutputFileSpec) OutputLocation() string {
	if len(r.FileKey2) > 0 {
		return r.FileKey2
	}
	return r.FileKey
}
func (r *OutputFileSpec) SetOutputLocation(s string) {
	r.FileKey2 = s
}

// Note: refactoring using FileConfig.FileName is synonym to Name
func (r OutputFileSpec) Name() string {
	if len(r.FileName2) > 0 {
		return r.FileName2
	}
	return r.FileName
}
func (r *OutputFileSpec) SetName(s string) {
	r.FileName2 = s
}

type TableColumnSpec struct {
	Name    string `json:"name"`
	RdfType string `json:"rdf_type,omitempty"`
	IsArray bool   `json:"as_array,omitzero"`
}

type PipeSpec struct {
	// Type range: fan_out, splitter, merge_files
	Type           string               `json:"type"`
	InputChannel   InputChannelConfig   `json:"input_channel"`
	SplitterConfig *SplitterSpec        `json:"splitter_config,omitzero"`
	Apply          []TransformationSpec `json:"apply"`
	OutputFile     *string              `json:"output_file,omitzero"` // for merge_files
}

// ConditionalPipe: Each step are executed conditionally
// When is nil, the step is always executed.
// Available expr variables (see above):
// multi_step_sharding as int, when > 0, nbr of shards is nbr_partition**2
// total_file_size in bytes
// nbr_partitions as int (used for hashing purpose)
// use_ecs_tasks is true to use ecs fargate task
// use_ecs_tasks_when is an expression as the when property.
type ConditionalPipeSpec struct {
	StepName        string          `json:"step_name,omitempty"`
	UseEcsTasks     bool            `json:"use_ecs_tasks,omitzero"`
	UseEcsTasksWhen *ExpressionNode `json:"use_ecs_tasks_when,omitzero"`
	PipesConfig     []PipeSpec      `json:"pipes_config"`
	When            *ExpressionNode `json:"when,omitzero"`
}

type SplitterSpec struct {
	// Type range: standard (default), ext_count
	// standard: split on Column / DefaultSplitterValue / ShardOn, create partition for each value
	// ext_count: split on Column / DefaultSplitterValue / ShardOn + N, N = 0..ExtPartitionsCount-1
	//            where each partition has up to PartitionRowCount rows
	Type                 string          `json:"type,omitempty"`
	Column               string          `json:"column,omitempty"`                 // splitter column
	DefaultSplitterValue string          `json:"default_splitter_value,omitempty"` // splitter default value
	ShardOn              *HashExpression `json:"shard_on,omitzero"`                // splitter hash on the fly
	PartitionRowCount    int             `json:"partition_row_count,omitzero"`     // nbr of row for each ext partition
}

type TransformationSpec struct {
	// Type range: map_record, aggregate, analyze, high_freq, partition_writer,
	//	anonymize, distinct, shuffling, group_by, filter, sort, jetrules, clustering
	// Format takes precedence over SchemaProvider's Format (from OutputChannelConfig)
	Type                  string                     `json:"type"`
	NewRecord             bool                       `json:"new_record,omitzero"`
	Columns               []TransformationColumnSpec `json:"columns,omitempty"`
	MapRecordConfig       *MapRecordSpec             `json:"map_record_config,omitzero"`
	AnalyzeConfig         *AnalyzeSpec               `json:"analyze_config,omitzero"`
	HighFreqColumns       []*HighFreqSpec            `json:"high_freq_columns,omitempty"` // Type high_freq
	PartitionWriterConfig *PartitionWriterSpec       `json:"partition_writer_config,omitzero"`
	AnonymizeConfig       *AnonymizeSpec             `json:"anonymize_config,omitzero"`
	DistinctConfig        *DistinctSpec              `json:"distinct_config,omitzero"`
	ShufflingConfig       *ShufflingSpec             `json:"shuffling_config,omitzero"`
	GroupByConfig         *GroupBySpec               `json:"group_by_config,omitzero"`
	FilterConfig          *FilterSpec                `json:"filter_config,omitzero"`
	SortConfig            *SortSpec                  `json:"sort_config,omitzero"`
	JetrulesConfig        *JetrulesSpec              `json:"jetrules_config,omitzero"`
	ClusteringConfig      *ClusteringSpec            `json:"clustering_config,omitzero"`
	OutputChannel         OutputChannelConfig        `json:"output_channel"`
}

type MapRecordSpec struct {
	FileMappingTableName string `json:"file_mapping_table_name"`
}

// SchemaProvider is used for external configuration, such as date format
type AnalyzeSpec struct {
	SchemaProvider                  string              `json:"schema_provider,omitempty"`
	ScrubChars                      string              `json:"scrub_chars,omitempty"`
	DistinctValuesWhenLessThanCount int                 `json:"distinct_values_when_less_than_count,omitzero"`
	PadShortRowsWithNulls           bool                `json:"pad_short_rows_with_nulls,omitzero"`
	EntityHints                     []*EntityHint       `json:"entity_hints,omitempty"`
	RegexTokens                     []RegexNode         `json:"regex_tokens,omitempty"`
	LookupTokens                    []LookupTokenNode   `json:"lookup_tokens,omitempty"`
	KeywordTokens                   []KeywordTokenNode  `json:"keyword_tokens,omitempty"`
	FunctionTokens                  []FunctionTokenNode `json:"function_tokens,omitempty"`
}

// Defines the identification and handling of bad rows
// Currently only used for input_row channel
// BadRowsStepId: step id in stage location to output bad rows
// The input row is considered a bad row when any of WhenCriteria applies
// then the row is sent to bad row channel and remove from the input rows.
type BadRowsSpec struct {
	BadRowsStepId string `json:"bad_rows_step_id,omitempty"`
	// WhenCriteria  []BadRowsCriteria `json:"when_criteria,omitempty"`
}

type InputChannelConfig struct {
	// Type range: memory (default), input, stage
	// Format: csv, headerless_csv, etc.
	// ReadBatchSize: nbr of rows to read per record (format: parquet)
	// Compression: none, snappy (parquet: always snappy)
	// Note: SchemaProvider, Compression, Format for Type input are provided via
	// ComputePipesCommonArgs.SourcesConfig (ie input_registry table).
	// BadRowsConfig: Specify how to handle bad rows.
	// HasGroupedRows indicates that the channel contains grouped rows,
	// most likely from the group_by operator.
	// Note: The input_row channel (main input) will be cast to the
	// rdf type specified by the domain class of the main input source.
	FileConfig
	Type             string `json:"type"`
	Name             string `json:"name"`
	SchemaProvider   string `json:"schema_provider,omitempty"`
	ReadStepId       string `json:"read_step_id,omitempty"`
	SamplingRate     int    `json:"sampling_rate,omitzero"`
	SamplingMaxCount int    `json:"sampling_max_count,omitzero"`
	HasGroupedRows   bool   `json:"has_grouped_rows,omitzero"`
}

type OutputChannelConfig struct {
	// Type range: memory (default), stage, output, sql
	// Format: csv, headerless_csv, etc.
	// NbrRowsInRecord: nbr of rows in record (format: parquet)
	// Compression: none, snappy (default).
	// UseInputParquetSchema to use the same schema as the input file.
	// Must have save_parquet_schema = true in the cpipes first input_channel.
	// OutputLocation: jetstore_s3_input, jetstore_s3_output (default), or custom location.
	// When OutputLocation is jetstore_s3_input it will also write to the input bucket.
	// When OutputLocation uses a custom location, it replaces KeyPrefix and FileName.
	// OutputLocation must ends with "/" if we want to use default file name.
	// Note: refactoring using FileConfig.FileKey is synonym to OutputLocation
	// KeyPrefix is optional, default to $PATH_FILE_KEY.
	// Use $CURRENT_PARTITION_LABEL in KeyPrefix and FileName to substitute with
	// current partition label.
	// Other available env substitution:
	// $FILE_KEY main input file key.
	// $SESSIONID current session id.
	// $PROCESS_NAME current process name.
	// $PATH_FILE_KEY file key path portion.
	// $NAME_FILE_KEY file key file name portion (empty when in part files mode).
	// $SHARD_ID current node id.
	// $JETS_PARTITION_LABEL current node partition label.
	FileConfig
	Type                  string `json:"type"`
	Name                  string `json:"name,omitempty"`
	UseInputParquetSchema bool   `json:"use_input_parquet_schema,omitzero"` // Type stage,output
	SchemaProvider        string `json:"schema_provider,omitempty"`         // Type stage,output, alt to Format
	WriteStepId           string `json:"write_step_id,omitempty"`           // Type stage
	OutputTableKey        string `json:"output_table_key,omitempty"`        // Type sql
	FileKey2              string `json:"output_location,omitempty"`         // Type output
	SpecName              string `json:"channel_spec_name,omitempty"`
}

// Note: refactoring using FileConfig.FileKey is synonym to OutputLocation
func (r OutputChannelConfig) OutputLocation() string {
	if len(r.FileKey2) > 0 {
		return r.FileKey2
	}
	return r.FileKey
}
func (r *OutputChannelConfig) SetOutputLocation(s string) {
	r.FileKey2 = s
}

type PathSubstitution struct {
	Replace string `json:"replace"`
	With    string `json:"with"`
}

type DataSchemaSpec struct {
	Columns string `json:"column"`
	RdfType string `json:"rdf_type,omitempty"`
}

type EntityHint struct {
	Entity             string   `json:"entity"`
	NameFragments      []string `json:"column_name_fragments,omitempty"`
	ExclusionFragments []string `json:"exclusion_fragments,omitempty"`
}

type RegexNode struct {
	Name             string `json:"name"`
	Rexpr            string `json:"re,omitempty"`
	UseScrubbedValue bool   `json:"use_scrubbed_value,omitzero"`
}

type LookupTokenNode struct {
	Name   string   `json:"lookup_name"`
	KeyRe  string   `json:"key_re,omitempty"`
	Tokens []string `json:"tokens,omitempty"`
}

type KeywordTokenNode struct {
	Name     string   `json:"name"`
	Keywords []string `json:"keywords,omitempty"`
}

// Type: parse_date
type FunctionTokenNode struct {
	Type               string            `json:"type"`
	MinMaxDateFormat   string            `json:"minmax_date_format,omitempty"`
	ParseDateArguments []ParseDateFTSpec `json:"parse_date_args,omitempty"`
}

// The date format is using a reference date of
// Mon Jan 2 15:04:05 MST 2006 (see https://pkg.go.dev/time#Layout)
// Will use DateFormat when provided, otherwise take the format from
// the Schema Provider when avail. Will default to DefaultDateFormat
// or will use the jetstore date parser if no default provided or
// if UseJetstoreParser is true.
// year_less_than and year_greater_than is an additional condition
// to the match result.
type ParseDateFTSpec struct {
	Token             string `json:"token"`
	DefaultDateFormat string `json:"default_date_format,omitempty"`
	DateFormat        string `json:"date_format,omitempty"`
	UseJetstoreParser bool   `json:"use_jetstore_date_parser,omitzero"`
	YearLessThan      int    `json:"year_less_than,omitzero"`
	YearGreaterThan   int    `json:"year_greater_than,omitzero"`
}

// top_pct correspond the top percentile of the data,
// ie, retain the distinct values that correspond to
//
//	totalCount * top_pct / 100
//
// where totalCount is all the count of value for the column.
// If top_pct then all distinct alues are retained.
// top_rank correspond to the percentage of the distinct
// values to retain. That is:
//
//	nbrDistinctValues * top_rank / 100
//
// where nbrDistinctValues is the number of distinct values
// for the column. Note that the distinct values are by descending
// frequence of occurence.
type HighFreqSpec struct {
	Name          string `json:"name"`
	KeyRe         string `json:"key_re,omitempty"`
	TopPercentile int    `json:"top_pct,omitzero"`
	TopRank       int    `json:"top_rank,omitzero"`
	re            *regexp.Regexp
}

// DeviceWriterType range: csv_writer, parquet_writer, fixed_width_writer
// JetsPartitionKey used by partition_writer as the default value for jet_partition
// use $JETS_PARTITION_LABEL for current node input partition
// When StreamDataOut is true, data is stream to s3 rather than written locally
// and then copied to s3. Useful for large files that would exceed local storage capacity.
type PartitionWriterSpec struct {
	DeviceWriterType string  `json:"device_writer_type,omitempty"`
	JetsPartitionKey *string `json:"jets_partition_key,omitzero"`
	PartitionSize    int     `json:"partition_size,omitzero"`
	SamplingRate     int     `json:"sampling_rate,omitzero"`
	SamplingMaxCount int     `json:"sampling_max_count,omitzero"`
	StreamDataOut    bool    `json:"stream_data_out,omitzero"`
}

// LookupName is name of lookup table containing the file metadata from analyze operator
// AnonymizeType is column name in lookup table that specifiy how to anonymize (value: date, text)
// KeyPrefix is column name of lookup table to use as prefix of the anonymized value
// InputDateLayout is the format for parsing the input date (incoming data)
// OutputDateLayout is the format to use for anonymized date, will be set at 1st of the month of the original date
// KeyDateLayout is the format to use in the key mapping file (crosswalk file)
// DefaultInvalidDate is a placeholder to use as the anonymized date when the input date
// (the date to anonymize) is not valid. If unspecified, the input value is used unchanged
// as the output value.
// DefaultInvalidDate must be a valid date in the format YYYY/MM/DD or MM/DD/YYYY
// so it can be parsed using JetStore default date parser. The date will be formatted
// according to KeyDateLayout.
// OutputDateLayout defaults to InputDateLayout.
// KeyDateLayout defaults to OutputDateLayout.
// SchemaProvider is used to get the DateLayout / KeyDateLayout if not specified here.
// If date format is not specified, the default format for both OutputDateFormat and KeyDateFormat
// is "2006/01/02", ie. yyyy/MM/dd and the rdf.ParseDate() is used to parse the input date.
type AnonymizeSpec struct {
	LookupName           string              `json:"lookup_name,omitempty"`
	AnonymizeType        string              `json:"anonymize_type,omitempty"`
	KeyPrefix            string              `json:"key_prefix,omitempty"`
	InputDateLayout      string              `json:"input_date_layout,omitempty"`
	OutputDateLayout     string              `json:"output_date_layout,omitempty"`
	KeyDateLayout        string              `json:"key_date_layout,omitempty"`
	DefaultInvalidDate   string              `json:"default_invalid_date,omitempty"`
	SchemaProvider       string              `json:"schema_provider,omitempty"`
	AdjustFieldWidthOnFW bool                `json:"adjust_field_width_on_fixed_width_file,omitzero"`
	OmitPrefixOnFW       bool                `json:"omit_prefix_on_fixed_width_file,omitzero"`
	KeysOutputChannel    OutputChannelConfig `json:"keys_output_channel"`
}

type DistinctSpec struct {
	DistinctOn []string `json:"distinct_on,omitempty"`
}

type ShufflingSpec struct {
	MaxInputSampleSize    int               `json:"max_input_sample_size,omitzero"`
	OutputSampleSize      int               `json:"output_sample_size,omitzero"`
	PadShortRowsWithNulls bool              `json:"pad_short_rows_with_nulls,omitzero"`
	FilterColumns         *FilterColumnSpec `json:"filter_columns,omitzero"`
}

type FilterColumnSpec struct {
	LookupName     string   `json:"lookup_name,omitempty"`
	LookupColumn   string   `json:"lookup_column,omitempty"`
	RetainOnValues []string `json:"retain_on_values,omitempty"`
}

// Specify either domain_key, group_by_name, group_by_pos or group_by_count.
// group_by_count has priority over other mode of grouping.
// group_by_name wins when both group_by_name and group_by_pos are specified.
// domain_key use the domain key info to compute the composite key
// At least one must be specified.
type GroupBySpec struct {
	GroupByName  []string `json:"group_by_name,omitempty"`
	GroupByPos   []int    `json:"group_by_pos,omitempty"`
	GroupByCount int      `json:"group_by_count,omitzero"`
	DomainKey    string   `json:"domain_key,omitempty"`
}

// Filter row base on a when criteria
type FilterSpec struct {
	When           ExpressionNode `json:"when"`
	MaxOutputCount int            `json:"max_output_records,omitzero"`
}

// Sort using composite key
// sort_by column names making the composite key
// domain_key use the domain key info to compute the composite key
type SortSpec struct {
	DomainKey    string   `json:"domain_key,omitempty"`
	SortByColumn []string `json:"sort_by,omitempty"`
}

// MaxLooping overrides the value in the jetrules metastore
type JetrulesSpec struct {
	ProcessName             string                `json:"process_name,omitempty"`
	InputRdfType            string                `json:"input_rdf_type,omitempty"`
	MaxInputCount           int                   `json:"max_input_count,omitzero"`
	PoolSize                int                   `json:"pool_size,omitzero"`
	MaxReteSessionsSaved    int                   `json:"max_rete_sessions_saved,omitzero"`
	MaxLooping              int                   `json:"max_looping,omitzero"`
	CurrentSourcePeriod     int                   `json:"current_source_period,omitzero"`
	CurrentSourcePeriodDate string                `json:"current_source_period_date,omitempty"`
	CurrentSourcePeriodType string                `json:"current_source_period_type,omitempty"`
	RuleConfig              []map[string]any      `json:"rule_config,omitempty"`
	MetadataInputSources    []CsvSourceSpec       `json:"metadata_input_sources,omitempty"`
	IsDebug                 bool                  `json:"is_debug,omitzero"`
	OutputChannels          []OutputChannelConfig `json:"output_channels,omitempty"`
}

// If is_debug is true, correlation results are forwarded to s3 otherwise
// the correlation_output_channel is only used to specify the intermediate
// channels between the pool manager and the workers.
// MinColumn1NonNilCount is min nbr of column1 distinct values observed
// MinColumn2NonNilCount is min nbr of non nil values of column2 for a worker to report the correlation.
// ClusterDataSubclassification contains data_classification values, when found in a
// cluster all columns member of the cluster get that value as data_subclassification.
type ClusteringSpec struct {
	MaxInputCount                int                     `json:"max_input_count,omitzero"`
	MinColumn1NonNilCount        int                     `json:"min_column1_non_null_count,omitzero"`
	MinColumn2NonNilCount        int                     `json:"min_column2_non_null_count,omitzero"`
	TargetColumnsLookup          TargetColumnsLookupSpec `json:"target_columns_lookup"`
	ClusterDataSubclassification []string                `json:"cluster_data_subclassification,omitempty"`
	SoloDataSubclassification    []string                `json:"solo_data_subclassification,omitempty"`
	TransitiveDataClassification []string                `json:"transitive_data_classification,omitempty"`
	IsDebug                      bool                    `json:"is_debug,omitzero"`
	CorrelationOutputChannel     *OutputChannelConfig    `json:"correlation_output_channel,omitzero"`
}

type TargetColumnsLookupSpec struct {
	LookupName                  string   `json:"lookup_name"`
	DataClassificationColumn    string   `json:"data_classification_column,omitempty"`
	Column1ClassificationValues []string `json:"column1_classification_values,omitempty"`
	Column2ClassificationValues []string `json:"column2_classification_values,omitempty"`
}

type TransformationColumnSpec struct {
	// Type range: select, multi_select, value, eval, map, hash
	// count, distinct_count, sum, min, case,
	// map_reduce, lookup
	Name           string                     `json:"name"`
	Type           string                     `json:"type"`
	Expr           *string                    `json:"expr,omitempty"`
	ExprArray      []string                   `json:"expr_array,omitempty"`
	MapExpr        *MapExpression             `json:"map_expr,omitzero"`
	EvalExpr       *ExpressionNode            `json:"eval_expr,omitzero"`
	HashExpr       *HashExpression            `json:"hash_expr,omitzero"`
	Where          *ExpressionNode            `json:"where,omitzero"`
	CaseExpr       []CaseExpression           `json:"case_expr,omitempty"` // case operator
	ElseExpr       []*ExpressionNode          `json:"else_expr,omitempty"` // case operator
	MapOn          *string                    `json:"map_on,omitzero"`
	AlternateMapOn []string                   `json:"alternate_map_on,omitempty"`
	ApplyMap       []TransformationColumnSpec `json:"apply_map,omitempty"`
	ApplyReduce    []TransformationColumnSpec `json:"apply_reduce,omitempty"`
	LookupName     *string                    `json:"lookup_name,omitzero"`
	LookupKey      []LookupColumnSpec         `json:"key,omitempty"`
	LookupValues   []LookupColumnSpec         `json:"values,omitempty"`
}

type LookupColumnSpec struct {
	// Type range: select, value
	Name string  `json:"name,omitempty"`
	Type string  `json:"type,omitempty"`
	Expr *string `json:"expr,omitzero"`
}

// Hash using values from columns.
// Case single column, use Expr.
// Case multi column, use CompositeExpr.
// Expr takes precedence if both are populated.
// DomainKey is specified as an object_type. DomainKeysJson provides the
// mapping between domain keys and columns.
// AlternateCompositeExpr is used when Expr or CompositeExpr returns nil or empty.
// MultiStepShardingMode values: 'limited_range', 'full_range' or empty.
// NoPartitions indicated not to assign the hash to a partition (no modulo operation).
// ComputeDomainKey flag indicate to compute the domain key rather than a simple hash.
// This consider the hashing algo used and delimitor between the key components.
type HashExpression struct {
	Expr                   string   `json:"expr,omitempty"`
	CompositeExpr          []string `json:"composite_expr,omitempty"`
	DomainKey              string   `json:"domain_key,omitempty"`
	NbrJetsPartitions      *uint64  `json:"nbr_jets_partitions,omitzero"`
	MultiStepShardingMode  string   `json:"multi_step_sharding_mode,omitempty"`
	AlternateCompositeExpr []string `json:"alternate_composite_expr,omitempty"`
	NoPartitions           bool     `json:"no_partitions,omitzero"`
	ComputeDomainKey       bool     `json:"compute_domain_key,omitzero"`
}

type MapExpression struct {
	CleansingFunction string `json:"cleansing_function,omitempty"`
	Argument          string `json:"argument,omitempty"`
	Default           string `json:"default,omitempty"`
	ErrMsg            string `json:"err_msg,omitempty"`
	RdfType           string `json:"rdf_type,omitempty"`
}

type ExpressionNode struct {
	// Type is for leaf nodes: select, value
	// Name is for CaseExpression.Then and TransformationColumnSpec.ElseExpr
	// to indicate which column to set the calculated value
	Name      string          `json:"name,omitempty"` // TransformationColumnSpec case operator
	Type      string          `json:"type,omitempty"`
	Expr      string          `json:"expr,omitempty"`
	ExprList  []string        `json:"expr_list,omitempty"`
	AsRdfType string          `json:"as_rdf_type,omitempty"`
	Arg       *ExpressionNode `json:"arg,omitzero"`
	Lhs       *ExpressionNode `json:"lhs,omitzero"`
	Op        string          `json:"op,omitempty"`
	Rhs       *ExpressionNode `json:"rhs,omitzero"`
}

type CaseExpression struct {
	When ExpressionNode    `json:"when"`
	Then []*ExpressionNode `json:"then"`
}
