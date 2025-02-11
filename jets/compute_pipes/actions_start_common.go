package compute_pipes

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"strconv"

	"github.com/artisoft-io/jetstore/jets/workspace"
	"github.com/jackc/pgx/v4/pgxpool"
)

var workspaceHome, wsPrefix string

func init() {
	workspaceHome = os.Getenv("WORKSPACES_HOME")
	wsPrefix = os.Getenv("WORKSPACE")
}

// Collect and prepare cpipes configuration for both sharding and reducing steps.
// InputColumns correspond to the domain column in the main input file, which
// can be a subset of the columns in the main_input schema provider based on
// source_config table.
// InputColumns can be empty if needs to be read from the input file.
type CpipesStartup struct {
	CpConfig                      ComputePipesConfig
	ProcessName                   string
	InputColumns                  []string
	MainInputSchemaProviderConfig *SchemaProviderSpec
	EnvSettings                   map[string]any
	PipelineConfigKey             int
	InputSessionId                string
	SourcePeriodKey               int
	OperatorEmail                 string
}

func (args *StartComputePipesArgs) initializeCpipes(ctx context.Context, dbpool *pgxpool.Pool) (*CpipesStartup, error) {
	cpipesStartup := &CpipesStartup{}
	var err error

	// Check if we need to sync the workspace files
	_, err = workspace.SyncComputePipesWorkspace(dbpool)
	if err != nil {
		log.Panicf("error while synching workspace files from db: %v", err)
	}

	// get pe info and pipeline config
	// cpipesConfigFN is file name within workspace
	var client, org, objectType, inputFormat, compression string
	var schemaProviderJson string
	var isPartFile int
	var cpipesConfigFN, icJson, icPosCsv, inputFormatDataJson sql.NullString
	log.Println("CPIPES, loading pipeline configurations")
	stmt := `
	SELECT	ir.client, ir.org, ir.object_type, ir.source_period_key, ir.schema_provider_json, 
		pe.pipeline_config_key, pe.process_name, pe.input_session_id, pe.user_email,
		sc.input_columns_json, sc.input_columns_positions_csv, sc.input_format, sc.compression, 
		sc.is_part_files, sc.input_format_data_json,
		pc.main_rules
	FROM 
		jetsapi.pipeline_execution_status pe,
		jetsapi.input_registry ir,
		jetsapi.source_config sc,
		jetsapi.process_config pc
	WHERE pe.main_input_registry_key = ir.key
		AND pe.key = $1
		AND sc.client = ir.client
		AND sc.org = ir.org
		AND sc.object_type = ir.object_type
		AND pc.process_name = pe.process_name`
	err = dbpool.QueryRow(ctx, stmt, args.PipelineExecKey).Scan(
		&client, &org, &objectType, &cpipesStartup.SourcePeriodKey, &schemaProviderJson,
		&cpipesStartup.PipelineConfigKey, &cpipesStartup.ProcessName, &cpipesStartup.InputSessionId, &cpipesStartup.OperatorEmail,
		&icJson, &icPosCsv, &inputFormat, &compression, &isPartFile, &inputFormatDataJson,
		&cpipesConfigFN)
	if err != nil {
		return cpipesStartup, fmt.Errorf("query pipeline configurations failed: %v", err)
	}
	if !cpipesConfigFN.Valid || len(cpipesConfigFN.String) == 0 {
		return cpipesStartup, fmt.Errorf("error: process_config table does not have a cpipes config file name in main_rules column")
	}

	// Get the cpipes_config json from workspace
	configFile := fmt.Sprintf("%s/%s/%s", workspaceHome, wsPrefix, cpipesConfigFN.String)
	cpJson, err := os.ReadFile(configFile)
	if err != nil {
		return cpipesStartup, fmt.Errorf("while reading cpipes config from workspace: %v", err)
	}
	err = json.Unmarshal(cpJson, &cpipesStartup.CpConfig)
	if err != nil {
		return cpipesStartup, fmt.Errorf("while unmarshaling compute pipes json (StartShardingComputePipes): %s", err)
	}
	// Adjust ChannelSpec having columns specified by a jetrules class
	for i := range cpipesStartup.CpConfig.Channels {
		chSpec := &cpipesStartup.CpConfig.Channels[i]
		if len(chSpec.ClassName) > 0 {
			// Get the columns from the local workspace
			columns, err := GetDomainProperties(chSpec.ClassName, chSpec.DirectPropertiesOnly)
			if err != nil {
				return cpipesStartup, fmt.Errorf(
					"while getting domain properties for channel spec class name %s: %v (does workspace_control.json needs to be updated?)",
					chSpec.ClassName, err)
			}
			if len(chSpec.Columns) > 0 {
				columns = append(columns, chSpec.Columns...)
			}
			chSpec.Columns = columns
		}
	}

	// Get the schema provider from schemaProviderJson:
	//   - Populate the input columns (cpipesStartup.InputColumns)
	//   - Populate inputFormat, compression, delimiter, detect_encoding
	//   - Populate inputFormatDataJson for xlsx
	//   - Put SchemaName into env (done in CoordinateComputePipes)
	//   - Put the schema provider in compute pipes json
	// First find if a schema provider already exist for "main_input"
	for _, sp := range cpipesStartup.CpConfig.SchemaProviders {
		if sp.SourceType == "main_input" {
			cpipesStartup.MainInputSchemaProviderConfig = sp
			break
		}
	}
	if cpipesStartup.MainInputSchemaProviderConfig == nil {
		// Create and initialize a default SchemaProviderSpec
		cpipesStartup.MainInputSchemaProviderConfig = &SchemaProviderSpec{
			Type:                "default",
			Key:                 "_main_input_",
			SourceType:          "main_input",
			Client:              client,
			Vendor:              org,
			ObjectType:          objectType,
			Format:              inputFormat,
			Compression:         compression,
			InputFormatDataJson: inputFormatDataJson.String,
		}
		if isPartFile == 1 {
			cpipesStartup.MainInputSchemaProviderConfig.IsPartFiles = true
		}
		if cpipesStartup.CpConfig.SchemaProviders == nil {
			cpipesStartup.CpConfig.SchemaProviders = make([]*SchemaProviderSpec, 0)
		}
		cpipesStartup.CpConfig.SchemaProviders = append(cpipesStartup.CpConfig.SchemaProviders, cpipesStartup.MainInputSchemaProviderConfig)
	} else {
		// Initialize unspecified value in main schema provider using the source_config table values
		if cpipesStartup.MainInputSchemaProviderConfig.Client == "" {
			cpipesStartup.MainInputSchemaProviderConfig.Client = client
		}
		if cpipesStartup.MainInputSchemaProviderConfig.Vendor == "" {
			cpipesStartup.MainInputSchemaProviderConfig.Vendor = org
		}
		if cpipesStartup.MainInputSchemaProviderConfig.ObjectType == "" {
			cpipesStartup.MainInputSchemaProviderConfig.ObjectType = objectType
		}
		if cpipesStartup.MainInputSchemaProviderConfig.Format == "" {
			cpipesStartup.MainInputSchemaProviderConfig.Format = inputFormat
		}
		if cpipesStartup.MainInputSchemaProviderConfig.Compression == "" {
			cpipesStartup.MainInputSchemaProviderConfig.Compression = compression
		}
		if cpipesStartup.MainInputSchemaProviderConfig.InputFormatDataJson == "" {
			cpipesStartup.MainInputSchemaProviderConfig.InputFormatDataJson = inputFormatDataJson.String
		}
	}
	mainInputSchemaProvider := cpipesStartup.MainInputSchemaProviderConfig
	if len(schemaProviderJson) > 0 {
		err = json.Unmarshal([]byte(schemaProviderJson), mainInputSchemaProvider)
		if err != nil {
			return cpipesStartup, fmt.Errorf("while unmarshaling schema_provider_json: %s", err)
		}
	}
	cpipesStartup.EnvSettings = PrepareCpipesEnv(&cpipesStartup.CpConfig, mainInputSchemaProvider)

	// The main_input schema provider should always have the key _main_input_.
	mainInputSchemaProvider.Key = "_main_input_"
	ic := &cpipesStartup.CpConfig.ReducingPipesConfig[0][0].InputChannel

	// The file compression is specified from input_channel, if not take it from main schema provider,
	// if not it taken from input_source table above
	if ic.Compression == "" {
		ic.Compression = mainInputSchemaProvider.Compression
	} else {
		// Override the compression from schema provider and from input_source table
		// Note: not expected to have to do this, usually the schema provider will have
		// the right value. This is for completness and to ensure everything is in sync
		mainInputSchemaProvider.Compression = ic.Compression
	}

	// The csv delimiter is specified from input_channel, if not take it from main schema provider
	if ic.Delimiter == 0 {
		ic.Delimiter = mainInputSchemaProvider.Delimiter
	} else {
		// Override schema provider with value specified in input_channel config
		mainInputSchemaProvider.Delimiter = ic.Delimiter
	}

	// File format
	if ic.Format == "" {
		ic.Format = mainInputSchemaProvider.Format
	} else {
		mainInputSchemaProvider.Format = ic.Format
	}

	// Input channel for sharding step always have the _main_input_ schema provider
	ic.SchemaProvider = mainInputSchemaProvider.Key

	// Set the fixed_width column spec to the schema provider
	if len(icPosCsv.String) > 0 {
		mainInputSchemaProvider.FixedWidthColumnsCsv = icPosCsv.String
	}

	// InputColumns - the main input file domain columns, order of priority:
	//	- Take the columns from source_config table if specified.
	//	- Take the columns from schema provider if specified.
	//  - Otherwise, leave it empty. They will be taken from the first input file.
	// NOTE: case of fixed_width: take the columns from the icPosCsv (fixed width spec)
	//*TODO Read a subset of the fixed_width columns
	// Need to initialize the schema provider to get the column info
	sp := NewDefaultSchemaProvider()
	err = sp.Initialize(dbpool, mainInputSchemaProvider, nil, cpipesStartup.CpConfig.ClusterConfig.IsDebugMode)
	if err != nil {
		return cpipesStartup, fmt.Errorf("while initializing schema provider to get fixed_width headers: %s", err)
	}
	switch {
	case mainInputSchemaProvider.Format == "fixed_width":
		cpipesStartup.InputColumns, _ = sp.FixedWidthFileHeaders()
	case len(icJson.String) > 0:
		// Get the input columns info
		err = json.Unmarshal([]byte(icJson.String), &cpipesStartup.InputColumns)
		if err != nil {
			return cpipesStartup, fmt.Errorf("while unmarshaling input_columns_json: %s", err)
		}
	case len(mainInputSchemaProvider.Columns) > 0:
		// Get the input columns from the schema provider
		cpipesStartup.InputColumns = sp.ColumnNames()
	}

	return cpipesStartup, nil
}

