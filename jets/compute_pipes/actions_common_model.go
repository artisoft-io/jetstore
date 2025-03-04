package compute_pipes

import (
	"encoding/json"
	"fmt"
	"os"
	"regexp"

	"github.com/artisoft-io/jetstore/jets/awsi"
)

// Common functions and types for cp lambda version
// NOTE: SchemaProviders for externally specified schema, are specified
//       in the input_registry table via register_file_key action.
//       The schema providers are put into the ComputePipesConfig.
// Note: SchemaProviderSpec.Key must match the value specified in
//       OutputChannelConfig.SchemaProvider

// Argument to start_cp (start_sharding_cp, start_reducing_cp)
// for starting the cp cluster
// UseECSTask is currently used only for 'reducing' mode
type StartComputePipesArgs struct {
	PipelineExecKey int                  `json:"pipeline_execution_key"`
	FileKey         string               `json:"file_key,omitempty"`
	SessionId       string               `json:"session_id,omitempty"`
	StepId          *int                 `json:"step_id"`
	ClusterInfo     *ClusterShardingInfo `json:"cluster_sharding_info"`
	UseECSTask      bool                 `json:"use_ecs_tasks"`
}

// Contains info about the clustersharding. This info
// is determined during the sharding phase in [ShardFileKeys]
// and passed to the StartReducing actions via [StartComputePipesArgs]
type ClusterShardingInfo struct {
	TotalFileSize     int64 `json:"total_file_size"`
	MaxNbrPartitions  int   `json:"max_nbr_partitions"`
	NbrPartitions     int   `json:"nbr_partitions"`
	MultiStepSharding int   `json:"multi_step_sharding"`
}

// Arguments to start cp_node for sharding and reducing
// minimal set of arguments to reduce the size of the json
// to call the lambda functions
type ComputePipesNodeArgs struct {
	NodeId             int    `json:"id"`
	JetsPartitionLabel string `json:"jp,omitempty"`
	PipelineExecKey    int    `json:"pe"`
}

// Common arguments factored out and put into the
// the ComputePipesConfig component
// MergeFile property is to indicate that the pipe
// is at the stage of merging the part files into a single output file.
// This will use a single node since merge_files has a single partition
// to read from (the step_id prior to merge_files writes a single partition).
// Note: Client and Org are the pipeline execution client and org and may
// be different than the client/vendor of the actual data (case using
// stand-in client/org name). In that situation the actual
// client/vendor of the data is specified at run time via the SchemaProviders
// on table input_registry.
type ComputePipesCommonArgs struct {
	CpipesMode        string            `json:"cpipes_mode,omitempty"`
	Client            string            `json:"client,omitempty"`
	Org               string            `json:"org,omitempty"`
	ObjectType        string            `json:"object_type,omitempty"`
	FileKey           string            `json:"file_key,omitempty"`
	SessionId         string            `json:"session_id,omitempty"`
	MainInputStepId   string            `json:"read_step_id,omitempty"`
	MergeFiles        bool              `json:"merge_files"`
	InputSessionId    string            `json:"input_session_id,omitempty"`
	SourcePeriodKey   int               `json:"source_period_key"`
	ProcessName       string            `json:"process_name,omitempty"`
	SourcesConfig     SourcesConfigSpec `json:"sources_config"`
	PipelineConfigKey int               `json:"pipeline_config_key"`
	UserEmail         string            `json:"user_email,omitempty"`
}

// SourcesConfigSpec contains carry over configuration from
// table source_config. It has provision for multiple input
// sources to be merged via domian keys
type SourcesConfigSpec struct {
	MainInput     *InputSourceSpec   `json:"main_input"`
	MergedInput   []*InputSourceSpec `json:"merged_inputs"`
	InjectedInput []*InputSourceSpec `json:"injected_inputs"`
}

// InputColumns correspond to columns in the input files, this
// applies to reducing as well as sharding steps.
// For the case of sharding step, it includes columns from part files key.
// DomainKeys is taken from:
//   - source_config table or main schema provider for source_type = 'file'
//   - domain_keys_registry table or schema_provider / input_source_spec for source_type = 'domain_table'
//
// DomainClass is taken from:
//   - domain_keys_registry table or schema_provider / input_source_spec for source_type = 'domain_table'
//
// Note: for source_type = 'file', DomainClass does not apply, the file needs to be mapped first.
type InputSourceSpec struct {
	InputColumns       []string           `json:"input_columns"`
	InputParquetSchema *ParquetSchemaInfo `json:"input_parquet_schema,omitempty"`
	DomainClass        string             `json:"domain_class,omitempty"`
	DomainKeys         *DomainKeysSpec    `json:"domain_keys_spec,omitempty"`
}

