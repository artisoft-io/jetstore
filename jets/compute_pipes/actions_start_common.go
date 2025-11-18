package compute_pipes

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"strconv"
	"strings"

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
// InputColumnsOriginal is the original input columns before uniquefying them.
// It is empty if InputColumns is empty or already unique.
// MainInputDomainKeysSpec contains the domain keys spec based on source_config
// table, which can be overriden by value from the main schema provider.
// MainInputDomainClass applies when input_registry.input_type = 'domain_table'
type CpipesStartup struct {
	CpConfig                      ComputePipesConfig         `json:"compute_pipes_config"`
	ProcessName                   string                     `json:"process_name,omitempty"`
	InputColumns                  []string                   `json:"input_columns,omitempty"`
	InputColumnsOriginal          []string                   `json:"input_columns_original,omitempty"`
	MainInputSchemaProviderConfig *SchemaProviderSpec        `json:"main_input_schema_provider_config,omitzero"`
	MainInputDomainKeysSpec       *DomainKeysSpec            `json:"main_input_domain_keys_spec,omitzero"`
	MainInputDomainClass          string                     `json:"main_input_domain_class,omitempty"`
	DomainKeysSpecByClass         map[string]*DomainKeysSpec `json:"domain_keys_spec_by_class,omitzero"`
	EnvSettings                   map[string]any             `json:"env_settings,omitzero"`
	PipelineConfigKey             int                        `json:"pipeline_config_key,omitempty"`
	InputSessionId                string                     `json:"input_session_id,omitempty"`
	SourcePeriodKey               int                        `json:"source_period_key,omitempty"`
	OperatorEmail                 string                     `json:"operator_email,omitempty"`
}

func (args *StartComputePipesArgs) reducingInitializeCpipes(ctx context.Context, dbpool *pgxpool.Pool) (*CpipesStartup, error) {
	// Get the cpipes startup info from cpipes_execution_status table
	stmt := "SELECT cpipes_startup_json, input_parquet_schema_json, input_row_columns_json FROM jetsapi.cpipes_execution_status WHERE session_id = $1"
	var cpipesStartupJson string
	var inputParquetSchemaJson string
	var inputRowColumnsJson string
	var cpipesStartup = CpipesStartup{}
	err := dbpool.QueryRow(ctx, stmt, args.SessionId).Scan(&cpipesStartupJson, &inputParquetSchemaJson, &inputRowColumnsJson)
	if err != nil {
		return nil, fmt.Errorf("error while getting cpipes startup info: %v", err)
	}

	// Unmarshal the cpipesStartupJson
	err = json.Unmarshal([]byte(cpipesStartupJson), &cpipesStartup)
	if err != nil {
		return nil, fmt.Errorf("error while unmarshalling cpipes startup json: %v", err)
	}

	// Connect the MainInputSchemaProviderConfig pointer
	for i := range cpipesStartup.CpConfig.SchemaProviders {
		if cpipesStartup.CpConfig.SchemaProviders[i].SourceType == "main_input" {
			cpipesStartup.MainInputSchemaProviderConfig = cpipesStartup.CpConfig.SchemaProviders[i]
			break
		}
	}
	if cpipesStartup.MainInputSchemaProviderConfig == nil {
		return nil, fmt.Errorf("error: main_input schema provider not found in cpipes config")
	}

	// Unmarshal the inputParquetSchemaJson into MainInputSchemaProvider.ParquetSchema
	var ParquetSchema ParquetSchemaInfo
	err = json.Unmarshal([]byte(inputParquetSchemaJson), &ParquetSchema)
	if err != nil {
		return nil, fmt.Errorf("while unmarshalling input_parquet_schema_json ->%s<-: %v", inputParquetSchemaJson, err)
	}
	cpipesStartup.MainInputSchemaProviderConfig.ParquetSchema = &ParquetSchema
	
	// Unmarshal the inputRowColumnsJson into InputColumns and InputColumnsOriginal
	var inputRowColumns InputRowColumns
	err = json.Unmarshal([]byte(inputRowColumnsJson), &inputRowColumns)
	if err != nil {
		return nil, fmt.Errorf("while unmarshalling input_row_columns_json ->%s<-: %v", inputRowColumnsJson, err)
	}
	cpipesStartup.InputColumns = inputRowColumns.MainInput
	cpipesStartup.InputColumnsOriginal = inputRowColumns.OriginalHeaders
	
	// Set the Cpipes env settings to be the same as the  schema provider env
	cpipesStartup.EnvSettings = cpipesStartup.MainInputSchemaProviderConfig.Env
	return &cpipesStartup, nil
}