func GetMaxConcurrency(nbrNodes, defaultMaxConcurrency int) int {
	if nbrNodes < 1 {
		return 1
	}
	if defaultMaxConcurrency == 0 {
		v := os.Getenv("TASK_MAX_CONCURRENCY")
		if v != "" {
			var err error
			defaultMaxConcurrency, err = strconv.Atoi(os.Getenv("TASK_MAX_CONCURRENCY"))
			if err != nil {
				defaultMaxConcurrency = 10
			}
		}
	}

	maxConcurrency := defaultMaxConcurrency
	if maxConcurrency < 1 {
		maxConcurrency = 1
	}
	return maxConcurrency
}

// Function to prune the lookupConfig and return only the lookup used in the pipeConfig
// Returns an error if pipeConfig has reference to a lookup not in lookupConfig
func SelectActiveLookupTable(lookupConfig []*LookupSpec, pipeConfig []PipeSpec) ([]*LookupSpec, error) {
	// get a mapping of lookup table name to lookup table spec -- all lookup tables
	lookupMap := make(map[string]*LookupSpec)
	for _, config := range lookupConfig {
		if config != nil {
			lookupMap[config.Key] = config
		}
	}
	// Identify the used lookup tables in this step
	activeTables := make([]*LookupSpec, 0)
	for i := range pipeConfig {
		for j := range pipeConfig[i].Apply {
			transformationSpec := &pipeConfig[i].Apply[j]
			// Check for column transformation of type lookup
			for k := range transformationSpec.Columns {
				name := pipeConfig[i].Apply[j].Columns[k].LookupName
				if name != nil {
					spec := lookupMap[*name]
					if spec == nil {
						return nil,
							fmt.Errorf("error: lookup table '%s' is not defined, please verify the column transformation", *name)
					}
					activeTables = append(activeTables, spec)
				}
			}
			switch transformationSpec.Type {
			case "analyze":
				// Check for Analyze transformation using lookup tables
				if transformationSpec.AnalyzeConfig != nil && transformationSpec.AnalyzeConfig.LookupTokens != nil {
					for k := range transformationSpec.AnalyzeConfig.LookupTokens {
						lookupTokenNode := &transformationSpec.AnalyzeConfig.LookupTokens[k]
						spec := lookupMap[lookupTokenNode.Name]
						if spec == nil {
							return nil,
								fmt.Errorf(
									"error: lookup table '%s' is not defined, please verify the column transformation", lookupTokenNode.Name)
						}
						activeTables = append(activeTables, spec)
					}
				}
			case "anonymize":
				// Check for Anonymize transformation using lookup tables
				if transformationSpec.AnonymizeConfig != nil {
					name := transformationSpec.AnonymizeConfig.LookupName
					if len(name) > 0 {
						spec := lookupMap[name]
						if spec == nil {
							return nil,
								fmt.Errorf(
									"error: lookup table '%s' used by anonymize operator is not defined, please verify the configuration", name)
						}
						activeTables = append(activeTables, spec)
					}
				}
			case "shuffling":
				// Check for Shuffling transformation using lookup tables
				if transformationSpec.ShufflingConfig != nil && transformationSpec.ShufflingConfig.FilterColumns != nil {
					name := transformationSpec.ShufflingConfig.FilterColumns.LookupName
					if len(name) > 0 {
						spec := lookupMap[name]
						if spec == nil {
							return nil,
								fmt.Errorf(
									"error: lookup table '%s' used by shuffling operator is not defined, please verify the configuration", name)
						}
						activeTables = append(activeTables, spec)
					}
				}
			case "clustering":
				// Check for Clustering transformation using lookup tables
				if transformationSpec.ClusteringConfig != nil {
					name := transformationSpec.ClusteringConfig.TargetColumnsLookup.LookupName
					if len(name) > 0 {
						spec := lookupMap[name]
						if spec == nil {
							return nil,
								fmt.Errorf(
									"error: lookup table '%s' used by clustering operator is not defined, please verify the configuration", name)
						}
						activeTables = append(activeTables, spec)
					}
				}
			}
		}
	}
	return activeTables, nil
}

