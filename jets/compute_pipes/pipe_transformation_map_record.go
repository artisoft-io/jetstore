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
	source            *InputChannel
	outputCh         *OutputChannel
	columnEvaluators []TransformationColumnEvaluator
	spec             *TransformationSpec
	doneCh           chan struct{}
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
		if err != nil {
			err = fmt.Errorf("while calling column transformation from map_record: %v", err)
			log.Println(err)
			return err
		}
	}
	if !ctx.spec.NewRecord {
		// resize the slice in case we're dropping column on the output
		if len(*currentValues) > len(ctx.outputCh.Config.Columns) {
			*currentValues = (*currentValues)[:len(ctx.outputCh.Config.Columns)]
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
		if config.IsDebug {
			log.Printf("MapRecordTransformationPipe loading %d mapping items from mapping table: %s",
				len(inputMappingItems), fileMappingTableName)
		}
		// Get the domain data properties from local workspace to get the rdf type
		propertyMap, err := GetWorkspaceDataProperties()
		if err != nil {
			return nil, fmt.Errorf("while getting data property details from workspace: %v", err)
		}
		// Construct the mapping column evaluators
		for i := range inputMappingItems {
			mappingExp := &inputMappingItems[i]
			node := propertyMap[mappingExp.DataProperty]
			if node == nil {
				// Check if this is a "local variable for rules", ie if it's added to the input class
				_, ok := (*outputCh.Columns)[mappingExp.DataProperty]
				if ok {
					node = &rete.DataPropertyNode{Type: "text"}
				} else {
					return nil, fmt.Errorf("error: property name not found in workspace metastore or input channel: %v",
					mappingExp.DataProperty)
				}
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
	return &MapRecordTransformationPipe{
		source:            source,
		outputCh:         outputCh,
		columnEvaluators: columnEvaluators,
		spec:             spec,
		doneCh:           ctx.done,
	}, nil
}