type ParquetSchemaInfo struct {
	Schema      string `json:"schema"`
	Compression string `json:"compression,omitempty"`
}

type FileKeyInfo struct {
	key   string
	size  int
	start int
	end   int
}

// Full arguments to cp_node for sharding and reducing
type ComputePipesArgs struct {
	ComputePipesNodeArgs
	ComputePipesCommonArgs
}

// Write the compute pipes arguments as json to s3 at location specified by s3Location
// This is currently not used, needed for Distributed Map
func WriteCpipesArgsToS3(cpa []ComputePipesNodeArgs, s3Location string) error {
	cpJson, err := json.Marshal(cpa)
	if err != nil {
		return fmt.Errorf("while marshalling cpipes arg to json: %v", err)
	}
	f, err := os.CreateTemp("", "cp_args")
	if err != nil {
		return err
	}
	defer os.Remove(f.Name()) // clean up
	if _, err := f.Write(cpJson); err != nil {
		return err
	}
	_, err = f.Seek(0, 0)
	if err != nil {
		return err
	}
	err = awsi.UploadToS3(os.Getenv("JETS_BUCKET"), os.Getenv("JETS_REGION"), s3Location, f)
	if err != nil {
		return fmt.Errorf("while copying cpipes arg to s3: %v", err)
	}
	if err := f.Close(); err != nil {
		return err
	}
	return nil
}

// This is currently not used, needed for Distributed Map (for testing purpose)
func ReadCpipesArgsFromS3(s3Location string) ([]ComputePipesNodeArgs, error) {
	f, err := os.CreateTemp("", "cp_args")
	if err != nil {
		return nil, err
	}
	defer os.Remove(f.Name()) // clean up

	// Download the object
	_, err = awsi.DownloadFromS3(os.Getenv("JETS_BUCKET"), os.Getenv("JETS_REGION"), s3Location, f)
	if err != nil {
		return nil, fmt.Errorf("failed to download cpipes arg from s3: %v", err)
	}
	fname := f.Name()
	err = f.Close()
	if err != nil {
		return nil, err
	}
	buf, err := os.ReadFile(fname)
	if err != nil {
		return nil, fmt.Errorf("failed to read cpipes arg downloaded from s3: %v", err)
	}
	var cpa []ComputePipesNodeArgs
	err = json.Unmarshal(buf, &cpa)
	if err != nil {
		return nil, fmt.Errorf("while unmarshaling cpipes arg downloaded from s3: %s", err)
	}
	return cpa, nil
}

// runReportsCommand := []string{
// 	"-processName", processName.(string),
// 	"-sessionId", sessionId.(string),
// 	"-filePath", strings.Replace(fileKey.(string), os.Getenv("JETS_s3_INPUT_PREFIX"), os.Getenv("JETS_s3_OUTPUT_PREFIX"), 1),
// }
// status_update arguments:
// map[string]interface{}
// {
//  "-peKey": peKey,
//  "-status": "completed",
//  "failureDetails": {...}
// }

// Returned by the cp_starter for a cpipes run
// CpipesCommandsS3Key is for Distributed Map, currently not used
type ComputePipesRun struct {
	CpipesCommands       interface{}            `json:"cpipesCommands"`
	CpipesCommandsS3Key  string                 `json:"cpipesCommandsS3Key,omitempty"`
	CpipesMaxConcurrency int                    `json:"cpipesMaxConcurrency"`
	StartReducing        StartComputePipesArgs  `json:"startReducing"`
	IsLastReducing       bool                   `json:"isLastReducing"`
	UseECSReducingTask   bool                   `json:"useECSReducingTask"`
	ReportsCommand       []string               `json:"reportsCommand"`
	SuccessUpdate        map[string]interface{} `json:"successUpdate"`
	ErrorUpdate          map[string]interface{} `json:"errorUpdate"`
}

type FileName struct {
	LocalFileName string
	InFileKeyInfo FileKeyInfo
}

type CompiledPartFileComponent struct {
	ColumnName string
	Regex      *regexp.Regexp
}

type ComputePipesContext struct {
	ComputePipesArgs
	CpConfig              *ComputePipesConfig
	FileKeyComponents     map[string]interface{}
	PartFileKeyComponents []CompiledPartFileComponent
	AddionalInputHeaders  []string
	EnvSettings           map[string]interface{}
	SamplingCount         int
	InputFileKeys         []*FileKeyInfo
	ChResults             *ChannelResults
	KillSwitch            chan struct{}
	Done                  chan struct{}
	ErrCh                 chan error
	FileNamesCh           chan FileName
	DownloadS3ResultCh    chan DownloadS3Result // avoid to modify ChannelResult for now...
	S3DeviceMgr           *S3DeviceManager
	SchemaManager         *SchemaManager
}