// Function to prune the output tables and return only the tables used in pipeConfig
// Returns an error if pipeConfig makes reference to a non-existent table
func SelectActiveOutputTable(tableConfig []*TableSpec, pipeConfig []PipeSpec) ([]*TableSpec, error) {
	// get a mapping of table name to table spec
	tableMap := make(map[string]*TableSpec)
	for i := range tableConfig {
		if tableConfig[i] != nil {
			tableMap[tableConfig[i].Key] = tableConfig[i]
		}
	}
	// Identify the used tables
	activeTables := make([]*TableSpec, 0)
	for i := range pipeConfig {
		for j := range pipeConfig[i].Apply {
			transformationSpec := &pipeConfig[i].Apply[j]
			if len(transformationSpec.OutputChannel.OutputTableKey) > 0 {
				spec := tableMap[transformationSpec.OutputChannel.OutputTableKey]
				if spec == nil {
					return nil, fmt.Errorf(
						"error: Output Table spec %s not found, is used in output_channel",
						transformationSpec.OutputChannel.OutputTableKey)
				}
				activeTables = append(activeTables, spec)
			}
		}
	}
	return activeTables, nil
}

func GetOutputFileConfig(cpConfig *ComputePipesConfig, outputFileKey string ) *OutputFileSpec {
	for i := range cpConfig.OutputFiles {
		if outputFileKey == cpConfig.OutputFiles[i].Key {
			return &cpConfig.OutputFiles[i]
		}
	}
	return nil
}

