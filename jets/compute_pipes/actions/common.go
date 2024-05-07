package actions

import (
	"regexp"

	"github.com/artisoft-io/jetstore/jets/compute_pipes"
)

// Common functions and types for cp lambda version

// Argument to start_cp for starting the cp cluster
type StartComputePipesArgs struct {
	PipelineExecKey int    `json:"pipeline_execution_key"`
	FileKey         string `json:"file_key"`
	SessionId       string `json:"session_id"`
}

// Argument to cp_node for sharding and reducing
type ComputePipesArgs struct {
	NodeId             int      `json:"node_id"`
	CpipesMode         string   `json:"cpipes_mode"`
	NbrNodes           int      `json:"nbr_nodes"`
	JetsPartitionLabel string   `json:"jets_partition_label"`
	Client             string   `json:"client"`
	Org                string   `json:"org"`
	ObjectType         string   `json:"object_type"`
	InputSessionId     string   `json:"input_session_id"`
	SessionId          string   `json:"session_id"`
	SourcePeriodKey    int      `json:"source_period_key"`
	ProcessName        string   `json:"process_name"`
	FileKey            string   `json:"file_key"`
	InputColumns       []string `json:"input_columns"`
	PipelineExecKey    int      `json:"pipeline_execution_key"`
	PipelineConfigKey  int      `json:"pipeline_config_key"`
	UserEmail          string   `json:"user_email"`
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
type ComputePipesRun struct {
	CpipesCommands []ComputePipesArgs     `json:"cpipesCommands"`
	StartReducing  StartComputePipesArgs  `json:"startReducing"`
	ReportsCommand []string               `json:"reportsCommand"`
	SuccessUpdate  map[string]interface{} `json:"successUpdate"`
	ErrorUpdate    map[string]interface{} `json:"errorUpdate"`
}

type FileName struct {
	LocalFileName string
	InFileKey     string
}

type CompiledPartFileComponent struct {
	ColumnName string
	Regex      *regexp.Regexp
}

type ComputePipesContext struct {
	ComputePipesArgs
	CpConfig              *compute_pipes.ComputePipesConfig
	FileKeyComponents     map[string]interface{}
	PartFileKeyComponents []CompiledPartFileComponent
	EnvSettings           map[string]interface{}
	ChResults             *compute_pipes.ChannelResults
	Done                  chan struct{}
	ErrCh                 chan error
	FileNamesCh           chan FileName
	DownloadS3ResultCh    chan DownloadS3Result // avoid to modify ChannelResult for now...
}

// Struct used in input_columns_json of table source_config for cpipes
type InputColumnsDef struct {
	ShardingInput []string `json:"sharding_input"`
	ReducingInput []string `json:"reducing_input"`
}
