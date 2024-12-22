package compute_pipes

import (
	"fmt"
	"log"
)

// Worker to perform jetrules execute rules function

type ClusteringWorker struct {
	config                *ClusteringSpec
	source                *InputChannel
	column1               string
	columns2              []string
	correlationEvaluators []*distinctCountCorrelationEval
	outputChannel         *OutputChannel
	done                  chan struct{}
	errCh                 chan error
}

// source and outputChannel are provided for their spec, the data is sent and recieved
// via inputCh and outputCh
func NewClusteringWorker(config *ClusteringSpec, source *InputChannel,
	column1 string, columns2 []string, outputChannel *OutputChannel, 
	done chan struct{}, errCh chan error) *ClusteringWorker {
	// Create the evaluators
	evaluators := make([]*distinctCountCorrelationEval, 0, len(columns2))
	for _, c := range columns2 {
		evaluators = append(evaluators, &distinctCountCorrelationEval{
			columnPos:      source.columns[c],
			distinctValues: make(map[string]bool),
		})
	}

	// log.Println("New Pool Worker Created")
	return &ClusteringWorker{
		config:                config,
		source:                source,
		column1:               column1,
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
	for i, column2 := range ctx.columns2 {
		result := make([]any, len(ctx.outputChannel.config.Columns))
		result[ctx.outputChannel.columns["column_name_1"]] = ctx.column1
		result[ctx.outputChannel.columns["column_name_2"]] = column2
		distinctCount := len(ctx.correlationEvaluators[i].distinctValues)
		result[ctx.outputChannel.columns["distinct_count"]] = distinctCount
		if ctx.correlationEvaluators[i].nonNilCount > 0 {
			distinctCountPct := float64(distinctCount) / float64(ctx.correlationEvaluators[i].nonNilCount) * 100.0
			result[ctx.outputChannel.columns["distinct_count_pct"]] = distinctCountPct
		}
		// Send the out the result
		select {
		case outputCh <- result:
		case <-ctx.done:
			log.Println("Clustering Pool Worker interrupted while DoWork")
			return
		}
	}
	select {
	case resultCh <- ClusteringResult{}:
	case <-ctx.done:
		log.Println("Clustering Pool Worker interrupted while DoWork (2)")
	}
}

// A modified version of the distinct column transformation operator.
type distinctCountCorrelationEval struct {
	columnPos      int
	distinctValues map[string]bool
	nonNilCount    int
}

func (eval *distinctCountCorrelationEval) Apply(input []interface{}) error {
	value := input[eval.columnPos]
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
	return nil
}