// Function to validate the PipeSpec output channel config
// Apply a default snappy compression if compression is not specified
// and channel Type 'stage'
func ValidatePipeSpecConfig(cpConfig *ComputePipesConfig, pipeConfig []PipeSpec) error {
	for i := range pipeConfig {
		pipeSpec := &pipeConfig[i]
		// log.Printf("VALIDATE PIPESPEC %s\n", pipeSpec.Type)
		switch pipeSpec.InputChannel.Type {
		case "stage":
			if len(pipeSpec.InputChannel.SchemaProvider) > 0 {
				sp := getSchemaProvider(cpConfig.SchemaProviders, pipeSpec.InputChannel.SchemaProvider)
				if sp == nil {
					return fmt.Errorf("error: invalid cpipes config. input_channel has reference to "+
						"schema_provider %s, but does not exists", pipeSpec.InputChannel.SchemaProvider)
				}
				if len(pipeSpec.InputChannel.Format) == 0 {
					pipeSpec.InputChannel.Format = sp.Format
				}
				if pipeSpec.InputChannel.Delimiter == 0 {
					pipeSpec.InputChannel.Delimiter = sp.Delimiter
				}
				if len(pipeSpec.InputChannel.Compression) == 0 {
					pipeSpec.InputChannel.Compression = sp.Compression
				}
			}
		}
		// Check that we don't have two input channel reading from the same channel,
		// this creates record lost since they steal records from each other
		for k := range pipeConfig {
			if i != k && pipeSpec.InputChannel.Name == pipeConfig[k].InputChannel.Name {
				return fmt.Errorf("error: invalid cpipes config. two input_channel reading from "+
					"the same channel %s, this will create record loss", pipeSpec.InputChannel.Name)
			}
		}
		// PipeSpec Type specific validations
		switch pipeSpec.Type {
		case "merge_files":
			if pipeSpec.OutputFile == nil || len(*pipeSpec.OutputFile) == 0 {
				return fmt.Errorf("error: merge_file must have output_file set")
			}
			outputFileSpec := GetOutputFileConfig(cpConfig, *pipeSpec.OutputFile)
			if outputFileSpec == nil {
				return fmt.Errorf("error: Output file config '%s' not found", *pipeSpec.OutputFile)
			}
			if outputFileSpec.OutputLocation == "" {
				outputFileSpec.OutputLocation = "jetstore_s3_output"
			}
		}
		for j := range pipeSpec.Apply {
			transformationConfig := &pipeSpec.Apply[j]
			outputChConfig := &transformationConfig.OutputChannel
			// log.Printf("VALIDATE PIPESPEC %s APPLY %s OUTPUT %s SP %s\n", pipeSpec.Type, transformationConfig.Type, transformationConfig.OutputChannel.Name, transformationConfig.OutputChannel.SchemaProvider)
			sp := getSchemaProvider(cpConfig.SchemaProviders, outputChConfig.SchemaProvider)
			// validate transformation pipe config
			switch transformationConfig.Type {
			case "partition_writer":
				if transformationConfig.PartitionWriterConfig == nil {
					return fmt.Errorf(
						"error: invalid cpipes config, must provide 'partition_writer_config'" +
							" for transformation pipe of type 'partition_writer'")
				}
				config := transformationConfig.PartitionWriterConfig
				if config.DeviceWriterType == "" && sp == nil {
					return fmt.Errorf(
						"error: invalid cpipes config, must provide 'device_writer_type' or 'output_channel.schema_provider'"+
							" for output channel %s of transformation pipe of type 'partition_writer'", outputChConfig.Name)
				}
				if config.DeviceWriterType == "" {
					var deviceWriterType string
					switch sp.Format {
					case "csv", "headerless_csv":
						deviceWriterType = "csv_writer"
					case "parquet", "parquet_select":
						deviceWriterType = "parquet_writer"
					case "fixed_width":
						deviceWriterType = "fixed_width_writer"
					default:
						err := fmt.Errorf("error: unsupported output file format: %s (in NewPartitionWriterTransformationPipe)", sp.Format)
						log.Println(err)
						return err
					}
					config.DeviceWriterType = deviceWriterType
					outputChConfig.Format = sp.Format
				}
			case "anonymize":
				if transformationConfig.AnonymizeConfig == nil {
					return fmt.Errorf("error: cpipes config is missing anonymize_config for anonymize operator")
				}
				keyOutputChannel := &transformationConfig.AnonymizeConfig.KeysOutputChannel
				err := validateOutputChConfig(keyOutputChannel, getSchemaProvider(cpConfig.SchemaProviders, keyOutputChannel.SchemaProvider))
				if err != nil {
					return err
				}
			case "jetrules":
				if transformationConfig.JetrulesConfig == nil {
					return fmt.Errorf("error: cpipes config is missing jetrules_config for jetrules operator")
				}
				if transformationConfig.JetrulesConfig.PoolSize < 1 {
					log.Println("WARNING: jetrules pool worker size is unset, setting to 1")
					transformationConfig.JetrulesConfig.PoolSize = 1
				}
				outputChConfig = nil // The outputChannel is replaced by JetrulesConfig.JetrulesOutput channels
				for k := range transformationConfig.JetrulesConfig.OutputChannels {
					outCh := &transformationConfig.JetrulesConfig.OutputChannels[k]
					err := validateOutputChConfig(outCh, getSchemaProvider(cpConfig.SchemaProviders, outCh.SchemaProvider))
					if err != nil {
						return err
					}
				}
			case "clustering":
				if transformationConfig.ClusteringConfig == nil ||
					transformationConfig.ClusteringConfig.CorrelationOutputChannel == nil {
					return fmt.Errorf(
						"error: cpipes config is missing clustering_config or correlation_output_channel for clustering operator")
				}
				outCh := transformationConfig.ClusteringConfig.CorrelationOutputChannel
				err := validateOutputChConfig(outCh, getSchemaProvider(cpConfig.SchemaProviders, outCh.SchemaProvider))
				if err != nil {
					return err
				}
			}
			err := validateOutputChConfig(outputChConfig, sp)
			if err != nil {
				return err
			}
		}
	}
	return nil
}

