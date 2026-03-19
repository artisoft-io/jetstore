package compute_pipes

import (
	"encoding/json"
	"fmt"
	"log"

	"github.com/artisoft-io/jetstore/jets/jetrules/rete"
	"github.com/artisoft-io/jetstore/jets/utils"
)

// map_record TransformationSpec implementing PipeTransformationEvaluator interface
// map_record: each input record is mapped to the output

type MapRecordTransformationPipe struct {
	source           *InputChannel
	outputCh         *OutputChannel
	columnEvaluators []TransformationColumnEvaluator
	errorCount       int
	errorOutputCh    *OutputChannel
	spec             *TransformationSpec
	doneCh           chan struct{}
	builderContext   *BuilderContext
}

// Implementing interface PipeTransformationEvaluator
func (ctx *MapRecordTransformationPipe) Apply(input *[]any) error {
	if input == nil {
		return fmt.Errorf("error: input is nil in MapRecordTransformationPipe.Apply")
	}
	var currentValues *[]any
	var inBytes []byte
	// Debug logging of input record
	if ctx.spec.MapRecordConfig != nil && ctx.spec.MapRecordConfig.IsDebug {
		data, err := utils.ZipSlices(ctx.source.Config.Columns, *input)
		if err != nil {
			return fmt.Errorf("while zipping input columns and values for debug logging: %v", err)
		}
		inBytes, _ = json.Marshal(data)
		log.Println()
		log.Printf("MapRecordTransformationPipe input (zipped): %s", string(inBytes))
		log.Println()
	}

	if ctx.spec.NewRecord {
		v := make([]any, len(ctx.outputCh.Config.Columns))
		currentValues = &v
		// initialize the column evaluators
		for i := range ctx.columnEvaluators {
			ctx.columnEvaluators[i].InitializeCurrentValue(currentValues)
		}
	} else {
		currentValues = input
	}
	// Apply the column transformation for each column
	for i := range ctx.columnEvaluators {
		err := ctx.columnEvaluators[i].Update(currentValues, input)
		if err != nil && ctx.errorOutputCh != nil && ctx.errorCount < 10 {
			peRow := ctx.builderContext.NewProcessError()
			peRow.ErrorMessage = err.Error()
			peRow.write2Chan(ctx.errorOutputCh, ctx.doneCh)
			log.Printf("mapping error: %s", err.Error())
			ctx.errorCount++
		} else {
			if err != nil && ctx.spec.MapRecordConfig != nil && ctx.spec.MapRecordConfig.IsDebug {
				log.Printf("mapping error: %s", err.Error())
			}
		}
	}
	if !ctx.spec.NewRecord {
		// resize the slice in case we're dropping column on the output
		if len(*currentValues) > len(ctx.outputCh.Config.Columns) {
			*currentValues = (*currentValues)[:len(ctx.outputCh.Config.Columns)]
		}
	}
	if ctx.outputCh.Config.ClassName != "" {
		// Set rdf:type to output channel class name if it's not set by the mapping
		typeIdx, ok := (*ctx.outputCh.Columns)["rdf:type"]
		if ok && (*currentValues)[typeIdx] == nil {
			(*currentValues)[typeIdx] = fmt.Sprintf(`{"%s"}`, ctx.outputCh.Config.ClassName)
		}
	}

	var outBytes []byte
	// Debug logging of output record
	if ctx.spec.MapRecordConfig != nil && ctx.spec.MapRecordConfig.IsDebug {
		data, err := utils.ZipSlices(ctx.outputCh.Config.Columns, *currentValues)
		if err != nil {
			return fmt.Errorf("while zipping output columns and values for debug logging: %v", err)
		}
		outBytes, _ = json.Marshal(data)
		log.Printf("MapRecordTransformationPipe output (zipped): %s", string(outBytes))
		log.Println()
	}

	// Send the result to output
	select {
	case ctx.outputCh.Channel <- *currentValues:
	case <-ctx.doneCh:
		log.Printf("MapRecordTransformationPipe writing to '%s' interrupted", ctx.outputCh.Name)
		return nil
	}
	return nil
}

func (ctx *MapRecordTransformationPipe) Done() error {
	return nil
}

func (ctx *MapRecordTransformationPipe) Finally() {}