func (args *StartComputePipesArgs) shardingInitializeCpipes(ctx context.Context, dbpool *pgxpool.Pool) (*CpipesStartup, error) {
	cpipesStartup := &CpipesStartup{}
	var err error

	// Check if we need to sync the workspace files
	_, err = workspace.SyncComputePipesWorkspace(dbpool)
	if err != nil {
		return nil, fmt.Errorf("error while synching workspace files from db: %v", err)
	}

	// get pe info and pipeline config
	// cpipesConfigFN is cpipes config file name within workspace
	// tableName is the input_registry.table_name, needed when source_type = 'domain_table' since it correspond to className
	var client, org, objectType, inputFormat, compression, tableName, sourceType string
	var schemaProviderJson string
	var isPartFile int
	var cpipesConfigFN, icJson, icPosCsv, icDomainKeys, inputFormatDataJson sql.NullString
	log.Println("CPIPES, loading pipeline configurations")
	stmt := `
	SELECT	ir.client, ir.org, ir.object_type, ir.source_period_key, ir.schema_provider_json, 
		ir.table_name, ir.source_type,
		pe.pipeline_config_key, pe.process_name, pe.input_session_id, pe.user_email,
		sc.input_columns_json, sc.input_columns_positions_csv, sc.domain_keys_json, 
		sc.input_format, sc.compression, sc.is_part_files, sc.input_format_data_json,
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
		&client, &org, &objectType, &cpipesStartup.SourcePeriodKey, &schemaProviderJson, &tableName, &sourceType,
		&cpipesStartup.PipelineConfigKey, &cpipesStartup.ProcessName, &cpipesStartup.InputSessionId, &cpipesStartup.OperatorEmail,
		&icJson, &icPosCsv, &icDomainKeys, &inputFormat, &compression, &isPartFile, &inputFormatDataJson,
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
		return cpipesStartup, fmt.Errorf("while unmarshaling compute pipes json (initializeCpipes): %s", err)
	}

	// Adjust ChannelSpec having columns specified by a jetrules class
	classNames := make(map[string]bool)
	if sourceType == "domain_table" {
		classNames[tableName] = true // since domain class name is the table_name for source_type = 'domain_table'
	}
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
			if len(chSpec.DomainKeys) == 0 {
				// Only get the DomainKeyInfo from domain_keys_registry if not specified
				// in cpipes config via DomainKeys
				classNames[chSpec.ClassName] = true
			}
		}
	}

	cpipesStartup.DomainKeysSpecByClass = make(map[string]*DomainKeysSpec, len(classNames))
	// log.Printf("***@@@ initialize CPIPES Class Names are %v\n", classNames)
	if len(classNames) > 0 {
		// Get the domain_key_json from domain_keys_registry table
		// Example of how this table is populated from base__workspace_init_db.sql:
		// INSERT INTO jetsapi.domain_keys_registry (entity_rdf_type, object_types, domain_keys_json) VALUES
		// ('wrs:Eligibility', '{"Eligibility"}', '{"Eligibility":"wrs:Generated_ID","jets:hashing_override":"none"}'),
		var buf strings.Builder
		buf.WriteString("SELECT entity_rdf_type, object_types, domain_keys_json FROM jetsapi.domain_keys_registry WHERE entity_rdf_type IN (")
		first := true
		for c, _ := range classNames {
			if !first {
				buf.WriteString(",")
			}
			first = false
			buf.WriteString("'")
			buf.WriteString(c)
			buf.WriteString("'")
		}
		buf.WriteString(")")
		bufStr := buf.String()
		// log.Printf("*** Getting domain_key_json from domain_keys_registry table: %s\n", bufStr)
		rows, err := dbpool.Query(ctx, bufStr)
		if err == nil {
			defer rows.Close()
			for rows.Next() {
				// scan the row
				var className string
				var domainKeysJson sql.NullString
				var objTypes []string
				if err = rows.Scan(&className, &objTypes, &domainKeysJson); err != nil {
					return cpipesStartup, fmt.Errorf("while querying domain_keys_json from domain_keys_registry: %s", err)
				}
				if len(domainKeysJson.String) == 0 {
					domainKeysJson.String = "\"jets:key\""
				}
				var v any
				err = json.Unmarshal([]byte(domainKeysJson.String), &v)
				if err != nil {
					return cpipesStartup, fmt.Errorf(
						"while unmarshaling domain_keys_json from domain_keys_registry for class %s: %v", className, err)
				}
				// mainObjectType here is the default object_type for the domain key json, it is needed when the
				// json contains only a string/array (column names) and not a struct with {object_type: column_names}
				mainObjectType := ""
				if len(objTypes) == 1 {
					mainObjectType = objTypes[0]
				}
				dkSpec, err := ParseDomainKeyInfo(mainObjectType, v)
				if err != nil {
					return cpipesStartup, fmt.Errorf("while parsing domain_keys_info2: %s", err)
				}
				cpipesStartup.DomainKeysSpecByClass[className] = dkSpec
			}
		} else {
			log.Printf("WARNING: Error while querying table domain_keys_registry: %v\n", err)
		}
	}
	// Set the DomainKeysSpec to the ChannelSpec
	for i := range cpipesStartup.CpConfig.Channels {
		chSpec := &cpipesStartup.CpConfig.Channels[i]
		if len(chSpec.DomainKeys) > 0 {
			// chSpec.DomainKeys must use a struct with {object_type: column_names}
			dkSpec, err := ParseDomainKeyInfo("", chSpec.DomainKeys)
			if err != nil {
				return cpipesStartup, fmt.Errorf("while parsing domain_keys_info2: %s", err)
			}
			chSpec.DomainKeysInfo = dkSpec
		} else {
			if len(chSpec.ClassName) > 0 {
				// log.Printf("*** Channel %s chSpec.ClassName %s\n", chSpec.Name, chSpec.ClassName)
				chSpec.DomainKeysInfo = cpipesStartup.DomainKeysSpecByClass[chSpec.ClassName]
			}
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
			Type:       "default",
			Key:        "_main_input_",
			SourceType: "main_input",
			Client:     client,
			Vendor:     org,
			ObjectType: objectType,
			FileConfig: FileConfig{
				Format:              inputFormat,
				Compression:         compression,
				Bucket:              bucketName,
				FileKey:             args.FileKey,
				InputFormatDataJson: inputFormatDataJson.String,
			},
			Env: make(map[string]any),
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
		if cpipesStartup.MainInputSchemaProviderConfig.Bucket == "" {
			cpipesStartup.MainInputSchemaProviderConfig.Bucket = bucketName
		}
		if cpipesStartup.MainInputSchemaProviderConfig.FileKey == "" {
			cpipesStartup.MainInputSchemaProviderConfig.FileKey = args.FileKey
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

	// Parse the Domain Key Info from source_config and main input schema provider
	switch sourceType {
	case "file":
		// Main input file is an external file
		// log.Printf("*** sourceType is 'file', icDomainKeys: %s\n", icDomainKeys.String)
		var dkInfo any
		switch {
		case len(mainInputSchemaProvider.DomainKeys) > 0:
			dkInfo = mainInputSchemaProvider.DomainKeys
		default:
			if len(icDomainKeys.String) == 0 {
				icDomainKeys.String = "\"jets:key\""
			}
			err = json.Unmarshal([]byte(icDomainKeys.String), &dkInfo)
			if err != nil {
				return cpipesStartup,
					fmt.Errorf("while unmarshaling domain_keys_json for the main input source (case source_type is file): %s", err)
			}
		}
		dkSpec, err := ParseDomainKeyInfo(objectType, dkInfo)
		if err != nil {
			return cpipesStartup, fmt.Errorf("while parsing domain_keys_info for the main input source (case source_type is file): %s", err)
		}
		cpipesStartup.MainInputDomainKeysSpec = dkSpec
	case "domain_table":
		// Main input file is a domain entity, ie, an entity mapped into a jetstore data model
		cpipesStartup.MainInputDomainClass = tableName
		cpipesStartup.MainInputDomainKeysSpec = cpipesStartup.DomainKeysSpecByClass[tableName]
	}

	// The main_input schema provider should always have the key _main_input_.
	// Note: cpipesStartup.CpConfig.MainInputChannel() returns the sharding first input channel
	// regardless if we are currently in reducing step.
	// This is to ensure mainInputSchemaProvider is always in sync with the mainInputChannel at each step.
	mainInputSchemaProvider.Key = "_main_input_"
	inputChannelConfig := cpipesStartup.CpConfig.MainInputChannel()

	// Sync the inputChannelConfig and mainInputSchemaProvider
	// Priority: inputChannelConfig, mainInputSchemaProvider, and then source_config table (which served
	// as defaults to mainInputSchemaProvider)
	syncInputChannelWithSchemaProvider(inputChannelConfig, mainInputSchemaProvider)

	// Input channel for sharding step always have the _main_input_ schema provider
	inputChannelConfig.SchemaProvider = mainInputSchemaProvider.Key

	// Set the fixed_width column spec to the schema provider
	if len(icPosCsv.String) > 0 {
		mainInputSchemaProvider.FixedWidthColumnsCsv = icPosCsv.String
	}

	// InputColumns - the main input file domain columns, order of priority:
	//	- Take the columns from source_config table if specified. (this has higher priority in case it's a subset of
	//    of the columns from schema provider)
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
					for _, lookupTable := range transformationSpec.AnonymizeConfig.DeidLookups {
						spec := lookupMap[lookupTable]
						if spec == nil {
							return nil,
								fmt.Errorf(
									"error: lookup table '%s' used by anonymize operator is not defined, please verify the configuration", lookupTable)
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

// Function to apply all conditional transformation spec in the pipeConfig
func ApplyAllConditionalTransformationSpec(pipeConfig []PipeSpec, env map[string]any) error {
	// Need to convert the conditional expression to evalExpression for evaluation
	builderContext := ExprBuilderContext(env)

	// Visit all transformation spec in the pipeConfig
	for i := range pipeConfig {
		pipeSpec := &pipeConfig[i]
		for j := range pipeSpec.Apply {
			transformationSpec := &pipeSpec.Apply[j]
			if transformationSpec.ConditionalConfig != nil {

				// build the evalExpression for each when condition
				for _, conditionalSpec := range transformationSpec.ConditionalConfig {
					evaluator, err := builderContext.BuildExprNodeEvaluator("conditional_config", nil, &conditionalSpec.When)
					if err != nil {
						return fmt.Errorf("error building evaluator for conditional transformation %d: %v", j, err)
					}

					// Evaluate the when condition
					v, err := evaluator.eval(env)
					if err != nil {
						return fmt.Errorf("error evaluating when condition for transformation %d: %v", j, err)
					}
					if ToBool(v) {
						// Apply the Then spec
						if len(conditionalSpec.Then.Type) > 0 {
							// Replace the host transformationSpec altogether
							*transformationSpec = conditionalSpec.Then
						} else {
							// Override the fields in the host transformationSpec
							err := MergeTransformationSpec(transformationSpec, &conditionalSpec.Then)
							if err != nil {
								return fmt.Errorf("error merging conditional transformation %d: %v", j, err)
							}
						}
					}
				}
			}
		}
	}
	return nil
}

func MergeTransformationSpec(host, override *TransformationSpec) error {
	// Merge the fields from the override spec into the host spec
	if override == nil || host == nil {
		return nil
	}
	// Check if we replace the host spec altogether
	if len(override.Type) > 0 {
		*host = *override
		return nil
	}
	// Merge the non scalar fields
	if len(override.Columns) > 0 {
		host.Columns = override.Columns
	}
	if override.MapRecordConfig != nil {
		host.MapRecordConfig = override.MapRecordConfig
	}
	if override.AnalyzeConfig != nil {
		host.AnalyzeConfig = override.AnalyzeConfig
	}
	if len(override.HighFreqColumns) > 0 {
		host.HighFreqColumns = override.HighFreqColumns
	}
	if override.PartitionWriterConfig != nil {
		host.PartitionWriterConfig = override.PartitionWriterConfig
	}
	if override.AnonymizeConfig != nil {
		host.AnonymizeConfig = override.AnonymizeConfig
	}
	if override.DistinctConfig != nil {
		host.DistinctConfig = override.DistinctConfig
	}
	if override.ShufflingConfig != nil {
		host.ShufflingConfig = override.ShufflingConfig
	}
	if override.GroupByConfig != nil {
		host.GroupByConfig = override.GroupByConfig
	}
	if override.FilterConfig != nil {
		host.FilterConfig = override.FilterConfig
	}
	if override.SortConfig != nil {
		host.SortConfig = override.SortConfig
	}
	if override.JetrulesConfig != nil {
		host.JetrulesConfig = override.JetrulesConfig
	}
	if override.ClusteringConfig != nil {
		host.ClusteringConfig = override.ClusteringConfig
	}
	if override.OutputChannel.Name != "" {
		host.OutputChannel = override.OutputChannel
	}
	return nil
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

func GetOutputFileConfig(cpConfig *ComputePipesConfig, outputFileKey string) *OutputFileSpec {
	for i := range cpConfig.OutputFiles {
		if outputFileKey == cpConfig.OutputFiles[i].Key {
			return &cpConfig.OutputFiles[i]
		}
	}
	return nil
}

// Function to validate the PipeSpec output channel config
// Apply a default snappy compression if compression is not specified
// and channel Type 'stage'.
// This function also syncs the input and ouput channels with the associated schema provider.
func (args *CpipesStartup) ValidatePipeSpecConfig(cpConfig *ComputePipesConfig, pipeConfig []PipeSpec) error {
	for i := range pipeConfig {
		pipeSpec := &pipeConfig[i]
		// log.Printf("VALIDATE PIPESPEC %s\n", pipeSpec.Type)
		if pipeSpec.InputChannel.Type == "" {
			pipeSpec.InputChannel.Type = "memory"
		}
		switch pipeSpec.InputChannel.Type {
		case "input":
			if i != 0 {
				return fmt.Errorf("configuration error: Only the first input_channel can be of type 'input'")
			}
		case "stage":
			if i != 0 {
				return fmt.Errorf("configuration error: Only the first input_channel can be of type 'stage'")
			}
			if len(pipeSpec.InputChannel.SchemaProvider) > 0 {
				sp := getSchemaProvider(cpConfig.SchemaProviders, pipeSpec.InputChannel.SchemaProvider)
				if sp == nil {
					return fmt.Errorf("configuration error: input_channel has reference to "+
						"schema_provider %s, but does not exists", pipeSpec.InputChannel.SchemaProvider)
				}
				syncInputChannelWithSchemaProvider(&pipeSpec.InputChannel, sp)
			}
			// Apply defaults
			if pipeSpec.InputChannel.Delimiter == 0 {
				pipeSpec.InputChannel.Delimiter = ','
			}
			if len(pipeSpec.InputChannel.Format) == 0 {
				pipeSpec.InputChannel.Format = "headerless_csv"
			}
			if len(pipeSpec.InputChannel.Compression) == 0 {
				pipeSpec.InputChannel.Compression = "snappy"
			}
		case "memory":
		default:
			return fmt.Errorf("configuration error: unknown input_channel.type: %s", pipeSpec.InputChannel.Type)
		}
		// Check that we don't have two input channel reading from the same channel,
		// this creates record lost since they steal records from each other
		for k := range pipeConfig {
			if i != k && pipeSpec.InputChannel.Name == pipeConfig[k].InputChannel.Name {
				return fmt.Errorf("configuration error: two input_channel reading from "+
					"the same channel %s, this will create record loss", pipeSpec.InputChannel.Name)
			}
		}
		// PipeSpec Type specific validations
		switch pipeSpec.Type {
		case "merge_files":
			if pipeSpec.OutputFile == nil || len(*pipeSpec.OutputFile) == 0 {
				return fmt.Errorf("configuration error: merge_file must have output_file set")
			}
			outputFileSpec := GetOutputFileConfig(cpConfig, *pipeSpec.OutputFile)
			if outputFileSpec == nil {
				return fmt.Errorf("configuration error: Output file config '%s' not found", *pipeSpec.OutputFile)
			}
			if outputFileSpec.OutputLocation() == "" {
				outputFileSpec.SetOutputLocation("jetstore_s3_output")
			}
		}
		for j := range pipeSpec.Apply {
			transformationConfig := &pipeSpec.Apply[j]
			outputChConfig := &transformationConfig.OutputChannel
			// log.Printf("*** VALIDATE PIPESPEC %s APPLY %s OUTPUT %s SP %s\n",
			// 	pipeSpec.Type, transformationConfig.Type, transformationConfig.OutputChannel.Name,
			// 	transformationConfig.OutputChannel.SchemaProvider)
			sp := getSchemaProvider(cpConfig.SchemaProviders, outputChConfig.SchemaProvider)
			// validate transformation pipe config
			switch transformationConfig.Type {
			case "partition_writer":
				if transformationConfig.PartitionWriterConfig == nil {
					return fmt.Errorf(
						"configuration error: must provide 'partition_writer_config'" +
							" for transformation pipe of type 'partition_writer'")
				}
				config := transformationConfig.PartitionWriterConfig
				switch config.DeviceWriterType {
				case "csv_writer", "parquet_writer", "fixed_width_writer":
				default:
					if config.DeviceWriterType == "" && sp == nil {
						return fmt.Errorf(
							"configuration error: must provide 'device_writer_type' or 'output_channel.schema_provider'"+
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
							err := fmt.Errorf("configuration error: unsupported output file format: %s (in NewPartitionWriterTransformationPipe)", sp.Format)
							log.Println(err)
							return err
						}
						config.DeviceWriterType = deviceWriterType
						outputChConfig.Format = sp.Format
					} else {
						return fmt.Errorf(
							"configuration error: unknown/invalid device_writer_type '%s' for partition_writer (valid type: csv_writer, parquet_writer, fixed_width_writer)",
							config.DeviceWriterType)
					}
				}
			case "anonymize":
				if transformationConfig.AnonymizeConfig == nil {
					return fmt.Errorf("configuration error: missing anonymize_config for anonymize operator")
				}
				keyOutputChannel := transformationConfig.AnonymizeConfig.KeysOutputChannel
				if keyOutputChannel != nil {
					err := validateOutputChConfig(keyOutputChannel, getSchemaProvider(cpConfig.SchemaProviders, keyOutputChannel.SchemaProvider))
					if err != nil {
						return err
					}
				}
			case "jetrules":
				if transformationConfig.JetrulesConfig == nil {
					return fmt.Errorf("configuration error: missing jetrules_config for jetrules operator")
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
						"configuration error: missing clustering_config or correlation_output_channel for clustering operator")
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

// Sync the following properties from FileConfig betwwen args inputChannel and schemaProvider:
//   - Compression
//   - Delimiter
//   - DetectEncoding
//   - DomainClass
//   - DomainKeys
//   - Encoding
//   - EnforceRowMaxLength
//   - EnforceRowMinLength
//   - Format
//   - IsPartFiles
//   - NbrRowsInRecord
//   - NoQuotes
//   - ParquetSchema
//   - QuoteAllRecords
//   - ReadBatchSize
//   - ReadDateLayout
//   - TrimColumns
//   - UseLazyQuotes
//   - UseLazyQuotesSpecial
//   - VariableFieldsPerRecord
//   - WriteDateLayout
//
// Priority: inputChannelConfig, mainInputSchemaProvider, and then source_config table (which served
// as defaults to mainInputSchemaProvider)
func syncInputChannelWithSchemaProvider(ic *InputChannelConfig, sp *SchemaProviderSpec) {
	if ic.Compression == "" {
		ic.Compression = sp.Compression
	} else {
		sp.Compression = ic.Compression
	}

	if ic.Delimiter == 0 {
		ic.Delimiter = sp.Delimiter
	} else {
		sp.Delimiter = ic.Delimiter
	}

	if !ic.DetectEncoding {
		ic.DetectEncoding = sp.DetectEncoding
	} else {
		sp.DetectEncoding = ic.DetectEncoding
	}

	if !ic.DetectCrAsEol {
		ic.DetectCrAsEol = sp.DetectCrAsEol
	} else {
		sp.DetectCrAsEol = ic.DetectCrAsEol
	}

	if ic.DomainClass == "" {
		ic.DomainClass = sp.DomainClass
	} else {
		sp.DomainClass = ic.DomainClass
	}

	if ic.DomainKeys == nil {
		ic.DomainKeys = sp.DomainKeys
	} else {
		sp.DomainKeys = ic.DomainKeys
	}

	if ic.Encoding == "" {
		ic.Encoding = sp.Encoding
	} else {
		sp.Encoding = ic.Encoding
	}

	if !ic.EnforceRowMaxLength {
		ic.EnforceRowMaxLength = sp.EnforceRowMaxLength
	} else {
		sp.EnforceRowMaxLength = ic.EnforceRowMaxLength
	}

	if !ic.EnforceRowMinLength {
		ic.EnforceRowMinLength = sp.EnforceRowMinLength
	} else {
		sp.EnforceRowMinLength = ic.EnforceRowMinLength
	}

	if ic.Format == "" {
		ic.Format = sp.Format
	} else {
		sp.Format = ic.Format
	}

	if !ic.IsPartFiles {
		ic.IsPartFiles = sp.IsPartFiles
	} else {
		sp.IsPartFiles = ic.IsPartFiles
	}

	if ic.NbrRowsInRecord == 0 {
		ic.NbrRowsInRecord = sp.NbrRowsInRecord
	} else {
		sp.NbrRowsInRecord = ic.NbrRowsInRecord
	}

	if ic.MultiColumnsInput {
		sp.MultiColumnsInput = true
	} else {
		ic.MultiColumnsInput = sp.MultiColumnsInput
	}

	if !ic.NoQuotes {
		ic.NoQuotes = sp.NoQuotes
	} else {
		sp.NoQuotes = ic.NoQuotes
	}

	if ic.ParquetSchema == nil {
		ic.ParquetSchema = sp.ParquetSchema
	} else {
		sp.ParquetSchema = ic.ParquetSchema
	}

	if !ic.PutHeadersOnFirstPartition {
		ic.PutHeadersOnFirstPartition = sp.PutHeadersOnFirstPartition
	} else {
		sp.PutHeadersOnFirstPartition = ic.PutHeadersOnFirstPartition
	}

	if !ic.QuoteAllRecords {
		ic.QuoteAllRecords = sp.QuoteAllRecords
	} else {
		sp.QuoteAllRecords = ic.QuoteAllRecords
	}

	if ic.ReadBatchSize == 0 {
		ic.ReadBatchSize = sp.ReadBatchSize
	} else {
		sp.ReadBatchSize = ic.ReadBatchSize
	}

	if ic.ReadDateLayout == "" {
		ic.ReadDateLayout = sp.ReadDateLayout
	} else {
		sp.ReadDateLayout = ic.ReadDateLayout
	}

	if !ic.TrimColumns {
		ic.TrimColumns = sp.TrimColumns
	} else {
		sp.TrimColumns = ic.TrimColumns
	}

	if !ic.UseLazyQuotes {
		ic.UseLazyQuotes = sp.UseLazyQuotes
	} else {
		sp.UseLazyQuotes = ic.UseLazyQuotes
	}

	if !ic.UseLazyQuotesSpecial {
		ic.UseLazyQuotesSpecial = sp.UseLazyQuotesSpecial
	} else {
		sp.UseLazyQuotesSpecial = ic.UseLazyQuotesSpecial
	}

	if !ic.VariableFieldsPerRecord {
		ic.VariableFieldsPerRecord = sp.VariableFieldsPerRecord
	} else {
		sp.VariableFieldsPerRecord = ic.VariableFieldsPerRecord
	}

	if ic.WriteDateLayout == "" {
		ic.WriteDateLayout = sp.WriteDateLayout
	} else {
		sp.WriteDateLayout = ic.WriteDateLayout
	}
}

// Sync the following properties from FileConfig betwwen args outputChannel and schemaProvider:
//   - Compression
//   - Delimiter
//   - DomainClass
//   - DomainKeys
//   - Encoding
//   - Format
//   - NbrRowsInRecord
//   - NoQuotes
//   - OutputEncoding
//   - ParquetSchema
//   - QuoteAllRecords
//   - ReadBatchSize
//   - ReadDateLayout
//   - WriteDateLayout
//
// Priority: outputChannelConfig then schemaProvider
func syncOutputChannelWithSchemaProvider(ic *OutputChannelConfig, sp *SchemaProviderSpec) {
	if ic.Compression == "" {
		ic.Compression = sp.Compression
	} else {
		sp.Compression = ic.Compression
	}

	if ic.Delimiter == 0 {
		ic.Delimiter = sp.Delimiter
	} else {
		sp.Delimiter = ic.Delimiter
	}

	if ic.DomainClass == "" {
		ic.DomainClass = sp.DomainClass
	} else {
		sp.DomainClass = ic.DomainClass
	}

	if ic.DomainKeys == nil {
		ic.DomainKeys = sp.DomainKeys
	} else {
		sp.DomainKeys = ic.DomainKeys
	}

	if ic.Encoding == "" {
		ic.Encoding = sp.Encoding
	} else {
		sp.Encoding = ic.Encoding
	}

	if ic.OutputEncoding == "" {
		ic.OutputEncoding = sp.OutputEncoding
	} else {
		sp.OutputEncoding = ic.OutputEncoding
	}
	if sp.OutputEncoding == "" {
		sp.OutputEncoding = sp.Encoding
	}
	if ic.OutputEncoding == "" {
		ic.OutputEncoding = ic.Encoding
	}

	if ic.Format == "" {
		ic.Format = sp.Format
	} else {
		sp.Format = ic.Format
	}

	if ic.NbrRowsInRecord == 0 {
		ic.NbrRowsInRecord = sp.NbrRowsInRecord
	} else {
		sp.NbrRowsInRecord = ic.NbrRowsInRecord
	}

	if !ic.NoQuotes {
		ic.NoQuotes = sp.NoQuotes
	} else {
		sp.NoQuotes = ic.NoQuotes
	}

	if ic.ParquetSchema == nil {
		ic.ParquetSchema = sp.ParquetSchema
	} else {
		sp.ParquetSchema = ic.ParquetSchema
	}

	if !ic.PutHeadersOnFirstPartition {
		ic.PutHeadersOnFirstPartition = sp.PutHeadersOnFirstPartition
	} else {
		sp.PutHeadersOnFirstPartition = ic.PutHeadersOnFirstPartition
	}

	if !ic.QuoteAllRecords {
		ic.QuoteAllRecords = sp.QuoteAllRecords
	} else {
		sp.QuoteAllRecords = ic.QuoteAllRecords
	}

	if ic.ReadBatchSize == 0 {
		ic.ReadBatchSize = sp.ReadBatchSize
	} else {
		sp.ReadBatchSize = ic.ReadBatchSize
	}

	if ic.ReadDateLayout == "" {
		ic.ReadDateLayout = sp.ReadDateLayout
	} else {
		sp.ReadDateLayout = ic.ReadDateLayout
	}

	if ic.WriteDateLayout == "" {
		ic.WriteDateLayout = sp.WriteDateLayout
	} else {
		sp.WriteDateLayout = ic.WriteDateLayout
	}
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
			return fmt.Errorf("configuration error: must provide output_table_key when output_channel type is 'sql'")
		}
		outputChConfig.Name = outputChConfig.OutputTableKey
		outputChConfig.SpecName = outputChConfig.OutputTableKey
	default:
		if len(outputChConfig.Name) == 0 || outputChConfig.Name == outputChConfig.SpecName {
			return fmt.Errorf(
				"configuration error: output_channel.name '%s' must not be empty or same as output_channel.channel_spec_name '%s'",
				outputChConfig.Name, outputChConfig.SpecName)
		}
		switch outputChConfig.Type {
		case "stage":
			if sp != nil {
				syncOutputChannelWithSchemaProvider(outputChConfig, sp)
			}
			if strings.HasPrefix(outputChConfig.Format, "parquet") {
				outputChConfig.Format = "parquet"
				outputChConfig.Compression = ""
				outputChConfig.Delimiter = 0
				if sp != nil {
					sp.Format = "parquet"
					sp.Compression = ""
					sp.Delimiter = 0
				}
			} else {
				// Apply defaults
				if outputChConfig.Delimiter == 0 {
					outputChConfig.Delimiter = ','
				}
				if outputChConfig.Compression == "" {
					outputChConfig.Compression = "snappy"
				}
				if outputChConfig.Format == "" {
					outputChConfig.Format = "headerless_csv"
				}
			}

			if len(outputChConfig.WriteStepId) == 0 {
				return fmt.Errorf("configuration error: write_step_id is not specified in output_channel '%s' of type 'stage'",
					outputChConfig.Name)
			}
		case "output":
			if sp != nil {
				syncOutputChannelWithSchemaProvider(outputChConfig, sp)
			}
			if outputChConfig.UseOriginalHeaders && outputChConfig.SpecName != "input_row" {
				return fmt.Errorf(
					"configuration error: output_channel.use_original_headers can only be true when output_channel.spec_name is 'input_row'")
			}
			if strings.HasPrefix(outputChConfig.Format, "parquet") {
				outputChConfig.Format = "parquet"
				outputChConfig.Compression = ""
				outputChConfig.Delimiter = 0
			} else {
				if outputChConfig.Format == "" {
					return fmt.Errorf("configuration error: format is not specified in output_channel '%s' of type 'output'",
						outputChConfig.Name)
				}
				if outputChConfig.Delimiter == 0 {
					outputChConfig.Delimiter = ','
				}
				if outputChConfig.Compression == "" {
					outputChConfig.Compression = "none"
				}
			}
			if len(outputChConfig.OutputLocation()) == 0 {
				outputChConfig.SetOutputLocation("jetstore_s3_output")
			}

		case "memory":
			outputChConfig.Format = ""
			outputChConfig.Compression = ""
			outputChConfig.Delimiter = 0
		default:
			return fmt.Errorf(
				"configuration error: unknown output_channel config type: %s (expecting: memory (default), stage, output, sql)",
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

func GetChannelSpec(channels []ChannelSpec, name string) *ChannelSpec {
	if name == "" {
		return nil
	}
	for i := range channels {
		if channels[i].Name == name {
			return &channels[i]
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
	// The main schema provider env is used as the overall env context.
	if mainSchemaProviderConfig.Env == nil {
		mainSchemaProviderConfig.Env = make(map[string]any)
	}
	mainSchemaProviderConfig.Env["$INPUT_BUCKET"] = mainSchemaProviderConfig.Bucket
	mainSchemaProviderConfig.Env["$MAIN_SCHEMA_NAME"] = mainSchemaProviderConfig.SchemaName

	for i := range cpConfig.Context {
		if cpConfig.Context[i].Type == "value" {
			mainSchemaProviderConfig.Env[cpConfig.Context[i].Key] = cpConfig.Context[i].Expr
		}
	}

	if cpConfig.ClusterConfig.IsDebugMode {
		b, err := json.Marshal(mainSchemaProviderConfig.Env)
		log.Printf("PrepareCpipesEnv: Cpipes Env: %s, err? %v\n", string(b), err)
	}
	return mainSchemaProviderConfig.Env
}

func (cpipesStartup *CpipesStartup) EvalUseEcsTask(stepId int) (bool, error) {
	pipeSpec := cpipesStartup.CpConfig.ConditionalPipesConfig
	result := false
	if len(pipeSpec) > stepId {
		result = pipeSpec[stepId].UseEcsTasks
		if pipeSpec[stepId].UseEcsTasksWhen != nil {
			builderContext := ExprBuilderContext(cpipesStartup.EnvSettings)
			evaluator, err := builderContext.BuildExprNodeEvaluator("use_ecs_tasks", nil, pipeSpec[stepId].UseEcsTasksWhen)
			if err != nil {
				return false, err
			}
			v, err := evaluator.eval(cpipesStartup.EnvSettings)
			if err != nil {
				return false, err
			}
			return ToBool(v), nil
		}
	}
	return result, nil
}

// Function to get the column to add to the input file(s),
// these columns are added to the input_row channel.
// They are taken from the channel config with name input_row.
func GetAdditionalInputColumns(cpConfig *ComputePipesConfig) []string {
	if cpConfig == nil {
		return nil
	}
	for i := range cpConfig.Channels {
		if cpConfig.Channels[i].Name == "input_row" {
			return cpConfig.Channels[i].Columns
		}
	}
	return nil
}