func validateOutputChConfig(outputChConfig *OutputChannelConfig, sp *SchemaProviderSpec) error {
	if outputChConfig == nil {
		return nil
	}
	if outputChConfig.Type == "" {
		outputChConfig.Type = "memory"
	}
	switch outputChConfig.Type {
	case "sql":
		if len(outputChConfig.OutputTableKey) == 0 {
			return fmt.Errorf("error: invalid cpipes config, must provide output_table_key when output_channel type is 'sql'")
		}
		outputChConfig.Name = outputChConfig.OutputTableKey
		outputChConfig.SpecName = outputChConfig.OutputTableKey
	default:
		if len(outputChConfig.Name) == 0 || outputChConfig.Name == outputChConfig.SpecName {
			return fmt.Errorf(
				"error: invalid cpipes config, output_channel.name '%s' must not be empty or same as output_channel.channel_spec_name '%s'",
				outputChConfig.Name, outputChConfig.SpecName)
		}
		switch outputChConfig.Type {
		case "stage":
			if outputChConfig.Format == "" {
				if sp != nil {
					outputChConfig.Format = sp.Format
				}
				if outputChConfig.Format == "" {
					outputChConfig.Format = "headerless_csv"
				}
			}
			if outputChConfig.Compression == "" {
				if sp != nil {
					outputChConfig.Compression = sp.Compression
				}
				if outputChConfig.Compression == "" {
					outputChConfig.Compression = "snappy"
				}
			}
			if outputChConfig.Delimiter == 0 {
				if sp != nil {
					outputChConfig.Delimiter = sp.Delimiter
				}
			}
			if len(outputChConfig.WriteStepId) == 0 {
				return fmt.Errorf("error: invalid cpipes config, write_step_id is not specified in output_channel '%s' of type 'stage'",
					outputChConfig.Name)
			}
		case "output":
			if outputChConfig.Format == "" {
				if sp != nil {
					outputChConfig.Format = sp.Format
				}
				if outputChConfig.Format == "" {
					return fmt.Errorf("error: invalid cpipes config, format is not specified in output_channel '%s' of type 'output'",
						outputChConfig.Name)
				}
			}
			if outputChConfig.Delimiter == 0 {
				if sp != nil {
					outputChConfig.Delimiter = sp.Delimiter
				}
			}
			if outputChConfig.Compression == "" {
				if sp != nil {
					outputChConfig.Compression = sp.Compression
				}
				if outputChConfig.Compression == "" {
					outputChConfig.Compression = "none"
				}
			}
			if len(outputChConfig.OutputLocation) == 0 {
				outputChConfig.OutputLocation = "jetstore_s3_output"
			}
			switch outputChConfig.OutputLocation {
			case "jetstore_s3_input", "jetstore_s3_output":
			default:
				return fmt.Errorf(
					"error: invalid cpipes config, invalid output_location '%s' in output_channel '%s' of type"+
						" 'output', expecting jetstore_s3_input or jetstore_s3_output",
					outputChConfig.OutputLocation, outputChConfig.Name)
			}

		case "memory":
			outputChConfig.Format = ""
			outputChConfig.Compression = ""
			outputChConfig.Delimiter = 0
		default:
			return fmt.Errorf(
				"error: invalid cpipes config, unknown output_channel config type: %s (expecting: memory (default), stage, output, sql)",
				outputChConfig.Type)
		}
	}
	return nil
}

