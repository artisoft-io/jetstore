package compute_pipes

import (
	"encoding/json"
	"fmt"
	"os"
	"regexp"

	"github.com/artisoft-io/jetstore/jets/awsi"
)

// Common functions and types for cp lambda version

// Argument to start_cp (start_sharding_cp, start_reducing_cp)
// for starting the cp cluster
// MaxConcurrency is to have a specified value of max concurrency
type StartComputePipesArgs struct {
	PipelineExecKey int     `json:"pipeline_execution_key"`
	FileKey         string  `json:"file_key"`
	SessionId       string  `json:"session_id"`
	InputStepId     *string `json:"input_step_id"`
	CurrentStep     *int    `json:"current_step"`
	UseECSTask      bool    `json:"use_ecs_tasks"`
	MaxConcurrency  int     `json:"max_concurrency"`
}

type InputStats struct {
	TotalPartfileCount int
	TotalSizeMb        int
}

// Arguments to start cp_node for sharding and reducing
// minimal set of arguments to reduce the size of the json
// to call the lambda functions
type ComputePipesNodeArgs struct {
	NodeId             int      `json:"id"`
	JetsPartitionLabel string   `json:"jp"`
	PipelineExecKey    int      `json:"pe"`
}

// Common arguments factored out and put into the
// the ComputePipesConfig component
type ComputePipesCommonArgs struct {
	Client             string   `json:"client"`
	Org                string   `json:"org"`
	ObjectType         string   `json:"object_type"`
	FileKey            string   `json:"file_key"`
	SessionId          string   `json:"session_id"`
	StepId             string   `json:"step_id"`
	InputSessionId     string   `json:"input_session_id"`
	SourcePeriodKey    int      `json:"source_period_key"`
	ProcessName        string   `json:"process_name"`
	InputColumns       []string `json:"input_columns"`
	PipelineConfigKey  int      `json:"pipeline_config_key"`
	UserEmail          string   `json:"user_email"`
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
	InFileKey     string
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
	ChResults             *ChannelResults
	Done                  chan struct{}
	ErrCh                 chan error
	FileNamesCh           chan FileName
	DownloadS3ResultCh    chan DownloadS3Result // avoid to modify ChannelResult for now...
	S3DeviceMgr           *S3DeviceManager
}
