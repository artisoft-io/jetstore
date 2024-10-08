package compute_pipes

import (
	"fmt"
	"log"
)

// map_record TransformationSpec implementing PipeTransformationEvaluator interface
// map_record: each input record is mapped to the output, done calls the close on the output channel

type MapRecordTransformationPipe struct {
	outputCh         *OutputChannel
	columnEvaluators []TransformationColumnEvaluator
	spec             *TransformationSpec
	doneCh           chan struct{}
}

// Implementing interface PipeTransformationEvaluator
func (ctx *MapRecordTransformationPipe) apply(input *[]interface{}) error {
	if input == nil {
		return fmt.Errorf("error: input is nil in MapRecordTransformationPipe.apply")
	}
	var currentValues *[]interface{}
	if ctx.spec.NewRecord {
		v := make([]interface{}, len(ctx.outputCh.config.Columns))
		currentValues = &v
		// initialize the column evaluators
		for i := range ctx.columnEvaluators {
			ctx.columnEvaluators[i].initializeCurrentValue(currentValues)
		}	
	} else {
		currentValues = input
	}
	// apply the column transformation for each column
	for i := range ctx.columnEvaluators {
		err := ctx.columnEvaluators[i].update(currentValues, input)
		if err != nil {
			err = fmt.Errorf("while calling column transformation from map_record: %v", err)
			log.Println(err)
			return err
		}
	}
	// Notify the column evaluator that we're done
	// fmt.Println("**!@@ calling done on column evaluator from MapRecordTransformationPipe for output", ctx.outputCh.config.Name)
	for i := range ctx.columnEvaluators {
		err := ctx.columnEvaluators[i].done(currentValues)
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
		log.Printf("MapRecordTransformationPipe writing to '%s' interrupted", ctx.outputCh.config.Name)
		return nil
	}
	return nil
}

func (ctx *MapRecordTransformationPipe) done() error {
	return nil
}

func (ctx *MapRecordTransformationPipe) finally() {}

func (ctx *BuilderContext) NewMapRecordTransformationPipe(source *InputChannel, outputCh *OutputChannel, spec *TransformationSpec) (*MapRecordTransformationPipe, error) {
	// Prepare the column evaluators
	var err error
	columnEvaluators := make([]TransformationColumnEvaluator, len(spec.Columns))
	for i := range spec.Columns {
		// log.Printf("**& build TransformationColumn[%d] of type %s for output %s", i, spec.Type, spec.Output)
		columnEvaluators[i], err = ctx.buildTransformationColumnEvaluator(source, outputCh, &spec.Columns[i])
		if err != nil {
			err = fmt.Errorf("while buildTransformationColumnEvaluator (in NewMapRecordTransformationPipe) %v", err)
			log.Println(err)
			return nil, err
		}
	}
	return &MapRecordTransformationPipe{
		outputCh:         outputCh,
		columnEvaluators: columnEvaluators,
		spec:             spec,
		doneCh:           ctx.done,
	}, nil
}
