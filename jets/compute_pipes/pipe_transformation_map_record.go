package compute_pipes

import (
	"fmt"
	"log"

	"github.com/artisoft-io/jetstore/jets/jetrules/rete"
)

// map_record TransformationSpec implementing PipeTransformationEvaluator interface
// map_record: each input record is mapped to the output

type MapRecordTransformationPipe struct {
	outputCh         *OutputChannel
	columnEvaluators []TransformationColumnEvaluator
	spec             *TransformationSpec
	doneCh           chan struct{}
}

// Implementing interface PipeTransformationEvaluator
func (ctx *MapRecordTransformationPipe) Apply(input *[]interface{}) error {
	if input == nil {
		return fmt.Errorf("error: input is nil in MapRecordTransformationPipe.Apply")
	}
	var currentValues *[]interface{}
	if ctx.spec.NewRecord {
		v := make([]interface{}, len(ctx.outputCh.config.Columns))
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
	// Notify the column evaluator that we're done
	// fmt.Println("**!@@ calling done on column evaluator from MapRecordTransformationPipe for output", ctx.outputCh.name)
	for i := range ctx.columnEvaluators {
		err := ctx.columnEvaluators[i].Done(currentValues)
		if err != nil {
			return fmt.Errorf("while calling done on column evaluator from AggregateTransformationPipe: %v", err)
		}
	}
	if !ctx.spec.NewRecord {
		// resize the slice in case we're dropping column on the output
		if len(*currentValues) > len(ctx.outputCh.config.Columns) {
			*currentValues = (*currentValues)[:len(ctx.outputCh.config.Columns)]
		}
	}
	// Send the result to output
	select {
	case ctx.outputCh.channel <- *currentValues:
	case <-ctx.doneCh:
		log.Printf("MapRecordTransformationPipe writing to '%s' interrupted", ctx.outputCh.name)
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
		// Load the mapping spec from jetsapi.process_mapping
		inputMappingItems, err := GetInputMapping(ctx.dbpool, config.FileMappingTableName)
		if err != nil {
			return nil, fmt.Errorf("while getting mapping details from jetstore db: %v", err)
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
				_, ok := (*outputCh.columns)[mappingExp.DataProperty]
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
		outputCh:         outputCh,
		columnEvaluators: columnEvaluators,
		spec:             spec,
		doneCh:           ctx.done,
	}, nil
}
