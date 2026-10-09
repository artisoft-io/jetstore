package compute_pipes

import (
	"fmt"
	"log"
)

// Worker for clustering operator

type ClusteringWorker struct {
	config                *ClusteringSpec
	source                *InputChannel
	column1               *string
	columns2              []string
	correlationEvaluators []*distinctCountCorrelationEval
	outputChannel         *OutputChannel
	done                  chan struct{}
	errCh                 chan error
}

// source and outputChannel are provided for their spec, the data is sent and recieved
// via inputCh and outputCh
func NewClusteringWorker(config *ClusteringSpec, source *InputChannel,
	column1 *string, columns2 []string, outputChannel *OutputChannel,
	done chan struct{}, errCh chan error) *ClusteringWorker {
	// Create the evaluators
	evaluators := make([]*distinctCountCorrelationEval, 0, len(columns2))
	for _, c := range columns2 {
		if c != *column1 {
			evaluators = append(evaluators, &distinctCountCorrelationEval{
				column2:    &c,
				column2Pos: (*source.Columns)[c],
				values:     newValueHistogram(),
			})
		}
	}
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
	name1Pos := (*ctx.outputChannel.Columns)["column_name_1"]
	name2Pos := (*ctx.outputChannel.Columns)["column_name_2"]
	countPos := (*ctx.outputChannel.Columns)["distinct_count"]
	totalPos := (*ctx.outputChannel.Columns)["total_non_nil_count"]
	nLogNPos := (*ctx.outputChannel.Columns)["joint_n_log_n"]
	for _, evaluator := range ctx.correlationEvaluators {
		if evaluator.values.total > ctx.config.MinColumn2NonNilCount {
			result := make([]any, len(ctx.outputChannel.Config.Columns))
			result[name1Pos] = *ctx.column1
			result[name2Pos] = *evaluator.column2
			result[countPos] = evaluator.values.distinctCount()
			result[totalPos] = evaluator.values.total
			// The group's contribution to the joint entropy. The pool manager
			// sums it across the groups of one column1, which is exact because
			// the groups partition the rows.
			result[nLogNPos] = evaluator.values.sumNLog
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
//
// It kept a set of the distinct values seen until 2026-09-05 and keeps their
// counts now. The map is the same size; what the counts buy is the group's
// value distribution, which is what the joint entropy is computed from.
type distinctCountCorrelationEval struct {
	column2    *string
	column2Pos int
	values     *valueHistogram
}

func (eval *distinctCountCorrelationEval) Apply(input []interface{}) {
	value := input[eval.column2Pos]
	if value != nil {
		str, ok := value.(string)
		if !ok {
			str = fmt.Sprintf("%v", value)
		}
		if len(str) > 0 {
			eval.values.add(str)
		}
	}
}