func (ctx *BuilderContext) NewMapRecordTransformationPipe(source *InputChannel, outputCh *OutputChannel,
	spec *TransformationSpec) (*MapRecordTransformationPipe, error) {

	config := spec.MapRecordConfig
	// Prepare the column evaluators
	columnEvaluators := make([]TransformationColumnEvaluator, 0, len(spec.Columns))
	// Check if we use the mapping spec from jetstore ui
	if config != nil && len(config.FileMappingTableName) > 0 {
		// Apply environment variable substitution
		fileMappingTableName := utils.ReplaceEnvVars(config.FileMappingTableName, ctx.env)
		// Load the mapping spec from jetsapi.process_mapping
		inputMappingItems, err := GetInputMapping(ctx.dbpool, fileMappingTableName)
		if err != nil {
			return nil, fmt.Errorf("while getting mapping details from jetstore db: %v", err)
		}
		if len(inputMappingItems) == 0 {
			return nil, fmt.Errorf("error: no mapping items found in jetstore db for mapping table: %s",
				fileMappingTableName)
		}
		codeValueMapping, err := GetCodeValueMapping(ctx.dbpool, fileMappingTableName)
		if err != nil {
			return nil, fmt.Errorf("while getting code value mapping details from jetstore db: %v", err)
		}
		if config.IsDebug {
			log.Printf("MapRecordTransformationPipe loading %d mapping items from mapping table: %s, with code value mapping length: %d",
				len(inputMappingItems), fileMappingTableName, len(codeValueMapping))
		}
		// Get the domain data properties from local workspace to get the rdf type
		propertyMap, err := GetWorkspaceDataProperties()
		if err != nil {
			return nil, fmt.Errorf("while getting data property details from workspace: %v", err)
		}
		// Construct the mapping column evaluators
		if config.IsDebug {
			log.Printf("*** Columns in input channel: %v", source.Config.Columns)
			log.Printf("*** Columns in output channel: %v", outputCh.Config.Columns)
		}
		var cvm map[string]string
		for i := range inputMappingItems {
			mappingExp := &inputMappingItems[i]
			node := propertyMap[mappingExp.DataProperty]
			if node == nil {
				// Check if this is a "local variable for rules", ie if it's added to the input class
				_, ok := (*outputCh.Columns)[mappingExp.DataProperty]
				if ok {
					node = &rete.DataPropertyNode{Type: "text"}
					log.Printf("Note: mapping expression data property '%s' is not found in workspace metastore but found in input channel columns, treating it as text type", mappingExp.DataProperty)
				} else {
					return nil, fmt.Errorf("error: property name not found in workspace metastore or input channel: %v",
						mappingExp.DataProperty)
				}
			}
			cvm = nil
			if codeValueMapping != nil {
				cvm = codeValueMapping[mappingExp.DataProperty]
			}
			ce, err := ctx.BuildMapTCEvaluator(source, outputCh, &TransformationColumnSpec{
				Name: mappingExp.DataProperty,
				Type: "map",
				Expr: &mappingExp.InputColumn.String,
				MapExpr: &MapExpression{
					CleansingFunction: mappingExp.CleansingFunctionName.String,
					Argument:          mappingExp.Argument.String,
					Default:           mappingExp.DefaultValue.String,
					ErrMsg:            mappingExp.ErrorMessage.String,
					CodeValueMapping:  cvm,
					RdfType:           node.Type,
				},
			})
			if err != nil {
				return nil, fmt.Errorf("while creating the map column evaluator (BuildMapTCEvaluator): %v", err)
			}
			columnEvaluators = append(columnEvaluators, ce)
		}
	}
	for i := range spec.Columns {
		// log.Printf("**& build TransformationColumn[%d] of type %s for output %s", i, spec.Type, spec.Output)
		ce, err := ctx.BuildTransformationColumnEvaluator(source, outputCh, &spec.Columns[i])
		if err != nil {
			err = fmt.Errorf("while BuildTransformationColumnEvaluator (in NewMapRecordTransformationPipe) %v", err)
			log.Println(err)
			return nil, err
		}
		columnEvaluators = append(columnEvaluators, ce)
	}

	// Get the error channel if configured
	var errorOutputCh *OutputChannel
	var err error
	if config.ErrorChannel != nil {
		if len(config.ErrorChannel.Name) == 0 {
			return nil, fmt.Errorf("error: error_channel name cannot be empty")
		}
		if len(config.ErrorChannel.SpecName) == 0 {
			return nil, fmt.Errorf("error: error_channel spec name cannot be empty")
		}
		errorOutputCh, err = ctx.channelRegistry.GetOutputChannel(config.ErrorChannel.Name)
		if err != nil {
			return nil, err
		}
	}
	return &MapRecordTransformationPipe{
		source:           source,
		outputCh:         outputCh,
		columnEvaluators: columnEvaluators,
		spec:             spec,
		doneCh:           ctx.done,
		errorOutputCh:    errorOutputCh,
		builderContext:   ctx,
	}, nil
}
