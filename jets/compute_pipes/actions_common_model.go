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
// MaxConcurrency is to have a specified value of max concurrency
type StartComputePipesArgs struct {
	PipelineExecKey int    `json:"pipeline_execution_key"`
	FileKey         string `json:"file_key"`
	SessionId       string `json:"session_id"`
	StepId          *int   `json:"step_id"`
	UseECSTask      bool   `json:"use_ecs_tasks"`
	MaxConcurrency  int    `json:"max_concurrency"`
}

type InputStats struct {
	TotalPartfileCount int
	TotalSizeMb        int
}

// Arguments to start cp_node for sharding and reducing
// minimal set of arguments to reduce the size of the json
// to call the lambda functions
type ComputePipesNodeArgs struct {
	NodeId             int    `json:"id"`
	JetsPartitionLabel string `json:"jp"`
	PipelineExecKey    int    `json:"pe"`
}

// Common arguments factored out and put into the
// the ComputePipesConfig component
// MergeFile property is to indicate that the pipe
// is at the stage of merging the part files into a single output file.
// This will use a single node since merge_files has a single partition
// to read from (the step_id prior to merge_files writes a single partition).
// Note: Client and Org are the pipeline execution client and org and may
//
//	be different than the client/vendor of the actual data (case using
//	stand-in client/org name). In that situation the actual
//	client/vendor of the data is specified at run time via the SchemaProviders
//	on table input_registry.
type ComputePipesCommonArgs struct {
	CpipesMode        string            `json:"cpipes_mode"`
	Client            string            `json:"client"`
	Org               string            `json:"org"`
	ObjectType        string            `json:"object_type"`
	FileKey           string            `json:"file_key"`
	SessionId         string            `json:"session_id"`
	MainInputStepId   string            `json:"read_step_id"`
	MergeFiles        bool              `json:"merge_files"`
	InputSessionId    string            `json:"input_session_id"`
	SourcePeriodKey   int               `json:"source_period_key"`
	ProcessName       string            `json:"process_name"`
	SourcesConfig     SourcesConfigSpec `json:"sources_config"`
	PipelineConfigKey int               `json:"pipeline_config_key"`
	UserEmail         string            `json:"user_email"`
}

// SourcesConfigSpec contains carry over configuration from
// table source_config. It has provision for multiple input
// sources to be merged via domian keys
type SourcesConfigSpec struct {
	MainInput     *InputSourceSpec   `json:"main_input"`
	MergedInput   []*InputSourceSpec `json:"merged_inputs"`
	InjectedInput []*InputSourceSpec `json:"injected_inputs"`
}

// InputSourceSpec contains carry over configuration from
// table source_config.
// SaveParquetSchema applied to parquet input files, it saves
// the schema at the root of the stage area under the current session_id.
// See SchemaProviderSpec for details on SchemaProvider
// InputSourceSpec properties override SchemaProviderSpec
// properties.
type InputSourceSpec struct {
	InputColumns        []string           `json:"input_columns"`
	ClassName           string             `json:"class_name,omitempty"`
	Format              string             `json:"format,omitempty"`
	SaveParquetSchema   bool               `json:"save_parquet_schema,omitempty"`
	Compression         string             `json:"compression,omitempty"`
	InputFormatDataJson string             `json:"input_format_data_json,omitempty"`
	SchemaProvider      string             `json:"schema_provider,omitempty"`
	InputParquetSchema  *ParquetSchemaInfo `json:"input_parquet_schema,omitempty"`
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
	CpipesCommandsS3Key  string                 `json:"cpipesCommandsS3Key"`
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
