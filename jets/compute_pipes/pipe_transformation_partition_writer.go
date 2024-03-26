package compute_pipes

import (
	"fmt"
	"log"
)

// partition_writer TransformationSpec implementing PipeTransformationEvaluator interface
// partition_writer: bundle input records into fixed-sized partitions.
// The output channel in the pipe spec config correspond to the intermediate channel to the actual 
// device writer. Currently supporting writing to s3 jetstore input path

type PartitionWriterTransformationPipe struct {
	outputCh         *OutputChannel
	columnEvaluators []TransformationColumnEvaluator
	doneCh           chan struct{}
}

// Implementing interface PipeTransformationEvaluator
func (ctx *PartitionWriterTransformationPipe) apply(input *[]interface{}) error {
	currentValues := make([]interface{}, len(ctx.outputCh.config.Columns))
	// initialize the column evaluators
	for i := range ctx.columnEvaluators {
		ctx.columnEvaluators[i].initializeCurrentValue(&currentValues)
	}
	// apply the column transformation for each column
	for i := range ctx.columnEvaluators {
		err := ctx.columnEvaluators[i].update(&currentValues, input)
		if err != nil {
			err = fmt.Errorf("while calling column transformation from partition_writer: %v", err)
			log.Println(err)
			return err
		}
	}
	// Notify the column evaluator that we're done
	// fmt.Println("**! calling done on column evaluator from PartitionWriterTransformationPipe for output", ctx.outputCh.config.Name)
	for i := range ctx.columnEvaluators {
		err := ctx.columnEvaluators[i].done(&currentValues)
		if err != nil {
			return fmt.Errorf("while calling done on column evaluator from AggregateTransformationPipe: %v", err)
		}
	}
	// Send the result to output
	// fmt.Println("**! partition_writer loop out row:", currentValues, "to outCh:", ctx.outputCh.config.Name)
	select {
	case ctx.outputCh.channel <- currentValues:
	case <-ctx.doneCh:
		log.Printf("PartitionWriterTransformationPipe writting to '%s' interrupted", ctx.outputCh.config.Name)
		return nil
	}

	return nil
}
func (ctx *PartitionWriterTransformationPipe) done() error {
	return nil
}

func (ctx *BuilderContext) NewPartitionWriterTransformationPipe(source *InputChannel, outputCh *OutputChannel, spec *TransformationSpec) (*PartitionWriterTransformationPipe, error) {
	// Prepare the column evaluators
	var err error
	columnEvaluators := make([]TransformationColumnEvaluator, len(spec.Columns))
	for i := range spec.Columns {
		// log.Printf("**& build TransformationColumn[%d] of type %s for output %s", i, spec.Type, spec.Output)
		columnEvaluators[i], err = ctx.buildTransformationColumnEvaluator(source, outputCh, &spec.Columns[i])
		if err != nil {
			err = fmt.Errorf("while buildTransformationColumnEvaluator (in buildPipeTransformationEvaluator) %v", err)
			fmt.Println(err)
			return nil, err
		}
	}
	return &PartitionWriterTransformationPipe{
		outputCh:         outputCh,
		columnEvaluators: columnEvaluators,
		doneCh:           ctx.done,
	}, nil
}
