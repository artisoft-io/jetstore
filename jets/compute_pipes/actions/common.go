package actions

import (
	"github.com/artisoft-io/jetstore/jets/compute_pipes"
)

// Common functions and types for cp lambda version

type ComputePipesArgs struct {
	NodeId             int      `json:"node_id"`
	CpipesMode         string   `json:"cpipes_mode"`
	NbrNodes           int      `json:"nbr_nodes"`
	JetsPartition      int      `json:"jets_partition"`
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

type FileName struct {
	LocalFileName string
	InFileKey     string
}

type ComputePipesContext struct {
	ComputePipesArgs
	CpConfig           *compute_pipes.ComputePipesConfig
	FileKeyComponents  map[string]interface{}
	EnvSettings        map[string]interface{}
	ChResults          *compute_pipes.ChannelResults
	Done               chan struct{}
	ErrCh              chan error
	FileNamesCh        chan FileName
	DownloadS3ResultCh chan DownloadS3Result // avoid to modify ChannelResult for now...
}