func getSchemaProvider(schemaProviders []*SchemaProviderSpec, key string) *SchemaProviderSpec {
	if key == "" {
		return nil
	}
	for _, sp := range schemaProviders {
		if sp.Key == key {
			return sp
		}
	}
	return nil
}

// Function to collect env settings from cpipes config and main schema provider.
// Important for site specific configuration, in particular used in API gateway notification
func PrepareCpipesEnv(cpConfig *ComputePipesConfig, mainSchemaProviderConfig *SchemaProviderSpec) map[string]any {
	//* IMPORTANT: Make sure a key is not the prefix of another key
	//  e.g. $FILE_KEY and $FILE_KEY_PATH is BAD since $FILE_KEY_PATH may get
	//  the value of $FILE_KEY with a dandling _PATH
	envSettings := map[string]any{
		"$INPUT_BUCKET":     mainSchemaProviderConfig.Bucket,
		"$MAIN_SCHEMA_NAME": mainSchemaProviderConfig.SchemaName,
	}

	for i := range cpConfig.Context {
		if cpConfig.Context[i].Type == "value" {
			envSettings[cpConfig.Context[i].Key] = cpConfig.Context[i].Expr
		}
	}
	for k, v := range mainSchemaProviderConfig.Env {
		envSettings[k] = v
	}
	if cpConfig.ClusterConfig.IsDebugMode {
		b, err := json.Marshal(envSettings)
		log.Printf("PrepareCpipesEnv: Cpipes Env: %s, err? %v\n", string(b), err)
	}
	return envSettings
}
