package compute_pipes

import (
	"fmt"
	"log"
)

// Worker to perform jetrules execute rules function

type ClusteringWorker struct {
	config                *ClusteringSpec
	source                *InputChannel
	column1               *string
	column1Value          any
	columns2              []string
	correlationEvaluators []*distinctCountCorrelationEval
	outputChannel         *OutputChannel
	done                  chan struct{}
	errCh                 chan error
}

// source and outputChannel are provided for their spec, the data is sent and recieved
// via inputCh and outputCh
func NewClusteringWorker(config *ClusteringSpec, source *InputChannel,
	column1 *string, column1Value any, columns2 []string, outputChannel *OutputChannel,
	done chan struct{}, errCh chan error) *ClusteringWorker {
	// Create the evaluators
	evaluators := make([]*distinctCountCorrelationEval, 0, len(columns2))
	for _, c := range columns2 {
		if c != *column1 {
			evaluators = append(evaluators, &distinctCountCorrelationEval{
				column2:        &c,
				column2Pos:     source.columns[c],
				distinctValues: make(map[string]bool),
			})
		}
	}
	return &ClusteringWorker{
		config:                config,
		source:                source,
		column1:               column1,
		column1Value:          column1Value,
		columns2:              columns2,
		correlationEvaluators: evaluators,
		outputChannel:         outputChannel,
		done:                  done,
		errCh:                 errCh,
	}
}

func (ctx *ClusteringWorker) DoWork(inputCh <-chan []any, outputCh chan<- []any, resultCh chan ClusteringResult) {
	for row := range inputCh {
		for _, eval := range ctx.correlationEvaluators {
			eval.Apply(row)
		}
	}
	// done, send the result out
	for _, evaluator := range ctx.correlationEvaluators {
		if evaluator.nonNilCount > 2 {
			result := make([]any, len(ctx.outputChannel.config.Columns))
			result[ctx.outputChannel.columns["column_name_1"]] = *ctx.column1
			result[ctx.outputChannel.columns["column1_value"]] = ctx.column1Value
			result[ctx.outputChannel.columns["column_name_2"]] = *evaluator.column2
			result[ctx.outputChannel.columns["distinct_count"]] = len(evaluator.distinctValues)
			result[ctx.outputChannel.columns["total_non_null_count"]] = evaluator.nonNilCount
			// Send the out the result
			select {
			case outputCh <- result:
			case <-ctx.done:
				log.Println("Clustering Pool Worker interrupted while DoWork")
				return
			}
		}
	}
	select {
	case resultCh <- ClusteringResult{}:
	case <-ctx.done:
		log.Println("Clustering Pool Worker interrupted while DoWork (2)")
	}
}

// A modified version of the distinct column transformation operator.
// column2Pos correspond to the position of column2 in the input row.
type distinctCountCorrelationEval struct {
	column2        *string
	column2Pos     int
	distinctValues map[string]bool
	nonNilCount    int
}

func (eval *distinctCountCorrelationEval) Apply(input []interface{}) {
	value := input[eval.column2Pos]
	if value != nil {
		str, ok := value.(string)
		if !ok {
			str = fmt.Sprintf("%v", value)
		}
		if len(str) > 0 {
			eval.distinctValues[str] = true
			eval.nonNilCount += 1
		}
	}
}